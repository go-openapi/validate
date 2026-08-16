//go:build !windows && !darwin

// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate_test

import (
	"path/filepath"
	"testing"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/testify/v2/require"
	"github.com/go-openapi/validate"
)

func Test_ParallelPool(t *testing.T) {
	// Hitting more stringent memory & threads constraints on windows, when running -race, so we disable this on
	// that target OS. Typically, a CI runner breaks with "ThreadSanitizer failed to allocate ..."
	// Also -race in this context times out on macos. We need our validation on our platform only.

	fixture1 := filepath.Join("testdata", "bugs", "1429", "swagger.yaml")
	fixture2 := filepath.Join("testdata", "bugs", "2866", "2866.yaml")
	fixture3 := filepath.Join("testdata", "bugs", "43", "fixture-43.yaml")

	t.Run("should validate in parallel", func(t *testing.T) {
		for range 20 {
			t.Run("validating fixture 1", func(t *testing.T) {
				t.Parallel()

				doc1, err := loads.Spec(fixture1)
				require.NoError(t, err)
				require.NotNil(t, doc1)
				require.NoError(t, validate.Spec(doc1, strfmt.Default))
			})

			t.Run("validating fixture 2", func(t *testing.T) {
				t.Parallel()

				doc2, err := loads.Spec(fixture2)
				require.NoError(t, err)
				require.NotNil(t, doc2)
				require.NoError(t, validate.Spec(doc2, strfmt.Default))
			})

			t.Run("validating fixture 2", func(t *testing.T) {
				t.Parallel()

				doc3, err := loads.Spec(fixture3)
				require.NoError(t, err)
				require.NotNil(t, doc3)
				require.NoError(t, validate.Spec(doc3, strfmt.Default))
			})
		}
	})
}
