// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"
)

const paramLocationsFixture = `{
  "swagger": "2.0",
  "info": {"title": "t", "version": "1"},
  "parameters": {
    "sharedBody": {"name": "user", "in": "body", "schema": {"type": "object"}}
  },
  "paths": {
    "/pets": {
      "parameters": [
        {"name": "onPathItem", "in": "query", "type": "string"}
      ],
      "get": {
        "operationId": "list",
        "parameters": [
          {"name": "first", "in": "query", "type": "string"},
          {"name": "second", "in": "header", "type": "string"},
          {"$ref": "#/parameters/sharedBody"}
        ],
        "responses": {"200": {"description": "ok"}}
      },
      "post": {
        "operationId": "create",
        "parameters": [{"name": "only", "in": "query", "type": "string"}],
        "responses": {"200": {"description": "ok"}}
      }
    }
  }
}`

const (
	methodGet  = "GET"
	methodPost = "POST"
	inQuery    = "query"
	inHeader   = "header"
)

func paramLocationsOf(t *testing.T) paramLocations {
	t.Helper()

	d, err := loads.Analyzed(json.RawMessage(paramLocationsFixture), "")
	require.NoError(t, err)

	return newParamLocations(d.Spec())
}

func TestParamLocations_AddressedByIndex(t *testing.T) {
	t.Parallel()

	locations := paramLocationsOf(t)

	for _, tt := range []struct {
		name    string
		in      string
		method  string
		pointer string
	}{
		{name: "first", in: inQuery, method: methodGet, pointer: "/paths/~1pets/get/parameters/0"},
		{name: "second", in: inHeader, method: methodGet, pointer: "/paths/~1pets/get/parameters/1"},
		{name: "only", in: inQuery, method: methodPost, pointer: "/paths/~1pets/post/parameters/0"},
	} {
		assert.EqualT(t, tt.pointer, locations.at("/pets", tt.method, tt.in, tt.name).pointer(),
			"unexpected location for %q", tt.name)
	}
}

func TestParamLocations_FallsBackToThePathItem(t *testing.T) {
	t.Parallel()

	locations := paramLocationsOf(t)

	// declared once by the path item, shared by every operation under it
	for _, method := range []string{methodGet, methodPost} {
		assert.EqualT(t, "/paths/~1pets/parameters/0",
			locations.at("/pets", method, inQuery, "onPathItem").pointer(),
			"unexpected location under %s", method)
	}
}

func TestParamLocations_ResolvesASharedParameter(t *testing.T) {
	t.Parallel()

	locations := paramLocationsOf(t)

	// the entry is written as a $ref, so its name comes from the definition,
	// while the location stays the site the operation declares it at
	assert.EqualT(t, "/paths/~1pets/get/parameters/2",
		locations.at("/pets", methodGet, "body", "user").pointer())
}

func TestParamLocations_UnknownParameterKeepsItsName(t *testing.T) {
	t.Parallel()

	locations := paramLocationsOf(t)

	// a parameter too broken to be identified has no index to point at, so
	// the name stands in: the location no longer resolves, but a message
	// built from it still says what it is about
	unknown := locations.at("/pets", methodGet, inQuery, "neverDeclared")
	assert.EqualT(t, "/paths/~1pets/get/parameters/neverDeclared", unknown.pointer())
	assert.EqualT(t, "paths./pets.get.parameters.neverDeclared", unknown.dotted())
}

func TestParamLocations_PointerIndexesButMessageNames(t *testing.T) {
	t.Parallel()

	locations := paramLocationsOf(t)
	located := locations.at("/pets", methodGet, inHeader, "second")

	assert.EqualT(t, "/paths/~1pets/get/parameters/1", located.pointer(),
		"the document addresses a parameter by index")
	assert.EqualT(t, "paths./pets.get.parameters.second", located.dotted(),
		"a message is more useful naming it")
}

func TestParamLocations_ReportedThroughSpecValidation(t *testing.T) {
	t.Parallel()

	// a query parameter typed as an array without items: reported against the
	// entry the operation declares, which is what a document can address
	const raw = `{
	  "swagger": "2.0",
	  "info": {"title": "t", "version": "1"},
	  "paths": {
	    "/pets": {
	      "get": {
	        "operationId": "list",
	        "parameters": [
	          {"name": "ok", "in": "query", "type": "string"},
	          {"name": "tags", "in": "query", "type": "array"}
	        ],
	        "responses": {"200": {"description": "ok"}}
	      }
	    }
	  }
	}`

	doc, err := loads.Analyzed(json.RawMessage(raw), "")
	require.NoError(t, err)

	validator := NewSpecValidator(doc.Schema(), strfmt.Default)
	validator.SetContinueOnErrors(true)
	res, _ := validator.Validate(doc)

	const declaredAt = "/paths/~1pets/get/parameters/1"

	var found bool
	for _, located := range res.LocatedErrors() {
		if !strings.Contains(located.Err.Error(), "tags") && !strings.HasPrefix(located.Pointer, declaredAt) {
			continue
		}

		found = true
		// the check may report the parameter itself or something inside it,
		// but never the operation or the array holding it
		assert.StringContainsT(t, located.Pointer, declaredAt,
			"expected %q to be located in the offending parameter", located.Err)
	}
	require.True(t, found, "expected the array parameter without items to be reported")
}
