package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupValuePackageMigrationTestDB(t *testing.T) *gorm.DB {
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

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		DB = oldDB
		LOG_DB = oldLogDB
		common.RedisEnabled = oldRedisEnabled
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		initCol()
	})

	return db
}

func TestValuePackagePlanFieldsPersist(t *testing.T) {
	setupValuePackageMigrationTestDB(t)
	require.NoError(t, DB.AutoMigrate(&SubscriptionPlan{}))

	plan := &SubscriptionPlan{
		Title:                 "日卡",
		PriceAmount:           9.90,
		Currency:              "USD",
		DurationUnit:          SubscriptionDurationDay,
		DurationValue:         1,
		Enabled:               true,
		PlanKind:              SubscriptionPlanKindValuePackage,
		PackageType:           ValuePackageTypeDay,
		PackageLevel:          ValuePackageLevelDay,
		ModelGroup:            "day-card",
		ConcurrencyLimit:      1,
		Limit5hAmount:         1000,
		Limit7dAmount:         7000,
		Benefits:              "[\"5小时额度\",\"7天额度\"]",
		LdxpProductUrl:        "https://ldxp.example.test/day",
		LdxpProductName:       "日卡商品",
		LdxpProductAmount:     9.90,
		LdxpProductRef:        "day-card-prod",
		LdxpSessionTTLSeconds: 1800,
		GiftResetCount:        3,
	}
	require.NoError(t, DB.Create(plan).Error)

	var got SubscriptionPlan
	require.NoError(t, DB.First(&got, plan.Id).Error)
	require.Equal(t, SubscriptionPlanKindValuePackage, got.PlanKind)
	require.Equal(t, ValuePackageTypeDay, got.PackageType)
	require.Equal(t, ValuePackageLevelDay, got.PackageLevel)
	require.Equal(t, "day-card", got.ModelGroup)
	require.Equal(t, 1, got.ConcurrencyLimit)
	require.EqualValues(t, 1000, got.Limit5hAmount)
	require.EqualValues(t, 7000, got.Limit7dAmount)
	require.Equal(t, "[\"5小时额度\",\"7天额度\"]", got.Benefits)
	require.Equal(t, "https://ldxp.example.test/day", got.LdxpProductUrl)
	require.Equal(t, "日卡商品", got.LdxpProductName)
	require.Equal(t, 9.90, got.LdxpProductAmount)
	require.Equal(t, "day-card-prod", got.LdxpProductRef)
	require.EqualValues(t, 1800, got.LdxpSessionTTLSeconds)
	require.Equal(t, 3, got.GiftResetCount)
}

func TestEnsureSubscriptionPlanTableSQLiteAddsValuePackageColumns(t *testing.T) {
	setupValuePackageMigrationTestDB(t)
	require.NoError(t, DB.Exec("CREATE TABLE `subscription_plans` (`id` integer, `title` varchar(128) NOT NULL, PRIMARY KEY (`id`))").Error)
	require.NoError(t, DB.Exec("INSERT INTO `subscription_plans` (`id`, `title`) VALUES (1, 'Legacy Plan')").Error)

	require.NoError(t, ensureSubscriptionPlanTableSQLite())

	for _, col := range []string{
		"plan_kind",
		"package_type",
		"package_level",
		"model_group",
		"concurrency_limit",
		"limit_5h_amount",
		"limit_7d_amount",
		"benefits",
		"ldxp_product_url",
		"ldxp_product_name",
		"ldxp_product_amount",
		"ldxp_product_ref",
		"ldxp_session_ttl_seconds",
		"gift_reset_count",
	} {
		require.True(t, DB.Migrator().HasColumn(&SubscriptionPlan{}, col), "missing column %s", col)
	}

	var got struct {
		Id                    int
		Title                 string
		PlanKind              string
		ConcurrencyLimit      int
		Limit5hAmount         int64
		Limit7dAmount         int64
		LdxpProductAmount     float64
		LdxpSessionTTLSeconds int64
		GiftResetCount        int
	}
	require.NoError(t, DB.Table("subscription_plans").Where("id = ?", 1).First(&got).Error)
	require.Equal(t, 1, got.Id)
	require.Equal(t, "Legacy Plan", got.Title)
	require.Equal(t, SubscriptionPlanKindSubscription, got.PlanKind)
	require.Equal(t, 1, got.ConcurrencyLimit)
	require.EqualValues(t, 0, got.Limit5hAmount)
	require.EqualValues(t, 0, got.Limit7dAmount)
	require.Equal(t, 0.0, got.LdxpProductAmount)
	require.EqualValues(t, 0, got.LdxpSessionTTLSeconds)
	require.Zero(t, got.GiftResetCount)
	require.NoError(t, ensureSubscriptionPlanTableSQLite(), "migration must be repeatable")
}

func TestEnsureUserSubscriptionTableSQLiteAddsCoveredColumns(t *testing.T) {
	setupValuePackageMigrationTestDB(t)
	require.NoError(t, DB.Exec("CREATE TABLE `user_subscriptions` (`id` integer, `user_id` integer, `plan_id` integer, PRIMARY KEY (`id`))").Error)
	require.NoError(t, DB.Exec("INSERT INTO `user_subscriptions` (`id`, `user_id`, `plan_id`) VALUES (1, 100, 200)").Error)

	require.NoError(t, ensureUserSubscriptionTableSQLite())

	require.True(t, DB.Migrator().HasColumn(&UserSubscription{}, "covered_by_subscription_id"))
	require.True(t, DB.Migrator().HasColumn(&UserSubscription{}, "covered_time"))
	require.True(t, DB.Migrator().HasColumn(&UserSubscription{}, "quota_epoch"))

	var got struct {
		Id                      int
		UserId                  int
		PlanId                  int
		CoveredBySubscriptionId int
		CoveredTime             int64
		QuotaEpoch              int64
	}
	require.NoError(t, DB.Table("user_subscriptions").Where("id = ?", 1).First(&got).Error)
	require.Equal(t, 1, got.Id)
	require.Equal(t, 100, got.UserId)
	require.Equal(t, 200, got.PlanId)
	require.Equal(t, 0, got.CoveredBySubscriptionId)
	require.EqualValues(t, 0, got.CoveredTime)
	require.EqualValues(t, 0, got.QuotaEpoch)
}

func TestEnsureValuePackageQuotaEpochTablesSQLiteAddsColumnsWithoutLosingRows(t *testing.T) {
	setupValuePackageMigrationTestDB(t)
	require.NoError(t, DB.Exec("CREATE TABLE `subscription_pre_consume_records` (`id` integer PRIMARY KEY, `request_id` varchar(64), `pre_consumed` bigint NOT NULL DEFAULT 0)").Error)
	require.NoError(t, DB.Exec("CREATE TABLE `value_package_usage_records` (`id` integer PRIMARY KEY, `request_id` varchar(64), `quota` bigint NOT NULL DEFAULT 0)").Error)
	require.NoError(t, DB.Exec("CREATE TABLE `value_package_quota_resets` (`id` integer PRIMARY KEY, `reset_at` bigint)").Error)
	require.NoError(t, DB.Exec("INSERT INTO `subscription_pre_consume_records` (`id`, `request_id`, `pre_consumed`) VALUES (1, 'legacy-preconsume', 11)").Error)
	require.NoError(t, DB.Exec("INSERT INTO `value_package_usage_records` (`id`, `request_id`, `quota`) VALUES (1, 'legacy-usage', 12)").Error)
	require.NoError(t, DB.Exec("INSERT INTO `value_package_quota_resets` (`id`, `reset_at`) VALUES (1, 13)").Error)

	require.NoError(t, ensureValuePackageQuotaEpochTablesSQLite())

	require.True(t, DB.Migrator().HasColumn(&SubscriptionPreConsumeRecord{}, "quota_epoch"))
	require.True(t, DB.Migrator().HasColumn(&ValuePackageUsageRecord{}, "quota_epoch"))
	require.True(t, DB.Migrator().HasColumn(&ValuePackageQuotaReset{}, "from_epoch"))
	require.True(t, DB.Migrator().HasColumn(&ValuePackageQuotaReset{}, "to_epoch"))
	require.True(t, DB.Migrator().HasColumn(&ValuePackageQuotaReset{}, "amount_used_before"))

	var preConsume struct {
		Id          int
		RequestId   string
		PreConsumed int64
		QuotaEpoch  int64
	}
	require.NoError(t, DB.Table("subscription_pre_consume_records").First(&preConsume, 1).Error)
	require.Equal(t, "legacy-preconsume", preConsume.RequestId)
	require.EqualValues(t, 11, preConsume.PreConsumed)
	require.Zero(t, preConsume.QuotaEpoch)

	var usage struct {
		Id         int
		RequestId  string
		Quota      int64
		QuotaEpoch int64
	}
	require.NoError(t, DB.Table("value_package_usage_records").First(&usage, 1).Error)
	require.Equal(t, "legacy-usage", usage.RequestId)
	require.EqualValues(t, 12, usage.Quota)
	require.Zero(t, usage.QuotaEpoch)

	var reset struct {
		Id               int
		ResetAt          int64
		FromEpoch        int64
		ToEpoch          int64
		AmountUsedBefore int64
	}
	require.NoError(t, DB.Table("value_package_quota_resets").First(&reset, 1).Error)
	require.EqualValues(t, 13, reset.ResetAt)
	require.Zero(t, reset.FromEpoch)
	require.Zero(t, reset.ToEpoch)
	require.Zero(t, reset.AmountUsedBefore)
}

func TestValuePackageResetCountMigrationAddsPreferenceColumnAndTables(t *testing.T) {
	setupValuePackageMigrationTestDB(t)

	require.NoError(t, DB.AutoMigrate(&UserValuePackagePreference{}))
	require.True(t, DB.Migrator().HasColumn(&UserValuePackagePreference{}, "reset_count"))

	require.NoError(t, DB.AutoMigrate(&ValuePackageQuotaReset{}, &ValuePackageResetCountLedger{}))
	require.True(t, DB.Migrator().HasTable(&ValuePackageQuotaReset{}))
	require.True(t, DB.Migrator().HasTable(&ValuePackageResetCountLedger{}))
}

func TestEnsureUserValuePackagePreferenceTableSQLiteAddsRequiredColumns(t *testing.T) {
	setupValuePackageMigrationTestDB(t)
	require.NoError(t, DB.Exec("CREATE TABLE `user_value_package_preferences` (`id` integer, `user_id` integer, `enabled` numeric DEFAULT 0, `active_user_subscription_id` integer DEFAULT 0, `created_at` bigint, `updated_at` bigint, PRIMARY KEY (`id`))").Error)
	require.NoError(t, DB.Exec("INSERT INTO `user_value_package_preferences` (`id`, `user_id`, `enabled`, `active_user_subscription_id`, `created_at`, `updated_at`) VALUES (1, 100, 1, 200, 10, 20)").Error)

	require.NoError(t, ensureUserValuePackagePreferenceTableSQLite())

	require.True(t, DB.Migrator().HasColumn(&UserValuePackagePreference{}, "reset_count"))
	require.True(t, DB.Migrator().HasColumn(&UserValuePackagePreference{}, "wallet_fallback_enabled"))
	var got struct {
		Id                    int
		UserId                int
		ResetCount            int
		WalletFallbackEnabled *bool
	}
	require.NoError(t, DB.Table("user_value_package_preferences").Where("id = ?", 1).First(&got).Error)
	require.Equal(t, 1, got.Id)
	require.Equal(t, 100, got.UserId)
	require.Equal(t, 0, got.ResetCount)
	require.Nil(t, got.WalletFallbackEnabled)
}

func TestValuePackageMigrateDBCreatesTablesAndColumns(t *testing.T) {
	setupValuePackageMigrationTestDB(t)

	require.NoError(t, migrateDB())

	require.True(t, DB.Migrator().HasColumn(&SubscriptionPlan{}, "plan_kind"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionPlan{}, "concurrency_limit"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionPlan{}, "ldxp_session_ttl_seconds"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionPlan{}, "gift_reset_count"))
	require.True(t, DB.Migrator().HasColumn(&UserSubscription{}, "covered_by_subscription_id"))
	require.True(t, DB.Migrator().HasColumn(&UserSubscription{}, "covered_time"))
	require.True(t, DB.Migrator().HasColumn(&UserSubscription{}, "quota_epoch"))
	require.True(t, DB.Migrator().HasTable(&UserValuePackagePreference{}))
	require.True(t, DB.Migrator().HasColumn(&UserValuePackagePreference{}, "wallet_fallback_enabled"))
	require.True(t, DB.Migrator().HasTable(&ValuePackageUsageRecord{}))
	require.True(t, DB.Migrator().HasTable(&ValuePackageQuotaReset{}))
	require.True(t, DB.Migrator().HasTable(&ValuePackageResetCountLedger{}))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionPreConsumeRecord{}, "quota_epoch"))
	require.True(t, DB.Migrator().HasColumn(&ValuePackageUsageRecord{}, "quota_epoch"))
	require.True(t, DB.Migrator().HasColumn(&ValuePackageQuotaReset{}, "from_epoch"))
	require.True(t, DB.Migrator().HasColumn(&ValuePackageQuotaReset{}, "to_epoch"))
	require.True(t, DB.Migrator().HasColumn(&ValuePackageQuotaReset{}, "amount_used_before"))

	var columns []struct {
		Name         string `gorm:"column:name"`
		DefaultValue string `gorm:"column:dflt_value"`
	}
	require.NoError(t, DB.Raw("PRAGMA table_info(`subscription_plans`)").Scan(&columns).Error)
	var priceAmountDefault string
	var giftResetCountDefault string
	for _, column := range columns {
		switch column.Name {
		case "price_amount":
			priceAmountDefault = column.DefaultValue
		case "gift_reset_count":
			giftResetCountDefault = column.DefaultValue
		}
	}
	require.Equal(t, "0", priceAmountDefault)
	require.Equal(t, "0", giftResetCountDefault)
	require.NoError(t, migrateDB(), "full migration must be repeatable")
}

func TestValuePackageNewTablesMigrate(t *testing.T) {
	setupValuePackageMigrationTestDB(t)
	require.NoError(t, DB.AutoMigrate(&UserValuePackagePreference{}, &ValuePackageUsageRecord{}, &ValuePackageQuotaReset{}, &ValuePackageResetCountLedger{}))
	require.True(t, DB.Migrator().HasTable(&UserValuePackagePreference{}))
	require.True(t, DB.Migrator().HasTable(&ValuePackageUsageRecord{}))
	require.True(t, DB.Migrator().HasTable(&ValuePackageQuotaReset{}))
	require.True(t, DB.Migrator().HasTable(&ValuePackageResetCountLedger{}))
}

func TestEnsureSubscriptionOrderTableSQLiteAddsUserSubscriptionID(t *testing.T) {
	setupValuePackageMigrationTestDB(t)
	require.NoError(t, DB.Exec("CREATE TABLE `subscription_orders` (`id` integer, `user_id` integer, `plan_id` integer, `trade_no` varchar(255), PRIMARY KEY (`id`))").Error)
	require.NoError(t, DB.Exec("INSERT INTO `subscription_orders` (`id`, `user_id`, `plan_id`, `trade_no`) VALUES (1, 100, 200, 'legacy-order')").Error)

	require.NoError(t, ensureSubscriptionOrderTableSQLite())

	require.True(t, DB.Migrator().HasColumn(&SubscriptionOrder{}, "user_subscription_id"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionOrder{}, "gift_reset_count"))
	var got struct {
		Id                 int
		UserSubscriptionId int
		GiftResetCount     int
	}
	require.NoError(t, DB.Table("subscription_orders").Where("id = ?", 1).First(&got).Error)
	require.Equal(t, 1, got.Id)
	require.Equal(t, 0, got.UserSubscriptionId)
	require.Zero(t, got.GiftResetCount)
	require.NoError(t, ensureSubscriptionOrderTableSQLite(), "migration must be repeatable")

	require.NoError(t, DB.Table("subscription_orders").Where("id = ?", 1).Update("user_subscription_id", 321).Error)
	var order SubscriptionOrder
	require.NoError(t, DB.First(&order, 1).Error)
	require.Equal(t, 321, order.UserSubscriptionId)
}
