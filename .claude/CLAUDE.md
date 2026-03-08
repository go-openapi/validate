# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Go implementation of [JSON Pointer (RFC 6901)](https://datatracker.ietf.org/doc/html/rfc6901) for navigating
and mutating JSON documents represented as Go values. Unlike most implementations, it works not only with
`map[string]any` and slices, but also with Go structs (resolved via `json` struct tags and reflection).

See [docs/MAINTAINERS.md](../docs/MAINTAINERS.md) for CI/CD, release process, and repo structure details.

### Package layout (single package)

| File | Contents |
|------|----------|
| `pointer.go` | Core types (`Pointer`, `JSONPointable`, `JSONSetable`), `New`, `Get`, `Set`, `Offset`, `Escape`/`Unescape` |
| `errors.go` | Sentinel errors: `ErrPointer`, `ErrInvalidStart`, `ErrUnsupportedValueType` |

### Key API

- `New(string) (Pointer, error)` — parse a JSON pointer string (e.g. `"/foo/0/bar"`)
- `Pointer.Get(document any) (any, reflect.Kind, error)` — retrieve a value
- `Pointer.Set(document, value any) (any, error)` — set a value (document must be pointer/map/slice)
- `Pointer.Offset(jsonString string) (int64, error)` — byte offset of token in raw JSON
- `GetForToken` / `SetForToken` — single-level convenience helpers
- `Escape` / `Unescape` — RFC 6901 token escaping (`~0` ↔ `~`, `~1` ↔ `/`)

Custom types can implement `JSONPointable` (for Get) or `JSONSetable` (for Set) to bypass reflection.

### Dependencies

- `github.com/go-openapi/swag/jsonname` — struct tag → JSON field name resolution
- `github.com/go-openapi/testify/v2` — test-only assertions

### Notable historical design decisions

See also .claude/plans/ROADMAP.md.

- Struct fields **must** have a `json` tag to be reachable; untagged fields are ignored
  (differs from `encoding/json` which defaults to the Go field name).
- Anonymous embedded struct fields are traversed only if tagged.
- The RFC 6901 `"-"` array suffix (append) is **not** implemented.

