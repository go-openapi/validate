//go:build validatedebug

package validate

import (
	"path/filepath"
	"testing"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/require"
)

func Test_Debug_2866(t *testing.T) {
	// This test to be run with build flag "validatedebug": it uses the debug pools and asserts that
	// all allocated objects are indeed redeemed at the end of the spec validation.

	resetPools()
	fp := filepath.Join("fixtures", "bugs", "2866", "2866.yaml")

	doc, err := loads.Spec(fp)
	require.NoError(t, err)
	require.NotNil(t, doc)

	require.NoError(t, Spec(doc, strfmt.Default))

	require.True(t, pools.allIsRedeemed(t))
}
