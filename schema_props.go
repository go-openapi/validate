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
	"log"
	"reflect"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
)

type schemaPropsValidator struct {
	Path            string
	In              string
	AllOf           []spec.Schema
	OneOf           []spec.Schema
	AnyOf           []spec.Schema
	Not             *spec.Schema
	Dependencies    spec.Dependencies
	anyOfValidators []SchemaValidator
	allOfValidators []SchemaValidator
	oneOfValidators []SchemaValidator
	notValidator    *SchemaValidator
	Root            interface{}
	KnownFormats    strfmt.Registry
}

func (s *schemaPropsValidator) SetPath(path string) {
	s.Path = path
}

func newSchemaPropsValidator(path string, in string, allOf, oneOf, anyOf []spec.Schema, not *spec.Schema, deps spec.Dependencies, root interface{}, formats strfmt.Registry) *schemaPropsValidator {
	var anyValidators []SchemaValidator
	for _, v := range anyOf {
		anyValidators = append(anyValidators, *NewSchemaValidator(&v, root, path, formats))
	}
	var allValidators []SchemaValidator
	for _, v := range allOf {
		allValidators = append(allValidators, *NewSchemaValidator(&v, root, path, formats))
	}
	var oneValidators []SchemaValidator
	for _, v := range oneOf {
		oneValidators = append(oneValidators, *NewSchemaValidator(&v, root, path, formats))
	}

	var notValidator *SchemaValidator
	if not != nil {
		notValidator = NewSchemaValidator(not, root, path, formats)
	}

	return &schemaPropsValidator{
		Path:            path,
		In:              in,
		AllOf:           allOf,
		OneOf:           oneOf,
		AnyOf:           anyOf,
		Not:             not,
		Dependencies:    deps,
		anyOfValidators: anyValidators,
		allOfValidators: allValidators,
		oneOfValidators: oneValidators,
		notValidator:    notValidator,
		Root:            root,
		KnownFormats:    formats,
	}
}

func (s *schemaPropsValidator) Applies(source interface{}, kind reflect.Kind) bool {
	r := reflect.TypeOf(source) == specSchemaType
	if Debug {
		log.Printf("schema props validator for %q applies %t for %T (kind: %v)\n", s.Path, r, source, kind)
	}
	return r
}

func (s *schemaPropsValidator) Validate(data interface{}) *Result {
	mainResult := new(Result)
	var firstSuccess *Result
	if len(s.anyOfValidators) > 0 {
		var bestFailures *Result
		succeededOnce := false
		for _, anyOfSchema := range s.anyOfValidators {
			// DEBUG
			//fmt.Printf("DEBUG: Validate anyOf: %s\n", s.Path)
			//d := spew.ConfigState{MaxDepth: 1}
			//d2 := spew.ConfigState{MaxDepth: 2}
			//d.Dump(anyOfSchema)
			//fmt.Println("EDEBUG")
			// Recursively validate schemas
			result := anyOfSchema.Validate(data)
			//fmt.Println("DEBUG: Validate anyOf:result")
			//d2.Dump(result)
			//d2.Printf("DEBUG: result.IsValid()=%t\n", result.IsValid())
			//fmt.Println("EDEBUG")
			if result.IsValid() {
				//fmt.Println("DEBUG: this one IsValid")
				bestFailures = nil
				succeededOnce = true
				if firstSuccess == nil {
					firstSuccess = result
				}
				break
			}
			if bestFailures == nil || result.MatchCount > bestFailures.MatchCount {
				bestFailures = result
			}
		}

		if !succeededOnce {
			mainResult.AddErrors(errors.New(errors.CompositeErrorCode, fmt.Sprintf("\"%s\" must validate at least one schema (anyOf)", s.Path)))
		}
		if bestFailures != nil {
			mainResult.Merge(bestFailures)
		} else if firstSuccess != nil {
			mainResult.Merge(firstSuccess)
		}
	}

	if len(s.oneOfValidators) > 0 {
		var bestFailures *Result
		var firstSuccess *Result
		validated := 0

		for _, oneOfSchema := range s.oneOfValidators {
			result := oneOfSchema.Validate(data)
			if result.IsValid() {
				validated++
				bestFailures = nil
				if firstSuccess == nil {
					firstSuccess = result
				}
				continue
			}
			if validated == 0 && (bestFailures == nil || result.MatchCount > bestFailures.MatchCount) {
				bestFailures = result
			}
		}

		if validated != 1 {
			// TODO: this ones is too often detected. Should be filtered if better error assessment found
			additionalMsg := ""
			if validated == 0 {
				additionalMsg = "Found none valid"
			} else {
				additionalMsg = fmt.Sprintf("Found %d valid alternatives", validated)
			}
			mainResult.AddErrors(errors.New(errors.CompositeErrorCode, fmt.Sprintf("\"%s\" must validate one and only one schema (oneOf). %s", s.Path, additionalMsg)))
			if bestFailures != nil {
				mainResult.Merge(bestFailures)
			}
		} else if firstSuccess != nil {
			mainResult.Merge(firstSuccess)
		}
	}

	if len(s.allOfValidators) > 0 {
		validated := 0

		for _, allOfSchema := range s.allOfValidators {
			result := allOfSchema.Validate(data)
			if result.IsValid() {
				validated++
			}
			mainResult.Merge(result)
		}

		if validated != len(s.allOfValidators) {
			additionalMsg := ""
			if validated == 0 {
				additionalMsg = ". None validated"
			}
			mainResult.AddErrors(errors.New(errors.CompositeErrorCode, fmt.Sprintf("\"%s\" must validate all the schemas (allOf)%s", s.Path, additionalMsg)))
		}
	}

	if s.notValidator != nil {
		result := s.notValidator.Validate(data)
		if result.IsValid() {
			mainResult.AddErrors(errors.New(errors.CompositeErrorCode, fmt.Sprintf("\"%s\" must not validate the schema (not)", s.Path)))
		}
	}

	if s.Dependencies != nil && len(s.Dependencies) > 0 && reflect.TypeOf(data).Kind() == reflect.Map {
		val := data.(map[string]interface{})
		for key := range val {
			if dep, ok := s.Dependencies[key]; ok {

				if dep.Schema != nil {
					mainResult.Merge(NewSchemaValidator(dep.Schema, s.Root, s.Path+"."+key, s.KnownFormats).Validate(data))
					continue
				}

				if len(dep.Property) > 0 {
					for _, depKey := range dep.Property {
						if _, ok := val[depKey]; !ok {
							mainResult.AddErrors(errors.New(errors.CompositeErrorCode, "\"%s\" has a dependency on %s", s.Path, depKey))
						}
					}
				}
			}
		}
	}

	mainResult.Inc()
	return mainResult
}
