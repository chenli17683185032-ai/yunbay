package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestImageModelOnNonImageEndpointsReturnsBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name        string
		path        string
		body        string
		relayFormat types.RelayFormat
	}{
		{
			name:        "chat completions",
			path:        "/v1/chat/completions",
			body:        `{"model":"gpt-image-2","messages":[{"role":"user","content":"draw"}]}`,
			relayFormat: types.RelayFormatOpenAI,
		},
		{
			name:        "responses",
			path:        "/v1/responses",
			body:        `{"model":"gpt-image-2","input":"draw"}`,
			relayFormat: types.RelayFormatOpenAIResponses,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			c.Request.Header.Set("Content-Type", "application/json")

			Relay(c, tt.relayFormat)

			require.Equal(t, http.StatusBadRequest, recorder.Code)

			var response struct {
				Error types.OpenAIError `json:"error"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			require.Equal(t, string(types.ErrorCodeInvalidRequest), response.Error.Code)
			require.Contains(t, response.Error.Message, "/v1/images/generations")
			require.Contains(t, response.Error.Message, "/v1/images/edits")
		})
	}
}
