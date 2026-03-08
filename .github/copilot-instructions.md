# Copilot Instructions

## Project Overview

Go implementation of a OpenAPI v2 (swagger) validator and JSON Schema draft 4 validator

## Package Layout (single package)

| File | Contents |
|------|----------|

## Key API

## Design Decisions

## Dependencies

## Conventions

- All `.go` files must have SPDX license headers (Apache-2.0).
- Commits require DCO sign-off (`git commit -s`).
- Linting: `golangci-lint run` — config in `.golangci.yml` (posture: `default: all` with explicit disables).
- Every `//nolint` directive **must** have an inline comment explaining why.
- Tests: `go test ./...` with `-race`. CI runs on `{ubuntu, macos, windows} x {stable, oldstable}`.
- Test framework: `github.com/go-openapi/testify/v2` (not `stretchr/testify`).

See `.github/copilot/` (symlinked to `.claude/rules/`) for detailed rules on Go conventions, linting, and testing.
