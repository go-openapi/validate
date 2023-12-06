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
	"context"
	"math"
	"testing"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValues_ValidateIntEnum(t *testing.T) {
	enumValues := []interface{}{1, 2, 3}

	require.Error(t, Enum("test", "body", int64(5), enumValues))
	require.Nil(t, Enum("test", "body", int64(1), enumValues))
}

func TestValues_ValidateEnum(t *testing.T) {
	enumValues := []string{"aa", "bb", "cc"}

	require.Error(t, Enum("test", "body", "a", enumValues))
	require.Nil(t, Enum("test", "body", "bb", enumValues))

	type CustomString string

	require.Error(t, Enum("test", "body", CustomString("a"), enumValues))
	require.Nil(t, Enum("test", "body", CustomString("bb"), enumValues))
}

func TestValues_ValidateNilEnum(t *testing.T) {
	enumValues := []string{"aa", "bb", "cc"}

	require.Error(t, Enum("test", "body", nil, enumValues))
}

// Check edge cases in Enum
func TestValues_Enum_EdgeCases(t *testing.T) {
	enumValues := "aa, bb, cc"

	// No validation occurs: enumValues is not a slice
	require.Nil(t, Enum("test", "body", int64(1), enumValues))

	// TODO(TEST): edge case: value is not a concrete type
	// It's really a go internals challenge
	// to figure a test case to demonstrate
	// this case must be checked (!!)
}

func TestValues_ValidateEnumCaseInsensitive(t *testing.T) {
	enumValues := []string{"aa", "bb", "cc"}

	require.Error(t, EnumCase("test", "body", "a", enumValues, true))
	require.Nil(t, EnumCase("test", "body", "bb", enumValues, true))
	require.Error(t, EnumCase("test", "body", "BB", enumValues, true))
	require.Error(t, EnumCase("test", "body", "a", enumValues, false))
	require.Nil(t, EnumCase("test", "body", "bb", enumValues, false))
	require.Nil(t, EnumCase("test", "body", "BB", enumValues, false))
	require.Error(t, EnumCase("test", "body", int64(1), enumValues, false))
}

func TestValues_ValidateUniqueItems(t *testing.T) {
	itemsNonUnique := []interface{}{
		[]int32{1, 2, 3, 4, 4, 5},
		[]string{"aa", "bb", "cc", "cc", "dd"},
	}
	for _, v := range itemsNonUnique {
		require.Error(t, UniqueItems("test", "body", v))
	}

	itemsUnique := []interface{}{
		[]int32{1, 2, 3},
		"I'm a string",
		map[string]int{
			"aaa": 1111,
			"b":   2,
			"ccc": 333,
		},
		nil,
	}
	for _, v := range itemsUnique {
		require.Nil(t, UniqueItems("test", "body", v))
	}
}

func TestValues_ValidateMinLength(t *testing.T) {
	const minLength = int64(5)
	require.Error(t, MinLength("test", "body", "aa", minLength))
	require.Nil(t, MinLength("test", "body", "aaaaa", minLength))
}

func TestValues_ValidateMaxLength(t *testing.T) {
	const maxLength = int64(5)
	require.Error(t, MaxLength("test", "body", "bbbbbb", maxLength))
	require.Nil(t, MaxLength("test", "body", "aa", maxLength))
}

func TestValues_ReadOnly(t *testing.T) {
	const (
		path = "test"
		in   = "body"
	)

	ReadOnlySuccess := []interface{}{
		"",
		0,
		nil,
	}

	// fail only when operation type is request
	ReadOnlyFail := []interface{}{
		" ",
		"bla-bla-bla",
		2,
		[]interface{}{21, []int{}, "testString"},
	}

	t.Run("No operation context", func(t *testing.T) {
		// readonly should not have any effect
		ctx := context.Background()
		for _, v := range ReadOnlySuccess {
			require.Nil(t, ReadOnly(ctx, path, in, v))
		}
		for _, v := range ReadOnlyFail {
			require.Nil(t, ReadOnly(ctx, path, in, v))
		}

	})
	t.Run("operationType request", func(t *testing.T) {
		ctx := WithOperationRequest(context.Background())
		for _, v := range ReadOnlySuccess {
			require.Nil(t, ReadOnly(ctx, path, in, v))
		}
		for _, v := range ReadOnlyFail {
			require.Error(t, ReadOnly(ctx, path, in, v))
		}
	})
	t.Run("operationType response", func(t *testing.T) {
		ctx := WithOperationResponse(context.Background())
		for _, v := range ReadOnlySuccess {
			require.Nil(t, ReadOnly(ctx, path, in, v))
		}
		for _, v := range ReadOnlyFail {
			require.Nil(t, ReadOnly(ctx, path, in, v))
		}
	})
}

func TestValues_ValidateRequired(t *testing.T) {
	const (
		path = "test"
		in   = "body"
	)

	RequiredFail := []interface{}{
		"",
		0,
		nil,
	}

	for _, v := range RequiredFail {
		require.Error(t, Required(path, in, v))
	}

	RequiredSuccess := []interface{}{
		" ",
		"bla-bla-bla",
		2,
		[]interface{}{21, []int{}, "testString"},
	}

	for _, v := range RequiredSuccess {
		require.Nil(t, Required(path, in, v))
	}

}

func TestValues_ValidateRequiredNumber(t *testing.T) {
	require.Error(t, RequiredNumber("test", "body", 0))
	require.Nil(t, RequiredNumber("test", "body", 1))
}

func TestValuMultipleOf(t *testing.T) {
	// positive
	require.Nil(t, MultipleOf("test", "body", 9, 3))
	require.Nil(t, MultipleOf("test", "body", 9.3, 3.1))
	require.Nil(t, MultipleOf("test", "body", 9.1, 0.1))
	require.Nil(t, MultipleOf("test", "body", 3, 0.3))
	require.Nil(t, MultipleOf("test", "body", 6, 0.3))
	require.Nil(t, MultipleOf("test", "body", 1, 0.25))
	require.Nil(t, MultipleOf("test", "body", 8, 0.2))

	// zero
	require.Error(t, MultipleOf("test", "body", 9, 0))
	require.Error(t, MultipleOf("test", "body", 9.1, 0))

	// negative
	require.Error(t, MultipleOf("test", "body", 3, 0.4))
	require.Error(t, MultipleOf("test", "body", 9.1, 0.2))
	require.Error(t, MultipleOf("test", "body", 9.34, 0.1))

	// error on negative factor
	require.Error(t, MultipleOf("test", "body", 9.34, -0.1))
}

// Test edge case for Pattern (in regular spec, no invalid regexp should reach there)
func TestValues_Pattern_Edgecases(t *testing.T) {
	require.Nil(t, Pattern("path", "in", "pick-a-boo", `.*-[a-z]-.*`))

	t.Run("with invalid regexp", func(t *testing.T) {
		err := Pattern("path", "in", "pick-a-boo", `.*-[a(-z]-^).*`)
		require.Error(t, err)
		assert.Equal(t, int(err.Code()), int(errors.PatternFailCode))
		assert.Contains(t, err.Error(), "pattern is invalid")
	})

	t.Run("with valid regexp, invalid pattern", func(t *testing.T) {
		err := Pattern("path", "in", "pick-8-boo", `.*-[a-z]-.*`)
		require.Error(t, err)
		assert.Equal(t, int(err.Code()), int(errors.PatternFailCode))
		assert.NotContains(t, err.Error(), "pattern is invalid")
		assert.Contains(t, err.Error(), "should match")
	})
}

// Test edge cases in FormatOf
// not easily tested with full specs
func TestValues_FormatOf_EdgeCases(t *testing.T) {
	var err *errors.Validation

	err = FormatOf("path", "in", "bugz", "", nil)
	require.Error(t, err)
	assert.Equal(t, int(err.Code()), int(errors.InvalidTypeCode))
	assert.Contains(t, err.Error(), "bugz is an invalid type name")

	err = FormatOf("path", "in", "bugz", "", strfmt.Default)
	require.Error(t, err)
	assert.Equal(t, int(err.Code()), int(errors.InvalidTypeCode))
	assert.Contains(t, err.Error(), "bugz is an invalid type name")
}

// Test edge cases in MaximumNativeType
// not easily exercised with full specs
func TestValues_MaximumNative(t *testing.T) {
	require.Nil(t, MaximumNativeType("path", "in", int(5), 10, false))
	require.Nil(t, MaximumNativeType("path", "in", uint(5), 10, true))
	require.Nil(t, MaximumNativeType("path", "in", int8(5), 10, true))
	require.Nil(t, MaximumNativeType("path", "in", uint8(5), 10, true))
	require.Nil(t, MaximumNativeType("path", "in", int16(5), 10, true))
	require.Nil(t, MaximumNativeType("path", "in", uint16(5), 10, true))
	require.Nil(t, MaximumNativeType("path", "in", int32(5), 10, true))
	require.Nil(t, MaximumNativeType("path", "in", uint32(5), 10, true))
	require.Nil(t, MaximumNativeType("path", "in", int64(5), 10, true))
	require.Nil(t, MaximumNativeType("path", "in", uint64(5), 10, true))
	require.Nil(t, MaximumNativeType("path", "in", float32(5.5), 10, true))
	require.Nil(t, MaximumNativeType("path", "in", float64(5.5), 10, true))

	var err *errors.Validation

	err = MaximumNativeType("path", "in", int32(10), 10, true)
	require.Error(t, err)
	code := int(err.Code())
	assert.Equal(t, errors.MaxFailCode, code)

	err = MaximumNativeType("path", "in", uint(10), 10, true)
	require.Error(t, err)
	code = int(err.Code())
	assert.Equal(t, errors.MaxFailCode, code)

	err = MaximumNativeType("path", "in", int64(12), 10, false)
	require.Error(t, err)
	code = int(err.Code())
	assert.Equal(t, errors.MaxFailCode, code)

	err = MaximumNativeType("path", "in", float32(12.6), 10, false)
	require.Error(t, err)
	code = int(err.Code())
	assert.Equal(t, int(errors.MaxFailCode), code)

	err = MaximumNativeType("path", "in", float64(12.6), 10, false)
	require.Error(t, err)
	code = int(err.Code())
	assert.Equal(t, int(errors.MaxFailCode), code)

	err = MaximumNativeType("path", "in", uint(5), -10, true)
	require.Error(t, err)
	code = int(err.Code())
	assert.Equal(t, int(errors.MaxFailCode), code)
}

// Test edge cases in MinimumNativeType
// not easily exercised with full specs
func TestValues_MinimumNative(t *testing.T) {
	require.Nil(t, MinimumNativeType("path", "in", int(5), 0, false))
	require.Nil(t, MinimumNativeType("path", "in", uint(5), 0, true))
	require.Nil(t, MinimumNativeType("path", "in", int8(5), 0, true))
	require.Nil(t, MinimumNativeType("path", "in", uint8(5), 0, true))
	require.Nil(t, MinimumNativeType("path", "in", int16(5), 0, true))
	require.Nil(t, MinimumNativeType("path", "in", uint16(5), 0, true))
	require.Nil(t, MinimumNativeType("path", "in", int32(5), 0, true))
	require.Nil(t, MinimumNativeType("path", "in", uint32(5), 0, true))
	require.Nil(t, MinimumNativeType("path", "in", int64(5), 0, true))
	require.Nil(t, MinimumNativeType("path", "in", uint64(5), 0, true))
	require.Nil(t, MinimumNativeType("path", "in", float32(5.5), 0, true))
	require.Nil(t, MinimumNativeType("path", "in", float64(5.5), 0, true))

	var err *errors.Validation

	err = MinimumNativeType("path", "in", uint(10), 10, true)
	require.Error(t, err)
	code := int(err.Code())
	assert.Equal(t, int(errors.MinFailCode), code)

	err = MinimumNativeType("path", "in", uint(10), 10, true)
	require.Error(t, err)
	code = int(err.Code())
	assert.Equal(t, int(errors.MinFailCode), code)

	err = MinimumNativeType("path", "in", int64(8), 10, false)
	require.Error(t, err)
	code = int(err.Code())
	assert.Equal(t, int(errors.MinFailCode), code)

	err = MinimumNativeType("path", "in", float32(12.6), 20, false)
	require.Error(t, err)
	code = int(err.Code())
	assert.Equal(t, int(errors.MinFailCode), code)

	err = MinimumNativeType("path", "in", float64(12.6), 20, false)
	require.Error(t, err)
	code = int(err.Code())
	assert.Equal(t, int(errors.MinFailCode), code)

	require.Nil(t, MinimumNativeType("path", "in", uint(5), -10, true))
}

// Test edge cases in MaximumNativeType
// not easily exercised with full specs
func TestValues_MultipleOfNative(t *testing.T) {
	require.Nil(t, MultipleOfNativeType("path", "in", int(5), 1))
	require.Nil(t, MultipleOfNativeType("path", "in", uint(5), 1))
	require.Nil(t, MultipleOfNativeType("path", "in", int8(5), 1))
	require.Nil(t, MultipleOfNativeType("path", "in", uint8(5), 1))
	require.Nil(t, MultipleOfNativeType("path", "in", int16(5), 1))
	require.Nil(t, MultipleOfNativeType("path", "in", uint16(5), 1))
	require.Nil(t, MultipleOfNativeType("path", "in", int32(5), 1))
	require.Nil(t, MultipleOfNativeType("path", "in", uint32(5), 1))
	require.Nil(t, MultipleOfNativeType("path", "in", int64(5), 1))
	require.Nil(t, MultipleOfNativeType("path", "in", uint64(5), 1))

	var err *errors.Validation

	err = MultipleOfNativeType("path", "in", int64(5), 0)
	require.Error(t, err)
	code := int(err.Code())
	assert.Equal(t, int(errors.MultipleOfMustBePositiveCode), code)

	err = MultipleOfNativeType("path", "in", uint64(5), 0)
	require.Error(t, err)
	code = int(err.Code())
	assert.Equal(t, int(errors.MultipleOfMustBePositiveCode), code)

	err = MultipleOfNativeType("path", "in", int64(5), -1)
	require.Error(t, err)
	code = int(err.Code())
	assert.Equal(t, int(errors.MultipleOfMustBePositiveCode), code)

	err = MultipleOfNativeType("path", "in", int64(11), 5)
	require.Error(t, err)
	code = int(err.Code())
	assert.Equal(t, int(errors.MultipleOfFailCode), code)

	err = MultipleOfNativeType("path", "in", uint64(11), 5)
	require.Error(t, err)
	code = int(err.Code())
	assert.Equal(t, int(errors.MultipleOfFailCode), code)
}

// Test edge cases in IsValueValidAgainstRange
// not easily exercised with full specs: we did not simulate these formats in full specs
func TestValues_IsValueValidAgainstRange(t *testing.T) {
	require.NoError(t, IsValueValidAgainstRange(float32(123.45), "number", "float32", "prefix", "path"))
	require.NoError(t, IsValueValidAgainstRange(float64(123.45), "number", "float32", "prefix", "path"))
	require.NoError(t, IsValueValidAgainstRange(int64(123), "number", "float", "prefix", "path"))
	require.NoError(t, IsValueValidAgainstRange(int64(123), "integer", "", "prefix", "path"))
	require.NoError(t, IsValueValidAgainstRange(int64(123), "integer", "int64", "prefix", "path"))
	require.NoError(t, IsValueValidAgainstRange(int64(123), "integer", "uint64", "prefix", "path"))
	require.NoError(t, IsValueValidAgainstRange(int64(2147483647), "integer", "int32", "prefix", "path"))
	require.NoError(t, IsValueValidAgainstRange(int64(2147483647), "integer", "uint32", "prefix", "path"))

	var err error
	// Error case (do not occur in normal course of a validation)
	err = IsValueValidAgainstRange(float64(math.MaxFloat64), "integer", "", "prefix", "path")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be of type integer (default format)")

	// Checking a few limits
	err = IsValueValidAgainstRange("123", "number", "", "prefix", "path")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "called with invalid (non numeric) val type")
}
