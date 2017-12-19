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
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/loads/fmts"
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

type expectedMessage struct {
	message              string
	withContinueOnErrors bool // should be expected only when SetContinueOnErrors(true)
	isRegexp             bool // expected message is interpreted as regexp (with regexp.MatchString())
}

type expectedFixture struct {
	comment           string
	todo              string
	expectedLoadError bool // expect error on load: skip validate step
	expectedValid     bool // expect valid spec
	expectedMessages  []expectedMessage
	expectedWarnings  []expectedMessage
}

type expectedMap map[string]expectedFixture

// Test message improvements, issue #44 and some more
// ContinueOnErrors mode on
// WARNING: this test is very demanding and constructed with varied scenarios,
// which are not necessarily "unitary". Expect multiple changes in messages whenever
// altering the validator.
func Test_MessageQualityContinueOnErrors_Issue44(t *testing.T) {
	//t.SkipNow()
	state := continueOnErrors
	SetContinueOnErrors(true)
	defer func() {
		SetContinueOnErrors(state)
	}()
	testMessageQuality(t, true) /* set haltOnErrors=true to iterate spec by spec */
}

// ContinueOnErrors mode off
func Test_MessageQualityStopOnErrors_Issue44(t *testing.T) {
	//t.SkipNow()
	state := continueOnErrors
	SetContinueOnErrors(false)
	defer func() {
		SetContinueOnErrors(state)
	}()
	testMessageQuality(t, false) /* set haltOnErrors=true to iterate spec by spec */
}

// Verifies the production of validation error messages in multiple
// spec scenarios.
//
// The objective is to demonstrate:
// - messages are stable
// - validation continues as much as possible, even in presence of many errors
//
// haltOnErrors is used in dev mode to study and fix testcases step by step (output is pretty verbose)
// set SWAGGER_DEBUG_TEST=1 env to get a report of messages at the end of each test.
// expectedMessage{"", false, false},
func testMessageQuality(t *testing.T, haltOnErrors bool) {
	// TODO: load all this stuff from a yaml map
	tested := expectedMap{
		"bitbucket.json": expectedFixture{
			comment:           "Path differing by only a trailing / are not considered duplicates",
			todo:              "",
			expectedLoadError: false,
			expectedValid:     true,
			expectedWarnings:  []expectedMessage{},
			expectedMessages:  []expectedMessage{},
		},
		"fixture-342.yaml": expectedFixture{
			comment:           "Panic on interface conversion: early stop on error prevents the panic, but continuing it goes in, it goes down",
			todo:              "$ref have no sibling should be more general",
			expectedLoadError: false,
			expectedValid:     false,
			expectedWarnings: []expectedMessage{
				expectedMessage{"definition \"#/definitions/sample_info\" is not used anywhere", true, false},
				// TODO: Just this very special case is currently supported. $ref sibling should be more general
				expectedMessage{"$ref property should have no sibling in \"\".sid", true, false},
			},
			expectedMessages: []expectedMessage{
				expectedMessage{"\"paths./get_main_object.get.parameters\" must validate one and only one schema (oneOf). Found none valid", false, false},
				expectedMessage{"invalid definition as Schema for parameter sid in body in operation \"\"", true, false},
				expectedMessage{"some parameters definitions are broken in \"/get_main_object\".GET. Cannot continue validating parameters for operation", true, false},
			},
		},
		"fixture-342-2.yaml": expectedFixture{
			comment:           "Botched correction attempt for fixture-342",
			todo:              "",
			expectedLoadError: true,
			expectedValid:     false,
			expectedWarnings:  []expectedMessage{},
			expectedMessages: []expectedMessage{
				expectedMessage{".*json: cannot unmarshal object into Go struct field.*", false, true},
			},
		},
		"fixture-859-good.yaml": expectedFixture{
			comment:           "Issue#859: clear message on unresolved $ref. Valid spec baseline for further scenarios",
			todo:              "",
			expectedLoadError: false,
			expectedValid:     true,
			expectedWarnings: []expectedMessage{
				// TODO: how come we got this warning?
				expectedMessage{"definition \"#/definitions/myoutput\" is not used anywhere", false, false},
			},
			expectedMessages: []expectedMessage{},
		},
		"fixture-859.yaml": expectedFixture{
			comment:           "Issue#859: clear message on unresolved $ref. First scenario for messages",
			todo:              "Need supplement for items, arrays and other nested structures",
			expectedLoadError: false,
			expectedValid:     false,
			expectedWarnings: []expectedMessage{
				// Continue on errors...
				expectedMessage{"definition \"#/definitions/myoutputs\" is not used anywhere", true, false},
				expectedMessage{"parameter \"#/parameters/rateLimits\" is not used anywhere", true, false},
				expectedMessage{"definition \"#/definitions/records\" is not used anywhere", true, false},
				expectedMessage{"definition \"#/definitions/myparams\" is not used anywhere", true, false},
			},
			expectedMessages: []expectedMessage{
				expectedMessage{"\"paths./.get.parameters\" must validate one and only one schema (oneOf). Found none valid", false, false},
				// Continue on errors...
				expectedMessage{`some references could not be resolved in spec\. First found: object has no key ".*"`, true, true},
				expectedMessage{"could not resolve reference in \"/\".GET to $ref #/parameters/rateLimit: object has no key \"rateLimit\"", true, false},
				expectedMessage{"some parameters definitions are broken in \"/\".POST. Cannot continue validating parameters for operation", true, false},
				expectedMessage{"some parameters definitions are broken in \"/\".GET. Cannot continue validating parameters for operation", true, false},
				expectedMessage{"could not resolve reference in \"/\".POST to $ref #/parameters/rateLimit: object has no key \"rateLimit\"", true, false},
			},
		},
		"fixture-859-2.yaml": expectedFixture{
			comment:           "Issue#859: clear message on unresolved $ref. Additional scenario with items",
			todo:              "",
			expectedLoadError: false,
			expectedValid:     false,
			expectedWarnings: []expectedMessage{
				// Continue on errors...
				expectedMessage{"definition \"#/definitions/myoutput\" is not used anywhere", true, false},
			},
			expectedMessages: []expectedMessage{
				//expectedMessage{"\"paths./.get.parameters\" must validate one and only one schema (oneOf). Found none valid", false, false},
				// Continue on errors...
				// TODO: should have a better message. This is disapointing
				expectedMessage{`some references could not be resolved in spec\. First found: object has no key ".*"`, false, true},
				// Fixed in spec
				//expectedMessage{"definitions.myoutput in body must be of type array", false, false},
				//expectedMessage{"", false, false},
			},
		},
		"fixture-161.json": expectedFixture{
			comment:           "Issue#161: default value as object",
			todo:              "This spec may also be used to check example values",
			expectedLoadError: false,
			expectedValid:     false,
			expectedWarnings:  []expectedMessage{},
			expectedMessages: []expectedMessage{
				expectedMessage{"default value for requestBody in body does not validate its schema", false, false},
				expectedMessage{"requestBody.default in body must be of type object: \"string\"", false, false},
			},
		},
		"fixture-161-2.json": expectedFixture{
			comment:           "Variant Issue#161: this is a partially fixed spec. In this version, the name param type is fixed, but the default value remains wrongly typed",
			todo:              "",
			expectedLoadError: false,
			expectedValid:     false,
			expectedWarnings:  []expectedMessage{},
			expectedMessages: []expectedMessage{
				expectedMessage{"default value for requestBody in body does not validate its schema", false, false},
				expectedMessage{"requestBody.default in body must be of type object: \"string\"", false, false},
			},
		},
		"fixture-161-good.json": expectedFixture{
			comment:           "Issue#161: this is the corresponding corrected spec which should be valid",
			todo:              "",
			expectedLoadError: false,
			expectedValid:     true,
			expectedWarnings:  []expectedMessage{},
			expectedMessages:  []expectedMessage{},
		},
		"fixture-collisions.yaml": expectedFixture{
			comment:           "A supplement scenario for uniqueness tests in paths, operations, parameters",
			todo:              "",
			expectedLoadError: false,
			expectedValid:     false,
			expectedWarnings:  []expectedMessage{},
			expectedMessages: []expectedMessage{
				// ok
				// More than one param in body
				expectedMessage{"\"paths./bigbody/get.get.parameters\" must validate one and only one schema (oneOf). Found none valid", false, false},
				expectedMessage{"paths./dupparam/get.get.parameters in body shouldn't contain duplicates", false, false},
				// ok. Fixed message flip/flop with sort...
				expectedMessage{"operation \"ope2\" has more than 1 body param: [\"loadBalancerId2\" \"loadBalancerId3\"]", true, false},
				// Duplicate operations
				expectedMessage{"\"ope6\" is defined 2 times", true, false},
				expectedMessage{"\"ope5\" is defined 2 times", true, false},
				// Duplicate path
				expectedMessage{"path /duplpath/{id1}/get overlaps with /duplpath/{id2}/get", true, false},
				// Duplicate param
				expectedMessage{"duplicate parameter name \"id2\" for \"query\" in operation \"ope7\"", true, false},
				// Duplicate param in path
				expectedMessage{"params in path \"/loadBalancers/{loadBalancerId}/backendSets/{loadBalancerId}/get\" must be unique: \"{loadBalancerId}\" conflicts whith \"{loadBalancerId}\"", true, false},
				//expectedMessage{"", true, false},
			},
		},
		// Checking integer boundaries
		"fixture-constraints-on-numbers.yaml": expectedFixture{
			comment:           "A supplement scenario for native vs float-based constraint verifications on integers (multipleOf,maximum, minimum).",
			todo:              "This scenario supports current checks, that is for constraints on schemas with a default value only. It should be generalized (issue#581) and also for example values (issue#1231)",
			expectedLoadError: false,
			expectedValid:     false,
			expectedWarnings: []expectedMessage{
				expectedMessage{"param1 in query has a default but no valid schema", false, false},
				expectedMessage{"param2 in query has a default but no valid schema", false, false},
				expectedMessage{"param3 in query has a default but no valid schema", false, false},
				expectedMessage{"param4 in query has a default but no valid schema", false, false},
				expectedMessage{"param5 in query has a default but no valid schema", false, false},
				expectedMessage{"param6 in query has a default but no valid schema", false, false},
				expectedMessage{"param7 in query has a default but no valid schema", false, false},
				expectedMessage{"param8 in query has a default but no valid schema", false, false},
			},
			expectedMessages: []expectedMessage{
				expectedMessage{"default value for param1 in query does not validate its schema", false, false},
				expectedMessage{"default value for param2 in query does not validate its schema", false, false},
				expectedMessage{"default value for param3 in query does not validate its schema", false, false},
				expectedMessage{"default value for param4 in query does not validate its schema", false, false},
				expectedMessage{"default value for param5 in query does not validate its schema", false, false},
				expectedMessage{"default value for param6 in query does not validate its schema", false, false},
				expectedMessage{"default value for param7 in query does not validate its schema", false, false},
				expectedMessage{"default value for param8 in query does not validate its schema", false, false},
				// param1: ok
				expectedMessage{"param1 in query should be a multiple of 2.147483648e+09", false, false},
				expectedMessage{"MultipleOf value must be of type integer with format int32 in param1", false, false},
				// param2: ok
				expectedMessage{"MultipleOf value must be of type integer with format int32 in param2", false, false},
				expectedMessage{"Checked value must be of type integer with format int32 in param2", false, false},
				// param3: ok
				expectedMessage{"Checked value must be of type integer with format int32 in param3", false, false},
				expectedMessage{"param3 in query should be a multiple of 10", false, false},
				// param4: ok
				expectedMessage{"Checked value must be of type integer with format int32 in param4", false, false},
				// param5: ok
				expectedMessage{"Checked value must be of type integer with format int32 in param5", false, false},
				expectedMessage{"param5 in query should be less than or equal to 2.147483647e+09", false, false},
				// param6: ok
				expectedMessage{"Checked value must be of type integer with format uint32 in param6", false, false},
				// param7: ok
				expectedMessage{"Checked value must be of type integer with format int32 in param7", false, false},
				// param8: ok
				expectedMessage{"Checked value must be of type integer with format uint32 in param8", false, false},
				expectedMessage{"param8 in query should be greater than or equal to 2.147483647e+09", false, false},
			},
		},
		"fixture-581-inline-param.yaml": expectedFixture{
			comment:           "A variation on the theme of number constraints, inspired by isssue#581. Focuses on inline params",
			todo:              "The negative multiple message should be part of he error validation errors. Still limited by support of default values check",
			expectedLoadError: false,
			expectedValid:     false,
			expectedWarnings: []expectedMessage{
				expectedMessage{"definition \"#/definitions/myId\" is not used anywhere", true, false},
				expectedMessage{"inlineMaxInt in query has a default but no valid schema", true, false},
				expectedMessage{"inlineInfiniteInt in query has a default but no valid schema", true, false},
				expectedMessage{"negFactor in query has a default but no valid schema", true, false},
				expectedMessage{"inlineMinInt in query has a default but no valid schema", true, false},
				// TODO: unstable?
				//expectedMessage{"inlineInfiniteInt2 in query has a default but no valid schema", false, false},
				//expectedMessage{"negFactor2 in query has a default but no valid schema", false, false},
				//expectedMessage{"negFactor3 in query has a default but no valid schema", false, false},
			},
			expectedMessages: []expectedMessage{
				// ok
				expectedMessage{"\"paths./fixture.get.parameters\" must validate one and only one schema (oneOf). Found none valid", false, false},
				expectedMessage{"default value for inlineInfiniteInt in query does not validate its schema", true, false},
				expectedMessage{"default value for inlineMaxInt in query does not validate its schema", true, false},
				expectedMessage{"default value for inlineMinInt in query does not validate its schema", true, false},
				expectedMessage{"default value for negFactor in query does not validate its schema", true, false},
				// ok
				expectedMessage{"Maximum boundary value must be of type integer (default format) in inlineMaxInt", true, false},
				// ok
				expectedMessage{"Minimum boundary value must be of type integer (default format) in inlineMinInt", true, false},
				// ok
				expectedMessage{"Minimum boundary value must be of type integer (default format) in inlineInfiniteInt", true, false},
				expectedMessage{"Maximum boundary value must be of type integer (default format) in inlineInfiniteInt", true, false},
				// ok
				// ok: values are checked to verify the format, but unmarshalling was performed as float64 => hence number here (this precision should disappear with proper message)
				// TODO: more specific message to be added in pkg errors [Validation]) [minor defect]
				expectedMessage{"negFactor in query must be of type number, because: factor in multipleOf must be positive: -300", true, false},
				// ok: default validation still carries on as float64
				// TODO: should be clear in message that this is a default value [minor defect]
				//expectedMessage{"inlineMinInt.default in query should be less than or equal to 1", true, false},
				expectedMessage{"inlineMinInt in query should be less than or equal to 1", true, false},
				// ok
				expectedMessage{"definitions.myId.uint64.default in body should be less than or equal to 0", true, false},
				expectedMessage{"definitions.myId.uint8.default in body should be less than or equal to 255", true, false},
				// TODO: missing validation of boundaries when no default value to trigger the test
			},
		},
		"fixture-581-inline-param-format.yaml": expectedFixture{
			comment:           "",
			todo:              "",
			expectedLoadError: false,
			expectedValid:     false,
			expectedWarnings: []expectedMessage{
				expectedMessage{"definition \"#/definitions/myId\" is not used anywhere", true, false},
				expectedMessage{"inlineMaxInt in query has a default but no valid schema", true, false},
				expectedMessage{"inlineInfiniteInt2 in query has a default but no valid schema", true, false},
				expectedMessage{"negFactor2 in query has a default but no valid schema", true, false},
				expectedMessage{"inlineInfiniteInt in query has a default but no valid schema", true, false},
				expectedMessage{"negFactor in query has a default but no valid schema", true, false},
				expectedMessage{"negFactor3 in query has a default but no valid schema", true, false},
				expectedMessage{"inlineMinInt in query has a default but no valid schema", true, false},
			},
			expectedMessages: []expectedMessage{
				// ok
				expectedMessage{"default value for inlineInfiniteInt in query does not validate its schema", true, false},
				expectedMessage{"default value for inlineInfiniteInt2 in query does not validate its schema", true, false},
				expectedMessage{"default value for inlineMaxInt in query does not validate its schema", true, false},
				expectedMessage{"default value for inlineMinInt in query does not validate its schema", true, false},
				expectedMessage{"default value for negFactor in query does not validate its schema", true, false},
				expectedMessage{"default value for negFactor2 in query does not validate its schema", true, false},
				expectedMessage{"default value for negFactor3 in query does not validate its schema", true, false},
				// Ok
				expectedMessage{"\"paths./fixture.get.parameters\" must validate one and only one schema (oneOf). Found none valid", false, false},
				// ok
				expectedMessage{"Checked value must be of type integer with format uint32 in inlineInfiniteInt", true, false},
				// ok: default value check proceeds with fallback on float64
				expectedMessage{"inlineInfiniteInt in query should be greater than or equal to 0", true, false},
				expectedMessage{"Checked value must be of type integer with format uint32 in negFactor3", true, false},
				// ok : factor is checked as number since value does not verify format
				// TODO: better message in errors pkg
				expectedMessage{"negFactor in query must be of type number, because: factor in multipleOf must be positive: -300", true, false},
				// ok: constraint verified
				expectedMessage{"negFactor2 in query should be a multiple of 3", true, false},
				// ok
				expectedMessage{"Minimum boundary value must be of type integer with format uint64 in inlineMaxInt", true, false},
				expectedMessage{"Maximum boundary value must be of type integer with format uint64 in inlineMaxInt", true, false},
				// ok
				// ok with regular negative value
				expectedMessage{"Minimum boundary value must be of type integer with format uint32 in inlineMinInt", true, false},
				expectedMessage{"Maximum boundary value must be of type integer with format uint32 in inlineMinInt", true, false},
				// ok
				expectedMessage{"definitions.myId.uint64.default in body should be less than or equal to 0", true, false},
				expectedMessage{"definitions.myId.uint8.default in body should be less than or equal to 255", true, false},
				expectedMessage{"Checked value must be of type integer with format uint32 in inlineInfiniteInt2", true, false},
				// TODO: should be more accurate that it is a default
				expectedMessage{"inlineInfiniteInt2 in query should be greater than or equal to 0", true, false},
				//expectedMessage{"inlineInfiniteInt2.default in query should be greater than or equal to 0", true, false},
				// TODO: missing validation of boundaries when no default value to trigger the test
			},
		},
		"fixture-581.yaml": expectedFixture{
			comment:           "Issue#581 : value and type checking in constraints",
			todo:              "issue#581 not solved since only inline params are subject to this validation",
			expectedLoadError: false,
			expectedValid:     false,
			expectedWarnings: []expectedMessage{
				expectedMessage{"definition \"#/definitions/myId\" is not used anywhere", true, false},
			},
			expectedMessages: []expectedMessage{
				expectedMessage{"\"paths./fixture.get.parameters\" must validate one and only one schema (oneOf). Found none valid", false, false},
				expectedMessage{"definitions.myId.uint8.default in body should be less than or equal to 255", true, false},
				// TODO: issue#581 not solved since only inline params are subject to this validation
			},
		},
		"fixture-581-good.yaml": expectedFixture{
			comment:           "",
			todo:              "",
			expectedLoadError: false,
			expectedValid:     true,
			expectedWarnings:  []expectedMessage{},
			expectedMessages:  []expectedMessage{},
		},
		"fixture-581-good-numbers.yaml": expectedFixture{
			comment:           "",
			todo:              "",
			expectedLoadError: false,
			expectedValid:     true,
			expectedWarnings:  []expectedMessage{},
			expectedMessages:  []expectedMessage{},
		},
		// Checking examples validation
		"fixture-valid-example-property.yaml": expectedFixture{
			comment:           "",
			todo:              "",
			expectedLoadError: false,
			expectedValid:     true,
			expectedWarnings:  []expectedMessage{},
			expectedMessages:  []expectedMessage{},
		},
		"fixture-invalid-example-property.yaml": expectedFixture{
			comment:           "",
			todo:              "",
			expectedLoadError: false,
			// TODO: this example should be detected as invalid (issue#1231)
			expectedValid:    true,
			expectedWarnings: []expectedMessage{},
			expectedMessages: []expectedMessage{},
		},
		"fixture-1231.yaml": expectedFixture{
			comment:           "",
			todo:              "",
			expectedLoadError: false,
			expectedValid:     false,
			expectedWarnings:  []expectedMessage{},
			expectedMessages: []expectedMessage{
				expectedMessage{"\"parameters.customerIdParam\" must validate one and only one schema (oneOf). Found none valid", false, false},
				// ContinueOnErrors
				expectedMessage{"/v1/broker/{customer_id}.id in body must be of type uuid: \"mycustomer\"", true, false},
				expectedMessage{"/v1/broker/{customer_id}.create_date in body must be of type date-time: \"bad-date\"", true, false},
			},
		},
		"fixture-1171.yaml": expectedFixture{
			comment:           "An invalid array definition",
			todo:              "Missing check on $ref sibling",
			expectedLoadError: false,
			expectedValid:     false,
			expectedWarnings:  []expectedMessage{},
			expectedMessages: []expectedMessage{
				expectedMessage{"\"paths./servers/{server_id}/zones.get.parameters\" must validate one and only one schema (oneOf). Found none valid", false, false},
				expectedMessage{"\"paths./server/getBody.get.parameters\" must validate one and only one schema (oneOf). Found none valid", false, false},
				expectedMessage{"\"paths./server/getBody.get.responses.200\" must validate one and only one schema (oneOf). Found none valid", false, false},
				expectedMessage{"\"paths./server/getBody.get.responses.200.schema\" must validate one and only one schema (oneOf). Found none valid", false, false},
				expectedMessage{"paths./server/getBody.get.responses.200.schema.properties.name in body must be of type object: \"string\"", false, false},
				expectedMessage{"paths./server/getBody.get.responses.200.schema.properties.$ref in body must be of type object: \"string\"", false, false},
				expectedMessage{"paths./server/getBody.get.responses.200.description in body is required", false, false},
				expectedMessage{"items in definitions.Zones is required", false, false},
				expectedMessage{"\"definitions.InvalidZone.items\" must validate at least one schema (anyOf)", false, false},
				expectedMessage{"definitions.InvalidZone.items.name in body is a forbidden property", false, false},
				// ContinueOnErrors...
				expectedMessage{"path param \"other_server_id\" is not present in path \"/servers/{server_id}/zones\"", true, false},
				expectedMessage{"operation \"getBody\" has more than 1 body param: [\"\" \"yet_other_server_id\"]", true, false},
				expectedMessage{"body param \"yet_other_server_id\" for \"getBody\" is a collection without an element type (array requires items definition)", true, false},
				expectedMessage{"in operation \"listZones\",path param \"other_server_id\" must be declared as required", true, false},
				// Only when continue on errors true
				// TODO: missing here $ref sibling constraint
			},
		},
		"fixture-1238.yaml": expectedFixture{
			comment:           "",
			todo:              "",
			expectedLoadError: false,
			expectedValid:     false,
			expectedWarnings:  []expectedMessage{},
			expectedMessages: []expectedMessage{
				expectedMessage{"definitions.RRSets in body must be of type array", false, false},
			},
		},
		"fixture-1243.yaml": expectedFixture{
			comment:           "",
			todo:              "",
			expectedLoadError: false,
			expectedValid:     false,
			expectedWarnings:  []expectedMessage{},
			expectedMessages: []expectedMessage{
				// ok
				expectedMessage{"\"paths./loadBalancers/{loadBalancerId}/backendSets.get.responses.200\" must validate one and only one schema (oneOf). Found none valid", false, false},
				// ok
				expectedMessage{"paths./loadBalancers/{loadBalancerId}/backendSets.get.responses.200.headers.opc-response-id.$ref in body is a forbidden property", false, false},
				// ok
				expectedMessage{"paths./loadBalancers/{loadBalancerId}/backendSets.get.responses.200.headers.opc-response-id.type in body is required", false, false},
				// ok
				expectedMessage{"path param \"{loadBalancerId}\" has no parameter definition", true, false},
				// ok
				expectedMessage{"in \"paths./loadBalancers/{loadBalancerId}/backendSets.get.responses.200\": $ref are not allowed in headers. In context for header \"opc-response-id\", one may not use $ref=\":#/x-descriptions/opc-response-id\"", false, false},
			},
		},
		"fixture-1243-2.yaml": expectedFixture{
			comment:           "",
			todo:              "",
			expectedLoadError: false,
			expectedValid:     false,
			expectedWarnings:  []expectedMessage{},
			expectedMessages: []expectedMessage{
				expectedMessage{"path param \"{loadBalancerId}\" has no parameter definition", false, false},
			},
		},
		"fixture-1243-3.yaml": expectedFixture{
			comment:           "",
			todo:              "",
			expectedLoadError: false,
			expectedValid:     false,
			expectedWarnings:  []expectedMessage{},
			expectedMessages: []expectedMessage{
				// TODO: unstable message?
				//expectedMessage{"path param \"{loadBalancerId}\" has no parameter definition", false, false},
				expectedMessage{"\"paths./loadBalancers/{loadBalancerId}/backendSets.get.parameters\" must validate one and only one schema (oneOf). Found none valid", false, false},
				// ContinueOnErrors
				expectedMessage{"in operation \"ListBackendSets\",path param \"loadBalancerId\" must be declared as required", true, false},
			},
		},
		"fixture-1243-4.yaml": expectedFixture{
			comment:           "Check garbled path strings",
			todo:              "",
			expectedLoadError: false,
			expectedValid:     false,
			expectedWarnings: []expectedMessage{
				expectedMessage{"in path \"/othercheck/{sid }/warnMe\", param \"{sid }\" contains {,} or white space. Albeit not stricly illegal, this is probably no what you want", true, false},
				expectedMessage{"path stripped from path parameters /othercheck/{X/warnMe contains {,} or white space. This is probably no what you want.", true, false},
				expectedMessage{"path stripped from path parameters /othercheck/{si/d}warnMe contains {,} or white space. This is probably no what you want.", true, false},
			},
			expectedMessages: []expectedMessage{
				// ok
				expectedMessage{"\"paths./loadBalancers/{aLotOfLoadBalancerIds}/backendSets.get.parameters\" must validate one and only one schema (oneOf). Found none valid", false, false},
				// ok
				// ContinueOnErrors
				expectedMessage{"path param \"{aLotOfLoadBalancerIds}\" has no parameter definition", true, false},
				expectedMessage{"path /loadBalancers/{aLotOfLoadBalancerIds}/backendSets overlaps with /loadBalancers/{loadBalancerId}/backendSets", true, false},
				expectedMessage{"path param \"{sid }\" has no parameter definition", true, false},
				expectedMessage{"path param \"sid\" is not present in path \"/othercheck/{si/d}warnMe\"", true, false},
				expectedMessage{"path param \"sid\" is not present in path \"/othercheck/{sid }/warnMe\"", true, false},
			},
		},
		"fixture-1243-5.yaml": expectedFixture{
			comment:           "",
			todo:              "",
			expectedLoadError: false,
			expectedValid:     false,
			expectedWarnings: []expectedMessage{
				//ok
				expectedMessage{"in path \"/othercheck/{sid }/warnMe\", param \"{sid }\" contains {,} or white space. Albeit not stricly illegal, this is probably no what you want", true, false},
				expectedMessage{"path stripped from path parameters /othercheck/{X/warnMe contains {,} or white space. This is probably no what you want.", true, false},
				expectedMessage{"path stripped from path parameters /othercheck/{si/d}warnMe contains {,} or white space. This is probably no what you want.", true, false},
			},
			expectedMessages: []expectedMessage{
				// ok
				expectedMessage{"\"paths./loadBalancers/{aLotOfLoadBalancerIds}/backendSets.get.parameters\" must validate one and only one schema (oneOf). Found none valid", false, false},
				//ok
				expectedMessage{"\"paths./othercheck/{{sid}/warnMe.get.parameters\" must validate one and only one schema (oneOf). Found none valid", false, false},
				//ok
				expectedMessage{"\"paths./othercheck/{sid }/warnMe.get.parameters\" must validate one and only one schema (oneOf). Found none valid", false, false},
				//ok
				expectedMessage{"\"paths./othercheck/{si/d}warnMe.get.parameters\" must validate one and only one schema (oneOf). Found none valid", false, false},
				//ok
				// ContinueOnErrors
				expectedMessage{"path param \"{aLotOfLoadBalancerIds}\" has no parameter definition", true, false},
				//ok
				expectedMessage{"path param \"{sid}\" has no parameter definition", true, false},
				//ok
				expectedMessage{"path param \"{sid }\" has no parameter definition", true, false},
				//ok
				expectedMessage{"path /loadBalancers/{aLotOfLoadBalancerIds}/backendSets overlaps with /loadBalancers/{loadBalancerId}/backendSets", true, false},
			},
		},
		// Interesting to see how json supports more string garbling cases
		"fixture-1243-5.json": expectedFixture{
			comment:           "",
			todo:              "",
			expectedLoadError: false,
			expectedValid:     false,
			expectedWarnings: []expectedMessage{
				expectedMessage{"in path \"/othercheck/{sid }/warnMe\", param \"{sid }\" contains {,} or white space. Albeit not stricly illegal, this is probably no what you want", false, false},
				expectedMessage{"path stripped from path parameters /othercheck/{X/warnMe contains {,} or white space. This is probably no what you want.", false, false},
				expectedMessage{"path stripped from path parameters /othercheck/{si/d}warnMe contains {,} or white space. This is probably no what you want.", false, false},
			},
			expectedMessages: []expectedMessage{
				// ok
				expectedMessage{"path param \"{sid}\" has no parameter definition", false, false},
				expectedMessage{"path param \"{sid\" is not present in path \"/othercheck/{{sid}/warnMe\"", false, false},
				// ok
				expectedMessage{"path param \"sid\" is not present in path \"/othercheck/{si/d}warnMe\"", false, false},
				expectedMessage{"path /loadBalancers/{aLotOfLoadBalancerIds}/backendSets overlaps with /loadBalancers/{loadBalancerId}/backendSets", false, false},
			},
		},
		// Load error: incomplete JSON into a yaml file ...
		"fixture-1289-donotload.yaml": expectedFixture{
			comment:           "",
			todo:              "",
			expectedLoadError: true,
			expectedValid:     false,
			expectedWarnings:  []expectedMessage{},
			expectedMessages: []expectedMessage{
				expectedMessage{`.*yaml:.+`, false, true},
			},
		},
		// Load error: incomplete JSON ...
		"fixture-1289-donotload.json": expectedFixture{
			comment:           "",
			todo:              "",
			expectedLoadError: true,
			expectedValid:     false,
			expectedWarnings:  []expectedMessage{},
			expectedMessages: []expectedMessage{
				// Since this error is provided by an external package (go-openapi/loads, only asserts the surface of things)
				expectedMessage{`.*yaml:.+`, false, true},
			},
		},
		"fixture-1289.yaml": expectedFixture{
			comment:           "",
			todo:              "",
			expectedLoadError: false,
			expectedValid:     false,
			expectedWarnings:  []expectedMessage{},
			expectedMessages: []expectedMessage{
				//TODO: more explicit message (warning about siblings)
				expectedMessage{"items in definitions.getSomeIds.properties.someIds is required", false, false},
			},
		},
		"fixture-1289-good.yaml": expectedFixture{
			comment:           "",
			todo:              "",
			expectedLoadError: false,
			expectedValid:     true,
			expectedWarnings:  []expectedMessage{},
			expectedMessages:  []expectedMessage{},
		},
		"fixture-1243-good.yaml": expectedFixture{
			comment:           "",
			todo:              "",
			expectedLoadError: false,
			expectedValid:     true,
			expectedWarnings:  []expectedMessage{},
			expectedMessages:  []expectedMessage{},
		},
		"fixture-1050.yaml": expectedFixture{
			comment:           "Valid spec: fix issue#1050 (dot separated path params)",
			todo:              "",
			expectedLoadError: false,
			expectedValid:     true,
			expectedWarnings:  []expectedMessage{},
			expectedMessages:  []expectedMessage{},
		},
		"petstore-expanded.json": expectedFixture{
			comment:           "Fail Ref expansion in ContinueOnErrors mode panics",
			todo:              "",
			expectedLoadError: false,
			expectedValid:     false,
			expectedWarnings:  []expectedMessage{},
			expectedMessages: []expectedMessage{
				expectedMessage{"invalid ref \"pet\"", false, false},
				expectedMessage{"could not resolve reference in newPet to $ref pet: open /home/ubuntu/thefundschain/poc.1.0.2/src/github.com/go-openapi/validate/fixtures/validation/pet: no such file or directory", true, false},
			},
		},
	}

	//Debug = true
	err := filepath.Walk(filepath.Join("fixtures", "validation"),
		func(path string, info os.FileInfo, err error) error {
			_, found := tested[info.Name()]
			errs := 0
			if !info.IsDir() && found && tested[info.Name()].expectedValid == false {
				// Checking invalid specs
				t.Logf("Testing messages for invalid spec: %s", path)
				if DebugTest {
					if tested[info.Name()].comment != "" {
						t.Logf("\tDEVMODE: Comment: %s", tested[info.Name()].comment)
					}
					if tested[info.Name()].todo != "" {
						t.Logf("\tDEVMODE: Todo: %s", tested[info.Name()].todo)
					}
				}
				doc, err := loads.Spec(path)

				// Check specs with load errors (error is located in pkg loads or spec)
				if tested[info.Name()].expectedLoadError == true {
					// Expect a load error: no further validation may possibly be conducted.
					if assert.Error(t, err, "Expected this spec to return a load error") {
						errs += verifyLoadErrors(t, err, tested[info.Name()].expectedMessages)
						if errs == 0 {
							// spec does not load as expected
							return nil
						}
					} else {
						errs++
					}
				}
				if errs > 0 {
					if haltOnErrors {
						return fmt.Errorf("Test halted: stop on error mode")
					}
					return nil
				}

				if assert.NoError(t, err, "Expected this spec to load properly") {
					// Validate the spec document
					validator := NewSpecValidator(doc.Schema(), strfmt.Default)
					res, warn := validator.Validate(doc)

					// Check specs with load errors (error is located in pkg loads or spec)
					if !assert.False(t, res.IsValid(), "Expected this spec to be invalid") {
						errs++
					}

					verifyErrorsVsWarnings(t, res, warn)
					errs += verifyErrors(t, res, tested[info.Name()].expectedMessages, "error")
					errs += verifyErrors(t, warn, tested[info.Name()].expectedWarnings, "warning")

					// DEVMODE allows developers to experiment and tune expected results
					if DebugTest && errs > 0 {
						reportTest(t, path, res, tested[info.Name()].expectedMessages, "error")
						reportTest(t, path, warn, tested[info.Name()].expectedWarnings, "warning")
					}
				} else {
					errs++
				}

				if errs > 0 {
					t.Logf("Message qualification on Spec validation failed for %s", path)
				}
			} else {
				// Expecting no message (e.g.valid spec): 0 message expected
				if !info.IsDir() && found && tested[info.Name()].expectedValid {
					t.Logf("Testing valid spec: %s", path)
					if DebugTest {
						if tested[info.Name()].comment != "" {
							t.Logf("\tDEVMODE: Comment: %s", tested[info.Name()].comment)
						}
						if tested[info.Name()].todo != "" {
							t.Logf("\tDEVMODE: Todo: %s", tested[info.Name()].todo)
						}
					}
					doc, err := loads.Spec(path)
					if assert.NoError(t, err, "Expected this spec to load without error") {
						validator := NewSpecValidator(doc.Schema(), strfmt.Default)
						res, warn := validator.Validate(doc)
						if !assert.True(t, res.IsValid(), "Expected this spec to be valid") {
							errs++
						}
						errs += verifyErrors(t, warn, tested[info.Name()].expectedWarnings, "warning")
						if DebugTest && errs > 0 {
							reportTest(t, path, res, tested[info.Name()].expectedMessages, "error")
							reportTest(t, path, warn, tested[info.Name()].expectedWarnings, "warning")
						}
					} else {
						errs++
					}
				}
			}
			if haltOnErrors && errs > 0 {
				return fmt.Errorf("Test halted: stop on error mode")
			}
			return nil
		})
	if err != nil {
		t.Logf("%v", err)
		t.Fail()
	}
}

// Prints out a recap of error messages. To be enabled during development / test iterations
func reportTest(t *testing.T, path string, res *Result, expectedMessages []expectedMessage, msgtype string) {
	var verifiedErrors, lines []string
	for _, e := range res.Errors {
		verifiedErrors = append(verifiedErrors, e.Error())
	}
	t.Logf("DEVMODE:Recap of returned %s messages while validating %s ", msgtype, path)
	for _, v := range verifiedErrors {
		status := fmt.Sprintf("Unexpected %s", msgtype)
		for _, s := range expectedMessages {
			if (s.withContinueOnErrors == true && continueOnErrors == true) || s.withContinueOnErrors == false {
				if s.isRegexp {
					if matched, _ := regexp.MatchString(s.message, v); matched {
						status = fmt.Sprintf("Expected %s", msgtype)
						break
					}
				} else {
					if strings.Contains(v, s.message) {
						status = fmt.Sprintf("Expected %s", msgtype)
						break
					}
				}
			}
		}
		lines = append(lines, fmt.Sprintf("[%s]%s", status, v))
	}

	for _, s := range expectedMessages {
		if (s.withContinueOnErrors == true && continueOnErrors == true) || s.withContinueOnErrors == false {
			status := fmt.Sprintf("Missing %s", msgtype)
			for _, v := range verifiedErrors {
				if s.isRegexp {
					if matched, _ := regexp.MatchString(s.message, v); matched {
						status = fmt.Sprintf("Expected %s", msgtype)
						break
					}
				} else {
					if strings.Contains(v, s.message) {
						status = fmt.Sprintf("Expected %s", msgtype)
						break
					}
				}
			}
			if status != fmt.Sprintf("Expected %s", msgtype) {
				lines = append(lines, fmt.Sprintf("[%s]%s", status, s.message))
			}
		}
	}
	if len(lines) > 0 {
		sort.Strings(lines)
		for _, line := range lines {
			t.Logf(line)
		}
	}
}

func verifyErrorsVsWarnings(t *testing.T, res, warn *Result) {
	// First verification of result conventions: results are redundant, just a matter of presentation
	w := len(warn.Errors)
	assert.Len(t, res.Warnings, w)
	assert.Len(t, warn.Warnings, 0)
	assert.Subset(t, res.Warnings, warn.Errors)
	assert.Subset(t, warn.Errors, res.Warnings)
}

func verifyErrors(t *testing.T, res *Result, expectedMessages []expectedMessage, msgtype string) (errs int) {
	var verifiedErrors []string
	var numExpected int

	for _, e := range res.Errors {
		verifiedErrors = append(verifiedErrors, e.Error())
	}
	for _, s := range expectedMessages {
		if (s.withContinueOnErrors == true && continueOnErrors == true) || s.withContinueOnErrors == false {
			numExpected++
		}
	}

	// We got the expected number of messages (e.g. no duplicates, no uncontrolled side-effect, ...)
	if !assert.Len(t, verifiedErrors, numExpected, "Unexpected number of %s messages returned. Wanted %d, got %d", msgtype, numExpected, len(verifiedErrors)) {
		errs++
	}

	// Check that all expected messages are here
	for _, s := range expectedMessages {
		found := false
		if (s.withContinueOnErrors == true && continueOnErrors == true) || s.withContinueOnErrors == false {
			for _, v := range verifiedErrors {
				if s.isRegexp {
					if matched, _ := regexp.MatchString(s.message, v); matched {
						found = true
						break
					}
				} else {
					if strings.Contains(v, s.message) {
						found = true
						break
					}
				}
			}
			if !assert.True(t, found, "Missing expected %s message: %s", msgtype, s.message) {
				errs++
			}
		}
	}

	// Check for no unexpected message
	for _, v := range verifiedErrors {
		found := false
		for _, s := range expectedMessages {
			if (s.withContinueOnErrors == true && continueOnErrors == true) || s.withContinueOnErrors == false {
				if s.isRegexp {
					if matched, _ := regexp.MatchString(s.message, v); matched {
						found = true
						break
					}
				} else {
					if strings.Contains(v, s.message) {
						found = true
						break
					}
				}
			}
		}
		if !assert.True(t, found, "Unexpected %s message: %s", msgtype, v) {
			errs++
		}
	}
	return
}

// Perform several matchedes on single error message
// Process here error messages from loads (normally unit tested in the load package:
// we just want to figure out how all this is captured at the validate package level.
func verifyLoadErrors(t *testing.T, err error, expectedMessages []expectedMessage) (errs int) {
	v := err.Error()
	for _, s := range expectedMessages {
		found := false
		if s.isRegexp {
			if matched, _ := regexp.MatchString(s.message, v); matched {
				found = true
				break
			}
		} else {
			if strings.Contains(v, s.message) {
				found = true
				break
			}
		}
		if !assert.True(t, found, "Unexpected load error: %s", v) {
			errs++
		}
	}
	return
}
