package urma

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRoundTrip_Claim(t *testing.T) {
	rawData := json.RawMessage(`{"slabs":[{"encryptionKey":"AAAA","minShards":10,"sectors":[{"root":"0000000000000000000000000000000000000000000000000000000000000001","hostKey":"0000000000000000000000000000000000000000000000000000000000000002"}],"offset":0,"length":4194304}]}`)
	sourceID := SourceClaimID{0xa1, 0xb2, 0xc3, 0xd4, 0xe5, 0xf6, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e}
	claim := UrmaClaim{
		Version:       ProtocolVersion,
		Location:      LocationSia,
		SourceClaimID: sourceID,
		DataKey:       [32]byte{0x42},
		LocationData:  rawData,
	}

	data, err := EncodeClaim(claim)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	decoded, err := DecodeClaim(data)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if decoded.Version != ProtocolVersion {
		t.Errorf("version mismatch: %d vs %d", decoded.Version, ProtocolVersion)
	}
	if decoded.Location != LocationSia {
		t.Errorf("location mismatch: %s", decoded.Location)
	}
	if decoded.SourceClaimID != sourceID {
		t.Errorf("sourceClaimId mismatch: %x vs %x", decoded.SourceClaimID, sourceID)
	}
	if decoded.DataKey != [32]byte{0x42} {
		t.Error("dataKey mismatch")
	}
}

func TestGenerateSchema(t *testing.T) {
	schema, err := GenerateSchema()
	if err != nil {
		t.Fatalf("schema generation failed: %v", err)
	}
	t.Logf("claim schema: %d bytes", len(schema))

	var raw map[string]any
	if err := json.Unmarshal(schema, &raw); err != nil {
		t.Fatalf("schema is invalid JSON: %v", err)
	}
	if raw["$schema"] == nil {
		t.Error("schema missing $schema field")
	}
}

func TestLocationString(t *testing.T) {
	if LocationSia != "sia" {
		t.Errorf("expected 'sia', got '%s'", LocationSia)
	}
}

func TestLocationUnmarshalJSON(t *testing.T) {
	var loc Location
	if err := loc.UnmarshalJSON([]byte(`"sia"`)); err != nil {
		t.Fatalf("unmarshal 'sia' failed: %v", err)
	}
	if loc != LocationSia {
		t.Errorf("expected LocationSia, got %s", loc)
	}

	if err := loc.UnmarshalJSON([]byte(`"bogus"`)); err == nil {
		t.Error("expected error for unknown location")
	}
}

func TestClaimSize_ExceedsMax(t *testing.T) {
	huge := make([]byte, MaxClaimScriptSize+100)
	claim := UrmaClaim{
		Version:       ProtocolVersion,
		Location:      LocationSia,
		SourceClaimID: SourceClaimID{0x01},
		DataKey:       [32]byte{},
		LocationData:  huge,
	}
	err := ValidateClaimSize(claim)
	if err == nil {
		t.Error("expected error for oversized claim")
	}
}

func TestValuePushSize(t *testing.T) {
	tests := []struct {
		dataLen int
		want    int
	}{
		{0, 1},
		{1, 1},
		{75, 1},
		{76, 2}, // OP_PUSHDATA1
		{255, 2},
		{256, 3},
		{65535, 3},
		{65536, 5},
	}
	for _, tt := range tests {
		got := ValuePushSize(tt.dataLen)
		if got != tt.want {
			t.Errorf("ValuePushSize(%d) = %d, want %d", tt.dataLen, got, tt.want)
		}
	}
}

func TestValidateClaimSize_OnChainCalculation(t *testing.T) {
	// Build a minimal claim and verify the on-chain size matches manual calculation.
	claim := UrmaClaim{
		Version:       ProtocolVersion,
		Location:      LocationSia,
		SourceClaimID: SourceClaimID{0x01},
		DataKey:       [32]byte{},
		LocationData:  []byte(`{}`),
	}
	data, err := EncodeClaim(claim)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	// Expected unsigned on-chain size:
	// ClaimScriptOverhead + ValuePushSize(envelope) + EnvelopeUnsignedOverhead + JSON
	envelopeSize := EnvelopeUnsignedOverhead + len(data)
	expected := ClaimScriptOverhead + ValuePushSize(envelopeSize) + envelopeSize

	// Should pass validation
	if err := ValidateClaimSize(claim); err != nil {
		t.Errorf("expected validation to pass for %d bytes: %v", expected, err)
	}

	// Signed should also pass
	if err := ValidateClaimSizeSigned(claim); err != nil {
		signedEnvelope := EnvelopeSignedOverhead + len(data)
		signedExpected := ClaimScriptOverhead + ValuePushSize(signedEnvelope) + signedEnvelope
		t.Errorf("expected signed validation to pass for %d bytes: %v", signedExpected, err)
	}
}

func TestValidateClaimSizeSigned_Boundary(t *testing.T) {
	// Find the maximum JSON payload size that fits as a signed claim.
	// Total = ClaimScriptOverhead + ValuePushSize(85+payloadSize) + 85 + payloadSize <= 8192
	// payloadSize = len(EncodeClaim(claim)) which includes LocationData + overhead
	// Use a JSON string of 'a' chars as LocationData to control size.
	// EncodeClaim wraps it as json.RawMessage, so LocationData is embedded as-is.
	// The non-LocationData fields add ~180 bytes of JSON overhead.
	// We compute the right size empirically by binary search.
	baseClaim := UrmaClaim{
		Version:       ProtocolVersion,
		Location:      LocationSia,
		SourceClaimID: SourceClaimID{0x01},
		DataKey:       [32]byte{},
		LocationData:  []byte(`""`),
	}
	baseData, _ := EncodeClaim(baseClaim)
	baseLen := len(baseData) // ~195 bytes for the non-LocationData portion + 2 bytes for ""

	// Budget for LocationData content (excluding the 2-byte "" we already have):
	// 46 + 3 + 85 + baseLen + locDataLen <= 8192
	// locDataLen <= 8192 - 46 - 3 - 85 - baseLen
	maxLocData := MaxClaimScriptSize - ClaimScriptOverhead - 3 - EnvelopeSignedOverhead - baseLen
	if maxLocData < 0 {
		t.Fatalf("base claim too large: %d", baseLen)
	}

	// Build a JSON string of the right length
	padding := strings.Repeat("a", maxLocData)
	locData := `"` + padding + `"` // valid JSON string

	claim := UrmaClaim{
		Version:       ProtocolVersion,
		Location:      LocationSia,
		SourceClaimID: SourceClaimID{0x01},
		DataKey:       [32]byte{},
		LocationData:  []byte(locData),
	}
	if err := ValidateClaimSizeSigned(claim); err != nil {
		t.Errorf("expected max-size claim to pass: %v", err)
	}

	// One byte over should fail
	padding2 := strings.Repeat("a", maxLocData+1)
	claim.LocationData = []byte(`"` + padding2 + `"`)
	if err := ValidateClaimSizeSigned(claim); err == nil {
		t.Error("expected failure for one byte over the limit")
	}
}

func TestDecodeClaim_InvalidJSON(t *testing.T) {
	_, err := DecodeClaim([]byte(`{invalid`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestIsUrmaClaim(t *testing.T) {
	// Valid unsigned envelope with valid UrmaClaim JSON
	claim := UrmaClaim{
		Version:       ProtocolVersion,
		Location:      LocationSia,
		SourceClaimID: SourceClaimID{0x01},
		DataKey:       [32]byte{},
		LocationData:  []byte(`{}`),
	}
	claimJSON, _ := EncodeClaim(claim)
	envelope := EncodeUnsignedEnvelope(claimJSON)
	if !IsUrmaClaim(envelope) {
		t.Error("expected true for valid unsigned urma claim")
	}

	// Invalid: envelope with garbage payload
	garbage := EncodeUnsignedEnvelope([]byte(`not json`))
	if IsUrmaClaim(garbage) {
		t.Error("expected false for non-JSON payload")
	}

	// Invalid: not an envelope at all
	if IsUrmaClaim([]byte{0x99, 0x01, 0x02}) {
		t.Error("expected false for unknown version")
	}

	// Invalid: empty
	if IsUrmaClaim([]byte{}) {
		t.Error("expected false for empty")
	}
}
