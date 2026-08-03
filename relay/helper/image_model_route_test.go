package helper

import (
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestImageModelRejectedOnChatCompletions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-image-2","messages":[{"role":"user","content":"draw"}]}`))
	c.Request.Header.Set("Content-Type", "application/json")

	_, err := GetAndValidateTextRequest(c, relayconstant.RelayModeChatCompletions)
	require.Error(t, err)
	require.Contains(t, err.Error(), "/v1/images/generations")
	require.Contains(t, err.Error(), "/v1/images/edits")
}

func TestImageModelRejectedOnResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-image-2","input":"draw"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	_, err := GetAndValidateResponsesRequest(c)
	require.Error(t, err)
	require.Contains(t, err.Error(), "/v1/images/generations")
	require.Contains(t, err.Error(), "/v1/images/edits")
}

func TestGrokVideoModelRejectedOnTextEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	chat, _ := gin.CreateTestContext(httptest.NewRecorder())
	chat.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"grok-imagine-video-1.5","messages":[{"role":"user","content":"draw"}]}`))
	chat.Request.Header.Set("Content-Type", "application/json")
	_, err := GetAndValidateTextRequest(chat, relayconstant.RelayModeChatCompletions)
	require.Error(t, err)
	require.Contains(t, err.Error(), "/v1/videos/generations")

	responses, _ := gin.CreateTestContext(httptest.NewRecorder())
	responses.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"grok-imagine-video","input":"draw"}`))
	responses.Request.Header.Set("Content-Type", "application/json")
	_, err = GetAndValidateResponsesRequest(responses)
	require.Error(t, err)
	require.Contains(t, err.Error(), "/v1/videos/generations")
}

func TestImageModelAcceptedOnImageGeneration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{"model":"gpt-image-2","prompt":"draw"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	request, err := GetAndValidOpenAIImageRequest(c, relayconstant.RelayModeImagesGenerations)
	require.NoError(t, err)
	require.Equal(t, "gpt-image-2", request.Model)
	require.Equal(t, "auto", request.Quality)
}

func TestGPTImage2MultipartEditDefaultsQuality(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var body strings.Builder
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-2"))
	require.NoError(t, writer.WriteField("prompt", "edit"))
	require.NoError(t, writer.Close())
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", strings.NewReader(body.String()))
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())

	request, err := GetAndValidOpenAIImageRequest(c, relayconstant.RelayModeImagesEdits)
	require.NoError(t, err)
	require.Equal(t, "gpt-image-2", request.Model)
	require.Equal(t, "standard", request.Quality)
}
