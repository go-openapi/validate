// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"encoding/json"
	"testing"

	"github.com/go-openapi/analysis"
	"github.com/go-openapi/loads"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"
)

// indexRefs analyzes a document body, wrapped into the smallest valid spec.
func indexRefs(t *testing.T, body string) refLocations {
	t.Helper()

	doc := `{"swagger":"2.0","info":{"title":"t","version":"1"},` + body + `}`
	d, err := loads.Analyzed(json.RawMessage(doc), "")
	require.NoError(t, err)

	return newRefLocations(analysis.New(d.Spec()))
}

func TestRefLocations_FindsDeclarations(t *testing.T) {
	t.Parallel()

	locations := indexRefs(t, `
		"paths": {
			"/pets": {
				"get": {
					"parameters": [
						{"name": "id", "in": "path"},
						{"$ref": "#/parameters/tagsParam"}
					],
					"responses": {"200": {"schema": {"$ref": "#/definitions/Pet"}}}
				}
			}
		},
		"definitions": {
			"Pet": {"properties": {"owner": {"$ref": "#/definitions/Owner"}}},
			"Remote": {"$ref": "https://elsewhere.example/schema.json"}
		}`)

	assert.EqualT(t, "/paths/~1pets/get/parameters/1", locations.at("#/parameters/tagsParam").pointer())
	assert.EqualT(t, "/paths/~1pets/get/responses/200/schema", locations.at("#/definitions/Pet").pointer())
	assert.EqualT(t, "/definitions/Pet/properties/owner", locations.at("#/definitions/Owner").pointer())
	assert.EqualT(t, "/definitions/Remote", locations.at("https://elsewhere.example/schema.json").pointer())
}

func TestRefLocations_UnknownReferenceIsTheRoot(t *testing.T) {
	t.Parallel()

	locations := indexRefs(t, `"definitions": {"Pet": {"type": "object"}}`)

	assert.True(t, locations.at("#/definitions/Nope").isEmpty())
	assert.EqualT(t, "", locations.at("#/definitions/Nope").pointer())
}

func TestRefLocations_SkipsExampleData(t *testing.T) {
	t.Parallel()

	// an example may legitimately hold a "$ref" member: it declares nothing
	locations := indexRefs(t, `
		"definitions": {
			"Pet": {
				"example": {"$ref": "#/definitions/NotAReference"},
				"examples": {"application/json": {"$ref": "#/definitions/NotAReferenceEither"}}
			}
		}`)

	assert.True(t, locations.at("#/definitions/NotAReference").isEmpty())
	assert.True(t, locations.at("#/definitions/NotAReferenceEither").isEmpty())
}

func TestRefLocations_DefaultResponseIsADeclarationNotAValue(t *testing.T) {
	t.Parallel()

	locations := indexRefs(t, `
		"paths": {
			"/pets": {
				"get": {
					"responses": {
						"default": {"$ref": "#/responses/operationError"}
					}
				}
			}
		},
		"definitions": {
			"Pet": {
				"default": {"$ref": "#/definitions/NotAReference"}
			}
		}`)

	assert.EqualT(t, "/paths/~1pets/get/responses/default",
		locations.at("#/responses/operationError").pointer())
	assert.True(t, locations.at("#/definitions/NotAReference").isEmpty(),
		"a default value holds data: a $ref member there declares nothing")
}

func TestRefLocations_TieBreakIsStable(t *testing.T) {
	t.Parallel()

	// the same reference declared twice: whichever is kept, it must be the
	// same one on every run, since maps are walked in random order
	const doc = `
		"definitions": {
			"Zebra": {"$ref": "#/definitions/Pet"},
			"Ant": {"$ref": "#/definitions/Pet"}
		}`

	first := indexRefs(t, doc).at("#/definitions/Pet").pointer()
	for range 20 {
		assert.EqualT(t, first, indexRefs(t, doc).at("#/definitions/Pet").pointer())
	}
	assert.EqualT(t, "/definitions/Ant", first)
}

func TestRefLocations_CoverEveryAnalyzedReference(t *testing.T) {
	t.Parallel()

	// the index and the two diagnostics read the same analyzer, so every
	// reference they can report is one the index knows where to find
	d, err := loads.Analyzed(json.RawMessage(`{
		"swagger": "2.0",
		"info": {"title": "t", "version": "1"},
		"parameters": {"tagsParam": {"name": "tags", "in": "query", "type": "string"}},
		"paths": {
			"/pets": {
				"get": {
					"parameters": [{"$ref": "#/parameters/tagsParam"}],
					"responses": {
						"200": {"description": "ok", "schema": {"$ref": "#/definitions/Pet"}},
						"default": {"$ref": "#/responses/oops"}
					}
				}
			}
		},
		"responses": {"oops": {"description": "oops"}},
		"definitions": {
			"Pet": {"type": "object", "properties": {"owner": {"$ref": "#/definitions/Owner"}}},
			"Owner": {"type": "object"}
		}
	}`), "")
	require.NoError(t, err)

	analyzer := analysis.New(d.Spec())
	locations := newRefLocations(analyzer)

	refs := analyzer.AllRefs()
	require.NotEmpty(t, refs)
	for _, found := range refs {
		ref := found
		assert.False(t, locations.at(ref.String()).isEmpty(),
			"expected a location for %q", ref.String())
	}
}

func TestValidateDubiousRefs_LocatesTheReference(t *testing.T) {
	t.Parallel()

	doc := `{
		"swagger": "2.0",
		"info": {"title": "t", "version": "1"},
		"paths": {},
		"definitions": {
			"A": {"$ref": "file:///etc/passwd"}
		}
	}`

	res := dubiousValidatorFromJSON(t, doc).validateDubiousRefs()
	located := res.LocatedWarnings()
	require.Len(t, located, 1)

	assert.StringContainsT(t, located[0].Err.Error(), "escapes the spec's base path")
	assert.EqualT(t, "/definitions/A", located[0].Pointer)
}

func TestRefLocations_ReportedThroughSpecValidation(t *testing.T) {
	t.Parallel()

	// end to end: refLocations is built by Validate, so an invalid $ref
	// reported to a caller of the public API carries where it was declared.
	const raw = `{
	  "swagger": "2.0",
	  "info": {"title": "t", "version": "1"},
	  "paths": {"/pets": {"get": {"operationId": "g", "responses": {"200": {"description": "ok"}}}}},
	  "definitions": {"Bad": {"$ref": "file:///etc/passwd"}}
	}`

	doc, err := loads.Analyzed(json.RawMessage(raw), "")
	require.NoError(t, err)

	res, _ := NewSpecValidator(doc.Schema(), strfmt.Default).Validate(doc)
	require.False(t, res.IsValid())

	located := res.LocatedErrors()
	require.Len(t, located, 1)
	assert.StringContainsT(t, located[0].Err.Error(), "invalid ref")
	assert.EqualT(t, "/definitions/Bad", located[0].Pointer)
}

func TestValidateDubiousRefs_HostSpreadHasNoSingleLocation(t *testing.T) {
	t.Parallel()

	// the warning is about the set of hosts, not about one reference
	doc := `{
		"swagger": "2.0",
		"info": {"title": "t", "version": "1"},
		"paths": {},
		"definitions": {
			"A": {"$ref": "http://host-one.example/a.json"},
			"B": {"$ref": "https://host-two.example/b.json"}
		}
	}`

	res := dubiousValidatorFromJSON(t, doc).validateDubiousRefs()
	located := res.LocatedWarnings()
	require.Len(t, located, 1)
	assert.Empty(t, located[0].Pointer)
}
