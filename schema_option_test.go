// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"testing"

	"github.com/go-openapi/testify/v2/require"
)

func TestSchemaOptions(t *testing.T) {
	t.Run("EnableObjectArrayTypeCheck", func(t *testing.T) {
		opts := &SchemaValidatorOptions{}
		setter := EnableObjectArrayTypeCheck(true)
		setter(opts)
		require.TrueT(t, opts.EnableObjectArrayTypeCheck)
	})

	t.Run("skipSchemataResult", func(t *testing.T) {
		opts := &SchemaValidatorOptions{}
		setter := WithSkipSchemataResult(true)
		setter(opts)
		require.TrueT(t, opts.skipSchemataResult)
	})

	t.Run("maxSchemaCompositionDepth", func(t *testing.T) {
		opts := &SchemaValidatorOptions{}
		setter := WithMaxSchemaCompositionDepth(42)
		setter(opts)
		require.Equal(t, 42, opts.maxSchemaCompositionDepthLimit())
	})

	t.Run("default maxSchemaCompositionDepth", func(t *testing.T) {
		opts := &SchemaValidatorOptions{}
		require.Equal(t, defaultMaxSchemaCompositionDepth, opts.maxSchemaCompositionDepthLimit())
	})

	t.Run("default Options()", func(t *testing.T) {
		opts := &SchemaValidatorOptions{}
		setters := opts.Options()

		target := &SchemaValidatorOptions{}
		for _, apply := range setters {
			apply(target)
		}
		require.Equal(t, opts, target)
	})

	t.Run("all set Options()", func(t *testing.T) {
		opts := &SchemaValidatorOptions{
			EnableObjectArrayTypeCheck:    true,
			EnableArrayMustHaveItemsCheck: true,
			maxSchemaCompositionDepth:     128,
			recycleValidators:             true,
			recycleResult:                 true,
			skipSchemataResult:            true,
		}
		setters := opts.Options()

		target := &SchemaValidatorOptions{}
		for _, apply := range setters {
			apply(target)
		}
		require.Equal(t, opts, target)
	})
}
