// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"encoding/json"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"
)

const (
	firstParam     = "/paths/~1a/get/parameters/0"
	firstParamName = firstParam + "/name"
)

func validatorHoldingDocument(t *testing.T, raw string) *SpecValidator {
	t.Helper()

	var document any
	require.NoError(t, json.Unmarshal([]byte(raw), &document))

	return &SpecValidator{document: document}
}

func TestResolvable_TrimsToWhatTheDocumentHolds(t *testing.T) {
	t.Parallel()

	s := validatorHoldingDocument(t, `{
  "paths": {"/a": {"get": {"parameters": [{"name": "p", "in": "query"}]}}},
  "definitions": {"a~b": {"type": "object"}}
}`)

	for _, testCase := range []struct {
		name    string
		pointer string
		want    string
	}{
		{"a pointer that addresses a node is untouched", firstParamName, firstParamName},
		{"a member the node does not hold is cut off", firstParam + "/type", firstParam},
		{"everything below the first miss goes too", firstParam + "/schema/properties/n", firstParam},
		{"an index past the end of an array is cut off", "/paths/~1a/get/parameters/1", "/paths/~1a/get/parameters"},
		{"a member of a scalar is cut off", firstParamName + "/deeper", firstParamName},
		{"an escaped token is unescaped before the lookup", "/definitions/a~0b/type", "/definitions/a~0b/type"},
		{"a miss at the first token leaves the root", "/nowhere/at/all", ""},
		{"the root is the root", "", ""},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.EqualT(t, testCase.want, s.resolvable(testCase.pointer))
		})
	}
}

func TestResolvable_WithoutADocumentSaysNothing(t *testing.T) {
	t.Parallel()

	// nothing is known about what holds what, so no location may be shortened
	s := new(SpecValidator)
	assert.EqualT(t, "/paths/~1a", s.resolvable("/paths/~1a"))
}

func TestPathSegments_CosmeticTokensStayOutOfThePointer(t *testing.T) {
	t.Parallel()

	at := newPathSegments(swaggerPaths, "/a", "get").child(swaggerParameters).cosmeticChild("broken")

	assert.EqualT(t, "/paths/~1a/get/parameters", at.pointer())
	assert.EqualT(t, "paths./a.get.parameters.broken", at.dotted())

	t.Run("nothing below a cosmetic token is addressable either", func(t *testing.T) {
		t.Parallel()

		below := at.child(jsonType)
		assert.EqualT(t, "/paths/~1a/get/parameters", below.pointer())
		assert.EqualT(t, "paths./a.get.parameters.broken.type", below.dotted())

		deeper := at.children(jsonSchema, jsonProperties)
		assert.EqualT(t, "/paths/~1a/get/parameters", deeper.pointer())
		assert.EqualT(t, "paths./a.get.parameters.broken.schema.properties", deeper.dotted())
	})
}
