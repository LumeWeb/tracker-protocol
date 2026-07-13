package sia

import (
	"encoding/json"
	"fmt"

	"go.lumeweb.com/urma"
	"go.sia.tech/indexd/slabs"
)

// PagePlan describes one page in the build plan: which blobs go on it and
// whether it has a next page.
type PagePlan struct {
	// Blobs is the list of ManifestBlob entries for this page.
	Blobs []ManifestBlob

	// PageIndex is the 0-based position in the page chain.
	PageIndex int

	// HasNext is true if there is a subsequent page.
	HasNext bool
}

// BuildChainResult is the output of BuildChain. It contains the fully
// constructed UrmaClaim, the root SlabSlice pointing to the first
// manifest page object, and the total number of pages.
type BuildChainResult struct {
	// Claim is the UrmaClaim ready to be published.
	Claim urma.UrmaClaim

	// RootSlab is the SlabSlice pointing to the first manifest page object
	// on Sia (also embedded in Claim.LocationData).
	RootSlab slabs.SlabSlice

	// PageCount is the total number of manifest pages in the chain.
	PageCount int
}

// UploadFunc is the callback the caller provides to BuildChain. It receives
// the JSON-serialized ManifestPage and must upload it to Sia, returning the
// resulting SlabSlice that points to the stored object.
//
// This callback is called once per page, in reverse order (last page first).
// The caller does NOT need to handle linking — BuildChain wires up the Next
// pointers internally.
type UploadFunc func(pageJSON []byte) (slabs.SlabSlice, error)

// PageBuilder accumulates blobs and builds a paginated manifest chain for
// Sia-backed urma claims.
//
// Usage:
//
//	pb := NewPageBuilder(streamName, streamType, suggestedFileName, dataKey)
//	pb.Add(blob1)
//	pb.Add(blob2)
//	// ...
//	result, err := pb.BuildChain(maxBlobsPerPage, uploadFunc)
type PageBuilder struct {
	streamName        string
	streamType        string
	suggestedFileName string
	sourceClaimID     urma.SourceClaimID
	dataKey           [32]byte
	blobs             []ManifestBlob
}

// NewPageBuilder creates a PageBuilder for a Sia-backed stream. The stream
// metadata fields populate the first page's Manifest.
func NewPageBuilder(streamName, streamType, suggestedFileName string, sourceClaimID urma.SourceClaimID, dataKey [32]byte) *PageBuilder {
	return &PageBuilder{
		streamName:        streamName,
		streamType:        streamType,
		suggestedFileName: suggestedFileName,
		sourceClaimID:     sourceClaimID,
		dataKey:           dataKey,
	}
}

// Add appends a ManifestBlob to the builder.
func (pb *PageBuilder) Add(blob ManifestBlob) {
	pb.blobs = append(pb.blobs, blob)
}

// BlobCount returns the number of blobs added so far.
func (pb *PageBuilder) BlobCount() int {
	return len(pb.blobs)
}

// Plan divides the accumulated blobs into pages of at most maxBlobsPerPage
// blobs each. The first page carries the stream metadata (Manifest), and
// continuation pages carry only blobs (ManifestBlobs).
//
// If there are zero blobs, Plan returns a single page with an empty Manifest.
func (pb *PageBuilder) Plan(maxBlobsPerPage int) []PagePlan {
	if maxBlobsPerPage < 1 {
		maxBlobsPerPage = 1
	}

	var plans []PagePlan
	totalBlobs := len(pb.blobs)
	pageCount := (totalBlobs + maxBlobsPerPage - 1) / maxBlobsPerPage
	if pageCount == 0 {
		pageCount = 1
	}

	for i := 0; i < pageCount; i++ {
		start := i * maxBlobsPerPage
		end := start + maxBlobsPerPage
		if end > totalBlobs {
			end = totalBlobs
		}
		plans = append(plans, PagePlan{
			Blobs:     pb.blobs[start:end],
			PageIndex: i,
			HasNext:   i < pageCount-1,
		})
	}
	return plans
}

// ManifestPage builds a urma.ManifestPage from a PagePlan. The
// first page (PageIndex 0) encodes a Manifest (stream metadata + blobs).
// Continuation pages encode a ManifestBlobs (blobs only). If the plan has a
// next page, the provided nextSlab is embedded as the Next pointer.
func (pb *PageBuilder) ManifestPage(plan PagePlan, nextSlab *slabs.SlabSlice) (urma.ManifestPage, error) {
	if plan.PageIndex == 0 {
		m := Manifest{
			StreamName:        pb.streamName,
			StreamType:        pb.streamType,
			SuggestedFileName: pb.suggestedFileName,
			Blobs:             plan.Blobs,
		}
		return EncodeManifestPage(m, nextSlab)
	}

	mb := ManifestBlobs{
		Blobs: plan.Blobs,
	}
	return EncodeManifestBlobsPage(mb, nextSlab)
}

// EstimateClaimSize estimates the total on-chain size of the UrmaClaim
// when encoded as an unsigned claim value envelope: script overhead + value
// push prefix + envelope overhead + JSON payload.
func EstimateClaimSize(sourceClaimID urma.SourceClaimID, dataKey [32]byte, root slabs.SlabSlice) (int, error) {
	ld, err := json.Marshal(root)
	if err != nil {
		return 0, fmt.Errorf("marshal root slab: %w", err)
	}
	claim := urma.UrmaClaim{
		Version:       urma.ProtocolVersion,
		Location:      urma.LocationSia,
		SourceClaimID: sourceClaimID,
		DataKey:       dataKey,
		LocationData:  ld,
	}
	data, err := urma.EncodeClaim(claim)
	if err != nil {
		return 0, err
	}
	envelopeSize := urma.EnvelopeUnsignedOverhead + len(data)
	return urma.ClaimScriptOverhead + urma.ValuePushSize(envelopeSize) + envelopeSize, nil
}

// FitsClaim checks whether a claim with the given dataKey and root SlabSlice
// would fit within MaxClaimScriptSize as an unsigned envelope.
func FitsClaim(sourceClaimID urma.SourceClaimID, dataKey [32]byte, root slabs.SlabSlice) bool {
	size, err := EstimateClaimSize(sourceClaimID, dataKey, root)
	if err != nil {
		return false
	}
	return size <= urma.MaxClaimScriptSize
}

// EstimateClaimSizeSigned estimates the total on-chain size of the UrmaClaim
// when encoded as a signed claim value envelope: script overhead + value
// push prefix + signed envelope overhead (85 bytes) + JSON payload.
func EstimateClaimSizeSigned(sourceClaimID urma.SourceClaimID, dataKey [32]byte, root slabs.SlabSlice) (int, error) {
	ld, err := json.Marshal(root)
	if err != nil {
		return 0, fmt.Errorf("marshal root slab: %w", err)
	}
	claim := urma.UrmaClaim{
		Version:       urma.ProtocolVersion,
		Location:      urma.LocationSia,
		SourceClaimID: sourceClaimID,
		DataKey:       dataKey,
		LocationData:  ld,
	}
	data, err := urma.EncodeClaim(claim)
	if err != nil {
		return 0, err
	}
	envelopeSize := urma.EnvelopeSignedOverhead + len(data)
	return urma.ClaimScriptOverhead + urma.ValuePushSize(envelopeSize) + envelopeSize, nil
}

// FitsClaimSigned checks whether a claim with the given dataKey and root
// SlabSlice would fit within MaxClaimScriptSize as a signed envelope.
func FitsClaimSigned(sourceClaimID urma.SourceClaimID, dataKey [32]byte, root slabs.SlabSlice) bool {
	size, err := EstimateClaimSizeSigned(sourceClaimID, dataKey, root)
	if err != nil {
		return false
	}
	return size <= urma.MaxClaimScriptSize
}

// BuildChain orchestrates the full reverse-build cycle:
//
//  1. Plan the pages from the accumulated blobs.
//  2. Start from the LAST page and work backwards.
//  3. For each page: build the ManifestPage JSON (with the Next pointer from
//     the previously uploaded page), upload it via the callback, and store
//     the resulting SlabSlice.
//  4. The first page's SlabSlice becomes the root in the UrmaClaim.
//
// The caller provides an UploadFunc that handles the actual Sia upload.
// BuildChain handles all linking — the caller never touches Next pointers.
func (pb *PageBuilder) BuildChain(maxBlobsPerPage int, upload UploadFunc) (*BuildChainResult, error) {
	if upload == nil {
		return nil, fmt.Errorf("upload function is required")
	}

	plans := pb.Plan(maxBlobsPerPage)
	pageCount := len(plans)

	// Build in reverse: last page → first page.
	var nextSlab *slabs.SlabSlice
	for i := pageCount - 1; i >= 0; i-- {
		page, err := pb.ManifestPage(plans[i], nextSlab)
		if err != nil {
			return nil, fmt.Errorf("build page %d: %w", i, err)
		}

		pageJSON, err := json.Marshal(page)
		if err != nil {
			return nil, fmt.Errorf("marshal page %d: %w", i, err)
		}

		slab, err := upload(pageJSON)
		if err != nil {
			return nil, fmt.Errorf("upload page %d: %w", i, err)
		}

		// This slab becomes the Next pointer for the previous page.
		slabCopy := slab
		nextSlab = &slabCopy
	}

	// nextSlab now points to the first page's slab (the last one uploaded).
	rootSlab := *nextSlab

	claim, err := EncodeClaim(pb.sourceClaimID, pb.dataKey, rootSlab)
	if err != nil {
		return nil, fmt.Errorf("encode claim: %w", err)
	}

	return &BuildChainResult{
		Claim:     claim,
		RootSlab:  rootSlab,
		PageCount: pageCount,
	}, nil
}
