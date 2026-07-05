package model

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	require.NoError(t, db.AutoMigrate(&User{}, &TopUp{}, &SubscriptionPlan{}, &SubscriptionOrder{}, &UserSubscription{}, &UserValuePackagePreference{}, &ValuePackageUsageRecord{}, &SubscriptionPreConsumeRecord{}, &AffiliateCommission{}, &AffiliateWithdrawal{}))

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

func TestValuePackageUpdateLockUsesGormClause(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DryRun: true})
	require.NoError(t, err)

	stmt := withUpdateLock(db).Where("id = ?", 1).First(&User{}).Statement

	locking, ok := stmt.Clauses["FOR"].Expression.(clause.Locking)
	require.True(t, ok)
	require.Equal(t, "UPDATE", locking.Strength)
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

func createRegularSubscriptionPlanForValuePackageTest(t *testing.T, title string, totalAmount int64) SubscriptionPlan {
	t.Helper()
	plan := SubscriptionPlan{
		Title:         title,
		PriceAmount:   9.9,
		Currency:      "USD",
		DurationUnit:  SubscriptionDurationDay,
		DurationValue: 1,
		Enabled:       true,
		PlanKind:      SubscriptionPlanKindSubscription,
		TotalAmount:   totalAmount,
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

func TestCreateUserSubscriptionFromPlanTxRejectsValuePackagePlan(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3200, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)

	var createdSub *UserSubscription
	var createErr error
	txErr := DB.Transaction(func(tx *gorm.DB) error {
		createdSub, createErr = CreateUserSubscriptionFromPlanTx(tx, user.Id, &day, "test")
		return createErr
	})

	require.Error(t, createErr)
	require.Error(t, txErr)
	require.Nil(t, createdSub)
	require.Contains(t, createErr.Error(), "超值套餐不能通过普通订阅创建")

	var count int64
	require.NoError(t, DB.Model(&UserSubscription{}).Where("user_id = ? AND plan_id = ?", user.Id, day.Id).Count(&count).Error)
	require.Zero(t, count)
}

func TestHasActiveUserSubscriptionIgnoresOnlyValuePackage(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3201, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	createActiveValuePackageSub(t, user.Id, day, now-10, now+3600)
	orphan := UserSubscription{UserId: user.Id, PlanId: 999999, AmountTotal: 100, StartTime: now - 10, EndTime: now + 3600, Status: UserSubscriptionStatusActive, Source: "test-orphan"}
	require.NoError(t, DB.Create(&orphan).Error)

	hasActive, err := HasActiveUserSubscription(user.Id)

	require.NoError(t, err)
	require.False(t, hasActive)

	regular := createRegularSubscriptionPlanForValuePackageTest(t, "regular sub", 100)
	createActiveValuePackageSub(t, user.Id, regular, now-10, now+3600)

	hasActive, err = HasActiveUserSubscription(user.Id)
	require.NoError(t, err)
	require.True(t, hasActive)
}

func TestPreConsumeUserSubscriptionSkipsValuePackage(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3202, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	valuePackageSub := createActiveValuePackageSub(t, user.Id, day, now-10, now+3600)

	result, err := PreConsumeUserSubscription("vp-only-preconsume", user.Id, "gpt-test", 0, 1)

	require.Error(t, err)
	require.Nil(t, result)
	var recordCount int64
	require.NoError(t, DB.Model(&SubscriptionPreConsumeRecord{}).Where("user_id = ?", user.Id).Count(&recordCount).Error)
	require.Zero(t, recordCount)

	regular := createRegularSubscriptionPlanForValuePackageTest(t, "metered regular sub", 10)
	regularSub := createActiveValuePackageSub(t, user.Id, regular, now-10, now+3600)

	result, err = PreConsumeUserSubscription("regular-with-vp-preconsume", user.Id, "gpt-test", 0, 4)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, regularSub.Id, result.UserSubscriptionId)
	require.EqualValues(t, 4, result.PreConsumed)
	require.EqualValues(t, 0, result.AmountUsedBefore)
	require.EqualValues(t, 4, result.AmountUsedAfter)

	var reloadedValuePackage UserSubscription
	require.NoError(t, DB.First(&reloadedValuePackage, valuePackageSub.Id).Error)
	require.Zero(t, reloadedValuePackage.AmountUsed)
	var reloadedRegular UserSubscription
	require.NoError(t, DB.First(&reloadedRegular, regularSub.Id).Error)
	require.EqualValues(t, 4, reloadedRegular.AmountUsed)
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
	activated, err := ActivateValuePackage(user.Id, existing.Id)
	require.NoError(t, err)
	require.True(t, activated.Preference.Enabled)
	order := SubscriptionOrder{UserId: user.Id, PlanId: month.Id, Money: month.PriceAmount, TradeNo: "vp-extend-order", PaymentMethod: PaymentMethodLDXP, PaymentProvider: PaymentProviderLDXP, Status: common.TopUpStatusPending, CreateTime: now}
	require.NoError(t, DB.Create(&order).Error)

	completed, err := CompleteValuePackageOrder("vp-extend-order", "payload", PaymentProviderLDXP, PaymentMethodLDXP, true)

	require.NoError(t, err)
	require.Equal(t, existing.Id, completed.Id)
	require.GreaterOrEqual(t, completed.EndTime, existing.EndTime+29*86400)
	var reloaded User
	require.NoError(t, DB.First(&reloaded, user.Id).Error)
	require.Equal(t, UserGroupTiyan, reloaded.Group)
	state, err := GetValuePackageState(user.Id)
	require.NoError(t, err)
	require.False(t, state.Preference.Enabled)
	require.Equal(t, completed.Id, state.Preference.ActiveUserSubscriptionId)
	require.NotNil(t, state.Subscription)
	require.Equal(t, completed.Id, state.Subscription.Id)
	require.NotNil(t, state.Plan)
	require.Equal(t, month.Id, state.Plan.Id)
}

func TestCompleteValuePackagePurchaseCoversLowerPlanAndCountsVIPTopup(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3005, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 30)
	now := common.GetTimestamp()
	lower := createActiveValuePackageSub(t, user.Id, day, now-100, now+3600)
	activated, err := ActivateValuePackage(user.Id, lower.Id)
	require.NoError(t, err)
	require.True(t, activated.Preference.Enabled)
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
	state, err := GetValuePackageState(user.Id)
	require.NoError(t, err)
	require.False(t, state.Preference.Enabled)
	require.Equal(t, completed.Id, state.Preference.ActiveUserSubscriptionId)
	require.NotNil(t, state.Subscription)
	require.Equal(t, completed.Id, state.Subscription.Id)
	require.NotNil(t, state.Plan)
	require.Equal(t, month.Id, state.Plan.Id)
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
	require.Equal(t, sub.Id, state.Preference.ActiveUserSubscriptionId)
	require.NotNil(t, state.Subscription)
	require.Equal(t, sub.Id, state.Subscription.Id)
	require.NotNil(t, state.Plan)
	require.Equal(t, day.Id, state.Plan.Id)

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

func TestActivateValuePackageReturnsUsageSummary(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3400, UserGroupVIP)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	day.TotalAmount = 100
	day.Limit5hAmount = 50
	day.Limit7dAmount = 75
	require.NoError(t, DB.Save(&day).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-10, now+3600)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("amount_used", int64(25)).Error)
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "activate-summary-recent", Quota: 10, CreatedAt: now}))

	state, err := ActivateValuePackage(user.Id, sub.Id)

	require.NoError(t, err)
	require.NotNil(t, state)
	require.True(t, state.Preference.Enabled)
	require.Equal(t, sub.Id, state.Preference.ActiveUserSubscriptionId)
	require.NotNil(t, state.Subscription)
	require.Equal(t, sub.Id, state.Subscription.Id)
	require.NotNil(t, state.Plan)
	require.Equal(t, day.Id, state.Plan.Id)
	require.NotNil(t, state.Usage)
	require.EqualValues(t, 25, state.Usage.TotalUsed)
	require.EqualValues(t, 100, state.Usage.TotalLimit)
	require.EqualValues(t, 10, state.Usage.Used5h)
	require.EqualValues(t, 50, state.Usage.Limit5h)
}

func TestGetValuePackageStateIncludesUsageSummary(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3401, UserGroupVIP)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	day.TotalAmount = 200
	day.Limit5hAmount = 100
	day.Limit7dAmount = 150
	require.NoError(t, DB.Save(&day).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-10, now+3600)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("amount_used", int64(80)).Error)
	_, err := upsertValuePackagePreferenceTx(DB, user.Id, true, sub.Id)
	require.NoError(t, err)
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "summary-recent", Quota: 40, CreatedAt: now}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "summary-older", Quota: 30, CreatedAt: now - 6*3600}))

	state, err := GetValuePackageState(user.Id)

	require.NoError(t, err)
	require.NotNil(t, state)
	require.NotNil(t, state.Usage)
	require.EqualValues(t, 80, state.Usage.TotalUsed)
	require.EqualValues(t, 200, state.Usage.TotalLimit)
	require.EqualValues(t, 120, state.Usage.TotalRemaining)
	require.InDelta(t, 40.0, state.Usage.TotalPercent, 0.001)
	require.EqualValues(t, 40, state.Usage.Used5h)
	require.EqualValues(t, 100, state.Usage.Limit5h)
	require.InDelta(t, 40.0, state.Usage.Percent5h, 0.001)
	require.EqualValues(t, 70, state.Usage.Used7d)
	require.EqualValues(t, 150, state.Usage.Limit7d)
	require.InDelta(t, 46.666, state.Usage.Percent7d, 0.01)
	require.False(t, state.Usage.Exhausted)
	require.Empty(t, state.Usage.ExhaustedReason)

	var reloaded User
	require.NoError(t, DB.First(&reloaded, user.Id).Error)
	require.Equal(t, UserGroupVIP, reloaded.Group)
}

func TestGetActiveValuePackageForRelayOmitsUsageSummary(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3403, UserGroupVIP)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	day.TotalAmount = 200
	day.Limit5hAmount = 100
	day.Limit7dAmount = 150
	require.NoError(t, DB.Save(&day).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-10, now+3600)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("amount_used", int64(80)).Error)
	_, err := upsertValuePackagePreferenceTx(DB, user.Id, true, sub.Id)
	require.NoError(t, err)
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "relay-summary-recent", Quota: 40, CreatedAt: now}))

	state, err := GetValuePackageState(user.Id)

	require.NoError(t, err)
	require.NotNil(t, state)
	require.NotNil(t, state.Usage)

	relayState, err := GetActiveValuePackageForRelay(user.Id)

	require.NoError(t, err)
	require.NotNil(t, relayState)
	require.NotNil(t, relayState.Subscription)
	require.Equal(t, sub.Id, relayState.Subscription.Id)
	require.NotNil(t, relayState.Plan)
	require.Equal(t, day.Id, relayState.Plan.Id)
	require.NotEmpty(t, relayState.Plan.ModelGroup)
	require.Nil(t, relayState.Usage)
}

func TestGetValuePackageStateMarksExhaustedUsageSummary(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3402, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	day.TotalAmount = 100
	day.Limit5hAmount = 1000
	day.Limit7dAmount = 1000
	require.NoError(t, DB.Save(&day).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-10, now+3600)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("amount_used", int64(100)).Error)
	_, err := upsertValuePackagePreferenceTx(DB, user.Id, true, sub.Id)
	require.NoError(t, err)

	state, err := GetValuePackageState(user.Id)

	require.NoError(t, err)
	require.NotNil(t, state.Usage)
	require.True(t, state.Usage.Exhausted)
	require.Equal(t, ValuePackageExhaustedReasonTotal, state.Usage.ExhaustedReason)
	require.Equal(t, ValuePackageQuotaExhaustedUserMessage, state.Usage.ExhaustedMessage)
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
	activated, err := ActivateValuePackage(user.Id, first.Id)
	require.NoError(t, err)
	require.True(t, activated.Preference.Enabled)
	require.Equal(t, first.Id, activated.Preference.ActiveUserSubscriptionId)

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
	state, err := GetValuePackageState(user.Id)
	require.NoError(t, err)
	require.True(t, state.Preference.Enabled)
	require.Equal(t, first.Id, state.Preference.ActiveUserSubscriptionId)
}

func TestCompleteValuePackageOrderCreatesDisabledPreferenceForPaidButNotActivatedPackage(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3111, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	order := SubscriptionOrder{UserId: user.Id, PlanId: day.Id, Money: day.PriceAmount, TradeNo: "vp-paid-not-activated", PaymentMethod: PaymentMethodLDXP, PaymentProvider: PaymentProviderLDXP, Status: common.TopUpStatusPending, CreateTime: now}
	require.NoError(t, DB.Create(&order).Error)

	completed, err := CompleteValuePackageOrder(order.TradeNo, "payload", PaymentProviderLDXP, PaymentMethodLDXP, true)
	require.NoError(t, err)
	require.NotNil(t, completed)

	state, err := GetValuePackageState(user.Id)
	require.NoError(t, err)
	require.False(t, state.Preference.Enabled)
	require.Equal(t, completed.Id, state.Preference.ActiveUserSubscriptionId)
	require.NotNil(t, state.Subscription)
	require.Equal(t, completed.Id, state.Subscription.Id)
	require.NotNil(t, state.Plan)
	require.Equal(t, day.Id, state.Plan.Id)
}

func TestGetValuePackageStateKeepsActiveSubscriptionWhenPlanDisabled(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3112, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-10, now+3600)
	_, err := ActivateValuePackage(user.Id, sub.Id)
	require.NoError(t, err)
	require.NoError(t, DB.Model(&SubscriptionPlan{}).Where("id = ?", day.Id).Update("enabled", false).Error)
	InvalidateSubscriptionPlanCache(day.Id)

	plans, err := GetValuePackagePlansForUser(user.Id)
	require.NoError(t, err)
	require.Empty(t, plans)

	state, err := GetValuePackageState(user.Id)
	require.NoError(t, err)
	require.True(t, state.Preference.Enabled)
	require.Equal(t, sub.Id, state.Preference.ActiveUserSubscriptionId)
	require.NotNil(t, state.Subscription)
	require.Equal(t, sub.Id, state.Subscription.Id)
	require.NotNil(t, state.Plan)
	require.Equal(t, day.Id, state.Plan.Id)
	require.False(t, state.Plan.Enabled)
}

func TestAdminBindSubscriptionRejectsValuePackagePlan(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3113, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)

	msg, err := AdminBindSubscription(user.Id, day.Id, "")

	require.Error(t, err)
	require.Empty(t, msg)
	require.Contains(t, err.Error(), "超值套餐不能通过普通订阅绑定，请使用超值套餐专用流程")
	var subCount int64
	require.NoError(t, DB.Model(&UserSubscription{}).Where("user_id = ?", user.Id).Count(&subCount).Error)
	require.Zero(t, subCount)
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

func TestCompleteValuePackageOrderPreservesOrderLookupDBError(t *testing.T) {
	setupValuePackageTestDB(t)
	forcedErr := errors.New("forced subscription order lookup failure")
	callbackName := "test:force_subscription_order_lookup_error:" + strings.ReplaceAll(t.Name(), "/", "_")
	require.NoError(t, DB.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema == nil || tx.Statement.Schema.Name != "SubscriptionOrder" {
			return
		}
		where := fmt.Sprintf("%#v", tx.Statement.Clauses["WHERE"].Expression)
		if strings.Contains(where, "trade_no = ?") {
			tx.AddError(forcedErr)
		}
	}))
	t.Cleanup(func() {
		require.NoError(t, DB.Callback().Query().Remove(callbackName))
	})

	completed, err := CompleteValuePackageOrder("vp-order-db-error", "payload", PaymentProviderLDXP, PaymentMethodLDXP, true)

	require.Error(t, err)
	require.ErrorIs(t, err, forcedErr)
	require.False(t, errors.Is(err, ErrSubscriptionOrderNotFound), "infrastructure lookup errors must not be converted to order-not-found business errors")
	require.Nil(t, completed)
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

func TestCompleteValuePackageOrderCreatesAffiliateCommissionForInviter(t *testing.T) {
	setupValuePackageTestDB(t)
	inviter := createValuePackageUser(t, 3110, UserGroupVIP)
	invitee := createValuePackageUser(t, 3111, UserGroupTiyan)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", invitee.Id).Update("inviter_id", inviter.Id).Error)
	week := createValuePackagePlan(t, ValuePackageTypeWeek, ValuePackageLevelWeek, 7, 28.8)
	now := common.GetTimestamp()
	order := SubscriptionOrder{UserId: invitee.Id, PlanId: week.Id, Money: week.PriceAmount, TradeNo: "vp-affiliate-week-order", PaymentMethod: PaymentMethodLDXP, PaymentProvider: PaymentProviderLDXP, Status: common.TopUpStatusPending, CreateTime: now}
	require.NoError(t, DB.Create(&order).Error)

	completed, err := CompleteValuePackageOrder(order.TradeNo, "payload", PaymentProviderLDXP, PaymentMethodLDXP, true)
	require.NoError(t, err)
	require.NotNil(t, completed)

	var topup TopUp
	require.NoError(t, DB.Where("trade_no = ?", order.TradeNo).First(&topup).Error)
	var commission AffiliateCommission
	require.NoError(t, DB.Where("topup_id = ?", topup.Id).First(&commission).Error)
	require.Equal(t, inviter.Id, commission.InviterUserId)
	require.Equal(t, invitee.Id, commission.InviteeUserId)
	require.Equal(t, order.TradeNo, commission.TradeNo)
	require.Equal(t, 28.8, commission.BaseMoney)
	require.Equal(t, 4.32, commission.CommissionMoney)
	require.Equal(t, AffiliateCommissionStatusAvailable, commission.Status)

	_, err = CompleteValuePackageOrder(order.TradeNo, "payload-retry", PaymentProviderLDXP, PaymentMethodLDXP, true)
	require.NoError(t, err)
	var count int64
	require.NoError(t, DB.Model(&AffiliateCommission{}).Where("topup_id = ?", topup.Id).Count(&count).Error)
	require.Equal(t, int64(1), count)
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

func TestCompleteValuePackageOrderPreservesCompletedSubscriptionLookupDBError(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3107, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	order := SubscriptionOrder{UserId: user.Id, PlanId: day.Id, Money: day.PriceAmount, TradeNo: "vp-completed-subscription-db-error", PaymentMethod: PaymentMethodLDXP, PaymentProvider: PaymentProviderLDXP, Status: common.TopUpStatusPending, CreateTime: now}
	require.NoError(t, DB.Create(&order).Error)

	completed, err := CompleteValuePackageOrder(order.TradeNo, "payload-1", PaymentProviderLDXP, PaymentMethodLDXP, true)
	require.NoError(t, err)
	require.NotNil(t, completed)

	forcedErr := errors.New("forced completed subscription lookup failure")
	callbackName := "test:force_completed_subscription_lookup_error:" + strings.ReplaceAll(t.Name(), "/", "_")
	require.NoError(t, DB.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema == nil || tx.Statement.Schema.Name != "UserSubscription" {
			return
		}
		where := fmt.Sprintf("%#v", tx.Statement.Clauses["WHERE"].Expression)
		if strings.Contains(where, "id = ? AND user_id = ?") {
			tx.AddError(forcedErr)
		}
	}))
	t.Cleanup(func() {
		require.NoError(t, DB.Callback().Query().Remove(callbackName))
	})

	retry, err := CompleteValuePackageOrder(order.TradeNo, "payload-2", PaymentProviderLDXP, PaymentMethodLDXP, true)

	require.Error(t, err)
	require.ErrorIs(t, err, forcedErr)
	require.False(t, errors.Is(err, ErrCompletedSubscriptionNotFound), "infrastructure lookup errors must not be converted to completed-subscription business errors")
	require.Nil(t, retry)
}

func TestCompleteValuePackageOrderReturnsTypedErrorWhenCompletedSubscriptionMissing(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3108, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	order := SubscriptionOrder{UserId: user.Id, PlanId: day.Id, Money: day.PriceAmount, TradeNo: "vp-completed-subscription-missing", PaymentMethod: PaymentMethodLDXP, PaymentProvider: PaymentProviderLDXP, Status: common.TopUpStatusPending, CreateTime: now}
	require.NoError(t, DB.Create(&order).Error)

	completed, err := CompleteValuePackageOrder(order.TradeNo, "payload-1", PaymentProviderLDXP, PaymentMethodLDXP, true)
	require.NoError(t, err)
	require.NotNil(t, completed)

	require.NoError(t, DB.Delete(&UserSubscription{}, completed.Id).Error)

	retry, err := CompleteValuePackageOrder(order.TradeNo, "payload-2", PaymentProviderLDXP, PaymentMethodLDXP, true)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrCompletedSubscriptionNotFound)
	require.Nil(t, retry)
}

func TestCompleteSubscriptionOrderRejectsAlreadySuccessfulValuePackageOrder(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3114, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	order := SubscriptionOrder{
		UserId:          user.Id,
		PlanId:          day.Id,
		Money:           day.PriceAmount,
		TradeNo:         "vp-already-success-ordinary-complete",
		PaymentMethod:   PaymentMethodStripe,
		PaymentProvider: PaymentProviderStripe,
		Status:          common.TopUpStatusSuccess,
		CreateTime:      common.GetTimestamp(),
		CompleteTime:    common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(&order).Error)

	err := CompleteSubscriptionOrder(order.TradeNo, "payload", PaymentProviderStripe, PaymentMethodStripe)

	require.Error(t, err)
	require.Contains(t, err.Error(), "超值套餐仅支持联动小铺购买")
}

func TestPreConsumeValuePackageSubscriptionChecksRollingReservationWindows(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3310, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	day.TotalAmount = 1000
	day.Limit5hAmount = 100
	day.Limit7dAmount = 0
	require.NoError(t, DB.Save(&day).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-10, now+3600)
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "existing-90", Quota: 90, CreatedAt: now}))

	_, err := PreConsumeValuePackageSubscription("rolling-reserve-too-large", user.Id, sub.Id, 20)
	require.Error(t, err)
	require.Contains(t, err.Error(), "subscription quota insufficient")

	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, sub.Id).Error)
	require.EqualValues(t, 0, reloaded.AmountUsed)
	var failedCount int64
	require.NoError(t, DB.Model(&ValuePackageUsageRecord{}).Where("request_id = ?", "rolling-reserve-too-large").Count(&failedCount).Error)
	require.EqualValues(t, 0, failedCount)

	result, err := PreConsumeValuePackageSubscription("rolling-reserve-fits", user.Id, sub.Id, 10)
	require.NoError(t, err)
	require.Equal(t, sub.Id, result.UserSubscriptionId)
	require.EqualValues(t, 10, result.PreConsumed)

	used5h, used7d, err := GetValuePackageWindowUsage(user.Id, sub.Id, now)
	require.NoError(t, err)
	require.EqualValues(t, 100, used5h)
	require.EqualValues(t, 100, used7d)
}

func TestRecordValuePackageUsageUpsertsBySubscriptionAndRequest(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3311, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-10, now+3600)

	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "same-request", Quota: 20, CreatedAt: now}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "same-request", Quota: 45, CreatedAt: now}))

	var count int64
	require.NoError(t, DB.Model(&ValuePackageUsageRecord{}).Where("user_subscription_id = ? AND request_id = ?", sub.Id, "same-request").Count(&count).Error)
	require.EqualValues(t, 1, count)
	used5h, used7d, err := GetValuePackageWindowUsage(user.Id, sub.Id, now)
	require.NoError(t, err)
	require.EqualValues(t, 45, used5h)
	require.EqualValues(t, 45, used7d)
}

func TestListActiveValuePackageUsageRowsReturnsRealtimeWindowUsage(t *testing.T) {
	setupValuePackageTestDB(t)
	now := common.GetTimestamp()
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	day.TotalAmount = 10000
	day.Limit5hAmount = 1000
	day.Limit7dAmount = 5000
	require.NoError(t, DB.Save(&day).Error)
	week := createValuePackagePlan(t, ValuePackageTypeWeek, ValuePackageLevelWeek, 7, 9.9)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)

	activeUser := createValuePackageUser(t, 3501, UserGroupTiyan)
	activeSub := createActiveValuePackageSub(t, activeUser.Id, day, now-100, now+86400)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", activeSub.Id).Updates(map[string]any{"amount_used": int64(700)}).Error)
	require.NoError(t, DB.Create(&UserValuePackagePreference{UserId: activeUser.Id, Enabled: true, ActiveUserSubscriptionId: activeSub.Id}).Error)
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: activeUser.Id, UserSubscriptionId: activeSub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "active-5h", Quota: 100, CreatedAt: now - 3600}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: activeUser.Id, UserSubscriptionId: activeSub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "active-7d", Quota: 200, CreatedAt: now - 6*3600}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: activeUser.Id, UserSubscriptionId: activeSub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "active-old", Quota: 999, CreatedAt: now - 8*24*3600}))

	disabledUser := createValuePackageUser(t, 3502, UserGroupTiyan)
	disabledSub := createActiveValuePackageSub(t, disabledUser.Id, week, now-100, now+86400)
	require.NoError(t, DB.Create(&UserValuePackagePreference{UserId: disabledUser.Id, Enabled: false, ActiveUserSubscriptionId: disabledSub.Id}).Error)

	otherActiveUser := createValuePackageUser(t, 3503, UserGroupTiyan)
	otherActiveSub := createActiveValuePackageSub(t, otherActiveUser.Id, month, now-100, now+86400)
	require.NoError(t, DB.Create(&UserValuePackagePreference{UserId: otherActiveUser.Id, Enabled: true, ActiveUserSubscriptionId: activeSub.Id}).Error)
	_ = otherActiveSub

	expiredUser := createValuePackageUser(t, 3504, UserGroupTiyan)
	expiredSub := createActiveValuePackageSub(t, expiredUser.Id, day, now-86400, now-1)
	require.NoError(t, DB.Create(&UserValuePackagePreference{UserId: expiredUser.Id, Enabled: true, ActiveUserSubscriptionId: expiredSub.Id}).Error)

	rows, err := ListActiveValuePackageUsageRows(now)

	require.NoError(t, err)
	require.Len(t, rows, 1)
	row := rows[0]
	require.Equal(t, activeUser.Id, row.UserId)
	require.Equal(t, activeUser.Username, row.Username)
	require.Equal(t, activeSub.Id, row.Subscription.Id)
	require.Equal(t, day.Id, row.Plan.Id)
	require.NotNil(t, row.Usage)
	require.EqualValues(t, 100, row.Usage.Used5h)
	require.EqualValues(t, 300, row.Usage.Used7d)
	require.EqualValues(t, 1000, row.Usage.Limit5h)
	require.EqualValues(t, 5000, row.Usage.Limit7d)
	require.EqualValues(t, 700, row.Usage.TotalUsed)
	require.EqualValues(t, 9300, row.Usage.TotalRemaining)
}

func TestRefundSubscriptionPreConsumeRevokesValuePackageUsageReservation(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3312, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	day.TotalAmount = 1000
	require.NoError(t, DB.Save(&day).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-10, now+3600)

	_, err := PreConsumeValuePackageSubscription("vp-refund-reservation", user.Id, sub.Id, 100)
	require.NoError(t, err)
	used5h, used7d, err := GetValuePackageWindowUsage(user.Id, sub.Id, now)
	require.NoError(t, err)
	require.EqualValues(t, 100, used5h)
	require.EqualValues(t, 100, used7d)

	require.NoError(t, RefundSubscriptionPreConsume("vp-refund-reservation"))
	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, sub.Id).Error)
	require.EqualValues(t, 0, reloaded.AmountUsed)
	used5h, used7d, err = GetValuePackageWindowUsage(user.Id, sub.Id, now)
	require.NoError(t, err)
	require.EqualValues(t, 0, used5h)
	require.EqualValues(t, 0, used7d)

	require.NoError(t, RefundSubscriptionPreConsume("vp-refund-reservation"))
	require.NoError(t, DB.First(&reloaded, sub.Id).Error)
	require.EqualValues(t, 0, reloaded.AmountUsed)
	used5h, used7d, err = GetValuePackageWindowUsage(user.Id, sub.Id, now)
	require.NoError(t, err)
	require.EqualValues(t, 0, used5h)
	require.EqualValues(t, 0, used7d)
}

func TestPreConsumeValuePackageSubscriptionOnlyAcceptsValuePackageAndIsIdempotent(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3301, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	day.TotalAmount = 10
	require.NoError(t, DB.Save(&day).Error)
	now := common.GetTimestamp()
	valueSub := createActiveValuePackageSub(t, user.Id, day, now-10, now+3600)

	result, err := PreConsumeValuePackageSubscription("vp-specific-preconsume", user.Id, valueSub.Id, 4)
	require.NoError(t, err)
	require.Equal(t, valueSub.Id, result.UserSubscriptionId)
	require.EqualValues(t, 4, result.PreConsumed)
	require.EqualValues(t, 0, result.AmountUsedBefore)
	require.EqualValues(t, 4, result.AmountUsedAfter)

	result, err = PreConsumeValuePackageSubscription("vp-specific-preconsume", user.Id, valueSub.Id, 4)
	require.NoError(t, err)
	require.Equal(t, valueSub.Id, result.UserSubscriptionId)
	require.EqualValues(t, 4, result.PreConsumed)
	require.NoError(t, DB.First(&valueSub, valueSub.Id).Error)
	require.EqualValues(t, 4, valueSub.AmountUsed)

	_, err = PreConsumeValuePackageSubscription("vp-specific-insufficient", user.Id, valueSub.Id, 7)
	require.Error(t, err)
	require.Contains(t, err.Error(), "subscription quota insufficient")

	regular := createRegularSubscriptionPlanForValuePackageTest(t, "regular specific", 100)
	regularSub := createActiveValuePackageSub(t, user.Id, regular, now-10, now+3600)
	_, err = PreConsumeValuePackageSubscription("regular-specific-preconsume", user.Id, regularSub.Id, 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not value package")
}

func TestPreConsumeValuePackageSubscriptionUsesUserFacingExhaustedMessage(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3403, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	day.TotalAmount = 10
	require.NoError(t, DB.Save(&day).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-10, now+3600)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("amount_used", int64(10)).Error)

	_, err := PreConsumeValuePackageSubscription("vp-exhausted-message", user.Id, sub.Id, 1)

	require.Error(t, err)
	require.Contains(t, err.Error(), "subscription quota insufficient")
	require.Contains(t, err.Error(), ValuePackageQuotaExhaustedUserMessage)
}

func TestValuePackageNormalizeDefaultsUsesCNYCurrency(t *testing.T) {
	plan := SubscriptionPlan{
		PlanKind: SubscriptionPlanKindValuePackage,
		Currency: "USD",
	}

	plan.NormalizeDefaults()

	require.Equal(t, "CNY", plan.Currency)
}

func TestSubscriptionNormalizeDefaultsKeepsStandardPlansUSD(t *testing.T) {
	plan := SubscriptionPlan{
		PlanKind: SubscriptionPlanKindSubscription,
		Currency: "CNY",
	}

	plan.NormalizeDefaults()

	require.Equal(t, "USD", plan.Currency)
}
