package model

import (
	"errors"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestConsumeValuePackageResetCountClearsAllUsageAndAdvancesEpoch(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3801, UserGroupVIP)
	plan := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	plan.TotalAmount = 1000
	plan.Limit5hAmount = 500
	plan.Limit7dAmount = 900
	require.NoError(t, DB.Save(&plan).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, plan, now-2*3600, now+30*valuePackageDaySeconds)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Updates(map[string]interface{}{
		"amount_used": 300,
		"quota_epoch": int64(4),
	}).Error)
	require.NoError(t, DB.Create(&UserValuePackagePreference{
		UserId:                   user.Id,
		Enabled:                  true,
		ActiveUserSubscriptionId: sub.Id,
		ResetCount:               1,
	}).Error)
	for index, quota := range []int64{200, 100} {
		require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{
			UserId:             user.Id,
			UserSubscriptionId: sub.Id,
			PlanId:             plan.Id,
			PackageType:        plan.PackageType,
			ModelGroup:         plan.ModelGroup,
			RequestId:          "epoch-reset-history-" + string(rune('a'+index)),
			Quota:              quota,
			QuotaEpoch:         4,
			CreatedAt:          now - int64(index+1)*600,
		}))
	}

	plan.TotalAmount = 2000
	require.NoError(t, DB.Save(&plan).Error)
	state, err := ConsumeValuePackageResetCount(user.Id, sub.Id, now, user.Id, "reset all usage")

	require.NoError(t, err)
	require.NotNil(t, state)
	require.NotNil(t, state.Subscription)
	require.EqualValues(t, 1000, state.Subscription.AmountTotal)
	require.Zero(t, state.Subscription.AmountUsed)
	require.EqualValues(t, 5, state.Subscription.QuotaEpoch)
	require.NotNil(t, state.Usage)
	require.Zero(t, state.Usage.TotalUsed)
	require.Zero(t, state.Usage.Used5h)
	require.Zero(t, state.Usage.Used7d)

	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, sub.Id).Error)
	require.EqualValues(t, 1000, reloaded.AmountTotal)
	require.Zero(t, reloaded.AmountUsed)
	require.EqualValues(t, 5, reloaded.QuotaEpoch)

	var reset ValuePackageQuotaReset
	require.NoError(t, DB.Where("user_subscription_id = ?", sub.Id).First(&reset).Error)
	require.EqualValues(t, 4, reset.FromEpoch)
	require.EqualValues(t, 5, reset.ToEpoch)
	require.EqualValues(t, 300, reset.AmountUsedBefore)

	var history []ValuePackageUsageRecord
	require.NoError(t, DB.Where("user_subscription_id = ?", sub.Id).Order("id asc").Find(&history).Error)
	require.Len(t, history, 2)
	for _, record := range history {
		require.EqualValues(t, 4, record.QuotaEpoch)
		require.Positive(t, record.Quota)
	}
}

func TestValuePackageFixedCycleRenewalClearsUsedWithoutIncreasingTotal(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3807, UserGroupVIP)
	plan := createValuePackagePlan(t, ValuePackageTypeWeek, ValuePackageLevelWeek, 7, 9.9)
	plan.TotalAmount = 900
	require.NoError(t, DB.Save(&plan).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, plan, now-8*valuePackageDaySeconds, now+6*valuePackageDaySeconds)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Updates(map[string]interface{}{
		"amount_used":     800,
		"quota_epoch":     int64(3),
		"last_reset_time": now - 8*valuePackageDaySeconds,
		"next_reset_time": now - valuePackageDaySeconds,
	}).Error)

	result, err := PreConsumeValuePackageSubscription("epoch-cycle-renewal", user.Id, sub.Id, 100)
	require.NoError(t, err)
	require.EqualValues(t, 900, result.AmountTotal)
	require.EqualValues(t, 100, result.AmountUsedAfter)
	require.EqualValues(t, 4, result.QuotaEpoch)

	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, sub.Id).Error)
	require.EqualValues(t, 900, reloaded.AmountTotal)
	require.EqualValues(t, 100, reloaded.AmountUsed)
	require.EqualValues(t, 4, reloaded.QuotaEpoch)
	var reset ValuePackageQuotaReset
	require.NoError(t, DB.Where("user_subscription_id = ? AND source = ?", sub.Id, ValuePackageQuotaResetSourceCycleRenewal).First(&reset).Error)
	require.EqualValues(t, 800, reset.AmountUsedBefore)
}

func TestResetDueSubscriptionsAdvancesValuePackageEpochOnce(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3808, UserGroupVIP)
	plan := createValuePackagePlan(t, ValuePackageTypeWeek, ValuePackageLevelWeek, 7, 9.9)
	plan.TotalAmount = 900
	require.NoError(t, DB.Save(&plan).Error)
	now := GetDBTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, plan, now-8*valuePackageDaySeconds, now+6*valuePackageDaySeconds)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Updates(map[string]interface{}{
		"amount_used":     800,
		"quota_epoch":     int64(3),
		"last_reset_time": sub.StartTime,
		"next_reset_time": sub.StartTime + valuePackageWeekSeconds,
	}).Error)

	resetCount, err := ResetDueSubscriptions(10)
	require.NoError(t, err)
	require.Equal(t, 1, resetCount)
	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, sub.Id).Error)
	require.Zero(t, reloaded.AmountUsed)
	require.EqualValues(t, 4, reloaded.QuotaEpoch)
	require.Zero(t, reloaded.NextResetTime)
	var eventCount int64
	require.NoError(t, DB.Model(&ValuePackageQuotaReset{}).Where("user_subscription_id = ? AND source = ?", sub.Id, ValuePackageQuotaResetSourceCycleRenewal).Count(&eventCount).Error)
	require.EqualValues(t, 1, eventCount)

	resetCount, err = ResetDueSubscriptions(10)
	require.NoError(t, err)
	require.Zero(t, resetCount)
}

func TestConsumeValuePackageResetCountRollbackPreservesCountQuotaAndEpoch(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3802, UserGroupVIP)
	plan := createValuePackagePlan(t, ValuePackageTypeWeek, ValuePackageLevelWeek, 7, 9.9)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, plan, now-100, now+7*valuePackageDaySeconds)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Updates(map[string]interface{}{
		"amount_used": 300,
		"quota_epoch": int64(4),
	}).Error)
	require.NoError(t, DB.Create(&UserValuePackagePreference{UserId: user.Id, Enabled: true, ActiveUserSubscriptionId: sub.Id, ResetCount: 1}).Error)

	callbackName := "test:fail_value_package_epoch_update:" + strings.ReplaceAll(t.Name(), "/", "_")
	forcedErr := errors.New("forced subscription epoch update failure")
	require.NoError(t, DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "UserSubscription" {
			tx.AddError(forcedErr)
		}
	}))
	t.Cleanup(func() { require.NoError(t, DB.Callback().Update().Remove(callbackName)) })

	_, err := ConsumeValuePackageResetCount(user.Id, sub.Id, now, user.Id, "must roll back")
	require.ErrorIs(t, err, forcedErr)

	var pref UserValuePackagePreference
	require.NoError(t, DB.Where("user_id = ?", user.Id).First(&pref).Error)
	require.Equal(t, 1, pref.ResetCount)
	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, sub.Id).Error)
	require.EqualValues(t, 300, reloaded.AmountUsed)
	require.EqualValues(t, 4, reloaded.QuotaEpoch)
	var resetCount int64
	require.NoError(t, DB.Model(&ValuePackageQuotaReset{}).Where("user_subscription_id = ?", sub.Id).Count(&resetCount).Error)
	require.Zero(t, resetCount)
	var ledgerCount int64
	require.NoError(t, DB.Model(&ValuePackageResetCountLedger{}).Where("user_id = ?", user.Id).Count(&ledgerCount).Error)
	require.Zero(t, ledgerCount)
}

func TestLateValuePackageSettlementDoesNotChargeResetEpoch(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3803, UserGroupVIP)
	plan := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	plan.TotalAmount = 1000
	plan.Limit5hAmount = 1000
	plan.Limit7dAmount = 1000
	require.NoError(t, DB.Save(&plan).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, plan, now-100, now+30*valuePackageDaySeconds)
	require.NoError(t, DB.Create(&UserValuePackagePreference{UserId: user.Id, Enabled: true, ActiveUserSubscriptionId: sub.Id, ResetCount: 1}).Error)

	_, err := PreConsumeValuePackageSubscription("epoch-late-settle", user.Id, sub.Id, 100)
	require.NoError(t, err)
	_, err = ConsumeValuePackageResetCount(user.Id, sub.Id, now, user.Id, "advance epoch")
	require.NoError(t, err)
	result, err := ReserveValuePackageUsageToTarget("epoch-late-settle", user.Id, sub.Id, 150)

	require.NoError(t, err)
	require.EqualValues(t, 150, result.PreConsumed)
	require.Zero(t, result.AmountUsedBefore)
	require.Zero(t, result.AmountUsedAfter)
	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, sub.Id).Error)
	require.EqualValues(t, 1, reloaded.QuotaEpoch)
	require.Zero(t, reloaded.AmountUsed)
	var usage ValuePackageUsageRecord
	require.NoError(t, DB.Where("user_subscription_id = ? AND request_id = ?", sub.Id, "epoch-late-settle").First(&usage).Error)
	require.EqualValues(t, 0, usage.QuotaEpoch)
	require.EqualValues(t, 150, usage.Quota)
}

func TestLateValuePackageRefundDoesNotReduceResetEpoch(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3804, UserGroupVIP)
	plan := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	plan.TotalAmount = 1000
	plan.Limit5hAmount = 1000
	plan.Limit7dAmount = 1000
	require.NoError(t, DB.Save(&plan).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, plan, now-100, now+30*valuePackageDaySeconds)
	require.NoError(t, DB.Create(&UserValuePackagePreference{UserId: user.Id, Enabled: true, ActiveUserSubscriptionId: sub.Id, ResetCount: 1}).Error)

	_, err := PreConsumeValuePackageSubscription("epoch-late-refund", user.Id, sub.Id, 100)
	require.NoError(t, err)
	_, err = ConsumeValuePackageResetCount(user.Id, sub.Id, now, user.Id, "advance epoch")
	require.NoError(t, err)
	_, err = PreConsumeValuePackageSubscription("epoch-current-use", user.Id, sub.Id, 40)
	require.NoError(t, err)
	require.NoError(t, RefundSubscriptionPreConsume("epoch-late-refund"))

	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, sub.Id).Error)
	require.EqualValues(t, 1, reloaded.QuotaEpoch)
	require.EqualValues(t, 40, reloaded.AmountUsed)
	var oldUsage ValuePackageUsageRecord
	require.NoError(t, DB.Where("user_subscription_id = ? AND request_id = ?", sub.Id, "epoch-late-refund").First(&oldUsage).Error)
	require.EqualValues(t, 0, oldUsage.QuotaEpoch)
	require.Zero(t, oldUsage.Quota)
}

func TestCurrentEpochTargetReplacementUpdatesAmountUsed(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3805, UserGroupVIP)
	plan := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	plan.TotalAmount = 1000
	plan.Limit5hAmount = 1000
	plan.Limit7dAmount = 1000
	require.NoError(t, DB.Save(&plan).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, plan, now-100, now+30*valuePackageDaySeconds)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("quota_epoch", int64(3)).Error)

	_, err := PreConsumeValuePackageSubscription("epoch-current-target", user.Id, sub.Id, 10)
	require.NoError(t, err)
	_, err = ReserveValuePackageUsageToTarget("epoch-current-target", user.Id, sub.Id, 20)
	require.NoError(t, err)
	result, err := ReserveValuePackageUsageToTarget("epoch-current-target", user.Id, sub.Id, 5)

	require.NoError(t, err)
	require.EqualValues(t, 20, result.AmountUsedBefore)
	require.EqualValues(t, 5, result.AmountUsedAfter)
	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, sub.Id).Error)
	require.EqualValues(t, 5, reloaded.AmountUsed)
	var usage ValuePackageUsageRecord
	require.NoError(t, DB.Where("user_subscription_id = ? AND request_id = ?", sub.Id, "epoch-current-target").First(&usage).Error)
	require.EqualValues(t, 3, usage.QuotaEpoch)
	require.EqualValues(t, 5, usage.Quota)
}

func TestCurrentEpochRefundUsesFinalRequestTarget(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3807, UserGroupVIP)
	plan := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	plan.TotalAmount = 1000
	plan.Limit5hAmount = 1000
	plan.Limit7dAmount = 1000
	require.NoError(t, DB.Save(&plan).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, plan, now-100, now+30*valuePackageDaySeconds)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("quota_epoch", int64(2)).Error)

	first, err := PreConsumeValuePackageSubscription("epoch-current-refund", user.Id, sub.Id, 100)
	require.NoError(t, err)
	require.EqualValues(t, 2, first.QuotaEpoch)
	repeated, err := PreConsumeValuePackageSubscription("epoch-current-refund", user.Id, sub.Id, 100)
	require.NoError(t, err)
	require.EqualValues(t, 2, repeated.QuotaEpoch)
	_, err = ReserveValuePackageUsageToTarget("epoch-current-refund", user.Id, sub.Id, 150)
	require.NoError(t, err)
	require.NoError(t, RefundSubscriptionPreConsume("epoch-current-refund"))

	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, sub.Id).Error)
	require.Zero(t, reloaded.AmountUsed)
	var usage ValuePackageUsageRecord
	require.NoError(t, DB.Where("user_subscription_id = ? AND request_id = ?", sub.Id, "epoch-current-refund").First(&usage).Error)
	require.EqualValues(t, 2, usage.QuotaEpoch)
	require.Zero(t, usage.Quota)
}

func TestValuePackageUsageSummaryIgnoresPriorEpochRecords(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3806, UserGroupVIP)
	plan := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	plan.TotalAmount = 1000
	plan.Limit5hAmount = 1000
	plan.Limit7dAmount = 1000
	require.NoError(t, DB.Save(&plan).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, plan, now-100, now+30*valuePackageDaySeconds)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Updates(map[string]interface{}{
		"amount_used": 40,
		"quota_epoch": int64(1),
	}).Error)
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "epoch-prior-usage", Quota: 900, QuotaEpoch: 0, CreatedAt: now - 20}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "epoch-current-usage", Quota: 40, QuotaEpoch: 1, CreatedAt: now - 10}))

	details, err := GetValuePackageWindowUsageDetails(user.Id, sub.Id, now)
	require.NoError(t, err)
	require.EqualValues(t, 40, details.Used5h)
	require.EqualValues(t, 40, details.Used7d)
}
