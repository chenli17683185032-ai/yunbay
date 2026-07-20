package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
)

func TestIsImageGenerationModelIncludesGPTImageModels(t *testing.T) {
	tests := []struct {
		name  string
		model string
	}{
		{name: "gpt-image-1", model: "gpt-image-1"},
		{name: "gpt-image-1.5", model: "gpt-image-1.5"},
		{name: "gpt-image-2", model: "gpt-image-2"},
		{name: "versioned gpt-image-2", model: "gpt-image-2-2026-04-21"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !IsImageGenerationModel(tt.model) {
				t.Fatalf("IsImageGenerationModel(%q) = false, want true", tt.model)
			}
		})
	}
}

func TestIsImageGenerationModelDoesNotClassifyTextModels(t *testing.T) {
	for _, model := range []string{"gpt-5.4", "gpt-4o-mini", "text-embedding-3-large"} {
		if IsImageGenerationModel(model) {
			t.Fatalf("IsImageGenerationModel(%q) = true, want false", model)
		}
	}
}

func TestGetEndpointTypesByChannelTypePrefersImageEndpointForGPTImage2(t *testing.T) {
	endpoints := GetEndpointTypesByChannelType(constant.ChannelTypeOpenAI, "gpt-image-2")
	if len(endpoints) < 2 {
		t.Fatalf("GetEndpointTypesByChannelType returned %v, want image and OpenAI endpoints", endpoints)
	}
	if endpoints[0] != constant.EndpointTypeImageGeneration {
		t.Fatalf("first endpoint = %q, want %q", endpoints[0], constant.EndpointTypeImageGeneration)
	}
	if endpoints[1] != constant.EndpointTypeOpenAI {
		t.Fatalf("second endpoint = %q, want %q", endpoints[1], constant.EndpointTypeOpenAI)
	}
}
