package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestOrderManagementAnalyticsAggregatesProductionLdxpFields(t *testing.T) {
	truncateTables(t)

	records := []*LdxpTopupSession{
		{SessionId: "s1", UserId: 1, TopupId: 7001, Money: 10.00, WorkerAmount: 10.30, WorkerOrderNo: "LD_OK", MailOrderNo: "LD_OK", MailAmount: 10.30, Status: LdxpStatusSuccess, CreatedTime: 1782518400},
		{SessionId: "s2", UserId: 2, Money: 500.00, WorkerAmount: 425.00, WorkerOrderNo: "LD_BAD", MailOrderNo: "LD_BAD", MailAmount: 400.00, Status: LdxpStatusVerifyFailed, ErrorCode: "amount_mismatch", CreatedTime: 1782518500},
		{SessionId: "s3", UserId: 3, Money: 20.00, WorkerAmount: 20.60, WorkerOrderNo: "LD_WAIT", Status: LdxpStatusWorkerPaid, ErrorCode: "waiting_mail", CreatedTime: 1782604800},
		{SessionId: "s4", UserId: 4, Money: 1000.00, WorkerAmount: 1030.00, WorkerOrderNo: "LD_CANCEL", Status: LdxpStatusCanceled, CreatedTime: 1782604900},
	}
	for _, record := range records {
		require.NoError(t, DB.Create(record).Error)
	}

	result, err := GetOrderManagementAnalytics(1782518400, 1782691199)
	require.NoError(t, err)
	assert.Equal(t, int64(1000), result.Summary.SiteAmountCents)
	assert.Equal(t, int64(1030), result.Summary.ExternalPaidCents)
	assert.Equal(t, 1, result.Summary.OrderCount)
	assert.Equal(t, 1, result.Summary.MailVerifiedCount)
	assert.Equal(t, 0, result.Summary.MailPendingCount)
	assert.Equal(t, 0, result.Summary.MailErrorCount)
	assert.Equal(t, 1.0, result.Summary.MailVerifiedRate)
	require.Len(t, result.Daily, 1)
	assert.Equal(t, "2026-06-27", result.Daily[0].Date)
	assert.Equal(t, int64(1000), result.Daily[0].SiteAmountCents)
	assert.Equal(t, 1, result.Daily[0].OrderCount)
}

func TestOrderManagementOrdersExcludeUnsettledLdxpSessions(t *testing.T) {
	truncateTables(t)

	plan := &SubscriptionPlan{
		Title:        "月卡",
		PlanKind:     SubscriptionPlanKindValuePackage,
		PackageType:  ValuePackageTypeMonth,
		PackageLevel: ValuePackageLevelMonth,
		TotalAmount:  30000,
	}
	require.NoError(t, DB.Create(plan).Error)
	order := &SubscriptionOrder{
		UserId:          9,
		PlanId:          plan.Id,
		Money:           19.9,
		TradeNo:         "LDXP_VP-month-order",
		PaymentMethod:   PaymentMethodLDXP,
		PaymentProvider: PaymentProviderLDXP,
		Status:          common.TopUpStatusSuccess,
		CreateTime:      1782518300,
		CompleteTime:    1782518400,
	}
	require.NoError(t, DB.Create(order).Error)

	records := []*LdxpTopupSession{
		{SessionId: "settled_topup", UserId: 1, TopupId: 8001, Money: 10.00, WorkerAmount: 10.30, WorkerOrderNo: "LD_SETTLED_TOPUP", Status: LdxpStatusSuccess, CreatedTime: 1782518400},
		{SessionId: "settled_redemption", UserId: 2, RedemptionId: 9001, Money: 10.00, WorkerAmount: 10.00, WorkerOrderNo: "LD_SETTLED_REDEEM", Status: LdxpStatusSuccess, CreatedTime: 1782518500},
		{SessionId: "settled_value_package", UserId: 9, Purpose: LdxpPurposeValuePackage, SubscriptionOrderId: order.Id, SubscriptionPlanId: plan.Id, Money: 19.90, WorkerAmount: 19.90, WorkerOrderNo: "LD_VALUE_PACKAGE", Status: LdxpStatusSuccess, CreatedTime: 1782518550},
		{SessionId: "created", UserId: 3, Money: 1000.00, WorkerAmount: 0.00, WorkerOrderNo: "LD_CREATED", Status: LdxpStatusCreated, CreatedTime: 1782518600},
		{SessionId: "paid_waiting_mail", UserId: 4, Money: 50.00, WorkerAmount: 51.50, WorkerOrderNo: "LD_WAIT", Status: LdxpStatusWorkerPaid, ErrorCode: "waiting_mail", CreatedTime: 1782518700},
		{SessionId: "canceled", UserId: 5, Money: 500.00, WorkerAmount: 515.00, WorkerOrderNo: "LD_CANCELED", Status: LdxpStatusCanceled, CreatedTime: 1782518800},
		{SessionId: "expired", UserId: 6, Money: 30.00, WorkerAmount: 30.90, WorkerOrderNo: "LD_EXPIRED", Status: LdxpStatusExpired, CreatedTime: 1782518900},
		{SessionId: "worker_failed", UserId: 7, Money: 20.00, WorkerAmount: 0.00, WorkerOrderNo: "LD_FAILED", Status: LdxpStatusWorkerFailed, CreatedTime: 1782519000},
		{SessionId: "success_without_accounting", UserId: 8, Money: 999.00, WorkerAmount: 999.00, WorkerOrderNo: "LD_SUCCESS_NO_TOPUP", Status: LdxpStatusSuccess, CreatedTime: 1782519100},
	}
	for _, record := range records {
		require.NoError(t, DB.Create(record).Error)
	}

	result, err := GetOrderManagementAnalytics(1782518400, 1782691199)
	require.NoError(t, err)
	assert.Equal(t, int64(3990), result.Summary.SiteAmountCents)
	assert.Equal(t, int64(4020), result.Summary.ExternalPaidCents)
	assert.Equal(t, 3, result.Summary.OrderCount)

	rows, total, err := ListOrderManagementOrders(1782518400, 1782691199, "", "", 0, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	require.Len(t, rows, 3)
	assert.ElementsMatch(t, []string{"settled_topup", "settled_redemption", "settled_value_package"}, []string{rows[0].SessionId, rows[1].SessionId, rows[2].SessionId})

	valuePackageRow := findOrderManagementRowBySessionId(t, rows, "settled_value_package")
	assert.Equal(t, OrderTypeSubscription, valuePackageRow.BillingOrderType)
	assert.Equal(t, "LDXP_VP-month-order", valuePackageRow.TradeNo)
	assert.Equal(t, plan.Id, valuePackageRow.PlanId)
	assert.Equal(t, "月卡", valuePackageRow.PlanTitle)
}

func TestOrderManagementOrdersHonorDeletionMarksForValuePackageOrders(t *testing.T) {
	truncateTables(t)

	plan := &SubscriptionPlan{
		Title:        "周卡",
		PlanKind:     SubscriptionPlanKindValuePackage,
		PackageType:  ValuePackageTypeWeek,
		PackageLevel: ValuePackageLevelWeek,
		TotalAmount:  7000,
	}
	require.NoError(t, DB.Create(plan).Error)
	order := &SubscriptionOrder{
		UserId:          12,
		PlanId:          plan.Id,
		Money:           9.9,
		TradeNo:         "LDXP_VP-week-order",
		PaymentMethod:   PaymentMethodLDXP,
		PaymentProvider: PaymentProviderLDXP,
		Status:          common.TopUpStatusSuccess,
		CreateTime:      1782518300,
		CompleteTime:    1782518400,
	}
	require.NoError(t, DB.Create(order).Error)
	require.NoError(t, DB.Create(&LdxpTopupSession{
		SessionId:           "deleted_value_package",
		UserId:              12,
		Purpose:             LdxpPurposeValuePackage,
		SubscriptionOrderId: order.Id,
		SubscriptionPlanId:  plan.Id,
		Money:               9.90,
		WorkerAmount:        9.90,
		WorkerOrderNo:       "LD_VALUE_PACKAGE_DELETE",
		Status:              LdxpStatusSuccess,
		CreatedTime:         1782518550,
	}).Error)

	rows, total, err := ListOrderManagementOrders(1782518400, 1782691199, "", "", 0, 20)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, rows, 1)

	require.NoError(t, MarkAdminOrderDeleted(OrderTypeSubscription, order.TradeNo, 101, "test order"))

	rows, total, err = ListOrderManagementOrders(1782518400, 1782691199, "", "", 0, 20)
	require.NoError(t, err)
	assert.EqualValues(t, 0, total)
	assert.Empty(t, rows)
}

func TestOrderManagementAnalyticsHonorsDeletionMarksForValuePackageOrders(t *testing.T) {
	truncateTables(t)

	plan := &SubscriptionPlan{
		Title:        "日卡",
		PlanKind:     SubscriptionPlanKindValuePackage,
		PackageType:  ValuePackageTypeDay,
		PackageLevel: ValuePackageLevelDay,
		TotalAmount:  1000,
	}
	require.NoError(t, DB.Create(plan).Error)
	order := &SubscriptionOrder{
		UserId:          13,
		PlanId:          plan.Id,
		Money:           3.9,
		TradeNo:         "LDXP_VP-day-deleted-analytics",
		PaymentMethod:   PaymentMethodLDXP,
		PaymentProvider: PaymentProviderLDXP,
		Status:          common.TopUpStatusSuccess,
		CreateTime:      1782518300,
		CompleteTime:    1782518400,
	}
	require.NoError(t, DB.Create(order).Error)
	require.NoError(t, DB.Create(&LdxpTopupSession{
		SessionId:           "deleted_value_package_analytics",
		UserId:              13,
		Purpose:             LdxpPurposeValuePackage,
		SubscriptionOrderId: order.Id,
		SubscriptionPlanId:  plan.Id,
		Money:               3.90,
		WorkerAmount:        3.90,
		WorkerOrderNo:       "LD_VALUE_PACKAGE_DELETED_ANALYTICS",
		Status:              LdxpStatusSuccess,
		CreatedTime:         1782518550,
	}).Error)

	result, err := GetOrderManagementAnalytics(1782518400, 1782691199)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Summary.OrderCount)

	require.NoError(t, MarkAdminOrderDeleted(OrderTypeSubscription, order.TradeNo, 101, "test analytics"))

	result, err = GetOrderManagementAnalytics(1782518400, 1782691199)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Summary.OrderCount)
	assert.EqualValues(t, 0, result.Summary.SiteAmountCents)
}

func findOrderManagementRowBySessionId(t *testing.T, rows []OrderManagementOrderRow, sessionId string) OrderManagementOrderRow {
	t.Helper()
	for _, row := range rows {
		if row.SessionId == sessionId {
			return row
		}
	}
	t.Fatalf("row %q not found", sessionId)
	return OrderManagementOrderRow{}
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

func TestOrderManagementValuePackageOrderShowsAffiliateInfo(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&User{Id: 771, Username: "vp-inviter", Status: common.UserStatusEnabled, AffCode: "aff771"}).Error)
	require.NoError(t, DB.Create(&User{Id: 881, Username: "vp-buyer", Status: common.UserStatusEnabled, AffCode: "aff881", InviterId: 771}).Error)
	plan := &SubscriptionPlan{
		Title:        "畅享周卡",
		PlanKind:     SubscriptionPlanKindValuePackage,
		PackageType:  ValuePackageTypeWeek,
		PackageLevel: ValuePackageLevelWeek,
		TotalAmount:  0,
	}
	require.NoError(t, DB.Create(plan).Error)
	order := &SubscriptionOrder{UserId: 881, PlanId: plan.Id, Money: 28.8, TradeNo: "vp-order-affiliate-row", PaymentMethod: PaymentMethodLDXP, PaymentProvider: PaymentProviderLDXP, Status: common.TopUpStatusSuccess, CreateTime: 1782604700, CompleteTime: 1782604800}
	require.NoError(t, DB.Create(order).Error)
	topup := &TopUp{UserId: 881, Amount: 0, Money: 28.8, TradeNo: order.TradeNo, PaymentMethod: PaymentMethodLDXP, PaymentProvider: PaymentProviderLDXP, CreateTime: order.CreateTime, CompleteTime: order.CompleteTime, Status: common.TopUpStatusSuccess}
	require.NoError(t, DB.Create(topup).Error)
	require.NoError(t, DB.Create(&AffiliateCommission{CommissionId: "affc_vp_order_row", InviterUserId: 771, InviteeUserId: 881, TopupId: topup.Id, TradeNo: order.TradeNo, BaseMoney: 28.8, Rate: AffiliateCommissionRate, CommissionMoney: 4.32, Status: AffiliateCommissionStatusAvailable, CreatedTime: 1782604800}).Error)

	var reloadedOrder SubscriptionOrder
	require.NoError(t, DB.Where("trade_no = ?", order.TradeNo).First(&reloadedOrder).Error)
	session := &LdxpTopupSession{
		SessionId:           "vp_affiliate_row_session",
		UserId:              881,
		Purpose:             LdxpPurposeValuePackage,
		SubscriptionOrderId: reloadedOrder.Id,
		SubscriptionPlanId:  plan.Id,
		Money:               28.8,
		WorkerAmount:        28.8,
		WorkerOrderNo:       "LDVP-AFF",
		MailOrderNo:         "LDVP-AFF",
		MailAmount:          28.8,
		Status:              LdxpStatusSuccess,
		CreatedTime:         1782604800,
	}
	require.NoError(t, DB.Create(session).Error)

	rows, total, err := ListOrderManagementOrders(1782518400, 1782691199, "", "vp-order-affiliate-row", 0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	require.Equal(t, OrderTypeSubscription, rows[0].BillingOrderType)
	require.Equal(t, 771, rows[0].AffiliateInviterId)
	require.Equal(t, int64(432), rows[0].AffiliateCommissionCents)
	require.Equal(t, AffiliateCommissionStatusAvailable, rows[0].AffiliateStatus)
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

func TestListAdminOrdersIncludesTopupsAndSubscriptionOrders(t *testing.T) {
	truncateTables(t)

	plan := &SubscriptionPlan{
		Title:         "Pro Monthly",
		PriceAmount:   12.5,
		Currency:      "USD",
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
		CustomSeconds: 0,
	}
	require.NoError(t, DB.Create(plan).Error)

	topup := &TopUp{
		UserId:          11,
		Amount:          1000,
		Money:           10,
		TradeNo:         "topup-001",
		PaymentMethod:   PaymentMethodStripe,
		PaymentProvider: PaymentProviderStripe,
		CreateTime:      100,
		CompleteTime:    110,
		Status:          common.TopUpStatusSuccess,
	}
	require.NoError(t, DB.Create(topup).Error)

	subscription := &SubscriptionOrder{
		UserId:          22,
		PlanId:          plan.Id,
		Money:           12.5,
		TradeNo:         "sub-001",
		PaymentMethod:   PaymentMethodBalance,
		PaymentProvider: PaymentProviderBalance,
		Status:          common.TopUpStatusPending,
		CreateTime:      200,
		CompleteTime:    0,
	}
	require.NoError(t, DB.Create(subscription).Error)

	records, total, err := ListAdminOrders(&common.PageInfo{Page: 1, PageSize: 10}, "")
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	require.Len(t, records, 2)

	assert.Equal(t, OrderTypeSubscription, records[0].OrderType)
	assert.Equal(t, subscription.Id, records[0].Id)
	assert.Equal(t, "sub-001", records[0].TradeNo)
	assert.Equal(t, plan.Id, records[0].PlanId)
	assert.Equal(t, "Pro Monthly", records[0].PlanTitle)
	assert.Equal(t, SubscriptionDurationMonth, records[0].DurationUnit)
	assert.Equal(t, 1, records[0].DurationValue)
	assert.EqualValues(t, 0, records[0].CustomSeconds)

	assert.Equal(t, OrderTypeTopup, records[1].OrderType)
	assert.Equal(t, topup.Id, records[1].Id)
	assert.Equal(t, "topup-001", records[1].TradeNo)
	assert.EqualValues(t, 1000, records[1].Amount)
	assert.Equal(t, 10.0, records[1].Money)
}

func TestListAdminOrdersSearchesSubscriptionTradeNo(t *testing.T) {
	truncateTables(t)

	matching := &SubscriptionOrder{
		UserId:          31,
		Money:           20,
		TradeNo:         "sub-search-hit",
		PaymentMethod:   PaymentMethodBalance,
		PaymentProvider: PaymentProviderBalance,
		Status:          common.TopUpStatusSuccess,
		CreateTime:      300,
	}
	require.NoError(t, DB.Create(matching).Error)
	require.NoError(t, DB.Create(&SubscriptionOrder{
		UserId:          32,
		Money:           30,
		TradeNo:         "sub-other",
		PaymentMethod:   PaymentMethodBalance,
		PaymentProvider: PaymentProviderBalance,
		Status:          common.TopUpStatusSuccess,
		CreateTime:      400,
	}).Error)
	require.NoError(t, DB.Create(&TopUp{
		UserId:          33,
		Amount:          4000,
		Money:           40,
		TradeNo:         "topup-other",
		PaymentMethod:   PaymentMethodStripe,
		PaymentProvider: PaymentProviderStripe,
		CreateTime:      500,
		Status:          common.TopUpStatusSuccess,
	}).Error)

	records, total, err := ListAdminOrders(&common.PageInfo{Page: 1, PageSize: 10}, " search-hit ")
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, records, 1)
	assert.Equal(t, OrderTypeSubscription, records[0].OrderType)
	assert.Equal(t, matching.TradeNo, records[0].TradeNo)
}

func TestListAdminOrdersNilPageInfoUsesDefaultPagination(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&TopUp{
		UserId:          51,
		Amount:          1000,
		Money:           10,
		TradeNo:         "topup-nil-page",
		PaymentMethod:   PaymentMethodStripe,
		PaymentProvider: PaymentProviderStripe,
		CreateTime:      700,
		Status:          common.TopUpStatusSuccess,
	}).Error)

	records, total, err := ListAdminOrders(nil, "")
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, records, 1)
	assert.Equal(t, "topup-nil-page", records[0].TradeNo)
}

func TestListAdminOrdersPaginationBeyondTotalReturnsEmptySliceWithTotal(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&TopUp{
		UserId:          61,
		Amount:          2000,
		Money:           20,
		TradeNo:         "topup-page-one",
		PaymentMethod:   PaymentMethodStripe,
		PaymentProvider: PaymentProviderStripe,
		CreateTime:      800,
		Status:          common.TopUpStatusSuccess,
	}).Error)

	records, total, err := ListAdminOrders(&common.PageInfo{Page: 2, PageSize: 10}, "")
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	assert.NotNil(t, records)
	assert.Empty(t, records)
}

func TestMarkAdminOrderDeletedCanMarkNonexistentOrder(t *testing.T) {
	truncateTables(t)

	require.NoError(t, MarkAdminOrderDeleted(OrderTypeSubscription, "missing-sub-order", 303, "hide future scan"))

	var mark OrderDeletionMark
	require.NoError(t, DB.Where("order_type = ? AND trade_no = ?", OrderTypeSubscription, "missing-sub-order").First(&mark).Error)
	assert.Equal(t, 303, mark.DeletedBy)
	assert.Equal(t, "hide future scan", mark.Reason)
	assert.NotZero(t, mark.DeletedTime)
}

func TestMarkAdminOrderDeletedEmptyReasonDoesNotOverwriteExistingReason(t *testing.T) {
	truncateTables(t)

	require.NoError(t, MarkAdminOrderDeleted(OrderTypeTopup, "topup-keep-reason", 404, "keep this reason"))
	require.NoError(t, MarkAdminOrderDeleted(OrderTypeTopup, "topup-keep-reason", 505, "   "))

	var mark OrderDeletionMark
	require.NoError(t, DB.Where("order_type = ? AND trade_no = ?", OrderTypeTopup, "topup-keep-reason").First(&mark).Error)
	assert.Equal(t, 505, mark.DeletedBy)
	assert.Equal(t, "keep this reason", mark.Reason)
	assert.NotZero(t, mark.DeletedTime)
}

func TestMarkAdminOrderDeletedHidesOrderAndIsIdempotent(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&TopUp{
		UserId:          41,
		Amount:          5000,
		Money:           50,
		TradeNo:         "topup-hide-me",
		PaymentMethod:   PaymentMethodStripe,
		PaymentProvider: PaymentProviderStripe,
		CreateTime:      600,
		Status:          common.TopUpStatusSuccess,
	}).Error)

	records, total, err := ListAdminOrders(&common.PageInfo{Page: 1, PageSize: 10}, "")
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, records, 1)

	require.NoError(t, MarkAdminOrderDeleted(" topup ", " topup-hide-me ", 101, "first reason"))
	require.NoError(t, MarkAdminOrderDeleted(OrderTypeTopup, "topup-hide-me", 202, "second reason"))

	records, total, err = ListAdminOrders(&common.PageInfo{Page: 1, PageSize: 10}, "")
	require.NoError(t, err)
	assert.EqualValues(t, 0, total)
	assert.Empty(t, records)

	var marks []OrderDeletionMark
	require.NoError(t, DB.Find(&marks).Error)
	require.Len(t, marks, 1)
	assert.Equal(t, OrderTypeTopup, marks[0].OrderType)
	assert.Equal(t, "topup-hide-me", marks[0].TradeNo)
	assert.Equal(t, 202, marks[0].DeletedBy)
	assert.Equal(t, "second reason", marks[0].Reason)
	assert.NotZero(t, marks[0].DeletedTime)
}

func TestMarkAdminOrderDeletedTreatsConcurrentInsertAsIdempotent(t *testing.T) {
	truncateTables(t)

	const tradeNo = "topup-concurrent-insert"
	const callbackName = "test:inject_order_deletion_mark_concurrent_insert"
	injected := false
	require.NoError(t, DB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		mark, ok := tx.Statement.Dest.(*OrderDeletionMark)
		if !ok || injected || mark.OrderType != OrderTypeTopup || mark.TradeNo != tradeNo {
			return
		}
		injected = true
		err := tx.Session(&gorm.Session{NewDB: true}).Create(&OrderDeletionMark{
			OrderType:   OrderTypeTopup,
			TradeNo:     tradeNo,
			DeletedBy:   606,
			DeletedTime: 12345,
			Reason:      "injected reason",
		}).Error
		if err != nil {
			tx.AddError(err)
		}
	}))
	t.Cleanup(func() {
		_ = DB.Callback().Create().Remove(callbackName)
	})

	err := MarkAdminOrderDeleted(OrderTypeTopup, tradeNo, 707, "caller reason")
	require.NoError(t, err)
	assert.True(t, injected)

	var marks []OrderDeletionMark
	require.NoError(t, DB.Where("order_type = ? AND trade_no = ?", OrderTypeTopup, tradeNo).Find(&marks).Error)
	require.Len(t, marks, 1)
	assert.Equal(t, 707, marks[0].DeletedBy)
	assert.Equal(t, "caller reason", marks[0].Reason)
	assert.NotEqualValues(t, 12345, marks[0].DeletedTime)
}

func TestMarkAdminOrderDeletedValidatesOrderTypeAndTradeNo(t *testing.T) {
	truncateTables(t)

	assert.Error(t, MarkAdminOrderDeleted("unknown", "trade-no", 1, ""))
	assert.Error(t, MarkAdminOrderDeleted(OrderTypeTopup, "   ", 1, ""))
	assert.Error(t, MarkAdminOrderDeleted(OrderTypeSubscription, "", 1, ""))

	var count int64
	require.NoError(t, DB.Model(&OrderDeletionMark{}).Count(&count).Error)
	assert.EqualValues(t, 0, count)
}

func TestAdminOrderHelperSignatures(t *testing.T) {
	truncateTables(t)

	normalized := normalizeAdminOrderType(" topup ")
	assert.Equal(t, OrderTypeTopup, normalized)
	assert.Empty(t, normalizeAdminOrderType(" invalid "))

	require.NoError(t, DB.Create(&OrderDeletionMark{
		OrderType:   OrderTypeTopup,
		TradeNo:     "helper-signature-trade",
		DeletedBy:   606,
		DeletedTime: 12345,
	}).Error)
	marks, err := loadOrderDeletionMarkSet()
	require.NoError(t, err)
	var _ map[string]struct{} = marks
	_, ok := marks[orderDeletionKey(OrderTypeTopup, "helper-signature-trade")]
	assert.True(t, ok)

	plan := &SubscriptionPlan{
		Title:         "Helper Plan",
		PriceAmount:   9.9,
		Currency:      "USD",
		DurationUnit:  SubscriptionDurationDay,
		DurationValue: 7,
		CustomSeconds: 0,
	}
	require.NoError(t, DB.Create(plan).Error)
	records, err := attachSubscriptionPlanFields([]AdminOrderRecord{{
		OrderType: OrderTypeSubscription,
		PlanId:    plan.Id,
		TradeNo:   "helper-subscription",
	}})
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "Helper Plan", records[0].PlanTitle)
	assert.Equal(t, SubscriptionDurationDay, records[0].DurationUnit)
	assert.Equal(t, 7, records[0].DurationValue)
}
