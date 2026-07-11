package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGPT56DefaultRatios(t *testing.T) {
	tests := []struct {
		model      string
		modelRatio float64
	}{
		{model: "gpt-5.6", modelRatio: 2.5},
		{model: "gpt-5.6-sol", modelRatio: 2.5},
		{model: "gpt-5.6-terra", modelRatio: 1.25},
		{model: "gpt-5.6-luna", modelRatio: 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			require.Contains(t, defaultModelRatio, tt.model)
			require.Equal(t, tt.modelRatio, defaultModelRatio[tt.model])
			require.Equal(t, 6.0, GetCompletionRatio(tt.model))
			require.Contains(t, defaultCacheRatio, tt.model)
			require.Equal(t, 0.1, defaultCacheRatio[tt.model])
			require.Contains(t, defaultCreateCacheRatio, tt.model)
			require.Equal(t, 1.25, defaultCreateCacheRatio[tt.model])
		})
	}
}

func TestGPT56AliasMatchesSol(t *testing.T) {
	const alias = "gpt-5.6"
	const sol = "gpt-5.6-sol"

	require.Equal(t, defaultModelRatio[sol], defaultModelRatio[alias])
	require.Equal(t, GetCompletionRatio(sol), GetCompletionRatio(alias))
	require.Equal(t, defaultCacheRatio[sol], defaultCacheRatio[alias])
	require.Equal(t, defaultCreateCacheRatio[sol], defaultCreateCacheRatio[alias])
}

func TestGPT56UnsupportedVariantsHaveNoDefaultPricing(t *testing.T) {
	for _, model := range []string{"gpt-5.6-pro", "gpt-5.6-unknown"} {
		t.Run(model, func(t *testing.T) {
			require.NotContains(t, defaultModelRatio, model)
			require.NotContains(t, defaultCacheRatio, model)
			require.NotContains(t, defaultCreateCacheRatio, model)
		})
	}
}
