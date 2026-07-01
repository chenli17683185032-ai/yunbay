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

	var saved LdxpTopupSession
	require.NoError(t, DB.Where("session_id = ?", "ldxp_test_1").First(&saved).Error)
	assert.Equal(t, int64(50000), saved.SiteAmountCents)
	assert.Equal(t, int64(42500), saved.ExternalPaidCents)
	assert.Equal(t, MailCheckStatusPending, saved.MailStatus)
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

	updated, err := MarkAffiliateWithdrawalPaid(withdrawal.Id, 99, "已通过支付宝打款")
	require.NoError(t, err)
	assert.Equal(t, AffiliateWithdrawalStatusPaid, updated.Status)
	assert.Equal(t, 99, updated.ProcessedBy)
	assert.NotZero(t, updated.ProcessedTime)

	_, err = MarkAffiliateWithdrawalPaid(withdrawal.Id, 99, "重复点击")
	require.ErrorIs(t, err, ErrAffiliateWithdrawalAlreadyProcessed)
}
