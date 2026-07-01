package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const ldxpControllerTestWorkerToken = "test-worker-token"

func setupLdxpTopupControllerTest(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	t.Setenv("LDXP_AUTO_TOPUP_ENABLED", "true")
	t.Setenv("LDXP_WORKER_TOKEN", ldxpControllerTestWorkerToken)
	t.Setenv("LDXP_WORKER_TOKEN_FILE", "")
	t.Setenv("LDXP_CONTACT_EMAIL", "ldxp-test@example.test")
	t.Setenv("LDXP_TOPUP_PRODUCTS_JSON", `[
		{"amount":10,"money":0.10,"product_url":"https://ldxp.example.test/10","product_name":"LDXP 10 Test"},
		{"amount":20,"money":0.20,"product_url":"https://ldxp.example.test/20","product_name":"LDXP 20 Test"},
		{"amount":30,"money":0.30,"product_url":"https://ldxp.example.test/30","product_name":"LDXP 30 Test"},
		{"amount":50,"money":0.50,"product_url":"https://ldxp.example.test/50","product_name":"LDXP 50 Test"},
		{"amount":100,"money":1.00,"product_url":"https://ldxp.example.test/100","product_name":"LDXP 100 Test"},
		{"amount":500,"money":5.00,"product_url":"https://ldxp.example.test/500","product_name":"LDXP 500 Test"}
	]`)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db

	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.LdxpTopupSession{},
		&model.LdxpMailEvent{},
		&model.Redemption{},
		&model.TopUp{},
		&model.Log{},
		&model.AffiliateCommission{},
		&model.AffiliateWithdrawal{},
	))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		_ = os.Unsetenv("LDXP_WORKER_TOKEN_FILE")
	})

	return db
}

func performLdxpControllerRequest(handler gin.HandlerFunc, method string, path string, body any, userID int, headers map[string]string) *httptest.ResponseRecorder {
	router := gin.New()
	routePath := ldxpControllerRoutePattern(path)
	router.Handle(method, routePath, func(c *gin.Context) {
		if userID > 0 {
			c.Set("id", userID)
		}
		handler(c)
	})

	var reqBody bytes.Buffer
	if body != nil {
		payload, err := common.Marshal(body)
		if err != nil {
			panic(err)
		}
		reqBody.Write(payload)
	}
	req := httptest.NewRequest(method, path, &reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func ldxpControllerRoutePattern(path string) string {
	if strings.HasPrefix(path, "/ldxp/topup/session/") {
		if strings.HasSuffix(path, "/cancel") {
			return "/ldxp/topup/session/:session_id/cancel"
		}
		return "/ldxp/topup/session/:session_id"
	}
	if strings.HasPrefix(path, "/ldxp/worker/sessions/") {
		if strings.HasSuffix(path, "/active") {
			return "/ldxp/worker/sessions/:session_id/active"
		}
		if strings.HasSuffix(path, "/qr") {
			return "/ldxp/worker/sessions/:session_id/qr"
		}
		if strings.HasSuffix(path, "/result") {
			return "/ldxp/worker/sessions/:session_id/result"
		}
		if strings.HasSuffix(path, "/error") {
			return "/ldxp/worker/sessions/:session_id/error"
		}
	}
	return path
}

func createLdxpControllerTestUser(t *testing.T, username string) *model.User {
	t.Helper()
	user := &model.User{
		Username:    username,
		Password:    "password123",
		DisplayName: username,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Email:       username + "@example.test",
		Group:       "default",
		AffCode:     "aff_" + username,
	}
	require.NoError(t, model.DB.Create(user).Error)
	return user
}

func createLdxpControllerSession(t *testing.T, userID int, sessionID string, status string) *model.LdxpTopupSession {
	t.Helper()
	now := common.GetTimestamp()
	session := &model.LdxpTopupSession{
		SessionId:     sessionID,
		UserId:        userID,
		Amount:        20,
		Money:         0.20,
		ProductUrl:    "https://ldxp.example.test/20",
		ProductName:   "LDXP 20 Test",
		ContactEmail:  "ldxp-test@example.test",
		Status:        status,
		CreatedTime:   now,
		UpdatedTime:   now,
		ExpiredTime:   now + 1200,
		WorkerId:      "worker-a",
		WorkerOrderNo: "LDORDER123",
	}
	require.NoError(t, model.InsertLdxpTopupSession(session))
	return session
}

func assertLdxpAPIResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	require.Equal(t, http.StatusOK, recorder.Code)
	return decodeTestResponse(t, recorder)
}

func TestCreateLdxpTopupSessionRequiresUser(t *testing.T) {
	setupLdxpTopupControllerTest(t)

	recorder := performLdxpControllerRequest(CreateLdxpTopupSession, http.MethodPost, "/ldxp/topup/session", gin.H{"amount": 20}, 0, nil)

	body := assertLdxpAPIResponse(t, recorder)
	assert.Equal(t, false, body["success"])
}

func TestCreateLdxpTopupSessionReturnsConfiguredAmounts(t *testing.T) {
	setupLdxpTopupControllerTest(t)
	user := createLdxpControllerTestUser(t, "ldxp_create")

	recorder := performLdxpControllerRequest(CreateLdxpTopupSession, http.MethodPost, "/ldxp/topup/session", gin.H{"amount": 20}, user.Id, nil)

	body := assertLdxpAPIResponse(t, recorder)
	require.Equal(t, true, body["success"])
	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(20), data["amount"])
	assert.Equal(t, 0.20, data["money"])
	assert.NotEmpty(t, data["session_id"])
	assert.Equal(t, "created", data["status"])
}

func TestCreateLdxpTopupSessionIgnoresLegacyAllowlist(t *testing.T) {
	setupLdxpTopupControllerTest(t)
	t.Setenv("LDXP_ALLOWED_USERNAMES", "jiance001")
	user := createLdxpControllerTestUser(t, "ordinary_user")

	recorder := performLdxpControllerRequest(CreateLdxpTopupSession, http.MethodPost, "/ldxp/topup/session", gin.H{"amount": 20}, user.Id, nil)

	body := assertLdxpAPIResponse(t, recorder)
	require.Equal(t, true, body["success"])
	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(20), data["amount"])
}

func TestCreateLdxpTopupSessionAllowsConfiguredUsername(t *testing.T) {
	setupLdxpTopupControllerTest(t)
	t.Setenv("LDXP_ALLOWED_USERNAMES", " jiance001 ")
	user := createLdxpControllerTestUser(t, "jiance001")

	recorder := performLdxpControllerRequest(CreateLdxpTopupSession, http.MethodPost, "/ldxp/topup/session", gin.H{"amount": 20}, user.Id, nil)

	body := assertLdxpAPIResponse(t, recorder)
	require.Equal(t, true, body["success"])
	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(20), data["amount"])
}

func TestGetLdxpTopupSessionRequiresOwner(t *testing.T) {
	setupLdxpTopupControllerTest(t)
	owner := createLdxpControllerTestUser(t, "ldxp_owner")
	other := createLdxpControllerTestUser(t, "ldxp_other")
	createLdxpControllerSession(t, owner.Id, "ldxp-owner-only", model.LdxpStatusCreated)

	recorder := performLdxpControllerRequest(GetLdxpTopupSession, http.MethodGet, "/ldxp/topup/session/ldxp-owner-only", nil, other.Id, nil)

	body := assertLdxpAPIResponse(t, recorder)
	assert.Equal(t, false, body["success"])
}

func TestCancelLdxpTopupSession(t *testing.T) {
	setupLdxpTopupControllerTest(t)
	user := createLdxpControllerTestUser(t, "ldxp_cancel")
	createLdxpControllerSession(t, user.Id, "ldxp-cancel", model.LdxpStatusQrReady)

	recorder := performLdxpControllerRequest(CancelLdxpTopupSession, http.MethodPost, "/ldxp/topup/session/ldxp-cancel/cancel", nil, user.Id, nil)

	body := assertLdxpAPIResponse(t, recorder)
	require.Equal(t, true, body["success"])
	persisted, err := model.GetLdxpTopupSessionBySessionId("ldxp-cancel")
	require.NoError(t, err)
	assert.Equal(t, model.LdxpStatusCanceled, persisted.Status)
}

func TestWorkerClaimRequiresToken(t *testing.T) {
	setupLdxpTopupControllerTest(t)

	recorder := performLdxpControllerRequest(WorkerClaimLdxpTopupSession, http.MethodPost, "/ldxp/worker/sessions/claim", gin.H{"worker_id": "worker-a"}, 0, nil)

	body := assertLdxpAPIResponse(t, recorder)
	assert.Equal(t, false, body["success"])
}

func TestWorkerGetLdxpSessionActiveReflectsUserCancel(t *testing.T) {
	setupLdxpTopupControllerTest(t)
	user := createLdxpControllerTestUser(t, "ldxp_worker_active")
	createLdxpControllerSession(t, user.Id, "ldxp-worker-active", model.LdxpStatusWorkerClaimed)

	activeRecorder := performLdxpControllerRequest(WorkerGetLdxpSessionActive, http.MethodPost, "/ldxp/worker/sessions/ldxp-worker-active/active", gin.H{
		"worker_id": "worker-a",
	}, 0, map[string]string{"X-LDXP-Worker-Token": ldxpControllerTestWorkerToken})
	activeBody := assertLdxpAPIResponse(t, activeRecorder)
	require.Equal(t, true, activeBody["success"])
	activeData, ok := activeBody["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, activeData["active"])
	assert.Equal(t, model.LdxpStatusWorkerClaimed, activeData["status"])

	require.NoError(t, service.CancelLdxpTopupSession("ldxp-worker-active", user.Id))

	canceledRecorder := performLdxpControllerRequest(WorkerGetLdxpSessionActive, http.MethodPost, "/ldxp/worker/sessions/ldxp-worker-active/active", gin.H{
		"worker_id": "worker-a",
	}, 0, map[string]string{"X-LDXP-Worker-Token": ldxpControllerTestWorkerToken})
	canceledBody := assertLdxpAPIResponse(t, canceledRecorder)
	require.Equal(t, true, canceledBody["success"])
	canceledData, ok := canceledBody["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, false, canceledData["active"])
	assert.Equal(t, model.LdxpStatusCanceled, canceledData["status"])
}

func TestWorkerGetLdxpSessionActiveTreatsMissingSessionAsInactive(t *testing.T) {
	setupLdxpTopupControllerTest(t)

	recorder := performLdxpControllerRequest(WorkerGetLdxpSessionActive, http.MethodPost, "/ldxp/worker/sessions/ldxp-missing/active", gin.H{
		"worker_id": "worker-a",
	}, 0, map[string]string{"X-LDXP-Worker-Token": ldxpControllerTestWorkerToken})
	body := assertLdxpAPIResponse(t, recorder)
	require.Equal(t, true, body["success"])
	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "ldxp-missing", data["session_id"])
	assert.Equal(t, false, data["active"])
}

func TestWorkerClaimPaidWatchReturnsQrReadySession(t *testing.T) {
	setupLdxpTopupControllerTest(t)
	user := createLdxpControllerTestUser(t, "ldxp_paid_watch")
	session := createLdxpControllerSession(t, user.Id, "ldxp-paid-watch", model.LdxpStatusQrReady)
	require.NoError(t, model.DB.Model(&model.LdxpTopupSession{}).
		Where("id = ?", session.Id).
		Updates(map[string]interface{}{
			"amount":              10,
			"money":               10,
			"worker_amount":       10.3,
			"worker_product_name": "LDXP 10",
			"qr_page_url":         "https://excashier.alipay.com/standard/auth.htm",
		}).Error)

	recorder := performLdxpControllerRequest(WorkerClaimLdxpPaidWatchSession, http.MethodPost, "/ldxp/worker/sessions/claim-paid-watch", gin.H{
		"worker_id": "worker-a",
	}, 0, map[string]string{"X-LDXP-Worker-Token": ldxpControllerTestWorkerToken})

	body := assertLdxpAPIResponse(t, recorder)
	require.Equal(t, true, body["success"])
	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "ldxp-paid-watch", data["session_id"])
	assert.Equal(t, float64(10), data["amount"])
	assert.Equal(t, float64(10), data["money"])
	assert.Equal(t, "LDORDER123", data["worker_order_no"])
	assert.Equal(t, 10.3, data["worker_amount"])
	assert.Equal(t, "LDXP 10", data["worker_product_name"])
	assert.Equal(t, "https://excashier.alipay.com/standard/auth.htm", data["qr_page_url"])
}

func TestWorkerQrUpdateRequiresToken(t *testing.T) {
	setupLdxpTopupControllerTest(t)
	user := createLdxpControllerTestUser(t, "ldxp_qr")
	createLdxpControllerSession(t, user.Id, "ldxp-qr", model.LdxpStatusWorkerClaimed)

	recorder := performLdxpControllerRequest(WorkerRecordLdxpQr, http.MethodPost, "/ldxp/worker/sessions/ldxp-qr/qr", gin.H{
		"worker_id":           "worker-a",
		"worker_order_no":     "LDORDER123",
		"worker_amount":       0.20,
		"worker_product_name": "LDXP 20 Test",
		"qr_code":             "data:image/png;base64,secret-qr",
	}, 0, nil)

	body := assertLdxpAPIResponse(t, recorder)
	assert.Equal(t, false, body["success"])
}

func TestWorkerResultTriggersVerify(t *testing.T) {
	setupLdxpTopupControllerTest(t)
	user := createLdxpControllerTestUser(t, "ldxp_result")
	createLdxpControllerSession(t, user.Id, "ldxp-result", model.LdxpStatusQrReady)
	event, err := serviceSaveLdxpControllerMailEventForTest("LDORDER123", "DIFFERENT-CARD-KEY")
	require.NoError(t, err)
	assert.NotZero(t, event.Id)

	recorder := performLdxpControllerRequest(WorkerRecordLdxpResult, http.MethodPost, "/ldxp/worker/sessions/ldxp-result/result", gin.H{
		"worker_id":           "worker-a",
		"worker_order_no":     "LDORDER123",
		"worker_amount":       0.20,
		"worker_product_name": "LDXP 20 Test",
		"worker_card_key":     "LDXP-CARD-KEY-123",
		"worker_status_text":  "已付款成功",
		"worker_success_url":  "https://ldxp.example.test/success",
	}, 0, map[string]string{"X-LDXP-Worker-Token": ldxpControllerTestWorkerToken})

	body := assertLdxpAPIResponse(t, recorder)
	require.Equal(t, true, body["success"])
	persisted, err := model.GetLdxpTopupSessionBySessionId("ldxp-result")
	require.NoError(t, err)
	assert.Equal(t, model.LdxpStatusVerifyFailed, persisted.Status)
	assert.Equal(t, "card_mismatch", persisted.ErrorCode)
}

func TestWorkerMailEventIngestRequiresToken(t *testing.T) {
	setupLdxpTopupControllerTest(t)

	recorder := performLdxpControllerRequest(WorkerRecordLdxpMailEvent, http.MethodPost, "/ldxp/worker/mail-events", gin.H{
		"message_id": "mail-token-test",
		"raw_hash":   "mail-token-test-hash",
		"order_no":   "LDORDER123",
		"amount":     0.20,
		"card_key":   "LDXP-CARD-KEY-123",
	}, 0, nil)

	body := assertLdxpAPIResponse(t, recorder)
	assert.Equal(t, false, body["success"])
}

func serviceSaveLdxpControllerMailEventForTest(orderNo string, cardKey string) (*model.LdxpMailEvent, error) {
	messageID := "message-" + orderNo
	event := &model.LdxpMailEvent{
		MessageId:    &messageID,
		ImapUid:      "uid-" + orderNo,
		RawHash:      "raw-" + orderNo,
		MailFrom:     "seller@example.test",
		MailTo:       "ldxp-test@example.test",
		Subject:      "LDXP paid",
		ReceivedTime: common.GetTimestamp(),
		OrderNo:      orderNo,
		Amount:       0.20,
		ProductName:  "LDXP 20 Test",
		CardKey:      cardKey,
		PaidTime:     common.GetTimestamp(),
		CreatedTime:  common.GetTimestamp(),
	}
	if err := model.InsertLdxpMailEvent(event); err != nil {
		return nil, err
	}
	return event, nil
}

func TestWorkerMailEventMatchErrorReturnsFailureWithoutLeakingDetails(t *testing.T) {
	setupLdxpTopupControllerTest(t)
	user := createLdxpControllerTestUser(t, "ldxp_mail_match_error")
	createLdxpControllerSession(t, user.Id, "ldxp-mail-match-error", model.LdxpStatusWorkerPaid)
	require.NoError(t, model.DB.Model(&model.LdxpTopupSession{}).Where("session_id = ?", "ldxp-mail-match-error").Updates(map[string]interface{}{
		"worker_card_key":      "SECRET-CARD-KEY-123",
		"worker_amount":        0.20,
		"worker_detected_time": common.GetTimestamp(),
	}).Error)
	messageID := "mail-match-error-new"
	event := &model.LdxpMailEvent{
		MessageId:        &messageID,
		RawHash:          "mail-match-error-new-hash",
		OrderNo:          "LDORDER123",
		Amount:           0.20,
		CardKey:          "SECRET-CARD-KEY-123",
		Processed:        true,
		MatchedSessionId: "ldxp-other-session",
		CreatedTime:      common.GetTimestamp(),
	}
	require.NoError(t, model.InsertLdxpMailEvent(event))

	recorder := performLdxpControllerRequest(WorkerRecordLdxpMailEvent, http.MethodPost, "/ldxp/worker/mail-events", gin.H{
		"message_id":   messageID,
		"raw_hash":     "mail-match-error-new-hash",
		"order_no":     "LDORDER123",
		"amount":       0.20,
		"card_key":     "SECRET-CARD-KEY-123",
		"body_excerpt": "card SECRET-CARD-KEY-123 internal raw error",
	}, 0, map[string]string{"X-LDXP-Worker-Token": ldxpControllerTestWorkerToken})

	body := assertLdxpAPIResponse(t, recorder)
	assert.Equal(t, false, body["success"])
	message, _ := body["message"].(string)
	assert.Equal(t, "ldxp mail match failed", message)
	assert.NotContains(t, message, "SECRET-CARD-KEY-123")
	assert.NotContains(t, message, "already attached")
	assert.NotContains(t, message, "already processed")

	var persistedEvent model.LdxpMailEvent
	require.NoError(t, model.DB.Where("raw_hash = ?", "mail-match-error-new-hash").First(&persistedEvent).Error)
	assert.True(t, persistedEvent.Processed)
	assert.Equal(t, "ldxp-other-session", persistedEvent.MatchedSessionId)
	persistedSession, err := model.GetLdxpTopupSessionBySessionId("ldxp-mail-match-error")
	require.NoError(t, err)
	assert.Empty(t, persistedSession.MailCardKey)
}

func TestWorkerClaimConfigErrorDoesNotLeakTokenFilePath(t *testing.T) {
	setupLdxpTopupControllerTest(t)
	missingPath := "/tmp/definitely-missing-secret-file"
	t.Setenv("LDXP_WORKER_TOKEN", "")
	t.Setenv("LDXP_WORKER_TOKEN_FILE", missingPath)

	recorder := performLdxpControllerRequest(WorkerClaimLdxpTopupSession, http.MethodPost, "/ldxp/worker/sessions/claim", gin.H{"worker_id": "worker-a"}, 0, map[string]string{"X-LDXP-Worker-Token": ldxpControllerTestWorkerToken})

	body := assertLdxpAPIResponse(t, recorder)
	assert.Equal(t, false, body["success"])
	message, _ := body["message"].(string)
	assert.Equal(t, "worker auth unavailable", message)
	assert.NotContains(t, message, missingPath)
	assert.NotContains(t, message, "LDXP_WORKER_TOKEN_FILE")
}

func TestCreateLdxpTopupSessionConfigErrorDoesNotLeakConfigInternals(t *testing.T) {
	setupLdxpTopupControllerTest(t)
	user := createLdxpControllerTestUser(t, "ldxp_create_config_error")
	missingPath := "/tmp/definitely-missing-secret-file"
	t.Setenv("LDXP_WORKER_TOKEN", "")
	t.Setenv("LDXP_WORKER_TOKEN_FILE", missingPath)

	recorder := performLdxpControllerRequest(CreateLdxpTopupSession, http.MethodPost, "/ldxp/topup/session", gin.H{"amount": 20}, user.Id, nil)

	body := assertLdxpAPIResponse(t, recorder)
	assert.Equal(t, false, body["success"])
	message, _ := body["message"].(string)
	assert.Equal(t, "ldxp topup unavailable", message)
	assert.NotContains(t, message, missingPath)
	assert.NotContains(t, message, "LDXP_WORKER_TOKEN_FILE")
}

func TestWorkerErrorPublicViewAndPersistenceDoNotExposeRawDebugFields(t *testing.T) {
	setupLdxpTopupControllerTest(t)
	user := createLdxpControllerTestUser(t, "ldxp_worker_error_public")
	createLdxpControllerSession(t, user.Id, "ldxp-worker-error-public", model.LdxpStatusQrReady)
	rawError := "data:image/png;base64,AAA SECRET-CARD /private/path/ldxp-snapshot.png raw failure body"

	recorder := performLdxpControllerRequest(WorkerRecordLdxpError, http.MethodPost, "/ldxp/worker/sessions/ldxp-worker-error-public/error", gin.H{
		"worker_id":     "worker-a",
		"error_code":    " browser_failed ",
		"error_message": rawError,
		"snapshot_path": " /private/path/ldxp-snapshot.png ",
	}, 0, map[string]string{"X-LDXP-Worker-Token": ldxpControllerTestWorkerToken})
	body := assertLdxpAPIResponse(t, recorder)
	require.Equal(t, true, body["success"])

	viewRecorder := performLdxpControllerRequest(GetLdxpTopupSession, http.MethodGet, "/ldxp/topup/session/ldxp-worker-error-public", nil, user.Id, nil)
	viewBody := assertLdxpAPIResponse(t, viewRecorder)
	require.Equal(t, true, viewBody["success"])
	data, ok := viewBody["data"].(map[string]interface{})
	require.True(t, ok)
	publicError, _ := data["error_message"].(string)
	assert.Equal(t, "Worker failed, please contact support", publicError)
	assert.NotContains(t, publicError, "SECRET-CARD")
	assert.NotContains(t, publicError, "data:image/png;base64")
	assert.NotContains(t, publicError, "/private/path")

	persisted, err := model.GetLdxpTopupSessionBySessionId("ldxp-worker-error-public")
	require.NoError(t, err)
	assert.Equal(t, "Worker failed, please contact support", persisted.ErrorMessage)
	assert.Equal(t, "ldxp-snapshot.png", persisted.DebugSnapshotPath)
	assert.NotContains(t, persisted.DebugSnapshotPath, "/private/path")
}

func TestWorkerQrRejectsOversizedQRCodeWithoutUpdatingSession(t *testing.T) {
	setupLdxpTopupControllerTest(t)
	user := createLdxpControllerTestUser(t, "ldxp_qr_oversized")
	createLdxpControllerSession(t, user.Id, "ldxp-qr-oversized", model.LdxpStatusWorkerClaimed)
	oversizedQR := strings.Repeat("Q", 512*1024+1)

	recorder := performLdxpControllerRequest(WorkerRecordLdxpQr, http.MethodPost, "/ldxp/worker/sessions/ldxp-qr-oversized/qr", gin.H{
		"worker_id":       "worker-a",
		"worker_order_no": "LDORDER123",
		"worker_amount":   0.20,
		"qr_code":         oversizedQR,
	}, 0, map[string]string{"X-LDXP-Worker-Token": ldxpControllerTestWorkerToken})

	body := assertLdxpAPIResponse(t, recorder)
	assert.Equal(t, false, body["success"])
	persisted, err := model.GetLdxpTopupSessionBySessionId("ldxp-qr-oversized")
	require.NoError(t, err)
	assert.Equal(t, model.LdxpStatusWorkerClaimed, persisted.Status)
	assert.Empty(t, persisted.QrCode)
}

func TestWorkerMailEventRejectsOversizedBodyExcerptWithoutInsert(t *testing.T) {
	setupLdxpTopupControllerTest(t)
	oversizedExcerpt := strings.Repeat("M", 4097)

	recorder := performLdxpControllerRequest(WorkerRecordLdxpMailEvent, http.MethodPost, "/ldxp/worker/mail-events", gin.H{
		"message_id":   "mail-oversized-excerpt",
		"raw_hash":     "mail-oversized-excerpt-hash",
		"order_no":     "LDORDER123",
		"amount":       0.20,
		"card_key":     "SECRET-CARD-KEY-123",
		"body_excerpt": oversizedExcerpt,
	}, 0, map[string]string{"X-LDXP-Worker-Token": ldxpControllerTestWorkerToken})

	body := assertLdxpAPIResponse(t, recorder)
	assert.Equal(t, false, body["success"])
	var count int64
	require.NoError(t, model.DB.Model(&model.LdxpMailEvent{}).Where("raw_hash = ?", "mail-oversized-excerpt-hash").Count(&count).Error)
	assert.EqualValues(t, 0, count)
}
