// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"encoding/json"
	"testing"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"
)

const (
	nameProp  = "name"
	dummyProp = "dummy"
)

// The tests below pin down where a validation error says it happened.
//
// They exist because locations used to be assembled by string concatenation,
// which lost array indices altogether, mixed dotted and bracketed notations,
// and duplicated keyword suffixes.

func TestErrorLocation_ArrayItemsCarryTheirIndex(t *testing.T) {
	t.Parallel()

	// the item index used to be dropped: the sub-validators that report the
	// error were built before the index was known.
	schema := new(spec.Schema)
	require.NoError(t, json.Unmarshal([]byte(`{
		"type": "object",
		"properties": {
			"friends": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {"name": {"type": "string"}},
					"required": ["age"]
				}
			}
		}
	}`), schema))

	data := map[string]any{
		"friends": []any{
			map[string]any{nameProp: "ok", "age": 1},
			map[string]any{nameProp: 42},
		},
	}

	res := NewSchemaValidator(schema, nil, "", strfmt.Default).Validate(data)
	require.False(t, res.IsValid())

	messages := errorMessages(res)
	assert.SliceContainsT(t, messages, "friends.1.name in body must be of type string: \"integer\"")
	assert.SliceContainsT(t, messages, "friends.1.age in body is required")

	for _, msg := range messages {
		assert.StringNotContainsT(t, msg, "friends.name", "expected no location to elide the item index")
	}
}

func TestErrorLocation_TupleItemsCarryTheirIndex(t *testing.T) {
	t.Parallel()

	schema := new(spec.Schema)
	require.NoError(t, json.Unmarshal([]byte(`{
		"type": "array",
		"items": [{"type": "string"}, {"type": "integer"}]
	}`), schema))

	res := NewSchemaValidator(schema, nil, "", strfmt.Default).Validate([]any{1, "two"})
	require.False(t, res.IsValid())

	messages := errorMessages(res)
	assert.SliceContainsT(t, messages, "0 in body must be of type string: \"integer\"")
	assert.SliceContainsT(t, messages, "1 in body must be of type integer: \"string\"")
}

func TestErrorLocation_RootRequiredHasNoLeadingSeparator(t *testing.T) {
	t.Parallel()

	// a missing property at the root used to be reported as ".swagger",
	// because the empty root was concatenated with a separator.
	schema := new(spec.Schema)
	require.NoError(t, json.Unmarshal([]byte(`{
		"type": "object",
		"required": ["swagger"],
		"properties": {"swagger": {"type": "string"}}
	}`), schema))

	res := NewSchemaValidator(schema, nil, "", strfmt.Default).Validate(map[string]any{})
	require.False(t, res.IsValid())
	require.Len(t, res.Errors, 1)

	assert.EqualError(t, res.Errors[0], "swagger in body is required")
}

func TestErrorLocation_SpecDefaultsAndExamples(t *testing.T) {
	t.Parallel()

	const raw = `{
	  "swagger": "2.0",
	  "info": {"title": "t", "version": "1"},
	  "paths": {
	    "/pets/{id}": {
	      "get": {
	        "operationId": "getPet",
	        "parameters": [
	          {"name": "id", "in": "path", "required": true, "type": "string"}
	        ],
	        "responses": {
	          "200": {
	            "description": "ok",
	            "schema": {"$ref": "#/definitions/Pet"},
	            "examples": {
	              "application/json": {"friends": [{"name": 7}]}
	            }
	          }
	        }
	      }
	    }
	  },
	  "definitions": {
	    "Pet": {
	      "type": "object",
	      "properties": {
	        "name": {"type": "string"},
	        "friends": {"type": "array", "items": {"$ref": "#/definitions/Pet"}},
	        "tuple": {
	          "type": "array",
	          "items": [{"type": "string", "default": 1}, {"type": "integer", "default": "x"}]
	        }
	      }
	    }
	  }
	}`

	doc, err := loads.Analyzed(json.RawMessage(raw), "")
	require.NoError(t, err)

	res, _ := NewSpecValidator(doc.Schema(), strfmt.Default).Validate(doc)
	require.False(t, res.IsValid())

	errs := errorMessages(res)
	warns := warningMessages(res)

	t.Run("tuple item defaults use an index token, not a bracket", func(t *testing.T) {
		assert.SliceContainsT(t, errs,
			"definitions.Pet.tuple.items.0.default in body must be of type string: \"number\"")
		assert.SliceContainsT(t, errs,
			"definitions.Pet.tuple.items.1.default in body must be of type integer: \"string\"")

		for _, msg := range errs {
			assert.StringNotContainsT(t, msg, "items[", "expected no bracketed index notation")
		}
	})

	t.Run("the default keyword is not appended twice", func(t *testing.T) {
		for _, msg := range append(errs, warns...) {
			assert.StringNotContainsT(t, msg, ".default.default")
			assert.StringNotContainsT(t, msg, ".example.example")
		}
	})

	t.Run("an example inside an array carries the item index", func(t *testing.T) {
		assert.SliceContainsT(t, warns,
			"paths./pets/{id}.get.responses.200.examples.friends.0.name in body must be of type string: \"number\"")
	})

	t.Run("a response is located by its operation, not by the bare status code", func(t *testing.T) {
		for _, msg := range append(errs, warns...) {
			assert.NotEqualT(t, "200 in response has invalid pattern", msg)
		}
		assert.SliceContainsT(t, warns,
			"paths./pets/{id}.get.responses.200.examples.friends.0.name in body must be of type string: \"number\"")
	})

	t.Run("the method is spelled as the document spells it", func(t *testing.T) {
		for _, msg := range append(errs, warns...) {
			assert.StringNotContainsT(t, msg, ".GET.", "expected the lower-case path item key")
		}
	})
}

func TestErrorLocation_ExampleItemsSkipSchemaOnlyChecks(t *testing.T) {
	t.Parallel()

	// restoring the item index moved example data from "x.example" to
	// "x.example.0", which must still count as being inside an example:
	// the array-must-have-items check does not apply to plain data.
	validator := newObjectValidator(
		newPathSegments("itemsparam", swaggerExample, "0"),
		"body", nil, nil, nil, nil, nil, nil, nil, nil,
		&SchemaValidatorOptions{EnableObjectArrayTypeCheck: true, EnableArrayMustHaveItemsCheck: true},
	)

	res := validator.Validate(map[string]any{jsonItems: dummyProp})
	assert.Empty(t, res.Errors, "expected schema-only checks to be skipped inside an example")
}

func errorMessages(res *Result) []string {
	messages := make([]string, 0, len(res.Errors))
	for _, err := range res.Errors {
		messages = append(messages, err.Error())
	}

	return messages
}
