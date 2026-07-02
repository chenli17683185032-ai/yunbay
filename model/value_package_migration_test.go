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
}

func TestEnsureSubscriptionPlanTableSQLiteAddsValuePackageColumns(t *testing.T) {
	setupValuePackageMigrationTestDB(t)
	require.NoError(t, DB.Exec("CREATE TABLE `subscription_plans` (`id` integer, `title` varchar(128) NOT NULL, PRIMARY KEY (`id`))").Error)

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
	} {
		require.True(t, DB.Migrator().HasColumn(&SubscriptionPlan{}, col), "missing column %s", col)
	}
}

func TestValuePackageNewTablesMigrate(t *testing.T) {
	setupValuePackageMigrationTestDB(t)
	require.NoError(t, DB.AutoMigrate(&UserValuePackagePreference{}, &ValuePackageUsageRecord{}))
	require.True(t, DB.Migrator().HasTable(&UserValuePackagePreference{}))
	require.True(t, DB.Migrator().HasTable(&ValuePackageUsageRecord{}))
}
