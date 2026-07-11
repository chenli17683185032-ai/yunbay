package model

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const ValuePackageResetReconcileVersion = "value_package_reset_reconcile_v1"

const (
	ValuePackageResetReconcileAnchorNative        = "native"
	ValuePackageResetReconcileAnchorB2Migration   = "b2_migration"
	ValuePackageResetReconcileAnchorUserReset     = "user_reset"
	ValuePackageResetReconcileAnchorPackagePeriod = "package_period"
)

var (
	errValuePackageResetReconcileManifestMismatch = errors.New("value package reset reconcile manifest mismatch")
	errValuePackageResetReconcileAuthorization    = errors.New("value package reset reconcile authorization changed")
)

type ValuePackageResetReconcileReceipt struct {
	MigrationVersion string `json:"migration_version" gorm:"type:varchar(64);primaryKey"`
	ManifestHash     string `json:"manifest_hash" gorm:"type:varchar(64);primaryKey"`
	B2ManifestHash   string `json:"b2_manifest_hash" gorm:"type:varchar(64);not null"`
	AppliedAt        int64  `json:"applied_at" gorm:"type:bigint;not null"`
	Updated          int    `json:"updated" gorm:"type:int;not null"`
}

type ValuePackageResetReconcileRow struct {
	SubscriptionID int    `json:"subscription_id"`
	UserID         int    `json:"user_id"`
	PlanID         int    `json:"plan_id"`
	PackageType    string `json:"package_type"`
	EndTime        int64  `json:"end_time"`

	AnchorType     string `json:"anchor_type"`
	AnchorAt       int64  `json:"anchor_at"`
	AnchorResetID  int    `json:"anchor_reset_id"`
	B2BaselineUsed int64  `json:"b2_baseline_used"`

	OldTotal     int64 `json:"old_total"`
	OldUsed      int64 `json:"old_used"`
	OldEpoch     int64 `json:"old_epoch"`
	NewTotal     int64 `json:"new_total"`
	NewUsed      int64 `json:"new_used"`
	NewEpoch     int64 `json:"new_epoch"`
	CycleStart   int64 `json:"cycle_start"`
	NextCycleAt  int64 `json:"next_cycle_at"`
	EpochAfterAt int64 `json:"epoch_after_at"`
	EpochAfterID int   `json:"epoch_after_id"`

	EvidenceRecords int64 `json:"evidence_records"`
	EvidenceUsage   int64 `json:"evidence_usage"`
	NeedsUpdate     bool  `json:"needs_update"`
}

type ValuePackageResetReconcileReport struct {
	ReconcileNow   int64                           `json:"reconcile_now"`
	B2ManifestHash string                          `json:"b2_manifest_hash"`
	B2AppliedAt    int64                           `json:"b2_applied_at"`
	Rows           []ValuePackageResetReconcileRow `json:"rows"`
	ManifestHash   string                          `json:"manifest_hash"`
	Updated        int                             `json:"updated"`
	AlreadyApplied bool                            `json:"already_applied"`
}

type valuePackageResetReconcileManifestRow struct {
	SubscriptionID int    `json:"subscription_id"`
	UserID         int    `json:"user_id"`
	PlanID         int    `json:"plan_id"`
	PackageType    string `json:"package_type"`
	EndTime        int64  `json:"end_time"`
	PlanTotal      int64  `json:"plan_total"`
	OldTotal       int64  `json:"old_total"`
	OldEpoch       int64  `json:"old_epoch"`
	AnchorType     string `json:"anchor_type"`
	AnchorAt       int64  `json:"anchor_at"`
	AnchorResetID  int    `json:"anchor_reset_id"`
	B2BaselineUsed int64  `json:"b2_baseline_used"`
	CycleStart     int64  `json:"cycle_start"`
	NextCycleAt    int64  `json:"next_cycle_at"`
	EpochAfterAt   int64  `json:"epoch_after_at"`
	EpochAfterID   int    `json:"epoch_after_id"`
}

type valuePackageResetReconcileB2State struct {
	ManifestHash string
	AppliedAt    int64
	RowsBySubID  map[int]LegacyValuePackageQuotaMigrationRow
}

type valuePackageResetReconcileReset struct {
	ID               int
	ResetAt          int64
	FromEpoch        int64
	ToEpoch          int64
	AmountUsedBefore int64
}

func PrepareValuePackageResetReconcileSchema(db *gorm.DB) error {
	if db == nil {
		return errors.New("value package reset reconcile db is nil")
	}
	columns := []struct {
		model interface{}
		field string
	}{
		{model: &UserSubscription{}, field: "QuotaEpoch"},
		{model: &SubscriptionPreConsumeRecord{}, field: "QuotaEpoch"},
		{model: &ValuePackageUsageRecord{}, field: "QuotaEpoch"},
		{model: &ValuePackageQuotaReset{}, field: "FromEpoch"},
		{model: &ValuePackageQuotaReset{}, field: "ToEpoch"},
		{model: &ValuePackageQuotaReset{}, field: "AmountUsedBefore"},
	}
	for _, column := range columns {
		if !db.Migrator().HasTable(column.model) {
			if err := db.AutoMigrate(column.model); err != nil {
				return fmt.Errorf("prepare value package reset reconcile table: %w", err)
			}
		}
		if db.Migrator().HasColumn(column.model, column.field) {
			continue
		}
		if err := db.Migrator().AddColumn(column.model, column.field); err != nil {
			return fmt.Errorf("prepare value package reset reconcile column %s: %w", column.field, err)
		}
	}
	return nil
}

func ValidateValuePackageResetReconcileSchema(db *gorm.DB) error {
	if db == nil {
		return errors.New("value package reset reconcile db is nil")
	}
	columns := []struct {
		model interface{}
		field string
	}{
		{model: &UserSubscription{}, field: "QuotaEpoch"},
		{model: &SubscriptionPreConsumeRecord{}, field: "QuotaEpoch"},
		{model: &ValuePackageUsageRecord{}, field: "QuotaEpoch"},
		{model: &ValuePackageQuotaReset{}, field: "FromEpoch"},
		{model: &ValuePackageQuotaReset{}, field: "ToEpoch"},
		{model: &ValuePackageQuotaReset{}, field: "AmountUsedBefore"},
	}
	for _, column := range columns {
		if !db.Migrator().HasTable(column.model) || !db.Migrator().HasColumn(column.model, column.field) {
			return fmt.Errorf("value package reset reconcile schema is missing %s; run --prepare-schema first", column.field)
		}
	}
	return nil
}

func PreviewValuePackageResetReconcile(db *gorm.DB, now int64, b2Report *LegacyValuePackageQuotaMigrationReport) (*ValuePackageResetReconcileReport, error) {
	if db == nil {
		return nil, errors.New("value package reset reconcile db is nil")
	}
	if now <= 0 {
		return nil, errors.New("value package reset reconcile now must be positive")
	}
	b2State, err := validateValuePackageResetReconcileB2Report(db, b2Report)
	if err != nil {
		return nil, err
	}
	return buildValuePackageResetReconcileReport(db, now, b2State, false)
}

func validateValuePackageResetReconcileB2Report(db *gorm.DB, report *LegacyValuePackageQuotaMigrationReport) (*valuePackageResetReconcileB2State, error) {
	if report == nil {
		return nil, errors.New("value package reset reconcile B2 report is required")
	}
	if report.MigrationNow <= 0 {
		return nil, errors.New("value package reset reconcile B2 migration time is invalid")
	}
	manifestHash := strings.TrimSpace(report.ManifestHash)
	if len(manifestHash) != sha256.Size*2 {
		return nil, errors.New("value package reset reconcile B2 manifest is invalid")
	}
	recomputedHash, err := legacyValuePackageQuotaMigrationManifestHash(report.Rows)
	if err != nil {
		return nil, fmt.Errorf("value package reset reconcile B2 manifest: %w", err)
	}
	if recomputedHash != manifestHash {
		return nil, errors.New("value package reset reconcile B2 report manifest mismatch")
	}

	rowsBySubID := make(map[int]LegacyValuePackageQuotaMigrationRow, len(report.Rows))
	for _, row := range report.Rows {
		if row.SubscriptionID <= 0 || row.PlanID <= 0 || row.AmountUsed < 0 || row.OldTotal != 0 || row.Grant <= 0 || row.NewTotal != row.AmountUsed+row.Grant || row.EndTime <= 0 {
			return nil, fmt.Errorf("value package reset reconcile B2 row %d is invalid", row.SubscriptionID)
		}
		switch strings.TrimSpace(row.PackageType) {
		case ValuePackageTypeDay, ValuePackageTypeWeek, ValuePackageTypeMonth:
		default:
			return nil, fmt.Errorf("value package reset reconcile B2 row %d package type is invalid", row.SubscriptionID)
		}
		if _, exists := rowsBySubID[row.SubscriptionID]; exists {
			return nil, fmt.Errorf("value package reset reconcile B2 subscription %d is duplicated", row.SubscriptionID)
		}
		rowsBySubID[row.SubscriptionID] = row
	}

	var receipt ValuePackageQuotaMigrationReceipt
	result := db.Where("migration_version = ? AND manifest_hash = ?", LegacyValuePackageQuotaMigrationVersion, manifestHash).
		Limit(1).
		Find(&receipt)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 || receipt.AppliedAt <= 0 || receipt.Updated != len(report.Rows) {
		return nil, errors.New("value package reset reconcile B2 receipt does not match report")
	}
	return &valuePackageResetReconcileB2State{
		ManifestHash: manifestHash,
		AppliedAt:    receipt.AppliedAt,
		RowsBySubID:  rowsBySubID,
	}, nil
}

func buildValuePackageResetReconcileReport(db *gorm.DB, now int64, b2State *valuePackageResetReconcileB2State, lockRows bool) (*ValuePackageResetReconcileReport, error) {
	if b2State == nil {
		return nil, errors.New("value package reset reconcile B2 state is nil")
	}

	var subscriptions []UserSubscription
	query := db.Where("status = ? AND end_time > ?", UserSubscriptionStatusActive, now).Order("id asc")
	if lockRows {
		query = withUpdateLock(query)
	}
	if err := query.Find(&subscriptions).Error; err != nil {
		return nil, err
	}

	rows := make([]ValuePackageResetReconcileRow, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		var plan SubscriptionPlan
		planQuery := db.Where("id = ?", subscription.PlanId)
		if lockRows {
			planQuery = withUpdateLock(planQuery)
		}
		result := planQuery.Limit(1).Find(&plan)
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected != 1 {
			continue
		}
		normalizeValuePackagePlan(&plan)
		if !plan.IsValuePackage() {
			continue
		}
		packageType, skipReason := legacyValuePackageQuotaMigrationPlanAuthorization(&plan)
		if skipReason != "" {
			return nil, fmt.Errorf("value package reset reconcile subscription %d plan is invalid: %s", subscription.Id, skipReason)
		}

		lastReset, err := getLastValuePackageResetForReconcile(db, subscription.UserId, subscription.Id, now, lockRows)
		if err != nil {
			return nil, err
		}
		b2Row, hasB2Row := b2State.RowsBySubID[subscription.Id]
		if hasB2Row && (b2Row.PlanID != subscription.PlanId || strings.TrimSpace(b2Row.PackageType) != packageType || b2Row.Grant != plan.TotalAmount) {
			return nil, fmt.Errorf("value package reset reconcile subscription %d no longer matches B2 authorization", subscription.Id)
		}
		if hasB2Row {
			if err := authenticateValuePackageResetReconcileB2Total(&subscription, &plan, b2Row); err != nil {
				return nil, err
			}
		}
		if subscription.QuotaEpoch != 0 {
			return nil, fmt.Errorf("value package reset reconcile subscription %d epoch is not legacy zero", subscription.Id)
		}

		row := ValuePackageResetReconcileRow{
			SubscriptionID: subscription.Id,
			UserID:         subscription.UserId,
			PlanID:         subscription.PlanId,
			PackageType:    packageType,
			EndTime:        subscription.EndTime,
			AnchorType:     ValuePackageResetReconcileAnchorNative,
			OldTotal:       subscription.AmountTotal,
			OldUsed:        subscription.AmountUsed,
			OldEpoch:       subscription.QuotaEpoch,
			NewTotal:       plan.TotalAmount,
			NewUsed:        subscription.AmountUsed,
			NewEpoch:       1,
		}

		packagePeriodAt := valuePackageResetReconcileCurrentPeriodStart(&subscription, packageType, b2Row, hasB2Row, now)
		row.CycleStart = packagePeriodAt
		if packagePeriodAt > 0 {
			nextCycleAt := packagePeriodAt + valuePackageResetReconcilePeriodSeconds(packageType)
			if nextCycleAt < subscription.EndTime {
				row.NextCycleAt = nextCycleAt
			}
		}
		row.AnchorType, row.AnchorAt = ValuePackageResetReconcileAnchorNative, 0
		if hasB2Row {
			row.AnchorType, row.AnchorAt = ValuePackageResetReconcileAnchorB2Migration, b2State.AppliedAt
			row.B2BaselineUsed = b2Row.AmountUsed
		}
		if lastReset != nil && lastReset.ResetAt > row.AnchorAt {
			row.AnchorType, row.AnchorAt = ValuePackageResetReconcileAnchorUserReset, lastReset.ResetAt
			row.AnchorResetID = lastReset.ID
		}
		if packagePeriodAt > row.AnchorAt {
			row.AnchorType, row.AnchorAt = ValuePackageResetReconcileAnchorPackagePeriod, packagePeriodAt
			row.AnchorResetID = 0
			row.B2BaselineUsed = 0
		}

		switch row.AnchorType {
		case ValuePackageResetReconcileAnchorUserReset, ValuePackageResetReconcileAnchorPackagePeriod:
			if row.AnchorType == ValuePackageResetReconcileAnchorUserReset && (lastReset.FromEpoch != 0 || lastReset.ToEpoch != 0 || lastReset.AmountUsedBefore != 0) {
				return nil, fmt.Errorf("value package reset reconcile subscription %d reset metadata is not legacy zero", subscription.Id)
			}
			if row.AnchorType == ValuePackageResetReconcileAnchorUserReset {
				var ambiguousRecords int64
				if err := db.Model(&ValuePackageUsageRecord{}).
					Where("user_id = ? AND user_subscription_id = ? AND created_at = ?", subscription.UserId, subscription.Id, row.AnchorAt).
					Count(&ambiguousRecords).Error; err != nil {
					return nil, err
				}
				if ambiguousRecords > 0 {
					return nil, fmt.Errorf("value package reset reconcile subscription %d has ambiguous usage in the same second as user reset", subscription.Id)
				}
			}
			evidence, err := getValuePackageResetReconcileCurrentRecords(db, subscription, row.AnchorType, row.AnchorAt, 0, now)
			if err != nil {
				return nil, err
			}
			usage := sumValuePackageResetReconcileUsage(evidence)
			if usage > subscription.AmountUsed {
				return nil, fmt.Errorf("value package reset reconcile subscription %d reset usage exceeds stored usage", subscription.Id)
			}
			row.EvidenceRecords = int64(len(evidence))
			row.EvidenceUsage = usage
			row.NewUsed = usage
		case ValuePackageResetReconcileAnchorB2Migration:
			if subscription.AmountUsed < b2Row.AmountUsed {
				return nil, fmt.Errorf("value package reset reconcile subscription %d stored usage is below B2 baseline", subscription.Id)
			}
			row.EpochAfterAt, row.EpochAfterID, err = getValuePackageResetReconcileB2EpochBoundary(db, subscription, b2Row.AmountUsed, now)
			if err != nil {
				return nil, err
			}
			evidence, err := getValuePackageResetReconcileCurrentRecords(db, subscription, row.AnchorType, row.AnchorAt, b2Row.AmountUsed, now)
			if err != nil {
				return nil, err
			}
			newUsed := subscription.AmountUsed - b2Row.AmountUsed
			usage := sumValuePackageResetReconcileUsage(evidence)
			if usage != newUsed {
				return nil, fmt.Errorf("value package reset reconcile subscription %d B2 usage evidence mismatch: stored delta=%d evidence=%d", subscription.Id, newUsed, usage)
			}
			row.EvidenceRecords = int64(len(evidence))
			row.EvidenceUsage = usage
			row.NewUsed = newUsed
		case ValuePackageResetReconcileAnchorNative:
			evidence, err := getValuePackageResetReconcileCurrentRecords(db, subscription, row.AnchorType, 0, 0, now)
			if err != nil {
				return nil, err
			}
			usage := sumValuePackageResetReconcileUsage(evidence)
			if usage != subscription.AmountUsed {
				return nil, fmt.Errorf("value package reset reconcile subscription %d native usage evidence mismatch: stored=%d evidence=%d", subscription.Id, subscription.AmountUsed, usage)
			}
			row.EvidenceRecords = int64(len(evidence))
			row.EvidenceUsage = usage
		}
		if row.NewUsed > row.NewTotal {
			return nil, fmt.Errorf("value package reset reconcile subscription %d rebuilt usage exceeds plan total", subscription.Id)
		}
		row.NeedsUpdate = row.OldTotal != row.NewTotal || row.OldUsed != row.NewUsed || row.OldEpoch != row.NewEpoch
		rows = append(rows, row)
	}

	manifestHash, err := valuePackageResetReconcileManifestHash(rows)
	if err != nil {
		return nil, err
	}
	return &ValuePackageResetReconcileReport{
		ReconcileNow:   now,
		B2ManifestHash: b2State.ManifestHash,
		B2AppliedAt:    b2State.AppliedAt,
		Rows:           rows,
		ManifestHash:   manifestHash,
	}, nil
}

func getLastValuePackageResetForReconcile(db *gorm.DB, userID int, subscriptionID int, now int64, lockRow bool) (*valuePackageResetReconcileReset, error) {
	var reset ValuePackageQuotaReset
	query := db.Where("user_id = ? AND user_subscription_id = ? AND source = ? AND reset_at <= ?", userID, subscriptionID, ValuePackageQuotaResetSourceUserConsumeCount, now).
		Order("reset_at desc, id desc")
	if lockRow {
		query = withUpdateLock(query)
	}
	result := query.Limit(1).Find(&reset)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &valuePackageResetReconcileReset{
		ID:               reset.Id,
		ResetAt:          reset.ResetAt,
		FromEpoch:        reset.FromEpoch,
		ToEpoch:          reset.ToEpoch,
		AmountUsedBefore: reset.AmountUsedBefore,
	}, nil
}

func getValuePackageResetReconcileCurrentRecords(db *gorm.DB, subscription UserSubscription, anchorType string, anchorAt int64, b2Baseline int64, now int64) ([]ValuePackageUsageRecord, error) {
	var records []ValuePackageUsageRecord
	if err := db.Where("user_id = ? AND user_subscription_id = ? AND created_at <= ?", subscription.UserId, subscription.Id, now).
		Order("created_at asc, id asc").
		Find(&records).Error; err != nil {
		return nil, err
	}
	for _, record := range records {
		if record.Quota < 0 {
			return nil, fmt.Errorf("value package reset reconcile subscription %d has negative usage evidence", subscription.Id)
		}
		if record.QuotaEpoch != 0 {
			return nil, fmt.Errorf("value package reset reconcile subscription %d usage epoch is not legacy zero", subscription.Id)
		}
	}

	switch anchorType {
	case ValuePackageResetReconcileAnchorNative:
		return records, nil
	case ValuePackageResetReconcileAnchorUserReset, ValuePackageResetReconcileAnchorPackagePeriod:
		if anchorAt <= 0 {
			return nil, errors.New("value package reset reconcile usage anchor is invalid")
		}
		index := sort.Search(len(records), func(index int) bool { return records[index].CreatedAt >= anchorAt })
		return records[index:], nil
	case ValuePackageResetReconcileAnchorB2Migration:
		if anchorAt <= 0 || b2Baseline < 0 {
			return nil, errors.New("value package reset reconcile B2 usage anchor is invalid")
		}
		var historical int64
		index := 0
		for index < len(records) && historical < b2Baseline {
			if records[index].CreatedAt > anchorAt {
				return nil, fmt.Errorf("value package reset reconcile subscription %d B2 baseline requires post-apply evidence", subscription.Id)
			}
			historical += records[index].Quota
			if historical > b2Baseline {
				return nil, fmt.Errorf("value package reset reconcile subscription %d B2 baseline splits a usage record", subscription.Id)
			}
			index++
		}
		if historical != b2Baseline {
			return nil, fmt.Errorf("value package reset reconcile subscription %d B2 baseline does not match usage prefix", subscription.Id)
		}
		return records[index:], nil
	default:
		return nil, fmt.Errorf("value package reset reconcile subscription %d anchor type is invalid", subscription.Id)
	}
}

func getValuePackageResetReconcileB2EpochBoundary(db *gorm.DB, subscription UserSubscription, b2Baseline int64, now int64) (int64, int, error) {
	if b2Baseline == 0 {
		return 0, 0, nil
	}
	var records []ValuePackageUsageRecord
	if err := db.Where("user_id = ? AND user_subscription_id = ? AND created_at <= ?", subscription.UserId, subscription.Id, now).
		Order("created_at asc, id asc").
		Find(&records).Error; err != nil {
		return 0, 0, err
	}
	var historical int64
	for _, record := range records {
		if record.Quota < 0 || record.QuotaEpoch != 0 {
			return 0, 0, fmt.Errorf("value package reset reconcile subscription %d has invalid legacy usage evidence", subscription.Id)
		}
		historical += record.Quota
		if historical > b2Baseline {
			return 0, 0, fmt.Errorf("value package reset reconcile subscription %d B2 baseline splits a usage record", subscription.Id)
		}
		if historical == b2Baseline {
			if record.Id == int(^uint(0)>>1) {
				return 0, 0, fmt.Errorf("value package reset reconcile subscription %d usage id overflow", subscription.Id)
			}
			return record.CreatedAt, record.Id + 1, nil
		}
	}
	return 0, 0, fmt.Errorf("value package reset reconcile subscription %d B2 baseline does not match usage prefix", subscription.Id)
}

func sumValuePackageResetReconcileUsage(records []ValuePackageUsageRecord) int64 {
	var usage int64
	for _, record := range records {
		usage += record.Quota
	}
	return usage
}

func valuePackageResetReconcilePeriodSeconds(packageType string) int64 {
	switch packageType {
	case ValuePackageTypeDay:
		return valuePackageDaySeconds
	case ValuePackageTypeWeek:
		return valuePackageWeekSeconds
	case ValuePackageTypeMonth:
		return valuePackageMonthSeconds
	default:
		return 0
	}
}

func valuePackageResetReconcileCurrentPeriodStart(subscription *UserSubscription, packageType string, b2Row LegacyValuePackageQuotaMigrationRow, hasB2Row bool, now int64) int64 {
	if subscription == nil || now <= subscription.StartTime || now >= subscription.EndTime {
		return 0
	}
	duration := valuePackageResetReconcilePeriodSeconds(strings.TrimSpace(packageType))
	if duration <= 0 {
		return 0
	}
	span := subscription.EndTime - subscription.StartTime
	if span > duration && span%duration == 0 {
		periodIndex := (now - subscription.StartTime) / duration
		if periodIndex > 0 {
			return subscription.StartTime + periodIndex*duration
		}
	}
	if hasB2Row && subscription.EndTime > b2Row.EndTime && (subscription.EndTime-b2Row.EndTime)%duration == 0 && now >= b2Row.EndTime {
		return b2Row.EndTime + ((now-b2Row.EndTime)/duration)*duration
	}
	return 0
}

func authenticateValuePackageResetReconcileB2Total(subscription *UserSubscription, plan *SubscriptionPlan, b2Row LegacyValuePackageQuotaMigrationRow) error {
	if subscription.AmountTotal == b2Row.NewTotal {
		return nil
	}
	duration := valuePackageResetReconcilePeriodSeconds(strings.TrimSpace(plan.PackageType))
	if duration <= 0 || subscription.EndTime <= b2Row.EndTime || (subscription.EndTime-b2Row.EndTime)%duration != 0 {
		return fmt.Errorf("value package reset reconcile subscription %d current total does not authenticate B2 baseline", subscription.Id)
	}
	extensions := (subscription.EndTime - b2Row.EndTime) / duration
	if extensions <= 0 || b2Row.NewTotal > int64(^uint64(0)>>1)-extensions*plan.TotalAmount {
		return fmt.Errorf("value package reset reconcile subscription %d B2 extension total is invalid", subscription.Id)
	}
	if subscription.AmountTotal != b2Row.NewTotal+extensions*plan.TotalAmount {
		return fmt.Errorf("value package reset reconcile subscription %d current total does not authenticate B2 baseline", subscription.Id)
	}
	return nil
}

func valuePackageResetReconcileManifestHash(rows []ValuePackageResetReconcileRow) (string, error) {
	manifestRows := make([]valuePackageResetReconcileManifestRow, 0, len(rows))
	for _, row := range rows {
		manifestRows = append(manifestRows, valuePackageResetReconcileManifestRow{
			SubscriptionID: row.SubscriptionID,
			UserID:         row.UserID,
			PlanID:         row.PlanID,
			PackageType:    row.PackageType,
			EndTime:        row.EndTime,
			PlanTotal:      row.NewTotal,
			OldTotal:       row.OldTotal,
			OldEpoch:       row.OldEpoch,
			AnchorType:     row.AnchorType,
			AnchorAt:       row.AnchorAt,
			AnchorResetID:  row.AnchorResetID,
			B2BaselineUsed: row.B2BaselineUsed,
			CycleStart:     row.CycleStart,
			NextCycleAt:    row.NextCycleAt,
			EpochAfterAt:   row.EpochAfterAt,
			EpochAfterID:   row.EpochAfterID,
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

func ApplyValuePackageResetReconcile(db *gorm.DB, now int64, b2Report *LegacyValuePackageQuotaMigrationReport, manifestHash string) (*ValuePackageResetReconcileReport, error) {
	if db == nil {
		return nil, errors.New("value package reset reconcile db is nil")
	}
	if now <= 0 {
		return nil, errors.New("value package reset reconcile now must be positive")
	}
	manifestHash = strings.TrimSpace(manifestHash)
	if len(manifestHash) != sha256.Size*2 {
		return nil, errors.New("value package reset reconcile manifest is required")
	}

	var applied *ValuePackageResetReconcileReport
	err := db.Transaction(func(tx *gorm.DB) error {
		b2State, err := validateValuePackageResetReconcileB2Report(tx, b2Report)
		if err != nil {
			return err
		}

		var existingReceipt ValuePackageResetReconcileReceipt
		receiptResult := tx.Where("migration_version = ? AND manifest_hash = ?", ValuePackageResetReconcileVersion, manifestHash).
			Limit(1).
			Find(&existingReceipt)
		if receiptResult.Error != nil {
			return receiptResult.Error
		}
		if receiptResult.RowsAffected == 1 {
			if existingReceipt.B2ManifestHash != b2State.ManifestHash {
				return errors.New("value package reset reconcile receipt B2 manifest mismatch")
			}
			applied = &ValuePackageResetReconcileReport{
				ReconcileNow:   now,
				B2ManifestHash: b2State.ManifestHash,
				B2AppliedAt:    b2State.AppliedAt,
				Rows:           []ValuePackageResetReconcileRow{},
				ManifestHash:   manifestHash,
				AlreadyApplied: true,
			}
			return nil
		}

		current, err := buildValuePackageResetReconcileReport(tx, now, b2State, true)
		if err != nil {
			return err
		}
		if current.ManifestHash != manifestHash {
			return errValuePackageResetReconcileManifestMismatch
		}

		updated := 0
		for index := range current.Rows {
			row := &current.Rows[index]
			if !row.NeedsUpdate {
				continue
			}
			result := tx.Model(&UserSubscription{}).
				Where("id = ? AND user_id = ? AND plan_id = ? AND status = ? AND end_time = ? AND end_time > ? AND amount_total = ? AND amount_used = ? AND quota_epoch = ?", row.SubscriptionID, row.UserID, row.PlanID, UserSubscriptionStatusActive, row.EndTime, now, row.OldTotal, row.OldUsed, row.OldEpoch).
				Updates(map[string]interface{}{
					"amount_total":    row.NewTotal,
					"amount_used":     row.NewUsed,
					"quota_epoch":     row.NewEpoch,
					"last_reset_time": row.CycleStart,
					"next_reset_time": row.NextCycleAt,
					"updated_at":      common.GetTimestamp(),
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errValuePackageResetReconcileAuthorization
			}
			if err := applyValuePackageResetReconcileEpoch(tx, *row, b2State, now); err != nil {
				return err
			}
			updated++
		}

		receipt := ValuePackageResetReconcileReceipt{
			MigrationVersion: ValuePackageResetReconcileVersion,
			ManifestHash:     manifestHash,
			B2ManifestHash:   b2State.ManifestHash,
			AppliedAt:        common.GetTimestamp(),
			Updated:          updated,
		}
		if err := tx.Create(&receipt).Error; err != nil {
			return err
		}
		current.Updated = updated
		applied = current
		return nil
	})
	if err != nil {
		return nil, err
	}
	return applied, nil
}

func applyValuePackageResetReconcileEpoch(tx *gorm.DB, row ValuePackageResetReconcileRow, _ *valuePackageResetReconcileB2State, now int64) error {
	if tx == nil {
		return errors.New("value package reset reconcile transaction is nil")
	}
	if row.NewEpoch <= 0 {
		return errors.New("value package reset reconcile epoch is invalid")
	}
	query := tx.Model(&ValuePackageUsageRecord{}).
		Where("user_id = ? AND user_subscription_id = ? AND quota_epoch = ? AND created_at <= ?", row.UserID, row.SubscriptionID, row.OldEpoch, now)
	switch row.AnchorType {
	case ValuePackageResetReconcileAnchorNative:
	case ValuePackageResetReconcileAnchorB2Migration:
		if row.EpochAfterAt > 0 {
			query = query.Where("created_at > ? OR (created_at = ? AND id >= ?)", row.EpochAfterAt, row.EpochAfterAt, row.EpochAfterID)
		}
	case ValuePackageResetReconcileAnchorUserReset, ValuePackageResetReconcileAnchorPackagePeriod:
		if row.AnchorAt <= 0 {
			return errors.New("value package reset reconcile usage anchor is invalid")
		}
		query = query.Where("created_at >= ?", row.AnchorAt)
	default:
		return errors.New("value package reset reconcile anchor type is invalid")
	}
	var requestIDs []string
	if err := query.Pluck("request_id", &requestIDs).Error; err != nil {
		return err
	}
	if err := query.Update("quota_epoch", row.NewEpoch).Error; err != nil {
		return err
	}
	if len(requestIDs) > 0 {
		if err := tx.Model(&SubscriptionPreConsumeRecord{}).
			Where("user_id = ? AND user_subscription_id = ? AND quota_epoch = ? AND request_id IN ?", row.UserID, row.SubscriptionID, row.OldEpoch, requestIDs).
			Update("quota_epoch", row.NewEpoch).Error; err != nil {
			return err
		}
	}
	return nil
}
