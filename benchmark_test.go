package validate

import (
	"path/filepath"
	"testing"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/require"
)

func Benchmark_KubernetesSpec(b *testing.B) {
	fp := filepath.Join("fixtures", "go-swagger", "canary", "kubernetes", "swagger.json")
	doc, err := loads.Spec(fp)
	require.NoError(b, err)
	require.NotNil(b, doc)

	b.Run("validating kubernetes API", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			validator := NewSpecValidator(doc.Schema(), strfmt.Default)
			validator.Options.SkipSchemataResult = true
			res, _ := validator.Validate(doc)
			if res == nil || !res.IsValid() {
				b.FailNow()
			}
		}
	})
}
