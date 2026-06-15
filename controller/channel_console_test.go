package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelConsoleControllerTestDB(t *testing.T) {
	t.Helper()

	initModelListColumnNames(t)
	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(
		&model.Channel{},
		&model.Ability{},
		&model.ChannelConsoleChannel{},
		&model.ChannelConsoleModelPrice{},
		&model.ChannelConsoleHealthCheck{},
	))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

func TestChannelConsolePreviewHandler(t *testing.T) {
	setupChannelConsoleControllerTestDB(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/channel-console/import/preview", strings.NewReader(`{"raw_input":"sk-redacted-example"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	PreviewChannelConsoleImport(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeChannelConsoleResponse(t, recorder)
	require.Equal(t, true, body["success"])
	data := body["data"].(map[string]interface{})
	require.Equal(t, "openai", data["provider"])
	require.Equal(t, "https://api.openai.com", data["base_url"])
}

func TestChannelConsoleCommitListAndDetailHandlers(t *testing.T) {
	setupChannelConsoleControllerTestDB(t)

	commitRecorder := httptest.NewRecorder()
	commitCtx, _ := gin.CreateTestContext(commitRecorder)
	commitCtx.Request = httptest.NewRequest(http.MethodPost, "/api/channel-console/import/commit", strings.NewReader(`{"raw_input":"sk-redacted-example","group":"vip","models":["gpt-4o-mini"]}`))
	commitCtx.Request.Header.Set("Content-Type", "application/json")

	CommitChannelConsoleImport(commitCtx)

	require.Equal(t, http.StatusOK, commitRecorder.Code)
	commitBody := decodeChannelConsoleResponse(t, commitRecorder)
	require.Equal(t, true, commitBody["success"])
	commitData := commitBody["data"].(map[string]interface{})
	channelID := int(commitData["channel_id"].(float64))
	require.Greater(t, channelID, 0)

	listRecorder := httptest.NewRecorder()
	listCtx, _ := gin.CreateTestContext(listRecorder)
	listCtx.Request = httptest.NewRequest(http.MethodGet, "/api/channel-console/channels", nil)

	ListChannelConsoleChannels(listCtx)

	require.Equal(t, http.StatusOK, listRecorder.Code)
	listBody := decodeChannelConsoleResponse(t, listRecorder)
	require.Equal(t, true, listBody["success"])
	channels := listBody["data"].([]interface{})
	require.Len(t, channels, 1)

	detailRecorder := httptest.NewRecorder()
	detailCtx, _ := gin.CreateTestContext(detailRecorder)
	detailCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", channelID)}}
	detailCtx.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/channel-console/channels/%d", channelID), nil)

	GetChannelConsoleChannel(detailCtx)

	require.Equal(t, http.StatusOK, detailRecorder.Code)
	detailBody := decodeChannelConsoleResponse(t, detailRecorder)
	require.Equal(t, true, detailBody["success"])
	detailData := detailBody["data"].(map[string]interface{})
	require.NotNil(t, detailData["channel"])
	require.NotNil(t, detailData["console"])
	require.NotNil(t, detailData["prices"])
	require.NotNil(t, detailData["health_checks"])
}

func TestChannelConsoleCommitHandlerReturnsServiceError(t *testing.T) {
	setupChannelConsoleControllerTestDB(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/channel-console/import/commit", strings.NewReader(`{"raw_input":"Base URL: https://gateway.example.com"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	CommitChannelConsoleImport(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeChannelConsoleResponse(t, recorder)
	require.Equal(t, false, body["success"])
	require.Contains(t, body["message"], "未识别到 API Key")
}

func TestChannelConsoleDetailRejectsInvalidID(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: "bad"}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/channel-console/channels/bad", nil)

	GetChannelConsoleChannel(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeChannelConsoleResponse(t, recorder)
	require.Equal(t, false, body["success"])
	require.Equal(t, "invalid channel id", body["message"])
}

func decodeChannelConsoleResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	return body
}
