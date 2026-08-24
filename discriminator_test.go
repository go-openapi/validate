// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/go-openapi/analysis"
	"github.com/go-openapi/loads"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"
)

// discriminatorSpec builds a specification holding the given definitions and nothing else of note.
func discriminatorSpec(definitions string) string {
	return fmt.Sprintf(`{
		"swagger": "2.0",
		"info": {"title": "t", "version": "1"},
		"paths": {},
		"definitions": %s
	}`, definitions)
}

// discriminatorValidatorFromJSON builds a SpecValidator wired the way Validate does, up to the
// point validateDiscriminators needs.
func discriminatorValidatorFromJSON(t *testing.T, doc string) *SpecValidator {
	t.Helper()

	d, err := loads.Analyzed(json.RawMessage(doc), "")
	require.NoError(t, err)

	s := NewSpecValidator(d.Schema(), strfmt.Default)
	s.spec = d
	s.analyzer = analysis.New(d.Spec())

	return s
}

func TestValidateDiscriminators(t *testing.T) {
	t.Parallel()

	const (
		petTypeNotDefined  = `discriminator "petType" of "Pet" is not defined as a property of that schema`
		petTypeNotRequired = `discriminator "petType" of "Pet" is not in the required property list`
	)

	tests := []struct {
		name       string
		defs       string
		wantErrors []string
	}{
		{
			name: "discriminator defined and required",
			defs: `{
				"Pet": {
					"type": "object",
					"discriminator": "petType",
					"required": ["petType"],
					"properties": {"petType": {"type": "string"}}
				}
			}`,
		},
		{
			name: "no discriminator at all",
			defs: `{"Pet": {"type": "object", "properties": {"name": {"type": "string"}}}}`,
		},
		{
			name: "discriminator names an undeclared property",
			defs: `{
				"Pet": {
					"type": "object",
					"discriminator": "petType",
					"required": ["petType"],
					"properties": {"name": {"type": "string"}}
				}
			}`,
			wantErrors: []string{petTypeNotDefined},
		},
		{
			name: "discriminator property is optional",
			defs: `{
				"Pet": {
					"type": "object",
					"discriminator": "petType",
					"properties": {"petType": {"type": "string"}}
				}
			}`,
			wantErrors: []string{petTypeNotRequired},
		},
		{
			name: "discriminator neither defined nor required reports both",
			defs: `{
				"Pet": {
					"type": "object",
					"discriminator": "petType",
					"properties": {"name": {"type": "string"}}
				}
			}`,
			wantErrors: []string{
				petTypeNotDefined,
				petTypeNotRequired,
			},
		},
		{
			name: "property and required contributed by an allOf $ref",
			defs: `{
				"Base": {
					"type": "object",
					"required": ["petType"],
					"properties": {"petType": {"type": "string"}}
				},
				"Pet": {
					"discriminator": "petType",
					"allOf": [
						{"$ref": "#/definitions/Base"},
						{"type": "object", "properties": {"name": {"type": "string"}}}
					]
				}
			}`,
		},
		{
			name: "property contributed by an inline allOf member",
			defs: `{
				"Pet": {
					"discriminator": "petType",
					"allOf": [
						{"type": "object", "required": ["petType"], "properties": {"petType": {"type": "string"}}}
					]
				}
			}`,
		},
		{
			name: "allOf contributes the property but nothing requires it",
			defs: `{
				"Base": {"type": "object", "properties": {"petType": {"type": "string"}}},
				"Pet": {
					"discriminator": "petType",
					"allOf": [{"$ref": "#/definitions/Base"}]
				}
			}`,
			wantErrors: []string{petTypeNotRequired},
		},
		{
			name: "several definitions are all checked, in name order",
			defs: `{
				"Alpha": {"type": "object", "discriminator": "kind", "required": ["kind"]},
				"Beta": {"type": "object", "discriminator": "sort", "required": ["sort"]}
			}`,
			wantErrors: []string{
				`discriminator "kind" of "Alpha" is not defined as a property of that schema`,
				`discriminator "sort" of "Beta" is not defined as a property of that schema`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			res := discriminatorValidatorFromJSON(t, discriminatorSpec(tc.defs)).validateDiscriminators()
			assert.Equal(t, tc.wantErrors, nonEmpty(errorMessages(res)))
			assert.Empty(t, warningMessages(res), "the rule reports errors only")
		})
	}
}

// TestDiscriminatorInNestedSchema covers a discriminator that sits below a definition rather than
// on it: the same slip, wherever it is written.
func TestDiscriminatorInNestedSchema(t *testing.T) {
	t.Parallel()

	doc := discriminatorSpec(`{
		"Zoo": {
			"type": "object",
			"properties": {
				"resident": {
					"type": "object",
					"discriminator": "petType",
					"properties": {"name": {"type": "string"}}
				}
			}
		}
	}`)

	res := discriminatorValidatorFromJSON(t, doc).validateDiscriminators()
	assert.Equal(t, []string{
		`discriminator "petType" of "Zoo.resident" is not defined as a property of that schema`,
		`discriminator "petType" of "Zoo.resident" is not in the required property list`,
	}, errorMessages(res))
}

// TestDiscriminatorBehindRefCheckedOnce guards the choice not to follow a $ref: the fault is
// reported where the definition is written, not again at every schema pointing at it.
func TestDiscriminatorBehindRefCheckedOnce(t *testing.T) {
	t.Parallel()

	doc := discriminatorSpec(`{
		"Pet": {"type": "object", "discriminator": "petType"},
		"First": {"$ref": "#/definitions/Pet"},
		"Second": {"type": "object", "properties": {"pet": {"$ref": "#/definitions/Pet"}}}
	}`)

	res := discriminatorValidatorFromJSON(t, doc).validateDiscriminators()
	assert.Equal(t, []string{
		`discriminator "petType" of "Pet" is not defined as a property of that schema`,
		`discriminator "petType" of "Pet" is not in the required property list`,
	}, errorMessages(res))
}

// TestDiscriminatorRecursiveDefinitionTerminates guards the same choice against the reason it was
// made: a definition holding itself would not terminate if the walk followed $ref.
func TestDiscriminatorRecursiveDefinitionTerminates(t *testing.T) {
	t.Parallel()

	doc := discriminatorSpec(`{
		"Node": {
			"type": "object",
			"discriminator": "kind",
			"required": ["kind"],
			"properties": {
				"kind": {"type": "string"},
				"child": {"$ref": "#/definitions/Node"}
			}
		}
	}`)

	res := discriminatorValidatorFromJSON(t, doc).validateDiscriminators()
	assert.Empty(t, errorMessages(res))
}

// TestDiscriminatorLocations pins down where a discriminator finding says it happened. The
// pointers come from a full Validate, which trims a location to a node the document holds.
func TestDiscriminatorLocations(t *testing.T) {
	t.Parallel()

	doc := discriminatorSpec(`{
		"Pet": {"type": "object", "discriminator": "petType", "required": ["petType"]},
		"Zoo": {
			"type": "object",
			"properties": {
				"resident": {"type": "object", "discriminator": "kind", "required": ["kind"]}
			}
		}
	}`)

	d, err := loads.Analyzed(json.RawMessage(doc), "")
	require.NoError(t, err)

	validator := NewSpecValidator(d.Schema(), strfmt.Default)
	validator.SetContinueOnErrors(true)
	errs, _ := validator.Validate(d)

	pointers := pointersOf(errs.LocatedErrors())
	assert.SliceContainsT(t, pointers, "/definitions/Pet/discriminator")
	assert.SliceContainsT(t, pointers, "/definitions/Zoo/properties/resident/discriminator")
}

// TestDiscriminatorMakesSpecInvalid checks that the rule reaches Spec, the package's front door,
// rather than only the validator method the other tests call.
func TestDiscriminatorMakesSpecInvalid(t *testing.T) {
	t.Parallel()

	doc := discriminatorSpec(`{"Pet": {"type": "object", "discriminator": "petType"}}`)

	d, err := loads.Analyzed(json.RawMessage(doc), "")
	require.NoError(t, err)

	err = Spec(d, strfmt.Default)
	require.Error(t, err)
	assert.StringContainsT(t, err.Error(), `discriminator "petType" of "Pet"`)
}
