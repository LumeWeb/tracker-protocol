package urma

import (
	"encoding/json"
	"testing"
)

func TestManifestPage_SinglePage(t *testing.T) {
	page := ManifestPage{
		Data: json.RawMessage(`{"blobs":[]}`),
	}

	data, err := json.Marshal(page)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded ManifestPage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if string(decoded.Data) != `{"blobs":[]}` {
		t.Errorf("data mismatch: %s", decoded.Data)
	}
	if decoded.Next != nil {
		t.Error("next should be nil for single page")
	}
}

func TestManifestPage_WithNext(t *testing.T) {
	page := ManifestPage{
		Data: json.RawMessage(`{"blobs":[]}`),
		Next: json.RawMessage(`{"slab":"abc"}`),
	}

	data, err := json.Marshal(page)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded ManifestPage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if string(decoded.Data) != `{"blobs":[]}` {
		t.Errorf("data mismatch: %s", decoded.Data)
	}
	if string(decoded.Next) != `{"slab":"abc"}` {
		t.Errorf("next mismatch: %s", decoded.Next)
	}
}

func TestManifestPage_OmitEmptyNext(t *testing.T) {
	page := ManifestPage{
		Data: json.RawMessage(`{}`),
	}

	data, err := json.Marshal(page)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Verify "next" field is omitted when nil
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to map failed: %v", err)
	}
	if _, ok := raw["next"]; ok {
		t.Error("next field should be omitted when nil")
	}
}
