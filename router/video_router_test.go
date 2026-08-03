package router

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestVideoRouterRegistersGrokCompatibleGenerationPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetVideoRouter(r)

	for _, route := range r.Routes() {
		if route.Method == "POST" && route.Path == "/v1/videos/generations" {
			return
		}
	}
	t.Fatal("POST /v1/videos/generations is not registered")
}
