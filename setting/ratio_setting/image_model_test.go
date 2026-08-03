package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGPTImage2DefaultBillingRatios(t *testing.T) {
	require.Equal(t, 2.5, defaultModelRatio["gpt-image-2"])
	require.Equal(t, 6.0, defaultCompletionRatio["gpt-image-2"])
	require.Equal(t, 1.6, defaultImageRatio["gpt-image-2"])
}

func TestGrokMediaDefaultPerUnitPrices(t *testing.T) {
	want := map[string]float64{
		"grok-imagine":               0.05,
		"grok-imagine-edit":          0.02,
		"grok-imagine-image":         0.02,
		"grok-imagine-image-quality": 0.05,
		"grok-imagine-video":         0.05,
		"grok-imagine-video-1.5":     0.08,
	}
	for model, price := range want {
		require.Equal(t, price, defaultModelPrice[model], model)
	}
}
