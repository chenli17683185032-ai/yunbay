package xai

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertImageRequestPreservesGrokBillingSize(t *testing.T) {
	a := &Adaptor{}
	converted, err := a.ConvertImageRequest(nil, &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeImagesGenerations}, dto.ImageRequest{
		Model:  "grok-imagine-image-quality",
		Prompt: "draw",
		Size:   "2K",
	})
	require.NoError(t, err)
	request, ok := converted.(ImageRequest)
	require.True(t, ok)
	require.Equal(t, "2K", request.Size)
}

func TestConvertImageEditRequestPreservesMultipartFiles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "grok-imagine-edit"))
	require.NoError(t, writer.WriteField("prompt", "edit"))
	require.NoError(t, writer.WriteField("size", "2K"))
	for _, name := range []string{"first.png", "second.png"} {
		part, err := writer.CreateFormFile("image[]", name)
		require.NoError(t, err)
		_, err = part.Write([]byte("fake image " + name))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", &body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	request, err := helper.GetAndValidOpenAIImageRequest(c, relayconstant.RelayModeImagesEdits)
	require.NoError(t, err)

	converted, err := (&Adaptor{}).ConvertImageRequest(c, &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeImagesEdits}, *request)
	require.NoError(t, err)
	reader, ok := converted.(io.Reader)
	require.True(t, ok, "converted type %T is not an io.Reader", converted)
	payload, err := io.ReadAll(reader)
	require.NoError(t, err)

	_, params, err := mime.ParseMediaType(c.Request.Header.Get("Content-Type"))
	require.NoError(t, err)
	parsed := multipart.NewReader(bytes.NewReader(payload), params["boundary"])
	form, err := parsed.ReadForm(1 << 20)
	require.NoError(t, err)
	require.Equal(t, []string{"edit"}, form.Value["prompt"])
	require.Equal(t, []string{"2K"}, form.Value["size"])
	require.Len(t, form.File["image[]"], 2)
}
