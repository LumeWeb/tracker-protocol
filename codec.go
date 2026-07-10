package trackerprotocol

import (
	"encoding/json"
	"fmt"

	"github.com/invopop/jsonschema"
)

// ValidateClaimSize checks that a serialized TrackerClaim does not exceed
// MaxClaimSize (8192 bytes).
func ValidateClaimSize(claim TrackerClaim) error {
	data, err := EncodeClaim(claim)
	if err != nil {
		return err
	}
	if len(data) > MaxClaimSize {
		return fmt.Errorf("claim size %d exceeds maximum %d", len(data), MaxClaimSize)
	}
	return nil
}

// EncodeClaim serializes a TrackerClaim to JSON.
func EncodeClaim(claim TrackerClaim) ([]byte, error) {
	return json.Marshal(claim)
}

// DecodeClaim deserializes a TrackerClaim from JSON.
func DecodeClaim(data []byte) (TrackerClaim, error) {
	var claim TrackerClaim
	if err := json.Unmarshal(data, &claim); err != nil {
		return TrackerClaim{}, fmt.Errorf("decode claim: %w", err)
	}
	return claim, nil
}

// GenerateSchema generates a JSON Schema for the TrackerClaim type.
func GenerateSchema() ([]byte, error) {
	reflector := jsonschema.Reflector{
		DoNotReference: true,
	}
	schema := reflector.Reflect(TrackerClaim{})
	return json.MarshalIndent(schema, "", "  ")
}
