// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
)

// Test edge cases in slice_validator which are difficult
// to simulate with specs
// (this one is a trivial, just to check all methods are filled)
func TestSliceValidator_EdgeCases(t *testing.T) {
	s := newSliceValidator("", "", nil, nil, false, nil, nil, nil, nil, nil)
	s.SetPath("path")
	assert.Equal(t, "path", s.Path)

	r := s.Validate(nil)
	assert.NotNil(t, r)
	assert.True(t, r.IsValid())
}
