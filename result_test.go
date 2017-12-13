package validate

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test AddError() uniqueness
func TestResult_AddError(t *testing.T) {
	r := Result{}
	r.AddErrors(fmt.Errorf("One error"))
	r.AddErrors(fmt.Errorf("Another error"))
	r.AddErrors(fmt.Errorf("One error"))
	r.AddErrors(fmt.Errorf("One error"))
	r.AddErrors(fmt.Errorf("One error"))
	r.AddErrors(fmt.Errorf("One error"), fmt.Errorf("Another error"))

	assert.Len(t, r.Errors, 2)
	assert.Contains(t, r.Errors, fmt.Errorf("One error"))
	assert.Contains(t, r.Errors, fmt.Errorf("Another error"))
}
