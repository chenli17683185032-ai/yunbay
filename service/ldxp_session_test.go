package service

import (
	"errors"
	"fmt"
	"regexp"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupLdxpSessionServiceTest(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(&model.User{}, &model.LdxpTopupSession{}, &model.LdxpMailEvent{}))
	cleanup := func() {
		require.NoError(t, model.DB.Exec("DELETE FROM ldxp_topup_sessions").Error)
		require.NoError(t, model.DB.Exec("DELETE FROM ldxp_mail_events").Error)
		require.NoError(t, model.DB.Exec("DELETE FROM users").Error)
	}
	cleanup()
	t.Cleanup(cleanup)
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
		QRCode:            "created-qr",
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
		QRCode:            "updated-qr",
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
	require.NoError(t, model.DB.Create(&model.User{Id: 1001, Username: "user-1001"}).Error)

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
		QRCode:        "created-qr",
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
		QRCode:        "wrong-worker-qr",
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
		assert.Equal(t, "failed to fetch qr", persisted.ErrorMessage)
		assert.Equal(t, "/tmp/ldxp-snapshot.png", persisted.DebugSnapshotPath)
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
