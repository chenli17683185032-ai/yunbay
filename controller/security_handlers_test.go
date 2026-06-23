package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestFetchCustomOAuthDiscoveryBlocksPrivateURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/custom-oauth-provider/discovery", strings.NewReader(`{"well_known_url":"http://127.0.0.1/.well-known/openid-configuration"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	FetchCustomOAuthDiscovery(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeTestResponse(t, recorder)
	require.Equal(t, false, body["success"])
	require.Contains(t, body["message"], "SSRF")
}

func TestFetchModelsBlocksPrivateBaseURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/channel/fetch_models", strings.NewReader(`{"base_url":"http://127.0.0.1","type":1,"key":"sk-test"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	FetchModels(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeTestResponse(t, recorder)
	require.Equal(t, false, body["success"])
	require.Contains(t, body["message"], "SSRF")
}
