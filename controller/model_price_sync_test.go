package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDecodeModelPriceSyncRequestAllowsMissingOpenRouterChannel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/ratio_sync/model_price/preview", strings.NewReader(`{"models":["gpt-4.1"]}`))

	req, ok := decodeModelPriceSyncRequest(ctx)
	if !ok {
		t.Fatalf("decodeModelPriceSyncRequest rejected missing channel, response: %s", recorder.Body.String())
	}
	if req.OpenRouterChannelID != 0 {
		t.Fatalf("OpenRouterChannelID = %d, want 0 for public catalog", req.OpenRouterChannelID)
	}
	if len(req.Models) != 1 || req.Models[0] != "gpt-4.1" {
		t.Fatalf("Models = %#v, want [gpt-4.1]", req.Models)
	}
}
