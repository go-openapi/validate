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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSchemaOptions(t *testing.T) {
	t.Run("EnableObjectArrayTypeCheck", func(t *testing.T) {
		opts := &SchemaValidatorOptions{}
		setter := EnableObjectArrayTypeCheck(true)
		setter(opts)
		require.True(t, opts.EnableObjectArrayTypeCheck)
	})

	t.Run("skipSchemataResult", func(t *testing.T) {
		opts := &SchemaValidatorOptions{}
		setter := WithSkipSchemataResult(true)
		setter(opts)
		require.True(t, opts.skipSchemataResult)
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
