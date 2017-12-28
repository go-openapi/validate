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
	"math"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
)

// Test edge cases in slice_validator which are difficult
// to simulate with specs
func TestSchemaValidator_EdgeCases(t *testing.T) {
	var s *SchemaValidator

	res := s.Validate("123")
	assert.NotNil(t, res)
	assert.True(t, res.IsValid())

	s = NewSchemaValidator(nil, nil, "", strfmt.Default)
	assert.Nil(t, s)

	v := "ABC"
	b := s.Applies(v, reflect.String)
	assert.False(t, b)

	sp := spec.Schema{}
	b = s.Applies(&sp, reflect.Struct)
	assert.True(t, b)

	spp := spec.Float64Property()

	s = NewSchemaValidator(spp, nil, "", strfmt.Default)

	s.SetPath("path")
	assert.Equal(t, "path", s.Path)

	r := s.Validate(nil)
	assert.NotNil(t, r)
	assert.False(t, r.IsValid())

	// Validating json.Number data against number|float64
	j := json.Number("123")
	r = s.Validate(j)
	assert.True(t, r.IsValid())

	// Validating json.Number data against integer|int32
	spp = spec.Int32Property()
	s = NewSchemaValidator(spp, nil, "", strfmt.Default)
	j = json.Number("123")
	r = s.Validate(j)
	assert.True(t, r.IsValid())

	bignum := swag.FormatFloat64(math.MaxFloat64)
	j = json.Number(bignum)
	r = s.Validate(j)
	assert.False(t, r.IsValid())

	// Validating incorrect json.Number data
	spp = spec.Float64Property()
	s = NewSchemaValidator(spp, nil, "", strfmt.Default)
	j = json.Number("AXF")
	r = s.Validate(j)
	assert.False(t, r.IsValid())
}
