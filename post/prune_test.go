// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package post

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	validate "github.com/go-openapi/validate"
)

var pruneFixturesPath = filepath.Join("..", "fixtures", "pruning")

func TestPrune(t *testing.T) {
	schema, err := pruningFixture()
	require.NoError(t, err)

	x := map[string]any{
		"foo": 42,
		"bar": 42,
		"x":   42,
		"nested": map[string]any{
			"x": 42,
			"inner": map[string]any{
				"foo": 42,
				"bar": 42,
				"x":   42,
			},
		},
		"all": map[string]any{
			"foo": 42,
			"bar": 42,
			"x":   42,
		},
		"any": map[string]any{
			"foo": 42,
			"bar": 42,
			"x":   42,
		},
		"one": map[string]any{
			"bar": 42,
			"x":   42,
		},
		"array": []any{
			map[string]any{
				"foo": 42,
				"bar": 123,
			},
			map[string]any{
				"x": 42,
				"y": 123,
			},
		},
	}
	t.Logf("Before: %v", x)

	validator := validate.NewSchemaValidator(schema, nil, "", strfmt.Default)
	r := validator.Validate(x)
	assert.Falsef(t, r.HasErrors(), "unexpected validation error: %v", r.AsError())

	Prune(r)
	t.Logf("After: %v", x)
	expected := map[string]any{
		"foo": 42,
		"bar": 42,
		"nested": map[string]any{
			"inner": map[string]any{
				"foo": 42,
				"bar": 42,
			},
		},
		"all": map[string]any{
			"foo": 42,
			"bar": 42,
		},
		"any": map[string]any{
			// intentionally only list one: the first matching
			"foo": 42,
		},
		"one": map[string]any{
			"bar": 42,
		},
		"array": []any{
			map[string]any{
				"foo": 42,
			},
			map[string]any{},
		},
	}
	assert.Equal(t, expected, x)
}

func pruningFixture() (*spec.Schema, error) {
	fname := filepath.Join(pruneFixturesPath, "schema.json")
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
