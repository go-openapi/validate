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

package validate_test

import (
	"fmt"

	"github.com/go-openapi/loads"    // Spec loading
	"github.com/go-openapi/strfmt"   // OpenAPI format extensions
	"github.com/go-openapi/validate" // This package
)

func ExampleSpec() {
	// Example with high level spec validation call, without showing warnings

	//url := "http://petstore.swagger.io/v2/swagger.json"
	//doc, err := loads.JSONSpec(url)
	path := "fixtures/validation/valid-ref.json"
	doc, err := loads.Spec(path)
	if err == nil {
		validate.SetContinueOnErrors(true)         // Set global options
		errs := validate.Spec(doc, strfmt.Default) // Validates spec with default Swagger 2.0 format definitions

		if errs == nil {
			fmt.Println("This spec is valid")
		} else {
			fmt.Printf("This spec has some validation errors: %v\n", errs)
		}
	} else {
		fmt.Println("Could not load this spec")
	}
	// Output: This spec is valid
}

func ExampleSpecValidator_Validate() {
	// Example of spec validation call with full result

	//url := "http://petstore.swagger.io/v2/swagger.json"
	//doc, err := loads.JSONSpec(url)
	path := "fixtures/validation/valid-ref.json"
	doc, err := loads.Spec(path)
	if err == nil {
		validator := validate.NewSpecValidator(doc.Schema(), strfmt.Default)
		validator.SetContinueOnErrors(true)  // Set option for this validator
		result, _ := validator.Validate(doc) // Validates spec with default Swagger 2.0 format definitions
		if result.IsValid() {
			fmt.Println("This spec is valid")
		} else {
			fmt.Println("This spec has some validation errors")
		}
		if result.HasWarnings() {
			fmt.Println("This spec has some validation warnings")
		}
	}
	// Output:
	// This spec is valid
	// This spec has some validation warnings
}
