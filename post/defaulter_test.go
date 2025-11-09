// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package post

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var defaulterFixturesPath = filepath.Join("..", "fixtures", "defaulting")

func TestDefaulter(t *testing.T) {
	schema, err := defaulterFixture()
	require.NoError(t, err)

	validator := validate.NewSchemaValidator(schema, nil, "", strfmt.Default)
	x := defaulterFixtureInput()
	t.Logf("Before: %v", x)

	r := validator.Validate(x)
	assert.Falsef(t, r.HasErrors(), "unexpected validation error: %v", r.AsError())

	ApplyDefaults(r)
	t.Logf("After: %v", x)
	var expected any
	err = json.Unmarshal([]byte(`{
		"existing": 100,
		"int": 42,
		"str": "Hello",
		"obj": {"foo": "bar"},
		"nested": {"inner": 7},
		"all": {"foo": 42, "bar": 42},
		"any": {"foo": 42},
		"one": {"bar": 42}
	}`), &expected)
	require.NoError(t, err)
	assert.Equal(t, expected, x)
}

func TestDefaulterSimple(t *testing.T) {
	schema := spec.Schema{
		SchemaProps: spec.SchemaProps{
			Properties: map[string]spec.Schema{
				"int": {
					SchemaProps: spec.SchemaProps{
						Default: float64(42),
					},
				},
				"str": {
					SchemaProps: spec.SchemaProps{
						Default: "Hello",
					},
				},
			},
		},
	}
	validator := validate.NewSchemaValidator(&schema, nil, "", strfmt.Default)
	x := make(map[string]any)
	t.Logf("Before: %v", x)
	r := validator.Validate(x)
	assert.Falsef(t, r.HasErrors(), "unexpected validation error: %v", r.AsError())

	ApplyDefaults(r)
	t.Logf("After: %v", x)
	var expected any
	err := json.Unmarshal([]byte(`{
		"int": 42,
		"str": "Hello"
	}`), &expected)
	require.NoError(t, err)
	assert.Equal(t, expected, x)
}

func BenchmarkDefaulting(b *testing.B) {
	b.ReportAllocs()

	schema, err := defaulterFixture()
	require.NoError(b, err)

	for b.Loop() {
		validator := validate.NewSchemaValidator(schema, nil, "", strfmt.Default)
		x := defaulterFixtureInput()
		r := validator.Validate(x)
		assert.Falsef(b, r.HasErrors(), "unexpected validation error: %v", r.AsError())
		ApplyDefaults(r)
	}
}

func defaulterFixtureInput() map[string]any {
	return map[string]any{
		"existing": float64(100),
		"nested":   map[string]any{},
		"all":      map[string]any{},
		"any":      map[string]any{},
		"one":      map[string]any{},
	}
}

func defaulterFixture() (*spec.Schema, error) {
	fname := filepath.Join(defaulterFixturesPath, "schema.json")
	b, err := os.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	var schema spec.Schema
	if err := json.Unmarshal(b, &schema); err != nil {
		return nil, err
	}

	return &schema, spec.ExpandSchema(&schema, nil, nil /*new(noopResCache)*/)
}
