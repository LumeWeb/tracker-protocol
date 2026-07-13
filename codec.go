package urma

import (
	"encoding/json"
	"fmt"

	"github.com/invopop/jsonschema"
)

// ValidateClaimSize checks that a serialized UrmaClaim fits on-chain when
// encoded as an unsigned claim value envelope. The total on-chain size is:
//
//	ClaimScriptOverhead + valuePushPrefix + EnvelopeUnsignedOverhead + JSON
//
// where valuePushPrefix depends on the envelope size (1-3 bytes per the
// canonical push encoding in lbcd's script builder).
func ValidateClaimSize(claim UrmaClaim) error {
	data, err := EncodeClaim(claim)
	if err != nil {
		return err
	}
	total := ClaimScriptOverhead + ValuePushSize(EnvelopeUnsignedOverhead+len(data)) + EnvelopeUnsignedOverhead + len(data)
	if total > MaxClaimScriptSize {
		return fmt.Errorf("claim size %d exceeds maximum %d", total, MaxClaimScriptSize)
	}
	return nil
}

// ValidateClaimSizeSigned checks that a serialized UrmaClaim fits on-chain
// when encoded as a signed claim value envelope. The total on-chain size is:
//
//	ClaimScriptOverhead + valuePushPrefix + EnvelopeSignedOverhead + JSON
//
// The signed envelope adds EnvelopeSignedOverhead (85 bytes) for the version
// byte, channel ClaimID, and signature.
func ValidateClaimSizeSigned(claim UrmaClaim) error {
	data, err := EncodeClaim(claim)
	if err != nil {
		return err
	}
	total := ClaimScriptOverhead + ValuePushSize(EnvelopeSignedOverhead+len(data)) + EnvelopeSignedOverhead + len(data)
	if total > MaxClaimScriptSize {
		return fmt.Errorf("signed claim size %d exceeds maximum %d", total, MaxClaimScriptSize)
	}
	return nil
}

// ValuePushSize returns the number of bytes the canonical script push prefix
// occupies for a data element of the given length, matching lbcd's
// canonicalDataSize logic.
func ValuePushSize(dataLen int) int {
	switch {
	case dataLen == 0:
		return 1
	case dataLen < 0x4c: // OP_PUSHDATA1
		return 1
	case dataLen <= 0xff:
		return 2
	case dataLen <= 0xffff:
		return 3
	default:
		return 5
	}
}

// EncodeClaim serializes a UrmaClaim to JSON.
func EncodeClaim(claim UrmaClaim) ([]byte, error) {
	return json.Marshal(claim)
}

// DecodeClaim deserializes a UrmaClaim from JSON.
func DecodeClaim(data []byte) (UrmaClaim, error) {
	var claim UrmaClaim
	if err := json.Unmarshal(data, &claim); err != nil {
		return UrmaClaim{}, fmt.Errorf("decode claim: %w", err)
	}
	return claim, nil
}

// GenerateSchema generates a JSON Schema for the UrmaClaim type.
func GenerateSchema() ([]byte, error) {
	reflector := jsonschema.Reflector{
		DoNotReference: true,
	}
	schema := reflector.Reflect(UrmaClaim{})
	return json.MarshalIndent(schema, "", "  ")
}
