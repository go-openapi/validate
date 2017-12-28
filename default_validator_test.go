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
	"path/filepath"
	"testing"

	"github.com/go-openapi/analysis"
	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/assert"
)

func TestDefault_ValidateDefaultValueAgainstSchema(t *testing.T) {
	doc, _ := loads.Analyzed(PetStoreJSONMessage, "")
	validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
	validator.spec = doc
	validator.analyzer = analysis.New(doc.Spec())
	myDefaultValidator := &defaultValidator{SpecValidator: validator}
	res := myDefaultValidator.validateDefaultValueValidAgainstSchema()
	assert.Empty(t, res.Errors)

	tests := []string{
		"parameter",
		"parameter-ref",
		"parameter-items",
		"header",
		"header-items",
		"schema",
		"schema-ref",
		"schema-additionalProperties",
		"schema-patternProperties",
		"schema-items",
		"schema-allOf",
		"parameter-schema",
		"default-response",
		"header-response",
		// - DONE: header in default response
		// - invalid schema in default response
		//"default-response-PatternProperties",
		// - header in default response with patternProperties
		// - header in default response with failed patternProperties
		// - header in response
		// - failed header in response
		// - items in header in response
		// - header in response with failed patternProperties
		// - invalid schema in response
		// - items in schema in response
		// - patternProperties in schema in response
		// - additionalProperties in schema in response
		// - Pattern validation
	}

	for _, tt := range tests {
		path := filepath.Join("fixtures", "validation", "valid-default-value-"+tt+".json")
		//t.Logf("Testing valid default values for: %s", path)
		doc, err := loads.Spec(path)
		if assert.NoError(t, err) {
			validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
			validator.spec = doc
			validator.analyzer = analysis.New(doc.Spec())
			myDefaultValidator := &defaultValidator{SpecValidator: validator}
			res := myDefaultValidator.validateDefaultValueValidAgainstSchema()
			assert.Empty(t, res.Errors, tt+" should not have errors")
		}

		path = filepath.Join("fixtures", "validation", "invalid-default-value-"+tt+".json")
		//t.Logf("Testing invalid default values for: %s", path)
		doc, err = loads.Spec(path)
		if assert.NoError(t, err) {
			validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
			validator.spec = doc
			validator.analyzer = analysis.New(doc.Spec())
			myDefaultValidator := &defaultValidator{SpecValidator: validator}
			res := myDefaultValidator.validateDefaultValueValidAgainstSchema()
			assert.NotEmpty(t, res.Errors, tt+" should have errors")
			// Update: now we have an additional message to explain it's all about a default value
			// Example:
			// - default value for limit in query does not validate its Schema
			// - limit in query must be of type integer: "string"]
			assert.True(t, len(res.Errors) >= 1, tt+" should have at least 1 error")
		}
	}
}
