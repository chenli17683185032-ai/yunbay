package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupValuePackageResetReconcileCommandDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.SubscriptionPlan{},
		&model.UserSubscription{},
		&model.ValuePackageUsageRecord{},
		&model.ValuePackageQuotaReset{},
		&model.ValuePackageQuotaMigrationReceipt{},
	))
	t.Cleanup(func() {
		sqlDB, closeErr := db.DB()
		if closeErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func seedValuePackageResetReconcileCommand(t *testing.T, db *gorm.DB) *model.LegacyValuePackageQuotaMigrationReport {
	t.Helper()
	plan := model.SubscriptionPlan{Title: "week", PlanKind: model.SubscriptionPlanKindValuePackage, PackageType: model.ValuePackageTypeWeek, TotalAmount: 45_000_000, DurationUnit: model.SubscriptionDurationDay, DurationValue: 7}
	require.NoError(t, db.Create(&plan).Error)
	sub := model.UserSubscription{UserId: 1, PlanId: plan.Id, AmountTotal: 50_000_000, AmountUsed: 5_000_000, StartTime: 1_999_990_000, EndTime: 2_000_010_000, Status: model.UserSubscriptionStatusActive, Source: "redemption"}
	require.NoError(t, db.Create(&sub).Error)
	require.NoError(t, db.Create(&model.ValuePackageUsageRecord{UserId: sub.UserId, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: model.ValuePackageTypeWeek, RequestId: "before-b2", Quota: 5_000_000, CreatedAt: 1_999_999_000}).Error)
	report := &model.LegacyValuePackageQuotaMigrationReport{
		MigrationNow: 1_999_999_500,
		Rows:         []model.LegacyValuePackageQuotaMigrationRow{{SubscriptionID: sub.Id, PlanID: plan.Id, PackageType: model.ValuePackageTypeWeek, AmountUsed: 5_000_000, OldTotal: 0, Grant: plan.TotalAmount, NewTotal: 50_000_000, EndTime: sub.EndTime}},
		Skipped:      map[string]int{},
	}
	manifest := "a4cd503d36fb321e1e5a39f2a45ce943baaff0c83184180eb0d3f2b34835db5d"
	report.ManifestHash = manifest
	require.NoError(t, db.Create(&model.ValuePackageQuotaMigrationReceipt{MigrationVersion: model.LegacyValuePackageQuotaMigrationVersion, ManifestHash: manifest, AppliedAt: report.MigrationNow, Updated: 1}).Error)
	return report
}

func TestRunValuePackageResetReconcilePreviewDoesNotCreateReceiptTable(t *testing.T) {
	db := setupValuePackageResetReconcileCommandDB(t)
	report := seedValuePackageResetReconcileCommand(t, db)

	_, err := runValuePackageResetReconcile(db, 2_000_000_000, false, "", report)

	require.Error(t, err, "fixture manifest must be rejected rather than silently trusted")
	require.False(t, db.Migrator().HasTable(&model.ValuePackageResetReconcileReceipt{}))
}

func TestRunValuePackageResetReconcileApplyRequiresManifestBeforeSchemaWrite(t *testing.T) {
	db := setupValuePackageResetReconcileCommandDB(t)

	_, err := runValuePackageResetReconcile(db, 2_000_000_000, true, "", &model.LegacyValuePackageQuotaMigrationReport{})

	require.Error(t, err)
	require.Contains(t, err.Error(), "--manifest")
	require.False(t, db.Migrator().HasTable(&model.ValuePackageResetReconcileReceipt{}))
}

func TestRunValuePackageResetReconcilePreviewRejectsLegacySchemaWithoutWriting(t *testing.T) {
	db := setupValuePackageResetReconcileCommandDB(t)
	require.NoError(t, db.AutoMigrate(&model.SubscriptionPreConsumeRecord{}))
	for _, column := range []struct {
		model interface{}
		field string
	}{
		{model: &model.UserSubscription{}, field: "QuotaEpoch"},
		{model: &model.SubscriptionPreConsumeRecord{}, field: "QuotaEpoch"},
		{model: &model.ValuePackageUsageRecord{}, field: "QuotaEpoch"},
		{model: &model.ValuePackageQuotaReset{}, field: "FromEpoch"},
		{model: &model.ValuePackageQuotaReset{}, field: "ToEpoch"},
		{model: &model.ValuePackageQuotaReset{}, field: "AmountUsedBefore"},
	} {
		require.NoError(t, db.Migrator().DropColumn(column.model, column.field))
	}

	_, err := runValuePackageResetReconcile(db, 2_000_000_000, false, "", &model.LegacyValuePackageQuotaMigrationReport{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "--prepare-schema")
	require.False(t, db.Migrator().HasColumn(&model.UserSubscription{}, "QuotaEpoch"))
	require.NoError(t, model.PrepareValuePackageResetReconcileSchema(db))
	require.True(t, db.Migrator().HasColumn(&model.UserSubscription{}, "QuotaEpoch"))
	require.True(t, db.Migrator().HasColumn(&model.SubscriptionPreConsumeRecord{}, "QuotaEpoch"))
	require.True(t, db.Migrator().HasColumn(&model.ValuePackageUsageRecord{}, "QuotaEpoch"))
	require.True(t, db.Migrator().HasColumn(&model.ValuePackageQuotaReset{}, "FromEpoch"))
	require.True(t, db.Migrator().HasColumn(&model.ValuePackageQuotaReset{}, "ToEpoch"))
	require.True(t, db.Migrator().HasColumn(&model.ValuePackageQuotaReset{}, "AmountUsedBefore"))
}

func TestReadAppliedB2ReportUsesCommonDecoder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "b2.json")
	payload, err := common.Marshal(&model.LegacyValuePackageQuotaMigrationReport{MigrationNow: 123, ManifestHash: strings.Repeat("a", 64), Rows: []model.LegacyValuePackageQuotaMigrationRow{}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, payload, 0o600))

	report, err := readAppliedB2Report(path)

	require.NoError(t, err)
	require.EqualValues(t, 123, report.MigrationNow)
	require.Equal(t, strings.Repeat("a", 64), report.ManifestHash)
}

func TestReadAppliedB2ReportRequiresPath(t *testing.T) {
	report, err := readAppliedB2Report(" ")

	require.Error(t, err)
	require.Nil(t, report)
	require.Contains(t, err.Error(), "--b2-report")
}

func TestValuePackageResetReconcileBinaryAcceptsPrepareSchemaFlag(t *testing.T) {
	binaryPath := filepath.Join(t.TempDir(), "value-package-reset-reconcile")
	build := exec.Command("go", "build", "-buildvcs=false", "-o", binaryPath, ".")
	build.Env = os.Environ()
	output, err := build.CombinedOutput()
	require.NoError(t, err, string(output))

	command := exec.Command(binaryPath, "--prepare-schema")
	command.Dir = t.TempDir()
	command.Env = append(os.Environ(), "SQL_DSN=local", "SQLITE_PATH="+filepath.Join(command.Dir, "reconcile.db"), "LOG_SQL_DSN=")
	output, err = command.CombinedOutput()
	require.NoError(t, err, string(output))
	require.Contains(t, string(output), `{"prepared":true}`)
}
