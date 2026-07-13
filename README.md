# Urma

Protocol for decentralized storage claims on the LBRY blockchain. Defines the data structures that allow a client to locate, retrieve, and decrypt a file stored on a decentralized storage network (Sia, and future backends).

## How It Works

An **UrmaClaim** is a compact JSON descriptor embedded in a blockchain claim. It contains:

- **Version** — protocol version (currently 0)
- **Location** — storage backend identifier (`"sia"`)
- **DataKey** — AES-256 encryption key for file content
- **LocationData** — location-specific retrieval data (e.g. a Sia `SlabSlice` pointing to the first manifest page)
- **SourceClaimID** — the LBRY source claim ID this Urma claim provides an alternative storage source for

The claim points to a **manifest chain** — a DAG of `ManifestPage` objects stored off-chain. Each page contains blob entries mapping LBRY blob hashes to Sia slab locations, and an optional `Next` pointer to the next page. The consumer follows the chain until `Next` is nil.

### Claim Naming

Urma claims use the name `u-<source_claim_id_hex>`, derived from the LBRY source content claim. A client that finds content on LBRY computes the Urma name from the source claim ID and queries the claimtrie for matching Urma claims.

### Envelope and Signing

Claim values are wrapped in an envelope with a version byte. Unsigned envelopes (`0x00`) contain just the JSON payload. Signed envelopes (`0x01`) include a channel ClaimID and ECDSA signature, allowing creator-owned Urma claims that are cryptographically bound to a channel.

### Size Validation

The full on-chain claim script must fit within the 8192-byte `MaxClaimScriptSize` limit. This includes script overhead, envelope overhead, value push prefix, and the JSON payload. The protocol provides `ValidateClaimSize` and `ValidateClaimSizeSigned` for this accounting.

## Architecture

```
urma/
├── claim.go          # UrmaClaim, SourceClaimID, MaxClaimScriptSize
├── codec.go          # EncodeClaim, DecodeClaim, ValidateClaimSize, GenerateSchema
├── envelope.go       # Envelope encoding/decoding, signing, verification
├── manifest.go       # ManifestPage (core paging type)
├── sia/
│   ├── sia.go        # Manifest, ManifestBlobs, ManifestBlob, encode/decode helpers
│   ├── pagebuilder.go # PageBuilder, Plan, BuildChain, FitsClaim
│   └── schema.go     # JSON Schema generation for Sia types
├── cmd/
│   └── schemas/      # CLI tool to generate JSON Schema files
└── schemas/          # Generated schemas (gitignored)
```

### Key Design Principles

- **Core is agnostic.** Core types use `json.RawMessage` for location-specific data. No backend-specific types in core.
- **No redefined types.** Backend types come from their native packages directly (e.g. `go.sia.tech/indexd/slabs`).
- **Child packages import parent, never reverse.**
- **No upload client.** Protocol defines data structures and helpers only.
- **Location is a string type.** Not a numeric enum.

## Getting Started

### Prerequisites

- Go 1.26+

### Build

```bash
go build ./...
```

### Test

```bash
go test -count=1 ./...
```

### Generate Schemas

```bash
go run ./cmd/schemas
# Writes to schemas/*.json
```

## Usage

### Building a Sia-backed claim

```go
import (
    "go.lumeweb.com/urma/sia"
)

pb := sia.NewPageBuilder("movie.mp4", "lbryfile", "movie.mp4", dataKey)
pb.Add(sia.FromSlabSlice(slab1, blobHash1, iv1, blobLen1, 0))
pb.Add(sia.FromSlabSlice(slab2, blobHash2, iv2, blobLen2, 1))

result, err := pb.BuildChain(maxBlobsPerPage, uploadFunc)
// result.Claim is ready to publish
// result.RootSlab points to the first manifest page
// result.PageCount is the total number of pages
```

### Decoding a claim

```go
claim, err := urma.DecodeClaim(rawJSON)
rootSlab, err := sia.DecodeRootSlab(claim)
// Fetch and decode manifest pages following the Next chain
```

## Dependencies

- `go.sia.tech/core` — Sia core types
- `go.sia.tech/indexd` — Slab/Sector types for Sia storage
- `github.com/invopop/jsonschema` — JSON Schema generation
- `github.com/decred/dcrd/dcrec/secp256k1/v4` — ECDSA signing and verification

## License

MIT
