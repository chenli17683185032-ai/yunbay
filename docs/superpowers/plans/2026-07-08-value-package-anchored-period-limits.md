# Value Package Anchored Period Limits Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix 云贝超值套餐 so 7-day limits are anchored to subscription start time, reset behavior differs by day/week/month cards, month cards are fixed 30 days, and admin/user UI clearly configures and displays 1/7/30-day total limits.

**Architecture:** Keep the existing database schema and use `total_amount` as the package-period total quota. Add small backend helpers that compute anchored 7-day windows from `UserSubscription.StartTime`, apply `ValuePackageQuotaReset` to 5-hour windows for every package and additionally to month-card 7-day period usage, then reuse those helpers in state summary, pre-consume, realtime reserve, and management rows. On the frontend, centralize value-package labels/visibility in a small helper module so the mutate drawer, admin cards, and user package card show the same semantics.

**Tech Stack:** Go 1.22+, Gin, GORM v2, SQLite/MySQL/PostgreSQL-compatible queries; React 19, TypeScript, Rsbuild, Bun, i18next; tests with Go `testing`/`testify` and Node `node:test`.

---

## Execution Notes

- The current worktree may already contain uncommitted changes in `/Users/ethan/Documents/yunbay/model/subscription.go` and `/Users/ethan/Documents/yunbay/model/value_package_test.go` from the previous fixed 5-hour window bugfix. Do not reset or overwrite them. Implement on top of them.
- Follow TDD strictly: for every production-code change below, add the named failing test first, run it and confirm the expected failure, then implement the smallest code that passes.
- Do not add database fields. Use existing `total_amount`, `limit_5h_amount`, `limit_7d_amount`, `duration_unit`, `duration_value`, and `custom_seconds`.
- Do not directly call `encoding/json` marshal/unmarshal in business code. This plan does not require new backend JSON handling.
- Keep SQL simple GORM queries and Go-side filtering for cross-database compatibility.

## File Structure

Backend files:

- Modify `/Users/ethan/Documents/yunbay/model/subscription.go`
  - Add anchored-window helpers.
  - Normalize value-package fixed durations.
  - Replace 7-day rolling usage with anchored 7-day period usage.
  - Apply reset scope by package type.
  - Update pre-consume and reserve checks to use the same usage helper.
  - Update management-row usage summaries.
- Modify `/Users/ethan/Documents/yunbay/model/value_package_test.go`
  - Add backend regression tests for anchored 7-day windows, reset scope, day-card 7-day ignore, week-card total quota, month-card 30-day duration, reserve replacement, and error text.

Frontend files:

- Modify `/Users/ethan/Documents/yunbay/web/default/src/features/subscriptions/constants.ts`
  - Change month-card duration to `day/30`.
- Modify `/Users/ethan/Documents/yunbay/web/default/src/features/subscriptions/lib/plan-form.ts`
  - Normalize value-package `limit_7d_amount` submission by package type.
- Modify `/Users/ethan/Documents/yunbay/web/default/src/features/subscriptions/lib/plan-form-value-package.test.ts`
  - Add payload/duration tests for day/week/month package limits.
- Create `/Users/ethan/Documents/yunbay/web/default/src/features/subscriptions/lib/value-package-limit-labels.ts`
  - Central helper for total-limit labels, descriptions, and 7-day-period-limit visibility.
- Create `/Users/ethan/Documents/yunbay/web/default/src/features/subscriptions/lib/value-package-limit-labels.test.ts`
  - Unit tests for label keys and field visibility.
- Modify `/Users/ethan/Documents/yunbay/web/default/src/features/subscriptions/components/subscriptions-mutate-drawer.tsx`
  - Use dynamic total-limit labels/descriptions.
  - Hide 7-day period limit unless package type is month.
- Modify `/Users/ethan/Documents/yunbay/web/default/src/features/subscriptions/components/subscriptions-mutate-drawer-value-package-source.test.ts`
  - Source-level guard for dynamic labels and month-only 7-day period field.
- Modify `/Users/ethan/Documents/yunbay/web/default/src/features/subscriptions/components/value-package-admin-cards.tsx`
  - Show 1/7/30-day total limit and month-only 7-day period limit.
- Modify `/Users/ethan/Documents/yunbay/web/default/src/features/value-packages/components/value-package-card.tsx`
  - Show dynamic total-limit label.
  - Hide 7-day period usage except month-card `limit_7d > 0`.
  - Change reset confirmation text to day/week 5h-only and month 5h+7d.
- Modify `/Users/ethan/Documents/yunbay/web/default/src/features/value-packages/types.ts`
  - No required type change if existing fields remain; edit only if TypeScript requires helper imports.
- Modify locale files:
  - `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/en.json`
  - `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/zh.json`
  - `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/fr.json`
  - `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/ja.json`
  - `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/ru.json`
  - `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/vi.json`

---

### Task 1: Backend foundation helpers and fixed package durations

**Files:**
- Modify: `/Users/ethan/Documents/yunbay/model/value_package_test.go`
- Modify: `/Users/ethan/Documents/yunbay/model/subscription.go`

- [ ] **Step 1: Inspect current worktree and preserve existing changes**

Run:

```bash
cd /Users/ethan/Documents/yunbay
git status --short --branch
git diff -- model/subscription.go model/value_package_test.go | sed -n '1,220p'
```

Expected:

```text
model/subscription.go and model/value_package_test.go may already be modified.
No command in this plan should run git reset or overwrite those changes.
```

- [ ] **Step 2: Write failing helper/duration tests**

Append these tests to `/Users/ethan/Documents/yunbay/model/value_package_test.go` near the other value-package window tests:

```go
func TestValuePackageAnchoredWindowUsesSubscriptionStartAndClampsToEnd(t *testing.T) {
	start := int64(1_700_000_000)
	end := start + 30*24*3600

	first := calcValuePackageAnchoredWindow(start, end, 7*24*3600, start+3*24*3600)
	require.EqualValues(t, start, first.Start)
	require.EqualValues(t, start+7*24*3600, first.End)

	second := calcValuePackageAnchoredWindow(start, end, 7*24*3600, start+8*24*3600)
	require.EqualValues(t, start+7*24*3600, second.Start)
	require.EqualValues(t, start+14*24*3600, second.End)

	shortFinal := calcValuePackageAnchoredWindow(start, end, 7*24*3600, start+29*24*3600)
	require.EqualValues(t, start+28*24*3600, shortFinal.Start)
	require.EqualValues(t, end, shortFinal.End)
}

func TestNormalizeValuePackagePlanUsesFixedDurations(t *testing.T) {
	day := SubscriptionPlan{PlanKind: SubscriptionPlanKindValuePackage, PackageType: ValuePackageTypeDay, DurationUnit: SubscriptionDurationMonth, DurationValue: 99, Limit7dAmount: 123}
	normalizeValuePackagePlan(&day)
	require.Equal(t, SubscriptionDurationDay, day.DurationUnit)
	require.Equal(t, 1, day.DurationValue)
	require.EqualValues(t, 0, day.CustomSeconds)
	require.EqualValues(t, 0, day.Limit7dAmount)

	week := SubscriptionPlan{PlanKind: SubscriptionPlanKindValuePackage, PackageType: ValuePackageTypeWeek, DurationUnit: SubscriptionDurationMonth, DurationValue: 99}
	normalizeValuePackagePlan(&week)
	require.Equal(t, SubscriptionDurationDay, week.DurationUnit)
	require.Equal(t, 7, week.DurationValue)
	require.EqualValues(t, 0, week.CustomSeconds)

	month := SubscriptionPlan{PlanKind: SubscriptionPlanKindValuePackage, PackageType: ValuePackageTypeMonth, DurationUnit: SubscriptionDurationMonth, DurationValue: 1}
	normalizeValuePackagePlan(&month)
	require.Equal(t, SubscriptionDurationDay, month.DurationUnit)
	require.Equal(t, 30, month.DurationValue)
	require.EqualValues(t, 0, month.CustomSeconds)
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run:

```bash
cd /Users/ethan/Documents/yunbay
go test ./model -run 'TestValuePackageAnchoredWindowUsesSubscriptionStartAndClampsToEnd|TestNormalizeValuePackagePlanUsesFixedDurations' -count=1
```

Expected:

```text
FAIL
undefined: calcValuePackageAnchoredWindow
```

or failures showing month still normalizes to `month/1` and day still retains `Limit7dAmount`.

- [ ] **Step 4: Add helper and duration normalization**

Modify `/Users/ethan/Documents/yunbay/model/subscription.go`.

Add these constants near the other value-package constants:

```go
const (
	valuePackage5hWindowSeconds = int64(5 * 3600)
	valuePackage7dWindowSeconds = int64(7 * 24 * 3600)
	valuePackageDaySeconds      = int64(24 * 3600)
	valuePackageWeekSeconds     = int64(7 * 24 * 3600)
	valuePackageMonthSeconds    = int64(30 * 24 * 3600)
)
```

Add this type and helper near `maxInt64`:

```go
type valuePackageAnchoredWindow struct {
	Start int64
	End   int64
}

func calcValuePackageAnchoredWindow(startTime int64, endTime int64, windowSeconds int64, now int64) valuePackageAnchoredWindow {
	if startTime <= 0 || windowSeconds <= 0 {
		return valuePackageAnchoredWindow{}
	}
	if now <= 0 || now < startTime {
		now = startTime
	}
	index := (now - startTime) / windowSeconds
	windowStart := startTime + index*windowSeconds
	windowEnd := windowStart + windowSeconds
	if endTime > 0 && windowEnd > endTime {
		windowEnd = endTime
	}
	if windowEnd <= windowStart {
		return valuePackageAnchoredWindow{}
	}
	return valuePackageAnchoredWindow{Start: windowStart, End: windowEnd}
}

func valuePackageHas7dWindow(plan *SubscriptionPlan) bool {
	return plan != nil && plan.IsValuePackage() && plan.PackageType != ValuePackageTypeDay && plan.Limit7dAmount > 0
}

func valuePackageResetClears7d(plan *SubscriptionPlan) bool {
	return plan != nil && plan.IsValuePackage() && plan.PackageType == ValuePackageTypeMonth && plan.Limit7dAmount > 0
}
```

Update `normalizeValuePackagePlan` in `/Users/ethan/Documents/yunbay/model/subscription.go` so it contains this logic after package level normalization and before concurrency normalization:

```go
	switch plan.PackageType {
	case ValuePackageTypeDay:
		plan.DurationUnit = SubscriptionDurationDay
		plan.DurationValue = 1
		plan.CustomSeconds = 0
		plan.Limit7dAmount = 0
	case ValuePackageTypeWeek:
		plan.DurationUnit = SubscriptionDurationDay
		plan.DurationValue = 7
		plan.CustomSeconds = 0
	case ValuePackageTypeMonth:
		plan.DurationUnit = SubscriptionDurationDay
		plan.DurationValue = 30
		plan.CustomSeconds = 0
	}
```

Update `NormalizeDefaults` so value-package plans are normalized when controllers call `plan.NormalizeDefaults()`. Do not call `normalizeValuePackagePlan(p)` from inside `NormalizeDefaults`, because `normalizeValuePackagePlan` already calls `plan.NormalizeDefaults()` and that would recurse. Use this direct branch instead:

```go
	if p.PlanKind == SubscriptionPlanKindValuePackage {
		p.Currency = "CNY"
		switch p.PackageType {
		case ValuePackageTypeDay:
			p.DurationUnit = SubscriptionDurationDay
			p.DurationValue = 1
			p.CustomSeconds = 0
			p.Limit7dAmount = 0
		case ValuePackageTypeWeek:
			p.DurationUnit = SubscriptionDurationDay
			p.DurationValue = 7
			p.CustomSeconds = 0
		case ValuePackageTypeMonth:
			p.DurationUnit = SubscriptionDurationDay
			p.DurationValue = 30
			p.CustomSeconds = 0
		}
	} else {
		p.Currency = "USD"
	}
```

- [ ] **Step 5: Run tests to verify they pass**

Run:

```bash
cd /Users/ethan/Documents/yunbay
go test ./model -run 'TestValuePackageAnchoredWindowUsesSubscriptionStartAndClampsToEnd|TestNormalizeValuePackagePlanUsesFixedDurations' -count=1
```

Expected:

```text
ok  	github.com/QuantumNous/new-api/model
```

- [ ] **Step 6: Commit Task 1**

```bash
cd /Users/ethan/Documents/yunbay
git add model/subscription.go model/value_package_test.go
git commit -m "fix: normalize value package periods"
```

---

### Task 2: Backend anchored 7-day usage details and reset scope

**Files:**
- Modify: `/Users/ethan/Documents/yunbay/model/value_package_test.go`
- Modify: `/Users/ethan/Documents/yunbay/model/subscription.go`

- [ ] **Step 1: Write failing anchored-window and reset-scope tests**

Append these tests to `/Users/ethan/Documents/yunbay/model/value_package_test.go` near `TestValuePackageWindowUsageCountsUsageAfterLastReset`:

```go
func TestValuePackageWindowUsageAnchors7dToSubscriptionStart(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3801, UserGroupTiyan)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	month.Limit7dAmount = 100
	require.NoError(t, DB.Save(&month).Error)
	now := common.GetTimestamp()
	start := now - 8*24*3600
	sub := createActiveValuePackageSub(t, user.Id, month, start, start+30*24*3600)

	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ModelGroup: month.ModelGroup, RequestId: "previous-anchored-window", Quota: 80, CreatedAt: start + 2*24*3600}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ModelGroup: month.ModelGroup, RequestId: "current-anchored-window", Quota: 15, CreatedAt: start + 7*24*3600 + 3600}))

	details, err := GetValuePackageWindowUsageDetails(user.Id, sub.Id, now)

	require.NoError(t, err)
	require.NotNil(t, details)
	require.EqualValues(t, 15, details.Used7d)
	require.EqualValues(t, start+7*24*3600, details.Earliest7dCreatedAt)
	require.EqualValues(t, start+14*24*3600, details.ResetAt7d)
	require.EqualValues(t, start+14*24*3600-now, details.ResetSeconds7d)
}

func TestValuePackageWindowUsageResetScopeByPackageType(t *testing.T) {
	setupValuePackageTestDB(t)
	now := common.GetTimestamp()

	weekUser := createValuePackageUser(t, 3802, UserGroupTiyan)
	week := createValuePackagePlan(t, ValuePackageTypeWeek, ValuePackageLevelWeek, 7, 19.9)
	week.Limit7dAmount = 100
	require.NoError(t, DB.Save(&week).Error)
	weekStart := now - 2*24*3600
	weekSub := createActiveValuePackageSub(t, weekUser.Id, week, weekStart, weekStart+7*24*3600)
	require.NoError(t, DB.Create(&ValuePackageQuotaReset{UserId: weekUser.Id, UserSubscriptionId: weekSub.Id, PlanId: week.Id, PackageType: week.PackageType, ResetAt: now - 1800, Source: ValuePackageQuotaResetSourceUserConsumeCount, CreatedByUserId: weekUser.Id}).Error)
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: weekUser.Id, UserSubscriptionId: weekSub.Id, PlanId: week.Id, PackageType: week.PackageType, ModelGroup: week.ModelGroup, RequestId: "week-before-reset", Quota: 40, CreatedAt: now - 3600}))

	weekDetails, err := GetValuePackageWindowUsageDetails(weekUser.Id, weekSub.Id, now)
	require.NoError(t, err)
	require.EqualValues(t, 0, weekDetails.Used5h)
	require.EqualValues(t, 40, weekDetails.Used7d)

	monthUser := createValuePackageUser(t, 3803, UserGroupTiyan)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	month.Limit7dAmount = 100
	require.NoError(t, DB.Save(&month).Error)
	monthStart := now - 2*24*3600
	monthSub := createActiveValuePackageSub(t, monthUser.Id, month, monthStart, monthStart+30*24*3600)
	require.NoError(t, DB.Create(&ValuePackageQuotaReset{UserId: monthUser.Id, UserSubscriptionId: monthSub.Id, PlanId: month.Id, PackageType: month.PackageType, ResetAt: now - 1800, Source: ValuePackageQuotaResetSourceUserConsumeCount, CreatedByUserId: monthUser.Id}).Error)
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: monthUser.Id, UserSubscriptionId: monthSub.Id, PlanId: month.Id, PackageType: month.PackageType, ModelGroup: month.ModelGroup, RequestId: "month-before-reset", Quota: 40, CreatedAt: now - 3600}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: monthUser.Id, UserSubscriptionId: monthSub.Id, PlanId: month.Id, PackageType: month.PackageType, ModelGroup: month.ModelGroup, RequestId: "month-after-reset", Quota: 10, CreatedAt: now - 900}))

	monthDetails, err := GetValuePackageWindowUsageDetails(monthUser.Id, monthSub.Id, now)
	require.NoError(t, err)
	require.EqualValues(t, 10, monthDetails.Used5h)
	require.EqualValues(t, 10, monthDetails.Used7d)
	require.EqualValues(t, now-900, monthDetails.Earliest7dCreatedAt)
	require.EqualValues(t, monthStart+7*24*3600, monthDetails.ResetAt7d)
}

func TestValuePackageWindowUsageDayCardIgnores7dLimit(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3804, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	day.Limit7dAmount = 100
	require.NoError(t, DB.Save(&day).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-3600, now+23*3600)
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "day-usage", Quota: 40, CreatedAt: now - 900}))

	details, err := GetValuePackageWindowUsageDetails(user.Id, sub.Id, now)
	usage := buildValuePackageUsageSummaryFromDetails(&sub, &day, details, now)

	require.NoError(t, err)
	require.NotNil(t, usage)
	require.EqualValues(t, 0, usage.Used7d)
	require.EqualValues(t, 0, usage.Limit7d)
	require.False(t, usage.Limited7d)
	require.EqualValues(t, 0, usage.ResetAt7d)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
cd /Users/ethan/Documents/yunbay
go test ./model -run 'TestValuePackageWindowUsageAnchors7dToSubscriptionStart|TestValuePackageWindowUsageResetScopeByPackageType|TestValuePackageWindowUsageDayCardIgnores7dLimit' -count=1
```

Expected:

```text
FAIL
```

Expected failure reasons include old rolling 7-day usage counting previous anchored windows, week reset clearing 7-day usage, or day cards exposing `Limit7d`.

- [ ] **Step 3: Replace record-to-details calculation**

In `/Users/ethan/Documents/yunbay/model/subscription.go`, replace the existing `valuePackageUsageDetailsFromRecords(records []ValuePackageUsageRecord, now int64)` function with this implementation:

```go
func buildValuePackageWindowUsageDetailsFromRecords(sub *UserSubscription, plan *SubscriptionPlan, records []ValuePackageUsageRecord, lastResetAt int64, now int64) *ValuePackageWindowUsageDetails {
	details := &ValuePackageWindowUsageDetails{}
	if sub == nil || plan == nil || now <= 0 {
		return details
	}

	positiveRecords := make([]ValuePackageUsageRecord, 0, len(records))
	for _, record := range records {
		if record.Quota <= 0 || record.CreatedAt <= 0 || record.CreatedAt > now {
			continue
		}
		positiveRecords = append(positiveRecords, record)
	}

	recordsAfterReset := make([]ValuePackageUsageRecord, 0, len(positiveRecords))
	for _, record := range positiveRecords {
		if lastResetAt > 0 && record.CreatedAt < lastResetAt {
			continue
		}
		recordsAfterReset = append(recordsAfterReset, record)
	}

	details.Used5h, details.Earliest5hCreatedAt = valuePackageFixedWindowUsageDetails(recordsAfterReset, valuePackage5hWindowSeconds, now)
	if details.Used5h > 0 && details.Earliest5hCreatedAt > 0 {
		details.ResetAt5h = details.Earliest5hCreatedAt + valuePackage5hWindowSeconds
		details.ResetSeconds5h = details.ResetAt5h - now
		if details.ResetSeconds5h < 0 {
			details.ResetSeconds5h = 0
		}
	}

	if !valuePackageHas7dWindow(plan) {
		return details
	}

	window := calcValuePackageAnchoredWindow(sub.StartTime, sub.EndTime, valuePackage7dWindowSeconds, now)
	if window.Start <= 0 || window.End <= window.Start {
		return details
	}

	effectiveStart := window.Start
	if valuePackageResetClears7d(plan) && lastResetAt > effectiveStart && lastResetAt <= now {
		effectiveStart = lastResetAt
	}

	for _, record := range positiveRecords {
		if record.CreatedAt < effectiveStart || record.CreatedAt >= window.End {
			continue
		}
		details.Used7d += record.Quota
		if details.Earliest7dCreatedAt == 0 || record.CreatedAt < details.Earliest7dCreatedAt {
			details.Earliest7dCreatedAt = record.CreatedAt
		}
	}
	details.ResetAt7d = window.End
	details.ResetSeconds7d = details.ResetAt7d - now
	if details.ResetSeconds7d < 0 {
		details.ResetSeconds7d = 0
	}
	return details
}
```

Update `buildValuePackageUsageSummaryFromDetails` so day cards and disabled 7-day limits do not expose 7-day values:

```go
	limit7d := int64(0)
	if valuePackageHas7dWindow(plan) {
		limit7d = plan.Limit7dAmount
	}
	limited7d := limit7d > 0 && usageDetails.Used7d >= limit7d
```

and in the summary literal use:

```go
		Limit7d:        limit7d,
		Percent7d:      valuePackagePercent(usageDetails.Used7d, limit7d),
```

and change the reset display guard to:

```go
	if limit7d > 0 {
		summary.ResetAt7d = usageDetails.ResetAt7d
		summary.ResetSeconds7d = usageDetails.ResetSeconds7d
	}
```

- [ ] **Step 4: Update database usage-details query to load sub/plan and use the helper**

Replace `getValuePackageWindowUsageDetailsTx` in `/Users/ethan/Documents/yunbay/model/subscription.go` with:

```go
// ResetAt5h is based on the earliest current 5-hour positive usage after the last reset.
// ResetAt7d is based on the current subscription-start anchored 7-day period end.
func getValuePackageWindowUsageDetailsTx(tx *gorm.DB, userId int, userSubscriptionId int, now int64) (*ValuePackageWindowUsageDetails, error) {
	if tx == nil {
		tx = DB
	}
	if now <= 0 {
		now = getDBTimestampTx(tx)
	}
	var sub UserSubscription
	if err := tx.Where("id = ? AND user_id = ?", userSubscriptionId, userId).First(&sub).Error; err != nil {
		return nil, err
	}
	plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
	if err != nil {
		return nil, err
	}
	normalizeValuePackagePlan(plan)
	lastResetAt, err := getLastValuePackageQuotaResetAtTx(tx, userId, userSubscriptionId, now)
	if err != nil {
		return nil, err
	}

	lowerBound := sub.StartTime
	if lowerBound <= 0 {
		lowerBound = now - valuePackage7dWindowSeconds
	}
	var usageRecords []ValuePackageUsageRecord
	if err := tx.Where("user_id = ? AND user_subscription_id = ? AND created_at >= ? AND created_at <= ? AND quota > ?", userId, userSubscriptionId, lowerBound, now, 0).
		Order("created_at asc, id asc").
		Find(&usageRecords).Error; err != nil {
		return nil, err
	}
	return buildValuePackageWindowUsageDetailsFromRecords(&sub, plan, usageRecords, lastResetAt, now), nil
}
```

- [ ] **Step 5: Update management-row summary calculation**

In `listValuePackageManagementRowsTx`, replace the usage-record query and grouping logic with a subscription-start lower bound and no reset filtering:

```go
	minStart := now - valuePackage7dWindowSeconds
	for _, sub := range pageSubs {
		if sub.StartTime > 0 && (minStart == 0 || sub.StartTime < minStart) {
			minStart = sub.StartTime
		}
	}

	var usageRecords []ValuePackageUsageRecord
	if err := tx.Where("user_subscription_id IN ? AND created_at >= ? AND created_at <= ? AND quota > ?", subIDs, minStart, now, 0).
		Order("user_subscription_id asc, created_at asc, id asc").
		Find(&usageRecords).Error; err != nil {
		return nil, err
	}
	usageRecordsBySubID := make(map[int][]ValuePackageUsageRecord, len(pageSubs))
	for _, record := range usageRecords {
		usageRecordsBySubID[record.UserSubscriptionId] = append(usageRecordsBySubID[record.UserSubscriptionId], record)
	}
```

Then replace the per-row usage construction with:

```go
		details := buildValuePackageWindowUsageDetailsFromRecords(&sub, &plan, usageRecordsBySubID[sub.Id], lastResetBySubID[sub.Id], now)
		usage := buildValuePackageUsageSummaryFromDetails(&sub, &plan, details, now)
```

- [ ] **Step 6: Run targeted tests**

Run:

```bash
cd /Users/ethan/Documents/yunbay
go test ./model -run 'TestValuePackageWindowUsageAnchors7dToSubscriptionStart|TestValuePackageWindowUsageResetScopeByPackageType|TestValuePackageWindowUsageDayCardIgnores7dLimit|TestValuePackageWindowUsage' -count=1
```

Expected:

```text
ok  	github.com/QuantumNous/new-api/model
```

If existing day-card tests still assert nonzero `Used7d` or `Limit7d`, update those tests to the new product rule: day cards ignore 7-day usage and expose `Limit7d = 0`.

- [ ] **Step 7: Commit Task 2**

```bash
cd /Users/ethan/Documents/yunbay
git add model/subscription.go model/value_package_test.go
git commit -m "fix: anchor value package 7d windows"
```

---

### Task 3: Backend pre-consume and reserve enforcement use anchored windows

**Files:**
- Modify: `/Users/ethan/Documents/yunbay/model/value_package_test.go`
- Modify: `/Users/ethan/Documents/yunbay/model/subscription.go`

- [ ] **Step 1: Write failing pre-consume tests**

Append these tests to `/Users/ethan/Documents/yunbay/model/value_package_test.go` near the existing `PreConsumeValuePackageSubscription` tests:

```go
func TestPreConsumeValuePackageSubscriptionIgnoresDayCard7dLimit(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3810, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	day.TotalAmount = 1000
	day.Limit5hAmount = 1000
	day.Limit7dAmount = 1
	require.NoError(t, DB.Save(&day).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-3600, now+23*3600)

	res, err := PreConsumeValuePackageSubscription("day-ignore-7d", user.Id, sub.Id, 10)

	require.NoError(t, err)
	require.EqualValues(t, 10, res.AmountUsedAfter)
}

func TestPreConsumeValuePackageSubscriptionWeekResetDoesNotClear7dLimit(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3811, UserGroupTiyan)
	week := createValuePackagePlan(t, ValuePackageTypeWeek, ValuePackageLevelWeek, 7, 19.9)
	week.TotalAmount = 1000
	week.Limit5hAmount = 1000
	week.Limit7dAmount = 50
	require.NoError(t, DB.Save(&week).Error)
	now := common.GetTimestamp()
	start := now - 2*24*3600
	sub := createActiveValuePackageSub(t, user.Id, week, start, start+7*24*3600)
	require.NoError(t, DB.Create(&ValuePackageQuotaReset{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: week.Id, PackageType: week.PackageType, ResetAt: now - 1800, Source: ValuePackageQuotaResetSourceUserConsumeCount, CreatedByUserId: user.Id}).Error)
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: week.Id, PackageType: week.PackageType, ModelGroup: week.ModelGroup, RequestId: "week-before-reset-preconsume", Quota: 45, CreatedAt: now - 3600}))
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("amount_used", int64(45)).Error)

	_, err := PreConsumeValuePackageSubscription("week-after-reset-preconsume", user.Id, sub.Id, 10)

	require.Error(t, err)
	require.Contains(t, err.Error(), "7d period limit exceeded")
	require.NotContains(t, err.Error(), "rolling")
}

func TestPreConsumeValuePackageSubscriptionMonthResetClearsCurrent7dLimit(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3812, UserGroupTiyan)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	month.TotalAmount = 1000
	month.Limit5hAmount = 1000
	month.Limit7dAmount = 50
	require.NoError(t, DB.Save(&month).Error)
	now := common.GetTimestamp()
	start := now - 2*24*3600
	sub := createActiveValuePackageSub(t, user.Id, month, start, start+30*24*3600)
	require.NoError(t, DB.Create(&ValuePackageQuotaReset{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ResetAt: now - 1800, Source: ValuePackageQuotaResetSourceUserConsumeCount, CreatedByUserId: user.Id}).Error)
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ModelGroup: month.ModelGroup, RequestId: "month-before-reset-preconsume", Quota: 45, CreatedAt: now - 3600}))
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("amount_used", int64(45)).Error)

	res, err := PreConsumeValuePackageSubscription("month-after-reset-preconsume", user.Id, sub.Id, 10)

	require.NoError(t, err)
	require.EqualValues(t, 55, res.AmountUsedAfter)
}

func TestPreConsumeValuePackageSubscriptionDoesNotUseRolling7dWindow(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3813, UserGroupTiyan)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	month.TotalAmount = 1000
	month.Limit5hAmount = 1000
	month.Limit7dAmount = 50
	require.NoError(t, DB.Save(&month).Error)
	now := common.GetTimestamp()
	start := now - 8*24*3600
	sub := createActiveValuePackageSub(t, user.Id, month, start, start+30*24*3600)
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ModelGroup: month.ModelGroup, RequestId: "previous-period-preconsume", Quota: 45, CreatedAt: start + 2*24*3600}))
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("amount_used", int64(45)).Error)

	res, err := PreConsumeValuePackageSubscription("current-period-preconsume", user.Id, sub.Id, 10)

	require.NoError(t, err)
	require.EqualValues(t, 55, res.AmountUsedAfter)
}
```

- [ ] **Step 2: Run pre-consume tests to verify they fail**

Run:

```bash
cd /Users/ethan/Documents/yunbay
go test ./model -run 'TestPreConsumeValuePackageSubscription(IgnoresDayCard7dLimit|WeekResetDoesNotClear7dLimit|MonthResetClearsCurrent7dLimit|DoesNotUseRolling7dWindow)' -count=1
```

Expected:

```text
FAIL
```

Expected failure reasons include day cards rejecting on 7-day limit, week reset clearing 7-day usage, or errors containing `7d rolling limit exceeded`.

- [ ] **Step 3: Write failing reserve replacement test**

Append this test near existing `ReserveValuePackageUsageToTarget` tests:

```go
func TestReserveValuePackageUsageToTargetUsesAnchored7dWindowForReplacement(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3814, UserGroupVIP)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	month.TotalAmount = 1000
	month.Limit5hAmount = 1000
	month.Limit7dAmount = 50
	require.NoError(t, DB.Save(&month).Error)
	now := common.GetTimestamp()
	start := now - 8*24*3600
	sub := createActiveValuePackageSub(t, user.Id, month, start, start+30*24*3600)
	previousPeriodCreatedAt := now - 7*24*3600
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ModelGroup: month.ModelGroup, RequestId: "reserve-previous-period", Quota: 10, CreatedAt: previousPeriodCreatedAt}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ModelGroup: month.ModelGroup, RequestId: "reserve-current-period", Quota: 45, CreatedAt: now - 3600}))
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("amount_used", int64(55)).Error)

	res, err := ReserveValuePackageUsageToTarget("reserve-previous-period", user.Id, sub.Id, 20)

	require.NoError(t, err)
	require.EqualValues(t, 55, res.AmountUsedBefore)
	require.EqualValues(t, 65, res.AmountUsedAfter)
	used5h, used7d, err := GetValuePackageWindowUsage(user.Id, sub.Id, now)
	require.NoError(t, err)
	require.EqualValues(t, 45, used5h)
	require.EqualValues(t, 45, used7d)
}
```

- [ ] **Step 4: Run reserve test to verify it fails**

Run:

```bash
cd /Users/ethan/Documents/yunbay
go test ./model -run 'TestReserveValuePackageUsageToTargetUsesAnchored7dWindowForReplacement' -count=1
```

Expected:

```text
FAIL
```

Expected old behavior: replacement is counted in rolling 7-day usage and can exceed `Limit7dAmount`.

- [ ] **Step 5: Update pre-consume enforcement**

In `PreConsumeValuePackageSubscription` in `/Users/ethan/Documents/yunbay/model/subscription.go`, replace:

```go
		used5h, used7d, err := getValuePackageWindowUsageTx(tx, userId, sub.Id, now)
```

with:

```go
		usageDetails, err := getValuePackageWindowUsageDetailsTx(tx, userId, sub.Id, now)
```

Then replace limit checks with:

```go
		if plan.Limit5hAmount > 0 && usageDetails.Used5h+amount > plan.Limit5hAmount {
			return fmt.Errorf("subscription quota insufficient: %s, 5h limit exceeded, need=%d", ValuePackageQuotaExhaustedUserMessage, amount)
		}
		if valuePackageHas7dWindow(plan) && usageDetails.Used7d+amount > plan.Limit7dAmount {
			return fmt.Errorf("subscription quota insufficient: %s, 7d period limit exceeded, need=%d", ValuePackageQuotaExhaustedUserMessage, amount)
		}
```

- [ ] **Step 6: Update reserve replacement enforcement**

In `ReserveValuePackageUsageToTarget`, replace the old `start7d := maxInt64(now-7*24*3600, lastResetAt)` query block with a subscription-start query:

```go
			lowerBound := sub.StartTime
			if lowerBound <= 0 {
				lowerBound = now - valuePackage7dWindowSeconds
			}
			var usageRecords []ValuePackageUsageRecord
			if err := tx.Where("user_id = ? AND user_subscription_id = ? AND created_at >= ? AND created_at <= ? AND (quota > ? OR request_id = ?)", userId, sub.Id, lowerBound, now, 0, requestId).
				Order("created_at asc, id asc").
				Find(&usageRecords).Error; err != nil {
				return err
			}
```

Keep the existing replacement construction for `nextUsageRecords`, including the earlier fixed-5h behavior. Replace:

```go
			next5h, _ := valuePackageFixedWindowUsageDetails(nextUsageRecords, 5*3600, now)
			next7d, _ := valuePackageRollingUsageDetails(nextUsageRecords)
```

with:

```go
			nextDetails := buildValuePackageWindowUsageDetailsFromRecords(&sub, plan, nextUsageRecords, lastResetAt, now)
```

Then replace limit checks with:

```go
			if plan.Limit5hAmount > 0 && nextDetails.Used5h > plan.Limit5hAmount {
				return fmt.Errorf("subscription quota insufficient: %s, 5h limit exceeded, need=%d", ValuePackageQuotaExhaustedUserMessage, targetQuota)
			}
			if valuePackageHas7dWindow(plan) && nextDetails.Used7d > plan.Limit7dAmount {
				return fmt.Errorf("subscription quota insufficient: %s, 7d period limit exceeded, need=%d", ValuePackageQuotaExhaustedUserMessage, targetQuota)
			}
```

- [ ] **Step 7: Run targeted tests**

Run:

```bash
cd /Users/ethan/Documents/yunbay
go test ./model -run 'TestPreConsumeValuePackageSubscription(IgnoresDayCard7dLimit|WeekResetDoesNotClear7dLimit|MonthResetClearsCurrent7dLimit|DoesNotUseRolling7dWindow)|TestReserveValuePackageUsageToTargetUsesAnchored7dWindowForReplacement|TestReserveValuePackageUsageToTarget' -count=1
```

Expected:

```text
ok  	github.com/QuantumNous/new-api/model
```

- [ ] **Step 8: Commit Task 3**

```bash
cd /Users/ethan/Documents/yunbay
git add model/subscription.go model/value_package_test.go
git commit -m "fix: enforce value package anchored limits"
```

---

### Task 4: Backend management summaries and order duration regression coverage

**Files:**
- Modify: `/Users/ethan/Documents/yunbay/model/value_package_test.go`
- Modify: `/Users/ethan/Documents/yunbay/model/subscription.go`

- [ ] **Step 1: Write failing management-row test**

Append this test near existing `ListValuePackageManagementRows` tests:

```go
func TestListValuePackageManagementRowsUsesAnchored7dAndResetScope(t *testing.T) {
	setupValuePackageTestDB(t)
	now := common.GetTimestamp()
	user := createValuePackageUser(t, 3820, UserGroupTiyan)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	month.TotalAmount = 1000
	month.Limit5hAmount = 100
	month.Limit7dAmount = 100
	require.NoError(t, DB.Save(&month).Error)
	start := now - 8*24*3600
	sub := createActiveValuePackageSub(t, user.Id, month, start, start+30*24*3600)
	require.NoError(t, DB.Create(&UserValuePackagePreference{UserId: user.Id, Enabled: true, ActiveUserSubscriptionId: sub.Id}).Error)
	require.NoError(t, DB.Create(&ValuePackageQuotaReset{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ResetAt: now - 1800, Source: ValuePackageQuotaResetSourceUserConsumeCount, CreatedByUserId: user.Id}).Error)
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ModelGroup: month.ModelGroup, RequestId: "management-previous-period", Quota: 60, CreatedAt: start + 2*24*3600}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ModelGroup: month.ModelGroup, RequestId: "management-before-reset", Quota: 30, CreatedAt: now - 3600}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: month.Id, PackageType: month.PackageType, ModelGroup: month.ModelGroup, RequestId: "management-after-reset", Quota: 10, CreatedAt: now - 900}))
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("amount_used", int64(100)).Error)

	result, err := ListValuePackageManagementRows(ValuePackageManagementFilter{Page: 1, PageSize: 20, Active: "active"}, now)

	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	require.EqualValues(t, 10, result.Items[0].Usage.Used5h)
	require.EqualValues(t, 10, result.Items[0].Usage.Used7d)
	require.EqualValues(t, 100, result.Items[0].Usage.TotalUsed)
}
```

- [ ] **Step 2: Run management-row test to verify it fails**

Run:

```bash
cd /Users/ethan/Documents/yunbay
go test ./model -run 'TestListValuePackageManagementRowsUsesAnchored7dAndResetScope' -count=1
```

Expected:

```text
FAIL
```

Expected old behavior: management rows pre-filter with last reset for all packages and use rolling 7-day usage details.

- [ ] **Step 3: Write failing month order duration test**

Append this test near value-package order completion tests:

```go
func TestCompleteValuePackageOrderCreatesMonthCardForThirtyDays(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3821, UserGroupTiyan)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	month.DurationUnit = SubscriptionDurationMonth
	month.DurationValue = 1
	require.NoError(t, DB.Save(&month).Error)
	now := common.GetTimestamp()
	order := SubscriptionOrder{UserId: user.Id, PlanId: month.Id, Money: month.PriceAmount, TradeNo: "month-30-days-order", PaymentMethod: PaymentMethodLDXP, PaymentProvider: PaymentProviderLDXP, Status: common.TopUpStatusPending, CreateTime: now}
	require.NoError(t, DB.Create(&order).Error)

	completed, err := CompleteValuePackageOrder(order.TradeNo, "payload", PaymentProviderLDXP, PaymentMethodLDXP, false)

	require.NoError(t, err)
	require.NotNil(t, completed)
	require.EqualValues(t, 30*24*3600, completed.EndTime-completed.StartTime)
}
```

- [ ] **Step 4: Run month order duration test to verify it fails if backend normalization is incomplete**

Run:

```bash
cd /Users/ethan/Documents/yunbay
go test ./model -run 'TestCompleteValuePackageOrderCreatesMonthCardForThirtyDays' -count=1
```

Expected before Task 1 backend normalization is fully applied in order flow:

```text
FAIL
```

If this test already passes because Task 1 normalization reaches `CompleteValuePackageOrder`, keep the test as regression coverage.

- [ ] **Step 5: Finish management-row implementation if Task 2 did not already complete it**

Confirm `listValuePackageManagementRowsTx` uses `buildValuePackageWindowUsageDetailsFromRecords(&sub, &plan, records, lastResetAt, now)` for each row. The final row block must include:

```go
		details := buildValuePackageWindowUsageDetailsFromRecords(&sub, &plan, usageRecordsBySubID[sub.Id], lastResetBySubID[sub.Id], now)
		usage := buildValuePackageUsageSummaryFromDetails(&sub, &plan, details, now)
		items = append(items, ValuePackageManagementRow{
			UserId:             user.Id,
			Username:           user.Username,
			DisplayName:        user.DisplayName,
			PackageType:        plan.PackageType,
			PlanTitle:          plan.Title,
			SubscriptionId:     sub.Id,
			SubscriptionStatus: sub.Status,
			StartTime:          sub.StartTime,
			EndTime:            sub.EndTime,
			Enabled:            pref.Enabled && pref.ActiveUserSubscriptionId == sub.Id,
			ResetCount:         pref.ResetCount,
			Usage:              usage,
			LastResetAt:        lastResetBySubID[sub.Id],
		})
```

- [ ] **Step 6: Run backend value-package test slice**

Run:

```bash
cd /Users/ethan/Documents/yunbay
go test ./model -run 'ValuePackage|Subscription' -count=1
```

Expected:

```text
ok  	github.com/QuantumNous/new-api/model
```

- [ ] **Step 7: Commit Task 4**

```bash
cd /Users/ethan/Documents/yunbay
git add model/subscription.go model/value_package_test.go
git commit -m "fix: summarize value package anchored usage"
```

---

### Task 5: Frontend plan-form duration and payload normalization

**Files:**
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/features/subscriptions/constants.ts`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/features/subscriptions/lib/plan-form.ts`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/features/subscriptions/lib/plan-form-value-package.test.ts`
- Create: `/Users/ethan/Documents/yunbay/web/default/src/features/subscriptions/lib/value-package-limit-labels.ts`
- Create: `/Users/ethan/Documents/yunbay/web/default/src/features/subscriptions/lib/value-package-limit-labels.test.ts`

- [ ] **Step 1: Write failing label-helper tests**

Create `/Users/ethan/Documents/yunbay/web/default/src/features/subscriptions/lib/value-package-limit-labels.test.ts`:

```ts
/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import test from 'node:test'
import {
  getValuePackageTotalLimitDescriptionKey,
  getValuePackageTotalLimitLabelKey,
  shouldExposeValuePackage7dPeriodLimit,
} from './value-package-limit-labels'

test('value package total limit labels follow package period', () => {
  assert.equal(getValuePackageTotalLimitLabelKey('day'), '1-day total limit')
  assert.equal(getValuePackageTotalLimitLabelKey('week'), '7-day total limit')
  assert.equal(getValuePackageTotalLimitLabelKey('month'), '30-day total limit')
  assert.equal(
    getValuePackageTotalLimitDescriptionKey('week'),
    'Week cards can use this total quota from activation time until the 7-day expiration. 0 means unlimited total quota.'
  )
})

test('only month cards expose optional 7-day period limit in admin UI', () => {
  assert.equal(shouldExposeValuePackage7dPeriodLimit('day'), false)
  assert.equal(shouldExposeValuePackage7dPeriodLimit('week'), false)
  assert.equal(shouldExposeValuePackage7dPeriodLimit('month'), true)
})
```

- [ ] **Step 2: Run label-helper tests to verify they fail**

Run:

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun test src/features/subscriptions/lib/value-package-limit-labels.test.ts
```

Expected:

```text
FAIL
Cannot find module './value-package-limit-labels'
```

- [ ] **Step 3: Create label helper module**

Create `/Users/ethan/Documents/yunbay/web/default/src/features/subscriptions/lib/value-package-limit-labels.ts`:

```ts
/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

type ValuePackageTypeLike = 'day' | 'week' | 'month' | string | undefined

export function getValuePackageTotalLimitLabelKey(
  packageType: ValuePackageTypeLike
): string {
  switch (packageType) {
    case 'day':
      return '1-day total limit'
    case 'week':
      return '7-day total limit'
    case 'month':
      return '30-day total limit'
    default:
      return 'Package total limit'
  }
}

export function getValuePackageTotalLimitDescriptionKey(
  packageType: ValuePackageTypeLike
): string {
  switch (packageType) {
    case 'day':
      return 'Day cards can use this total quota during the 1-day validity period. 0 means unlimited total quota.'
    case 'week':
      return 'Week cards can use this total quota from activation time until the 7-day expiration. 0 means unlimited total quota.'
    case 'month':
      return 'Month cards can use this total quota from activation time until the 30-day expiration. 0 means unlimited total quota.'
    default:
      return '0 means unlimited. The value is converted to quota units when saved.'
  }
}

export function shouldExposeValuePackage7dPeriodLimit(
  packageType: ValuePackageTypeLike
): boolean {
  return packageType === 'month'
}

export const VALUE_PACKAGE_7D_PERIOD_LIMIT_LABEL_KEY = '7-day period limit'

export const VALUE_PACKAGE_7D_PERIOD_LIMIT_DESCRIPTION_KEY =
  'Optional month-card period quota. It resets from activation time every fixed 7 days, and month-card reset can clear current 7-day period usage. 0 disables this period limit.'

export const VALUE_PACKAGE_RESET_CONFIRM_MESSAGE_KEY =
  "This will consume 1 reset count. Day and week cards clear only the 5-hour usage window. Month cards clear both the 5-hour usage window and the current 7-day period usage. This will not restore total quota or extend expiration."
```

- [ ] **Step 4: Add failing plan-form tests for duration and payload**

Modify `/Users/ethan/Documents/yunbay/web/default/src/features/subscriptions/lib/plan-form-value-package.test.ts` so the top import section includes:

```ts
import { getValuePackageDuration } from '../constants'
```

Then append these tests after the existing tests:

```ts
test('value package month duration submits as fixed 30 days', () => {
  assert.deepEqual(getValuePackageDuration('week'), {
    duration_unit: 'day',
    duration_value: 7,
    custom_seconds: 0,
  })
  assert.deepEqual(getValuePackageDuration('month'), {
    duration_unit: 'day',
    duration_value: 30,
    custom_seconds: 0,
  })
})

test('value package total_amount carries week and month total limits', () => {
  const weekPayload = formValuesToPlanPayload({
    ...PLAN_FORM_DEFAULTS,
    title: '周卡',
    plan_kind: 'value_package' as const,
    package_type: 'week' as const,
    total_amount: 700,
    limit_7d_amount: 99,
    model_group: 'week-card',
  })
  assert.equal(weekPayload.plan.duration_unit, 'day')
  assert.equal(weekPayload.plan.duration_value, 7)
  assert.equal(typeof weekPayload.plan.total_amount, 'number')
  assert.equal(weekPayload.plan.limit_7d_amount, 0)

  const monthPayload = formValuesToPlanPayload({
    ...PLAN_FORM_DEFAULTS,
    title: '月卡',
    plan_kind: 'value_package' as const,
    package_type: 'month' as const,
    total_amount: 3000,
    limit_7d_amount: 700,
    model_group: 'month-card',
  })
  assert.equal(monthPayload.plan.duration_unit, 'day')
  assert.equal(monthPayload.plan.duration_value, 30)
  assert.equal(typeof monthPayload.plan.total_amount, 'number')
  assert.equal(typeof monthPayload.plan.limit_7d_amount, 'number')
  assert.ok(Number(monthPayload.plan.limit_7d_amount) > 0)
})

test('day package payload clears 7-day period limit', () => {
  const payload = formValuesToPlanPayload({
    ...PLAN_FORM_DEFAULTS,
    title: '日卡',
    plan_kind: 'value_package' as const,
    package_type: 'day' as const,
    limit_7d_amount: 500,
    model_group: 'day-card',
  })

  assert.equal(payload.plan.limit_7d_amount, 0)
})
```

If `plan-form-value-package.test.ts` already imports from `../constants`, merge `getValuePackageDuration` into that existing import instead of creating a duplicate import.

- [ ] **Step 5: Run frontend tests to verify failures**

Run:

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun test src/features/subscriptions/lib/value-package-limit-labels.test.ts src/features/subscriptions/lib/plan-form-value-package.test.ts
```

Expected:

```text
FAIL
```

Expected old behavior: month duration returns `month/1`, and week/day payload keeps `limit_7d_amount`.

- [ ] **Step 6: Update constants and payload conversion**

In `/Users/ethan/Documents/yunbay/web/default/src/features/subscriptions/constants.ts`, change the month card entry to:

```ts
  {
    value: 'month',
    labelKey: 'Month Card',
    level: 3,
    durationUnit: 'day',
    durationValue: 30,
  },
```

In `/Users/ethan/Documents/yunbay/web/default/src/features/subscriptions/lib/plan-form.ts`, import the visibility helper:

```ts
import { shouldExposeValuePackage7dPeriodLimit } from './value-package-limit-labels'
```

Then change the `limit_7d_amount` payload assignment to:

```ts
      limit_7d_amount: shouldExposeValuePackage7dPeriodLimit(
        values.package_type
      )
        ? parseQuotaFromDollars(Number(values.limit_7d_amount || 0))
        : 0,
```

- [ ] **Step 7: Run frontend unit tests**

Run:

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun test src/features/subscriptions/lib/value-package-limit-labels.test.ts src/features/subscriptions/lib/plan-form-value-package.test.ts
```

Expected:

```text
pass
```

- [ ] **Step 8: Commit Task 5**

```bash
cd /Users/ethan/Documents/yunbay
git add web/default/src/features/subscriptions/constants.ts \
  web/default/src/features/subscriptions/lib/plan-form.ts \
  web/default/src/features/subscriptions/lib/plan-form-value-package.test.ts \
  web/default/src/features/subscriptions/lib/value-package-limit-labels.ts \
  web/default/src/features/subscriptions/lib/value-package-limit-labels.test.ts
git commit -m "fix: normalize value package form limits"
```

---

### Task 6: Frontend admin and user package UI semantics

**Files:**
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/features/subscriptions/components/subscriptions-mutate-drawer.tsx`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/features/subscriptions/components/subscriptions-mutate-drawer-value-package-source.test.ts`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/features/subscriptions/components/value-package-admin-cards.tsx`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/features/value-packages/components/value-package-card.tsx`

- [ ] **Step 1: Write failing source guard for mutate drawer**

Modify `/Users/ethan/Documents/yunbay/web/default/src/features/subscriptions/components/subscriptions-mutate-drawer-value-package-source.test.ts` and add this test:

```ts
test('mutate drawer source uses dynamic value package total and month-only 7d period labels', async () => {
  const source = await readFile(sourcePath, 'utf8')

  assert.match(source, /getValuePackageTotalLimitLabelKey/)
  assert.match(source, /getValuePackageTotalLimitDescriptionKey/)
  assert.match(source, /shouldExposeValuePackage7dPeriodLimit/)
  assert.match(source, /VALUE_PACKAGE_7D_PERIOD_LIMIT_LABEL_KEY/)
  assert.doesNotMatch(source, /<FormLabel>limit_7d_amount<\/FormLabel>/)
})
```

- [ ] **Step 2: Run source guard to verify it fails**

Run:

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun test src/features/subscriptions/components/subscriptions-mutate-drawer-value-package-source.test.ts
```

Expected:

```text
FAIL
```

Expected old behavior: source still contains `<FormLabel>limit_7d_amount</FormLabel>` and no dynamic label helper imports.

- [ ] **Step 3: Update mutate drawer labels and 7-day period field visibility**

In `/Users/ethan/Documents/yunbay/web/default/src/features/subscriptions/components/subscriptions-mutate-drawer.tsx`, import helpers:

```ts
import {
  getValuePackageTotalLimitDescriptionKey,
  getValuePackageTotalLimitLabelKey,
  shouldExposeValuePackage7dPeriodLimit,
  VALUE_PACKAGE_7D_PERIOD_LIMIT_DESCRIPTION_KEY,
  VALUE_PACKAGE_7D_PERIOD_LIMIT_LABEL_KEY,
} from '../lib/value-package-limit-labels'
```

After `const packageType = form.watch('package_type')`, add:

```ts
  const showValuePackage7dPeriodLimit =
    isValuePackage && shouldExposeValuePackage7dPeriodLimit(packageType)
```

In the package-type change effect that currently sets duration and package level, also clear 7-day period limit when hidden:

```ts
    if (!shouldExposeValuePackage7dPeriodLimit(packageType)) {
      form.setValue('limit_7d_amount', 0, {
        shouldDirty: true,
        shouldValidate: true,
      })
    }
```

Replace the `total_amount` form label block with dynamic labels:

```tsx
                      <FormLabel>
                        {t(
                          isValuePackage
                            ? getValuePackageTotalLimitLabelKey(packageType)
                            : 'Received amount'
                        )}
                      </FormLabel>
```

Replace its description with:

```tsx
                      <FormDescription>
                        {t(
                          isValuePackage
                            ? getValuePackageTotalLimitDescriptionKey(
                                packageType
                              )
                            : '0 means unlimited. The value is converted to quota units when saved.'
                        )}
                      </FormDescription>
```

Wrap the `limit_7d_amount` field so it only renders for month cards:

```tsx
                  {showValuePackage7dPeriodLimit ? (
                    <FormField
                      control={form.control}
                      name='limit_7d_amount'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            {t(VALUE_PACKAGE_7D_PERIOD_LIMIT_LABEL_KEY)}
                          </FormLabel>
                          <FormControl>
                            <Input
                              {...field}
                              type='number'
                              min={0}
                              onChange={(e) =>
                                field.onChange(parseFloat(e.target.value) || 0)
                              }
                            />
                          </FormControl>
                          <FormDescription>
                            {t(VALUE_PACKAGE_7D_PERIOD_LIMIT_DESCRIPTION_KEY)}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  ) : null}
```

- [ ] **Step 4: Update admin cards**

In `/Users/ethan/Documents/yunbay/web/default/src/features/subscriptions/components/value-package-admin-cards.tsx`, import:

```ts
import {
  getValuePackageTotalLimitLabelKey,
  shouldExposeValuePackage7dPeriodLimit,
  VALUE_PACKAGE_7D_PERIOD_LIMIT_LABEL_KEY,
} from '../lib/value-package-limit-labels'
```

In the card details section, add a row for total limit:

```tsx
                    <div className='flex justify-between gap-3'>
                      <span className='text-muted-foreground'>
                        {t(getValuePackageTotalLimitLabelKey(plan.package_type))}
                      </span>
                      <span className='font-medium'>
                        {quotaUnitsToDollars(Number(plan.total_amount || 0))}
                      </span>
                    </div>
```

Change `limit_5h_amount` label to:

```tsx
                        {t('5-hour limit')}
```

Replace the existing unconditional `limit_7d_amount` row with:

```tsx
                    {shouldExposeValuePackage7dPeriodLimit(
                      plan.package_type
                    ) && Number(plan.limit_7d_amount || 0) > 0 ? (
                      <div className='flex justify-between gap-3'>
                        <span className='text-muted-foreground'>
                          {t(VALUE_PACKAGE_7D_PERIOD_LIMIT_LABEL_KEY)}
                        </span>
                        <span className='font-medium'>
                          {quotaUnitsToDollars(
                            Number(plan.limit_7d_amount || 0)
                          )}
                        </span>
                      </div>
                    ) : null}
```

- [ ] **Step 5: Update user value-package card labels and reset text**

In `/Users/ethan/Documents/yunbay/web/default/src/features/value-packages/components/value-package-card.tsx`, import helpers from the subscriptions helper path:

```ts
import {
  getValuePackageTotalLimitLabelKey,
  shouldExposeValuePackage7dPeriodLimit,
  VALUE_PACKAGE_7D_PERIOD_LIMIT_LABEL_KEY,
  VALUE_PACKAGE_RESET_CONFIRM_MESSAGE_KEY,
} from '@/features/subscriptions/lib/value-package-limit-labels'
```

Add derived booleans after `const usage = ...`:

```ts
  const show7dPeriodLimit = shouldExposeValuePackage7dPeriodLimit(
    plan.package_type
  )
```

Replace reset confirmation string with:

```ts
    const confirmed = window.confirm(t(VALUE_PACKAGE_RESET_CONFIRM_MESSAGE_KEY))
```

Change the static package-card 7-day limit tile to render only for month cards with configured limit:

```tsx
          {show7dPeriodLimit && Number(plan.limit_7d_amount || 0) > 0 ? (
            <div className='rounded-lg border p-3'>
              <div className='text-muted-foreground text-xs font-medium'>
                {t(VALUE_PACKAGE_7D_PERIOD_LIMIT_LABEL_KEY)}
              </div>
              <div className='mt-1 font-semibold tabular-nums'>
                {formatLimitAmount(Number(plan.limit_7d_amount || 0), t)}
              </div>
            </div>
          ) : null}
```

Change usage progress total label:

```tsx
              label={t(getValuePackageTotalLimitLabelKey(plan.package_type))}
```

Wrap the 7-day usage progress row:

```tsx
            {show7dPeriodLimit && usage.limit_7d > 0 ? (
              <LimitProgressRow
                label={t(VALUE_PACKAGE_7D_PERIOD_LIMIT_LABEL_KEY)}
                used={usage.used_7d}
                limit={usage.limit_7d}
                percent={usage.percent_7d}
                resetSeconds={usage.reset_seconds_7d}
                limited={usage.limited_7d}
                showReset
              />
            ) : null}
```

- [ ] **Step 6: Run source and type tests**

Run:

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun test src/features/subscriptions/components/subscriptions-mutate-drawer-value-package-source.test.ts
bun run typecheck
```

Expected:

```text
pass
```

and:

```text
tsc -b exits 0
```

- [ ] **Step 7: Commit Task 6**

```bash
cd /Users/ethan/Documents/yunbay
git add web/default/src/features/subscriptions/components/subscriptions-mutate-drawer.tsx \
  web/default/src/features/subscriptions/components/subscriptions-mutate-drawer-value-package-source.test.ts \
  web/default/src/features/subscriptions/components/value-package-admin-cards.tsx \
  web/default/src/features/value-packages/components/value-package-card.tsx
git commit -m "fix: clarify value package limit UI"
```

---

### Task 7: Frontend i18n translations

**Files:**
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/en.json`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/zh.json`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/fr.json`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/ja.json`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/ru.json`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/vi.json`
- Create temporarily: `/Users/ethan/Documents/yunbay/web/default/scripts/add-value-package-limit-i18n.mjs`
- Delete after use: `/Users/ethan/Documents/yunbay/web/default/scripts/add-value-package-limit-i18n.mjs`

- [ ] **Step 1: Run i18n sync before editing**

Run:

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun run i18n:sync
```

Expected:

```text
sync completes and writes/updates src/i18n/locales/_reports/_sync-report.json
```

- [ ] **Step 2: Add translation script**

Create `/Users/ethan/Documents/yunbay/web/default/scripts/add-value-package-limit-i18n.mjs`:

```javascript
import fs from 'node:fs/promises'
import path from 'node:path'

const LOCALES_DIR = path.resolve('src/i18n/locales')

function stableStringify(obj) {
  return JSON.stringify(obj, null, 2) + '\n'
}

const newKeys = {
  en: {
    '1-day total limit': '1-day total limit',
    '7-day total limit': '7-day total limit',
    '30-day total limit': '30-day total limit',
    '7-day period limit': '7-day period limit',
    'Day cards can use this total quota during the 1-day validity period. 0 means unlimited total quota.': 'Day cards can use this total quota during the 1-day validity period. 0 means unlimited total quota.',
    'Week cards can use this total quota from activation time until the 7-day expiration. 0 means unlimited total quota.': 'Week cards can use this total quota from activation time until the 7-day expiration. 0 means unlimited total quota.',
    'Month cards can use this total quota from activation time until the 30-day expiration. 0 means unlimited total quota.': 'Month cards can use this total quota from activation time until the 30-day expiration. 0 means unlimited total quota.',
    'Optional month-card period quota. It resets from activation time every fixed 7 days, and month-card reset can clear current 7-day period usage. 0 disables this period limit.': 'Optional month-card period quota. It resets from activation time every fixed 7 days, and month-card reset can clear current 7-day period usage. 0 disables this period limit.',
    "This will consume 1 reset count. Day and week cards clear only the 5-hour usage window. Month cards clear both the 5-hour usage window and the current 7-day period usage. This will not restore total quota or extend expiration.": "This will consume 1 reset count. Day and week cards clear only the 5-hour usage window. Month cards clear both the 5-hour usage window and the current 7-day period usage. This will not restore total quota or extend expiration.",
  },
  zh: {
    '1-day total limit': '1 天总限额',
    '7-day total limit': '7 天总限额',
    '30-day total limit': '30 天总限额',
    '7-day period limit': '每 7 天限额',
    'Day cards can use this total quota during the 1-day validity period. 0 means unlimited total quota.': '日卡有效期 1 天内最多可用的总额度，0 表示总额度不限。',
    'Week cards can use this total quota from activation time until the 7-day expiration. 0 means unlimited total quota.': '周卡从开通时刻起到 7 天过期前最多可用的总额度，0 表示总额度不限。',
    'Month cards can use this total quota from activation time until the 30-day expiration. 0 means unlimited total quota.': '月卡从开通时刻起到 30 天过期前最多可用的总额度，0 表示总额度不限。',
    'Optional month-card period quota. It resets from activation time every fixed 7 days, and month-card reset can clear current 7-day period usage. 0 disables this period limit.': '可选的月卡阶段限额。从开通时刻开始每固定 7 天自动重置，月卡手动重置也可以清空当前 7 天阶段用量；0 表示不启用该阶段限额。',
    "This will consume 1 reset count. Day and week cards clear only the 5-hour usage window. Month cards clear both the 5-hour usage window and the current 7-day period usage. This will not restore total quota or extend expiration.": '本次会消耗 1 次重置次数。日卡和周卡只清空 5 小时短窗口用量；月卡会同时清空 5 小时短窗口用量和当前 7 天阶段用量。本次操作不会恢复总额度，也不会延长有效期。',
  },
  fr: {
    '1-day total limit': 'Limite totale sur 1 jour',
    '7-day total limit': 'Limite totale sur 7 jours',
    '30-day total limit': 'Limite totale sur 30 jours',
    '7-day period limit': 'Limite par période de 7 jours',
    'Day cards can use this total quota during the 1-day validity period. 0 means unlimited total quota.': 'Les cartes journalières peuvent utiliser ce quota total pendant la période de validité de 1 jour. 0 signifie aucun plafond total.',
    'Week cards can use this total quota from activation time until the 7-day expiration. 0 means unlimited total quota.': "Les cartes hebdomadaires peuvent utiliser ce quota total depuis l'activation jusqu'à l'expiration après 7 jours. 0 signifie aucun plafond total.",
    'Month cards can use this total quota from activation time until the 30-day expiration. 0 means unlimited total quota.': "Les cartes mensuelles peuvent utiliser ce quota total depuis l'activation jusqu'à l'expiration après 30 jours. 0 signifie aucun plafond total.",
    'Optional month-card period quota. It resets from activation time every fixed 7 days, and month-card reset can clear current 7-day period usage. 0 disables this period limit.': "Quota périodique facultatif pour les cartes mensuelles. Il se réinitialise tous les 7 jours fixes depuis l'activation, et la réinitialisation d'une carte mensuelle peut effacer l'usage de la période de 7 jours en cours. 0 désactive cette limite.",
    "This will consume 1 reset count. Day and week cards clear only the 5-hour usage window. Month cards clear both the 5-hour usage window and the current 7-day period usage. This will not restore total quota or extend expiration.": "Cette action consommera 1 réinitialisation. Les cartes journalières et hebdomadaires effacent seulement la fenêtre d'usage de 5 heures. Les cartes mensuelles effacent à la fois la fenêtre de 5 heures et l'usage de la période de 7 jours en cours. Cela ne restaure pas le quota total et ne prolonge pas l'expiration.",
  },
  ja: {
    '1-day total limit': '1日間の合計上限',
    '7-day total limit': '7日間の合計上限',
    '30-day total limit': '30日間の合計上限',
    '7-day period limit': '7日ごとの期間上限',
    'Day cards can use this total quota during the 1-day validity period. 0 means unlimited total quota.': '日カードは1日間の有効期間中にこの合計クォータを使用できます。0 は合計クォータ無制限を意味します。',
    'Week cards can use this total quota from activation time until the 7-day expiration. 0 means unlimited total quota.': '週カードは有効化時刻から7日後の期限までこの合計クォータを使用できます。0 は合計クォータ無制限を意味します。',
    'Month cards can use this total quota from activation time until the 30-day expiration. 0 means unlimited total quota.': '月カードは有効化時刻から30日後の期限までこの合計クォータを使用できます。0 は合計クォータ無制限を意味します。',
    'Optional month-card period quota. It resets from activation time every fixed 7 days, and month-card reset can clear current 7-day period usage. 0 disables this period limit.': '月カード用の任意の期間クォータです。有効化時刻から固定の7日ごとにリセットされ、月カードのリセットでは現在の7日間期間の使用量もクリアできます。0 はこの期間上限を無効にします。',
    "This will consume 1 reset count. Day and week cards clear only the 5-hour usage window. Month cards clear both the 5-hour usage window and the current 7-day period usage. This will not restore total quota or extend expiration.": 'リセット回数を1回消費します。日カードと週カードは5時間の使用ウィンドウのみをクリアします。月カードは5時間の使用ウィンドウと現在の7日間期間の使用量の両方をクリアします。合計クォータは復元されず、有効期限も延長されません。',
  },
  ru: {
    '1-day total limit': 'Общий лимит на 1 день',
    '7-day total limit': 'Общий лимит на 7 дней',
    '30-day total limit': 'Общий лимит на 30 дней',
    '7-day period limit': 'Лимит за 7-дневный период',
    'Day cards can use this total quota during the 1-day validity period. 0 means unlimited total quota.': 'Дневные карты могут использовать эту общую квоту в течение 1 дня действия. 0 означает отсутствие общего лимита.',
    'Week cards can use this total quota from activation time until the 7-day expiration. 0 means unlimited total quota.': 'Недельные карты могут использовать эту общую квоту с момента активации до истечения 7 дней. 0 означает отсутствие общего лимита.',
    'Month cards can use this total quota from activation time until the 30-day expiration. 0 means unlimited total quota.': 'Месячные карты могут использовать эту общую квоту с момента активации до истечения 30 дней. 0 означает отсутствие общего лимита.',
    'Optional month-card period quota. It resets from activation time every fixed 7 days, and month-card reset can clear current 7-day period usage. 0 disables this period limit.': 'Необязательная периодическая квота для месячной карты. Она сбрасывается каждые фиксированные 7 дней с момента активации, а сброс месячной карты может очистить использование текущего 7-дневного периода. 0 отключает этот периодический лимит.',
    "This will consume 1 reset count. Day and week cards clear only the 5-hour usage window. Month cards clear both the 5-hour usage window and the current 7-day period usage. This will not restore total quota or extend expiration.": 'Это спишет 1 сброс. Дневные и недельные карты очищают только 5-часовое окно использования. Месячные карты очищают и 5-часовое окно, и использование текущего 7-дневного периода. Это не восстановит общую квоту и не продлит срок действия.',
  },
  vi: {
    '1-day total limit': 'Giới hạn tổng 1 ngày',
    '7-day total limit': 'Giới hạn tổng 7 ngày',
    '30-day total limit': 'Giới hạn tổng 30 ngày',
    '7-day period limit': 'Giới hạn theo chu kỳ 7 ngày',
    'Day cards can use this total quota during the 1-day validity period. 0 means unlimited total quota.': 'Thẻ ngày có thể dùng tổng hạn mức này trong thời hạn 1 ngày. 0 nghĩa là không giới hạn tổng hạn mức.',
    'Week cards can use this total quota from activation time until the 7-day expiration. 0 means unlimited total quota.': 'Thẻ tuần có thể dùng tổng hạn mức này từ thời điểm kích hoạt đến khi hết hạn sau 7 ngày. 0 nghĩa là không giới hạn tổng hạn mức.',
    'Month cards can use this total quota from activation time until the 30-day expiration. 0 means unlimited total quota.': 'Thẻ tháng có thể dùng tổng hạn mức này từ thời điểm kích hoạt đến khi hết hạn sau 30 ngày. 0 nghĩa là không giới hạn tổng hạn mức.',
    'Optional month-card period quota. It resets from activation time every fixed 7 days, and month-card reset can clear current 7-day period usage. 0 disables this period limit.': 'Hạn mức chu kỳ tùy chọn cho thẻ tháng. Hạn mức này tự đặt lại theo mỗi chu kỳ cố định 7 ngày tính từ thời điểm kích hoạt, và thao tác đặt lại thẻ tháng có thể xóa mức dùng của chu kỳ 7 ngày hiện tại. 0 sẽ tắt giới hạn chu kỳ này.',
    "This will consume 1 reset count. Day and week cards clear only the 5-hour usage window. Month cards clear both the 5-hour usage window and the current 7-day period usage. This will not restore total quota or extend expiration.": 'Thao tác này sẽ tiêu hao 1 lượt đặt lại. Thẻ ngày và thẻ tuần chỉ xóa cửa sổ sử dụng 5 giờ. Thẻ tháng xóa cả cửa sổ 5 giờ và mức dùng của chu kỳ 7 ngày hiện tại. Thao tác này không khôi phục tổng hạn mức và không gia hạn thời hạn.',
  },
}

async function main() {
  let totalApplied = 0

  for (const [locale, translations] of Object.entries(newKeys)) {
    const filePath = path.join(LOCALES_DIR, `${locale}.json`)
    const json = JSON.parse(await fs.readFile(filePath, 'utf8'))
    let applied = 0

    for (const [key, value] of Object.entries(translations)) {
      if (json.translation[key] !== value) {
        json.translation[key] = value
        applied++
      }
    }

    json.translation = Object.fromEntries(
      Object.entries(json.translation).sort(([a], [b]) => a.localeCompare(b))
    )
    await fs.writeFile(filePath, stableStringify(json), 'utf8')
    console.log(`${locale}: ${applied} translations applied`)
    totalApplied += applied
  }

  console.log(`Total: ${totalApplied} translations applied`)
}

main().catch((error) => {
  console.error(error)
  process.exitCode = 1
})
```

- [ ] **Step 3: Apply translations and sync**

Run:

```bash
cd /Users/ethan/Documents/yunbay/web/default
node scripts/add-value-package-limit-i18n.mjs
bun run i18n:sync
rm scripts/add-value-package-limit-i18n.mjs
```

Expected:

```text
translations applied
sync completes
```

- [ ] **Step 4: Verify keys are present in every locale**

Run:

```bash
cd /Users/ethan/Documents/yunbay/web/default
node - <<'NODE'
const fs = require('node:fs')
const path = require('node:path')
const keys = [
  '1-day total limit',
  '7-day total limit',
  '30-day total limit',
  '7-day period limit',
  'Day cards can use this total quota during the 1-day validity period. 0 means unlimited total quota.',
  'Week cards can use this total quota from activation time until the 7-day expiration. 0 means unlimited total quota.',
  'Month cards can use this total quota from activation time until the 30-day expiration. 0 means unlimited total quota.',
  'Optional month-card period quota. It resets from activation time every fixed 7 days, and month-card reset can clear current 7-day period usage. 0 disables this period limit.',
  "This will consume 1 reset count. Day and week cards clear only the 5-hour usage window. Month cards clear both the 5-hour usage window and the current 7-day period usage. This will not restore total quota or extend expiration.",
]
for (const locale of ['en', 'zh', 'fr', 'ja', 'ru', 'vi']) {
  const json = JSON.parse(fs.readFileSync(path.join('src/i18n/locales', `${locale}.json`), 'utf8'))
  for (const key of keys) {
    if (!json.translation[key]) {
      throw new Error(`${locale} missing ${key}`)
    }
  }
}
console.log('all value-package limit keys found')
NODE
```

Expected:

```text
all value-package limit keys found
```

- [ ] **Step 5: Commit Task 7**

```bash
cd /Users/ethan/Documents/yunbay
git add web/default/src/i18n/locales/en.json \
  web/default/src/i18n/locales/zh.json \
  web/default/src/i18n/locales/fr.json \
  web/default/src/i18n/locales/ja.json \
  web/default/src/i18n/locales/ru.json \
  web/default/src/i18n/locales/vi.json
git commit -m "fix: add value package limit translations"
```

---

### Task 8: Full verification and regression cleanup

**Files:**
- Modify only files required by failing verification from Tasks 1-7.

- [ ] **Step 1: Search for stale rolling 7-day semantics**

Run:

```bash
cd /Users/ethan/Documents/yunbay
rg -n "7d rolling|rolling 7|7-day rolling|limit_7d_amount|7-day limit in displayed dollars|clear your current package's 5-hour and 7-day" model web/default/src
```

Expected:

```text
No backend error string contains "7d rolling".
No frontend user/admin copy says reset clears both 5-hour and 7-day for all package types.
Remaining limit_7d_amount matches schema, DTO, tests, or month-card period-limit UI.
```

- [ ] **Step 2: Run backend regression suite**

Run:

```bash
cd /Users/ethan/Documents/yunbay
go test ./model ./middleware ./controller ./service -run 'ValuePackage|Subscription|BillingSession|RealtimeValuePackage|OrderManagement' -count=1
```

Expected:

```text
ok for all listed packages
```

- [ ] **Step 3: Run frontend tests and typecheck**

Run:

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun test src/features/subscriptions/lib/value-package-limit-labels.test.ts \
  src/features/subscriptions/lib/plan-form-value-package.test.ts \
  src/features/subscriptions/components/subscriptions-mutate-drawer-value-package-source.test.ts
bun run typecheck
```

Expected:

```text
all selected bun tests pass
tsc -b exits 0
```

- [ ] **Step 4: Run frontend production build**

Run:

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun run build
```

Expected:

```text
rsbuild exits 0
```

- [ ] **Step 5: Check whitespace and final diff**

Run:

```bash
cd /Users/ethan/Documents/yunbay
git diff --check
git status --short
```

Expected:

```text
git diff --check has no output
Only intended files are modified if there are uncommitted verification fixes
```

- [ ] **Step 6: Commit verification fixes if any were required**

If Step 2, Step 3, Step 4, or Step 5 required edits, commit only those edits:

```bash
cd /Users/ethan/Documents/yunbay
git add model/subscription.go model/value_package_test.go \
  web/default/src/features/subscriptions/constants.ts \
  web/default/src/features/subscriptions/lib/plan-form.ts \
  web/default/src/features/subscriptions/lib/plan-form-value-package.test.ts \
  web/default/src/features/subscriptions/lib/value-package-limit-labels.ts \
  web/default/src/features/subscriptions/lib/value-package-limit-labels.test.ts \
  web/default/src/features/subscriptions/components/subscriptions-mutate-drawer.tsx \
  web/default/src/features/subscriptions/components/subscriptions-mutate-drawer-value-package-source.test.ts \
  web/default/src/features/subscriptions/components/value-package-admin-cards.tsx \
  web/default/src/features/value-packages/components/value-package-card.tsx \
  web/default/src/i18n/locales/en.json \
  web/default/src/i18n/locales/zh.json \
  web/default/src/i18n/locales/fr.json \
  web/default/src/i18n/locales/ja.json \
  web/default/src/i18n/locales/ru.json \
  web/default/src/i18n/locales/vi.json
git commit -m "fix: stabilize value package period limits"
```

Expected:

```text
Commit is created only when verification fixes changed files.
```

---

## Self-Review Notes

Spec coverage mapping:

- 7-day anchored to `UserSubscription.StartTime`: Tasks 1, 2, 3, 4.
- Reset scope by package type: Tasks 2, 3, 6.
- Day cards ignore 7-day limit: Tasks 1, 2, 3, 5, 6.
- Week cards use `total_amount` for 7-day total and reset only 5h: Tasks 2, 3, 5, 6.
- Month cards fixed 30 days and reset 5h + current 7-day period usage: Tasks 1, 2, 3, 4, 5, 6.
- Admin form can set 7-day and 30-day total limits through `total_amount`: Tasks 5, 6.
- User/admin UI no longer presents `limit_7d_amount` as total limit: Tasks 5, 6, 7.
- Error text no longer says rolling: Tasks 3, 8.
- Cross-database safety: Tasks 2 and 4 use ordinary GORM comparisons and Go filtering.
- i18n completeness: Task 7.
- Verification: Task 8.

Self-review scan: no unresolved placeholder markers or vague implementation instructions remain in the executable task list.
