package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"gorm.io/gorm"
)

type Redemption struct {
	Id           int            `json:"id"`
	UserId       int            `json:"user_id"`
	Key          string         `json:"key" gorm:"type:char(32);uniqueIndex"`
	Status       int            `json:"status" gorm:"default:1"`
	Name         string         `json:"name" gorm:"index"`
	Quota        int            `json:"quota" gorm:"default:100"`
	Kind         string         `json:"kind" gorm:"type:varchar(32);default:'legacy';index"`
	Amount       int64          `json:"amount" gorm:"default:0"`
	Money        float64        `json:"money" gorm:"default:0"`
	CountAsTopUp bool           `json:"count_as_topup" gorm:"default:false"`
	BatchId      string         `json:"batch_id" gorm:"type:varchar(64);default:'';index"`
	Source       string         `json:"source" gorm:"type:varchar(32);default:'manual';index"`
	ExportedTime int64          `json:"exported_time" gorm:"bigint;default:0"`
	CreatedTime  int64          `json:"created_time" gorm:"bigint"`
	RedeemedTime int64          `json:"redeemed_time" gorm:"bigint"`
	Count        int            `json:"count" gorm:"-:all"` // only for api request
	UsedUserId   int            `json:"used_user_id"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
	ExpiredTime  int64          `json:"expired_time" gorm:"bigint"` // 过期时间，0 表示不过期
}

const (
	RedemptionKindLegacy      = "legacy"
	RedemptionKindPaidTopUp   = "paid_topup"
	RedemptionKindPromoCredit = "promo_credit"
	RedemptionKindCoupon      = "coupon"
)

const (
	RedemptionSourceManual = "manual"
	RedemptionSourceLDXP   = "ldxp"
	RedemptionSourcePromo  = "promo"
)

type RedeemRedemptionMeta struct {
	Id           int     `json:"id"`
	Kind         string  `json:"kind"`
	Quota        int     `json:"quota"`
	Amount       int64   `json:"amount"`
	Money        float64 `json:"money"`
	CountAsTopUp bool    `json:"count_as_topup"`
	BatchId      string  `json:"batch_id"`
	Source       string  `json:"source"`
}

type RedeemResult struct {
	Quota      int                  `json:"quota"`
	Redemption RedeemRedemptionMeta `json:"redemption"`
}

func GetAllRedemptions(startIdx int, num int) (redemptions []*Redemption, total int64, err error) {
	// 开始事务
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获取总数
	err = tx.Model(&Redemption{}).Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// 获取分页数据
	err = tx.Order("id desc").Limit(num).Offset(startIdx).Find(&redemptions).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// 提交事务
	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return redemptions, total, nil
}

func SearchRedemptions(keyword string, startIdx int, num int) (redemptions []*Redemption, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Build query based on keyword type
	query := tx.Model(&Redemption{})

	pattern := keyword + "%"
	// Only try to convert to ID if the string represents a valid integer
	if id, err := strconv.Atoi(keyword); err == nil {
		query = query.Where("id = ? OR name LIKE ? OR batch_id LIKE ?", id, pattern, pattern)
	} else {
		query = query.Where("name LIKE ? OR batch_id LIKE ?", pattern, pattern)
	}

	// Get total count
	err = query.Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// Get paginated data
	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&redemptions).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return redemptions, total, nil
}

func GetRedemptionById(id int) (*Redemption, error) {
	if id == 0 {
		return nil, errors.New("id 为空！")
	}
	redemption := Redemption{Id: id}
	var err error = nil
	err = DB.First(&redemption, "id = ?", id).Error
	return &redemption, err
}

func GenerateRedemptionBatchId(source string, amount int64, now int64) string {
	if source == "" {
		source = RedemptionSourceManual
	}
	return fmt.Sprintf("RDM%s%d%d", strings.ToUpper(source), amount, now)
}

func CreateRedemptionTopUpTradeNo(redemptionID int, userID int) string {
	return fmt.Sprintf("RDM%dU%d", redemptionID, userID)
}

func NormalizeRedemptionForCreate(redemption *Redemption) error {
	if redemption == nil {
		return ErrRedemptionInvalid
	}
	redemption.Kind = strings.TrimSpace(redemption.Kind)
	redemption.Source = strings.TrimSpace(redemption.Source)
	redemption.BatchId = strings.TrimSpace(redemption.BatchId)
	if redemption.Kind == "" {
		redemption.Kind = RedemptionKindPromoCredit
	}
	if redemption.Source == "" {
		if redemption.Kind == RedemptionKindPaidTopUp {
			redemption.Source = RedemptionSourceLDXP
		} else {
			redemption.Source = RedemptionSourcePromo
		}
	}
	if redemption.BatchId == "" {
		redemption.BatchId = GenerateRedemptionBatchId(redemption.Source, redemption.Amount, common.GetTimestamp())
	}
	return ValidateRedemptionKindForCreate(redemption)
}

func ValidateRedemptionKindForCreate(redemption *Redemption) error {
	if redemption == nil {
		return ErrRedemptionInvalid
	}
	switch redemption.Kind {
	case RedemptionKindPaidTopUp:
		if redemption.Quota <= 0 || redemption.Amount <= 0 || redemption.Money <= 0 || !redemption.CountAsTopUp {
			return ErrRedemptionInvalid
		}
		return nil
	case RedemptionKindPromoCredit:
		if redemption.Quota <= 0 || redemption.Amount < 0 || redemption.Money < 0 || redemption.CountAsTopUp {
			return ErrRedemptionInvalid
		}
		return nil
	default:
		return ErrRedemptionUnsupportedKind
	}
}

func normalizeRedeemedRedemption(redemption *Redemption) {
	if redemption.Kind == "" {
		redemption.Kind = RedemptionKindLegacy
	}
	if redemption.Source == "" {
		redemption.Source = RedemptionSourceManual
	}
}

func buildRedeemResult(redemption *Redemption) *RedeemResult {
	normalizeRedeemedRedemption(redemption)
	return &RedeemResult{
		Quota: redemption.Quota,
		Redemption: RedeemRedemptionMeta{
			Id:           redemption.Id,
			Kind:         redemption.Kind,
			Quota:        redemption.Quota,
			Amount:       redemption.Amount,
			Money:        redemption.Money,
			CountAsTopUp: redemption.CountAsTopUp,
			BatchId:      redemption.BatchId,
			Source:       redemption.Source,
		},
	}
}

func GetRedemptionsByBatchId(batchId string) ([]*Redemption, error) {
	batchId = strings.TrimSpace(batchId)
	if batchId == "" {
		return nil, ErrRedemptionInvalid
	}
	var redemptions []*Redemption
	err := DB.Where("batch_id = ?", batchId).Order("id asc").Find(&redemptions).Error
	return redemptions, err
}

func MarkRedemptionsExported(batchId string, exportedTime int64) error {
	batchId = strings.TrimSpace(batchId)
	if batchId == "" {
		return ErrRedemptionInvalid
	}
	return DB.Model(&Redemption{}).Where("batch_id = ?", batchId).Update("exported_time", exportedTime).Error
}

func redeemError(err error) error {
	if errors.Is(err, ErrRedemptionNotProvided) ||
		errors.Is(err, ErrRedemptionInvalid) ||
		errors.Is(err, ErrRedemptionUsed) ||
		errors.Is(err, ErrRedemptionExpired) ||
		errors.Is(err, ErrRedemptionUnsupportedKind) {
		return err
	}
	common.SysError("redemption failed: " + err.Error())
	return ErrRedeemFailed
}

func Redeem(key string, userId int) (*RedeemResult, error) {
	if key == "" {
		return nil, ErrRedemptionNotProvided
	}
	if userId == 0 {
		return nil, ErrRedemptionInvalid
	}
	redemption := &Redemption{}

	common.RandomSleep()
	err := DB.Transaction(func(tx *gorm.DB) error {
		err := tx.Set("gorm:query_option", "FOR UPDATE").Where(commonKeyCol+" = ?", key).First(redemption).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRedemptionInvalid
			}
			return err
		}
		if redemption.Status != common.RedemptionCodeStatusEnabled {
			return ErrRedemptionUsed
		}
		now := common.GetTimestamp()
		if redemption.ExpiredTime != 0 && redemption.ExpiredTime < now {
			return ErrRedemptionExpired
		}
		normalizeRedeemedRedemption(redemption)
		if redemption.Kind == RedemptionKindCoupon || (redemption.Kind != RedemptionKindLegacy && redemption.Kind != RedemptionKindPaidTopUp && redemption.Kind != RedemptionKindPromoCredit) {
			return ErrRedemptionUnsupportedKind
		}
		if redemption.Kind == RedemptionKindPaidTopUp && (!redemption.CountAsTopUp || redemption.Amount <= 0 || redemption.Money <= 0) {
			return ErrRedemptionInvalid
		}
		if redemption.Kind == RedemptionKindPromoCredit && redemption.CountAsTopUp {
			return ErrRedemptionInvalid
		}
		result := tx.Model(&User{}).Where("id = ?", userId).Update("quota", gorm.Expr("quota + ?", redemption.Quota))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrRedemptionInvalid
		}
		redemption.RedeemedTime = now
		redemption.Status = common.RedemptionCodeStatusUsed
		redemption.UsedUserId = userId
		if err = tx.Save(redemption).Error; err != nil {
			return err
		}
		if redemption.Kind == RedemptionKindPaidTopUp && redemption.CountAsTopUp {
			topUp := &TopUp{
				UserId:          userId,
				Amount:          redemption.Amount,
				Money:           redemption.Money,
				TradeNo:         CreateRedemptionTopUpTradeNo(redemption.Id, userId),
				PaymentMethod:   PaymentMethodRedemptionCode,
				PaymentProvider: PaymentProviderRedemptionCode,
				CreateTime:      now,
				CompleteTime:    now,
				Status:          common.TopUpStatusSuccess,
			}
			return tx.Create(topUp).Error
		}
		return nil
	})
	if err != nil {
		return nil, redeemError(err)
	}
	RecordLog(userId, LogTypeTopup, fmt.Sprintf("通过兑换码充值 %s，兑换码ID %d", logger.LogQuota(redemption.Quota), redemption.Id))
	return buildRedeemResult(redemption), nil
}

func (redemption *Redemption) Insert() error {
	var err error
	err = DB.Create(redemption).Error
	return err
}

func (redemption *Redemption) SelectUpdate() error {
	// This can update zero values
	return DB.Model(redemption).Select("redeemed_time", "status").Updates(redemption).Error
}

// Update Make sure your token's fields is completed, because this will update non-zero values
func (redemption *Redemption) Update() error {
	var err error
	err = DB.Model(redemption).Select("name", "status", "quota", "redeemed_time", "expired_time").Updates(redemption).Error
	return err
}

func (redemption *Redemption) Delete() error {
	var err error
	err = DB.Delete(redemption).Error
	return err
}

func DeleteRedemptionById(id int) (err error) {
	if id == 0 {
		return errors.New("id 为空！")
	}
	redemption := Redemption{Id: id}
	err = DB.Where(redemption).First(&redemption).Error
	if err != nil {
		return err
	}
	return redemption.Delete()
}

func DeleteInvalidRedemptions() (int64, error) {
	now := common.GetTimestamp()
	result := DB.Where("status IN ? OR (status = ? AND expired_time != 0 AND expired_time < ?)", []int{common.RedemptionCodeStatusUsed, common.RedemptionCodeStatusDisabled}, common.RedemptionCodeStatusEnabled, now).Delete(&Redemption{})
	return result.RowsAffected, result.Error
}
