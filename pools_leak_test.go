// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag/pools"
	"github.com/go-openapi/testify/v2/require"
)

// TestPools_NoLeakOnSpecValidation asserts that validating a specification
// gives back every object it borrowed.
//
// The assertion only bites when built with the "poolsdebug" tag, which turns
// on borrow tracking; without it the test still runs the validation, and the
// pools panic on nothing. Run the instrumented build with:
//
//	go test -tags poolsdebug ./...
func TestPools_NoLeakOnSpecValidation(t *testing.T) {
	resetPools()
	pools.ResetTracking()
	t.Cleanup(pools.ResetTracking)

	fp := filepath.Join("testdata", "bugs", "2866", "2866.yaml")
	doc, err := loads.Spec(fp)
	require.NoError(t, err)
	require.NotNil(t, doc)

	require.NoError(t, Spec(doc, strfmt.Default))

	require.True(t, pools.AssertNoLeaks(t))
}

// TestPools_NoLeakOnSchemaValidation covers the other entry point that
// recycles: validating data against a schema, rather than a whole spec.
func TestPools_NoLeakOnSchemaValidation(t *testing.T) {
	resetPools()
	pools.ResetTracking()
	t.Cleanup(pools.ResetTracking)

	schema := new(spec.Schema)
	require.NoError(t, json.Unmarshal([]byte(`{
		"type": "object",
		"required": ["name"],
		"properties": {
			"name": {"type": "string"},
			"friends": {"type": "array", "items": {"type": "object", "required": ["age"]}},
			"either": {"oneOf": [{"type": "string"}, {"type": "integer"}]}
		}
	}`), schema))

	const friendsProp = "friends"

	for _, data := range []any{
		map[string]any{nameProp: "ok", friendsProp: []any{map[string]any{"age": 1}}, "either": "s"},
		map[string]any{friendsProp: []any{map[string]any{}}, "either": true},
		nil,
	} {
		_ = AgainstSchema(schema, data, strfmt.Default)
	}

	require.True(t, pools.AssertNoLeaks(t))
}

// TestPools_NoLeakOnNilSchema exercises the path that hands back the shared
// empty result, which the pool must not be given.
func TestPools_NoLeakOnNilSchema(t *testing.T) {
	resetPools()
	pools.ResetTracking()
	t.Cleanup(pools.ResetTracking)

	require.NoError(t, AgainstSchema(nil, map[string]any{}, strfmt.Default))
	require.True(t, pools.AssertNoLeaks(t))
}

// TestPools_NoDoubleRedeemOnPanic builds a validator that panics inside Validate: the $ref of
// property "x" is expanded when the object validator walks the data. The validators that
// redeem themselves on the way out must not be redeemed again by their parent.
//
// Like the other tests here, it only bites with the "poolsdebug" tag: the pools then panic
// on a double redeem, and that panic replaces the one from the schema.
func TestPools_NoDoubleRedeemOnPanic(t *testing.T) {
	resetPools()
	pools.ResetTracking()
	t.Cleanup(resetPools)
	t.Cleanup(pools.ResetTracking)

	var schema spec.Schema
	require.NoError(t, json.Unmarshal([]byte(`{"properties":{"x":{"$ref":"#/definitions/missing"}}}`), &schema))

	require.PanicsWithValue(t,
		`Invalid schema provided to SchemaValidator: nil value has no field "missing": JSON pointer error`,
		func() { _ = AgainstSchema(&schema, map[string]any{"x": 1}, strfmt.Default) },
	)
}
