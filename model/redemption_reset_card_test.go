package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedResetCardValuePackagePlan(t *testing.T, giftResetCount int) *SubscriptionPlan {
	t.Helper()
	plan := SubscriptionPlan{
		Title:          "日卡",
		DurationUnit:   SubscriptionDurationDay,
		DurationValue:  1,
		Enabled:        true,
		TotalAmount:    10000,
		PlanKind:       SubscriptionPlanKindValuePackage,
		PackageType:    ValuePackageTypeDay,
		PackageLevel:   ValuePackageLevelDay,
		ModelGroup:     "plus",
		GiftResetCount: giftResetCount,
	}
	require.NoError(t, DB.Create(&plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)
	t.Cleanup(func() { InvalidateSubscriptionPlanCache(plan.Id) })
	return &plan
}

func resetCountLedgersForTest(t *testing.T, userId int) []ValuePackageResetCountLedger {
	t.Helper()
	var ledgers []ValuePackageResetCountLedger
	require.NoError(t, DB.Where("user_id = ?", userId).Order("id asc").Find(&ledgers).Error)
	return ledgers
}

func TestRedeemResetCardCodeGrantsResetCount(t *testing.T) {
	truncateTables(t)
	seedRedemptionUser(t, 31, 100)
	code := Redemption{UserId: 1, Key: "RESETCARDCODE31", Name: "重置卡", Type: RedemptionTypeResetCard, ResetCardCount: 3, Status: common.RedemptionCodeStatusEnabled, CreatedTime: common.GetTimestamp()}
	require.NoError(t, DB.Create(&code).Error)

	result, err := Redeem("RESETCARDCODE31", 31)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, RedemptionTypeResetCard, result.Type)
	assert.Equal(t, 3, result.ResetCardCount)
	assert.Equal(t, 0, result.Quota)
	assert.Equal(t, 3, result.Redemption.ResetCardCount)

	// 用户额度不变，重置次数增加
	var user User
	require.NoError(t, DB.Where("id = ?", 31).First(&user).Error)
	assert.Equal(t, 100, user.Quota)

	var pref UserValuePackagePreference
	require.NoError(t, DB.Where("user_id = ?", 31).First(&pref).Error)
	assert.Equal(t, 3, pref.ResetCount)

	ledgers := resetCountLedgersForTest(t, 31)
	require.Len(t, ledgers, 1)
	assert.Equal(t, 3, ledgers[0].Delta)
	assert.Equal(t, 0, ledgers[0].BeforeCount)
	assert.Equal(t, 3, ledgers[0].AfterCount)
	assert.Equal(t, ValuePackageResetCountLedgerSourceRedemption, ledgers[0].Source)

	var saved Redemption
	require.NoError(t, DB.Where("id = ?", code.Id).First(&saved).Error)
	assert.Equal(t, common.RedemptionCodeStatusUsed, saved.Status)
	assert.Equal(t, 31, saved.UsedUserId)
}

func TestRedeemResetCardCodeCannotBeRedeemedTwice(t *testing.T) {
	truncateTables(t)
	seedRedemptionUser(t, 32, 100)
	code := Redemption{UserId: 1, Key: "RESETCARDCODE32", Name: "重置卡", Type: RedemptionTypeResetCard, ResetCardCount: 1, Status: common.RedemptionCodeStatusEnabled, CreatedTime: common.GetTimestamp()}
	require.NoError(t, DB.Create(&code).Error)

	_, err := Redeem("RESETCARDCODE32", 32)
	require.NoError(t, err)
	_, err = Redeem("RESETCARDCODE32", 32)
	assert.True(t, errors.Is(err, ErrRedemptionUsed))

	var pref UserValuePackagePreference
	require.NoError(t, DB.Where("user_id = ?", 32).First(&pref).Error)
	assert.Equal(t, 1, pref.ResetCount)
	assert.Len(t, resetCountLedgersForTest(t, 32), 1)
}

func TestRedeemResetCardCodeWithInvalidCountFailsAndKeepsCode(t *testing.T) {
	truncateTables(t)
	seedRedemptionUser(t, 33, 100)
	code := Redemption{UserId: 1, Key: "RESETCARDCODE33", Name: "坏重置卡", Type: RedemptionTypeResetCard, ResetCardCount: 0, Status: common.RedemptionCodeStatusEnabled, CreatedTime: common.GetTimestamp()}
	require.NoError(t, DB.Create(&code).Error)

	result, err := Redeem("RESETCARDCODE33", 33)
	assert.True(t, errors.Is(err, ErrRedemptionInvalid))
	assert.Nil(t, result)

	var saved Redemption
	require.NoError(t, DB.Where("id = ?", code.Id).First(&saved).Error)
	assert.Equal(t, common.RedemptionCodeStatusEnabled, saved.Status)
	assert.Equal(t, 0, saved.UsedUserId)
}

func TestRedeemResetCardCodeForMissingUserRollsBack(t *testing.T) {
	truncateTables(t)
	code := Redemption{UserId: 1, Key: "RESETCARDMISSINGUSER", Name: "重置卡", Type: RedemptionTypeResetCard, ResetCardCount: 2, Status: common.RedemptionCodeStatusEnabled, CreatedTime: common.GetTimestamp()}
	require.NoError(t, DB.Create(&code).Error)

	result, err := Redeem(code.Key, 39999)
	require.Error(t, err)
	assert.Nil(t, result)

	var saved Redemption
	require.NoError(t, DB.First(&saved, code.Id).Error)
	assert.Equal(t, common.RedemptionCodeStatusEnabled, saved.Status)
	assert.Zero(t, saved.UsedUserId)

	var preferenceCount int64
	require.NoError(t, DB.Model(&UserValuePackagePreference{}).Where("user_id = ?", 39999).Count(&preferenceCount).Error)
	assert.Zero(t, preferenceCount)
	assert.Empty(t, resetCountLedgersForTest(t, 39999))
}

func TestRedeemSubscriptionCodeGiftsResetCardsForValuePackage(t *testing.T) {
	truncateTables(t)
	seedRedemptionUser(t, 34, 100)
	plan := seedResetCardValuePackagePlan(t, 2)
	code := Redemption{UserId: 1, Key: "GIFTPACKAGECODE", Name: "日卡码", Type: RedemptionTypeSubscription, PlanId: plan.Id, Status: common.RedemptionCodeStatusEnabled, CreatedTime: common.GetTimestamp()}
	require.NoError(t, DB.Create(&code).Error)

	result, err := Redeem("GIFTPACKAGECODE", 34)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, RedemptionTypeSubscription, result.Type)
	assert.Equal(t, 2, result.GiftResetCount)

	var pref UserValuePackagePreference
	require.NoError(t, DB.Where("user_id = ?", 34).First(&pref).Error)
	assert.True(t, pref.Enabled)
	assert.Equal(t, 2, pref.ResetCount)

	ledgers := resetCountLedgersForTest(t, 34)
	require.Len(t, ledgers, 1)
	assert.Equal(t, 2, ledgers[0].Delta)
	assert.Equal(t, ValuePackageResetCountLedgerSourcePlanGift, ledgers[0].Source)
}

func TestRedeemSubscriptionCodeReturnsActualClampedGiftCount(t *testing.T) {
	truncateTables(t)
	seedRedemptionUser(t, 340, 100)
	plan := seedResetCardValuePackagePlan(t, MaxSubscriptionPlanGiftResetCount+7)
	code := Redemption{UserId: 1, Key: "GIFTCLAMPCODE", Name: "日卡码", Type: RedemptionTypeSubscription, PlanId: plan.Id, Status: common.RedemptionCodeStatusEnabled, CreatedTime: common.GetTimestamp()}
	require.NoError(t, DB.Create(&code).Error)

	result, err := Redeem(code.Key, 340)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, MaxSubscriptionPlanGiftResetCount, result.GiftResetCount)

	var pref UserValuePackagePreference
	require.NoError(t, DB.Where("user_id = ?", 340).First(&pref).Error)
	assert.Equal(t, MaxSubscriptionPlanGiftResetCount, pref.ResetCount)
	ledgers := resetCountLedgersForTest(t, 340)
	require.Len(t, ledgers, 1)
	assert.Equal(t, MaxSubscriptionPlanGiftResetCount, ledgers[0].Delta)
}

func TestRedeemSubscriptionCodeGiftsResetCardsOnExtend(t *testing.T) {
	truncateTables(t)
	seedRedemptionUser(t, 35, 100)
	plan := seedResetCardValuePackagePlan(t, 1)
	for _, key := range []string{"GIFTEXTENDCODE1", "GIFTEXTENDCODE2"} {
		code := Redemption{UserId: 1, Key: key, Name: "日卡码", Type: RedemptionTypeSubscription, PlanId: plan.Id, Status: common.RedemptionCodeStatusEnabled, CreatedTime: common.GetTimestamp()}
		require.NoError(t, DB.Create(&code).Error)
		result, err := Redeem(key, 35)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.GiftResetCount)
	}

	// 第二次兑换走续期路径，仍应赠送
	var subs []UserSubscription
	require.NoError(t, DB.Where("user_id = ? AND plan_id = ?", 35, plan.Id).Find(&subs).Error)
	require.Len(t, subs, 1)

	var pref UserValuePackagePreference
	require.NoError(t, DB.Where("user_id = ?", 35).First(&pref).Error)
	assert.Equal(t, 2, pref.ResetCount)
	assert.Len(t, resetCountLedgersForTest(t, 35), 2)
}

func TestRedeemSubscriptionCodeDoesNotGiftForRegularPlan(t *testing.T) {
	truncateTables(t)
	seedRedemptionUser(t, 36, 100)
	plan := SubscriptionPlan{Title: "普通订阅", DurationUnit: SubscriptionDurationDay, DurationValue: 1, Enabled: true, TotalAmount: 1000, GiftResetCount: 5}
	require.NoError(t, DB.Create(&plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)
	t.Cleanup(func() { InvalidateSubscriptionPlanCache(plan.Id) })
	code := Redemption{UserId: 1, Key: "REGULARPLANCODE", Name: "订阅码", Type: RedemptionTypeSubscription, PlanId: plan.Id, Status: common.RedemptionCodeStatusEnabled, CreatedTime: common.GetTimestamp()}
	require.NoError(t, DB.Create(&code).Error)

	result, err := Redeem("REGULARPLANCODE", 36)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 0, result.GiftResetCount)

	var pref UserValuePackagePreference
	err = DB.Where("user_id = ?", 36).First(&pref).Error
	if err == nil {
		assert.Equal(t, 0, pref.ResetCount)
	}
	assert.Empty(t, resetCountLedgersForTest(t, 36))
}

func TestNormalizeRedemptionForCreateResetCardDefaultsAndValidates(t *testing.T) {
	truncateTables(t)

	// 默认张数为 1，并清零其他字段
	redemption := &Redemption{Type: RedemptionTypeResetCard, Quota: 500, Amount: 10, Money: 5, CountAsTopUp: true, PlanId: 7}
	require.NoError(t, NormalizeRedemptionForCreate(redemption))
	assert.Equal(t, 1, redemption.ResetCardCount)
	assert.Equal(t, 0, redemption.Quota)
	assert.Equal(t, 0, redemption.PlanId)
	assert.Equal(t, RedemptionKindPromoCredit, redemption.Kind)
	assert.Equal(t, int64(0), redemption.Amount)
	assert.Equal(t, float64(0), redemption.Money)
	assert.False(t, redemption.CountAsTopUp)
	assert.Equal(t, RedemptionSourcePromo, redemption.Source)
	assert.NotEmpty(t, redemption.BatchId)

	valid := &Redemption{Type: RedemptionTypeResetCard, ResetCardCount: 10}
	require.NoError(t, NormalizeRedemptionForCreate(valid))
	assert.Equal(t, 10, valid.ResetCardCount)

	for _, count := range []int{-1, MaxRedemptionResetCardCount + 1} {
		invalid := &Redemption{Type: RedemptionTypeResetCard, ResetCardCount: count}
		err := NormalizeRedemptionForCreate(invalid)
		assert.True(t, errors.Is(err, ErrRedemptionResetCardCountInvalid), "count=%d", count)
	}

	// 非重置卡类型不保留张数
	quotaCode := &Redemption{Type: RedemptionTypeQuota, Quota: 100, ResetCardCount: 9}
	require.NoError(t, NormalizeRedemptionForCreate(quotaCode))
	assert.Equal(t, 0, quotaCode.ResetCardCount)
}
