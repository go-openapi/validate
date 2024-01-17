// Copyright 2017 go-swagger maintainers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validate

import (
	"reflect"
	"strings"
	"testing"

	"github.com/go-openapi/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func itemsFixture() map[string]interface{} {
	return map[string]interface{}{
		"type":  "array",
		"items": "dummy",
	}
}

func expectAllValid(t *testing.T, ov EntityValidator, dataValid, dataInvalid map[string]interface{}) {
	res := ov.Validate(dataValid)
	assert.Empty(t, res.Errors)

	res = ov.Validate(dataInvalid)
	assert.Empty(t, res.Errors)
}

func expectOnlyInvalid(t *testing.T, ov EntityValidator, dataValid, dataInvalid map[string]interface{}) {
	res := ov.Validate(dataValid)
	assert.Empty(t, res.Errors)

	res = ov.Validate(dataInvalid)
	assert.NotEmpty(t, res.Errors)
}

func TestItemsMustBeTypeArray(t *testing.T) {
	ov := newObjectValidator("", "", nil, nil, nil, nil, nil, nil, nil, nil, nil)
	dataValid := itemsFixture()
	dataInvalid := map[string]interface{}{
		"type":  "object",
		"items": "dummy",
	}
	expectAllValid(t, ov, dataValid, dataInvalid)

	ov.Options.EnableObjectArrayTypeCheck = true
	expectOnlyInvalid(t, ov, dataValid, dataInvalid)
}

func TestItemsMustHaveType(t *testing.T) {
	ov := newObjectValidator("", "", nil, nil, nil, nil, nil, nil, nil, nil, nil)
	dataValid := itemsFixture()
	dataInvalid := map[string]interface{}{
		"items": "dummy",
	}
	expectAllValid(t, ov, dataValid, dataInvalid)

	ov.Options.EnableObjectArrayTypeCheck = true
	expectOnlyInvalid(t, ov, dataValid, dataInvalid)
}

func TestTypeArrayMustHaveItems(t *testing.T) {
	ov := newObjectValidator("", "", nil, nil, nil, nil, nil, nil, nil, nil, nil)
	dataValid := itemsFixture()
	dataInvalid := map[string]interface{}{
		"type": "array",
		"key":  "dummy",
	}
	expectAllValid(t, ov, dataValid, dataInvalid)

	ov.Options.EnableArrayMustHaveItemsCheck = true
	expectOnlyInvalid(t, ov, dataValid, dataInvalid)
}

// Test edge cases in object_validator which are difficult
// to simulate with specs
// (this one is a trivial, just to check all methods are filled)
func TestObjectValidator_EdgeCases(t *testing.T) {
	s := newObjectValidator("", "", nil, nil, nil, nil, nil, nil, nil, nil, nil)
	s.SetPath("path")
	assert.Equal(t, "path", s.Path)
}

func TestObjectValidatorApply(t *testing.T) {
	s := newObjectValidator("", "", nil, nil, nil, nil, nil, nil, nil, nil, nil)
	require.True(t, s.Applies(&spec.Schema{}, reflect.Map))
	require.False(t, s.Applies(&spec.Response{}, reflect.Map))
	require.False(t, s.Applies(&struct{}{}, reflect.Map))
}

func TestObjectValidatorPatternProperties(t *testing.T) {
	patternWithValid := spec.SchemaProperties{
		"valid": spec.Schema{
			SchemaProps: spec.SchemaProps{
				Type: []string{"string"},
			},
		},
		"#(.((garbled": spec.Schema{
			SchemaProps: spec.SchemaProps{
				Type: []string{"string"},
			},
		},
	}

	patternGarbled := spec.SchemaProperties{
		"#(.((garbled": spec.Schema{
			SchemaProps: spec.SchemaProps{
				Type: []string{"string"},
			},
		},
	}

	t.Run("should ignore invalid regexp in pattern properties", func(t *testing.T) {
		s := newObjectValidator("test", "body", nil, nil, nil, nil, nil, patternWithValid, nil, nil, nil)

		res := s.Validate(map[string]interface{}{"valid": "test_string"})
		require.NotNil(t, res)
		require.Empty(t, res.Errors)
	})

	t.Run("should report forbidden property when invalid regexp in pattern properties", func(t *testing.T) {
		s := newObjectValidator("test", "body", nil, nil, nil, nil, nil, patternGarbled, nil, nil, nil)

		res := s.Validate(map[string]interface{}{"valid": "test_string"})
		require.NotNil(t, res)
		require.Empty(t, res.Errors)
	})

	t.Run("should ignore invalid regexp in pattern properties of additional properties", func(t *testing.T) {
		s := newObjectValidator("test", "body", nil, nil, nil, nil, &spec.SchemaOrBool{
			Schema: &spec.Schema{},
			Allows: false,
		}, patternWithValid, nil, nil, nil)

		res := s.Validate(map[string]interface{}{"valid": "test_string"})
		require.NotNil(t, res)
		require.Empty(t, res.Errors)
	})

	t.Run("should report forbidden property when invalid regexp in pattern properties of additional properties", func(t *testing.T) {
		s := newObjectValidator("test", "body", nil, nil, nil, nil, &spec.SchemaOrBool{
			Schema: &spec.Schema{},
			Allows: false,
		}, patternGarbled, nil, nil, nil)

		res := s.Validate(map[string]interface{}{"valid": "test_string"})
		require.NotNil(t, res)
		require.Len(t, res.Errors, 1)
		require.ErrorContains(t, res.Errors[0], "forbidden property")
	})
}

func TestObjectValidatorNilData(t *testing.T) {
	t.Run("object Validate should NOT panic on nil data", func(t *testing.T) {
		s := newObjectValidator("", "", nil, nil, nil, nil, nil, nil, nil, nil, nil)
		require.NotPanics(t, func() {
			_ = s.Validate(nil)
		})

		res := s.Validate(nil)
		require.NotNil(t, res)
		require.Empty(t, res.Errors)
	})

	t.Run("object Validate should validate required on nil data", func(t *testing.T) {
		s := newObjectValidator("", "", nil, nil, []string{"wanted"}, nil, nil, nil, nil, nil, nil)
		res := s.Validate(nil)
		require.NotNil(t, res)
		require.NotEmpty(t, res.Errors)
	})

	t.Run("object Validate should NOT panic on unexpected input", func(t *testing.T) {
		s := newObjectValidator("", "", nil, nil, []string{"wanted"}, nil, nil, nil, nil, nil, nil)
		res := s.Validate(map[string]string{"wanted": "not expected"})
		require.NotNil(t, res)
		require.Len(t, res.Errors, 1)
		require.ErrorContains(t, res.Errors[0], "expected an object")
	})

	t.Run("object Validate should NOT panic on nil input (with array type check)", func(t *testing.T) {
		s := newObjectValidator("", "", nil, nil, []string{"wanted"}, nil, nil, nil, nil, nil, &SchemaValidatorOptions{
			EnableArrayMustHaveItemsCheck: true,
			EnableObjectArrayTypeCheck:    true,
		})
		res := s.Validate(nil)
		require.NotNil(t, res)
		require.Len(t, res.Errors, 1)
		require.ErrorContains(t, res.Errors[0], "wanted is required")
	})
}

func TestObjectValidatorWithHeaderProperty(t *testing.T) {
	t.Run("should report extra information about forbidden $ref in this context", func(t *testing.T) {
		s := newObjectValidator("test", "body", nil, nil, nil, nil, &spec.SchemaOrBool{
			Schema: &spec.Schema{},
			Allows: false,
		}, nil, nil, nil, nil)

		res := s.Validate(map[string]interface{}{
			"headers": map[string]interface{}{
				"X-Custom": map[string]interface{}{
					"$ref": "#/definitions/myHeader",
				},
			},
		})
		require.NotNil(t, res)
		require.Len(t, res.Errors, 2)
		found := 0
		for _, err := range res.Errors {
			switch {
			case strings.Contains(err.Error(), "forbidden property"):
				found++
			case strings.Contains(err.Error(), "$ref are not allowed in headers"):
				found++
			}
		}
		require.Equal(t, 2, found)
	})

	t.Run("should NOT report extra information when header is not detected", func(t *testing.T) {
		s := newObjectValidator("test", "body", nil, nil, nil, nil, &spec.SchemaOrBool{
			Schema: &spec.Schema{},
			Allows: false,
		}, nil, nil, nil, nil)

		t.Run("when key is not headers", func(t *testing.T) {
			res := s.Validate(map[string]interface{}{
				"Headers": map[string]interface{}{
					"X-Custom": map[string]interface{}{
						"$ref": "#/definitions/myHeader",
					},
				},
			})
			require.NotNil(t, res)
			require.Len(t, res.Errors, 1)
		})

		t.Run("when key is not the expected map", func(t *testing.T) {
			res := s.Validate(map[string]interface{}{
				"headers": map[string]string{
					"X-Custom": "#/definitions/myHeader",
				},
			})
			require.NotNil(t, res)
			require.Len(t, res.Errors, 1)
		})

		t.Run("when key content not the expected map", func(t *testing.T) {
			res := s.Validate(map[string]interface{}{
				"headers": map[string]interface{}{
					"X-Custom": 1,
				},
			})
			require.NotNil(t, res)
			require.Len(t, res.Errors, 1)
		})

		t.Run("when key content not the expected map", func(t *testing.T) {
			res := s.Validate(map[string]interface{}{
				"headers": map[string]interface{}{
					"X-Custom": nil,
				},
			})
			require.NotNil(t, res)
			require.Len(t, res.Errors, 1)
		})

		t.Run("when header is not a valid $ref", func(t *testing.T) {
			res := s.Validate(map[string]interface{}{
				"headers": map[string]interface{}{
					"X-Custom": map[string]interface{}{
						"$ref": 1,
					},
				},
			})
			require.NotNil(t, res)
			require.Len(t, res.Errors, 1)
		})

		t.Run("when header is not a $ref", func(t *testing.T) {
			res := s.Validate(map[string]interface{}{
				"headers": map[string]interface{}{
					"X-Custom": map[string]interface{}{
						"ref": "#/definitions/myHeader",
					},
				},
			})
			require.NotNil(t, res)
			require.Len(t, res.Errors, 1)
		})
	})
}

func TestObjectValidatorWithDefault(t *testing.T) {
	/*
		maxProperties, minProperties *int64, required []string, properties spec.SchemaProperties,
		additionalProperties *spec.SchemaOrBool, patternProperties spec.SchemaProperties,
		root interface{}, formats strfmt.Registry, opts *SchemaValidatorOptions) *objectValidator {
	*/
	t.Run("should accept required populated with a default", func(t *testing.T) {
		s := newObjectValidator("test", "body", nil, nil,
			[]string{"wanted"},
			spec.SchemaProperties{
				"wanted": spec.Schema{
					SchemaProps: spec.SchemaProps{
						Default: "default_value"},
				},
			},
			nil, nil,
			nil, nil, nil)
		res := s.Validate(nil)
		require.NotNil(t, res)
		require.Empty(t, res.Errors)
	})
}
