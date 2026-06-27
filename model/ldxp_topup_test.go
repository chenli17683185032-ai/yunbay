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
		DB.Exec("DELETE FROM ldxp_topup_sessions")
		DB.Exec("DELETE FROM ldxp_mail_events")
	}
	cleanup()
	t.Cleanup(cleanup)
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
		MessageId:    "message-1",
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
	duplicate := &LdxpMailEvent{MessageId: "message-2", ImapUid: "uid-2", RawHash: "same-raw-hash", OrderNo: "order-2"}

	require.NoError(t, InsertLdxpMailEvent(first))
	require.Error(t, InsertLdxpMailEvent(duplicate))

	persisted, err := GetLdxpMailEventByOrderNo("order-1")
	require.NoError(t, err)
	assert.Equal(t, "same-raw-hash", persisted.RawHash)
	assert.Equal(t, "CARD-KEY-1", persisted.CardKey)
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
