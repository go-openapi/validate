// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"encoding/json"
	"testing"

	"github.com/go-openapi/analysis"
	"github.com/go-openapi/loads"
	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"
)

const chainedRefsFixture = `{
  "swagger": "2.0",
  "info": {"title": "t", "version": "1"},
  "paths": {
    "/a": {"get": {"operationId": "a",
      "responses": {"200": {"$ref": "#/responses/shared"}}}},
    "/loop": {"get": {"operationId": "loop",
      "responses": {"200": {"description": "ok", "schema": {"$ref": "#/definitions/Recursive"}}}}}
  },
  "responses": {"shared": {"description": "ok", "schema": {"$ref": "#/definitions/A"}}},
  "definitions": {
    "A": {"type": "object", "properties": {"n": {"type": "string"}}},
    "Recursive": {"type": "object", "properties": {"self": {"$ref": "#/definitions/Recursive"}}}
  }
}`

func redirectsOf(t *testing.T) refRedirects {
	t.Helper()

	doc, err := loads.Analyzed(json.RawMessage(chainedRefsFixture), "")
	require.NoError(t, err)

	return newRefRedirects(analysis.New(doc.Spec()))
}

func TestRefRedirects_FollowsToWhatTheDocumentHolds(t *testing.T) {
	t.Parallel()

	redirects := redirectsOf(t)

	t.Run("a pointer below a $ref is followed, as many times as it takes", func(t *testing.T) {
		t.Parallel()

		// the response is a $ref to a shared response whose schema is itself a
		// $ref: two hops before the pointer lands on something written down
		assert.EqualT(t, "/definitions/A/properties/n",
			redirects.through("/paths/~1a/get/responses/200/schema/properties/n"))
	})

	t.Run("a pointer that stops on the $ref is left alone", func(t *testing.T) {
		t.Parallel()

		// that node exists, and it is where a reader goes to amend the reference
		assert.EqualT(t, "/paths/~1a/get/responses/200",
			redirects.through("/paths/~1a/get/responses/200"))
	})

	t.Run("a pointer that crosses no $ref is left alone", func(t *testing.T) {
		t.Parallel()

		assert.EqualT(t, "/definitions/A/properties/n",
			redirects.through("/definitions/A/properties/n"))
		assert.EqualT(t, "", redirects.through(""))
	})

	t.Run("a definition referring to itself does not spin", func(t *testing.T) {
		t.Parallel()

		// each hop shortens the pointer by "properties/self", so this bottoms
		// out; the hop bound is what guarantees it whatever the document says
		assert.EqualT(t, "/definitions/Recursive/properties/self",
			redirects.through("/definitions/Recursive/properties/self/properties/self"))
	})
}

func TestRefRedirects_IgnoresRemoteReferences(t *testing.T) {
	t.Parallel()

	doc, err := loads.Analyzed(json.RawMessage(`{
  "swagger": "2.0",
  "info": {"title": "t", "version": "1"},
  "paths": {"/a": {"get": {"operationId": "a",
    "responses": {"200": {"description": "ok", "schema": {"$ref": "elsewhere.json#/definitions/A"}}}}}}
}`), "")
	require.NoError(t, err)

	// nothing in this document says what a remote reference leads to, so the
	// pointer is left as it stands and trimming has the last word
	redirects := newRefRedirects(analysis.New(doc.Spec()))
	assert.EqualT(t, "/paths/~1a/get/responses/200/schema/properties/n",
		redirects.through("/paths/~1a/get/responses/200/schema/properties/n"))
}
