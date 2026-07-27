package model

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/QuantumNous/new-api/common"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// SVIP 是身份标识（非用户组）：有效充值累计达到阈值后点亮，用于彰显身份与提示管理员，
// 不影响分组与倍率。有效充值口径（单位：分）只包含三类来源：
//  1. 联动小铺充值（卡密兑换 / 直充两条分支）
//  2. 购买超值套餐（月卡等，仅联动小铺支付成交）
//  3. 管理员增加余额且显式勾选「计入有效充值」
const (
	SVIPThresholdMoney       = 200.0
	SVIPThresholdCents int64 = 20_000

	SVIPValidTopupReconcileVersion = "svip_valid_topup_reconcile_v1"
)

type SVIPValidTopupReconcileReceipt struct {
	MigrationVersion string `json:"migration_version" gorm:"type:varchar(64);primaryKey"`
	AppliedAt        int64  `json:"applied_at" gorm:"type:bigint;not null"`
}

func IsSVIPValidTopupCents(cents int64) bool {
	return cents >= SVIPThresholdCents
}

func (user *User) IsSVIP() bool {
	if user == nil {
		return false
	}
	return IsSVIPValidTopupCents(user.ValidTopupCents)
}

// MoneyToValidTopupCents 元 → 分，四舍五入，负数按 0 处理。
func MoneyToValidTopupCents(money float64) int64 {
	if money <= 0 || math.IsNaN(money) || math.IsInf(money, 0) {
		return 0
	}
	cents := decimal.NewFromFloat(money).Mul(decimal.NewFromInt(100)).Round(0)
	if cents.GreaterThan(decimal.NewFromInt(math.MaxInt64)) {
		return 0
	}
	return cents.IntPart()
}

func AmountToValidTopupCents(amount int64) int64 {
	if amount <= 0 || amount > math.MaxInt64/100 {
		return 0
	}
	return amount * 100
}

func AddUserValidTopupCentsTx(tx *gorm.DB, userId int, cents int64) error {
	if tx == nil {
		return errors.New("nil transaction")
	}
	if userId <= 0 || cents <= 0 {
		return nil
	}
	result := tx.Model(&User{}).Where("id = ?", userId).Updates(map[string]interface{}{
		"valid_topup_cents":         gorm.Expr("valid_topup_cents + ?", cents),
		"valid_topup_history_cents": gorm.Expr("valid_topup_history_cents + ?", cents),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func AddUserValidTopupCents(userId int, cents int64) error {
	return AddUserValidTopupCentsTx(DB, userId, cents)
}

// IncreaseUserQuotaAndValidTopupCents atomically applies an admin balance credit
// and its optional SVIP accumulation to the same user row.
func IncreaseUserQuotaAndValidTopupCents(userId int, quota int, cents int64) error {
	if userId <= 0 {
		return gorm.ErrRecordNotFound
	}
	if quota <= 0 {
		return errors.New("quota must be positive")
	}
	if cents < 0 {
		return errors.New("valid topup cents cannot be negative")
	}

	updates := map[string]interface{}{
		"quota": gorm.Expr("quota + ?", quota),
	}
	if cents > 0 {
		updates["valid_topup_cents"] = gorm.Expr("valid_topup_cents + ?", cents)
	}
	result := DB.Model(&User{}).Where("id = ?", userId).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	if err := invalidateUserCache(userId); err != nil {
		common.SysLog("failed to invalidate user cache after admin quota credit: " + err.Error())
	}
	return nil
}

// validTopupCentsForTopUp 与 topUpVIPQualifiedAmount 同口径：Amount（面值，元）优先，否则 Money。
// 只应对联动小铺 / 卡密 / 超值套餐来源的 TopUp 调用，其他 provider 的 Amount 不是人民币元。
func validTopupCentsForTopUp(topUp TopUp) int64 {
	if topUp.Amount > 0 {
		return AmountToValidTopupCents(topUp.Amount)
	}
	return MoneyToValidTopupCents(topUp.Money)
}

// backfillUserValidTopupCents 从 top_ups 流水一次性回填历史有效充值：
// 联动小铺（含超值套餐）与卡密兑换的成功流水计入；Stripe/易支付/Creem/Waffo 等不计入。
// 历史上管理员手动加的余额没有流水、无法区分是否为充值，不回填（此后由开关决定）。
// 首次运行用一次性凭证把现有累计与历史流水对齐；此后按已核算历史水位补差，
// 从而保留管理员计入的额外累计，并修复回滚到旧版本期间新增但未实时累计的流水。
// 整个批次与初始化凭证在一个事务内完成。
func backfillUserValidTopupCents() error {
	var updated int64
	err := DB.Transaction(func(tx *gorm.DB) error {
		var receipt SVIPValidTopupReconcileReceipt
		receiptResult := tx.Where("migration_version = ?", SVIPValidTopupReconcileVersion).
			Limit(1).
			Find(&receipt)
		if receiptResult.Error != nil {
			return receiptResult.Error
		}
		initialized := receiptResult.RowsAffected == 1

		var topUps []TopUp
		if err := tx.Model(&TopUp{}).
			Where("status = ?", common.TopUpStatusSuccess).
			Where("payment_provider IN ?", []string{PaymentProviderLDXP, PaymentProviderRedemptionCode}).
			Find(&topUps).Error; err != nil {
			return err
		}
		totals := make(map[int]int64)
		for _, topUp := range topUps {
			cents := validTopupCentsForTopUp(topUp)
			if cents <= 0 {
				continue
			}
			if totals[topUp.UserId] > math.MaxInt64-cents {
				return errors.New("valid topup backfill total overflow")
			}
			totals[topUp.UserId] += cents
		}

		userIDs := make([]int, 0, len(totals))
		for userId := range totals {
			userIDs = append(userIDs, userId)
		}
		sort.Ints(userIDs)

		for _, userId := range userIDs {
			cents := totals[userId]
			if cents <= 0 {
				continue
			}

			var user User
			userResult := tx.Select("id", "valid_topup_cents", "valid_topup_history_cents").
				Where("id = ?", userId).
				Limit(1).
				Find(&user)
			if userResult.Error != nil {
				return userResult.Error
			}
			if userResult.RowsAffected == 0 {
				continue
			}

			accountedHistory := user.TopupWatermark
			if !initialized {
				accountedHistory = user.ValidTopupCents
				if accountedHistory < 0 {
					accountedHistory = 0
				}
				if accountedHistory > cents {
					accountedHistory = cents
				}
			}

			missingCents := int64(0)
			if cents > accountedHistory {
				missingCents = cents - accountedHistory
			}
			if missingCents > 0 && user.ValidTopupCents > math.MaxInt64-missingCents {
				return errors.New("valid topup reconciliation total overflow")
			}

			targetHistory := user.TopupWatermark
			if cents > targetHistory {
				targetHistory = cents
			}
			if missingCents == 0 && targetHistory == user.TopupWatermark {
				continue
			}

			updates := map[string]interface{}{
				"valid_topup_history_cents": targetHistory,
			}
			if missingCents > 0 {
				updates["valid_topup_cents"] = gorm.Expr("valid_topup_cents + ?", missingCents)
			}
			result := tx.Model(&User{}).
				Where("id = ? AND valid_topup_cents = ? AND valid_topup_history_cents = ?", userId, user.ValidTopupCents, user.TopupWatermark).
				Updates(updates)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return fmt.Errorf("valid topup reconciliation state changed for user %d", userId)
			}
			updated += result.RowsAffected
		}

		if !initialized {
			if err := tx.Create(&SVIPValidTopupReconcileReceipt{
				MigrationVersion: SVIPValidTopupReconcileVersion,
				AppliedAt:        common.GetTimestamp(),
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if updated > 0 {
		common.SysLog("SVIP valid topup backfill completed")
	}
	return nil
}
