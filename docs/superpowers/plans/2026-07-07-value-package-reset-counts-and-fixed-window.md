# Value Package Reset Counts and Fixed Window Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix day/week/month value-package reset countdowns so later usage does not push the current 5-hour/7-day restore time back to a full window, and add admin-managed reset counts that users can spend to reset only the short-cycle 5-hour/7-day buckets.

**Architecture:** The backend remains authoritative for value-package usage, reset events, reset-count balances, and admin adjustments. Rolling-window usage will use `MIN(created_at)` plus a `last_reset_at` lower bound, while total quota remains based on `UserSubscription.AmountUsed`. The frontend will add one user-side “重置额度” button directly below each package card’s start/close button and a separate admin “超值套餐管理” page under order management for reset-count operations and realtime usage.

**Tech Stack:** Go 1.22+, Gin, GORM v2, SQLite/MySQL/PostgreSQL-compatible migrations; React 19, TypeScript, Rsbuild, Base UI/shadcn-style project components, Tailwind, i18next; Bun for frontend scripts.

---

## Source Spec

Implement `docs/superpowers/specs/2026-07-07-value-package-reset-counts-and-fixed-window-design.md`.

This plan supersedes the earlier OpenAI-style reset-time plan only where reset-time semantics conflict. Use this plan’s `MIN(created_at) + window` rule, not the earlier `MAX(created_at) + window` rule.

## Scope and Non-Goals

In scope:

- Fix reset countdowns:
  - 5h reset is based on the earliest positive usage still in the effective 5h window.
  - 7d reset is based on the earliest positive usage still in the effective 7d window.
  - A later small usage in the same window must not reset the countdown to a full 5h/7d.
- Add a reset-count balance to every user’s value-package preference.
- Add reset event and reset-count ledger models.
- Let users spend one reset count to clear only the current active value package’s 5h/7d window usage.
- Add admin APIs for listing value-package users and set/add/subtract reset counts.
- Add independent admin page `/order-management/value-packages`.
- Add user-side reset button directly below the existing start/close main action button in every value-package card.
- Preserve LDXP discount config and routing-group behavior.

Out of scope:

- No paid self-purchase flow for reset counts in this iteration.
- No total quota reset.
- No subscription expiration extension.
- No deletion or mutation of historical `value_package_usage_records`.
- No changes to Plus/Pro distributor model groups.
- No deployment until user explicitly says to deploy.

## Current Code Landmarks

Backend:

- `model/subscription.go`
  - `UserValuePackagePreference`
  - `ValuePackageUsageRecord`
  - `ValuePackageUsageSummary`
  - `ValuePackageWindowUsageDetails`
  - `GetValuePackageState`
  - `ListActiveValuePackageUsageRows`
  - `buildValuePackageUsageSummaryTx`
  - `ActivateValuePackage`
  - `DeactivateValuePackage`
  - `GetValuePackageWindowUsageDetails`
- `model/main.go`
  - AutoMigrate list currently includes `UserValuePackagePreference` and `ValuePackageUsageRecord`.
- `middleware/value_package.go`
  - Performs value-package quota checks and limit error messages.
- `controller/value_package.go`
  - User value-package endpoints.
- `controller/order_management.go`
  - `AdminOrderManagementValuePackageUsage`.
- `router/api-router.go`
  - User `/api/value-packages/*` group.
  - Admin `/api/order-management/admin/*` group.

Frontend:

- `web/default/src/features/value-packages/api.ts`
- `web/default/src/features/value-packages/hooks/use-value-packages.ts`
- `web/default/src/features/value-packages/types.ts`
- `web/default/src/features/value-packages/components/value-package-card.tsx`
- `web/default/src/features/value-packages/lib/reset-time.ts`
- `web/default/src/features/order-management/api.ts`
- `web/default/src/features/order-management/types.ts`
- `web/default/src/features/order-management/components/value-package-usage-table.tsx`
- `web/default/src/features/order-management/index.tsx`
- `web/default/src/routes/_authenticated/order-management/index.tsx`
- `web/default/src/hooks/sidebar-data-model.ts`

## Planned File Map

Backend model and migration:

- Modify: `model/subscription.go`
  - Rename detailed-window timestamp fields from `Latest*CreatedAt` to `Earliest*CreatedAt`.
  - Add `ResetCount` to `UserValuePackagePreference`.
  - Add `ValuePackageQuotaReset` and `ValuePackageResetCountLedger` models.
  - Add service functions for reset-count adjustment and user reset.
  - Make rolling-window details use `last_reset_at` and `MIN(created_at)`.
- Modify: `model/main.go`
  - AutoMigrate new models.
  - Ensure SQLite add-column path for `user_value_package_preferences.reset_count`.
- Modify: `model/value_package_test.go`
  - Add/adjust model tests for MIN reset semantics, reset event lower bound, reset-count consume, admin adjustment, and concurrency.
- Modify: `model/value_package_migration_test.go`
  - Assert new table migration and `reset_count` column.

Backend middleware/controller/router:

- Modify: `middleware/value_package.go`
  - Continue using detailed reset seconds, now with MIN/reset-event semantics.
- Modify: `middleware/value_package_test.go`
  - Assert later usage does not extend limit error reset countdown.
  - Assert reset event lets exhausted user pass.
- Modify: `controller/value_package.go`
  - Add `ResetValuePackageQuotaSelf` user endpoint.
- Modify: `controller/order_management.go`
  - Add admin list and reset-count adjustment handlers.
- Modify: `router/api-router.go`
  - Register user reset endpoint and admin management endpoints.
- Modify: `controller/value_package_test.go`
  - Add user reset endpoint tests.
- Modify: `controller/order_management_test.go`
  - Add admin management list/adjust tests.

Frontend user value packages:

- Modify: `web/default/src/features/value-packages/types.ts`
  - Add `reset_count` to preference.
  - Add reset API response/request types if needed.
- Modify: `web/default/src/features/value-packages/api.ts`
  - Add `resetValuePackageQuota`.
- Modify: `web/default/src/features/value-packages/hooks/use-value-packages.ts`
  - Add `resetQuota` action.
- Modify: `web/default/src/features/value-packages/components/value-package-card.tsx`
  - Add button directly below main action button.
  - Show remaining reset count.
- Modify: `web/default/src/features/value-packages/components/value-package-card-source.test.ts`
  - Assert reset button placement and API prop presence.

Frontend admin management:

- Modify: `web/default/src/features/order-management/types.ts`
  - Add admin value-package management row/page types.
- Modify: `web/default/src/features/order-management/api.ts`
  - Add list and adjust API functions.
- Create: `web/default/src/features/order-management/components/value-package-management-page.tsx`
  - Independent admin management table and adjustment dialog.
- Create: `web/default/src/features/order-management/components/value-package-management-page-source.test.ts`
  - Source-level coverage for fields, route/API use, and adjustment controls.
- Create: `web/default/src/routes/_authenticated/order-management/value-packages.tsx`
  - Route wrapper for independent page.
- Modify: `web/default/src/hooks/sidebar-data-model.ts`
  - Add admin navigation entry to `/order-management/value-packages`.
- Modify: `web/default/src/features/order-management/order-management-source.test.ts`
  - Ensure old order-management page does not absorb the new management table and links or navigation refer to the new page.

Frontend i18n:

- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/zh.json`
- Modify: `web/default/src/i18n/locales/fr.json`
- Modify: `web/default/src/i18n/locales/ja.json`
- Modify: `web/default/src/i18n/locales/ru.json`
- Modify: `web/default/src/i18n/locales/vi.json`
  - Add all new user/admin text.

---

## Task 1: Fix reset countdown semantics from latest usage to earliest effective usage

**Files:**

- Modify: `model/subscription.go`
- Modify: `model/value_package_test.go`

- [ ] **Step 1: Write failing regression test for later usage not extending 5h reset**

In `model/value_package_test.go`, add this test near `TestValuePackageRollingUsageWindows`:

```go
func TestValuePackageWindowUsageDetailsDoesNotExtendResetWithLaterUsage(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3010, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-100, now+3600)

	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{
		UserId:             user.Id,
		UserSubscriptionId: sub.Id,
		PlanId:             day.Id,
		PackageType:        day.PackageType,
		ModelGroup:         day.ModelGroup,
		RequestId:          "first-usage",
		Quota:              50,
		CreatedAt:          now - 4*3600,
	}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{
		UserId:             user.Id,
		UserSubscriptionId: sub.Id,
		PlanId:             day.Id,
		PackageType:        day.PackageType,
		ModelGroup:         day.ModelGroup,
		RequestId:          "later-small-usage",
		Quota:              1,
		CreatedAt:          now - 2*3600,
	}))

	details, err := GetValuePackageWindowUsageDetails(user.Id, sub.Id, now)

	require.NoError(t, err)
	require.NotNil(t, details)
	require.EqualValues(t, 51, details.Used5h)
	require.EqualValues(t, now-4*3600, details.Earliest5hCreatedAt)
	require.EqualValues(t, now+3600, details.ResetAt5h)
	require.EqualValues(t, 3600, details.ResetSeconds5h)
	require.EqualValues(t, 51, details.Used7d)
	require.EqualValues(t, now-4*3600, details.Earliest7dCreatedAt)
	require.EqualValues(t, now-4*3600+7*24*3600, details.ResetAt7d)
}
```

- [ ] **Step 2: Run test and verify it fails before implementation**

Run:

```bash
go test ./model -run TestValuePackageWindowUsageDetailsDoesNotExtendResetWithLaterUsage -count=1 -timeout=300s
```

Expected before implementation:

- Compile fails because `Earliest5hCreatedAt` does not exist, or
- Test fails because reset is based on the later usage (`now + 3h`) instead of `now + 1h`.

- [ ] **Step 3: Rename detail fields and switch aggregation to MIN**

In `model/subscription.go`, replace `ValuePackageWindowUsageDetails` with:

```go
type ValuePackageWindowUsageDetails struct {
	Used5h              int64
	Earliest5hCreatedAt int64
	ResetAt5h           int64
	ResetSeconds5h      int64
	Used7d              int64
	Earliest7dCreatedAt int64
	ResetAt7d           int64
	ResetSeconds7d      int64
}
```

Then update `getValuePackageWindowUsageDetailsTx` local structs and select clauses from latest/MAX to earliest/MIN:

```go
var usage5h struct {
	Used              int64
	EarliestCreatedAt int64
}
if err := tx.Model(&ValuePackageUsageRecord{}).
	Where("user_id = ? AND user_subscription_id = ? AND created_at >= ? AND quota > ?", userId, userSubscriptionId, now-5*3600, 0).
	Select("COALESCE(SUM(quota), 0) AS used, COALESCE(MIN(created_at), 0) AS earliest_created_at").
	Scan(&usage5h).Error; err != nil {
	return nil, err
}
details.Used5h = usage5h.Used
details.Earliest5hCreatedAt = usage5h.EarliestCreatedAt
if details.Used5h > 0 && details.Earliest5hCreatedAt > 0 {
	details.ResetAt5h = details.Earliest5hCreatedAt + 5*3600
	details.ResetSeconds5h = details.ResetAt5h - now
	if details.ResetSeconds5h < 0 {
		details.ResetSeconds5h = 0
	}
}
```

Do the same for 7d:

```go
var usage7d struct {
	Used              int64
	EarliestCreatedAt int64
}
if err := tx.Model(&ValuePackageUsageRecord{}).
	Where("user_id = ? AND user_subscription_id = ? AND created_at >= ? AND quota > ?", userId, userSubscriptionId, now-7*24*3600, 0).
	Select("COALESCE(SUM(quota), 0) AS used, COALESCE(MIN(created_at), 0) AS earliest_created_at").
	Scan(&usage7d).Error; err != nil {
	return nil, err
}
details.Used7d = usage7d.Used
details.Earliest7dCreatedAt = usage7d.EarliestCreatedAt
if details.Used7d > 0 && details.Earliest7dCreatedAt > 0 {
	details.ResetAt7d = details.Earliest7dCreatedAt + 7*24*3600
	details.ResetSeconds7d = details.ResetAt7d - now
	if details.ResetSeconds7d < 0 {
		details.ResetSeconds7d = 0
	}
}
```

- [ ] **Step 4: Update existing tests that reference Latest fields**

In `model/value_package_test.go`, replace:

```go
details.Latest5hCreatedAt
details.Latest7dCreatedAt
```

with:

```go
details.Earliest5hCreatedAt
details.Earliest7dCreatedAt
```

Adjust expectations where existing tests expected `MAX(created_at)`. For example, in `TestValuePackageRollingUsageWindows`, the 7d window includes records at `now-6h` and `now-1h`, so new reset expectations should be:

```go
require.EqualValues(t, now-6*3600, details.Earliest7dCreatedAt)
require.EqualValues(t, now-6*3600+7*24*3600, details.ResetAt7d)
require.EqualValues(t, 7*24*3600-6*3600, details.ResetSeconds7d)
```

Keep 5h expectations based on the earliest record inside the last 5h.

- [ ] **Step 5: Run focused model tests**

Run:

```bash
go test ./model -run 'ValuePackageWindowUsageDetails|ValuePackageRollingUsageWindows|ValuePackageUsageSummaryResetFields' -count=1 -timeout=300s
```

Expected: all selected model tests pass.

- [ ] **Step 6: Commit Task 1**

```bash
git add model/subscription.go model/value_package_test.go
git commit -m "fix: keep value package reset window stable"
```

---

## Task 2: Add reset-count schema, migration coverage, and ledger models

**Files:**

- Modify: `model/subscription.go`
- Modify: `model/main.go`
- Modify: `model/value_package_migration_test.go`
- Modify: `model/value_package_test.go`

- [ ] **Step 1: Write failing migration test for reset_count and new tables**

In `model/value_package_migration_test.go`, add:

```go
func TestValuePackageResetCountMigrationAddsPreferenceColumnAndTables(t *testing.T) {
	setupValuePackageMigrationTestDB(t)

	require.NoError(t, DB.AutoMigrate(&UserValuePackagePreference{}))
	require.True(t, DB.Migrator().HasColumn(&UserValuePackagePreference{}, "reset_count"))
	require.NoError(t, DB.AutoMigrate(&ValuePackageQuotaReset{}, &ValuePackageResetCountLedger{}))
	require.True(t, DB.Migrator().HasTable(&ValuePackageQuotaReset{}))
	require.True(t, DB.Migrator().HasTable(&ValuePackageResetCountLedger{}))
}
```

Also add an old-table SQLite migration test:

```go
func TestEnsureUserValuePackagePreferenceTableSQLiteAddsResetCount(t *testing.T) {
	setupValuePackageMigrationTestDB(t)

	require.NoError(t, DB.Exec(`CREATE TABLE user_value_package_preferences (
		id integer primary key autoincrement,
		user_id integer,
		enabled numeric,
		active_user_subscription_id integer,
		created_at bigint,
		updated_at bigint
	)`).Error)

	require.NoError(t, ensureUserValuePackagePreferenceTableSQLite())
	require.True(t, DB.Migrator().HasColumn(&UserValuePackagePreference{}, "reset_count"))
}
```

- [ ] **Step 2: Run migration tests and verify they fail**

Run:

```bash
go test ./model -run 'ValuePackageResetCountMigration|EnsureUserValuePackagePreferenceTableSQLiteAddsResetCount' -count=1 -timeout=300s
```

Expected: fail because new structs/function/column do not exist.

- [ ] **Step 3: Add model fields and ledger structs**

In `model/subscription.go`, add to `UserValuePackagePreference`:

```go
ResetCount int `json:"reset_count" gorm:"default:0"`
```

Add constants near value-package constants:

```go
const (
	ValuePackageQuotaResetSourceUserConsumeCount = "user_consume_count"
	ValuePackageQuotaResetSourceAdminManualReset  = "admin_manual_reset"

	ValuePackageResetCountLedgerSourceAdminSet      = "admin_set"
	ValuePackageResetCountLedgerSourceAdminAdd      = "admin_add"
	ValuePackageResetCountLedgerSourceAdminSubtract = "admin_subtract"
	ValuePackageResetCountLedgerSourceUserConsume   = "user_consume"
)
```

Add structs near `ValuePackageUsageRecord`:

```go
type ValuePackageQuotaReset struct {
	Id                 int    `json:"id"`
	UserId             int    `json:"user_id" gorm:"index:idx_vp_reset_user_time,priority:1"`
	UserSubscriptionId int    `json:"user_subscription_id" gorm:"index"`
	PlanId             int    `json:"plan_id" gorm:"index"`
	PackageType        string `json:"package_type" gorm:"type:varchar(16);index"`
	ResetAt            int64  `json:"reset_at" gorm:"bigint;index:idx_vp_reset_user_time,priority:2"`
	Source             string `json:"source" gorm:"type:varchar(32);index"`
	CreatedByUserId    int    `json:"created_by_user_id" gorm:"index"`
	Note               string `json:"note" gorm:"type:text"`
}

func (r *ValuePackageQuotaReset) BeforeCreate(tx *gorm.DB) error {
	if r.ResetAt == 0 {
		r.ResetAt = common.GetTimestamp()
	}
	return nil
}

type ValuePackageResetCountLedger struct {
	Id              int    `json:"id"`
	UserId          int    `json:"user_id" gorm:"index:idx_vp_reset_count_ledger_user_time,priority:1"`
	Delta           int    `json:"delta"`
	BeforeCount     int    `json:"before_count"`
	AfterCount      int    `json:"after_count"`
	Source          string `json:"source" gorm:"type:varchar(32);index"`
	CreatedByUserId int    `json:"created_by_user_id" gorm:"index"`
	CreatedAt       int64  `json:"created_at" gorm:"bigint;index:idx_vp_reset_count_ledger_user_time,priority:2"`
	Note            string `json:"note" gorm:"type:text"`
}

func (l *ValuePackageResetCountLedger) BeforeCreate(tx *gorm.DB) error {
	if l.CreatedAt == 0 {
		l.CreatedAt = common.GetTimestamp()
	}
	return nil
}
```

- [ ] **Step 4: Add migrations**

In `model/main.go`, add these structs to both AutoMigrate model lists where `UserValuePackagePreference` and `ValuePackageUsageRecord` appear:

```go
&ValuePackageQuotaReset{},
&ValuePackageResetCountLedger{},
```

Add a SQLite helper near the existing ensure helpers:

```go
func ensureUserValuePackagePreferenceTableSQLite() error {
	if !DB.Migrator().HasTable(&UserValuePackagePreference{}) {
		return nil
	}
	if !DB.Migrator().HasColumn(&UserValuePackagePreference{}, "reset_count") {
		if err := DB.Exec("ALTER TABLE user_value_package_preferences ADD COLUMN reset_count integer DEFAULT 0").Error; err != nil {
			return err
		}
	}
	return nil
}
```

Call it in SQLite migration paths after `ensureUserSubscriptionTableSQLite()`:

```go
if err := ensureUserValuePackagePreferenceTableSQLite(); err != nil {
	return err
}
```

- [ ] **Step 5: Update test AutoMigrate calls**

Update test setup AutoMigrate calls that need the new models, especially in:

- `model/value_package_test.go`
- `middleware/value_package_test.go`
- `controller/value_package_test.go`
- `controller/order_management_test.go`
- `service/ldxp_session_test.go` and `service/ldxp_verify_test.go` only if their setup AutoMigrate fails after adding the new model types

For `model/value_package_test.go` setup, include:

```go
&ValuePackageQuotaReset{},
&ValuePackageResetCountLedger{},
```

- [ ] **Step 6: Run migration/model tests**

Run:

```bash
go test ./model -run 'ValuePackageResetCountMigration|EnsureUserValuePackagePreferenceTableSQLiteAddsResetCount|ValuePackage' -count=1 -timeout=300s
```

Expected: pass.

- [ ] **Step 7: Commit Task 2**

```bash
git add model/subscription.go model/main.go model/value_package_migration_test.go model/value_package_test.go middleware/value_package_test.go controller/value_package_test.go controller/order_management_test.go service

git commit -m "feat: add value package reset count schema"
```

If `git add service` stages unrelated files, replace it with exact changed test files from `git status --short`.

---

## Task 3: Add reset-count model services and reset-aware window calculation

**Files:**

- Modify: `model/subscription.go`
- Modify: `model/value_package_test.go`

- [ ] **Step 1: Write failing tests for reset-aware windows and count consume**

Add to `model/value_package_test.go`:

```go
func TestConsumeValuePackageResetCountResetsShortWindowsOnly(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3011, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	day.TotalAmount = 1000
	day.Limit5hAmount = 100
	day.Limit7dAmount = 200
	require.NoError(t, DB.Save(&day).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-100, now+3600)
	require.NoError(t, DB.Create(&UserValuePackagePreference{UserId: user.Id, Enabled: true, ActiveUserSubscriptionId: sub.Id, ResetCount: 1}).Error)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("amount_used", int64(300)).Error)
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "before-reset-5h", Quota: 100, CreatedAt: now - 1800}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "before-reset-7d", Quota: 50, CreatedAt: now - 6*3600}))

	state, err := ConsumeValuePackageResetCount(user.Id, sub.Id, now, user.Id, "test reset")

	require.NoError(t, err)
	require.NotNil(t, state)
	require.EqualValues(t, 0, state.Preference.ResetCount)
	require.NotNil(t, state.Usage)
	require.EqualValues(t, 0, state.Usage.Used5h)
	require.EqualValues(t, 0, state.Usage.Used7d)
	require.EqualValues(t, 300, state.Usage.TotalUsed)

	var resets []ValuePackageQuotaReset
	require.NoError(t, DB.Where("user_id = ? AND user_subscription_id = ?", user.Id, sub.Id).Find(&resets).Error)
	require.Len(t, resets, 1)
	require.Equal(t, ValuePackageQuotaResetSourceUserConsumeCount, resets[0].Source)
	require.EqualValues(t, now, resets[0].ResetAt)

	var ledger ValuePackageResetCountLedger
	require.NoError(t, DB.Where("user_id = ?", user.Id).First(&ledger).Error)
	require.Equal(t, ValuePackageResetCountLedgerSourceUserConsume, ledger.Source)
	require.EqualValues(t, -1, ledger.Delta)
	require.EqualValues(t, 1, ledger.BeforeCount)
	require.EqualValues(t, 0, ledger.AfterCount)
}
```

Add a second test proving post-reset usage counts again:

```go
func TestValuePackageWindowUsageCountsUsageAfterLastReset(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3012, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-100, now+3600)
	require.NoError(t, DB.Create(&ValuePackageQuotaReset{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ResetAt: now - 1800, Source: ValuePackageQuotaResetSourceUserConsumeCount, CreatedByUserId: user.Id}).Error)
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "before-reset", Quota: 100, CreatedAt: now - 3600}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "after-reset", Quota: 25, CreatedAt: now - 900}))

	details, err := GetValuePackageWindowUsageDetails(user.Id, sub.Id, now)

	require.NoError(t, err)
	require.EqualValues(t, 25, details.Used5h)
	require.EqualValues(t, now-900, details.Earliest5hCreatedAt)
	require.EqualValues(t, now-900+5*3600, details.ResetAt5h)
	require.EqualValues(t, 25, details.Used7d)
}
```

- [ ] **Step 2: Run tests and verify they fail**

Run:

```bash
go test ./model -run 'ConsumeValuePackageResetCount|WindowUsageCountsUsageAfterLastReset' -count=1 -timeout=300s
```

Expected: fail because service function and reset-aware lower bound do not exist.

- [ ] **Step 3: Add reset lower-bound helper**

In `model/subscription.go`, add:

```go
func getLastValuePackageQuotaResetAtTx(tx *gorm.DB, userId int, userSubscriptionId int) (int64, error) {
	if tx == nil {
		tx = DB
	}
	var resetAt int64
	err := tx.Model(&ValuePackageQuotaReset{}).
		Where("user_id = ? AND user_subscription_id = ? AND source = ?", userId, userSubscriptionId, ValuePackageQuotaResetSourceUserConsumeCount).
		Select("COALESCE(MAX(reset_at), 0)").
		Scan(&resetAt).Error
	return resetAt, err
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
```

If `maxInt64` already exists, reuse the existing helper instead of adding a duplicate.

- [ ] **Step 4: Apply last_reset_at to window starts**

At the top of `getValuePackageWindowUsageDetailsTx`, after `now` normalization, load reset bound:

```go
lastResetAt, err := getLastValuePackageQuotaResetAtTx(tx, userId, userSubscriptionId)
if err != nil {
	return nil, err
}
start5h := maxInt64(now-5*3600, lastResetAt)
start7d := maxInt64(now-7*24*3600, lastResetAt)
```

Then replace the query lower bounds:

```go
Where("user_id = ? AND user_subscription_id = ? AND created_at >= ? AND quota > ?", userId, userSubscriptionId, start5h, 0)
```

and:

```go
Where("user_id = ? AND user_subscription_id = ? AND created_at >= ? AND quota > ?", userId, userSubscriptionId, start7d, 0)
```

- [ ] **Step 5: Add reset-count adjustment types and functions**

In `model/subscription.go`, add:

```go
type ValuePackageResetCountAdjustMode string

const (
	ValuePackageResetCountAdjustSet      ValuePackageResetCountAdjustMode = "set"
	ValuePackageResetCountAdjustAdd      ValuePackageResetCountAdjustMode = "add"
	ValuePackageResetCountAdjustSubtract ValuePackageResetCountAdjustMode = "subtract"
)

type ValuePackageResetCountAdjustment struct {
	UserId      int    `json:"user_id"`
	OldCount    int    `json:"old_count"`
	NewCount    int    `json:"new_count"`
	Delta       int    `json:"delta"`
	Mode        string `json:"mode"`
	Reason      string `json:"reason"`
	AdminUserId int    `json:"admin_user_id"`
}
```

Add function:

```go
func AdjustValuePackageResetCount(userId int, mode ValuePackageResetCountAdjustMode, value int, reason string, adminUserId int) (*ValuePackageResetCountAdjustment, error) {
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	if value < 0 {
		return nil, errors.New("重置次数不能为负数")
	}
	var result *ValuePackageResetCountAdjustment
	err := DB.Transaction(func(tx *gorm.DB) error {
		pref, err := ensureValuePackagePreferenceForUpdateTx(tx, userId)
		if err != nil {
			return err
		}
		oldCount := pref.ResetCount
		newCount := oldCount
		source := ""
		switch mode {
		case ValuePackageResetCountAdjustSet:
			newCount = value
			source = ValuePackageResetCountLedgerSourceAdminSet
		case ValuePackageResetCountAdjustAdd:
			newCount = oldCount + value
			source = ValuePackageResetCountLedgerSourceAdminAdd
		case ValuePackageResetCountAdjustSubtract:
			newCount = oldCount - value
			if newCount < 0 {
				newCount = 0
			}
			source = ValuePackageResetCountLedgerSourceAdminSubtract
		default:
			return errors.New("无效的调整模式")
		}
		delta := newCount - oldCount
		if err := tx.Model(&UserValuePackagePreference{}).Where("user_id = ?", userId).Update("reset_count", newCount).Error; err != nil {
			return err
		}
		if err := tx.Create(&ValuePackageResetCountLedger{UserId: userId, Delta: delta, BeforeCount: oldCount, AfterCount: newCount, Source: source, CreatedByUserId: adminUserId, Note: strings.TrimSpace(reason)}).Error; err != nil {
			return err
		}
		result = &ValuePackageResetCountAdjustment{UserId: userId, OldCount: oldCount, NewCount: newCount, Delta: delta, Mode: string(mode), Reason: strings.TrimSpace(reason), AdminUserId: adminUserId}
		return nil
	})
	return result, err
}
```

Implement `ensureValuePackagePreferenceForUpdateTx` in the same file:

```go
func ensureValuePackagePreferenceForUpdateTx(tx *gorm.DB, userId int) (*UserValuePackagePreference, error) {
	if tx == nil {
		tx = DB
	}
	var pref UserValuePackagePreference
	err := tx.Where("user_id = ?", userId).First(&pref).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		pref = UserValuePackagePreference{UserId: userId, Enabled: false, ActiveUserSubscriptionId: 0, ResetCount: 0}
		if err := tx.Create(&pref).Error; err != nil {
			return nil, err
		}
		return &pref, nil
	}
	if err != nil {
		return nil, err
	}
	return &pref, nil
}
```

Ensure `model/subscription.go` imports `strings` already; if yes reuse it.

- [ ] **Step 6: Add user consume reset-count service**

In `model/subscription.go`, add:

```go
func ConsumeValuePackageResetCount(userId int, userSubscriptionId int, resetAt int64, actorUserId int, note string) (*ValuePackageState, error) {
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	if resetAt <= 0 {
		resetAt = common.GetTimestamp()
	}
	var state *ValuePackageState
	err := DB.Transaction(func(tx *gorm.DB) error {
		pref, err := ensureValuePackagePreferenceForUpdateTx(tx, userId)
		if err != nil {
			return err
		}
		if pref.ResetCount <= 0 {
			return errors.New("重置次数不足")
		}
		if !pref.Enabled || pref.ActiveUserSubscriptionId <= 0 {
			return errors.New("请先启用超值套餐后再重置额度")
		}
		if userSubscriptionId > 0 && userSubscriptionId != pref.ActiveUserSubscriptionId {
			return errors.New("当前套餐不匹配，请刷新后重试")
		}

		var sub UserSubscription
		if err := tx.Where("id = ? AND user_id = ? AND status = ? AND end_time > ?", pref.ActiveUserSubscriptionId, userId, UserSubscriptionStatusActive, resetAt).First(&sub).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("当前没有可重置的超值套餐")
			}
			return err
		}
		plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
		if err != nil {
			return err
		}
		normalizeValuePackagePlan(plan)
		if !plan.IsValuePackage() {
			return errors.New("当前没有可重置的超值套餐")
		}

		oldCount := pref.ResetCount
		newCount := oldCount - 1
		result := tx.Model(&UserValuePackagePreference{}).Where("user_id = ? AND reset_count > ?", userId, 0).Update("reset_count", newCount)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("重置次数不足或状态已变化，请刷新后重试")
		}
		if err := tx.Create(&ValuePackageQuotaReset{UserId: userId, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ResetAt: resetAt, Source: ValuePackageQuotaResetSourceUserConsumeCount, CreatedByUserId: actorUserId, Note: strings.TrimSpace(note)}).Error; err != nil {
			return err
		}
		if err := tx.Create(&ValuePackageResetCountLedger{UserId: userId, Delta: -1, BeforeCount: oldCount, AfterCount: newCount, Source: ValuePackageResetCountLedgerSourceUserConsume, CreatedByUserId: actorUserId, Note: strings.TrimSpace(note)}).Error; err != nil {
			return err
		}
		pref.ResetCount = newCount
		usage, err := buildValuePackageUsageSummaryTx(tx, userId, &sub, plan, resetAt)
		if err != nil {
			return err
		}
		state = &ValuePackageState{Preference: *pref, Subscription: &sub, Plan: plan, Usage: usage, Billing: buildValuePackageBillingState(pref, &sub, plan)}
		return nil
	})
	return state, err
}
```

If `buildValuePackageBillingState` expects pointer fields as currently defined, use exactly the same call pattern already used by `GetValuePackageState`/`ActivateValuePackage`.

- [ ] **Step 7: Add tests for admin adjustment**

Add:

```go
func TestAdjustValuePackageResetCountSupportsSetAddSubtract(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3013, UserGroupTiyan)

	setResult, err := AdjustValuePackageResetCount(user.Id, ValuePackageResetCountAdjustSet, 3, "set reason", 99)
	require.NoError(t, err)
	require.EqualValues(t, 0, setResult.OldCount)
	require.EqualValues(t, 3, setResult.NewCount)

	addResult, err := AdjustValuePackageResetCount(user.Id, ValuePackageResetCountAdjustAdd, 2, "add reason", 99)
	require.NoError(t, err)
	require.EqualValues(t, 3, addResult.OldCount)
	require.EqualValues(t, 5, addResult.NewCount)

	subtractResult, err := AdjustValuePackageResetCount(user.Id, ValuePackageResetCountAdjustSubtract, 10, "subtract reason", 99)
	require.NoError(t, err)
	require.EqualValues(t, 5, subtractResult.OldCount)
	require.EqualValues(t, 0, subtractResult.NewCount)

	var pref UserValuePackagePreference
	require.NoError(t, DB.Where("user_id = ?", user.Id).First(&pref).Error)
	require.EqualValues(t, 0, pref.ResetCount)

	var count int64
	require.NoError(t, DB.Model(&ValuePackageResetCountLedger{}).Where("user_id = ?", user.Id).Count(&count).Error)
	require.EqualValues(t, 3, count)
}
```

- [ ] **Step 8: Run focused model tests**

Run:

```bash
go test ./model -run 'ValuePackageWindowUsage|ConsumeValuePackageResetCount|AdjustValuePackageResetCount|ValuePackageUsageSummaryResetFields' -count=1 -timeout=300s
```

Expected: pass.

- [ ] **Step 9: Commit Task 3**

```bash
git add model/subscription.go model/value_package_test.go
git commit -m "feat: reset value package short windows"
```

---

## Task 4: Add user reset API and middleware regression coverage

**Files:**

- Modify: `controller/value_package.go`
- Modify: `router/api-router.go`
- Modify: `controller/value_package_test.go`
- Modify: `middleware/value_package_test.go`

- [ ] **Step 1: Write failing controller test for user reset endpoint**

In `controller/value_package_test.go`, add:

```go
func TestResetValuePackageQuotaSelfConsumesResetCount(t *testing.T) {
	setupValuePackageControllerTestDB(t)
	user := createValuePackageControllerUser(t, 9201, model.UserGroupTiyan)
	plan := createValuePackageControllerPlan(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay, 1, 3.9)
	plan.TotalAmount = 1000
	plan.Limit5hAmount = 100
	plan.Limit7dAmount = 200
	require.NoError(t, model.DB.Save(&plan).Error)
	now := common.GetTimestamp()
	sub := createValuePackageControllerSubscription(t, user.Id, plan, now-100, now+3600)
	require.NoError(t, model.DB.Create(&model.UserValuePackagePreference{UserId: user.Id, Enabled: true, ActiveUserSubscriptionId: sub.Id, ResetCount: 1}).Error)
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("id = ?", sub.Id).Update("amount_used", int64(300)).Error)
	require.NoError(t, model.RecordValuePackageUsage(&model.ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "before-reset", Quota: 100, CreatedAt: now - 1800}))

	rec := valuePackageControllerRequest(ResetValuePackageQuotaSelf, http.MethodPost, "/value-packages/reset-quota", gin.H{"user_subscription_id": sub.Id}, user.Id)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Success bool                     `json:"success"`
		Data    model.ValuePackageState  `json:"data"`
		Message string                   `json:"message"`
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)
	require.EqualValues(t, 0, resp.Data.Preference.ResetCount)
	require.NotNil(t, resp.Data.Usage)
	require.EqualValues(t, 0, resp.Data.Usage.Used5h)
	require.EqualValues(t, 0, resp.Data.Usage.Used7d)
	require.EqualValues(t, 300, resp.Data.Usage.TotalUsed)
}
```

Add insufficient count test:

```go
func TestResetValuePackageQuotaSelfRejectsWithoutCount(t *testing.T) {
	setupValuePackageControllerTestDB(t)
	user := createValuePackageControllerUser(t, 9202, model.UserGroupTiyan)
	plan := createValuePackageControllerPlan(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	sub := createValuePackageControllerSubscription(t, user.Id, plan, now-100, now+3600)
	require.NoError(t, model.DB.Create(&model.UserValuePackagePreference{UserId: user.Id, Enabled: true, ActiveUserSubscriptionId: sub.Id, ResetCount: 0}).Error)

	rec := valuePackageControllerRequest(ResetValuePackageQuotaSelf, http.MethodPost, "/value-packages/reset-quota", gin.H{"user_subscription_id": sub.Id}, user.Id)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "重置次数不足")
}
```


- [ ] **Step 2: Run controller tests and verify failure**

Run:

```bash
go test ./controller -run 'ResetValuePackageQuotaSelf' -count=1 -timeout=300s
```

Expected: fail because handler/route does not exist.

- [ ] **Step 3: Add request type and handler**

In `controller/value_package.go`, add:

```go
type valuePackageResetQuotaRequest struct {
	UserSubscriptionId int `json:"user_subscription_id"`
}

func ResetValuePackageQuotaSelf(c *gin.Context) {
	userId := c.GetInt("id")
	var req valuePackageResetQuotaRequest
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			common.ApiErrorMsg(c, "参数错误")
			return
		}
	}
	state, err := model.ConsumeValuePackageResetCount(userId, req.UserSubscriptionId, common.GetTimestamp(), userId, "user reset quota")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, state)
}
```

- [ ] **Step 4: Register user route**

In `router/api-router.go`, inside `valuePackageRoute` group, add:

```go
valuePackageRoute.POST("/reset-quota", middleware.CriticalRateLimit(), controller.ResetValuePackageQuotaSelf)
```

Place it near activate/deactivate.

- [ ] **Step 5: Add middleware regression tests**

In `middleware/value_package_test.go`, add two focused tests using the existing middleware test setup pattern from the same file. The first test must seed an active value-package user with `Limit5hAmount = 100`, then create two records:

```go
require.NoError(t, model.RecordValuePackageUsage(&model.ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "first", Quota: 99, CreatedAt: now - 4*3600}))
require.NoError(t, model.RecordValuePackageUsage(&model.ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "later", Quota: 1, CreatedAt: now - 2*3600}))
```

Invoke `ValuePackageEntitlement()` for that user and assert the request is rejected because `used_5h == 100`. The response body must contain `1 小时` and must not contain `3 小时`, proving the later request did not push the countdown from `now+1h` to `now+3h`.

The second test must create the same exhausted window, then insert a reset event at `now`:

```go
require.NoError(t, model.DB.Create(&model.ValuePackageQuotaReset{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ResetAt: now, Source: model.ValuePackageQuotaResetSourceUserConsumeCount, CreatedByUserId: user.Id}).Error)
```

Invoke `ValuePackageEntitlement()` again and assert the middleware does not abort. Use the test file's existing pattern for detecting `c.Next()`; if it uses a sentinel handler, assert that sentinel was reached.

- [ ] **Step 6: Run focused controller and middleware tests**

Run:

```bash
go test ./controller ./middleware -run 'ResetValuePackageQuotaSelf|ValuePackage.*Reset|ValuePackage.*Limit' -count=1 -timeout=300s
```

Expected: pass.

- [ ] **Step 7: Commit Task 4**

```bash
git add controller/value_package.go router/api-router.go controller/value_package_test.go middleware/value_package_test.go
git commit -m "feat: add value package quota reset api"
```

---

## Task 5: Add admin management APIs for value-package users and reset counts

**Files:**

- Modify: `model/subscription.go`
- Modify: `controller/order_management.go`
- Modify: `router/api-router.go`
- Modify: `controller/order_management_test.go`

- [ ] **Step 1: Write failing model/list test**

In `model/value_package_test.go`, add:

```go
func TestListValuePackageManagementRowsIncludesResetCountAndUsage(t *testing.T) {
	setupValuePackageTestDB(t)
	now := common.GetTimestamp()
	plan := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	plan.TotalAmount = 1000
	plan.Limit5hAmount = 100
	plan.Limit7dAmount = 500
	require.NoError(t, DB.Save(&plan).Error)
	user := createValuePackageUser(t, 3014, UserGroupTiyan)
	sub := createActiveValuePackageSub(t, user.Id, plan, now-100, now+86400)
	require.NoError(t, DB.Create(&UserValuePackagePreference{UserId: user.Id, Enabled: true, ActiveUserSubscriptionId: sub.Id, ResetCount: 4}).Error)
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "mgmt-usage", Quota: 25, CreatedAt: now - 1800}))

	result, err := ListValuePackageManagementRows(ValuePackageManagementFilter{Keyword: user.Username, PackageType: "all", Active: "active", Page: 1, PageSize: 20}, now)

	require.NoError(t, err)
	require.EqualValues(t, 1, result.Total)
	require.Len(t, result.Items, 1)
	row := result.Items[0]
	require.Equal(t, user.Id, row.UserId)
	require.Equal(t, user.Username, row.Username)
	require.EqualValues(t, 4, row.ResetCount)
	require.Equal(t, plan.PackageType, row.PackageType)
	require.NotNil(t, row.Usage)
	require.EqualValues(t, 25, row.Usage.Used5h)
}
```

- [ ] **Step 2: Run model list test and verify failure**

Run:

```bash
go test ./model -run TestListValuePackageManagementRowsIncludesResetCountAndUsage -count=1 -timeout=300s
```

Expected: fail because types/functions do not exist.

- [ ] **Step 3: Add model DTOs and list function**

In `model/subscription.go`, add:

```go
type ValuePackageManagementFilter struct {
	Keyword     string
	PackageType string
	Active      string
	Page        int
	PageSize    int
}

type ValuePackageManagementResult struct {
	Items []ValuePackageManagementRow `json:"items"`
	Total int64                       `json:"total"`
}

type ValuePackageManagementRow struct {
	UserId             int                       `json:"user_id"`
	Username           string                    `json:"username"`
	DisplayName        string                    `json:"display_name"`
	PackageType        string                    `json:"package_type"`
	PlanTitle          string                    `json:"plan_title"`
	SubscriptionId     int                       `json:"subscription_id"`
	SubscriptionStatus string                    `json:"subscription_status"`
	StartTime          int64                     `json:"start_time"`
	EndTime            int64                     `json:"end_time"`
	Enabled            bool                      `json:"enabled"`
	ResetCount         int                       `json:"reset_count"`
	Usage              *ValuePackageUsageSummary `json:"usage"`
	LastResetAt        int64                     `json:"last_reset_at"`
}
```

Implement:

```go
func ListValuePackageManagementRows(filter ValuePackageManagementFilter, now int64) (*ValuePackageManagementResult, error) {
	if now <= 0 {
		now = common.GetTimestamp()
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	var subs []UserSubscription
	query := DB.Model(&UserSubscription{}).Where("status = ?", UserSubscriptionStatusActive)
	if filter.Active == "expired" {
		query = DB.Model(&UserSubscription{}).Where("end_time <= ?", now)
	} else if filter.Active == "all" {
		query = DB.Model(&UserSubscription{})
	} else {
		query = query.Where("end_time > ?", now)
	}
	if err := query.Order("end_time desc, id desc").Find(&subs).Error; err != nil {
		return nil, err
	}

	items := make([]ValuePackageManagementRow, 0)
	keyword := strings.ToLower(strings.TrimSpace(filter.Keyword))
	packageType := strings.TrimSpace(filter.PackageType)
	for _, sub := range subs {
		plan, err := getSubscriptionPlanByIdTx(DB, sub.PlanId)
		if err != nil {
			continue
		}
		normalizeValuePackagePlan(plan)
		if !plan.IsValuePackage() {
			continue
		}
		if packageType != "" && packageType != "all" && plan.PackageType != packageType {
			continue
		}
		var user User
		if err := DB.Where("id = ?", sub.UserId).First(&user).Error; err != nil {
			continue
		}
		if keyword != "" {
			idText := strconv.Itoa(user.Id)
			if !strings.Contains(strings.ToLower(user.Username), keyword) && !strings.Contains(strings.ToLower(user.DisplayName), keyword) && !strings.Contains(idText, keyword) {
				continue
			}
		}
		pref := UserValuePackagePreference{UserId: user.Id}
		_ = DB.Where("user_id = ?", user.Id).First(&pref).Error
		usage, err := buildValuePackageUsageSummaryTx(DB, user.Id, &sub, plan, now)
		if err != nil {
			return nil, err
		}
		lastResetAt, err := getLastValuePackageQuotaResetAtTx(DB, user.Id, sub.Id)
		if err != nil {
			return nil, err
		}
		items = append(items, ValuePackageManagementRow{UserId: user.Id, Username: user.Username, DisplayName: user.DisplayName, PackageType: plan.PackageType, PlanTitle: plan.Title, SubscriptionId: sub.Id, SubscriptionStatus: sub.Status, StartTime: sub.StartTime, EndTime: sub.EndTime, Enabled: pref.Enabled && pref.ActiveUserSubscriptionId == sub.Id, ResetCount: pref.ResetCount, Usage: usage, LastResetAt: lastResetAt})
	}
	total := int64(len(items))
	start := (filter.Page - 1) * filter.PageSize
	if start >= len(items) {
		return &ValuePackageManagementResult{Items: []ValuePackageManagementRow{}, Total: total}, nil
	}
	end := start + filter.PageSize
	if end > len(items) {
		end = len(items)
	}
	return &ValuePackageManagementResult{Items: items[start:end], Total: total}, nil
}
```

Ensure `model/subscription.go` imports `strconv` if not already imported. If `strconv` already exists, do not duplicate.

- [ ] **Step 4: Write failing controller tests**

In `controller/order_management_test.go`, add tests:

```go
func TestAdminValuePackageManagementUsersReturnsRows(t *testing.T) {
	setupOrderManagementControllerTestDB(t)
	now := common.GetTimestamp()
	user := &model.User{Id: 8893, Username: "vp-management-user", Password: "password123", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: model.UserGroupTiyan, AffCode: "vp-management-aff"}
	require.NoError(t, model.DB.Create(user).Error)
	plan := &model.SubscriptionPlan{
		Title:         "月卡",
		Enabled:       true,
		PlanKind:      model.SubscriptionPlanKindValuePackage,
		PackageType:   model.ValuePackageTypeMonth,
		PackageLevel:  model.ValuePackageLevelMonth,
		Currency:      "CNY",
		DurationUnit:  model.SubscriptionDurationMonth,
		DurationValue: 1,
		ModelGroup:    "month-card",
		TotalAmount:   10000,
		Limit5hAmount: 500,
		Limit7dAmount: 5000,
		PriceAmount:   29.9,
	}
	require.NoError(t, model.DB.Create(plan).Error)
	sub := &model.UserSubscription{UserId: user.Id, PlanId: plan.Id, AmountTotal: plan.TotalAmount, AmountUsed: 350, StartTime: now - 100, EndTime: now + 86400, Status: model.UserSubscriptionStatusActive, Source: "test"}
	require.NoError(t, model.DB.Create(sub).Error)
	require.NoError(t, model.DB.Create(&model.UserValuePackagePreference{UserId: user.Id, Enabled: true, ActiveUserSubscriptionId: sub.Id, ResetCount: 3}).Error)
	require.NoError(t, model.RecordValuePackageUsage(&model.ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: plan.Id, PackageType: plan.PackageType, ModelGroup: plan.ModelGroup, RequestId: "management-usage", Quota: 50, CreatedAt: now - 1800}))

	ctx, recorder := newOrderManagementContext(http.MethodGet, "/api/order-management/admin/value-packages/users?page=1&page_size=20", "")

	AdminOrderManagementValuePackageUsers(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"reset_count":3`)
	require.Contains(t, recorder.Body.String(), `"items"`)
	require.Contains(t, recorder.Body.String(), "vp-management-user")
}
```

Add adjustment test:

```go
func TestAdminAdjustValuePackageResetCount(t *testing.T) {
	setupOrderManagementControllerTestDB(t)
	user := &model.User{Id: 8892, Username: "vp-reset-adjust", Password: "password123", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: model.UserGroupTiyan, AffCode: "vp-reset-adjust-aff"}
	require.NoError(t, model.DB.Create(user).Error)
	require.NoError(t, model.DB.Create(&model.UserValuePackagePreference{UserId: user.Id, ResetCount: 1}).Error)

	ctx, recorder := newOrderManagementContext(http.MethodPost, fmt.Sprintf("/api/order-management/admin/value-packages/users/%d/reset-count", user.Id), `{"mode":"add","value":2,"reason":"test"}`)

	AdminAdjustValuePackageResetCount(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"old_count":1`)
	require.Contains(t, recorder.Body.String(), `"new_count":3`)
}
```

- [ ] **Step 5: Add controller handlers**

In `controller/order_management.go`, add request types:

```go
type valuePackageResetCountAdjustRequest struct {
	Mode   string `json:"mode"`
	Value  int    `json:"value"`
	Reason string `json:"reason"`
}
```

Add list handler:

```go
func AdminOrderManagementValuePackageUsers(c *gin.Context) {
	pageInfo := getOrderManagementPageInfo(c)
	result, err := model.ListValuePackageManagementRows(model.ValuePackageManagementFilter{
		Keyword:     c.Query("keyword"),
		PackageType: c.DefaultQuery("package_type", "all"),
		Active:      c.DefaultQuery("active", "active"),
		Page:        pageInfo.Page,
		PageSize:    pageInfo.PageSize,
	}, common.GetTimestamp())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}
```

Add adjust handler:

```go
func AdminAdjustValuePackageResetCount(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || userId <= 0 {
		common.ApiErrorMsg(c, "无效的用户 ID")
		return
	}
	var req valuePackageResetCountAdjustRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	result, err := model.AdjustValuePackageResetCount(userId, model.ValuePackageResetCountAdjustMode(req.Mode), req.Value, req.Reason, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "value_package.reset_count.adjust", map[string]interface{}{"user_id": userId, "mode": req.Mode, "value": req.Value, "old_count": result.OldCount, "new_count": result.NewCount})
	common.ApiSuccess(c, result)
}
```

Use `common.DecodeJson`, not `encoding/json`.

- [ ] **Step 6: Register admin routes**

In `router/api-router.go`, inside `orderManagementAdminRoute`, add:

```go
orderManagementAdminRoute.GET("/value-packages/users", controller.AdminOrderManagementValuePackageUsers)
orderManagementAdminRoute.POST("/value-packages/users/:user_id/reset-count", controller.AdminAdjustValuePackageResetCount)
```

- [ ] **Step 7: Run focused tests**

Run:

```bash
go test ./model ./controller -run 'ValuePackageManagement|Admin.*ValuePackage.*Reset|AdminOrderManagementValuePackageUsers|AdjustValuePackageResetCount' -count=1 -timeout=300s
```

Expected: pass.

- [ ] **Step 8: Commit Task 5**

```bash
git add model/subscription.go controller/order_management.go router/api-router.go model/value_package_test.go controller/order_management_test.go
git commit -m "feat: manage value package reset counts"
```

---

## Task 6: Add frontend user reset button below the main action button

**Files:**

- Modify: `web/default/src/features/value-packages/types.ts`
- Modify: `web/default/src/features/value-packages/api.ts`
- Modify: `web/default/src/features/value-packages/hooks/use-value-packages.ts`
- Modify: `web/default/src/features/value-packages/components/value-package-card.tsx`
- Modify: `web/default/src/features/value-packages/components/value-package-card-source.test.ts`

- [ ] **Step 1: Write source test for button placement**

In `web/default/src/features/value-packages/components/value-package-card-source.test.ts`, add assertions that the reset button is inside `CardFooter` after the main action button. Example source-level test:

```ts
test('value package reset quota button is rendered directly below the main action button', async () => {
  const source = await Bun.file(sourcePath).text()
  assert.match(source, /onResetQuota/)
  assert.match(source, /Reset quota/)
  assert.match(source, /Remaining reset count/)
  assert.match(
    source,
    /<CardFooter[\s\S]*<Button[\s\S]*onClick=\{handleAction\}[\s\S]*\{actionLabel\}[\s\S]*<Button[\s\S]*onClick=\{handleResetQuota\}[\s\S]*\{t\('Reset quota'\)\}[\s\S]*<\/CardFooter>/
  )
})
```

- [ ] **Step 2: Run the source test and verify failure**

Run:

```bash
cd web/default
bun test src/features/value-packages/components/value-package-card-source.test.ts
```

Expected: fail because reset button/API props do not exist.

- [ ] **Step 3: Add frontend types and API function**

In `web/default/src/features/value-packages/types.ts`, add to `UserValuePackagePreference`:

```ts
reset_count: number
```

In `web/default/src/features/value-packages/api.ts`, add:

```ts
export async function resetValuePackageQuota(
  userSubscriptionId?: number
): Promise<ApiResponse<ValuePackageState>> {
  const res = await api.post('/api/value-packages/reset-quota', {
    user_subscription_id: userSubscriptionId,
  })
  return res.data
}
```

- [ ] **Step 4: Add hook action**

In `web/default/src/features/value-packages/hooks/use-value-packages.ts`, import `resetValuePackageQuota` and add:

```ts
const resetQuota = useCallback(
  async (userSubscriptionId?: number) => {
    setActionKey(`reset-quota-${userSubscriptionId || 'active'}`)
    try {
      const response = await resetValuePackageQuota(userSubscriptionId)
      if (!isApiSuccess(response) || !response.data) {
        const message = getErrorMessage(
          response.message,
          t('Failed to reset value package quota')
        )
        toast.error(message)
        return false
      }
      setState(response.data)
      syncGlobalState(response.data)
      toast.success(t('Value package quota reset'))
      return true
    } catch (_error) {
      toast.error(t('Failed to reset value package quota'))
      return false
    } finally {
      setActionKey(null)
    }
  },
  [syncGlobalState, t]
)
```

Return it:

```ts
resetQuota,
```

- [ ] **Step 5: Wire hook to page/card props**

Inspect `web/default/src/features/value-packages/index.tsx` for existing `ValuePackageCard` props. Add:

```tsx
onResetQuota={resetQuota}
```

and pass loading state via existing `actionKey`.

- [ ] **Step 6: Add card props and button directly below main button**

In `web/default/src/features/value-packages/components/value-package-card.tsx`, add props:

```ts
onResetQuota: (userSubscriptionId?: number) => void | Promise<boolean>
```

Add handler:

```ts
const handleResetQuota = () => {
  if (!state?.subscription?.id && !cardState.userSubscriptionId) {
    return
  }
  const confirmed = window.confirm(
    t(
      "This will consume 1 reset count and clear your current package's 5-hour and 7-day usage windows. It will not restore total quota or extend expiration."
    )
  )
  if (!confirmed) {
    return
  }
  void onResetQuota(state?.subscription?.id || cardState.userSubscriptionId)
}
```

Use the existing props already passed to `ValuePackageCard` from `web/default/src/features/value-packages/index.tsx`. The reset target must be the active subscription id represented by the card state. If `cardState.userSubscriptionId` is available, pass that value; otherwise pass `undefined` so the backend resets the currently active package from the user's preference. Do not add global stores for this button.

Change `CardFooter` from a single button to a vertical stack:

```tsx
<CardFooter className='bg-muted/35 flex flex-col gap-2 p-4 sm:p-5'>
  <Button
    className='w-full'
    variant={cardState.kind === 'running' ? 'outline' : 'default'}
    disabled={disabled}
    onClick={handleAction}
  >
    {isBusy ? <Loader2 className='animate-spin' data-icon='inline-start' /> : null}
    {cardState.kind === 'start' && !isBusy ? <Play data-icon='inline-start' /> : null}
    {actionLabel}
  </Button>
  {(cardState.kind === 'running' || cardState.kind === 'start') ? (
    <Button
      className='w-full'
      variant='secondary'
      disabled={resetCount <= 0 || resetBusy || disabled}
      onClick={handleResetQuota}
    >
      {resetBusy ? <Loader2 className='animate-spin' data-icon='inline-start' /> : null}
      {t('Reset quota')}
    </Button>
  ) : null}
  {(cardState.kind === 'running' || cardState.kind === 'start') ? (
    <p className='text-muted-foreground text-center text-xs'>
      {t('Remaining reset count')}: {resetCount}
    </p>
  ) : null}
</CardFooter>
```

Define:

```ts
const resetCount = Number(state?.preference?.reset_count || 0)
const resetBusy = actionKey === `reset-quota-${state?.subscription?.id || cardState.userSubscriptionId || 'active'}`
```

Use whichever state/prop names exist in the component. Keep the reset button directly after the main action button in JSX.

- [ ] **Step 7: Run frontend focused tests**

Run:

```bash
cd web/default
bun test src/features/value-packages/components/value-package-card-source.test.ts
```

Expected: pass.

- [ ] **Step 8: Commit Task 6**

```bash
git add web/default/src/features/value-packages/types.ts web/default/src/features/value-packages/api.ts web/default/src/features/value-packages/hooks/use-value-packages.ts web/default/src/features/value-packages/index.tsx web/default/src/features/value-packages/components/value-package-card.tsx web/default/src/features/value-packages/components/value-package-card-source.test.ts
git commit -m "feat: add value package quota reset button"
```

---

## Task 7: Add independent admin value-package management page

**Files:**

- Modify: `web/default/src/features/order-management/types.ts`
- Modify: `web/default/src/features/order-management/api.ts`
- Create: `web/default/src/features/order-management/components/value-package-management-page.tsx`
- Create: `web/default/src/features/order-management/components/value-package-management-page-source.test.ts`
- Create: `web/default/src/routes/_authenticated/order-management/value-packages.tsx`
- Modify: `web/default/src/hooks/sidebar-data-model.ts`
- Modify: `web/default/src/features/order-management/order-management-source.test.ts`

- [ ] **Step 1: Write source tests for independent page**

Create `web/default/src/features/order-management/components/value-package-management-page-source.test.ts`:

```ts
import { describe, expect, test } from 'bun:test'
import { readFileSync } from 'node:fs'

const sourcePath = new URL('./value-package-management-page.tsx', import.meta.url)

describe('value package management page source', () => {
  test('uses dedicated admin APIs and exposes reset count controls', () => {
    const source = readFileSync(sourcePath, 'utf8')
    expect(source).toContain('getValuePackageManagementUsers')
    expect(source).toContain('adjustValuePackageResetCount')
    expect(source).toContain('reset_count')
    expect(source).toContain('Reset count')
    expect(source).toContain('set')
    expect(source).toContain('add')
    expect(source).toContain('subtract')
  })

  test('shows realtime quota fields on independent page', () => {
    const source = readFileSync(sourcePath, 'utf8')
    expect(source).toContain('used_5h')
    expect(source).toContain('used_7d')
    expect(source).toContain('total_remaining')
    expect(source).toContain('last_reset_at')
  })
})
```

This test will fail until the file exists.

- [ ] **Step 2: Add route source test assertion**

In `web/default/src/features/order-management/order-management-source.test.ts`, add assertions:

```ts
test('value package management lives on a dedicated route', async () => {
  const routeSource = await Bun.file(
    new URL('../../routes/_authenticated/order-management/value-packages.tsx', import.meta.url)
  ).text()
  assert.match(routeSource, /ValuePackageManagementPage/)
})
```

The test must read the exact route file path shown above; create that route file in Step 7 so the test can pass.

- [ ] **Step 3: Run source tests and verify failure**

Run:

```bash
cd web/default
bun test src/features/order-management/components/value-package-management-page-source.test.ts src/features/order-management/order-management-source.test.ts
```

Expected: fail because files/functions do not exist.

- [ ] **Step 4: Add frontend types**

In `web/default/src/features/order-management/types.ts`, add:

```ts
export interface ValuePackageManagementRow {
  user_id: number
  username: string
  display_name: string
  package_type: string
  plan_title: string
  subscription_id: number
  subscription_status: string
  start_time: number
  end_time: number
  enabled: boolean
  reset_count: number
  usage: OrderManagementValuePackageUsageSummary | null
  last_reset_at: number
}

export interface ValuePackageManagementResponse {
  items: ValuePackageManagementRow[]
  total: number
}

export interface AdjustValuePackageResetCountRequest {
  mode: 'set' | 'add' | 'subtract'
  value: number
  reason: string
}

export interface AdjustValuePackageResetCountResponse {
  user_id: number
  old_count: number
  new_count: number
  delta: number
  mode: string
  reason: string
  admin_user_id: number
}
```

- [ ] **Step 5: Add API functions**

In `web/default/src/features/order-management/api.ts`, import the new types and add:

```ts
export interface GetValuePackageManagementUsersParams {
  page?: number
  page_size?: number
  keyword?: string
  package_type?: 'day' | 'week' | 'month' | 'all'
  active?: 'active' | 'expired' | 'all'
}

export async function getValuePackageManagementUsers(
  params: GetValuePackageManagementUsersParams = {}
): Promise<ApiResponse<ValuePackageManagementResponse>> {
  const res = await api.get(
    `/api/order-management/admin/value-packages/users${withDefinedParams(params)}`
  )
  return res.data
}

export async function adjustValuePackageResetCount(
  userId: number,
  payload: AdjustValuePackageResetCountRequest
): Promise<ApiResponse<AdjustValuePackageResetCountResponse>> {
  const res = await api.post(
    `/api/order-management/admin/value-packages/users/${userId}/reset-count`,
    payload
  )
  return res.data
}
```

- [ ] **Step 6: Create management page component**

Create `web/default/src/features/order-management/components/value-package-management-page.tsx` with this minimum working component:

```tsx
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { formatValuePackageResetLine } from '@/features/value-packages/lib/reset-time'
import {
  adjustValuePackageResetCount,
  getValuePackageManagementUsers,
  type GetValuePackageManagementUsersParams,
} from '../api'
import type { ValuePackageManagementRow } from '../types'

export function ValuePackageManagementPage() {
  const { t } = useTranslation()
  const [keyword, setKeyword] = useState('')
  const [packageType, setPackageType] = useState<'all' | 'day' | 'week' | 'month'>('all')
  const [active, setActive] = useState<'active' | 'expired' | 'all'>('active')
  const [rows, setRows] = useState<ValuePackageManagementRow[]>([])
  const [loading, setLoading] = useState(false)
  const [editing, setEditing] = useState<ValuePackageManagementRow | null>(null)
  const [mode, setMode] = useState<'set' | 'add' | 'subtract'>('add')
  const [value, setValue] = useState('1')
  const [reason, setReason] = useState('')

  const params = useMemo<GetValuePackageManagementUsersParams>(() => ({
    page: 1,
    page_size: 50,
    keyword,
    package_type: packageType,
    active,
  }), [active, keyword, packageType])

  async function loadRows() {
    setLoading(true)
    try {
      const response = await getValuePackageManagementUsers(params)
      if (!response.success || !response.data) {
        toast.error(response.message || t('Failed to load value package users'))
        return
      }
      setRows(response.data.items || [])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void loadRows()
    const timer = window.setInterval(() => void loadRows(), 15000)
    return () => window.clearInterval(timer)
  }, [params])

  async function submitAdjust() {
    if (!editing) return
    const numericValue = Number(value)
    if (!Number.isFinite(numericValue) || numericValue < 0) {
      toast.error(t('Reset count must be a non-negative number'))
      return
    }
    const response = await adjustValuePackageResetCount(editing.user_id, {
      mode,
      value: numericValue,
      reason,
    })
    if (!response.success) {
      toast.error(response.message || t('Failed to adjust reset count'))
      return
    }
    toast.success(t('Reset count updated'))
    setEditing(null)
    setReason('')
    await loadRows()
  }

  return (
    <div className='flex flex-col gap-4 p-4'>
      <div>
        <h1 className='text-2xl font-bold'>{t('Value Package Management')}</h1>
        <p className='text-muted-foreground text-sm'>{t('Manage day, week, and month package reset counts and realtime quota.')}</p>
      </div>
      <div className='grid gap-3 md:grid-cols-4'>
        <Input value={keyword} onChange={(event) => setKeyword(event.target.value)} placeholder={t('Search user')} />
        <Select value={packageType} onValueChange={(next) => setPackageType(next as typeof packageType)}>
          <SelectTrigger><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value='all'>{t('All packages')}</SelectItem>
            <SelectItem value='day'>{t('Day package')}</SelectItem>
            <SelectItem value='week'>{t('Week package')}</SelectItem>
            <SelectItem value='month'>{t('Month package')}</SelectItem>
          </SelectContent>
        </Select>
        <Select value={active} onValueChange={(next) => setActive(next as typeof active)}>
          <SelectTrigger><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value='active'>{t('Active')}</SelectItem>
            <SelectItem value='expired'>{t('Expired')}</SelectItem>
            <SelectItem value='all'>{t('All')}</SelectItem>
          </SelectContent>
        </Select>
        <Button variant='outline' onClick={() => void loadRows()} disabled={loading}>{t('Refresh')}</Button>
      </div>
      <div className='rounded-lg border'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('User')}</TableHead>
              <TableHead>{t('Package')}</TableHead>
              <TableHead>{t('Reset count')}</TableHead>
              <TableHead>{t('5-hour limit')}</TableHead>
              <TableHead>{t('7-day limit')}</TableHead>
              <TableHead>{t('Remaining quota')}</TableHead>
              <TableHead>{t('Last reset')}</TableHead>
              <TableHead>{t('Actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((row) => (
              <TableRow key={`${row.user_id}-${row.subscription_id}`}>
                <TableCell>{row.username}<div className='text-muted-foreground text-xs'>#{row.user_id}</div></TableCell>
                <TableCell>{row.plan_title || row.package_type}</TableCell>
                <TableCell>{row.reset_count}</TableCell>
                <TableCell>{row.usage?.used_5h ?? 0} / {row.usage?.limit_5h ?? 0}<div className='text-muted-foreground text-xs'>{formatValuePackageResetLine({ limit: row.usage?.limit_5h || 0, resetSeconds: row.usage?.reset_seconds_5h || 0, limited: row.usage?.limited_5h || false, t })}</div></TableCell>
                <TableCell>{row.usage?.used_7d ?? 0} / {row.usage?.limit_7d ?? 0}<div className='text-muted-foreground text-xs'>{formatValuePackageResetLine({ limit: row.usage?.limit_7d || 0, resetSeconds: row.usage?.reset_seconds_7d || 0, limited: row.usage?.limited_7d || false, t })}</div></TableCell>
                <TableCell>{row.usage?.total_remaining ?? 0}</TableCell>
                <TableCell>{row.last_reset_at ? new Date(row.last_reset_at * 1000).toLocaleString() : t('Never')}</TableCell>
                <TableCell><Button size='sm' variant='outline' onClick={() => setEditing(row)}>{t('Adjust reset count')}</Button></TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      {editing ? (
        <div className='rounded-lg border p-4'>
          <h2 className='font-semibold'>{t('Adjust reset count')} - {editing.username}</h2>
          <div className='mt-3 grid gap-3 md:grid-cols-4'>
            <div><Label>{t('Mode')}</Label><Select value={mode} onValueChange={(next) => setMode(next as typeof mode)}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent><SelectItem value='set'>{t('Set')}</SelectItem><SelectItem value='add'>{t('Add')}</SelectItem><SelectItem value='subtract'>{t('Subtract')}</SelectItem></SelectContent></Select></div>
            <div><Label>{t('Value')}</Label><Input value={value} onChange={(event) => setValue(event.target.value)} /></div>
            <div className='md:col-span-2'><Label>{t('Reason')}</Label><Input value={reason} onChange={(event) => setReason(event.target.value)} /></div>
          </div>
          <div className='mt-3 flex gap-2'><Button onClick={() => void submitAdjust()}>{t('Confirm')}</Button><Button variant='outline' onClick={() => setEditing(null)}>{t('Cancel')}</Button></div>
        </div>
      ) : null}
    </div>
  )
}
```

Before writing this file, inspect `web/default/src/features/order-management/index.tsx` and reuse the same table, button, input, select, and toast import paths used there. Keep imports project-local; do not add new UI dependencies.

- [ ] **Step 7: Add route**

Create `web/default/src/routes/_authenticated/order-management/value-packages.tsx`:

```tsx
import { createFileRoute } from '@tanstack/react-router'
import { ValuePackageManagementPage } from '@/features/order-management/components/value-package-management-page'

export const Route = createFileRoute('/_authenticated/order-management/value-packages')({
  component: ValuePackageManagementPage,
})
```

- [ ] **Step 8: Add sidebar entry**

In `web/default/src/hooks/sidebar-data-model.ts`, add an admin-accessible item near existing order management entry:

```ts
{
  title: t('Value Package Management'),
  url: '/order-management/value-packages',
  icon: icons.valuePackages,
},
```

Use an icon that exists in the `icons` map. If `icons.valuePackages` exists, use it; otherwise use `icons.package` or `icons.layoutDashboard` after checking the file.

- [ ] **Step 9: Run source tests**

Run:

```bash
cd web/default
bun test src/features/order-management/components/value-package-management-page-source.test.ts src/features/order-management/order-management-source.test.ts
```

Expected: pass.

- [ ] **Step 10: Commit Task 7**

```bash
git add web/default/src/features/order-management/types.ts web/default/src/features/order-management/api.ts web/default/src/features/order-management/components/value-package-management-page.tsx web/default/src/features/order-management/components/value-package-management-page-source.test.ts web/default/src/routes/_authenticated/order-management/value-packages.tsx web/default/src/hooks/sidebar-data-model.ts web/default/src/features/order-management/order-management-source.test.ts
git commit -m "feat: add value package management page"
```

---

## Task 8: Add i18n translations and type/build verification

**Files:**

- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/zh.json`
- Modify: `web/default/src/i18n/locales/fr.json`
- Modify: `web/default/src/i18n/locales/ja.json`
- Modify: `web/default/src/i18n/locales/ru.json`
- Modify: `web/default/src/i18n/locales/vi.json`

- [ ] **Step 1: Run i18n sync to find missing keys**

Run:

```bash
cd web/default
bun run i18n:sync
```

Expected before translations: generated report may show missing untranslated keys.

- [ ] **Step 2: Add translations for all new keys**

Ensure every locale includes these keys at minimum:

```json
{
  "Reset quota": "Reset quota",
  "Remaining reset count": "Remaining reset count",
  "Failed to reset value package quota": "Failed to reset value package quota",
  "Value package quota reset": "Value package quota reset",
  "This will consume 1 reset count and clear your current package's 5-hour and 7-day usage windows. It will not restore total quota or extend expiration.": "This will consume 1 reset count and clear your current package's 5-hour and 7-day usage windows. It will not restore total quota or extend expiration.",
  "Value Package Management": "Value Package Management",
  "Manage day, week, and month package reset counts and realtime quota.": "Manage day, week, and month package reset counts and realtime quota.",
  "Search user": "Search user",
  "All packages": "All packages",
  "Day package": "Day package",
  "Week package": "Week package",
  "Month package": "Month package",
  "Reset count": "Reset count",
  "Last reset": "Last reset",
  "Adjust reset count": "Adjust reset count",
  "Mode": "Mode",
  "Set": "Set",
  "Add": "Add",
  "Subtract": "Subtract",
  "Reason": "Reason",
  "Reset count must be a non-negative number": "Reset count must be a non-negative number",
  "Failed to load value package users": "Failed to load value package users",
  "Failed to adjust reset count": "Failed to adjust reset count",
  "Reset count updated": "Reset count updated",
  "Never": "Never"
}
```

Translate values appropriately in `zh`, `fr`, `ja`, `ru`, `vi`. Preserve placeholders exactly if any are added later.

- [ ] **Step 3: Re-run i18n sync**

Run:

```bash
cd web/default
bun run i18n:sync
```

Expected: `missingCount: 0`, `untranslatedCount: 0`.

- [ ] **Step 4: Run frontend tests/typecheck/build**

Run:

```bash
cd web/default
bun test \
  src/features/value-packages/components/value-package-card-source.test.ts \
  src/features/order-management/components/value-package-management-page-source.test.ts \
  src/features/order-management/order-management-source.test.ts \
  src/features/value-packages/lib/reset-time.test.ts \
  src/features/order-management/components/value-package-usage-table-source.test.ts
bun run typecheck
bun run build
```

Expected: all tests pass, typecheck exits 0, build exits 0.

- [ ] **Step 5: Commit Task 8**

```bash
git add web/default/src/i18n/locales/en.json web/default/src/i18n/locales/zh.json web/default/src/i18n/locales/fr.json web/default/src/i18n/locales/ja.json web/default/src/i18n/locales/ru.json web/default/src/i18n/locales/vi.json
git commit -m "chore: translate value package reset count ui"
```

---

## Task 9: Full regression, main merge, GitHub push, and deployment gate

**Files:**

- No planned source changes unless verification reveals a bug.

- [ ] **Step 1: Run backend focused regression**

Run:

```bash
go test ./model ./middleware ./controller -run 'ValuePackage|OrderManagement' -count=1 -timeout=300s
```

Expected: pass.

- [ ] **Step 2: Run backend package regression**

Run:

```bash
go test ./middleware ./service ./model ./controller -count=1 -timeout=300s
```

Expected: pass.

- [ ] **Step 3: Verify LDXP discount config unchanged**

Run:

```bash
rg -n '"amount":50,"money":47\.5|"amount":100,"money":90|"amount":500,"money":425' service/ldxp_config.go
```

Expected output includes all three discount rows:

```text
50 -> 47.5
100 -> 90
500 -> 425
```

- [ ] **Step 4: Verify no routing-group regression**

Run:

```bash
rg -n 'ContextKeyUsingGroup|ContextKeyTokenGroup|ContextKeyUserGroup|day-card|week-card|month-card' middleware/value_package.go relay/common/relay_info.go service/billing_session.go
```

Expected:

- No new code that sets `ContextKeyUsingGroup` or `ContextKeyTokenGroup` to day/week/month card groups.
- Existing billing identity logic remains separate from distributor routing.

- [ ] **Step 5: Run git status and inspect diff**

Run:

```bash
git status --short
git log --oneline --decorate -8
git diff origin/main...HEAD --stat
```

Expected:

- Only intended files changed.
- No secrets, build artifacts, `web/default/dist`, `node_modules`, or server files staged.

- [ ] **Step 6: Merge to local main only after all tests pass**

Run:

```bash
git checkout main
git pull --ff-only origin main
git merge --ff-only codex/value-package-fixed-reset-window
```

Expected: fast-forward merge succeeds.

- [ ] **Step 7: Push GitHub main**

Run:

```bash
git push origin main
```

Expected: push succeeds and `origin/main` matches local `HEAD`.

- [ ] **Step 8: Deployment gate**

Do not deploy automatically unless the user has explicitly said to deploy after implementation. If the user says deploy:

1. Use server credentials from the private local desktop Yunbei folder as previously established.
2. Create source and image backups before rebuilding.
3. Use rsync exclusions for `.git/`, `.worktrees/`, `node_modules/`, `dist/`, `.env`, `docker-compose.prod.yml`, logs, tmp, DB files.
4. Rebuild only `new-api` through compose with the production env file.
5. Restart only `new-api` unless another service must change.
6. Verify:
   - `.yunbay-deploy-sha` matches `git rev-parse HEAD`.
   - `yunbay-new-api` is `running healthy`.
   - `https://yunbay.xyz/api/status` returns HTTP 200 and `success=true`.
   - Admin value-package management API returns `reset_count` fields.
   - User value package self API returns `preference.reset_count`.

- [ ] **Step 9: Final report**

Report in Chinese:

- branch and commit range;
- tests run and pass/fail evidence;
- whether main/GitHub pushed;
- whether deployment was performed or waiting for deployment approval;
- explicit statement that total quota and expiration are not reset;
- explicit statement that routing groups were not changed.

