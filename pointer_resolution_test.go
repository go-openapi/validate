// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/go-openapi/jsonpointer"
	"github.com/go-openapi/loads"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"
)

// The tests below pin down that a reported location addresses a node the
// document contains, and which node that is.
//
// They exist because a check walks an expanded, model-level view of a
// specification: parameters merged in from a path item, schemata reached
// through a $ref, values validated by the runtime validators a generated client
// uses. Each of those used to produce a pointer that walked off the document.

// resolvedPointers pairs every finding of a document with the pointer reported
// for it, having first verified that the pointer addresses something.
type resolvedPointers map[string]string

func TestPointerResolution_ValueBelowItsHolder(t *testing.T) {
	t.Parallel()

	t.Run("a simple parameter default is located on the default", func(t *testing.T) {
		t.Parallel()

		// the parameter is named zzz so that a pointer built from the name
		// rather than from the document is unmistakable
		found := locatedFindings(t, `{"swagger":"2.0","info":{"title":"t","version":"1"},
 "paths":{"/a":{"get":{"operationId":"a",
   "parameters":[{"name":"zzz","in":"query","type":"integer","default":"nope"}],
   "responses":{"200":{"description":"ok"}}}}}}`)

		assert.EqualT(t, "/paths/~1a/get/parameters/0/default",
			found[`zzz in query must be of type integer: "string"`])
		assert.EqualT(t, "/paths/~1a/get/parameters/0",
			found["default value for zzz in query does not validate its schema"])
	})

	t.Run("a response header default is located on the default", func(t *testing.T) {
		t.Parallel()

		found := locatedFindings(t, `{"swagger":"2.0","info":{"title":"t","version":"1"},
 "paths":{"/a":{"get":{"operationId":"a","responses":{"200":{"description":"ok",
   "headers":{"X-Count":{"type":"integer","default":"nope"}}}}}}}}`)

		assert.EqualT(t, "/paths/~1a/get/responses/200/headers/X-Count/default",
			found[`X-Count in header must be of type integer: "string"`])
	})

	t.Run("a body parameter is entered through its schema", func(t *testing.T) {
		t.Parallel()

		// the response side gained this token first; a body parameter holds its
		// schema under the same member and needs the same one
		found := locatedFindings(t, `{"swagger":"2.0","info":{"title":"t","version":"1"},
 "paths":{"/a":{"post":{"operationId":"a",
   "parameters":[{"name":"payload","in":"body",
     "schema":{"type":"object","properties":{"n":{"type":"integer","default":"nope"}}}}],
   "responses":{"200":{"description":"ok"}}}}}}`)

		assert.EqualT(t, "/paths/~1a/post/parameters/0/schema/properties/n/default",
			found[`paths./a.post.parameters.payload.n.default in body must be of type integer: "string"`])
	})
}

func TestPointerResolution_PathTemplate(t *testing.T) {
	t.Parallel()

	// the variable name used to stand where the path key belongs, unescaped:
	// ghostvar makes it plain which token was taken
	found := locatedFindings(t, `{"swagger":"2.0","info":{"title":"t","version":"1"},
 "paths":{"/deep/nested/{ghostvar}":{"get":{"operationId":"a",
   "responses":{"200":{"description":"ok"}}}}}}`)

	assert.EqualT(t, "/paths/~1deep~1nested~1{ghostvar}",
		found[`path param "{ghostvar}" has no parameter definition`])
}

func TestPointerResolution_FindingsWithADefiniteSite(t *testing.T) {
	t.Parallel()

	t.Run("a duplicate operationId names the first operation using it", func(t *testing.T) {
		t.Parallel()

		found := locatedFindings(t, `{"swagger":"2.0","info":{"title":"t","version":"1"},
 "paths":{"/a":{"get":{"operationId":"dup","responses":{"200":{"description":"ok"}}}},
          "/b":{"get":{"operationId":"dup","responses":{"200":{"description":"ok"}}}}}}`)

		assert.EqualT(t, "/paths/~1a/get/operationId", found[`"dup" is defined 2 times`])
	})

	t.Run("an unresolvable reference is located where it is declared", func(t *testing.T) {
		t.Parallel()

		found := locatedFindings(t, `{"swagger":"2.0","info":{"title":"t","version":"1"},
 "paths":{"/a":{"get":{"operationId":"a","responses":{"200":{"description":"ok",
   "schema":{"$ref":"#/definitions/Missing"}}}}}}}`)

		// expansion reports the whole document in one message, so the finding
		// is given the site of the reference it could not follow
		require.NotEmpty(t, found)
		for message, pointer := range found {
			if strings.Contains(message, "could not be resolved") {
				assert.EqualT(t, "/paths/~1a/get/responses/200/schema", pointer)
			}
		}
	})
}

func TestPointerResolution_ThroughARef(t *testing.T) {
	t.Parallel()

	// the response is a bare $ref, so nothing exists below it: the location has
	// to be followed to what the document does hold, hopping twice on the way
	found := locatedFindings(t, `{"swagger":"2.0","info":{"title":"t","version":"1"},
 "paths":{"/a":{"get":{"operationId":"a","responses":{"200":{"$ref":"#/responses/shared"}}}}},
 "responses":{"shared":{"description":"ok","schema":{"$ref":"#/definitions/A"}}},
 "definitions":{"A":{"type":"object","default":{"n":"not-an-object"},
   "properties":{"n":{"type":"object"}}}}}`)

	assert.EqualT(t, "/definitions/A/default/n",
		found[`paths./a.get.responses.200.default.n in body must be of type object: "string"`])
	assert.EqualT(t, "/paths/~1a/get/responses/200",
		found[`in operation "a", default value in response 200 does not validate its schema`])
}

// TestPointerResolution_EveryFixture is the standing guarantee: no document in
// the corpus may report a location that addresses nothing.
func TestPointerResolution_EveryFixture(t *testing.T) {
	t.Parallel()

	var files []string
	for _, pattern := range []string{
		filepath.Join("fixtures", "validation", "*.json"),
		filepath.Join("fixtures", "validation", "*.yaml"),
		filepath.Join("fixtures", "bugs", "*", "*.json"),
		filepath.Join("fixtures", "bugs", "*", "*.yaml"),
		filepath.Join("fixtures", "go-swagger", "*", "*", "*.json"),
		filepath.Join("fixtures", "go-swagger", "*", "*", "*.yaml"),
		filepath.Join("fixtures", "petstore", "*.json"),
	} {
		matched, err := filepath.Glob(pattern)
		require.NoError(t, err)
		files = append(files, matched...)
	}
	require.NotEmpty(t, files)

	var specs, findings int
	for _, file := range files {
		doc, err := loads.Spec(file)
		if err != nil {
			// a fixture that does not even load says nothing about locations
			continue
		}

		var document any
		if err := json.Unmarshal(doc.Raw(), &document); err != nil {
			continue
		}

		func() {
			// a handful of fixtures are deliberately degenerate
			defer func() { _ = recover() }()

			validator := NewSpecValidator(doc.Schema(), strfmt.Default)
			validator.Options.ContinueOnErrors = true
			res, _ := validator.Validate(doc)
			if res == nil {
				return
			}
			specs++

			for _, located := range append(res.LocatedErrors(), res.LocatedWarnings()...) {
				findings++
				assert.TrueT(t, addresses(document, located.Pointer),
					"%s: %q addresses nothing (%v)", filepath.Base(file), located.Pointer, located.Err)
			}
		}()
	}

	t.Logf("%d specifications, %d findings", specs, findings)
	require.NotZero(t, findings)
}

// locatedFindings validates a document and returns its findings by message,
// asserting first that every pointer addresses a node the document holds.
func locatedFindings(t *testing.T, raw string) resolvedPointers {
	t.Helper()

	var document any
	require.NoError(t, json.Unmarshal([]byte(raw), &document))

	doc, err := loads.Analyzed(json.RawMessage(raw), "")
	require.NoError(t, err)

	validator := NewSpecValidator(doc.Schema(), strfmt.Default)
	validator.Options.ContinueOnErrors = true
	res, _ := validator.Validate(doc)
	require.NotNil(t, res)

	found := make(resolvedPointers)
	for _, located := range append(res.LocatedErrors(), res.LocatedWarnings()...) {
		assert.TrueT(t, addresses(document, located.Pointer),
			"%q addresses nothing (%v)", located.Pointer, located.Err)
		found[located.Err.Error()] = located.Pointer
	}

	return found
}

// addresses walks a JSON pointer over a decoded document, independently of the
// code that produced it.
func addresses(document any, pointer string) bool {
	if pointer == "" {
		return true // the whole document
	}
	if !strings.HasPrefix(pointer, "/") {
		return false
	}

	node := document
	for token := range strings.SplitSeq(pointer[1:], "/") {
		switch held := node.(type) {
		case map[string]any:
			member, isHeld := held[jsonpointer.Unescape(token)]
			if !isHeld {
				return false
			}
			node = member
		case []any:
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(held) {
				return false
			}
			node = held[index]
		default:
			return false
		}
	}

	return true
}
