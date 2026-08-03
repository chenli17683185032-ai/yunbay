package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestGrokImageTokenMetaIncludesOutputsAndInputsExactlyOnce(t *testing.T) {
	var request ImageRequest
	err := common.Unmarshal([]byte(`{
		"model":"grok-imagine-image-quality",
		"prompt":"edit",
		"n":2,
		"size":"2K",
		"image":{"url":"https://example.com/a.png"},
		"images":["https://example.com/b.png",{"image_url":"https://example.com/c.png"}]
	}`), &request)
	if err != nil {
		t.Fatalf("unmarshal image request: %v", err)
	}

	if got := request.GetInputImageCount(); got != 3 {
		t.Fatalf("GetInputImageCount = %d, want 3", got)
	}
	meta := request.GetTokenCountMeta()
	if meta.ImagePriceRatio != 3.4 {
		t.Fatalf("ImagePriceRatio = %v, want 3.4", meta.ImagePriceRatio)
	}
}
