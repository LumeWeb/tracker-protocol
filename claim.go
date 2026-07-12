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
	"encoding/hex"
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

// MarshalJSON encodes SourceClaimID as a lowercase hex string.
func (s SourceClaimID) MarshalJSON() ([]byte, error) {
	return json.Marshal(hex.EncodeToString(s[:]))
}

// UnmarshalJSON decodes SourceClaimID from a hex string.
func (s *SourceClaimID) UnmarshalJSON(b []byte) error {
	var hexStr string
	if err := json.Unmarshal(b, &hexStr); err != nil {
		return err
	}
	decoded, err := hex.DecodeString(hexStr)
	if err != nil {
		return fmt.Errorf("invalid sourceClaimId: %w", err)
	}
	if len(decoded) != SourceClaimIDLen {
		return fmt.Errorf("sourceClaimId must be %d bytes, got %d", SourceClaimIDLen, len(decoded))
	}
	copy(s[:], decoded)
	return nil
}

// SourceClaimIDLen is the length of a LBRY ClaimID in bytes.
// A ClaimID is derived from the claim's outpoint: RIPEMD160(SHA256(tx:vout)).
const SourceClaimIDLen = 20

// SourceClaimID is the 20-byte ClaimID of the LBRY source claim that this
// tracker provides an alternative storage source for. It is encoded as a
// 40-character lowercase hex string in JSON.
//
// This field is a verification value, not a discovery value. The client
// already knows the source claim's ClaimID before querying for trackers
// (it found the content on LBRY, then computes the tracker name from the
// ClaimID). When decoding a tracker claim, the client verifies that this
// field matches the expected source ClaimID — rejecting mismatched or
// spoofed trackers.
type SourceClaimID [SourceClaimIDLen]byte

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

	// SourceClaimID is the ClaimID of the LBRY source claim this tracker
	// mirrors. Encoded as 40-char lowercase hex in JSON. Used by clients to
	// verify the tracker corresponds to the expected source content.
	SourceClaimID SourceClaimID `json:"sourceClaimId" jsonschema:"description=Hex-encoded 20-byte ClaimID of the LBRY source claim,format=hex,pattern=[0-9a-f]{40}"`

	// DataKey is the encryption key for the file content (AES-256, 32 bytes).
	// This is always present — even when the manifest is external, the consumer
	// needs this key to decrypt the data. Encoded as base64 in JSON.
	DataKey [32]byte `json:"dataKey" jsonschema:"description=AES-256 encryption key for file content,format=byte"`

	// LocationData is the location-specific retrieval data. Its structure
	// depends on Location. The consumer decodes this using the appropriate
	// location package (e.g. slabs.SlabSlice for "sia").
	LocationData json.RawMessage `json:"locationData" jsonschema:"description=Location-specific retrieval data. Structure depends on the location field"`
}
