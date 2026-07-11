package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeKnownOpenAICodexModel_GPT56ExactAliases(t *testing.T) {
	tests := map[string]string{
		"gpt-5.6":            "gpt-5.6-sol",
		"gpt-5.6-sol":        "gpt-5.6-sol",
		"gpt-5.6-terra":      "gpt-5.6-terra",
		"gpt-5.6-luna":       "gpt-5.6-luna",
		"openai/gpt-5.6-sol": "gpt-5.6-sol",
		"gpt5.6-terra":       "gpt-5.6-terra",
		"gpt_5.6_luna":       "gpt-5.6-luna",
	}

	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			require.Equal(t, want, normalizeKnownOpenAICodexModel(input))
		})
	}
}

func TestNormalizeCodexModel_GPT56UnknownModelDoesNotFallback(t *testing.T) {
	unknownModels := []string{
		"gpt-5.6-pro",
		"gpt-5.6-unknown",
		"gpt-5.6-preview",
		"gpt-5.6-high",
		"gpt-5.60",
	}

	for _, model := range unknownModels {
		t.Run(model, func(t *testing.T) {
			require.Empty(t, normalizeKnownOpenAICodexModel(model))
			require.Equal(t, model, normalizeCodexModel(model))
			require.NotEqual(t, "gpt-5.4", normalizeCodexModel(model))
		})
	}
}
