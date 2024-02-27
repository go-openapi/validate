package validate_test

import (
	"path/filepath"
	"testing"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	"github.com/stretchr/testify/require"
)

func Test_ParallelPool(t *testing.T) {
	fixture1 := filepath.Join("fixtures", "bugs", "1429", "swagger.yaml")
	fixture2 := filepath.Join("fixtures", "bugs", "2866", "2866.yaml")
	fixture3 := filepath.Join("fixtures", "bugs", "43", "fixture-43.yaml")

	t.Run("should validate in parallel", func(t *testing.T) {
		for i := 0; i < 20; i++ {
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
