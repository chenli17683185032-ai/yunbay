package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupCampaignMailTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	t.Cleanup(func() {
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func createCampaignMailUser(t *testing.T, db *gorm.DB, id int, email string) {
	t.Helper()
	user := model.User{Id: id, Username: fmt.Sprintf("mail-user-%d", id), Role: 1, Email: email, AffCode: fmt.Sprintf("mail-aff-%d", id)}
	require.NoError(t, db.Create(&user).Error)
}

func TestBuildCampaignRecipientsNormalizesAndDeduplicates(t *testing.T) {
	db := setupCampaignMailTestDB(t)
	createCampaignMailUser(t, db, 1, " ROOT@Example.com ")
	createCampaignMailUser(t, db, 2, "")
	createCampaignMailUser(t, db, 3, "not-an-email")
	createCampaignMailUser(t, db, 4, "root@example.com")
	createCampaignMailUser(t, db, 5, "member@example.com")

	preview, err := previewCampaignMail(db)
	require.NoError(t, err)
	require.Equal(t, int64(5), preview.TotalUsers)
	require.Equal(t, 2, preview.RecipientCount)
	require.Equal(t, 1, preview.MissingEmail)
	require.Equal(t, 1, preview.InvalidEmail)
	require.Equal(t, 1, preview.DuplicateEmail)
	require.Equal(t, "root@example.com", preview.Recipients[0].Email)
	require.Len(t, preview.ManifestHash, 64)

	repeated, err := previewCampaignMail(db)
	require.NoError(t, err)
	require.Equal(t, preview.ManifestHash, repeated.ManifestHash)
}

func TestCampaignMailPilotAndBulkAreIdempotent(t *testing.T) {
	db := setupCampaignMailTestDB(t)
	createCampaignMailUser(t, db, 1, "root@example.com")
	createCampaignMailUser(t, db, 2, "member@example.com")
	preview, err := previewCampaignMail(db)
	require.NoError(t, err)
	require.NoError(t, prepareCampaignMail(db, preview))

	originalSend := sendCampaignEmail
	originalSleep := campaignSleep
	var delivered []string
	sendCampaignEmail = func(subject string, receiver string, content string) error {
		require.Equal(t, campaignSubject, subject)
		require.Equal(t, campaignHTML, content)
		delivered = append(delivered, receiver)
		return nil
	}
	campaignSleep = func(time.Duration) {}
	t.Cleanup(func() {
		sendCampaignEmail = originalSend
		campaignSleep = originalSleep
	})

	require.NoError(t, sendCampaignPilot(db, 3))
	require.Equal(t, []string{"root@example.com"}, delivered)
	require.NoError(t, sendCampaignPilot(db, 3))
	require.Equal(t, []string{"root@example.com"}, delivered)

	require.NoError(t, sendCampaignBulk(db, 0, 3))
	require.ElementsMatch(t, []string{"root@example.com", "member@example.com"}, delivered)
	require.NoError(t, sendCampaignBulk(db, 0, 3))
	require.Len(t, delivered, 2)

	status, err := campaignMailStatus(db)
	require.NoError(t, err)
	require.Equal(t, int64(2), status.Sent)
	require.Zero(t, status.Pending)
	require.Zero(t, status.Failed)
	require.NotZero(t, status.CompletedAt)
}

func TestCampaignMailRetriesTemporaryFailure(t *testing.T) {
	db := setupCampaignMailTestDB(t)
	createCampaignMailUser(t, db, 1, "root@example.com")
	preview, err := previewCampaignMail(db)
	require.NoError(t, err)
	require.NoError(t, prepareCampaignMail(db, preview))

	originalSend := sendCampaignEmail
	originalSleep := campaignSleep
	attempts := 0
	sendCampaignEmail = func(string, string, string) error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary SMTP failure")
		}
		return nil
	}
	campaignSleep = func(time.Duration) {}
	t.Cleanup(func() {
		sendCampaignEmail = originalSend
		campaignSleep = originalSleep
	})

	require.NoError(t, sendCampaignPilot(db, 3))
	require.Equal(t, 3, attempts)
	var receipt campaignMailReceipt
	require.NoError(t, db.First(&receipt).Error)
	require.Equal(t, mailStatusSent, receipt.Status)
	require.Equal(t, 3, receipt.Attempts)
}

func TestCampaignMailPilotReportsExhaustedFailure(t *testing.T) {
	db := setupCampaignMailTestDB(t)
	createCampaignMailUser(t, db, 1, "root@example.com")
	preview, err := previewCampaignMail(db)
	require.NoError(t, err)
	require.NoError(t, prepareCampaignMail(db, preview))

	originalSend := sendCampaignEmail
	originalSleep := campaignSleep
	sendCampaignEmail = func(string, string, string) error { return errors.New("permanent SMTP failure") }
	campaignSleep = func(time.Duration) {}
	t.Cleanup(func() {
		sendCampaignEmail = originalSend
		campaignSleep = originalSleep
	})

	err = sendCampaignPilot(db, 2)
	require.ErrorContains(t, err, "exhausted")
	status, statusErr := campaignMailStatus(db)
	require.NoError(t, statusErr)
	require.Equal(t, int64(1), status.Failed)
}

func TestCampaignMailBulkStopsAfterFatalSenderError(t *testing.T) {
	db := setupCampaignMailTestDB(t)
	createCampaignMailUser(t, db, 1, "first@example.com")
	createCampaignMailUser(t, db, 2, "second@example.com")
	preview, err := previewCampaignMail(db)
	require.NoError(t, err)
	require.NoError(t, prepareCampaignMail(db, preview))

	attempts := 0
	err = sendCampaignBulkWithSender(db, 0, 1, func(string, string, string) error {
		attempts++
		return &campaignEmailFatalError{err: errors.New("connection lost")}
	})
	require.ErrorContains(t, err, "connection lost")
	require.Equal(t, 1, attempts)

	status, statusErr := campaignMailStatus(db)
	require.NoError(t, statusErr)
	require.Equal(t, int64(1), status.Failed)
	require.Equal(t, int64(1), status.Pending)
	require.Zero(t, status.Sending)
}

func TestCampaignMailPilotUsesHighestRoleRecipientWithEmail(t *testing.T) {
	db := setupCampaignMailTestDB(t)
	createCampaignMailUser(t, db, 1, "")
	createCampaignMailUser(t, db, 2, "admin@example.com")
	createCampaignMailUser(t, db, 3, "member@example.com")
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", 2).Update("role", 10).Error)
	preview, err := previewCampaignMail(db)
	require.NoError(t, err)
	require.NoError(t, prepareCampaignMail(db, preview))

	originalSend := sendCampaignEmail
	originalSleep := campaignSleep
	var delivered string
	sendCampaignEmail = func(_ string, receiver string, _ string) error {
		delivered = receiver
		return nil
	}
	campaignSleep = func(time.Duration) {}
	t.Cleanup(func() {
		sendCampaignEmail = originalSend
		campaignSleep = originalSleep
	})

	require.NoError(t, sendCampaignPilot(db, 1))
	require.Equal(t, "admin@example.com", delivered)
}

func TestAuthorizeCampaignMailResumesPreparedManifestAfterNewRegistration(t *testing.T) {
	db := setupCampaignMailTestDB(t)
	createCampaignMailUser(t, db, 1, "root@example.com")
	createCampaignMailUser(t, db, 2, "member@example.com")
	preview, err := previewCampaignMail(db)
	require.NoError(t, err)
	require.NoError(t, prepareCampaignMail(db, preview))

	createCampaignMailUser(t, db, 3, "new-member@example.com")
	require.NoError(t, authorizeCampaignMail(db, preview.ManifestHash))

	originalSend := sendCampaignEmail
	originalSleep := campaignSleep
	var delivered []string
	sendCampaignEmail = func(_ string, receiver string, _ string) error {
		delivered = append(delivered, receiver)
		return nil
	}
	campaignSleep = func(time.Duration) {}
	t.Cleanup(func() {
		sendCampaignEmail = originalSend
		campaignSleep = originalSleep
	})

	require.NoError(t, sendCampaignBulk(db, 0, 1))
	require.ElementsMatch(t, []string{"root@example.com", "member@example.com"}, delivered)
	require.NotContains(t, delivered, "new-member@example.com")
}

func TestAuthorizeCampaignMailRejectsDriftWithoutPreparedBatch(t *testing.T) {
	db := setupCampaignMailTestDB(t)
	createCampaignMailUser(t, db, 1, "root@example.com")
	preview, err := previewCampaignMail(db)
	require.NoError(t, err)

	createCampaignMailUser(t, db, 2, "new-member@example.com")
	err = authorizeCampaignMail(db, preview.ManifestHash)
	require.ErrorContains(t, err, "recipient manifest mismatch")
}

func TestAuthorizeCampaignMailRejectsDifferentPreparedManifest(t *testing.T) {
	db := setupCampaignMailTestDB(t)
	createCampaignMailUser(t, db, 1, "root@example.com")
	preview, err := previewCampaignMail(db)
	require.NoError(t, err)
	require.NoError(t, prepareCampaignMail(db, preview))

	createCampaignMailUser(t, db, 2, "new-member@example.com")
	changed, err := previewCampaignMail(db)
	require.NoError(t, err)
	err = authorizeCampaignMail(db, changed.ManifestHash)
	require.ErrorContains(t, err, "existing mail batch does not match")
}

func TestCampaignHTMLContainsApprovedCopy(t *testing.T) {
	for _, text := range []string{
		"VIP会员计费倍率下调至0.15",
		"日卡1张、周卡2张、月卡4张",
		"已充值余额30%的补贴",
		"Grok4.5专属访问通道",
		"上调幅度也不会超过当前定价",
	} {
		require.Contains(t, campaignHTML, text)
	}
}

func TestParseCLIArgsUsesProvidedAction(t *testing.T) {
	options, err := parseCLIArgs([]string{
		"--action", "pilot",
		"--manifest", "authorized-hash",
		"--interval", "750ms",
		"--max-attempts", "2",
	})
	require.NoError(t, err)
	require.Equal(t, "pilot", options.Action)
	require.Equal(t, "authorized-hash", options.Manifest)
	require.Equal(t, 750*time.Millisecond, options.Interval)
	require.Equal(t, 2, options.MaxAttempts)
	require.Equal(t, []string{"vip-activity-mail"}, commonInitArgs("vip-activity-mail"))
}
