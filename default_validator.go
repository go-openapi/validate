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

	"github.com/go-openapi/errors"
	"github.com/go-openapi/spec"
)

// defaultValidator validates default values in a spec.
// According to Swagger spec, default values MUST validate their schema.
type defaultValidator struct {
	SpecValidator *SpecValidator
}

// Validate validates the default values declared in the swagger spec
//
func (d *defaultValidator) Validate() (errs *Result) {
	errs = new(Result)
	if d == nil || d.SpecValidator == nil {
		return errs
	}
	errs.Merge(d.validateDefaultValueValidAgainstSchema()) // error -
	return errs
}

func (d *defaultValidator) validateDefaultValueValidAgainstSchema() *Result {
	// every default value that is specified must validate against the schema for that property
	// headers, items, parameters, schema

	res := new(Result)
	s := d.SpecValidator

	for method, pathItem := range s.analyzer.Operations() {
		if pathItem != nil { // Safeguard
			for path, op := range pathItem {
				// parameters
				for _, param := range paramHelp.safeExpandedParamsFor(path, method, op.ID, res, s) {
					if param.Default != nil && param.Required {
						res.AddWarnings(errors.New(errors.CompositeErrorCode, "%s in %s has a default value and is required as parameter", param.Name, param.In))
					}

					// Check simple parameters first
					// default values provided must validate against their inline definition (no explicit schema)
					if param.Default != nil && param.Schema == nil {
						// check param default value is valid
						red := NewParamValidator(&param, s.KnownFormats).Validate(param.Default)
						if red.HasErrorsOrWarnings() {
							res.AddErrors(errors.New(errors.CompositeErrorCode, "default value for %s in %s does not validate its schema", param.Name, param.In))
							res.Merge(red)
						}
					}

					// Recursively follows Items and Schemas
					if param.Items != nil {
						red := d.validateDefaultValueItemsAgainstSchema(param.Name, param.In, &param, param.Items)
						if red.HasErrorsOrWarnings() {
							res.AddErrors(errors.New(errors.CompositeErrorCode, "default value for %s.items in %s does not validate its schema", param.Name, param.In))
							res.Merge(red)
						}
					}

					if param.Schema != nil {
						// Validate default value against schema
						red := d.validateDefaultValueSchemaAgainstSchema(param.Name, param.In, param.Schema)
						if red.HasErrorsOrWarnings() {
							res.AddErrors(errors.New(errors.CompositeErrorCode, "default value for %s in %s does not validate its schema", param.Name, param.In))
							res.Merge(red)
						}
					}
				}

				if op.Responses != nil {
					if op.Responses.Default != nil {
						// Same constraint on default Responses
						res.Merge(d.validateDefaultInResponse(op.Responses.Default, "default", 0, op.ID))
					}
					// Same constraint on regular Responses
					if op.Responses.StatusCodeResponses != nil { // Safeguard
						for code, r := range op.Responses.StatusCodeResponses {
							res.Merge(d.validateDefaultInResponse(&r, "response", code, op.ID))
						}
					}
				} else {
					// Empty op.ID means there is no meaningful operation: no need to report a specific message
					// TODO(TEST): test that no response definition ends up with an error
					if op.ID != "" {
						res.AddErrors(errors.New(errors.CompositeErrorCode, "operation %q has no valid response", op.ID))
					}
				}
			}
		}
	}
	if s.spec.Spec().Definitions != nil { // Safeguard
		for nm, sch := range s.spec.Spec().Definitions {
			res.Merge(d.validateDefaultValueSchemaAgainstSchema(fmt.Sprintf("definitions.%s", nm), "body", &sch))
		}
	}
	return res
}

func (d *defaultValidator) validateDefaultInResponse(response *spec.Response, responseType string, responseCode int, operationID string) *Result {
	var responseName, responseCodeAsStr string

	res := new(Result)
	s := d.SpecValidator

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
			if h.Default != nil {
				red := NewHeaderValidator(nm, &h, s.KnownFormats).Validate(h.Default)
				if red.HasErrorsOrWarnings() {
					msg := "in operation %q, default value in header %s for %s does not validate its schema"
					res.AddErrors(errors.New(errors.CompositeErrorCode, msg, operationID, nm, responseName))
					res.Merge(red)
				}
			}

			// Headers have inline definition, like params
			if h.Items != nil {
				red := d.validateDefaultValueItemsAgainstSchema(nm, "header", &h, h.Items)
				if red.HasErrorsOrWarnings() {
					msg := "in operation %q, default value in header.items %s for %s does not validate its schema"
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
		red := d.validateDefaultValueSchemaAgainstSchema(responseCodeAsStr, "response", response.Schema)
		if red.HasErrorsOrWarnings() {
			// Additional message to make sure the context of the error is not lost
			msg := "in operation %q, default value in %s does not validate its schema"
			res.AddErrors(errors.New(errors.CompositeErrorCode, msg, operationID, responseName))
			res.Merge(red)
		}
	}
	return res
}

func (d *defaultValidator) validateDefaultValueSchemaAgainstSchema(path, in string, schema *spec.Schema) *Result {
	res := new(Result)
	s := d.SpecValidator
	if schema != nil { // Safeguard
		if schema.Default != nil {
			res.Merge(NewSchemaValidator(schema, s.spec.Spec(), path+".default", s.KnownFormats).Validate(schema.Default))
		}
		if schema.Items != nil {
			if schema.Items.Schema != nil {
				res.Merge(d.validateDefaultValueSchemaAgainstSchema(path+".items.default", in, schema.Items.Schema))
			}
			// Multiple schemas in items
			if schema.Items.Schemas != nil { // Safeguard
				for i, sch := range schema.Items.Schemas {
					res.Merge(d.validateDefaultValueSchemaAgainstSchema(fmt.Sprintf("%s.items[%d].default", path, i), in, &sch))
				}
			}
		}
		if _, err := compileRegexp(schema.Pattern); err != nil {
			res.AddErrors(errors.New(errors.CompositeErrorCode, "%s in %s has invalid pattern: %q", path, in, schema.Pattern))
		}
		if schema.AdditionalItems != nil && schema.AdditionalItems.Schema != nil {
			res.Merge(d.validateDefaultValueSchemaAgainstSchema(fmt.Sprintf("%s.additionalItems", path), in, schema.AdditionalItems.Schema))
		}
		for propName, prop := range schema.Properties {
			res.Merge(d.validateDefaultValueSchemaAgainstSchema(path+"."+propName, in, &prop))
		}
		for propName, prop := range schema.PatternProperties {
			res.Merge(d.validateDefaultValueSchemaAgainstSchema(path+"."+propName, in, &prop))
		}
		if schema.AdditionalProperties != nil && schema.AdditionalProperties.Schema != nil {
			res.Merge(d.validateDefaultValueSchemaAgainstSchema(fmt.Sprintf("%s.additionalProperties", path), in, schema.AdditionalProperties.Schema))
		}
		if schema.AllOf != nil {
			for i, aoSch := range schema.AllOf {
				res.Merge(d.validateDefaultValueSchemaAgainstSchema(fmt.Sprintf("%s.allOf[%d]", path, i), in, &aoSch))
			}
		}
	}
	return res
}

func (d *defaultValidator) validateDefaultValueItemsAgainstSchema(path, in string, root interface{}, items *spec.Items) *Result {
	res := new(Result)
	s := d.SpecValidator
	if items != nil {
		if items.Default != nil {
			res.Merge(newItemsValidator(path, in, items, root, s.KnownFormats).Validate(0, items.Default))
		}
		if items.Items != nil {
			// TODO(TEST): test case
			res.Merge(d.validateDefaultValueItemsAgainstSchema(path+"[0].default", in, root, items.Items))
		}
		if _, err := compileRegexp(items.Pattern); err != nil {
			res.AddErrors(errors.New(errors.CompositeErrorCode, "%s in %s has invalid pattern: %q", path, in, items.Pattern))
		}
	}
	return res
}
