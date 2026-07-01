package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	MailCheckStatusNotRequired     = "not_required"
	MailCheckStatusPending         = "pending"
	MailCheckStatusWaitingMail     = "waiting_mail"
	MailCheckStatusChecking        = "checking"
	MailCheckStatusVerified        = "verified"
	MailCheckStatusOrderMismatch   = "order_mismatch"
	MailCheckStatusAmountMismatch  = "amount_mismatch"
	MailCheckStatusMailParseFailed = "mail_parse_failed"
	MailCheckStatusMailFetchFailed = "mail_fetch_failed"
	MailCheckStatusTimeout         = "timeout"
)

const (
	AffiliateCommissionStatusPending   = "pending"
	AffiliateCommissionStatusAvailable = "available"
	AffiliateCommissionStatusWithdrawn = "withdrawn"
	AffiliateCommissionStatusRejected  = "rejected"
)

const (
	AffiliateWithdrawalStatusPending  = "pending"
	AffiliateWithdrawalStatusPaid     = "paid"
	AffiliateWithdrawalStatusRejected = "rejected"
)

type LdxpTopupSession struct {
	Id                int    `json:"id"`
	SessionId         string `json:"session_id" gorm:"uniqueIndex;type:varchar(64)"`
	UserId            int    `json:"user_id" gorm:"index"`
	TopUpId           int    `json:"topup_id" gorm:"index;default:0"`
	TradeNo           string `json:"trade_no" gorm:"type:varchar(255);index"`
	SiteAmountCents   int64  `json:"site_amount_cents" gorm:"type:bigint;not null;default:0"`
	ExternalPaidCents int64  `json:"external_paid_cents" gorm:"type:bigint;not null;default:0"`
	WorkerOrderNo     string `json:"worker_order_no" gorm:"type:varchar(64);index"`
	WorkerAmountCents int64  `json:"worker_amount_cents" gorm:"type:bigint;not null;default:0"`
	MailOrderNo       string `json:"mail_order_no" gorm:"type:varchar(64);index"`
	MailAmountCents   int64  `json:"mail_amount_cents" gorm:"type:bigint;not null;default:0"`
	MailStatus        string `json:"mail_status" gorm:"type:varchar(32);index;default:'pending'"`
	MailEventId       int    `json:"mail_event_id" gorm:"index;default:0"`
	ErrorCode         string `json:"error_code" gorm:"type:varchar(64);default:''"`
	ErrorMessage      string `json:"error_message" gorm:"type:varchar(512);default:''"`
	CreatedTime       int64  `json:"created_time" gorm:"index"`
	PaidTime          int64  `json:"paid_time" gorm:"index;default:0"`
	VerifiedTime      int64  `json:"verified_time" gorm:"index;default:0"`
	UpdatedTime       int64  `json:"updated_time" gorm:"autoUpdateTime"`
}

type LdxpMailEvent struct {
	Id            int    `json:"id"`
	SourceAccount string `json:"source_account" gorm:"type:varchar(128);index"`
	MessageId     string `json:"message_id" gorm:"type:varchar(255);index"`
	ImapUid       string `json:"imap_uid" gorm:"type:varchar(64);index"`
	RawHash       string `json:"raw_hash" gorm:"type:char(64);uniqueIndex"`
	Subject       string `json:"subject" gorm:"type:varchar(255);default:''"`
	FromAddress   string `json:"from_address" gorm:"type:varchar(255);default:''"`
	ProductName   string `json:"product_name" gorm:"type:varchar(255);default:''"`
	OrderNo       string `json:"order_no" gorm:"type:varchar(64);index"`
	PaidCents     int64  `json:"paid_cents" gorm:"type:bigint;not null;default:0"`
	Quantity      int    `json:"quantity" gorm:"type:int;not null;default:0"`
	PaymentTime   int64  `json:"payment_time" gorm:"index;default:0"`
	ContentMasked string `json:"content_masked" gorm:"type:text"`
	ParseStatus   string `json:"parse_status" gorm:"type:varchar(32);index;default:'parsed'"`
	ParseError    string `json:"parse_error" gorm:"type:varchar(512);default:''"`
	CreatedTime   int64  `json:"created_time" gorm:"index"`
}

type AffiliateCommission struct {
	Id              int    `json:"id"`
	InviterUserId   int    `json:"inviter_user_id" gorm:"index"`
	InviteeUserId   int    `json:"invitee_user_id" gorm:"index"`
	TopUpId         int    `json:"topup_id" gorm:"index;default:0"`
	SessionId       string `json:"session_id" gorm:"type:varchar(64);index"`
	TradeNo         string `json:"trade_no" gorm:"type:varchar(255);index"`
	BaseMoneyCents  int64  `json:"base_money_cents" gorm:"type:bigint;not null;default:0"`
	RateBps         int    `json:"rate_bps" gorm:"type:int;not null;default:0"`
	CommissionCents int64  `json:"commission_cents" gorm:"type:bigint;not null;default:0"`
	Status          string `json:"status" gorm:"type:varchar(32);index;default:'available'"`
	CreatedTime     int64  `json:"created_time" gorm:"index"`
	ConfirmedTime   int64  `json:"confirmed_time" gorm:"index;default:0"`
	WithdrawalId    int    `json:"withdrawal_id" gorm:"index;default:0"`
}

type AffiliateWithdrawal struct {
	Id            int    `json:"id"`
	WithdrawalId  string `json:"withdrawal_id" gorm:"uniqueIndex;type:varchar(64)"`
	UserId        int    `json:"user_id" gorm:"index"`
	AmountCents   int64  `json:"amount_cents" gorm:"type:bigint;not null;default:0"`
	Contact       string `json:"contact" gorm:"type:varchar(255);not null"`
	Remark        string `json:"remark" gorm:"type:varchar(512);default:''"`
	Status        string `json:"status" gorm:"type:varchar(32);index;default:'pending'"`
	AdminRemark   string `json:"admin_remark" gorm:"type:varchar(512);default:''"`
	CreatedTime   int64  `json:"created_time" gorm:"index"`
	ProcessedTime int64  `json:"processed_time" gorm:"index;default:0"`
	ProcessedBy   int    `json:"processed_by" gorm:"index;default:0"`
}

var (
	ErrAffiliateWithdrawalNotFound         = errors.New("affiliate withdrawal not found")
	ErrAffiliateWithdrawalAlreadyProcessed = errors.New("affiliate withdrawal already processed")
)

func MarkAffiliateWithdrawalPaid(id int, adminId int, adminRemark string) (*AffiliateWithdrawal, error) {
	return transitionAffiliateWithdrawal(id, adminId, adminRemark, AffiliateWithdrawalStatusPaid)
}

func RejectAffiliateWithdrawal(id int, adminId int, adminRemark string) (*AffiliateWithdrawal, error) {
	return transitionAffiliateWithdrawal(id, adminId, adminRemark, AffiliateWithdrawalStatusRejected)
}

func transitionAffiliateWithdrawal(id int, adminId int, adminRemark string, status string) (*AffiliateWithdrawal, error) {
	if id <= 0 {
		return nil, ErrAffiliateWithdrawalNotFound
	}

	var withdrawal AffiliateWithdrawal
	err := DB.Transaction(func(tx *gorm.DB) error {
		processedTime := common.GetTimestamp()
		result := tx.Model(&AffiliateWithdrawal{}).
			Where("id = ? AND status = ?", id, AffiliateWithdrawalStatusPending).
			Updates(map[string]interface{}{
				"status":         status,
				"processed_by":   adminId,
				"processed_time": processedTime,
				"admin_remark":   strings.TrimSpace(adminRemark),
			})
		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected == 0 {
			if err := tx.First(&withdrawal, id).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrAffiliateWithdrawalNotFound
				}
				return err
			}
			return ErrAffiliateWithdrawalAlreadyProcessed
		}

		return tx.First(&withdrawal, id).Error
	})
	if err != nil {
		return nil, err
	}

	return &withdrawal, nil
}
