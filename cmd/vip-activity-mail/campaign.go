package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net/mail"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const (
	campaignMailKey = "vip-recharge-rebate-20260717-v1-mail"
	campaignSubject = "云贝限时福利：VIP 低至 0.15 倍率，老用户充值补贴已到账"

	mailStatusPending = "pending"
	mailStatusSending = "sending"
	mailStatusSent    = "sent"
	mailStatusFailed  = "failed"
)

const campaignHTML = `<!doctype html>
<html lang="zh-CN">
<body style="margin:0;padding:24px;background:#f6f7f9;color:#202124;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI','PingFang SC','Microsoft YaHei',sans-serif;line-height:1.7;">
  <div style="max-width:680px;margin:0 auto;background:#ffffff;border:1px solid #e6e8eb;padding:28px;">
    <p>经过多轮系统调试优化，目前平台连接质量、各项功能均已稳定运行。</p>
    <p>👏👏👏👏给大家带来重磅福利：VIP会员计费倍率下调至0.15，SVIP原有赠送规则保持不变，当前性价比已经远超20X Pro会员。</p>
    <p>超值套餐用户可领取额度清零重置券：日卡1张、周卡2张、月卡4张。</p>
    <p>此前充值过VIP的老用户，将额外发放已充值余额30%的补贴，直接到账账户。</p>
    <p>所有会员还能免费使用Grok4.5专属访问通道。</p>
    <p>本次全部福利为限时活动，不保证长期永久生效；即便后续价格回调，上调幅度也不会超过当前定价。</p>
  </div>
</body>
</html>`

var (
	sendCampaignEmail = common.SendEmail
	campaignSleep     = time.Sleep
)

type campaignRecipient struct {
	UserID        int
	Email         string
	RecipientHash string
}

type campaignMailManifestRow struct {
	UserID        int    `json:"user_id"`
	RecipientHash string `json:"recipient_hash"`
}

type campaignMailManifest struct {
	CampaignKey string                    `json:"campaign_key"`
	SubjectHash string                    `json:"subject_hash"`
	BodyHash    string                    `json:"body_hash"`
	Recipients  []campaignMailManifestRow `json:"recipients"`
}

type campaignMailPreview struct {
	CampaignKey    string              `json:"campaign_key"`
	Subject        string              `json:"subject"`
	ManifestHash   string              `json:"manifest_hash"`
	BodyHash       string              `json:"body_hash"`
	TotalUsers     int64               `json:"total_users"`
	RecipientCount int                 `json:"recipient_count"`
	MissingEmail   int                 `json:"missing_email"`
	InvalidEmail   int                 `json:"invalid_email"`
	DuplicateEmail int                 `json:"duplicate_email"`
	Recipients     []campaignRecipient `json:"-"`
}

type campaignMailBatch struct {
	CampaignKey    string `gorm:"type:varchar(64);primaryKey"`
	ManifestHash   string `gorm:"type:varchar(64);not null"`
	SubjectHash    string `gorm:"type:varchar(64);not null"`
	BodyHash       string `gorm:"type:varchar(64);not null"`
	RecipientCount int    `gorm:"not null"`
	PreparedAt     int64  `gorm:"not null"`
	CompletedAt    int64  `gorm:"not null;default:0"`
}

func (campaignMailBatch) TableName() string {
	return "campaign_mail_batches"
}

type campaignMailReceipt struct {
	CampaignKey   string `gorm:"type:varchar(64);primaryKey"`
	RecipientHash string `gorm:"type:varchar(64);primaryKey"`
	UserID        int    `gorm:"index;not null"`
	Email         string `gorm:"type:varchar(320);not null"`
	Status        string `gorm:"type:varchar(16);index;not null"`
	Attempts      int    `gorm:"not null;default:0"`
	LastError     string `gorm:"type:text"`
	PreparedAt    int64  `gorm:"not null"`
	LastAttemptAt int64  `gorm:"not null;default:0"`
	SentAt        int64  `gorm:"not null;default:0"`
}

func (campaignMailReceipt) TableName() string {
	return "campaign_mail_receipts"
}

type campaignMailStatusReport struct {
	CampaignKey    string `json:"campaign_key"`
	ManifestHash   string `json:"manifest_hash"`
	RecipientCount int    `json:"recipient_count"`
	Pending        int64  `json:"pending"`
	Sending        int64  `json:"sending"`
	Sent           int64  `json:"sent"`
	Failed         int64  `json:"failed"`
	CompletedAt    int64  `json:"completed_at"`
}

func hashText(value string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(value)))
}

func normalizeCampaignEmail(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return "", errors.New("email is empty")
	}
	parsed, err := mail.ParseAddress(normalized)
	if err != nil || !strings.EqualFold(parsed.Address, normalized) {
		return "", errors.New("email is invalid")
	}
	return normalized, nil
}

func buildCampaignRecipients(db *gorm.DB) (*campaignMailPreview, error) {
	if db == nil {
		return nil, errors.New("database is nil")
	}
	var totalUsers int64
	if err := db.Model(&model.User{}).Count(&totalUsers).Error; err != nil {
		return nil, err
	}
	var users []model.User
	if err := db.Select("id", "email").Order("id asc").Find(&users).Error; err != nil {
		return nil, err
	}

	preview := &campaignMailPreview{
		CampaignKey: campaignMailKey,
		Subject:     campaignSubject,
		TotalUsers:  totalUsers,
		BodyHash:    hashText(campaignHTML),
	}
	seen := make(map[string]struct{}, len(users))
	for _, user := range users {
		if strings.TrimSpace(user.Email) == "" {
			preview.MissingEmail++
			continue
		}
		email, err := normalizeCampaignEmail(user.Email)
		if err != nil {
			preview.InvalidEmail++
			continue
		}
		if _, ok := seen[email]; ok {
			preview.DuplicateEmail++
			continue
		}
		seen[email] = struct{}{}
		preview.Recipients = append(preview.Recipients, campaignRecipient{
			UserID:        user.Id,
			Email:         email,
			RecipientHash: hashText(email),
		})
	}
	preview.RecipientCount = len(preview.Recipients)

	manifestRows := make([]campaignMailManifestRow, 0, len(preview.Recipients))
	for _, recipient := range preview.Recipients {
		manifestRows = append(manifestRows, campaignMailManifestRow{
			UserID:        recipient.UserID,
			RecipientHash: recipient.RecipientHash,
		})
	}
	sort.Slice(manifestRows, func(i, j int) bool {
		if manifestRows[i].RecipientHash == manifestRows[j].RecipientHash {
			return manifestRows[i].UserID < manifestRows[j].UserID
		}
		return manifestRows[i].RecipientHash < manifestRows[j].RecipientHash
	})
	manifestPayload, err := common.Marshal(campaignMailManifest{
		CampaignKey: campaignMailKey,
		SubjectHash: hashText(campaignSubject),
		BodyHash:    preview.BodyHash,
		Recipients:  manifestRows,
	})
	if err != nil {
		return nil, err
	}
	preview.ManifestHash = fmt.Sprintf("%x", sha256.Sum256(manifestPayload))
	return preview, nil
}

func previewCampaignMail(db *gorm.DB) (*campaignMailPreview, error) {
	return buildCampaignRecipients(db)
}

func authorizeCampaignMail(db *gorm.DB, manifest string) error {
	preview, err := previewCampaignMail(db)
	if err != nil {
		return err
	}
	if preview.ManifestHash == manifest {
		return prepareCampaignMail(db, preview)
	}

	prepared, err := validatePreparedCampaignMail(db, manifest)
	if err != nil {
		return err
	}
	if prepared {
		return nil
	}
	return fmt.Errorf("recipient manifest mismatch: current=%s authorized=%s", preview.ManifestHash, manifest)
}

func validatePreparedCampaignMail(db *gorm.DB, manifest string) (bool, error) {
	if !db.Migrator().HasTable(&campaignMailBatch{}) {
		return false, nil
	}
	var batch campaignMailBatch
	result := db.Where("campaign_key = ?", campaignMailKey).Limit(1).Find(&batch)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected != 1 {
		return false, nil
	}
	if batch.ManifestHash != manifest {
		return false, errors.New("existing mail batch does not match the authorized manifest")
	}
	if batch.SubjectHash != hashText(campaignSubject) || batch.BodyHash != hashText(campaignHTML) {
		return false, errors.New("existing mail batch content does not match the campaign")
	}
	var receiptCount int64
	if err := db.Model(&campaignMailReceipt{}).Where("campaign_key = ?", campaignMailKey).Count(&receiptCount).Error; err != nil {
		return false, err
	}
	if receiptCount != int64(batch.RecipientCount) {
		return false, errors.New("existing mail receipt count does not match the batch")
	}
	return true, nil
}

func prepareCampaignMail(db *gorm.DB, preview *campaignMailPreview) error {
	if preview == nil || preview.ManifestHash == "" || preview.RecipientCount == 0 {
		return errors.New("mail preview is invalid")
	}
	if err := db.AutoMigrate(&campaignMailBatch{}, &campaignMailReceipt{}); err != nil {
		return fmt.Errorf("prepare mail receipt schema: %w", err)
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var existing campaignMailBatch
		result := tx.Where("campaign_key = ?", campaignMailKey).Limit(1).Find(&existing)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 1 {
			if existing.ManifestHash != preview.ManifestHash || existing.RecipientCount != preview.RecipientCount || existing.SubjectHash != hashText(campaignSubject) || existing.BodyHash != preview.BodyHash {
				return errors.New("existing mail batch does not match the authorized manifest")
			}
			var receiptCount int64
			if err := tx.Model(&campaignMailReceipt{}).Where("campaign_key = ?", campaignMailKey).Count(&receiptCount).Error; err != nil {
				return err
			}
			if receiptCount != int64(existing.RecipientCount) {
				return errors.New("existing mail receipt count does not match the batch")
			}
			return nil
		}

		now := time.Now().Unix()
		batch := campaignMailBatch{
			CampaignKey:    campaignMailKey,
			ManifestHash:   preview.ManifestHash,
			SubjectHash:    hashText(campaignSubject),
			BodyHash:       preview.BodyHash,
			RecipientCount: preview.RecipientCount,
			PreparedAt:     now,
		}
		if err := tx.Create(&batch).Error; err != nil {
			return err
		}
		receipts := make([]campaignMailReceipt, 0, len(preview.Recipients))
		for _, recipient := range preview.Recipients {
			receipts = append(receipts, campaignMailReceipt{
				CampaignKey:   campaignMailKey,
				RecipientHash: recipient.RecipientHash,
				UserID:        recipient.UserID,
				Email:         recipient.Email,
				Status:        mailStatusPending,
				PreparedAt:    now,
			})
		}
		return tx.CreateInBatches(receipts, 100).Error
	})
}

func sendCampaignPilot(db *gorm.DB, maxAttempts int) error {
	var receipt campaignMailReceipt
	result := db.Table("campaign_mail_receipts AS receipt").
		Select("receipt.*").
		Joins("JOIN users AS user_row ON user_row.id = receipt.user_id").
		Where("receipt.campaign_key = ?", campaignMailKey).
		Order("user_row.role DESC, receipt.user_id ASC").
		Limit(1).
		Scan(&receipt)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("pilot recipient is missing from the campaign")
	}
	sent, err := deliverCampaignReceipt(db, receipt, maxAttempts)
	if err != nil {
		return err
	}
	if !sent {
		return errors.New("pilot recipient exhausted the retry limit")
	}
	return nil
}

func sendCampaignBulk(db *gorm.DB, interval time.Duration, maxAttempts int) error {
	return sendCampaignBulkWithSender(db, interval, maxAttempts, sendCampaignEmail)
}

func sendCampaignBulkWithSender(db *gorm.DB, interval time.Duration, maxAttempts int, sender campaignEmailSender) error {
	var receipts []campaignMailReceipt
	if err := db.Where("campaign_key = ? AND status IN ? AND attempts < ?", campaignMailKey, []string{mailStatusPending, mailStatusFailed}, maxAttempts).
		Order("user_id asc").Find(&receipts).Error; err != nil {
		return err
	}
	var failed int
	for index, receipt := range receipts {
		sent, err := deliverCampaignReceiptWithSender(db, receipt, maxAttempts, sender)
		if err != nil {
			return err
		}
		if !sent {
			failed++
		}
		if interval > 0 && index < len(receipts)-1 {
			campaignSleep(interval)
		}
	}
	if err := refreshCampaignMailBatch(db); err != nil {
		return err
	}
	status, err := campaignMailStatus(db)
	if err != nil {
		return err
	}
	if failed > 0 || status.Failed > 0 || status.Sending > 0 {
		return fmt.Errorf("campaign is incomplete: failed=%d sending=%d", status.Failed, status.Sending)
	}
	return nil
}

func deliverCampaignReceipt(db *gorm.DB, receipt campaignMailReceipt, maxAttempts int) (bool, error) {
	return deliverCampaignReceiptWithSender(db, receipt, maxAttempts, sendCampaignEmail)
}

func deliverCampaignReceiptWithSender(db *gorm.DB, receipt campaignMailReceipt, maxAttempts int, sender campaignEmailSender) (bool, error) {
	for receipt.Attempts < maxAttempts {
		if err := db.Where("campaign_key = ? AND recipient_hash = ?", receipt.CampaignKey, receipt.RecipientHash).First(&receipt).Error; err != nil {
			return false, err
		}
		switch receipt.Status {
		case mailStatusSent:
			return true, nil
		case mailStatusSending:
			return false, fmt.Errorf("recipient %s has uncertain sending state", receipt.RecipientHash)
		case mailStatusPending, mailStatusFailed:
		default:
			return false, fmt.Errorf("recipient %s has invalid status %q", receipt.RecipientHash, receipt.Status)
		}

		now := time.Now().Unix()
		claim := db.Model(&campaignMailReceipt{}).
			Where("campaign_key = ? AND recipient_hash = ? AND status IN ? AND attempts = ?", receipt.CampaignKey, receipt.RecipientHash, []string{mailStatusPending, mailStatusFailed}, receipt.Attempts).
			Updates(map[string]interface{}{
				"status":          mailStatusSending,
				"attempts":        gorm.Expr("attempts + 1"),
				"last_attempt_at": now,
			})
		if claim.Error != nil {
			return false, claim.Error
		}
		if claim.RowsAffected != 1 {
			continue
		}
		receipt.Attempts++

		sendErr := sender(campaignSubject, receipt.Email, campaignHTML)
		if sendErr == nil {
			result := db.Model(&campaignMailReceipt{}).
				Where("campaign_key = ? AND recipient_hash = ? AND status = ?", receipt.CampaignKey, receipt.RecipientHash, mailStatusSending).
				Updates(map[string]interface{}{
					"status":     mailStatusSent,
					"sent_at":    time.Now().Unix(),
					"last_error": "",
				})
			if result.Error != nil || result.RowsAffected != 1 {
				return false, fmt.Errorf("delivery succeeded but receipt update failed for %s", receipt.RecipientHash)
			}
			return true, nil
		}

		message := sendErr.Error()
		if len(message) > 1000 {
			message = message[:1000]
		}
		result := db.Model(&campaignMailReceipt{}).
			Where("campaign_key = ? AND recipient_hash = ? AND status = ?", receipt.CampaignKey, receipt.RecipientHash, mailStatusSending).
			Updates(map[string]interface{}{
				"status":     mailStatusFailed,
				"last_error": message,
			})
		if result.Error != nil || result.RowsAffected != 1 {
			return false, fmt.Errorf("SMTP failed and receipt update failed for %s", receipt.RecipientHash)
		}
		var fatalErr *campaignEmailFatalError
		if errors.As(sendErr, &fatalErr) {
			return false, fatalErr
		}
		if receipt.Attempts < maxAttempts {
			campaignSleep(time.Duration(1<<uint(receipt.Attempts)) * time.Second)
		}
	}
	return false, nil
}

func campaignMailStatus(db *gorm.DB) (*campaignMailStatusReport, error) {
	var batch campaignMailBatch
	result := db.Where("campaign_key = ?", campaignMailKey).Limit(1).Find(&batch)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, errors.New("campaign mail batch does not exist")
	}
	report := &campaignMailStatusReport{
		CampaignKey:    campaignMailKey,
		ManifestHash:   batch.ManifestHash,
		RecipientCount: batch.RecipientCount,
		CompletedAt:    batch.CompletedAt,
	}
	var rows []struct {
		Status string
		Count  int64
	}
	if err := db.Model(&campaignMailReceipt{}).
		Select("status, COUNT(*) AS count").
		Where("campaign_key = ?", campaignMailKey).
		Group("status").Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		switch row.Status {
		case mailStatusPending:
			report.Pending = row.Count
		case mailStatusSending:
			report.Sending = row.Count
		case mailStatusSent:
			report.Sent = row.Count
		case mailStatusFailed:
			report.Failed = row.Count
		}
	}
	return report, nil
}

func refreshCampaignMailBatch(db *gorm.DB) error {
	report, err := campaignMailStatus(db)
	if err != nil {
		return err
	}
	completedAt := int64(0)
	if report.Sent == int64(report.RecipientCount) && report.Pending == 0 && report.Sending == 0 && report.Failed == 0 {
		completedAt = time.Now().Unix()
	}
	return db.Model(&campaignMailBatch{}).
		Where("campaign_key = ?", campaignMailKey).
		Update("completed_at", completedAt).Error
}
