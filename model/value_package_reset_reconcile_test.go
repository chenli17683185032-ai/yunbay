package model

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	valuePackageResetReconcileTestNow      = int64(2_000_000_000)
	valuePackageResetReconcileTestApplied  = valuePackageResetReconcileTestNow - 500
	valuePackageResetReconcileTestManifest = "value-package-reset-reconcile-b2"
)

type valuePackageResetReconcileFixture struct {
	Plan              SubscriptionPlan
	B2NoReset         UserSubscription
	ResetAfterB2      UserSubscription
	ResetBeforeB2     UserSubscription
	Native            UserSubscription
	B2Report          LegacyValuePackageQuotaMigrationReport
	ResetAfterB2At    int64
	ResetBeforeB2At   int64
	B2NoResetExpected int64
	ResetExpected     int64
}

func setupValuePackageResetReconcileTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupValuePackageTestDB(t)
	require.NoError(t, db.AutoMigrate(&ValuePackageQuotaMigrationReceipt{}, &ValuePackageResetReconcileReceipt{}))
	return db
}

func seedValuePackageResetReconcileUsage(t *testing.T, sub UserSubscription, createdAt int64, quota int64, suffix string) {
	t.Helper()
	require.NoError(t, DB.Create(&ValuePackageUsageRecord{
		UserId:             sub.UserId,
		UserSubscriptionId: sub.Id,
		PlanId:             sub.PlanId,
		PackageType:        ValuePackageTypeWeek,
		ModelGroup:         "week-card",
		RequestId:          "reconcile-" + suffix,
		Quota:              quota,
		CreatedAt:          createdAt,
	}).Error)
}

func seedValuePackageResetReconcileFixture(t *testing.T) valuePackageResetReconcileFixture {
	t.Helper()
	now := valuePackageResetReconcileTestNow
	plan := seedLegacyValuePackageQuotaMigrationPlan(t, ValuePackageTypeWeek, 45_000_000, SubscriptionPlanKindValuePackage)
	plan.ModelGroup = "week-card"
	require.NoError(t, DB.Save(&plan).Error)

	seedSub := func(userID int, amountTotal int64, amountUsed int64, source string) UserSubscription {
		sub := seedLegacyValuePackageQuotaMigrationSub(t, plan.Id, UserSubscriptionStatusActive, now+10_000+int64(userID), amountTotal, amountUsed)
		require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Updates(map[string]interface{}{
			"user_id":    userID,
			"start_time": now - 10_000,
			"source":     source,
		}).Error)
		sub.UserId = userID
		sub.StartTime = now - 10_000
		sub.Source = source
		return sub
	}

	b2NoResetBaseline := int64(70_000_000)
	b2NoResetExpected := int64(3_000_000)
	b2NoReset := seedSub(7101, b2NoResetBaseline+plan.TotalAmount, b2NoResetBaseline+b2NoResetExpected, "redemption")
	seedValuePackageResetReconcileUsage(t, b2NoReset, valuePackageResetReconcileTestApplied, b2NoResetBaseline, "b2-before")
	seedValuePackageResetReconcileUsage(t, b2NoReset, valuePackageResetReconcileTestApplied, 1_000_000, "b2-boundary")
	seedValuePackageResetReconcileUsage(t, b2NoReset, valuePackageResetReconcileTestApplied+1, 2_000_000, "b2-after")
	require.NoError(t, DB.Create(&SubscriptionPreConsumeRecord{RequestId: "reconcile-b2-after", UserId: b2NoReset.UserId, UserSubscriptionId: b2NoReset.Id, PreConsumed: 2_000_000, Status: "consumed"}).Error)

	resetAfterB2Baseline := int64(20_000_000)
	resetExpected := int64(4_000_000)
	resetAfterB2 := seedSub(7102, resetAfterB2Baseline+plan.TotalAmount, resetAfterB2Baseline+resetExpected, "redemption")
	resetAfterB2At := valuePackageResetReconcileTestApplied + 100
	seedValuePackageResetReconcileUsage(t, resetAfterB2, valuePackageResetReconcileTestApplied, resetAfterB2Baseline, "late-reset-before")
	seedValuePackageResetReconcileUsage(t, resetAfterB2, resetAfterB2At+1, 1_500_000, "late-reset-boundary")
	seedValuePackageResetReconcileUsage(t, resetAfterB2, resetAfterB2At+2, 2_500_000, "late-reset-after")
	require.NoError(t, DB.Create(&ValuePackageQuotaReset{
		UserId:             resetAfterB2.UserId,
		UserSubscriptionId: resetAfterB2.Id,
		PlanId:             plan.Id,
		PackageType:        ValuePackageTypeWeek,
		ResetAt:            resetAfterB2At,
		Source:             ValuePackageQuotaResetSourceUserConsumeCount,
		CreatedByUserId:    resetAfterB2.UserId,
	}).Error)

	resetBeforeB2Baseline := int64(9_000_000)
	resetBeforeB2Expected := int64(2_000_000)
	resetBeforeB2 := seedSub(7103, resetBeforeB2Baseline+plan.TotalAmount, resetBeforeB2Baseline+resetBeforeB2Expected, "redemption")
	resetBeforeB2At := valuePackageResetReconcileTestApplied - 100
	seedValuePackageResetReconcileUsage(t, resetBeforeB2, resetBeforeB2At, resetBeforeB2Baseline, "early-reset-before-b2")
	seedValuePackageResetReconcileUsage(t, resetBeforeB2, valuePackageResetReconcileTestApplied, resetBeforeB2Expected, "early-reset-after-b2")
	require.NoError(t, DB.Create(&ValuePackageQuotaReset{
		UserId:             resetBeforeB2.UserId,
		UserSubscriptionId: resetBeforeB2.Id,
		PlanId:             plan.Id,
		PackageType:        ValuePackageTypeWeek,
		ResetAt:            resetBeforeB2At,
		Source:             ValuePackageQuotaResetSourceUserConsumeCount,
		CreatedByUserId:    resetBeforeB2.UserId,
	}).Error)

	native := seedSub(7104, plan.TotalAmount+123, 999, "ldxp")
	native.StartTime = now - 8*valuePackageDaySeconds
	native.EndTime = native.StartTime + 2*valuePackageWeekSeconds
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", native.Id).Updates(map[string]interface{}{
		"start_time": native.StartTime,
		"end_time":   native.EndTime,
	}).Error)
	seedValuePackageResetReconcileUsage(t, native, now-100, 999, "native-evidence-mismatch")

	report := LegacyValuePackageQuotaMigrationReport{
		MigrationNow: valuePackageResetReconcileTestApplied,
		Rows: []LegacyValuePackageQuotaMigrationRow{
			{SubscriptionID: b2NoReset.Id, PlanID: plan.Id, PackageType: ValuePackageTypeWeek, AmountUsed: b2NoResetBaseline, OldTotal: 0, Grant: plan.TotalAmount, NewTotal: b2NoResetBaseline + plan.TotalAmount, EndTime: b2NoReset.EndTime},
			{SubscriptionID: resetAfterB2.Id, PlanID: plan.Id, PackageType: ValuePackageTypeWeek, AmountUsed: resetAfterB2Baseline, OldTotal: 0, Grant: plan.TotalAmount, NewTotal: resetAfterB2Baseline + plan.TotalAmount, EndTime: resetAfterB2.EndTime},
			{SubscriptionID: resetBeforeB2.Id, PlanID: plan.Id, PackageType: ValuePackageTypeWeek, AmountUsed: resetBeforeB2Baseline, OldTotal: 0, Grant: plan.TotalAmount, NewTotal: resetBeforeB2Baseline + plan.TotalAmount, EndTime: resetBeforeB2.EndTime},
		},
		Skipped: map[string]int{},
	}
	manifestHash, err := legacyValuePackageQuotaMigrationManifestHash(report.Rows)
	require.NoError(t, err)
	report.ManifestHash = manifestHash
	require.NoError(t, DB.Create(&ValuePackageQuotaMigrationReceipt{
		MigrationVersion: LegacyValuePackageQuotaMigrationVersion,
		ManifestHash:     manifestHash,
		AppliedAt:        valuePackageResetReconcileTestApplied,
		Updated:          len(report.Rows),
	}).Error)

	return valuePackageResetReconcileFixture{
		Plan:              plan,
		B2NoReset:         b2NoReset,
		ResetAfterB2:      resetAfterB2,
		ResetBeforeB2:     resetBeforeB2,
		Native:            native,
		B2Report:          report,
		ResetAfterB2At:    resetAfterB2At,
		ResetBeforeB2At:   resetBeforeB2At,
		B2NoResetExpected: b2NoResetExpected,
		ResetExpected:     resetExpected,
	}
}

func TestValuePackageResetReconcilePreviewUsesLatestResetOrB2Baseline(t *testing.T) {
	setupValuePackageResetReconcileTestDB(t)
	fixture := seedValuePackageResetReconcileFixture(t)

	report, err := PreviewValuePackageResetReconcile(DB, valuePackageResetReconcileTestNow, &fixture.B2Report)

	require.NoError(t, err)
	require.Len(t, report.Rows, 4)
	require.Regexp(t, `^[0-9a-f]{64}$`, report.ManifestHash)
	require.Equal(t, fixture.B2Report.ManifestHash, report.B2ManifestHash)
	rows := make(map[int]ValuePackageResetReconcileRow, len(report.Rows))
	for _, row := range report.Rows {
		rows[row.SubscriptionID] = row
	}

	b2 := rows[fixture.B2NoReset.Id]
	require.Equal(t, ValuePackageResetReconcileAnchorB2Migration, b2.AnchorType)
	require.Equal(t, valuePackageResetReconcileTestApplied, b2.AnchorAt)
	require.Equal(t, fixture.B2NoResetExpected, b2.NewUsed)
	require.Equal(t, fixture.Plan.TotalAmount, b2.NewTotal)
	require.EqualValues(t, 2, b2.EvidenceRecords)

	lateReset := rows[fixture.ResetAfterB2.Id]
	require.Equal(t, ValuePackageResetReconcileAnchorUserReset, lateReset.AnchorType)
	require.Equal(t, fixture.ResetAfterB2At, lateReset.AnchorAt)
	require.Equal(t, fixture.ResetExpected, lateReset.NewUsed)
	require.EqualValues(t, 2, lateReset.EvidenceRecords)

	earlyReset := rows[fixture.ResetBeforeB2.Id]
	require.Equal(t, ValuePackageResetReconcileAnchorB2Migration, earlyReset.AnchorType)
	require.Equal(t, valuePackageResetReconcileTestApplied, earlyReset.AnchorAt)
	require.EqualValues(t, 2_000_000, earlyReset.NewUsed)

	native := rows[fixture.Native.Id]
	require.Equal(t, ValuePackageResetReconcileAnchorPackagePeriod, native.AnchorType)
	require.Equal(t, fixture.Native.StartTime+valuePackageWeekSeconds, native.AnchorAt)
	require.Equal(t, native.AnchorAt, native.CycleStart)
	require.Equal(t, fixture.Native.AmountUsed, native.NewUsed)
	require.Equal(t, fixture.Plan.TotalAmount, native.NewTotal)

	for _, original := range []UserSubscription{fixture.B2NoReset, fixture.ResetAfterB2, fixture.ResetBeforeB2, fixture.Native} {
		var reloaded UserSubscription
		require.NoError(t, DB.First(&reloaded, original.Id).Error)
		require.Equal(t, original.AmountTotal, reloaded.AmountTotal)
		require.Equal(t, original.AmountUsed, reloaded.AmountUsed)
	}
}

func TestValuePackageResetReconcileB2EvidenceMismatchFailsClosed(t *testing.T) {
	setupValuePackageResetReconcileTestDB(t)
	fixture := seedValuePackageResetReconcileFixture(t)
	require.NoError(t, DB.Model(&ValuePackageUsageRecord{}).
		Where("user_subscription_id = ? AND request_id = ?", fixture.B2NoReset.Id, "reconcile-b2-after").
		Update("quota", 2_000_001).Error)

	report, err := PreviewValuePackageResetReconcile(DB, valuePackageResetReconcileTestNow, &fixture.B2Report)

	require.Error(t, err)
	require.Nil(t, report)
	require.Contains(t, err.Error(), "B2 usage evidence mismatch")
}

func TestValuePackageResetReconcileRejectsUsageInSameSecondAsUserReset(t *testing.T) {
	setupValuePackageResetReconcileTestDB(t)
	fixture := seedValuePackageResetReconcileFixture(t)
	require.NoError(t, DB.Model(&ValuePackageUsageRecord{}).
		Where("user_subscription_id = ? AND request_id = ?", fixture.ResetAfterB2.Id, "reconcile-late-reset-boundary").
		Update("created_at", fixture.ResetAfterB2At).Error)

	report, err := PreviewValuePackageResetReconcile(DB, valuePackageResetReconcileTestNow, &fixture.B2Report)

	require.Error(t, err)
	require.Nil(t, report)
	require.Contains(t, err.Error(), "ambiguous usage in the same second")
}

func TestValuePackageResetReconcileRejectsTamperedB2UsageBaseline(t *testing.T) {
	setupValuePackageResetReconcileTestDB(t)
	fixture := seedValuePackageResetReconcileFixture(t)
	tampered := fixture.B2Report
	tampered.Rows = append([]LegacyValuePackageQuotaMigrationRow(nil), fixture.B2Report.Rows...)
	tampered.Rows[0].AmountUsed--
	tampered.Rows[0].NewTotal--

	report, err := PreviewValuePackageResetReconcile(DB, valuePackageResetReconcileTestNow, &tampered)

	require.Error(t, err)
	require.Nil(t, report)
	require.Contains(t, err.Error(), "does not authenticate B2 baseline")
}

func TestValuePackageResetReconcileApplyUsesLockedLatestUsageAndCAS(t *testing.T) {
	setupValuePackageResetReconcileTestDB(t)
	fixture := seedValuePackageResetReconcileFixture(t)
	preview, err := PreviewValuePackageResetReconcile(DB, valuePackageResetReconcileTestNow, &fixture.B2Report)
	require.NoError(t, err)

	seedValuePackageResetReconcileUsage(t, fixture.B2NoReset, valuePackageResetReconcileTestNow-1, 500_000, "after-preview")
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", fixture.B2NoReset.Id).
		Update("amount_used", gorm.Expr("amount_used + ?", 500_000)).Error)

	applied, err := ApplyValuePackageResetReconcile(DB, valuePackageResetReconcileTestNow, &fixture.B2Report, preview.ManifestHash)

	require.NoError(t, err)
	require.Equal(t, 4, applied.Updated)
	require.Len(t, applied.Rows, 4)
	for _, row := range applied.Rows {
		var reloaded UserSubscription
		require.NoError(t, DB.First(&reloaded, row.SubscriptionID).Error)
		require.Equal(t, fixture.Plan.TotalAmount, reloaded.AmountTotal)
		require.Equal(t, row.NewUsed, reloaded.AmountUsed)
		require.Equal(t, row.CycleStart, reloaded.LastResetTime)
		require.Equal(t, row.NextCycleAt, reloaded.NextResetTime)
	}
	var b2Row ValuePackageResetReconcileRow
	for _, row := range applied.Rows {
		if row.SubscriptionID == fixture.B2NoReset.Id {
			b2Row = row
		}
	}
	require.Equal(t, fixture.B2NoResetExpected+500_000, b2Row.NewUsed)
	var preConsume SubscriptionPreConsumeRecord
	require.NoError(t, DB.Where("request_id = ?", "reconcile-b2-after").First(&preConsume).Error)
	require.EqualValues(t, b2Row.NewEpoch, preConsume.QuotaEpoch)
	var boundaryUsage ValuePackageUsageRecord
	require.NoError(t, DB.Where("request_id = ?", "reconcile-b2-boundary").First(&boundaryUsage).Error)
	require.EqualValues(t, b2Row.NewEpoch, boundaryUsage.QuotaEpoch)

	var receipt ValuePackageResetReconcileReceipt
	require.NoError(t, DB.Where("migration_version = ? AND manifest_hash = ?", ValuePackageResetReconcileVersion, preview.ManifestHash).First(&receipt).Error)
	require.Equal(t, fixture.B2Report.ManifestHash, receipt.B2ManifestHash)
	require.Equal(t, 4, receipt.Updated)

	replayed, err := ApplyValuePackageResetReconcile(DB, valuePackageResetReconcileTestNow, &fixture.B2Report, preview.ManifestHash)
	require.NoError(t, err)
	require.Zero(t, replayed.Updated)
	require.True(t, replayed.AlreadyApplied)
}

func TestValuePackageResetReconcileRejectsStaleStableManifest(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, fixture valuePackageResetReconcileFixture)
	}{
		{name: "plan total", mutate: func(t *testing.T, fixture valuePackageResetReconcileFixture) {
			require.NoError(t, DB.Model(&SubscriptionPlan{}).Where("id = ?", fixture.Plan.Id).Update("total_amount", fixture.Plan.TotalAmount+1).Error)
		}},
		{name: "new user reset", mutate: func(t *testing.T, fixture valuePackageResetReconcileFixture) {
			require.NoError(t, DB.Create(&ValuePackageQuotaReset{UserId: fixture.Native.UserId, UserSubscriptionId: fixture.Native.Id, PlanId: fixture.Plan.Id, PackageType: ValuePackageTypeWeek, ResetAt: valuePackageResetReconcileTestNow - 1, Source: ValuePackageQuotaResetSourceUserConsumeCount, CreatedByUserId: fixture.Native.UserId}).Error)
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupValuePackageResetReconcileTestDB(t)
			fixture := seedValuePackageResetReconcileFixture(t)
			preview, err := PreviewValuePackageResetReconcile(DB, valuePackageResetReconcileTestNow, &fixture.B2Report)
			require.NoError(t, err)
			test.mutate(t, fixture)

			report, err := ApplyValuePackageResetReconcile(DB, valuePackageResetReconcileTestNow, &fixture.B2Report, preview.ManifestHash)

			require.Error(t, err)
			require.Nil(t, report)
			for _, original := range []UserSubscription{fixture.B2NoReset, fixture.ResetAfterB2, fixture.ResetBeforeB2, fixture.Native} {
				var reloaded UserSubscription
				require.NoError(t, DB.First(&reloaded, original.Id).Error)
				require.Equal(t, original.AmountTotal, reloaded.AmountTotal)
			}
		})
	}
}

func TestValuePackageResetReconcileApplyIsAtomicOnUpdateFailure(t *testing.T) {
	setupValuePackageResetReconcileTestDB(t)
	fixture := seedValuePackageResetReconcileFixture(t)
	preview, err := PreviewValuePackageResetReconcile(DB, valuePackageResetReconcileTestNow, &fixture.B2Report)
	require.NoError(t, err)

	forcedErr := errors.New("forced reconcile update failure")
	callbackName := "test:value_package_reset_reconcile_update_failure"
	updates := 0
	require.NoError(t, DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "UserSubscription" {
			updates++
			if updates == 2 {
				tx.AddError(forcedErr)
			}
		}
	}))
	t.Cleanup(func() { require.NoError(t, DB.Callback().Update().Remove(callbackName)) })

	report, err := ApplyValuePackageResetReconcile(DB, valuePackageResetReconcileTestNow, &fixture.B2Report, preview.ManifestHash)

	require.ErrorIs(t, err, forcedErr)
	require.Nil(t, report)
	for _, original := range []UserSubscription{fixture.B2NoReset, fixture.ResetAfterB2, fixture.ResetBeforeB2, fixture.Native} {
		var reloaded UserSubscription
		require.NoError(t, DB.First(&reloaded, original.Id).Error)
		require.Equal(t, original.AmountTotal, reloaded.AmountTotal)
		require.Equal(t, original.AmountUsed, reloaded.AmountUsed)
	}
	var receiptCount int64
	require.NoError(t, DB.Model(&ValuePackageResetReconcileReceipt{}).Count(&receiptCount).Error)
	require.Zero(t, receiptCount)
}

func TestValuePackageResetReconcileApplyIsAtomicOnReceiptFailure(t *testing.T) {
	setupValuePackageResetReconcileTestDB(t)
	fixture := seedValuePackageResetReconcileFixture(t)
	preview, err := PreviewValuePackageResetReconcile(DB, valuePackageResetReconcileTestNow, &fixture.B2Report)
	require.NoError(t, err)

	forcedErr := errors.New("forced reconcile receipt failure")
	callbackName := "test:value_package_reset_reconcile_receipt_failure"
	require.NoError(t, DB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "ValuePackageResetReconcileReceipt" {
			tx.AddError(forcedErr)
		}
	}))
	t.Cleanup(func() { require.NoError(t, DB.Callback().Create().Remove(callbackName)) })

	report, err := ApplyValuePackageResetReconcile(DB, valuePackageResetReconcileTestNow, &fixture.B2Report, preview.ManifestHash)

	require.ErrorIs(t, err, forcedErr)
	require.Nil(t, report)
	for _, original := range []UserSubscription{fixture.B2NoReset, fixture.ResetAfterB2, fixture.ResetBeforeB2, fixture.Native} {
		var reloaded UserSubscription
		require.NoError(t, DB.First(&reloaded, original.Id).Error)
		require.Equal(t, original.AmountTotal, reloaded.AmountTotal)
		require.Equal(t, original.AmountUsed, reloaded.AmountUsed)
	}
}

func TestValuePackageResetReconcileValidatesB2ReportAndArguments(t *testing.T) {
	setupValuePackageResetReconcileTestDB(t)
	fixture := seedValuePackageResetReconcileFixture(t)

	_, err := PreviewValuePackageResetReconcile(nil, valuePackageResetReconcileTestNow, &fixture.B2Report)
	require.Error(t, err)
	_, err = PreviewValuePackageResetReconcile(DB, 0, &fixture.B2Report)
	require.Error(t, err)
	badReport := fixture.B2Report
	badReport.ManifestHash = strings.Repeat("a", 64)
	_, err = PreviewValuePackageResetReconcile(DB, valuePackageResetReconcileTestNow, &badReport)
	require.Error(t, err)
	_, err = ApplyValuePackageResetReconcile(DB, valuePackageResetReconcileTestNow, &fixture.B2Report, "")
	require.Error(t, err)
}
