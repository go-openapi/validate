// Copyright 2015 go-swagger maintainers
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
	"testing"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test edge cases in schema_props_validator which are difficult
// to simulate with specs
// (this one is a trivial, just to check all methods are filled)
func TestSchemaPropsValidator_EdgeCases(t *testing.T) {
	t.Run("should validate props against empty validator", func(t *testing.T) {
		s := newSchemaPropsValidator(
			"", "", nil, nil, nil, nil, nil, nil, strfmt.Default, nil)
		s.SetPath("path")
		assert.Equal(t, "path", s.Path)
	})

	t.Run("with allOf", func(t *testing.T) {
		makeValidator := func() EntityValidator {
			return newSchemaPropsValidator(
				"path", "body",
				[]spec.Schema{
					*spec.StringProperty(),
					*spec.StrFmtProperty("date"),
				}, nil, nil, nil, nil, nil, strfmt.Default, &SchemaValidatorOptions{recycleValidators: true})
		}

		t.Run("should validate date string", func(t *testing.T) {
			s := makeValidator()

			const data = "2024-01-25"
			res := s.Validate(data)
			require.NotNil(t, res)
			require.Empty(t, res.Errors)
		})

		t.Run("should NOT validate unformatted string", func(t *testing.T) {
			s := makeValidator()

			const data = "string_value"
			res := s.Validate(data)
			require.NotNil(t, res)
			require.NotEmpty(t, res.Errors)
		})

		t.Run("should NOT validate number", func(t *testing.T) {
			s := makeValidator()

			const data = 1
			res := s.Validate(data)
			require.NotNil(t, res)
			require.NotEmpty(t, res.Errors)
		})
	})

	t.Run("with oneOf", func(t *testing.T) {
		makeValidator := func() EntityValidator {
			return newSchemaPropsValidator(
				"path", "body",
				nil,
				[]spec.Schema{
					*spec.Int64Property(),
					*spec.StrFmtProperty("date"),
				}, nil, nil, nil, nil, strfmt.Default, &SchemaValidatorOptions{recycleValidators: true})
		}

		t.Run("should validate date string", func(t *testing.T) {
			s := makeValidator()

			const data = "2024-01-01"
			res := s.Validate(data)
			require.NotNil(t, res)
			require.Empty(t, res.Errors)
		})

		t.Run("should validate number", func(t *testing.T) {
			s := makeValidator()

			const data = 1
			res := s.Validate(data)
			require.NotNil(t, res)
			require.Empty(t, res.Errors)
		})
	})

	t.Run("with anyOf", func(t *testing.T) {
		makeValidator := func() EntityValidator {
			return newSchemaPropsValidator(
				"path", "body",
				nil,
				nil,
				[]spec.Schema{
					*spec.StringProperty(),
					*spec.StrFmtProperty("date"),
				}, nil, nil, nil, strfmt.Default, &SchemaValidatorOptions{recycleValidators: true})
		}

		t.Run("should validate date string", func(t *testing.T) {
			s := makeValidator()

			const data = "2024-01-01"
			res := s.Validate(data)
			require.NotNil(t, res)
			require.Empty(t, res.Errors)
		})

		t.Run("should validate unformatted string", func(t *testing.T) {
			s := makeValidator()

			const data = "string_value"
			res := s.Validate(data)
			require.NotNil(t, res)
			require.Empty(t, res.Errors)
		})
	})

	t.Run("with not", func(t *testing.T) {
		makeValidator := func() EntityValidator {
			return newSchemaPropsValidator(
				"path", "body",
				nil,
				nil,
				nil,
				spec.StringProperty(),
				nil, nil, strfmt.Default, &SchemaValidatorOptions{recycleValidators: true})
		}

		t.Run("should validate number", func(t *testing.T) {
			s := makeValidator()

			const data = 1
			res := s.Validate(data)
			require.NotNil(t, res)
			require.Empty(t, res.Errors)
		})

		t.Run("should NOT validate string", func(t *testing.T) {
			s := makeValidator()

			const data = "string_value"
			res := s.Validate(data)
			require.NotNil(t, res)
			require.NotEmpty(t, res.Errors)
		})
	})

	t.Run("with nested schema props", func(t *testing.T) {
		makeValidator := func() EntityValidator {
			return newSchemaValidator(
				&spec.Schema{
					SchemaProps: spec.SchemaProps{
						AllOf: []spec.Schema{
							{
								SchemaProps: spec.SchemaProps{
									OneOf: []spec.Schema{
										{
											SchemaProps: spec.SchemaProps{
												AnyOf: []spec.Schema{
													{
														SchemaProps: spec.SchemaProps{
															Not: spec.StringProperty(),
														},
													},
													*spec.BoolProperty(),
												},
											},
										},
										*spec.StringProperty(),
									},
								},
							},
							*spec.Int64Property(),
						},
					},
				},
				nil,
				"root",
				strfmt.Default, &SchemaValidatorOptions{recycleValidators: true})
		}

		t.Run("should validate number", func(t *testing.T) {
			s := makeValidator()

			const data = 1
			res := s.Validate(data)
			require.NotNil(t, res)
			require.Empty(t, res.Errors)
		})

		t.Run("should NOT validate string", func(t *testing.T) {
			s := makeValidator()

			const data = "string_value"
			res := s.Validate(data)
			require.NotNil(t, res)
			require.NotEmpty(t, res.Errors)
		})

		t.Run("should exit early and redeem children validator", func(t *testing.T) {
			s := makeValidator()

			res := s.Validate(nil)
			require.NotNil(t, res)
			require.Empty(t, res.Errors)
		})
	})
}
