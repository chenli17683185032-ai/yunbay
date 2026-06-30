package model

import (
	"errors"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const AffiliateCommissionRate = 0.15

const (
	AffiliateCommissionStatusAvailable = "available"
	AffiliateCommissionStatusCanceled  = "canceled"
)

const (
	AffiliateWithdrawalStatusPending  = "pending"
	AffiliateWithdrawalStatusPaid     = "paid"
	AffiliateWithdrawalStatusRejected = "rejected"
	AffiliateWithdrawalStatusCanceled = "canceled"
)

var (
	ErrAffiliateInvalidAmount        = errors.New("affiliate invalid amount")
	ErrAffiliateContactRequired      = errors.New("affiliate contact required")
	ErrAffiliateInsufficientBalance  = errors.New("affiliate insufficient balance")
	ErrAffiliateWithdrawalNotFound   = errors.New("affiliate withdrawal not found")
	ErrAffiliateWithdrawalBadStatus  = errors.New("affiliate withdrawal bad status")
	ErrAffiliateCommissionInvalidTop = errors.New("affiliate commission invalid topup")
)

type AffiliateCommission struct {
	Id              int     `json:"id"`
	CommissionId    string  `json:"commission_id" gorm:"type:varchar(64);uniqueIndex"`
	InviterUserId   int     `json:"inviter_user_id" gorm:"index"`
	InviteeUserId   int     `json:"invitee_user_id" gorm:"index"`
	TopupId         int     `json:"topup_id" gorm:"uniqueIndex"`
	TradeNo         string  `json:"trade_no" gorm:"type:varchar(255);index"`
	BaseMoney       float64 `json:"base_money"`
	Rate            float64 `json:"rate"`
	CommissionMoney float64 `json:"commission_money"`
	Status          string  `json:"status" gorm:"type:varchar(32);index"`
	CreatedTime     int64   `json:"created_time" gorm:"bigint"`
	UpdatedTime     int64   `json:"updated_time" gorm:"bigint"`
}

type AffiliateWithdrawal struct {
	Id            int     `json:"id"`
	WithdrawalId  string  `json:"withdrawal_id" gorm:"type:varchar(64);uniqueIndex"`
	UserId        int     `json:"user_id" gorm:"index"`
	Amount        float64 `json:"amount"`
	Contact       string  `json:"contact" gorm:"type:varchar(255)"`
	Remark        string  `json:"remark" gorm:"type:text"`
	Status        string  `json:"status" gorm:"type:varchar(32);index"`
	AdminRemark   string  `json:"admin_remark" gorm:"type:text"`
	CreatedTime   int64   `json:"created_time" gorm:"bigint"`
	UpdatedTime   int64   `json:"updated_time" gorm:"bigint"`
	ProcessedTime int64   `json:"processed_time" gorm:"bigint;default:0"`
}

type AffiliateWithdrawalListResult struct {
	Items []*AffiliateWithdrawal `json:"items"`
	Total int64                  `json:"total"`
}

type AffiliateSummary struct {
	AffCode        string  `json:"aff_code"`
	InviteCount    int64   `json:"invite_count"`
	Rate           float64 `json:"rate"`
	TotalMoney     float64 `json:"total_money"`
	AvailableMoney float64 `json:"available_money"`
	FrozenMoney    float64 `json:"frozen_money"`
	WithdrawnMoney float64 `json:"withdrawn_money"`
}

func roundAffiliateMoney(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return value
	}
	rounded, _ := decimal.NewFromFloat(value).Round(2).Float64()
	return rounded
}

func newAffiliatePublicID(prefix string) string {
	return prefix + strings.ReplaceAll(common.GetUUID(), "-", "")
}

func normalizeAffiliateWithdrawalInput(amount float64, contact string, remark string) (float64, string, string, error) {
	amount = roundAffiliateMoney(amount)
	if amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return 0, "", "", ErrAffiliateInvalidAmount
	}

	contact = strings.TrimSpace(contact)
	if contact == "" {
		return 0, "", "", ErrAffiliateContactRequired
	}
	contactRunes := []rune(contact)
	if len(contactRunes) > 255 {
		contact = string(contactRunes[:255])
	}

	return amount, contact, strings.TrimSpace(remark), nil
}

func MaybeCreateAffiliateCommissionForTopUpTx(tx *gorm.DB, topUp *TopUp) error {
	if tx == nil {
		return gorm.ErrInvalidDB
	}
	if topUp == nil {
		return ErrAffiliateCommissionInvalidTop
	}
	if topUp.Id <= 0 || topUp.UserId <= 0 || topUp.Status != common.TopUpStatusSuccess || topUp.Money <= 0 {
		return nil
	}

	var existing AffiliateCommission
	if err := tx.Select("id").Where("topup_id = ?", topUp.Id).First(&existing).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	} else {
		return nil
	}

	var invitee User
	if err := tx.Select("id", "inviter_id").Where("id = ?", topUp.UserId).First(&invitee).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if invitee.InviterId <= 0 || invitee.InviterId == invitee.Id {
		return nil
	}

	var inviter User
	if err := tx.Select("id").Where("id = ?", invitee.InviterId).First(&inviter).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	commissionMoney := roundAffiliateMoney(topUp.Money * AffiliateCommissionRate)
	if commissionMoney <= 0 || math.IsNaN(commissionMoney) || math.IsInf(commissionMoney, 0) {
		return nil
	}

	now := common.GetTimestamp()
	commission := AffiliateCommission{
		CommissionId:    newAffiliatePublicID("affc_"),
		InviterUserId:   inviter.Id,
		InviteeUserId:   invitee.Id,
		TopupId:         topUp.Id,
		TradeNo:         topUp.TradeNo,
		BaseMoney:       roundAffiliateMoney(topUp.Money),
		Rate:            AffiliateCommissionRate,
		CommissionMoney: commissionMoney,
		Status:          AffiliateCommissionStatusAvailable,
		CreatedTime:     now,
		UpdatedTime:     now,
	}
	return tx.Create(&commission).Error
}

func MaybeCreateAffiliateCommissionForTopUp(topupID int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var topUp TopUp
		if err := tx.Where("id = ?", topupID).First(&topUp).Error; err != nil {
			return err
		}
		return MaybeCreateAffiliateCommissionForTopUpTx(tx, &topUp)
	})
}

func lockAffiliateUserTx(tx *gorm.DB, userID int) (*User, error) {
	if tx == nil {
		return nil, gorm.ErrInvalidDB
	}
	query := tx.Where("id = ?", userID)
	if !common.UsingSQLite {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var user User
	if err := query.First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func GetAffiliateSummary(userID int) (*AffiliateSummary, error) {
	return GetAffiliateSummaryTx(DB, userID)
}

func GetAffiliateSummaryTx(tx *gorm.DB, userID int) (*AffiliateSummary, error) {
	if tx == nil {
		return nil, gorm.ErrInvalidDB
	}

	var user User
	if err := tx.Select("id", "aff_code").Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}

	summary := &AffiliateSummary{
		AffCode: user.AffCode,
		Rate:    AffiliateCommissionRate,
	}
	if err := tx.Model(&User{}).Where("inviter_id = ?", userID).Count(&summary.InviteCount).Error; err != nil {
		return nil, err
	}

	var totalCommission float64
	if err := tx.Model(&AffiliateCommission{}).
		Where("inviter_user_id = ? AND status = ?", userID, AffiliateCommissionStatusAvailable).
		Select("COALESCE(SUM(commission_money), 0)").
		Scan(&totalCommission).Error; err != nil {
		return nil, err
	}

	var frozenMoney float64
	if err := tx.Model(&AffiliateWithdrawal{}).
		Where("user_id = ? AND status = ?", userID, AffiliateWithdrawalStatusPending).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&frozenMoney).Error; err != nil {
		return nil, err
	}

	var withdrawnMoney float64
	if err := tx.Model(&AffiliateWithdrawal{}).
		Where("user_id = ? AND status = ?", userID, AffiliateWithdrawalStatusPaid).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&withdrawnMoney).Error; err != nil {
		return nil, err
	}

	summary.TotalMoney = roundAffiliateMoney(totalCommission)
	summary.FrozenMoney = roundAffiliateMoney(frozenMoney)
	summary.WithdrawnMoney = roundAffiliateMoney(withdrawnMoney)
	summary.AvailableMoney = roundAffiliateMoney(math.Max(summary.TotalMoney-summary.FrozenMoney-summary.WithdrawnMoney, 0))
	return summary, nil
}

func CreateAffiliateWithdrawal(userID int, amount float64, contact string, remark string) (*AffiliateWithdrawal, error) {
	amount, contact, remark, err := normalizeAffiliateWithdrawalInput(amount, contact, remark)
	if err != nil {
		return nil, err
	}

	var withdrawal *AffiliateWithdrawal
	err = DB.Transaction(func(tx *gorm.DB) error {
		created, err := CreateAffiliateWithdrawalTx(tx, userID, amount, contact, remark)
		if err != nil {
			return err
		}
		withdrawal = created
		return nil
	})
	if err != nil {
		return nil, err
	}
	return withdrawal, nil
}

func CreateAffiliateWithdrawalTx(tx *gorm.DB, userID int, amount float64, contact string, remark string) (*AffiliateWithdrawal, error) {
	if tx == nil {
		return nil, gorm.ErrInvalidDB
	}
	amount, contact, remark, err := normalizeAffiliateWithdrawalInput(amount, contact, remark)
	if err != nil {
		return nil, err
	}
	if _, err = lockAffiliateUserTx(tx, userID); err != nil {
		return nil, err
	}

	summary, err := GetAffiliateSummaryTx(tx, userID)
	if err != nil {
		return nil, err
	}
	if summary.AvailableMoney < amount {
		return nil, ErrAffiliateInsufficientBalance
	}

	now := common.GetTimestamp()
	withdrawal := &AffiliateWithdrawal{
		WithdrawalId: newAffiliatePublicID("affw_"),
		UserId:       userID,
		Amount:       amount,
		Contact:      contact,
		Remark:       remark,
		Status:       AffiliateWithdrawalStatusPending,
		CreatedTime:  now,
		UpdatedTime:  now,
	}
	if err = tx.Create(withdrawal).Error; err != nil {
		return nil, err
	}
	return withdrawal, nil
}

func loadPendingAffiliateWithdrawalForUpdate(tx *gorm.DB, id int) (*AffiliateWithdrawal, error) {
	if tx == nil {
		return nil, gorm.ErrInvalidDB
	}
	if id <= 0 {
		return nil, ErrAffiliateWithdrawalNotFound
	}

	query := tx.Where("id = ?", id)
	if !common.UsingSQLite {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var withdrawal AffiliateWithdrawal
	if err := query.First(&withdrawal).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAffiliateWithdrawalNotFound
		}
		return nil, err
	}
	if withdrawal.Status != AffiliateWithdrawalStatusPending {
		return nil, ErrAffiliateWithdrawalBadStatus
	}
	return &withdrawal, nil
}

func MarkAffiliateWithdrawalPaid(id int, adminRemark string) (*AffiliateWithdrawal, error) {
	var withdrawal *AffiliateWithdrawal
	err := DB.Transaction(func(tx *gorm.DB) error {
		loaded, err := loadPendingAffiliateWithdrawalForUpdate(tx, id)
		if err != nil {
			return err
		}
		now := common.GetTimestamp()
		loaded.Status = AffiliateWithdrawalStatusPaid
		loaded.AdminRemark = strings.TrimSpace(adminRemark)
		loaded.UpdatedTime = now
		loaded.ProcessedTime = now
		if err = tx.Save(loaded).Error; err != nil {
			return err
		}
		withdrawal = loaded
		return nil
	})
	if err != nil {
		return nil, err
	}
	return withdrawal, nil
}

func RejectAffiliateWithdrawal(id int, adminRemark string) (*AffiliateWithdrawal, error) {
	var withdrawal *AffiliateWithdrawal
	err := DB.Transaction(func(tx *gorm.DB) error {
		loaded, err := loadPendingAffiliateWithdrawalForUpdate(tx, id)
		if err != nil {
			return err
		}
		now := common.GetTimestamp()
		loaded.Status = AffiliateWithdrawalStatusRejected
		loaded.AdminRemark = strings.TrimSpace(adminRemark)
		loaded.UpdatedTime = now
		loaded.ProcessedTime = now
		if err = tx.Save(loaded).Error; err != nil {
			return err
		}
		withdrawal = loaded
		return nil
	})
	if err != nil {
		return nil, err
	}
	return withdrawal, nil
}

func GetAffiliateWithdrawals(pageInfo *common.PageInfo, status string) (*AffiliateWithdrawalListResult, error) {
	if pageInfo == nil {
		pageInfo = &common.PageInfo{
			Page:     1,
			PageSize: common.ItemsPerPage,
		}
	}
	if pageInfo.Page < 1 {
		pageInfo.Page = 1
	}
	if pageInfo.PageSize <= 0 {
		pageInfo.PageSize = common.ItemsPerPage
	}

	query := DB.Model(&AffiliateWithdrawal{})
	if status = strings.TrimSpace(status); status != "" {
		query = query.Where("status = ?", status)
	}

	result := &AffiliateWithdrawalListResult{}
	if err := query.Count(&result.Total).Error; err != nil {
		return nil, err
	}
	if err := query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&result.Items).Error; err != nil {
		return nil, err
	}
	return result, nil
}
