package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSubscriptionPlanVisibilityTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalRedisEnabled := common.RedisEnabled

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	originalDB := DB
	originalLogDB := LOG_DB
	DB = db
	LOG_DB = db

	t.Cleanup(func() {
		DB = originalDB
		LOG_DB = originalLogDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.RedisEnabled = originalRedisEnabled
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	require.NoError(t, db.AutoMigrate(&User{}, &SubscriptionPlan{}, &SubscriptionOrder{}, &UserSubscription{}, &TopUp{}))
	return db
}

func setPaymentComplianceForModelPlanVisibilityTest(t *testing.T, confirmed bool) {
	t.Helper()

	paymentSetting := operation_setting.GetPaymentSetting()
	originalConfirmed := paymentSetting.ComplianceConfirmed
	originalTermsVersion := paymentSetting.ComplianceTermsVersion
	originalConfirmedAt := paymentSetting.ComplianceConfirmedAt
	originalConfirmedBy := paymentSetting.ComplianceConfirmedBy
	originalConfirmedIP := paymentSetting.ComplianceConfirmedIP

	paymentSetting.ComplianceConfirmed = confirmed
	if confirmed {
		paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	} else {
		paymentSetting.ComplianceTermsVersion = ""
	}
	paymentSetting.ComplianceConfirmedAt = 0
	paymentSetting.ComplianceConfirmedBy = 0
	paymentSetting.ComplianceConfirmedIP = ""

	t.Cleanup(func() {
		paymentSetting.ComplianceConfirmed = originalConfirmed
		paymentSetting.ComplianceTermsVersion = originalTermsVersion
		paymentSetting.ComplianceConfirmedAt = originalConfirmedAt
		paymentSetting.ComplianceConfirmedBy = originalConfirmedBy
		paymentSetting.ComplianceConfirmedIP = originalConfirmedIP
	})
}

func createSubscriptionVisibilityPlan(t *testing.T, plan SubscriptionPlan) SubscriptionPlan {
	t.Helper()
	if plan.Id > 0 {
		InvalidateSubscriptionPlanCache(plan.Id)
	}
	t.Cleanup(func() {
		if plan.Id > 0 {
			InvalidateSubscriptionPlanCache(plan.Id)
		}
	})

	// gorm:"default:true" may turn a false create into the DB default, so disabled plans are explicitly updated after insert.
	desiredEnabled := plan.Enabled
	if plan.Title == "" {
		plan.Title = fmt.Sprintf("Plan %d", plan.Id)
	}
	if plan.Currency == "" {
		plan.Currency = "USD"
	}
	if plan.DurationUnit == "" {
		plan.DurationUnit = SubscriptionDurationMonth
	}
	if plan.DurationValue == 0 {
		plan.DurationValue = 1
	}
	require.NoError(t, DB.Create(&plan).Error)
	if plan.Id > 0 {
		InvalidateSubscriptionPlanCache(plan.Id)
	}
	if !desiredEnabled {
		require.NoError(t, DB.Model(&SubscriptionPlan{}).Where("id = ?", plan.Id).Update("enabled", false).Error)
		if plan.Id > 0 {
			InvalidateSubscriptionPlanCache(plan.Id)
		}
		plan.Enabled = false
	}
	return plan
}

func listAdminPlansForVisibilityTest(t *testing.T) []SubscriptionPlan {
	t.Helper()
	var plans []SubscriptionPlan
	require.NoError(t, DB.Order("sort_order desc, id desc").Find(&plans).Error)
	for i := range plans {
		plans[i].NormalizeDefaults()
	}
	return plans
}

func listUserVisiblePlansForVisibilityTest(t *testing.T) []SubscriptionPlan {
	t.Helper()
	if !operation_setting.IsPaymentComplianceConfirmed() {
		return []SubscriptionPlan{}
	}
	var plans []SubscriptionPlan
	require.NoError(t, DB.Where("enabled = ?", true).Order("sort_order desc, id desc").Find(&plans).Error)
	for i := range plans {
		plans[i].NormalizeDefaults()
	}
	return plans
}

func TestSubscriptionPlanVisibility_AdminSeesAllUserSeesEnabledOnly(t *testing.T) {
	setupSubscriptionPlanVisibilityTestDB(t)
	setPaymentComplianceForModelPlanVisibilityTest(t, true)

	createSubscriptionVisibilityPlan(t, SubscriptionPlan{Id: 1001, Title: "Disabled Plan", Enabled: false, SortOrder: 100, PriceAmount: 19.9, TotalAmount: 1000})
	createSubscriptionVisibilityPlan(t, SubscriptionPlan{Id: 1002, Title: "Enabled Plan", Enabled: true, SortOrder: 10, PriceAmount: 29.9, TotalAmount: 2000})

	adminPlans := listAdminPlansForVisibilityTest(t)
	require.Len(t, adminPlans, 2)
	require.Equal(t, "Disabled Plan", adminPlans[0].Title)
	require.Equal(t, "Enabled Plan", adminPlans[1].Title)

	userPlans := listUserVisiblePlansForVisibilityTest(t)
	require.Len(t, userPlans, 1)
	require.Equal(t, "Enabled Plan", userPlans[0].Title)
	require.True(t, userPlans[0].Enabled)
}

func TestSubscriptionPlanVisibility_UserSeesEmptyWhenComplianceNotConfirmed(t *testing.T) {
	setupSubscriptionPlanVisibilityTestDB(t)
	setPaymentComplianceForModelPlanVisibilityTest(t, false)

	createSubscriptionVisibilityPlan(t, SubscriptionPlan{Id: 1101, Title: "Enabled But Locked", Enabled: true, SortOrder: 1, PriceAmount: 29.9, TotalAmount: 2000})

	adminPlans := listAdminPlansForVisibilityTest(t)
	require.Len(t, adminPlans, 1)
	require.Equal(t, "Enabled But Locked", adminPlans[0].Title)

	userPlans := listUserVisiblePlansForVisibilityTest(t)
	require.Empty(t, userPlans)
}

func TestPurchaseSubscriptionWithBalanceRejectsDisabledPlan(t *testing.T) {
	setupSubscriptionPlanVisibilityTestDB(t)
	setPaymentComplianceForModelPlanVisibilityTest(t, true)

	user := &User{Id: 1201, Username: "disabled_plan_buyer", Status: common.UserStatusEnabled, Quota: 1000000}
	require.NoError(t, DB.Create(user).Error)

	plan := createSubscriptionVisibilityPlan(t, SubscriptionPlan{Id: 1202, Title: "Disabled Purchase Plan", Enabled: false, PriceAmount: 1, TotalAmount: 1000})

	err := PurchaseSubscriptionWithBalance(user.Id, plan.Id)
	require.Error(t, err)
	require.Contains(t, err.Error(), "套餐未启用")

	var subCount int64
	require.NoError(t, DB.Model(&UserSubscription{}).Where("user_id = ?", user.Id).Count(&subCount).Error)
	require.Zero(t, subCount)

	var orderCount int64
	require.NoError(t, DB.Model(&SubscriptionOrder{}).Where("user_id = ?", user.Id).Count(&orderCount).Error)
	require.Zero(t, orderCount)

	var reloaded User
	require.NoError(t, DB.Select("quota").Where("id = ?", user.Id).First(&reloaded).Error)
	require.Equal(t, 1000000, reloaded.Quota)
}
