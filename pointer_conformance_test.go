// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
)

// conformanceCase is one minimal document with one fault, and the node a reader
// has to go to in order to amend it.
type conformanceCase struct {
	name string
	doc  string
	// says which finding of the document the case is about
	message string
	pointer string
}

// The grid below pins the location of one finding per kind of fault, over the
// whole shape of a Swagger document: the root, the info block, an operation, a
// parameter wherever it may be declared, a response, a definition.
//
// It is a conformance matrix rather than a set of regression tests. Each row
// works today; the point is that none of them may quietly stop working, since
// a location is only ever as useful as it is stable.
var conformanceCases = []conformanceCase{
	{
		name:    "a document with no info block",
		doc:     `{"swagger":"2.0","paths":{"/a":{"get":{"operationId":"a","responses":{"200":{"description":"ok"}}}}}}`,
		message: "info in body is required",
		pointer: "",
	},
	{
		name:    "a document with no paths",
		doc:     `{"swagger":"2.0","info":{"title":"t","version":"1"}}`,
		message: "paths in body is required",
		pointer: "",
	},
	{
		name:    "an info block with no title",
		doc:     `{"swagger":"2.0","info":{"version":"1"},"paths":{}}`,
		message: "info.title in body is required",
		pointer: "/info",
	},
	{
		name:    "a license with no name",
		doc:     `{"swagger":"2.0","info":{"title":"t","version":"1","license":{"url":"http://x"}},"paths":{}}`,
		message: "info.license.name in body is required",
		pointer: "/info/license",
	},
	{
		name:    "external documentation with no url",
		doc:     `{"swagger":"2.0","info":{"title":"t","version":"1"},"paths":{},"externalDocs":{"description":"d"}}`,
		message: "externalDocs.url in body is required",
		pointer: "/externalDocs",
	},
	{
		name:    "a tag with no name",
		doc:     `{"swagger":"2.0","info":{"title":"t","version":"1"},"paths":{},"tags":[{"description":"d"}]}`,
		message: "tags.0.name in body is required",
		pointer: "/tags/0",
	},
	{
		name: "a response with no description",
		doc: `{"swagger":"2.0","info":{"title":"t","version":"1"},
 "paths":{"/a":{"get":{"operationId":"a","responses":{"200":{}}}}}}`,
		message: "paths./a.get.responses.200.description in body is required",
		pointer: "/paths/~1a/get/responses/200",
	},
	{
		name: "a shared response with no description",
		doc: `{"swagger":"2.0","info":{"title":"t","version":"1"},
 "paths":{"/a":{"get":{"operationId":"a","responses":{"200":{"$ref":"#/responses/shared"}}}}},
 "responses":{"shared":{}}}`,
		message: "responses.shared.description in body is required",
		pointer: "/responses/shared",
	},
	{
		name: "an operation with no responses",
		doc: `{"swagger":"2.0","info":{"title":"t","version":"1"},
 "paths":{"/a":{"get":{"operationId":"a"}}}}`,
		message: "paths./a.get.responses in body is required",
		pointer: "/paths/~1a/get",
	},
	{
		name: "a body parameter with no schema",
		doc: `{"swagger":"2.0","info":{"title":"t","version":"1"},
 "paths":{"/a":{"post":{"operationId":"a","parameters":[{"name":"p","in":"body"}],
   "responses":{"200":{"description":"ok"}}}}}}`,
		message: "invalid definition for parameter p in body in operation \"a\"",
		pointer: "/paths/~1a/post/parameters/0",
	},
	{
		name: "a parameter with no name",
		doc: `{"swagger":"2.0","info":{"title":"t","version":"1"},
 "paths":{"/a":{"get":{"operationId":"a","parameters":[{"in":"query","type":"string"}],
   "responses":{"200":{"description":"ok"}}}}}}`,
		message: "paths./a.get.parameters.0.name in body is required",
		pointer: firstParam,
	},
	{
		name: "a shared parameter with no type",
		doc: `{"swagger":"2.0","info":{"title":"t","version":"1"},
 "paths":{"/a":{"get":{"operationId":"a","parameters":[{"$ref":"#/parameters/shared"}],
   "responses":{"200":{"description":"ok"}}}}},
 "parameters":{"shared":{"name":"s","in":"query"}}}`,
		message: "parameters.shared.type in body is required",
		pointer: "/parameters/shared",
	},
	{
		name: "a path-item parameter with no type",
		doc: `{"swagger":"2.0","info":{"title":"t","version":"1"},
 "paths":{"/a":{"parameters":[{"name":"s","in":"query"}],
   "get":{"operationId":"a","responses":{"200":{"description":"ok"}}}}}}`,
		message: "paths./a.parameters.0.type in body is required",
		pointer: "/paths/~1a/parameters/0",
	},
	{
		name: "an array parameter with no items",
		doc: `{"swagger":"2.0","info":{"title":"t","version":"1"},
 "paths":{"/a":{"get":{"operationId":"a","parameters":[{"name":"s","in":"query","type":"array"}],
   "responses":{"200":{"description":"ok"}}}}}}`,
		message: "param \"s\" for \"a\" is a collection without an element type (array requires item definition)",
		pointer: firstParam,
	},
	{
		name: "a response header with no type",
		doc: `{"swagger":"2.0","info":{"title":"t","version":"1"},
 "paths":{"/a":{"get":{"operationId":"a","responses":{"200":{"description":"ok",
   "headers":{"X":{"format":"int32"}}}}}}}}`,
		message: "paths./a.get.responses.200.headers.X.type in body is required",
		pointer: "/paths/~1a/get/responses/200/headers/X",
	},
	{
		name: "an array response schema with no items",
		doc: `{"swagger":"2.0","info":{"title":"t","version":"1"},
 "paths":{"/a":{"get":{"operationId":"a","responses":{"200":{"description":"ok",
   "schema":{"type":"array"}}}}}}}`,
		message: "response body for \"a\" is a collection without an element type (array requires items definition)",
		pointer: "/paths/~1a/get/responses/200/schema",
	},
	{
		name: "an array definition property with no items",
		doc: `{"swagger":"2.0","info":{"title":"t","version":"1"},
 "paths":{"/a":{"get":{"operationId":"a","responses":{"200":{"description":"ok",
   "schema":{"$ref":"#/definitions/A"}}}}}},
 "definitions":{"A":{"type":"object","properties":{"list":{"type":"array"}}}}}`,
		message: "items in definitions.A.properties.list is required",
		pointer: "/definitions/A/properties/list",
	},
	{
		name: "a security definition with no type",
		doc: `{"swagger":"2.0","info":{"title":"t","version":"1"},"paths":{},
 "securityDefinitions":{"k":{"name":"api_key","in":"header"}}}`,
		message: "securityDefinitions.k.type in body is required",
		pointer: "/securityDefinitions/k",
	},
	{
		name: "a required entry naming an undeclared property",
		doc: `{"swagger":"2.0","info":{"title":"t","version":"1"},
 "paths":{"/a":{"get":{"operationId":"a","responses":{"200":{"description":"ok",
   "schema":{"$ref":"#/definitions/A"}}}}}},
 "definitions":{"A":{"type":"object","required":["ghost"],"properties":{"real":{"type":"string"}}}}}`,
		message: `"ghost" is present in required but not defined as property in definition "A"`,
		pointer: "/definitions/A/required/0",
	},
	{
		name: "a path parameter absent from the path template",
		doc: `{"swagger":"2.0","info":{"title":"t","version":"1"},
 "paths":{"/a":{"get":{"operationId":"a",
   "parameters":[{"name":"id","in":"path","required":true,"type":"string"}],
   "responses":{"200":{"description":"ok"}}}}}}`,
		message: `path param "id" is not present in path "/a"`,
		pointer: "/paths/~1a",
	},
	{
		name: "more than one body parameter",
		doc: `{"swagger":"2.0","info":{"title":"t","version":"1"},
 "paths":{"/a":{"post":{"operationId":"a","parameters":[
   {"name":"one","in":"body","schema":{"type":"string"}},
   {"name":"two","in":"body","schema":{"type":"string"}}],
   "responses":{"200":{"description":"ok"}}}}}}`,
		message: `operation "a" has more than 1 body param: ["one" "two"]`,
		pointer: "/paths/~1a/post",
	},
	{
		name: "a nested array parameter with no items",
		doc: `{"swagger":"2.0","info":{"title":"t","version":"1"},
 "paths":{"/a":{"get":{"operationId":"a",
   "parameters":[{"name":"s","in":"query","type":"array","items":{"type":"array"}}],
   "responses":{"200":{"description":"ok"}}}}}}`,
		message: "items in paths./a.get.parameters.s.items is required",
		pointer: "/paths/~1a/get/parameters/0/items",
	},
	{
		name: "an array additionalProperties with no items",
		doc: `{"swagger":"2.0","info":{"title":"t","version":"1"},
 "paths":{"/a":{"get":{"operationId":"a","responses":{"200":{"description":"ok",
   "schema":{"$ref":"#/definitions/A"}}}}}},
 "definitions":{"A":{"type":"object","additionalProperties":{"type":"array"}}}}`,
		message: "items in definitions.A.additionalProperties is required",
		pointer: "/definitions/A/additionalProperties",
	},
	{
		name: "an array property of an allOf member with no items",
		doc: `{"swagger":"2.0","info":{"title":"t","version":"1"},
 "paths":{"/a":{"get":{"operationId":"a","responses":{"200":{"description":"ok",
   "schema":{"$ref":"#/definitions/A"}}}}}},
 "definitions":{"A":{"allOf":[{"type":"object","properties":{"l":{"type":"array"}}}]}}}`,
		message: "items in definitions.A.allOf.0.properties.l is required",
		pointer: "/definitions/A/allOf/0/properties/l",
	},
	{
		name: "a property declared twice through allOf",
		doc: `{"swagger":"2.0","info":{"title":"t","version":"1"},
 "paths":{"/a":{"get":{"operationId":"a","responses":{"200":{"description":"ok",
   "schema":{"$ref":"#/definitions/Child"}}}}}},
 "definitions":{
   "Base":{"type":"object","properties":{"dup":{"type":"string"}}},
   "Child":{"allOf":[{"$ref":"#/definitions/Base"},{"type":"object","properties":{"dup":{"type":"string"}}}]}}}`,
		message: `definition "Child" contains duplicate properties: [Child.dup]`,
		pointer: "/definitions/Child",
	},
	{
		name: "a parameter declared twice in an operation",
		doc: `{"swagger":"2.0","info":{"title":"t","version":"1"},
 "paths":{"/a":{"get":{"operationId":"a","parameters":[
   {"name":"s","in":"query","type":"string"},
   {"name":"s","in":"query","type":"string"}],
   "responses":{"200":{"description":"ok"}}}}}}`,
		message: `duplicate parameter name "s" for "query" in operation "a"`,
		// the second declaration is the offending one
		pointer: "/paths/~1a/get/parameters/1",
	},
}

func TestPointerConformance(t *testing.T) {
	t.Parallel()

	for _, testCase := range conformanceCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			found := locatedFindings(t, testCase.doc)

			matched := false
			for message, pointer := range found {
				if message != testCase.message {
					continue
				}

				matched = true
				assert.EqualT(t, testCase.pointer, pointer, "for %q", message)
			}

			assert.TrueT(t, matched, "expected a finding containing %q, got %v", testCase.message, found)
		})
	}
}
