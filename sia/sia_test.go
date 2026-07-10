package sia

import (
	"encoding/json"
	"testing"

	"go.lumeweb.com/tracker-protocol"
	"go.sia.tech/indexd/slabs"
)

func makeSlabSlice(minShards, totalShards uint) slabs.SlabSlice {
	sectors := make([]slabs.PinnedSector, totalShards)
	for i := range sectors {
		root := make([]byte, 32)
		root[0] = byte(i)
		hostKey := make([]byte, 32)
		hostKey[0] = byte(i + 1)
		sectors[i] = slabs.PinnedSector{
			Root:    [32]byte(root),
			HostKey: [32]byte(hostKey),
		}
	}
	return slabs.SlabSlice{
		EncryptionKey: [32]byte{0xAA},
		MinShards:     minShards,
		Sectors:       sectors,
		Offset:        0,
		Length:        4194304,
	}
}

func TestEncodeDecodeClaim(t *testing.T) {
	dataKey := [32]byte{0x42}
	root := makeSlabSlice(10, 15)

	claim, err := EncodeClaim(dataKey, root)
	if err != nil {
		t.Fatalf("encode claim: %v", err)
	}

	if claim.Version != trackerprotocol.ProtocolVersion {
		t.Errorf("version: %d vs %d", claim.Version, trackerprotocol.ProtocolVersion)
	}
	if claim.Location != trackerprotocol.LocationSia {
		t.Errorf("location: %s", claim.Location)
	}
	if claim.DataKey != dataKey {
		t.Error("dataKey mismatch")
	}

	decoded, err := DecodeRootSlab(claim)
	if err != nil {
		t.Fatalf("decode root slab: %v", err)
	}
	if decoded.EncryptionKey != root.EncryptionKey {
		t.Error("encryption key mismatch")
	}
	if decoded.MinShards != root.MinShards {
		t.Error("minShards mismatch")
	}
	if len(decoded.Sectors) != len(root.Sectors) {
		t.Errorf("sectors: %d vs %d", len(decoded.Sectors), len(root.Sectors))
	}
}

func TestDecodeRootSlab_WrongLocation(t *testing.T) {
	claim := trackerprotocol.TrackerClaim{
		Version:      trackerprotocol.ProtocolVersion,
		Location:     trackerprotocol.Location("bogus"),
		DataKey:      [32]byte{},
		LocationData: []byte(`{}`),
	}
	_, err := DecodeRootSlab(claim)
	if err == nil {
		t.Error("expected error for wrong location")
	}
}

func TestManifestBlob_RoundTrip(t *testing.T) {
	ss := makeSlabSlice(10, 15)
	mb := FromSlabSlice(ss, "abc123", "iv456", 2097152, 0)

	if mb.BlobHash != "abc123" {
		t.Errorf("blobHash: %s", mb.BlobHash)
	}
	if mb.IV != "iv456" {
		t.Errorf("iv: %s", mb.IV)
	}
	if mb.BlobLength != 2097152 {
		t.Errorf("blobLength: %d", mb.BlobLength)
	}
	if mb.BlobNum != 0 {
		t.Errorf("blobNum: %d", mb.BlobNum)
	}

	recovered := mb.ToSlabSlice()
	if recovered.EncryptionKey != ss.EncryptionKey {
		t.Error("slab key mismatch")
	}
	if recovered.MinShards != ss.MinShards {
		t.Error("minShards mismatch")
	}
	if len(recovered.Sectors) != len(ss.Sectors) {
		t.Error("sectors mismatch")
	}
	if recovered.Offset != ss.Offset {
		t.Error("offset mismatch")
	}
	if recovered.Length != ss.Length {
		t.Error("length mismatch")
	}
}

func TestEncodeDecodeManifestPage(t *testing.T) {
	m := Manifest{
		StreamName:        "test.mp4",
		StreamType:        "lbryfile",
		SuggestedFileName: "test.mp4",
		Blobs: []ManifestBlob{
			FromSlabSlice(makeSlabSlice(10, 15), "hash1", "iv1", 1000, 0),
		},
	}

	next := makeSlabSlice(10, 15)
	page, err := EncodeManifestPage(m, &next)
	if err != nil {
		t.Fatalf("encode manifest page: %v", err)
	}

	if len(page.Data) == 0 {
		t.Error("data should not be empty")
	}
	if len(page.Next) == 0 {
		t.Error("next should not be empty")
	}

	decoded, err := DecodeManifestPage(page)
	if err != nil {
		t.Fatalf("decode manifest page: %v", err)
	}
	if decoded.StreamName != "test.mp4" {
		t.Errorf("streamName: %s", decoded.StreamName)
	}
	if len(decoded.Blobs) != 1 {
		t.Fatalf("blobs: %d", len(decoded.Blobs))
	}
	if decoded.Blobs[0].BlobHash != "hash1" {
		t.Errorf("blob hash: %s", decoded.Blobs[0].BlobHash)
	}

	nextSlab, err := DecodeNextSlab(page)
	if err != nil {
		t.Fatalf("decode next slab: %v", err)
	}
	if nextSlab == nil {
		t.Fatal("next slab should not be nil")
	}
	if nextSlab.EncryptionKey != next.EncryptionKey {
		t.Error("next slab key mismatch")
	}
}

func TestEncodeDecodeManifestBlobsPage(t *testing.T) {
	mb := ManifestBlobs{
		Blobs: []ManifestBlob{
			FromSlabSlice(makeSlabSlice(10, 15), "hash2", "iv2", 2000, 1),
			FromSlabSlice(makeSlabSlice(10, 15), "hash3", "iv3", 3000, 2),
		},
	}

	page, err := EncodeManifestBlobsPage(mb, nil)
	if err != nil {
		t.Fatalf("encode manifest blobs page: %v", err)
	}

	if len(page.Data) == 0 {
		t.Error("data should not be empty")
	}
	if page.Next != nil {
		t.Error("next should be nil for last page")
	}

	decoded, err := DecodeManifestBlobsPage(page)
	if err != nil {
		t.Fatalf("decode manifest blobs page: %v", err)
	}
	if len(decoded.Blobs) != 2 {
		t.Fatalf("blobs: %d", len(decoded.Blobs))
	}
	if decoded.Blobs[0].BlobHash != "hash2" {
		t.Errorf("blob 0 hash: %s", decoded.Blobs[0].BlobHash)
	}

	// Verify Next is omitted in JSON
	var raw map[string]json.RawMessage
	json.Unmarshal(page.Next, &raw) // should be nil/empty
	if len(page.Next) > 0 {
		t.Error("next should be omitted")
	}
}

func TestDecodeNextSlab_NoNext(t *testing.T) {
	page := trackerprotocol.ManifestPage{
		Data: json.RawMessage(`{}`),
	}

	next, err := DecodeNextSlab(page)
	if err != nil {
		t.Fatalf("decode next slab: %v", err)
	}
	if next != nil {
		t.Error("next should be nil")
	}
}
