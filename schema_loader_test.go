// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag/loading"
	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"
)

// TestSchemaValidator_WithPathLoader verifies that a document loader injected through
// WithPathLoader is the one used to resolve a remote $ref when building a schema validator — the
// hook a caller uses to confine loading when validating input from an untrusted source.
func TestSchemaValidator_WithPathLoader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"type":"string"}`))
	}))
	defer srv.Close()

	var calls int
	loader := func(pth string, opts ...loading.Option) (json.RawMessage, error) {
		calls++
		return loading.LoadFromFileOrHTTP(pth, opts...)
	}

	schema := &spec.Schema{SchemaProps: spec.SchemaProps{Ref: spec.MustCreateRef(srv.URL + "/schema.json")}}
	v := NewSchemaValidator(schema, nil, "", strfmt.Default, WithPathLoader(loader))

	require.NotNil(t, v)
	assert.TrueT(t, calls > 0, "expected the injected loader to resolve the remote $ref")
}

// TestSpecValidator_WithPathLoader verifies that WithPathLoader passed to the spec validator
// constructors reaches the schema options consumed by both the schema validation and the $ref
// resolution ($ref resolveRef) performed during spec validation.
func TestSpecValidator_WithPathLoader(t *testing.T) {
	loader := func(string, ...loading.Option) (json.RawMessage, error) { return nil, nil }

	s := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default, WithPathLoader(loader))
	require.NotNil(t, s.schemaOptions)
	require.NotNil(t, s.schemaOptions.pathLoaderWithOptions,
		"WithPathLoader must reach the spec validator's schema options (used by resolveRef and schema validation)")

	// built-in defaults are still applied alongside the injected option
	assert.TrueT(t, s.schemaOptions.EnableObjectArrayTypeCheck)
	assert.TrueT(t, s.schemaOptions.recycleValidators)
}
