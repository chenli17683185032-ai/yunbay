package model

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupLdxpTopupTest(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&LdxpTopupSession{}, &LdxpMailEvent{}))
	cleanup := func() {
		require.NoError(t, DB.Exec("DELETE FROM ldxp_topup_sessions").Error)
		require.NoError(t, DB.Exec("DELETE FROM ldxp_mail_events").Error)
	}
	cleanup()
	t.Cleanup(cleanup)
}

func ldxpMessageId(value string) *string {
	return &value
}

func TestLdxpTopupSessionModelMigratesAndPersists(t *testing.T) {
	setupLdxpTopupTest(t)

	session := &LdxpTopupSession{
		SessionId:    "ldxp-session-persist",
		UserId:       1001,
		Amount:       200,
		Money:        19.99,
		ProductUrl:   "https://example.test/product/1",
		ProductName:  "LDXP Topup Card",
		ContactEmail: "buyer@example.test",
		Status:       LdxpStatusCreated,
		CreatedTime:  100,
		UpdatedTime:  100,
		ExpiredTime:  200,
	}

	require.NoError(t, InsertLdxpTopupSession(session))
	assert.NotZero(t, session.Id)

	persisted, err := GetLdxpTopupSessionBySessionId("ldxp-session-persist")
	require.NoError(t, err)
	require.NotNil(t, persisted)
	assert.Equal(t, "ldxp-session-persist", persisted.SessionId)
	assert.Equal(t, 1001, persisted.UserId)
	assert.EqualValues(t, 200, persisted.Amount)
	assert.Equal(t, 19.99, persisted.Money)
	assert.Equal(t, "https://example.test/product/1", persisted.ProductUrl)
	assert.Equal(t, "LDXP Topup Card", persisted.ProductName)
	assert.Equal(t, "buyer@example.test", persisted.ContactEmail)
	assert.Equal(t, LdxpStatusCreated, persisted.Status)
	assert.EqualValues(t, 200, persisted.ExpiredTime)

	forUser, err := GetLdxpTopupSessionForUser("ldxp-session-persist", 1001)
	require.NoError(t, err)
	assert.Equal(t, persisted.Id, forUser.Id)

	_, err = GetLdxpTopupSessionForUser("ldxp-session-persist", 1002)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound), "got %v", err)
}

func TestLdxpTopupSessionIDIsUnique(t *testing.T) {
	setupLdxpTopupTest(t)

	first := &LdxpTopupSession{SessionId: "duplicate-session", UserId: 1001, Status: LdxpStatusCreated, ExpiredTime: 200}
	second := &LdxpTopupSession{SessionId: "duplicate-session", UserId: 1002, Status: LdxpStatusCreated, ExpiredTime: 200}

	require.NoError(t, InsertLdxpTopupSession(first))
	require.Error(t, InsertLdxpTopupSession(second))
}

func TestLdxpMailEventDedupesByRawHash(t *testing.T) {
	setupLdxpTopupTest(t)

	first := &LdxpMailEvent{
		MessageId:    ldxpMessageId("message-1"),
		ImapUid:      "uid-1",
		RawHash:      "same-raw-hash",
		MailFrom:     "sender@example.test",
		MailTo:       "buyer@example.test",
		Subject:      "Paid",
		ReceivedTime: 100,
		OrderNo:      "order-1",
		Amount:       9.99,
		ProductName:  "LDXP Card",
		CardKey:      "CARD-KEY-1",
		PaidTime:     101,
		BodyExcerpt:  "paid successfully",
		CreatedTime:  102,
	}
	duplicate := &LdxpMailEvent{MessageId: ldxpMessageId("message-2"), ImapUid: "uid-2", RawHash: "same-raw-hash", OrderNo: "order-2"}

	require.NoError(t, InsertLdxpMailEvent(first))
	require.Error(t, InsertLdxpMailEvent(duplicate))

	persisted, err := GetLdxpMailEventByOrderNo("order-1")
	require.NoError(t, err)
	assert.Equal(t, "same-raw-hash", persisted.RawHash)
	assert.Equal(t, "CARD-KEY-1", persisted.CardKey)
}

func TestLdxpMailEventDedupesByMessageId(t *testing.T) {
	setupLdxpTopupTest(t)

	first := &LdxpMailEvent{
		MessageId: ldxpMessageId("same-message-id"),
		ImapUid:   "uid-message-1",
		RawHash:   "message-raw-hash-1",
		OrderNo:   "message-order-1",
	}
	duplicate := &LdxpMailEvent{
		MessageId: ldxpMessageId("same-message-id"),
		ImapUid:   "uid-message-2",
		RawHash:   "message-raw-hash-2",
		OrderNo:   "message-order-2",
	}

	require.NoError(t, InsertLdxpMailEvent(first))
	require.Error(t, InsertLdxpMailEvent(duplicate))
}

func TestLdxpMailEventAllowsMultipleMissingMessageIds(t *testing.T) {
	setupLdxpTopupTest(t)

	first := &LdxpMailEvent{RawHash: "missing-message-raw-hash-1", OrderNo: "missing-message-order-1"}
	second := &LdxpMailEvent{RawHash: "missing-message-raw-hash-2", OrderNo: "missing-message-order-2"}
	blank := &LdxpMailEvent{MessageId: ldxpMessageId("  	"), RawHash: "missing-message-raw-hash-3", OrderNo: "missing-message-order-3"}

	require.NoError(t, InsertLdxpMailEvent(first))
	require.NoError(t, InsertLdxpMailEvent(second))
	require.NoError(t, InsertLdxpMailEvent(blank))
	assert.Nil(t, blank.MessageId)
}

func TestInsertLdxpMailEventRejectsInvalidRawHash(t *testing.T) {
	setupLdxpTopupTest(t)

	require.ErrorIs(t, InsertLdxpMailEvent(nil), gorm.ErrInvalidData)
	require.ErrorIs(t, InsertLdxpMailEvent(&LdxpMailEvent{MessageId: ldxpMessageId("missing-raw-hash"), RawHash: " 	"}), gorm.ErrInvalidData)
}

func TestClaimSelectedLdxpTopupSessionDoesNotOverwriteStaleClaim(t *testing.T) {
	setupLdxpTopupTest(t)

	now := int64(1_000)
	session := &LdxpTopupSession{
		SessionId:   "stale-claim",
		UserId:      1001,
		Status:      LdxpStatusCreated,
		CreatedTime: 10,
		UpdatedTime: 10,
		ExpiredTime: now + 100,
	}
	require.NoError(t, InsertLdxpTopupSession(session))

	staleCandidate := *session
	require.NoError(t, DB.Model(&LdxpTopupSession{}).
		Where("id = ?", session.Id).
		Updates(map[string]interface{}{
			"status":       LdxpStatusWorkerClaimed,
			"worker_id":    "worker-a",
			"updated_time": now,
		}).Error)

	claimed, err := claimSelectedLdxpTopupSession(DB, &staleCandidate, "worker-b", now+1)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	assert.Nil(t, claimed)

	persisted, err := GetLdxpTopupSessionBySessionId("stale-claim")
	require.NoError(t, err)
	assert.Equal(t, LdxpStatusWorkerClaimed, persisted.Status)
	assert.Equal(t, "worker-a", persisted.WorkerId)
	assert.Equal(t, now, persisted.UpdatedTime)
}

func TestGetClaimableLdxpSessionSkipsExpiredAndNonCreated(t *testing.T) {
	setupLdxpTopupTest(t)

	now := int64(1_000)
	sessions := []*LdxpTopupSession{
		{SessionId: "expired-created", UserId: 1001, Status: LdxpStatusCreated, CreatedTime: 10, UpdatedTime: 10, ExpiredTime: now},
		{SessionId: "already-claimed", UserId: 1001, Status: LdxpStatusWorkerClaimed, CreatedTime: 20, UpdatedTime: 20, ExpiredTime: now + 100},
		{SessionId: "claimable", UserId: 1001, Status: LdxpStatusCreated, CreatedTime: 30, UpdatedTime: 30, ExpiredTime: now + 100},
	}
	for _, session := range sessions {
		require.NoError(t, InsertLdxpTopupSession(session))
	}

	claimed, err := ClaimNextLdxpTopupSession("worker-a", now)
	require.NoError(t, err)
	require.NotNil(t, claimed)
	assert.Equal(t, "claimable", claimed.SessionId)
	assert.Equal(t, LdxpStatusWorkerClaimed, claimed.Status)
	assert.Equal(t, "worker-a", claimed.WorkerId)
	assert.Equal(t, now, claimed.UpdatedTime)

	persisted, err := GetLdxpTopupSessionBySessionId("claimable")
	require.NoError(t, err)
	assert.Equal(t, LdxpStatusWorkerClaimed, persisted.Status)
	assert.Equal(t, "worker-a", persisted.WorkerId)
	assert.Equal(t, now, persisted.UpdatedTime)

	_, err = ClaimNextLdxpTopupSession("worker-b", now)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound), "got %v", err)
}

func TestClaimNextLdxpPaidWatchSessionReturnsQrReadySession(t *testing.T) {
	setupLdxpTopupTest(t)

	now := int64(1_000)
	sessions := []*LdxpTopupSession{
		{SessionId: "paid-watch-missing-url", UserId: 1001, Status: LdxpStatusQrReady, WorkerId: "worker-a", WorkerOrderNo: "LDNOURL", CreatedTime: 10, UpdatedTime: 10, ExpiredTime: now + 100},
		{SessionId: "paid-watch-other-worker", UserId: 1001, Status: LdxpStatusQrReady, WorkerId: "worker-b", WorkerOrderNo: "LDOTHER", QrPageUrl: "https://example.test/other", CreatedTime: 20, UpdatedTime: 20, ExpiredTime: now + 100},
		{SessionId: "paid-watch-worker-paid", UserId: 1001, Status: LdxpStatusWorkerPaid, WorkerId: "worker-a", WorkerOrderNo: "LDPAID", QrPageUrl: "https://example.test/paid", CreatedTime: 30, UpdatedTime: 30, ExpiredTime: now + 100},
		{SessionId: "paid-watch-ready", UserId: 1001, Status: LdxpStatusQrReady, WorkerId: "worker-a", WorkerOrderNo: "LDREADY", QrPageUrl: "https://example.test/ready", CreatedTime: 40, UpdatedTime: 40, ExpiredTime: now + 100},
	}
	for _, session := range sessions {
		require.NoError(t, InsertLdxpTopupSession(session))
	}

	watch, err := ClaimNextLdxpPaidWatchSession("worker-a", now)

	require.NoError(t, err)
	require.NotNil(t, watch)
	assert.Equal(t, "paid-watch-ready", watch.SessionId)
	assert.Equal(t, LdxpStatusQrReady, watch.Status)
	assert.Equal(t, "LDREADY", watch.WorkerOrderNo)
	assert.Equal(t, "https://example.test/ready", watch.QrPageUrl)
}

func TestClaimNextLdxpPaidWatchSessionRotatesBetweenQrReadySessions(t *testing.T) {
	setupLdxpTopupTest(t)

	now := int64(1_000)
	sessions := []*LdxpTopupSession{
		{
			SessionId:     "paid-watch-first",
			UserId:        1001,
			Status:        LdxpStatusQrReady,
			WorkerId:      "worker-a",
			WorkerOrderNo: "LDFIRST",
			QrPageUrl:     "https://example.test/first",
			CreatedTime:   10,
			UpdatedTime:   10,
			ExpiredTime:   now + 100,
		},
		{
			SessionId:     "paid-watch-second",
			UserId:        1002,
			Status:        LdxpStatusQrReady,
			WorkerId:      "worker-a",
			WorkerOrderNo: "LDSECOND",
			QrPageUrl:     "https://example.test/second",
			CreatedTime:   20,
			UpdatedTime:   20,
			ExpiredTime:   now + 100,
		},
	}
	for _, session := range sessions {
		require.NoError(t, InsertLdxpTopupSession(session))
	}

	first, err := ClaimNextLdxpPaidWatchSession("worker-a", now)
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.Equal(t, "paid-watch-first", first.SessionId)

	second, err := ClaimNextLdxpPaidWatchSession("worker-a", now+1)
	require.NoError(t, err)
	require.NotNil(t, second)
	assert.Equal(t, "paid-watch-second", second.SessionId)

	persistedFirst, err := GetLdxpTopupSessionBySessionId("paid-watch-first")
	require.NoError(t, err)
	assert.Equal(t, "worker-a", persistedFirst.PaidWatchWorkerId)
	assert.Equal(t, now, persistedFirst.PaidWatchClaimedTime)

	persistedSecond, err := GetLdxpTopupSessionBySessionId("paid-watch-second")
	require.NoError(t, err)
	assert.Equal(t, "worker-a", persistedSecond.PaidWatchWorkerId)
	assert.Equal(t, now+1, persistedSecond.PaidWatchClaimedTime)
}

func TestClaimNextLdxpPaidWatchSessionPrefersNeverWatchedSession(t *testing.T) {
	setupLdxpTopupTest(t)

	now := int64(1_000)
	sessions := []*LdxpTopupSession{
		{
			SessionId:            "paid-watch-already-polled",
			UserId:               1001,
			Status:               LdxpStatusQrReady,
			WorkerId:             "worker-a",
			WorkerOrderNo:        "LDPOLLED",
			QrPageUrl:            "https://example.test/polled",
			CreatedTime:          10,
			UpdatedTime:          10,
			ExpiredTime:          now + 100,
			PaidWatchWorkerId:    "worker-a",
			PaidWatchClaimedTime: now - 1,
		},
		{
			SessionId:     "paid-watch-never-polled",
			UserId:        1002,
			Status:        LdxpStatusQrReady,
			WorkerId:      "worker-a",
			WorkerOrderNo: "LDNEVER",
			QrPageUrl:     "https://example.test/never",
			CreatedTime:   20,
			UpdatedTime:   20,
			ExpiredTime:   now + 100,
		},
	}
	for _, session := range sessions {
		require.NoError(t, InsertLdxpTopupSession(session))
	}

	watch, err := ClaimNextLdxpPaidWatchSession("worker-a", now)

	require.NoError(t, err)
	require.NotNil(t, watch)
	assert.Equal(t, "paid-watch-never-polled", watch.SessionId)
	assert.Equal(t, now, watch.PaidWatchClaimedTime)
}

func TestUpdateLdxpSessionStatusPersistsWorkerFields(t *testing.T) {
	setupLdxpTopupTest(t)

	session := &LdxpTopupSession{
		SessionId:   "worker-update",
		UserId:      1001,
		Status:      LdxpStatusCreated,
		CreatedTime: 100,
		UpdatedTime: 100,
		ExpiredTime: 500,
	}
	require.NoError(t, InsertLdxpTopupSession(session))

	session.Status = LdxpStatusWorkerPaid
	session.WorkerId = "worker-a"
	session.WorkerOrderNo = "worker-order-1"
	session.WorkerAmount = 19.99
	session.WorkerProductName = "LDXP Paid Card"
	session.WorkerCardKey = "CARD-KEY-2"
	session.WorkerStatusText = "paid"
	session.WorkerSuccessUrl = "https://example.test/success"
	session.WorkerDetectedTime = 300
	session.UpdatedTime = 301
	require.NoError(t, SaveLdxpTopupSession(session))

	persisted, err := GetLdxpTopupSessionBySessionId("worker-update")
	require.NoError(t, err)
	assert.Equal(t, LdxpStatusWorkerPaid, persisted.Status)
	assert.Equal(t, "worker-a", persisted.WorkerId)
	assert.Equal(t, "worker-order-1", persisted.WorkerOrderNo)
	assert.Equal(t, 19.99, persisted.WorkerAmount)
	assert.Equal(t, "LDXP Paid Card", persisted.WorkerProductName)
	assert.Equal(t, "CARD-KEY-2", persisted.WorkerCardKey)
	assert.Equal(t, "paid", persisted.WorkerStatusText)
	assert.Equal(t, "https://example.test/success", persisted.WorkerSuccessUrl)
	assert.EqualValues(t, 300, persisted.WorkerDetectedTime)
	assert.EqualValues(t, 301, persisted.UpdatedTime)
}

func TestLdxpTopupSessionPersistsValuePackagePurpose(t *testing.T) {
	setupLdxpTopupTest(t)

	session := &LdxpTopupSession{
		SessionId:           "ldxp-vp-session",
		UserId:              1001,
		Amount:              0,
		Money:               9.90,
		ProductUrl:          "https://example.test/value-package/day",
		ProductName:         "日卡",
		Status:              LdxpStatusCreated,
		Purpose:             LdxpPurposeValuePackage,
		SubscriptionOrderId: 7001,
		SubscriptionPlanId:  8001,
		CreatedTime:         100,
		UpdatedTime:         100,
		ExpiredTime:         200,
	}

	require.NoError(t, InsertLdxpTopupSession(session))
	persisted, err := GetLdxpTopupSessionBySessionId("ldxp-vp-session")
	require.NoError(t, err)
	assert.Equal(t, LdxpPurposeValuePackage, persisted.Purpose)
	assert.Equal(t, 7001, persisted.SubscriptionOrderId)
	assert.Equal(t, 8001, persisted.SubscriptionPlanId)
}
