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

// securitySpec builds a specification whose security definitions cover the three scheme shapes a
// requirement can name: an apiKey, a basic auth, and an oauth2 scheme declaring one scope.
//
// rootSecurity and opSecurity are spliced in as authored JSON so that a test can write an empty
// array, an empty requirement object, or no member at all.
func securitySpec(rootSecurity, opSecurity string) string {
	return fmt.Sprintf(`{
		"swagger": "2.0",
		"info": {"title": "t", "version": "1"},
		"securityDefinitions": {
			"api_key": {"type": "apiKey", "name": "api_key", "in": "header"},
			"basic_auth": {"type": "basic"},
			"petstore_auth": {
				"type": "oauth2",
				"flow": "implicit",
				"authorizationUrl": "https://example.com/auth",
				"scopes": {"read:pets": "read your pets"}
			}
		},
		%s
		"paths": {
			"/pets": {
				"get": {
					"responses": {"200": {"description": "ok"}}%s
				}
			}
		}
	}`, rootSecurity, opSecurity)
}

func rootSecurity(requirements string) string {
	return `"security": ` + requirements + `,`
}

func opSecurity(requirements string) string {
	return `, "security": ` + requirements
}

// securityValidatorFromJSON builds a SpecValidator wired the way Validate does, up to the point
// validateSecurityRequirements needs: the document and an analyzer over it.
func securityValidatorFromJSON(t *testing.T, doc string) *SpecValidator {
	t.Helper()

	d, err := loads.Analyzed(json.RawMessage(doc), "")
	require.NoError(t, err)

	s := NewSpecValidator(d.Schema(), strfmt.Default)
	s.spec = d
	s.analyzer = analysis.New(d.Spec())

	return s
}

func TestValidateSecurityRequirements(t *testing.T) {
	t.Parallel()

	const undeclaredScheme = `security requirement "unknown_scheme" is not declared in securityDefinitions`

	tests := []struct {
		name         string
		doc          string
		wantErrors   []string
		wantWarnings []string
	}{
		{
			name: "scheme declared in securityDefinitions",
			doc:  securitySpec(rootSecurity(`[{"api_key": []}]`), opSecurity(`[{"petstore_auth": ["read:pets"]}]`)),
		},
		{
			name:       "undeclared scheme at the document level",
			doc:        securitySpec(rootSecurity(`[{"unknown_scheme": []}]`), ""),
			wantErrors: []string{undeclaredScheme},
		},
		{
			name:       "undeclared scheme on an operation",
			doc:        securitySpec("", opSecurity(`[{"unknown_scheme": []}]`)),
			wantErrors: []string{undeclaredScheme},
		},
		{
			name: "scopes on an apiKey scheme",
			doc:  securitySpec("", opSecurity(`[{"api_key": ["read:pets"]}]`)),
			wantErrors: []string{
				`security requirement "api_key" lists scopes (read:pets), but the security scheme it names is of type "apiKey": only oauth2 requirements carry scopes`,
			},
		},
		{
			name: "scopes on a basic scheme, at the document level",
			doc:  securitySpec(rootSecurity(`[{"basic_auth": ["a", "b"]}]`), ""),
			wantErrors: []string{
				`security requirement "basic_auth" lists scopes (a, b), but the security scheme it names is of type "basic": only oauth2 requirements carry scopes`,
			},
		},
		{
			name: "oauth2 scope the scheme does not declare",
			doc:  securitySpec("", opSecurity(`[{"petstore_auth": ["read:pets", "write:pets"]}]`)),
			wantWarnings: []string{
				`security requirement "petstore_auth" requires scope "write:pets", which the security scheme does not declare`,
			},
		},
		{
			name: "empty requirement object makes security optional",
			doc:  securitySpec(rootSecurity(`[{"api_key": []}, {}]`), ""),
		},
		{
			name: "empty security array drops the document requirements",
			doc:  securitySpec(rootSecurity(`[{"api_key": []}]`), opSecurity(`[]`)),
		},
		{
			name: "no security member at all",
			doc:  securitySpec("", ""),
		},
		{
			name: "several requirements are all checked",
			doc:  securitySpec(rootSecurity(`[{"first_unknown": []}, {"second_unknown": []}]`), ""),
			wantErrors: []string{
				`security requirement "first_unknown" is not declared in securityDefinitions`,
				`security requirement "second_unknown" is not declared in securityDefinitions`,
			},
		},
		{
			name: "several schemes in one requirement are all checked",
			doc:  securitySpec("", opSecurity(`[{"api_key": ["nope"], "unknown_scheme": []}]`)),
			wantErrors: []string{
				`security requirement "api_key" lists scopes (nope), but the security scheme it names is of type "apiKey": only oauth2 requirements carry scopes`,
				undeclaredScheme,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			res := securityValidatorFromJSON(t, tc.doc).validateSecurityRequirements()
			assert.Equal(t, tc.wantErrors, nonEmpty(errorMessages(res)))
			assert.Equal(t, tc.wantWarnings, nonEmpty(warningMessages(res)))
		})
	}
}

// nonEmpty reports an empty list of messages as nil, so that a test case may leave its
// expectation out altogether.
func nonEmpty(messages []string) []string {
	if len(messages) == 0 {
		return nil
	}

	return messages
}

// TestSecurityRequirementLocations pins down where a security finding says it happened. The
// pointers are produced by a full Validate, which is what trims a location down to a node the
// document holds.
func TestSecurityRequirementLocations(t *testing.T) {
	t.Parallel()

	doc := securitySpec(
		rootSecurity(`[{"api_key": []}, {"unknown_scheme": []}]`),
		opSecurity(`[{"basic_auth": ["nope"]}, {"petstore_auth": ["write:pets"]}]`),
	)

	d, err := loads.Analyzed(json.RawMessage(doc), "")
	require.NoError(t, err)

	errs, warns := NewSpecValidator(d.Schema(), strfmt.Default).Validate(d)

	assert.SliceContainsT(t, pointersOf(errs.LocatedErrors()), "/security/1/unknown_scheme")
	assert.SliceContainsT(t, pointersOf(errs.LocatedErrors()), "/paths/~1pets/get/security/0/basic_auth")
	assert.SliceContainsT(t, pointersOf(warns.LocatedErrors()), "/paths/~1pets/get/security/1/petstore_auth")
}

// TestSecurityScopeWarningKeepsSpecValid guards the choice made for the third rule: an oauth2
// requirement naming an undeclared scope warns, and the specification stays valid.
func TestSecurityScopeWarningKeepsSpecValid(t *testing.T) {
	t.Parallel()

	doc := securitySpec("", opSecurity(`[{"petstore_auth": ["write:pets"]}]`))

	d, err := loads.Analyzed(json.RawMessage(doc), "")
	require.NoError(t, err)

	require.NoError(t, Spec(d, strfmt.Default))

	_, warns := NewSpecValidator(d.Schema(), strfmt.Default).Validate(d)
	assert.SliceContainsT(t, errorMessages(warns),
		`security requirement "petstore_auth" requires scope "write:pets", which the security scheme does not declare`)
}
