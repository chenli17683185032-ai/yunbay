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
	require.NoError(t, db.AutoMigrate(&User{}, &TopUp{}, &SubscriptionPlan{}, &SubscriptionOrder{}, &UserSubscription{}, &UserValuePackagePreference{}, &ValuePackageUsageRecord{}, &ValuePackageQuotaReset{}, &ValuePackageResetCountLedger{}, &SubscriptionPreConsumeRecord{}, &AffiliateCommission{}, &AffiliateWithdrawal{}))

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
	require.True(t, state.Preference.Enabled)
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
	require.True(t, state.Preference.Enabled)
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

func TestDeactivateValuePackageDoesNotGetAutoEnabledOnNextStateLoad(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3008, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-100, now+3600)
	activated, err := ActivateValuePackage(user.Id, sub.Id)
	require.NoError(t, err)
	require.True(t, activated.Preference.Enabled)

	deactivated, err := DeactivateValuePackage(user.Id)
	require.NoError(t, err)
	require.False(t, deactivated.Preference.Enabled)

	state, err := GetValuePackageState(user.Id)

	require.NoError(t, err)
	require.NotNil(t, state)
	require.False(t, state.Preference.Enabled)
	require.Equal(t, sub.Id, state.Preference.ActiveUserSubscriptionId)
	require.NotNil(t, state.Subscription)
	require.Equal(t, sub.Id, state.Subscription.Id)
}

func TestValuePackageRollingUsageWindows(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3007, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-7*3600, now+3600)

	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: ValuePackageTypeDay, ModelGroup: "day-card", RequestId: "old", Quota: 900, CreatedAt: now - int64(6*time.Hour/time.Second)}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: ValuePackageTypeDay, ModelGroup: "day-card", RequestId: "recent", Quota: 900, CreatedAt: now - int64(time.Hour/time.Second)}))

	used5h, used7d, err := GetValuePackageWindowUsage(user.Id, sub.Id, now)
	require.NoError(t, err)
	require.EqualValues(t, 900, used5h)
	require.EqualValues(t, 0, used7d)

	details, err := GetValuePackageWindowUsageDetails(user.Id, sub.Id, now)
	require.NoError(t, err)
	require.NotNil(t, details)
	require.EqualValues(t, 900, details.Used5h)
	require.EqualValues(t, now-3600, details.Earliest5hCreatedAt)
	require.EqualValues(t, now+4*3600, details.ResetAt5h)
	require.EqualValues(t, 4*3600, details.ResetSeconds5h)
	require.EqualValues(t, 0, details.Used7d)
	require.EqualValues(t, 0, details.Earliest7dCreatedAt)
	require.EqualValues(t, 0, details.ResetAt7d)
	require.EqualValues(t, 0, details.ResetSeconds7d)
}

func TestValuePackageAnchoredWindowUsesSubscriptionStartAndClampsToEnd(t *testing.T) {
	start := int64(1_700_000_000)
	end := start + valuePackageMonthSeconds

	first := calcValuePackageAnchoredWindow(start, end, valuePackage7dWindowSeconds, start+3*valuePackageDaySeconds)
	require.EqualValues(t, start, first.Start)
	require.EqualValues(t, start+valuePackage7dWindowSeconds, first.End)

	second := calcValuePackageAnchoredWindow(start, end, valuePackage7dWindowSeconds, start+8*valuePackageDaySeconds)
	require.EqualValues(t, start+valuePackage7dWindowSeconds, second.Start)
	require.EqualValues(t, start+2*valuePackage7dWindowSeconds, second.End)

	shortFinal := calcValuePackageAnchoredWindow(start, end, valuePackage7dWindowSeconds, start+29*valuePackageDaySeconds)
	require.EqualValues(t, start+4*valuePackage7dWindowSeconds, shortFinal.Start)
	require.EqualValues(t, end, shortFinal.End)
}

func TestNormalizeValuePackagePlanUsesFixedDurations(t *testing.T) {
	day := SubscriptionPlan{PlanKind: SubscriptionPlanKindValuePackage, PackageType: ValuePackageTypeDay, DurationUnit: SubscriptionDurationMonth, DurationValue: 99, Limit7dAmount: 123}
	normalizeValuePackagePlan(&day)
	require.Equal(t, SubscriptionDurationDay, day.DurationUnit)
	require.Equal(t, 1, day.DurationValue)
	require.EqualValues(t, 0, day.CustomSeconds)
	require.EqualValues(t, 0, day.Limit7dAmount)

	week := SubscriptionPlan{PlanKind: SubscriptionPlanKindValuePackage, PackageType: ValuePackageTypeWeek, DurationUnit: SubscriptionDurationMonth, DurationValue: 99}
	normalizeValuePackagePlan(&week)
	require.Equal(t, SubscriptionDurationDay, week.DurationUnit)
	require.Equal(t, 7, week.DurationValue)
	require.EqualValues(t, 0, week.CustomSeconds)

	month := SubscriptionPlan{PlanKind: SubscriptionPlanKindValuePackage, PackageType: ValuePackageTypeMonth, DurationUnit: SubscriptionDurationMonth, DurationValue: 1}
	normalizeValuePackagePlan(&month)
	require.Equal(t, SubscriptionDurationDay, month.DurationUnit)
	require.Equal(t, 30, month.DurationValue)
	require.EqualValues(t, 0, month.CustomSeconds)
}

func TestValuePackageWindowUsageAnchors7dToSubscriptionStart(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3018, UserGroupTiyan)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	month.TotalAmount = 10000
	month.Limit5hAmount = 10000
	month.Limit7dAmount = 1000
	require.NoError(t, DB.Save(&month).Error)
	now := common.GetTimestamp()
	start := now - 8*valuePackageDaySeconds
	end := start + valuePackageMonthSeconds
	sub := createActiveValuePackageSub(t, user.Id, month, start, end)
	currentWindowStart := start + valuePackage7dWindowSeconds
	currentWindowEnd := start + 2*valuePackage7dWindowSeconds
	currentUsageAt := currentWindowStart + valuePackageDaySeconds/2

	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ModelGroup: month.ModelGroup, RequestId: "anchored-previous-cycle", Quota: 100, CreatedAt: start + 2*valuePackageDaySeconds}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ModelGroup: month.ModelGroup, RequestId: "anchored-current-cycle", Quota: 200, CreatedAt: currentUsageAt}))

	used5h, used7d, err := GetValuePackageWindowUsage(user.Id, sub.Id, now)
	require.NoError(t, err)
	require.EqualValues(t, 0, used5h)
	require.EqualValues(t, 200, used7d)

	details, err := GetValuePackageWindowUsageDetails(user.Id, sub.Id, now)
	require.NoError(t, err)
	require.NotNil(t, details)
	require.EqualValues(t, 200, details.Used7d)
	require.EqualValues(t, currentUsageAt, details.Earliest7dCreatedAt)
	require.EqualValues(t, currentWindowEnd, details.ResetAt7d)
	require.EqualValues(t, currentWindowEnd-now, details.ResetSeconds7d)
	require.NotEqualValues(t, currentUsageAt+valuePackage7dWindowSeconds, details.ResetAt7d)
}

func TestValuePackageWindowUsageResetScopeByPackageType(t *testing.T) {
	t.Run("week reset clears 5h only", func(t *testing.T) {
		setupValuePackageTestDB(t)
		user := createValuePackageUser(t, 3019, UserGroupTiyan)
		week := createValuePackagePlan(t, ValuePackageTypeWeek, ValuePackageLevelWeek, 7, 9.9)
		week.TotalAmount = 10000
		week.Limit5hAmount = 1000
		week.Limit7dAmount = 1000
		require.NoError(t, DB.Save(&week).Error)
		now := common.GetTimestamp()
		start := now - 2*valuePackageDaySeconds
		end := start + valuePackageWeekSeconds
		sub := createActiveValuePackageSub(t, user.Id, week, start, end)
		resetAt := now - valuePackageDaySeconds/24
		beforeResetAt := resetAt - valuePackageDaySeconds/24
		afterResetAt := resetAt + valuePackageDaySeconds/48

		require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: week.Id, PackageType: week.PackageType, ModelGroup: week.ModelGroup, RequestId: "week-before-reset", Quota: 100, CreatedAt: beforeResetAt}))
		require.NoError(t, DB.Create(&ValuePackageQuotaReset{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: week.Id, PackageType: week.PackageType, ResetAt: resetAt, Source: ValuePackageQuotaResetSourceUserConsumeCount, CreatedByUserId: user.Id}).Error)
		require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: week.Id, PackageType: week.PackageType, ModelGroup: week.ModelGroup, RequestId: "week-after-reset", Quota: 30, CreatedAt: afterResetAt}))

		details, err := GetValuePackageWindowUsageDetails(user.Id, sub.Id, now)
		require.NoError(t, err)
		require.NotNil(t, details)
		require.EqualValues(t, 30, details.Used5h)
		require.EqualValues(t, afterResetAt, details.Earliest5hCreatedAt)
		require.EqualValues(t, afterResetAt+valuePackage5hWindowSeconds, details.ResetAt5h)
		require.EqualValues(t, 130, details.Used7d)
		require.EqualValues(t, beforeResetAt, details.Earliest7dCreatedAt)
		require.EqualValues(t, start+valuePackage7dWindowSeconds, details.ResetAt7d)
		require.EqualValues(t, start+valuePackage7dWindowSeconds-now, details.ResetSeconds7d)
	})

	t.Run("month reset clears 5h and current 7d phase", func(t *testing.T) {
		setupValuePackageTestDB(t)
		user := createValuePackageUser(t, 3029, UserGroupTiyan)
		month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
		month.TotalAmount = 10000
		month.Limit5hAmount = 1000
		month.Limit7dAmount = 1000
		require.NoError(t, DB.Save(&month).Error)
		now := common.GetTimestamp()
		start := now - 8*valuePackageDaySeconds
		end := start + valuePackageMonthSeconds
		sub := createActiveValuePackageSub(t, user.Id, month, start, end)
		currentWindowStart := start + valuePackage7dWindowSeconds
		currentWindowEnd := start + 2*valuePackage7dWindowSeconds
		resetAt := now - valuePackageDaySeconds/24
		beforeResetAt := resetAt - valuePackageDaySeconds/24
		afterResetAt := resetAt + valuePackageDaySeconds/48

		require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ModelGroup: month.ModelGroup, RequestId: "month-previous-cycle", Quota: 500, CreatedAt: start + 2*valuePackageDaySeconds}))
		require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ModelGroup: month.ModelGroup, RequestId: "month-before-reset", Quota: 100, CreatedAt: beforeResetAt}))
		require.Greater(t, beforeResetAt, currentWindowStart)
		require.NoError(t, DB.Create(&ValuePackageQuotaReset{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ResetAt: resetAt, Source: ValuePackageQuotaResetSourceUserConsumeCount, CreatedByUserId: user.Id}).Error)
		require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ModelGroup: month.ModelGroup, RequestId: "month-after-reset", Quota: 30, CreatedAt: afterResetAt}))

		details, err := GetValuePackageWindowUsageDetails(user.Id, sub.Id, now)
		require.NoError(t, err)
		require.NotNil(t, details)
		require.EqualValues(t, 30, details.Used5h)
		require.EqualValues(t, afterResetAt, details.Earliest5hCreatedAt)
		require.EqualValues(t, afterResetAt+valuePackage5hWindowSeconds, details.ResetAt5h)
		require.EqualValues(t, 30, details.Used7d)
		require.EqualValues(t, afterResetAt, details.Earliest7dCreatedAt)
		require.EqualValues(t, currentWindowEnd, details.ResetAt7d)
		require.EqualValues(t, currentWindowEnd-now, details.ResetSeconds7d)
		require.NotEqualValues(t, resetAt+valuePackage7dWindowSeconds, details.ResetAt7d)
	})
}

func TestValuePackageWindowUsageDayCardIgnores7dLimit(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3039, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	day.TotalAmount = 1000
	day.Limit5hAmount = 1000
	day.Limit7dAmount = 50
	require.NoError(t, DB.Save(&day).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-valuePackageDaySeconds/24, now+valuePackageDaySeconds)
	require.NoError(t, DB.Create(&UserValuePackagePreference{UserId: user.Id, Enabled: true, ActiveUserSubscriptionId: sub.Id}).Error)
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "day-ignores-7d", Quota: 80, CreatedAt: now - valuePackageDaySeconds/48}))

	used5h, used7d, err := GetValuePackageWindowUsage(user.Id, sub.Id, now)
	require.NoError(t, err)
	require.EqualValues(t, 80, used5h)
	require.EqualValues(t, 0, used7d)

	details, err := GetValuePackageWindowUsageDetails(user.Id, sub.Id, now)
	require.NoError(t, err)
	require.NotNil(t, details)
	require.EqualValues(t, 80, details.Used5h)
	require.EqualValues(t, 0, details.Used7d)
	require.EqualValues(t, 0, details.Earliest7dCreatedAt)
	require.EqualValues(t, 0, details.ResetAt7d)
	require.EqualValues(t, 0, details.ResetSeconds7d)

	state, err := GetValuePackageState(user.Id)
	require.NoError(t, err)
	require.NotNil(t, state)
	require.NotNil(t, state.Usage)
	require.EqualValues(t, 0, state.Usage.Used7d)
	require.EqualValues(t, 0, state.Usage.Limit7d)
	require.EqualValues(t, 0, state.Usage.Percent7d)
	require.False(t, state.Usage.Limited7d)
	require.EqualValues(t, 0, state.Usage.ResetAt7d)
	require.EqualValues(t, 0, state.Usage.ResetSeconds7d)
}

func TestValuePackageWindowUsageUsesCreatedAtAnchorWhenStartTimeMissing(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3049, UserGroupTiyan)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	month.TotalAmount = 10000
	month.Limit5hAmount = 10000
	month.Limit7dAmount = 1000
	require.NoError(t, DB.Save(&month).Error)
	now := common.GetTimestamp()
	legacyStart := now - 8*valuePackageDaySeconds
	end := legacyStart + valuePackageMonthSeconds
	sub := createActiveValuePackageSub(t, user.Id, month, legacyStart, end)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).UpdateColumns(map[string]any{
		"start_time": int64(0),
		"created_at": legacyStart,
		"updated_at": legacyStart,
	}).Error)
	currentWindowStart := legacyStart + valuePackage7dWindowSeconds
	currentWindowEnd := legacyStart + 2*valuePackage7dWindowSeconds
	currentUsageAt := currentWindowStart + valuePackageDaySeconds/2

	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ModelGroup: month.ModelGroup, RequestId: "legacy-previous-cycle", Quota: 100, CreatedAt: legacyStart + 2*valuePackageDaySeconds}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ModelGroup: month.ModelGroup, RequestId: "legacy-current-cycle", Quota: 200, CreatedAt: currentUsageAt}))

	details, err := GetValuePackageWindowUsageDetails(user.Id, sub.Id, now)
	require.NoError(t, err)
	require.NotNil(t, details)
	require.EqualValues(t, 200, details.Used7d)
	require.EqualValues(t, currentUsageAt, details.Earliest7dCreatedAt)
	require.EqualValues(t, currentWindowEnd, details.ResetAt7d)
	require.EqualValues(t, currentWindowEnd-now, details.ResetSeconds7d)

	result, err := ListValuePackageManagementRows(ValuePackageManagementFilter{Keyword: user.Username, PackageType: "all", Active: "active", Page: 1, PageSize: 20}, now)
	require.NoError(t, err)
	require.EqualValues(t, 1, result.Total)
	require.Len(t, result.Items, 1)
	require.NotNil(t, result.Items[0].Usage)
	require.EqualValues(t, 200, result.Items[0].Usage.Used7d)
	require.EqualValues(t, currentWindowEnd, result.Items[0].Usage.ResetAt7d)
}

func TestValuePackageWindowUsageDetailsDoesNotExtendResetWithLaterUsage(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3010, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-5*3600, now+3600)

	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "earliest-window-usage", Quota: 50, CreatedAt: now - 4*3600}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "later-small-usage", Quota: 1, CreatedAt: now - 2*3600}))

	details, err := GetValuePackageWindowUsageDetails(user.Id, sub.Id, now)

	require.NoError(t, err)
	require.NotNil(t, details)
	require.EqualValues(t, 51, details.Used5h)
	require.EqualValues(t, now-4*3600, details.Earliest5hCreatedAt)
	require.EqualValues(t, now+3600, details.ResetAt5h)
	require.EqualValues(t, 3600, details.ResetSeconds5h)
	require.EqualValues(t, 0, details.Used7d)
	require.EqualValues(t, 0, details.Earliest7dCreatedAt)
	require.EqualValues(t, 0, details.ResetAt7d)
	require.EqualValues(t, 0, details.ResetSeconds7d)
}

func TestValuePackageFiveHourWindowClearsAllUsageAtFirstUsageExpiry(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3016, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-8*3600, now+3600)
	windowStart := now - 5*3600

	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "fixed-window-first", Quota: 80, CreatedAt: windowStart}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "fixed-window-later", Quota: 15, CreatedAt: windowStart + 4*3600}))

	details, err := GetValuePackageWindowUsageDetails(user.Id, sub.Id, now)

	require.NoError(t, err)
	require.NotNil(t, details)
	require.EqualValues(t, 0, details.Used5h)
	require.EqualValues(t, 0, details.Earliest5hCreatedAt)
	require.EqualValues(t, 0, details.ResetAt5h)
	require.EqualValues(t, 0, details.ResetSeconds5h)
	require.EqualValues(t, 0, details.Used7d)
}

func TestValuePackageFiveHourWindowRestartsFromNextUsageAfterClear(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3017, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-8*3600, now+3600)
	expiredWindowStart := now - 6*3600
	nextWindowStart := now - 30*60

	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "expired-window-first", Quota: 80, CreatedAt: expiredWindowStart}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "expired-window-later", Quota: 15, CreatedAt: expiredWindowStart + 4*3600}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "next-window-first", Quota: 12, CreatedAt: nextWindowStart}))

	details, err := GetValuePackageWindowUsageDetails(user.Id, sub.Id, now)

	require.NoError(t, err)
	require.NotNil(t, details)
	require.EqualValues(t, 12, details.Used5h)
	require.EqualValues(t, nextWindowStart, details.Earliest5hCreatedAt)
	require.EqualValues(t, nextWindowStart+5*3600, details.ResetAt5h)
	require.EqualValues(t, 270*60, details.ResetSeconds5h)
	require.EqualValues(t, 0, details.Used7d)
}

func TestValuePackageWindowUsageDetailsIgnoresZeroQuotaForReset(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3009, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-5*3600, now+3600)

	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "positive-usage", Quota: 100, CreatedAt: now - 4*3600}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "zero-usage", Quota: 0, CreatedAt: now - 3600}))

	details, err := GetValuePackageWindowUsageDetails(user.Id, sub.Id, now)

	require.NoError(t, err)
	require.NotNil(t, details)
	require.EqualValues(t, 100, details.Used5h)
	require.EqualValues(t, now-4*3600, details.Earliest5hCreatedAt)
	require.EqualValues(t, now+3600, details.ResetAt5h)
	require.EqualValues(t, 3600, details.ResetSeconds5h)
	require.EqualValues(t, 0, details.Used7d)
	require.EqualValues(t, 0, details.Earliest7dCreatedAt)
	require.EqualValues(t, 0, details.ResetAt7d)
	require.EqualValues(t, 0, details.ResetSeconds7d)
}

func TestValuePackageWindowUsageCountsUsageAfterLastReset(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3011, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	resetAt := now - 1800
	sub := createActiveValuePackageSub(t, user.Id, day, now-7200, now+3600)

	require.NoError(t, DB.Create(&ValuePackageQuotaReset{
		UserId:             user.Id,
		UserSubscriptionId: sub.Id,
		PlanId:             day.Id,
		PackageType:        day.PackageType,
		ResetAt:            resetAt,
		Source:             ValuePackageQuotaResetSourceUserConsumeCount,
		CreatedByUserId:    user.Id,
		Note:               "test reset lower bound",
	}).Error)
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "before-last-reset", Quota: 75, CreatedAt: now - 3600}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "after-last-reset", Quota: 25, CreatedAt: now - 900}))

	details, err := GetValuePackageWindowUsageDetails(user.Id, sub.Id, now)

	require.NoError(t, err)
	require.NotNil(t, details)
	require.EqualValues(t, 25, details.Used5h)
	require.EqualValues(t, now-900, details.Earliest5hCreatedAt)
	require.EqualValues(t, now-900+5*3600, details.ResetAt5h)
	require.EqualValues(t, 0, details.Used7d)
	require.EqualValues(t, 0, details.Earliest7dCreatedAt)
	require.EqualValues(t, 0, details.ResetAt7d)
	require.EqualValues(t, 0, details.ResetSeconds7d)
}

func TestValuePackageWindowUsageIgnoresFutureResetEvents(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3014, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-7200, now+3600)

	require.NoError(t, DB.Create(&ValuePackageQuotaReset{
		UserId:             user.Id,
		UserSubscriptionId: sub.Id,
		PlanId:             day.Id,
		PackageType:        day.PackageType,
		ResetAt:            now + 3600,
		Source:             ValuePackageQuotaResetSourceUserConsumeCount,
		CreatedByUserId:    user.Id,
		Note:               "future reset must not clear current window",
	}).Error)
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "current-usage-before-future-reset", Quota: 40, CreatedAt: now - 900}))

	details, err := GetValuePackageWindowUsageDetails(user.Id, sub.Id, now)

	require.NoError(t, err)
	require.NotNil(t, details)
	require.EqualValues(t, 40, details.Used5h)
	require.EqualValues(t, now-900, details.Earliest5hCreatedAt)
	require.EqualValues(t, 0, details.Used7d)
	require.EqualValues(t, 0, details.Earliest7dCreatedAt)
	require.EqualValues(t, 0, details.ResetAt7d)
	require.EqualValues(t, 0, details.ResetSeconds7d)
}

func TestConsumeValuePackageResetCountClampsFutureResetAtToNow(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3015, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-100, now+3600)
	require.NoError(t, DB.Create(&UserValuePackagePreference{UserId: user.Id, Enabled: true, ActiveUserSubscriptionId: sub.Id, ResetCount: 1}).Error)

	state, err := ConsumeValuePackageResetCount(user.Id, sub.Id, now+3600, user.Id, "future reset clamp")

	require.NoError(t, err)
	require.NotNil(t, state)
	var reset ValuePackageQuotaReset
	require.NoError(t, DB.Where("user_id = ? AND user_subscription_id = ?", user.Id, sub.Id).First(&reset).Error)
	require.LessOrEqual(t, reset.ResetAt, common.GetTimestamp())
	require.GreaterOrEqual(t, reset.ResetAt, now)
}

func TestConsumeValuePackageResetCountResetsShortWindowsOnly(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3012, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	day.TotalAmount = 1000
	day.Limit5hAmount = 100
	day.Limit7dAmount = 100
	require.NoError(t, DB.Save(&day).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-7200, now+3600)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("amount_used", int64(300)).Error)
	require.NoError(t, DB.Create(&UserValuePackagePreference{
		UserId:                   user.Id,
		Enabled:                  true,
		ActiveUserSubscriptionId: sub.Id,
		ResetCount:               1,
	}).Error)
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "reset-count-history-1", Quota: 200, CreatedAt: now - 3600}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "reset-count-history-2", Quota: 100, CreatedAt: now - 2*3600}))

	state, err := ConsumeValuePackageResetCount(user.Id, sub.Id, now, user.Id, "test reset")

	require.NoError(t, err)
	require.NotNil(t, state)
	require.Equal(t, 0, state.Preference.ResetCount)
	require.NotNil(t, state.Usage)
	require.EqualValues(t, 0, state.Usage.Used5h)
	require.EqualValues(t, 0, state.Usage.Used7d)
	require.EqualValues(t, 300, state.Usage.TotalUsed)

	var pref UserValuePackagePreference
	require.NoError(t, DB.Where("user_id = ?", user.Id).First(&pref).Error)
	require.Equal(t, 0, pref.ResetCount)

	var resets []ValuePackageQuotaReset
	require.NoError(t, DB.Where("user_id = ? AND user_subscription_id = ?", user.Id, sub.Id).Order("id asc").Find(&resets).Error)
	require.Len(t, resets, 1)
	require.Equal(t, ValuePackageQuotaResetSourceUserConsumeCount, resets[0].Source)
	require.EqualValues(t, now, resets[0].ResetAt)

	var ledgers []ValuePackageResetCountLedger
	require.NoError(t, DB.Where("user_id = ?", user.Id).Order("id asc").Find(&ledgers).Error)
	require.Len(t, ledgers, 1)
	require.Equal(t, -1, ledgers[0].Delta)
	require.Equal(t, 1, ledgers[0].BeforeCount)
	require.Equal(t, 0, ledgers[0].AfterCount)
	require.Equal(t, ValuePackageResetCountLedgerSourceUserConsume, ledgers[0].Source)

	var usageRecordCount int64
	require.NoError(t, DB.Model(&ValuePackageUsageRecord{}).Where("user_subscription_id = ?", sub.Id).Count(&usageRecordCount).Error)
	require.EqualValues(t, 2, usageRecordCount)
	var reloadedSub UserSubscription
	require.NoError(t, DB.First(&reloadedSub, sub.Id).Error)
	require.EqualValues(t, 300, reloadedSub.AmountUsed)
	require.EqualValues(t, day.TotalAmount, reloadedSub.AmountTotal)
	require.EqualValues(t, sub.EndTime, reloadedSub.EndTime)
}

func TestAdjustValuePackageResetCountSupportsSetAddSubtract(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3013, UserGroupTiyan)
	adminUserId := 9001

	setAdjustment, err := AdjustValuePackageResetCount(user.Id, ValuePackageResetCountAdjustModeSet, 3, "set reset count", adminUserId)
	require.NoError(t, err)
	require.Equal(t, 0, setAdjustment.OldCount)
	require.Equal(t, 3, setAdjustment.NewCount)
	require.Equal(t, 3, setAdjustment.Delta)

	addAdjustment, err := AdjustValuePackageResetCount(user.Id, ValuePackageResetCountAdjustModeAdd, 2, "add reset count", adminUserId)
	require.NoError(t, err)
	require.Equal(t, 3, addAdjustment.OldCount)
	require.Equal(t, 5, addAdjustment.NewCount)
	require.Equal(t, 2, addAdjustment.Delta)

	subtractAdjustment, err := AdjustValuePackageResetCount(user.Id, ValuePackageResetCountAdjustModeSubtract, 10, "subtract reset count", adminUserId)
	require.NoError(t, err)
	require.Equal(t, 5, subtractAdjustment.OldCount)
	require.Equal(t, 0, subtractAdjustment.NewCount)
	require.Equal(t, -5, subtractAdjustment.Delta)

	var pref UserValuePackagePreference
	require.NoError(t, DB.Where("user_id = ?", user.Id).First(&pref).Error)
	require.Equal(t, 0, pref.ResetCount)

	var ledgers []ValuePackageResetCountLedger
	require.NoError(t, DB.Where("user_id = ?", user.Id).Order("id asc").Find(&ledgers).Error)
	require.Len(t, ledgers, 3)
	require.Equal(t, []string{
		ValuePackageResetCountLedgerSourceAdminSet,
		ValuePackageResetCountLedgerSourceAdminAdd,
		ValuePackageResetCountLedgerSourceAdminSubtract,
	}, []string{ledgers[0].Source, ledgers[1].Source, ledgers[2].Source})
	require.Equal(t, []int{3, 2, -5}, []int{ledgers[0].Delta, ledgers[1].Delta, ledgers[2].Delta})
	require.Equal(t, []int{0, 3, 5}, []int{ledgers[0].BeforeCount, ledgers[1].BeforeCount, ledgers[2].BeforeCount})
	require.Equal(t, []int{3, 5, 0}, []int{ledgers[0].AfterCount, ledgers[1].AfterCount, ledgers[2].AfterCount})
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
	require.EqualValues(t, now+5*3600, state.Usage.ResetAt5h)
	require.InDelta(t, 5*3600, state.Usage.ResetSeconds5h, 2)
	require.False(t, state.Usage.Limited5h)
	require.EqualValues(t, 0, state.Usage.Used7d)
	require.EqualValues(t, 0, state.Usage.Limit7d)
	require.EqualValues(t, 0, state.Usage.ResetAt7d)
	require.EqualValues(t, 0, state.Usage.ResetSeconds7d)
	require.False(t, state.Usage.Limited7d)
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
	require.EqualValues(t, now+5*3600, state.Usage.ResetAt5h)
	require.InDelta(t, 5*3600, state.Usage.ResetSeconds5h, 2)
	require.False(t, state.Usage.Limited5h)
	require.EqualValues(t, 0, state.Usage.Used7d)
	require.EqualValues(t, 0, state.Usage.Limit7d)
	require.EqualValues(t, 0, state.Usage.Percent7d)
	require.EqualValues(t, 0, state.Usage.ResetAt7d)
	require.EqualValues(t, 0, state.Usage.ResetSeconds7d)
	require.False(t, state.Usage.Limited7d)
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

func TestValuePackageUsageSummaryResetFieldsForEmptyAndUnlimitedWindows(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3410, UserGroupVIP)
	plan := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	plan.TotalAmount = 1000
	plan.Limit5hAmount = 0
	plan.Limit7dAmount = 0
	require.NoError(t, DB.Save(&plan).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, plan, now-10, now+86400)

	state, err := ActivateValuePackage(user.Id, sub.Id)

	require.NoError(t, err)
	require.NotNil(t, state)
	require.NotNil(t, state.Usage)
	require.EqualValues(t, 0, state.Usage.Used5h)
	require.EqualValues(t, 0, state.Usage.Limit5h)
	require.EqualValues(t, 0, state.Usage.ResetAt5h)
	require.EqualValues(t, 0, state.Usage.ResetSeconds5h)
	require.False(t, state.Usage.Limited5h)
	require.EqualValues(t, 0, state.Usage.Used7d)
	require.EqualValues(t, 0, state.Usage.Limit7d)
	require.EqualValues(t, 0, state.Usage.ResetAt7d)
	require.EqualValues(t, 0, state.Usage.ResetSeconds7d)
	require.False(t, state.Usage.Limited7d)
	require.False(t, state.Usage.Exhausted)
	require.Empty(t, state.Usage.ExhaustedReason)
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

func TestCompleteValuePackageOrderCreatesEnabledPreferenceByDefault(t *testing.T) {
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
	require.True(t, state.Preference.Enabled)
	require.Equal(t, completed.Id, state.Preference.ActiveUserSubscriptionId)
	require.NotNil(t, state.Subscription)
	require.Equal(t, completed.Id, state.Subscription.Id)
	require.NotNil(t, state.Plan)
	require.Equal(t, day.Id, state.Plan.Id)
}

func TestGetValuePackageStateAutoEnablesExistingActivePackageWithoutPreference(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3114, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-10, now+3600)

	state, err := GetValuePackageState(user.Id)

	require.NoError(t, err)
	require.NotNil(t, state)
	require.True(t, state.Preference.Enabled)
	require.Equal(t, sub.Id, state.Preference.ActiveUserSubscriptionId)
	require.NotNil(t, state.Subscription)
	require.Equal(t, sub.Id, state.Subscription.Id)
	require.NotNil(t, state.Plan)
	require.Equal(t, day.Id, state.Plan.Id)

	var pref UserValuePackagePreference
	require.NoError(t, DB.Where("user_id = ?", user.Id).First(&pref).Error)
	require.True(t, pref.Enabled)
	require.Equal(t, sub.Id, pref.ActiveUserSubscriptionId)
}

func TestGetValuePackageStateAutoEnablesLegacyDefaultDisabledPreference(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3116, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-10, now+3600)
	legacyCreatedAt := now - 100
	require.NoError(t, DB.Create(&UserValuePackagePreference{UserId: user.Id, Enabled: false, ActiveUserSubscriptionId: sub.Id}).Error)
	require.NoError(t, DB.Model(&UserValuePackagePreference{}).Where("user_id = ?", user.Id).Updates(map[string]any{
		"created_at": legacyCreatedAt,
		"updated_at": legacyCreatedAt,
	}).Error)

	state, err := GetValuePackageState(user.Id)

	require.NoError(t, err)
	require.NotNil(t, state)
	require.True(t, state.Preference.Enabled)
	require.Equal(t, sub.Id, state.Preference.ActiveUserSubscriptionId)
	require.NotNil(t, state.Subscription)
	require.Equal(t, sub.Id, state.Subscription.Id)
	require.NotNil(t, state.Plan)
	require.Equal(t, day.Id, state.Plan.Id)

	var pref UserValuePackagePreference
	require.NoError(t, DB.Where("user_id = ?", user.Id).First(&pref).Error)
	require.True(t, pref.Enabled)
	require.Equal(t, sub.Id, pref.ActiveUserSubscriptionId)
}

func TestGetValuePackageStateDoesNotAutoEnableManuallyDisabledPackage(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3115, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-10, now+3600)
	legacyCreatedAt := now - 100
	manualDisabledAt := now
	require.NoError(t, DB.Create(&UserValuePackagePreference{UserId: user.Id, Enabled: false, ActiveUserSubscriptionId: sub.Id}).Error)
	require.NoError(t, DB.Model(&UserValuePackagePreference{}).Where("user_id = ?", user.Id).Updates(map[string]any{
		"created_at": legacyCreatedAt,
		"updated_at": manualDisabledAt,
	}).Error)

	state, err := GetValuePackageState(user.Id)

	require.NoError(t, err)
	require.NotNil(t, state)
	require.False(t, state.Preference.Enabled)
	require.Equal(t, sub.Id, state.Preference.ActiveUserSubscriptionId)
	require.NotNil(t, state.Subscription)
	require.Equal(t, sub.Id, state.Subscription.Id)
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
	require.NotNil(t, state.Billing)
	require.True(t, state.Billing.Active)
	require.Equal(t, day.ModelGroup, state.Billing.PackageGroup)
	require.Equal(t, ValuePackageEffectiveBillingRatio, state.Billing.EffectiveRatio)
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
	require.EqualValues(t, 0, used7d)
}

func TestPreConsumeValuePackageSubscriptionIgnoresDayCard7dLimit(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3320, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	day.TotalAmount = 1000
	day.Limit5hAmount = 1000
	day.Limit7dAmount = 1
	require.NoError(t, DB.Save(&day).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-10, now+3600)

	result, err := PreConsumeValuePackageSubscription("preconsume-day-ignores-7d", user.Id, sub.Id, 10)

	require.NoError(t, err)
	require.Equal(t, sub.Id, result.UserSubscriptionId)
	require.EqualValues(t, 10, result.PreConsumed)
	require.EqualValues(t, 0, result.AmountUsedBefore)
	require.EqualValues(t, 10, result.AmountUsedAfter)
	used5h, used7d, err := GetValuePackageWindowUsage(user.Id, sub.Id, common.GetTimestamp())
	require.NoError(t, err)
	require.EqualValues(t, 10, used5h)
	require.EqualValues(t, 0, used7d)
}

func TestPreConsumeValuePackageSubscriptionWeekResetDoesNotClear7dLimit(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3321, UserGroupTiyan)
	week := createValuePackagePlan(t, ValuePackageTypeWeek, ValuePackageLevelWeek, 7, 9.9)
	week.TotalAmount = 1000
	week.Limit5hAmount = 1000
	week.Limit7dAmount = 50
	require.NoError(t, DB.Save(&week).Error)
	now := common.GetTimestamp()
	start := now - 2*valuePackageDaySeconds
	sub := createActiveValuePackageSub(t, user.Id, week, start, start+valuePackageWeekSeconds)
	resetAt := now - 30*60
	beforeResetAt := resetAt - 30*60
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: week.Id, PackageType: week.PackageType, ModelGroup: week.ModelGroup, RequestId: "preconsume-week-before-reset", Quota: 45, CreatedAt: beforeResetAt}))
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("amount_used", int64(45)).Error)
	require.NoError(t, DB.Create(&ValuePackageQuotaReset{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: week.Id, PackageType: week.PackageType, ResetAt: resetAt, Source: ValuePackageQuotaResetSourceUserConsumeCount, CreatedByUserId: user.Id}).Error)

	_, err := PreConsumeValuePackageSubscription("preconsume-week-reset-keeps-7d", user.Id, sub.Id, 10)

	require.Error(t, err)
	require.Contains(t, err.Error(), "7d period limit exceeded")
	require.NotContains(t, err.Error(), "rolling")
	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, sub.Id).Error)
	require.EqualValues(t, 45, reloaded.AmountUsed)
	var failedCount int64
	require.NoError(t, DB.Model(&ValuePackageUsageRecord{}).Where("request_id = ?", "preconsume-week-reset-keeps-7d").Count(&failedCount).Error)
	require.EqualValues(t, 0, failedCount)
}

func TestPreConsumeValuePackageSubscriptionMonthResetClearsCurrent7dLimit(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3322, UserGroupTiyan)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	month.TotalAmount = 1000
	month.Limit5hAmount = 1000
	month.Limit7dAmount = 50
	require.NoError(t, DB.Save(&month).Error)
	now := common.GetTimestamp()
	start := now - 2*valuePackageDaySeconds
	sub := createActiveValuePackageSub(t, user.Id, month, start, start+valuePackageMonthSeconds)
	resetAt := now - 30*60
	beforeResetAt := resetAt - 30*60
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ModelGroup: month.ModelGroup, RequestId: "preconsume-month-before-reset", Quota: 45, CreatedAt: beforeResetAt}))
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("amount_used", int64(45)).Error)
	require.NoError(t, DB.Create(&ValuePackageQuotaReset{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ResetAt: resetAt, Source: ValuePackageQuotaResetSourceUserConsumeCount, CreatedByUserId: user.Id}).Error)

	result, err := PreConsumeValuePackageSubscription("preconsume-month-reset-clears-7d", user.Id, sub.Id, 10)

	require.NoError(t, err)
	require.Equal(t, sub.Id, result.UserSubscriptionId)
	require.EqualValues(t, 10, result.PreConsumed)
	require.EqualValues(t, 45, result.AmountUsedBefore)
	require.EqualValues(t, 55, result.AmountUsedAfter)
	used5h, used7d, err := GetValuePackageWindowUsage(user.Id, sub.Id, common.GetTimestamp())
	require.NoError(t, err)
	require.EqualValues(t, 10, used5h)
	require.EqualValues(t, 10, used7d)
}

func TestPreConsumeValuePackageSubscriptionDoesNotUseRolling7dWindow(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3323, UserGroupTiyan)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	month.TotalAmount = 1000
	month.Limit5hAmount = 1000
	month.Limit7dAmount = 50
	require.NoError(t, DB.Save(&month).Error)
	now := common.GetTimestamp()
	start := now - 8*valuePackageDaySeconds
	sub := createActiveValuePackageSub(t, user.Id, month, start, start+valuePackageMonthSeconds)
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ModelGroup: month.ModelGroup, RequestId: "preconsume-previous-anchored-7d", Quota: 45, CreatedAt: now - 6*valuePackageDaySeconds}))
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("amount_used", int64(45)).Error)

	result, err := PreConsumeValuePackageSubscription("preconsume-anchored-not-rolling", user.Id, sub.Id, 10)

	require.NoError(t, err)
	require.Equal(t, sub.Id, result.UserSubscriptionId)
	require.EqualValues(t, 10, result.PreConsumed)
	require.EqualValues(t, 45, result.AmountUsedBefore)
	require.EqualValues(t, 55, result.AmountUsedAfter)
	used5h, used7d, err := GetValuePackageWindowUsage(user.Id, sub.Id, common.GetTimestamp())
	require.NoError(t, err)
	require.EqualValues(t, 10, used5h)
	require.EqualValues(t, 10, used7d)
}

func TestPreConsumeValuePackageSubscriptionAllowsAfterFixedFiveHourWindowExpires(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3312, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	day.TotalAmount = 1000
	day.Limit5hAmount = 100
	day.Limit7dAmount = 500
	require.NoError(t, DB.Save(&day).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-8*3600, now+3600)
	windowStart := now - 5*3600
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "preconsume-fixed-window-first", Quota: 90, CreatedAt: windowStart}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "preconsume-fixed-window-later", Quota: 10, CreatedAt: windowStart + 4*3600}))

	result, err := PreConsumeValuePackageSubscription("preconsume-next-window", user.Id, sub.Id, 20)

	require.NoError(t, err)
	require.Equal(t, sub.Id, result.UserSubscriptionId)
	require.EqualValues(t, 20, result.PreConsumed)
	used5h, used7d, err := GetValuePackageWindowUsage(user.Id, sub.Id, common.GetTimestamp())
	require.NoError(t, err)
	require.EqualValues(t, 20, used5h)
	require.EqualValues(t, 0, used7d)
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
	require.EqualValues(t, 0, used7d)
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
	activeSub := createActiveValuePackageSub(t, activeUser.Id, day, now-7*3600, now+86400)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", activeSub.Id).Updates(map[string]any{"amount_used": int64(700)}).Error)
	require.NoError(t, DB.Create(&UserValuePackagePreference{UserId: activeUser.Id, Enabled: true, ActiveUserSubscriptionId: activeSub.Id}).Error)
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: activeUser.Id, UserSubscriptionId: activeSub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "active-5h", Quota: 100, CreatedAt: now - 3600}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: activeUser.Id, UserSubscriptionId: activeSub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "active-7d", Quota: 200, CreatedAt: now - 6*3600}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: activeUser.Id, UserSubscriptionId: activeSub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "active-old", Quota: 999, CreatedAt: now - 8*24*3600}))

	disabledUser := createValuePackageUser(t, 3502, UserGroupTiyan)
	disabledSub := createActiveValuePackageSub(t, disabledUser.Id, week, now-100, now+86400)
	require.NoError(t, DB.Create(&UserValuePackagePreference{UserId: disabledUser.Id, Enabled: false, ActiveUserSubscriptionId: disabledSub.Id}).Error)
	require.NoError(t, DB.Model(&UserValuePackagePreference{}).Where("user_id = ?", disabledUser.Id).Updates(map[string]any{
		"created_at": now - 100,
		"updated_at": now,
	}).Error)

	otherActiveUser := createValuePackageUser(t, 3503, UserGroupTiyan)
	otherActiveSub := createActiveValuePackageSub(t, otherActiveUser.Id, month, now-100, now+86400)
	require.NoError(t, DB.Create(&UserValuePackagePreference{UserId: otherActiveUser.Id, Enabled: true, ActiveUserSubscriptionId: activeSub.Id}).Error)
	_ = otherActiveSub

	expiredUser := createValuePackageUser(t, 3504, UserGroupTiyan)
	expiredSub := createActiveValuePackageSub(t, expiredUser.Id, day, now-86400, now-1)
	require.NoError(t, DB.Create(&UserValuePackagePreference{UserId: expiredUser.Id, Enabled: true, ActiveUserSubscriptionId: expiredSub.Id}).Error)

	noPrefUser := createValuePackageUser(t, 3505, UserGroupTiyan)
	noPrefSub := createActiveValuePackageSub(t, noPrefUser.Id, week, now-100, now+86400)

	legacyDefaultUser := createValuePackageUser(t, 3506, UserGroupTiyan)
	legacyDefaultSub := createActiveValuePackageSub(t, legacyDefaultUser.Id, month, now-100, now+86400)
	require.NoError(t, DB.Create(&UserValuePackagePreference{UserId: legacyDefaultUser.Id, Enabled: false, ActiveUserSubscriptionId: legacyDefaultSub.Id}).Error)
	require.NoError(t, DB.Model(&UserValuePackagePreference{}).Where("user_id = ?", legacyDefaultUser.Id).Updates(map[string]any{
		"created_at": now - 100,
		"updated_at": now - 100,
	}).Error)

	rows, err := ListActiveValuePackageUsageRows(now)

	require.NoError(t, err)
	require.Len(t, rows, 3)
	row := rows[0]
	require.Equal(t, activeUser.Id, row.UserId)
	require.Equal(t, activeUser.Username, row.Username)
	require.Equal(t, activeSub.Id, row.Subscription.Id)
	require.Equal(t, day.Id, row.Plan.Id)
	require.NotNil(t, row.Usage)
	require.EqualValues(t, 100, row.Usage.Used5h)
	require.EqualValues(t, 0, row.Usage.Used7d)
	require.EqualValues(t, 1000, row.Usage.Limit5h)
	require.EqualValues(t, 0, row.Usage.Limit7d)
	require.EqualValues(t, 700, row.Usage.TotalUsed)
	require.EqualValues(t, 9300, row.Usage.TotalRemaining)

	require.Equal(t, noPrefUser.Id, rows[1].UserId)
	require.Equal(t, noPrefSub.Id, rows[1].Subscription.Id)
	require.Equal(t, week.Id, rows[1].Plan.Id)

	require.Equal(t, legacyDefaultUser.Id, rows[2].UserId)
	require.Equal(t, legacyDefaultSub.Id, rows[2].Subscription.Id)
	require.Equal(t, month.Id, rows[2].Plan.Id)

	var noPref UserValuePackagePreference
	require.NoError(t, DB.Where("user_id = ?", noPrefUser.Id).First(&noPref).Error)
	require.True(t, noPref.Enabled)
	require.Equal(t, noPrefSub.Id, noPref.ActiveUserSubscriptionId)
	var legacyDefault UserValuePackagePreference
	require.NoError(t, DB.Where("user_id = ?", legacyDefaultUser.Id).First(&legacyDefault).Error)
	require.True(t, legacyDefault.Enabled)
	require.Equal(t, legacyDefaultSub.Id, legacyDefault.ActiveUserSubscriptionId)
}

func TestListValuePackageManagementRowsIncludesResetCountAndUsage(t *testing.T) {
	setupValuePackageTestDB(t)
	now := common.GetTimestamp()
	plan := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	plan.TotalAmount = 1000
	plan.Limit5hAmount = 100
	plan.Limit7dAmount = 500
	require.NoError(t, DB.Save(&plan).Error)
	user := createValuePackageUser(t, 3020, UserGroupTiyan)
	sub := createActiveValuePackageSub(t, user.Id, plan, now-3600, now+86400)
	require.NoError(t, DB.Create(&UserValuePackagePreference{UserId: user.Id, Enabled: true, ActiveUserSubscriptionId: sub.Id, ResetCount: 4}).Error)
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "mgmt-usage", Quota: 25, CreatedAt: now - 1800}))

	result, err := ListValuePackageManagementRows(ValuePackageManagementFilter{Keyword: user.Username, PackageType: "all", Active: "active", Page: 1, PageSize: 20}, now)

	require.NoError(t, err)
	require.EqualValues(t, 1, result.Total)
	require.Len(t, result.Items, 1)
	row := result.Items[0]
	require.Equal(t, user.Id, row.UserId)
	require.Equal(t, user.Username, row.Username)
	require.EqualValues(t, 4, row.ResetCount)
	require.Equal(t, plan.PackageType, row.PackageType)
	require.Equal(t, plan.Title, row.PlanTitle)
	require.Equal(t, sub.Id, row.SubscriptionId)
	require.True(t, row.Enabled)
	require.NotNil(t, row.Usage)
	require.EqualValues(t, 25, row.Usage.Used5h)
}

func TestListValuePackageManagementRowsUsesFixedFiveHourWindow(t *testing.T) {
	setupValuePackageTestDB(t)
	now := common.GetTimestamp()
	plan := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	plan.Limit5hAmount = 100
	plan.Limit7dAmount = 500
	require.NoError(t, DB.Save(&plan).Error)
	user := createValuePackageUser(t, 3021, UserGroupTiyan)
	sub := createActiveValuePackageSub(t, user.Id, plan, now-8*3600, now+86400)
	require.NoError(t, DB.Create(&UserValuePackagePreference{UserId: user.Id, Enabled: true, ActiveUserSubscriptionId: sub.Id, ResetCount: 1}).Error)
	windowStart := now - 5*3600
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "mgmt-fixed-window-first", Quota: 80, CreatedAt: windowStart}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "mgmt-fixed-window-later", Quota: 15, CreatedAt: windowStart + 4*3600}))

	result, err := ListValuePackageManagementRows(ValuePackageManagementFilter{Keyword: user.Username, PackageType: "all", Active: "active", Page: 1, PageSize: 20}, now)

	require.NoError(t, err)
	require.EqualValues(t, 1, result.Total)
	require.Len(t, result.Items, 1)
	require.NotNil(t, result.Items[0].Usage)
	require.EqualValues(t, 0, result.Items[0].Usage.Used5h)
	require.EqualValues(t, 0, result.Items[0].Usage.ResetSeconds5h)
	require.EqualValues(t, 0, result.Items[0].Usage.Used7d)
}

func TestListValuePackageManagementRowsFiltersAndPaginatesInDatabaseSemantics(t *testing.T) {
	setupValuePackageTestDB(t)
	now := common.GetTimestamp()
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	week := createValuePackagePlan(t, ValuePackageTypeWeek, ValuePackageLevelWeek, 7, 9.9)
	regular := createRegularSubscriptionPlanForValuePackageTest(t, "regular", 1000)

	matched := createValuePackageUser(t, 3030, UserGroupTiyan)
	matched.DisplayName = "Needle Display"
	require.NoError(t, DB.Save(&matched).Error)
	matchedSub := createActiveValuePackageSub(t, matched.Id, week, now-100, now+86400)
	require.NoError(t, DB.Create(&UserValuePackagePreference{UserId: matched.Id, Enabled: true, ActiveUserSubscriptionId: matchedSub.Id, ResetCount: 2}).Error)

	otherWeek := createValuePackageUser(t, 3031, UserGroupTiyan)
	otherWeekSub := createActiveValuePackageSub(t, otherWeek.Id, week, now-90, now+86400)
	require.NoError(t, DB.Create(&UserValuePackagePreference{UserId: otherWeek.Id, Enabled: true, ActiveUserSubscriptionId: otherWeekSub.Id, ResetCount: 1}).Error)

	dayUser := createValuePackageUser(t, 3032, UserGroupTiyan)
	createActiveValuePackageSub(t, dayUser.Id, day, now-80, now+86400)
	regularUser := createValuePackageUser(t, 3033, UserGroupTiyan)
	createActiveValuePackageSub(t, regularUser.Id, regular, now-70, now+86400)

	byPackage, err := ListValuePackageManagementRows(ValuePackageManagementFilter{PackageType: ValuePackageTypeWeek, Active: "active", Page: 1, PageSize: 1}, now)
	require.NoError(t, err)
	require.EqualValues(t, 2, byPackage.Total)
	require.Len(t, byPackage.Items, 1)
	require.Equal(t, otherWeek.Id, byPackage.Items[0].UserId)

	secondPage, err := ListValuePackageManagementRows(ValuePackageManagementFilter{PackageType: ValuePackageTypeWeek, Active: "active", Page: 2, PageSize: 1}, now)
	require.NoError(t, err)
	require.EqualValues(t, 2, secondPage.Total)
	require.Len(t, secondPage.Items, 1)
	require.Equal(t, matched.Id, secondPage.Items[0].UserId)

	byKeyword, err := ListValuePackageManagementRows(ValuePackageManagementFilter{Keyword: "needle", PackageType: "all", Active: "active", Page: 1, PageSize: 20}, now)
	require.NoError(t, err)
	require.EqualValues(t, 1, byKeyword.Total)
	require.Len(t, byKeyword.Items, 1)
	require.Equal(t, matched.Id, byKeyword.Items[0].UserId)

	byUserId, err := ListValuePackageManagementRows(ValuePackageManagementFilter{Keyword: fmt.Sprintf("%d", otherWeek.Id), PackageType: "all", Active: "active", Page: 1, PageSize: 20}, now)
	require.NoError(t, err)
	require.EqualValues(t, 1, byUserId.Total)
	require.Len(t, byUserId.Items, 1)
	require.Equal(t, otherWeek.Id, byUserId.Items[0].UserId)
}

func TestListValuePackageManagementRowsStatusFiltersExcludeCancelledAndCovered(t *testing.T) {
	setupValuePackageTestDB(t)
	now := common.GetTimestamp()
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)

	expiredUser := createValuePackageUser(t, 3040, UserGroupTiyan)
	expiredSub := createActiveValuePackageSub(t, expiredUser.Id, day, now-86400, now-10)
	activeExpiredUser := createValuePackageUser(t, 3041, UserGroupTiyan)
	activeExpiredSub := createActiveValuePackageSub(t, activeExpiredUser.Id, day, now-86400, now-1)
	cancelledUser := createValuePackageUser(t, 3042, UserGroupTiyan)
	cancelledSub := createActiveValuePackageSub(t, cancelledUser.Id, day, now-86400, now-1)
	coveredUser := createValuePackageUser(t, 3043, UserGroupTiyan)
	coveredSub := createActiveValuePackageSub(t, coveredUser.Id, day, now-86400, now-1)
	activeUser := createValuePackageUser(t, 3044, UserGroupTiyan)
	activeSub := createActiveValuePackageSub(t, activeUser.Id, day, now-100, now+86400)

	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", expiredSub.Id).Update("status", UserSubscriptionStatusExpired).Error)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", cancelledSub.Id).Update("status", UserSubscriptionStatusCancelled).Error)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", coveredSub.Id).Update("status", UserSubscriptionStatusCovered).Error)
	_ = activeExpiredSub
	_ = activeSub

	expiredResult, err := ListValuePackageManagementRows(ValuePackageManagementFilter{Active: "expired", Page: 1, PageSize: 20}, now)
	require.NoError(t, err)
	require.EqualValues(t, 2, expiredResult.Total)
	require.ElementsMatch(t, []int{expiredUser.Id, activeExpiredUser.Id}, []int{expiredResult.Items[0].UserId, expiredResult.Items[1].UserId})

	allResult, err := ListValuePackageManagementRows(ValuePackageManagementFilter{Active: "all", Page: 1, PageSize: 20}, now)
	require.NoError(t, err)
	require.EqualValues(t, 3, allResult.Total)
	ids := make([]int, 0, len(allResult.Items))
	for _, row := range allResult.Items {
		ids = append(ids, row.UserId)
	}
	require.ElementsMatch(t, []int{expiredUser.Id, activeExpiredUser.Id, activeUser.Id}, ids)
}

func TestAdjustValuePackageResetCountRejectsInvalidAndNoopWithoutLedger(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3045, UserGroupTiyan)
	require.NoError(t, DB.Create(&UserValuePackagePreference{UserId: user.Id, ResetCount: 2}).Error)

	cases := []struct {
		name   string
		userID int
		mode   ValuePackageResetCountAdjustMode
		value  int
	}{
		{name: "missing user", userID: 999999, mode: ValuePackageResetCountAdjustModeAdd, value: 1},
		{name: "add zero", userID: user.Id, mode: ValuePackageResetCountAdjustModeAdd, value: 0},
		{name: "subtract zero", userID: user.Id, mode: ValuePackageResetCountAdjustModeSubtract, value: 0},
		{name: "set same", userID: user.Id, mode: ValuePackageResetCountAdjustModeSet, value: 2},
		{name: "invalid mode", userID: user.Id, mode: ValuePackageResetCountAdjustMode("bad"), value: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := AdjustValuePackageResetCount(tc.userID, tc.mode, tc.value, "invalid", 9001)
			require.Error(t, err)
		})
	}

	var pref UserValuePackagePreference
	require.NoError(t, DB.Where("user_id = ?", user.Id).First(&pref).Error)
	require.Equal(t, 2, pref.ResetCount)
	var ledgerCount int64
	require.NoError(t, DB.Model(&ValuePackageResetCountLedger{}).Count(&ledgerCount).Error)
	require.Zero(t, ledgerCount)
	var orphanPrefCount int64
	require.NoError(t, DB.Model(&UserValuePackagePreference{}).Where("user_id = ?", 999999).Count(&orphanPrefCount).Error)
	require.Zero(t, orphanPrefCount)
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
	require.EqualValues(t, 0, used7d)

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

func TestGetValuePackageStateIncludesInactiveBillingStateWithoutActiveSubscription(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3502, UserGroupTiyan)

	state, err := GetValuePackageState(user.Id)

	require.NoError(t, err)
	require.NotNil(t, state)
	require.NotNil(t, state.Billing)
	require.False(t, state.Billing.Active)
}

func TestGetValuePackageStateIncludesAuthoritativeBillingState(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3501, UserGroupVIP)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	month.ModelGroup = "month-card"
	require.NoError(t, DB.Save(&month).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, month, now-10, now+3600)
	_, err := ActivateValuePackage(user.Id, sub.Id)
	require.NoError(t, err)

	state, err := GetValuePackageState(user.Id)

	require.NoError(t, err)
	require.NotNil(t, state.Billing)
	require.True(t, state.Billing.Active)
	require.Equal(t, "month-card", state.Billing.PackageGroup)
	require.Equal(t, float64(1), state.Billing.EffectiveRatio)
	require.Equal(t, month.Id, state.Billing.PlanId)
	require.Equal(t, month.Title, state.Billing.PlanTitle)
}

func TestReserveValuePackageUsageToTargetEnforcesRollingLimitAtomically(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3601, UserGroupVIP)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	month.ModelGroup = "month-card"
	month.TotalAmount = 1000
	month.Limit5hAmount = 10
	month.Limit7dAmount = 1000
	require.NoError(t, DB.Save(&month).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, month, now-10, now+3600)
	_, err := PreConsumeValuePackageSubscription("reserve-target-limit", user.Id, sub.Id, 1)
	require.NoError(t, err)

	_, err = ReserveValuePackageUsageToTarget("reserve-target-limit", user.Id, sub.Id, 20)

	require.Error(t, err)
	require.Contains(t, err.Error(), ValuePackageQuotaExhaustedUserMessage)
	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, sub.Id).Error)
	require.EqualValues(t, 1, reloaded.AmountUsed)
	used5h, used7d, err := GetValuePackageWindowUsage(user.Id, sub.Id, common.GetTimestamp())
	require.NoError(t, err)
	require.EqualValues(t, 1, used5h)
	require.EqualValues(t, 1, used7d)
}

func TestReserveValuePackageUsageToTargetReplacesExistingRequestQuota(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3602, UserGroupVIP)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	month.ModelGroup = "month-card"
	month.TotalAmount = 1000
	month.Limit5hAmount = 25
	month.Limit7dAmount = 1000
	require.NoError(t, DB.Save(&month).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, month, now-10, now+3600)
	_, err := PreConsumeValuePackageSubscription("reserve-target-replace", user.Id, sub.Id, 10)
	require.NoError(t, err)

	res, err := ReserveValuePackageUsageToTarget("reserve-target-replace", user.Id, sub.Id, 20)

	require.NoError(t, err)
	require.EqualValues(t, 10, res.AmountUsedBefore)
	require.EqualValues(t, 20, res.AmountUsedAfter)
	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, sub.Id).Error)
	require.EqualValues(t, 20, reloaded.AmountUsed)
	used5h, used7d, err := GetValuePackageWindowUsage(user.Id, sub.Id, common.GetTimestamp())
	require.NoError(t, err)
	require.EqualValues(t, 20, used5h)
	require.EqualValues(t, 20, used7d)
}

func TestReserveValuePackageUsageToTargetKeepsResetClearedRequestOutOfShortWindows(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3603, UserGroupVIP)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	month.ModelGroup = "month-card"
	month.TotalAmount = 1000
	month.Limit5hAmount = 50
	month.Limit7dAmount = 1000
	require.NoError(t, DB.Save(&month).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, month, now-7200, now+3600)
	require.NoError(t, DB.Create(&UserValuePackagePreference{UserId: user.Id, Enabled: true, ActiveUserSubscriptionId: sub.Id}).Error)
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{
		UserId:             user.Id,
		UserSubscriptionId: sub.Id,
		PlanId:             month.Id,
		PackageType:        month.PackageType,
		ModelGroup:         month.ModelGroup,
		RequestId:          "reserve-target-before-reset",
		Quota:              10,
		CreatedAt:          now - 3600,
	}))
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("amount_used", int64(10)).Error)
	require.NoError(t, DB.Create(&ValuePackageQuotaReset{
		UserId:             user.Id,
		UserSubscriptionId: sub.Id,
		PlanId:             month.Id,
		PackageType:        month.PackageType,
		ResetAt:            now - 1800,
		Source:             ValuePackageQuotaResetSourceUserConsumeCount,
		CreatedByUserId:    user.Id,
	}).Error)

	res, err := ReserveValuePackageUsageToTarget("reserve-target-before-reset", user.Id, sub.Id, 80)

	require.NoError(t, err)
	require.EqualValues(t, 10, res.AmountUsedBefore)
	require.EqualValues(t, 80, res.AmountUsedAfter)
	var usageRecord ValuePackageUsageRecord
	require.NoError(t, DB.Where("user_subscription_id = ? AND request_id = ?", sub.Id, "reserve-target-before-reset").First(&usageRecord).Error)
	require.EqualValues(t, now-3600, usageRecord.CreatedAt)
	used5h, used7d, err := GetValuePackageWindowUsage(user.Id, sub.Id, now)
	require.NoError(t, err)
	require.EqualValues(t, 0, used5h)
	require.EqualValues(t, 0, used7d)
}

func TestReserveValuePackageUsageToTargetUsesFixedFiveHourWindowForReplacement(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3604, UserGroupVIP)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	month.ModelGroup = "month-card"
	month.TotalAmount = 1000
	month.Limit5hAmount = 50
	month.Limit7dAmount = 1000
	require.NoError(t, DB.Save(&month).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, month, now-8*3600, now+3600)
	expiredWindowStart := now - 6*3600
	activeWindowStart := now - 30*60
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ModelGroup: month.ModelGroup, RequestId: "reserve-target-expired-window-anchor", Quota: 40, CreatedAt: expiredWindowStart}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ModelGroup: month.ModelGroup, RequestId: "reserve-target-expired-window-final", Quota: 10, CreatedAt: expiredWindowStart + 4*3600}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ModelGroup: month.ModelGroup, RequestId: "reserve-target-active-window", Quota: 45, CreatedAt: activeWindowStart}))
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("amount_used", int64(95)).Error)

	used5h, used7d, err := GetValuePackageWindowUsage(user.Id, sub.Id, now)
	require.NoError(t, err)
	require.EqualValues(t, 45, used5h)
	require.EqualValues(t, 95, used7d)

	res, err := ReserveValuePackageUsageToTarget("reserve-target-expired-window-final", user.Id, sub.Id, 20)

	require.NoError(t, err)
	require.EqualValues(t, 95, res.AmountUsedBefore)
	require.EqualValues(t, 105, res.AmountUsedAfter)
	var usageRecord ValuePackageUsageRecord
	require.NoError(t, DB.Where("user_subscription_id = ? AND request_id = ?", sub.Id, "reserve-target-expired-window-final").First(&usageRecord).Error)
	require.EqualValues(t, expiredWindowStart+4*3600, usageRecord.CreatedAt)
	require.EqualValues(t, 20, usageRecord.Quota)
	used5h, used7d, err = GetValuePackageWindowUsage(user.Id, sub.Id, now)
	require.NoError(t, err)
	require.EqualValues(t, 45, used5h)
	require.EqualValues(t, 105, used7d)
}

func TestReserveValuePackageUsageToTargetUsesAnchored7dWindowForReplacement(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3606, UserGroupVIP)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	month.ModelGroup = "month-card"
	month.TotalAmount = 1000
	month.Limit5hAmount = 1000
	month.Limit7dAmount = 50
	require.NoError(t, DB.Save(&month).Error)
	now := common.GetTimestamp()
	start := now - 8*valuePackageDaySeconds
	sub := createActiveValuePackageSub(t, user.Id, month, start, start+valuePackageMonthSeconds)
	oldRequestAt := now - 6*valuePackageDaySeconds
	currentPeriodAt := now - 30*60
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ModelGroup: month.ModelGroup, RequestId: "reserve-target-anchored-previous", Quota: 10, CreatedAt: oldRequestAt}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ModelGroup: month.ModelGroup, RequestId: "reserve-target-anchored-current", Quota: 45, CreatedAt: currentPeriodAt}))
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("amount_used", int64(55)).Error)

	res, err := ReserveValuePackageUsageToTarget("reserve-target-anchored-previous", user.Id, sub.Id, 20)

	require.NoError(t, err)
	require.Equal(t, sub.Id, res.UserSubscriptionId)
	require.EqualValues(t, 20, res.PreConsumed)
	require.EqualValues(t, 55, res.AmountUsedBefore)
	require.EqualValues(t, 65, res.AmountUsedAfter)
	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, sub.Id).Error)
	require.EqualValues(t, 65, reloaded.AmountUsed)
	var usageRecord ValuePackageUsageRecord
	require.NoError(t, DB.Where("user_subscription_id = ? AND request_id = ?", sub.Id, "reserve-target-anchored-previous").First(&usageRecord).Error)
	require.EqualValues(t, oldRequestAt, usageRecord.CreatedAt)
	require.EqualValues(t, 20, usageRecord.Quota)
	used5h, used7d, err := GetValuePackageWindowUsage(user.Id, sub.Id, now)
	require.NoError(t, err)
	require.EqualValues(t, 45, used5h)
	require.EqualValues(t, 45, used7d)
}

func TestReserveValuePackageUsageToTargetCountsZeroQuotaCurrentWindowReplacement(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3605, UserGroupVIP)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	month.TotalAmount = 1000
	month.Limit5hAmount = 50
	month.Limit7dAmount = 1000
	require.NoError(t, DB.Save(&month).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, month, now-10, now+3600)
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ModelGroup: month.ModelGroup, RequestId: "reserve-target-zero-current-window", Quota: 0, CreatedAt: now}))

	_, err := ReserveValuePackageUsageToTarget("reserve-target-zero-current-window", user.Id, sub.Id, 80)

	require.Error(t, err)
	require.Contains(t, err.Error(), ValuePackageQuotaExhaustedUserMessage)
	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, sub.Id).Error)
	require.EqualValues(t, 0, reloaded.AmountUsed)
	used5h, used7d, err := GetValuePackageWindowUsage(user.Id, sub.Id, now)
	require.NoError(t, err)
	require.EqualValues(t, 0, used5h)
	require.EqualValues(t, 0, used7d)
}

func TestDecreaseUserQuotaIfEnoughDoesNotOverdraw(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3701, UserGroupVIP)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Update("quota", 5).Error)

	err := DecreaseUserQuotaIfEnough(user.Id, 10)

	require.Error(t, err)
	var reloaded User
	require.NoError(t, DB.First(&reloaded, user.Id).Error)
	require.EqualValues(t, 5, reloaded.Quota)
}

func TestDecreaseUserQuotaIfEnoughConsumesWhenEnough(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3702, UserGroupVIP)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Update("quota", 15).Error)

	require.NoError(t, DecreaseUserQuotaIfEnough(user.Id, 10))

	var reloaded User
	require.NoError(t, DB.First(&reloaded, user.Id).Error)
	require.EqualValues(t, 5, reloaded.Quota)
}
