package main

import (
	"bytes"
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

func TestQuotaMigrationBinaryConnectionFailureKeepsStdoutEmpty(t *testing.T) {
	binaryPath := filepath.Join(t.TempDir(), "value-package-quota-migrate")
	build := exec.Command("go", "build", "-buildvcs=false", "-o", binaryPath, ".")
	buildOutput, err := build.CombinedOutput()
	require.NoError(t, err, string(buildOutput))

	command := exec.Command(binaryPath)
	command.Env = append(os.Environ(),
		"SQL_DSN=quota:quota@tcp(127.0.0.1:1)/quota?timeout=100ms&readTimeout=100ms&writeTimeout=100ms",
		"LOG_SQL_DSN=",
	)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	err = command.Run()

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	require.Equal(t, 1, exitErr.ExitCode())
	require.Empty(t, stdout.String())
	require.Contains(t, stderr.String(), "[FATAL]")
	require.Contains(t, stderr.String(), "127.0.0.1:1")
}

func TestRunQuotaMigrationCLIWritesOnlyJSONToStdoutAndClosesCleanly(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "quota-cli.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.SubscriptionPlan{}, &model.UserSubscription{}))
	require.NoError(t, db.Create(&model.UserSubscription{
		UserId:      1,
		PlanId:      999999,
		AmountTotal: 0,
		AmountUsed:  10,
		StartTime:   1_999_999_000,
		EndTime:     4_000_000_000,
		Status:      model.UserSubscriptionStatusActive,
		Source:      "cli-test",
	}).Error)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	oldSQLitePath := common.SQLitePath
	t.Cleanup(func() { common.SQLitePath = oldSQLitePath })
	t.Setenv("SQL_DSN", "local")
	t.Setenv("SQLITE_PATH", dbPath)
	t.Setenv("LOG_SQL_DSN", "")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	apply := false
	manifest := ""

	require.NotPanics(t, func() {
		require.NoError(t, runQuotaMigrationCLI(&stdout, &stderr, &apply, &manifest))
	})

	stdoutText := stdout.String()
	require.NotEmpty(t, strings.TrimSpace(stdoutText))
	require.NotContains(t, stdoutText, "[SYS]")
	require.NotContains(t, stdoutText, "record not found")
	var report model.LegacyValuePackageQuotaMigrationReport
	require.NoError(t, common.Unmarshal([]byte(strings.TrimSpace(stdoutText)), &report))
	require.Equal(t, 1, report.Skipped["missing_plan"])
	require.Contains(t, stderr.String(), "using SQLite")
	require.Contains(t, stderr.String(), "record not found")
}

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
