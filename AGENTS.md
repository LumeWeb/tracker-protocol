# AGENTS.md

## Module

`go.lumeweb.com/LumeWeb/tracker-protocol`

Go 1.26.

## Build & Test

```bash
go build ./...
go test -count=1 ./...
```

## Conventions

- **Core is agnostic.** Core types use `json.RawMessage` for location-specific data. No backend-specific types in core.
- **No redefined types.** Backend types come from their native packages directly.
- **Child packages import parent, never reverse.**
- **No upload client.** Protocol defines data structures and helpers only.
- **No duplicated data.** Stream metadata lives only on page 0.
- **Location is a string type.** Not a numeric enum.
- Pre-commit: `gofmt`, `go vet`, `go build ./...`, `go test -count=1 ./...`.
