package model

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const legacyValuePackageQuotaMigrationTestNow = int64(2_000_000_000)

func setupLegacyValuePackageQuotaMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupValuePackageTestDB(t)
	require.NoError(t, db.AutoMigrate(&ValuePackageQuotaMigrationReceipt{}))
	return db
}

func seedLegacyValuePackageQuotaMigrationPlan(t *testing.T, packageType string, grant int64, planKind string) SubscriptionPlan {
	t.Helper()
	plan := SubscriptionPlan{
		Title:         "sensitive-plan-title-" + packageType,
		PlanKind:      planKind,
		PackageType:   packageType,
		TotalAmount:   grant,
		DurationUnit:  SubscriptionDurationDay,
		DurationValue: 1,
		Enabled:       false,
	}
	require.NoError(t, DB.Create(&plan).Error)
	return plan
}

func seedLegacyValuePackageQuotaMigrationSub(t *testing.T, planID int, status string, endTime int64, amountTotal int64, amountUsed int64) UserSubscription {
	t.Helper()
	sub := UserSubscription{
		UserId:      9000 + planID,
		PlanId:      planID,
		AmountTotal: amountTotal,
		AmountUsed:  amountUsed,
		StartTime:   legacyValuePackageQuotaMigrationTestNow - 100,
		EndTime:     endTime,
		Status:      status,
		Source:      "migration-test",
	}
	require.NoError(t, DB.Create(&sub).Error)
	return sub
}

func TestLegacyValuePackageQuotaMigrationPreviewFiltersAndDoesNotWrite(t *testing.T) {
	setupLegacyValuePackageQuotaMigrationTestDB(t)
	now := legacyValuePackageQuotaMigrationTestNow
	day := seedLegacyValuePackageQuotaMigrationPlan(t, ValuePackageTypeDay, 2400, SubscriptionPlanKindValuePackage)
	week := seedLegacyValuePackageQuotaMigrationPlan(t, ValuePackageTypeWeek, 4500, SubscriptionPlanKindValuePackage)
	month := seedLegacyValuePackageQuotaMigrationPlan(t, ValuePackageTypeMonth, 22000, SubscriptionPlanKindValuePackage)
	valid := []UserSubscription{
		seedLegacyValuePackageQuotaMigrationSub(t, day.Id, UserSubscriptionStatusActive, now+1000, 0, 100),
		seedLegacyValuePackageQuotaMigrationSub(t, week.Id, UserSubscriptionStatusActive, now+2000, 0, 700),
		seedLegacyValuePackageQuotaMigrationSub(t, month.Id, UserSubscriptionStatusActive, now+3000, 0, 1300),
	}

	seedLegacyValuePackageQuotaMigrationSub(t, day.Id, UserSubscriptionStatusExpired, now+1000, 0, 1)
	seedLegacyValuePackageQuotaMigrationSub(t, day.Id, UserSubscriptionStatusActive, now, 0, 1)
	seedLegacyValuePackageQuotaMigrationSub(t, day.Id, UserSubscriptionStatusActive, now+1000, 99, 1)
	seedLegacyValuePackageQuotaMigrationSub(t, 999999, UserSubscriptionStatusActive, now+1000, 0, 1)
	ordinary := seedLegacyValuePackageQuotaMigrationPlan(t, ValuePackageTypeDay, 500, SubscriptionPlanKindSubscription)
	seedLegacyValuePackageQuotaMigrationSub(t, ordinary.Id, UserSubscriptionStatusActive, now+1000, 0, 1)
	invalidType := seedLegacyValuePackageQuotaMigrationPlan(t, "quarter", 500, SubscriptionPlanKindValuePackage)
	seedLegacyValuePackageQuotaMigrationSub(t, invalidType.Id, UserSubscriptionStatusActive, now+1000, 0, 1)
	zeroGrant := seedLegacyValuePackageQuotaMigrationPlan(t, ValuePackageTypeDay, 0, SubscriptionPlanKindValuePackage)
	seedLegacyValuePackageQuotaMigrationSub(t, zeroGrant.Id, UserSubscriptionStatusActive, now+1000, 0, 1)
	negativeGrant := seedLegacyValuePackageQuotaMigrationPlan(t, ValuePackageTypeWeek, -1, SubscriptionPlanKindValuePackage)
	seedLegacyValuePackageQuotaMigrationSub(t, negativeGrant.Id, UserSubscriptionStatusActive, now+1000, 0, 1)

	report, err := PreviewLegacyValuePackageQuotaMigration(DB, now)

	require.NoError(t, err)
	require.Equal(t, now, report.MigrationNow)
	require.Len(t, report.Rows, 3)
	for i, row := range report.Rows {
		require.Equal(t, valid[i].Id, row.SubscriptionID)
		require.Equal(t, valid[i].PlanId, row.PlanID)
		require.Equal(t, valid[i].AmountUsed, row.AmountUsed)
		require.Zero(t, row.OldTotal)
		require.Equal(t, row.AmountUsed+row.Grant, row.NewTotal)
		require.Equal(t, valid[i].EndTime, row.EndTime)
	}
	require.Equal(t, map[string]int{
		"invalid_package_type": 1,
		"invalid_plan_total":   2,
		"missing_plan":         1,
		"not_value_package":    1,
	}, report.Skipped)
	require.Zero(t, report.Updated)
	require.Regexp(t, regexp.MustCompile(`^[0-9a-f]{64}$`), report.ManifestHash)
	for _, sub := range valid {
		var reloaded UserSubscription
		require.NoError(t, DB.First(&reloaded, sub.Id).Error)
		require.Zero(t, reloaded.AmountTotal)
	}
}

func TestLegacyValuePackageQuotaMigrationManifestUsesOnlyStableAuthorizationFields(t *testing.T) {
	setupLegacyValuePackageQuotaMigrationTestDB(t)
	now := legacyValuePackageQuotaMigrationTestNow
	plan := seedLegacyValuePackageQuotaMigrationPlan(t, ValuePackageTypeWeek, 4500, SubscriptionPlanKindValuePackage)
	sub := seedLegacyValuePackageQuotaMigrationSub(t, plan.Id, UserSubscriptionStatusActive, now+10000, 0, 700)

	preview, err := PreviewLegacyValuePackageQuotaMigration(DB, now)
	require.NoError(t, err)
	require.Len(t, preview.Rows, 1)

	stableRows := []struct {
		SubscriptionID int    `json:"subscription_id"`
		PlanID         int    `json:"plan_id"`
		PackageType    string `json:"package_type"`
		Grant          int64  `json:"grant"`
		EndTime        int64  `json:"end_time"`
	}{
		{SubscriptionID: sub.Id, PlanID: plan.Id, PackageType: ValuePackageTypeWeek, Grant: plan.TotalAmount, EndTime: sub.EndTime},
	}
	stableJSON, err := common.Marshal(stableRows)
	require.NoError(t, err)
	wantHash := fmt.Sprintf("%x", sha256.Sum256(stableJSON))
	require.Equal(t, wantHash, preview.ManifestHash)

	payload, err := common.Marshal(preview)
	require.NoError(t, err)
	payloadText := string(payload)
	for _, forbidden := range []string{"username", "email", "title", "token", "sensitive-plan-title"} {
		require.NotContains(t, payloadText, forbidden)
	}
}

func TestLegacyValuePackageQuotaMigrationHashIgnoresNowAndUsageAndApplyUsesLockedUsage(t *testing.T) {
	setupLegacyValuePackageQuotaMigrationTestDB(t)
	now := legacyValuePackageQuotaMigrationTestNow
	plan := seedLegacyValuePackageQuotaMigrationPlan(t, ValuePackageTypeDay, 2400, SubscriptionPlanKindValuePackage)
	sub := seedLegacyValuePackageQuotaMigrationSub(t, plan.Id, UserSubscriptionStatusActive, now+10000, 0, 100)
	preview, err := PreviewLegacyValuePackageQuotaMigration(DB, now)
	require.NoError(t, err)

	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("amount_used", 777).Error)
	previewAfterUsage, err := PreviewLegacyValuePackageQuotaMigration(DB, now)
	require.NoError(t, err)
	previewAtDifferentNow, err := PreviewLegacyValuePackageQuotaMigration(DB, now+60)
	require.NoError(t, err)
	require.Equal(t, preview.ManifestHash, previewAfterUsage.ManifestHash)
	require.Equal(t, preview.ManifestHash, previewAtDifferentNow.ManifestHash)

	applied, err := ApplyLegacyValuePackageQuotaMigration(DB, now+60, preview.ManifestHash)
	require.NoError(t, err)
	require.Equal(t, 1, applied.Updated)
	require.Len(t, applied.Rows, 1)
	require.EqualValues(t, 777, applied.Rows[0].AmountUsed)
	require.EqualValues(t, 3177, applied.Rows[0].NewTotal)
	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, sub.Id).Error)
	require.EqualValues(t, 777, reloaded.AmountUsed)
	require.EqualValues(t, 3177, reloaded.AmountTotal)
}

func TestLegacyValuePackageQuotaMigrationRejectsStaleAuthorization(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(t *testing.T, target UserSubscription, targetPlan SubscriptionPlan, replacement SubscriptionPlan, now int64)
	}{
		{name: "plan grant", mutate: func(t *testing.T, _ UserSubscription, plan SubscriptionPlan, _ SubscriptionPlan, _ int64) {
			require.NoError(t, DB.Model(&SubscriptionPlan{}).Where("id = ?", plan.Id).Update("total_amount", plan.TotalAmount+1).Error)
		}},
		{name: "plan type", mutate: func(t *testing.T, _ UserSubscription, plan SubscriptionPlan, _ SubscriptionPlan, _ int64) {
			require.NoError(t, DB.Model(&SubscriptionPlan{}).Where("id = ?", plan.Id).Update("package_type", ValuePackageTypeMonth).Error)
		}},
		{name: "subscription plan id", mutate: func(t *testing.T, sub UserSubscription, _ SubscriptionPlan, replacement SubscriptionPlan, _ int64) {
			require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("plan_id", replacement.Id).Error)
		}},
		{name: "subscription end", mutate: func(t *testing.T, sub UserSubscription, _ SubscriptionPlan, _ SubscriptionPlan, _ int64) {
			require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("end_time", sub.EndTime+1).Error)
		}},
		{name: "membership status", mutate: func(t *testing.T, sub UserSubscription, _ SubscriptionPlan, _ SubscriptionPlan, _ int64) {
			require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("status", UserSubscriptionStatusExpired).Error)
		}},
		{name: "membership end", mutate: func(t *testing.T, sub UserSubscription, _ SubscriptionPlan, _ SubscriptionPlan, now int64) {
			require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("end_time", now).Error)
		}},
	}

	for _, tt := range mutations {
		t.Run(tt.name, func(t *testing.T) {
			setupLegacyValuePackageQuotaMigrationTestDB(t)
			now := legacyValuePackageQuotaMigrationTestNow
			plan := seedLegacyValuePackageQuotaMigrationPlan(t, ValuePackageTypeWeek, 4500, SubscriptionPlanKindValuePackage)
			replacement := seedLegacyValuePackageQuotaMigrationPlan(t, ValuePackageTypeWeek, 9999, SubscriptionPlanKindValuePackage)
			target := seedLegacyValuePackageQuotaMigrationSub(t, plan.Id, UserSubscriptionStatusActive, now+1000, 0, 100)
			other := seedLegacyValuePackageQuotaMigrationSub(t, plan.Id, UserSubscriptionStatusActive, now+2000, 0, 200)
			preview, err := PreviewLegacyValuePackageQuotaMigration(DB, now)
			require.NoError(t, err)
			tt.mutate(t, target, plan, replacement, now)

			report, err := ApplyLegacyValuePackageQuotaMigration(DB, now, preview.ManifestHash)
			require.Error(t, err)
			require.Nil(t, report)
			for _, id := range []int{target.Id, other.Id} {
				var reloaded UserSubscription
				require.NoError(t, DB.First(&reloaded, id).Error)
				require.Zero(t, reloaded.AmountTotal)
			}
		})
	}
}

func TestLegacyValuePackageQuotaMigrationApplyIsAtomicAndIdempotent(t *testing.T) {
	setupLegacyValuePackageQuotaMigrationTestDB(t)
	now := legacyValuePackageQuotaMigrationTestNow
	plan := seedLegacyValuePackageQuotaMigrationPlan(t, ValuePackageTypeMonth, 22000, SubscriptionPlanKindValuePackage)
	subs := []UserSubscription{
		seedLegacyValuePackageQuotaMigrationSub(t, plan.Id, UserSubscriptionStatusActive, now+1000, 0, 100),
		seedLegacyValuePackageQuotaMigrationSub(t, plan.Id, UserSubscriptionStatusActive, now+2000, 0, 900),
	}
	preview, err := PreviewLegacyValuePackageQuotaMigration(DB, now)
	require.NoError(t, err)

	applied, err := ApplyLegacyValuePackageQuotaMigration(DB, now, preview.ManifestHash)
	require.NoError(t, err)
	require.Equal(t, len(subs), applied.Updated)
	require.Len(t, applied.Rows, len(subs))
	for i, before := range subs {
		row := applied.Rows[i]
		require.Equal(t, before.Id, row.SubscriptionID)
		require.Equal(t, before.AmountUsed, row.AmountUsed)
		require.Equal(t, before.AmountUsed+plan.TotalAmount, row.NewTotal)
		var after UserSubscription
		require.NoError(t, DB.First(&after, before.Id).Error)
		require.Equal(t, before.Status, after.Status)
		require.Equal(t, before.EndTime, after.EndTime)
		require.Equal(t, before.AmountUsed, after.AmountUsed)
		require.Equal(t, row.NewTotal, after.AmountTotal)
	}
	var receipt ValuePackageQuotaMigrationReceipt
	require.NoError(t, DB.Where("migration_version = ? AND manifest_hash = ?", LegacyValuePackageQuotaMigrationVersion, preview.ManifestHash).First(&receipt).Error)
	require.Equal(t, LegacyValuePackageQuotaMigrationVersion, receipt.MigrationVersion)
	require.Equal(t, preview.ManifestHash, receipt.ManifestHash)
	require.Positive(t, receipt.AppliedAt)
	require.Equal(t, len(subs), receipt.Updated)

	replayed, err := ApplyLegacyValuePackageQuotaMigration(DB, now, preview.ManifestHash)
	require.NoError(t, err)
	require.Zero(t, replayed.Updated)
	require.Empty(t, replayed.Rows)
	require.Equal(t, preview.ManifestHash, replayed.ManifestHash)
	var receiptCount int64
	require.NoError(t, DB.Model(&ValuePackageQuotaMigrationReceipt{}).Count(&receiptCount).Error)
	require.EqualValues(t, 1, receiptCount)
}

func TestLegacyValuePackageQuotaMigrationSecondUpdateFailureRollsBackAll(t *testing.T) {
	setupLegacyValuePackageQuotaMigrationTestDB(t)
	now := legacyValuePackageQuotaMigrationTestNow
	plan := seedLegacyValuePackageQuotaMigrationPlan(t, ValuePackageTypeDay, 2400, SubscriptionPlanKindValuePackage)
	first := seedLegacyValuePackageQuotaMigrationSub(t, plan.Id, UserSubscriptionStatusActive, now+1000, 0, 100)
	second := seedLegacyValuePackageQuotaMigrationSub(t, plan.Id, UserSubscriptionStatusActive, now+2000, 0, 200)
	preview, err := PreviewLegacyValuePackageQuotaMigration(DB, now)
	require.NoError(t, err)

	forcedErr := errors.New("forced second migration update failure")
	callbackName := "test:legacy_value_package_quota_migration_second_update"
	updateCount := 0
	require.NoError(t, DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "UserSubscription" {
			updateCount++
			if updateCount == 2 {
				tx.AddError(forcedErr)
			}
		}
	}))
	t.Cleanup(func() { require.NoError(t, DB.Callback().Update().Remove(callbackName)) })

	report, err := ApplyLegacyValuePackageQuotaMigration(DB, now, preview.ManifestHash)
	require.ErrorIs(t, err, forcedErr)
	require.Nil(t, report)
	for _, id := range []int{first.Id, second.Id} {
		var reloaded UserSubscription
		require.NoError(t, DB.First(&reloaded, id).Error)
		require.Zero(t, reloaded.AmountTotal)
	}
	var receiptCount int64
	require.NoError(t, DB.Model(&ValuePackageQuotaMigrationReceipt{}).Count(&receiptCount).Error)
	require.Zero(t, receiptCount)
}

func TestLegacyValuePackageQuotaMigrationReceiptFailureRollsBackAllUpdates(t *testing.T) {
	setupLegacyValuePackageQuotaMigrationTestDB(t)
	now := legacyValuePackageQuotaMigrationTestNow
	plan := seedLegacyValuePackageQuotaMigrationPlan(t, ValuePackageTypeDay, 2400, SubscriptionPlanKindValuePackage)
	first := seedLegacyValuePackageQuotaMigrationSub(t, plan.Id, UserSubscriptionStatusActive, now+1000, 0, 100)
	second := seedLegacyValuePackageQuotaMigrationSub(t, plan.Id, UserSubscriptionStatusActive, now+2000, 0, 200)
	preview, err := PreviewLegacyValuePackageQuotaMigration(DB, now)
	require.NoError(t, err)

	forcedErr := errors.New("forced migration receipt creation failure")
	callbackName := "test:legacy_value_package_quota_migration_receipt_failure"
	require.NoError(t, DB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "ValuePackageQuotaMigrationReceipt" {
			tx.AddError(forcedErr)
		}
	}))
	t.Cleanup(func() { require.NoError(t, DB.Callback().Create().Remove(callbackName)) })

	report, err := ApplyLegacyValuePackageQuotaMigration(DB, now, preview.ManifestHash)

	require.ErrorIs(t, err, forcedErr)
	require.Nil(t, report)
	for _, id := range []int{first.Id, second.Id} {
		var reloaded UserSubscription
		require.NoError(t, DB.First(&reloaded, id).Error)
		require.Zero(t, reloaded.AmountTotal)
	}
	var receiptCount int64
	require.NoError(t, DB.Model(&ValuePackageQuotaMigrationReceipt{}).Count(&receiptCount).Error)
	require.Zero(t, receiptCount)
}

func TestLegacyValuePackageQuotaMigrationReceiptDoesNotBlockRestoredTargets(t *testing.T) {
	setupLegacyValuePackageQuotaMigrationTestDB(t)
	now := legacyValuePackageQuotaMigrationTestNow
	plan := seedLegacyValuePackageQuotaMigrationPlan(t, ValuePackageTypeWeek, 4500, SubscriptionPlanKindValuePackage)
	target := seedLegacyValuePackageQuotaMigrationSub(t, plan.Id, UserSubscriptionStatusActive, now+1000, 0, 700)
	preview, err := PreviewLegacyValuePackageQuotaMigration(DB, now)
	require.NoError(t, err)
	first, err := ApplyLegacyValuePackageQuotaMigration(DB, now, preview.ManifestHash)
	require.NoError(t, err)
	require.Equal(t, 1, first.Updated)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", target.Id).Update("amount_total", 0).Error)

	restored, err := ApplyLegacyValuePackageQuotaMigration(DB, now, preview.ManifestHash)

	require.NoError(t, err)
	require.Equal(t, 1, restored.Updated)
	require.Len(t, restored.Rows, 1)
	require.EqualValues(t, target.AmountUsed+plan.TotalAmount, restored.Rows[0].NewTotal)
	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, target.Id).Error)
	require.EqualValues(t, target.AmountUsed+plan.TotalAmount, reloaded.AmountTotal)
}

func TestLegacyValuePackageQuotaMigrationConditionalMembershipChangeDoesNotPartiallyWrite(t *testing.T) {
	setupLegacyValuePackageQuotaMigrationTestDB(t)
	now := legacyValuePackageQuotaMigrationTestNow
	plan := seedLegacyValuePackageQuotaMigrationPlan(t, ValuePackageTypeWeek, 4500, SubscriptionPlanKindValuePackage)
	changed := seedLegacyValuePackageQuotaMigrationSub(t, plan.Id, UserSubscriptionStatusActive, now+1000, 0, 100)
	other := seedLegacyValuePackageQuotaMigrationSub(t, plan.Id, UserSubscriptionStatusActive, now+2000, 0, 200)
	preview, err := PreviewLegacyValuePackageQuotaMigration(DB, now)
	require.NoError(t, err)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", changed.Id).Update("amount_total", 1).Error)

	report, err := ApplyLegacyValuePackageQuotaMigration(DB, now, preview.ManifestHash)
	require.Error(t, err)
	require.Nil(t, report)
	var changedAfter UserSubscription
	require.NoError(t, DB.First(&changedAfter, changed.Id).Error)
	require.EqualValues(t, 1, changedAfter.AmountTotal)
	var otherAfter UserSubscription
	require.NoError(t, DB.First(&otherAfter, other.Id).Error)
	require.Zero(t, otherAfter.AmountTotal)
}

func TestLegacyValuePackageQuotaMigrationRejectsStaleManifestWhenAllTargetsDisappear(t *testing.T) {
	setupLegacyValuePackageQuotaMigrationTestDB(t)
	now := legacyValuePackageQuotaMigrationTestNow
	plan := seedLegacyValuePackageQuotaMigrationPlan(t, ValuePackageTypeDay, 2400, SubscriptionPlanKindValuePackage)
	target := seedLegacyValuePackageQuotaMigrationSub(t, plan.Id, UserSubscriptionStatusActive, now+1000, 0, 100)
	preview, err := PreviewLegacyValuePackageQuotaMigration(DB, now)
	require.NoError(t, err)
	require.Len(t, preview.Rows, 1)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", target.Id).Update("status", UserSubscriptionStatusExpired).Error)

	report, err := ApplyLegacyValuePackageQuotaMigration(DB, now, preview.ManifestHash)

	require.Error(t, err)
	require.Nil(t, report)
	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, target.Id).Error)
	require.Zero(t, reloaded.AmountTotal)
}

func TestLegacyValuePackageQuotaMigrationRejectsArbitraryManifestForInitiallyEmptyTargets(t *testing.T) {
	setupLegacyValuePackageQuotaMigrationTestDB(t)

	report, err := ApplyLegacyValuePackageQuotaMigration(DB, legacyValuePackageQuotaMigrationTestNow, strings.Repeat("a", 64))

	require.Error(t, err)
	require.Nil(t, report)
}

func TestLegacyValuePackageQuotaMigrationValidatesArguments(t *testing.T) {
	setupLegacyValuePackageQuotaMigrationTestDB(t)
	_, err := PreviewLegacyValuePackageQuotaMigration(nil, legacyValuePackageQuotaMigrationTestNow)
	require.Error(t, err)
	_, err = PreviewLegacyValuePackageQuotaMigration(DB, 0)
	require.Error(t, err)
	_, err = ApplyLegacyValuePackageQuotaMigration(nil, legacyValuePackageQuotaMigrationTestNow, strings.Repeat("a", 64))
	require.Error(t, err)
	_, err = ApplyLegacyValuePackageQuotaMigration(DB, 0, strings.Repeat("a", 64))
	require.Error(t, err)
	_, err = ApplyLegacyValuePackageQuotaMigration(DB, legacyValuePackageQuotaMigrationTestNow, "")
	require.Error(t, err)
}
