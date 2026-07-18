package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestNormalizeChannelTestEndpointUsesImageEndpointForGPTImageModels(t *testing.T) {
	for _, modelName := range []string{"gpt-image-1.5", "gpt-image-2", "gpt-image-2-2026-04-21"} {
		t.Run(modelName, func(t *testing.T) {
			got := normalizeChannelTestEndpoint(&model.Channel{Type: constant.ChannelTypeCodex}, modelName, "")
			require.Equal(t, string(constant.EndpointTypeImageGeneration), got)
		})
	}
}

func TestNormalizeChannelTestEndpointRejectsNonImageOverride(t *testing.T) {
	got := normalizeChannelTestEndpoint(&model.Channel{Type: constant.ChannelTypeCodex}, "gpt-image-2", string(constant.EndpointTypeOpenAIResponse))
	require.Equal(t, string(constant.EndpointTypeImageGeneration), got)
}
