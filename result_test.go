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

// Test AddError() uniqueness
func TestResult_AddError(t *testing.T) {
	r := Result{}
	r.AddErrors(errors.New("one error"))
	r.AddErrors(errors.New("another error"))
	r.AddErrors(errors.New("one error"))
	r.AddErrors(errors.New("one error"))
	r.AddErrors(errors.New("one error"))
	r.AddErrors(errors.New("one error"), errors.New("another error"))

	assert.Len(t, r.Errors, 2)
	assert.Contains(t, r.Errors, errors.New("one error"))
	assert.Contains(t, r.Errors, errors.New("another error"))
}

func TestResult_AddNilError(t *testing.T) {
	r := Result{}
	r.AddErrors(nil)
	assert.Empty(t, r.Errors)

	errArray := []error{errors.New("one Error"), nil, errors.New("another error")}
	r.AddErrors(errArray...)
	assert.Len(t, r.Errors, 2)
}

func TestResult_AddWarnings(t *testing.T) {
	r := Result{}
	r.AddErrors(errors.New("one Error"))
	assert.Len(t, r.Errors, 1)
	assert.Empty(t, r.Warnings)

	r.AddWarnings(errors.New("one Warning"))
	assert.Len(t, r.Errors, 1)
	assert.Len(t, r.Warnings, 1)
}

func TestResult_Merge(t *testing.T) {
	r := Result{}
	r.AddErrors(errors.New("one Error"))
	r.AddWarnings(errors.New("one Warning"))
	r.Inc()
	assert.Len(t, r.Errors, 1)
	assert.Len(t, r.Warnings, 1)
	assert.Equal(t, 1, r.MatchCount)

	// Merge with same
	r2 := Result{}
	r2.AddErrors(errors.New("one Error"))
	r2.AddWarnings(errors.New("one Warning"))
	r2.Inc()

	r.Merge(&r2)

	assert.Len(t, r.Errors, 1)
	assert.Len(t, r.Warnings, 1)
	assert.Equal(t, 2, r.MatchCount)

	// Merge with new
	r3 := Result{}
	r3.AddErrors(errors.New("new Error"))
	r3.AddWarnings(errors.New("new Warning"))
	r3.Inc()

	r.Merge(&r3)

	assert.Len(t, r.Errors, 2)
	assert.Len(t, r.Warnings, 2)
	assert.Equal(t, 3, r.MatchCount)
}

func errorFixture() (Result, Result, Result) {
	r := Result{}
	r.AddErrors(errors.New("one Error"))
	r.AddWarnings(errors.New("one Warning"))
	r.Inc()

	// same
	r2 := Result{}
	r2.AddErrors(errors.New("one Error"))
	r2.AddWarnings(errors.New("one Warning"))
	r2.Inc()

	// new
	r3 := Result{}
	r3.AddErrors(errors.New("new Error"))
	r3.AddWarnings(errors.New("new Warning"))
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

	r.AddWarnings(errors.New("one Warning"))
	assert.True(t, r.IsValid())
	assert.False(t, r.HasErrors())

	r.AddErrors(errors.New("one Error"))
	assert.False(t, r.IsValid())
	assert.True(t, r.HasErrors())
}

func TestResult_HasWarnings(t *testing.T) {
	r := Result{}

	assert.False(t, r.HasWarnings())

	r.AddErrors(errors.New("one Error"))
	assert.False(t, r.HasWarnings())

	r.AddWarnings(errors.New("one Warning"))
	assert.True(t, r.HasWarnings())
}

func TestResult_HasErrorsOrWarnings(t *testing.T) {
	r := Result{}
	r2 := Result{}

	assert.False(t, r.HasErrorsOrWarnings())

	r.AddErrors(errors.New("one Error"))
	assert.True(t, r.HasErrorsOrWarnings())

	r2.AddWarnings(errors.New("one Warning"))
	assert.True(t, r2.HasErrorsOrWarnings())

	r.Merge(&r2)
	assert.True(t, r.HasErrorsOrWarnings())
}

func TestResult_keepRelevantErrors(t *testing.T) {
	r := Result{}
	r.AddErrors(errors.New("one Error"))
	r.AddErrors(errors.New("IMPORTANT!Another Error"))
	r.AddWarnings(errors.New("one warning"))
	r.AddWarnings(errors.New("IMPORTANT!Another warning"))
	assert.Len(t, r.keepRelevantErrors().Errors, 1)
	assert.Len(t, r.keepRelevantErrors().Warnings, 1)
}

func TestResult_AsError(t *testing.T) {
	r := Result{}
	require.NoError(t, r.AsError())
	r.AddErrors(errors.New("one Error"))
	r.AddErrors(errors.New("additional Error"))
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
