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
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Data structure for jsonschema-suite fixtures
type schemaTestT struct {
	Description string       `json:"description"`
	Schema      *spec.Schema `json:"schema"`
	Tests       []struct {
		Description string      `json:"description"`
		Data        interface{} `json:"data"`
		Valid       bool        `json:"valid"`
	}
}

var jsonSchemaFixturesPath = filepath.Join("fixtures", "jsonschema_suite")
var schemaFixturesPath = filepath.Join("fixtures", "schemas")
var formatFixturesPath = filepath.Join("fixtures", "formats")

func enabled() []string {
	// Standard fixtures from JSON schema suite
	return []string{
		"minLength",
		"maxLength",
		"pattern",
		"type",
		"minimum",
		"maximum",
		"multipleOf",
		"enum",
		"default",
		"dependencies",
		"items",
		"maxItems",
		"maxProperties",
		"minItems",
		"minProperties",
		"patternProperties",
		"required",
		"additionalItems",
		"uniqueItems",
		"properties",
		"additionalProperties",
		"allOf",
		"not",
		"oneOf",
		"anyOf",
		"ref",
		"definitions",
		"refRemote",
		"format",
	}
}

var optionalFixtures = []string{
	// Optional fixtures from JSON schema suite: at the moment, these are disabled
	// "zeroTerminatedFloats",
	// "format",	/* error on strict URI formatting */
	// "bignum",
	// "ecmascript-regex",
}

var extendedFixtures = []string{
	"extended-format",
}

func isEnabled(nm string) bool {
	return swag.ContainsStringsCI(enabled(), nm)
}

func isOptionalEnabled(nm string) bool {
	return swag.ContainsStringsCI(optionalFixtures, nm)
}

func isExtendedEnabled(nm string) bool {
	return swag.ContainsStringsCI(extendedFixtures, nm)
}

func TestJSONSchemaSuite(t *testing.T) {
	// Internal local server to serve remote $ref
	go func() {
		svr := &http.Server{
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			Addr:         "localhost:1234",
			Handler:      http.FileServer(http.Dir(jsonSchemaFixturesPath + "/remotes")),
		}
		err := svr.ListenAndServe()
		if err != nil {
			panic(err.Error())
		}
	}()

	files, err := os.ReadDir(jsonSchemaFixturesPath)
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		fileName := f.Name()
		t.Run(fileName, func(t *testing.T) {
			t.Parallel()
			specName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
			if !isEnabled(specName) {
				t.Logf("WARNING: fixture from jsonschema-test-suite not enabled: %s", specName)
				return
			}
			t.Log("Running " + specName)
			b, _ := os.ReadFile(filepath.Join(jsonSchemaFixturesPath, fileName))
			doTestSchemaSuite(t, b)
		})
	}
}

func TestSchemaFixtures(t *testing.T) {
	files, err := os.ReadDir(schemaFixturesPath)
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		fileName := f.Name()
		t.Run(fileName, func(t *testing.T) {
			t.Parallel()
			specName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
			t.Log("Running " + specName)
			b, _ := os.ReadFile(filepath.Join(schemaFixturesPath, fileName))
			doTestSchemaSuite(t, b)
		})
	}
}

func expandOpts(base string) *spec.ExpandOptions {
	return &spec.ExpandOptions{
		RelativeBase:    base,
		SkipSchemas:     false,
		ContinueOnError: false,
	}
}

func TestOptionalJSONSchemaSuite(t *testing.T) {
	jsonOptionalSchemaFixturesPath := filepath.Join(jsonSchemaFixturesPath, "optional")
	files, err := os.ReadDir(jsonOptionalSchemaFixturesPath)
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		fileName := f.Name()
		t.Run(fileName, func(t *testing.T) {
			t.Parallel()
			specName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
			if !isOptionalEnabled(specName) {
				t.Logf("INFO: fixture from jsonschema-test-suite [optional] not enabled: %s", specName)
				return
			}
			t.Log("Running [optional] " + specName)
			b, _ := os.ReadFile(filepath.Join(jsonOptionalSchemaFixturesPath, fileName))
			doTestSchemaSuite(t, b)
		})
	}
}

// Further testing with all formats recognized by strfmt
func TestFormat_JSONSchemaExtended(t *testing.T) {
	files, err := os.ReadDir(formatFixturesPath)
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		fileName := f.Name()
		t.Run(fileName, func(t *testing.T) {
			t.Parallel()
			specName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
			if !isExtendedEnabled(specName) {
				t.Logf("INFO: fixture from extended tests suite [formats] not enabled: %s", specName)
				return
			}
			t.Log("Running [extended formats] " + specName)
			b, _ := os.ReadFile(filepath.Join(formatFixturesPath, fileName))
			doTestSchemaSuite(t, b)
		})
	}
}

func doTestSchemaSuite(t *testing.T, doc []byte) {
	// run a test formatted as per jsonschema-test-suite
	var testDescriptions []schemaTestT
	eru := json.Unmarshal(doc, &testDescriptions)
	require.NoError(t, eru)

	for _, testDescription := range testDescriptions {
		b, _ := testDescription.Schema.MarshalJSON()
		tmpFile, err := os.CreateTemp(os.TempDir(), "validate-test")
		require.NoError(t, err)
		_, _ = tmpFile.Write(b)
		tmpFile.Close()
		defer func() { _ = os.Remove(tmpFile.Name()) }()
		err = spec.ExpandSchemaWithBasePath(testDescription.Schema, nil, expandOpts(tmpFile.Name()))
		require.NoError(t, err, testDescription.Description+" should expand cleanly")

		validator := NewSchemaValidator(testDescription.Schema, nil, "data", strfmt.Default)
		for _, test := range testDescription.Tests {
			result := validator.Validate(test.Data)
			assert.NotNil(t, result, test.Description+" should validate")

			if test.Valid {
				assert.Empty(t, result.Errors, test.Description+" should not have errors")
			} else {
				assert.NotEmpty(t, result.Errors, test.Description+" should have errors")
			}
		}
	}
}
