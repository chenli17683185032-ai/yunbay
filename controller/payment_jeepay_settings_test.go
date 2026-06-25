package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupJeepaySettingsTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	origUsingSQLite := common.UsingSQLite
	origUsingMySQL := common.UsingMySQL
	origUsingPostgreSQL := common.UsingPostgreSQL
	origRedisEnabled := common.RedisEnabled
	origDB := model.DB
	origLOGDB := model.LOG_DB

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.Option{}))
	model.InitOptionMap()

	t.Cleanup(func() {
		common.UsingSQLite = origUsingSQLite
		common.UsingMySQL = origUsingMySQL
		common.UsingPostgreSQL = origUsingPostgreSQL
		common.RedisEnabled = origRedisEnabled
		model.DB = origDB
		model.LOG_DB = origLOGDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestGetJeepaySettings(t *testing.T) {
	setupJeepaySettingsTestDB(t)

	require.NoError(t, model.UpdateOption("JeepayEnabled", "true"))
	require.NoError(t, model.UpdateOption("JeepayAlipayEnabled", "true"))
	require.NoError(t, model.UpdateOption("JeepayBaseUrl", "https://jeepay.example.com"))
	require.NoError(t, model.UpdateOption("JeepayMchNo", "mch_123"))
	require.NoError(t, model.UpdateOption("JeepayAppId", "app_123"))
	require.NoError(t, model.UpdateOption("JeepayAppSecret", "secret_123"))
	require.NoError(t, model.UpdateOption("JeepayNotifyUrl", "https://pay.example.com/notify"))
	require.NoError(t, model.UpdateOption("JeepayReturnUrl", "https://pay.example.com/return"))
	require.NoError(t, model.UpdateOption("JeepaySubject", "充值"))
	require.NoError(t, model.UpdateOption("JeepayBody", "测试充值"))
	require.NoError(t, model.UpdateOption("JeepayTimeoutMs", "15000"))
	require.NoError(t, model.UpdateOption("JeepayAliDisplayName", "支付宝"))
	require.NoError(t, model.UpdateOption("JeepayAliDisplayColor", "rgba(1, 2, 3, 1)"))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/system-settings/payment/jeepay", nil)

	GetJeepaySettings(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeTestResponse(t, recorder)
	require.Equal(t, true, body["success"])

	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "https://jeepay.example.com", data["JeepayBaseUrl"])
	require.Equal(t, "mch_123", data["JeepayMchNo"])
	require.Equal(t, "app_123", data["JeepayAppId"])
	require.Equal(t, true, data["JeepayAppSecretConfigured"])
	_, leaked := data["JeepayAppSecret"]
	require.False(t, leaked, "secret must not be returned in GET response")
}

func TestSaveJeepaySettingsKeepsExistingSecretWhenEmpty(t *testing.T) {
	setupJeepaySettingsTestDB(t)

	require.NoError(t, model.UpdateOption("JeepayAppSecret", "old_secret"))
	require.NoError(t, model.UpdateOption("JeepayBaseUrl", "https://old.example.com"))

	reqBody := `{
		"JeepayEnabled": true,
		"JeepayAlipayEnabled": true,
		"JeepayBaseUrl": "https://new.example.com",
		"JeepayMchNo": "mch_456",
		"JeepayAppId": "app_456",
		"JeepayAppSecret": "",
		"JeepayNotifyUrl": "https://pay.example.com/notify",
		"JeepayReturnUrl": "https://pay.example.com/return",
		"JeepaySubject": "充值新",
		"JeepayBody": "测试充值新",
		"JeepayTimeoutMs": 30000,
		"JeepayAliDisplayName": "支付宝新",
		"JeepayAliDisplayColor": "rgba(4, 5, 6, 1)"
	}`
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/system-settings/payment/jeepay", strings.NewReader(reqBody))
	ctx.Request.Header.Set("Content-Type", "application/json")

	SaveJeepaySettings(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeTestResponse(t, recorder)
	require.Equal(t, true, body["success"])
	require.NotEmpty(t, body["message"])

	require.Equal(t, "old_secret", setting.JeepayAppSecret)
	require.Equal(t, "https://new.example.com", setting.JeepayBaseUrl)
	require.Equal(t, "mch_456", setting.JeepayMchNo)
	require.Equal(t, "app_456", setting.JeepayAppId)
	require.Equal(t, "https://pay.example.com/notify", setting.JeepayNotifyUrl)
	require.Equal(t, "https://pay.example.com/return", setting.JeepayReturnUrl)
	require.Equal(t, "充值新", setting.JeepaySubject)
	require.Equal(t, "测试充值新", setting.JeepayBody)
	require.Equal(t, 30000, setting.JeepayTimeoutMs)
	require.Equal(t, "支付宝新", setting.JeepayAliDisplayName)
	require.Equal(t, "rgba(4, 5, 6, 1)", setting.JeepayAliDisplayColor)

	appSecret, ok := common.OptionMap["JeepayAppSecret"]
	require.True(t, ok)
	require.Equal(t, "old_secret", common.Interface2String(appSecret))
}

func TestSaveJeepaySettingsUpdatesSecretAtomicallyWithOtherFields(t *testing.T) {
	setupJeepaySettingsTestDB(t)

	require.NoError(t, model.UpdateOption("JeepayAppSecret", "old_secret"))

	reqBody := `{
		"JeepayEnabled": false,
		"JeepayAlipayEnabled": true,
		"JeepayBaseUrl": "https://atomic.example.com",
		"JeepayMchNo": "mch_atomic",
		"JeepayAppId": "app_atomic",
		"JeepayAppSecret": "new_secret",
		"JeepayNotifyUrl": "https://atomic.example.com/notify",
		"JeepayReturnUrl": "https://atomic.example.com/return",
		"JeepaySubject": "原子更新",
		"JeepayBody": "原子更新正文",
		"JeepayTimeoutMs": 12000,
		"JeepayAliDisplayName": "支付宝原子",
		"JeepayAliDisplayColor": "rgba(7, 8, 9, 1)"
	}`
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/system-settings/payment/jeepay", strings.NewReader(reqBody))
	ctx.Request.Header.Set("Content-Type", "application/json")

	SaveJeepaySettings(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeTestResponse(t, recorder)
	require.Equal(t, true, body["success"])

	require.Equal(t, "new_secret", setting.JeepayAppSecret)
	require.Equal(t, "https://atomic.example.com", setting.JeepayBaseUrl)

	appSecret, ok := common.OptionMap["JeepayAppSecret"]
	require.True(t, ok)
	require.Equal(t, "new_secret", common.Interface2String(appSecret))
}
