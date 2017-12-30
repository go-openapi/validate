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
	"fmt"
	"strconv"
	"strings"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/spec"
)

// ExampleValidator validates example values defined in a spec
type exampleValidator struct {
	SpecValidator *SpecValidator
}

// Validate validates the example values declared in the swagger spec
// Example values MUST conform to their schema.
//
// With Swagger 2.0, examples are supported in:
//   - schemas
//   - individual property
//   - responses
//
//  NOTE: examples should not supported in in line parameters definitions and headers??
func (ex *exampleValidator) Validate() (errs *Result) {
	errs = new(Result)
	if ex == nil || ex.SpecValidator == nil {
		return errs
	}
	errs.Merge(ex.validateExampleValueValidAgainstSchema()) // error -

	return errs
}

func (ex *exampleValidator) validateExampleValueValidAgainstSchema() *Result {
	// every example value that is specified must validate against the schema for that property
	// in: schemas, properties, object, items
	// not in: headers, parameters without schema

	res := new(Result)
	s := ex.SpecValidator

	for method, pathItem := range s.analyzer.Operations() {
		if pathItem != nil { // Safeguard
			for path, op := range pathItem {
				// parameters
				for _, param := range paramHelp.safeExpandedParamsFor(path, method, op.ID, res, s) {

					// As of swagger 2.0, Examples are not supported in simple parameters
					// However, it looks like it is supported by go-openapi

					// Check simple parameters first
					// default values provided must validate against their inline definition (no explicit schema)
					if param.Example != nil && param.Schema == nil {
						// check param default value is valid
						red := NewParamValidator(&param, s.KnownFormats).Validate(param.Example)
						if red.HasErrorsOrWarnings() {
							res.AddErrors(errors.New(errors.CompositeErrorCode, "example value for %s in %s does not validate its schema", param.Name, param.In))
							res.Merge(red)
						}
					}

					// Recursively follows Items and Schemas
					if param.Items != nil {
						red := ex.validateExampleValueItemsAgainstSchema(param.Name, param.In, &param, param.Items)
						if red.HasErrorsOrWarnings() {
							res.AddErrors(errors.New(errors.CompositeErrorCode, "example value for %s.items in %s does not validate its schema", param.Name, param.In))
							res.Merge(red)
						}
					}

					if param.Schema != nil {
						// Validate example value against schema
						red := ex.validateExampleValueSchemaAgainstSchema(param.Name, param.In, param.Schema)
						if red.HasErrorsOrWarnings() {
							res.AddErrors(errors.New(errors.CompositeErrorCode, "example value for %s in %s does not validate its schema", param.Name, param.In))
							res.Merge(red)
						}
					}
				}

				if op.Responses != nil {
					if op.Responses.Default != nil {
						// Same constraint on default Response
						res.Merge(ex.validateExampleInResponse(op.Responses.Default, "default", path, 0, op.ID))
					}
					// Same constraint on regular Responses
					if op.Responses.StatusCodeResponses != nil { // Safeguard
						for code, r := range op.Responses.StatusCodeResponses {
							res.Merge(ex.validateExampleInResponse(&r, "response", path, code, op.ID))
						}
					}
				} else {
					// Empty op.ID means there is no meaningful operation: no need to report a specific message
					if op.ID != "" {
						res.AddErrors(errors.New(errors.CompositeErrorCode, "operation %q has no valid response", op.ID))
					}
				}
			}
		}
	}
	if s.spec.Spec().Definitions != nil { // Safeguard
		for nm, sch := range s.spec.Spec().Definitions {
			res.Merge(ex.validateExampleValueSchemaAgainstSchema(fmt.Sprintf("definitions.%s", nm), "body", &sch))
		}
	}
	return res
}

func (ex *exampleValidator) validateExampleInResponse(response *spec.Response, responseType, path string, responseCode int, operationID string) *Result {
	var responseName, responseCodeAsStr string

	res := new(Result)
	s := ex.SpecValidator

	// Recursively follow possible $ref's
	for response.Ref.String() != "" {
		obj, _, err := response.Ref.GetPointer().Get(s.spec.Spec())
		if err != nil {
			// NOTE: with new ref expansion in spec, this code is no more reachable
			errorHelp.addPointerError(res, err, response.Ref.String(), strings.Join([]string{"\"" + path + "\"", response.ResponseProps.Schema.ID}, "."))
			return res
		}
		// Here we may expect type assertion to be guaranteed (not like in the Parameter case)
		nr := obj.(spec.Response)
		response = &nr
	}

	// Message variants
	if responseType == "default" {
		responseCodeAsStr = "default"
		responseName = "default response"
	} else {
		responseCodeAsStr = strconv.Itoa(responseCode)
		responseName = "response " + responseCodeAsStr
	}

	if response.Headers != nil { // Safeguard
		for nm, h := range response.Headers {
			if h.Example != nil {
				red := NewHeaderValidator(nm, &h, s.KnownFormats).Validate(h.Example)
				if red.HasErrorsOrWarnings() {
					msg := "in operation %q, example value in header %s for %s does not validate its schema"
					res.AddErrors(errors.New(errors.CompositeErrorCode, msg, operationID, nm, responseName))
					res.Merge(red)
				}
			}

			// Headers have inline definition, like params
			if h.Items != nil {
				red := ex.validateExampleValueItemsAgainstSchema(nm, "header", &h, h.Items)
				if red.HasErrorsOrWarnings() {
					msg := "in operation %q, example value in header.items %s for %s does not validate its schema"
					res.AddErrors(errors.New(errors.CompositeErrorCode, msg, operationID, nm, responseName))
					res.Merge(red)
				}
			}

			if _, err := compileRegexp(h.Pattern); err != nil {
				msg := "in operation %q, header %s for %s has invalid pattern %q: %v"
				res.AddErrors(errors.New(errors.CompositeErrorCode, msg, operationID, nm, responseName, h.Pattern, err))
			}

			// Headers don't have schema
		}
	}
	if response.Schema != nil {
		red := ex.validateExampleValueSchemaAgainstSchema(responseCodeAsStr, "response", response.Schema)
		if red.HasErrorsOrWarnings() {
			// Additional message to make sure the context of the error is not lost
			msg := "in operation %q, example value in %s does not validate its schema"
			res.AddErrors(errors.New(errors.CompositeErrorCode, msg, operationID, responseName))
			res.Merge(red)
		}
	}

	if response.Examples != nil {
		if response.Schema != nil {
			if example, ok := response.Examples["application/json"]; ok {
				res.Merge(NewSchemaValidator(response.Schema, s.spec.Spec(), path, s.KnownFormats).Validate(example))
			} else {
				// TODO: validate other media types too
				res.AddWarnings(errors.New(errors.CompositeErrorCode, "No validation attempt for examples for media types other than application/json, in operation %q, %s:", operationID, responseName))
			}
		} else {
			// TODO(TEST)
			res.AddWarnings(errors.New(errors.CompositeErrorCode, "Examples provided without schema in operation %q, %s:", operationID, responseName))
		}

	}
	return res
}

func (ex *exampleValidator) validateExampleValueSchemaAgainstSchema(path, in string, schema *spec.Schema) *Result {
	res := new(Result)
	s := ex.SpecValidator
	if schema != nil { // Safeguard
		if schema.Example != nil {
			res.Merge(NewSchemaValidator(schema, s.spec.Spec(), path+".example", s.KnownFormats).Validate(schema.Example))
		}
		if schema.Items != nil {
			if schema.Items.Schema != nil {
				res.Merge(ex.validateExampleValueSchemaAgainstSchema(path+".items.example", in, schema.Items.Schema))
			}
			// Multiple schemas in items
			if schema.Items.Schemas != nil { // Safeguard
				for i, sch := range schema.Items.Schemas {
					res.Merge(ex.validateExampleValueSchemaAgainstSchema(fmt.Sprintf("%s.items[%d].example", path, i), in, &sch))
				}
			}
		}
		if _, err := compileRegexp(schema.Pattern); err != nil {
			res.AddErrors(errors.New(errors.CompositeErrorCode, "%s in %s has invalid pattern: %q", path, in, schema.Pattern))
		}
		if schema.AdditionalItems != nil && schema.AdditionalItems.Schema != nil {
			res.Merge(ex.validateExampleValueSchemaAgainstSchema(fmt.Sprintf("%s.additionalItems", path), in, schema.AdditionalItems.Schema))
		}
		for propName, prop := range schema.Properties {
			res.Merge(ex.validateExampleValueSchemaAgainstSchema(path+"."+propName, in, &prop))
		}
		for propName, prop := range schema.PatternProperties {
			res.Merge(ex.validateExampleValueSchemaAgainstSchema(path+"."+propName, in, &prop))
		}
		if schema.AdditionalProperties != nil && schema.AdditionalProperties.Schema != nil {
			res.Merge(ex.validateExampleValueSchemaAgainstSchema(fmt.Sprintf("%s.additionalProperties", path), in, schema.AdditionalProperties.Schema))
		}
		if schema.AllOf != nil {
			for i, aoSch := range schema.AllOf {
				res.Merge(ex.validateExampleValueSchemaAgainstSchema(fmt.Sprintf("%s.allOf[%d]", path, i), in, &aoSch))
			}
		}
	}
	return res
}

func (ex *exampleValidator) validateExampleValueItemsAgainstSchema(path, in string, root interface{}, items *spec.Items) *Result {
	res := new(Result)
	s := ex.SpecValidator
	if items != nil {
		if items.Example != nil {
			res.Merge(newItemsValidator(path, in, items, root, s.KnownFormats).Validate(0, items.Example))
		}
		if items.Items != nil {
			// TODO(TEST): test case
			res.Merge(ex.validateExampleValueItemsAgainstSchema(path+"[0].example", in, root, items.Items))
		}
		if _, err := compileRegexp(items.Pattern); err != nil {
			res.AddErrors(errors.New(errors.CompositeErrorCode, "%s in %s has invalid pattern: %q", path, in, items.Pattern))
		}
	}
	return res
}
