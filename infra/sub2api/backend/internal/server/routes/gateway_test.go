package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newGatewayRoutesTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	RegisterGatewayRoutes(
		router,
		&handler.Handlers{
			Gateway:       &handler.GatewayHandler{},
			OpenAIGateway: &handler.OpenAIGatewayHandler{},
		},
		servermiddleware.APIKeyAuthMiddleware(func(c *gin.Context) {
			platform := c.GetHeader("X-Test-Platform")
			if platform == "" {
				platform = service.PlatformOpenAI
			}
			groupID := int64(1)
			c.Set(string(servermiddleware.ContextKeyAPIKey), &service.APIKey{
				GroupID: &groupID,
				Group:   &service.Group{Platform: platform},
			})
			c.Next()
		}),
		nil,
		nil,
		nil,
		nil,
		&config.Config{},
	)

	return router
}

func TestGatewayRoutes(t *testing.T) {
	router := newGatewayRoutesTestRouter()

	tests := []struct {
		name     string
		path     string
		platform string
		body     string
	}{
		{name: "messages", path: "/v1/messages", platform: service.PlatformAnthropic, body: `{"model":"claude-3-5-sonnet"}`},
		{name: "count_tokens", path: "/v1/messages/count_tokens", platform: service.PlatformAnthropic, body: `{"model":"claude-3-5-sonnet","messages":[]}`},
		{name: "responses", path: "/v1/responses", platform: service.PlatformOpenAI, body: `{"model":"gpt-5"}`},
		{name: "chat_completions", path: "/v1/chat/completions", platform: service.PlatformOpenAI, body: `{"model":"gpt-4o","messages":[]}`},
		{name: "embeddings", path: "/v1/embeddings", platform: service.PlatformOpenAI, body: `{"model":"text-embedding-3-large","input":"hi"}`},
		{name: "images_generations", path: "/v1/images/generations", platform: service.PlatformOpenAI, body: `{"model":"gpt-image-2","prompt":"draw a cat"}`},
		{name: "images_edits", path: "/v1/images/edits", platform: service.PlatformOpenAI, body: `{"model":"gpt-image-2","prompt":"draw a cat"}`},
		{name: "codex_responses", path: "/backend-api/codex/responses", platform: service.PlatformOpenAI, body: `{"model":"gpt-5"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Test-Platform", tt.platform)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)
			require.NotEqual(t, http.StatusNotFound, w.Code, "path=%s should be registered", tt.path)
		})
	}
}

func TestGatewayRoutesOpenAIResponsesCompactPathIsRegistered(t *testing.T) {
	router := newGatewayRoutesTestRouter()

	for _, path := range []string{
		"/v1/responses/compact",
		"/responses/compact",
		"/backend-api/codex/responses",
		"/backend-api/codex/responses/compact",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"gpt-5"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Platform", service.PlatformOpenAI)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		require.NotEqual(t, http.StatusNotFound, w.Code, "path=%s should hit OpenAI responses handler", path)
	}
}

func TestGatewayRoutesOpenAIImagesPathsAreRegistered(t *testing.T) {
	router := newGatewayRoutesTestRouter()

	for _, path := range []string{
		"/v1/images/generations",
		"/v1/images/edits",
		"/images/generations",
		"/images/edits",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"gpt-image-2","prompt":"draw a cat"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Platform", service.PlatformOpenAI)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		require.NotEqual(t, http.StatusNotFound, w.Code, "path=%s should hit OpenAI images handler", path)
	}
}
