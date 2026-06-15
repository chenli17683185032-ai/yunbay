package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/channelconsole"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelConsoleControllerTestDB(t *testing.T) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	originalDB := model.DB
	originalLOGDB := model.LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalRedisEnabled := common.RedisEnabled
	originalBatchUpdateEnabled := common.BatchUpdateEnabled

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
		&model.ChannelConsolePool{},
		&model.ChannelConsoleCredential{},
		&model.ChannelConsoleModelPrice{},
		&model.ChannelConsoleHealthCheck{},
	))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = originalDB
		model.LOG_DB = originalLOGDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.RedisEnabled = originalRedisEnabled
		common.BatchUpdateEnabled = originalBatchUpdateEnabled
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

	regularChannel := &model.Channel{
		Type:   1,
		Key:    "sk-regular-channel-should-not-leak",
		Name:   "regular channel",
		Status: common.ChannelStatusEnabled,
		Models: "gpt-4o-mini",
		Group:  "default",
	}
	require.NoError(t, model.DB.Create(regularChannel).Error)

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
	listData := listBody["data"].(map[string]interface{})
	require.Equal(t, float64(1), listData["total"])
	require.Equal(t, float64(1), listData["page"])
	require.NotZero(t, listData["page_size"])
	items := listData["items"].([]interface{})
	require.Len(t, items, 1)
	listItem := items[0].(map[string]interface{})
	listChannel := listItem["channel"].(map[string]interface{})
	require.Equal(t, float64(channelID), listChannel["id"])
	require.NotContains(t, listChannel, "key")
	require.NotContains(t, listChannel, "setting")
	require.NotContains(t, listChannel, "param_override")
	require.NotContains(t, listChannel, "header_override")
	require.NotContains(t, listChannel, "settings")
	require.NotNil(t, listItem["console"])

	detailRecorder := httptest.NewRecorder()
	detailCtx, _ := gin.CreateTestContext(detailRecorder)
	detailCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", channelID)}}
	detailCtx.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/channel-console/channels/%d", channelID), nil)

	GetChannelConsoleChannel(detailCtx)

	require.Equal(t, http.StatusOK, detailRecorder.Code)
	detailBody := decodeChannelConsoleResponse(t, detailRecorder)
	require.Equal(t, true, detailBody["success"])
	detailData := detailBody["data"].(map[string]interface{})
	detailChannel := detailData["channel"].(map[string]interface{})
	require.Equal(t, float64(channelID), detailChannel["id"])
	require.NotContains(t, detailChannel, "key")
	require.NotContains(t, detailChannel, "setting")
	require.NotContains(t, detailChannel, "param_override")
	require.NotContains(t, detailChannel, "header_override")
	require.NotContains(t, detailChannel, "settings")
	require.NotNil(t, detailData["console"])
	require.NotNil(t, detailData["prices"])
	require.NotNil(t, detailData["health_checks"])

	regularDetailRecorder := httptest.NewRecorder()
	regularDetailCtx, _ := gin.CreateTestContext(regularDetailRecorder)
	regularDetailCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", regularChannel.Id)}}
	regularDetailCtx.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/channel-console/channels/%d", regularChannel.Id), nil)

	GetChannelConsoleChannel(regularDetailCtx)

	require.Equal(t, http.StatusOK, regularDetailRecorder.Code)
	regularDetailBody := decodeChannelConsoleResponse(t, regularDetailRecorder)
	require.Equal(t, false, regularDetailBody["success"])
	require.Contains(t, regularDetailBody["message"], "渠道控制台元数据不存在")
}

func TestChannelConsoleBatchDeleteHandlerSkipsNonConsoleChannels(t *testing.T) {
	setupChannelConsoleControllerTestDB(t)

	result, err := channelconsole.CommitImport(channelconsole.ImportCommitRequest{
		RawInput: "sk-redacted-example",
		Models:   []string{"gpt-4o-mini"},
	})
	require.NoError(t, err)

	regularChannel := &model.Channel{
		Type:   1,
		Key:    "sk-regular-channel",
		Name:   "regular channel",
		Status: common.ChannelStatusEnabled,
		Models: "gpt-4o-mini",
		Group:  "default",
	}
	require.NoError(t, model.DB.Create(regularChannel).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/channel-console/channels/batch-delete",
		strings.NewReader(fmt.Sprintf(`{"ids":[%d,%d]}`, result.ChannelID, regularChannel.Id)),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	BatchDeleteChannelConsoleChannels(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeChannelConsoleResponse(t, recorder)
	require.Equal(t, true, body["success"])
	data := body["data"].(map[string]interface{})
	require.Equal(t, float64(2), data["requested"])
	require.Equal(t, float64(1), data["deleted"])
	skipped := data["skipped_ids"].([]interface{})
	require.Len(t, skipped, 1)
	require.Equal(t, float64(regularChannel.Id), skipped[0])

	var count int64
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", result.ChannelID).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", regularChannel.Id).Count(&count).Error)
	require.Equal(t, int64(1), count)
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

func TestChannelConsoleHealthCheckHandlerRecordsManualRequest(t *testing.T) {
	setupChannelConsoleControllerTestDB(t)

	result, err := channelconsole.CommitImport(channelconsole.ImportCommitRequest{
		RawInput: "sk-redacted-example",
		Models:   []string{"gpt-4o-mini"},
	})
	require.NoError(t, err)

	originalRunner := channelConsoleHealthCheckRunner
	channelConsoleHealthCheckRunner = func(channel *model.Channel, checkType string) channelConsoleHealthCheckOutcome {
		require.Equal(t, result.ChannelID, channel.Id)
		require.Equal(t, channelconsole.HealthCheckTypeManual, checkType)
		return channelConsoleHealthCheckOutcome{
			status:         channelconsole.HealthHealthy,
			modelName:      "gpt-4o-mini",
			responseTimeMs: 321,
		}
	}
	t.Cleanup(func() {
		channelConsoleHealthCheckRunner = originalRunner
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", result.ChannelID)}}
	ctx.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/channel-console/channels/%d/health-check", result.ChannelID), nil)

	CheckChannelConsoleHealth(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeChannelConsoleResponse(t, recorder)
	require.Equal(t, true, body["success"])
	data := body["data"].(map[string]interface{})
	require.Equal(t, float64(result.ChannelID), data["channel_id"])
	require.Equal(t, "manual", data["check_type"])
	require.Equal(t, "healthy", data["status"])
	require.Equal(t, "gpt-4o-mini", data["model_name"])
	require.Equal(t, float64(321), data["response_time_ms"])
	require.Empty(t, data["error_code"])
	require.Empty(t, data["error_message"])
	require.NotZero(t, data["checked_at"])

	checks, err := model.ListChannelConsoleHealthChecks(result.ChannelID, 50)
	require.NoError(t, err)
	require.Len(t, checks, 1)
	require.Equal(t, "manual", checks[0].CheckType)
	require.Equal(t, "healthy", checks[0].Status)

	meta, err := model.GetChannelConsoleChannelByChannelID(result.ChannelID)
	require.NoError(t, err)
	require.Equal(t, checks[0].CheckedAt, meta.LastHealthCheckAt)
	require.Empty(t, meta.LastErrorCode)
	require.Empty(t, meta.LastErrorMessage)
	require.Equal(t, "healthy", meta.HealthStatus)
}

func TestChannelConsoleHealthCheckRejectsInvalidID(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: "bad"}}
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/channel-console/channels/bad/health-check", nil)

	CheckChannelConsoleHealth(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeChannelConsoleResponse(t, recorder)
	require.Equal(t, false, body["success"])
	require.Equal(t, "invalid channel id", body["message"])
}

func TestChannelConsoleHealthCheckRejectsNonConsoleChannel(t *testing.T) {
	setupChannelConsoleControllerTestDB(t)

	regularChannel := &model.Channel{
		Type:   1,
		Key:    "sk-regular-channel",
		Name:   "regular channel",
		Status: common.ChannelStatusEnabled,
		Models: "gpt-4o-mini",
		Group:  "default",
	}
	require.NoError(t, model.DB.Create(regularChannel).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", regularChannel.Id)}}
	ctx.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/channel-console/channels/%d/health-check", regularChannel.Id), nil)

	CheckChannelConsoleHealth(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeChannelConsoleResponse(t, recorder)
	require.Equal(t, false, body["success"])
	require.Contains(t, body["message"], "渠道控制台元数据不存在")

	checks, err := model.ListChannelConsoleHealthChecks(regularChannel.Id, 50)
	require.NoError(t, err)
	require.Empty(t, checks)
}

func decodeChannelConsoleResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	return body
}
