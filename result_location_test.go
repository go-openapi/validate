// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"
)

// Locations are kept in a slice parallel to the errors. The tests below guard
// that invariant across the operations that reshuffle a Result, and check the
// pointers actually resolve into the validated document.

func TestResultLocations_AlignedAcrossMerges(t *testing.T) {
	t.Parallel()

	first := new(Result)
	first.addErrorsAt(newPathSegments("a", "0"), errOne)
	first.AddErrors(errAnother) // no location

	second := new(Result)
	second.addErrorsAt(newPathSegments("b"), errNew)
	second.addWarningsAt(newPathSegments("c"), errOneWarning)

	first.Merge(second)

	located := first.LocatedErrors()
	require.Len(t, located, 3)
	assert.EqualT(t, "/a/0", located[0].Pointer)
	assert.EqualT(t, errOne, located[0].Err)
	assert.Empty(t, located[1].Pointer, "expected AddErrors to leave the location unknown")
	assert.EqualT(t, errAnother, located[1].Err)
	assert.EqualT(t, "/b", located[2].Pointer)
	assert.EqualT(t, errNew, located[2].Err)

	warnings := first.LocatedWarnings()
	require.Len(t, warnings, 1)
	assert.EqualT(t, "/c", warnings[0].Pointer)
}

func TestResultLocations_SurviveMergeAsWarnings(t *testing.T) {
	t.Parallel()

	source := new(Result)
	source.addErrorsAt(newPathSegments("a"), errOne)
	source.addWarningsAt(newPathSegments("b"), errOneWarning)

	target := new(Result)
	target.MergeAsWarnings(source)

	assert.Empty(t, target.Errors)
	located := target.LocatedWarnings()
	require.Len(t, located, 2)
	assert.EqualT(t, "/a", located[0].Pointer)
	assert.EqualT(t, "/b", located[1].Pointer)
}

func TestResultLocations_SurviveMergeAsErrors(t *testing.T) {
	t.Parallel()

	source := new(Result)
	source.addErrorsAt(newPathSegments("a"), errOne)
	source.addWarningsAt(newPathSegments("b"), errOneWarning)

	target := new(Result)
	target.MergeAsErrors(source)

	located := target.LocatedErrors()
	require.Len(t, located, 2)
	assert.EqualT(t, "/a", located[0].Pointer)
	assert.EqualT(t, "/b", located[1].Pointer)
}

func TestResultLocations_DedupeKeepsTheFirstLocation(t *testing.T) {
	t.Parallel()

	// AddErrors drops a message that is already reported; the location slice
	// must not gain an entry for the error that was dropped.
	res := new(Result)
	res.addErrorsAt(newPathSegments("a"), errOne)
	res.addErrorsAt(newPathSegments("b"), errOne)

	require.Len(t, res.Errors, 1)
	located := res.LocatedErrors()
	require.Len(t, located, 1)
	assert.EqualT(t, "/a", located[0].Pointer)
}

func TestResultLocations_ClearedOnRecycle(t *testing.T) {
	t.Parallel()

	res := pools.poolOfResults.BorrowResult()
	res.addErrorsAt(newPathSegments("a"), errOne)
	require.Len(t, res.LocatedErrors(), 1)

	res = res.cleared()
	assert.Empty(t, res.Errors)
	assert.Empty(t, res.LocatedErrors(), "expected a recycled result to leak no location")
}

func TestResultLocations_KeepRelevantErrors(t *testing.T) {
	t.Parallel()

	res := new(Result)
	res.addErrorsAt(newPathSegments("dropped"), errOne)
	res.addErrorsAt(newPathSegments("kept"), errImportant)

	stripped := res.keepRelevantErrors()
	located := stripped.LocatedErrors()
	require.Len(t, located, 1)
	assert.EqualT(t, "/kept", located[0].Pointer)
}

func TestResultLocations_UnknownWhenNeverRecorded(t *testing.T) {
	t.Parallel()

	res := new(Result)
	res.AddErrors(errOne, errAnother)

	for _, located := range res.LocatedErrors() {
		assert.Empty(t, located.Pointer)
	}
}

func TestResultLocations_SchemaValidation(t *testing.T) {
	t.Parallel()

	schema := new(spec.Schema)
	require.NoError(t, json.Unmarshal([]byte(`{
		"type": "object",
		"properties": {
			"friends": {
				"type": "array",
				"items": {"type": "object", "properties": {"name": {"type": "string"}}, "required": ["age"]}
			},
			"n~x/y": {"type": "string", "maxLength": 2}
		}
	}`), schema))

	data := map[string]any{
		"friends": []any{map[string]any{nameProp: 42}},
		"n~x/y":   "far too long",
	}

	res := NewSchemaValidator(schema, nil, "", strfmt.Default).Validate(data)
	require.False(t, res.IsValid())

	pointers := make([]string, 0, len(res.Errors))
	for _, located := range res.LocatedErrors() {
		pointers = append(pointers, located.Pointer)
	}

	assert.SliceContainsT(t, pointers, "/friends/0/name")
	assert.SliceContainsT(t, pointers, "/friends/0/age")
	assert.SliceContainsT(t, pointers, "/n~0x~1y", "expected the token to be escaped")
}

func TestResultLocations_SpecValidation(t *testing.T) {
	t.Parallel()

	const raw = `{
	  "swagger": "2.0",
	  "info": {"title": "t", "version": "1"},
	  "paths": {
	    "/pets/{id}": {
	      "get": {
	        "operationId": "getPet",
	        "parameters": [{"name": "id", "in": "path", "required": true, "type": "string"}],
	        "responses": {
	          "200": {
	            "description": "ok",
	            "schema": {"$ref": "#/definitions/Pet"},
	            "examples": {"application/json": {"friends": [{"name": 7}]}}
	          }
	        }
	      }
	    }
	  },
	  "definitions": {
	    "Pet": {
	      "type": "object",
	      "properties": {
	        "name": {"type": "string", "default": 42},
	        "friends": {"type": "array", "items": {"$ref": "#/definitions/Pet"}}
	      }
	    },
	    "Unused": {"type": "object"}
	  }
	}`

	doc, err := loads.Analyzed(json.RawMessage(raw), "")
	require.NoError(t, err)

	res, warns := NewSpecValidator(doc.Schema(), strfmt.Default).Validate(doc)
	require.False(t, res.IsValid())

	t.Run("a default is located in the definition that declares it", func(t *testing.T) {
		assert.SliceContainsT(t, pointersOf(res.LocatedErrors()), "/definitions/Pet/name/default")
	})

	t.Run("an example is located under its response", func(t *testing.T) {
		assert.SliceContainsT(t, pointersOf(warns.LocatedErrors()),
			"/paths/~1pets~1{id}/get/responses/200/examples/friends/0/name")
	})

	t.Run("an unused definition is located", func(t *testing.T) {
		assert.SliceContainsT(t, pointersOf(warns.LocatedErrors()), "/definitions/Unused")
	})

	t.Run("every reported location is a valid JSON pointer", func(t *testing.T) {
		for _, located := range append(res.LocatedErrors(), warns.LocatedErrors()...) {
			if located.Pointer == "" {
				continue
			}

			assert.EqualT(t, uint8('/'), located.Pointer[0],
				"expected a pointer to start with a separator, got %q", located.Pointer)
		}
	})
}

func TestResultLocations_RequiredEntryIsLocated(t *testing.T) {
	t.Parallel()

	// the TUI anchors on where the offending text sits, so a required entry
	// that names no property points at the entry, not at the definition
	const raw = `{
	  "swagger": "2.0",
	  "info": {"title": "t", "version": "1"},
	  "paths": {"/pets": {"get": {"operationId": "g", "responses": {"200": {"description": "ok"}}}}},
	  "definitions": {
	    "Pet": {
	      "type": "object",
	      "required": ["name", "notDeclared"],
	      "properties": {
	        "name": {"type": "string"},
	        "readOnlyOne": {"type": "string", "readOnly": true}
	      }
	    },
	    "Owner": {
	      "type": "object",
	      "required": ["readOnlyOne"],
	      "properties": {"readOnlyOne": {"type": "string", "readOnly": true}}
	    }
	  }
	}`

	doc, err := loads.Analyzed(json.RawMessage(raw), "")
	require.NoError(t, err)

	validator := NewSpecValidator(doc.Schema(), strfmt.Default)
	validator.SetContinueOnErrors(true)
	res, warns := validator.Validate(doc)

	t.Run("an undefined required property points at its entry", func(t *testing.T) {
		var found bool
		for _, located := range res.LocatedErrors() {
			if !strings.Contains(located.Err.Error(), "notDeclared") {
				continue
			}

			found = true
			// index 1 of Pet's required array, not "/definitions/Pet"
			assert.EqualT(t, "/definitions/Pet/required/1", located.Pointer)
		}
		require.True(t, found, "expected the undefined required property to be reported")
	})

	t.Run("a required and readOnly property points at its entry", func(t *testing.T) {
		var found bool
		for _, located := range warns.LocatedErrors() {
			if !strings.Contains(located.Err.Error(), "readOnly") {
				continue
			}

			found = true
			assert.EqualT(t, "/definitions/Owner/required/0", located.Pointer)
		}
		require.True(t, found, "expected the readOnly-and-required warning to be reported")
	})
}

func pointersOf(located []Located) []string {
	pointers := make([]string, 0, len(located))
	for _, l := range located {
		pointers = append(pointers, l.Pointer)
	}

	return pointers
}
