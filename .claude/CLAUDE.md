# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Go library for validating OpenAPI v2 (Swagger) specifications and JSON Schema draft 4 data.
It is part of the [go-openapi](https://github.com/go-openapi) ecosystem and used by [go-swagger](https://github.com/go-swagger/go-swagger).

The library provides two main capabilities:
1. **Spec validation** — validates a Swagger 2.0 spec document against the JSON meta-schema, plus extra semantic rules (path uniqueness, parameter consistency, $ref resolution, etc.)
2. **Schema validation** — validates arbitrary data against a JSON Schema draft 4 schema (tested against the official JSON-Schema-Test-Suite)

API is stable. This is legacy/maintenance code — deep refactoring is not worthwhile;
a ground-up replacement is the long-term plan.

See [docs/MAINTAINERS.md](../docs/MAINTAINERS.md) for CI/CD, release process, and repo structure details.

### Package layout

| Package | Contents |
|---------|----------|
| `validate` (root) | Spec validator (`SpecValidator`), schema validator (`SchemaValidator`), `AgainstSchema()`, individual type/format/object/slice/string/number validators, result handling, pools |
| `post` | Post-validation transforms: `ApplyDefaults` (fills in schema defaults) and `Prune` (removes additional properties) |

### Key API

- `Spec(doc, formats) error` — high-level spec validation
- `NewSpecValidator(schema, formats) *SpecValidator` — configurable spec validator
- `AgainstSchema(schema, data, formats, ...Option) error` — validate data against a JSON schema
- `NewSchemaValidator(schema, root, path, formats, ...Option) *SchemaValidator` — configurable schema validator
- `post.ApplyDefaults(result)` — apply default values from validation result
- `post.Prune(result)` — remove undeclared properties from validation result

### Dependencies

Key runtime dependencies (see `go.mod` for full list):
- `github.com/go-openapi/spec` — Swagger 2.0 spec model
- `github.com/go-openapi/analysis` — spec analysis and flattening
- `github.com/go-openapi/loads` — spec loading (used in tests/examples)
- `github.com/go-openapi/errors` — structured validation errors
- `github.com/go-openapi/strfmt` — format registry (date-time, uuid, email, etc.)
- `github.com/go-openapi/testify/v2` — test-only assertions (zero-dep testify fork)

### Architecture notes

The validator chain is built from `valueValidator` implementations (type, format, string, number, slice, object, common, schemaProps), assembled by `SchemaValidator`. Validators and results are pooled for performance (`pools.go`). The codebase has known complexity issues (many `//nolint:gocognit` deferrals) stemming from the original design.

