package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Redemption struct {
	Id           int            `json:"id"`
	UserId       int            `json:"user_id"`
	Key          string         `json:"key" gorm:"type:char(32);uniqueIndex"`
	Status       int            `json:"status" gorm:"default:1"`
	Name         string         `json:"name" gorm:"index"`
	Quota        int            `json:"quota" gorm:"default:100"`
	Type         string         `json:"type" gorm:"type:varchar(32);default:'quota';index"`
	PlanId       int            `json:"plan_id" gorm:"index;default:0"`
	PlanTitle    string         `json:"plan_title,omitempty" gorm:"-:all"`
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
	// 重置卡兑换码：每个码兑换后发放的重置次数
	ResetCardCount int `json:"reset_card_count" gorm:"default:0"`
	// 兑换套餐码时按套餐配置赠送的重置卡张数，仅兑换结果使用
	GiftResetCount int `json:"gift_reset_count,omitempty" gorm:"-:all"`
}

const (
	RedemptionTypeQuota        = "quota"
	RedemptionTypeSubscription = "subscription"
	RedemptionTypeResetCard    = "reset_card"
)

const MaxRedemptionResetCardCount = 100

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
	Id             int     `json:"id"`
	Type           string  `json:"type"`
	PlanId         int     `json:"plan_id,omitempty"`
	PlanTitle      string  `json:"plan_title,omitempty"`
	Kind           string  `json:"kind"`
	Quota          int     `json:"quota"`
	Amount         int64   `json:"amount"`
	Money          float64 `json:"money"`
	CountAsTopUp   bool    `json:"count_as_topup"`
	BatchId        string  `json:"batch_id"`
	Source         string  `json:"source"`
	ResetCardCount int     `json:"reset_card_count,omitempty"`
}

type RedeemResult struct {
	Type      string `json:"type"`
	Quota     int    `json:"quota,omitempty"`
	PlanId    int    `json:"plan_id,omitempty"`
	PlanTitle string `json:"plan_title,omitempty"`
	// 重置卡兑换码发放的重置次数
	ResetCardCount int `json:"reset_card_count,omitempty"`
	// 套餐开通时按套餐配置赠送的重置卡张数
	GiftResetCount int                  `json:"gift_reset_count,omitempty"`
	Redemption     RedeemRedemptionMeta `json:"redemption"`
}

// RedemptionStatusFilterExpired is the virtual status filter value for enabled
// codes whose expired_time has passed; it is not a real DB status.
const RedemptionStatusFilterExpired = "expired"

// applyRedemptionStatusFilter narrows a redemption list query by the status
// filter sent from the admin UI. Supported values: "1" (enabled and not
// expired), "2" (disabled), "3" (used) and the virtual "expired" (enabled but
// past expired_time). Multiple values may be comma-separated and are OR-ed
// together; unrecognized values are ignored.
func applyRedemptionStatusFilter(query *gorm.DB, statusFilter string) *gorm.DB {
	statusFilter = strings.TrimSpace(statusFilter)
	if statusFilter == "" {
		return query
	}
	now := common.GetTimestamp()
	var conditions []string
	var args []interface{}
	for _, value := range strings.Split(statusFilter, ",") {
		switch strings.TrimSpace(value) {
		case "":
			continue
		case RedemptionStatusFilterExpired:
			conditions = append(conditions, "(status = ? AND expired_time != 0 AND expired_time < ?)")
			args = append(args, common.RedemptionCodeStatusEnabled, now)
		case strconv.Itoa(common.RedemptionCodeStatusEnabled):
			conditions = append(conditions, "(status = ? AND (expired_time = 0 OR expired_time >= ?))")
			args = append(args, common.RedemptionCodeStatusEnabled, now)
		default:
			status, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				continue
			}
			conditions = append(conditions, "status = ?")
			args = append(args, status)
		}
	}
	if len(conditions) == 0 {
		return query
	}
	return query.Where(strings.Join(conditions, " OR "), args...)
}

func GetAllRedemptions(startIdx int, num int, statusFilter string) (redemptions []*Redemption, total int64, err error) {
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

	query := applyRedemptionStatusFilter(tx.Model(&Redemption{}), statusFilter)

	// 获取总数
	err = query.Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// 获取分页数据
	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&redemptions).Error
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

func SearchRedemptions(keyword string, startIdx int, num int, statusFilter string) (redemptions []*Redemption, total int64, err error) {
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
	query = applyRedemptionStatusFilter(query, statusFilter)

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

func normalizeRedemptionType(redemptionType string) string {
	switch strings.TrimSpace(redemptionType) {
	case "", RedemptionTypeQuota:
		return RedemptionTypeQuota
	case RedemptionTypeSubscription:
		return RedemptionTypeSubscription
	case RedemptionTypeResetCard:
		return RedemptionTypeResetCard
	default:
		return ""
	}
}

func validateRedemptionPlanForCreate(redemption *Redemption) error {
	if redemption == nil {
		return ErrRedemptionInvalid
	}
	redemption.Type = normalizeRedemptionType(redemption.Type)
	if redemption.Type == "" {
		return ErrRedemptionInvalid
	}
	if redemption.Type == RedemptionTypeResetCard {
		if redemption.ResetCardCount == 0 {
			redemption.ResetCardCount = 1
		}
		if redemption.ResetCardCount < 1 || redemption.ResetCardCount > MaxRedemptionResetCardCount {
			return ErrRedemptionResetCardCountInvalid
		}
		redemption.PlanId = 0
		redemption.Quota = 0
		redemption.Kind = RedemptionKindPromoCredit
		redemption.Amount = 0
		redemption.Money = 0
		redemption.CountAsTopUp = false
		if strings.TrimSpace(redemption.Source) == "" {
			redemption.Source = RedemptionSourcePromo
		}
		return nil
	}
	if redemption.Type != RedemptionTypeSubscription {
		redemption.ResetCardCount = 0
		return nil
	}
	if redemption.PlanId <= 0 {
		return ErrRedemptionInvalid
	}
	if _, err := getSubscriptionPlanByIdFreshTx(nil, redemption.PlanId); err != nil {
		return err
	}
	redemption.ResetCardCount = 0
	redemption.Quota = 0
	redemption.Kind = RedemptionKindPromoCredit
	redemption.Amount = 0
	redemption.Money = 0
	redemption.CountAsTopUp = false
	if strings.TrimSpace(redemption.Source) == "" {
		redemption.Source = RedemptionSourcePromo
	}
	return nil
}

func ValidateRedemptionForCreate(redemption *Redemption) error {
	if err := NormalizeRedemptionForCreate(redemption); err != nil {
		return err
	}
	return nil
}

func GenerateRedemptionBatchId(source string, amount int64, now int64) string {
	if source == "" {
		source = RedemptionSourceManual
	}
	suffix, err := common.GenerateRandomCharsKey(8)
	if err != nil {
		suffix = common.GetUUID()[:8]
	}

	amountPart := strconv.FormatInt(amount, 10)
	timePart := strconv.FormatInt(now, 10)
	sourcePart := strings.ToUpper(source)
	maxSourceLen := 64 - len("RDM") - len(amountPart) - len(timePart) - len("-") - len(suffix)
	if maxSourceLen < 0 {
		maxSourceLen = 0
	}
	if len(sourcePart) > maxSourceLen {
		sourcePart = sourcePart[:maxSourceLen]
	}
	return fmt.Sprintf("RDM%s%s%s-%s", sourcePart, amountPart, timePart, suffix)
}

func CreateRedemptionTopUpTradeNo(redemptionID int, userID int) string {
	return fmt.Sprintf("RDM%dU%d", redemptionID, userID)
}

func NormalizeRedemptionForCreate(redemption *Redemption) error {
	if redemption == nil {
		return ErrRedemptionInvalid
	}
	if err := validateRedemptionPlanForCreate(redemption); err != nil {
		return err
	}
	if redemption.Type == RedemptionTypeSubscription || redemption.Type == RedemptionTypeResetCard {
		if redemption.BatchId == "" {
			redemption.BatchId = GenerateRedemptionBatchId(redemption.Source, redemption.Amount, common.GetTimestamp())
		}
		return nil
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
	redemption.Type = normalizeRedemptionType(redemption.Type)
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
		Type:           redemption.Type,
		Quota:          redemption.Quota,
		PlanId:         redemption.PlanId,
		PlanTitle:      redemption.PlanTitle,
		ResetCardCount: redemption.ResetCardCount,
		GiftResetCount: redemption.GiftResetCount,
		Redemption: RedeemRedemptionMeta{
			Id:             redemption.Id,
			Type:           redemption.Type,
			PlanId:         redemption.PlanId,
			PlanTitle:      redemption.PlanTitle,
			Kind:           redemption.Kind,
			Quota:          redemption.Quota,
			Amount:         redemption.Amount,
			Money:          redemption.Money,
			CountAsTopUp:   redemption.CountAsTopUp,
			BatchId:        redemption.BatchId,
			Source:         redemption.Source,
			ResetCardCount: redemption.ResetCardCount,
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

	common.RandomSleep()
	var result *RedeemResult
	vipUpgraded := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		result, vipUpgraded, err = redeemWithTxResult(tx, key, userId)
		return err
	})
	if err != nil {
		return nil, err
	}
	if vipUpgraded {
		_ = UpdateUserGroupCache(userId, UserGroupVIP)
	}
	RecordRedeemLog(userId, result)
	return result, nil
}

func RedeemWithTx(tx *gorm.DB, key string, userId int) (*RedeemResult, error) {
	result, _, err := redeemWithTxResult(tx, key, userId)
	return result, err
}

func redeemWithTxResult(tx *gorm.DB, key string, userId int) (*RedeemResult, bool, error) {
	if tx == nil {
		return nil, false, ErrRedemptionInvalid
	}
	if key == "" {
		return nil, false, ErrRedemptionNotProvided
	}
	if userId == 0 {
		return nil, false, ErrRedemptionInvalid
	}
	redemption := &Redemption{}
	vipUpgraded, err := redeemWithTx(tx, key, userId, redemption)
	if err != nil {
		return nil, false, redeemError(err)
	}
	return buildRedeemResult(redemption), vipUpgraded, nil
}

func claimRedemptionCodeTx(tx *gorm.DB, redemption *Redemption, userId int, now int64) error {
	claim := tx.Model(&Redemption{}).
		Where("id = ? AND status = ?", redemption.Id, common.RedemptionCodeStatusEnabled).
		Updates(map[string]interface{}{
			"status":        common.RedemptionCodeStatusUsed,
			"redeemed_time": now,
			"used_user_id":  userId,
		})
	if claim.Error != nil {
		return claim.Error
	}
	if claim.RowsAffected != 1 {
		return ErrRedemptionUsed
	}
	redemption.RedeemedTime = now
	redemption.Status = common.RedemptionCodeStatusUsed
	redemption.UsedUserId = userId
	return nil
}

func redeemWithTx(tx *gorm.DB, key string, userId int, redemption *Redemption) (bool, error) {
	query := tx.Where(commonKeyCol+" = ?", key)
	if !common.UsingSQLite {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := query.First(redemption).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, ErrRedemptionInvalid
		}
		return false, err
	}
	if redemption.Status != common.RedemptionCodeStatusEnabled {
		return false, ErrRedemptionUsed
	}
	now := common.GetTimestamp()
	if redemption.ExpiredTime != 0 && redemption.ExpiredTime < now {
		return false, ErrRedemptionExpired
	}
	normalizeRedeemedRedemption(redemption)
	if redemption.Type == "" {
		return false, ErrRedemptionInvalid
	}

	if redemption.Type == RedemptionTypeSubscription {
		plan, err := getSubscriptionPlanByIdFreshTx(tx, redemption.PlanId)
		if err != nil {
			return false, err
		}
		if err := claimRedemptionCodeTx(tx, redemption, userId, now); err != nil {
			return false, err
		}
		if plan.IsValuePackage() {
			_, err = CreateValuePackageSubscriptionFromPlanTx(tx, userId, plan, "redemption")
		} else {
			_, err = CreateUserSubscriptionFromPlanTx(tx, userId, plan, "redemption")
		}
		if err != nil {
			return false, err
		}
		redemption.Quota = 0
		redemption.PlanTitle = plan.Title
		if plan.IsValuePackage() {
			redemption.GiftResetCount = ClampSubscriptionPlanGiftResetCount(plan.GiftResetCount)
		}
		return false, nil
	}

	if redemption.Type == RedemptionTypeResetCard {
		if redemption.ResetCardCount <= 0 || redemption.ResetCardCount > MaxRedemptionResetCardCount {
			return false, ErrRedemptionInvalid
		}
		if err := ensureExistingUserForUpdateTx(tx, userId); err != nil {
			return false, err
		}
		if err := claimRedemptionCodeTx(tx, redemption, userId, now); err != nil {
			return false, err
		}
		if _, err := grantValuePackageResetCountTx(tx, userId, redemption.ResetCardCount,
			ValuePackageResetCountLedgerSourceRedemption, userId,
			fmt.Sprintf("兑换码兑换重置卡，兑换码ID %d", redemption.Id)); err != nil {
			return false, err
		}
		redemption.Quota = 0
		return false, nil
	}

	if redemption.Kind == RedemptionKindCoupon || (redemption.Kind != RedemptionKindLegacy && redemption.Kind != RedemptionKindPaidTopUp && redemption.Kind != RedemptionKindPromoCredit) {
		return false, ErrRedemptionUnsupportedKind
	}
	if redemption.Kind == RedemptionKindPaidTopUp && (!redemption.CountAsTopUp || redemption.Amount <= 0 || redemption.Money <= 0) {
		return false, ErrRedemptionInvalid
	}
	if redemption.Kind == RedemptionKindPromoCredit && redemption.CountAsTopUp {
		return false, ErrRedemptionInvalid
	}
	if err := claimRedemptionCodeTx(tx, redemption, userId, now); err != nil {
		return false, err
	}
	result := tx.Model(&User{}).Where("id = ?", userId).Update("quota", gorm.Expr("quota + ?", redemption.Quota))
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected != 1 {
		return false, ErrRedemptionInvalid
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
		if err := tx.Create(topUp).Error; err != nil {
			return false, err
		}
		if err := AddUserValidTopupCentsTx(tx, userId, validTopupCentsForTopUp(*topUp)); err != nil {
			return false, err
		}
		if err := MaybeCreateAffiliateCommissionForTopUpTx(tx, topUp); err != nil {
			return false, err
		}
		upgraded, err := MaybeUpgradeUserToVIPTx(tx, userId)
		if err != nil {
			return false, err
		}
		return upgraded, nil
	}
	return false, nil
}

func RecordRedeemLog(userId int, result *RedeemResult) {
	if result == nil {
		return
	}
	if result.Type == RedemptionTypeSubscription {
		if result.GiftResetCount > 0 {
			RecordLog(userId, LogTypeTopup, fmt.Sprintf("通过兑换码开通套餐 %s，赠送 %d 张重置卡，兑换码ID %d", result.PlanTitle, result.GiftResetCount, result.Redemption.Id))
			return
		}
		RecordLog(userId, LogTypeTopup, fmt.Sprintf("通过兑换码开通套餐 %s，兑换码ID %d", result.PlanTitle, result.Redemption.Id))
		return
	}
	if result.Type == RedemptionTypeResetCard {
		RecordLog(userId, LogTypeTopup, fmt.Sprintf("通过兑换码获得 %d 张重置卡，兑换码ID %d", result.ResetCardCount, result.Redemption.Id))
		return
	}
	RecordLog(userId, LogTypeTopup, fmt.Sprintf("通过兑换码充值 %s，兑换码ID %d", logger.LogQuota(result.Quota), result.Redemption.Id))
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
	err = DB.Model(redemption).Select("name", "status", "quota", "type", "plan_id", "kind", "amount", "money", "count_as_topup", "batch_id", "source", "redeemed_time", "expired_time", "reset_card_count").Updates(redemption).Error
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
