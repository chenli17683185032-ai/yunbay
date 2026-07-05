package model

import (
	"testing"

	"gorm.io/gorm"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedRedemptionUser(t *testing.T, userId int, quota int) {
	t.Helper()
	require.NoError(t, DB.Create(&User{Id: userId, Username: "redeem_user", Quota: quota, Status: common.UserStatusEnabled}).Error)
}

func TestRedeemLegacyEmptyTypeActsAsQuotaCode(t *testing.T) {
	truncateTables(t)
	seedRedemptionUser(t, 21, 100)
	code := Redemption{UserId: 1, Key: "LEGACYQUOTACODE", Name: "legacy", Status: common.RedemptionCodeStatusEnabled, Quota: 500, CreatedTime: common.GetTimestamp()}
	require.NoError(t, DB.Create(&code).Error)
	require.NoError(t, DB.Model(&Redemption{}).Where("id = ?", code.Id).Update("type", "").Error)

	var legacy Redemption
	require.NoError(t, DB.Where("id = ?", code.Id).First(&legacy).Error)
	require.Equal(t, "", legacy.Type)

	result, err := Redeem("LEGACYQUOTACODE", 21)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, RedemptionTypeQuota, result.Type)
	assert.Equal(t, 500, result.Quota)

	var user User
	require.NoError(t, DB.Where("id = ?", 21).First(&user).Error)
	assert.Equal(t, 600, user.Quota)
}

func TestRedeemSubscriptionCodeCreatesUserSubscription(t *testing.T) {
	truncateTables(t)
	seedRedemptionUser(t, 22, 100)
	plan := SubscriptionPlan{Title: "日卡", DurationUnit: SubscriptionDurationDay, DurationValue: 1, Enabled: true, TotalAmount: 1000}
	require.NoError(t, DB.Create(&plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)
	code := Redemption{UserId: 1, Key: "SUBSCRIPTIONCODE", Name: "日卡码", Type: RedemptionTypeSubscription, PlanId: plan.Id, Status: common.RedemptionCodeStatusEnabled, CreatedTime: common.GetTimestamp()}
	require.NoError(t, DB.Create(&code).Error)

	result, err := Redeem("SUBSCRIPTIONCODE", 22)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, RedemptionTypeSubscription, result.Type)
	assert.Equal(t, plan.Id, result.PlanId)
	assert.Equal(t, "日卡", result.PlanTitle)
	assert.Equal(t, 0, result.Quota)

	var user User
	require.NoError(t, DB.Where("id = ?", 22).First(&user).Error)
	assert.Equal(t, 100, user.Quota)

	var subs []UserSubscription
	require.NoError(t, DB.Where("user_id = ? AND plan_id = ? AND source = ?", 22, plan.Id, "redemption").Find(&subs).Error)
	require.Len(t, subs, 1)
	assert.Equal(t, int64(1000), subs[0].AmountTotal)

	var saved Redemption
	require.NoError(t, DB.Where("id = ?", code.Id).First(&saved).Error)
	assert.Equal(t, common.RedemptionCodeStatusUsed, saved.Status)
	assert.Equal(t, 22, saved.UsedUserId)
}

func TestRedeemSubscriptionCodeCreatesValuePackageSubscription(t *testing.T) {
	truncateTables(t)
	seedRedemptionUser(t, 26, 100)
	plan := SubscriptionPlan{
		Title:         "月卡",
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		TotalAmount:   30000,
		PlanKind:      SubscriptionPlanKindValuePackage,
		PackageType:   ValuePackageTypeMonth,
		PackageLevel:  ValuePackageLevelMonth,
		ModelGroup:    "plus",
	}
	require.NoError(t, DB.Create(&plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)
	code := Redemption{UserId: 1, Key: "VALUEPACKAGECODE", Name: "月卡码", Type: RedemptionTypeSubscription, PlanId: plan.Id, Status: common.RedemptionCodeStatusEnabled, CreatedTime: common.GetTimestamp()}
	require.NoError(t, DB.Create(&code).Error)

	result, err := Redeem("VALUEPACKAGECODE", 26)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, RedemptionTypeSubscription, result.Type)
	assert.Equal(t, plan.Id, result.PlanId)
	assert.Equal(t, "月卡", result.PlanTitle)

	var subs []UserSubscription
	require.NoError(t, DB.Where("user_id = ? AND plan_id = ? AND source = ?", 26, plan.Id, "redemption").Find(&subs).Error)
	require.Len(t, subs, 1)
	assert.Equal(t, UserSubscriptionStatusActive, subs[0].Status)
	assert.Equal(t, int64(30000), subs[0].AmountTotal)

	var pref UserValuePackagePreference
	require.NoError(t, DB.Where("user_id = ?", 26).First(&pref).Error)
	assert.False(t, pref.Enabled)
	assert.Equal(t, subs[0].Id, pref.ActiveUserSubscriptionId)

	var saved Redemption
	require.NoError(t, DB.Where("id = ?", code.Id).First(&saved).Error)
	assert.Equal(t, common.RedemptionCodeStatusUsed, saved.Status)
	assert.Equal(t, 26, saved.UsedUserId)
}

func TestRedeemSubscriptionCodeDoesNotConsumeCodeWhenPlanMissing(t *testing.T) {
	truncateTables(t)
	seedRedemptionUser(t, 23, 100)
	code := Redemption{UserId: 1, Key: "MISSINGPLANCODE", Name: "坏码", Type: RedemptionTypeSubscription, PlanId: 9999, Status: common.RedemptionCodeStatusEnabled, CreatedTime: common.GetTimestamp()}
	require.NoError(t, DB.Create(&code).Error)

	result, err := Redeem("MISSINGPLANCODE", 23)
	assert.Error(t, err)
	assert.Nil(t, result)

	var saved Redemption
	require.NoError(t, DB.Where("id = ?", code.Id).First(&saved).Error)
	assert.Equal(t, common.RedemptionCodeStatusEnabled, saved.Status)
	assert.Equal(t, 0, saved.UsedUserId)
}

func TestRedeemDoesNotApplySideEffectsWhenCodeClaimedConcurrently(t *testing.T) {
	truncateTables(t)
	seedRedemptionUser(t, 24, 100)
	code := Redemption{UserId: 1, Key: "CASQUOTACODE", Name: "cas", Type: RedemptionTypeQuota, Status: common.RedemptionCodeStatusEnabled, Quota: 500, CreatedTime: common.GetTimestamp()}
	require.NoError(t, DB.Create(&code).Error)

	callbackName := "test:claim_redemption_before_update"
	claimed := false
	require.NoError(t, DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if claimed || tx.Statement == nil || tx.Statement.Schema == nil || tx.Statement.Schema.Name != "Redemption" {
			return
		}
		claimed = true
		if err := tx.Session(&gorm.Session{NewDB: true}).Model(&Redemption{}).
			Where("id = ?", code.Id).
			Updates(map[string]interface{}{
				"status":       common.RedemptionCodeStatusUsed,
				"used_user_id": 999,
			}).Error; err != nil {
			tx.AddError(err)
		}
	}))
	t.Cleanup(func() {
		_ = DB.Callback().Update().Remove(callbackName)
	})

	result, err := Redeem("CASQUOTACODE", 24)
	assert.Error(t, err)
	assert.Nil(t, result)

	var user User
	require.NoError(t, DB.Where("id = ?", 24).First(&user).Error)
	assert.Equal(t, 100, user.Quota)

	var saved Redemption
	require.NoError(t, DB.Where("id = ?", code.Id).First(&saved).Error)
	assert.Equal(t, common.RedemptionCodeStatusEnabled, saved.Status)
	assert.Equal(t, 0, saved.UsedUserId)
}

func TestRedeemSubscriptionCodeUsesFreshPlanLookup(t *testing.T) {
	truncateTables(t)
	seedRedemptionUser(t, 25, 100)
	plan := SubscriptionPlan{Title: "缓存套餐", DurationUnit: SubscriptionDurationDay, DurationValue: 1, Enabled: true, TotalAmount: 1000}
	require.NoError(t, DB.Create(&plan).Error)
	_, err := GetSubscriptionPlanById(plan.Id)
	require.NoError(t, err)
	t.Cleanup(func() { InvalidateSubscriptionPlanCache(plan.Id) })
	require.NoError(t, DB.Delete(&SubscriptionPlan{}, plan.Id).Error)

	code := Redemption{UserId: 1, Key: "STALEPLANCODE", Name: " stale plan ", Type: RedemptionTypeSubscription, PlanId: plan.Id, Status: common.RedemptionCodeStatusEnabled, CreatedTime: common.GetTimestamp()}
	require.NoError(t, DB.Create(&code).Error)

	result, err := Redeem("STALEPLANCODE", 25)
	assert.Error(t, err)
	assert.Nil(t, result)

	var saved Redemption
	require.NoError(t, DB.Where("id = ?", code.Id).First(&saved).Error)
	assert.Equal(t, common.RedemptionCodeStatusEnabled, saved.Status)
	assert.Equal(t, 0, saved.UsedUserId)

	var subs []UserSubscription
	require.NoError(t, DB.Where("user_id = ? AND plan_id = ?", 25, plan.Id).Find(&subs).Error)
	assert.Empty(t, subs)
}

func TestRedeemValidateRedemptionForCreateUsesFreshPlanLookup(t *testing.T) {
	truncateTables(t)
	plan := SubscriptionPlan{Title: "缓存校验套餐", DurationUnit: SubscriptionDurationDay, DurationValue: 1, Enabled: true, TotalAmount: 1000}
	require.NoError(t, DB.Create(&plan).Error)
	_, err := GetSubscriptionPlanById(plan.Id)
	require.NoError(t, err)
	t.Cleanup(func() { InvalidateSubscriptionPlanCache(plan.Id) })
	require.NoError(t, DB.Delete(&SubscriptionPlan{}, plan.Id).Error)

	err = ValidateRedemptionForCreate(&Redemption{Type: RedemptionTypeSubscription, PlanId: plan.Id})
	assert.Error(t, err)
}
