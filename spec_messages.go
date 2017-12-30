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
	"net/http"

	"github.com/go-openapi/errors"
)

const (
	// error messages related to spec validation and retured as results

	// InvalidDocument states that spec validation only processes spec.Document objects
	InvalidDocument = "spec validator can only validate spec.Document objects"
	// InvalidReference indicates that a $ref property could not be resolved
	InvalidReference = "invalid ref %q"
	// NoParameterInPath indicates that a path was found without any parameter
	NoParameterInPath = "path param %q has no parameter definition"
	// ArrayRequiresItems ...
	ArrayRequiresItems = "%s for %q is a collection without an element type (array requires items definition)"
	// InvalidItemsPattern indicates an Items definition with invalid pattern
	InvalidItemsPattern = "%s for %q has invalid items pattern: %q"
	// UnresolvedReferences indicates that at least one $ref could not be resolved
	UnresolvedReferences = "some references could not be resolved in spec. First found: %v"
	// NoValidPath indicates that no single path could be validated. If Paths is empty, this message is only a warning.
	NoValidPath = "spec has no valid path defined"
)

const (
	// InternalErrorCode reports an internal technical error
	InternalErrorCode = http.StatusInternalServerError
	// NotFoundErrorCode indicates that a resource (e.g. a $ref) could not be found
	NotFoundErrorCode = http.StatusNotFound
)

func invalidDocumentMsg() errors.Error {
	return errors.New(InternalErrorCode, InvalidDocument)
}

func invalidRefMsg(path string) errors.Error {
	return errors.New(NotFoundErrorCode, InvalidReference, path)
}

func unresolvedReferencesMsg(err error) errors.Error {
	return errors.New(errors.CompositeErrorCode, UnresolvedReferences, err)
}

func noValidPathMsg() errors.Error {
	return errors.New(errors.CompositeErrorCode, NoValidPath)
}

/*
--errors.New(404, "invalid ref %q", r.String())
--errors.New(errors.CompositeErrorCode, "path param %q has no parameter definition", l)
--errors.New(errors.CompositeErrorCode, "%s for %q is a collection without an element type (array requires items definition)", prefix, opID)
--errors.New(errors.CompositeErrorCode, "%s for %q has invalid items pattern: %q", prefix, opID, schema.Pattern)
-- res.AddErrors(fmt.Errorf("some references could not be resolved in spec. First found: %v", err))

-- res.AddErrors(errors.New(errors.CompositeErrorCode, "spec has no valid path defined"))
res.AddErrors(errors.New(errors.CompositeErrorCode, "%q contains an empty path parameter", k))
res.AddErrors(errors.New(errors.CompositeErrorCode, "%q is defined %d times", k, v))
res.AddErrors(errors.New(errors.CompositeErrorCode, "definition %q has circular ancestry: %v", k, ancs))
res.AddErrors(errors.New(errors.CompositeErrorCode, "definition %q contains duplicate properties: %v", k, pns))
res.AddErrors(errors.New(errors.CompositeErrorCode, "param %q for %q is a collection without an element type (array requires item definition)", param.Name, op.ID))
res.AddErrors(errors.New(errors.CompositeErrorCode, "param %q for %q is a collection without an element type (array requires item definition)", param.Name, op.ID))
res.AddErrors(errors.New(errors.CompositeErrorCode, "header %q for %q is a collection without an element type (array requires items definition)", hn, op.ID))
res.AddErrors(errors.New(errors.CompositeErrorCode, "path param %q is not present in path %q", p, path))
res.AddErrors(errors.New(errors.CompositeErrorCode, "Pattern \"%q\" is invalid", pp))
res.AddErrors(errors.New(errors.CompositeErrorCode, "%q is present in required but not defined as property in definition %q", pn, d))
res.AddErrors(errors.New(errors.CompositeErrorCode, "path %s overlaps with %s", path, methodPaths[method][pathToAdd]))
res.AddErrors(errors.New(errors.CompositeErrorCode, "path %s overlaps with %s", methodPaths[method][pathToAdd], path))
res.AddErrors(errors.New(errors.CompositeErrorCode, "operation %q has invalid pattern in param %q: %q", op.ID, pr.Name, pr.Pattern))
res.AddErrors(errors.New(errors.CompositeErrorCode, "in operation %q,path param %q must be declared as required", op.ID, pr.Name))
res.AddErrors(errors.New(errors.CompositeErrorCode, "operation %q has both formData and body parameters. Only one such In: type may be used for a given operation", op.ID))
res.AddErrors(errors.New(errors.CompositeErrorCode, "operation %q has more than 1 body param: %v", op.ID, bodyParams))
res.AddErrors(errors.New(errors.CompositeErrorCode, "params in path %q must be unique: %q conflicts with %q", path, p, q))
res.AddErrors(errors.New(errors.CompositeErrorCode, "duplicate parameter name %q for %q in operation %q", pr.Name, pr.In, op.ID))
res.AddErrors(errors.New(errors.CompositeErrorCode, "parameter %q is not used anywhere", k))
res.AddErrors(errors.New(errors.CompositeErrorCode, "response %q is not used anywhere", k))
res.AddErrors(errors.New(errors.CompositeErrorCode, "definition %q is not used anywhere", k))

res.AddWarnings(errors.New(errors.CompositeErrorCode, "spec has no valid path defined"))
res.AddWarnings(errors.New(errors.CompositeErrorCode, "in path %q, param %q contains {,} or white space. Albeit not stricly illegal, this is probably no what you want", path, p))
res.AddWarnings(errors.New(errors.CompositeErrorCode, "path stripped from path parameters %s contains {,} or white space. This is probably no what you want.", pathToAdd))

*/
