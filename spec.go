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
	"sort"
	"strings"

	"github.com/go-openapi/analysis"
	"github.com/go-openapi/errors"
	"github.com/go-openapi/jsonpointer"
	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
)

// Spec validates an OpenAPI 2.0 specification document.
//
// Returns an error flattening in a single standard error, all validation messages.
//
//  - TODO: warnings about the use of id keyword
//  - TODO: $ref should not have siblings
//  - TODO: make sure documentation reflects all checks and warnings
//  - TODO: warning when readOnly and required
//  - TODO: check on discriminators
//
// NOTE: SecurityScopes are maps: no need to check uniqueness
//
func Spec(doc *loads.Document, formats strfmt.Registry) error {
	errs, _ /*warns*/ := NewSpecValidator(doc.Schema(), formats).Validate(doc)
	if errs.HasErrors() {
		return errors.CompositeValidationError(errs.Errors...)
	}
	return nil
}

// SpecValidator validates a swagger 2.0 spec
type SpecValidator struct {
	schema       *spec.Schema // swagger 2.0 schema
	spec         *loads.Document
	analyzer     *analysis.Spec
	expanded     *loads.Document
	KnownFormats strfmt.Registry
	Options      Opts // validation options
}

// NewSpecValidator creates a new swagger spec validator instance
func NewSpecValidator(schema *spec.Schema, formats strfmt.Registry) *SpecValidator {
	return &SpecValidator{
		schema:       schema,
		KnownFormats: formats,
		Options:      defaultOpts,
	}
}

// Validate validates the swagger spec
func (s *SpecValidator) Validate(data interface{}) (errs *Result, warnings *Result) {
	var sd *loads.Document
	errs = new(Result)

	switch v := data.(type) {
	case *loads.Document:
		sd = v
	}
	if sd == nil {
		errs.AddErrors(invalidDocumentMsg())
		return
	}
	s.spec = sd
	s.analyzer = analysis.New(sd.Spec())

	warnings = new(Result)

	schv := NewSchemaValidator(s.schema, nil, "", s.KnownFormats)
	var obj interface{}

	// Raw spec unmarshalling errors
	if err := json.Unmarshal(sd.Raw(), &obj); err != nil {
		// TODO(TEST): test case
		// TODO: better message
		errs.AddErrors(err)
		return
	}

	defer func() {
		// errs holds all errors and warnings,
		// warnings only warnings
		errs.MergeAsWarnings(warnings)
		warnings.AddErrors(errs.Warnings...)
	}()

	errs.Merge(schv.Validate(obj)) // error -
	// There may be a point in continuing to try and determine more accurate errors
	if !s.Options.ContinueOnErrors && errs.HasErrors() {
		return // no point in continuing
	}

	errs.Merge(s.validateReferencesValid()) // error -
	// There may be a point in continuing to try and determine more accurate errors
	if !s.Options.ContinueOnErrors && errs.HasErrors() {
		return // no point in continuing
	}

	errs.Merge(s.validateDuplicateOperationIDs())
	errs.Merge(s.validateDuplicatePropertyNames()) // error -
	errs.Merge(s.validateParameters())             // error -
	errs.Merge(s.validateItems())                  // error -
	errs.Merge(s.validateRequiredDefinitions())    // error -

	// There may be a point in continuing to try and determine more accurate errors
	if !s.Options.ContinueOnErrors && errs.HasErrors() {
		return // no point in continuing
	}

	// Validate default values
	df := &defaultValidator{SpecValidator: s}
	errs.Merge(df.Validate())

	// Validate example values
	ex := &exampleValidator{SpecValidator: s}
	errs.Merge(ex.Validate())

	errs.Merge(s.validateNonEmptyPathParamNames())

	// TODO: validate numeric constraints (issue#581): this should be handled like defaults and examples

	errs.Merge(s.validateRefNoSibling()) // warning only
	errs.Merge(s.validateReferenced())   // warning only

	return
}

func (s *SpecValidator) validateNonEmptyPathParamNames() *Result {
	res := new(Result)
	if s.spec.Spec().Paths == nil {
		// There is no Paths object: error
		res.AddErrors(noValidPathMsg())
	} else {
		if s.spec.Spec().Paths.Paths == nil {
			// Paths may be empty: warning
			res.AddWarnings(noValidPathMsg())
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
// not supported by go-swagger. Shouldn't it be a warning?
// Ideally, the behavior error vs warning should be an optional setting (e.g. go-swagger mode)
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

		ancs, rec := s.validateCircularAncestry(k, sch, knownanc)
		if rec != nil && (rec.HasErrors() || !rec.HasWarnings()) {
			res.Merge(rec)
		}
		if len(ancs) > 0 {
			res.AddErrors(errors.New(errors.CompositeErrorCode, "definition %q has circular ancestry: %v", k, ancs))
			return res
		}

		knowns := make(map[string]struct{})
		dups, rep := s.validateSchemaPropertyNames(k, sch, knowns)
		if rep != nil && (rep.HasErrors() || rep.HasWarnings()) {
			res.Merge(rep)
		}
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
	// NOTE: it looks like with the new spec resolver, this code is now unrecheable
	return spec.ResolveRef(s.spec.Spec(), ref)
}

func (s *SpecValidator) validateSchemaPropertyNames(nm string, sch spec.Schema, knowns map[string]struct{}) ([]dupProp, *Result) {
	var dups []dupProp

	schn := nm
	schc := &sch
	res := new(Result)

	for schc.Ref.String() != "" {
		// gather property names
		reso, err := s.resolveRef(&schc.Ref)
		if err != nil {
			errorHelp.addPointerError(res, err, schc.Ref.String(), nm)
			return dups, res
		}
		schc = reso
		schn = sch.Ref.String()
	}

	if len(schc.AllOf) > 0 {
		for _, chld := range schc.AllOf {
			dup, rep := s.validateSchemaPropertyNames(schn, chld, knowns)
			if rep != nil && (rep.HasErrors() || rep.HasWarnings()) {
				res.Merge(rep)
			}
			dups = append(dups, dup...)
		}
		return dups, res
	}

	for k := range schc.Properties {
		_, ok := knowns[k]
		if ok {
			dups = append(dups, dupProp{Name: k, Definition: schn})
		} else {
			knowns[k] = struct{}{}
		}
	}

	return dups, res
}

func (s *SpecValidator) validateCircularAncestry(nm string, sch spec.Schema, knowns map[string]struct{}) ([]string, *Result) {
	res := new(Result)

	if sch.Ref.String() == "" && len(sch.AllOf) == 0 {
		// TODO(TEST): test case
		return nil, res
	}
	var ancs []string

	schn := nm
	schc := &sch

	for schc.Ref.String() != "" {
		reso, err := s.resolveRef(&schc.Ref)
		if err != nil {
			errorHelp.addPointerError(res, err, schc.Ref.String(), nm)
			return ancs, res
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
			return ancs, res
		}
	}

	if len(schc.AllOf) > 0 {
		for _, chld := range schc.AllOf {
			if chld.Ref.String() != "" || len(chld.AllOf) > 0 {
				anc, rec := s.validateCircularAncestry(schn, chld, knowns)
				if rec != nil && (rec.HasErrors() || !rec.HasWarnings()) {
					res.Merge(rec)
				}
				ancs = append(ancs, anc...)
				if len(ancs) > 0 {
					return ancs, res
				}
			}
		}
	}
	return ancs, res
}

func (s *SpecValidator) validateItems() *Result {
	// validate parameter, items, schema and response objects for presence of item if type is array
	res := new(Result)

	for method, pi := range s.analyzer.Operations() {
		for path, op := range pi {
			for _, param := range paramHelp.safeExpandedParamsFor(path, method, op.ID, res, s) {

				if param.TypeName() == "array" && param.ItemsTypeName() == "" {
					res.AddErrors(errors.New(errors.CompositeErrorCode, "param %q for %q is a collection without an element type (array requires item definition)", param.Name, op.ID))
					continue
				}
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
	// TODO(TEST): test case - Items defined without schema
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
	res.MergeAsWarnings(s.validateReferencedParameters())
	res.MergeAsWarnings(s.validateReferencedResponses())
	res.MergeAsWarnings(s.validateReferencedDefinitions())
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
						// TODO(TEST): test case
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
						// TODO(TEST): test case
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
	rexGarbledPathSegment := mustCompileRegexp(`.*[{}\s]+.*`)
	for method, pi := range s.analyzer.Operations() {
		methodPaths := make(map[string]map[string]string)
		if pi != nil { // Safeguard
			for path, op := range pi {
				pathToAdd := pathHelp.stripParametersInPath(path)

				// Warn on garbled path afer param stripping
				if rexGarbledPathSegment.MatchString(pathToAdd) {
					res.AddWarnings(errors.New(errors.CompositeErrorCode, "path stripped from path parameters %s contains {,} or white space. This is probably no what you want.", pathToAdd))
				}

				// Check uniqueness of stripped paths
				if _, found := methodPaths[method][pathToAdd]; found {

					// Sort names for stable, testable output
					if strings.Compare(path, methodPaths[method][pathToAdd]) < 0 {
						res.AddErrors(errors.New(errors.CompositeErrorCode, "path %s overlaps with %s", path, methodPaths[method][pathToAdd]))
					} else {
						res.AddErrors(errors.New(errors.CompositeErrorCode, "path %s overlaps with %s", methodPaths[method][pathToAdd], path))
					}
				} else {
					if _, found := methodPaths[method]; !found {
						methodPaths[method] = map[string]string{}
					}
					methodPaths[method][pathToAdd] = path //Original non stripped path

				}

				var bodyParams []string
				var paramNames []string
				var hasForm, hasBody bool

				// Check parameters names uniqueness for operation
				res.Merge(s.checkUniqueParams(path, method, op))

				for _, pr := range paramHelp.safeExpandedParamsFor(path, method, op.ID, res, s) {

					// Validate pattern regexp for parameters with a Pattern property
					if _, err := compileRegexp(pr.Pattern); err != nil {
						res.AddErrors(errors.New(errors.CompositeErrorCode, "operation %q has invalid pattern in param %q: %q", op.ID, pr.Name, pr.Pattern))
					}

					// There must be at most one parameter in body: list them all
					if pr.In == "body" {
						bodyParams = append(bodyParams, fmt.Sprintf("%q", pr.Name))
						hasBody = true
					}

					if pr.In == "path" {
						paramNames = append(paramNames, pr.Name)
						// Path declared in path must have the required: true property
						if !pr.Required {
							res.AddErrors(errors.New(errors.CompositeErrorCode, "in operation %q,path param %q must be declared as required", op.ID, pr.Name))
						}
					}

					if pr.In == "formData" {
						hasForm = true
					}
				}

				// In:formData and In:body are mutually exclusive
				if hasBody && hasForm {
					res.AddErrors(errors.New(errors.CompositeErrorCode, "operation %q has both formData and body parameters. Only one such In: type may be used for a given operation", op.ID))
				}
				// There must be at most one body param
				// Accurately report situations when more than 1 body param is declared (possibly unnamed)
				if len(bodyParams) > 1 {
					sort.Strings(bodyParams)
					res.AddErrors(errors.New(errors.CompositeErrorCode, "operation %q has more than 1 body param: %v", op.ID, bodyParams))
				}

				// Check uniqueness of parameters in path
				paramsInPath := pathHelp.extractPathParams(path)
				for i, p := range paramsInPath {
					for j, q := range paramsInPath {
						if p == q && i > j {
							res.AddErrors(errors.New(errors.CompositeErrorCode, "params in path %q must be unique: %q conflicts with %q", path, p, q))
							break
						}
					}
				}

				// Warns about possible malformed params in path
				rexGarbledParam := mustCompileRegexp(`{.*[{}\s]+.*}`)
				for _, p := range paramsInPath {
					if rexGarbledParam.MatchString(p) {
						res.AddWarnings(errors.New(errors.CompositeErrorCode, "in path %q, param %q contains {,} or white space. Albeit not stricly illegal, this is probably no what you want", path, p))
					}
				}

				// Match params from path vs params from params section
				res.Merge(s.validatePathParamPresence(path, paramsInPath, paramNames))
			}
		}
	}
	return res
}

func (s *SpecValidator) validateReferencesValid() *Result {
	// each reference must point to a valid object
	res := new(Result)
	for _, r := range s.analyzer.AllRefs() {
		if !r.IsValidURI(s.spec.SpecFilePath()) {
			// TODO(TEST): test case
			res.AddErrors(invalidRefMsg(r.String()))
		}
	}
	if !res.HasErrors() {
		// NOTE: with default settings, loads.Document.Expanded()
		// stops on first error. Anyhow, the expand option to continue
		// on errors fails to report errors at all.
		exp, err := s.spec.Expanded()
		if err != nil {
			res.AddErrors(unresolvedReferencesMsg(err))
		}
		s.expanded = exp
	}
	return res
}

// $ref may not have siblings
// Spec: $ref siblings are ignored. So this check produces a warning
// TODO: check that $refs are only found in schemas
func (s *SpecValidator) validateRefNoSibling() *Result {
	return nil
}

func (s *SpecValidator) checkUniqueParams(path, method string, op *spec.Operation) *Result {
	// Check for duplicate parameters declaration in param section.
	// Each parameter should have a unique `name` and `type` combination
	// TODO: could be factorized in analysis (when constructing the params map)
	// However, there are some issues with such a factorization:
	// - analysis does not seem to fully expand params
	// - param keys may be altered by x-go-name
	res := new(Result)
	pnames := make(map[string]struct{})

	if op.Parameters != nil { // Safeguard
		for _, ppr := range op.Parameters {
			ok := false
			pr, red := paramHelp.resolveParam(path, method, op.ID, ppr, s)
			res.Merge(red)

			if pr.Name != "" { // params with empty name does no participate the check
				key := fmt.Sprintf("%s#%s", pr.In, pr.Name)

				if _, ok = pnames[key]; ok {
					res.AddErrors(errors.New(errors.CompositeErrorCode, "duplicate parameter name %q for %q in operation %q", pr.Name, pr.In, op.ID))
				}
				pnames[key] = struct{}{}
			}
		}
	}
	return res
}

// SetContinueOnErrors sets the ContinueOnErrors option for this validator.
func (s *SpecValidator) SetContinueOnErrors(c bool) {
	s.Options.ContinueOnErrors = c
}
