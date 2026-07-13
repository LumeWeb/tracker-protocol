package sia

import (
	"encoding/json"
	"fmt"

	"github.com/invopop/jsonschema"
	"go.lumeweb.com/urma"
)

// GenerateSchema generates a combined JSON Schema document covering all
// Sia-specific types used in the urma protocol: Manifest, ManifestBlobs,
// and ManifestBlob. The core UrmaClaim schema is also included for
// completeness.
//
// The output is a self-contained JSON Schema (no $ref to external documents)
// suitable for publishing as a protocol specification.
func GenerateSchema() ([]byte, error) {
	reflector := jsonschema.Reflector{
		DoNotReference: true,
		ExpandedStruct: true,
	}

	schema := reflector.Reflect(&struct {
		Claim         urma.UrmaClaim `json:"claim"`
		Manifest      Manifest       `json:"manifest"`
		ManifestBlobs ManifestBlobs  `json:"manifestBlobs"`
		ManifestBlob  ManifestBlob   `json:"manifestBlob"`
	}{})

	return json.MarshalIndent(schema, "", "  ")
}

// GenerateManifestSchema generates a JSON Schema for the Sia Manifest type
// (page 0 payload).
func GenerateManifestSchema() ([]byte, error) {
	reflector := jsonschema.Reflector{
		DoNotReference: true,
		ExpandedStruct: true,
	}
	schema := reflector.Reflect(Manifest{})
	return json.MarshalIndent(schema, "", "  ")
}

// GenerateManifestBlobsSchema generates a JSON Schema for the ManifestBlobs
// type (continuation page payload).
func GenerateManifestBlobsSchema() ([]byte, error) {
	reflector := jsonschema.Reflector{
		DoNotReference: true,
		ExpandedStruct: true,
	}
	schema := reflector.Reflect(ManifestBlobs{})
	return json.MarshalIndent(schema, "", "  ")
}

// GenerateManifestBlobSchema generates a JSON Schema for the ManifestBlob
// type (single blob entry).
func GenerateManifestBlobSchema() ([]byte, error) {
	reflector := jsonschema.Reflector{
		DoNotReference: true,
		ExpandedStruct: true,
	}
	schema := reflector.Reflect(ManifestBlob{})
	return json.MarshalIndent(schema, "", "  ")
}

// GenerateClaimSchema generates a JSON Schema for the core UrmaClaim type.
func GenerateClaimSchema() ([]byte, error) {
	return urma.GenerateSchema()
}

// WriteSchema writes all generated schemas to a map keyed by type name.
// Useful for CLI tools that write each schema to a separate file.
func WriteSchemas() (map[string][]byte, error) {
	schemas := make(map[string][]byte)

	combined, err := GenerateSchema()
	if err != nil {
		return nil, fmt.Errorf("generate combined schema: %w", err)
	}
	schemas["sia"] = combined

	manifest, err := GenerateManifestSchema()
	if err != nil {
		return nil, fmt.Errorf("generate manifest schema: %w", err)
	}
	schemas["manifest"] = manifest

	blobs, err := GenerateManifestBlobsSchema()
	if err != nil {
		return nil, fmt.Errorf("generate manifest blobs schema: %w", err)
	}
	schemas["manifest-blobs"] = blobs

	blob, err := GenerateManifestBlobSchema()
	if err != nil {
		return nil, fmt.Errorf("generate manifest blob schema: %w", err)
	}
	schemas["manifest-blob"] = blob

	claim, err := GenerateClaimSchema()
	if err != nil {
		return nil, fmt.Errorf("generate claim schema: %w", err)
	}
	schemas["claim"] = claim

	return schemas, nil
}
