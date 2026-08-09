// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"
)

func TestPathSegmentsRendering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    pathSegments
		dotted  string
		pointer string
	}{
		{
			name:    "document root",
			path:    rootPath(),
			dotted:  "",
			pointer: "",
		},
		{
			name:    "single token",
			path:    rootPath().child("definitions"),
			dotted:  "definitions",
			pointer: "/definitions",
		},
		{
			name:    "nested properties",
			path:    newPathSegments("definitions", "Pet", "name"),
			dotted:  "definitions.Pet.name",
			pointer: "/definitions/Pet/name",
		},
		{
			name:    "array item",
			path:    newPathSegments("friends").item(0).child("name"),
			dotted:  "friends.0.name",
			pointer: "/friends/0/name",
		},
		{
			name:    "token holding the dotted separator",
			path:    newPathSegments("a.b", "c"),
			dotted:  "a.b.c", // ambiguous, which is the whole point of the pointer form
			pointer: "/a.b/c",
		},
		{
			name:    "token needing RFC 6901 escaping",
			path:    newPathSegments("n~x/y", "0"),
			dotted:  "n~x/y.0",
			pointer: "/n~0x~1y/0",
		},
		{
			name:    "templated swagger path",
			path:    newPathSegments("paths", "/pets/{id}", "get"),
			dotted:  "paths./pets/{id}.get",
			pointer: "/paths/~1pets~1{id}/get",
		},
		{
			name:    "empty token",
			path:    newPathSegments("properties", ""),
			dotted:  "properties.",
			pointer: "/properties/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.dotted, tt.path.dotted())
			assert.Equal(t, tt.dotted, tt.path.String())
			assert.Equal(t, tt.pointer, tt.path.pointer())
		})
	}
}

func TestPathSegmentsDisplayDiffersFromPointer(t *testing.T) {
	t.Parallel()

	// a document addresses an array member by index, while a message reads
	// far better naming it
	path := newPathSegments("paths", "/pets", "get", "parameters").childAs("1", "tags")

	assert.EqualT(t, "/paths/~1pets/get/parameters/1", path.pointer())
	assert.EqualT(t, "paths./pets.get.parameters.tags", path.dotted())

	t.Run("structure reads the addressed token, not the readable one", func(t *testing.T) {
		t.Parallel()

		assert.EqualT(t, "1", path.last(), "expected the token the document uses")
		assert.True(t, path.hasSuffix(newPathSegments("parameters", "1")))
		assert.False(t, path.hasSuffix(newPathSegments("parameters", "tags")))
		assert.EqualT(t, "paths./pets.get.parameters", path.trimIndexes().dotted())
	})

	t.Run("children of a renamed token keep both forms", func(t *testing.T) {
		t.Parallel()

		child := path.child("items")
		assert.EqualT(t, "/paths/~1pets/get/parameters/1/items", child.pointer())
		assert.EqualT(t, "paths./pets.get.parameters.tags.items", child.dotted())
	})
}

func TestPathSegmentsStructuralTokensAreAddressedNotShown(t *testing.T) {
	t.Parallel()

	// a document needs "properties" to address a member of a schema, but no
	// message has ever spelled it
	path := newPathSegments("definitions", "Pet").
		structuralChild(jsonProperties).
		child("name").
		child(jsonDefault)

	assert.EqualT(t, "/definitions/Pet/properties/name/default", path.pointer())
	assert.EqualT(t, "definitions.Pet.name.default", path.dotted())

	t.Run("a structural token is not what the value is", func(t *testing.T) {
		t.Parallel()

		// the predicates telling schema from data ask what the value is, and
		// plumbing must not answer
		inside := newPathSegments("responses", "200", "examples").structuralChild("application/json")
		assert.EqualT(t, swaggerExamples, inside.last())
		assert.EqualT(t, "200", inside.beforeLast())
	})

	t.Run("structural tokens still address", func(t *testing.T) {
		t.Parallel()

		assert.True(t, path.hasSuffix(newPathSegments("properties", "name", "default")),
			"expected structure to see the token a document uses")
	})
}

func TestPathSegmentsAppendIsCopyOnWrite(t *testing.T) {
	t.Parallel()

	// a parent spawning several children is the common case: each child must
	// get its own backing array, or siblings overwrite one another.
	parent := newPathSegments("definitions", "Pet")
	require.Equal(t, 2, len(parent))

	first := parent.child("name")
	second := parent.child("age")
	third := parent.item(3)

	assert.Equal(t, "definitions.Pet", parent.dotted())
	assert.Equal(t, "definitions.Pet.name", first.dotted())
	assert.Equal(t, "definitions.Pet.age", second.dotted())
	assert.Equal(t, "definitions.Pet.3", third.dotted())
}

func TestPathSegmentsChildren(t *testing.T) {
	t.Parallel()

	parent := newPathSegments("paths")

	assert.Equal(t, "paths./pets.get.responses", parent.children("/pets", "get", "responses").dotted())
	assert.Equal(t, "/paths/~1pets/get/responses", parent.children("/pets", "get", "responses").pointer())
	assert.Equal(t, "paths", parent.children().dotted())
	assert.Equal(t, "paths", parent.dotted(), "expected the parent to be left alone")
}

func TestPathSegmentsInspection(t *testing.T) {
	t.Parallel()

	t.Run("with an empty path", func(t *testing.T) {
		t.Parallel()

		empty := rootPath()
		assert.True(t, empty.isEmpty())
		assert.Empty(t, empty.last())
		assert.Empty(t, empty.beforeLast())
	})

	t.Run("with a single token", func(t *testing.T) {
		t.Parallel()

		single := newPathSegments("properties")
		assert.False(t, single.isEmpty())
		assert.Equal(t, "properties", single.last())
		assert.Empty(t, single.beforeLast(), "expected no token before the only one")
	})

	t.Run("with several tokens", func(t *testing.T) {
		t.Parallel()

		path := newPathSegments("definitions", "Pet", "properties")
		assert.Equal(t, "properties", path.last())
		assert.Equal(t, "Pet", path.beforeLast())
	})
}

func TestPathSegmentsHasSuffix(t *testing.T) {
	t.Parallel()

	path := newPathSegments("definitions", "Pet", "friends", "Pet")

	assert.True(t, path.hasSuffix(newPathSegments("Pet")))
	assert.True(t, path.hasSuffix(newPathSegments("friends", "Pet")))
	assert.True(t, path.hasSuffix(path))
	assert.True(t, path.hasSuffix(rootPath()), "expected the empty suffix to always match")

	assert.False(t, path.hasSuffix(newPathSegments("friends")))
	assert.False(t, path.hasSuffix(newPathSegments("Dog")))
	assert.False(t, path.hasSuffix(newPathSegments("definitions", "Pet", "friends", "Pet", "name")))

	// tokens are compared whole: no mid-token match like the dotted string used to allow
	assert.False(t, newPathSegments("abc").hasSuffix(newPathSegments("bc")))
}
