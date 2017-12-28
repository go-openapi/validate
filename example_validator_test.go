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

func TestExample_ValidatesExamplesAgainstSchema(t *testing.T) {
	tests := []string{
		"response",
		"response-ref",
	}

	for _, tt := range tests {
		doc, err := loads.Spec(filepath.Join("fixtures", "validation", "valid-example-"+tt+".json"))
		if assert.NoError(t, err) {
			validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
			validator.spec = doc
			validator.analyzer = analysis.New(doc.Spec())
			myExampleValidator := &exampleValidator{SpecValidator: validator}
			res := myExampleValidator.validateExamplesValidAgainstSchema()
			assert.Empty(t, res.Errors, tt+" should not have errors")
		}

		doc, err = loads.Spec(filepath.Join("fixtures", "validation", "invalid-example-"+tt+".json"))
		if assert.NoError(t, err) {
			validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
			validator.spec = doc
			validator.analyzer = analysis.New(doc.Spec())
			myExampleValidator := &exampleValidator{SpecValidator: validator}
			res := myExampleValidator.validateExamplesValidAgainstSchema()
			assert.NotEmpty(t, res.Errors, tt+" should have errors")
			assert.Len(t, res.Errors, 1, tt+" should have 1 error")
		}
	}
}