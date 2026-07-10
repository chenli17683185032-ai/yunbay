package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

var (
	applyMigration = flag.Bool("apply", false, "apply the authorized value package quota migration")
	manifestHash   = flag.String("manifest", "", "authorized preview manifest hash required with --apply")
)

func main() {
	common.InitEnv()
	if err := model.InitDBWithoutMigrations(); err != nil {
		log.Fatal("database initialization failed")
	}
	defer func() {
		if err := model.CloseDB(); err != nil {
			log.Print("database close failed")
		}
	}()

	now := model.GetDBTimestamp()
	report, err := runLegacyValuePackageQuotaMigration(model.DB, now, *applyMigration, *manifestHash)
	if err != nil {
		log.Fatal(err)
	}
	payload, err := common.Marshal(report)
	if err != nil {
		log.Fatal("report encoding failed")
	}
	if _, err := fmt.Fprintln(os.Stdout, string(payload)); err != nil {
		log.Fatal("report output failed")
	}
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
