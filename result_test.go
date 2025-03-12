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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	errOne        = errors.New("one Error")
	errAnother    = errors.New("another Error")
	errNew        = errors.New("new Error")
	errAdditional = errors.New("additional Error")
	errImportant  = errors.New("IMPORTANT!Another Error")

	errOneWarning       = errors.New("one Warning")
	errNewWarning       = errors.New("new Warning")
	errImportantWarning = errors.New("IMPORTANT!Another Warning")
)

// Test AddError() uniqueness
func TestResult_AddError(t *testing.T) {
	r := Result{}
	r.AddErrors(errOne)
	r.AddErrors(errAnother)
	r.AddErrors(errOne)
	r.AddErrors(errOne)
	r.AddErrors(errOne)
	r.AddErrors(errOne, errAnother)

	assert.Len(t, r.Errors, 2)
	assert.Contains(t, r.Errors, errOne)
	assert.Contains(t, r.Errors, errAnother)
}

func TestResult_AddNilError(t *testing.T) {
	r := Result{}
	r.AddErrors(nil)
	assert.Empty(t, r.Errors)

	errArray := []error{errOne, nil, errAnother}
	r.AddErrors(errArray...)
	assert.Len(t, r.Errors, 2)
}

func TestResult_AddWarnings(t *testing.T) {
	r := Result{}
	r.AddErrors(errOne)
	assert.Len(t, r.Errors, 1)
	assert.Empty(t, r.Warnings)

	r.AddWarnings(errOneWarning)
	assert.Len(t, r.Errors, 1)
	assert.Len(t, r.Warnings, 1)
}

func TestResult_Merge(t *testing.T) {
	r := Result{}
	r.AddErrors(errOne)
	r.AddWarnings(errOneWarning)
	r.Inc()
	assert.Len(t, r.Errors, 1)
	assert.Len(t, r.Warnings, 1)
	assert.Equal(t, 1, r.MatchCount)

	// Merge with same
	r2 := Result{}
	r2.AddErrors(errOne)
	r2.AddWarnings(errOneWarning)
	r2.Inc()

	r.Merge(&r2)

	assert.Len(t, r.Errors, 1)
	assert.Len(t, r.Warnings, 1)
	assert.Equal(t, 2, r.MatchCount)

	// Merge with new
	r3 := Result{}
	r3.AddErrors(errNew)
	r3.AddWarnings(errNewWarning)
	r3.Inc()

	r.Merge(&r3)

	assert.Len(t, r.Errors, 2)
	assert.Len(t, r.Warnings, 2)
	assert.Equal(t, 3, r.MatchCount)
}

func errorFixture() (Result, Result, Result) {
	r := Result{}
	r.AddErrors(errOne)
	r.AddWarnings(errOneWarning)
	r.Inc()

	// same
	r2 := Result{}
	r2.AddErrors(errOne)
	r2.AddWarnings(errOneWarning)
	r2.Inc()

	// new
	r3 := Result{}
	r3.AddErrors(errNew)
	r3.AddWarnings(errNewWarning)
	r3.Inc()
	return r, r2, r3
}

func TestResult_MergeAsErrors(t *testing.T) {
	r, r2, r3 := errorFixture()
	assert.Len(t, r.Errors, 1)
	assert.Len(t, r.Warnings, 1)
	assert.Equal(t, 1, r.MatchCount)

	r.MergeAsErrors(&r2, &r3)

	assert.Len(t, r.Errors, 4) // One Warning added to Errors
	assert.Len(t, r.Warnings, 1)
	assert.Equal(t, 3, r.MatchCount)
}

func TestResult_MergeAsWarnings(t *testing.T) {
	r, r2, r3 := errorFixture()
	assert.Len(t, r.Errors, 1)
	assert.Len(t, r.Warnings, 1)
	assert.Equal(t, 1, r.MatchCount)

	r.MergeAsWarnings(&r2, &r3)

	assert.Len(t, r.Errors, 1) // One Warning added to Errors
	assert.Len(t, r.Warnings, 4)
	assert.Equal(t, 3, r.MatchCount)
}

func TestResult_IsValid(t *testing.T) {
	r := Result{}

	assert.True(t, r.IsValid())
	assert.False(t, r.HasErrors())

	r.AddWarnings(errOneWarning)
	assert.True(t, r.IsValid())
	assert.False(t, r.HasErrors())

	r.AddErrors(errOne)
	assert.False(t, r.IsValid())
	assert.True(t, r.HasErrors())
}

func TestResult_HasWarnings(t *testing.T) {
	r := Result{}

	assert.False(t, r.HasWarnings())

	r.AddErrors(errOne)
	assert.False(t, r.HasWarnings())

	r.AddWarnings(errOneWarning)
	assert.True(t, r.HasWarnings())
}

func TestResult_HasErrorsOrWarnings(t *testing.T) {
	r := Result{}
	r2 := Result{}

	assert.False(t, r.HasErrorsOrWarnings())

	r.AddErrors(errOne)
	assert.True(t, r.HasErrorsOrWarnings())

	r2.AddWarnings(errOneWarning)
	assert.True(t, r2.HasErrorsOrWarnings())

	r.Merge(&r2)
	assert.True(t, r.HasErrorsOrWarnings())
}

func TestResult_keepRelevantErrors(t *testing.T) {
	r := Result{}
	r.AddErrors(errOne)
	r.AddErrors(errImportant)
	r.AddWarnings(errOneWarning)
	r.AddWarnings(errImportantWarning)
	assert.Len(t, r.keepRelevantErrors().Errors, 1)
	assert.Len(t, r.keepRelevantErrors().Warnings, 1)
}

func TestResult_AsError(t *testing.T) {
	r := Result{}
	require.NoError(t, r.AsError())
	r.AddErrors(errOne)
	r.AddErrors(errAdditional)
	res := r.AsError()
	require.Error(t, res)

	assert.Contains(t, res.Error(), "validation failure list:") // Expected from pkg errors
	assert.Contains(t, res.Error(), "one Error")                // Expected from pkg errors
	assert.Contains(t, res.Error(), "additional Error")         // Expected from pkg errors
}

// Test methods which suppport a call on a nil instance
func TestResult_NilInstance(t *testing.T) {
	var r *Result
	assert.True(t, r.IsValid())
	assert.False(t, r.HasErrors())
	assert.False(t, r.HasWarnings())
	assert.False(t, r.HasErrorsOrWarnings())
}
