// Package sia provides Sia-specific helpers for constructing and decoding
// tracker protocol claims and manifests.
//
// A Sia-backed tracker claim's LocationData is a slabs.SlabSlice pointing to
// the first ManifestPage object stored on Sia. The ManifestPage contains a
// Manifest payload (with blob entries mapping LBRY blob hashes to Sia
// SlabSlices) and an optional Next pointer (another SlabSlice) to the next
// page.
package sia

import (
	"encoding/json"
	"fmt"

	"go.lumeweb.com/LumeWeb/tracker-protocol"
	"go.sia.tech/indexd/slabs"
)

// ManifestBlobs is the data structure for continuation pages (page 1+,
// 0-indexed) in the manifest chain. It carries only the blob entries —
// stream metadata lives exclusively on page 0 (Manifest). The consumer
// knows their position in the chain and decodes accordingly.
type ManifestBlobs struct {
	Blobs []ManifestBlob `json:"blobs" jsonschema:"description=Blob entries for this continuation page"`
}

// Manifest is the Sia-specific payload stored in the first ManifestPage's Data
// field. It contains the LBRY stream metadata and the first page of blob
// entries. If not all blobs fit, ManifestPage.Next points to additional pages
// whose Data is a ManifestBlobs (blobs only, no stream metadata).
//
// A Manifest can be reconstructed into a LBRY sdhash blob on demand — it
// carries all the necessary data (stream key, per-blob IVs, blob hashes,
// lengths) to produce a byte-identical SDBlob.
type Manifest struct {
	// StreamName is the original stream name (hex-encoded in LBRY sdhash).
	StreamName string `json:"streamName" jsonschema:"description=Original stream name"`

	// StreamType is the LBRY stream type (typically "lbryfile").
	StreamType string `json:"streamType" jsonschema:"description=LBRY stream type"`

	// SuggestedFileName is the suggested file name for the stream.
	SuggestedFileName string `json:"suggestedFileName" jsonschema:"description=Suggested file name"`

	// Blobs is the list of blob entries for this first page. Each maps a
	// LBRY blob hash to its Sia location.
	Blobs []ManifestBlob `json:"blobs" jsonschema:"description=Blob entries mapping LBRY blob hashes to Sia locations"`
}

// ManifestBlob is the unified per-blob entry that carries both the Sia
// retrieval data (sectors, slab key) and the LBRY compatibility data
// (blob hash, per-blob IV). It does NOT embed slabs.SlabSlice directly —
// instead it flattens the Sia fields so there's no confusion between
// Sia's Offset/Length (slice within slab) and the LBRY blob Length.
type ManifestBlob struct {
	// BlobHash is the SHA-384 hash of the encrypted LBRY blob (hex-encoded).
	// This is the LBRY blob hash used to identify and retrieve the blob.
	BlobHash string `json:"blobHash" jsonschema:"description=SHA-384 hash of encrypted blob (hex)"`

	// IV is the AES initialization vector for this LBRY blob (hex-encoded).
	// This is the LBRY per-blob IV, NOT the Sia slab encryption key.
	IV string `json:"iv" jsonschema:"description=AES IV for blob content (hex)"`

	// BlobLength is the size of the LBRY blob in bytes (max 2 MiB).
	BlobLength int `json:"blobLength" jsonschema:"description=LBRY blob size in bytes"`

	// BlobNum is the position of this blob in the stream (0-indexed).
	BlobNum int `json:"blobNum" jsonschema:"description=Position in stream (0-indexed)"`

	// SlabKey is the Sia slab encryption key for this blob's object.
	SlabKey slabs.EncryptionKey `json:"slabKey" jsonschema:"description=Sia slab encryption key"`

	// MinShards is the minimum number of sectors needed to reconstruct
	// the slab data.
	MinShards uint `json:"minShards" jsonschema:"description=Minimum sectors for recovery"`

	// Sectors is the list of pinned sectors for this blob's slab.
	Sectors []slabs.PinnedSector `json:"sectors" jsonschema:"description=Pinned sectors for this blob"`

	// SlabOffset is the byte offset into the slab where this blob's data starts.
	SlabOffset uint32 `json:"slabOffset" jsonschema:"description=Byte offset into slab data"`

	// SlabLength is the number of data bytes in the slab slice (may differ
	// from BlobLength if the blob is stored with padding or multiple blobs
	// share a slab).
	SlabLength uint32 `json:"slabLength" jsonschema:"description=Number of data bytes in slab slice"`
}

// ToSlabSlice converts a ManifestBlob back into a slabs.SlabSlice for
// Sia retrieval.
func (b ManifestBlob) ToSlabSlice() slabs.SlabSlice {
	return slabs.SlabSlice{
		EncryptionKey: b.SlabKey,
		MinShards:     b.MinShards,
		Sectors:       b.Sectors,
		Offset:        b.SlabOffset,
		Length:        b.SlabLength,
	}
}

// FromSlabSlice creates a ManifestBlob from a slabs.SlabSlice and the
// LBRY-specific fields (blob hash, IV, blob length, blob number).
func FromSlabSlice(ss slabs.SlabSlice, blobHash, iv string, blobLength, blobNum int) ManifestBlob {
	return ManifestBlob{
		BlobHash:   blobHash,
		IV:         iv,
		BlobLength: blobLength,
		BlobNum:    blobNum,
		SlabKey:    ss.EncryptionKey,
		MinShards:  ss.MinShards,
		Sectors:    ss.Sectors,
		SlabOffset: ss.Offset,
		SlabLength: ss.Length,
	}
}

// EncodeClaim creates a TrackerClaim whose LocationData is a SlabSlice
// pointing to the first ManifestPage object on Sia. The caller provides the
// root SlabSlice (obtained after uploading the first manifest page to Sia).
//
// The dataKey is the LBRY stream encryption key (stored in the claim for
// the consumer to decrypt blob content after retrieval).
func EncodeClaim(dataKey [32]byte, root slabs.SlabSlice) (trackerprotocol.TrackerClaim, error) {
	ld, err := json.Marshal(root)
	if err != nil {
		return trackerprotocol.TrackerClaim{}, fmt.Errorf("marshal root slab: %w", err)
	}
	claim := trackerprotocol.TrackerClaim{
		Version:      trackerprotocol.ProtocolVersion,
		Location:     trackerprotocol.LocationSia,
		DataKey:      dataKey,
		LocationData: ld,
	}
	if err := trackerprotocol.ValidateClaimSize(claim); err != nil {
		return trackerprotocol.TrackerClaim{}, err
	}
	return claim, nil
}

// DecodeRootSlab decodes the LocationData of a Sia claim into the root
// SlabSlice pointing to the first ManifestPage object on Sia.
func DecodeRootSlab(claim trackerprotocol.TrackerClaim) (slabs.SlabSlice, error) {
	if claim.Location != trackerprotocol.LocationSia {
		return slabs.SlabSlice{}, fmt.Errorf("expected location '%s', got '%s'", trackerprotocol.LocationSia, claim.Location)
	}
	var ss slabs.SlabSlice
	if err := json.Unmarshal(claim.LocationData, &ss); err != nil {
		return slabs.SlabSlice{}, fmt.Errorf("unmarshal root slab: %w", err)
	}
	return ss, nil
}

// EncodeManifestPage creates a ManifestPage from a Manifest and an optional
// next-page SlabSlice. The resulting page is JSON-serialized and stored as a
// Sia object.
func EncodeManifestPage(m Manifest, next *slabs.SlabSlice) (trackerprotocol.ManifestPage, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return trackerprotocol.ManifestPage{}, fmt.Errorf("marshal manifest: %w", err)
	}
	return encodePage(data, next)
}

// EncodeManifestBlobsPage creates a ManifestPage for a continuation page
// (page 1+, 0-indexed) from a ManifestBlobs and an optional next-page SlabSlice.
func EncodeManifestBlobsPage(mb ManifestBlobs, next *slabs.SlabSlice) (trackerprotocol.ManifestPage, error) {
	data, err := json.Marshal(mb)
	if err != nil {
		return trackerprotocol.ManifestPage{}, fmt.Errorf("marshal manifest blobs: %w", err)
	}
	return encodePage(data, next)
}

func encodePage(data []byte, next *slabs.SlabSlice) (trackerprotocol.ManifestPage, error) {
	page := trackerprotocol.ManifestPage{
		Data: data,
	}
	if next != nil {
		nextJSON, err := json.Marshal(next)
		if err != nil {
			return trackerprotocol.ManifestPage{}, fmt.Errorf("marshal next slab: %w", err)
		}
		page.Next = nextJSON
	}
	return page, nil
}

// DecodeManifestPage decodes the first ManifestPage's Data into a Sia Manifest.
func DecodeManifestPage(page trackerprotocol.ManifestPage) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(page.Data, &m); err != nil {
		return Manifest{}, fmt.Errorf("unmarshal manifest: %w", err)
	}
	return m, nil
}

// DecodeManifestBlobsPage decodes a continuation page's Data into ManifestBlobs.
func DecodeManifestBlobsPage(page trackerprotocol.ManifestPage) (ManifestBlobs, error) {
	var mb ManifestBlobs
	if err := json.Unmarshal(page.Data, &mb); err != nil {
		return ManifestBlobs{}, fmt.Errorf("unmarshal manifest blobs: %w", err)
	}
	return mb, nil
}

// DecodeNextSlab decodes the Next pointer of a ManifestPage into a SlabSlice.
// Returns nil, nil if there is no next page.
func DecodeNextSlab(page trackerprotocol.ManifestPage) (*slabs.SlabSlice, error) {
	if len(page.Next) == 0 {
		return nil, nil
	}
	var ss slabs.SlabSlice
	if err := json.Unmarshal(page.Next, &ss); err != nil {
		return nil, fmt.Errorf("unmarshal next slab: %w", err)
	}
	return &ss, nil
}
