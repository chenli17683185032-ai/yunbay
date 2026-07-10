package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func makeMidjourneyRefundTask(userID, channelID, quota int, billingContext model.MidjourneyBillingContext) *model.Midjourney {
	return &model.Midjourney{
		UserId:         userID,
		ChannelId:      channelID,
		MjId:           "mj-refund",
		Action:         "IMAGINE",
		Quota:          quota,
		BillingContext: billingContext,
	}
}

func setupMidjourneyRefundTest(t *testing.T) {
	t.Helper()
	truncate(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Midjourney{}))
	t.Cleanup(func() { model.DB.Exec("DELETE FROM midjourneys") })
}

func persistMidjourneyRefundTask(t *testing.T, task *model.Midjourney) {
	t.Helper()
	require.NoError(t, task.Insert())
}

func seedMidjourneyPreConsumeRecord(t *testing.T, requestID string, userID, subscriptionID int, quota int64) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.SubscriptionPreConsumeRecord{
		RequestId: requestID, UserId: userID, UserSubscriptionId: subscriptionID,
		PreConsumed: quota, Status: "consumed",
	}).Error)
}

func TestRefundMidjourneyQuotaWalletRestoresFundingAndToken(t *testing.T) {
	setupMidjourneyRefundTest(t)
	const userID, tokenID, channelID, quota = 41, 42, 43, 500
	seedUser(t, userID, 100)
	seedToken(t, tokenID, userID, "mj-wallet-token", 200)
	seedChannel(t, channelID)
	task := makeMidjourneyRefundTask(userID, channelID, quota, model.MidjourneyBillingContext{
		BillingSource: BillingSourceWallet, TokenId: tokenID, BillingUsingGroup: "group-a",
		EffectiveGroupRatio: 0.5,
	})
	persistMidjourneyRefundTask(t, task)

	require.NoError(t, RefundMidjourneyQuota(context.Background(), task, "terminal failure"))

	require.Equal(t, 600, getUserQuota(t, userID))
	require.Equal(t, 700, getTokenRemainQuota(t, tokenID))
	log := getLastLog(t)
	require.NotNil(t, log)
	require.Equal(t, quota, log.Quota)
	require.Equal(t, "group-a", log.Group)
}

func TestRefundMidjourneyQuotaRegularSubscriptionUsesOriginalFunding(t *testing.T) {
	setupMidjourneyRefundTest(t)
	const userID, tokenID, channelID, subscriptionID, quota = 51, 52, 53, 54, 600
	seedUser(t, userID, 0)
	seedToken(t, tokenID, userID, "mj-sub-token", 300)
	seedChannel(t, channelID)
	seedSubscription(t, subscriptionID, userID, 10000, quota)
	seedMidjourneyPreConsumeRecord(t, "mj-regular-request", userID, subscriptionID, quota)
	task := makeMidjourneyRefundTask(userID, channelID, quota, model.MidjourneyBillingContext{
		BillingSource: BillingSourceSubscription, SubscriptionId: subscriptionID,
		RequestId: "mj-regular-request", TokenId: tokenID, BillingUsingGroup: "group-a",
		EffectiveGroupRatio: 1, SubscriptionRatioSource: SubscriptionRatioSourceRegular,
	})
	persistMidjourneyRefundTask(t, task)

	require.NoError(t, RefundMidjourneyQuota(context.Background(), task, "terminal failure"))

	require.Zero(t, getUserQuota(t, userID))
	require.Zero(t, getSubscriptionUsed(t, subscriptionID))
	require.Equal(t, 900, getTokenRemainQuota(t, tokenID))
}

func TestRefundMidjourneyQuotaValuePackageRevokesUsage(t *testing.T) {
	for _, tt := range []struct {
		name   string
		ratio  float64
		source string
	}{
		{name: "configured", ratio: 0.45, source: SubscriptionRatioSourceConfigured},
		{name: "default one x", ratio: 1, source: SubscriptionRatioSourceDefault},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setupMidjourneyRefundTest(t)
			const userID, tokenID, channelID, subscriptionID, quota = 61, 62, 63, 64, 700
			requestID := "mj-value-package-" + tt.name
			seedUser(t, userID, 0)
			seedToken(t, tokenID, userID, "mj-vp-token", 400)
			seedChannel(t, channelID)
			seedSubscription(t, subscriptionID, userID, 10000, quota)
			seedMidjourneyPreConsumeRecord(t, requestID, userID, subscriptionID, quota)
			require.NoError(t, model.DB.Create(&model.ValuePackageUsageRecord{
				UserId: userID, UserSubscriptionId: subscriptionID, PlanId: 65,
				PackageType: model.ValuePackageTypeMonth, ModelGroup: "month-card",
				RequestId: requestID, Quota: quota,
			}).Error)
			task := makeMidjourneyRefundTask(userID, channelID, quota, model.MidjourneyBillingContext{
				BillingSource: BillingSourceSubscription, SubscriptionId: subscriptionID,
				RequestId: requestID, TokenId: tokenID,
				ValuePackageSubscriptionId: subscriptionID, ValuePackagePlanId: 65,
				ValuePackageBillingGroup: "month-card", BillingUsingGroup: "group-a",
				EffectiveGroupRatio: tt.ratio, SubscriptionRatioSource: tt.source,
			})
			persistMidjourneyRefundTask(t, task)

			require.NoError(t, RefundMidjourneyQuota(context.Background(), task, "terminal failure"))

			require.Zero(t, getUserQuota(t, userID))
			require.Zero(t, getSubscriptionUsed(t, subscriptionID))
			require.Equal(t, 1100, getTokenRemainQuota(t, tokenID))
			var usage model.ValuePackageUsageRecord
			require.NoError(t, model.DB.Where("user_subscription_id = ? AND request_id = ?", subscriptionID, requestID).First(&usage).Error)
			require.Zero(t, usage.Quota)
			log := getLastLog(t)
			require.NotNil(t, log)
			var other map[string]any
			require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
			require.Equal(t, "month-card", other["value_package_billing_group"])
			require.Equal(t, "group-a", other["value_package_billing_using_group"])
			require.Equal(t, tt.ratio, other["value_package_effective_ratio"])
			require.Equal(t, tt.source, other["value_package_ratio_source"])
		})
	}
}

func TestRefundMidjourneyQuotaLegacyRecordFallsBackToWallet(t *testing.T) {
	setupMidjourneyRefundTest(t)
	const userID, channelID, quota = 71, 72, 800
	seedUser(t, userID, 100)
	seedChannel(t, channelID)
	task := makeMidjourneyRefundTask(userID, channelID, quota, model.MidjourneyBillingContext{})
	task.MjId = fmt.Sprintf("legacy-%d", userID)
	persistMidjourneyRefundTask(t, task)

	require.NoError(t, RefundMidjourneyQuota(context.Background(), task, "terminal failure"))

	require.Equal(t, 900, getUserQuota(t, userID))
}

func TestRefundMidjourneyQuotaRetriesTokenLegWithoutRepeatingWalletRefund(t *testing.T) {
	setupMidjourneyRefundTest(t)
	const userID, tokenID, channelID, quota = 91, 92, 93, 500
	seedUser(t, userID, 100)
	seedToken(t, tokenID, userID, "mj-retry-token", 200)
	seedChannel(t, channelID)
	task := makeMidjourneyRefundTask(userID, channelID, quota, model.MidjourneyBillingContext{
		BillingSource: BillingSourceWallet, TokenId: tokenID, BillingUsingGroup: "group-a",
	})
	persistMidjourneyRefundTask(t, task)
	callbackName := "test:fail_first_mj_token_refund"
	failed := false
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if !failed && tx.Statement.Schema != nil && tx.Statement.Schema.Name == "Token" {
			failed = true
			tx.AddError(fmt.Errorf("forced token refund failure"))
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Update().Remove(callbackName) })

	require.Error(t, RefundMidjourneyQuota(context.Background(), task, "first attempt"))
	require.Equal(t, 600, getUserQuota(t, userID))
	require.Equal(t, 200, getTokenRemainQuota(t, tokenID))
	var afterFirst model.Midjourney
	require.NoError(t, model.DB.First(&afterFirst, task.Id).Error)
	require.True(t, afterFirst.BillingContext.FundingRefunded)
	require.False(t, afterFirst.BillingContext.TokenRefunded)
	require.False(t, afterFirst.BillingContext.BillingRefunded)
	require.NoError(t, RefundMidjourneyQuota(context.Background(), &afterFirst, "retry"))
	require.Equal(t, 600, getUserQuota(t, userID))
	require.Equal(t, 700, getTokenRemainQuota(t, tokenID))
	var completed model.Midjourney
	require.NoError(t, model.DB.First(&completed, task.Id).Error)
	require.True(t, completed.BillingContext.FundingRefunded)
	require.True(t, completed.BillingContext.TokenRefunded)
	require.True(t, completed.BillingContext.BillingRefunded)
	require.NoError(t, RefundMidjourneyQuota(context.Background(), &completed, "duplicate"))
	require.Equal(t, 600, getUserQuota(t, userID))
	require.Equal(t, 700, getTokenRemainQuota(t, tokenID))
	var refundLogs int64
	require.NoError(t, model.DB.Model(&model.Log{}).Where("type = ?", model.LogTypeRefund).Count(&refundLogs).Error)
	require.Equal(t, int64(1), refundLogs)
}

func TestRefundMidjourneyQuotaRollsBackWalletWhenFundingStatePersistenceFails(t *testing.T) {
	setupMidjourneyRefundTest(t)
	const userID, channelID, quota = 101, 102, 500
	seedUser(t, userID, 100)
	seedChannel(t, channelID)
	task := makeMidjourneyRefundTask(userID, channelID, quota, model.MidjourneyBillingContext{
		BillingSource: BillingSourceWallet, BillingUsingGroup: "group-a",
	})
	persistMidjourneyRefundTask(t, task)
	callbackName := "test:fail_mj_funding_state_update"
	failed := false
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if !failed && tx.Statement.Schema != nil && tx.Statement.Schema.Name == "Midjourney" {
			failed = true
			tx.AddError(fmt.Errorf("forced billing context failure"))
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Update().Remove(callbackName) })

	require.Error(t, RefundMidjourneyQuota(context.Background(), task, "first attempt"))
	require.Equal(t, 100, getUserQuota(t, userID))
	var retryTask model.Midjourney
	require.NoError(t, model.DB.First(&retryTask, task.Id).Error)
	require.NoError(t, RefundMidjourneyQuota(context.Background(), &retryTask, "retry"))
	require.Equal(t, 600, getUserQuota(t, userID))
}

func TestRefundMidjourneyQuotaRetriesIdempotentSubscriptionAfterStatePersistenceFails(t *testing.T) {
	setupMidjourneyRefundTest(t)
	const userID, channelID, subscriptionID, quota = 111, 112, 113, 500
	seedUser(t, userID, 0)
	seedChannel(t, channelID)
	seedSubscription(t, subscriptionID, userID, 10000, quota)
	seedMidjourneyPreConsumeRecord(t, "mj-idempotent-sub", userID, subscriptionID, quota)
	task := makeMidjourneyRefundTask(userID, channelID, quota, model.MidjourneyBillingContext{
		BillingSource: BillingSourceSubscription, SubscriptionId: subscriptionID,
		RequestId: "mj-idempotent-sub", BillingUsingGroup: "group-a",
	})
	persistMidjourneyRefundTask(t, task)
	callbackName := "test:fail_mj_subscription_state_update"
	failed := false
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if !failed && tx.Statement.Schema != nil && tx.Statement.Schema.Name == "Midjourney" {
			failed = true
			tx.AddError(fmt.Errorf("forced subscription context failure"))
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Update().Remove(callbackName) })

	require.Error(t, RefundMidjourneyQuota(context.Background(), task, "first attempt"))
	require.Zero(t, getSubscriptionUsed(t, subscriptionID))
	var retryTask model.Midjourney
	require.NoError(t, model.DB.First(&retryTask, task.Id).Error)
	require.NoError(t, RefundMidjourneyQuota(context.Background(), &retryTask, "retry"))
	require.Zero(t, getSubscriptionUsed(t, subscriptionID))
	var completed model.Midjourney
	require.NoError(t, model.DB.First(&completed, task.Id).Error)
	require.True(t, completed.BillingContext.FundingRefunded)
	require.True(t, completed.BillingContext.BillingRefunded)
}
