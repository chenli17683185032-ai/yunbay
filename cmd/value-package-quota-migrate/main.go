package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

var (
	applyMigration = flag.Bool("apply", false, "apply the authorized value package quota migration")
	manifestHash   = flag.String("manifest", "", "authorized preview manifest hash required with --apply")
)

func main() {
	common.InitEnv()
	if err := model.InitDB(); err != nil {
		log.Fatal("database initialization failed")
	}
	defer func() {
		if err := model.CloseDB(); err != nil {
			log.Print("database close failed")
		}
	}()

	now := model.GetDBTimestamp()
	var (
		report *model.LegacyValuePackageQuotaMigrationReport
		err    error
	)
	if *applyMigration {
		authorizedManifest := strings.TrimSpace(*manifestHash)
		if authorizedManifest == "" {
			log.Fatal("--manifest is required with --apply")
		}
		report, err = model.ApplyLegacyValuePackageQuotaMigration(model.DB, now, authorizedManifest)
	} else {
		report, err = model.PreviewLegacyValuePackageQuotaMigration(model.DB, now)
	}
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
