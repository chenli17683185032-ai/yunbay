package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func initJeepayTopupModelColumns(t *testing.T) {
	t.Helper()

	originalIsMasterNode := common.IsMasterNode
	originalSQLitePath := common.SQLitePath
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalSQLDSN, hadSQLDSN := os.LookupEnv("SQL_DSN")

	defer func() {
		common.IsMasterNode = originalIsMasterNode
		common.SQLitePath = originalSQLitePath
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		if hadSQLDSN {
			require.NoError(t, os.Setenv("SQL_DSN", originalSQLDSN))
		} else {
			require.NoError(t, os.Unsetenv("SQL_DSN"))
		}
	}()

	common.IsMasterNode = false
	common.SQLitePath = fmt.Sprintf("file:%s_init?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	common.UsingSQLite = false
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	require.NoError(t, os.Setenv("SQL_DSN", "local"))

	require.NoError(t, model.InitDB())
	if model.DB != nil {
		sqlDB, err := model.DB.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	}
}

func setupJeepayTopupControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	initJeepayTopupModelColumns(t)

	gin.SetMode(gin.TestMode)
	common.OptionMapRWMutex.RLock()
	origOptionMap := common.OptionMap
	common.OptionMapRWMutex.RUnlock()
	origUsingSQLite := common.UsingSQLite
	origUsingMySQL := common.UsingMySQL
	origUsingPostgreSQL := common.UsingPostgreSQL
	origRedisEnabled := common.RedisEnabled
	origDB := model.DB
	origLOGDB := model.LOG_DB
	origJeepayEnabled := setting.JeepayEnabled
	origJeepayAlipayEnabled := setting.JeepayAlipayEnabled
	origJeepayBaseURL := setting.JeepayBaseUrl
	origJeepayMchNo := setting.JeepayMchNo
	origJeepayAppID := setting.JeepayAppId
	origJeepayAppSecret := setting.JeepayAppSecret
	origJeepayNotifyURL := setting.JeepayNotifyUrl
	origJeepayReturnURL := setting.JeepayReturnUrl
	origJeepaySubject := setting.JeepaySubject
	origJeepayBody := setting.JeepayBody
	origJeepayTimeoutMs := setting.JeepayTimeoutMs
	origJeepayAliDisplayName := setting.JeepayAliDisplayName
	origJeepayAliDisplayColor := setting.JeepayAliDisplayColor
	origPayMethods := operation_setting.PayMethods
	origMinTopup := operation_setting.MinTopUp
	paymentSetting := operation_setting.GetPaymentSetting()
	origComplianceConfirmed := paymentSetting.ComplianceConfirmed
	origComplianceTermsVersion := paymentSetting.ComplianceTermsVersion

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.Option{}, &model.User{}, &model.TopUp{}, &model.Log{}))
	model.InitOptionMap()
	operation_setting.PayMethods = []map[string]string{}
	operation_setting.MinTopUp = 1
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion

	t.Cleanup(func() {
		common.UsingSQLite = origUsingSQLite
		common.UsingMySQL = origUsingMySQL
		common.UsingPostgreSQL = origUsingPostgreSQL
		common.RedisEnabled = origRedisEnabled
		model.DB = origDB
		model.LOG_DB = origLOGDB
		setting.JeepayEnabled = origJeepayEnabled
		setting.JeepayAlipayEnabled = origJeepayAlipayEnabled
		setting.JeepayBaseUrl = origJeepayBaseURL
		setting.JeepayMchNo = origJeepayMchNo
		setting.JeepayAppId = origJeepayAppID
		setting.JeepayAppSecret = origJeepayAppSecret
		setting.JeepayNotifyUrl = origJeepayNotifyURL
		setting.JeepayReturnUrl = origJeepayReturnURL
		setting.JeepaySubject = origJeepaySubject
		setting.JeepayBody = origJeepayBody
		setting.JeepayTimeoutMs = origJeepayTimeoutMs
		setting.JeepayAliDisplayName = origJeepayAliDisplayName
		setting.JeepayAliDisplayColor = origJeepayAliDisplayColor
		operation_setting.PayMethods = origPayMethods
		operation_setting.MinTopUp = origMinTopup
		paymentSetting.ComplianceConfirmed = origComplianceConfirmed
		paymentSetting.ComplianceTermsVersion = origComplianceTermsVersion
		common.OptionMapRWMutex.Lock()
		common.OptionMap = origOptionMap
		common.OptionMapRWMutex.Unlock()
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func insertJeepayTopupTestUser(t *testing.T, id int, quota int) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{Id: id, Username: fmt.Sprintf("user_%d", id), Status: common.UserStatusEnabled, Quota: quota}).Error)
}

func TestRequestJeepayPayRejectsUnsupportedMethod(t *testing.T) {
	setupJeepayTopupControllerTestDB(t)
	insertJeepayTopupTestUser(t, 1001, 0)
	setting.JeepayEnabled = true
	setting.JeepayAlipayEnabled = true
	setting.JeepayBaseUrl = "https://jeepay.example.com"
	setting.JeepayMchNo = "mch_123"
	setting.JeepayAppId = "app_123"
	setting.JeepayAppSecret = "secret_123"

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 1001)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/jeepay/pay", strings.NewReader(`{"amount":100,"payment_method":"alipay"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	RequestJeepayPay(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeTestResponse(t, recorder)
	require.Equal(t, false, body["success"])
	require.Equal(t, "支付方式不存在", body["message"])
}

func TestGetTopUpInfoIncludesJeepayAlipayMethod(t *testing.T) {
	setupJeepayTopupControllerTestDB(t)
	setting.JeepayEnabled = true
	setting.JeepayAlipayEnabled = true
	setting.JeepayBaseUrl = "https://jeepay.example.com"
	setting.JeepayMchNo = "mch_123"
	setting.JeepayAppId = "app_123"
	setting.JeepayAppSecret = "secret_123"
	setting.JeepayAliDisplayName = "支付宝扫码"
	setting.JeepayAliDisplayColor = "rgba(0, 112, 255, 1)"

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/topup/info", nil)

	GetTopUpInfo(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeTestResponse(t, recorder)
	require.Equal(t, true, body["success"])
	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok)

	methods := data["pay_methods"].([]interface{})
	found := false
	for _, item := range methods {
		method := item.(map[string]interface{})
		if method["type"] == "jeepay_ali_cashier" {
			found = true
			require.Equal(t, "支付宝扫码", method["name"])
			require.Equal(t, "rgba(0, 112, 255, 1)", method["color"])
			require.Equal(t, "1", method["min_topup"])
		}
	}
	require.True(t, found, "expected Jeepay Alipay method in topup info")
}

func TestRequestJeepayPayReturnsSuccessResponse(t *testing.T) {
	setupJeepayTopupControllerTestDB(t)
	insertJeepayTopupTestUser(t, 1003, 0)

	jeepayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/pay/unifiedOrder", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"code":"0","data":{"payData":{"payUrl":"https://cashier.example.com/pay/JEPAY-ORDER"}}}`))
		require.NoError(t, err)
	}))
	t.Cleanup(jeepayServer.Close)

	setting.JeepayEnabled = true
	setting.JeepayAlipayEnabled = true
	setting.JeepayBaseUrl = jeepayServer.URL
	setting.JeepayMchNo = "mch_123"
	setting.JeepayAppId = "app_123"
	setting.JeepayAppSecret = "secret_123"

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 1003)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/jeepay/pay", strings.NewReader(`{"amount":100,"payment_method":"jeepay_ali_cashier"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	RequestJeepayPay(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeTestResponse(t, recorder)
	require.Equal(t, true, body["success"])
	require.Equal(t, "success", body["message"])
	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok)
	require.NotEmpty(t, data["payment_url"])
}

func TestRequestJeepayPayUsesConfiguredNotifyAndReturnURLs(t *testing.T) {
	setupJeepayTopupControllerTestDB(t)
	insertJeepayTopupTestUser(t, 1004, 0)

	var capturedNotifyURL string
	var capturedReturnURL string

	jeepayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/pay/unifiedOrder", r.URL.Path)

		var payload map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		capturedNotifyURL = common.Interface2String(payload["notifyUrl"])
		capturedReturnURL = common.Interface2String(payload["returnUrl"])

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"code":"0","data":{"payData":{"payUrl":"https://cashier.example.com/pay/JEPAY-ORDER-2"}}}`))
		require.NoError(t, err)
	}))
	t.Cleanup(jeepayServer.Close)

	setting.JeepayEnabled = true
	setting.JeepayAlipayEnabled = true
	setting.JeepayBaseUrl = jeepayServer.URL
	setting.JeepayMchNo = "mch_456"
	setting.JeepayAppId = "app_456"
	setting.JeepayAppSecret = "secret_456"
	setting.JeepayNotifyUrl = "https://pay.yunbay.xyz/api/jeepay/notify"
	setting.JeepayReturnUrl = "https://yunbay.xyz/wallet?show_history=true"

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 1004)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/jeepay/pay", strings.NewReader(`{"amount":100,"payment_method":"jeepay_ali_cashier"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	RequestJeepayPay(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, setting.JeepayNotifyUrl, capturedNotifyURL)
	require.Equal(t, setting.JeepayReturnUrl, capturedReturnURL)
}

func TestJeepayNotifyReturnsSuccessWhenRechargeSucceeds(t *testing.T) {
	setupJeepayTopupControllerTestDB(t)
	insertJeepayTopupTestUser(t, 1002, 0)
	setting.JeepayEnabled = true
	setting.JeepayAlipayEnabled = true
	setting.JeepayBaseUrl = "https://jeepay.example.com"
	setting.JeepayMchNo = "mch_123"
	setting.JeepayAppId = "app_123"
	setting.JeepayAppSecret = "secret_123"

	topUp := &model.TopUp{
		UserId:          1002,
		Amount:          100,
		Money:           12.34,
		TradeNo:         "jeepay-notify-001",
		PaymentMethod:   "jeepay_ali_cashier",
		PaymentProvider: model.PaymentProviderJeepay,
		Status:          common.TopUpStatusPending,
		CreateTime:      time.Now().Unix(),
	}
	require.NoError(t, topUp.Insert())

	payload := map[string]string{
		"mchNo":      setting.JeepayMchNo,
		"appId":      setting.JeepayAppId,
		"mchOrderNo": topUp.TradeNo,
		"wayCode":    "ALI_QR",
		"state":      "2",
		"amount":     "1234",
	}
	payload["sign"] = jeepaySign(payload, setting.JeepayAppSecret)
	body := common.GetJsonString(payload)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/jeepay/notify", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.RemoteAddr = "127.0.0.1:12345"

	JeepayNotify(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "SUCCESS", recorder.Body.String())

	stored := model.GetTopUpByTradeNo(topUp.TradeNo)
	require.NotNil(t, stored)
	require.Equal(t, common.TopUpStatusSuccess, stored.Status)
}
