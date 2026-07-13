package sia

import (
	"encoding/json"
	"testing"

	"go.lumeweb.com/tracker-protocol"
	"go.sia.tech/indexd/slabs"
)

func TestPageBuilder_Plan(t *testing.T) {
	pb := NewPageBuilder("test.mp4", "lbryfile", "test.mp4", trackerprotocol.SourceClaimID{0xa1}, [32]byte{0x01})

	for i := 0; i < 9; i++ {
		pb.Add(FromSlabSlice(makeSlabSlice(10, 15), "hash", "iv", 1000, i))
	}

	plans := pb.Plan(4)
	if len(plans) != 3 {
		t.Fatalf("expected 3 pages, got %d", len(plans))
	}
	if len(plans[0].Blobs) != 4 {
		t.Errorf("page 0: %d blobs", len(plans[0].Blobs))
	}
	if len(plans[1].Blobs) != 4 {
		t.Errorf("page 1: %d blobs", len(plans[1].Blobs))
	}
	if len(plans[2].Blobs) != 1 {
		t.Errorf("page 2: %d blobs", len(plans[2].Blobs))
	}
	if plans[0].PageIndex != 0 || plans[2].PageIndex != 2 {
		t.Error("page index mismatch")
	}
	if !plans[0].HasNext || plans[2].HasNext {
		t.Error("hasNext mismatch")
	}
}

func TestPageBuilder_Plan_ZeroBlobs(t *testing.T) {
	pb := NewPageBuilder("test", "lbryfile", "test", trackerprotocol.SourceClaimID{0xa1}, [32]byte{0x01})
	plans := pb.Plan(4)
	if len(plans) != 1 {
		t.Fatalf("expected 1 page for zero blobs, got %d", len(plans))
	}
	if len(plans[0].Blobs) != 0 {
		t.Errorf("expected 0 blobs, got %d", len(plans[0].Blobs))
	}
	if plans[0].HasNext {
		t.Error("single page should not have next")
	}
}

func TestPageBuilder_ManifestPage_FirstPage(t *testing.T) {
	pb := NewPageBuilder("stream.mp4", "lbryfile", "stream.mp4", trackerprotocol.SourceClaimID{0xa1}, [32]byte{0x01})
	plan := PagePlan{
		Blobs:     []ManifestBlob{FromSlabSlice(makeSlabSlice(10, 15), "h1", "i1", 1000, 0)},
		PageIndex: 0,
		HasNext:   true,
	}
	next := makeSlabSlice(10, 15)

	page, err := pb.ManifestPage(plan, &next)
	if err != nil {
		t.Fatalf("manifest page: %v", err)
	}

	m, err := DecodeManifestPage(page)
	if err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if m.StreamName != "stream.mp4" {
		t.Errorf("streamName: %s", m.StreamName)
	}
	if len(m.Blobs) != 1 {
		t.Errorf("blobs: %d", len(m.Blobs))
	}
	if len(page.Next) == 0 {
		t.Error("next should not be empty")
	}
}

func TestPageBuilder_ManifestPage_ContinuationPage(t *testing.T) {
	pb := NewPageBuilder("stream.mp4", "lbryfile", "stream.mp4", trackerprotocol.SourceClaimID{0xa1}, [32]byte{0x01})
	plan := PagePlan{
		Blobs: []ManifestBlob{
			FromSlabSlice(makeSlabSlice(10, 15), "h2", "i2", 2000, 1),
			FromSlabSlice(makeSlabSlice(10, 15), "h3", "i3", 3000, 2),
		},
		PageIndex: 1,
		HasNext:   false,
	}

	page, err := pb.ManifestPage(plan, nil)
	if err != nil {
		t.Fatalf("manifest page: %v", err)
	}

	mb, err := DecodeManifestBlobsPage(page)
	if err != nil {
		t.Fatalf("decode manifest blobs: %v", err)
	}
	if len(mb.Blobs) != 2 {
		t.Errorf("blobs: %d", len(mb.Blobs))
	}
	if mb.Blobs[0].BlobHash != "h2" {
		t.Errorf("blob 0: %s", mb.Blobs[0].BlobHash)
	}
	if len(page.Next) != 0 {
		t.Error("next should be empty")
	}
}

func TestEstimateClaimSize(t *testing.T) {
	dataKey := [32]byte{0x42}
	root := makeSlabSlice(10, 15)

	size, err := EstimateClaimSize(trackerprotocol.SourceClaimID{0xa1}, dataKey, root)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	if size <= 0 {
		t.Errorf("size should be positive, got %d", size)
	}
	if size > trackerprotocol.MaxClaimScriptSize {
		t.Errorf("size %d exceeds max %d", size, trackerprotocol.MaxClaimScriptSize)
	}
}

func TestFitsClaim(t *testing.T) {
	dataKey := [32]byte{0x42}
	root := makeSlabSlice(10, 15)
	if !FitsClaim(trackerprotocol.SourceClaimID{0xa1}, dataKey, root) {
		t.Error("expected claim to fit")
	}
}

func TestEstimateClaimSize_IncludesOverhead(t *testing.T) {
	dataKey := [32]byte{0x42}
	root := makeSlabSlice(10, 15)

	size, err := EstimateClaimSize(trackerprotocol.SourceClaimID{0xa1}, dataKey, root)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}

	// Size must account for script overhead (46) + push prefix + envelope overhead (1) + JSON
	// A size that only included JSON would be ~= 300 bytes; with overhead it should be ~350+
	if size < trackerprotocol.ClaimScriptOverhead {
		t.Errorf("size %d does not include script overhead %d", size, trackerprotocol.ClaimScriptOverhead)
	}
}

func TestEstimateClaimSizeSigned(t *testing.T) {
	dataKey := [32]byte{0x42}
	root := makeSlabSlice(10, 15)

	unsigned, err := EstimateClaimSize(trackerprotocol.SourceClaimID{0xa1}, dataKey, root)
	if err != nil {
		t.Fatalf("unsigned estimate: %v", err)
	}

	signed, err := EstimateClaimSizeSigned(trackerprotocol.SourceClaimID{0xa1}, dataKey, root)
	if err != nil {
		t.Fatalf("signed estimate: %v", err)
	}

	// Signed should be larger by EnvelopeSignedOverhead - EnvelopeUnsignedOverhead = 84 bytes
	// (push prefix may also change, so allow some variance)
	diff := signed - unsigned
	if diff < 84 {
		t.Errorf("signed-unsigned diff %d, expected at least 84", diff)
	}
	if diff > 86 {
		t.Errorf("signed-unsigned diff %d, expected at most 86", diff)
	}
}

func TestFitsClaimSigned(t *testing.T) {
	dataKey := [32]byte{0x42}
	root := makeSlabSlice(10, 15)
	if !FitsClaimSigned(trackerprotocol.SourceClaimID{0xa1}, dataKey, root) {
		t.Error("expected signed claim to fit")
	}
}

func TestPageBuilder_BuildChain_SinglePage(t *testing.T) {
	pb := NewPageBuilder("single.mp4", "lbryfile", "single.mp4", trackerprotocol.SourceClaimID{0xa1}, [32]byte{0x42})
	pb.Add(FromSlabSlice(makeSlabSlice(10, 15), "h1", "i1", 1000, 0))
	pb.Add(FromSlabSlice(makeSlabSlice(10, 15), "h2", "i2", 2000, 1))

	uploadCalls := 0
	result, err := pb.BuildChain(10, func(pageJSON []byte) (slabs.SlabSlice, error) {
		uploadCalls++
		// Verify the page has no Next (single page)
		var page trackerprotocol.ManifestPage
		if err := json.Unmarshal(pageJSON, &page); err != nil {
			t.Fatalf("unmarshal page: %v", err)
		}
		if len(page.Next) != 0 {
			t.Error("single page should have no next")
		}
		return makeSlabSlice(10, 15), nil
	})
	if err != nil {
		t.Fatalf("build chain: %v", err)
	}
	if uploadCalls != 1 {
		t.Errorf("expected 1 upload call, got %d", uploadCalls)
	}
	if result.PageCount != 1 {
		t.Errorf("page count: %d", result.PageCount)
	}
	if result.Claim.Location != trackerprotocol.LocationSia {
		t.Errorf("location: %s", result.Claim.Location)
	}
	if result.Claim.DataKey != [32]byte{0x42} {
		t.Error("dataKey mismatch")
	}
}

func TestPageBuilder_BuildChain_MultiPage(t *testing.T) {
	pb := NewPageBuilder("multi.mp4", "lbryfile", "multi.mp4", trackerprotocol.SourceClaimID{0xa1}, [32]byte{0x42})

	for i := 0; i < 9; i++ {
		pb.Add(FromSlabSlice(makeSlabSlice(10, 15), "h", "i", 1000, i))
	}

	uploadCalls := 0
	var pageNextFlags []bool
	result, err := pb.BuildChain(4, func(pageJSON []byte) (slabs.SlabSlice, error) {
		uploadCalls++
		var page trackerprotocol.ManifestPage
		if err := json.Unmarshal(pageJSON, &page); err != nil {
			t.Fatalf("unmarshal page: %v", err)
		}
		pageNextFlags = append(pageNextFlags, len(page.Next) != 0)
		return makeSlabSlice(10, 15), nil
	})
	if err != nil {
		t.Fatalf("build chain: %v", err)
	}
	if uploadCalls != 3 {
		t.Errorf("expected 3 upload calls, got %d", uploadCalls)
	}
	if result.PageCount != 3 {
		t.Errorf("page count: %d", result.PageCount)
	}
	// Pages are uploaded in reverse, so:
	// upload 0 = page 2 (last, no next)
	// upload 1 = page 1 (has next)
	// upload 2 = page 0 (has next)
	if pageNextFlags[0] {
		t.Error("last page should have no next")
	}
	if !pageNextFlags[1] || !pageNextFlags[2] {
		t.Error("pages 0 and 1 should have next")
	}
}

func TestPageBuilder_BuildChain_NilUpload(t *testing.T) {
	pb := NewPageBuilder("test", "lbryfile", "test", trackerprotocol.SourceClaimID{0xa1}, [32]byte{0x01})
	pb.Add(FromSlabSlice(makeSlabSlice(10, 15), "h", "i", 1000, 0))
	_, err := pb.BuildChain(10, nil)
	if err == nil {
		t.Error("expected error for nil upload func")
	}
}

func TestPageBuilder_BuildChain_ZeroBlobs(t *testing.T) {
	pb := NewPageBuilder("empty", "lbryfile", "empty", trackerprotocol.SourceClaimID{0xa1}, [32]byte{0x01})
	result, err := pb.BuildChain(10, func(pageJSON []byte) (slabs.SlabSlice, error) {
		return makeSlabSlice(10, 15), nil
	})
	if err != nil {
		t.Fatalf("build chain: %v", err)
	}
	if result.PageCount != 1 {
		t.Errorf("page count: %d", result.PageCount)
	}
}

func TestPageBuilder_BlobCount(t *testing.T) {
	pb := NewPageBuilder("test", "lbryfile", "test", trackerprotocol.SourceClaimID{0xa1}, [32]byte{0x01})
	if pb.BlobCount() != 0 {
		t.Errorf("expected 0 blobs, got %d", pb.BlobCount())
	}
	pb.Add(FromSlabSlice(makeSlabSlice(10, 15), "h", "i", 1000, 0))
	pb.Add(FromSlabSlice(makeSlabSlice(10, 15), "h", "i", 1000, 1))
	if pb.BlobCount() != 2 {
		t.Errorf("expected 2 blobs, got %d", pb.BlobCount())
	}
}
