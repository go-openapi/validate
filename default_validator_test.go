// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"path/filepath"
	"testing"

	"github.com/go-openapi/analysis"
	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"
)

const (
	jsonExt       = ".json"
	hasErrorMsg   = " should have errors"
	noErrorMsg    = " should not have errors"
	hasWarningMsg = " should have warnings"
)

func TestDefault_ValidatePetStore(t *testing.T) {
	doc, _ := loads.Analyzed(PetStoreJSONMessage, "")
	validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
	validator.spec = doc
	validator.analyzer = analysis.New(doc.Spec())
	myDefaultValidator := &defaultValidator{SpecValidator: validator}
	res := myDefaultValidator.Validate()
	assert.Empty(t, res.Errors)
}

func makeSpecValidator(t *testing.T, fp string) *SpecValidator {
	doc, err := loads.Spec(fp)
	require.NoError(t, err)

	validator := NewSpecValidator(spec.MustLoadSwagger20Schema(), strfmt.Default)
	validator.spec = doc
	validator.analyzer = analysis.New(doc.Spec())
	return validator
}

func TestDefault_ValidateDefaults(t *testing.T) {
	tests := []string{
		"parameter",
		"parameter-required",
		"parameter-ref",
		"parameter-items",
		"header",
		"header-items",
		"schema",
		"schema-ref",
		"schema-additionalProperties",
		"schema-patternProperties",
		"schema-items",
		"schema-allOf",
		"parameter-schema",
		"default-response",
		"header-response",
		"header-items-default-response",
		"header-items-response",
		"header-pattern",
		"header-badpattern",
		"schema-items-allOf",
		"response-ref",
	}

	for _, tt := range tests {
		path := filepath.Join("fixtures", "validation", "default", "valid-default-value-"+tt+jsonExt)
		if DebugTest {
			t.Logf("Testing valid default values for: %s", path)
		}
		validator := makeSpecValidator(t, path)
		myDefaultValidator := &defaultValidator{SpecValidator: validator}
		res := myDefaultValidator.Validate()
		assert.Empty(t, res.Errors, tt+noErrorMsg)

		// Special case: warning only
		if tt == "parameter-required" {
			warns := verifiedTestWarnings(res)
			assert.Contains(t, warns, "limit in query has a default value and is required as parameter")
		}

		path = filepath.Join("fixtures", "validation", "default", "invalid-default-value-"+tt+jsonExt)
		if DebugTest {
			t.Logf("Testing invalid default values for: %s", path)
		}

		validator = makeSpecValidator(t, path)
		myDefaultValidator = &defaultValidator{SpecValidator: validator}
		res = myDefaultValidator.Validate()
		assert.NotEmpty(t, res.Errors, tt+hasErrorMsg)

		// Update: now we have an additional message to explain it's all about a default value
		// Example:
		// - default value for limit in query does not validate its Schema
		// - limit in query must be of type integer: "string"]
		assert.NotEmptyf(t, res.Errors, tt+" should have at least 1 error")
	}
}

func TestDefault_EdgeCase(t *testing.T) {
	// Testing guards
	var myDefaultvalidator *defaultValidator
	res := myDefaultvalidator.Validate()
	assert.True(t, res.IsValid())

	myDefaultvalidator = &defaultValidator{}
	res = myDefaultvalidator.Validate()
	assert.True(t, res.IsValid())
}
