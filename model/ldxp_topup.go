package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	LdxpStatusCreated       = "created"
	LdxpStatusWorkerClaimed = "worker_claimed"
	LdxpStatusQrReady       = "qr_ready"
	LdxpStatusWorkerPaid    = "worker_paid"
	LdxpStatusVerified      = "verified"
	LdxpStatusRedeemed      = "redeemed"
	LdxpStatusSuccess       = "success"
	LdxpStatusCanceled      = "canceled"
	LdxpStatusExpired       = "expired"
	LdxpStatusWorkerFailed  = "worker_failed"
	LdxpStatusMailTimeout   = "mail_timeout"
	LdxpStatusVerifyFailed  = "verify_failed"
	LdxpStatusRedeemFailed  = "redeem_failed"
)

type LdxpTopupSession struct {
	Id                   int            `json:"id"`
	SessionId            string         `json:"session_id" gorm:"type:varchar(64);uniqueIndex"`
	UserId               int            `json:"user_id" gorm:"index"`
	Amount               int64          `json:"amount"`
	Money                float64        `json:"money"`
	ProductUrl           string         `json:"product_url" gorm:"type:text"`
	ProductName          string         `json:"product_name" gorm:"type:text"`
	ContactEmail         string         `json:"contact_email" gorm:"type:varchar(255)"`
	Status               string         `json:"status" gorm:"type:varchar(64);index"`
	WorkerId             string         `json:"worker_id" gorm:"type:varchar(128);index"`
	QrCode               string         `json:"qr_code" gorm:"type:text"`
	QrPageUrl            string         `json:"qr_page_url" gorm:"type:text"`
	QrReadyTime          int64          `json:"qr_ready_time" gorm:"bigint"`
	WorkerOrderNo        string         `json:"worker_order_no" gorm:"type:varchar(64);index"`
	WorkerAmount         float64        `json:"worker_amount"`
	WorkerProductName    string         `json:"worker_product_name" gorm:"type:text"`
	WorkerCardKey        string         `json:"worker_card_key" gorm:"type:varchar(255);index"`
	WorkerStatusText     string         `json:"worker_status_text" gorm:"type:varchar(64)"`
	WorkerSuccessUrl     string         `json:"worker_success_url" gorm:"type:text"`
	WorkerDetectedTime   int64          `json:"worker_detected_time" gorm:"bigint"`
	MailMessageId        string         `json:"mail_message_id" gorm:"type:varchar(255)"`
	MailOrderNo          string         `json:"mail_order_no" gorm:"type:varchar(64);index"`
	MailAmount           float64        `json:"mail_amount"`
	MailProductName      string         `json:"mail_product_name" gorm:"type:text"`
	MailCardKey          string         `json:"mail_card_key" gorm:"type:varchar(255);index"`
	MailFrom             string         `json:"mail_from" gorm:"type:varchar(255)"`
	MailTo               string         `json:"mail_to" gorm:"type:varchar(255)"`
	MailSubject          string         `json:"mail_subject" gorm:"type:text"`
	MailReceivedTime     int64          `json:"mail_received_time" gorm:"bigint"`
	VerifiedTime         int64          `json:"verified_time" gorm:"bigint"`
	RedeemedTime         int64          `json:"redeemed_time" gorm:"bigint"`
	TopupId              int            `json:"topup_id" gorm:"index"`
	RedemptionId         int            `json:"redemption_id" gorm:"index"`
	ErrorCode            string         `json:"error_code" gorm:"type:varchar(64)"`
	ErrorMessage         string         `json:"error_message" gorm:"type:text"`
	DebugSnapshotPath    string         `json:"debug_snapshot_path" gorm:"type:text"`
	CreatedTime          int64          `json:"created_time" gorm:"bigint;index"`
	UpdatedTime          int64          `json:"updated_time" gorm:"bigint"`
	ExpiredTime          int64          `json:"expired_time" gorm:"bigint;index"`
	PaidWatchWorkerId    string         `json:"paid_watch_worker_id" gorm:"type:varchar(128);index"`
	PaidWatchClaimedTime int64          `json:"paid_watch_claimed_time" gorm:"bigint;index"`
	DeletedAt            gorm.DeletedAt `gorm:"index"`
}

type LdxpMailEvent struct {
	Id               int            `json:"id"`
	MessageId        *string        `json:"message_id,omitempty" gorm:"type:varchar(255);uniqueIndex"`
	ImapUid          string         `json:"imap_uid" gorm:"type:varchar(128);index"`
	RawHash          string         `json:"raw_hash" gorm:"type:varchar(128);uniqueIndex"`
	MailFrom         string         `json:"mail_from" gorm:"type:varchar(255)"`
	MailTo           string         `json:"mail_to" gorm:"type:varchar(255)"`
	Subject          string         `json:"subject" gorm:"type:text"`
	ReceivedTime     int64          `json:"received_time" gorm:"bigint;index"`
	OrderNo          string         `json:"order_no" gorm:"type:varchar(64);index"`
	Amount           float64        `json:"amount"`
	ProductName      string         `json:"product_name" gorm:"type:text"`
	CardKey          string         `json:"card_key" gorm:"type:varchar(255);index"`
	PaidTime         int64          `json:"paid_time" gorm:"bigint"`
	BodyExcerpt      string         `json:"body_excerpt" gorm:"type:text"`
	MatchedSessionId string         `json:"matched_session_id" gorm:"type:varchar(64);index"`
	Processed        bool           `json:"processed" gorm:"default:false"`
	ErrorMessage     string         `json:"error_message" gorm:"type:text"`
	CreatedTime      int64          `json:"created_time" gorm:"bigint"`
	DeletedAt        gorm.DeletedAt `gorm:"index"`
}

func (LdxpTopupSession) TableName() string {
	return "ldxp_topup_sessions"
}

func (LdxpMailEvent) TableName() string {
	return "ldxp_mail_events"
}

func InsertLdxpTopupSession(session *LdxpTopupSession) error {
	return DB.Create(session).Error
}

func GetLdxpTopupSessionBySessionId(sessionId string) (*LdxpTopupSession, error) {
	var session LdxpTopupSession
	err := DB.Where("session_id = ?", sessionId).First(&session).Error
	return &session, err
}

func GetLdxpTopupSessionForUser(sessionId string, userId int) (*LdxpTopupSession, error) {
	var session LdxpTopupSession
	err := DB.Where("session_id = ? AND user_id = ?", sessionId, userId).First(&session).Error
	return &session, err
}

func ClaimNextLdxpTopupSession(workerId string, now int64) (*LdxpTopupSession, error) {
	var claimed *LdxpTopupSession
	err := DB.Transaction(func(tx *gorm.DB) error {
		for i := 0; i < 3; i++ {
			var candidate LdxpTopupSession
			query := tx.Where("status = ? AND expired_time > ?", LdxpStatusCreated, now).Order("created_time ASC, id ASC")
			if !common.UsingSQLite {
				query = query.Clauses(clause.Locking{Strength: "UPDATE"})
			}
			if err := query.First(&candidate).Error; err != nil {
				return err
			}

			updated, err := claimSelectedLdxpTopupSession(tx, &candidate, workerId, now)
			if err == nil {
				claimed = updated
				return nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		return gorm.ErrRecordNotFound
	})
	if err != nil {
		return nil, err
	}
	return claimed, nil
}

func ClaimNextLdxpPaidWatchSession(workerId string, now int64) (*LdxpTopupSession, error) {
	workerId = strings.TrimSpace(workerId)
	if workerId == "" {
		return nil, gorm.ErrInvalidData
	}
	var claimed *LdxpTopupSession
	err := DB.Transaction(func(tx *gorm.DB) error {
		var candidate LdxpTopupSession
		query := tx.
			Where("status = ? AND worker_id = ? AND expired_time > ? AND worker_order_no <> '' AND qr_page_url <> ''", LdxpStatusQrReady, workerId, now).
			Order("COALESCE(paid_watch_claimed_time, 0) ASC, updated_time ASC, id ASC")
		if !common.UsingSQLite {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.First(&candidate).Error; err != nil {
			return err
		}

		result := tx.Model(&LdxpTopupSession{}).
			Where("id = ? AND status = ? AND worker_id = ? AND expired_time > ?", candidate.Id, LdxpStatusQrReady, workerId, now).
			Updates(map[string]interface{}{
				"paid_watch_worker_id":    workerId,
				"paid_watch_claimed_time": now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}

		var updated LdxpTopupSession
		if err := tx.Where("id = ?", candidate.Id).First(&updated).Error; err != nil {
			return err
		}
		claimed = &updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return claimed, nil
}

func claimSelectedLdxpTopupSession(tx *gorm.DB, candidate *LdxpTopupSession, workerId string, now int64) (*LdxpTopupSession, error) {
	if candidate == nil {
		return nil, gorm.ErrInvalidData
	}
	result := tx.Model(&LdxpTopupSession{}).
		Where("id = ? AND status = ? AND expired_time > ?", candidate.Id, LdxpStatusCreated, now).
		Updates(map[string]interface{}{
			"status":       LdxpStatusWorkerClaimed,
			"worker_id":    workerId,
			"updated_time": now,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, gorm.ErrRecordNotFound
	}

	var claimed LdxpTopupSession
	if err := tx.Where("id = ?", candidate.Id).First(&claimed).Error; err != nil {
		return nil, err
	}
	return &claimed, nil
}

func SaveLdxpTopupSession(session *LdxpTopupSession) error {
	return DB.Save(session).Error
}

func InsertLdxpMailEvent(event *LdxpMailEvent) error {
	if event == nil || strings.TrimSpace(event.RawHash) == "" {
		return gorm.ErrInvalidData
	}
	if event.MessageId != nil {
		messageId := strings.TrimSpace(*event.MessageId)
		if messageId == "" {
			event.MessageId = nil
		} else {
			event.MessageId = &messageId
		}
	}
	return DB.Create(event).Error
}

func GetLdxpMailEventByOrderNo(orderNo string) (*LdxpMailEvent, error) {
	var event LdxpMailEvent
	err := DB.Where("order_no = ?", orderNo).First(&event).Error
	return &event, err
}

func MarkExpiredLdxpTopupSessions(now int64) (int64, error) {
	result := DB.Model(&LdxpTopupSession{}).
		Where("status IN ? AND expired_time <= ?", []string{LdxpStatusCreated, LdxpStatusWorkerClaimed, LdxpStatusQrReady}, now).
		Updates(map[string]interface{}{
			"status":       LdxpStatusExpired,
			"updated_time": now,
		})
	return result.RowsAffected, result.Error
}
