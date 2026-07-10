package model

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	LegacyValuePackageQuotaMigrationVersion                = "value_package_quota_b2_v1"
	legacyValuePackageQuotaMigrationSkipMissingPlan        = "missing_plan"
	legacyValuePackageQuotaMigrationSkipNotValuePackage    = "not_value_package"
	legacyValuePackageQuotaMigrationSkipInvalidPackageType = "invalid_package_type"
	legacyValuePackageQuotaMigrationSkipInvalidPlanTotal   = "invalid_plan_total"
)

var (
	errLegacyValuePackageQuotaMigrationManifestMismatch = errors.New("legacy value package quota migration manifest mismatch")
	errLegacyValuePackageQuotaMigrationAuthorization    = errors.New("legacy value package quota migration authorization changed")
	errLegacyValuePackageQuotaMigrationNoCurrentTargets = errors.New("legacy value package quota migration authorization no longer matches current targets")
)

type ValuePackageQuotaMigrationReceipt struct {
	MigrationVersion string `json:"migration_version" gorm:"type:varchar(64);primaryKey"`
	ManifestHash     string `json:"manifest_hash" gorm:"type:varchar(64);primaryKey"`
	AppliedAt        int64  `json:"applied_at" gorm:"type:bigint;not null"`
	Updated          int    `json:"updated" gorm:"type:int;not null"`
}

type LegacyValuePackageQuotaMigrationRow struct {
	SubscriptionID int    `json:"subscription_id"`
	PlanID         int    `json:"plan_id"`
	PackageType    string `json:"package_type"`
	AmountUsed     int64  `json:"amount_used"`
	OldTotal       int64  `json:"old_total"`
	Grant          int64  `json:"grant"`
	NewTotal       int64  `json:"new_total"`
	EndTime        int64  `json:"end_time"`
}

type LegacyValuePackageQuotaMigrationReport struct {
	MigrationNow int64                                 `json:"migration_now"`
	Rows         []LegacyValuePackageQuotaMigrationRow `json:"rows"`
	Skipped      map[string]int                        `json:"skipped"`
	ManifestHash string                                `json:"manifest_hash"`
	Updated      int                                   `json:"updated"`
}

type legacyValuePackageQuotaMigrationManifestRow struct {
	SubscriptionID int    `json:"subscription_id"`
	PlanID         int    `json:"plan_id"`
	PackageType    string `json:"package_type"`
	Grant          int64  `json:"grant"`
	EndTime        int64  `json:"end_time"`
}

func PreviewLegacyValuePackageQuotaMigration(db *gorm.DB, now int64) (*LegacyValuePackageQuotaMigrationReport, error) {
	if db == nil {
		return nil, errors.New("legacy value package quota migration db is nil")
	}
	if now <= 0 {
		return nil, errors.New("legacy value package quota migration now must be positive")
	}
	return previewLegacyValuePackageQuotaMigration(db, now)
}

func previewLegacyValuePackageQuotaMigration(db *gorm.DB, now int64) (*LegacyValuePackageQuotaMigrationReport, error) {
	report := &LegacyValuePackageQuotaMigrationReport{
		MigrationNow: now,
		Rows:         make([]LegacyValuePackageQuotaMigrationRow, 0),
		Skipped:      make(map[string]int),
	}
	var candidates []UserSubscription
	if err := db.Where("status = ? AND end_time > ? AND amount_total = ?", UserSubscriptionStatusActive, now, 0).
		Order("id asc").
		Find(&candidates).Error; err != nil {
		return nil, err
	}
	for _, sub := range candidates {
		var plan SubscriptionPlan
		if err := db.First(&plan, sub.PlanId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				report.Skipped[legacyValuePackageQuotaMigrationSkipMissingPlan]++
				continue
			}
			return nil, err
		}
		packageType, skipReason := legacyValuePackageQuotaMigrationPlanAuthorization(&plan)
		if skipReason != "" {
			report.Skipped[skipReason]++
			continue
		}
		newTotal, err := legacyValuePackageQuotaMigrationNewTotal(sub.AmountUsed, plan.TotalAmount)
		if err != nil {
			return nil, err
		}
		report.Rows = append(report.Rows, LegacyValuePackageQuotaMigrationRow{
			SubscriptionID: sub.Id,
			PlanID:         sub.PlanId,
			PackageType:    packageType,
			AmountUsed:     sub.AmountUsed,
			OldTotal:       sub.AmountTotal,
			Grant:          plan.TotalAmount,
			NewTotal:       newTotal,
			EndTime:        sub.EndTime,
		})
	}
	sort.Slice(report.Rows, func(i, j int) bool {
		return report.Rows[i].SubscriptionID < report.Rows[j].SubscriptionID
	})
	manifestHash, err := legacyValuePackageQuotaMigrationManifestHash(report.Rows)
	if err != nil {
		return nil, err
	}
	report.ManifestHash = manifestHash
	return report, nil
}

func legacyValuePackageQuotaMigrationPlanAuthorization(plan *SubscriptionPlan) (string, string) {
	if plan == nil {
		return "", legacyValuePackageQuotaMigrationSkipMissingPlan
	}
	if !plan.IsValuePackage() {
		return "", legacyValuePackageQuotaMigrationSkipNotValuePackage
	}
	packageType := strings.TrimSpace(plan.PackageType)
	switch packageType {
	case ValuePackageTypeDay, ValuePackageTypeWeek, ValuePackageTypeMonth:
	default:
		return "", legacyValuePackageQuotaMigrationSkipInvalidPackageType
	}
	if plan.TotalAmount <= 0 {
		return "", legacyValuePackageQuotaMigrationSkipInvalidPlanTotal
	}
	return packageType, ""
}

func legacyValuePackageQuotaMigrationNewTotal(amountUsed int64, grant int64) (int64, error) {
	if amountUsed < 0 {
		return 0, errors.New("legacy value package quota migration amount used is invalid")
	}
	if grant <= 0 {
		return 0, errors.New("legacy value package quota migration grant is invalid")
	}
	if amountUsed > math.MaxInt64-grant {
		return 0, errors.New("legacy value package quota migration quota overflow")
	}
	return amountUsed + grant, nil
}

func legacyValuePackageQuotaMigrationManifestHash(rows []LegacyValuePackageQuotaMigrationRow) (string, error) {
	manifestRows := make([]legacyValuePackageQuotaMigrationManifestRow, 0, len(rows))
	for _, row := range rows {
		manifestRows = append(manifestRows, legacyValuePackageQuotaMigrationManifestRow{
			SubscriptionID: row.SubscriptionID,
			PlanID:         row.PlanID,
			PackageType:    row.PackageType,
			Grant:          row.Grant,
			EndTime:        row.EndTime,
		})
	}
	sort.Slice(manifestRows, func(i, j int) bool {
		return manifestRows[i].SubscriptionID < manifestRows[j].SubscriptionID
	})
	payload, err := common.Marshal(manifestRows)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(payload)), nil
}

func deriveLegacyValuePackageQuotaMigrationManifestRow(sub *UserSubscription, plan *SubscriptionPlan) (legacyValuePackageQuotaMigrationManifestRow, error) {
	if sub == nil || plan == nil {
		return legacyValuePackageQuotaMigrationManifestRow{}, errLegacyValuePackageQuotaMigrationAuthorization
	}
	packageType, skipReason := legacyValuePackageQuotaMigrationPlanAuthorization(plan)
	if skipReason != "" {
		return legacyValuePackageQuotaMigrationManifestRow{}, errLegacyValuePackageQuotaMigrationAuthorization
	}
	return legacyValuePackageQuotaMigrationManifestRow{
		SubscriptionID: sub.Id,
		PlanID:         sub.PlanId,
		PackageType:    packageType,
		Grant:          plan.TotalAmount,
		EndTime:        sub.EndTime,
	}, nil
}

func ApplyLegacyValuePackageQuotaMigration(db *gorm.DB, now int64, manifestHash string) (*LegacyValuePackageQuotaMigrationReport, error) {
	if db == nil {
		return nil, errors.New("legacy value package quota migration db is nil")
	}
	if now <= 0 {
		return nil, errors.New("legacy value package quota migration now must be positive")
	}
	manifestHash = strings.TrimSpace(manifestHash)
	if manifestHash == "" {
		return nil, errors.New("legacy value package quota migration manifest is required")
	}
	var applied *LegacyValuePackageQuotaMigrationReport
	err := db.Transaction(func(tx *gorm.DB) error {
		current, err := previewLegacyValuePackageQuotaMigration(tx, now)
		if err != nil {
			return err
		}
		if len(current.Rows) == 0 {
			var receipt ValuePackageQuotaMigrationReceipt
			result := tx.Where("migration_version = ? AND manifest_hash = ?", LegacyValuePackageQuotaMigrationVersion, manifestHash).
				Limit(1).
				Find(&receipt)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errLegacyValuePackageQuotaMigrationNoCurrentTargets
			}
			current.ManifestHash = manifestHash
			current.Updated = 0
			applied = current
			return nil
		}
		if current.ManifestHash != manifestHash {
			return errLegacyValuePackageQuotaMigrationManifestMismatch
		}

		authorized := make([]legacyValuePackageQuotaMigrationManifestRow, len(current.Rows))
		for i, row := range current.Rows {
			authorized[i] = legacyValuePackageQuotaMigrationManifestRow{
				SubscriptionID: row.SubscriptionID,
				PlanID:         row.PlanID,
				PackageType:    row.PackageType,
				Grant:          row.Grant,
				EndTime:        row.EndTime,
			}
		}
		actualRows := make([]LegacyValuePackageQuotaMigrationRow, 0, len(authorized))
		for _, authorization := range authorized {
			var lockedSub UserSubscription
			query := withUpdateLock(tx).
				Where("id = ? AND status = ? AND end_time > ? AND amount_total = ?", authorization.SubscriptionID, UserSubscriptionStatusActive, now, 0).
				First(&lockedSub)
			if query.Error != nil {
				if errors.Is(query.Error, gorm.ErrRecordNotFound) {
					return errLegacyValuePackageQuotaMigrationAuthorization
				}
				return query.Error
			}
			var lockedPlan SubscriptionPlan
			if err := withUpdateLock(tx).First(&lockedPlan, lockedSub.PlanId).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errLegacyValuePackageQuotaMigrationAuthorization
				}
				return err
			}
			lockedAuthorization, err := deriveLegacyValuePackageQuotaMigrationManifestRow(&lockedSub, &lockedPlan)
			if err != nil || lockedAuthorization != authorization {
				return errLegacyValuePackageQuotaMigrationAuthorization
			}
			newTotal, err := legacyValuePackageQuotaMigrationNewTotal(lockedSub.AmountUsed, lockedPlan.TotalAmount)
			if err != nil {
				return err
			}
			updatedAt := common.GetTimestamp()
			result := tx.Model(&UserSubscription{}).
				Where("id = ? AND plan_id = ? AND status = ? AND end_time = ? AND end_time > ? AND amount_total = ?", lockedSub.Id, lockedSub.PlanId, UserSubscriptionStatusActive, lockedSub.EndTime, now, 0).
				Updates(map[string]interface{}{"amount_total": newTotal, "updated_at": updatedAt})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errLegacyValuePackageQuotaMigrationAuthorization
			}
			actualRows = append(actualRows, LegacyValuePackageQuotaMigrationRow{
				SubscriptionID: lockedSub.Id,
				PlanID:         lockedSub.PlanId,
				PackageType:    lockedAuthorization.PackageType,
				AmountUsed:     lockedSub.AmountUsed,
				OldTotal:       lockedSub.AmountTotal,
				Grant:          lockedPlan.TotalAmount,
				NewTotal:       newTotal,
				EndTime:        lockedSub.EndTime,
			})
		}
		receipt := ValuePackageQuotaMigrationReceipt{
			MigrationVersion: LegacyValuePackageQuotaMigrationVersion,
			ManifestHash:     manifestHash,
			AppliedAt:        common.GetTimestamp(),
			Updated:          len(actualRows),
		}
		if err := tx.Where("migration_version = ? AND manifest_hash = ?", receipt.MigrationVersion, receipt.ManifestHash).
			FirstOrCreate(&receipt).Error; err != nil {
			return err
		}
		current.Rows = actualRows
		current.ManifestHash = manifestHash
		current.Updated = len(actualRows)
		applied = current
		return nil
	})
	if err != nil {
		return nil, err
	}
	return applied, nil
}
