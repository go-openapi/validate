// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag/conv"
)

func TestSchemaValidator_Validate_Pattern(t *testing.T) {
	schemaJSON := `
{
    "properties": {
        "name": {
            "type": "string",
            "pattern": "^[A-Za-z]+$",
            "minLength": 1
        },
        "place": {
            "type": "string",
            "pattern": "^[A-Za-z]+$",
            "minLength": 1
        }
    },
    "required": [
        "name"
    ]
}`

	schema := new(spec.Schema)
	require.NoError(t, json.Unmarshal([]byte(schemaJSON), schema))

	var input map[string]any
	inputJSON := `{"name": "Ivan"}`

	require.NoError(t, json.Unmarshal([]byte(inputJSON), &input))
	require.NoError(t, AgainstSchema(schema, input, strfmt.Default))

	input["place"] = json.Number("10")

	require.Error(t, AgainstSchema(schema, input, strfmt.Default))
}

func TestSchemaValidator_PatternProperties(t *testing.T) {
	schemaJSON := `
{
    "properties": {
        "name": {
            "type": "string",
            "pattern": "^[A-Za-z]+$",
            "minLength": 1
        }
	},
    "patternProperties": {
	  "address-[0-9]+": {
         "type": "string",
         "pattern": "^[\\s|a-z]+$"
	  }
    },
    "required": [
        "name"
    ],
	"additionalProperties": false
}`

	schema := new(spec.Schema)
	require.NoError(t, json.Unmarshal([]byte(schemaJSON), schema))

	var input map[string]any

	// ok
	inputJSON := `{"name": "Ivan","address-1": "sesame street"}`
	require.NoError(t, json.Unmarshal([]byte(inputJSON), &input))
	require.NoError(t, AgainstSchema(schema, input, strfmt.Default))

	// fail pattern regexp
	input["address-1"] = "1, Sesame Street"
	require.Error(t, AgainstSchema(schema, input, strfmt.Default))

	// fail patternProperties regexp
	inputJSON = `{"name": "Ivan","address-1": "sesame street","address-A": "address"}`
	require.NoError(t, json.Unmarshal([]byte(inputJSON), &input))
	require.Error(t, AgainstSchema(schema, input, strfmt.Default))
}

func TestSchemaValidator_Panic(t *testing.T) {
	assert.PanicsWithValue(t, `Invalid schema provided to SchemaValidator: object has no field "pointer-to-nowhere": JSON pointer error`, schemaValidatorPanicker)
}

func schemaValidatorPanicker() {
	schemaJSON := `
{
    "$ref": "#/pointer-to-nowhere"
}`

	schema := new(spec.Schema)
	_ = json.Unmarshal([]byte(schemaJSON), schema)

	var input map[string]any

	// ok
	inputJSON := `{"name": "Ivan","address-1": "sesame street"}`
	_ = json.Unmarshal([]byte(inputJSON), &input)
	// panics
	_ = AgainstSchema(schema, input, strfmt.Default)
}

// Test edge cases in schemaValidator which are difficult
// to simulate with specs.
func TestSchemaValidator_EdgeCases(t *testing.T) {
	var s *SchemaValidator

	res := s.Validate("123")
	assert.NotNil(t, res)
	assert.TrueT(t, res.IsValid())

	s = NewSchemaValidator(nil, nil, "", strfmt.Default)
	assert.Nil(t, s)

	v := "ABC"
	b := s.Applies(v, reflect.String)
	assert.FalseT(t, b)

	sp := spec.Schema{}
	b = s.Applies(&sp, reflect.Struct)
	assert.TrueT(t, b)

	spp := spec.Float64Property()

	s = NewSchemaValidator(spp, nil, "", strfmt.Default)

	s.SetPath("path")
	assert.EqualT(t, "path", s.Path)

	r := s.Validate(nil)
	assert.NotNil(t, r)
	assert.FalseT(t, r.IsValid())

	// Validating json.Number data against number|float64
	j := json.Number("123")
	r = s.Validate(j)
	assert.TrueT(t, r.IsValid())

	// Validating json.Number data against integer|int32
	spp = spec.Int32Property()
	s = NewSchemaValidator(spp, nil, "", strfmt.Default)
	j = json.Number("123")
	r = s.Validate(j)
	assert.TrueT(t, r.IsValid())

	bignum := conv.FormatFloat(math.MaxFloat64)
	j = json.Number(bignum)
	r = s.Validate(j)
	assert.FalseT(t, r.IsValid())

	// Validating incorrect json.Number data
	spp = spec.Float64Property()
	s = NewSchemaValidator(spp, nil, "", strfmt.Default)
	j = json.Number("AXF")
	r = s.Validate(j)
	assert.FalseT(t, r.IsValid())
}

func TestSchemaValidator_SchemaOptions(t *testing.T) {
	schemaJSON := `
{
	"properties": {
		"spec": {
			"properties": {
				"replicas": {
					"type": "integer"
				}
			}
		}
	}
}`

	schema := new(spec.Schema)
	require.NoError(t, json.Unmarshal([]byte(schemaJSON), schema))

	var input map[string]any
	inputJSON := `{"spec": {"items": ["foo", "bar"], "replicas": 1}}`
	require.NoError(t, json.Unmarshal([]byte(inputJSON), &input))

	// ok
	s := NewSchemaValidator(schema, nil, "", strfmt.Default, EnableObjectArrayTypeCheck(false))
	result := s.Validate(input)
	assert.TrueT(t, result.IsValid())

	// fail
	s = NewSchemaValidator(schema, nil, "", strfmt.Default, EnableObjectArrayTypeCheck(true))
	result = s.Validate(input)
	assert.FalseT(t, result.IsValid())
}

func TestSchemaValidator_TypeArray_Issue83(t *testing.T) {
	schemaJSON := `
{
	"type": "object"
}`

	schema := new(spec.Schema)
	require.NoError(t, json.Unmarshal([]byte(schemaJSON), schema))

	var input map[string]any
	inputJSON := `{"type": "array"}`

	require.NoError(t, json.Unmarshal([]byte(inputJSON), &input))
	// default behavior: jsonschema
	require.NoError(t, AgainstSchema(schema, input, strfmt.Default))

	// swagger behavior
	require.Error(t, AgainstSchema(schema, input, strfmt.Default, SwaggerSchema(true)))
}
