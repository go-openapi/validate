// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"
)

// repeats is how many times a document is validated when checking that its
// report does not move. Go randomises map iteration per range statement, so a
// two-key map picks the same starting bucket often enough that a handful of
// runs would not notice; a few dozen make an unstable report near-certain to
// show itself.
const repeats = 50

// twoFaultyDefinitionsFixture holds one fault in each of two definitions: Pet
// requires a property it never declares (an error), Tag marks one both required
// and readOnly (a warning).
//
// Walking definitions in map order, the check stops on the first fault it meets,
// so whether Tag was heard from at all used to depend on where the map started.
const twoFaultyDefinitionsFixture = `{
  "swagger": "2.0",
  "info": {"title": "t", "version": "1"},
  "paths": {
    "/pets": {"get": {"operationId": "getPets",
      "responses": {"200": {"description": "ok", "schema": {"$ref": "#/definitions/Pet"}}}}}
  },
  "definitions": {
    "Pet": {"type": "object", "required": ["notDeclared"], "properties": {"name": {"type": "string"}}},
    "Tag": {"type": "object", "required": ["readOnlyToo"], "properties": {"readOnlyToo": {"type": "string", "readOnly": true}}}
  }
}`

// twoCircularDefinitionsFixture holds two definitions that are each their own
// ancestor. Only one circular ancestry is reported, and which one it is used to
// be drawn from the map iteration order.
const twoCircularDefinitionsFixture = `{
  "swagger": "2.0",
  "info": {"title": "t", "version": "1"},
  "paths": {
    "/a": {"get": {"operationId": "getA",
      "responses": {"200": {"description": "ok", "schema": {"$ref": "#/definitions/A"}}}}},
    "/b": {"get": {"operationId": "getB",
      "responses": {"200": {"description": "ok", "schema": {"$ref": "#/definitions/B"}}}}}
  },
  "definitions": {
    "A": {"type": "object", "allOf": [{"$ref": "#/definitions/A"}]},
    "B": {"type": "object", "allOf": [{"$ref": "#/definitions/B"}]}
  }
}`

func TestDeterministic_RequiredDefinitions(t *testing.T) {
	t.Parallel()

	t.Run("stopping on the first fault always stops on the same one", func(t *testing.T) {
		t.Parallel()

		reports := repeatedReports(t, twoFaultyDefinitionsFixture, func(v *SpecValidator) {
			v.Options.ContinueOnErrors = false
		})

		assertSameReport(t, reports)
		assert.SliceContainsT(t, reports[0], "ERR /definitions/Pet/required/0")
	})

	t.Run("reporting everything reports both definitions", func(t *testing.T) {
		t.Parallel()

		reports := repeatedReports(t, twoFaultyDefinitionsFixture, func(v *SpecValidator) {
			v.Options.ContinueOnErrors = true
		})

		assertSameReport(t, reports)
		assert.SliceContainsT(t, reports[0], "ERR /definitions/Pet/required/0")
		assert.SliceContainsT(t, reports[0], "WARN /definitions/Tag/required/0")
	})
}

func TestDeterministic_CircularAncestry(t *testing.T) {
	t.Parallel()

	t.Run("the definition named is the first in name order", func(t *testing.T) {
		t.Parallel()

		reports := repeatedReports(t, twoCircularDefinitionsFixture, func(v *SpecValidator) {
			v.Options.ContinueOnErrors = false
		})

		assertSameReport(t, reports)
		assert.SliceContainsT(t, reports[0], "ERR /definitions/A")
	})

	t.Run("reporting everything reports both circular definitions", func(t *testing.T) {
		t.Parallel()

		// the check used to return on the first circular ancestry whatever the
		// options said, so the second definition could never be heard from
		reports := repeatedReports(t, twoCircularDefinitionsFixture, func(v *SpecValidator) {
			v.Options.ContinueOnErrors = true
		})

		assertSameReport(t, reports)
		assert.SliceContainsT(t, reports[0], "ERR /definitions/A")
		assert.SliceContainsT(t, reports[0], "ERR /definitions/B")
	})
}

// TestDeterministic_FullReport guards the whole sweep rather than the two known
// sites: a document exercising most checks must produce the very same report,
// in the very same order, on every run.
func TestDeterministic_FullReport(t *testing.T) {
	t.Parallel()

	// a whole report is a much finer probe than a single finding: every map a
	// check walks contributes to it, so a handful of runs suffice
	const runs = 20

	for _, fixture := range []string{
		filepath.Join("fixtures", "validation", "fixture-1231.yaml"),
		filepath.Join("fixtures", "validation", "fixture-additional-items-invalid-values.yaml"),
		filepath.Join("fixtures", "validation", "fixture-342.yaml"),
	} {
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			t.Parallel()

			reports := make([][]string, 0, runs)
			for range runs {
				doc, err := loads.Spec(fixture)
				require.NoError(t, err)

				validator := NewSpecValidator(doc.Schema(), strfmt.Default)
				validator.Options.ContinueOnErrors = true
				res, _ := validator.Validate(doc)
				reports = append(reports, report(res))
			}

			require.NotEmpty(t, reports[0], "expected the fixture to yield findings")
			assertSameReport(t, reports)
		})
	}
}

// repeatedReports validates the same document [repeats] times and returns one
// report per run, each in the order the checks emitted it.
func repeatedReports(t *testing.T, raw string, configure func(*SpecValidator)) [][]string {
	t.Helper()

	reports := make([][]string, 0, repeats)
	for range repeats {
		// reloading each time so that no state carried by the document or its
		// analyzer can smooth over an unstable walk
		doc, err := loads.Analyzed(json.RawMessage(raw), "")
		require.NoError(t, err)

		validator := NewSpecValidator(doc.Schema(), strfmt.Default)
		configure(validator)
		res, _ := validator.Validate(doc)
		reports = append(reports, report(res))
	}

	return reports
}

// report renders the located findings of a result as one line each, so that two
// runs can be compared on both what they found and where they said it was.
func report(res *Result) []string {
	lines := make([]string, 0, len(res.Errors)+len(res.Warnings))
	for _, located := range res.LocatedErrors() {
		lines = append(lines, "ERR "+located.Pointer)
	}
	for _, located := range res.LocatedWarnings() {
		lines = append(lines, "WARN "+located.Pointer)
	}

	return lines
}

func assertSameReport(t *testing.T, reports [][]string) {
	t.Helper()

	require.NotEmpty(t, reports)
	for i, got := range reports[1:] {
		assert.Equal(t, reports[0], got, "run %d reported differently from run 0", i+1)
	}
}
