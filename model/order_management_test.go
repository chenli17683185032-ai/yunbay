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

func TestOrderManagementAnalyticsAggregatesByDayInGo(t *testing.T) {
	truncateTables(t)

	records := []*LdxpTopupSession{
		{SessionId: "s1", UserId: 1, SiteAmountCents: 1000, ExternalPaidCents: 1030, MailStatus: MailCheckStatusVerified, CreatedTime: 1782518400},
		{SessionId: "s2", UserId: 2, SiteAmountCents: 50000, ExternalPaidCents: 42500, MailStatus: MailCheckStatusAmountMismatch, CreatedTime: 1782518500},
		{SessionId: "s3", UserId: 3, SiteAmountCents: 2000, ExternalPaidCents: 2060, MailStatus: MailCheckStatusPending, CreatedTime: 1782604800},
	}
	for _, record := range records {
		require.NoError(t, DB.Create(record).Error)
	}

	result, err := GetOrderManagementAnalytics(1782518400, 1782691199)
	require.NoError(t, err)
	assert.Equal(t, int64(53000), result.Summary.SiteAmountCents)
	assert.Equal(t, int64(45590), result.Summary.ExternalPaidCents)
	assert.Equal(t, 3, result.Summary.OrderCount)
	assert.Equal(t, 1, result.Summary.MailVerifiedCount)
	assert.Equal(t, 1, result.Summary.MailPendingCount)
	assert.Equal(t, 1, result.Summary.MailErrorCount)
	assert.Equal(t, float64(1)/float64(3), result.Summary.MailVerifiedRate)
	assert.Len(t, result.Daily, 2)
	assert.Equal(t, "2026-06-27", result.Daily[0].Date)
	assert.Equal(t, int64(51000), result.Daily[0].SiteAmountCents)
	assert.Equal(t, 2, result.Daily[0].OrderCount)
}

func TestOrderManagementAffiliateStatsIncludesWithdrawalInfo(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&User{Id: 77, Username: "inviter", Status: common.UserStatusEnabled, AffCode: "aff77"}).Error)
	require.NoError(t, DB.Create(&AffiliateCommission{InviterUserId: 77, InviteeUserId: 88, TradeNo: "trade1", BaseMoneyCents: 42500, RateBps: 1500, CommissionCents: 6375, Status: AffiliateCommissionStatusAvailable, CreatedTime: 1782518400}).Error)
	require.NoError(t, DB.Create(&AffiliateWithdrawal{WithdrawalId: "affw_pending", UserId: 77, AmountCents: 5000, Contact: "支付宝：138****8888", Status: AffiliateWithdrawalStatusPending, CreatedTime: 1782600000}).Error)

	result, err := GetAffiliateStats(1782518400, 1782691199, "pending", 0, 20)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Summary.AffiliateUserCount)
	assert.Equal(t, int64(6375), result.Summary.PeriodCommissionCents)
	assert.Equal(t, 1, result.Summary.PendingWithdrawalUserCount)
	assert.Equal(t, int64(5000), result.Summary.PendingWithdrawalCents)
	assert.Len(t, result.Items, 1)
	assert.NotNil(t, result.Items[0].Withdrawal)
	assert.Equal(t, "支付宝：138****8888", result.Items[0].Withdrawal.Contact)
}

func TestOrderManagementOrdersIncludesUsernameAndAffiliateInfo(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&User{Id: 77, Username: "inviter", Status: common.UserStatusEnabled, AffCode: "aff77"}).Error)
	require.NoError(t, DB.Create(&User{Id: 88, Username: "buyer", Status: common.UserStatusEnabled, AffCode: "aff88"}).Error)
	session := &LdxpTopupSession{
		SessionId:         "ldxp_order_test_1",
		UserId:            88,
		TopUpId:           7001,
		TradeNo:           "trade_order_1",
		SiteAmountCents:   50000,
		ExternalPaidCents: 42500,
		WorkerOrderNo:     "LD260628UZJ97P",
		MailOrderNo:       "LD260628UZJ97P",
		MailAmountCents:   42500,
		MailStatus:        MailCheckStatusVerified,
		CreatedTime:       1782604800,
		VerifiedTime:      1782604900,
	}
	require.NoError(t, DB.Create(session).Error)
	require.NoError(t, DB.Create(&AffiliateCommission{InviterUserId: 77, InviteeUserId: 88, TopUpId: 7001, SessionId: session.SessionId, TradeNo: session.TradeNo, BaseMoneyCents: 42500, RateBps: 1500, CommissionCents: 6375, Status: AffiliateCommissionStatusAvailable, CreatedTime: 1782604800}).Error)

	rows, total, err := ListOrderManagementOrders(1782518400, 1782691199, "", "LD260628", 0, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	assert.Equal(t, "buyer", rows[0].Username)
	assert.Equal(t, 77, rows[0].AffiliateInviterId)
	assert.Equal(t, int64(6375), rows[0].AffiliateCommissionCents)
	assert.Equal(t, AffiliateCommissionStatusAvailable, rows[0].AffiliateStatus)
}

func TestAffiliateSourceOrdersIncludesInviteeUsernameAndSessionStatus(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&User{Id: 77, Username: "inviter", Status: common.UserStatusEnabled, AffCode: "aff77"}).Error)
	require.NoError(t, DB.Create(&User{Id: 88, Username: "buyer", Status: common.UserStatusEnabled, AffCode: "aff88"}).Error)
	session := &LdxpTopupSession{
		SessionId:         "ldxp_source_test_1",
		UserId:            88,
		TopUpId:           7002,
		TradeNo:           "trade_source_1",
		SiteAmountCents:   50000,
		ExternalPaidCents: 42500,
		WorkerOrderNo:     "LD260628SOURCE",
		MailStatus:        MailCheckStatusVerified,
		CreatedTime:       1782604800,
	}
	require.NoError(t, DB.Create(session).Error)
	require.NoError(t, DB.Create(&AffiliateCommission{InviterUserId: 77, InviteeUserId: 88, TopUpId: 7002, SessionId: session.SessionId, TradeNo: session.TradeNo, BaseMoneyCents: 42500, RateBps: 1500, CommissionCents: 6375, Status: AffiliateCommissionStatusAvailable, CreatedTime: 1782604800}).Error)

	rows, err := GetAffiliateSourceOrders(77, 1782518400, 1782691199, 20)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(1782604800), rows[0].OrderTime)
	assert.Equal(t, 88, rows[0].InviteeUserId)
	assert.Equal(t, "buyer", rows[0].InviteeUsername)
	assert.Equal(t, "trade_source_1", rows[0].TradeNo)
	assert.Equal(t, "LD260628SOURCE", rows[0].WorkerOrderNo)
	assert.Equal(t, int64(42500), rows[0].BaseMoneyCents)
	assert.Equal(t, 1500, rows[0].RateBps)
	assert.Equal(t, int64(6375), rows[0].CommissionCents)
	assert.Equal(t, MailCheckStatusVerified, rows[0].MailStatus)
}
