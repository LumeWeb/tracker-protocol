// Package trackerprotocol defines the TrackerClaim spec — a self-contained,
// 8192-byte-maximum descriptor that tells a client how to locate, retrieve,
// and decrypt a file stored on a decentralized storage network.
//
// The design is storage-agnostic: a Location string identifies the backing
// network (sia, ipfs, future), and LocationData is a tagged union carrying
// the location-specific retrieval data as raw JSON. The claim always contains
// the file's encryption key so the consumer can decrypt once retrieved.
//
// The core package defines ONLY TrackerClaim, Location, and ManifestPage.
// Location-specific types (SlabSlice, SharedObject, etc.) are imported
// from their native packages (go.sia.tech/indexd/slabs, etc.) — no
// redefinition here.
package trackerprotocol

import (
	"encoding/json"
	"fmt"
)

// ProtocolVersion is the current version of the tracker protocol.
const ProtocolVersion uint8 = 1

// MaxClaimSize is the maximum serialized size of a TrackerClaim, per LBRY
// consensus rules. A claim that exceeds this size is invalid.
const MaxClaimSize = 8192

// Location identifies the storage backend for a claim.
type Location string

const (
	// LocationSia indicates the file is stored on the Sia decentralized
	// storage network via indexd SlabSlices.
	LocationSia Location = "sia"
	// Future: LocationIPFS, LocationArweave, etc.
)

func (l Location) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(l))
}

func (l *Location) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	switch Location(s) {
	case LocationSia:
		*l = LocationSia
	default:
		return fmt.Errorf("unknown location: %s", s)
	}
	return nil
}

// TrackerClaim is the top-level structure embedded in an 8192-byte claim.
// It contains everything needed to locate and begin retrieving a file.
//
// LocationData is raw JSON — its schema depends on Location. For "sia",
// it serializes to a slabs.SlabSlice pointing to the first ManifestPage.
// The consumer imports the location-specific package to decode it.
type TrackerClaim struct {
	// Version is the protocol version. Currently 1.
	Version uint8 `json:"version" jsonschema:"description=Protocol version,example=1"`

	// Location identifies the storage backend.
	Location Location `json:"location" jsonschema:"description=Storage backend identifier,enum=sia"`

	// DataKey is the encryption key for the file content (AES-256, 32 bytes).
	// This is always present — even when the manifest is external, the consumer
	// needs this key to decrypt the data. Encoded as base64 in JSON.
	DataKey [32]byte `json:"dataKey" jsonschema:"description=AES-256 encryption key for file content,format=byte"`

	// LocationData is the location-specific retrieval data. Its structure
	// depends on Location. The consumer decodes this using the appropriate
	// location package (e.g. slabs.SlabSlice for "sia").
	LocationData json.RawMessage `json:"locationData" jsonschema:"description=Location-specific retrieval data. Structure depends on the location field"`
}
