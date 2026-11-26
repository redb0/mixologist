package contract_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadSpec(t *testing.T) *openapi3.T {
	t.Helper()
	_, filename, _, _ := runtime.Caller(0)
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(filepath.Join(filepath.Dir(filename), "../../../api/openapi.yaml"))
	require.NoError(t, err)
	require.NoError(t, doc.Validate(loader.Context))
	return doc
}

func TestRecipeAmountSchema(t *testing.T) {
	schema := loadSpec(t).Components.Schemas["RecipeAmount"].Value

	tests := []struct {
		amount string
		valid  bool
	}{
		{"30", true},
		{"12.5", true},
		{"12.50", true},
		{"0.25", true},
		{"0.5", true},
		{"0.05", true},
		{"0.01", true},
		{"99999999.99", true},
		{"0", false},
		{"0.0", false},
		{"0.00", false},
		{"00.5", false},
		{"012", false},
		{"12.555", false},
		{"123456789", false},
		{"-1", false},
		{"1.", false},
		{".5", false},
		{"1,5", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.amount, func(t *testing.T) {
			err := schema.VisitJSON(tt.amount)

			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
