package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrderManagementModelsPersistCentsAndStatuses(t *testing.T) {
	truncateTables(t)

	session := &LdxpTopupSession{
		SessionId:         "ldxp_test_1",
		UserId:            1001,
		TradeNo:           "WAFFO_PANCAKE-1001-1",
		SiteAmountCents:   50000,
		ExternalPaidCents: 42500,
		WorkerOrderNo:     "LD260628UZJ97P",
		MailStatus:        MailCheckStatusPending,
		CreatedTime:       1782600000,
	}
	require.NoError(t, DB.Create(session).Error)

	mailEvent := &LdxpMailEvent{
		SourceAccount: "orders@example.com",
		MessageId:     "message-ldxp-test-1",
		ImapUid:       "uid-ldxp-test-1",
		RawHash:       "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		OrderNo:       "LD260628UZJ97P",
		PaidCents:     42500,
		ParseStatus:   "parsed",
		CreatedTime:   1782600010,
	}
	require.NoError(t, DB.Create(mailEvent).Error)

	commission := &AffiliateCommission{
		InviterUserId:   3001,
		InviteeUserId:   1001,
		SessionId:       session.SessionId,
		TradeNo:         session.TradeNo,
		BaseMoneyCents:  50000,
		RateBps:         1000,
		CommissionCents: 5000,
		Status:          AffiliateCommissionStatusAvailable,
		CreatedTime:     1782600020,
	}
	require.NoError(t, DB.Create(commission).Error)

	withdrawal := &AffiliateWithdrawal{
		WithdrawalId: "affw_model_test_1",
		UserId:       3001,
		AmountCents:  5000,
		Contact:      "支付宝：138****8888",
		Status:       AffiliateWithdrawalStatusPending,
		CreatedTime:  1782600030,
	}
	require.NoError(t, DB.Create(withdrawal).Error)

	var savedSession LdxpTopupSession
	require.NoError(t, DB.Where("session_id = ?", "ldxp_test_1").First(&savedSession).Error)
	assert.Equal(t, int64(50000), savedSession.SiteAmountCents)
	assert.Equal(t, int64(42500), savedSession.ExternalPaidCents)
	assert.Equal(t, MailCheckStatusPending, savedSession.MailStatus)

	var savedMailEvent LdxpMailEvent
	require.NoError(t, DB.Where("raw_hash = ?", mailEvent.RawHash).First(&savedMailEvent).Error)
	assert.Equal(t, int64(42500), savedMailEvent.PaidCents)
	assert.Equal(t, "parsed", savedMailEvent.ParseStatus)

	var savedCommission AffiliateCommission
	require.NoError(t, DB.Where("session_id = ?", session.SessionId).First(&savedCommission).Error)
	assert.Equal(t, int64(50000), savedCommission.BaseMoneyCents)
	assert.Equal(t, int64(5000), savedCommission.CommissionCents)
	assert.Equal(t, AffiliateCommissionStatusAvailable, savedCommission.Status)

	var savedWithdrawal AffiliateWithdrawal
	require.NoError(t, DB.Where("withdrawal_id = ?", withdrawal.WithdrawalId).First(&savedWithdrawal).Error)
	assert.Equal(t, int64(5000), savedWithdrawal.AmountCents)
	assert.Equal(t, AffiliateWithdrawalStatusPending, savedWithdrawal.Status)
}

func TestAffiliateWithdrawalPaidIsSingleTransition(t *testing.T) {
	truncateTables(t)

	withdrawal := &AffiliateWithdrawal{
		WithdrawalId: "affw_test_1",
		UserId:       2001,
		AmountCents:  5000,
		Contact:      "支付宝：138****8888",
		Status:       AffiliateWithdrawalStatusPending,
		CreatedTime:  common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(withdrawal).Error)

	updated, err := MarkAffiliateWithdrawalPaid(withdrawal.Id, 99, "  已通过支付宝打款  ")
	require.NoError(t, err)
	assert.Equal(t, AffiliateWithdrawalStatusPaid, updated.Status)
	assert.Equal(t, 99, updated.ProcessedBy)
	assert.NotZero(t, updated.ProcessedTime)
	assert.Equal(t, "已通过支付宝打款", updated.AdminRemark)

	var saved AffiliateWithdrawal
	require.NoError(t, DB.First(&saved, withdrawal.Id).Error)
	assert.Equal(t, AffiliateWithdrawalStatusPaid, saved.Status)
	assert.Equal(t, 99, saved.ProcessedBy)
	assert.NotZero(t, saved.ProcessedTime)
	assert.Equal(t, "已通过支付宝打款", saved.AdminRemark)

	_, err = MarkAffiliateWithdrawalPaid(withdrawal.Id, 99, "重复点击")
	require.ErrorIs(t, err, ErrAffiliateWithdrawalAlreadyProcessed)
}

func TestAffiliateWithdrawalPaidMissingReturnsNotFound(t *testing.T) {
	truncateTables(t)

	_, err := MarkAffiliateWithdrawalPaid(0, 99, "invalid id")
	require.ErrorIs(t, err, ErrAffiliateWithdrawalNotFound)

	_, err = MarkAffiliateWithdrawalPaid(999999, 99, "missing id")
	require.ErrorIs(t, err, ErrAffiliateWithdrawalNotFound)
}

func TestAffiliateWithdrawalRejectIsSingleTransition(t *testing.T) {
	truncateTables(t)

	withdrawal := &AffiliateWithdrawal{
		WithdrawalId: "affw_reject_test_1",
		UserId:       2002,
		AmountCents:  7800,
		Contact:      "微信：user@example.com",
		Status:       AffiliateWithdrawalStatusPending,
		CreatedTime:  common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(withdrawal).Error)

	updated, err := RejectAffiliateWithdrawal(withdrawal.Id, 100, "  信息不完整  ")
	require.NoError(t, err)
	assert.Equal(t, AffiliateWithdrawalStatusRejected, updated.Status)
	assert.Equal(t, 100, updated.ProcessedBy)
	assert.NotZero(t, updated.ProcessedTime)
	assert.Equal(t, "信息不完整", updated.AdminRemark)

	var saved AffiliateWithdrawal
	require.NoError(t, DB.First(&saved, withdrawal.Id).Error)
	assert.Equal(t, AffiliateWithdrawalStatusRejected, saved.Status)
	assert.Equal(t, 100, saved.ProcessedBy)
	assert.NotZero(t, saved.ProcessedTime)
	assert.Equal(t, "信息不完整", saved.AdminRemark)

	_, err = RejectAffiliateWithdrawal(withdrawal.Id, 100, "重复驳回")
	require.ErrorIs(t, err, ErrAffiliateWithdrawalAlreadyProcessed)
}
