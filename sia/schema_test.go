package sia

import (
	"encoding/json"
	"testing"
)

func TestGenerateSchema(t *testing.T) {
	schema, err := GenerateSchema()
	if err != nil {
		t.Fatalf("generate schema: %v", err)
	}
	if len(schema) == 0 {
		t.Fatal("schema should not be empty")
	}

	var raw map[string]any
	if err := json.Unmarshal(schema, &raw); err != nil {
		t.Fatalf("schema is invalid JSON: %v", err)
	}
	if raw["$schema"] == nil {
		t.Error("schema missing $schema field")
	}
}

func TestGenerateManifestSchema(t *testing.T) {
	schema, err := GenerateManifestSchema()
	if err != nil {
		t.Fatalf("generate manifest schema: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(schema, &raw); err != nil {
		t.Fatalf("schema is invalid JSON: %v", err)
	}
	props, ok := raw["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema missing properties")
	}
	if _, ok := props["streamName"]; !ok {
		t.Error("schema missing streamName property")
	}
	if _, ok := props["blobs"]; !ok {
		t.Error("schema missing blobs property")
	}
}

func TestGenerateManifestBlobSchema(t *testing.T) {
	schema, err := GenerateManifestBlobSchema()
	if err != nil {
		t.Fatalf("generate manifest blob schema: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(schema, &raw); err != nil {
		t.Fatalf("schema is invalid JSON: %v", err)
	}
	props, ok := raw["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema missing properties")
	}
	required := []string{"blobHash", "iv", "blobLength", "blobNum", "slabKey", "minShards", "sectors", "slabOffset", "slabLength"}
	for _, field := range required {
		if _, ok := props[field]; !ok {
			t.Errorf("schema missing property: %s", field)
		}
	}
}

func TestWriteSchemas(t *testing.T) {
	schemas, err := WriteSchemas()
	if err != nil {
		t.Fatalf("write schemas: %v", err)
	}

	expected := []string{"sia", "manifest", "manifest-blobs", "manifest-blob", "claim"}
	for _, name := range expected {
		if _, ok := schemas[name]; !ok {
			t.Errorf("missing schema: %s", name)
		}
	}
}
