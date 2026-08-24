// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/go-openapi/analysis"
	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"
)

// collectionFormatSpec builds a specification whose only operation declares the given parameters
// and the given headers on its 200 response.
func collectionFormatSpec(parameters, headers string) string {
	return fmt.Sprintf(`{
		"swagger": "2.0",
		"info": {"title": "t", "version": "1"},
		"paths": {
			"/x": {
				"get": {
					"parameters": [%s],
					"responses": {"200": {"description": "ok", "headers": {%s}}}
				}
			}
		}
	}`, parameters, headers)
}

// collectionFormatValidatorFromJSON builds a SpecValidator wired the way Validate does, up to the
// point validateCollectionFormats needs.
func collectionFormatValidatorFromJSON(t *testing.T, doc string) *SpecValidator {
	t.Helper()

	d, err := loads.Analyzed(json.RawMessage(doc), "")
	require.NoError(t, err)

	s := NewSpecValidator(d.Schema(), strfmt.Default)
	s.spec = d
	s.analyzer = analysis.New(d.Spec())
	s.paramLocations = newParamLocations(d.Spec())

	return s
}

func TestValidateCollectionFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		parameters   string
		headers      string
		wantWarnings []string
	}{
		{
			name:       "collectionFormat on an array",
			parameters: `{"name":"q","in":"query","type":"array","items":{"type":"string"},"collectionFormat":"pipes"}`,
		},
		{
			name:       "array without a collectionFormat",
			parameters: `{"name":"q","in":"query","type":"array","items":{"type":"string"}}`,
		},
		{
			name:       "no collectionFormat anywhere",
			parameters: `{"name":"q","in":"query","type":"string"}`,
		},
		{
			name:       "collectionFormat on a string parameter",
			parameters: `{"name":"q","in":"query","type":"string","collectionFormat":"csv"}`,
			wantWarnings: []string{
				`collectionFormat "csv" is ignored in parameter "q": it joins the members of an array, and the type is "string"`,
			},
		},
		{
			name:       "collectionFormat on an integer parameter",
			parameters: `{"name":"q","in":"query","type":"integer","collectionFormat":"pipes"}`,
			wantWarnings: []string{
				`collectionFormat "pipes" is ignored in parameter "q": it joins the members of an array, and the type is "integer"`,
			},
		},
		{
			name:       "collectionFormat on a file parameter",
			parameters: `{"name":"f","in":"formData","type":"file","collectionFormat":"csv"}`,
			wantWarnings: []string{
				`collectionFormat "csv" is ignored in parameter "f": it joins the members of an array, and the type is "file"`,
			},
		},
		{
			name:       "collectionFormat on the items of an array",
			parameters: `{"name":"q","in":"query","type":"array","items":{"type":"string","collectionFormat":"ssv"}}`,
			wantWarnings: []string{
				`collectionFormat "ssv" is ignored in items of parameter "q": it joins the members of an array, and the type is "string"`,
			},
		},
		{
			name:       "collectionFormat on the items of an array of arrays",
			parameters: `{"name":"q","in":"query","type":"array","items":{"type":"array","items":{"type":"string"},"collectionFormat":"tsv"}}`,
		},
		{
			name:    "collectionFormat on a string response header",
			headers: `"X":{"type":"string","collectionFormat":"csv"}`,
			wantWarnings: []string{
				`collectionFormat "csv" is ignored in header "X": it joins the members of an array, and the type is "string"`,
			},
		},
		{
			name:    "collectionFormat on an array response header",
			headers: `"X":{"type":"array","items":{"type":"string"},"collectionFormat":"csv"}`,
		},
		{
			name:    "collectionFormat on the items of a response header",
			headers: `"X":{"type":"array","items":{"type":"integer","collectionFormat":"pipes"}}`,
			wantWarnings: []string{
				`collectionFormat "pipes" is ignored in items of header "X": it joins the members of an array, and the type is "integer"`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			doc := collectionFormatSpec(tc.parameters, tc.headers)
			res := collectionFormatValidatorFromJSON(t, doc).validateCollectionFormats()

			assert.Equal(t, tc.wantWarnings, nonEmpty(warningMessages(res)))
			assert.Empty(t, errorMessages(res), "the rule reports warnings only")
		})
	}
}

// TestCollectionFormatKeepsSpecValid holds the rule to a warning: Swagger 2.0 says where
// collectionFormat applies without forbidding it elsewhere, so a specification that writes one on a
// string stays valid.
func TestCollectionFormatKeepsSpecValid(t *testing.T) {
	t.Parallel()

	doc := collectionFormatSpec(`{"name":"q","in":"query","type":"string","collectionFormat":"csv"}`, "")

	d, err := loads.Analyzed(json.RawMessage(doc), "")
	require.NoError(t, err)

	require.NoError(t, Spec(d, strfmt.Default))

	_, warns := NewSpecValidator(d.Schema(), strfmt.Default).Validate(d)
	assert.SliceContainsT(t, errorMessages(warns),
		`collectionFormat "csv" is ignored in parameter "q": it joins the members of an array, and the type is "string"`)
}

// TestCollectionFormatLocations pins down where the warning says it happened.
func TestCollectionFormatLocations(t *testing.T) {
	t.Parallel()

	doc := collectionFormatSpec(
		`{"name":"q","in":"query","type":"array","items":{"type":"string","collectionFormat":"ssv"}}`,
		`"X":{"type":"string","collectionFormat":"csv"}`,
	)

	d, err := loads.Analyzed(json.RawMessage(doc), "")
	require.NoError(t, err)

	_, warns := NewSpecValidator(d.Schema(), strfmt.Default).Validate(d)

	pointers := pointersOf(warns.LocatedErrors())
	assert.SliceContainsT(t, pointers, "/paths/~1x/get/parameters/0/items/collectionFormat")
	assert.SliceContainsT(t, pointers, "/paths/~1x/get/responses/200/headers/X/collectionFormat")
}

// TestCollectionFormatThroughRef covers a parameter reached through a $ref: the walk sees it
// expanded, so the warning is reported just as it is for one written in place.
func TestCollectionFormatThroughRef(t *testing.T) {
	t.Parallel()

	doc := `{
		"swagger": "2.0",
		"info": {"title": "t", "version": "1"},
		"parameters": {
			"Q": {"name": "q", "in": "query", "type": "string", "collectionFormat": "csv"}
		},
		"paths": {
			"/x": {
				"get": {
					"parameters": [{"$ref": "#/parameters/Q"}],
					"responses": {"200": {"description": "ok"}}
				}
			}
		}
	}`

	res := collectionFormatValidatorFromJSON(t, doc).validateCollectionFormats()
	assert.Equal(t, []string{
		`collectionFormat "csv" is ignored in parameter "q": it joins the members of an array, and the type is "string"`,
	}, warningMessages(res))
}

// TestCollectionFormatSkipsBodyParameters guards the one location the meta-schema handles on its
// own: a body parameter may not carry a collectionFormat at all, and that is an error rather than
// this warning.
func TestCollectionFormatSkipsBodyParameters(t *testing.T) {
	t.Parallel()

	doc := collectionFormatSpec(`{"name":"b","in":"body","schema":{"type":"string"}}`, "")

	res := collectionFormatValidatorFromJSON(t, doc).validateCollectionFormats()
	assert.Empty(t, warningMessages(res))
}

// TestCollectionFormatIsNotASchemaKeyword holds the rule to what it walks. collectionFormat is a
// member of [spec.SimpleSchema], which a parameter, a header and an items carry; [spec.Schema] has
// none, so validating an ordinary JSON schema can never reach this rule.
//
// The guard is on the type, not on the walk: a compile-time field access is what would break if a
// later version of go-openapi/spec moved the member onto Schema.
func TestCollectionFormatIsNotASchemaKeyword(t *testing.T) {
	t.Parallel()

	var (
		_ = spec.Items{}.CollectionFormat
		_ = spec.Header{}.CollectionFormat
		_ = spec.Parameter{}.CollectionFormat
	)

	raw, err := json.Marshal(spec.Schema{
		SchemaProps: spec.SchemaProps{Type: spec.StringOrArray{stringType}},
	})
	require.NoError(t, err)

	var members map[string]any
	require.NoError(t, json.Unmarshal(raw, &members))
	assert.MapNotContainsT(t, members, swaggerCollectionFormat)

	// and validating data against a schema reports nothing of the sort
	schema := new(spec.Schema)
	require.NoError(t, json.Unmarshal([]byte(`{"type": "string"}`), schema))
	require.NoError(t, AgainstSchema(schema, "a,b,c", strfmt.Default))
}
