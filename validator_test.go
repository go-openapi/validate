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
	"math"
	"reflect"
	"testing"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeaderValidator(t *testing.T) {
	t.Run("with no recycling", func(t *testing.T) {
		v := NewHeaderValidator("header", &spec.Header{}, strfmt.Default, SwaggerSchema(true))

		res := v.Validate(nil)
		require.Nil(t, res)
	})

	t.Run("with recycling", func(t *testing.T) {
		v := NewHeaderValidator("header", &spec.Header{}, strfmt.Default,
			SwaggerSchema(true), WithRecycleValidators(true), withRecycleResults(true),
		)

		t.Run("should validate nil data", func(t *testing.T) {
			res := v.Validate(nil)
			require.Nil(t, res)
		})

		t.Run("should validate only once", func(t *testing.T) {
			// we should not do that: the pool chain list is populated with a duplicate: needs a reset
			t.Cleanup(resetPools)
			require.Panics(t, func() {
				_ = v.Validate("header")
			})
		})
		t.Run("should validate non nil data", func(t *testing.T) {
			nv := NewHeaderValidator("header", &spec.Header{SimpleSchema: spec.SimpleSchema{Type: "string"}}, strfmt.Default,
				SwaggerSchema(true), WithRecycleValidators(true), withRecycleResults(true),
			)

			res := nv.Validate("X-GO")
			require.NotNil(t, res)
			require.Empty(t, res.Errors)
			require.True(t, res.wantsRedeemOnMerge)
			pools.poolOfResults.RedeemResult(res)
		})
	})
}

func TestParamValidator(t *testing.T) {
	v := NewParamValidator(&spec.Parameter{}, strfmt.Default, SwaggerSchema(true))

	res := v.Validate(nil)
	require.Nil(t, res)
}

func TestNumberValidator_EdgeCases(t *testing.T) {
	// Apply
	var min = float64(math.MinInt32 - 1)
	var max = float64(math.MaxInt32 + 1)

	v := newNumberValidator(
		"path",
		"in",
		nil,
		nil,
		&max, // *float64
		false,
		&min, // *float64
		false,
		// Allows for more accurate behavior regarding integers
		"integer",
		"int32",
		nil,
	)

	// numberValidator applies to: Parameter,Schema,Items,Header

	sources := []interface{}{
		new(spec.Parameter),
		new(spec.Schema),
		new(spec.Items),
		new(spec.Header),
	}

	testNumberApply(t, v, sources)

	assert.False(t, v.Applies(float64(32), reflect.Float64))

	// Now for different scenarios on Minimum, Maximum
	// - The Maximum value does not respect the Type|Format specification
	// - Value is checked as float64 with Maximum as float64 and fails
	res := v.Validate(int64(math.MaxInt32 + 2))
	assert.True(t, res.HasErrors())
	// - The Minimum value does not respect the Type|Format specification
	// - Value is checked as float64 with Maximum as float64 and fails
	res = v.Validate(int64(math.MinInt32 - 2))
	assert.True(t, res.HasErrors())
}

func testNumberApply(t *testing.T, v *numberValidator, sources []interface{}) {
	for _, source := range sources {
		// numberValidator does not applies to:
		assert.False(t, v.Applies(source, reflect.String))
		assert.False(t, v.Applies(source, reflect.Struct))
		// numberValidator applies to:
		assert.True(t, v.Applies(source, reflect.Int))
		assert.True(t, v.Applies(source, reflect.Int8))
		assert.True(t, v.Applies(source, reflect.Uint16))
		assert.True(t, v.Applies(source, reflect.Uint32))
		assert.True(t, v.Applies(source, reflect.Uint64))
		assert.True(t, v.Applies(source, reflect.Uint))
		assert.True(t, v.Applies(source, reflect.Uint8))
		assert.True(t, v.Applies(source, reflect.Uint16))
		assert.True(t, v.Applies(source, reflect.Uint32))
		assert.True(t, v.Applies(source, reflect.Uint64))
		assert.True(t, v.Applies(source, reflect.Float32))
		assert.True(t, v.Applies(source, reflect.Float64))
	}
}

func TestStringValidator_EdgeCases(t *testing.T) {
	// Apply

	v := newStringValidator(
		"", "", nil, false, false, nil, nil, "", nil,
	)

	// stringValidator applies to: Parameter,Schema,Items,Header

	sources := []interface{}{
		new(spec.Parameter),
		new(spec.Schema),
		new(spec.Items),
		new(spec.Header),
	}

	testStringApply(t, v, sources)

	assert.False(t, v.Applies("A string", reflect.String))
}

func testStringApply(t *testing.T, v *stringValidator, sources []interface{}) {
	for _, source := range sources {
		// numberValidator does not applies to:
		assert.False(t, v.Applies(source, reflect.Struct))
		assert.False(t, v.Applies(source, reflect.Int))
		// numberValidator applies to:
		assert.True(t, v.Applies(source, reflect.String))
	}
}

func TestBasicCommonValidator_EdgeCases(t *testing.T) {
	// Apply

	v := newBasicCommonValidator(
		"", "",
		nil, []interface{}{"a", nil, 3}, nil,
	)

	// basicCommonValidator applies to: Parameter,Schema,Header

	sources := []interface{}{
		new(spec.Parameter),
		new(spec.Schema),
		new(spec.Header),
	}

	testCommonApply(t, v, sources)

	assert.False(t, v.Applies("A string", reflect.String))

	t.Run("should validate Enum", func(t *testing.T) {
		res := v.Validate("a")
		require.Nil(t, res)

		res = v.Validate(3)
		require.Nil(t, res)

		res = v.Validate("b")
		require.NotNil(t, res)
		assert.True(t, res.HasErrors())
	})

	t.Run("shoud validate empty Enum", func(t *testing.T) {
		ev := newBasicCommonValidator(
			"", "",
			nil, nil, nil,
		)
		res := ev.Validate("a")
		require.Nil(t, res)

		res = ev.Validate(3)
		require.Nil(t, res)

		res = ev.Validate("b")
		require.Nil(t, res)
	})
}

func testCommonApply(t *testing.T, v *basicCommonValidator, sources []interface{}) {
	for _, source := range sources {
		assert.True(t, v.Applies(source, reflect.String))
	}
}

func TestBasicSliceValidator_EdgeCases(t *testing.T) {
	t.Run("should Apply", func(t *testing.T) {
		v := newBasicSliceValidator(
			"", "",
			nil, nil, nil, false, nil, nil, strfmt.Default, nil,
		)

		// basicCommonValidator applies to: Parameter,Schema,Header

		sources := []interface{}{
			new(spec.Parameter),
			new(spec.Items),
			new(spec.Header),
		}

		testSliceApply(t, v, sources)

		assert.False(t, v.Applies(new(spec.Schema), reflect.Slice))
		assert.False(t, v.Applies(new(spec.Parameter), reflect.String))
	})

	t.Run("with recycling", func(t *testing.T) {
		v := newBasicSliceValidator(
			"", "",
			nil, nil, nil, false, nil, nil, strfmt.Default,
			&SchemaValidatorOptions{recycleValidators: true},
		)

		res := v.Validate([]int{})
		require.Nil(t, res)
	})
}

func testSliceApply(t *testing.T, v *basicSliceValidator, sources []interface{}) {
	for _, source := range sources {
		assert.True(t, v.Applies(source, reflect.Slice))
	}
}

/* unused
type anything struct {
	anyProperty int
}

// hasDuplicates() is currently not exercised by common spec testcases
// (this method is not used by the validator atm)
// Here is a unit exerciser
// NOTE: this method is probably obsolete and superseeded by values.go:UniqueItems()
// which is superior in every respect to this one.
func TestBasicSliceValidator_HasDuplicates(t *testing.T) {
	s := basicSliceValidator{}
	// hasDuplicates() makes no hypothesis about the underlying object,
	// save being an array, slice or string (same constraint as reflect.Value.Index())
	// it also comes without safeguard or anything.
	vi := []int{1, 2, 3}
	vs := []string{"a", "b", "c"}
	vt := []anything{
		{anyProperty: 1},
		{anyProperty: 2},
		{anyProperty: 3},
	}
	assert.False(t, s.hasDuplicates(reflect.ValueOf(vi), len(vi)))
	assert.False(t, s.hasDuplicates(reflect.ValueOf(vs), len(vs)))
	assert.False(t, s.hasDuplicates(reflect.ValueOf(vt), len(vt)))

	di := []int{1, 1, 3}
	ds := []string{"a", "b", "a"}
	dt := []anything{
		{anyProperty: 1},
		{anyProperty: 2},
		{anyProperty: 2},
	}
	assert.True(t, s.hasDuplicates(reflect.ValueOf(di), len(di)))
	assert.True(t, s.hasDuplicates(reflect.ValueOf(ds), len(ds)))
	assert.True(t, s.hasDuplicates(reflect.ValueOf(dt), len(dt)))
}
*/
