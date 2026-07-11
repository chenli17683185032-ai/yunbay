package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func main() {
	apply, prepare, manifest, reportPath, err := parseValuePackageResetReconcileFlags(os.Args[1:])
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
	if err := runValuePackageResetReconcileCLI(os.Stdout, os.Stderr, apply, prepare, manifest, reportPath); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func parseValuePackageResetReconcileFlags(args []string) (bool, bool, string, string, error) {
	flags := flag.NewFlagSet("value-package-reset-reconcile", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	apply := flags.Bool("apply", false, "apply the authorized value package reset reconciliation")
	prepare := flags.Bool("prepare-schema", false, "prepare required quota epoch schema without reconciling data")
	manifest := flags.String("manifest", "", "authorized preview manifest hash required with --apply")
	reportPath := flags.String("b2-report", "", "path to the applied B2 migration JSON report")
	if err := flags.Parse(args); err != nil {
		return false, false, "", "", err
	}
	return *apply, *prepare, *manifest, *reportPath, nil
}

func runValuePackageResetReconcileCLI(stdout io.Writer, stderr io.Writer, apply bool, prepare bool, manifest string, reportPath string) (err error) {
	if prepare && apply {
		return fmt.Errorf("--prepare-schema and --apply cannot be used together")
	}
	common.LogWriterMu.Lock()
	previousWriter := gin.DefaultWriter
	previousErrorWriter := gin.DefaultErrorWriter
	gin.DefaultWriter = stderr
	gin.DefaultErrorWriter = stderr
	common.LogWriterMu.Unlock()
	defer func() {
		common.LogWriterMu.Lock()
		gin.DefaultWriter = previousWriter
		gin.DefaultErrorWriter = previousErrorWriter
		common.LogWriterMu.Unlock()
	}()
	var report *model.LegacyValuePackageQuotaMigrationReport
	if !prepare {
		report, err = readAppliedB2Report(reportPath)
		if err != nil {
			return err
		}
	}

	maintenanceLogger := gormlogger.New(log.New(stderr, "\r\n", log.LstdFlags), gormlogger.Config{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  gormlogger.Warn,
		IgnoreRecordNotFoundError: false,
		Colorful:                  false,
	})
	common.InitEnv()
	if err = model.InitDBWithoutMigrationsWithLogger(maintenanceLogger); err != nil {
		return fmt.Errorf("database initialization failed: %w", err)
	}
	defer func() {
		if closeErr := model.CloseDB(); closeErr != nil && err == nil {
			err = fmt.Errorf("database close failed: %w", closeErr)
		}
	}()
	if prepare {
		if err := model.PrepareValuePackageResetReconcileSchema(model.DB); err != nil {
			return fmt.Errorf("prepare reconcile quota epoch schema: %w", err)
		}
		if _, err := fmt.Fprintln(stdout, `{"prepared":true}`); err != nil {
			return fmt.Errorf("schema preparation output failed: %w", err)
		}
		return nil
	}

	reconcileReport, err := runValuePackageResetReconcile(model.DB, model.GetDBTimestamp(), apply, manifest, report)
	if err != nil {
		return err
	}
	payload, err := common.Marshal(reconcileReport)
	if err != nil {
		return fmt.Errorf("report encoding failed: %w", err)
	}
	if _, err := fmt.Fprintln(stdout, string(payload)); err != nil {
		return fmt.Errorf("report output failed: %w", err)
	}
	return nil
}

func readAppliedB2Report(path string) (*model.LegacyValuePackageQuotaMigrationReport, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errorsNewB2ReportRequired()
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open B2 report: %w", err)
	}
	defer file.Close()
	var report model.LegacyValuePackageQuotaMigrationReport
	if err := common.DecodeJson(file, &report); err != nil {
		return nil, fmt.Errorf("decode B2 report: %w", err)
	}
	return &report, nil
}

func errorsNewB2ReportRequired() error {
	return fmt.Errorf("--b2-report is required")
}

func runValuePackageResetReconcile(db *gorm.DB, now int64, apply bool, manifest string, b2Report *model.LegacyValuePackageQuotaMigrationReport) (*model.ValuePackageResetReconcileReport, error) {
	manifest = strings.TrimSpace(manifest)
	if apply && manifest == "" {
		return nil, fmt.Errorf("--manifest is required with --apply")
	}
	if err := model.ValidateValuePackageResetReconcileSchema(db); err != nil {
		return nil, err
	}
	if !apply {
		return model.PreviewValuePackageResetReconcile(db, now, b2Report)
	}
	if err := db.AutoMigrate(&model.ValuePackageResetReconcileReceipt{}); err != nil {
		return nil, fmt.Errorf("prepare reconcile receipt schema: %w", err)
	}
	return model.ApplyValuePackageResetReconcile(db, now, b2Report, manifest)
}
