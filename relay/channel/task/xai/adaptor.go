package xai

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const videoRequestContextKey = "xai_video_request"

var imageInputKeys = []string{
	"image",
	"images",
	"input_image",
	"input_images",
	"input_reference",
	"reference_images",
}

var videoInputKeys = []string{
	"video",
	"videos",
	"video_url",
	"input_video",
	"input_videos",
	"input_video_url",
}

type normalizedVideoRequest struct {
	payload map[string]any
	model   string
	price   common.GrokVideoPrice
}

type TaskAdaptor struct {
	taskcommon.BaseBilling
	apiKey  string
	baseURL string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	if info == nil || info.ChannelMeta == nil {
		return
	}
	a.apiKey = info.ApiKey
	a.baseURL = info.ChannelBaseUrl
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	var payload map[string]any
	if err := common.UnmarshalBodyReusable(c, &payload); err != nil {
		return invalidRequest(err)
	}

	modelName := stringValue(payload["model"])
	if !common.IsGrokVideoGenerationModel(modelName) {
		return invalidRequest(fmt.Errorf("unsupported Grok video model %q", modelName))
	}
	if strings.TrimSpace(stringValue(payload["prompt"])) == "" {
		return invalidRequest(fmt.Errorf("prompt is required"))
	}

	duration, err := optionalInteger(payload["duration"])
	if err != nil {
		return invalidRequest(fmt.Errorf("invalid duration: %w", err))
	}
	resolution := stringValue(payload["resolution"])
	if resolution == "" {
		resolution = stringValue(payload["size"])
	}

	inputImageCount := countMediaReferences(payload, imageInputKeys)
	inputVideoCount := countMediaReferences(payload, videoInputKeys)
	inputVideoSeconds, err := inputVideoDuration(payload)
	if err != nil {
		return invalidRequest(err)
	}
	if inputVideoCount > 0 && inputVideoSeconds == 0 {
		return invalidRequest(fmt.Errorf("input video duration is required"))
	}
	if inputVideoCount == 0 && inputVideoSeconds > 0 {
		return invalidRequest(fmt.Errorf("input video is required when input video duration is set"))
	}

	price, err := common.CalculateGrokVideoPrice(modelName, resolution, duration, inputImageCount, inputVideoSeconds)
	if err != nil {
		return invalidRequest(err)
	}

	c.Set(videoRequestContextKey, normalizedVideoRequest{
		payload: payload,
		model:   modelName,
		price:   price,
	})
	if info.TaskRelayInfo == nil {
		info.TaskRelayInfo = &relaycommon.TaskRelayInfo{}
	}
	if inputImageCount > 0 || inputVideoCount > 0 {
		info.Action = constant.TaskActionGenerate
	} else {
		info.Action = constant.TaskActionTextGenerate
	}
	return nil
}

func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	request, ok := getNormalizedVideoRequest(c)
	if !ok {
		return nil
	}
	basePrice := request.price.BasePrice
	if info != nil && info.PriceData.ModelPrice > 0 {
		basePrice = info.PriceData.ModelPrice
	}
	if basePrice <= 0 {
		return nil
	}
	return map[string]float64{
		"request_total": request.price.TotalPrice / basePrice,
	}
}

func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return strings.TrimRight(a.baseURL, "/") + "/v1/videos/generations", nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	request, ok := getNormalizedVideoRequest(c)
	if !ok {
		return nil, fmt.Errorf("request not found in context")
	}

	payload := make(map[string]any, len(request.payload))
	for key, value := range request.payload {
		payload[key] = value
	}

	upstreamModel := request.price.EffectiveModel
	if info != nil && info.ChannelMeta != nil && strings.TrimSpace(info.UpstreamModelName) != "" {
		upstreamModel = info.UpstreamModelName
		if upstreamModel == request.model && request.price.EffectiveModel != request.model {
			upstreamModel = request.price.EffectiveModel
		}
	}
	payload["model"] = upstreamModel
	payload["duration"] = request.price.DurationSeconds
	payload["resolution"] = request.price.Resolution
	delete(payload, "size")

	data, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (string, []byte, *dto.TaskError) {
	if resp == nil || resp.Body == nil {
		return "", nil, service.TaskErrorWrapperLocal(fmt.Errorf("upstream response is empty"), "invalid_response", http.StatusBadGateway)
	}
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	_ = resp.Body.Close()

	var payload map[string]any
	if err := common.Unmarshal(responseBody, &payload); err != nil {
		return "", nil, service.TaskErrorWrapper(err, "unmarshal_response_failed", http.StatusBadGateway)
	}
	upstreamTaskID := firstStringAtPaths(payload,
		[]string{"request_id"},
		[]string{"id"},
		[]string{"data", "request_id"},
		[]string{"data", "id"},
		[]string{"video", "request_id"},
		[]string{"video", "id"},
	)
	if upstreamTaskID == "" {
		return "", nil, service.TaskErrorWrapperLocal(fmt.Errorf("upstream task id is empty"), "invalid_response", http.StatusBadGateway)
	}

	video := dto.NewOpenAIVideo()
	if info != nil {
		video.ID = info.PublicTaskID
		video.TaskID = info.PublicTaskID
		video.Model = info.OriginModelName
	}
	video.CreatedAt = time.Now().Unix()
	c.JSON(http.StatusOK, video)
	return upstreamTaskID, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	url := fmt.Sprintf("%s/v1/videos/%s", strings.TrimRight(baseURL, "/"), taskID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	if client == nil {
		client = http.DefaultClient
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(responseBody []byte) (*relaycommon.TaskInfo, error) {
	var payload map[string]any
	if err := common.Unmarshal(responseBody, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal task result failed: %w", err)
	}

	status := strings.ToLower(firstStringAtPaths(payload,
		[]string{"status"},
		[]string{"data", "status"},
		[]string{"video", "status"},
	))
	result := &relaycommon.TaskInfo{}
	switch status {
	case "submitted", "created":
		result.Status = model.TaskStatusSubmitted
	case "queued", "pending":
		result.Status = model.TaskStatusQueued
	case "processing", "in_progress", "running":
		result.Status = model.TaskStatusInProgress
	case "completed", "succeeded", "success":
		result.Status = model.TaskStatusSuccess
	case "failed", "cancelled", "canceled":
		result.Status = model.TaskStatusFailure
	default:
		return nil, fmt.Errorf("unknown task status %q", status)
	}

	result.Url = firstStringAtPaths(payload,
		[]string{"url"},
		[]string{"video_url"},
		[]string{"video", "url"},
		[]string{"data", "url"},
		[]string{"data", "video", "url"},
		[]string{"output", "url"},
	)
	result.Reason = firstStringAtPaths(payload,
		[]string{"error", "message"},
		[]string{"data", "error", "message"},
		[]string{"message"},
	)
	result.Progress = progressString(firstValueAtPaths(payload,
		[]string{"progress"},
		[]string{"data", "progress"},
		[]string{"video", "progress"},
	))
	if result.Status == model.TaskStatusSuccess && result.Progress == "" {
		result.Progress = "100%"
	}
	if result.Status == model.TaskStatusFailure && result.Reason == "" {
		result.Reason = "task failed"
	}
	return result, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	video := dto.NewOpenAIVideo()
	video.ID = task.TaskID
	video.Model = task.Properties.OriginModelName
	video.Status = task.Status.ToVideoStatus()
	video.SetProgressStr(task.Progress)
	video.CreatedAt = task.CreatedAt
	if task.FinishTime > 0 {
		video.CompletedAt = task.FinishTime
	} else if task.UpdatedAt > 0 {
		video.CompletedAt = task.UpdatedAt
	}
	return common.Marshal(video)
}

func (a *TaskAdaptor) GetModelList() []string {
	return []string{"grok-imagine-video", "grok-imagine-video-1.5"}
}

func (a *TaskAdaptor) GetChannelName() string {
	return "xai"
}

func invalidRequest(err error) *dto.TaskError {
	return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
}

func getNormalizedVideoRequest(c *gin.Context) (normalizedVideoRequest, bool) {
	value, ok := c.Get(videoRequestContextKey)
	if !ok {
		return normalizedVideoRequest{}, false
	}
	request, ok := value.(normalizedVideoRequest)
	return request, ok
}

func countMediaReferences(payload map[string]any, keys []string) int {
	count := 0
	for _, key := range keys {
		value, ok := payload[key]
		if !ok {
			continue
		}
		raw, err := common.Marshal(value)
		if err == nil {
			count += common.CountJSONMediaReferences(raw)
		}
	}
	return count
}

func inputVideoDuration(payload map[string]any) (int, error) {
	for _, key := range []string{"input_video_duration", "input_video_seconds", "source_video_duration"} {
		if value, ok := payload[key]; ok {
			seconds, err := optionalInteger(value)
			if err != nil {
				return 0, fmt.Errorf("invalid %s: %w", key, err)
			}
			return seconds, nil
		}
	}

	seconds := 0
	for _, key := range videoInputKeys {
		if value, ok := payload[key]; ok {
			seconds += nestedMediaDuration(value)
		}
	}
	return seconds, nil
}

func nestedMediaDuration(value any) int {
	switch typed := value.(type) {
	case []any:
		total := 0
		for _, item := range typed {
			total += nestedMediaDuration(item)
		}
		return total
	case map[string]any:
		for _, key := range []string{"duration", "seconds"} {
			if seconds, err := optionalInteger(typed[key]); err == nil && seconds > 0 {
				return seconds
			}
		}
	}
	return 0
}

func optionalInteger(value any) (int, error) {
	if value == nil {
		return 0, nil
	}
	switch typed := value.(type) {
	case float64:
		integer := int(typed)
		if typed != float64(integer) {
			return 0, fmt.Errorf("must be a whole number")
		}
		return integer, nil
	case int:
		return typed, nil
	case string:
		if strings.TrimSpace(typed) == "" {
			return 0, nil
		}
		integer, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0, fmt.Errorf("must be a whole number")
		}
		return integer, nil
	default:
		return 0, fmt.Errorf("must be a whole number")
	}
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func firstStringAtPaths(payload map[string]any, paths ...[]string) string {
	return stringValue(firstValueAtPaths(payload, paths...))
}

func firstValueAtPaths(payload map[string]any, paths ...[]string) any {
	for _, path := range paths {
		var current any = payload
		found := true
		for _, key := range path {
			object, ok := current.(map[string]any)
			if !ok {
				found = false
				break
			}
			current, ok = object[key]
			if !ok {
				found = false
				break
			}
		}
		if found && current != nil {
			return current
		}
	}
	return nil
}

func progressString(value any) string {
	switch typed := value.(type) {
	case string:
		progress := strings.TrimSpace(typed)
		if progress == "" || strings.HasSuffix(progress, "%") {
			return progress
		}
		return progress + "%"
	case float64:
		return strconv.Itoa(int(typed)) + "%"
	case int:
		return strconv.Itoa(typed) + "%"
	default:
		return ""
	}
}
