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
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/go-openapi/analysis"
	"github.com/go-openapi/errors"
	"github.com/go-openapi/jsonpointer"
	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
)

var (
	// TODO: set default to false
	continueOnErrors = true
)

// SetContinueOnErrors ...
// For extended error reporting, it's better to pass all validations.
// For faster validation, it's better to give up early.
// SetContinueOnError(true) will set the validator to continue to the end of its checks.
func SetContinueOnErrors(c bool) {
	continueOnErrors = c
}

// Spec validates a spec document
// It validates the spec json against the json schema for swagger
// and then validates a number of extra rules that can't be expressed in json schema:
//
// 	- definition can't declare a property that's already defined by one of its ancestors
// 	- definition's ancestor can't be a descendant of the same model
// 	- path uniqueness: each api path should be non-verbatim (account for path param names) unique per method
// 	- each security reference should contain only unique scopes
// 	- each security scope in a security definition should be unique
//  - parameters in path must be unique
// 	- each path parameter must correspond to a parameter placeholder and vice versa
// 	- each referencable definition must have references
// 	- each definition property listed in the required array must be defined in the properties of the model
// 	- each parameter should have a unique `name` and `type` combination
// 	- each operation should have only 1 parameter of type body
// 	- each reference must point to a valid object
// 	- every default value that is specified must validate against the schema for that property
// 	- items property is required for all schemas/definitions of type `array`
//  - TODO as Warning: path parameters should not contain any of [{,},\w]
//  - TODO: $ref should not have siblings
//  - TODO: warnings id or $id
//  - path parameters must be declared a required
func Spec(doc *loads.Document, formats strfmt.Registry) error {
	errs, _ /*warns*/ := NewSpecValidator(doc.Schema(), formats).Validate(doc)
	if errs.HasErrors() {
		return errors.CompositeValidationError(errs.Errors...)
	}
	return nil
}

// AgainstSchema validates the specified data with the provided schema, when no schema
// is provided it uses the json schema as default
func AgainstSchema(schema *spec.Schema, data interface{}, formats strfmt.Registry) error {
	res := NewSchemaValidator(schema, nil, "", formats).Validate(data)
	if res.HasErrors() {
		return errors.CompositeValidationError(res.Errors...)
	}
	return nil
}

// SpecValidator validates a swagger spec
type SpecValidator struct {
	schema       *spec.Schema // swagger 2.0 schema
	spec         *loads.Document
	analyzer     *analysis.Spec
	expanded     *loads.Document
	KnownFormats strfmt.Registry
}

// NewSpecValidator creates a new swagger spec validator instance
func NewSpecValidator(schema *spec.Schema, formats strfmt.Registry) *SpecValidator {
	return &SpecValidator{
		schema:       schema,
		KnownFormats: formats,
	}
}

// Validate validates the swagger spec
func (s *SpecValidator) Validate(data interface{}) (errs *Result, warnings *Result) {
	var sd *loads.Document

	switch v := data.(type) {
	case *loads.Document:
		sd = v
	}
	if sd == nil {
		// TODO: should use a constant (from errors package?)
		errs = sErr(errors.New(500, "spec validator can only validate spec.Document objects"))
		return
	}
	s.spec = sd
	s.analyzer = analysis.New(sd.Spec())

	errs = new(Result)
	warnings = new(Result)

	schv := NewSchemaValidator(s.schema, nil, "", s.KnownFormats)
	var obj interface{}

	// Raw spec unmarshalling errors
	// TODO: some are already intercepted earlier with a log.Fatal
	if err := json.Unmarshal(sd.Raw(), &obj); err != nil {
		errs.AddErrors(err)
		return
	}

	errs.Merge(schv.Validate(obj)) // error -
	// There may be a point in continuing to try and determine more accurate errors
	if !continueOnErrors && errs.HasErrors() {
		return // no point in continuing
	}

	// TODO: in analyze, be more accurate about unresolved references
	errs.Merge(s.validateReferencesValid()) // error -
	// There may be a point in continuing to try and determine more accurate errors
	if !continueOnErrors && errs.HasErrors() {
		return // no point in continuing
	}

	errs.Merge(s.validateDuplicateOperationIDs())
	errs.Merge(s.validateDuplicatePropertyNames())         // error -
	errs.Merge(s.validateParameters())                     // error -
	errs.Merge(s.validateItems())                          // error -
	errs.Merge(s.validateRequiredDefinitions())            // error -
	errs.Merge(s.validateDefaultValueValidAgainstSchema()) // error -
	errs.Merge(s.validateExamplesValidAgainstSchema())     // error -
	errs.Merge(s.validateNonEmptyPathParamNames())

	warnings.Merge(s.validateRefNoSibling())         // warning
	warnings.Merge(s.validateUniqueSecurityScopes()) // warning
	warnings.Merge(s.validateReferenced())           // warning
	// TODO
	// warnings.Merge()
	return
}

func (s *SpecValidator) validateNonEmptyPathParamNames() *Result {
	res := new(Result)
	if s.spec.Spec().Paths == nil {
		res.AddErrors(errors.New(errors.CompositeErrorCode, "spec has no valid path defined"))
	} else {
		if s.spec.Spec().Paths.Paths == nil {
			res.AddErrors(errors.New(errors.CompositeErrorCode, "spec has no valid path defined"))
		} else {
			for k := range s.spec.Spec().Paths.Paths {
				if strings.Contains(k, "{}") {
					res.AddErrors(errors.New(errors.CompositeErrorCode, "%q contains an empty path parameter", k))
				}
			}

		}

	}
	return res
}

// TODO: there is a catch here. Duplicate operationId are not strictly forbidden, but
// not supported by go-swagger
func (s *SpecValidator) validateDuplicateOperationIDs() *Result {
	res := new(Result)
	known := make(map[string]int)
	for _, v := range s.analyzer.OperationIDs() {
		if v != "" {
			known[v]++
		}
	}
	for k, v := range known {
		if v > 1 {
			res.AddErrors(errors.New(errors.CompositeErrorCode, "%q is defined %d times", k, v))
		}
	}
	return res
}

type dupProp struct {
	Name       string
	Definition string
}

func (s *SpecValidator) validateDuplicatePropertyNames() *Result {
	// definition can't declare a property that's already defined by one of its ancestors
	res := new(Result)
	for k, sch := range s.spec.Spec().Definitions {
		if len(sch.AllOf) == 0 {
			continue
		}

		knownanc := map[string]struct{}{
			"#/definitions/" + k: struct{}{},
		}

		ancs := s.validateCircularAncestry(k, sch, knownanc)
		if len(ancs) > 0 {
			res.AddErrors(errors.New(errors.CompositeErrorCode, "definition %q has circular ancestry: %v", k, ancs))
			return res
		}

		knowns := make(map[string]struct{})
		dups := s.validateSchemaPropertyNames(k, sch, knowns)
		if len(dups) > 0 {
			var pns []string
			for _, v := range dups {
				pns = append(pns, v.Definition+"."+v.Name)
			}
			res.AddErrors(errors.New(errors.CompositeErrorCode, "definition %q contains duplicate properties: %v", k, pns))
		}

	}
	return res
}

func (s *SpecValidator) resolveRef(ref *spec.Ref) (*spec.Schema, error) {
	if s.spec.SpecFilePath() != "" {
		return spec.ResolveRefWithBase(s.spec.Spec(), ref, &spec.ExpandOptions{RelativeBase: s.spec.SpecFilePath()})
	}
	// TODO: interpret the raw error message from JsonPointer
	return spec.ResolveRef(s.spec.Spec(), ref)
}

func (s *SpecValidator) validateSchemaPropertyNames(nm string, sch spec.Schema, knowns map[string]struct{}) []dupProp {
	var dups []dupProp

	schn := nm
	schc := &sch
	for schc.Ref.String() != "" {
		// gather property names
		reso, err := s.resolveRef(&schc.Ref)
		if err != nil {
			panic(err)
		}
		schc = reso
		schn = sch.Ref.String()
	}

	if len(schc.AllOf) > 0 {
		for _, chld := range schc.AllOf {
			dups = append(dups, s.validateSchemaPropertyNames(schn, chld, knowns)...)
		}
		return dups
	}

	for k := range schc.Properties {
		_, ok := knowns[k]
		if ok {
			dups = append(dups, dupProp{Name: k, Definition: schn})
		} else {
			knowns[k] = struct{}{}
		}
	}

	return dups
}

func (s *SpecValidator) validateCircularAncestry(nm string, sch spec.Schema, knowns map[string]struct{}) []string {
	if sch.Ref.String() == "" && len(sch.AllOf) == 0 {
		return nil
	}
	var ancs []string

	schn := nm
	schc := &sch
	for schc.Ref.String() != "" {
		reso, err := s.resolveRef(&schc.Ref)
		if err != nil {
			panic(err)
		}
		schc = reso
		schn = sch.Ref.String()
	}

	if schn != nm && schn != "" {
		if _, ok := knowns[schn]; ok {
			ancs = append(ancs, schn)
		}
		knowns[schn] = struct{}{}

		if len(ancs) > 0 {
			return ancs
		}
	}

	if len(schc.AllOf) > 0 {
		for _, chld := range schc.AllOf {
			if chld.Ref.String() != "" || len(chld.AllOf) > 0 {
				ancs = append(ancs, s.validateCircularAncestry(schn, chld, knowns)...)
				if len(ancs) > 0 {
					return ancs
				}
			}
		}
	}

	return ancs
}

func (s *SpecValidator) validateItems() *Result {
	// validate parameter, items, schema and response objects for presence of item if type is array
	res := new(Result)

	// TODO: implement support for lookups of refs
	for method, pi := range s.analyzer.Operations() {
		for path, op := range pi {
			for _, param := range s.analyzer.ParamsFor(method, path) {
				if param.TypeName() == "array" && param.ItemsTypeName() == "" {
					res.AddErrors(errors.New(errors.CompositeErrorCode, "param %q for %q is a collection without an element type (array requires item definition)", param.Name, op.ID))
					continue
				}
				// TODO: what about other In?
				if param.In != "body" {
					if param.Items != nil {
						items := param.Items
						for items.TypeName() == "array" {
							if items.ItemsTypeName() == "" {
								res.AddErrors(errors.New(errors.CompositeErrorCode, "param %q for %q is a collection without an element type (array requires item definition)", param.Name, op.ID))
								break
							}
							items = items.Items
						}
					}
				} else {
					// In: body
					// TODO: better message
					if param.Schema != nil {
						if err := s.validateSchemaItems(*param.Schema, fmt.Sprintf("body param %q", param.Name), op.ID); err != nil {
							res.AddErrors(err)
						}
					}
				}
			}

			var responses []spec.Response
			if op.Responses != nil {
				if op.Responses.Default != nil {
					responses = append(responses, *op.Responses.Default)
				}
				if op.Responses.StatusCodeResponses != nil {
					for _, v := range op.Responses.StatusCodeResponses {
						responses = append(responses, v)
					}
				}
			}
			// TODO: isn't reponse mandatory?

			for _, resp := range responses {
				// Response headers with array
				for hn, hv := range resp.Headers {
					if hv.TypeName() == "array" && hv.ItemsTypeName() == "" {
						res.AddErrors(errors.New(errors.CompositeErrorCode, "header %q for %q is a collection without an element type (array requires items definition)", hn, op.ID))
					}
				}
				if resp.Schema != nil {
					if err := s.validateSchemaItems(*resp.Schema, "response body", op.ID); err != nil {
						res.AddErrors(err)
					}
				}
			}
		}
	}
	return res
}

// Verifies constraints on array type
func (s *SpecValidator) validateSchemaItems(schema spec.Schema, prefix, opID string) error {
	if !schema.Type.Contains("array") {
		return nil
	}

	if schema.Items == nil || schema.Items.Len() == 0 {
		return errors.New(errors.CompositeErrorCode, "%s for %q is a collection without an element type (array requires items definition)", prefix, opID)
	}

	if schema.Items.Schema != nil {
		schema = *schema.Items.Schema
		if _, err := compileRegexp(schema.Pattern); err != nil {
			return errors.New(errors.CompositeErrorCode, "%s for %q has invalid items pattern: %q", prefix, opID, schema.Pattern)
		}

		return s.validateSchemaItems(schema, prefix, opID)
	}

	return nil
}

func (s *SpecValidator) validateUniqueSecurityScopes() *Result {
	// Each authorization/security reference should contain only unique scopes.
	// (Example: For an oauth2 authorization/security requirement, when listing the required scopes,
	// each scope should only be listed once.)
	//
	// - securityDefinitions:
	//     OAuth2:
	//       type: oauth2
	//       scopes:
	//         read: blah blah
	//         write: blah blah
	//         read: xyz   <=  error
	// TODO: issue go-swagger/go-swagger#14
	// No use to make this check here since the Scopes structure is a map.
	// Attempting to load a spec with duplicate keys for scope would result
	// in a silent overwrite of the existing key in the spec package
	// (github.com/go-openapi/spec/security_scheme.go#L97)
	// To be solved in [go-openapi/spec] package.
	/*
		for _ , pi := range s.analyzer.Operations() {
			if pi != nil {	// Safeguard
				for k, sec := range.pi.SecurityDefinitionsFor(pi){
					if sec.SecuritySchemeProps != nil {
						if sec.SecuritySchemeProps.Scopes != nil {
							for scope, _ := range sec.SecuritySchemeProps.Scopes {
								// there can't be duplicates here. Too late
							}
						}

					}

				}
			}
		}
	*/
	return nil
}

func (s *SpecValidator) validatePathParamPresence(path string, fromPath, fromOperation []string) *Result {
	// Each defined operation path parameters must correspond to a named element in the API's path pattern.
	// (For example, you cannot have a path parameter named id for the following path /pets/{petId} but you must have a path parameter named petId.)
	res := new(Result)
	for _, l := range fromPath {
		var matched bool
		for _, r := range fromOperation {
			if l == "{"+r+"}" {
				matched = true
				break
			}
		}
		if !matched {
			res.Errors = append(res.Errors, errors.New(errors.CompositeErrorCode, "path param %q has no parameter definition", l))
		}
	}

	for _, p := range fromOperation {
		var matched bool
		for _, r := range fromPath {
			if "{"+p+"}" == r {
				matched = true
				break
			}
		}
		if !matched {
			res.AddErrors(errors.New(errors.CompositeErrorCode, "path param %q is not present in path %q", p, path))
		}
	}

	return res
}

func (s *SpecValidator) validateReferenced() *Result {
	var res Result
	res.Merge(s.validateReferencedParameters())
	res.Merge(s.validateReferencedResponses())
	res.Merge(s.validateReferencedDefinitions())
	return &res
}

func (s *SpecValidator) validateReferencedParameters() *Result {
	// Each referenceable definition must have references.
	params := s.spec.Spec().Parameters
	if len(params) == 0 {
		return nil
	}

	expected := make(map[string]struct{})
	for k := range params {
		expected["#/parameters/"+jsonpointer.Escape(k)] = struct{}{}
	}
	for _, k := range s.analyzer.AllParameterReferences() {
		if _, ok := expected[k]; ok {
			delete(expected, k)
		}
	}

	if len(expected) == 0 {
		return nil
	}
	var result Result
	for k := range expected {
		result.AddErrors(errors.New(errors.CompositeErrorCode, "parameter %q is not used anywhere", k))
	}
	return &result
}

func (s *SpecValidator) validateReferencedResponses() *Result {
	// Each referenceable definition must have references.
	responses := s.spec.Spec().Responses
	if len(responses) == 0 {
		return nil
	}

	expected := make(map[string]struct{})
	for k := range responses {
		expected["#/responses/"+jsonpointer.Escape(k)] = struct{}{}
	}
	for _, k := range s.analyzer.AllResponseReferences() {
		if _, ok := expected[k]; ok {
			delete(expected, k)
		}
	}

	if len(expected) == 0 {
		return nil
	}
	var result Result
	for k := range expected {
		result.AddErrors(errors.New(errors.CompositeErrorCode, "response %q is not used anywhere", k))
	}
	return &result
}

func (s *SpecValidator) validateReferencedDefinitions() *Result {
	// Each referenceable definition must have references.
	defs := s.spec.Spec().Definitions
	if len(defs) == 0 {
		return nil
	}

	expected := make(map[string]struct{})
	for k := range defs {
		expected["#/definitions/"+jsonpointer.Escape(k)] = struct{}{}
	}
	for _, k := range s.analyzer.AllDefinitionReferences() {
		if _, ok := expected[k]; ok {
			delete(expected, k)
		}
	}

	if len(expected) == 0 {
		return nil
	}
	var result Result
	for k := range expected {
		result.AddErrors(errors.New(errors.CompositeErrorCode, "definition %q is not used anywhere", k))
	}
	return &result
}

func (s *SpecValidator) validateRequiredDefinitions() *Result {
	// Each definition property listed in the required array must be defined in the properties of the model
	res := new(Result)
	for d, v := range s.spec.Spec().Definitions {
		if v.Required != nil { // Safeguard
		REQUIRED:
			for _, pn := range v.Required {
				if _, ok := v.Properties[pn]; ok {
					continue
				}

				for pp := range v.PatternProperties {
					re, err := compileRegexp(pp)
					if err != nil {
						// TODO: add error context
						res.AddErrors(errors.New(errors.CompositeErrorCode, "Pattern \"%q\" is invalid", pp))
						continue REQUIRED
					}
					if re.MatchString(pn) {
						continue REQUIRED
					}
				}

				if v.AdditionalProperties != nil {
					if v.AdditionalProperties.Allows {
						continue
					}
					if v.AdditionalProperties.Schema != nil {
						continue
					}
				}

				res.AddErrors(errors.New(errors.CompositeErrorCode, "%q is present in required but not defined as property in definition %q", pn, d))
			}
		}
	}
	return res
}

func (s *SpecValidator) validateParameters() *Result {
	// - for each method, path is unique, regardless of path parameters
	//   e.g. GET:/petstore/{id}, GET:/petstore/{pet}, GET:/petstore are
	//   considered duplicate paths
	// - each parameter should have a unique `name` and `type` combination
	// - each operation should have only 1 parameter of type body
	// - there must be at most 1 parameter in body
	// - parameters with pattern property must specify valid patterns
	// - $ref in parameters must resolve
	// - path param must be required
	res := new(Result)
	for method, pi := range s.analyzer.Operations() {
		methodPaths := make(map[string]map[string]string)
		if pi != nil { // Safeguard
			for path, op := range pi {
				// Check uniqueness of stripped paths
				pathToAdd := stripParametersInPath(path)
				if _, found := methodPaths[method][pathToAdd]; found {
					res.AddErrors(errors.New(errors.CompositeErrorCode, "path %s overlaps with %s", path, methodPaths[method][pathToAdd]))
				} else {
					if _, found := methodPaths[method]; !found {
						methodPaths[method] = map[string]string{}
					}
					methodPaths[method][pathToAdd] = path //Original non stripped path

				}

				ptypes := make(map[string]map[string]struct{})
				var firstBodyParam string
				sw := s.spec.Spec()
				var paramNames []string

				// Check for duplicate parameters declaration in param section
				if op.Parameters != nil { // Safeguard
				PARAMETERS:
					for _, ppr := range op.Parameters {
						pr := ppr
						for pr.Ref.String() != "" {
							obj, _, err := pr.Ref.GetPointer().Get(sw)
							if err != nil {
								addPointerError(res, err, pr.Ref.String(), strings.Join([]string{path, ppr.Name}, "/"))
								// Continue reporting errors if asked to do so
								if !continueOnErrors {
									break PARAMETERS
								}
							}
							pr = obj.(spec.Parameter)
						}
						pnames, ok := ptypes[pr.In]
						if !ok {
							pnames = make(map[string]struct{})
							ptypes[pr.In] = pnames
						}

						_, ok = pnames[pr.Name]
						if ok {
							res.AddErrors(errors.New(errors.CompositeErrorCode, "duplicate parameter name %q for %q in operation %q", pr.Name, pr.In, op.ID))
						}
						pnames[pr.Name] = struct{}{}
					}
				}

			PARAMETERS2:
				for _, ppr := range s.analyzer.ParamsFor(method, path) {
					pr := ppr
					for pr.Ref.String() != "" {
						obj, _, err := pr.Ref.GetPointer().Get(sw)
						if err != nil {
							addPointerError(res, err, pr.Ref.String(), strings.Join([]string{path, ppr.Name}, "/"))
							// Continue reporting errors if asked to do so
							if !continueOnErrors {
								break PARAMETERS2
							}
						}
						pr = obj.(spec.Parameter)
					}

					// Validate pattern for parameters with a pattern property
					if _, err := compileRegexp(pr.Pattern); err != nil {
						res.AddErrors(errors.New(errors.CompositeErrorCode, "operation %q has invalid pattern in param %q: %q", op.ID, pr.Name, pr.Pattern))
					}

					// There must be at most one parameter in body
					// TODO: and if there is none?
					if pr.In == "body" {
						if firstBodyParam != "" {
							res.AddErrors(errors.New(errors.CompositeErrorCode, "operation %q has more than 1 body param (accepted: %q, dropped: %q)", op.ID, firstBodyParam, pr.Name))
						}
						firstBodyParam = pr.Name
					}

					if pr.In == "path" {
						paramNames = append(paramNames, pr.Name)
						// Path declared in path must have the required: true property
						if !pr.Required {
							res.AddErrors(errors.New(errors.CompositeErrorCode, "in operation %q,path param %q must be declared as required", op.ID, pr.Name))
						}
					}
				}
				// Check uniqueness of parameters in path
				paramsInPath := extractPathParams(path)
				for i, p := range paramsInPath {
					for j, q := range paramsInPath {
						if p == q && i > j {
							res.AddErrors(errors.New(errors.CompositeErrorCode, "params in path %q must be unique: \"%q\" conflicts whith \"%q\"", path, p, q))
							break
						}

					}
				}

				// Warns about possible malformed params in path
				// TODO implement warning not error
				rexGarbledParam := mustCompileRegexp(`{.*[{}\s]+.*}`)
				for _, p := range paramsInPath {
					if rexGarbledParam.MatchString(p) {
						res.AddErrors(errors.New(errors.CompositeErrorCode, "in path %q, param \"%q\" contains {,} or white space. Albeit not stricly illegal, this is probably no what you want", path, p))
					}
				}
				// Match params from path vs params from params section
				res.Merge(s.validatePathParamPresence(path, paramsInPath, paramNames))
			}
		}
	}
	return res
}

func stripParametersInPath(path string) string {
	// Returns a path stripped from all path parameters, with multiple or trailing slashes removed
	// Stripping is performed on a slash-separated basis, e.g '/a{/b}' remains a{/b} and not /a

	// Regexp to extract parameters from path, with surrounding {}.
	// Note the important non-greedy modifier.
	rexParsePathParam := mustCompileRegexp(`{[^{}]+?}`)
	strippedSegments := []string{}

	for _, segment := range strings.Split(path, "/") {
		if segment != "" {
			strippedSegments = append(strippedSegments, rexParsePathParam.ReplaceAllString(segment, ""))
		}
	}
	return strings.Join(strippedSegments, "/")
}

func extractPathParams(path string) (params []string) {
	// Extracts all params from a path, with surrounding "{}"
	rexParsePathParam := mustCompileRegexp(`{[^{}]+?}`)

	for _, segment := range strings.Split(path, "/") {
		for _, v := range rexParsePathParam.FindAllStringSubmatch(segment, -1) {
			params = append(params, v...)
		}
	}
	return
}

func (s *SpecValidator) validateReferencesValid() *Result {
	// each reference must point to a valid object
	res := new(Result)
	for _, r := range s.analyzer.AllRefs() {
		if !r.IsValidURI(s.spec.SpecFilePath()) {
			res.AddErrors(errors.New(404, "invalid ref %q", r.String()))
		}
	}
	if !res.HasErrors() {
		exp, err := s.spec.Expanded()
		if err != nil {
			res.AddErrors(err)
		}
		s.expanded = exp
	}
	return res
}

func (s *SpecValidator) validateResponseExample(path string, r *spec.Response) *Result {
	// values provided as example in responses must validate the schema they examplify
	res := new(Result)

	// Recursively follow possible $ref's
	if r.Ref.String() != "" {
		nr, _, err := r.Ref.GetPointer().Get(s.spec.Spec())
		if err != nil {

			addPointerError(res, err, r.Ref.String(), strings.Join([]string{path, r.ResponseProps.Schema.ID}, "/"))
			return res
		}
		rr := nr.(spec.Response)
		return s.validateResponseExample(path, &rr)
	}

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

// TODO: issue #1231
func (s *SpecValidator) validateExamplesValidAgainstSchema() *Result {
	res := new(Result)

	for _, pathItem := range s.analyzer.Operations() {
		if pathItem != nil {
			for path, op := range pathItem {
				if op.Responses != nil {
					if op.Responses.Default != nil {
						dr := op.Responses.Default
						res.Merge(s.validateResponseExample(path, dr))
					}
					for _, r := range op.Responses.StatusCodeResponses {
						res.Merge(s.validateResponseExample(path, &r))

					}
				}
			}
		}
	}
	return res
}

func (s *SpecValidator) validateDefaultValueValidAgainstSchema() *Result {
	// every default value that is specified must validate against the schema for that property
	// headers, items, parameters, schema

	res := new(Result)

	for method, pathItem := range s.analyzer.Operations() {
		if pathItem != nil { // Safeguard
			for path, op := range pathItem {
				// parameters
				var hasForm, hasBody bool
			PARAMETERS:
				for _, pr := range s.analyzer.ParamsFor(method, path) {
					// expand ref is necessary
					param := pr
					for param.Ref.String() != "" {
						obj, _, err := param.Ref.GetPointer().Get(s.spec.Spec())
						if err != nil {
							addPointerError(res, err, param.Ref.String(), strings.Join([]string{path, param.Name}, "/"))
							// Continue reporting errors if asked to do so
							if !continueOnErrors {
								break PARAMETERS
							}
						}
						param = obj.(spec.Parameter)
					}
					if param.In == "formData" {
						if hasBody && !hasForm {
							res.AddErrors(errors.New(errors.CompositeErrorCode, "operation %q has both formData and body parameters. Only one such In: type may be used for a given operation", op.ID))
						}
						hasForm = true
					}
					if param.In == "body" {
						if hasForm && !hasBody {
							res.AddErrors(errors.New(errors.CompositeErrorCode, "operation %q has both body and formData parameters. Only one such In: type may be used for a given operation", op.ID))
						}
						hasBody = true
					}
					// check simple parameters first
					// default values provided must validate against their schema
					if param.Default != nil && param.Schema == nil {
						if Debug {
							log.Println(param.Name, "in", param.In, "has a default without a schema")
						}
						// check param valid
						// TODO: add error here to give user context
						res.Merge(NewParamValidator(&param, s.KnownFormats).Validate(param.Default))
					}

					// Recursively follows Items and Schemas
					if param.Items != nil {
						res.Merge(s.validateDefaultValueItemsAgainstSchema(param.Name, param.In, &param, param.Items))
					}

					if param.Schema != nil {
						res.Merge(s.validateDefaultValueSchemaAgainstSchema(param.Name, param.In, param.Schema))
					}
				}

				// Same constraint on default Responses
				if op.Responses != nil {
					if op.Responses.Default != nil {
						dr := op.Responses.Default
						for nm, h := range dr.Headers {
							if h.Default != nil {
								res.Merge(NewHeaderValidator(nm, &h, s.KnownFormats).Validate(h.Default))
							}
							if h.Items != nil {
								res.Merge(s.validateDefaultValueItemsAgainstSchema(nm, "header", &h, h.Items))
							}
							if _, err := compileRegexp(h.Pattern); err != nil {
								res.AddErrors(errors.New(errors.CompositeErrorCode, "operation %q has invalid pattern in default header %q: %q", op.ID, nm, h.Pattern))
							}
						}
						if dr.Schema != nil {
							res.Merge(s.validateDefaultValueSchemaAgainstSchema("default", "response", dr.Schema))
						}
					}
					if op.Responses.StatusCodeResponses != nil {
						for code, r := range op.Responses.StatusCodeResponses {
							for nm, h := range r.Headers {
								if h.Default != nil {
									res.Merge(NewHeaderValidator(nm, &h, s.KnownFormats).Validate(h.Default))
								}
								if h.Items != nil {
									res.Merge(s.validateDefaultValueItemsAgainstSchema(nm, "header", &h, h.Items))
								}
								if _, err := compileRegexp(h.Pattern); err != nil {
									res.AddErrors(errors.New(errors.CompositeErrorCode, "operation %q has invalid pattern in %v's header %q: %q", op.ID, code, nm, h.Pattern))
								}
							}
							if r.Schema != nil {
								res.Merge(s.validateDefaultValueSchemaAgainstSchema(strconv.Itoa(code), "response", r.Schema))
							}
						}
					} else {
						// NOTE: no additional message here since, for an unknown reason, this section does not always run
						// Empty op.ID means there is no meaningful operation: no need to report a specific message
						//if op.ID != "" {
						//	res.AddErrors(errors.New(errors.CompositeErrorCode, "response for operation %q has no valid status code section", op.ID)) // TODO: add name
						//}
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
			res.Merge(s.validateDefaultValueSchemaAgainstSchema(fmt.Sprintf("definitions.%s", nm), "body", &sch))
		}
	}
	return res
}

func (s *SpecValidator) validateDefaultValueSchemaAgainstSchema(path, in string, schema *spec.Schema) *Result {
	res := new(Result)
	if schema != nil { // Safeguard
		if schema.Default != nil {
			res.Merge(NewSchemaValidator(schema, s.spec.Spec(), path, s.KnownFormats).Validate(schema.Default))
		}
		if schema.Items != nil {
			if schema.Items.Schema != nil {
				res.Merge(s.validateDefaultValueSchemaAgainstSchema(path+".items", in, schema.Items.Schema))
			}
			for i, sch := range schema.Items.Schemas {
				res.Merge(s.validateDefaultValueSchemaAgainstSchema(fmt.Sprintf("%s.items[%d]", path, i), in, &sch))
			}
		}
		if _, err := compileRegexp(schema.Pattern); err != nil {
			res.AddErrors(errors.New(errors.CompositeErrorCode, "%s in %s has invalid pattern: %q", path, in, schema.Pattern))
		}
		if schema.AdditionalItems != nil && schema.AdditionalItems.Schema != nil {
			res.Merge(s.validateDefaultValueSchemaAgainstSchema(fmt.Sprintf("%s.additionalItems", path), in, schema.AdditionalItems.Schema))
		}
		for propName, prop := range schema.Properties {
			res.Merge(s.validateDefaultValueSchemaAgainstSchema(path+"."+propName, in, &prop))
		}
		for propName, prop := range schema.PatternProperties {
			res.Merge(s.validateDefaultValueSchemaAgainstSchema(path+"."+propName, in, &prop))
		}
		if schema.AdditionalProperties != nil && schema.AdditionalProperties.Schema != nil {
			res.Merge(s.validateDefaultValueSchemaAgainstSchema(fmt.Sprintf("%s.additionalProperties", path), in, schema.AdditionalProperties.Schema))
		}
		if schema.AllOf != nil {
			for i, aoSch := range schema.AllOf {
				res.Merge(s.validateDefaultValueSchemaAgainstSchema(fmt.Sprintf("%s.allOf[%d]", path, i), in, &aoSch))
			}
		}
	}
	return res
}

func (s *SpecValidator) validateDefaultValueItemsAgainstSchema(path, in string, root interface{}, items *spec.Items) *Result {
	res := new(Result)
	if items != nil {
		if items.Default != nil {
			res.Merge(newItemsValidator(path, in, items, root, s.KnownFormats).Validate(0, items.Default))
		}
		if items.Items != nil {
			res.Merge(s.validateDefaultValueItemsAgainstSchema(path+"[0]", in, root, items.Items))
		}
		if _, err := compileRegexp(items.Pattern); err != nil {
			res.AddErrors(errors.New(errors.CompositeErrorCode, "%s in %s has invalid pattern: %q", path, in, items.Pattern))
		}
	}
	return res
}

// $ref may not have siblings
// Spec: $ref siblings are ignored. So this check produces a warning
// TODO: check that $refs are only found in schemas
func (s *SpecValidator) validateRefNoSibling() *Result {
	//path, in string, root interface{}, items *spec.Items
	//spew.Dump(s.analyzer.AllReferences())
	//spew.Dump(s.analyzer.AllRefs())
	// One expects $ref to occur in schemas in params or responses
	//res := new(Result)
	//for _, schema := range s.analyzer.AllDefinitions() {
	/*
		for method, pi := range s.analyzer.Operations() {
			for path, op := range pi {
				// Look for $ref in params
				for _, param := range s.analyzer.ParamsFor(method, path) {
					fmt.Println("Param")
					spew.Dump(param.Refable)
					fmt.Println("ParamProps")
					spew.Dump(param.ParamProps)
					fmt.Println("Simple sch")
					spew.Dump(param.SimpleSchema)
				}
				// Look for $ref in responses
				var responses []spec.Response
				if op.Responses != nil {
					if op.Responses.Default != nil {
						responses = append(responses, *op.Responses.Default)
					}
					if op.Responses.StatusCodeResponses != nil {
						for _, v := range op.Responses.StatusCodeResponses {
							responses = append(responses, v)
						}
					}
				}
				for _, resp := range responses {
					fmt.Println("Response.Refable")
					spew.Dump(resp.Refable)
					fmt.Println("ResponseProps")
					spew.Dump(resp.ResponseProps)
				}
	*/
	/*
		fmt.Println("SchemaRef.Ref")
		spew.Dump(schema.Ref)
		fmt.Println("SchemaRef.Name")
		spew.Dump(schema.Name)
		fmt.Println("SchemaProps")
		spew.Dump(schema.Schema.SchemaProps)
		fmt.Println("Schema swaggerSchemaProps")
		spew.Dump(schema.Schema.SwaggerSchemaProps)
	*/
	//}
	return nil
}

func addPointerError(res *Result, err error, ref string, fromPath string) *Result {
	// Provides more context on error messages
	// reported by the jsoinpointer package
	var richErr error

	switch {
	case strings.Contains(err.Error(), "object has no key"):
		richErr = fmt.Errorf("Could not resolve reference in %s to $ref:%s [%s]", fromPath, ref, err)
	default:
		richErr = err
	}

	res.AddErrors(richErr)
	// TODO: change behavior of AddErrors
	return nil
}
