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
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	// PetStore20 json doc for swagger 2.0 pet store
	PetStore20 string

	// PetStoreJSONMessage json raw message for Petstore20
	PetStoreJSONMessage json.RawMessage
)

func init() {
	petstoreFixture := filepath.Join("fixtures", "petstore", "swagger.json")
	petstore, err := os.ReadFile(petstoreFixture)
	if err != nil {
		log.Fatalf("could not initialize fixture: %s: %v", petstoreFixture, err)
	}
	PetStoreJSONMessage = json.RawMessage(petstore)
	PetStore20 = string(petstore)
}

func stringItems() *spec.Items {
	return spec.NewItems().Typed(stringType, "")
}

func requiredError(param *spec.Parameter, data interface{}) *errors.Validation {
	return errors.Required(param.Name, param.In, data)
}

func maxErrorItems(path, in string, items *spec.Items, data interface{}) *errors.Validation {
	return errors.ExceedsMaximum(path, in, *items.Maximum, items.ExclusiveMaximum, data)
}

func minErrorItems(path, in string, items *spec.Items, data interface{}) *errors.Validation {
	return errors.ExceedsMinimum(path, in, *items.Minimum, items.ExclusiveMinimum, data)
}

func multipleOfErrorItems(path, in string, items *spec.Items, data interface{}) *errors.Validation {
	return errors.NotMultipleOf(path, in, *items.MultipleOf, data)
}

/*
func requiredErrorItems(path, in string) *errors.Validation {
	return errors.Required(path, in)
}
*/

func maxLengthErrorItems(path, in string, items *spec.Items, data interface{}) *errors.Validation {
	return errors.TooLong(path, in, *items.MaxLength, data)
}

func minLengthErrorItems(path, in string, items *spec.Items, data interface{}) *errors.Validation {
	return errors.TooShort(path, in, *items.MinLength, data)
}

func patternFailItems(path, in string, items *spec.Items, data interface{}) *errors.Validation {
	return errors.FailedPattern(path, in, items.Pattern, data)
}

func enumFailItems(path, in string, items *spec.Items, data interface{}) *errors.Validation {
	return errors.EnumFail(path, in, data, items.Enum)
}

func minItemsErrorItems(path, in string, items *spec.Items, data interface{}) *errors.Validation {
	return errors.TooFewItems(path, in, *items.MinItems, data)
}

func maxItemsErrorItems(path, in string, items *spec.Items, data interface{}) *errors.Validation {
	return errors.TooManyItems(path, in, *items.MaxItems, data)
}

func duplicatesErrorItems(path, in string) *errors.Validation {
	return errors.DuplicateItems(path, in)
}

func TestNumberItemsValidation(t *testing.T) {

	values := [][]interface{}{
		{23, 49, 56, 21, 14, 35, 28, 7, 42},
		{uint(23), uint(49), uint(56), uint(21), uint(14), uint(35), uint(28), uint(7), uint(42)},
		{float64(23), float64(49), float64(56), float64(21), float64(14), float64(35), float64(28), float64(7), float64(42)},
	}

	for i, v := range values {
		items := spec.NewItems()
		items.WithMaximum(makeFloat(v[1]), false)
		items.WithMinimum(makeFloat(v[3]), false)
		items.WithMultipleOf(makeFloat(v[7]))
		items.WithEnum(v[3], v[6], v[8], v[1])
		items.Typed("integer", "int32")
		parent := spec.QueryParam("factors").CollectionOf(items, "")
		path := fmt.Sprintf("factors.%d", i)
		validator := newItemsValidator(parent.Name, parent.In, items, parent, strfmt.Default, nil)

		// MultipleOf
		err := validator.Validate(i, v[0])
		assert.True(t, err.HasErrors())
		require.NotEmpty(t, err.Errors)
		require.EqualError(t, multipleOfErrorItems(path, validator.in, items, v[0]), err.Errors[0].Error())

		// Maximum
		err = validator.Validate(i, v[1])
		assert.True(t, err == nil || err.IsValid())
		err = validator.Validate(i, v[2])
		assert.True(t, err.HasErrors())
		require.NotEmpty(t, err.Errors)
		require.EqualError(t, maxErrorItems(path, validator.in, items, v[2]), err.Errors[0].Error())

		// ExclusiveMaximum
		items.ExclusiveMaximum = true
		// requires a new items validator because this is set a creation time
		validator = newItemsValidator(parent.Name, parent.In, items, parent, strfmt.Default, nil)
		err = validator.Validate(i, v[1])
		assert.True(t, err.HasErrors())
		require.NotEmpty(t, err.Errors)
		require.EqualError(t, maxErrorItems(path, validator.in, items, v[1]), err.Errors[0].Error())

		// Minimum
		err = validator.Validate(i, v[3])
		assert.True(t, err == nil || err.IsValid())
		err = validator.Validate(i, v[4])
		assert.True(t, err.HasErrors())
		require.NotEmpty(t, err.Errors)
		require.EqualError(t, minErrorItems(path, validator.in, items, v[4]), err.Errors[0].Error())

		// ExclusiveMinimum
		items.ExclusiveMinimum = true
		// requires a new items validator because this is set a creation time
		validator = newItemsValidator(parent.Name, parent.In, items, parent, strfmt.Default, nil)
		err = validator.Validate(i, v[3])
		assert.True(t, err.HasErrors())
		require.NotEmpty(t, err.Errors)
		require.EqualError(t, minErrorItems(path, validator.in, items, v[3]), err.Errors[0].Error())

		// Enum
		err = validator.Validate(i, v[5])
		assert.True(t, err.HasErrors())
		require.NotEmpty(t, err.Errors)
		require.EqualError(t, enumFailItems(path, validator.in, items, v[5]), err.Errors[0].Error())

		// Valid passes
		err = validator.Validate(i, v[6])
		assert.True(t, err == nil || err.IsValid())
	}

}

func TestStringItemsValidation(t *testing.T) {
	items := spec.NewItems().WithMinLength(3).WithMaxLength(5).WithPattern(`^[a-z]+$`).Typed(stringType, "")
	items.WithEnum("aaa", "bbb", "ccc")
	parent := spec.QueryParam("tags").CollectionOf(items, "")
	path := parent.Name + ".1"
	validator := newItemsValidator(parent.Name, parent.In, items, parent, strfmt.Default, nil)

	// required
	data := ""
	err := validator.Validate(1, data)
	assert.True(t, err.HasErrors())
	require.NotEmpty(t, err.Errors)
	require.EqualError(t, minLengthErrorItems(path, validator.in, items, data), err.Errors[0].Error())

	// MaxLength
	data = "abcdef"
	err = validator.Validate(1, data)
	assert.True(t, err.HasErrors())
	require.NotEmpty(t, err.Errors)
	require.EqualError(t, maxLengthErrorItems(path, validator.in, items, data), err.Errors[0].Error())

	// MinLength
	data = "a"
	err = validator.Validate(1, data)
	assert.True(t, err.HasErrors())
	require.NotEmpty(t, err.Errors)
	require.EqualError(t, minLengthErrorItems(path, validator.in, items, data), err.Errors[0].Error())

	// Pattern
	data = "a394"
	err = validator.Validate(1, data)
	assert.True(t, err.HasErrors())
	require.NotEmpty(t, err.Errors)
	require.EqualError(t, patternFailItems(path, validator.in, items, data), err.Errors[0].Error())

	// Enum
	data = "abcde"
	err = validator.Validate(1, data)
	assert.True(t, err.HasErrors())
	require.NotEmpty(t, err.Errors)
	require.EqualError(t, enumFailItems(path, validator.in, items, data), err.Errors[0].Error())

	// Valid passes
	err = validator.Validate(1, "bbb")
	assert.True(t, err == nil || err.IsValid())
}

func TestArrayItemsValidation(t *testing.T) {
	items := spec.NewItems().CollectionOf(stringItems(), "").WithMinItems(1).WithMaxItems(5).UniqueValues()
	items.WithEnum("aaa", "bbb", "ccc")
	parent := spec.QueryParam("tags").CollectionOf(items, "")
	path := parent.Name + ".1"
	validator := newItemsValidator(parent.Name, parent.In, items, parent, strfmt.Default, nil)

	// MinItems
	data := []string{}
	err := validator.Validate(1, data)
	assert.True(t, err.HasErrors())
	require.NotEmpty(t, err.Errors)
	require.EqualError(t, minItemsErrorItems(path, validator.in, items, len(data)), err.Errors[0].Error())
	// MaxItems
	data = []string{"a", "b", "c", "d", "e", "f"}
	err = validator.Validate(1, data)
	assert.True(t, err.HasErrors())
	require.NotEmpty(t, err.Errors)
	require.EqualError(t, maxItemsErrorItems(path, validator.in, items, len(data)), err.Errors[0].Error())
	// UniqueItems
	err = validator.Validate(1, []string{"a", "a"})
	assert.True(t, err.HasErrors())
	require.NotEmpty(t, err.Errors)
	require.EqualError(t, duplicatesErrorItems(path, validator.in), err.Errors[0].Error())

	// Enum
	data = []string{"a", "b", "c"}
	err = validator.Validate(1, data)
	assert.True(t, err.HasErrors())
	require.NotEmpty(t, err.Errors)
	require.EqualError(t, enumFailItems(path, validator.in, items, data), err.Errors[0].Error())

	// Items
	strItems := spec.NewItems().WithMinLength(3).WithMaxLength(5).WithPattern(`^[a-z]+$`).Typed(stringType, "")
	items = spec.NewItems().CollectionOf(strItems, "").WithMinItems(1).WithMaxItems(5).UniqueValues()
	validator = newItemsValidator(parent.Name, parent.In, items, parent, strfmt.Default, nil)

	data = []string{"aa", "bbb", "ccc"}
	err = validator.Validate(1, data)
	assert.True(t, err.HasErrors())
	require.NotEmpty(t, err.Errors)
	require.EqualError(t, minLengthErrorItems(path+".0", parent.In, strItems, data[0]), err.Errors[0].Error())
}
