package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestChatCompletionsToResponsesUsesCodexCompatibleShapeForGPT5(t *testing.T) {
	stream := false
	req := &dto.GeneralOpenAIRequest{
		Model:  "openai/gpt-5.5",
		Stream: &stream,
		Messages: []dto.Message{
			{Role: "user", Content: "hi"},
		},
	}

	got, err := ChatCompletionsRequestToResponsesRequest(req)

	require.NoError(t, err)
	require.Equal(t, "openai/gpt-5.5", got.Model)
	require.NotNil(t, got.Stream)
	require.False(t, *got.Stream, "chat compatibility should preserve the client stream mode")
	require.JSONEq(t, "\"\"", string(got.Instructions))
	require.JSONEq(t, "false", string(got.Store))
	require.NotEmpty(t, got.PromptCacheKey)
	require.JSONEq(t, "[\"reasoning.encrypted_content\"]", string(got.Include))
	require.NotNil(t, got.Reasoning)
	require.Equal(t, "minimal", got.Reasoning.Effort)
	require.JSONEq(t, "\"auto\"", string(got.ToolChoice))
	require.JSONEq(t, "false", string(got.ParallelToolCalls))
	require.Empty(t, got.Tools)
}

func TestChatCompletionsToResponsesDoesNotForceCodexShapeForRegularChatModel(t *testing.T) {
	stream := false
	req := &dto.GeneralOpenAIRequest{
		Model:  "gpt-4o-mini",
		Stream: &stream,
		Messages: []dto.Message{
			{Role: "user", Content: "hi"},
		},
	}

	got, err := ChatCompletionsRequestToResponsesRequest(req)

	require.NoError(t, err)
	require.Equal(t, "gpt-4o-mini", got.Model)
	require.Empty(t, got.PromptCacheKey)
	require.Empty(t, got.Tools)
	require.Nil(t, got.Reasoning)
}
