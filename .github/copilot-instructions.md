# Copilot Instructions

## Project Overview

Go library for validating OpenAPI v2 (Swagger) specifications and JSON Schema draft 4 data.
Part of the [go-openapi](https://github.com/go-openapi) ecosystem. API is stable; maintenance-only.

## Package Layout

| Package | Contents |
|---------|----------|
| `validate` (root) | Spec validator, schema validator, type/format/object/slice/string/number validators, result handling, pools |
| `post` | Post-validation transforms: `ApplyDefaults` and `Prune` |

## Key API

- `Spec(doc, formats) error` — high-level spec validation
- `AgainstSchema(schema, data, formats, ...Option) error` — validate data against a JSON schema
- `NewSchemaValidator(schema, root, path, formats, ...Option) *SchemaValidator`
- `post.ApplyDefaults(result)` / `post.Prune(result)`

## Dependencies

- `github.com/go-openapi/spec`, `analysis`, `loads`, `errors`, `strfmt`

## Conventions

- All `.go` files must have SPDX license headers (Apache-2.0).
- Commits require DCO sign-off (`git commit -s`).
- Linting: `golangci-lint run` — config in `.golangci.yml` (posture: `default: all` with explicit disables).
- Every `//nolint` directive **must** have an inline comment explaining why.
- Tests: `go test ./...` with `-race`. CI runs on `{ubuntu, macos, windows} x {stable, oldstable}`.
- Test framework: `github.com/go-openapi/testify/v2` (not `stretchr/testify`).

See `.github/copilot/` (symlinked to `.claude/rules/`) for detailed rules on Go conventions, linting, and testing.
