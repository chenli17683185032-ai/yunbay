package xai

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newVideoTestContext(t *testing.T, body string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/generations", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func TestTaskAdaptorEstimateBillingUsesWholeRequestPrice(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		basePrice float64
		wantRatio float64
	}{
		{
			name:      "standard 720p with two images",
			body:      `{"model":"grok-imagine-video","prompt":"animate","duration":10,"resolution":"720p","images":[{"url":"https://example.com/a.png"},{"image_url":"https://example.com/b.png"}]}`,
			basePrice: 0.05,
			wantRatio: 14.08,
		},
		{
			name:      "video 1.5 1080p with image",
			body:      `{"model":"grok-imagine-video-1.5","prompt":"animate","duration":4,"resolution":"1080p","image":{"url":"https://example.com/a.png"}}`,
			basePrice: 0.08,
			wantRatio: 12.625,
		},
		{
			name:      "video 1.5 text request follows upstream fallback",
			body:      `{"model":"grok-imagine-video-1.5","prompt":"waves","resolution":"720p"}`,
			basePrice: 0.08,
			wantRatio: 7,
		},
		{
			name:      "standard request includes input video seconds",
			body:      `{"model":"grok-imagine-video","prompt":"extend","duration":2,"resolution":"480p","video":{"url":"https://example.com/in.mp4","duration":6}}`,
			basePrice: 0.05,
			wantRatio: 3.2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newVideoTestContext(t, tt.body)
			info := &relaycommon.RelayInfo{PriceData: types.PriceData{ModelPrice: tt.basePrice}}
			a := &TaskAdaptor{}
			require.Nil(t, a.ValidateRequestAndSetAction(c, info))
			ratios := a.EstimateBilling(c, info)
			require.Len(t, ratios, 1)
			require.InDelta(t, tt.wantRatio, ratios["request_total"], 1e-9)
		})
	}
}

func TestTaskAdaptorRejectsInvalidDurationAndResolution(t *testing.T) {
	for _, body := range []string{
		`{"model":"grok-imagine-video","prompt":"waves","duration":16,"resolution":"480p"}`,
		`{"model":"grok-imagine-video","prompt":"waves","duration":8,"resolution":"1080p"}`,
		`{"model":"grok-imagine-video-1.5","prompt":"waves","duration":8,"resolution":"cinema"}`,
		`{"model":"grok-imagine-video","prompt":"extend","video":{"url":"https://example.com/in.mp4"}}`,
	} {
		c := newVideoTestContext(t, body)
		taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, &relaycommon.RelayInfo{})
		require.NotNil(t, taskErr, body)
		require.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
	}
}

func TestTaskAdaptorBuildRequestBodyUsesMappedModelAndNormalizedDefaults(t *testing.T) {
	c := newVideoTestContext(t, `{"model":"grok-imagine-video-1.5","prompt":"waves"}`)
	info := &relaycommon.RelayInfo{
		OriginModelName: "grok-imagine-video-1.5",
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "grok-imagine-video"},
	}
	a := &TaskAdaptor{}
	require.Nil(t, a.ValidateRequestAndSetAction(c, info))
	body, err := a.BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(data, &payload))
	require.Equal(t, "grok-imagine-video", payload["model"])
	require.Equal(t, float64(8), payload["duration"])
	require.Equal(t, "480p", payload["resolution"])
}

func TestTaskAdaptorDoResponseAcceptsNestedRequestID(t *testing.T) {
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/generations", bytes.NewBufferString(`{}`))
	info := &relaycommon.RelayInfo{
		OriginModelName: "grok-imagine-video",
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_public"},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"request_id":"upstream-123"}}`)),
	}

	taskID, taskData, taskErr := (&TaskAdaptor{}).DoResponse(c, resp, info)
	require.Nil(t, taskErr)
	require.Equal(t, "upstream-123", taskID)
	require.JSONEq(t, `{"data":{"request_id":"upstream-123"}}`, string(taskData))
	require.Contains(t, response.Body.String(), "task_public")
	require.NotContains(t, response.Body.String(), "upstream-123")
}

func TestTaskAdaptorParseTaskResultAndConvertToOpenAIVideo(t *testing.T) {
	a := &TaskAdaptor{}
	result, err := a.ParseTaskResult([]byte(`{"status":"completed","progress":100,"video":{"url":"https://example.com/out.mp4"}}`))
	require.NoError(t, err)
	require.Equal(t, string(model.TaskStatusSuccess), result.Status)
	require.Equal(t, "https://example.com/out.mp4", result.Url)

	task := &model.Task{
		TaskID:     "task_public",
		Status:     model.TaskStatusSuccess,
		Progress:   "100%",
		CreatedAt:  100,
		FinishTime: 110,
		Properties: model.Properties{OriginModelName: "grok-imagine-video"},
	}
	body, err := a.ConvertToOpenAIVideo(task)
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"task_public","object":"video","model":"grok-imagine-video","status":"completed","progress":100,"created_at":100,"completed_at":110}`, string(body))
}

func TestTaskAdaptorFetchTaskUsesXAIStatusEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/videos/upstream-123", r.URL.Path)
		require.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"pending"}`))
	}))
	defer server.Close()

	resp, err := (&TaskAdaptor{}).FetchTask(server.URL, "secret", map[string]any{"task_id": "upstream-123"}, "")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
