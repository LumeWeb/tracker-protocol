package urma

import (
	"encoding/hex"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

func TestEncodeDecode_UnsignedEnvelope(t *testing.T) {
	jsonPayload := []byte(`{"version":0,"location":"sia"}`)

	envelope := EncodeUnsignedEnvelope(jsonPayload)

	decoded, version, _, _, err := DecodeEnvelope(envelope)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if version != EnvelopeUnsigned {
		t.Errorf("version: 0x%02x", version)
	}
	if string(decoded) != string(jsonPayload) {
		t.Errorf("payload mismatch: %s", decoded)
	}
}

func TestEncodeDecode_SignedEnvelope(t *testing.T) {
	privKey, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	jsonPayload := []byte(`{"version":0,"location":"sia"}`)
	channelClaimID := [ChannelClaimIDLen]byte{0xa1, 0xb2, 0xc3}
	txInput0Hash := [OutpointLen]byte{}
	copy(txInput0Hash[:], []byte("0123456789abcdef0123456789abcdef0123"))

	envelope, err := EncodeSignedEnvelope(jsonPayload, channelClaimID, privKey, txInput0Hash)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	if len(envelope) != EnvelopeSignedOverhead+len(jsonPayload) {
		t.Errorf("envelope size: %d, expected %d", len(envelope), EnvelopeSignedOverhead+len(jsonPayload))
	}
	if envelope[EnvelopeVersionOffset] != EnvelopeSigned {
		t.Errorf("version byte: 0x%02x", envelope[EnvelopeVersionOffset])
	}

	decoded, version, decChannelID, sig, err := DecodeEnvelope(envelope)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if version != EnvelopeSigned {
		t.Errorf("version: 0x%02x", version)
	}
	if decChannelID != channelClaimID {
		t.Errorf("channel ID mismatch: %x", decChannelID)
	}
	if string(decoded) != string(jsonPayload) {
		t.Errorf("payload mismatch: %s", decoded)
	}
	if sig == [SignatureLen]byte{} {
		t.Error("signature is zero")
	}
}

func TestVerifySignedEnvelope_Valid(t *testing.T) {
	privKey, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pubKey := privKey.PubKey()

	jsonPayload := []byte(`{"version":0,"location":"sia","sourceClaimId":"a1b2c3"}`)
	channelClaimID := [ChannelClaimIDLen]byte{0xa1, 0xb2, 0xc3}
	txInput0Hash := [OutpointLen]byte{}
	for i := range txInput0Hash {
		txInput0Hash[i] = byte(i)
	}

	envelope, err := EncodeSignedEnvelope(jsonPayload, channelClaimID, privKey, txInput0Hash)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	if err := VerifySignedEnvelope(envelope, txInput0Hash, pubKey); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestVerifySignedEnvelope_WrongKey(t *testing.T) {
	privKey, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	wrongPrivKey, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	wrongPubKey := wrongPrivKey.PubKey()

	jsonPayload := []byte(`{"version":0}`)
	channelClaimID := [ChannelClaimIDLen]byte{0x01}
	txInput0Hash := [OutpointLen]byte{0xff}

	envelope, err := EncodeSignedEnvelope(jsonPayload, channelClaimID, privKey, txInput0Hash)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	if err := VerifySignedEnvelope(envelope, txInput0Hash, wrongPubKey); err == nil {
		t.Error("expected verification failure with wrong key")
	}
}

func TestVerifySignedEnvelope_WrongTxInput(t *testing.T) {
	privKey, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pubKey := privKey.PubKey()

	jsonPayload := []byte(`{"version":0}`)
	channelClaimID := [ChannelClaimIDLen]byte{0x01}
	txInput0Hash := [OutpointLen]byte{0xaa}

	envelope, err := EncodeSignedEnvelope(jsonPayload, channelClaimID, privKey, txInput0Hash)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	wrongTxInput := [OutpointLen]byte{0xbb}
	if err := VerifySignedEnvelope(envelope, wrongTxInput, pubKey); err == nil {
		t.Error("expected verification failure with wrong tx input")
	}
}

func TestVerifySignedEnvelope_UnsignedRejected(t *testing.T) {
	jsonPayload := []byte(`{"version":0}`)
	envelope := EncodeUnsignedEnvelope(jsonPayload)
	txInput0Hash := [OutpointLen]byte{}
	privKey, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pubKey := privKey.PubKey()

	if err := VerifySignedEnvelope(envelope, txInput0Hash, pubKey); err == nil {
		t.Error("expected error verifying unsigned envelope as signed")
	}
}

func TestDecodeEnvelope_UnknownVersion(t *testing.T) {
	data := []byte{0x99, 0x01, 0x02}
	_, _, _, _, err := DecodeEnvelope(data)
	if err == nil {
		t.Error("expected error for unknown version")
	}
}

func TestDecodeEnvelope_Empty(t *testing.T) {
	_, _, _, _, err := DecodeEnvelope([]byte{})
	if err == nil {
		t.Error("expected error for empty envelope")
	}
}

func TestDecodeEnvelope_SignedTooShort(t *testing.T) {
	data := []byte{EnvelopeSigned, 0x01, 0x02}
	_, _, _, _, err := DecodeEnvelope(data)
	if err == nil {
		t.Error("expected error for short signed envelope")
	}
}

func TestComputeSignatureDigest(t *testing.T) {
	txInput0Hash := [OutpointLen]byte{0x01}
	channelClaimID := [ChannelClaimIDLen]byte{0x02}
	claimJSON := []byte(`{"version":0}`)

	digest := ComputeSignatureDigest(txInput0Hash, channelClaimID, claimJSON)

	allZero := true
	for _, b := range digest {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("digest is all zeros")
	}

	digest2 := ComputeSignatureDigest(txInput0Hash, channelClaimID, claimJSON)
	if digest != digest2 {
		t.Error("digest is not deterministic")
	}

	digest3 := ComputeSignatureDigest([OutpointLen]byte{0x03}, channelClaimID, claimJSON)
	if digest == digest3 {
		t.Error("different input should produce different digest")
	}
}

func TestOutpointToBytes(t *testing.T) {
	txid := [TxIDLen]byte{}
	for i := range txid {
		txid[i] = byte(i)
	}
	vout := uint32(7)

	op := OutpointToBytes(txid, vout)

	for i := 0; i < TxIDLen; i++ {
		if op[i] != txid[i] {
			t.Errorf("byte %d: %x vs %x", i, op[i], txid[i])
		}
	}
	if op[TxIDLen] != 7 || op[TxIDLen+1] != 0 || op[TxIDLen+2] != 0 || op[TxIDLen+3] != 0 {
		t.Errorf("vout bytes: %x", op[TxIDLen:])
	}
}

func TestEnvelopeConstants(t *testing.T) {
	if EnvelopeSignedOverhead != EnvelopeVersionLen+ChannelClaimIDLen+SignatureLen {
		t.Errorf("SignedOverhead mismatch: %d", EnvelopeSignedOverhead)
	}
	if EnvelopeSignedOverhead != 85 {
		t.Errorf("expected 85, got %d", EnvelopeSignedOverhead)
	}
	if SignatureLen != 64 {
		t.Errorf("expected 64, got %d", SignatureLen)
	}
	if SignatureCompactLen != SignatureLen+1 {
		t.Errorf("expected %d, got %d", SignatureLen+1, SignatureCompactLen)
	}
	if ChannelClaimIDLen != 20 {
		t.Errorf("expected 20, got %d", ChannelClaimIDLen)
	}
	if EnvelopeChannelIDOffset != EnvelopeVersionLen {
		t.Errorf("ChannelIDOffset mismatch: %d", EnvelopeChannelIDOffset)
	}
	if EnvelopeSignatureOffset != EnvelopeVersionLen+ChannelClaimIDLen {
		t.Errorf("SignatureOffset mismatch: %d", EnvelopeSignatureOffset)
	}
	if EnvelopePayloadOffset != EnvelopeVersionLen+ChannelClaimIDLen+SignatureLen {
		t.Errorf("PayloadOffset mismatch: %d", EnvelopePayloadOffset)
	}
	if OutpointLen != TxIDLen+VoutLen {
		t.Errorf("OutpointLen mismatch: %d", OutpointLen)
	}
	if OutpointLen != 36 {
		t.Errorf("expected 36, got %d", OutpointLen)
	}
	// ClaimNameLen derived from SourceClaimIDLen
	if ClaimNameLen != len(urmaNamePrefix)+SourceClaimIDLen*2 {
		t.Errorf("ClaimNameLen = %d, expected %d", ClaimNameLen, len(urmaNamePrefix)+SourceClaimIDLen*2)
	}
	if ClaimNameLen != 42 {
		t.Errorf("expected 42, got %d", ClaimNameLen)
	}
	// ClaimScriptOverhead derived from components
	if ClaimScriptOverhead != OpcodeClaimName+ClaimNamePushPrefix+ClaimNameLen+Opcode2Drop+OpcodeDrop {
		t.Errorf("ClaimScriptOverhead = %d, expected %d", ClaimScriptOverhead, OpcodeClaimName+ClaimNamePushPrefix+ClaimNameLen+Opcode2Drop+OpcodeDrop)
	}
	if ClaimScriptOverhead != 46 {
		t.Errorf("expected 46, got %d", ClaimScriptOverhead)
	}
}

func TestUrmaClaimName(t *testing.T) {
	id := SourceClaimID{0xa1, 0xb2, 0xc3, 0xd4, 0xe5, 0xf6, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e}
	name := UrmaClaimName(id)
	if len(name) != ClaimNameLen {
		t.Errorf("name length %d, expected %d", len(name), ClaimNameLen)
	}
	if name[:2] != "u-" {
		t.Errorf("expected 'u-' prefix, got %q", name[:2])
	}
	// Verify hex encoding matches
	expected := "u-a1b2c3d4e5f60102030405060708090a0b0c0d0e"
	if name != expected {
		t.Errorf("got %q, expected %q", name, expected)
	}
	// Zero value
	zeroName := UrmaClaimName(SourceClaimID{})
	if zeroName != "u-0000000000000000000000000000000000000000" {
		t.Errorf("zero name: %q", zeroName)
	}
}

func TestIsEnvelope(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"unsigned", []byte{0x00, 0x01, 0x02}, true},
		{"signed", append([]byte{0x01}, make([]byte, 84)...), true},
		{"unknown version", []byte{0x99, 0x01}, false},
		{"empty", []byte{}, false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsEnvelope(tt.data); got != tt.want {
				t.Errorf("IsEnvelope() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsUnsignedEnvelope(t *testing.T) {
	if !IsUnsignedEnvelope([]byte{0x00, 0x01}) {
		t.Error("expected true for unsigned envelope")
	}
	if IsUnsignedEnvelope([]byte{0x01, 0x01}) {
		t.Error("expected false for signed envelope")
	}
	if IsUnsignedEnvelope([]byte{}) {
		t.Error("expected false for empty")
	}
}

func TestIsSignedEnvelope(t *testing.T) {
	signedData := make([]byte, EnvelopeSignedOverhead)
	signedData[0] = EnvelopeSigned
	if !IsSignedEnvelope(signedData) {
		t.Error("expected true for valid signed envelope")
	}
	if IsSignedEnvelope([]byte{0x01, 0x00}) {
		t.Error("expected false for too-short signed envelope")
	}
	if IsSignedEnvelope([]byte{0x00, 0x01}) {
		t.Error("expected false for unsigned envelope")
	}
}

func TestEncodeDecode_SignedEnvelope_LBRYCompat(t *testing.T) {
	privKey, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	jsonPayload := []byte(`{"version":0}`)
	channelClaimID := [ChannelClaimIDLen]byte{0x01}
	txInput0Hash := [OutpointLen]byte{0x02}

	envelope, err := EncodeSignedEnvelope(jsonPayload, channelClaimID, privKey, txInput0Hash)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	if envelope[EnvelopeVersionOffset] != EnvelopeSigned {
		t.Errorf("byte 0: 0x%02x", envelope[EnvelopeVersionOffset])
	}
	for i := 0; i < ChannelClaimIDLen; i++ {
		if envelope[EnvelopeChannelIDOffset+i] != channelClaimID[i] {
			t.Errorf("channel ID byte %d: %x", i, envelope[EnvelopeChannelIDOffset+i])
		}
	}
	sigBytes := envelope[EnvelopeSignatureOffset:EnvelopePayloadOffset]
	if len(sigBytes) != SignatureLen {
		t.Errorf("signature length: %d", len(sigBytes))
	}
	if string(envelope[EnvelopePayloadOffset:]) != string(jsonPayload) {
		t.Errorf("JSON payload mismatch")
	}

	t.Logf("envelope: %s...", hex.EncodeToString(envelope[:10]))
}
