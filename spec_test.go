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
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/go-openapi/analysis"
	"github.com/go-openapi/loads"
	"github.com/go-openapi/loads/fmts"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testID = "id"

// Enable long running tests by using cmd line arg,
// Usage: go test ... -args [-enable-long|-enable-go-swagger]
//
// -enable-long:       enable spec_test.go:TestIssue18 and messages_test.go:Test_Quality*
// -enable-go-swagger: enable non-regression tests against go-swagger fixtures (validation status) in swagger_test.go:Test_GoSwagger  (running about 110 specs...)
//
// If none enabled, these tests are skipped
// NOTE: replacing with go test -short and testing.Short() means that
// by default, every test is launched. With -enable-long, we just get the
// opposite...
var enableLongTests bool
var enableGoSwaggerTests bool

func init() {
	loads.AddLoader(fmts.YAMLMatcher, fmts.YAMLDoc)
	flag.BoolVar(&enableLongTests, "enable-long", false, "enable long runnning tests")
	flag.BoolVar(&enableGoSwaggerTests, "enable-go-swagger", false, "enable go-swagger non-regression test")
}

func skipNotify(t *testing.T) {
	t.Log("To enable this long running test, use -args -enable-long in your go test command line")
}

func debugTest(t *testing.T, path string, res *Result) {
	if DebugTest && t.Failed() {
		verifiedErrors := verifiedTestErrors(res)
		if len(verifiedErrors) > 0 {
			t.Logf("DEVMODE:Returned error messages validating %s ", path)
			for _, v := range verifiedErrors {
				t.Logf("%s", v)
			}
		}
		verifiedWarnings := verifiedTestWarnings(res)
		if len(verifiedWarnings) > 0 {
			t.Logf("DEVMODE: Returned warnings for %s:", path)
			for _, e := range res.Warnings {
				t.Logf("%v", e)
			}
		}
	}
}

func verifiedTestErrors(res *Result) []string {
	verifiedErrors := make([]string, 0, 50)
	for _, e := range res.Errors {
		verifiedErrors = append(verifiedErrors, e.Error())
	}
	return verifiedErrors
}

func verifiedTestWarnings(res *Result) []string {
	verifiedWarnings := make([]string, 0, 50)
	for _, e := range res.Warnings {
		verifiedWarnings = append(verifiedWarnings, e.Error())
	}
	return verifiedWarnings
}

func TestSpec_ExpandResponseLocalFile(t *testing.T) {
	res, _ := loadAndValidate(t, filepath.Join("fixtures", "local_expansion", "spec.yaml"))
	assert.True(t, res.IsValid())
	assert.Empty(t, res.Errors)
}

func TestSpec_ExpandResponseRecursive(t *testing.T) {
	res, _ := loadAndValidate(t, filepath.Join("fixtures", "recursive_expansion", "spec.yaml"))
	assert.True(t, res.IsValid())
	assert.Empty(t, res.Errors)
}

// Spec with no path
func TestSpec_Issue52(t *testing.T) {
	fp := filepath.Join("fixtures", "bugs", "52", "swagger.json")
	jstext, _ := os.ReadFile(fp)

	// as json schema
	var sch spec.Schema
	require.NoError(t, json.Unmarshal(jstext, &sch))

	schemaValidator := NewSchemaValidator(spec.MustLoadSwagger20Schema(), nil, "", strfmt.Default)
	res := schemaValidator.Validate(&sch)
	assert.False(t, res.IsValid())
	require.NotEmpty(t, res.Errors)
	require.EqualError(t, res.Errors[0], ".paths in body is required")

	// as swagger spec: path is set to nil
	// Here, validation stops as paths is initialized to empty
	res, _ = loadAndValidate(t, fp)
	assert.False(t, res.IsValid())

	verifiedErrors := verifiedTestErrors(res)
	assert.Len(t, verifiedErrors, 2, "Unexpected number of error messages returned")
	assert.Contains(t, verifiedErrors, ".paths in body is required")
	assert.Contains(t, verifiedErrors, "spec has no valid path defined")
}

func TestSpec_Issue53(t *testing.T) {
	fp := filepath.Join("fixtures", "bugs", "53", "noswagger.json")
	jstext, _ := os.ReadFile(fp)

	// as json schema
	var sch spec.Schema
	require.NoError(t, json.Unmarshal(jstext, &sch))

	schemaValidator := NewSchemaValidator(spec.MustLoadSwagger20Schema(), nil, "", strfmt.Default)
	res := schemaValidator.Validate(&sch)
	assert.False(t, res.IsValid())
	require.NotEmpty(t, res.Errors)
	require.EqualError(t, res.Errors[0], ".swagger in body is required")

	// as swagger despec
	res, _ = loadAndValidate(t, fp, false)
	require.False(t, res.IsValid())
	require.NotEmpty(t, res.Errors)
	require.EqualError(t, res.Errors[0], ".swagger in body is required")
}

func TestSpec_Issue62(t *testing.T) {
	fp := filepath.Join("fixtures", "bugs", "62", "swagger.json")

	// as swagger spec
	doc, err := loads.Spec(fp)
	require.NoError(t, err)

	validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
	res, _ := validator.Validate(doc)
	assert.NotEmpty(t, res.Errors)
	assert.True(t, res.HasErrors())
}

func TestSpec_Issue63(t *testing.T) {
	res, _ := loadAndValidate(t, filepath.Join("fixtures", "bugs", "63", "swagger.json"))
	assert.True(t, res.IsValid())
}

func TestSpec_Issue61_MultipleRefs(t *testing.T) {
	res, _ := loadAndValidate(t, filepath.Join("fixtures", "bugs", "61", "multiple-refs.json"))
	assert.Empty(t, res.Errors)
	assert.True(t, res.IsValid())
}

func TestSpec_Issue61_ResolvedRef(t *testing.T) {
	res, _ := loadAndValidate(t, filepath.Join("fixtures", "bugs", "61", "unresolved-ref-for-name.json"))
	assert.Empty(t, res.Errors)
	assert.True(t, res.IsValid())
}

// No error with this one
func TestSpec_Issue123(t *testing.T) {
	fp := filepath.Join("fixtures", "bugs", "123", "swagger.yml")
	res, _ := loadAndValidate(t, fp)
	assert.True(t, res.IsValid())
	assert.Empty(t, res.Errors)

	debugTest(t, fp, res)
}

func TestSpec_Issue6(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("fixtures", "bugs", "6", "*.json"))
	for _, path := range files {
		t.Logf("Tested spec=%s", path)
		res, _ := loadAndValidate(t, path)
		assert.False(t, res.IsValid())

		verifiedErrors := verifiedTestErrors(res)
		switch {
		case strings.Contains(path, "empty-responses.json"):
			assert.Contains(t, verifiedErrors, "\"paths./foo.get.responses\" must not validate the schema (not)")
			assert.Contains(t, verifiedErrors, "paths./foo.get.responses in body should have at least 1 properties")
		case strings.Contains(path, "no-responses.json"):
			assert.Contains(t, verifiedErrors, "paths./foo.get.responses in body is required")
		default:
			t.Logf("Returned error messages: %v", verifiedErrors)
			t.Fatal("fixture not tested. Please add assertions for messages")
		}

		debugTest(t, path, res)
	}
}

// check if invalid patterns are indeed invalidated
func TestSpec_Issue18(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("fixtures", "bugs", "18", "*.json"))
	for _, path := range files {
		t.Logf("Tested spec=%s", path)
		res, _ := loadAndValidate(t, path)
		assert.False(t, res.IsValid())

		verifiedErrors := verifiedTestErrors(res)
		switch {
		case strings.Contains(path, "headerItems.json"):
			assert.Contains(t, verifiedErrors, "X-Foo in header has invalid pattern: \")<-- bad pattern\"")
		case strings.Contains(path, "headers.json"):
			assert.Contains(t, verifiedErrors, "in operation \"\", header X-Foo for default response has invalid pattern \")<-- bad pattern\": error parsing regexp: unexpected ): `)<-- bad pattern`")
			//  in operation \"\", header X-Foo for default response has invalid pattern \")<-- bad pattern\": error parsing regexp: unexpected ): `)<-- bad pattern`
			assert.Contains(t, verifiedErrors, "in operation \"\", header X-Foo for response 402 has invalid pattern \")<-- bad pattern\": error parsing regexp: unexpected ): `)<-- bad pattern`")
			//  in operation "", header X-Foo for response 402 has invalid pattern ")<-- bad pattern": error parsing regexp: unexpected ): `)<-- bad pattern`

		case strings.Contains(path, "paramItems.json"):
			assert.Contains(t, verifiedErrors, "body param \"user\" for \"\" has invalid items pattern: \")<-- bad pattern\"")
			// Updated message: from "user.items in body has invalid pattern: \")<-- bad pattern\"" to:
			assert.Contains(t, verifiedErrors, "default value for user in body does not validate its schema")
			assert.Contains(t, verifiedErrors, "user.items.default in body has invalid pattern: \")<-- bad pattern\"")
		case strings.Contains(path, "parameters.json"):
			assert.Contains(t, verifiedErrors, "operation \"\" has invalid pattern in param \"userId\": \")<-- bad pattern\"")
		case strings.Contains(path, "schema.json"):
			// TODO: strange that the text does not say response "200"...
			assert.Contains(t, verifiedErrors, "200 in response has invalid pattern: \")<-- bad pattern\"")
		default:
			t.Logf("Returned error messages: %v", verifiedErrors)
			t.Fatal("fixture not tested. Please add assertions for messages")
		}

		debugTest(t, path, res)
	}
}

// check if a fragment path parameter is recognized, without error
func TestSpec_Issue39(t *testing.T) {
	fp := filepath.Join("fixtures", "bugs", "39", "swagger.yml")
	res, _ := loadAndValidate(t, fp)
	assert.True(t, res.IsValid())
	assert.Empty(t, res.Errors)
	debugTest(t, fp, res)
}

func TestSpec_ValidateDuplicatePropertyNames(t *testing.T) {
	// simple allOf
	doc, err := loads.Spec(filepath.Join("fixtures", "validation", "duplicateprops.json"))
	require.NoError(t, err)

	validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
	validator.spec = doc
	res := validator.validateDuplicatePropertyNames()
	assert.NotEmpty(t, res.Errors)
	assert.Len(t, res.Errors, 1)

	// nested allOf
	doc, err = loads.Spec(filepath.Join("fixtures", "validation", "nestedduplicateprops.json"))
	require.NoError(t, err)

	validator = NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
	validator.spec = doc
	res = validator.validateDuplicatePropertyNames()
	assert.NotEmpty(t, res.Errors)
	assert.Len(t, res.Errors, 1)
}

func TestSpec_ValidateNonEmptyPathParameterNames(t *testing.T) {
	doc, err := loads.Spec(filepath.Join("fixtures", "validation", "empty-path-param-name.json"))
	require.NoError(t, err)

	validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
	validator.spec = doc
	res := validator.validateNonEmptyPathParamNames()
	assert.NotEmpty(t, res.Errors)
	assert.Len(t, res.Errors, 1)
}

func TestSpec_ValidateCircularAncestry(t *testing.T) {
	doc, err := loads.Spec(filepath.Join("fixtures", "validation", "direct-circular-ancestor.json"))
	require.NoError(t, err)

	validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
	validator.spec = doc
	res := validator.validateDuplicatePropertyNames()
	assert.NotEmpty(t, res.Errors)
	assert.Len(t, res.Errors, 1)

	doc, err = loads.Spec(filepath.Join("fixtures", "validation", "indirect-circular-ancestor.json"))
	require.NoError(t, err)

	validator = NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
	validator.spec = doc
	res = validator.validateDuplicatePropertyNames()
	assert.NotEmpty(t, res.Errors)
	assert.Len(t, res.Errors, 1)

	doc, err = loads.Spec(filepath.Join("fixtures", "validation", "recursive-circular-ancestor.json"))
	require.NoError(t, err)

	validator = NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
	validator.spec = doc
	res = validator.validateDuplicatePropertyNames()
	assert.NotEmpty(t, res.Errors)
	assert.Len(t, res.Errors, 1)
}

func TestSpec_ValidateReferenced(t *testing.T) {
	doc, err := loads.Spec(filepath.Join("fixtures", "validation", "valid-referenced.yml"))
	require.NoError(t, err)

	validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
	validator.spec = doc
	validator.analyzer = analysis.New(doc.Spec())
	res := validator.validateReferenced()
	assert.Empty(t, res.Errors)

	doc, err = loads.Spec(filepath.Join("fixtures", "validation", "invalid-referenced.yml"))
	require.NoError(t, err)

	validator = NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
	validator.spec = doc
	validator.analyzer = analysis.New(doc.Spec())
	res = validator.validateReferenced()
	assert.Empty(t, res.Errors)
	assert.NotEmpty(t, res.Warnings)
	assert.Len(t, res.Warnings, 3)
}

func TestSpec_ValidateReferencesValid(t *testing.T) {
	doc, err := loads.Spec(filepath.Join("fixtures", "validation", "valid-ref.json"))
	require.NoError(t, err)

	validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
	validator.spec = doc
	validator.analyzer = analysis.New(doc.Spec())
	res := validator.validateReferencesValid()
	assert.Empty(t, res.Errors)

	doc, err = loads.Spec(filepath.Join("fixtures", "validation", "invalid-ref.json"))
	require.NoError(t, err)

	validator = NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
	validator.spec = doc
	validator.analyzer = analysis.New(doc.Spec())
	res = validator.validateReferencesValid()
	assert.NotEmpty(t, res.Errors)
}

func TestSpec_ValidateRequiredDefinitions(t *testing.T) {
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

func TestSpec_ValidateParameters(t *testing.T) {
	validatorForDoc := func(doc *loads.Document) *SpecValidator {
		// build a spec validator for some doc
		validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
		validator.spec = doc
		validator.analyzer = analysis.New(doc.Spec())

		return validator
	}

	t.Run("should validate classic PetStore", func(t *testing.T) {
		doc, err := loads.Analyzed(PetStoreJSONMessage, "")
		require.NoError(t, err)
		validator := validatorForDoc(doc)

		res := validator.validateParameters()
		require.Empty(t, res.Errors)
	})

	t.Run("should detect duplicate parameters", func(t *testing.T) {
		doc, err := loads.Analyzed(PetStoreJSONMessage, "")
		require.NoError(t, err)

		sw := doc.Spec()
		sw.Paths.Paths["/pets"].Get.Parameters = append(sw.Paths.Paths["/pets"].Get.Parameters, *spec.QueryParam("limit").Typed(stringType, ""))
		validator := validatorForDoc(doc)

		res := validator.validateParameters()
		require.NotEmpty(t, res.Errors)
		assert.Contains(t, res.Errors[0].Error(),
			`duplicate parameter name "limit" for "query" in operation "getAllPets"`,
		)
	})

	t.Run("should detect multiple parameters in body", func(t *testing.T) {
		doc, err := loads.Analyzed(PetStoreJSONMessage, "")
		require.NoError(t, err)

		sw := doc.Spec()
		sw.Paths.Paths["/pets"].Post.Parameters = append(sw.Paths.Paths["/pets"].Post.Parameters, *spec.BodyParam("fake", spec.RefProperty("#/definitions/Pet")))
		validator := validatorForDoc(doc)

		res := validator.validateParameters()
		assert.NotEmpty(t, res.Errors)
		require.Len(t, res.Errors, 1)
		assert.Contains(t, res.Errors[0].Error(), "has more than 1 body param")
	})

	t.Run("should detect invalid parameter schema in (modified) classic PetStore", func(t *testing.T) {
		fixture := filepath.Join("fixtures", "petstore", "swagger-invalid.json")

		t.Run("with raw JSON", func(t *testing.T) {
			// loading with full root document
			jazon, err := os.ReadFile(fixture)
			require.NoError(t, err)
			doc, err := loads.Analyzed(jazon, "")
			require.NoError(t, err)
			validator := validatorForDoc(doc)

			res := validator.validateParameters()
			require.Len(t, res.Errors, 2)
			assert.Contains(t, res.Errors[0].Error(),
				`"/pets.POST.parameters.pet" must validate one and only one schema (oneOf). Found none valid`,
			)
			assert.Contains(t, res.Errors[1].Error(),
				`/pets.POST.parameters.pet.schema.anyOf in body is a forbidden property`,
			)
		})
		t.Run("with loads.Spec", func(t *testing.T) {
			// loading like a regular user of this library
			doc, err := loads.Spec(fixture)
			require.NoError(t, err)

			err = Spec(doc, strfmt.Default)
			require.Error(t, err)
			require.ErrorContains(t, err,
				"definitions.newPet.anyOf in body is a forbidden property",
			)
		})

		t.Run("with invalid Swagger schema", func(t *testing.T) {
			doc, err := loads.Analyzed(PetStoreJSONMessage, "")
			require.NoError(t, err)
			validator := validatorForDoc(doc)
			delete(validator.schema.Definitions, "parameter")

			require.Panics(t, func() {
				_ = validator.validateParameters()
			})
		})
	})

	t.Run("should detect duplicate parameters", func(t *testing.T) {
		doc, err := loads.Analyzed(PetStoreJSONMessage, "")
		require.NoError(t, err)

		sw := doc.Spec()
		pp := sw.Paths.Paths["/pets/{id}"]
		pp.Delete = nil
		var nameParams []spec.Parameter
		for _, p := range pp.Parameters {
			if p.Name == testID {
				p.Name = "name"
				nameParams = append(nameParams, p)
			}
		}
		pp.Parameters = nameParams
		sw.Paths.Paths["/pets/{name}"] = pp
		validator := validatorForDoc(doc)

		res := validator.validateParameters()
		assert.NotEmpty(t, res.Errors)
		require.Len(t, res.Errors, 1)
		assert.Contains(t, res.Errors[0].Error(), "overlaps with")

		t.Run("should tolerate duplicate parameters, on option", func(t *testing.T) {
			// Disable strict path param uniqueness and ensure there is no error
			validator.Options.StrictPathParamUniqueness = false
			res := validator.validateParameters()
			require.Empty(t, res.Errors)
		})
	})

	t.Run("should detect mismatch with path parameter", func(t *testing.T) {
		doc, err := loads.Analyzed(PetStoreJSONMessage, "")
		require.NoError(t, err)

		sw := doc.Spec()
		pp := sw.Paths.Paths["/pets/{id}"]
		pp.Delete = nil
		var nameParams []spec.Parameter
		for _, p := range pp.Parameters {
			if p.Name == testID {
				p.Name = "name"
				nameParams = append(nameParams, p)
			}
		}
		pp.Get.Parameters = nameParams
		pp.Parameters = nil
		sw.Paths.Paths["/pets/{id}"] = pp
		validator := validatorForDoc(doc)

		res := validator.validateParameters()
		require.NotEmpty(t, res.Errors)
		require.Len(t, res.Errors, 2)
		assert.Contains(t, res.Errors[1].Error(),
			`is not present in path "/pets/{id}"`,
		)
		assert.Contains(t, res.Errors[0].Error(),
			"has no parameter definition",
		)
	})

	t.Run("with issue go-swagger/go-swagger#2527", func(t *testing.T) {
		basePath := filepath.Join("fixtures", "bugs", "2527")

		t.Run("should detect mismatch between parameter and schema", func(t *testing.T) {
			doc, err := loads.Spec(filepath.Join(basePath, "swagger.yml"))
			require.NoError(t, err)

			err = Spec(doc, strfmt.Default)
			require.Error(t, err)
			require.ErrorContains(t, err,
				`/deposits.GET.parameters..enum in body is a forbidden property`,
			)
			require.ErrorContains(t, err,
				`deposits.GET.parameters..type in body is a forbidden property`,
			)
			require.ErrorContains(t, err,
				`/deposits.GET.parameters..name in body is required`,
			)
			require.ErrorContains(t, err,
				`/deposits.GET.parameters..in in body is required`,
			)
		})

		t.Run("should validate fixed spec", func(t *testing.T) {
			doc, err := loads.Spec(filepath.Join(basePath, "swagger-fixed.yml"))
			require.NoError(t, err)

			require.NoError(t, Spec(doc, strfmt.Default))
		})

		t.Run("should detect missing name and in from refed parameter", func(t *testing.T) {
			doc, err := loads.Spec(filepath.Join(basePath, "swagger-other.yml"))
			require.NoError(t, err)

			err = Spec(doc, strfmt.Default)
			require.ErrorContains(t, err,
				`"parameters.missingName" must validate one and only one schema (oneOf). Found none valid`,
			)
			require.ErrorContains(t, err,
				`parameters.missingName.name in body is required`,
			)
			require.ErrorContains(t, err,
				`"parameters.missingIn" must validate one and only one schema (oneOf). Found none valid`,
			)
			require.ErrorContains(t, err,
				`parameters.missingIn.in in body is required`,
			)
		})

		t.Run("extra parameter JSONSchema validation should not result in duplicate errors", func(t *testing.T) {
			t.Run("with spec validator", func(t *testing.T) {
				doc, err := loads.Spec(filepath.Join(basePath, "swagger-schema-error.yml"))
				require.NoError(t, err)

				errs, warns := NewSpecValidator(doc.Schema(), strfmt.Default).Validate(doc)
				require.Len(t, errs.Errors, 3)
				require.Empty(t, warns.Errors)

				var found1, found2, found3 int
				for _, err := range errs.Errors {
					switch {
					case strings.Contains(err.Error(), `definitions.WrongSchema.descriptions in body is a forbidden property`):
						found1++
					case strings.Contains(err.Error(), `"definitions.WrongSchema.type" must validate at least one schema (anyOf)`):
						found2++
					case strings.Contains(err.Error(), `definitions.WrongSchema.type in body should be one of [array boolean integer null number object string]`):
						found3++
					}
				}

				t.Run("each message should appear exactly once", func(t *testing.T) {
					require.Equal(t, 1, found1)
					require.Equal(t, 1, found2)
					require.Equal(t, 1, found3)
				})
			})
		})
	})
}

func TestSpec_ValidateItems(t *testing.T) {
	doc, _ := loads.Analyzed(PetStoreJSONMessage, "")
	validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
	validator.spec = doc
	validator.analyzer = analysis.New(doc.Spec())
	res := validator.validateItems()
	assert.Empty(t, res.Errors)

	// in operation parameters
	sw := doc.Spec()
	sw.Paths.Paths["/pets"].Get.Parameters[0].Type = arrayType
	res = validator.validateItems()
	assert.NotEmpty(t, res.Errors)

	sw.Paths.Paths["/pets"].Get.Parameters[0].Items = spec.NewItems().Typed(stringType, "")
	res = validator.validateItems()
	assert.Empty(t, res.Errors)

	sw.Paths.Paths["/pets"].Get.Parameters[0].Items = spec.NewItems().Typed(arrayType, "")
	res = validator.validateItems()
	assert.NotEmpty(t, res.Errors)

	sw.Paths.Paths["/pets"].Get.Parameters[0].Items.Items = spec.NewItems().Typed(stringType, "")
	res = validator.validateItems()
	assert.Empty(t, res.Errors)

	// in global parameters
	sw.Parameters = make(map[string]spec.Parameter)
	sw.Parameters["other"] = *spec.SimpleArrayParam("other", arrayType, "csv")
	res = validator.validateItems()
	assert.Empty(t, res.Errors)

	// pp := spec.SimpleArrayParam("other", arrayType, "")
	// pp.Items = nil
	// sw.Parameters["other"] = *pp
	// res = validator.validateItems()
	// assert.NotEmpty(t, res.Errors)

	// in shared path object parameters
	doc, _ = loads.Analyzed(PetStoreJSONMessage, "")
	validator = NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
	validator.spec = doc
	validator.analyzer = analysis.New(doc.Spec())
	sw = doc.Spec()

	pa := sw.Paths.Paths["/pets"]
	pa.Parameters = []spec.Parameter{*spec.SimpleArrayParam("another", arrayType, "csv")}
	sw.Paths.Paths["/pets"] = pa
	res = validator.validateItems()
	assert.Empty(t, res.Errors)

	pa = sw.Paths.Paths["/pets"]
	pp := spec.SimpleArrayParam("other", arrayType, "")
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
	hdr.Type = arrayType
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

// Reuse known validated cases through the higher level Spec() call
func TestSpec_ValidDoc(t *testing.T) {
	fp := filepath.Join("fixtures", "local_expansion", "spec.yaml")
	doc2, err := loads.Spec(fp)
	require.NoError(t, err)
	err = Spec(doc2, strfmt.Default)
	require.NoError(t, err)
}

// Check higher level behavior on invalid spec doc
func TestSpec_InvalidDoc(t *testing.T) {
	doc, err := loads.Spec(filepath.Join("fixtures", "validation", "default", "invalid-default-value-parameter.json"))
	require.NoError(t, err)
	err = Spec(doc, strfmt.Default)
	require.Error(t, err)
}

func TestSpec_Validate_InvalidInterface(t *testing.T) {
	fp := filepath.Join("fixtures", "local_expansion", "spec.yaml")
	doc2, err := loads.Spec(fp)
	require.NoError(t, err)
	require.NotNil(t, doc2)

	validator := NewSpecValidator(doc2.Schema(), strfmt.Default)
	bug := "bzzz"
	res, _ := validator.Validate(bug)
	assert.NotEmpty(t, res.Errors)
	assert.Contains(t, res.Errors[0].Error(), "can only validate spec.Document objects")
}

func TestSpec_ValidateBodyFormDataParams(t *testing.T) {
	res, _ := loadAndValidate(t, filepath.Join("fixtures", "validation", "invalid-formdata-body-params.json"))
	assert.NotEmpty(t, res.Errors)
	assert.Len(t, res.Errors, 1)
}

func TestSpec_Issue73(t *testing.T) {
	res, _ := loadAndValidate(t, filepath.Join("fixtures", "bugs", "73", "fixture-swagger.yaml"))
	assert.Empty(t, res.Errors, " in fixture-swagger.yaml")

	res, _ = loadAndValidate(t, filepath.Join("fixtures", "bugs", "73", "fixture-swagger-2.yaml"))
	assert.Empty(t, res.Errors, "in fixture-swagger-2.yaml")

	res, _ = loadAndValidate(t, filepath.Join("fixtures", "bugs", "73", "fixture-swagger-3.yaml"))
	assert.Empty(t, res.Errors, "in fixture-swagger-3.yaml")

	res, _ = loadAndValidate(t, filepath.Join("fixtures", "bugs", "73", "fixture-swagger-good.yaml"))
	assert.Empty(t, res.Errors, " in fixture-swagger-good.yaml")
}

func TestSpec_Issue1341(t *testing.T) {
	// testing recursive walk with defaults and examples
	res, _ := loadAndValidate(t, filepath.Join("fixtures", "bugs", "1341", "fixture-1341-good.yaml"))
	assert.Empty(t, res.Errors, " in fixture-1341-good.yaml")
	assert.Len(t, res.Warnings, 1, " in fixture-1341-good.yaml")

	res, _ = loadAndValidate(t, filepath.Join("fixtures", "bugs", "1341", "fixture-1341.yaml"))
	assert.Empty(t, res.Errors, "in fixture-1341.yaml")
	assert.Empty(t, res.Warnings, "in fixture-1341.yaml")

	res, _ = loadAndValidate(t, filepath.Join("fixtures", "bugs", "1341", "fixture-1341-2.yaml"))
	assert.Empty(t, res.Errors, "in fixture-1341-2.yaml")
	assert.Empty(t, res.Warnings, "in fixture-1341-2.yaml")

	res, _ = loadAndValidate(t, filepath.Join("fixtures", "bugs", "1341", "fixture-1341-3.yaml"))
	assert.Empty(t, res.Errors, "in fixture-1341-3.yaml")
	assert.Len(t, res.Warnings, 4, "in fixture-1341-3.yaml")

	res, _ = loadAndValidate(t, filepath.Join("fixtures", "bugs", "1341", "fixture-1341-4.yaml"))
	assert.Empty(t, res.Errors, "in fixture-1341-4.yaml")
	assert.Empty(t, res.Warnings, "in fixture-1341-4.yaml")

	res, _ = loadAndValidate(t, filepath.Join("fixtures", "bugs", "1341", "fixture-1341-5.yaml"))
	assert.Len(t, res.Errors, 4, "in fixture-1341-5.yaml")
	assert.Empty(t, res.Warnings, "in fixture-1341-5.yaml")
}

// test go-swagger/go-swagger#1614 (circular refs)
func Test_Issue1614(t *testing.T) {
	path := filepath.Join("fixtures", "bugs", "1614", "gitea.json")
	testIssue(t, path, 0, 3)
}

// Test go-swagger/go-swagger#1621 (remote $ref)
func Test_Issue1621(t *testing.T) {
	path := filepath.Join("fixtures", "bugs", "1621", "fixture-1621.yaml")
	testIssue(t, path, 0, 0)
}

// Test go-swagger/go-swagger#1429 (remote $ref)
func Test_Issue1429(t *testing.T) {
	path := filepath.Join("fixtures", "bugs", "1429", "swagger.yaml")
	testIssue(t, path, 0, 0)
}

func TestSpec_ValidationTypeMismatch(t *testing.T) {
	doc, err := loads.Spec(filepath.Join("fixtures", "validation", "type-keyword-mismatch.yaml"))
	require.NoError(t, err)
	validator := NewSpecValidator(doc.Schema(), strfmt.Default)
	validator.spec = doc
	validator.analyzer = analysis.New(doc.Spec())
	res := validator.validateParameters()
	assert.NotEmpty(t, res.Warnings)
	assert.Len(t, res.Warnings, 3)

	warnings := verifiedTestWarnings(res)
	assert.Contains(t, warnings, `validation keywords of parameter "id" in path "/test/{id}/string" don't match its type string`)
	assert.Contains(t, warnings, `validation keywords of parameter "id" in path "/test/{id}/integer" don't match its type integer`)
	assert.Contains(t, warnings, `validation keywords of parameter "id" in path "/test/{id}/array" don't match its type array`)
}

func loadAndValidate(t testing.TB, fp string, early ...bool) (*Result, *Result) {
	doc, err := loads.Spec(fp)
	require.NoError(t, err)
	require.NotNil(t, doc)
	validator := NewSpecValidator(doc.Schema(), strfmt.Default)
	// for testing, we enable "ContinueOnErrors" by default
	if len(early) == 0 {
		validator.Options = Opts{ContinueOnErrors: true}
	} else {
		for _, flag := range early {
			validator.Options = Opts{ContinueOnErrors: flag}
		}
	}
	return validator.Validate(doc)
}

func TestItemsProperty_Issue43(t *testing.T) {
	for _, fixture := range []string{
		"fixture-43.yaml",
		"fixture-43-variants.yaml",
		"fixture-1456.yaml",
	} {
		fp := filepath.Join("fixtures", "bugs", "43", fixture)
		res, warnings := loadAndValidate(t, fp)
		assert.Truef(t, res.IsValid(), "expected spec from %s to be valid", fixture)
		assert.Emptyf(t, res.Errors, "expected no error in %s", fixture)
		assert.Emptyf(t, res.Warnings, "expected no warning in %s", fixture)
		assert.Emptyf(t, warnings, "expected no warning in %s", fixture)
	}

	fp := filepath.Join("fixtures", "bugs", "43", "fixture-43-fail.yaml")
	res, _ := loadAndValidate(t, fp)
	assert.Falsef(t, res.IsValid(), "expected spec to be invalid")
	assert.Greater(t, len(res.Errors), 3)

	fp = filepath.Join("fixtures", "validation", "fixture-1171.yaml")
	res, _ = loadAndValidate(t, fp)
	assert.Falsef(t, res.IsValid(), "expected spec to be invalid")
	assert.Greater(t, len(res.Errors), 3)
	found := false
	for _, e := range res.Errors {
		found = strings.Contains(e.Error(), "array requires items definition")
		if found {
			break
		}
	}
	assert.True(t, found)
}

func Test_Issue2137(t *testing.T) {
	fp := filepath.Join("fixtures", "bugs", "2137", "fixture-2137.yaml")
	res, _ := loadAndValidate(t, fp)
	assert.Falsef(t, res.IsValid(), "expected spec to be invalid")
	found := false
	for _, e := range res.Errors {
		found = strings.Contains(e.Error(), `"test" is defined 2 times`)
		if found {
			break
		}
	}
	assert.True(t, found)
}

func Test_Examples(t *testing.T) {
	fp := filepath.Join("fixtures", "bugs", "2649", "swagger.yaml")

	doc, err := loads.Spec(fp)
	require.NoError(t, err)
	require.NotNil(t, doc)

	validator := NewSpecValidator(doc.Schema(), strfmt.Default)
	validator.Options.SkipSchemataResult = true

	res, _ := validator.Validate(doc)
	if !assert.Truef(t, res.IsValid(), "expected spec to be valid") {
		spew.Dump(res.Errors)
	}
}

func Test_2866(t *testing.T) {
	// exercises fixture from go-swagger/go-swagger#2866, a test in go-swagger
	// that used to be problematic when using memory pools.

	fp := filepath.Join("fixtures", "bugs", "2866", "2866.yaml")

	doc, err := loads.Spec(fp)
	require.NoError(t, err)
	require.NotNil(t, doc)

	require.NoError(t, Spec(doc, strfmt.Default))
}
