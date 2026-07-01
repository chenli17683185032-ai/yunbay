package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrderManagementAnalyticsAggregatesProductionLdxpFields(t *testing.T) {
	truncateTables(t)

	records := []*LdxpTopupSession{
		{SessionId: "s1", UserId: 1, Money: 10.00, WorkerAmount: 10.30, WorkerOrderNo: "LD_OK", MailOrderNo: "LD_OK", MailAmount: 10.30, Status: LdxpStatusVerified, CreatedTime: 1782518400},
		{SessionId: "s2", UserId: 2, Money: 500.00, WorkerAmount: 425.00, WorkerOrderNo: "LD_BAD", MailOrderNo: "LD_BAD", MailAmount: 400.00, Status: LdxpStatusVerifyFailed, ErrorCode: "amount_mismatch", CreatedTime: 1782518500},
		{SessionId: "s3", UserId: 3, Money: 20.00, WorkerAmount: 20.60, WorkerOrderNo: "LD_WAIT", Status: LdxpStatusWorkerPaid, ErrorCode: "waiting_mail", CreatedTime: 1782604800},
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
	require.Len(t, result.Daily, 2)
	assert.Equal(t, "2026-06-27", result.Daily[0].Date)
	assert.Equal(t, int64(51000), result.Daily[0].SiteAmountCents)
	assert.Equal(t, 2, result.Daily[0].OrderCount)
}

func TestOrderManagementAffiliateStatsIncludesWithdrawalInfo(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&User{Id: 77, Username: "inviter", Status: common.UserStatusEnabled, AffCode: "aff77"}).Error)
	require.NoError(t, DB.Create(&AffiliateCommission{CommissionId: "affc_order_stats_1", InviterUserId: 77, InviteeUserId: 88, TradeNo: "trade1", BaseMoney: 425.00, Rate: 0.15, CommissionMoney: 63.75, Status: AffiliateCommissionStatusAvailable, CreatedTime: 1782518400}).Error)
	require.NoError(t, DB.Create(&AffiliateWithdrawal{WithdrawalId: "affw_pending", UserId: 77, Amount: 50.00, Contact: "支付宝：138****8888", Status: AffiliateWithdrawalStatusPending, CreatedTime: 1782600000}).Error)

	result, err := GetAffiliateStats(1782518400, 1782691199, AffiliateWithdrawalStatusPending, 0, 20)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Summary.AffiliateUserCount)
	assert.Equal(t, int64(6375), result.Summary.PeriodCommissionCents)
	assert.Equal(t, 1, result.Summary.PendingWithdrawalUserCount)
	assert.Equal(t, int64(5000), result.Summary.PendingWithdrawalCents)
	require.Len(t, result.Items, 1)
	require.NotNil(t, result.Items[0].Withdrawal)
	assert.Equal(t, "支付宝：138****8888", result.Items[0].Withdrawal.Contact)
	assert.Equal(t, int64(5000), result.Items[0].Withdrawal.AmountCents)
}

func TestOrderManagementOrdersIncludesUsernameAndAffiliateInfo(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&User{Id: 77, Username: "inviter", Status: common.UserStatusEnabled, AffCode: "aff77"}).Error)
	require.NoError(t, DB.Create(&User{Id: 88, Username: "buyer", Status: common.UserStatusEnabled, AffCode: "aff88"}).Error)
	session := &LdxpTopupSession{
		SessionId:     "ldxp_order_test_1",
		UserId:        88,
		TopupId:       7001,
		MailMessageId: "trade_order_1",
		Money:         500.00,
		WorkerAmount:  425.00,
		WorkerOrderNo: "LD260628UZJ97P",
		MailOrderNo:   "LD260628UZJ97P",
		MailAmount:    425.00,
		Status:        LdxpStatusVerified,
		CreatedTime:   1782604800,
		VerifiedTime:  1782604900,
	}
	require.NoError(t, DB.Create(session).Error)
	require.NoError(t, DB.Create(&AffiliateCommission{CommissionId: "affc_order_row_1", InviterUserId: 77, InviteeUserId: 88, TopupId: 7001, TradeNo: session.MailMessageId, BaseMoney: 425.00, Rate: 0.15, CommissionMoney: 63.75, Status: AffiliateCommissionStatusAvailable, CreatedTime: 1782604800}).Error)

	rows, total, err := ListOrderManagementOrders(1782518400, 1782691199, "", "LD260628", 0, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	assert.Equal(t, "buyer", rows[0].Username)
	assert.Equal(t, int64(50000), rows[0].SiteAmountCents)
	assert.Equal(t, int64(42500), rows[0].ExternalPaidCents)
	assert.Equal(t, MailCheckStatusVerified, rows[0].MailStatus)
	assert.Equal(t, 77, rows[0].AffiliateInviterId)
	assert.Equal(t, int64(6375), rows[0].AffiliateCommissionCents)
	assert.Equal(t, AffiliateCommissionStatusAvailable, rows[0].AffiliateStatus)
}

func TestAffiliateSourceOrdersIncludesInviteeUsernameAndSessionStatus(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&User{Id: 77, Username: "inviter", Status: common.UserStatusEnabled, AffCode: "aff77"}).Error)
	require.NoError(t, DB.Create(&User{Id: 88, Username: "buyer", Status: common.UserStatusEnabled, AffCode: "aff88"}).Error)
	session := &LdxpTopupSession{
		SessionId:     "ldxp_source_test_1",
		UserId:        88,
		TopupId:       7002,
		MailMessageId: "trade_source_1",
		Money:         500.00,
		WorkerAmount:  425.00,
		WorkerOrderNo: "LD260628SOURCE",
		MailOrderNo:   "LD260628SOURCE",
		MailAmount:    425.00,
		Status:        LdxpStatusVerified,
		CreatedTime:   1782604800,
	}
	require.NoError(t, DB.Create(session).Error)
	require.NoError(t, DB.Create(&AffiliateCommission{CommissionId: "affc_source_1", InviterUserId: 77, InviteeUserId: 88, TopupId: 7002, TradeNo: session.MailMessageId, BaseMoney: 425.00, Rate: 0.15, CommissionMoney: 63.75, Status: AffiliateCommissionStatusAvailable, CreatedTime: 1782604800}).Error)

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

func TestAffiliateStatsWithdrawalStatusFiltersItemsOnlyAndKeepsPendingSummary(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&User{Id: 77, Username: "available-only", Status: common.UserStatusEnabled, AffCode: "aff77"}).Error)
	require.NoError(t, DB.Create(&User{Id: 78, Username: "pending-user", Status: common.UserStatusEnabled, AffCode: "aff78"}).Error)
	require.NoError(t, DB.Create(&User{Id: 79, Username: "paid-user", Status: common.UserStatusEnabled, AffCode: "aff79"}).Error)
	require.NoError(t, DB.Create(&AffiliateCommission{CommissionId: "affc_filter_available", InviterUserId: 77, InviteeUserId: 880, TradeNo: "trade_available", BaseMoney: 100.00, Rate: 0.10, CommissionMoney: 10.00, Status: AffiliateCommissionStatusAvailable, CreatedTime: 1782518400}).Error)
	require.NoError(t, DB.Create(&AffiliateWithdrawal{WithdrawalId: "affw_pending_filter", UserId: 78, Amount: 50.00, Contact: "pending contact", Status: AffiliateWithdrawalStatusPending, CreatedTime: 1782600000}).Error)
	require.NoError(t, DB.Create(&AffiliateWithdrawal{WithdrawalId: "affw_paid_filter", UserId: 79, Amount: 70.00, Contact: "paid contact", Status: AffiliateWithdrawalStatusPaid, CreatedTime: 1782600001}).Error)

	pendingResult, err := GetAffiliateStats(1782518400, 1782691199, AffiliateWithdrawalStatusPending, 0, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), pendingResult.Total)
	require.Len(t, pendingResult.Items, 1)
	assert.Equal(t, 78, pendingResult.Items[0].UserId)
	require.NotNil(t, pendingResult.Items[0].Withdrawal)
	assert.Equal(t, AffiliateWithdrawalStatusPending, pendingResult.Items[0].Withdrawal.Status)
	assert.Equal(t, 1, pendingResult.Summary.PendingWithdrawalUserCount)
	assert.Equal(t, int64(5000), pendingResult.Summary.PendingWithdrawalCents)

	paidResult, err := GetAffiliateStats(1782518400, 1782691199, AffiliateWithdrawalStatusPaid, 0, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), paidResult.Total)
	require.Len(t, paidResult.Items, 1)
	assert.Equal(t, 79, paidResult.Items[0].UserId)
	require.NotNil(t, paidResult.Items[0].Withdrawal)
	assert.Equal(t, AffiliateWithdrawalStatusPaid, paidResult.Items[0].Withdrawal.Status)
	assert.Equal(t, 1, paidResult.Summary.PendingWithdrawalUserCount)
	assert.Equal(t, int64(5000), paidResult.Summary.PendingWithdrawalCents)
}

func TestAffiliateStatsExcludesCanceledCommissionsFromAmounts(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&User{Id: 77, Username: "inviter", Status: common.UserStatusEnabled, AffCode: "aff77"}).Error)
	require.NoError(t, DB.Create(&AffiliateCommission{CommissionId: "affc_amount_available", InviterUserId: 77, InviteeUserId: 88, TopupId: 7101, TradeNo: "trade_available", BaseMoney: 100.00, Rate: 0.10, CommissionMoney: 10.00, Status: AffiliateCommissionStatusAvailable, CreatedTime: 1782518400}).Error)
	require.NoError(t, DB.Create(&AffiliateCommission{CommissionId: "affc_amount_canceled", InviterUserId: 77, InviteeUserId: 89, TopupId: 7102, TradeNo: "trade_canceled", BaseMoney: 200.00, Rate: 0.10, CommissionMoney: 20.00, Status: AffiliateCommissionStatusCanceled, CreatedTime: 1782518500}).Error)

	stats, err := GetAffiliateStats(1782518400, 1782691199, "", 0, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1000), stats.Summary.PeriodCommissionCents)
	require.Len(t, stats.Items, 1)
	assert.Equal(t, int64(1000), stats.Items[0].PeriodCommissionCents)
	assert.Equal(t, int64(1000), stats.Items[0].TotalCommissionCents)

	analytics, err := GetOrderManagementAnalytics(1782518400, 1782691199)
	require.NoError(t, err)
	assert.Equal(t, int64(1000), analytics.Summary.AffiliateAmountCents)
}

func TestAffiliateWithdrawalPaidIsSingleTransition(t *testing.T) {
	truncateTables(t)

	withdrawal := &AffiliateWithdrawal{
		WithdrawalId: "affw_test_1",
		UserId:       2001,
		Amount:       50.00,
		Contact:      "支付宝：138****8888",
		Status:       AffiliateWithdrawalStatusPending,
		CreatedTime:  common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(withdrawal).Error)

	updated, err := MarkAffiliateWithdrawalPaid(withdrawal.Id, "  已通过支付宝打款  ")
	require.NoError(t, err)
	assert.Equal(t, AffiliateWithdrawalStatusPaid, updated.Status)
	assert.NotZero(t, updated.ProcessedTime)
	assert.Equal(t, "已通过支付宝打款", updated.AdminRemark)

	_, err = MarkAffiliateWithdrawalPaid(withdrawal.Id, "重复点击")
	require.ErrorIs(t, err, ErrAffiliateWithdrawalAlreadyProcessed)
}

func TestAffiliateWithdrawalPaidMissingReturnsNotFound(t *testing.T) {
	truncateTables(t)

	_, err := MarkAffiliateWithdrawalPaid(0, "invalid id")
	require.ErrorIs(t, err, ErrAffiliateWithdrawalNotFound)

	_, err = MarkAffiliateWithdrawalPaid(999999, "missing id")
	require.ErrorIs(t, err, ErrAffiliateWithdrawalNotFound)
}

func TestAffiliateWithdrawalRejectIsSingleTransition(t *testing.T) {
	truncateTables(t)

	withdrawal := &AffiliateWithdrawal{
		WithdrawalId: "affw_reject_test_1",
		UserId:       2002,
		Amount:       78.00,
		Contact:      "微信：user@example.com",
		Status:       AffiliateWithdrawalStatusPending,
		CreatedTime:  common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(withdrawal).Error)

	updated, err := RejectAffiliateWithdrawal(withdrawal.Id, "  信息不完整  ")
	require.NoError(t, err)
	assert.Equal(t, AffiliateWithdrawalStatusRejected, updated.Status)
	assert.NotZero(t, updated.ProcessedTime)
	assert.Equal(t, "信息不完整", updated.AdminRemark)

	_, err = RejectAffiliateWithdrawal(withdrawal.Id, "重复驳回")
	require.ErrorIs(t, err, ErrAffiliateWithdrawalAlreadyProcessed)
}
