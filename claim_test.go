package trackerprotocol

import (
	"encoding/json"
	"testing"
)

func TestRoundTrip_Claim(t *testing.T) {
	rawData := json.RawMessage(`{"slabs":[{"encryptionKey":"AAAA","minShards":10,"sectors":[{"root":"0000000000000000000000000000000000000000000000000000000000000001","hostKey":"0000000000000000000000000000000000000000000000000000000000000002"}],"offset":0,"length":4194304}]}`)
	claim := TrackerClaim{
		Version:      ProtocolVersion,
		Location:     LocationSia,
		DataKey:      [32]byte{0x42},
		LocationData: rawData,
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
	huge := make([]byte, MaxClaimSize+100)
	claim := TrackerClaim{
		Version:      ProtocolVersion,
		Location:     LocationSia,
		DataKey:      [32]byte{},
		LocationData: huge,
	}
	err := ValidateClaimSize(claim)
	if err == nil {
		t.Error("expected error for oversized claim")
	}
}

func TestDecodeClaim_InvalidJSON(t *testing.T) {
	_, err := DecodeClaim([]byte(`{invalid`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
