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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-openapi/analysis"
	"github.com/go-openapi/loads"
	"github.com/go-openapi/loads/fmts"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/assert"
)

var (
	// This debug environment variable allows to report and capture actual validation messages
	// during testing. It should be disabled (undefined) during CI tests.
	DebugTest = os.Getenv("SWAGGER_DEBUG_TEST") != ""
)

func init() {
	loads.AddLoader(fmts.YAMLMatcher, fmts.YAMLDoc)
}

func TestExpandResponseLocalFile(t *testing.T) {
	fp := filepath.Join("fixtures", "local_expansion", "spec.yaml")
	doc, err := loads.Spec(fp)
	if assert.NoError(t, err) {
		if assert.NotNil(t, doc) {
			validator := NewSpecValidator(doc.Schema(), strfmt.Default)
			res, _ := validator.Validate(doc)
			assert.True(t, res.IsValid())
			assert.Empty(t, res.Errors)
		}
	}
}

func TestExpandResponseRecursive(t *testing.T) {
	fp := filepath.Join("fixtures", "recursive_expansion", "spec.yaml")
	doc, err := loads.Spec(fp)
	if assert.NoError(t, err) {
		if assert.NotNil(t, doc) {
			validator := NewSpecValidator(doc.Schema(), strfmt.Default)
			res, _ := validator.Validate(doc)
			assert.True(t, res.IsValid())
			assert.Empty(t, res.Errors)
		}
	}
}

// Spec with no path
func TestIssue52(t *testing.T) {
	fp := filepath.Join("fixtures", "bugs", "52", "swagger.json")
	jstext, _ := ioutil.ReadFile(fp)

	defer func() {
		// TODO : false
		SetContinueOnErrors(true)
	}()

	// as json schema
	var sch spec.Schema
	if assert.NoError(t, json.Unmarshal(jstext, &sch)) {
		validator := NewSchemaValidator(spec.MustLoadSwagger20Schema(), nil, "", strfmt.Default)
		res := validator.Validate(&sch)
		assert.False(t, res.IsValid())
		assert.EqualError(t, res.Errors[0], ".paths in body is required")
	}

	// as swagger spec: path is set to nil
	// Here, validation stops as paths is initialized to empty
	SetContinueOnErrors(false)
	doc, err := loads.Spec(fp)
	if assert.NoError(t, err) {
		validator := NewSpecValidator(doc.Schema(), strfmt.Default)
		res, _ := validator.Validate(doc)
		assert.False(t, res.IsValid())
		assert.EqualError(t, res.Errors[0], ".paths in body is required")
	}
	// Here, validation continues, with invalid path from early checks as null.
	// This provides an additional (hopefully more informative) message.
	SetContinueOnErrors(true) //
	doc, err = loads.Spec(fp)
	if assert.NoError(t, err) {
		validator := NewSpecValidator(doc.Schema(), strfmt.Default)
		res, _ := validator.Validate(doc)
		assert.False(t, res.IsValid())
		var verifiedErrors []string
		for _, e := range res.Errors {
			verifiedErrors = append(verifiedErrors, e.Error())
		}
		assert.Len(t, verifiedErrors, 2, "Unexpected number of error messages returned")
		assert.Contains(t, verifiedErrors, ".paths in body is required")
		assert.Contains(t, verifiedErrors, "spec has no valid path defined")
	}
}

func TestIssue53(t *testing.T) {
	fp := filepath.Join("fixtures", "bugs", "53", "noswagger.json")
	jstext, _ := ioutil.ReadFile(fp)

	// as json schema
	var sch spec.Schema
	if assert.NoError(t, json.Unmarshal(jstext, &sch)) {
		validator := NewSchemaValidator(spec.MustLoadSwagger20Schema(), nil, "", strfmt.Default)
		res := validator.Validate(&sch)
		assert.False(t, res.IsValid())
		assert.EqualError(t, res.Errors[0], ".swagger in body is required")
	}

	// as swagger despec
	doc, err := loads.Spec(fp)
	if assert.NoError(t, err) {
		validator := NewSpecValidator(doc.Schema(), strfmt.Default)
		res, _ := validator.Validate(doc)
		if assert.False(t, res.IsValid()) {
			assert.EqualError(t, res.Errors[0], ".swagger in body is required")
		}
	}
}

func TestIssue62(t *testing.T) {
	// TODO: why skipping?
	t.SkipNow()
	fp := filepath.Join("fixtures", "bugs", "62", "swagger.json")

	// as swagger spec
	doc, err := loads.Spec(fp)
	if assert.NoError(t, err) {
		validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
		res, _ := validator.Validate(doc)
		assert.NotEmpty(t, res.Errors)
		assert.True(t, res.HasErrors())
	}
}

func TestIssue63(t *testing.T) {
	fp := filepath.Join("fixtures", "bugs", "63", "swagger.json")

	// as swagger spec
	doc, err := loads.Spec(fp)
	if assert.NoError(t, err) {
		validator := NewSpecValidator(doc.Schema(), strfmt.Default)
		res, _ := validator.Validate(doc)
		assert.True(t, res.IsValid())
	}
}

func TestIssue61_MultipleRefs(t *testing.T) {
	fp := filepath.Join("fixtures", "bugs", "61", "multiple-refs.json")

	// as swagger spec
	doc, err := loads.Spec(fp)
	if assert.NoError(t, err) {
		validator := NewSpecValidator(doc.Schema(), strfmt.Default)
		res, _ := validator.Validate(doc)
		assert.Empty(t, res.Errors)
		assert.True(t, res.IsValid())
	}
}

func TestIssue61_ResolvedRef(t *testing.T) {
	fp := filepath.Join("fixtures", "bugs", "61", "unresolved-ref-for-name.json")

	// as swagger spec
	doc, err := loads.Spec(fp)
	if assert.NoError(t, err) {
		validator := NewSpecValidator(doc.Schema(), strfmt.Default)
		res, _ := validator.Validate(doc)
		assert.Empty(t, res.Errors)
		assert.True(t, res.IsValid())
	}
}

// No error with this one
func TestIssue123(t *testing.T) {
	path := "swagger.yml"
	fp := filepath.Join("fixtures", "bugs", "123", path)

	// as swagger spec
	doc, err := loads.Spec(fp)
	if assert.NoError(t, err) {
		validator := NewSpecValidator(doc.Schema(), strfmt.Default)
		res, _ := validator.Validate(doc)
		assert.True(t, res.IsValid())

		var verifiedErrors []string
		for _, e := range res.Errors {
			verifiedErrors = append(verifiedErrors, e.Error())
		}
		switch {
		case strings.Contains(path, "swagger.yml"):
			assert.Empty(t, verifiedErrors)
		default:
			t.Logf("Returned error messages: %v", verifiedErrors)
			t.Fatal("fixture not tested. Please add assertions for messages")
		}

		if DebugTest {
			t.Logf("DEVMODE: Returned error messages validating %s ", path)
			for _, v := range verifiedErrors {
				t.Logf("%s", v)
			}
		}
	}
}

func TestIssue6(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("fixtures", "bugs", "6", "*.json"))
	for _, path := range files {
		t.Logf("Tested spec=%s", path)
		doc, err := loads.Spec(path)
		if assert.NoError(t, err) {
			validator := NewSpecValidator(doc.Schema(), strfmt.Default)
			res, _ := validator.Validate(doc)
			assert.False(t, res.IsValid())

			var verifiedErrors []string
			for _, e := range res.Errors {
				verifiedErrors = append(verifiedErrors, e.Error())
			}
			//spew.Dump(verifiedErrors)
			switch {
			/*
				spec_test.go:250: "paths./foo.get.responses" must not validate the schema (not)
				spec_test.go:250: paths./foo.get.responses in body should have at least 1 properties
				spec_test.go:215: Tested spec=fixtures/bugs/6/no-responses.json
				spec_test.go:248: Returned error messages validating fixtures/bugs/6/no-responses.json
				spec_test.go:250: paths./foo.get.responses in body is required
			*/
			case strings.Contains(path, "empty-responses.json"):
				// TODO: harmonize use of quotes in names
				// TODO: this not validation is cryptic
				assert.Contains(t, verifiedErrors, "\"paths./foo.get.responses\" must not validate the schema (not)")
				assert.Contains(t, verifiedErrors, "paths./foo.get.responses in body should have at least 1 properties")
			case strings.Contains(path, "no-responses.json"):
				assert.Contains(t, verifiedErrors, "paths./foo.get.responses in body is required")
			default:
				t.Logf("Returned error messages: %v", verifiedErrors)
				t.Fatal("fixture not tested. Please add assertions for messages")
			}
			if DebugTest {
				t.Logf("DEVMODE:Returned error messages validating %s ", path)
				for _, v := range verifiedErrors {
					t.Logf("%s", v)
				}
			}
		}
	}
}

// check if invalid patterns are indeed invalidated
func TestIssue18(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("fixtures", "bugs", "18", "*.json"))
	for _, path := range files {
		t.Logf("Tested spec=%s", path)
		doc, err := loads.Spec(path)
		if assert.NoError(t, err) {
			validator := NewSpecValidator(doc.Schema(), strfmt.Default)
			res, _ := validator.Validate(doc)
			assert.False(t, res.IsValid())

			var verifiedErrors []string
			for _, e := range res.Errors {
				verifiedErrors = append(verifiedErrors, e.Error())
			}
			switch {
			case strings.Contains(path, "headerItems.json"):
				assert.Contains(t, verifiedErrors, "X-Foo in header has invalid pattern: \")<-- bad pattern\"")
			case strings.Contains(path, "headers.json"):
				assert.Contains(t, verifiedErrors, "operation \"\" has invalid pattern in default header \"X-Foo\": \")<-- bad pattern\"")
			case strings.Contains(path, "paramItems.json"):
				assert.Contains(t, verifiedErrors, "body param \"user\" for \"\" has invalid items pattern: \")<-- bad pattern\"")
				assert.Contains(t, verifiedErrors, "user.items in body has invalid pattern: \")<-- bad pattern\"")
			case strings.Contains(path, "parameters.json"):
				assert.Contains(t, verifiedErrors, "operation \"\" has invalid pattern in param \"userId\": \")<-- bad pattern\"")
			case strings.Contains(path, "schema.json"):
				// TODO: strange that the text does not say response "200"...
				assert.Contains(t, verifiedErrors, "200 in response has invalid pattern: \")<-- bad pattern\"")
			default:
				t.Logf("Returned error messages: %v", verifiedErrors)
				t.Fatal("fixture not tested. Please add assertions for messages")
			}

			if DebugTest {
				t.Logf("DEVMODE: Returned error messages validating %s ", path)
				for _, v := range verifiedErrors {
					t.Logf("%s", v)
				}
			}
		}
	}
}

// check if a fragment path parameter is recognized, without error
func TestIssue39(t *testing.T) {
	path := "swagger.yml"
	fp := filepath.Join("fixtures", "bugs", "39", path)

	// as swagger spec
	doc, err := loads.Spec(fp)
	if assert.NoError(t, err) {
		validator := NewSpecValidator(doc.Schema(), strfmt.Default)
		res, _ := validator.Validate(doc)
		assert.True(t, res.IsValid())

		var verifiedErrors []string
		for _, e := range res.Errors {
			verifiedErrors = append(verifiedErrors, e.Error())
		}
		switch {
		case strings.Contains(path, "swagger.yml"):
			assert.Empty(t, verifiedErrors)
		default:
			t.Logf("Returned error messages: %v", verifiedErrors)
			t.Fatal("fixture not tested. Please add assertions for messages")
		}
		if DebugTest {
			t.Logf("DEVMODE: Returned error messages validating %s ", path)
			for _, v := range verifiedErrors {
				t.Logf("%s", v)
			}
		}
	}
}

func TestValidateDuplicatePropertyNames(t *testing.T) {
	// simple allOf
	doc, err := loads.Spec(filepath.Join("fixtures", "validation", "duplicateprops.json"))
	if assert.NoError(t, err) {
		validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
		validator.spec = doc
		res := validator.validateDuplicatePropertyNames()
		assert.NotEmpty(t, res.Errors)
		assert.Len(t, res.Errors, 1)

	}

	// nested allOf
	doc, err = loads.Spec(filepath.Join("fixtures", "validation", "nestedduplicateprops.json"))
	if assert.NoError(t, err) {
		validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
		validator.spec = doc
		res := validator.validateDuplicatePropertyNames()
		assert.NotEmpty(t, res.Errors)
		assert.Len(t, res.Errors, 1)

	}
}

func TestValidateNonEmptyPathParameterNames(t *testing.T) {
	doc, err := loads.Spec(filepath.Join("fixtures", "validation", "empty-path-param-name.json"))
	if assert.NoError(t, err) {
		validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
		validator.spec = doc
		res := validator.validateNonEmptyPathParamNames()
		assert.NotEmpty(t, res.Errors)
		assert.Len(t, res.Errors, 1)

	}
}

func TestValidateCircularAncestry(t *testing.T) {
	doc, err := loads.Spec(filepath.Join("fixtures", "validation", "direct-circular-ancestor.json"))
	if assert.NoError(t, err) {
		validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
		validator.spec = doc
		res := validator.validateDuplicatePropertyNames()
		assert.NotEmpty(t, res.Errors)
		assert.Len(t, res.Errors, 1)
	}

	doc, err = loads.Spec(filepath.Join("fixtures", "validation", "indirect-circular-ancestor.json"))
	if assert.NoError(t, err) {
		validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
		validator.spec = doc
		res := validator.validateDuplicatePropertyNames()
		assert.NotEmpty(t, res.Errors)
		assert.Len(t, res.Errors, 1)
	}

	doc, err = loads.Spec(filepath.Join("fixtures", "validation", "recursive-circular-ancestor.json"))
	if assert.NoError(t, err) {
		validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
		validator.spec = doc
		res := validator.validateDuplicatePropertyNames()
		assert.NotEmpty(t, res.Errors)
		assert.Len(t, res.Errors, 1)
	}

}

func TestValidateUniqueSecurityScopes(t *testing.T) {
}

func TestValidateReferenced(t *testing.T) {
	doc, err := loads.Spec(filepath.Join("fixtures", "validation", "valid-referenced.yml"))
	if assert.NoError(t, err) {
		validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
		validator.spec = doc
		validator.analyzer = analysis.New(doc.Spec())
		res := validator.validateReferenced()
		assert.Empty(t, res.Errors)
	}

	doc, err = loads.Spec(filepath.Join("fixtures", "validation", "invalid-referenced.yml"))
	if assert.NoError(t, err) {
		validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
		validator.spec = doc
		validator.analyzer = analysis.New(doc.Spec())
		res := validator.validateReferenced()
		assert.NotEmpty(t, res.Errors)
		assert.Len(t, res.Errors, 3)
	}
}

func TestValidateBodyFormDataParams(t *testing.T) {
	doc, err := loads.Spec(filepath.Join("fixtures", "validation", "invalid-formdata-body-params.json"))
	if assert.NoError(t, err) {
		validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
		validator.spec = doc
		validator.analyzer = analysis.New(doc.Spec())
		res := validator.validateDefaultValueValidAgainstSchema()
		assert.NotEmpty(t, res.Errors)
		assert.Len(t, res.Errors, 1)
	}
}

func TestValidateReferencesValid(t *testing.T) {
	doc, err := loads.Spec(filepath.Join("fixtures", "validation", "valid-ref.json"))
	if assert.NoError(t, err) {
		validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
		validator.spec = doc
		validator.analyzer = analysis.New(doc.Spec())
		res := validator.validateReferencesValid()
		assert.Empty(t, res.Errors)
	}

	doc, err = loads.Spec(filepath.Join("fixtures", "validation", "invalid-ref.json"))
	if assert.NoError(t, err) {
		validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
		validator.spec = doc
		validator.analyzer = analysis.New(doc.Spec())
		res := validator.validateReferencesValid()
		assert.NotEmpty(t, res.Errors)
		assert.Len(t, res.Errors, 1)
	}
}

func TestValidatesExamplesAgainstSchema(t *testing.T) {
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
			res := validator.validateExamplesValidAgainstSchema()
			assert.Empty(t, res.Errors, tt+" should not have errors")
		}

		doc, err = loads.Spec(filepath.Join("fixtures", "validation", "invalid-example-"+tt+".json"))
		if assert.NoError(t, err) {
			validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
			validator.spec = doc
			validator.analyzer = analysis.New(doc.Spec())
			res := validator.validateExamplesValidAgainstSchema()
			assert.NotEmpty(t, res.Errors, tt+" should have errors")
			assert.Len(t, res.Errors, 1, tt+" should have 1 error")
		}
	}
}

func TestValidateDefaultValueAgainstSchema(t *testing.T) {
	doc, _ := loads.Analyzed(PetStoreJSONMessage, "")
	validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
	validator.spec = doc
	validator.analyzer = analysis.New(doc.Spec())
	res := validator.validateDefaultValueValidAgainstSchema()
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
	}

	for _, tt := range tests {
		doc, err := loads.Spec(filepath.Join("fixtures", "validation", "valid-default-value-"+tt+".json"))
		if assert.NoError(t, err) {
			validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
			validator.spec = doc
			validator.analyzer = analysis.New(doc.Spec())
			res := validator.validateDefaultValueValidAgainstSchema()
			assert.Empty(t, res.Errors, tt+" should not have errors")
		}

		doc, err = loads.Spec(filepath.Join("fixtures", "validation", "invalid-default-value-"+tt+".json"))
		if assert.NoError(t, err) {
			validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
			validator.spec = doc
			validator.analyzer = analysis.New(doc.Spec())
			res := validator.validateDefaultValueValidAgainstSchema()
			assert.NotEmpty(t, res.Errors, tt+" should have errors")
			assert.Len(t, res.Errors, 1, tt+" should have 1 error")
		}
	}
}

func TestValidateRequiredDefinitions(t *testing.T) {
	doc, _ := loads.Analyzed(PetStoreJSONMessage, "")
	validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
	validator.spec = doc
	validator.analyzer = analysis.New(doc.Spec())
	res := validator.validateRequiredDefinitions()
	assert.Empty(t, res.Errors)

	// properties
	sw := doc.Spec()
	def := sw.Definitions["Tag"]
	def.Required = append(def.Required, "type")
	sw.Definitions["Tag"] = def
	res = validator.validateRequiredDefinitions()
	assert.NotEmpty(t, res.Errors)

	// pattern properties
	def.PatternProperties = make(map[string]spec.Schema)
	def.PatternProperties["ty.*"] = *spec.StringProperty()
	sw.Definitions["Tag"] = def
	res = validator.validateRequiredDefinitions()
	assert.Empty(t, res.Errors)

	def.PatternProperties = make(map[string]spec.Schema)
	def.PatternProperties["^ty.$"] = *spec.StringProperty()
	sw.Definitions["Tag"] = def
	res = validator.validateRequiredDefinitions()
	assert.NotEmpty(t, res.Errors)

	// additional properties
	def.PatternProperties = nil
	def.AdditionalProperties = &spec.SchemaOrBool{Allows: true}
	sw.Definitions["Tag"] = def
	res = validator.validateRequiredDefinitions()
	assert.Empty(t, res.Errors)

	def.AdditionalProperties = &spec.SchemaOrBool{Allows: false}
	sw.Definitions["Tag"] = def
	res = validator.validateRequiredDefinitions()
	assert.NotEmpty(t, res.Errors)
}

func TestValidateParameters(t *testing.T) {
	doc, _ := loads.Analyzed(PetStoreJSONMessage, "")
	validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
	validator.spec = doc
	validator.analyzer = analysis.New(doc.Spec())
	res := validator.validateParameters()
	assert.Empty(t, res.Errors)

	sw := doc.Spec()
	sw.Paths.Paths["/pets"].Get.Parameters = append(sw.Paths.Paths["/pets"].Get.Parameters, *spec.QueryParam("limit").Typed("string", ""))
	res = validator.validateParameters()
	assert.NotEmpty(t, res.Errors)

	doc, _ = loads.Analyzed(PetStoreJSONMessage, "")
	sw = doc.Spec()
	sw.Paths.Paths["/pets"].Post.Parameters = append(sw.Paths.Paths["/pets"].Post.Parameters, *spec.BodyParam("fake", spec.RefProperty("#/definitions/Pet")))
	validator = NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
	validator.spec = doc
	validator.analyzer = analysis.New(doc.Spec())
	res = validator.validateParameters()
	assert.NotEmpty(t, res.Errors)
	assert.Len(t, res.Errors, 1)
	assert.Contains(t, res.Errors[0].Error(), "has more than 1 body param")

	doc, _ = loads.Analyzed(PetStoreJSONMessage, "")
	sw = doc.Spec()
	pp := sw.Paths.Paths["/pets/{id}"]
	pp.Delete = nil
	var nameParams []spec.Parameter
	for _, p := range pp.Parameters {
		if p.Name == "id" {
			p.Name = "name"
			nameParams = append(nameParams, p)
		}
	}
	pp.Parameters = nameParams
	sw.Paths.Paths["/pets/{name}"] = pp

	validator = NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
	validator.spec = doc
	validator.analyzer = analysis.New(doc.Spec())
	res = validator.validateParameters()
	assert.NotEmpty(t, res.Errors)
	assert.Len(t, res.Errors, 1)
	assert.Contains(t, res.Errors[0].Error(), "overlaps with")

	doc, _ = loads.Analyzed(PetStoreJSONMessage, "")
	validator = NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
	validator.spec = doc
	validator.analyzer = analysis.New(doc.Spec())
	sw = doc.Spec()
	pp = sw.Paths.Paths["/pets/{id}"]
	pp.Delete = nil
	pp.Get.Parameters = nameParams
	pp.Parameters = nil
	sw.Paths.Paths["/pets/{id}"] = pp

	res = validator.validateParameters()
	assert.NotEmpty(t, res.Errors)
	assert.Len(t, res.Errors, 2)
	assert.Contains(t, res.Errors[1].Error(), "is not present in path \"/pets/{id}\"")
	assert.Contains(t, res.Errors[0].Error(), "has no parameter definition")
}

func TestValidateItems(t *testing.T) {
	doc, _ := loads.Analyzed(PetStoreJSONMessage, "")
	validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
	validator.spec = doc
	validator.analyzer = analysis.New(doc.Spec())
	res := validator.validateItems()
	assert.Empty(t, res.Errors)

	// in operation parameters
	sw := doc.Spec()
	sw.Paths.Paths["/pets"].Get.Parameters[0].Type = "array"
	res = validator.validateItems()
	assert.NotEmpty(t, res.Errors)

	sw.Paths.Paths["/pets"].Get.Parameters[0].Items = spec.NewItems().Typed("string", "")
	res = validator.validateItems()
	assert.Empty(t, res.Errors)

	sw.Paths.Paths["/pets"].Get.Parameters[0].Items = spec.NewItems().Typed("array", "")
	res = validator.validateItems()
	assert.NotEmpty(t, res.Errors)

	sw.Paths.Paths["/pets"].Get.Parameters[0].Items.Items = spec.NewItems().Typed("string", "")
	res = validator.validateItems()
	assert.Empty(t, res.Errors)

	// in global parameters
	sw.Parameters = make(map[string]spec.Parameter)
	sw.Parameters["other"] = *spec.SimpleArrayParam("other", "array", "csv")
	res = validator.validateItems()
	assert.Empty(t, res.Errors)

	//pp := spec.SimpleArrayParam("other", "array", "")
	//pp.Items = nil
	//sw.Parameters["other"] = *pp
	//res = validator.validateItems()
	//assert.NotEmpty(t, res.Errors)

	// in shared path object parameters
	doc, _ = loads.Analyzed(PetStoreJSONMessage, "")
	validator = NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
	validator.spec = doc
	validator.analyzer = analysis.New(doc.Spec())
	sw = doc.Spec()

	pa := sw.Paths.Paths["/pets"]
	pa.Parameters = []spec.Parameter{*spec.SimpleArrayParam("another", "array", "csv")}
	sw.Paths.Paths["/pets"] = pa
	res = validator.validateItems()
	assert.Empty(t, res.Errors)

	pa = sw.Paths.Paths["/pets"]
	pp := spec.SimpleArrayParam("other", "array", "")
	pp.Items = nil
	pa.Parameters = []spec.Parameter{*pp}
	sw.Paths.Paths["/pets"] = pa
	res = validator.validateItems()
	assert.NotEmpty(t, res.Errors)

	// in body param schema
	doc, _ = loads.Analyzed(PetStoreJSONMessage, "")
	validator = NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
	validator.spec = doc
	validator.analyzer = analysis.New(doc.Spec())
	sw = doc.Spec()
	pa = sw.Paths.Paths["/pets"]
	pa.Post.Parameters[0].Schema = spec.ArrayProperty(nil)
	res = validator.validateItems()
	assert.NotEmpty(t, res.Errors)

	// in response headers
	doc, _ = loads.Analyzed(PetStoreJSONMessage, "")
	validator = NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
	validator.spec = doc
	validator.analyzer = analysis.New(doc.Spec())
	sw = doc.Spec()
	pa = sw.Paths.Paths["/pets"]
	rp := pa.Post.Responses.StatusCodeResponses[200]
	var hdr spec.Header
	hdr.Type = "array"
	rp.Headers = make(map[string]spec.Header)
	rp.Headers["X-YADA"] = hdr
	pa.Post.Responses.StatusCodeResponses[200] = rp
	res = validator.validateItems()
	assert.NotEmpty(t, res.Errors)

	// in response schema
	doc, _ = loads.Analyzed(PetStoreJSONMessage, "")
	validator = NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
	validator.spec = doc
	validator.analyzer = analysis.New(doc.Spec())
	sw = doc.Spec()
	pa = sw.Paths.Paths["/pets"]
	rp = pa.Post.Responses.StatusCodeResponses[200]
	rp.Schema = spec.ArrayProperty(nil)
	pa.Post.Responses.StatusCodeResponses[200] = rp
	res = validator.validateItems()
	assert.NotEmpty(t, res.Errors)
}

type expectedMessage struct {
	message              string
	withContinueOnErrors bool
}

type expectedFixture struct {
	expectedLoadError bool
	expectedValid     bool
	expectedMessages  []expectedMessage
	expectedWarnings  []expectedMessage
}

type expectedMap map[string]expectedFixture

// TODO: test isse#1050
func Test_Issue1050(t *testing.T) {
}

// Test message improvements, issue #859
func Test_MessageQuality_Issue859(t *testing.T) {
	SetContinueOnErrors(true)
	defer func() {
		// TODO : false
		SetContinueOnErrors(true)
	}()

	tested := expectedMap{
		"fixture-1171.yaml": expectedFixture{
			expectedLoadError: false,
			expectedValid:     false,
			expectedWarnings:  []expectedMessage{},
			expectedMessages: []expectedMessage{
				/*
					spec_test.go:926: definitions.InvalidZone.items.name in body is a forbidden property
					spec_test.go:926: "paths./servers/{server_id}/zones.get.parameters" must validate one and only one schema (oneOf). Found none valid
					spec_test.go:926: "paths./server/getBody.get.parameters" must validate one and only one schema (oneOf). Found none valid
					spec_test.go:926: "paths./server/getBody.get.responses.200" must validate one and only one schema (oneOf). Found none valid
					spec_test.go:926: "paths./server/getBody.get.responses.200.schema" must validate one and only one schema (oneOf). Found none valid
					spec_test.go:926: paths./server/getBody.get.responses.200.schema.properties.name in body must be of type object: "string"
					spec_test.go:926: paths./server/getBody.get.responses.200.schema.properties.$ref in body must be of type object: "string"
					spec_test.go:926: paths./server/getBody.get.responses.200.description in body is required
					spec_test.go:926: in path "/servers/{server_id}/zones/{zone_id}", param ""{server_id}"" contains {,} or white space. Albeit not stricly illegal, this is probably no what you want
					spec_test.go:926: in path "/servers/{server_id}/zones/{zone_id}", param ""{zone_id}"" contains {,} or white space. Albeit not stricly illegal, this is probably no what you want
					spec_test.go:926: in path "/servers/{server_id}/zones", param ""{server_id}"" contains {,} or white space. Albeit not stricly illegal, this is probably no what you want
				*/
				// TODO: this message is cryptic in our context
				expectedMessage{"\"paths./servers/{server_id}/zones.get.parameters\" must validate one and only one schema (oneOf). Found none valid", false},
				expectedMessage{"\"paths./server/getBody.get.parameters\" must validate one and only one schema (oneOf). Found none valid", false},
				expectedMessage{"\"paths./server/getBody.get.responses.200\" must validate one and only one schema (oneOf). Found none valid", false},
				expectedMessage{"\"paths./server/getBody.get.responses.200.schema\" must validate one and only one schema (oneOf). Found none valid", false},
				// TODO: add found "string"
				expectedMessage{"paths./server/getBody.get.responses.200.schema.properties.name in body must be of type object: \"string\"", false},
				expectedMessage{"paths./server/getBody.get.responses.200.schema.properties.$ref in body must be of type object: \"string\"", false},
				expectedMessage{"paths./server/getBody.get.responses.200.description in body is required", false},
				expectedMessage{"items in definitions.Zones is required", false},
				expectedMessage{"\"definitions.InvalidZone.items\" must validate at least one schema (anyOf)", false},
				expectedMessage{"definitions.InvalidZone.items.name in body is a forbidden property", false},
				expectedMessage{"path param \"other_server_id\" is not present in path \"/servers/{server_id}/zones\"", false},
				expectedMessage{"operation \"getBody\" has more than 1 body param (accepted: \"yet_other_server_id\", dropped: \"\")", false},
				expectedMessage{"body param \"yet_other_server_id\" for \"getBody\" is a collection without an element type (array requires items definition)", false},
				expectedMessage{"in operation \"listZones\",path param \"other_server_id\" must be declared as required", false},
				// Only when continue on errors true
				// DONE: suppress this messages which is unstable
				// expectedMessage{"response for operation \"getBody\" has no valid status code section", true},
				// TODO: missing here $ref sibling constraint
			},
		},
		"fixture-1238.yaml": expectedFixture{
			expectedLoadError: false,
			expectedValid:     false,
			expectedWarnings:  []expectedMessage{},
			expectedMessages: []expectedMessage{
				expectedMessage{"definitions.RRSets in body must be of type array", false},
			},
		},
		"fixture-1243.yaml": expectedFixture{
			expectedLoadError: false,
			expectedValid:     false,
			expectedWarnings:  []expectedMessage{},
			expectedMessages: []expectedMessage{
				expectedMessage{"\"paths./loadBalancers/{loadBalancerId}/backendSets.get.responses.200\" must validate one and only one schema (oneOf). Found none valid", false},
				expectedMessage{"paths./loadBalancers/{loadBalancerId}/backendSets.get.responses.200.headers.opc-response-id.$ref in body is a forbidden property", false},
				expectedMessage{"paths./loadBalancers/{loadBalancerId}/backendSets.get.responses.200.headers.opc-response-id.type in body is required", false},
				expectedMessage{"path param \"{loadBalancerId}\" has no parameter definition", false},
			},
		},
		// Load error: incomplete JSON into a yaml file ...
		"fixture-1289-donotload.yaml": expectedFixture{
			expectedLoadError: true,
			expectedValid:     false,
			expectedWarnings:  []expectedMessage{},
			expectedMessages: []expectedMessage{
				expectedMessage{"yaml: line 15: did not find expected key", false},
			},
		},
		// Load error: incomplete JSON ...
		"fixture-1289-donotload.json": expectedFixture{
			expectedLoadError: true,
			expectedValid:     false,
			expectedWarnings:  []expectedMessage{},
			expectedMessages: []expectedMessage{
				// Since this error is provided by an external package (go-openapi/loads, only asserts the surface of things)
				expectedMessage{"yaml:", false},
			},
		},
		"fixture-1289.yaml": expectedFixture{
			expectedLoadError: false,
			expectedValid:     false,
			expectedWarnings:  []expectedMessage{},
			expectedMessages: []expectedMessage{
				//TODO: more explicit message (warning about siblings)
				expectedMessage{"items in definitions.getSomeIds.properties.someIds is required", false},
			},
		},
		"fixture-1289-good.yaml": expectedFixture{
			expectedLoadError: false,
			expectedValid:     true,
			expectedWarnings:  []expectedMessage{},
			expectedMessages:  []expectedMessage{},
		},
	}

	filepath.Walk(filepath.Join("fixtures", "validation"),
		func(path string, info os.FileInfo, err error) error {
			if !info.IsDir() && len(tested[info.Name()].expectedMessages) > 0 {
				t.Logf("Testing messages for spec=%s", path)
				errs := 0
				doc, err := loads.Spec(path)
				if tested[info.Name()].expectedLoadError == true {
					// Expect a load error: no further validation may possibly be conducted.
					// Process here error messages from loads (normally unit tested in the load package:
					// we just want to figure out how all this is captured at the validate package level.
					assert.Error(t, err)
					for _, m := range tested[info.Name()].expectedMessages {
						assert.Contains(t, err.Error(), m.message)
					}
					return nil
				}
				if assert.NoError(t, err) {
					validator := NewSpecValidator(doc.Schema(), strfmt.Default)
					// TODO: Assert the warning part as well
					res, _ := validator.Validate(doc)
					// Expect all submitted specs to be invalid
					if !assert.False(t, res.IsValid()) {
						errs++
					}

					var verifiedErrors []string
					for _, e := range res.Errors {
						verifiedErrors = append(verifiedErrors, e.Error())
					}
					// We got the expected number of messages (e.g. no duplicates, no uncontrolled side-effect, ...)
					if !assert.Len(t, verifiedErrors, len(tested[info.Name()].expectedMessages), "Unexpected number of error messages returned. Wanted %d, got %d", len(tested[info.Name()].expectedMessages), len(verifiedErrors)) {
						errs++
					}
					// All expected messages are here
					for _, v := range tested[info.Name()].expectedMessages {
						// Certain additional messages are only expected when continueOnErrors is true
						if (v.withContinueOnErrors == true && continueOnErrors == true) || v.withContinueOnErrors == false {
							if !assert.Contains(t, verifiedErrors, v.message, "Missing expected message: %s", v.message) {
								errs++
							}
						}
					}
					// No unexpected message
					expectedList := []string{}
					for _, s := range tested[info.Name()].expectedMessages {
						if (s.withContinueOnErrors == true && continueOnErrors == true) || s.withContinueOnErrors == false {
							expectedList = append(expectedList, s.message)
						}
					}
					if !assert.Subset(t, verifiedErrors, expectedList, "Some unexpected messages where reported") {
						errs++
						// Report unexpected messages
						for _, e := range verifiedErrors {
							found := false
							for _, v := range expectedList {
								if e == v {
									found = true
									break
								}
							}
							if !found {
								t.Logf("Unexpected message: %s", e)
							}
						}
					}
					if DebugTest && errs > 0 {
						t.Logf("DEVMODE:Returned error messages validating %s ", path)
						for _, v := range verifiedErrors {
							t.Logf("%s", v)
						}
					}
					// TODO: assert warnings
				} else {
					errs++
				}
				if errs > 0 {
					t.Logf("Spec validation for %s returned unexpected messages", path)
				}
			} else {
				// Expecting no message (e.g.valid spec): 0 message expected
				// Non configured fixtures are skipped (TODO: apply all)
				if !info.IsDir() && tested[info.Name()].expectedValid {
					t.Logf("Testing valid spec=%s", path)
					doc, err := loads.Spec(path)
					if assert.NoError(t, err) {
						validator := NewSpecValidator(doc.Schema(), strfmt.Default)
						res, _ := validator.Validate(doc)
						assert.True(t, res.IsValid())
						assert.Len(t, res.Errors, 0)
					}
				}
			}
			return nil
		})
}
