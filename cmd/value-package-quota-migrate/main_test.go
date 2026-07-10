package main

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupQuotaMigrationCommandDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, closeErr := db.DB()
		if closeErr == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, db.AutoMigrate(&model.SubscriptionPlan{}, &model.UserSubscription{}))
	return db
}

func TestRunLegacyValuePackageQuotaMigrationPreviewDoesNotCreateReceiptTable(t *testing.T) {
	db := setupQuotaMigrationCommandDB(t)

	report, err := runLegacyValuePackageQuotaMigration(db, 2_000_000_000, false, "")

	require.NoError(t, err)
	require.NotNil(t, report)
	require.False(t, db.Migrator().HasTable(&model.ValuePackageQuotaMigrationReceipt{}))
}

func TestRunLegacyValuePackageQuotaMigrationApplyCreatesOnlyReceiptSchemaOnFirstRun(t *testing.T) {
	db := setupQuotaMigrationCommandDB(t)
	now := int64(2_000_000_000)
	plan := model.SubscriptionPlan{
		Title:         "day card",
		PlanKind:      model.SubscriptionPlanKindValuePackage,
		PackageType:   model.ValuePackageTypeDay,
		TotalAmount:   2400,
		DurationUnit:  model.SubscriptionDurationDay,
		DurationValue: 1,
	}
	require.NoError(t, db.Create(&plan).Error)
	sub := model.UserSubscription{
		UserId:      1,
		PlanId:      plan.Id,
		AmountTotal: 0,
		AmountUsed:  100,
		StartTime:   now - 100,
		EndTime:     now + 100,
		Status:      model.UserSubscriptionStatusActive,
		Source:      "test",
	}
	require.NoError(t, db.Create(&sub).Error)
	preview, err := runLegacyValuePackageQuotaMigration(db, now, false, "")
	require.NoError(t, err)
	require.False(t, db.Migrator().HasTable(&model.ValuePackageQuotaMigrationReceipt{}))

	applied, err := runLegacyValuePackageQuotaMigration(db, now, true, preview.ManifestHash)

	require.NoError(t, err)
	require.Equal(t, 1, applied.Updated)
	require.True(t, db.Migrator().HasTable(&model.ValuePackageQuotaMigrationReceipt{}))
	var reloaded model.UserSubscription
	require.NoError(t, db.First(&reloaded, sub.Id).Error)
	require.EqualValues(t, 2500, reloaded.AmountTotal)
}
