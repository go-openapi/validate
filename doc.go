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

/*
Package validate provides methods to validate a swagger specification,
as well as tools to validate data against their schema.

This package follows Swagger 2.0. specification (aka OpenAPI 2.0). Reference
can be found here: https://github.com/OAI/OpenAPI-Specification/blob/master/versions/2.0.md.

Validating a specification

Validates a spec document (from JSON or YAML) against the JSON schema for swagger,
then checks a number of extra rules that can't be expressed in JSON schema.

Entry points:
  - Spec()
  - NewSpecValidator()
  - SpecValidator.Validate()

Reported as errors:
  - definition can't declare a property that's already defined by one of its ancestors
  - definition's ancestor can't be a descendant of the same model
  - path uniqueness: each api path should be non-verbatim (account for path param names) unique per method
  - each security reference should contain only unique scopes
  - each security scope in a security definition should be unique
  - parameters in path must be unique
  - each path parameter must correspond to a parameter placeholder and vice versa
  - each referenceable definition must have references
  - each definition property listed in the required array must be defined in the properties of the model
  - each parameter should have a unique `name` and `type` combination
  - each operation should have only 1 parameter of type body
  - each reference must point to a valid object
  - every default value that is specified must validate against the schema for that property
  - items property is required for all schemas/definitions of type `array`
  - path parameters must be declared a required
  - headers must not contain $ref
  - schema and property examples provided must validate against their respective object's schema

Reported as warnings:
  - path parameters should not contain any of [{,},\w]
  - empty path
  - unused definitions

Validating a schema

The schema validation toolkit validates data against JSON-schema-draft 04 schema.

It is tested again json-schema-testing-suite (https://github.com/json-schema-org/JSON-Schema-Test-Suite).

Entry points:
  - AgainstSchema()
  - ...

Known limitations

With the current version of this package, the following aspects of swagger are not yet supported:
  - examples in parameters and in schemas are not checked
  - default values and examples on responses only support application/json producer type
  - invalid numeric constraints (such as Minimum, etc..) are not checked except for default values
  - valid js ECMA regexp not supported by Go regexp engine are considered invalid
  - errors and warnings are not reported with key/line number in spec
  - rules for collectionFormat are not implemented
  - no specific rule for readOnly attribute in properties [not done here]
  - no specific rule for polymorphism support (discriminator) [not done here]
*/
package validate
