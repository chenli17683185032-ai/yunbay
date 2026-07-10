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

var (
	applyMigration = flag.Bool("apply", false, "apply the authorized value package quota migration")
	manifestHash   = flag.String("manifest", "", "authorized preview manifest hash required with --apply")
)

func main() {
	if err := runQuotaMigrationCLI(os.Stdout, os.Stderr, applyMigration, manifestHash); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func runQuotaMigrationCLI(stdout io.Writer, stderr io.Writer, applyFlag *bool, manifestFlag *string) (err error) {
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

	common.InitEnv()
	if err = model.InitDBWithoutMigrations(); err != nil {
		return fmt.Errorf("database initialization failed: %w", err)
	}
	defer func() {
		if closeErr := model.CloseDB(); closeErr != nil && err == nil {
			err = fmt.Errorf("database close failed: %w", closeErr)
		}
	}()
	maintenanceLogger := gormlogger.New(log.New(stderr, "\r\n", log.LstdFlags), gormlogger.Config{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  gormlogger.Warn,
		IgnoreRecordNotFoundError: false,
		Colorful:                  false,
	})
	model.DB = model.DB.Session(&gorm.Session{Logger: maintenanceLogger})
	model.LOG_DB = model.DB

	now := model.GetDBTimestamp()
	report, err := runLegacyValuePackageQuotaMigration(model.DB, now, *applyFlag, *manifestFlag)
	if err != nil {
		return err
	}
	payload, err := common.Marshal(report)
	if err != nil {
		return fmt.Errorf("report encoding failed: %w", err)
	}
	if _, err := fmt.Fprintln(stdout, string(payload)); err != nil {
		return fmt.Errorf("report output failed: %w", err)
	}
	return nil
}

func runLegacyValuePackageQuotaMigration(db *gorm.DB, now int64, apply bool, manifestHash string) (*model.LegacyValuePackageQuotaMigrationReport, error) {
	if !apply {
		return model.PreviewLegacyValuePackageQuotaMigration(db, now)
	}
	authorizedManifest := strings.TrimSpace(manifestHash)
	if authorizedManifest == "" {
		return nil, fmt.Errorf("--manifest is required with --apply")
	}
	if err := db.AutoMigrate(&model.ValuePackageQuotaMigrationReceipt{}); err != nil {
		return nil, fmt.Errorf("prepare migration receipt schema: %w", err)
	}
	return model.ApplyLegacyValuePackageQuotaMigration(db, now, authorizedManifest)
}
