package trackerprotocol

import "encoding/json"

// ManifestPage is a page in the manifest chain. The TrackerClaim's
// LocationData is a location-specific pointer (e.g. a Sia SlabSlice) to the
// first ManifestPage object stored off-chain.
//
// Each page contains location-specific manifest data (blob entries, stream
// metadata, etc.) and an optional Next pointer to the subsequent page.
// A nil Next indicates the last page. Consumers follow the chain until Next
// is nil, accumulating manifest data across all pages.
//
// The Data field is decoded by the location-specific package. For Sia, the
// first page decodes to a Manifest (stream metadata + blobs) and continuation
// pages decode to ManifestBlobs (blobs only). Next is a raw location-specific
// pointer to the next page's storage object.
type ManifestPage struct {
	// Data is the location-specific manifest payload for this page.
	Data json.RawMessage `json:"data"`

	// Next is a location-specific pointer to the next ManifestPage object.
	// nil means this is the last page.
	Next json.RawMessage `json:"next,omitempty"`
}
