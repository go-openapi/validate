// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
)

// definitionsDoc wraps a definitions block into the smallest document that
// reaches it, so that a case reads as the definitions it is about.
func definitionsDoc(definitions string) string {
	return `{"swagger":"2.0","info":{"title":"t","version":"1"},
 "paths":{"/a":{"get":{"operationId":"a","responses":{"200":{"description":"ok",
   "schema":{"$ref":"#/definitions/A"}}}}}},
 "definitions":` + definitions + `}`
}

// A required entry naming a property the schema never declares is a modelling
// slip wherever the schema sits, not only at the top of a definition.
func TestRequiredWalk_DescendsIntoNestedSchemas(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name        string
		definitions string
		// the message names the schema by the way down to it, so that it does
		// not send a reader to the definition holding it
		schema  string
		pointer string
	}{
		{
			name: "the schema of a property",
			definitions: `{"A":{"type":"object","properties":{
   "inner":{"type":"object","required":["ghost"],"properties":{"real":{"type":"string"}}}}}}`,
			schema:  "A.inner",
			pointer: "/definitions/A/properties/inner/required/0",
		},
		{
			name: "the schema of an array item",
			definitions: `{"A":{"type":"object","properties":{
   "list":{"type":"array","items":{"type":"object","required":["ghost"],"properties":{"real":{"type":"string"}}}}}}}`,
			schema:  "A.list.items",
			pointer: "/definitions/A/properties/list/items/required/0",
		},
		{
			name: "the schema of additionalProperties",
			definitions: `{"A":{"type":"object","additionalProperties":
   {"type":"object","required":["ghost"],"properties":{"real":{"type":"string"}}}}}`,
			schema:  "A.additionalProperties",
			pointer: "/definitions/A/additionalProperties/required/0",
		},
		{
			name: "a property schema nested in an allOf member",
			definitions: `{"A":{"allOf":[{"type":"object","properties":{
   "inner":{"type":"object","required":["ghost"],"properties":{"real":{"type":"string"}}}}}]}}`,
			schema:  "A.allOf.0.inner",
			pointer: "/definitions/A/allOf/0/properties/inner/required/0",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			found := locatedFindings(t, definitionsDoc(testCase.definitions))
			assert.EqualT(t, testCase.pointer,
				found[`"ghost" is present in required but not defined as property in schema "`+testCase.schema+`"`])
		})
	}
}

// Inside a composition, a required entry speaks of the instance the whole
// composition describes: it is legitimately met by a sibling member, or by no
// declaration at all, and data validation is what enforces it.
func TestRequiredWalk_LeavesCompositionAlone(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name        string
		definitions string
	}{
		{
			name: "allOf members each requiring what the other does not declare",
			definitions: `{"A":{"allOf":[
   {"type":"object","required":["a"]},
   {"type":"object","required":["b"]}]}}`,
		},
		{
			name: "a required entry declared by a sibling allOf member",
			definitions: `{"A":{"allOf":[
   {"type":"object","required":["a"]},
   {"type":"object","properties":{"a":{"type":"string"}}}]}}`,
		},
		{
			name: "a oneOf member requiring what it does not declare",
			definitions: `{"A":{"type":"object","properties":{
   "inner":{"oneOf":[{"type":"object","required":["a"]},{"type":"object","required":["b"]}]}}}}`,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			for message := range locatedFindings(t, definitionsDoc(testCase.definitions)) {
				assert.StringNotContainsT(t, message, "is present in required but not defined")
			}
		})
	}
}

// A property contributed by a base definition is declared just as plainly as
// one written in place.
func TestRequiredWalk_CountsPropertiesContributedByAllOf(t *testing.T) {
	t.Parallel()

	t.Run("declared by an allOf member written in place", func(t *testing.T) {
		t.Parallel()

		found := locatedFindings(t, definitionsDoc(`{"A":{"type":"object","required":["a"],
 "allOf":[{"type":"object","properties":{"a":{"type":"string"}}}]}}`))

		for message := range found {
			assert.StringNotContainsT(t, message, "is present in required but not defined")
		}
	})

	t.Run("declared by an allOf member written as a $ref", func(t *testing.T) {
		t.Parallel()

		found := locatedFindings(t, definitionsDoc(`{
 "Base":{"type":"object","properties":{"a":{"type":"string"}}},
 "A":{"type":"object","required":["a"],"allOf":[{"$ref":"#/definitions/Base"}]}}`))

		for message := range found {
			assert.StringNotContainsT(t, message, "is present in required but not defined")
		}
	})

	t.Run("still reported when nothing in the composition declares it", func(t *testing.T) {
		t.Parallel()

		found := locatedFindings(t, definitionsDoc(`{
 "Base":{"type":"object","properties":{"other":{"type":"string"}}},
 "A":{"type":"object","required":["a"],"allOf":[{"$ref":"#/definitions/Base"}]}}`))

		assert.EqualT(t, "/definitions/A/required/0",
			found[`"a" is present in required but not defined as property in definition "A"`])
	})
}

func TestRequiredWalk_QuietWhenTheSchemaIsSound(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name        string
		definitions string
	}{
		{
			name: "the nested property is declared",
			definitions: `{"A":{"type":"object","properties":{
   "inner":{"type":"object","required":["real"],"properties":{"real":{"type":"string"}}}}}}`,
		},
		{
			name: "the nested schema takes any property",
			definitions: `{"A":{"type":"object","properties":{
   "inner":{"type":"object","required":["anything"],"additionalProperties":true}}}}`,
		},
		{
			name: "the nested schema is a $ref, checked where it is defined",
			definitions: `{
 "Inner":{"type":"object","required":["real"],"properties":{"real":{"type":"string"}}},
 "A":{"type":"object","properties":{"inner":{"$ref":"#/definitions/Inner"}}}}`,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			for message := range locatedFindings(t, definitionsDoc(testCase.definitions)) {
				assert.StringNotContainsT(t, message, "is present in required but not defined")
			}
		})
	}
}

// A definition names itself, exactly as it always has.
func TestRequiredWalk_ADefinitionStillNamesItself(t *testing.T) {
	t.Parallel()

	found := locatedFindings(t, definitionsDoc(`{"A":{"type":"object","required":["ghost"],
 "properties":{"real":{"type":"string"}}}}`))

	assert.EqualT(t, "/definitions/A/required/0",
		found[`"ghost" is present in required but not defined as property in definition "A"`])
}

// A recursive definition is reached through a $ref, which the walk does not
// follow, so it cannot spin — and the slip is still reported once, where the
// definition writes it.
func TestRequiredWalk_ReportsARecursiveDefinitionOnce(t *testing.T) {
	t.Parallel()

	found := locatedFindings(t, definitionsDoc(`{"A":{"type":"object","required":["ghost"],
 "properties":{"self":{"$ref":"#/definitions/A"}}}}`))

	assert.EqualT(t, "/definitions/A/required/0",
		found[`"ghost" is present in required but not defined as property in definition "A"`])
}

// The readOnly warning follows the entry it is about, wherever that sits.
func TestRequiredWalk_ReadOnlyWarningIsLocatedToo(t *testing.T) {
	t.Parallel()

	found := locatedFindings(t, definitionsDoc(`{"A":{"type":"object","properties":{
 "inner":{"type":"object","required":["ro"],"properties":{"ro":{"type":"string","readOnly":true}}}}}}`))

	assert.EqualT(t, "/definitions/A/properties/inner/required/0",
		found[`Required property ro in "A.inner" should not be marked as both required and readOnly`])
}
