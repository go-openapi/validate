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
	"strings"

	"github.com/go-openapi/spec"
)

// ExampleValidator validates example values defined in a spec
type exampleValidator struct {
	SpecValidator *SpecValidator
}

// Validate validates the example values declared in the swagger spec
func (ex *exampleValidator) Validate() (errs *Result) {
	errs = new(Result)
	if ex == nil || ex.SpecValidator == nil {
		return errs
	}
	errs.Merge(ex.validateExamplesValidAgainstSchema()) // error -

	// TODO: errs.Merge(d.validateExampleValueValidAgainstSchema()) // error -
	return errs
}

func (ex *exampleValidator) validateResponseExample(path string, r *spec.Response) *Result {
	// values provided as example in responses must validate the schema they examplify
	res := new(Result)
	s := ex.SpecValidator

	// Recursively follow possible $ref's
	if r.Ref.String() != "" {
		nr, _, err := r.Ref.GetPointer().Get(s.spec.Spec())
		if err != nil {
			// NOTE: with new ref expansion in spec, this code is no more reachable
			errorHelp.addPointerError(res, err, r.Ref.String(), strings.Join([]string{"\"" + path + "\"", r.ResponseProps.Schema.ID}, "."))
			return res
		}
		// Here we may expect type assertion to be guaranteed (not like in the Parameter case)
		rr := nr.(spec.Response)
		return ex.validateResponseExample(path, &rr)
	}

	// NOTE: "examples" in responses vs "example" in other definitions
	if r.Examples != nil {
		if r.Schema != nil {
			if example, ok := r.Examples["application/json"]; ok {
				res.Merge(NewSchemaValidator(r.Schema, s.spec.Spec(), path, s.KnownFormats).Validate(example))
			}

			// TODO: validate other media types too
		}
	}
	return res
}

func (ex *exampleValidator) validateExamplesValidAgainstSchema() *Result {
	// validates all examples provided in a spec
	// - values provides as Examples in a response must validate the response's schema
	// - TODO: examples for params, etc..
	res := new(Result)
	s := ex.SpecValidator

	for _ /*method*/, pathItem := range s.analyzer.Operations() {
		if pathItem != nil { // Safeguard
			for path, op := range pathItem {
				// Check Examples in Responses
				if op.Responses != nil {
					if op.Responses.Default != nil {
						dr := op.Responses.Default
						res.Merge(ex.validateResponseExample(path, dr))
					}
					if op.Responses.StatusCodeResponses != nil { // Safeguard
						for _ /*code*/, r := range op.Responses.StatusCodeResponses {
							res.Merge(ex.validateResponseExample(path, &r))
						}
					}
				}
			}
		}
	}
	return res
}

/*
  TODO: spec scanner along the lines of default_validator.go
*/
