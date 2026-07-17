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
	"gorm.io/gorm/logger"
)

type cliOptions struct {
	Action      string
	Manifest    string
	Interval    time.Duration
	MaxAttempts int
}

func main() {
	options, err := parseCLIArgs(os.Args[1:])
	if err == nil {
		os.Args = commonInitArgs(os.Args[0])
		err = runCLI(os.Stdout, os.Stderr, options.Action, options.Manifest, options.Interval, options.MaxAttempts)
	}
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func commonInitArgs(program string) []string {
	return []string{program}
}

func parseCLIArgs(args []string) (cliOptions, error) {
	options := cliOptions{}
	flags := flag.NewFlagSet("vip-activity-mail", flag.ContinueOnError)
	flags.StringVar(&options.Action, "action", "preview", "preview, pilot, send, or status")
	flags.StringVar(&options.Manifest, "manifest", "", "authorized recipient manifest required for pilot/send")
	flags.DurationVar(&options.Interval, "interval", 600*time.Millisecond, "delay between bulk deliveries")
	flags.IntVar(&options.MaxAttempts, "max-attempts", 3, "maximum SMTP attempts per recipient")
	if err := flags.Parse(args); err != nil {
		return cliOptions{}, err
	}
	if flags.NArg() != 0 {
		return cliOptions{}, fmt.Errorf("unexpected positional arguments: %s", strings.Join(flags.Args(), " "))
	}
	return options, nil
}

func runCLI(stdout io.Writer, stderr io.Writer, action string, manifest string, interval time.Duration, maxAttempts int) (err error) {
	action = strings.TrimSpace(action)
	manifest = strings.TrimSpace(manifest)
	if interval < 0 {
		return fmt.Errorf("interval cannot be negative")
	}
	if maxAttempts <= 0 || maxAttempts > 5 {
		return fmt.Errorf("max-attempts must be between 1 and 5")
	}

	maintenanceLogger := logger.New(log.New(stderr, "\r\n", log.LstdFlags), logger.Config{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  logger.Warn,
		IgnoreRecordNotFoundError: true,
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
	model.InitOptionMap()

	var report interface{}
	switch action {
	case "preview":
		report, err = previewCampaignMail(model.DB)
	case "pilot", "send":
		if manifest == "" {
			return fmt.Errorf("--manifest is required for %s", action)
		}
		if common.SMTPServer == "" || common.SMTPAccount == "" || common.SMTPToken == "" || common.SMTPFrom == "" {
			return fmt.Errorf("SMTP configuration is incomplete")
		}
		if err = authorizeCampaignMail(model.DB, manifest); err != nil {
			return err
		}
		if action == "pilot" {
			err = sendCampaignPilot(model.DB, maxAttempts)
		} else {
			if strings.EqualFold(common.SMTPServer, "smtp.qq.com") {
				var session *qqCampaignSMTPSession
				session, err = newQQCampaignSMTPSession()
				if err == nil {
					err = sendCampaignBulkWithSender(model.DB, interval, maxAttempts, session.Send)
				}
				if session != nil {
					if closeErr := session.Close(); closeErr != nil && err == nil {
						err = fmt.Errorf("close QQ SMTP session: %w", closeErr)
					}
				}
			} else {
				err = sendCampaignBulk(model.DB, interval, maxAttempts)
			}
		}
		report, _ = campaignMailStatus(model.DB)
	case "status":
		report, err = campaignMailStatus(model.DB)
	default:
		return fmt.Errorf("unsupported action %q", action)
	}
	if report != nil {
		payload, marshalErr := common.Marshal(report)
		if marshalErr != nil {
			return fmt.Errorf("encode report: %w", marshalErr)
		}
		if _, writeErr := fmt.Fprintln(stdout, string(payload)); writeErr != nil {
			return fmt.Errorf("write report: %w", writeErr)
		}
	}
	return err
}
