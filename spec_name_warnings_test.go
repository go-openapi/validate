// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"
)

// dubiousNamesDoc puts a line break in every kind of name validateDubiousNames checks.
// R and S reach D through a $ref: D is reported once, where it is declared.
const dubiousNamesDoc = `{
	"swagger": "2.0",
	"info": {"title": "t", "version": "1"},
	"tags": [{"name": "t\n1"}],
	"securityDefinitions": {
		"k\ny": {"type": "apiKey", "name": "X-K\ny", "in": "header"}
	},
	"parameters": {
		"p\nk": {"name": "gp\nn", "in": "query", "type": "string"}
	},
	"responses": {
		"r\nk": {"description": "d", "headers": {"h\n1": {"type": "string"}}}
	},
	"paths": {
		"/a\nb": {
			"parameters": [{"name": "pp\nn", "in": "query", "type": "string"}],
			"get": {
				"operationId": "op\nid",
				"tags": ["ot\n1"],
				"parameters": [{"name": "op\nn", "in": "header", "type": "string"}],
				"responses": {
					"200": {
						"description": "ok",
						"headers": {"h\r2": {"type": "string"}},
						"schema": {"type": "object", "properties": {"ip\nr": {"type": "string"}}}
					}
				}
			}
		}
	},
	"definitions": {
		"D\nx": {
			"type": "object",
			"discriminator": "k\nd",
			"required": ["k\nd"],
			"properties": {"k\nd": {"type": "string"}, "p\r": {"type": "string"}}
		},
		"R": {"$ref": "#/definitions/D%0Ax"},
		"S": {"type": "object", "properties": {"r": {"$ref": "#/definitions/D%0Ax"}}}
	}
}`

// dubiousNamesWant maps the pointer of each finding in dubiousNamesDoc to the kind of name the
// message must give.
var dubiousNamesWant = map[string]string{
	"/tags/0/name":                                            nameKindTag,
	"/securityDefinitions/k\ny":                               nameKindSecurityDefinition,
	"/securityDefinitions/k\ny/name":                          nameKindAPIKeyName,
	"/parameters/p\nk":                                        nameKindParameterKey,
	"/parameters/p\nk/name":                                   nameKindParameter,
	"/responses/r\nk":                                         nameKindResponseKey,
	"/responses/r\nk/headers/h\n1":                            nameKindHeader,
	"/paths/~1a\nb":                                           nameKindPath,
	"/paths/~1a\nb/parameters/0/name":                         nameKindParameter,
	"/paths/~1a\nb/get/operationId":                           nameKindOperationID,
	"/paths/~1a\nb/get/tags/0":                                nameKindTag,
	"/paths/~1a\nb/get/parameters/0/name":                     nameKindParameter,
	"/paths/~1a\nb/get/responses/200/headers/h\r2":            nameKindHeader,
	"/paths/~1a\nb/get/responses/200/schema/properties/ip\nr": nameKindProperty,
	"/definitions/D\nx":                                       nameKindDefinition,
	"/definitions/D\nx/discriminator":                         nameKindDiscriminator,
	"/definitions/D\nx/properties/k\nd":                       nameKindProperty,
	"/definitions/D\nx/properties/p\r":                        nameKindProperty,
}

func TestValidateDubiousNames(t *testing.T) {
	t.Run("every kind of name is flagged where it is declared", func(t *testing.T) {
		res := dubiousValidatorFromJSON(t, dubiousNamesDoc).validateDubiousNames()

		assertDubiousNames(t, res.LocatedWarnings())
		assert.Empty(t, res.Errors)
	})

	t.Run("names without a line break are not flagged", func(t *testing.T) {
		clean := strings.NewReplacer(`\n`, "_", `\r`, "_").Replace(dubiousNamesDoc)
		res := dubiousValidatorFromJSON(t, clean).validateDubiousNames()

		assert.Empty(t, res.Warnings)
	})

	t.Run("a line break in a description is not flagged", func(t *testing.T) {
		doc := `{
			"swagger": "2.0",
			"info": {"title": "t", "version": "1", "description": "line 1\nline 2"},
			"paths": {},
			"definitions": {"A": {"type": "string", "description": "a\nb", "enum": ["x\ny"]}}
		}`
		res := dubiousValidatorFromJSON(t, doc).validateDubiousNames()

		assert.Empty(t, res.Warnings)
	})

	t.Run("Validate reports the findings as warnings and keeps the spec valid", func(t *testing.T) {
		d, err := loads.Analyzed(json.RawMessage(dubiousNamesDoc), "")
		require.NoError(t, err)

		errs, warnings := NewSpecValidator(d.Schema(), strfmt.Default).Validate(d)
		require.Truef(t, errs.IsValid(), "unexpected errors: %v", errs.Errors)

		// Validate hands warnings back as the errors of its warnings-only result.
		// The other checks warn about this document too: keep only ours.
		var located []Located
		for _, l := range warnings.LocatedErrors() {
			if strings.Contains(l.Err.Error(), "contains a line break") {
				located = append(located, l)
			}
		}

		assertDubiousNames(t, located)
	})
}

func assertDubiousNames(t *testing.T, located []Located) {
	t.Helper()

	got := make(map[string]string, len(located))
	for _, l := range located {
		got[l.Pointer] = l.Err.Error()
	}

	require.Lenf(t, got, len(dubiousNamesWant), "warnings: %v", got)
	for pointer, kind := range dubiousNamesWant {
		msg, found := got[pointer]
		if assert.Truef(t, found, "no warning at %q", pointer) {
			assert.Truef(t, strings.HasPrefix(msg, kind+" "), "warning at %q: %q, want kind %q", pointer, msg, kind)
		}
	}
}
