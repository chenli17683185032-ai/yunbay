package service

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupLdxpSessionServiceTest(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(&model.User{}, &model.SubscriptionOrder{}, &model.UserSubscription{}, &model.UserValuePackagePreference{}, &model.LdxpTopupSession{}, &model.LdxpMailEvent{}))
	ensureLdxpSessionSubscriptionPlanTable(t)
	cleanup := func() {
		require.NoError(t, model.DB.Exec("DELETE FROM ldxp_topup_sessions").Error)
		require.NoError(t, model.DB.Exec("DELETE FROM ldxp_mail_events").Error)
		require.NoError(t, model.DB.Exec("DELETE FROM user_value_package_preferences").Error)
		require.NoError(t, model.DB.Exec("DELETE FROM user_subscriptions").Error)
		require.NoError(t, model.DB.Exec("DELETE FROM subscription_orders").Error)
		require.NoError(t, model.DB.Exec("DELETE FROM subscription_plans").Error)
		require.NoError(t, model.DB.Exec("DELETE FROM users").Error)
	}
	cleanup()
	t.Cleanup(cleanup)
}

func ensureLdxpSessionSubscriptionPlanTable(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.Exec(`CREATE TABLE IF NOT EXISTS subscription_plans (
		id integer PRIMARY KEY AUTOINCREMENT,
		title varchar(128) NOT NULL,
		subtitle varchar(255) DEFAULT '',
		price_amount decimal(10,6) NOT NULL DEFAULT 0,
		currency varchar(8) NOT NULL DEFAULT 'USD',
		duration_unit varchar(16) NOT NULL DEFAULT 'month',
		duration_value integer NOT NULL DEFAULT 1,
		custom_seconds bigint NOT NULL DEFAULT 0,
		enabled numeric DEFAULT 1,
		sort_order integer DEFAULT 0,
		plan_kind varchar(32) NOT NULL DEFAULT 'subscription',
		package_type varchar(16) DEFAULT '',
		package_level integer DEFAULT 0,
		model_group varchar(64) DEFAULT '',
		concurrency_limit integer DEFAULT 1,
		limit5h_amount bigint NOT NULL DEFAULT 0,
		limit7d_amount bigint NOT NULL DEFAULT 0,
		benefits text,
		ldxp_product_url text,
		ldxp_product_name text,
		ldxp_product_amount decimal(10,6) NOT NULL DEFAULT 0,
		ldxp_product_ref varchar(128) DEFAULT '',
		ldxp_session_ttl_seconds bigint NOT NULL DEFAULT 0,
		allow_balance_pay numeric DEFAULT 1,
		stripe_price_id varchar(128) DEFAULT '',
		creem_product_id varchar(128) DEFAULT '',
		waffo_pancake_product_id varchar(128) DEFAULT '',
		max_purchase_per_user integer DEFAULT 0,
		upgrade_group varchar(64) DEFAULT '',
		total_amount bigint NOT NULL DEFAULT 0,
		quota_reset_period varchar(16) DEFAULT 'never',
		quota_reset_custom_seconds bigint DEFAULT 0,
		created_at bigint,
		updated_at bigint
	)`).Error)
}

func testLdxpSessionConfig(enabled bool) *LdxpConfig {
	return &LdxpConfig{
		Enabled:      enabled,
		ContactEmail: "buyer@example.test",
		Products: map[int64]LdxpProductConfig{
			10: {Amount: 10, Money: 0.10, ProductURL: "https://example.test/product/10", ProductName: "LDXP 10"},
			20: {Amount: 20, Money: 0.20, ProductURL: "https://example.test/product/20", ProductName: "LDXP 20"},
		},
		SessionTTLSeconds: 1200,
		QrTTLSeconds:      300,
	}
}

func insertLdxpSessionForServiceTest(t *testing.T, session *model.LdxpTopupSession) {
	t.Helper()
	require.NoError(t, model.InsertLdxpTopupSession(session))
}

func insertLdxpUserForServiceTest(t *testing.T, userID int) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{Id: userID, Username: fmt.Sprintf("user-%d", userID), AffCode: fmt.Sprintf("aff-%d", userID)}).Error)
}

func TestCreateLdxpTopupSessionRejectsDisabled(t *testing.T) {
	setupLdxpSessionServiceTest(t)

	view, err := CreateLdxpTopupSession(1001, 10, testLdxpSessionConfig(false))

	require.Error(t, err)
	assert.Nil(t, view)
	assert.Contains(t, err.Error(), "disabled")
}

func TestCreateLdxpTopupSessionRejectsUnsupportedAmount(t *testing.T) {
	setupLdxpSessionServiceTest(t)

	view, err := CreateLdxpTopupSession(1001, 30, testLdxpSessionConfig(true))

	require.Error(t, err)
	assert.Nil(t, view)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestCreateLdxpTopupSessionReusesActiveSessionForUser(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpUserForServiceTest(t, 1001)
	insertLdxpUserForServiceTest(t, 2002)
	now := common.GetTimestamp()
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:   "ldxp_active_reuse_existing",
		UserId:      1001,
		Amount:      10,
		Money:       0.10,
		ProductUrl:  "https://example.test/product/10",
		ProductName: "LDXP 10",
		Status:      model.LdxpStatusQrReady,
		WorkerId:    "worker-a",
		QrCode:      "data:image/png;base64,QR",
		CreatedTime: now - 10,
		UpdatedTime: now - 5,
		ExpiredTime: now + 1200,
	})
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:   "ldxp_other_user_active",
		UserId:      2002,
		Amount:      20,
		Money:       0.20,
		Status:      model.LdxpStatusCreated,
		CreatedTime: now - 9,
		UpdatedTime: now - 9,
		ExpiredTime: now + 1200,
	})
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:   "ldxp_terminal_not_reused",
		UserId:      1001,
		Amount:      20,
		Money:       0.20,
		Status:      model.LdxpStatusSuccess,
		CreatedTime: now - 8,
		UpdatedTime: now - 8,
		ExpiredTime: now + 1200,
	})

	view, err := CreateLdxpTopupSession(1001, 20, testLdxpSessionConfig(true))

	require.NoError(t, err)
	require.NotNil(t, view)
	assert.Equal(t, "ldxp_active_reuse_existing", view.SessionID)
	assert.EqualValues(t, 10, view.Amount, "active session is reused even when requested amount differs")
	assert.Equal(t, model.LdxpStatusQrReady, view.Status)
	assert.Equal(t, "data:image/png;base64,QR", view.QRCode)

	var count int64
	require.NoError(t, model.DB.Model(&model.LdxpTopupSession{}).Where("user_id = ?", 1001).Count(&count).Error)
	assert.EqualValues(t, 2, count, "reuse must not create another row for the same user")
}

func TestCreateLdxpTopupSessionPersistsCreatedState(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpUserForServiceTest(t, 1001)

	view, err := CreateLdxpTopupSession(1001, 10, testLdxpSessionConfig(true))

	require.NoError(t, err)
	require.NotNil(t, view)
	assert.Regexp(t, regexp.MustCompile(`^ldxp_[a-z0-9]{24}$`), view.SessionID)
	assert.EqualValues(t, 10, view.Amount)
	assert.Equal(t, 0.10, view.Money)
	assert.Equal(t, model.LdxpStatusCreated, view.Status)
	assert.Empty(t, view.QRCode)
	assert.Equal(t, 2000, view.PollIntervalMs)
	assert.NotZero(t, view.ExpiresAt)

	persisted, err := model.GetLdxpTopupSessionBySessionId(view.SessionID)
	require.NoError(t, err)
	assert.Equal(t, 1001, persisted.UserId)
	assert.EqualValues(t, 10, persisted.Amount)
	assert.Equal(t, 0.10, persisted.Money)
	assert.Equal(t, "https://example.test/product/10", persisted.ProductUrl)
	assert.Equal(t, "LDXP 10", persisted.ProductName)
	assert.Equal(t, "buyer@example.test", persisted.ContactEmail)
	assert.Equal(t, model.LdxpStatusCreated, persisted.Status)
	assert.Equal(t, persisted.CreatedTime, persisted.UpdatedTime)
	assert.EqualValues(t, persisted.CreatedTime+1200, persisted.ExpiredTime)
}

func TestCreateLdxpTopupSessionRejectsMissingUser(t *testing.T) {
	setupLdxpSessionServiceTest(t)

	view, err := CreateLdxpTopupSession(1001, 10, testLdxpSessionConfig(true))

	require.Error(t, err)
	assert.Nil(t, view)
	assert.ErrorIs(t, err, ErrLdxpInvalidSessionRequest)

	var count int64
	require.NoError(t, model.DB.Model(&model.LdxpTopupSession{}).Where("user_id = ?", 1001).Count(&count).Error)
	assert.EqualValues(t, 0, count)
}

func TestRecordLdxpQrMovesSessionToQrReady(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:   "ldxp_qr_ready_test",
		UserId:      1001,
		Amount:      10,
		Money:       0.10,
		Status:      model.LdxpStatusWorkerClaimed,
		WorkerId:    "worker-a",
		CreatedTime: 100,
		UpdatedTime: 100,
		ExpiredTime: 2000,
	})

	err := RecordLdxpQr("ldxp_qr_ready_test", LdxpWorkerQrPayload{
		WorkerID:          "worker-a",
		WorkerOrderNo:     "order-qr-1",
		WorkerAmount:      0.10,
		WorkerProductName: "LDXP 10",
		QRCode:            "data:image/png;base64,QR",
		QRPageURL:         "https://example.test/qr",
	})

	require.NoError(t, err)
	persisted, err := model.GetLdxpTopupSessionBySessionId("ldxp_qr_ready_test")
	require.NoError(t, err)
	assert.Equal(t, model.LdxpStatusQrReady, persisted.Status)
	assert.Equal(t, "worker-a", persisted.WorkerId)
	assert.Equal(t, "order-qr-1", persisted.WorkerOrderNo)
	assert.Equal(t, 0.10, persisted.WorkerAmount)
	assert.Equal(t, "LDXP 10", persisted.WorkerProductName)
	assert.Equal(t, "data:image/png;base64,QR", persisted.QrCode)
	assert.Equal(t, "https://example.test/qr", persisted.QrPageUrl)
	assert.NotZero(t, persisted.QrReadyTime)
	assert.Greater(t, persisted.UpdatedTime, int64(100))

	view, err := GetLdxpSessionPublicView("ldxp_qr_ready_test", 1001)
	require.NoError(t, err)
	assert.Equal(t, "data:image/png;base64,QR", view.QRCode)
}

func TestRecordLdxpQrRejectsUnsafeQRCodeSource(t *testing.T) {
	for _, tc := range []struct {
		name   string
		qrCode string
	}{
		{name: "javascript", qrCode: "javascript:alert(1)"},
		{name: "svg data", qrCode: "data:image/svg+xml,<svg onload=alert(1)>"},
		{name: "http", qrCode: "http://example.test/qr.png"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupLdxpSessionServiceTest(t)
			insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
				SessionId:   "ldxp_qr_unsafe_reject_" + strings.ReplaceAll(tc.name, " ", "_"),
				UserId:      1001,
				Amount:      10,
				Money:       0.10,
				Status:      model.LdxpStatusWorkerClaimed,
				WorkerId:    "worker-a",
				CreatedTime: 100,
				UpdatedTime: 100,
				ExpiredTime: 2000,
			})

			sessionID := "ldxp_qr_unsafe_reject_" + strings.ReplaceAll(tc.name, " ", "_")
			err := RecordLdxpQr(sessionID, LdxpWorkerQrPayload{
				WorkerID:          "worker-a",
				WorkerOrderNo:     "order-unsafe-1",
				WorkerAmount:      0.10,
				WorkerProductName: "LDXP 10",
				QRCode:            tc.qrCode,
				QRPageURL:         "https://example.test/qr",
			})

			require.Error(t, err)
			assert.ErrorIs(t, err, ErrLdxpInvalidSessionRequest)

			persisted, getErr := model.GetLdxpTopupSessionBySessionId(sessionID)
			require.NoError(t, getErr)
			assert.Equal(t, model.LdxpStatusWorkerClaimed, persisted.Status)
			assert.Empty(t, persisted.QrCode)
			assert.Empty(t, persisted.WorkerOrderNo)
		})
	}
}

func TestRecordLdxpWorkerResultMovesSessionToWorkerPaid(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:     "ldxp_worker_paid_test",
		UserId:        1001,
		Amount:        20,
		Money:         0.20,
		Status:        model.LdxpStatusQrReady,
		WorkerId:      "worker-a",
		WorkerOrderNo: "order-paid-1",
		QrCode:        "data:image/png;base64,QR",
		CreatedTime:   100,
		UpdatedTime:   100,
		ExpiredTime:   2000,
	})

	session, err := RecordLdxpWorkerResult("ldxp_worker_paid_test", LdxpWorkerResultPayload{
		WorkerID:          "worker-a",
		WorkerOrderNo:     "order-paid-1",
		WorkerAmount:      0.20,
		WorkerProductName: "LDXP 20",
		WorkerCardKey:     "SECRET-CARD-KEY",
		WorkerStatusText:  "paid",
		WorkerSuccessURL:  "https://example.test/success",
	})

	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, model.LdxpStatusWorkerPaid, session.Status)
	assert.Equal(t, "SECRET-CARD-KEY", session.WorkerCardKey)
	assert.NotZero(t, session.WorkerDetectedTime)

	persisted, err := model.GetLdxpTopupSessionBySessionId("ldxp_worker_paid_test")
	require.NoError(t, err)
	assert.Equal(t, model.LdxpStatusWorkerPaid, persisted.Status)
	assert.Equal(t, "worker-a", persisted.WorkerId)
	assert.Equal(t, "order-paid-1", persisted.WorkerOrderNo)
	assert.Equal(t, 0.20, persisted.WorkerAmount)
	assert.Equal(t, "LDXP 20", persisted.WorkerProductName)
	assert.Equal(t, "SECRET-CARD-KEY", persisted.WorkerCardKey)
	assert.Equal(t, "paid", persisted.WorkerStatusText)
	assert.Equal(t, "https://example.test/success", persisted.WorkerSuccessUrl)
	assert.NotZero(t, persisted.WorkerDetectedTime)

	view, err := GetLdxpSessionPublicView("ldxp_worker_paid_test", 1001)
	require.NoError(t, err)
	assert.Empty(t, view.QRCode, "terminal/paid views must not keep exposing QR code")
}

func TestCancelLdxpSessionOnlyOwnerAndCancelableStates(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	for _, session := range []*model.LdxpTopupSession{
		{SessionId: "cancel_created", UserId: 1001, Status: model.LdxpStatusCreated, CreatedTime: 100, UpdatedTime: 100, ExpiredTime: 2000},
		{SessionId: "cancel_claimed", UserId: 1001, Status: model.LdxpStatusWorkerClaimed, CreatedTime: 100, UpdatedTime: 100, ExpiredTime: 2000},
		{SessionId: "cancel_qr", UserId: 1001, Status: model.LdxpStatusQrReady, CreatedTime: 100, UpdatedTime: 100, ExpiredTime: 2000},
		{SessionId: "cancel_paid", UserId: 1001, Status: model.LdxpStatusWorkerPaid, CreatedTime: 100, UpdatedTime: 100, ExpiredTime: 2000},
		{SessionId: "cancel_other_owner", UserId: 2002, Status: model.LdxpStatusCreated, CreatedTime: 100, UpdatedTime: 100, ExpiredTime: 2000},
	} {
		insertLdxpSessionForServiceTest(t, session)
	}

	for _, sessionID := range []string{"cancel_created", "cancel_claimed", "cancel_qr"} {
		require.NoError(t, CancelLdxpTopupSession(sessionID, 1001))
		persisted, err := model.GetLdxpTopupSessionBySessionId(sessionID)
		require.NoError(t, err)
		assert.Equal(t, model.LdxpStatusCanceled, persisted.Status)
		assert.Greater(t, persisted.UpdatedTime, int64(100))
	}

	err := CancelLdxpTopupSession("cancel_paid", 1001)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not cancelable")
	paid, err := model.GetLdxpTopupSessionBySessionId("cancel_paid")
	require.NoError(t, err)
	assert.Equal(t, model.LdxpStatusWorkerPaid, paid.Status)

	err = CancelLdxpTopupSession("cancel_other_owner", 1001)
	require.Error(t, err)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound), "got %v", err)
	otherOwner, err := model.GetLdxpTopupSessionBySessionId("cancel_other_owner")
	require.NoError(t, err)
	assert.Equal(t, model.LdxpStatusCreated, otherOwner.Status)
}

func TestPublicLdxpSessionViewDoesNotExposeCardKey(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:     "ldxp_public_view_secret_test",
		UserId:        1001,
		Amount:        20,
		Money:         0.20,
		Status:        model.LdxpStatusWorkerPaid,
		WorkerOrderNo: "order-public-1",
		QrCode:        "data:image/png;base64,QR",
		WorkerCardKey: "SECRET-WORKER-CARD",
		MailCardKey:   "SECRET-MAIL-CARD",
		CreatedTime:   100,
		UpdatedTime:   100,
		ExpiredTime:   2000,
	})

	view, err := GetLdxpSessionPublicView("ldxp_public_view_secret_test", 1001)

	require.NoError(t, err)
	require.NotNil(t, view)
	assert.Equal(t, "ldxp_public_view_secret_test", view.SessionID)
	assert.Equal(t, "order-public-1", view.WorkerOrderNo)
	assert.Empty(t, view.QRCode)
	renderedView := fmt.Sprintf("%+v", view)
	assert.NotContains(t, renderedView, "SECRET-WORKER-CARD")
	assert.NotContains(t, renderedView, "SECRET-MAIL-CARD")
}

func TestGetLdxpWorkerSessionStateReflectsCancellation(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:   "ldxp_worker_state",
		UserId:      1001,
		Status:      model.LdxpStatusWorkerClaimed,
		WorkerId:    "worker-a",
		CreatedTime: 100,
		UpdatedTime: 100,
		ExpiredTime: common.GetTimestamp() + 1200,
	})

	state, err := GetLdxpWorkerSessionState("ldxp_worker_state", " worker-a ")
	require.NoError(t, err)
	require.NotNil(t, state)
	assert.Equal(t, "ldxp_worker_state", state.SessionID)
	assert.Equal(t, model.LdxpStatusWorkerClaimed, state.Status)
	assert.Equal(t, true, state.Active)

	require.NoError(t, CancelLdxpTopupSession("ldxp_worker_state", 1001))

	state, err = GetLdxpWorkerSessionState("ldxp_worker_state", "worker-a")
	require.NoError(t, err)
	require.NotNil(t, state)
	assert.Equal(t, model.LdxpStatusCanceled, state.Status)
	assert.Equal(t, false, state.Active)
}

func TestGetLdxpWorkerSessionStateRejectsDifferentWorker(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:   "ldxp_worker_state_wrong_worker",
		UserId:      1001,
		Status:      model.LdxpStatusWorkerClaimed,
		WorkerId:    "worker-a",
		CreatedTime: 100,
		UpdatedTime: 100,
		ExpiredTime: common.GetTimestamp() + 1200,
	})

	state, err := GetLdxpWorkerSessionState("ldxp_worker_state_wrong_worker", "worker-b")
	require.NoError(t, err)
	require.NotNil(t, state)
	assert.Equal(t, model.LdxpStatusWorkerClaimed, state.Status)
	assert.Equal(t, false, state.Active)
}

func TestClaimLdxpTopupSessionMovesCreatedSessionToWorkerClaimed(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	now := common.GetTimestamp()
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:   "ldxp_claim_created",
		UserId:      1001,
		Amount:      10,
		Money:       0.10,
		Status:      model.LdxpStatusCreated,
		CreatedTime: now - 10,
		UpdatedTime: now - 10,
		ExpiredTime: now + 1200,
	})

	claimed, err := ClaimLdxpTopupSession(" worker-a ", testLdxpSessionConfig(true))

	require.NoError(t, err)
	require.NotNil(t, claimed)
	assert.Equal(t, "ldxp_claim_created", claimed.SessionId)
	assert.Equal(t, model.LdxpStatusWorkerClaimed, claimed.Status)
	assert.Equal(t, "worker-a", claimed.WorkerId)
	assert.Greater(t, claimed.UpdatedTime, now-10)

	persisted, err := model.GetLdxpTopupSessionBySessionId("ldxp_claim_created")
	require.NoError(t, err)
	assert.Equal(t, model.LdxpStatusWorkerClaimed, persisted.Status)
	assert.Equal(t, "worker-a", persisted.WorkerId)
}

func TestClaimLdxpTopupSessionRejectsNilConfig(t *testing.T) {
	setupLdxpSessionServiceTest(t)

	session, err := ClaimLdxpTopupSession("worker-a", nil)

	require.Error(t, err)
	assert.Nil(t, session)
	assert.ErrorIs(t, err, ErrLdxpInvalidSessionRequest)
}

func TestClaimLdxpPaidWatchSessionReturnsQrReadySession(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	now := common.GetTimestamp()
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:         "ldxp_paid_watch_ready",
		UserId:            1001,
		Amount:            10,
		Money:             10,
		Status:            model.LdxpStatusQrReady,
		WorkerId:          "worker-a",
		WorkerOrderNo:     "LDWATCHREADY",
		WorkerAmount:      10.3,
		WorkerProductName: "LDXP 10",
		QrPageUrl:         "https://excashier.alipay.com/standard/auth.htm",
		CreatedTime:       now - 10,
		UpdatedTime:       now - 5,
		ExpiredTime:       now + 1200,
	})
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:   "ldxp_paid_watch_worker_paid",
		UserId:      1002,
		Amount:      20,
		Money:       20,
		Status:      model.LdxpStatusWorkerPaid,
		WorkerId:    "worker-a",
		CreatedTime: now - 20,
		UpdatedTime: now - 20,
		ExpiredTime: now + 1200,
	})

	session, err := ClaimLdxpPaidWatchSession(" worker-a ", testLdxpSessionConfig(true))

	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, "ldxp_paid_watch_ready", session.SessionId)
	assert.Equal(t, model.LdxpStatusQrReady, session.Status)
	assert.Equal(t, "LDWATCHREADY", session.WorkerOrderNo)
	assert.Equal(t, "https://excashier.alipay.com/standard/auth.htm", session.QrPageUrl)
}

func TestRecordLdxpQrRejectsMissingOrderNo(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:   "ldxp_qr_missing_order_reject",
		UserId:      1001,
		Status:      model.LdxpStatusWorkerClaimed,
		WorkerId:    "worker-a",
		CreatedTime: 100,
		UpdatedTime: 100,
		ExpiredTime: 2000,
	})

	err := RecordLdxpQr("ldxp_qr_missing_order_reject", LdxpWorkerQrPayload{
		WorkerID:          "worker-a",
		WorkerOrderNo:     "   ",
		WorkerAmount:      0.10,
		WorkerProductName: "LDXP 10",
		QRCode:            "data:image/png;base64,created",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrLdxpInvalidSessionRequest)
	persisted, err := model.GetLdxpTopupSessionBySessionId("ldxp_qr_missing_order_reject")
	require.NoError(t, err)
	assert.Equal(t, model.LdxpStatusWorkerClaimed, persisted.Status)
	assert.Empty(t, persisted.WorkerOrderNo)
	assert.Empty(t, persisted.QrCode)
	assert.EqualValues(t, 100, persisted.UpdatedTime)
}

func TestRecordLdxpQrRejectsOrderNoOverwrite(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:     "ldxp_qr_overwrite_reject",
		UserId:        1001,
		Status:        model.LdxpStatusQrReady,
		WorkerId:      "worker-a",
		WorkerOrderNo: "order-B",
		QrCode:        "original-qr",
		CreatedTime:   100,
		UpdatedTime:   100,
		ExpiredTime:   2000,
	})

	err := RecordLdxpQr("ldxp_qr_overwrite_reject", LdxpWorkerQrPayload{
		WorkerID:          "worker-a",
		WorkerOrderNo:     "order-A",
		WorkerAmount:      0.10,
		WorkerProductName: "UPDATED",
		QRCode:            "data:image/png;base64,updated",
		QRPageURL:         "https://example.test/updated",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	persisted, err := model.GetLdxpTopupSessionBySessionId("ldxp_qr_overwrite_reject")
	require.NoError(t, err)
	assert.Equal(t, model.LdxpStatusQrReady, persisted.Status)
	assert.Equal(t, "order-B", persisted.WorkerOrderNo)
	assert.Equal(t, "original-qr", persisted.QrCode)
	assert.Empty(t, persisted.QrPageUrl)
	assert.EqualValues(t, 100, persisted.UpdatedTime)
}

func TestRecordLdxpWorkerResultRejectsStaleOrderNo(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:          "ldxp_result_stale_order_reject",
		UserId:             1001,
		Status:             model.LdxpStatusQrReady,
		WorkerId:           "worker-a",
		WorkerOrderNo:      "order-B",
		QrCode:             "qr",
		WorkerCardKey:      "ORIGINAL-CARD",
		WorkerStatusText:   "original",
		WorkerDetectedTime: 150,
		CreatedTime:        100,
		UpdatedTime:        150,
		ExpiredTime:        2000,
	})

	session, err := RecordLdxpWorkerResult("ldxp_result_stale_order_reject", LdxpWorkerResultPayload{
		WorkerID:          "worker-a",
		WorkerOrderNo:     "order-A",
		WorkerAmount:      0.20,
		WorkerProductName: "LDXP 20",
		WorkerCardKey:     "NEW-CARD",
		WorkerStatusText:  "paid",
		WorkerSuccessURL:  "https://example.test/success",
	})

	require.Error(t, err)
	assert.Nil(t, session)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	persisted, err := model.GetLdxpTopupSessionBySessionId("ldxp_result_stale_order_reject")
	require.NoError(t, err)
	assert.Equal(t, model.LdxpStatusQrReady, persisted.Status)
	assert.Equal(t, "order-B", persisted.WorkerOrderNo)
	assert.Equal(t, "ORIGINAL-CARD", persisted.WorkerCardKey)
	assert.Equal(t, "original", persisted.WorkerStatusText)
	assert.EqualValues(t, 150, persisted.WorkerDetectedTime)
	assert.EqualValues(t, 150, persisted.UpdatedTime)
}

func TestCreateLdxpTopupSessionSerializesConcurrentCreates(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpUserForServiceTest(t, 1001)

	const workers = 16
	start := make(chan struct{})
	results := make(chan *LdxpSessionPublicView, workers)
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		go func() {
			<-start
			view, err := CreateLdxpTopupSession(1001, 10, testLdxpSessionConfig(true))
			if err != nil {
				errs <- err
				return
			}
			results <- view
		}()
	}
	close(start)

	views := make([]*LdxpSessionPublicView, 0, workers)
	for i := 0; i < workers; i++ {
		select {
		case err := <-errs:
			require.NoError(t, err)
		case view := <-results:
			views = append(views, view)
		}
	}

	require.Len(t, views, workers)
	sessionIDs := map[string]struct{}{}
	for _, view := range views {
		require.NotNil(t, view)
		sessionIDs[view.SessionID] = struct{}{}
	}
	assert.Len(t, sessionIDs, 1)

	var count int64
	require.NoError(t, model.DB.Model(&model.LdxpTopupSession{}).Where("user_id = ? AND status IN ?", 1001, []string{model.LdxpStatusCreated, model.LdxpStatusWorkerClaimed, model.LdxpStatusQrReady}).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestRecordLdxpQrRejectsCreatedSessionAndDifferentWorker(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:   "ldxp_qr_created_reject",
		UserId:      1001,
		Status:      model.LdxpStatusCreated,
		CreatedTime: 100,
		UpdatedTime: 100,
		ExpiredTime: 2000,
	})
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:   "ldxp_qr_worker_reject",
		UserId:      1001,
		Status:      model.LdxpStatusWorkerClaimed,
		WorkerId:    "worker-a",
		CreatedTime: 100,
		UpdatedTime: 100,
		ExpiredTime: 2000,
	})

	err := RecordLdxpQr("ldxp_qr_created_reject", LdxpWorkerQrPayload{
		WorkerID:      "worker-a",
		WorkerOrderNo: "order-created-should-not-write",
		QRCode:        "data:image/png;base64,created",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	created, err := model.GetLdxpTopupSessionBySessionId("ldxp_qr_created_reject")
	require.NoError(t, err)
	assert.Equal(t, model.LdxpStatusCreated, created.Status)
	assert.Empty(t, created.WorkerOrderNo)
	assert.Empty(t, created.QrCode)
	assert.EqualValues(t, 100, created.UpdatedTime)

	err = RecordLdxpQr("ldxp_qr_worker_reject", LdxpWorkerQrPayload{
		WorkerID:      "worker-b",
		WorkerOrderNo: "order-wrong-worker",
		QRCode:        "data:image/png;base64,wrongworker",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	claimed, err := model.GetLdxpTopupSessionBySessionId("ldxp_qr_worker_reject")
	require.NoError(t, err)
	assert.Equal(t, model.LdxpStatusWorkerClaimed, claimed.Status)
	assert.Equal(t, "worker-a", claimed.WorkerId)
	assert.Empty(t, claimed.WorkerOrderNo)
	assert.Empty(t, claimed.QrCode)
	assert.EqualValues(t, 100, claimed.UpdatedTime)
}

func TestRecordLdxpQrIgnoresCanceledSessionForSameWorker(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:     "ldxp_qr_canceled_noop",
		UserId:        1001,
		Status:        model.LdxpStatusCanceled,
		WorkerId:      "worker-a",
		WorkerOrderNo: "",
		CreatedTime:   100,
		UpdatedTime:   150,
		ExpiredTime:   2000,
	})

	err := RecordLdxpQr("ldxp_qr_canceled_noop", LdxpWorkerQrPayload{
		WorkerID:          "worker-a",
		WorkerOrderNo:     "order-after-cancel",
		WorkerAmount:      0.10,
		WorkerProductName: "LDXP 10",
		QRCode:            "data:image/png;base64,lateqr",
		QRPageURL:         "https://example.test/qr",
	})

	require.NoError(t, err)
	persisted, err := model.GetLdxpTopupSessionBySessionId("ldxp_qr_canceled_noop")
	require.NoError(t, err)
	assert.Equal(t, model.LdxpStatusCanceled, persisted.Status)
	assert.Empty(t, persisted.WorkerOrderNo)
	assert.Empty(t, persisted.QrCode)
	assert.EqualValues(t, 150, persisted.UpdatedTime)
}

func TestRecordLdxpWorkerErrorIgnoresCanceledSessionForSameWorker(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:   "ldxp_error_canceled_noop",
		UserId:      1001,
		Status:      model.LdxpStatusCanceled,
		WorkerId:    "worker-a",
		CreatedTime: 100,
		UpdatedTime: 150,
		ExpiredTime: 2000,
	})

	err := RecordLdxpWorkerError("ldxp_error_canceled_noop", "worker-a", "worker_flow_failed", "late worker error", "/app/snapshots/late.png")

	require.NoError(t, err)
	persisted, err := model.GetLdxpTopupSessionBySessionId("ldxp_error_canceled_noop")
	require.NoError(t, err)
	assert.Equal(t, model.LdxpStatusCanceled, persisted.Status)
	assert.Empty(t, persisted.ErrorCode)
	assert.Empty(t, persisted.ErrorMessage)
	assert.Empty(t, persisted.DebugSnapshotPath)
	assert.EqualValues(t, 150, persisted.UpdatedTime)
}

func TestRecordLdxpWorkerResultRejectsMissingOrderNo(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:   "ldxp_result_missing_order",
		UserId:      1001,
		Status:      model.LdxpStatusQrReady,
		WorkerId:    "worker-a",
		QrCode:      "qr",
		CreatedTime: 100,
		UpdatedTime: 100,
		ExpiredTime: 2000,
	})

	session, err := RecordLdxpWorkerResult("ldxp_result_missing_order", LdxpWorkerResultPayload{
		WorkerID:          "worker-a",
		WorkerOrderNo:     "   ",
		WorkerAmount:      0.10,
		WorkerProductName: "LDXP 10",
		WorkerCardKey:     "SHOULD-NOT-WRITE",
		WorkerStatusText:  "paid",
	})

	require.Error(t, err)
	assert.Nil(t, session)
	assert.ErrorIs(t, err, ErrLdxpInvalidSessionRequest)
	persisted, err := model.GetLdxpTopupSessionBySessionId("ldxp_result_missing_order")
	require.NoError(t, err)
	assert.Equal(t, model.LdxpStatusQrReady, persisted.Status)
	assert.Empty(t, persisted.WorkerOrderNo)
	assert.Empty(t, persisted.WorkerCardKey)
	assert.EqualValues(t, 100, persisted.UpdatedTime)
}

func TestRecordLdxpWorkerResultRejectsDifferentWorkerAndPaidOverwrite(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:     "ldxp_result_worker_reject",
		UserId:        1001,
		Status:        model.LdxpStatusQrReady,
		WorkerId:      "worker-a",
		WorkerOrderNo: "order-result-1",
		QrCode:        "qr",
		CreatedTime:   100,
		UpdatedTime:   100,
		ExpiredTime:   2000,
	})
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:          "ldxp_result_paid_reject",
		UserId:             1001,
		Status:             model.LdxpStatusWorkerPaid,
		WorkerId:           "worker-a",
		WorkerOrderNo:      "order-paid-1",
		WorkerCardKey:      "ORIGINAL-CARD-KEY",
		WorkerStatusText:   "paid-original",
		WorkerDetectedTime: 150,
		CreatedTime:        100,
		UpdatedTime:        150,
		ExpiredTime:        2000,
	})

	session, err := RecordLdxpWorkerResult("ldxp_result_worker_reject", LdxpWorkerResultPayload{
		WorkerID:      "worker-b",
		WorkerOrderNo: "order-result-1",
		WorkerCardKey: "WRONG-WORKER-CARD",
	})
	require.Error(t, err)
	assert.Nil(t, session)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	wrongWorker, err := model.GetLdxpTopupSessionBySessionId("ldxp_result_worker_reject")
	require.NoError(t, err)
	assert.Equal(t, model.LdxpStatusQrReady, wrongWorker.Status)
	assert.Equal(t, "worker-a", wrongWorker.WorkerId)
	assert.Empty(t, wrongWorker.WorkerCardKey)
	assert.EqualValues(t, 100, wrongWorker.UpdatedTime)

	session, err = RecordLdxpWorkerResult("ldxp_result_paid_reject", LdxpWorkerResultPayload{
		WorkerID:         "worker-a",
		WorkerOrderNo:    "order-paid-1",
		WorkerCardKey:    "REPLACEMENT-CARD-KEY",
		WorkerStatusText: "paid-replacement",
	})
	require.Error(t, err)
	assert.Nil(t, session)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	paid, err := model.GetLdxpTopupSessionBySessionId("ldxp_result_paid_reject")
	require.NoError(t, err)
	assert.Equal(t, model.LdxpStatusWorkerPaid, paid.Status)
	assert.Equal(t, "ORIGINAL-CARD-KEY", paid.WorkerCardKey)
	assert.Equal(t, "paid-original", paid.WorkerStatusText)
	assert.EqualValues(t, 150, paid.WorkerDetectedTime)
	assert.EqualValues(t, 150, paid.UpdatedTime)
}

func TestRecordLdxpWorkerErrorMovesClaimedOrQrSessionToWorkerFailed(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:   "ldxp_error_claimed_success",
		UserId:      1001,
		Status:      model.LdxpStatusWorkerClaimed,
		WorkerId:    "worker-a",
		CreatedTime: 100,
		UpdatedTime: 100,
		ExpiredTime: 2000,
	})
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:   "ldxp_error_qr_success",
		UserId:      1001,
		Status:      model.LdxpStatusQrReady,
		WorkerId:    "worker-a",
		QrCode:      "qr",
		CreatedTime: 100,
		UpdatedTime: 100,
		ExpiredTime: 2000,
	})

	for _, sessionID := range []string{"ldxp_error_claimed_success", "ldxp_error_qr_success"} {
		err := RecordLdxpWorkerError(sessionID, " worker-a ", " qr_failed ", " failed to fetch qr ", " /tmp/ldxp-snapshot.png ")
		require.NoError(t, err)

		persisted, err := model.GetLdxpTopupSessionBySessionId(sessionID)
		require.NoError(t, err)
		assert.Equal(t, model.LdxpStatusWorkerFailed, persisted.Status)
		assert.Equal(t, "worker-a", persisted.WorkerId)
		assert.Equal(t, "qr_failed", persisted.ErrorCode)
		assert.Equal(t, PublicLdxpWorkerFailedMessage(), persisted.ErrorMessage)
		assert.Equal(t, "ldxp-snapshot.png", persisted.DebugSnapshotPath)
		assert.Greater(t, persisted.UpdatedTime, int64(100))
	}
}

func TestRecordLdxpWorkerErrorRejectsCreatedSessionAndDifferentWorker(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:   "ldxp_error_created_reject",
		UserId:      1001,
		Status:      model.LdxpStatusCreated,
		CreatedTime: 100,
		UpdatedTime: 100,
		ExpiredTime: 2000,
	})
	insertLdxpSessionForServiceTest(t, &model.LdxpTopupSession{
		SessionId:   "ldxp_error_worker_reject",
		UserId:      1001,
		Status:      model.LdxpStatusQrReady,
		WorkerId:    "worker-a",
		CreatedTime: 100,
		UpdatedTime: 100,
		ExpiredTime: 2000,
	})

	err := RecordLdxpWorkerError("ldxp_error_created_reject", "worker-a", "created_error", "should not write", "/tmp/created.png")
	require.Error(t, err)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	created, err := model.GetLdxpTopupSessionBySessionId("ldxp_error_created_reject")
	require.NoError(t, err)
	assert.Equal(t, model.LdxpStatusCreated, created.Status)
	assert.Empty(t, created.ErrorCode)
	assert.Empty(t, created.DebugSnapshotPath)
	assert.EqualValues(t, 100, created.UpdatedTime)

	err = RecordLdxpWorkerError("ldxp_error_worker_reject", "worker-b", "wrong_worker", "should not write", "/tmp/wrong.png")
	require.Error(t, err)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	claimed, err := model.GetLdxpTopupSessionBySessionId("ldxp_error_worker_reject")
	require.NoError(t, err)
	assert.Equal(t, model.LdxpStatusQrReady, claimed.Status)
	assert.Equal(t, "worker-a", claimed.WorkerId)
	assert.Empty(t, claimed.ErrorCode)
	assert.Empty(t, claimed.DebugSnapshotPath)
	assert.EqualValues(t, 100, claimed.UpdatedTime)
}

func TestCreateLdxpValuePackageSessionUsesPlanProductConfig(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpUserForServiceTest(t, 1001)
	plan := model.SubscriptionPlan{
		Title:                 "日卡",
		PriceAmount:           9.9,
		Currency:              "USD",
		DurationUnit:          model.SubscriptionDurationDay,
		DurationValue:         1,
		Enabled:               true,
		PlanKind:              model.SubscriptionPlanKindValuePackage,
		PackageType:           model.ValuePackageTypeDay,
		PackageLevel:          model.ValuePackageLevelDay,
		ModelGroup:            "day-card",
		ConcurrencyLimit:      1,
		LdxpProductUrl:        "https://ldxp.example.test/day",
		LdxpProductName:       "日卡商品",
		LdxpProductAmount:     9.9,
		LdxpSessionTTLSeconds: 900,
	}
	require.NoError(t, model.DB.Create(&plan).Error)

	view, order, err := CreateLdxpValuePackageSession(1001, plan.Id, true, testLdxpSessionConfig(true))

	require.NoError(t, err)
	require.NotNil(t, view)
	require.NotNil(t, order)
	assert.Equal(t, 9.9, view.Money)
	assert.Equal(t, common.TopUpStatusPending, order.Status)
	persisted, err := model.GetLdxpTopupSessionBySessionId(view.SessionID)
	require.NoError(t, err)
	assert.Equal(t, model.LdxpPurposeValuePackage, persisted.Purpose)
	assert.Equal(t, order.Id, persisted.SubscriptionOrderId)
	assert.Equal(t, plan.Id, persisted.SubscriptionPlanId)
	assert.Equal(t, "https://ldxp.example.test/day", persisted.ProductUrl)
	assert.Equal(t, "日卡商品", persisted.ProductName)
	assert.EqualValues(t, persisted.CreatedTime+900, persisted.ExpiredTime)
}

func TestCreateLdxpValuePackageSessionRejectsMissingPlanTTL(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpUserForServiceTest(t, 1002)
	plan := model.SubscriptionPlan{
		Title:                 "日卡 TTL 缺失",
		PriceAmount:           9.9,
		Currency:              "USD",
		DurationUnit:          model.SubscriptionDurationDay,
		DurationValue:         1,
		Enabled:               true,
		PlanKind:              model.SubscriptionPlanKindValuePackage,
		PackageType:           model.ValuePackageTypeDay,
		PackageLevel:          model.ValuePackageLevelDay,
		ModelGroup:            "day-card",
		ConcurrencyLimit:      1,
		LdxpProductUrl:        "https://ldxp.example.test/day-ttl-missing",
		LdxpProductName:       "日卡商品 TTL 缺失",
		LdxpProductAmount:     9.9,
		LdxpSessionTTLSeconds: 0,
	}
	require.NoError(t, model.DB.Create(&plan).Error)

	view, order, err := CreateLdxpValuePackageSession(1002, plan.Id, true, testLdxpSessionConfig(true))

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrLdxpInvalidSessionRequest))
	assert.Contains(t, err.Error(), "ldxp product incomplete")
	assert.Nil(t, view)
	assert.Nil(t, order)

	var sessionCount int64
	require.NoError(t, model.DB.Model(&model.LdxpTopupSession{}).Where("user_id = ?", 1002).Count(&sessionCount).Error)
	assert.Zero(t, sessionCount)
	var orderCount int64
	require.NoError(t, model.DB.Model(&model.SubscriptionOrder{}).Where("user_id = ?", 1002).Count(&orderCount).Error)
	assert.Zero(t, orderCount)
}

func TestCreateLdxpValuePackageSessionRejectsActiveSessionWithoutOrderID(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpUserForServiceTest(t, 1003)
	plan := model.SubscriptionPlan{
		Title:                 "日卡 active corrupt",
		PriceAmount:           9.9,
		Currency:              "USD",
		DurationUnit:          model.SubscriptionDurationDay,
		DurationValue:         1,
		Enabled:               true,
		PlanKind:              model.SubscriptionPlanKindValuePackage,
		PackageType:           model.ValuePackageTypeDay,
		PackageLevel:          model.ValuePackageLevelDay,
		ModelGroup:            "day-card",
		ConcurrencyLimit:      1,
		LdxpProductUrl:        "https://ldxp.example.test/day-corrupt",
		LdxpProductName:       "日卡商品 corrupt",
		LdxpProductAmount:     9.9,
		LdxpSessionTTLSeconds: 900,
	}
	require.NoError(t, model.DB.Create(&plan).Error)
	now := common.GetTimestamp()
	require.NoError(t, model.InsertLdxpTopupSession(&model.LdxpTopupSession{
		SessionId:           "ldxp_value_package_corrupt_no_order",
		UserId:              1003,
		Money:               plan.LdxpProductAmount,
		ProductUrl:          plan.LdxpProductUrl,
		ProductName:         plan.LdxpProductName,
		Status:              model.LdxpStatusCreated,
		Purpose:             model.LdxpPurposeValuePackage,
		SubscriptionOrderId: 0,
		SubscriptionPlanId:  plan.Id,
		CreatedTime:         now,
		UpdatedTime:         now,
		ExpiredTime:         now + 900,
	}))

	view, order, err := CreateLdxpValuePackageSession(1003, plan.Id, true, testLdxpSessionConfig(true))

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrLdxpInvalidSessionRequest))
	assert.Contains(t, err.Error(), "active value package session mismatch")
	assert.Nil(t, view)
	assert.Nil(t, order)
}

func TestCreateLdxpValuePackageSessionRejectsActiveSessionMissingOrder(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpUserForServiceTest(t, 1004)
	plan := model.SubscriptionPlan{
		Title:                 "日卡 active missing order",
		PriceAmount:           9.9,
		Currency:              "USD",
		DurationUnit:          model.SubscriptionDurationDay,
		DurationValue:         1,
		Enabled:               true,
		PlanKind:              model.SubscriptionPlanKindValuePackage,
		PackageType:           model.ValuePackageTypeDay,
		PackageLevel:          model.ValuePackageLevelDay,
		ModelGroup:            "day-card",
		ConcurrencyLimit:      1,
		LdxpProductUrl:        "https://ldxp.example.test/day-missing-order",
		LdxpProductName:       "日卡商品 missing order",
		LdxpProductAmount:     9.9,
		LdxpSessionTTLSeconds: 900,
	}
	require.NoError(t, model.DB.Create(&plan).Error)
	now := common.GetTimestamp()
	require.NoError(t, model.InsertLdxpTopupSession(&model.LdxpTopupSession{
		SessionId:           "ldxp_value_package_corrupt_missing_order",
		UserId:              1004,
		Money:               plan.LdxpProductAmount,
		ProductUrl:          plan.LdxpProductUrl,
		ProductName:         plan.LdxpProductName,
		Status:              model.LdxpStatusCreated,
		Purpose:             model.LdxpPurposeValuePackage,
		SubscriptionOrderId: 424242,
		SubscriptionPlanId:  plan.Id,
		CreatedTime:         now,
		UpdatedTime:         now,
		ExpiredTime:         now + 900,
	}))

	view, order, err := CreateLdxpValuePackageSession(1004, plan.Id, true, testLdxpSessionConfig(true))

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrLdxpInvalidSessionRequest))
	assert.Contains(t, err.Error(), "active value package session mismatch")
	assert.Nil(t, view)
	assert.Nil(t, order)
}

func TestCreateLdxpValuePackageSessionPreservesActiveOrderLookupDBError(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpUserForServiceTest(t, 1014)
	plan := model.SubscriptionPlan{
		Title:                 "日卡 active db error",
		PriceAmount:           9.9,
		Currency:              "USD",
		DurationUnit:          model.SubscriptionDurationDay,
		DurationValue:         1,
		Enabled:               true,
		PlanKind:              model.SubscriptionPlanKindValuePackage,
		PackageType:           model.ValuePackageTypeDay,
		PackageLevel:          model.ValuePackageLevelDay,
		ModelGroup:            "day-card",
		ConcurrencyLimit:      1,
		LdxpProductUrl:        "https://ldxp.example.test/day-db-error",
		LdxpProductName:       "日卡商品 db error",
		LdxpProductAmount:     9.9,
		LdxpSessionTTLSeconds: 900,
	}
	require.NoError(t, model.DB.Create(&plan).Error)
	now := common.GetTimestamp()
	require.NoError(t, model.InsertLdxpTopupSession(&model.LdxpTopupSession{
		SessionId:           "ldxp_value_package_active_order_db_error",
		UserId:              1014,
		Money:               plan.LdxpProductAmount,
		ProductUrl:          plan.LdxpProductUrl,
		ProductName:         plan.LdxpProductName,
		Status:              model.LdxpStatusCreated,
		Purpose:             model.LdxpPurposeValuePackage,
		SubscriptionOrderId: 424243,
		SubscriptionPlanId:  plan.Id,
		CreatedTime:         now,
		UpdatedTime:         now,
		ExpiredTime:         now + 900,
	}))
	forcedErr := errors.New("forced active linked order lookup failure")
	callbackName := "test:force_active_linked_order_lookup_error:" + strings.ReplaceAll(t.Name(), "/", "_")
	require.NoError(t, model.DB.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema == nil || tx.Statement.Schema.Name != "SubscriptionOrder" {
			return
		}
		where := fmt.Sprintf("%#v", tx.Statement.Clauses["WHERE"].Expression)
		if strings.Contains(where, "id = ?") {
			tx.AddError(forcedErr)
		}
	}))
	t.Cleanup(func() {
		require.NoError(t, model.DB.Callback().Query().Remove(callbackName))
	})

	view, order, err := CreateLdxpValuePackageSession(1014, plan.Id, true, testLdxpSessionConfig(true))

	require.Error(t, err)
	require.ErrorIs(t, err, forcedErr)
	require.False(t, errors.Is(err, ErrLdxpInvalidSessionRequest), "infrastructure lookup errors must not be converted to invalid session request")
	assert.Nil(t, view)
	assert.Nil(t, order)
}

func TestCreateLdxpValuePackageSessionRejectsActiveSessionForDifferentPlan(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpUserForServiceTest(t, 1005)
	dayPlan := model.SubscriptionPlan{
		Title:                 "日卡 active mismatch",
		PriceAmount:           9.9,
		Currency:              "USD",
		DurationUnit:          model.SubscriptionDurationDay,
		DurationValue:         1,
		Enabled:               true,
		PlanKind:              model.SubscriptionPlanKindValuePackage,
		PackageType:           model.ValuePackageTypeDay,
		PackageLevel:          model.ValuePackageLevelDay,
		ModelGroup:            "day-card",
		ConcurrencyLimit:      1,
		LdxpProductUrl:        "https://ldxp.example.test/day-mismatch",
		LdxpProductName:       "日卡商品 mismatch",
		LdxpProductAmount:     9.9,
		LdxpSessionTTLSeconds: 900,
	}
	monthPlan := model.SubscriptionPlan{
		Title:                 "月卡 active mismatch",
		PriceAmount:           39.9,
		Currency:              "USD",
		DurationUnit:          model.SubscriptionDurationMonth,
		DurationValue:         1,
		Enabled:               true,
		PlanKind:              model.SubscriptionPlanKindValuePackage,
		PackageType:           model.ValuePackageTypeMonth,
		PackageLevel:          model.ValuePackageLevelMonth,
		ModelGroup:            "month-card",
		ConcurrencyLimit:      1,
		LdxpProductUrl:        "https://ldxp.example.test/month-mismatch",
		LdxpProductName:       "月卡商品 mismatch",
		LdxpProductAmount:     39.9,
		LdxpSessionTTLSeconds: 900,
	}
	require.NoError(t, model.DB.Create(&dayPlan).Error)
	require.NoError(t, model.DB.Create(&monthPlan).Error)
	now := common.GetTimestamp()
	order := model.SubscriptionOrder{
		UserId:          1005,
		PlanId:          dayPlan.Id,
		Money:           dayPlan.LdxpProductAmount,
		TradeNo:         "LDXP_VP-day-active-mismatch",
		PaymentMethod:   model.PaymentMethodLDXP,
		PaymentProvider: model.PaymentProviderLDXP,
		CreateTime:      now,
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, model.DB.Create(&order).Error)
	require.NoError(t, model.InsertLdxpTopupSession(&model.LdxpTopupSession{
		SessionId:           "ldxp_value_package_day_active_mismatch",
		UserId:              1005,
		Money:               dayPlan.LdxpProductAmount,
		ProductUrl:          dayPlan.LdxpProductUrl,
		ProductName:         dayPlan.LdxpProductName,
		Status:              model.LdxpStatusCreated,
		Purpose:             model.LdxpPurposeValuePackage,
		SubscriptionOrderId: order.Id,
		SubscriptionPlanId:  dayPlan.Id,
		ConfirmedCover:      true,
		CreatedTime:         now,
		UpdatedTime:         now,
		ExpiredTime:         now + 900,
	}))

	view, reusedOrder, err := CreateLdxpValuePackageSession(1005, monthPlan.Id, true, testLdxpSessionConfig(true))

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrLdxpInvalidSessionRequest))
	assert.Contains(t, err.Error(), "active value package session mismatch")
	assert.Nil(t, view)
	assert.Nil(t, reusedOrder)
}

func TestCreateLdxpValuePackageSessionRejectsActiveSessionForDifferentConfirmedCover(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpUserForServiceTest(t, 1006)
	plan := model.SubscriptionPlan{
		Title:                 "日卡 confirmed cover mismatch",
		PriceAmount:           9.9,
		Currency:              "USD",
		DurationUnit:          model.SubscriptionDurationDay,
		DurationValue:         1,
		Enabled:               true,
		PlanKind:              model.SubscriptionPlanKindValuePackage,
		PackageType:           model.ValuePackageTypeDay,
		PackageLevel:          model.ValuePackageLevelDay,
		ModelGroup:            "day-card",
		ConcurrencyLimit:      1,
		LdxpProductUrl:        "https://ldxp.example.test/confirmed-cover-mismatch",
		LdxpProductName:       "日卡商品 confirmed cover mismatch",
		LdxpProductAmount:     9.9,
		LdxpSessionTTLSeconds: 900,
	}
	require.NoError(t, model.DB.Create(&plan).Error)
	now := common.GetTimestamp()
	order := model.SubscriptionOrder{
		UserId:          1006,
		PlanId:          plan.Id,
		Money:           plan.LdxpProductAmount,
		TradeNo:         "LDXP_VP-confirmed-cover-mismatch",
		PaymentMethod:   model.PaymentMethodLDXP,
		PaymentProvider: model.PaymentProviderLDXP,
		CreateTime:      now,
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, model.DB.Create(&order).Error)
	require.NoError(t, model.InsertLdxpTopupSession(&model.LdxpTopupSession{
		SessionId:           "ldxp_value_package_confirmed_cover_mismatch",
		UserId:              1006,
		Money:               plan.LdxpProductAmount,
		ProductUrl:          plan.LdxpProductUrl,
		ProductName:         plan.LdxpProductName,
		Status:              model.LdxpStatusCreated,
		Purpose:             model.LdxpPurposeValuePackage,
		SubscriptionOrderId: order.Id,
		SubscriptionPlanId:  plan.Id,
		ConfirmedCover:      false,
		CreatedTime:         now,
		UpdatedTime:         now,
		ExpiredTime:         now + 900,
	}))

	view, reusedOrder, err := CreateLdxpValuePackageSession(1006, plan.Id, true, testLdxpSessionConfig(true))

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrLdxpInvalidSessionRequest))
	assert.Contains(t, err.Error(), "active value package session mismatch")
	assert.Nil(t, view)
	assert.Nil(t, reusedOrder)
}

func TestCreateLdxpValuePackageSessionRejectsActiveSessionLinkedOrderWrongUser(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpUserForServiceTest(t, 1007)
	insertLdxpUserForServiceTest(t, 2007)
	plan := model.SubscriptionPlan{
		Title:                 "日卡 active wrong user",
		PriceAmount:           9.9,
		Currency:              "USD",
		DurationUnit:          model.SubscriptionDurationDay,
		DurationValue:         1,
		Enabled:               true,
		PlanKind:              model.SubscriptionPlanKindValuePackage,
		PackageType:           model.ValuePackageTypeDay,
		PackageLevel:          model.ValuePackageLevelDay,
		ModelGroup:            "day-card",
		ConcurrencyLimit:      1,
		LdxpProductUrl:        "https://ldxp.example.test/day-wrong-user",
		LdxpProductName:       "日卡商品 wrong user",
		LdxpProductAmount:     9.9,
		LdxpSessionTTLSeconds: 900,
	}
	require.NoError(t, model.DB.Create(&plan).Error)
	now := common.GetTimestamp()
	order := model.SubscriptionOrder{
		UserId:          2007,
		PlanId:          plan.Id,
		Money:           plan.LdxpProductAmount,
		TradeNo:         "LDXP_VP-active-wrong-user",
		PaymentMethod:   model.PaymentMethodLDXP,
		PaymentProvider: model.PaymentProviderLDXP,
		CreateTime:      now,
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, model.DB.Create(&order).Error)
	require.NoError(t, model.InsertLdxpTopupSession(&model.LdxpTopupSession{
		SessionId:           "ldxp_value_package_active_wrong_user",
		UserId:              1007,
		Money:               plan.LdxpProductAmount,
		ProductUrl:          plan.LdxpProductUrl,
		ProductName:         plan.LdxpProductName,
		Status:              model.LdxpStatusCreated,
		Purpose:             model.LdxpPurposeValuePackage,
		SubscriptionOrderId: order.Id,
		SubscriptionPlanId:  plan.Id,
		ConfirmedCover:      true,
		CreatedTime:         now,
		UpdatedTime:         now,
		ExpiredTime:         now + 900,
	}))

	view, reusedOrder, err := CreateLdxpValuePackageSession(1007, plan.Id, true, testLdxpSessionConfig(true))

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrLdxpInvalidSessionRequest))
	assert.Contains(t, err.Error(), "active value package session mismatch")
	assert.Nil(t, view)
	assert.Nil(t, reusedOrder)
}

func TestCreateLdxpValuePackageSessionReusesExactMatchingActiveSessionAndOrder(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpUserForServiceTest(t, 1008)
	plan := model.SubscriptionPlan{
		Title:                 "日卡 exact reuse",
		PriceAmount:           9.9,
		Currency:              "USD",
		DurationUnit:          model.SubscriptionDurationDay,
		DurationValue:         1,
		Enabled:               true,
		PlanKind:              model.SubscriptionPlanKindValuePackage,
		PackageType:           model.ValuePackageTypeDay,
		PackageLevel:          model.ValuePackageLevelDay,
		ModelGroup:            "day-card",
		ConcurrencyLimit:      1,
		LdxpProductUrl:        "https://ldxp.example.test/day-exact-reuse",
		LdxpProductName:       "日卡商品 exact reuse",
		LdxpProductAmount:     9.9,
		LdxpSessionTTLSeconds: 900,
	}
	require.NoError(t, model.DB.Create(&plan).Error)
	now := common.GetTimestamp()
	order := model.SubscriptionOrder{
		UserId:          1008,
		PlanId:          plan.Id,
		Money:           plan.LdxpProductAmount,
		TradeNo:         "LDXP_VP-exact-reuse",
		PaymentMethod:   model.PaymentMethodLDXP,
		PaymentProvider: model.PaymentProviderLDXP,
		CreateTime:      now,
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, model.DB.Create(&order).Error)
	require.NoError(t, model.InsertLdxpTopupSession(&model.LdxpTopupSession{
		SessionId:           "ldxp_value_package_exact_reuse",
		UserId:              1008,
		Money:               plan.LdxpProductAmount,
		ProductUrl:          plan.LdxpProductUrl,
		ProductName:         plan.LdxpProductName,
		Status:              model.LdxpStatusCreated,
		Purpose:             model.LdxpPurposeValuePackage,
		SubscriptionOrderId: order.Id,
		SubscriptionPlanId:  plan.Id,
		ConfirmedCover:      true,
		CreatedTime:         now,
		UpdatedTime:         now,
		ExpiredTime:         now + 900,
	}))

	view, reusedOrder, err := CreateLdxpValuePackageSession(1008, plan.Id, true, testLdxpSessionConfig(true))

	require.NoError(t, err)
	require.NotNil(t, view)
	require.NotNil(t, reusedOrder)
	assert.Equal(t, "ldxp_value_package_exact_reuse", view.SessionID)
	assert.Equal(t, order.Id, reusedOrder.Id)
	var sessionCount int64
	require.NoError(t, model.DB.Model(&model.LdxpTopupSession{}).Where("user_id = ?", 1008).Count(&sessionCount).Error)
	assert.EqualValues(t, 1, sessionCount)
	var orderCount int64
	require.NoError(t, model.DB.Model(&model.SubscriptionOrder{}).Where("user_id = ?", 1008).Count(&orderCount).Error)
	assert.EqualValues(t, 1, orderCount)
}
