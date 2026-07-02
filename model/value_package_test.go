package model

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupValuePackageTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := DB
	oldLogDB := LOG_DB
	oldRedisEnabled := common.RedisEnabled
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL

	common.RedisEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	initCol()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	LOG_DB = db
	if subscriptionPlanCache != nil {
		_ = subscriptionPlanCache.Purge()
	}
	if subscriptionPlanInfoCache != nil {
		_ = subscriptionPlanInfoCache.Purge()
	}
	require.NoError(t, db.AutoMigrate(&User{}, &TopUp{}, &SubscriptionPlan{}, &SubscriptionOrder{}, &UserSubscription{}, &UserValuePackagePreference{}, &ValuePackageUsageRecord{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		DB = oldDB
		LOG_DB = oldLogDB
		if subscriptionPlanCache != nil {
			_ = subscriptionPlanCache.Purge()
		}
		if subscriptionPlanInfoCache != nil {
			_ = subscriptionPlanInfoCache.Purge()
		}
		common.RedisEnabled = oldRedisEnabled
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		initCol()
	})
	return db
}

func createValuePackageUser(t *testing.T, id int, group string) User {
	t.Helper()
	user := User{Id: id, Username: fmt.Sprintf("vp-user-%d", id), Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: group, AffCode: fmt.Sprintf("vp-aff-%d", id)}
	require.NoError(t, DB.Create(&user).Error)
	return user
}

func createValuePackagePlan(t *testing.T, packageType string, level int, durationDays int, price float64) SubscriptionPlan {
	t.Helper()
	plan := SubscriptionPlan{
		Title:             packageType + " card",
		PriceAmount:       price,
		Currency:          "USD",
		DurationUnit:      SubscriptionDurationDay,
		DurationValue:     durationDays,
		Enabled:           true,
		PlanKind:          SubscriptionPlanKindValuePackage,
		PackageType:       packageType,
		PackageLevel:      level,
		ModelGroup:        packageType + "-card",
		ConcurrencyLimit:  1,
		Limit5hAmount:     1000,
		Limit7dAmount:     5000,
		LdxpProductUrl:    "https://ldxp.example.test/" + packageType,
		LdxpProductName:   packageType + " product",
		LdxpProductAmount: price,
	}
	require.NoError(t, DB.Create(&plan).Error)
	return plan
}

func createActiveValuePackageSub(t *testing.T, userID int, plan SubscriptionPlan, start int64, end int64) UserSubscription {
	t.Helper()
	sub := UserSubscription{UserId: userID, PlanId: plan.Id, AmountTotal: plan.TotalAmount, StartTime: start, EndTime: end, Status: UserSubscriptionStatusActive, Source: "test"}
	require.NoError(t, DB.Create(&sub).Error)
	return sub
}

func TestValuePackagePurchaseIntentSameLevelExtends(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3001, UserGroupTiyan)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	now := common.GetTimestamp()
	createActiveValuePackageSub(t, user.Id, month, now-100, now+20*86400)

	intent, err := CheckValuePackagePurchaseIntent(user.Id, month.Id, false)

	require.NoError(t, err)
	require.Equal(t, ValuePackagePurchaseActionExtend, intent.Action)
	require.False(t, intent.RequiresConfirmation)
}

func TestValuePackagePurchaseIntentUpgradeRequiresConfirmation(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3002, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	now := common.GetTimestamp()
	createActiveValuePackageSub(t, user.Id, day, now-100, now+3600)

	intent, err := CheckValuePackagePurchaseIntent(user.Id, month.Id, false)

	require.NoError(t, err)
	require.Equal(t, ValuePackagePurchaseActionUpgrade, intent.Action)
	require.True(t, intent.RequiresConfirmation)

	intent, err = CheckValuePackagePurchaseIntent(user.Id, month.Id, true)
	require.NoError(t, err)
	require.Equal(t, ValuePackagePurchaseActionUpgrade, intent.Action)
	require.False(t, intent.RequiresConfirmation)
}

func TestValuePackagePurchaseIntentDowngradeRejected(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3003, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	now := common.GetTimestamp()
	createActiveValuePackageSub(t, user.Id, month, now-100, now+3600)

	_, err := CheckValuePackagePurchaseIntent(user.Id, day.Id, false)

	require.Error(t, err)
	require.Contains(t, err.Error(), "更高等级套餐")
}

func TestCompleteValuePackagePurchaseExtendsSameLevelWithoutChangingUserGroup(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3004, UserGroupTiyan)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	now := common.GetTimestamp()
	existing := createActiveValuePackageSub(t, user.Id, month, now-100, now+20*86400)
	order := SubscriptionOrder{UserId: user.Id, PlanId: month.Id, Money: month.PriceAmount, TradeNo: "vp-extend-order", PaymentMethod: PaymentMethodLDXP, PaymentProvider: PaymentProviderLDXP, Status: common.TopUpStatusPending, CreateTime: now}
	require.NoError(t, DB.Create(&order).Error)

	completed, err := CompleteValuePackageOrder("vp-extend-order", "payload", PaymentProviderLDXP, PaymentMethodLDXP, true)

	require.NoError(t, err)
	require.Equal(t, existing.Id, completed.Id)
	require.GreaterOrEqual(t, completed.EndTime, existing.EndTime+29*86400)
	var reloaded User
	require.NoError(t, DB.First(&reloaded, user.Id).Error)
	require.Equal(t, UserGroupTiyan, reloaded.Group)
}

func TestCompleteValuePackagePurchaseCoversLowerPlanAndCountsVIPTopup(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3005, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 30)
	now := common.GetTimestamp()
	lower := createActiveValuePackageSub(t, user.Id, day, now-100, now+3600)
	order := SubscriptionOrder{UserId: user.Id, PlanId: month.Id, Money: month.PriceAmount, TradeNo: "vp-upgrade-order", PaymentMethod: PaymentMethodLDXP, PaymentProvider: PaymentProviderLDXP, Status: common.TopUpStatusPending, CreateTime: now}
	require.NoError(t, DB.Create(&order).Error)

	completed, err := CompleteValuePackageOrder("vp-upgrade-order", "payload", PaymentProviderLDXP, PaymentMethodLDXP, true)

	require.NoError(t, err)
	require.NotEqual(t, lower.Id, completed.Id)
	var covered UserSubscription
	require.NoError(t, DB.First(&covered, lower.Id).Error)
	require.Equal(t, UserSubscriptionStatusCovered, covered.Status)
	require.Equal(t, completed.Id, covered.CoveredBySubscriptionId)
	var reloaded User
	require.NoError(t, DB.First(&reloaded, user.Id).Error)
	require.Equal(t, UserGroupVIP, reloaded.Group)
	var topUp TopUp
	require.NoError(t, DB.Where("trade_no = ?", "vp-upgrade-order").First(&topUp).Error)
	require.EqualValues(t, 0, topUp.Amount, "value package payment must not add wallet balance")
	require.Equal(t, 30.0, topUp.Money)
}

func TestActivateAndDeactivateValuePackageDoesNotStopClock(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3006, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-100, now+3600)

	state, err := ActivateValuePackage(user.Id, sub.Id)
	require.NoError(t, err)
	require.True(t, state.Preference.Enabled)
	require.Equal(t, sub.Id, state.Preference.ActiveUserSubscriptionId)

	state, err = DeactivateValuePackage(user.Id)
	require.NoError(t, err)
	require.False(t, state.Preference.Enabled)

	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, sub.Id).Error)
	require.Equal(t, sub.EndTime, reloaded.EndTime)
}

func TestValuePackageRollingUsageWindows(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3007, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-100, now+3600)

	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: ValuePackageTypeDay, ModelGroup: "day-card", RequestId: "old", Quota: 900, CreatedAt: now - int64(6*time.Hour/time.Second)}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: ValuePackageTypeDay, ModelGroup: "day-card", RequestId: "recent", Quota: 900, CreatedAt: now - int64(time.Hour/time.Second)}))

	used5h, used7d, err := GetValuePackageWindowUsage(user.Id, sub.Id, now)
	require.NoError(t, err)
	require.EqualValues(t, 900, used5h)
	require.EqualValues(t, 1800, used7d)
}

func TestCompleteValuePackageOrderIdempotentReturnsRecordedSubscription(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3101, UserGroupTiyan)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	now := common.GetTimestamp()
	order := SubscriptionOrder{UserId: user.Id, PlanId: month.Id, Money: month.PriceAmount, TradeNo: "vp-idempotent-order", PaymentMethod: PaymentMethodLDXP, PaymentProvider: PaymentProviderLDXP, Status: common.TopUpStatusPending, CreateTime: now}
	require.NoError(t, DB.Create(&order).Error)

	first, err := CompleteValuePackageOrder(order.TradeNo, "payload-1", PaymentProviderLDXP, PaymentMethodLDXP, true)
	require.NoError(t, err)
	require.NotNil(t, first)

	other := UserSubscription{UserId: user.Id, PlanId: month.Id, AmountTotal: month.TotalAmount, StartTime: now + 10, EndTime: first.EndTime + 90*86400, Status: UserSubscriptionStatusActive, Source: "test-other"}
	require.NoError(t, DB.Create(&other).Error)

	retry, err := CompleteValuePackageOrder(order.TradeNo, "payload-2", PaymentProviderLDXP, PaymentMethodLDXP, true)
	require.NoError(t, err)
	require.NotNil(t, retry)
	require.Equal(t, first.Id, retry.Id)
	require.NotEqual(t, other.Id, retry.Id)

	var reloadedOrder SubscriptionOrder
	require.NoError(t, DB.Where("trade_no = ?", order.TradeNo).First(&reloadedOrder).Error)
	require.Equal(t, first.Id, reloadedOrder.UserSubscriptionId)
}

func TestCompleteValuePackageOrderUnconfirmedUpgradeDoesNotMutateState(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3102, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	now := common.GetTimestamp()
	lower := createActiveValuePackageSub(t, user.Id, day, now-100, now+3600)
	order := SubscriptionOrder{UserId: user.Id, PlanId: month.Id, Money: month.PriceAmount, TradeNo: "vp-unconfirmed-upgrade", PaymentMethod: PaymentMethodLDXP, PaymentProvider: PaymentProviderLDXP, Status: common.TopUpStatusPending, CreateTime: now}
	require.NoError(t, DB.Create(&order).Error)

	completed, err := CompleteValuePackageOrder(order.TradeNo, "payload", PaymentProviderLDXP, PaymentMethodLDXP, false)
	require.Error(t, err)
	require.Nil(t, completed)

	var orderAfter SubscriptionOrder
	require.NoError(t, DB.Where("trade_no = ?", order.TradeNo).First(&orderAfter).Error)
	require.Equal(t, common.TopUpStatusPending, orderAfter.Status)
	require.Zero(t, orderAfter.UserSubscriptionId)
	var topupCount int64
	require.NoError(t, DB.Model(&TopUp{}).Where("trade_no = ?", order.TradeNo).Count(&topupCount).Error)
	require.Zero(t, topupCount)
	var subs []UserSubscription
	require.NoError(t, DB.Where("user_id = ?", user.Id).Find(&subs).Error)
	require.Len(t, subs, 1)
	require.Equal(t, lower.Id, subs[0].Id)
	require.Equal(t, UserSubscriptionStatusActive, subs[0].Status)
}

func TestActivateValuePackageRejectsNonValuePlanAndExpiredSubscription(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3103, UserGroupTiyan)
	now := common.GetTimestamp()
	nonValue := SubscriptionPlan{Title: "normal sub", PriceAmount: 9.9, Currency: "USD", DurationUnit: SubscriptionDurationDay, DurationValue: 1, Enabled: true, PlanKind: SubscriptionPlanKindSubscription}
	require.NoError(t, DB.Create(&nonValue).Error)
	nonValueSub := createActiveValuePackageSub(t, user.Id, nonValue, now-100, now+3600)

	state, err := ActivateValuePackage(user.Id, nonValueSub.Id)
	require.Error(t, err)
	require.Nil(t, state)

	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	expired := UserSubscription{UserId: user.Id, PlanId: day.Id, StartTime: now - 7200, EndTime: now - 3600, Status: UserSubscriptionStatusActive, Source: "test"}
	require.NoError(t, DB.Create(&expired).Error)
	state, err = ActivateValuePackage(user.Id, expired.Id)
	require.Error(t, err)
	require.Nil(t, state)
}

func TestCompleteValuePackageOrderRejectsExistingBalanceTopUpTradeNo(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3104, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	order := SubscriptionOrder{UserId: user.Id, PlanId: day.Id, Money: day.PriceAmount, TradeNo: "vp-existing-balance-topup", PaymentMethod: PaymentMethodLDXP, PaymentProvider: PaymentProviderLDXP, Status: common.TopUpStatusPending, CreateTime: now}
	require.NoError(t, DB.Create(&order).Error)
	require.NoError(t, DB.Create(&TopUp{UserId: user.Id, Amount: 100, Money: day.PriceAmount, TradeNo: order.TradeNo, PaymentMethod: PaymentMethodLDXP, PaymentProvider: PaymentProviderLDXP, Status: common.TopUpStatusPending, CreateTime: now}).Error)

	completed, err := CompleteValuePackageOrder(order.TradeNo, "payload", PaymentProviderLDXP, PaymentMethodLDXP, true)
	require.Error(t, err)
	require.Nil(t, completed)

	var orderAfter SubscriptionOrder
	require.NoError(t, DB.Where("trade_no = ?", order.TradeNo).First(&orderAfter).Error)
	require.Equal(t, common.TopUpStatusPending, orderAfter.Status)
	var topup TopUp
	require.NoError(t, DB.Where("trade_no = ?", order.TradeNo).First(&topup).Error)
	require.EqualValues(t, 100, topup.Amount)
}

func TestCompleteValuePackageOrderUsesActualPaymentMethodForOrderAndTopUp(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3105, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	order := SubscriptionOrder{UserId: user.Id, PlanId: day.Id, Money: day.PriceAmount, TradeNo: "vp-payment-method-consistency", PaymentMethod: PaymentProviderEpay, PaymentProvider: PaymentProviderLDXP, Status: common.TopUpStatusPending, CreateTime: now}
	require.NoError(t, DB.Create(&order).Error)

	completed, err := CompleteValuePackageOrder(order.TradeNo, "payload", PaymentProviderLDXP, PaymentMethodLDXP, true)
	require.NoError(t, err)
	require.NotNil(t, completed)

	var orderAfter SubscriptionOrder
	require.NoError(t, DB.Where("trade_no = ?", order.TradeNo).First(&orderAfter).Error)
	require.Equal(t, PaymentMethodLDXP, orderAfter.PaymentMethod)
	var topup TopUp
	require.NoError(t, DB.Where("trade_no = ?", order.TradeNo).First(&topup).Error)
	require.Equal(t, PaymentMethodLDXP, topup.PaymentMethod)
	require.Equal(t, orderAfter.PaymentProvider, topup.PaymentProvider)
	require.Zero(t, topup.Amount)
	require.Equal(t, orderAfter.Money, topup.Money)
}

func TestActivateAndDeactivateValuePackagePreferenceUpsertIsRepeatable(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3106, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-100, now+3600)

	state, err := DeactivateValuePackage(user.Id)
	require.NoError(t, err)
	require.False(t, state.Preference.Enabled)
	require.Zero(t, state.Preference.ActiveUserSubscriptionId)

	state, err = DeactivateValuePackage(user.Id)
	require.NoError(t, err)
	require.False(t, state.Preference.Enabled)

	state, err = ActivateValuePackage(user.Id, sub.Id)
	require.NoError(t, err)
	require.True(t, state.Preference.Enabled)
	require.Equal(t, sub.Id, state.Preference.ActiveUserSubscriptionId)

	state, err = ActivateValuePackage(user.Id, sub.Id)
	require.NoError(t, err)
	require.True(t, state.Preference.Enabled)
	require.Equal(t, sub.Id, state.Preference.ActiveUserSubscriptionId)

	var count int64
	require.NoError(t, DB.Model(&UserValuePackagePreference{}).Where("user_id = ?", user.Id).Count(&count).Error)
	require.EqualValues(t, 1, count)
}
