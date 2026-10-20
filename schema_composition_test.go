// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"
)

func TestCompositionBudget(t *testing.T) {
	// each of these schemas loops through allOf, anyOf, oneOf or not and their $ref:
	// without a budget, building the validator overflows the goroutine stack
	// (fatal, not recoverable) or never ends
	for _, tc := range []struct {
		name   string
		schema string
	}{
		{
			name:   "allOf cycle",
			schema: `{"definitions":{"CycleAllOf":{"allOf":[{"$ref":"#/definitions/CycleAllOf"}]}},"properties":{"x":{"$ref":"#/definitions/CycleAllOf"}}}`,
		},
		{
			name:   "anyOf cycle",
			schema: `{"definitions":{"CycleAnyOf":{"anyOf":[{"type":"string"},{"$ref":"#/definitions/CycleAnyOf"}]}},"properties":{"x":{"$ref":"#/definitions/CycleAnyOf"}}}`,
		},
		{
			name:   "oneOf cycle",
			schema: `{"definitions":{"CycleOneOf":{"oneOf":[{"$ref":"#/definitions/CycleOneOf"}]}},"properties":{"x":{"$ref":"#/definitions/CycleOneOf"}}}`,
		},
		{
			name:   "not cycle",
			schema: `{"definitions":{"CycleNot":{"not":{"$ref":"#/definitions/CycleNot"}}},"properties":{"x":{"$ref":"#/definitions/CycleNot"}}}`,
		},
		{
			name: "mutual cycle",
			schema: `{
				"definitions":{
					"CycleMutualA":{"allOf":[{"$ref":"#/definitions/CycleMutualB"}]},
					"CycleMutualB":{"anyOf":[{"$ref":"#/definitions/CycleMutualA"}]}
				},
				"properties":{"x":{"$ref":"#/definitions/CycleMutualA"}}
			}`,
		},
		{
			// 2 branches per level: 2^depth validators without a budget
			name: "fan-out cycle",
			schema: `{
				"definitions":{"CycleFanOut":{"allOf":[{"$ref":"#/definitions/CycleFanOut"},{"$ref":"#/definitions/CycleFanOut"}]}},
				"properties":{"x":{"$ref":"#/definitions/CycleFanOut"}}
			}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var schema spec.Schema
			require.NoError(t, json.Unmarshal([]byte(tc.schema), &schema))

			defer func() {
				r := recover()
				require.NotNil(t, r)
				msg, ok := r.(string)
				require.TrueT(t, ok)
				assert.True(t, strings.HasPrefix(msg, "Invalid schema provided to SchemaValidator: "))
				assert.True(t, strings.Contains(msg, fmt.Sprintf("deeper than %d levels or into more than %d validators", maxCompositionDepth, maxCompositionValidators)))
			}()

			_ = AgainstSchema(&schema, map[string]any{"x": 1}, strfmt.Default)
		})
	}
}

func TestCompositionBudgetRecursiveSchema(t *testing.T) {
	// a $ref back to an ancestor through properties or items is built as the data is walked:
	// the depth of the data bounds it, and it does not draw from the budget
	const tree = `{
		"definitions":{
			"BudgetNode":{
				"allOf":[
					{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}},
					{"properties":{"children":{"type":"array","items":{"$ref":"#/definitions/BudgetNode"}}}}
				]
			}
		},
		"properties":{"root":{"$ref":"#/definitions/BudgetNode"}}
	}`

	var schema spec.Schema
	require.NoError(t, json.Unmarshal([]byte(tree), &schema))

	// nest the data deeper than both caps: each level only builds the validators of one Node
	const depth = 2 * maxCompositionDepth
	node := map[string]any{nameProp: "leaf"}
	for range depth {
		node = map[string]any{nameProp: "node", "children": []any{node}}
	}

	require.NoError(t, AgainstSchema(&schema, map[string]any{"root": node}, strfmt.Default))

	node["children"] = []any{map[string]any{"children": []any{}}} // missing name
	require.Error(t, AgainstSchema(&schema, map[string]any{"root": node}, strfmt.Default))
}

func TestCompositionBudgetWideSchema(t *testing.T) {
	// many siblings within the budget still build
	const width = maxCompositionValidators
	branches := make([]spec.Schema, width)
	for i := range branches {
		branches[i] = *spec.StringProperty()
	}
	schema := &spec.Schema{SchemaProps: spec.SchemaProps{AllOf: branches}}

	require.NoError(t, AgainstSchema(schema, "a", strfmt.Default))
	require.Error(t, AgainstSchema(schema, 1, strfmt.Default))

	// one more panics
	schema.AllOf = append(schema.AllOf, *spec.StringProperty())
	require.Panics(t, func() { _ = AgainstSchema(schema, "a", strfmt.Default) })
}

func TestCompositionBudgetAmplification(t *testing.T) {
	// no cycle, and within maxCompositionDepth: A0 refers twice to A1, which refers twice to A2,
	// and so on. Expanding it stays within the budget of go-openapi/spec, but it would build
	// 2^levels validators.
	const levels = 13
	definitions := make(map[string]any, levels+1)
	for i := range levels {
		ref := map[string]any{jsonRef: fmt.Sprintf("#/definitions/BudgetA%d", i+1)}
		definitions[fmt.Sprintf("BudgetA%d", i)] = map[string]any{"allOf": []any{ref, ref}}
	}
	definitions[fmt.Sprintf("BudgetA%d", levels)] = map[string]any{jsonType: stringType}

	doc, err := json.Marshal(map[string]any{
		swaggerDefinitions: definitions,
		"properties":       map[string]any{"x": map[string]any{jsonRef: "#/definitions/BudgetA0"}},
	})
	require.NoError(t, err)

	var schema spec.Schema
	require.NoError(t, json.Unmarshal(doc, &schema))

	require.PanicsWithValue(t,
		invalidSchemaProvidedMsg(tooManyCompositionValidatorsMsg("x", maxCompositionDepth, maxCompositionValidators)).Error(),
		func() { _ = AgainstSchema(&schema, map[string]any{"x": "a"}, strfmt.Default) },
	)
}

func TestCompositionBudgetDeepSchema(t *testing.T) {
	// nesting allOf within the depth cap still builds
	schema := spec.StringProperty()
	for range maxCompositionDepth {
		schema = &spec.Schema{SchemaProps: spec.SchemaProps{AllOf: []spec.Schema{*schema}}}
	}

	require.NoError(t, AgainstSchema(schema, "a", strfmt.Default))
	require.Error(t, AgainstSchema(schema, 1, strfmt.Default))

	// one level more panics
	schema = &spec.Schema{SchemaProps: spec.SchemaProps{AllOf: []spec.Schema{*schema}}}
	require.Panics(t, func() { _ = AgainstSchema(schema, "a", strfmt.Default) })
}

func TestCompositionBudgetSpec(t *testing.T) {
	// Spec stops on the circular ancestry of CycleSpec before it checks defaults and examples.
	// ContinueOnErrors goes on: building a validator for the default or the example panics,
	// and the spec validator returns it as an error (default) or a warning (example).
	const doc = `{
		"swagger":"2.0",
		"info":{"title":"t","version":"1"},
		"paths":{},
		"definitions":{
			"CycleSpec":{"default":1,"example":1,"allOf":[{"$ref":"#/definitions/CycleSpec"}]}
		}
	}`

	analyzed, err := loads.Analyzed(json.RawMessage(doc), "")
	require.NoError(t, err)

	validator := NewSpecValidator(analyzed.Schema(), strfmt.Default)
	validator.SetContinueOnErrors(true)

	var errs, warnings *Result
	require.NotPanics(t, func() { errs, warnings = validator.Validate(analyzed) })

	const tooDeep = "nests allOf, anyOf, oneOf and not deeper than"
	assert.True(t, hasMessageContaining(errs.Errors, "has circular ancestry"))
	assert.True(t, hasMessageContaining(errs.Errors, tooDeep))
	assert.True(t, hasMessageContaining(warnings.Errors, tooDeep))
}

func hasMessageContaining(errs []error, substr string) bool {
	for _, err := range errs {
		if strings.Contains(err.Error(), substr) {
			return true
		}
	}

	return false
}
