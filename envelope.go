package urma

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
)

// urmaNamePrefix is the prefix for all urma claim names.
const urmaNamePrefix = "u-"

// Claim script opcodes and structural sizes.
const (
	// OpcodeClaimName is the byte size of the OP_CLAIMNAME opcode.
	OpcodeClaimName = 1

	// ClaimNamePushPrefix is the push prefix size for a urma claim name.
	// urma names are 42 bytes (< 76), so the canonical push is 1 byte.
	ClaimNamePushPrefix = 1

	// Opcode2Drop is the byte size of the OP_2DROP opcode.
	Opcode2Drop = 1

	// OpcodeDrop is the byte size of the OP_DROP opcode.
	OpcodeDrop = 1
)

// Envelope version bytes and sizes.
const (
	EnvelopeUnsigned byte = 0x00
	EnvelopeSigned   byte = 0x01

	// EnvelopeVersionLen is the byte size of the envelope version prefix.
	EnvelopeVersionLen = 1

	// ChannelClaimIDLen is the length of a channel ClaimID in bytes.
	ChannelClaimIDLen = 20

	// SignatureLen is the length of a compact ECDSA signature (r||s) in bytes.
	SignatureLen = 64

	// SignatureCompactLen is the length of a recoverable compact ECDSA
	// signature as produced by ecdsa.SignCompact: 1 recovery byte + 32 r + 32 s.
	SignatureCompactLen = SignatureLen + 1

	// SignatureRecoveryByteIdx is the index of the recovery byte in a
	// SignCompact result. LBRY uses non-recoverable 64-byte (r||s) signatures,
	// so this byte is stripped.
	SignatureRecoveryByteIdx = 0

	// EnvelopeVersionOffset is the byte offset of the version byte in an
	// envelope. Always 0, but named for readability.
	EnvelopeVersionOffset = 0

	// EnvelopeChannelIDOffset is the byte offset of the channel ClaimID field
	// in a signed envelope.
	EnvelopeChannelIDOffset = EnvelopeVersionLen

	// EnvelopeSignatureOffset is the byte offset of the signature field in a
	// signed envelope.
	EnvelopeSignatureOffset = EnvelopeVersionLen + ChannelClaimIDLen

	// EnvelopePayloadOffset is the byte offset of the JSON payload in a signed
	// envelope.
	EnvelopePayloadOffset = EnvelopeVersionLen + ChannelClaimIDLen + SignatureLen

	// TxIDLen is the byte length of a Bitcoin/LBRY transaction hash.
	TxIDLen = 32

	// VoutLen is the byte length of a transaction output index (uint32 LE).
	VoutLen = 4

	// OutpointLen is the byte length of a full outpoint (txid + vout).
	OutpointLen = TxIDLen + VoutLen

	// EnvelopeUnsignedOverhead is the number of bytes added by the unsigned
	// envelope: 1 version byte.
	EnvelopeUnsignedOverhead = EnvelopeVersionLen

	// EnvelopeSignedOverhead is the number of bytes added by the signed
	// envelope: 1 version + 20 channel ClaimID + 64 signature.
	EnvelopeSignedOverhead = EnvelopeVersionLen + ChannelClaimIDLen + SignatureLen

	// ClaimNameLen is the length of a urma claim name: "u-" prefix + 40 hex
	// chars (20-byte SourceClaimID). Derived from SourceClaimIDLen so it stays
	// in sync if the ClaimID length ever changes.
	ClaimNameLen = len(urmaNamePrefix) + SourceClaimIDLen*2

	// ClaimScriptOverhead is the fixed overhead in a claim script excluding
	// the value push and value itself. A claim name script is:
	//   OP_CLAIMNAME <name> <value> OP_2DROP OP_DROP
	// For a ClaimNameLen-byte urma name, this is:
	//   OpcodeClaimName + ClaimNamePushPrefix + ClaimNameLen + Opcode2Drop + OpcodeDrop
	ClaimScriptOverhead = OpcodeClaimName + ClaimNamePushPrefix + ClaimNameLen + Opcode2Drop + OpcodeDrop
)

// EncodeUnsignedEnvelope wraps a UrmaClaim JSON payload in an unsigned
// envelope: [0x00][JSON bytes].
func EncodeUnsignedEnvelope(claimJSON []byte) []byte {
	out := make([]byte, EnvelopeUnsignedOverhead+len(claimJSON))
	out[EnvelopeVersionOffset] = EnvelopeUnsigned
	copy(out[EnvelopeUnsignedOverhead:], claimJSON)
	return out
}

// EncodeSignedEnvelope wraps a UrmaClaim JSON payload in a signed envelope:
// [0x01][20-byte channel ClaimID][64-byte signature][JSON bytes].
//
// The signature is a compact ECDSA (r||s) signature over the SECP256k1 curve.
// The digest is SHA-256(tx_input_0_hash || channel_claim_id || json_bytes).
//
// txInput0Hash is the OutpointLen-byte OutPoint (TxIDLen-byte txid +
// VoutLen-byte vout) of the first transaction input.
func EncodeSignedEnvelope(claimJSON []byte, channelClaimID [ChannelClaimIDLen]byte, privKey *secp256k1.PrivateKey, txInput0Hash [OutpointLen]byte) ([]byte, error) {
	digest := ComputeSignatureDigest(txInput0Hash, channelClaimID, claimJSON)
	sig := ecdsa.SignCompact(privKey, digest[:], false)
	// SignCompact returns SignatureCompactLen bytes: [recovery][r][s].
	// LBRY uses 64-byte compact (r||s) without the recovery byte.
	if len(sig) != SignatureCompactLen {
		return nil, fmt.Errorf("unexpected signature length: %d", len(sig))
	}

	out := make([]byte, EnvelopeSignedOverhead+len(claimJSON))
	out[EnvelopeVersionOffset] = EnvelopeSigned
	copy(out[EnvelopeChannelIDOffset:EnvelopeSignatureOffset], channelClaimID[:])
	copy(out[EnvelopeSignatureOffset:EnvelopePayloadOffset], sig[SignatureRecoveryByteIdx+1:]) // strip recovery byte, keep r||s
	copy(out[EnvelopePayloadOffset:], claimJSON)
	return out, nil
}

// DecodeEnvelope unwraps a claim value envelope. It returns:
//   - The UrmaClaim JSON payload bytes
//   - The envelope version byte (0x00 or 0x01)
//   - For signed envelopes: the channel ClaimID and 64-byte signature
//   - For signed envelopes: the remaining bytes for digest computation
//
// For unsigned envelopes, channelClaimID and signature are zero values.
func DecodeEnvelope(data []byte) (claimJSON []byte, version byte, channelClaimID [ChannelClaimIDLen]byte, signature [SignatureLen]byte, err error) {
	if len(data) < EnvelopeVersionLen {
		return nil, 0, channelClaimID, signature, errors.New("empty envelope")
	}

	version = data[EnvelopeVersionOffset]
	switch version {
	case EnvelopeUnsigned:
		return data[EnvelopeUnsignedOverhead:], EnvelopeUnsigned, channelClaimID, signature, nil
	case EnvelopeSigned:
		if len(data) < EnvelopeSignedOverhead {
			return nil, 0, channelClaimID, signature, fmt.Errorf("signed envelope too short: %d bytes", len(data))
		}
		copy(channelClaimID[:], data[EnvelopeChannelIDOffset:EnvelopeSignatureOffset])
		copy(signature[:], data[EnvelopeSignatureOffset:EnvelopePayloadOffset])
		return data[EnvelopePayloadOffset:], EnvelopeSigned, channelClaimID, signature, nil
	default:
		return nil, 0, channelClaimID, signature, fmt.Errorf("unknown envelope version: 0x%02x", version)
	}
}

// ComputeSignatureDigest computes the SHA-256 digest used for claim value
// signatures:
//
//	digest = SHA-256(tx_input_0_hash || channel_claim_id || claim_json_bytes)
//
// txInput0Hash is the OutpointLen-byte OutPoint of the first transaction input.
// channelClaimID is the ChannelClaimIDLen-byte ClaimID of the signing channel.
// claimJSONBytes is the raw UrmaClaim JSON payload.
func ComputeSignatureDigest(txInput0Hash [OutpointLen]byte, channelClaimID [ChannelClaimIDLen]byte, claimJSONBytes []byte) [32]byte {
	h := sha256.New()
	h.Write(txInput0Hash[:])
	h.Write(channelClaimID[:])
	h.Write(claimJSONBytes)
	var digest [32]byte
	copy(digest[:], h.Sum(nil))
	return digest
}

// VerifySignedEnvelope verifies a signed claim value envelope against a
// channel public key. It performs the full 7-step verification from the spec:
//
//  1. Read byte 0 (must be 0x01)
//  2. Read bytes 1-20: channel ClaimID
//  3. Read bytes 21-84: ECDSA signature
//  4. Read bytes 85+: UrmaClaim JSON payload
//  5. Channel public key provided by caller (fetched from channel claim)
//  6. Recompute SHA-256(tx_input_0_hash || channel_claim_id || json_bytes)
//  7. Verify signature against digest using channel public key
//
// txInput0Hash is the OutpointLen-byte OutPoint of the first transaction input
// of the transaction that created the claim.
func VerifySignedEnvelope(envelope []byte, txInput0Hash [OutpointLen]byte, pubKey *secp256k1.PublicKey) error {
	claimJSON, version, channelClaimID, signature, err := DecodeEnvelope(envelope)
	if err != nil {
		return fmt.Errorf("decode envelope: %w", err)
	}
	if version != EnvelopeSigned {
		return errors.New("not a signed envelope")
	}

	digest := ComputeSignatureDigest(txInput0Hash, channelClaimID, claimJSON)

	// Parse the 64-byte compact signature (r||s) into r and s ModNScalars,
	// then construct an ecdsa.Signature and verify directly.
	var r, s secp256k1.ModNScalar
	r.SetByteSlice(signature[:SignatureLen/2])
	s.SetByteSlice(signature[SignatureLen/2:])

	sig := ecdsa.NewSignature(&r, &s)
	if !sig.Verify(digest[:], pubKey) {
		return errors.New("signature verification failed")
	}
	// Enforce canonical low-s to match lbcd consensus rules and prevent
	// signature malleability (s and n-s both verify otherwise).
	if s.IsOverHalfOrder() {
		return errors.New("signature is not canonical (high-s)")
	}
	return nil
}

// outpointToBytes converts a transaction outpoint (txid + vout) to the
// OutpointLen-byte representation used in signature digests. txid is the
// TxIDLen-byte transaction hash (little-endian as stored in wire format),
// and vout is the output index.
func OutpointToBytes(txid [TxIDLen]byte, vout uint32) [OutpointLen]byte {
	var out [OutpointLen]byte
	copy(out[:TxIDLen], txid[:])
	binary.LittleEndian.PutUint32(out[TxIDLen:], vout)
	return out
}

// IsEnvelope reports whether data begins with a valid envelope version byte
// (0x00 or 0x01). It does not validate the payload or check minimum length
// beyond byte 0.
func IsEnvelope(data []byte) bool {
	if len(data) < EnvelopeVersionLen {
		return false
	}
	return data[EnvelopeVersionOffset] == EnvelopeUnsigned || data[EnvelopeVersionOffset] == EnvelopeSigned
}

// IsUnsignedEnvelope reports whether data is an unsigned envelope (byte 0 == 0x00).
func IsUnsignedEnvelope(data []byte) bool {
	return len(data) >= EnvelopeVersionLen && data[EnvelopeVersionOffset] == EnvelopeUnsigned
}

// IsSignedEnvelope reports whether data is a signed envelope (byte 0 == 0x01)
// and is at least EnvelopeSignedOverhead bytes long.
func IsSignedEnvelope(data []byte) bool {
	return len(data) >= EnvelopeSignedOverhead && data[EnvelopeVersionOffset] == EnvelopeSigned
}

// UrmaClaimName returns the on-chain claim name for a given source ClaimID.
// The format is "u-" followed by the 40-character lowercase hex encoding of
// the 20-byte SourceClaimID, matching the spec's unified naming scheme.
func UrmaClaimName(sourceClaimID SourceClaimID) string {
	return urmaNamePrefix + hex.EncodeToString(sourceClaimID[:])
}
