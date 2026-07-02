# 超值套餐与会员仪式感 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the 日卡 / 周卡 / 月卡 “超值套餐” system described in `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/docs/superpowers/specs/2026-07-03-value-packages-design.md`, including per-card LDXP payment configuration, activation/deactivation, model-group switching, rolling limits, concurrency limits, user UI, admin UI, and one-time VIP celebration.

**Architecture:** Extend the existing subscription tables and LDXP session pipeline instead of creating a separate billing product stack. Use `SubscriptionPlan` with `plan_kind=value_package` for the three cards, `UserSubscription` for purchased instances, a small user preference table for the active/closed switch, and a dedicated value-package usage table for rolling limits. Relay requests consult the active package before channel selection, force the package model group while enabled, and record settled usage for rolling windows.

**Tech Stack:** Go 1.22+, Gin, GORM v2, SQLite/MySQL/PostgreSQL-compatible migrations, React 19 + TypeScript, Rsbuild, Base UI/shadcn components, Tailwind CSS, Bun for frontend scripts.

---

## Scope and sequencing

This plan intentionally includes the full feature because the user asked for an implementation plan after approving the full spec. Execute in order. Each task leaves the app in a testable state and should be committed before moving on.

Important project constraints:

- Use `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages` as the worktree.
- Do not modify protected project identity references.
- Backend JSON marshal/unmarshal calls in business code must use `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/common/json.go` wrappers.
- All DB changes must support SQLite, MySQL >= 5.7.8, PostgreSQL >= 9.6.
- Use Bun for `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default`.
- If implementation reaches billing expression files, read `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/pkg/billingexpr/expr.md` first. This plan does not require changing billing expressions.

## Planned file map

### Backend model and migration

- Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/subscription.go`
  - Add value-package fields to `SubscriptionPlan`.
  - Add covered-state fields to `UserSubscription`.
  - Add `UserValuePackagePreference` and `ValuePackageUsageRecord` models.
  - Add constants and service-style model functions for plan classification, purchase rules, activation, rolling limits, and usage records.
- Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/main.go`
  - AutoMigrate the new structs.
  - Extend `ensureSubscriptionPlanTableSQLite` for new `subscription_plans` columns.
  - Add SQLite add-column support for any new `user_subscriptions` columns.
- Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/value_package_test.go`
  - Cover purchase rules, activation/deactivation, rolling limit math, and VIP topup accounting behavior.

### Backend LDXP and API

- Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/ldxp_topup.go`
  - Add purpose and subscription-order linkage fields to `LdxpTopupSession`.
- Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/service/ldxp_session.go`
  - Add `CreateLdxpValuePackageSession` and purpose-aware active-session reuse.
- Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/service/ldxp_verify.go`
  - On verified LDXP session, branch to value-package order completion instead of wallet recharge when purpose is `value_package`.
- Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/controller/ldxp_topup.go`
  - Ensure public session response can be reused by value-package UI.
- Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/controller/value_package.go`
  - User-facing value-package APIs.
- Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/router/api-router.go`
  - Register the user routes under `/api/value-packages/*`; admin configuration continues to use the existing `/api/subscription/admin/plans` routes with the new value-package fields.
- Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/controller/value_package_test.go`
  - Cover API validation, purchase intent, LDXP session creation, activation/deactivation.
- Extend existing LDXP tests:
  - `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/ldxp_topup_test.go`
  - `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/service/ldxp_session_test.go`
  - `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/service/ldxp_verify_test.go`
  - `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/controller/ldxp_topup_test.go`

### Backend relay and limits

- Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/middleware/value_package.go`
  - Middleware that runs after auth and before `Distribute` to force using group when a package is enabled.
- Modify relay routers:
  - `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/router/relay-router.go`
  - `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/router/video-router.go`
- Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/service/funding_source.go`
  - Add package-aware subscription funding fields.
- Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/service/billing_session.go`
  - Use active value package before wallet/subscription preference when the package switch is enabled and record active package metadata in `RelayInfo`.
- Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/service/billing_session_test.go`
  - Prove an enabled value package bills the selected package subscription and does not consume wallet quota, even when `billing_preference` is `wallet_only`.
- Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/relay/common/relay_info.go`
  - Add value-package metadata fields for logging/settlement.
- Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/middleware/value_package_test.go`
  - Cover group forcing and explicit group override rejection.

### Backend VIP one-time ceremony

- Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/dto/user_settings.go`
  - Add `VipUpgradeModalSeen bool`.
- Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/controller/user.go`
  - Add a narrow endpoint for marking the VIP modal seen.
- No change to `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/user.go` is planned; `GetSelf` already returns the raw `setting` JSON and the new flag lives inside that JSON.
- Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/controller/user_vip_modal_test.go` for the new setting flag.

### Default frontend

- Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/value-packages/` files:
  - `api.ts`
  - `types.ts`
  - `index.tsx`
  - `hooks/use-value-packages.ts`
  - `lib/rules.ts`
  - `lib/rules.test.ts`
  - `components/value-package-card.tsx`
  - `components/value-package-payment-dialog.tsx`
  - `components/value-package-status-banner.tsx`
  - `components/value-package-source.test.ts`
- Create route `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/routes/_authenticated/value-packages/index.tsx`.
- Modify navigation:
  - `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/hooks/sidebar-data-model.ts`
  - `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/hooks/use-sidebar-data.ts`
  - `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/hooks/sidebar-data-model.test.ts`
- Modify wallet:
  - `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/wallet/index.tsx`
  - Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/wallet/components/value-packages-entry-card.tsx`
  - Add source test for wallet ordering.
- Modify admin subscriptions:
  - `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/subscriptions/types.ts`
  - `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/subscriptions/lib/plan-form.ts`
  - `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/subscriptions/constants.ts`
  - `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/subscriptions/index.tsx`
  - Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/subscriptions/components/value-package-admin-cards.tsx`
  - Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/subscriptions/components/subscriptions-mutate-drawer.tsx`
- Global glow and VIP ceremony:
  - Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/app-effects/global-entitlement-effects.tsx`
  - Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/app-effects/vip-upgrade-dialog.tsx`
  - Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/components/layout/components/authenticated-layout.tsx`
  - Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/styles/index.css`
- I18n:
  - Run `cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default && bun run i18n:sync` after adding text.
  - Then fill all generated keys in `web/default/src/i18n/locales/{en,zh,fr,ja,ru,vi}.json`.

---

## Task 1: Add value-package schema fields and migrations

**Files:**
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/subscription.go`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/main.go`
- Create: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/value_package_migration_test.go`

- [ ] **Step 1: Write failing migration/model tests**

Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/value_package_migration_test.go` with tests that assert the new fields persist and SQLite old-table migration adds columns.

```go
package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupValuePackageMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := DB
	oldLogDB := LOG_DB
	oldRedisEnabled := common.RedisEnabled
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL

	common.RedisEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	initCol()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	LOG_DB = db

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		DB = oldDB
		LOG_DB = oldLogDB
		common.RedisEnabled = oldRedisEnabled
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		initCol()
	})

	return db
}

func TestValuePackagePlanFieldsPersist(t *testing.T) {
	setupValuePackageMigrationTestDB(t)
	require.NoError(t, DB.AutoMigrate(&SubscriptionPlan{}))

	plan := &SubscriptionPlan{
		Title:                 "日卡",
		PriceAmount:           9.90,
		Currency:              "USD",
		DurationUnit:          SubscriptionDurationDay,
		DurationValue:         1,
		Enabled:               true,
		PlanKind:              SubscriptionPlanKindValuePackage,
		PackageType:           ValuePackageTypeDay,
		PackageLevel:          ValuePackageLevelDay,
		ModelGroup:            "day-card",
		ConcurrencyLimit:      1,
		Limit5hAmount:         1000,
		Limit7dAmount:         7000,
		Benefits:              "[\"5小时额度\",\"7天额度\"]",
		LdxpProductUrl:        "https://ldxp.example.test/day",
		LdxpProductName:       "日卡商品",
		LdxpProductAmount:     9.90,
		LdxpProductRef:        "day-card-prod",
		LdxpSessionTTLSeconds: 1800,
	}
	require.NoError(t, DB.Create(plan).Error)

	var got SubscriptionPlan
	require.NoError(t, DB.First(&got, plan.Id).Error)
	require.Equal(t, SubscriptionPlanKindValuePackage, got.PlanKind)
	require.Equal(t, ValuePackageTypeDay, got.PackageType)
	require.Equal(t, ValuePackageLevelDay, got.PackageLevel)
	require.Equal(t, "day-card", got.ModelGroup)
	require.Equal(t, 1, got.ConcurrencyLimit)
	require.EqualValues(t, 1000, got.Limit5hAmount)
	require.EqualValues(t, 7000, got.Limit7dAmount)
	require.Equal(t, "[\"5小时额度\",\"7天额度\"]", got.Benefits)
	require.Equal(t, "https://ldxp.example.test/day", got.LdxpProductUrl)
	require.Equal(t, "日卡商品", got.LdxpProductName)
	require.Equal(t, 9.90, got.LdxpProductAmount)
	require.Equal(t, "day-card-prod", got.LdxpProductRef)
	require.EqualValues(t, 1800, got.LdxpSessionTTLSeconds)
}

func TestEnsureSubscriptionPlanTableSQLiteAddsValuePackageColumns(t *testing.T) {
	setupValuePackageMigrationTestDB(t)
	require.NoError(t, DB.Exec("CREATE TABLE `subscription_plans` (`id` integer, `title` varchar(128) NOT NULL, PRIMARY KEY (`id`))").Error)

	require.NoError(t, ensureSubscriptionPlanTableSQLite())

	for _, col := range []string{
		"plan_kind",
		"package_type",
		"package_level",
		"model_group",
		"concurrency_limit",
		"limit_5h_amount",
		"limit_7d_amount",
		"benefits",
		"ldxp_product_url",
		"ldxp_product_name",
		"ldxp_product_amount",
		"ldxp_product_ref",
		"ldxp_session_ttl_seconds",
	} {
		require.True(t, DB.Migrator().HasColumn(&SubscriptionPlan{}, col), "missing column %s", col)
	}
}

func TestValuePackageNewTablesMigrate(t *testing.T) {
	setupValuePackageMigrationTestDB(t)
	require.NoError(t, DB.AutoMigrate(&UserValuePackagePreference{}, &ValuePackageUsageRecord{}))
	require.True(t, DB.Migrator().HasTable(&UserValuePackagePreference{}))
	require.True(t, DB.Migrator().HasTable(&ValuePackageUsageRecord{}))
}
```

- [ ] **Step 2: Run tests and verify they fail for missing fields**

Run:

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
go test ./model -run 'TestValuePackagePlanFieldsPersist|TestEnsureSubscriptionPlanTableSQLiteAddsValuePackageColumns|TestValuePackageNewTablesMigrate' -count=1
```

Expected: FAIL with compile errors such as `undefined: SubscriptionPlanKindValuePackage` or missing `SubscriptionPlan` fields.

- [ ] **Step 3: Add model fields and constants**

Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/subscription.go`.

Add constants near existing subscription constants:

```go
const (
	SubscriptionPlanKindSubscription = "subscription"
	SubscriptionPlanKindValuePackage = "value_package"
)

const (
	ValuePackageTypeDay   = "day"
	ValuePackageTypeWeek  = "week"
	ValuePackageTypeMonth = "month"
)

const (
	ValuePackageLevelDay   = 1
	ValuePackageLevelWeek  = 2
	ValuePackageLevelMonth = 3
)

const (
	UserSubscriptionStatusActive    = "active"
	UserSubscriptionStatusExpired   = "expired"
	UserSubscriptionStatusCancelled = "cancelled"
	UserSubscriptionStatusCovered   = "covered"
)
```

Extend `SubscriptionPlan`:

```go
	PlanKind     string `json:"plan_kind" gorm:"type:varchar(32);not null;default:'subscription'"`
	PackageType  string `json:"package_type" gorm:"type:varchar(16);default:''"`
	PackageLevel int    `json:"package_level" gorm:"type:int;default:0"`

	ModelGroup       string `json:"model_group" gorm:"type:varchar(64);default:''"`
	ConcurrencyLimit int    `json:"concurrency_limit" gorm:"type:int;default:1"`
	Limit5hAmount    int64  `json:"limit_5h_amount" gorm:"type:bigint;not null;default:0"`
	Limit7dAmount    int64  `json:"limit_7d_amount" gorm:"type:bigint;not null;default:0"`
	Benefits         string `json:"benefits" gorm:"type:text"`

	LdxpProductUrl        string  `json:"ldxp_product_url" gorm:"type:text"`
	LdxpProductName       string  `json:"ldxp_product_name" gorm:"type:text"`
	LdxpProductAmount     float64 `json:"ldxp_product_amount" gorm:"type:decimal(10,6);not null;default:0"`
	LdxpProductRef        string  `json:"ldxp_product_ref" gorm:"type:varchar(128);default:''"`
	LdxpSessionTTLSeconds int64   `json:"ldxp_session_ttl_seconds" gorm:"type:bigint;not null;default:0"`
```

Extend `UserSubscription`:

```go
	CoveredBySubscriptionId int   `json:"covered_by_subscription_id" gorm:"type:int;default:0"`
	CoveredTime             int64 `json:"covered_time" gorm:"type:bigint;default:0"`
```

Add new structs after `UserSubscription`:

```go
type UserValuePackagePreference struct {
	Id                       int  `json:"id"`
	UserId                   int  `json:"user_id" gorm:"uniqueIndex"`
	Enabled                  bool `json:"enabled" gorm:"default:false"`
	ActiveUserSubscriptionId int  `json:"active_user_subscription_id" gorm:"index;default:0"`
	CreatedAt                int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt                int64 `json:"updated_at" gorm:"bigint"`
}

func (p *UserValuePackagePreference) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	p.CreatedAt = now
	p.UpdatedAt = now
	return nil
}

func (p *UserValuePackagePreference) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = common.GetTimestamp()
	return nil
}

type ValuePackageUsageRecord struct {
	Id                 int    `json:"id"`
	UserId             int    `json:"user_id" gorm:"index:idx_vp_usage_user_time,priority:1"`
	UserSubscriptionId int    `json:"user_subscription_id" gorm:"index"`
	PlanId             int    `json:"plan_id" gorm:"index"`
	PackageType        string `json:"package_type" gorm:"type:varchar(16);index"`
	ModelGroup         string `json:"model_group" gorm:"type:varchar(64);index"`
	RequestId          string `json:"request_id" gorm:"type:varchar(64);index"`
	Quota              int64  `json:"quota" gorm:"type:bigint;not null;default:0"`
	CreatedAt          int64  `json:"created_at" gorm:"bigint;index:idx_vp_usage_user_time,priority:2"`
}

func (r *ValuePackageUsageRecord) BeforeCreate(tx *gorm.DB) error {
	if r.CreatedAt == 0 {
		r.CreatedAt = common.GetTimestamp()
	}
	return nil
}
```

Update `NormalizeDefaults()`:

```go
	if strings.TrimSpace(p.PlanKind) == "" {
		p.PlanKind = SubscriptionPlanKindSubscription
	}
	if p.ConcurrencyLimit <= 0 {
		p.ConcurrencyLimit = 1
	}
```

- [ ] **Step 4: Add migrations**

Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/main.go`.

Add `&UserValuePackagePreference{}` and `&ValuePackageUsageRecord{}` to both `AutoMigrate` model lists where `UserSubscription` and `SubscriptionPreConsumeRecord` are currently migrated.

Extend the SQLite `CREATE TABLE subscription_plans` DDL with:

```sql
`plan_kind` varchar(32) NOT NULL DEFAULT 'subscription',
`package_type` varchar(16) DEFAULT '',
`package_level` integer DEFAULT 0,
`model_group` varchar(64) DEFAULT '',
`concurrency_limit` integer DEFAULT 1,
`limit_5h_amount` bigint NOT NULL DEFAULT 0,
`limit_7d_amount` bigint NOT NULL DEFAULT 0,
`benefits` text,
`ldxp_product_url` text,
`ldxp_product_name` text,
`ldxp_product_amount` decimal(10,6) NOT NULL DEFAULT 0,
`ldxp_product_ref` varchar(128) DEFAULT '',
`ldxp_session_ttl_seconds` bigint NOT NULL DEFAULT 0,
```

Add matching entries to `required := []sqliteColumnDef{...}`:

```go
{Name: "plan_kind", DDL: "`plan_kind` varchar(32) NOT NULL DEFAULT 'subscription'"},
{Name: "package_type", DDL: "`package_type` varchar(16) DEFAULT ''"},
{Name: "package_level", DDL: "`package_level` integer DEFAULT 0"},
{Name: "model_group", DDL: "`model_group` varchar(64) DEFAULT ''"},
{Name: "concurrency_limit", DDL: "`concurrency_limit` integer DEFAULT 1"},
{Name: "limit_5h_amount", DDL: "`limit_5h_amount` bigint NOT NULL DEFAULT 0"},
{Name: "limit_7d_amount", DDL: "`limit_7d_amount` bigint NOT NULL DEFAULT 0"},
{Name: "benefits", DDL: "`benefits` text"},
{Name: "ldxp_product_url", DDL: "`ldxp_product_url` text"},
{Name: "ldxp_product_name", DDL: "`ldxp_product_name` text"},
{Name: "ldxp_product_amount", DDL: "`ldxp_product_amount` decimal(10,6) NOT NULL DEFAULT 0"},
{Name: "ldxp_product_ref", DDL: "`ldxp_product_ref` varchar(128) DEFAULT ''"},
{Name: "ldxp_session_ttl_seconds", DDL: "`ldxp_session_ttl_seconds` bigint NOT NULL DEFAULT 0"},
```

Add a focused `ensureUserSubscriptionTableSQLite()` helper in `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/main.go` mirroring `ensureSubscriptionPlanTableSQLite()`. It should add `covered_by_subscription_id` and `covered_time` with `ALTER TABLE ... ADD COLUMN` for existing SQLite databases. Call it in both `migrateDB()` and `migrateDBFast()` immediately after `ensureSubscriptionPlanTableSQLite()`.

- [ ] **Step 5: Run migration tests**

Run:

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
go test ./model -run 'TestValuePackagePlanFieldsPersist|TestEnsureSubscriptionPlanTableSQLiteAddsValuePackageColumns|TestValuePackageNewTablesMigrate' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
git add model/subscription.go model/main.go model/value_package_migration_test.go
git commit -m "feat: add value package schema"
```

---

## Task 2: Implement value-package purchase rules, preferences, and usage windows

**Files:**
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/subscription.go`
- Create: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/value_package_test.go`

- [ ] **Step 1: Write failing purchase-rule tests**

Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/value_package_test.go`.

```go
package model

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupValuePackageTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := DB
	oldLogDB := LOG_DB
	oldRedisEnabled := common.RedisEnabled
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL

	common.RedisEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	initCol()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	LOG_DB = db
	require.NoError(t, db.AutoMigrate(&User{}, &TopUp{}, &SubscriptionPlan{}, &SubscriptionOrder{}, &UserSubscription{}, &UserValuePackagePreference{}, &ValuePackageUsageRecord{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		DB = oldDB
		LOG_DB = oldLogDB
		common.RedisEnabled = oldRedisEnabled
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		initCol()
	})
	return db
}

func createValuePackageUser(t *testing.T, id int, group string) User {
	t.Helper()
	user := User{Id: id, Username: fmt.Sprintf("vp-user-%d", id), Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: group, AffCode: fmt.Sprintf("vp-aff-%d", id)}
	require.NoError(t, DB.Create(&user).Error)
	return user
}

func createValuePackagePlan(t *testing.T, packageType string, level int, durationDays int, price float64) SubscriptionPlan {
	t.Helper()
	plan := SubscriptionPlan{
		Title:             packageType + " card",
		PriceAmount:       price,
		Currency:          "USD",
		DurationUnit:      SubscriptionDurationDay,
		DurationValue:     durationDays,
		Enabled:           true,
		PlanKind:          SubscriptionPlanKindValuePackage,
		PackageType:       packageType,
		PackageLevel:      level,
		ModelGroup:        packageType + "-card",
		ConcurrencyLimit:  1,
		Limit5hAmount:     1000,
		Limit7dAmount:     5000,
		LdxpProductUrl:    "https://ldxp.example.test/" + packageType,
		LdxpProductName:   packageType + " product",
		LdxpProductAmount: price,
	}
	require.NoError(t, DB.Create(&plan).Error)
	return plan
}

func createActiveValuePackageSub(t *testing.T, userID int, plan SubscriptionPlan, start int64, end int64) UserSubscription {
	t.Helper()
	sub := UserSubscription{UserId: userID, PlanId: plan.Id, AmountTotal: plan.TotalAmount, StartTime: start, EndTime: end, Status: UserSubscriptionStatusActive, Source: "test"}
	require.NoError(t, DB.Create(&sub).Error)
	return sub
}

func TestValuePackagePurchaseIntentSameLevelExtends(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3001, UserGroupTiyan)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	now := common.GetTimestamp()
	createActiveValuePackageSub(t, user.Id, month, now-100, now+20*86400)

	intent, err := CheckValuePackagePurchaseIntent(user.Id, month.Id, false)

	require.NoError(t, err)
	require.Equal(t, ValuePackagePurchaseActionExtend, intent.Action)
	require.False(t, intent.RequiresConfirmation)
}

func TestValuePackagePurchaseIntentUpgradeRequiresConfirmation(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3002, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	now := common.GetTimestamp()
	createActiveValuePackageSub(t, user.Id, day, now-100, now+3600)

	intent, err := CheckValuePackagePurchaseIntent(user.Id, month.Id, false)

	require.NoError(t, err)
	require.Equal(t, ValuePackagePurchaseActionUpgrade, intent.Action)
	require.True(t, intent.RequiresConfirmation)

	intent, err = CheckValuePackagePurchaseIntent(user.Id, month.Id, true)
	require.NoError(t, err)
	require.Equal(t, ValuePackagePurchaseActionUpgrade, intent.Action)
	require.False(t, intent.RequiresConfirmation)
}

func TestValuePackagePurchaseIntentDowngradeRejected(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3003, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	now := common.GetTimestamp()
	createActiveValuePackageSub(t, user.Id, month, now-100, now+3600)

	_, err := CheckValuePackagePurchaseIntent(user.Id, day.Id, false)

	require.Error(t, err)
	require.Contains(t, err.Error(), "更高等级套餐")
}

func TestCompleteValuePackagePurchaseExtendsSameLevelWithoutChangingUserGroup(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3004, UserGroupTiyan)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
	now := common.GetTimestamp()
	existing := createActiveValuePackageSub(t, user.Id, month, now-100, now+20*86400)
	order := SubscriptionOrder{UserId: user.Id, PlanId: month.Id, Money: month.PriceAmount, TradeNo: "vp-extend-order", PaymentMethod: PaymentMethodLDXP, PaymentProvider: PaymentProviderLDXP, Status: common.TopUpStatusPending, CreateTime: now}
	require.NoError(t, DB.Create(&order).Error)

	completed, err := CompleteValuePackageOrder("vp-extend-order", "payload", PaymentProviderLDXP, PaymentMethodLDXP, true)

	require.NoError(t, err)
	require.Equal(t, existing.Id, completed.Id)
	require.GreaterOrEqual(t, completed.EndTime, existing.EndTime+29*86400)
	var reloaded User
	require.NoError(t, DB.First(&reloaded, user.Id).Error)
	require.Equal(t, UserGroupTiyan, reloaded.Group)
}

func TestCompleteValuePackagePurchaseCoversLowerPlanAndCountsVIPTopup(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3005, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 30)
	now := common.GetTimestamp()
	lower := createActiveValuePackageSub(t, user.Id, day, now-100, now+3600)
	order := SubscriptionOrder{UserId: user.Id, PlanId: month.Id, Money: month.PriceAmount, TradeNo: "vp-upgrade-order", PaymentMethod: PaymentMethodLDXP, PaymentProvider: PaymentProviderLDXP, Status: common.TopUpStatusPending, CreateTime: now}
	require.NoError(t, DB.Create(&order).Error)

	completed, err := CompleteValuePackageOrder("vp-upgrade-order", "payload", PaymentProviderLDXP, PaymentMethodLDXP, true)

	require.NoError(t, err)
	require.NotEqual(t, lower.Id, completed.Id)
	var covered UserSubscription
	require.NoError(t, DB.First(&covered, lower.Id).Error)
	require.Equal(t, UserSubscriptionStatusCovered, covered.Status)
	require.Equal(t, completed.Id, covered.CoveredBySubscriptionId)
	var reloaded User
	require.NoError(t, DB.First(&reloaded, user.Id).Error)
	require.Equal(t, UserGroupVIP, reloaded.Group)
	var topUp TopUp
	require.NoError(t, DB.Where("trade_no = ?", "vp-upgrade-order").First(&topUp).Error)
	require.EqualValues(t, 0, topUp.Amount, "value package payment must not add wallet balance")
	require.Equal(t, 30.0, topUp.Money)
}

func TestActivateAndDeactivateValuePackageDoesNotStopClock(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3006, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-100, now+3600)

	state, err := ActivateValuePackage(user.Id, sub.Id)
	require.NoError(t, err)
	require.True(t, state.Preference.Enabled)
	require.Equal(t, sub.Id, state.Preference.ActiveUserSubscriptionId)

	state, err = DeactivateValuePackage(user.Id)
	require.NoError(t, err)
	require.False(t, state.Preference.Enabled)

	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, sub.Id).Error)
	require.Equal(t, sub.EndTime, reloaded.EndTime)
}

func TestValuePackageRollingUsageWindows(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3007, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-100, now+3600)

	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: ValuePackageTypeDay, ModelGroup: "day-card", RequestId: "old", Quota: 900, CreatedAt: now - int64(6*time.Hour/time.Second)}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: ValuePackageTypeDay, ModelGroup: "day-card", RequestId: "recent", Quota: 900, CreatedAt: now - int64(time.Hour/time.Second)}))

	used5h, used7d, err := GetValuePackageWindowUsage(user.Id, sub.Id, now)
	require.NoError(t, err)
	require.EqualValues(t, 900, used5h)
	require.EqualValues(t, 1800, used7d)
}
```

- [ ] **Step 2: Run tests and verify they fail**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
go test ./model -run 'TestValuePackagePurchaseIntent|TestCompleteValuePackagePurchase|TestActivateAndDeactivateValuePackage|TestValuePackageRollingUsageWindows' -count=1
```

Expected: FAIL with undefined functions/constants.

- [ ] **Step 3: Add purchase-rule functions**

Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/subscription.go` and add after `buildSubscriptionSummaries`:

```go
const (
	ValuePackagePurchaseActionCreate  = "create"
	ValuePackagePurchaseActionExtend  = "extend"
	ValuePackagePurchaseActionUpgrade = "upgrade"
)

type ValuePackagePurchaseIntent struct {
	Action               string `json:"action"`
	RequiresConfirmation bool   `json:"requires_confirmation"`
	CurrentSubscription  *UserSubscription `json:"current_subscription,omitempty"`
	CurrentPlan          *SubscriptionPlan `json:"current_plan,omitempty"`
	TargetPlan           *SubscriptionPlan `json:"target_plan,omitempty"`
	Message              string `json:"message,omitempty"`
}

func (p *SubscriptionPlan) IsValuePackage() bool {
	return strings.TrimSpace(p.PlanKind) == SubscriptionPlanKindValuePackage
}

func normalizeValuePackagePlan(plan *SubscriptionPlan) {
	if plan == nil {
		return
	}
	plan.PlanKind = strings.TrimSpace(plan.PlanKind)
	if plan.PlanKind == "" {
		plan.PlanKind = SubscriptionPlanKindSubscription
	}
	plan.PackageType = strings.TrimSpace(plan.PackageType)
	if plan.PackageLevel <= 0 {
		switch plan.PackageType {
		case ValuePackageTypeDay:
			plan.PackageLevel = ValuePackageLevelDay
		case ValuePackageTypeWeek:
			plan.PackageLevel = ValuePackageLevelWeek
		case ValuePackageTypeMonth:
			plan.PackageLevel = ValuePackageLevelMonth
		}
	}
	if plan.ConcurrencyLimit <= 0 {
		plan.ConcurrencyLimit = 1
	}
}

func getActiveValuePackageSubscriptionsTx(tx *gorm.DB, userId int, now int64) ([]UserSubscription, error) {
	if tx == nil {
		tx = DB
	}
	var subs []UserSubscription
	err := tx.Where("user_id = ? AND status = ? AND end_time > ?", userId, UserSubscriptionStatusActive, now).
		Order("end_time desc, id desc").
		Find(&subs).Error
	if err != nil {
		return nil, err
	}
	out := make([]UserSubscription, 0, len(subs))
	for _, sub := range subs {
		plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
		if err != nil {
			return nil, err
		}
		normalizeValuePackagePlan(plan)
		if plan.IsValuePackage() {
			out = append(out, sub)
		}
	}
	return out, nil
}

func getHighestActiveValuePackageTx(tx *gorm.DB, userId int, now int64) (*UserSubscription, *SubscriptionPlan, error) {
	subs, err := getActiveValuePackageSubscriptionsTx(tx, userId, now)
	if err != nil {
		return nil, nil, err
	}
	var bestSub *UserSubscription
	var bestPlan *SubscriptionPlan
	for _, sub := range subs {
		plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
		if err != nil {
			return nil, nil, err
		}
		normalizeValuePackagePlan(plan)
		if bestPlan == nil || plan.PackageLevel > bestPlan.PackageLevel || (plan.PackageLevel == bestPlan.PackageLevel && sub.EndTime > bestSub.EndTime) {
			subCopy := sub
			bestSub = &subCopy
			bestPlan = plan
		}
	}
	return bestSub, bestPlan, nil
}

func CheckValuePackagePurchaseIntent(userId int, planId int, confirmedCover bool) (*ValuePackagePurchaseIntent, error) {
	if userId <= 0 || planId <= 0 {
		return nil, errors.New("invalid userId or planId")
	}
	plan, err := GetSubscriptionPlanById(planId)
	if err != nil {
		return nil, err
	}
	normalizeValuePackagePlan(plan)
	if !plan.IsValuePackage() {
		return nil, errors.New("目标套餐不是超值套餐")
	}
	if !plan.Enabled {
		return nil, errors.New("套餐未启用")
	}
	now := GetDBTimestamp()
	currentSub, currentPlan, err := getHighestActiveValuePackageTx(nil, userId, now)
	if err != nil {
		return nil, err
	}
	intent := &ValuePackagePurchaseIntent{Action: ValuePackagePurchaseActionCreate, TargetPlan: plan}
	if currentSub == nil || currentPlan == nil {
		return intent, nil
	}
	intent.CurrentSubscription = currentSub
	intent.CurrentPlan = currentPlan
	if plan.PackageLevel == currentPlan.PackageLevel {
		intent.Action = ValuePackagePurchaseActionExtend
		return intent, nil
	}
	if plan.PackageLevel < currentPlan.PackageLevel {
		return nil, errors.New("当前已有更高等级套餐未过期，暂不能购买低等级套餐")
	}
	intent.Action = ValuePackagePurchaseActionUpgrade
	if !confirmedCover {
		intent.RequiresConfirmation = true
		intent.Message = fmt.Sprintf("购买 %s 将直接覆盖当前 %s，剩余时间不会折算或顺延", plan.Title, currentPlan.Title)
	}
	return intent, nil
}
```

- [ ] **Step 4: Add complete order, activation, and usage functions**

Add these functions to `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/subscription.go`:

```go
type ValuePackageState struct {
	Preference   UserValuePackagePreference `json:"preference"`
	Subscription *UserSubscription          `json:"subscription,omitempty"`
	Plan         *SubscriptionPlan          `json:"plan,omitempty"`
}

func CompleteValuePackageOrder(tradeNo string, providerPayload string, expectedPaymentProvider string, actualPaymentMethod string, confirmedCover bool) (*UserSubscription, error) {
	if strings.TrimSpace(tradeNo) == "" {
		return nil, errors.New("tradeNo is empty")
	}
	var completed *UserSubscription
	var vipUpgraded bool
	var userId int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var order SubscriptionOrder
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
			return ErrSubscriptionOrderNotFound
		}
		if expectedPaymentProvider != "" && order.PaymentProvider != expectedPaymentProvider {
			return ErrPaymentMethodMismatch
		}
		if order.Status == common.TopUpStatusSuccess {
			var sub UserSubscription
			if err := tx.Where("user_id = ? AND plan_id = ?", order.UserId, order.PlanId).Order("end_time desc, id desc").First(&sub).Error; err == nil {
				completed = &sub
			}
			return nil
		}
		if order.Status != common.TopUpStatusPending {
			return ErrSubscriptionOrderStatusInvalid
		}
		plan, err := getSubscriptionPlanByIdTx(tx, order.PlanId)
		if err != nil {
			return err
		}
		normalizeValuePackagePlan(plan)
		if !plan.IsValuePackage() {
			return errors.New("order plan is not value package")
		}
		intent, err := checkValuePackagePurchaseIntentTx(tx, order.UserId, plan, confirmedCover)
		if err != nil {
			return err
		}
		if intent.RequiresConfirmation {
			return errors.New("购买高级套餐需要确认覆盖当前低级套餐")
		}
		nowUnix := GetDBTimestamp()
		start := time.Unix(nowUnix, 0)
		endUnix, err := calcPlanEndTime(start, plan)
		if err != nil {
			return err
		}
		switch intent.Action {
		case ValuePackagePurchaseActionExtend:
			var existing UserSubscription
			if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ?", intent.CurrentSubscription.Id).First(&existing).Error; err != nil {
				return err
			}
			base := existing.EndTime
			if base < nowUnix {
				base = nowUnix
			}
			duration := endUnix - nowUnix
			existing.EndTime = base + duration
			if err := tx.Save(&existing).Error; err != nil {
				return err
			}
			completed = &existing
		case ValuePackagePurchaseActionUpgrade:
			if intent.CurrentSubscription != nil {
				if err := tx.Model(&UserSubscription{}).Where("id = ?", intent.CurrentSubscription.Id).Updates(map[string]interface{}{
					"status":       UserSubscriptionStatusCovered,
					"covered_time": nowUnix,
					"updated_at":   common.GetTimestamp(),
				}).Error; err != nil {
					return err
				}
			}
			fallthrough
		case ValuePackagePurchaseActionCreate:
			sub := &UserSubscription{UserId: order.UserId, PlanId: plan.Id, AmountTotal: plan.TotalAmount, AmountUsed: 0, StartTime: nowUnix, EndTime: endUnix, Status: UserSubscriptionStatusActive, Source: "ldxp", CreatedAt: common.GetTimestamp(), UpdatedAt: common.GetTimestamp()}
			if err := tx.Create(sub).Error; err != nil {
				return err
			}
			completed = sub
			if intent.Action == ValuePackagePurchaseActionUpgrade && intent.CurrentSubscription != nil {
				if err := tx.Model(&UserSubscription{}).Where("id = ?", intent.CurrentSubscription.Id).Update("covered_by_subscription_id", sub.Id).Error; err != nil {
					return err
				}
			}
		default:
			return errors.New("unknown value package purchase action")
		}
		if err := upsertSubscriptionTopUpTx(tx, &order); err != nil {
			return err
		}
		vipUpgraded, err = MaybeUpgradeUserToVIPTx(tx, order.UserId)
		if err != nil {
			return err
		}
		order.Status = common.TopUpStatusSuccess
		order.CompleteTime = common.GetTimestamp()
		if providerPayload != "" {
			order.ProviderPayload = providerPayload
		}
		if actualPaymentMethod != "" {
			order.PaymentMethod = actualPaymentMethod
		}
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		userId = order.UserId
		return nil
	})
	if err != nil {
		return nil, err
	}
	if vipUpgraded && userId > 0 {
		_ = UpdateUserGroupCache(userId, UserGroupVIP)
	}
	return completed, nil
}

func checkValuePackagePurchaseIntentTx(tx *gorm.DB, userId int, plan *SubscriptionPlan, confirmedCover bool) (*ValuePackagePurchaseIntent, error) {
	if tx == nil {
		tx = DB
	}
	now := GetDBTimestamp()
	currentSub, currentPlan, err := getHighestActiveValuePackageTx(tx, userId, now)
	if err != nil {
		return nil, err
	}
	intent := &ValuePackagePurchaseIntent{Action: ValuePackagePurchaseActionCreate, TargetPlan: plan}
	if currentSub == nil || currentPlan == nil {
		return intent, nil
	}
	intent.CurrentSubscription = currentSub
	intent.CurrentPlan = currentPlan
	if plan.PackageLevel == currentPlan.PackageLevel {
		intent.Action = ValuePackagePurchaseActionExtend
		return intent, nil
	}
	if plan.PackageLevel < currentPlan.PackageLevel {
		return nil, errors.New("当前已有更高等级套餐未过期，暂不能购买低等级套餐")
	}
	intent.Action = ValuePackagePurchaseActionUpgrade
	if !confirmedCover {
		intent.RequiresConfirmation = true
		intent.Message = fmt.Sprintf("购买 %s 将直接覆盖当前 %s，剩余时间不会折算或顺延", plan.Title, currentPlan.Title)
	}
	return intent, nil
}

func ActivateValuePackage(userId int, userSubscriptionId int) (*ValuePackageState, error) {
	if userId <= 0 || userSubscriptionId <= 0 {
		return nil, errors.New("invalid activation args")
	}
	now := GetDBTimestamp()
	var state *ValuePackageState
	err := DB.Transaction(func(tx *gorm.DB) error {
		var sub UserSubscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ? AND user_id = ? AND status = ? AND end_time > ?", userSubscriptionId, userId, UserSubscriptionStatusActive, now).First(&sub).Error; err != nil {
			return err
		}
		plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
		if err != nil {
			return err
		}
		normalizeValuePackagePlan(plan)
		if !plan.IsValuePackage() {
			return errors.New("订阅不是超值套餐")
		}
		pref, err := upsertValuePackagePreferenceTx(tx, userId, true, sub.Id)
		if err != nil {
			return err
		}
		state = &ValuePackageState{Preference: *pref, Subscription: &sub, Plan: plan}
		return nil
	})
	return state, err
}

func DeactivateValuePackage(userId int) (*ValuePackageState, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	var state *ValuePackageState
	err := DB.Transaction(func(tx *gorm.DB) error {
		pref, err := upsertValuePackagePreferenceTx(tx, userId, false, 0)
		if err != nil {
			return err
		}
		state = &ValuePackageState{Preference: *pref}
		return nil
	})
	return state, err
}

func upsertValuePackagePreferenceTx(tx *gorm.DB, userId int, enabled bool, activeSubId int) (*UserValuePackagePreference, error) {
	var pref UserValuePackagePreference
	q := tx.Where("user_id = ?", userId).First(&pref)
	if errors.Is(q.Error, gorm.ErrRecordNotFound) {
		pref = UserValuePackagePreference{UserId: userId, Enabled: enabled, ActiveUserSubscriptionId: activeSubId}
		return &pref, tx.Create(&pref).Error
	}
	if q.Error != nil {
		return nil, q.Error
	}
	pref.Enabled = enabled
	if activeSubId > 0 || !enabled {
		pref.ActiveUserSubscriptionId = activeSubId
	}
	return &pref, tx.Save(&pref).Error
}

func RecordValuePackageUsage(record *ValuePackageUsageRecord) error {
	if record == nil || record.UserId <= 0 || record.UserSubscriptionId <= 0 || record.Quota <= 0 {
		return errors.New("invalid value package usage record")
	}
	return DB.Create(record).Error
}

func GetValuePackageWindowUsage(userId int, userSubscriptionId int, now int64) (int64, int64, error) {
	if now <= 0 {
		now = GetDBTimestamp()
	}
	var used5h int64
	if err := DB.Model(&ValuePackageUsageRecord{}).
		Where("user_id = ? AND user_subscription_id = ? AND created_at >= ?", userId, userSubscriptionId, now-5*3600).
		Select("COALESCE(SUM(quota), 0)").Scan(&used5h).Error; err != nil {
		return 0, 0, err
	}
	var used7d int64
	if err := DB.Model(&ValuePackageUsageRecord{}).
		Where("user_id = ? AND user_subscription_id = ? AND created_at >= ?", userId, userSubscriptionId, now-7*24*3600).
		Select("COALESCE(SUM(quota), 0)").Scan(&used7d).Error; err != nil {
		return 0, 0, err
	}
	return used5h, used7d, nil
}
```

- [ ] **Step 5: Run purchase-rule tests**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
go test ./model -run 'TestValuePackagePurchaseIntent|TestCompleteValuePackagePurchase|TestActivateAndDeactivateValuePackage|TestValuePackageRollingUsageWindows' -count=1
```

Expected: PASS.

- [ ] **Step 6: Run broader model subscription tests**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
go test ./model -run 'TestSubscription|TestValuePackage|TestMaybeUpgradeUserToVIP|TestRedeemPaidTopupAtThresholdUpgradesUserToVIP' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
git add model/subscription.go model/value_package_test.go
git commit -m "feat: add value package purchase rules"
```

---

## Task 3: Add LDXP value-package sessions and settlement branch

**Files:**
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/ldxp_topup.go`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/main.go`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/service/ldxp_session.go`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/service/ldxp_verify.go`
- Modify tests:
  - `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/ldxp_topup_test.go`
  - `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/service/ldxp_session_test.go`
  - `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/service/ldxp_verify_test.go`

- [ ] **Step 1: Write failing LDXP purpose tests**

Append to `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/ldxp_topup_test.go`:

```go
func TestLdxpTopupSessionPersistsValuePackagePurpose(t *testing.T) {
	setupLdxpTopupTest(t)

	session := &LdxpTopupSession{
		SessionId:           "ldxp-vp-session",
		UserId:              1001,
		Amount:              0,
		Money:               9.90,
		ProductUrl:          "https://example.test/value-package/day",
		ProductName:         "日卡",
		Status:              LdxpStatusCreated,
		Purpose:             LdxpPurposeValuePackage,
		SubscriptionOrderId: 7001,
		SubscriptionPlanId:  8001,
		CreatedTime:         100,
		UpdatedTime:         100,
		ExpiredTime:         200,
	}

	require.NoError(t, InsertLdxpTopupSession(session))
	persisted, err := GetLdxpTopupSessionBySessionId("ldxp-vp-session")
	require.NoError(t, err)
	assert.Equal(t, LdxpPurposeValuePackage, persisted.Purpose)
	assert.Equal(t, 7001, persisted.SubscriptionOrderId)
	assert.Equal(t, 8001, persisted.SubscriptionPlanId)
}
```

Append to `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/service/ldxp_session_test.go`:

```go
func TestCreateLdxpValuePackageSessionUsesPlanProductConfig(t *testing.T) {
	setupLdxpSessionServiceTest(t)
	insertLdxpUserForServiceTest(t, 1001)
	plan := model.SubscriptionPlan{
		Title:                 "日卡",
		PriceAmount:           9.9,
		Currency:              "USD",
		DurationUnit:          model.SubscriptionDurationDay,
		DurationValue:         1,
		Enabled:               true,
		PlanKind:              model.SubscriptionPlanKindValuePackage,
		PackageType:           model.ValuePackageTypeDay,
		PackageLevel:          model.ValuePackageLevelDay,
		ModelGroup:            "day-card",
		ConcurrencyLimit:      1,
		LdxpProductUrl:        "https://ldxp.example.test/day",
		LdxpProductName:       "日卡商品",
		LdxpProductAmount:     9.9,
		LdxpSessionTTLSeconds: 900,
	}
	require.NoError(t, model.DB.Create(&plan).Error)

	view, order, err := CreateLdxpValuePackageSession(1001, plan.Id, true, testLdxpSessionConfig(true))

	require.NoError(t, err)
	require.NotNil(t, view)
	require.NotNil(t, order)
	assert.Equal(t, 9.9, view.Money)
	assert.Equal(t, common.TopUpStatusPending, order.Status)
	persisted, err := model.GetLdxpTopupSessionBySessionId(view.SessionID)
	require.NoError(t, err)
	assert.Equal(t, model.LdxpPurposeValuePackage, persisted.Purpose)
	assert.Equal(t, order.Id, persisted.SubscriptionOrderId)
	assert.Equal(t, plan.Id, persisted.SubscriptionPlanId)
	assert.Equal(t, "https://ldxp.example.test/day", persisted.ProductUrl)
	assert.Equal(t, "日卡商品", persisted.ProductName)
	assert.EqualValues(t, persisted.CreatedTime+900, persisted.ExpiredTime)
}
```

Add a verify test in `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/service/ldxp_verify_test.go` following the setup style in that file. The test should create a value-package plan, pending `SubscriptionOrder`, LDXP session with `Purpose=LdxpPurposeValuePackage`, worker paid fields, then call `TryVerifyAndRedeemLdxpSession` and assert `UserSubscription` exists and user quota did not increase.

Use this exact assertion pattern in the new test body:

```go
result, err := TryVerifyAndRedeemLdxpSession(session.SessionId)
require.NoError(t, err)
require.True(t, result.Redeemed)
require.Equal(t, model.LdxpStatusSuccess, result.Status)

var subs []model.UserSubscription
require.NoError(t, model.DB.Where("user_id = ?", user.Id).Find(&subs).Error)
require.Len(t, subs, 1)
require.Equal(t, plan.Id, subs[0].PlanId)

var reloaded model.User
require.NoError(t, model.DB.First(&reloaded, user.Id).Error)
require.Equal(t, originalQuota, reloaded.Quota)
```

- [ ] **Step 2: Run LDXP tests and verify they fail**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
go test ./model ./service -run 'TestLdxpTopupSessionPersistsValuePackagePurpose|TestCreateLdxpValuePackageSessionUsesPlanProductConfig|Test.*ValuePackage.*Ldxp' -count=1
```

Expected: FAIL with undefined fields/functions.

- [ ] **Step 3: Add LDXP purpose fields and migration**

Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/ldxp_topup.go`:

Add constants:

```go
const (
	LdxpPurposeTopup        = "topup"
	LdxpPurposeValuePackage = "value_package"
)
```

Add fields to `LdxpTopupSession`:

```go
	Purpose             string `json:"purpose" gorm:"type:varchar(32);not null;default:'topup';index"`
	SubscriptionOrderId int    `json:"subscription_order_id" gorm:"index;default:0"`
	SubscriptionPlanId  int    `json:"subscription_plan_id" gorm:"index;default:0"`
	ConfirmedCover      bool   `json:"confirmed_cover" gorm:"default:false"`
```

Add a focused `ensureLdxpTopupSessionTableSQLite()` helper in `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/main.go` for existing SQLite databases. It should add `purpose`, `subscription_order_id`, `subscription_plan_id`, and `confirmed_cover` with `ALTER TABLE ... ADD COLUMN`; call it in both `migrateDB()` and `migrateDBFast()` after the existing SQLite subscription-table helpers.

- [ ] **Step 4: Add value-package session creation**

Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/service/ldxp_session.go`.

Add request type:

```go
type LdxpCreateValuePackageSessionRequest struct {
	PlanId         int  `json:"plan_id"`
	ConfirmedCover bool `json:"confirmed_cover"`
}
```

Add function:

```go
func CreateLdxpValuePackageSession(userID int, planID int, confirmedCover bool, cfg *LdxpConfig) (*LdxpSessionPublicView, *model.SubscriptionOrder, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("%w: missing config", ErrLdxpInvalidSessionRequest)
	}
	if !cfg.Enabled {
		return nil, nil, ErrLdxpTopupDisabled
	}
	if userID <= 0 || planID <= 0 {
		return nil, nil, fmt.Errorf("%w: invalid value package request", ErrLdxpInvalidSessionRequest)
	}
	plan, err := model.GetSubscriptionPlanById(planID)
	if err != nil {
		return nil, nil, err
	}
	if !plan.IsValuePackage() {
		return nil, nil, fmt.Errorf("%w: not value package", ErrLdxpInvalidSessionRequest)
	}
	if strings.TrimSpace(plan.LdxpProductUrl) == "" || strings.TrimSpace(plan.LdxpProductName) == "" || plan.LdxpProductAmount <= 0 {
		return nil, nil, fmt.Errorf("%w: ldxp product incomplete", ErrLdxpInvalidSessionRequest)
	}
	if _, err := model.CheckValuePackagePurchaseIntent(userID, planID, confirmedCover); err != nil {
		return nil, nil, err
	}

	now := common.GetTimestamp()
	ldxpCreateSessionMu.Lock()
	defer ldxpCreateSessionMu.Unlock()

	var session *model.LdxpTopupSession
	var order *model.SubscriptionOrder
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockLdxpUserRowIfPossible(tx, userID); err != nil {
			return err
		}
		if activeSession, err := findActiveLdxpValuePackageSessionForUserTx(tx, userID, now); err == nil {
			session = activeSession
			var existingOrder model.SubscriptionOrder
			if activeSession.SubscriptionOrderId > 0 {
				_ = tx.Where("id = ?", activeSession.SubscriptionOrderId).First(&existingOrder).Error
				order = &existingOrder
			}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		tradeNo := fmt.Sprintf("LDXP_VP-%d-%d-%s", userID, time.Now().UnixMilli(), common.GetRandomString(6))
		order = &model.SubscriptionOrder{UserId: userID, PlanId: plan.Id, Money: plan.LdxpProductAmount, TradeNo: tradeNo, PaymentMethod: model.PaymentMethodLDXP, PaymentProvider: model.PaymentProviderLDXP, CreateTime: now, Status: common.TopUpStatusPending}
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		sessionID, err := generateLdxpSessionID()
		if err != nil {
			return err
		}
		ttl := cfg.SessionTTLSeconds
		if plan.LdxpSessionTTLSeconds > 0 {
			ttl = plan.LdxpSessionTTLSeconds
		}
		session = &model.LdxpTopupSession{SessionId: sessionID, UserId: userID, Amount: 0, Money: plan.LdxpProductAmount, ProductUrl: plan.LdxpProductUrl, ProductName: plan.LdxpProductName, ContactEmail: cfg.ContactEmail, Status: model.LdxpStatusCreated, Purpose: model.LdxpPurposeValuePackage, SubscriptionOrderId: order.Id, SubscriptionPlanId: plan.Id, ConfirmedCover: confirmedCover, CreatedTime: now, UpdatedTime: now, ExpiredTime: now + ttl}
		return tx.Create(session).Error
	})
	if err != nil {
		return nil, nil, err
	}
	return publicLdxpSessionView(session), order, nil
}
```

Also add `findActiveLdxpValuePackageSessionForUserTx` next to `findActiveLdxpTopupSessionForUserTx`, filtering `purpose = value_package`. Update existing topup active session finder to filter `purpose = '' OR purpose = topup` so a package payment session does not block wallet recharge and vice versa.

- [ ] **Step 5: Branch LDXP verify settlement**

Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/service/ldxp_verify.go`.

In both paths after `persistLdxpVerifiedTx`, before direct topup or redemption, branch:

```go
if session.Purpose == model.LdxpPurposeValuePackage {
	if _, err := model.CompleteValuePackageOrder(ldxpValuePackageTradeNoTx(tx, session), "ldxp session verified", model.PaymentProviderLDXP, model.PaymentMethodLDXP, session.ConfirmedCover); err != nil {
		if updateErr := persistLdxpRedeemFailureTx(tx, session, strings.TrimSpace(err.Error())); updateErr != nil {
			return updateErr
		}
		result = &LdxpVerifyResult{Verified: true, Redeemed: false, Status: model.LdxpStatusRedeemFailed, ErrorCode: ldxpVerifyCodeRedeemFailed, ErrorMessage: strings.TrimSpace(err.Error())}
		return nil
	}
	if err := persistLdxpRedeemSuccessTx(tx, session); err != nil {
		return err
	}
	result = &LdxpVerifyResult{Verified: true, Redeemed: true, Status: model.LdxpStatusSuccess}
	return nil
}
```

Implement helper:

```go
func ldxpValuePackageTradeNoTx(tx *gorm.DB, session *model.LdxpTopupSession) string {
	if tx == nil || session == nil || session.SubscriptionOrderId <= 0 {
		return ""
	}
	var order model.SubscriptionOrder
	if err := tx.Where("id = ?", session.SubscriptionOrderId).First(&order).Error; err != nil {
		return ""
	}
	return order.TradeNo
}
```

- [ ] **Step 6: Run LDXP tests**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
go test ./model ./service -run 'TestLdxp|TestCreateLdxpValuePackageSession|Test.*ValuePackage.*Ldxp' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
git add model/ldxp_topup.go model/main.go model/ldxp_topup_test.go service/ldxp_session.go service/ldxp_session_test.go service/ldxp_verify.go service/ldxp_verify_test.go
git commit -m "feat: route ldxp payments to value packages"
```

---

## Task 4: Add value-package user/admin APIs

**Files:**
- Create: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/controller/value_package.go`
- Create: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/controller/value_package_test.go`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/router/api-router.go`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/controller/subscription.go`

- [ ] **Step 1: Write failing controller tests**

Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/controller/value_package_test.go` with these tests:

```go
package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupValuePackageControllerTest(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldRedisEnabled := common.RedisEnabled
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	common.RedisEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.SubscriptionPlan{}, &model.SubscriptionOrder{}, &model.UserSubscription{}, &model.UserValuePackagePreference{}, &model.ValuePackageUsageRecord{}, &model.LdxpTopupSession{}, &model.LdxpMailEvent{}, &model.TopUp{}))
	t.Setenv("LDXP_AUTO_TOPUP_ENABLED", "true")
	t.Setenv("LDXP_CONTACT_EMAIL", "buyer@example.test")
	t.Setenv("LDXP_TOPUP_PRODUCTS_JSON", `[{"amount":10,"money":10,"product_url":"https://example.test/10","product_name":"Topup 10"}]`)
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil { _ = sqlDB.Close() }
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.RedisEnabled = oldRedisEnabled
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
	})
	return db
}

func valuePackageControllerRequest(handler gin.HandlerFunc, method, path string, body any, userID int) *httptest.ResponseRecorder {
	router := gin.New()
	routePath := path
	if strings.Contains(path, "/plans/") && strings.Contains(path, "/ldxp/session") {
		routePath = "/value-packages/plans/:plan_id/ldxp/session"
	}
	router.Handle(method, routePath, func(c *gin.Context) {
		if userID > 0 { c.Set("id", userID) }
		handler(c)
	})
	var buf bytes.Buffer
	if body != nil {
		b, err := common.Marshal(body)
		if err != nil { panic(err) }
		buf.Write(b)
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil { req.Header.Set("Content-Type", "application/json") }
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func seedValuePackageControllerPlan(t *testing.T, packageType string, level int) model.SubscriptionPlan {
	t.Helper()
	plan := model.SubscriptionPlan{Title: packageType, PriceAmount: 9.9, Currency: "USD", DurationUnit: model.SubscriptionDurationDay, DurationValue: 1, Enabled: true, PlanKind: model.SubscriptionPlanKindValuePackage, PackageType: packageType, PackageLevel: level, ModelGroup: packageType + "-card", ConcurrencyLimit: 1, Limit5hAmount: 100, Limit7dAmount: 700, LdxpProductUrl: "https://ldxp.example.test/" + packageType, LdxpProductName: packageType + " product", LdxpProductAmount: 9.9}
	require.NoError(t, model.DB.Create(&plan).Error)
	return plan
}

func TestGetValuePackagePlansReturnsOnlyValuePackages(t *testing.T) {
	setupValuePackageControllerTest(t)
	user := createLdxpControllerTestUser(t, "vp_api_user")
	seedValuePackageControllerPlan(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay)
	require.NoError(t, model.DB.Create(&model.SubscriptionPlan{Title: "normal", PriceAmount: 1, Currency: "USD", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: true}).Error)

	rec := valuePackageControllerRequest(GetValuePackagePlans, http.MethodGet, "/value-packages/plans", nil, user.Id)

	body := decodeTestResponse(t, rec)
	require.Equal(t, true, body["success"])
	data := body["data"].(map[string]interface{})
	plans := data["plans"].([]interface{})
	require.Len(t, plans, 1)
}

func TestCreateValuePackageLdxpSessionCreatesPendingOrder(t *testing.T) {
	setupValuePackageControllerTest(t)
	user := createLdxpControllerTestUser(t, "vp_ldxp_user")
	plan := seedValuePackageControllerPlan(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay)

	rec := valuePackageControllerRequest(CreateValuePackageLdxpSession, http.MethodPost, fmt.Sprintf("/value-packages/plans/%d/ldxp/session", plan.Id), gin.H{"confirmed_cover": true}, user.Id)

	body := decodeTestResponse(t, rec)
	require.Equal(t, true, body["success"], rec.Body.String())
	var orders []model.SubscriptionOrder
	require.NoError(t, model.DB.Where("user_id = ? AND plan_id = ?", user.Id, plan.Id).Find(&orders).Error)
	require.Len(t, orders, 1)
}

func TestActivateAndDeactivateValuePackageAPI(t *testing.T) {
	setupValuePackageControllerTest(t)
	user := createLdxpControllerTestUser(t, "vp_active_user")
	plan := seedValuePackageControllerPlan(t, model.ValuePackageTypeDay, model.ValuePackageLevelDay)
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, StartTime: common.GetTimestamp() - 10, EndTime: common.GetTimestamp() + 3600, Status: model.UserSubscriptionStatusActive}
	require.NoError(t, model.DB.Create(&sub).Error)

	rec := valuePackageControllerRequest(ActivateValuePackageSelf, http.MethodPost, "/value-packages/activate", gin.H{"user_subscription_id": sub.Id}, user.Id)
	body := decodeTestResponse(t, rec)
	require.Equal(t, true, body["success"], rec.Body.String())

	rec = valuePackageControllerRequest(DeactivateValuePackageSelf, http.MethodPost, "/value-packages/deactivate", nil, user.Id)
	body = decodeTestResponse(t, rec)
	require.Equal(t, true, body["success"], rec.Body.String())
}
```

- [ ] **Step 2: Run controller tests and verify they fail**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
go test ./controller -run 'TestGetValuePackagePlans|TestCreateValuePackageLdxpSession|TestActivateAndDeactivateValuePackageAPI' -count=1
```

Expected: FAIL with undefined handlers.

- [ ] **Step 3: Add controller handlers**

Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/controller/value_package.go`:

```go
package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type valuePackageLdxpSessionRequest struct {
	ConfirmedCover bool `json:"confirmed_cover"`
}

type valuePackageActivateRequest struct {
	UserSubscriptionId int `json:"user_subscription_id"`
}

func GetValuePackagePlans(c *gin.Context) {
	userId := c.GetInt("id")
	plans, err := model.GetValuePackagePlansForUser(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	state, _ := model.GetValuePackageState(userId)
	common.ApiSuccess(c, gin.H{"plans": plans, "state": state})
}

func GetValuePackageSelf(c *gin.Context) {
	userId := c.GetInt("id")
	state, err := model.GetValuePackageState(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, state)
}

func GetValuePackagePurchaseIntent(c *gin.Context) {
	userId := c.GetInt("id")
	planId, _ := strconv.Atoi(c.Param("plan_id"))
	confirmed := c.Query("confirmed_cover") == "true" || c.Query("confirmed_cover") == "1"
	intent, err := model.CheckValuePackagePurchaseIntent(userId, planId, confirmed)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, intent)
}

func CreateValuePackageLdxpSession(c *gin.Context) {
	userId := c.GetInt("id")
	planId, _ := strconv.Atoi(c.Param("plan_id"))
	var req valuePackageLdxpSessionRequest
	if c.Request.Body != nil {
		_ = c.ShouldBindJSON(&req)
	}
	cfg, err := service.LoadLdxpConfig()
	if err != nil {
		common.ApiErrorMsg(c, "ldxp topup unavailable")
		return
	}
	view, order, err := service.CreateLdxpValuePackageSession(userId, planId, req.ConfirmedCover, cfg)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"session": view, "order_id": order.TradeNo})
}

func ActivateValuePackageSelf(c *gin.Context) {
	userId := c.GetInt("id")
	var req valuePackageActivateRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.UserSubscriptionId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	state, err := model.ActivateValuePackage(userId, req.UserSubscriptionId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, state)
}

func DeactivateValuePackageSelf(c *gin.Context) {
	userId := c.GetInt("id")
	state, err := model.DeactivateValuePackage(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, state)
}
```

Add model helpers in `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/subscription.go`:

```go
func GetValuePackagePlansForUser(userId int) ([]SubscriptionPlan, error) {
	var plans []SubscriptionPlan
	err := DB.Where("plan_kind = ?", SubscriptionPlanKindValuePackage).Order("package_level asc, sort_order desc, id desc").Find(&plans).Error
	for i := range plans {
		plans[i].NormalizeDefaults()
		normalizeValuePackagePlan(&plans[i])
	}
	return plans, err
}

func GetValuePackageState(userId int) (*ValuePackageState, error) {
	if userId <= 0 {
		return &ValuePackageState{}, nil
	}
	var pref UserValuePackagePreference
	if err := DB.Where("user_id = ?", userId).First(&pref).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &ValuePackageState{Preference: UserValuePackagePreference{UserId: userId}}, nil
		}
		return nil, err
	}
	state := &ValuePackageState{Preference: pref}
	if pref.ActiveUserSubscriptionId > 0 {
		var sub UserSubscription
		if err := DB.Where("id = ? AND user_id = ?", pref.ActiveUserSubscriptionId, userId).First(&sub).Error; err == nil {
			state.Subscription = &sub
			if plan, err := GetSubscriptionPlanById(sub.PlanId); err == nil {
				state.Plan = plan
			}
		}
	}
	return state, nil
}
```

- [ ] **Step 4: Register routes**

Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/router/api-router.go` after subscription routes:

```go
valuePackageRoute := apiRouter.Group("/value-packages")
valuePackageRoute.Use(middleware.UserAuth())
{
	valuePackageRoute.GET("/plans", controller.GetValuePackagePlans)
	valuePackageRoute.GET("/self", controller.GetValuePackageSelf)
	valuePackageRoute.GET("/plans/:plan_id/purchase-intent", controller.GetValuePackagePurchaseIntent)
	valuePackageRoute.POST("/plans/:plan_id/ldxp/session", middleware.CriticalRateLimit(), controller.CreateValuePackageLdxpSession)
	valuePackageRoute.POST("/activate", controller.ActivateValuePackageSelf)
	valuePackageRoute.POST("/deactivate", controller.DeactivateValuePackageSelf)
}
```

- [ ] **Step 5: Update subscription admin validation**

Modify `AdminCreateSubscriptionPlan` and `AdminUpdateSubscriptionPlan` in `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/controller/subscription.go` to call a shared validation helper.

Add helper:

```go
func normalizeAndValidateSubscriptionPlanRequest(plan *model.SubscriptionPlan) string {
	plan.NormalizeDefaults()
	if plan.PlanKind == model.SubscriptionPlanKindValuePackage {
		plan.UpgradeGroup = ""
		if plan.PackageType != model.ValuePackageTypeDay && plan.PackageType != model.ValuePackageTypeWeek && plan.PackageType != model.ValuePackageTypeMonth {
			return "套餐类型必须是日卡、周卡或月卡"
		}
		if plan.ModelGroup == "" {
			return "套餐模型分组不能为空"
		}
		if plan.ConcurrencyLimit < 1 || plan.ConcurrencyLimit > 2 {
			return "并发限制必须是1或2"
		}
		if plan.Limit5hAmount < 0 || plan.Limit7dAmount < 0 {
			return "限额不能为负数"
		}
		if plan.Limit7dAmount > 0 && plan.Limit5hAmount > 0 && plan.Limit7dAmount < plan.Limit5hAmount {
			return "7天限额不能小于5小时限额"
		}
		if plan.Enabled && (strings.TrimSpace(plan.LdxpProductUrl) == "" || strings.TrimSpace(plan.LdxpProductName) == "" || plan.LdxpProductAmount <= 0) {
			return "启用超值套餐前必须配置联动小铺付款链接、商品名称和付款金额"
		}
	}
	return ""
}
```

Call it after existing common validation and before create/update. Add new fields to the `updateMap` in `AdminUpdateSubscriptionPlan`.

- [ ] **Step 6: Run controller tests**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
go test ./controller -run 'TestGetValuePackagePlans|TestCreateValuePackageLdxpSession|TestActivateAndDeactivateValuePackageAPI|Test.*SubscriptionPlan' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
git add controller/value_package.go controller/value_package_test.go controller/subscription.go router/api-router.go model/subscription.go
git commit -m "feat: add value package APIs"
```

---

## Task 5: Enforce active package group, rolling limits, and concurrency in relay

**Files:**
- Create: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/middleware/value_package.go`
- Create: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/middleware/value_package_test.go`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/router/relay-router.go`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/router/video-router.go`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/relay/common/relay_info.go`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/subscription.go`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/service/funding_source.go`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/service/billing_session.go`
- Create: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/service/billing_session_test.go`

- [ ] **Step 1: Write failing middleware tests**

Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/middleware/value_package_test.go`:

```go
package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupValuePackageMiddlewareTest(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldRedisEnabled := common.RedisEnabled
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	common.RedisEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.SubscriptionPlan{}, &model.UserSubscription{}, &model.UserValuePackagePreference{}, &model.ValuePackageUsageRecord{}))
	t.Cleanup(func() {
		sqlDB, err := db.DB(); if err == nil { _ = sqlDB.Close() }
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.RedisEnabled = oldRedisEnabled
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
	})
	return db
}

func seedActiveMiddlewarePackage(t *testing.T, userID int, modelGroup string, limit5h int64, limit7d int64) model.UserSubscription {
	t.Helper()
	plan := model.SubscriptionPlan{Title: "day", PriceAmount: 9.9, Currency: "USD", DurationUnit: model.SubscriptionDurationDay, DurationValue: 1, Enabled: true, PlanKind: model.SubscriptionPlanKindValuePackage, PackageType: model.ValuePackageTypeDay, PackageLevel: model.ValuePackageLevelDay, ModelGroup: modelGroup, ConcurrencyLimit: 1, Limit5hAmount: limit5h, Limit7dAmount: limit7d}
	require.NoError(t, model.DB.Create(&plan).Error)
	sub := model.UserSubscription{UserId: userID, PlanId: plan.Id, StartTime: common.GetTimestamp() - 10, EndTime: common.GetTimestamp() + 3600, Status: model.UserSubscriptionStatusActive}
	require.NoError(t, model.DB.Create(&sub).Error)
	require.NoError(t, model.DB.Create(&model.UserValuePackagePreference{UserId: userID, Enabled: true, ActiveUserSubscriptionId: sub.Id}).Error)
	return sub
}

func TestValuePackageMiddlewareForcesUsingGroup(t *testing.T) {
	setupValuePackageMiddlewareTest(t)
	seedActiveMiddlewarePackage(t, 4001, "day-card", 1000, 5000)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUserId, 4001)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "gpt-plus")
		common.SetContextKey(c, constant.ContextKeyTokenGroup, "gpt-plus")
		c.Next()
	})
	router.Use(ValuePackageEntitlement())
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"using_group": common.GetContextKeyString(c, constant.ContextKeyUsingGroup), "value_package_id": common.GetContextKeyInt(c, constant.ContextKeyValuePackageSubscriptionId)})
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"using_group":"day-card"`)
}

func TestValuePackageMiddlewareRejectsOverWindowLimit(t *testing.T) {
	setupValuePackageMiddlewareTest(t)
	sub := seedActiveMiddlewarePackage(t, 4002, "day-card", 100, 5000)
	require.NoError(t, model.RecordValuePackageUsage(&model.ValuePackageUsageRecord{UserId: 4002, UserSubscriptionId: sub.Id, PlanId: sub.PlanId, PackageType: model.ValuePackageTypeDay, ModelGroup: "day-card", RequestId: "recent", Quota: 100, CreatedAt: common.GetTimestamp()}))

	router := gin.New()
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUserId, 4002)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "gpt-plus")
		c.Next()
	})
	router.Use(ValuePackageEntitlement())
	router.POST("/v1/chat/completions", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}
```

- [ ] **Step 2: Add context keys**

Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/constant/context_key.go`. Add these constants inside the existing `const` block after `ContextKeyUsingGroup`:

```go
ContextKeyValuePackageSubscriptionId ContextKey = "value_package_subscription_id"
ContextKeyValuePackagePlanId         ContextKey = "value_package_plan_id"
ContextKeyValuePackageModelGroup     ContextKey = "value_package_model_group"
ContextKeyValuePackagePackageType    ContextKey = "value_package_package_type"
```


- [ ] **Step 3: Implement middleware**

Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/middleware/value_package.go`:

```go
package middleware

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func ValuePackageEntitlement() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := common.GetContextKeyInt(c, constant.ContextKeyUserId)
		if userID <= 0 {
			c.Next()
			return
		}
		state, err := model.GetActiveValuePackageForRelay(userID)
		if err != nil || state == nil || state.Plan == nil || state.Subscription == nil {
			c.Next()
			return
		}
		used5h, used7d, err := model.GetValuePackageWindowUsage(userID, state.Subscription.Id, common.GetTimestamp())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "value package usage check failed"})
			c.Abort()
			return
		}
		if state.Plan.Limit5hAmount > 0 && used5h >= state.Plan.Limit5hAmount {
			c.JSON(http.StatusForbidden, gin.H{"error": "value package 5h limit exceeded"})
			c.Abort()
			return
		}
		if state.Plan.Limit7dAmount > 0 && used7d >= state.Plan.Limit7dAmount {
			c.JSON(http.StatusForbidden, gin.H{"error": "value package 7d limit exceeded"})
			c.Abort()
			return
		}
		group := strings.TrimSpace(state.Plan.ModelGroup)
		if group == "" {
			c.Next()
			return
		}
		common.SetContextKey(c, constant.ContextKeyUsingGroup, group)
		common.SetContextKey(c, constant.ContextKeyTokenGroup, group)
		common.SetContextKey(c, constant.ContextKeyValuePackageSubscriptionId, state.Subscription.Id)
		common.SetContextKey(c, constant.ContextKeyValuePackagePlanId, state.Plan.Id)
		common.SetContextKey(c, constant.ContextKeyValuePackageModelGroup, group)
		common.SetContextKey(c, constant.ContextKeyValuePackagePackageType, state.Plan.PackageType)
		c.Next()
	}
}
```

Add model helper:

```go
func GetActiveValuePackageForRelay(userId int) (*ValuePackageState, error) {
	state, err := GetValuePackageState(userId)
	if err != nil || state == nil || !state.Preference.Enabled || state.Preference.ActiveUserSubscriptionId <= 0 {
		return nil, err
	}
	if state.Subscription == nil || state.Plan == nil {
		return nil, nil
	}
	if state.Subscription.Status != UserSubscriptionStatusActive || state.Subscription.EndTime <= GetDBTimestamp() {
		return nil, nil
	}
	state.Plan.NormalizeDefaults()
	normalizeValuePackagePlan(state.Plan)
	if !state.Plan.IsValuePackage() || strings.TrimSpace(state.Plan.ModelGroup) == "" {
		return nil, nil
	}
	return state, nil
}
```

- [ ] **Step 4: Add middleware to relay routes**

Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/router/relay-router.go`:

- For `/pg`, use `middleware.UserAuth(), middleware.ValuePackageEntitlement(), middleware.Distribute()`.
- For `/v1/models`, `/v1beta/models`, and `/v1beta/openai/models`, use `middleware.TokenAuth(), middleware.ValuePackageEntitlement()` so model lists honor the active package group.
- For relay `/v1`, call `relayV1Router.Use(middleware.ValuePackageEntitlement())` immediately after `relayV1Router.Use(middleware.TokenAuth())` and before `relayV1Router.Use(middleware.ModelRequestRateLimit())`.
- For `/v1beta` relay, call `relayGeminiRouter.Use(middleware.ValuePackageEntitlement())` immediately after `TokenAuth()` and before `ModelRequestRateLimit()`.
- In `registerMjRouterGroup`, change `relayMjRouter.Use(middleware.TokenAuth(), middleware.Distribute())` to `relayMjRouter.Use(middleware.TokenAuth(), middleware.ValuePackageEntitlement(), middleware.Distribute())`.
- For `/suno`, change `relaySunoRouter.Use(middleware.TokenAuth(), middleware.Distribute())` to `relaySunoRouter.Use(middleware.TokenAuth(), middleware.ValuePackageEntitlement(), middleware.Distribute())`.

Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/router/video-router.go`:

- For `videoV1Router`, use `middleware.TokenAuth(), middleware.ValuePackageEntitlement(), middleware.Distribute()`.
- For `klingV1Router`, keep `middleware.KlingRequestConvert()` first, then use `middleware.TokenAuth(), middleware.ValuePackageEntitlement(), middleware.Distribute()`.
- For `jimengOfficialGroup`, keep `middleware.JimengRequestConvert()` first, then use `middleware.TokenAuth(), middleware.ValuePackageEntitlement(), middleware.Distribute()`.

- [ ] **Step 5: Record settled usage**

Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/relay/common/relay_info.go`, add fields to `RelayInfo` near the existing subscription fields:

```go
ValuePackageSubscriptionId int
ValuePackagePlanId         int
ValuePackageModelGroup     string
ValuePackagePackageType    string
```

Populate them in `genBaseRelayInfo` from the new context keys:

```go
ValuePackageSubscriptionId: common.GetContextKeyInt(c, constant.ContextKeyValuePackageSubscriptionId),
ValuePackagePlanId:         common.GetContextKeyInt(c, constant.ContextKeyValuePackagePlanId),
ValuePackageModelGroup:     common.GetContextKeyString(c, constant.ContextKeyValuePackageModelGroup),
ValuePackagePackageType:    common.GetContextKeyString(c, constant.ContextKeyValuePackagePackageType),
```

Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/service/billing_session.go` by adding this helper near `Settle`:

```go
func (s *BillingSession) recordValuePackageActualUsage(actualQuota int) {
	if s == nil || s.relayInfo == nil || s.relayInfo.ValuePackageSubscriptionId <= 0 || actualQuota <= 0 {
		return
	}
	if err := model.RecordValuePackageUsage(&model.ValuePackageUsageRecord{
		UserId:             s.relayInfo.UserId,
		UserSubscriptionId: s.relayInfo.ValuePackageSubscriptionId,
		PlanId:             s.relayInfo.ValuePackagePlanId,
		PackageType:        s.relayInfo.ValuePackagePackageType,
		ModelGroup:         s.relayInfo.ValuePackageModelGroup,
		RequestId:          s.relayInfo.RequestId,
		Quota:              int64(actualQuota),
	}); err != nil {
		common.SysLog(fmt.Sprintf("error recording value package usage (userId=%d, subscriptionId=%d, quota=%d): %s", s.relayInfo.UserId, s.relayInfo.ValuePackageSubscriptionId, actualQuota, err.Error()))
	}
}
```

Then modify `Settle(actualQuota int)` in two places:

```go
if delta == 0 {
	s.recordValuePackageActualUsage(actualQuota)
	s.settled = true
	return nil
}
```

and after token quota adjustment/logging, immediately before `s.settled = true`:

```go
s.recordValuePackageActualUsage(actualQuota)
```

This records usage even when actual quota exactly equals the pre-consumed amount.

- [ ] **Step 6: Implement concurrency limiter**

In `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/middleware/value_package.go`, add `sync` to imports and add this package-level helper:

```go
type valuePackageConcurrencyCounter struct {
	mu     sync.Mutex
	active int
}

var valuePackageConcurrency sync.Map // map[int]*valuePackageConcurrencyCounter

func acquireValuePackageSlot(subscriptionId int, limit int) (func(), bool) {
	if subscriptionId <= 0 {
		return func() {}, true
	}
	if limit <= 0 {
		limit = 1
	}
	if limit > 2 {
		limit = 2
	}
	raw, _ := valuePackageConcurrency.LoadOrStore(subscriptionId, &valuePackageConcurrencyCounter{})
	counter := raw.(*valuePackageConcurrencyCounter)
	counter.mu.Lock()
	defer counter.mu.Unlock()
	if counter.active >= limit {
		return nil, false
	}
	counter.active++
	return func() {
		counter.mu.Lock()
		defer counter.mu.Unlock()
		if counter.active > 0 {
			counter.active--
		}
	}, true
}
```

In `ValuePackageEntitlement`, after setting the value-package context keys and before `c.Next()`, add:

```go
release, ok := acquireValuePackageSlot(state.Subscription.Id, state.Plan.ConcurrencyLimit)
if !ok {
	c.JSON(http.StatusTooManyRequests, gin.H{"error": "value package concurrency limit exceeded"})
	c.Abort()
	return
}
defer release()
c.Next()
```

Remove the earlier direct `c.Next()` at the end of the middleware so the request always passes through the concurrency release path.

Append this helper test to `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/middleware/value_package_test.go`:

```go
func TestValuePackageConcurrencyLimiterRejectsSecondSlot(t *testing.T) {
	valuePackageConcurrency = sync.Map{}
	release, ok := acquireValuePackageSlot(901, 1)
	require.True(t, ok)
	require.NotNil(t, release)

	secondRelease, secondOK := acquireValuePackageSlot(901, 1)
	require.False(t, secondOK)
	require.Nil(t, secondRelease)

	release()
	thirdRelease, thirdOK := acquireValuePackageSlot(901, 1)
	require.True(t, thirdOK)
	require.NotNil(t, thirdRelease)
	thirdRelease()
}
```

- [ ] **Step 7: Write failing value-package billing tests**

Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/service/billing_session_test.go`:

```go
package service

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupValuePackageBillingSessionTest(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldRedisEnabled := common.RedisEnabled
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	common.RedisEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.SubscriptionPlan{}, &model.UserSubscription{}, &model.SubscriptionPreConsumeRecord{}, &model.UserValuePackagePreference{}, &model.ValuePackageUsageRecord{}))
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.RedisEnabled = oldRedisEnabled
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
	})
	return db
}

func TestValuePackageBillingIgnoresWalletOnlyPreference(t *testing.T) {
	setupValuePackageBillingSessionTest(t)
	user := model.User{Id: 5101, Username: "vp-billing", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: model.UserGroupTiyan, Quota: 99999, AffCode: "vp-billing-aff"}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "日卡", PriceAmount: 9.9, Currency: "USD", DurationUnit: model.SubscriptionDurationDay, DurationValue: 1, Enabled: true, PlanKind: model.SubscriptionPlanKindValuePackage, PackageType: model.ValuePackageTypeDay, PackageLevel: model.ValuePackageLevelDay, ModelGroup: "day-card", ConcurrencyLimit: 1, TotalAmount: 1000}
	require.NoError(t, model.DB.Create(&plan).Error)
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, AmountTotal: 1000, StartTime: common.GetTimestamp() - 10, EndTime: common.GetTimestamp() + 3600, Status: model.UserSubscriptionStatusActive}
	require.NoError(t, model.DB.Create(&sub).Error)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	relayInfo := &relaycommon.RelayInfo{UserId: user.Id, RequestId: "vp-billing-req", OriginModelName: "gpt-4o", IsPlayground: true, UserSetting: dto.UserSetting{BillingPreference: "wallet_only"}, ValuePackageSubscriptionId: sub.Id, ValuePackagePlanId: plan.Id, ValuePackageModelGroup: "day-card", ValuePackagePackageType: model.ValuePackageTypeDay}

	session, apiErr := NewBillingSession(ctx, relayInfo, 100)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	require.Equal(t, BillingSourceSubscription, relayInfo.BillingSource)
	require.Equal(t, sub.Id, relayInfo.SubscriptionId)

	var afterPre model.UserSubscription
	require.NoError(t, model.DB.First(&afterPre, sub.Id).Error)
	require.EqualValues(t, 100, afterPre.AmountUsed)
	quota, err := model.GetUserQuota(user.Id, true)
	require.NoError(t, err)
	require.Equal(t, 99999, quota)

	require.NoError(t, session.Settle(150))
	var afterSettle model.UserSubscription
	require.NoError(t, model.DB.First(&afterSettle, sub.Id).Error)
	require.EqualValues(t, 150, afterSettle.AmountUsed)
	used5h, used7d, err := model.GetValuePackageWindowUsage(user.Id, sub.Id, common.GetTimestamp())
	require.NoError(t, err)
	require.EqualValues(t, 150, used5h)
	require.EqualValues(t, 150, used7d)
}
```

Run:

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
go test ./service -run TestValuePackageBillingIgnoresWalletOnlyPreference -count=1
```

Expected: FAIL with missing `RelayInfo.ValuePackageSubscriptionId` or missing value-package funding behavior.

- [ ] **Step 8: Implement value-package funding path**

Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/subscription.go` so normal subscription billing ignores value-package plans:

- In `HasActiveUserSubscription`, replace the count-only query with a query for active subscriptions, load each plan with `getSubscriptionPlanByIdTx(nil, sub.PlanId)`, and return `true` only for plans where `!plan.IsValuePackage()`.
- In `PreConsumeUserSubscription`, inside the candidate loop immediately after loading `plan`, call `normalizeValuePackagePlan(plan)` and `continue` when `plan.IsValuePackage()` is true.

Add this specific pre-consume function to `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/model/subscription.go`:

```go
func PreConsumeValuePackageSubscription(requestId string, userId int, userSubscriptionId int, amount int64) (*SubscriptionPreConsumeResult, error) {
	if userId <= 0 || userSubscriptionId <= 0 {
		return nil, errors.New("invalid value package pre-consume args")
	}
	if strings.TrimSpace(requestId) == "" {
		return nil, errors.New("requestId is empty")
	}
	if amount <= 0 {
		return nil, errors.New("amount must be > 0")
	}
	now := GetDBTimestamp()
	result := &SubscriptionPreConsumeResult{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var existing SubscriptionPreConsumeRecord
		query := tx.Where("request_id = ?", requestId).Limit(1).Find(&existing)
		if query.Error != nil {
			return query.Error
		}
		if query.RowsAffected > 0 {
			if existing.Status == "refunded" {
				return errors.New("subscription pre-consume already refunded")
			}
			var sub UserSubscription
			if err := tx.Where("id = ?", existing.UserSubscriptionId).First(&sub).Error; err != nil {
				return err
			}
			result.UserSubscriptionId = sub.Id
			result.PreConsumed = existing.PreConsumed
			result.AmountTotal = sub.AmountTotal
			result.AmountUsedBefore = sub.AmountUsed
			result.AmountUsedAfter = sub.AmountUsed
			return nil
		}
		var sub UserSubscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ? AND user_id = ? AND status = ? AND end_time > ?", userSubscriptionId, userId, UserSubscriptionStatusActive, now).First(&sub).Error; err != nil {
			return errors.New("no active value package subscription")
		}
		plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
		if err != nil {
			return err
		}
		normalizeValuePackagePlan(plan)
		if !plan.IsValuePackage() {
			return errors.New("subscription is not a value package")
		}
		usedBefore := sub.AmountUsed
		if sub.AmountTotal > 0 && sub.AmountTotal-usedBefore < amount {
			return fmt.Errorf("subscription quota insufficient, need=%d", amount)
		}
		record := &SubscriptionPreConsumeRecord{RequestId: requestId, UserId: userId, UserSubscriptionId: sub.Id, PreConsumed: amount, Status: "consumed"}
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		sub.AmountUsed += amount
		if err := tx.Save(&sub).Error; err != nil {
			return err
		}
		result.UserSubscriptionId = sub.Id
		result.PreConsumed = amount
		result.AmountTotal = sub.AmountTotal
		result.AmountUsedBefore = usedBefore
		result.AmountUsedAfter = sub.AmountUsed
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
```

Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/service/funding_source.go`:

```go
type SubscriptionFunding struct {
	requestId      string
	userId         int
	modelName      string
	amount         int64
	subscriptionId int
	preConsumed    int64
	valuePackageSubscriptionId int
	AmountTotal     int64
	AmountUsedAfter int64
	PlanId          int
	PlanTitle       string
}
```

Change `SubscriptionFunding.PreConsume` so the first call is:

```go
var res *model.SubscriptionPreConsumeResult
var err error
if s.valuePackageSubscriptionId > 0 {
	res, err = model.PreConsumeValuePackageSubscription(s.requestId, s.userId, s.valuePackageSubscriptionId, s.amount)
} else {
	res, err = model.PreConsumeUserSubscription(s.requestId, s.userId, s.modelName, 0, s.amount)
}
if err != nil {
	return err
}
```

Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/service/billing_session.go` at the start of `NewBillingSession`, after the nil `relayInfo` guard and before reading `BillingPreference`:

```go
if relayInfo.ValuePackageSubscriptionId > 0 {
	subConsume := int64(preConsumedQuota)
	if subConsume <= 0 {
		subConsume = 1
	}
	session := &BillingSession{
		relayInfo: relayInfo,
		funding: &SubscriptionFunding{
			requestId: relayInfo.RequestId,
			userId: relayInfo.UserId,
			modelName: relayInfo.OriginModelName,
			amount: subConsume,
			valuePackageSubscriptionId: relayInfo.ValuePackageSubscriptionId,
		},
	}
	if apiErr := session.preConsume(c, int(subConsume)); apiErr != nil {
		return nil, apiErr
	}
	return session, nil
}
```

Run:

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
go test ./service -run TestValuePackageBillingIgnoresWalletOnlyPreference -count=1
```

Expected: PASS.

- [ ] **Step 9: Run middleware tests**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
go test ./middleware -run 'TestValuePackageMiddleware|TestValuePackageConcurrencyLimiter' -count=1
```

Expected: PASS.

- [ ] **Step 10: Run relay-adjacent tests**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
go test ./middleware ./service ./relay/helper -count=1
```

Expected: PASS.

- [ ] **Step 11: Commit**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
git add middleware/value_package.go middleware/value_package_test.go router/relay-router.go router/video-router.go relay/common/relay_info.go service/funding_source.go service/billing_session.go service/billing_session_test.go model/subscription.go constant
git commit -m "feat: enforce value package entitlements"
```

---

## Task 6: Add VIP modal setting API

**Files:**
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/dto/user_settings.go`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/controller/user.go`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/router/api-router.go`
- Create: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/controller/user_vip_modal_test.go`

- [ ] **Step 1: Write failing setting test**

Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/controller/user_vip_modal_test.go`:

```go
package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestMarkVIPUpgradeModalSeenPersistsSetting(t *testing.T) {
	setupLdxpTopupControllerTest(t)
	user := createLdxpControllerTestUser(t, "vip_modal_seen")
	user.Group = model.UserGroupVIP
	require.NoError(t, model.DB.Save(user).Error)

	recorder := performLdxpControllerRequest(MarkVIPUpgradeModalSeen, http.MethodPost, "/user/vip-upgrade-modal/seen", nil, user.Id, nil)

	body := assertLdxpAPIResponse(t, recorder)
	require.Equal(t, true, body["success"])
	setting, err := model.GetUserSetting(user.Id, true)
	require.NoError(t, err)
	require.True(t, setting.VipUpgradeModalSeen)
}

func TestMarkVIPUpgradeModalSeenRejectsNonVIP(t *testing.T) {
	setupLdxpTopupControllerTest(t)
	user := createLdxpControllerTestUser(t, "not_vip_modal_seen")
	user.Group = model.UserGroupTiyan
	require.NoError(t, model.DB.Save(user).Error)

	recorder := performLdxpControllerRequest(MarkVIPUpgradeModalSeen, http.MethodPost, "/user/vip-upgrade-modal/seen", nil, user.Id, nil)

	body := assertLdxpAPIResponse(t, recorder)
	require.Equal(t, false, body["success"])
}
```

- [ ] **Step 2: Add setting field**

Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/dto/user_settings.go`:

```go
VipUpgradeModalSeen bool `json:"vip_upgrade_modal_seen,omitempty"`
```

- [ ] **Step 3: Add controller endpoint**

In `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/controller/user.go`, add:

```go
func MarkVIPUpgradeModalSeen(c *gin.Context) {
	userID := c.GetInt("id")
	user, err := model.GetUserById(userID, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if user.Group != model.UserGroupVIP {
		common.ApiErrorMsg(c, "仅会员用户可确认会员弹窗")
		return
	}
	setting := user.GetSetting()
	setting.VipUpgradeModalSeen = true
	user.SetSetting(setting)
	if err := user.Update(false); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"vip_upgrade_modal_seen": true})
}
```

Register in `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/router/api-router.go` under authenticated user self routes:

```go
selfRoute.POST("/vip-upgrade-modal/seen", controller.MarkVIPUpgradeModalSeen)
```

- [ ] **Step 4: Run tests**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
go test ./controller -run 'TestMarkVIPUpgradeModalSeen' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
git add dto/user_settings.go controller/user.go router/api-router.go controller/user_vip_modal_test.go
git commit -m "feat: add vip celebration setting"
```

---

## Task 7: Add frontend types, API client, and pure value-package rules

**Files:**
- Create: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/value-packages/types.ts`
- Create: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/value-packages/api.ts`
- Create: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/value-packages/lib/rules.ts`
- Create: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/value-packages/lib/rules.test.ts`

- [ ] **Step 1: Write pure rule tests**

Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/value-packages/lib/rules.test.ts`:

```ts
import assert from 'node:assert/strict'
import test from 'node:test'
import {
  getPackageCardState,
  getPackageLevelLabel,
  shouldShowPackageGlow,
} from './rules'
import type { ValuePackagePlanRecord, ValuePackageState } from '../types'

function plan(type: 'day' | 'week' | 'month'): ValuePackagePlanRecord {
  const level = type === 'day' ? 1 : type === 'week' ? 2 : 3
  return {
    plan: {
      id: level,
      title: type,
      subtitle: '',
      price_amount: 9.9,
      currency: 'USD',
      duration_unit: 'day',
      duration_value: 1,
      quota_reset_period: 'never',
      enabled: true,
      sort_order: 0,
      allow_balance_pay: false,
      max_purchase_per_user: 0,
      total_amount: 0,
      plan_kind: 'value_package',
      package_type: type,
      package_level: level,
      model_group: `${type}-card`,
      concurrency_limit: 1,
      limit_5h_amount: 100,
      limit_7d_amount: 700,
      ldxp_product_url: 'https://example.test/product',
      ldxp_product_name: `${type} product`,
      ldxp_product_amount: 9.9,
    },
  }
}

test('unowned package shows purchase', () => {
  assert.equal(getPackageCardState(plan('day'), null).kind, 'purchase')
})

test('active selected package shows running', () => {
  const state: ValuePackageState = {
    preference: { user_id: 1, enabled: true, active_user_subscription_id: 10 },
    subscription: { id: 10, user_id: 1, plan_id: 1, status: 'active', start_time: 1, end_time: Math.floor(Date.now() / 1000) + 3600, amount_total: 0, amount_used: 0 },
    plan: plan('day').plan,
  }
  assert.equal(getPackageCardState(plan('day'), state).kind, 'running')
  assert.equal(shouldShowPackageGlow(state), true)
})

test('owned but disabled package shows start', () => {
  const state: ValuePackageState = {
    preference: { user_id: 1, enabled: false, active_user_subscription_id: 10 },
    subscription: { id: 10, user_id: 1, plan_id: 1, status: 'active', start_time: 1, end_time: Math.floor(Date.now() / 1000) + 3600, amount_total: 0, amount_used: 0 },
    plan: plan('day').plan,
  }
  assert.equal(getPackageCardState(plan('day'), state).kind, 'start')
})

test('package labels are stable', () => {
  assert.equal(getPackageLevelLabel('day'), '日卡')
  assert.equal(getPackageLevelLabel('week'), '周卡')
  assert.equal(getPackageLevelLabel('month'), '月卡')
})
```

- [ ] **Step 2: Add types and rules**

Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/value-packages/types.ts` with complete TypeScript types matching backend DTOs.

Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/value-packages/lib/rules.ts`:

```ts
import type { ValuePackagePlanRecord, ValuePackageState } from '../types'

export type ValuePackageCardStateKind = 'purchase' | 'start' | 'running' | 'expired' | 'disabled'

export function getPackageLevelLabel(type?: string): string {
  if (type === 'day') return '日卡'
  if (type === 'week') return '周卡'
  if (type === 'month') return '月卡'
  return '套餐'
}

export function shouldShowPackageGlow(state: ValuePackageState | null): boolean {
  if (!state?.preference?.enabled || !state.subscription || !state.plan) return false
  const now = Date.now() / 1000
  return state.subscription.status === 'active' && state.subscription.end_time > now
}

export function getPackageCardState(record: ValuePackagePlanRecord, state: ValuePackageState | null): { kind: ValuePackageCardStateKind; userSubscriptionId?: number } {
  const packagePlan = record.plan
  if (!packagePlan.enabled) return { kind: 'disabled' }
  const sub = state?.subscription
  if (!sub || state?.plan?.id !== packagePlan.id) return { kind: 'purchase' }
  if (sub.status !== 'active' || sub.end_time <= Date.now() / 1000) return { kind: 'expired' }
  if (state?.preference?.enabled) return { kind: 'running', userSubscriptionId: sub.id }
  return { kind: 'start', userSubscriptionId: sub.id }
}
```

- [ ] **Step 3: Add API client**

Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/value-packages/api.ts`:

```ts
import { api, type ApiRequestConfig } from '@/lib/api'
import type {
  ApiResponse,
  ValuePackagePlansResponse,
  ValuePackageState,
  ValuePackageLdxpSessionResponse,
  ValuePackagePurchaseIntent,
} from './types'

export async function getValuePackagePlans(): Promise<ApiResponse<ValuePackagePlansResponse>> {
  const res = await api.get('/api/value-packages/plans')
  return res.data
}

export async function getValuePackageSelf(): Promise<ApiResponse<ValuePackageState>> {
  const res = await api.get('/api/value-packages/self', { disableDuplicate: true } satisfies ApiRequestConfig)
  return res.data
}

export async function getValuePackagePurchaseIntent(planId: number, confirmedCover = false): Promise<ApiResponse<ValuePackagePurchaseIntent>> {
  const res = await api.get(`/api/value-packages/plans/${planId}/purchase-intent`, { params: { confirmed_cover: confirmedCover ? '1' : '0' }, skipBusinessError: true } satisfies ApiRequestConfig)
  return res.data
}

export async function createValuePackageLdxpSession(planId: number, confirmedCover: boolean): Promise<ApiResponse<ValuePackageLdxpSessionResponse>> {
  const res = await api.post(`/api/value-packages/plans/${planId}/ldxp/session`, { confirmed_cover: confirmedCover }, { skipBusinessError: true } satisfies ApiRequestConfig)
  return res.data
}

export async function activateValuePackage(userSubscriptionId: number): Promise<ApiResponse<ValuePackageState>> {
  const res = await api.post('/api/value-packages/activate', { user_subscription_id: userSubscriptionId })
  return res.data
}

export async function deactivateValuePackage(): Promise<ApiResponse<ValuePackageState>> {
  const res = await api.post('/api/value-packages/deactivate')
  return res.data
}

export async function markVipUpgradeModalSeen(): Promise<ApiResponse<{ vip_upgrade_modal_seen: boolean }>> {
  const res = await api.post('/api/user/vip-upgrade-modal/seen')
  return res.data
}
```

- [ ] **Step 4: Run frontend rule tests**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default
bun test src/features/value-packages/lib/rules.test.ts
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
git add web/default/src/features/value-packages/types.ts web/default/src/features/value-packages/api.ts web/default/src/features/value-packages/lib/rules.ts web/default/src/features/value-packages/lib/rules.test.ts
git commit -m "feat: add value package frontend primitives"
```

---

## Task 8: Build user-facing value package page and wallet entry

**Files:**
- Create: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/value-packages/index.tsx`
- Create: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/value-packages/hooks/use-value-packages.ts`
- Create: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/value-packages/components/value-package-card.tsx`
- Create: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/value-packages/components/value-package-payment-dialog.tsx`
- Create: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/value-packages/components/value-package-status-banner.tsx`
- Create: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/routes/_authenticated/value-packages/index.tsx`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/wallet/index.tsx`
- Create: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/wallet/components/value-packages-entry-card.tsx`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/hooks/sidebar-data-model.ts`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/hooks/use-sidebar-data.ts`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/hooks/sidebar-data-model.test.ts`

- [ ] **Step 1: Update sidebar tests first**

Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/hooks/sidebar-data-model.test.ts` expected ordinary user items to include the `超值套餐` / `Value Packages` route immediately before the wallet route. Add assertion:

```ts
assert.equal(
  items.some((item) => 'url' in item && item.url === '/value-packages'),
  true
)
```

Run:

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default
bun test src/hooks/sidebar-data-model.test.ts
```

Expected: FAIL until nav is updated.

- [ ] **Step 2: Add nav item**

Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/hooks/sidebar-data-model.ts`:

- Add a `Value Packages` item in ordinary user wallet group before `Wallet / Top up`.
- Add the same item in admin `personal` group before `Wallet`.

Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/hooks/use-sidebar-data.ts` to import `Sparkles` from `lucide-react` and add `valuePackages: Sparkles` to `SIDEBAR_ICONS`.

Run:

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default
bun test src/hooks/sidebar-data-model.test.ts
```

Expected: PASS.

- [ ] **Step 3: Build hook and page components**

Create `use-value-packages.ts` with fetch, activate/deactivate, purchase session, and refresh state.

Create `value-package-card.tsx` with buttons:

- `购买`
- `▶ 启动`
- `关闭使用`
- disabled `暂未开放` when LDXP config incomplete or plan disabled.

Create `value-package-payment-dialog.tsx` as a thin adapter around `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/wallet/components/ldxp-payment-dialog.tsx`: import `LdxpPaymentDialog`, accept the value-package session response, map `ValuePackageLdxpSessionResponse.session` to the existing `LdxpTopupSession` shape, and pass through `loading`, `error`, `onCancel`, and `onClose`.

Create `value-package-status-banner.tsx` to show:

- current active package;
- remaining time;
- warning that closing does not pause time.

Create `index.tsx` using `SectionPageLayout` with three cards.

Create route file:

```tsx
import { createFileRoute } from '@tanstack/react-router'
import { ValuePackages } from '@/features/value-packages'

export const Route = createFileRoute('/_authenticated/value-packages/')({
  component: ValuePackages,
})
```

- [ ] **Step 4: Add wallet entry card**

Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/wallet/components/value-packages-entry-card.tsx` with a `TitledCard` and button linking to `/value-packages`.

Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/wallet/index.tsx`:

- Import the entry card.
- Place it above `LdxpTopupCard` / recharge content.
- Remove the wallet-level `SubscriptionPlansCard` import and render path from `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/wallet/index.tsx`; the wallet page should show `ValuePackagesEntryCard` first, then the existing LDXP/recharge cards, then redemption and invite content.

- [ ] **Step 5: Add source tests for UI requirements**

Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/value-packages/components/value-package-card-source.test.ts` that reads the source and asserts it contains:

- `▶` start marker;
- `关闭使用` / translated key;
- `5` and `7` limit labels;
- warning text key for continued countdown.

Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/wallet/components/value-packages-entry-card-source.test.ts` asserting the wallet entry links to `/value-packages`.

- [ ] **Step 6: Run frontend tests**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default
bun test src/hooks/sidebar-data-model.test.ts src/features/value-packages/lib/rules.test.ts src/features/value-packages/components/value-package-card-source.test.ts src/features/wallet/components/value-packages-entry-card-source.test.ts
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
git add web/default/src/features/value-packages web/default/src/routes/_authenticated/value-packages web/default/src/features/wallet web/default/src/hooks/sidebar-data-model.ts web/default/src/hooks/use-sidebar-data.ts web/default/src/hooks/sidebar-data-model.test.ts
git commit -m "feat: add value package user UI"
```

---

## Task 9: Build complete admin card configuration UI

**Files:**
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/subscriptions/types.ts`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/subscriptions/lib/plan-form.ts`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/subscriptions/constants.ts`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/subscriptions/index.tsx`
- Create: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/subscriptions/components/value-package-admin-cards.tsx`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/subscriptions/components/subscriptions-mutate-drawer.tsx`
- Create tests under `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/subscriptions/`

- [ ] **Step 1: Write plan-form tests**

Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/subscriptions/lib/plan-form-value-package.test.ts`:

```ts
import assert from 'node:assert/strict'
import test from 'node:test'
import { formValuesToPlanPayload, planToFormValues, PLAN_FORM_DEFAULTS } from './plan-form'
import type { SubscriptionPlan } from '../types'

test('value package limit fields convert dollars to quota payload', () => {
  const values = {
    ...PLAN_FORM_DEFAULTS,
    title: '日卡',
    plan_kind: 'value_package' as const,
    package_type: 'day' as const,
    package_level: 1,
    model_group: 'day-card',
    concurrency_limit: 1,
    limit_5h_amount: 100,
    limit_7d_amount: 500,
    ldxp_product_url: 'https://ldxp.example.test/day',
    ldxp_product_name: '日卡商品',
    ldxp_product_amount: 9.9,
  }
  const payload = formValuesToPlanPayload(values)
  assert.equal(payload.plan.plan_kind, 'value_package')
  assert.equal(payload.plan.package_type, 'day')
  assert.equal(payload.plan.model_group, 'day-card')
  assert.equal(payload.plan.concurrency_limit, 1)
  assert.equal(typeof payload.plan.limit_5h_amount, 'number')
  assert.equal(typeof payload.plan.limit_7d_amount, 'number')
  assert.equal(payload.plan.ldxp_product_url, 'https://ldxp.example.test/day')
})

test('planToFormValues preserves per-card ldxp payment config', () => {
  const plan: SubscriptionPlan = {
    id: 1,
    title: '日卡',
    subtitle: '',
    price_amount: 9.9,
    currency: 'USD',
    duration_unit: 'day',
    duration_value: 1,
    quota_reset_period: 'never',
    enabled: true,
    sort_order: 0,
    allow_balance_pay: false,
    max_purchase_per_user: 0,
    total_amount: 0,
    plan_kind: 'value_package',
    package_type: 'day',
    package_level: 1,
    model_group: 'day-card',
    concurrency_limit: 1,
    limit_5h_amount: 100,
    limit_7d_amount: 700,
    ldxp_product_url: 'https://ldxp.example.test/day',
    ldxp_product_name: '日卡商品',
    ldxp_product_amount: 9.9,
  }
  const values = planToFormValues(plan)
  assert.equal(values.ldxp_product_url, 'https://ldxp.example.test/day')
  assert.equal(values.ldxp_product_name, '日卡商品')
  assert.equal(values.ldxp_product_amount, 9.9)
})
```

- [ ] **Step 2: Extend subscription plan types**

Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/subscriptions/types.ts` schema with fields:

```ts
plan_kind: z.enum(['subscription', 'value_package']).optional().default('subscription'),
package_type: z.enum(['day', 'week', 'month']).optional(),
package_level: z.number().optional(),
model_group: z.string().optional(),
concurrency_limit: z.number().optional(),
limit_5h_amount: z.number().optional(),
limit_7d_amount: z.number().optional(),
benefits: z.string().optional(),
ldxp_product_url: z.string().optional(),
ldxp_product_name: z.string().optional(),
ldxp_product_amount: z.number().optional(),
ldxp_product_ref: z.string().optional(),
ldxp_session_ttl_seconds: z.number().optional(),
```

- [ ] **Step 3: Extend plan form conversion**

Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/subscriptions/lib/plan-form.ts`:

- Add fields to Zod schema and defaults.
- Use `quotaUnitsToDollars` and `parseQuotaFromDollars` for `limit_5h_amount` and `limit_7d_amount` exactly as `total_amount` does.
- Force `allow_balance_pay=false` and `upgrade_group=''` when `plan_kind === 'value_package'`.

- [ ] **Step 4: Add admin cards**

Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/subscriptions/components/value-package-admin-cards.tsx`.

It should:

- Fetch all admin plans by calling `getAdminPlans()` with React Query key `['admin-subscription-plans', 'value-package-cards', refreshTrigger]`.
- Filter `plan.plan_kind === 'value_package'`.
- Render three fixed cards for day/week/month.
- Show `付款未配置` when `ldxp_product_url`, `ldxp_product_name`, or `ldxp_product_amount` is missing.
- Show price, duration, model group, concurrency, 5h limit, 7d limit.
- Provide edit buttons that set current row and open the existing mutate drawer.
- Provide create buttons for missing day/week/month by constructing a `templateRow` with default plan values and passing it as the current row with `id=0` to the existing mutate drawer.

- [ ] **Step 5: Extend mutate drawer UI**

Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/subscriptions/components/subscriptions-mutate-drawer.tsx`:

- Add `plan_kind` select.
- When value package selected, show:
  - package type;
  - package level as a read-only auto-filled field derived from package type (`day=1`, `week=2`, `month=3`);
  - model group;
  - concurrency limit select with 1 and 2;
  - limit 5h dollars field;
  - limit 7d dollars field;
  - benefits textarea;
  - LDXP payment link;
  - product name;
  - payment amount;
  - external product ref;
  - session TTL seconds.
- Hide Stripe/Creem/Waffo Pancake/Balance fields when `plan_kind === 'value_package'`.
- Show explanatory text: `保存后用户购买将直接调用现有联动小铺支付系统创建付款会话。`

- [ ] **Step 6: Add source tests for complete admin config**

Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/subscriptions/components/value-package-admin-cards-source.test.ts` that checks source contains:

- `day`, `week`, `month`;
- `ldxp_product_url`;
- `concurrency_limit`;
- `limit_5h_amount`;
- `limit_7d_amount`;
- `付款未配置` text key.

Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/subscriptions/components/subscriptions-mutate-drawer-value-package-source.test.ts` that checks mutate drawer source contains all required fields.

- [ ] **Step 7: Run admin frontend tests**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default
bun test src/features/subscriptions/lib/plan-form-value-package.test.ts src/features/subscriptions/components/value-package-admin-cards-source.test.ts src/features/subscriptions/components/subscriptions-mutate-drawer-value-package-source.test.ts
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
git add web/default/src/features/subscriptions
git commit -m "feat: add value package admin configuration UI"
```

---

## Task 10: Add global package/VIP glow and VIP celebration modal

**Files:**
- Create: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/app-effects/global-entitlement-effects.tsx`
- Create: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/app-effects/vip-upgrade-dialog.tsx`
- Create: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/app-effects/global-entitlement-effects-source.test.ts`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/components/layout/components/authenticated-layout.tsx`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/styles/index.css`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/stores/auth-store.ts`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/profile/types.ts`

- [ ] **Step 1: Add source test for priority**

Create `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/features/app-effects/global-entitlement-effects-source.test.ts`:

```ts
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./global-entitlement-effects.tsx', import.meta.url), 'utf8')

test('global entitlement effects prioritize package glow over vip glow', () => {
  assert.match(source, /value-package-glow/)
  assert.match(source, /vip-glow/)
  assert.match(source, /shouldShowPackageGlow/)
})
```

Expected initially: FAIL because file does not exist.

- [ ] **Step 2: Implement global effects**

Create `global-entitlement-effects.tsx`:

- Read the current user from `useAuthStore`; after marking the modal seen, call existing `getSelf()` and update the auth store.
- Fetch value package self via `getValuePackageSelf`.
- If `shouldShowPackageGlow(state)` true, add `data-entitlement-glow="value-package"` to `document.body`.
- Else if user group is `vip`, add `data-entitlement-glow="vip"`.
- Else remove the attribute.
- If user group is `vip` and `setting.vip_upgrade_modal_seen !== true`, render `VipUpgradeDialog`.
- On close, call `markVipUpgradeModalSeen()` and refresh auth user.

Create `vip-upgrade-dialog.tsx` with a tasteful black/gold card modal and text `恭喜你获得会员权益`.

Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/components/layout/components/authenticated-layout.tsx` to render `<GlobalEntitlementEffects />` inside non-fullscreen authenticated layout.

- [ ] **Step 3: Add CSS glow**

Modify `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/styles/index.css`:

```css
body[data-entitlement-glow='value-package']::before,
body[data-entitlement-glow='vip']::before {
  content: '';
  pointer-events: none;
  position: fixed;
  inset: 0;
  z-index: 50;
  border-radius: 0;
}

body[data-entitlement-glow='value-package']::before {
  box-shadow: inset 0 0 34px color-mix(in oklch, #facc15 45%, transparent), inset 0 0 90px color-mix(in oklch, #facc15 22%, transparent);
  animation: entitlement-glow-breathe 2.8s ease-in-out infinite;
}

body[data-entitlement-glow='vip']::before {
  box-shadow: inset 0 0 38px color-mix(in oklch, #f6c453 50%, transparent), inset 0 0 110px color-mix(in oklch, #111827 18%, transparent);
  animation: entitlement-glow-breathe 3.2s ease-in-out infinite;
}

@keyframes entitlement-glow-breathe {
  0%, 100% { opacity: .45; }
  50% { opacity: .9; }
}

@media (prefers-reduced-motion: reduce) {
  body[data-entitlement-glow='value-package']::before,
  body[data-entitlement-glow='vip']::before {
    animation: none;
  }
}
```

Use `z-index: 40` for the glow pseudo-element so existing dialogs with higher overlay layers stay visually dominant; `pointer-events: none` keeps clicks safe.

- [ ] **Step 4: Run source and type tests**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default
bun test src/features/app-effects/global-entitlement-effects-source.test.ts
bun run typecheck
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
git add web/default/src/features/app-effects web/default/src/components/layout/components/authenticated-layout.tsx web/default/src/styles/index.css web/default/src/**/*.ts web/default/src/**/*.tsx
git commit -m "feat: add entitlement glow and vip celebration"
```

When staging, inspect `git status --short` first and avoid accidentally staging unrelated generated files.

---

## Task 11: Complete i18n for all new frontend text

**Files:**
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/i18n/locales/en.json`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/i18n/locales/zh.json`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/i18n/locales/fr.json`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/i18n/locales/ja.json`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/i18n/locales/ru.json`
- Modify: `/Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default/src/i18n/locales/vi.json`

- [ ] **Step 1: Run i18n sync**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default
bun run i18n:sync
```

Expected: locale files updated with new keys.

- [ ] **Step 2: Fill translations**

Use the existing flat JSON style. For every new key related to value packages, add translations with these rules:

- English: natural product UI labels.
- Chinese: use the exact business vocabulary from the spec: `超值套餐`、`日卡`、`周卡`、`月卡`、`联动小铺付款链接`、`关闭使用`、`有效期仍会继续计算`.
- French/Japanese/Russian/Vietnamese: concise localized equivalents; keep only brand-like tokens such as `LDXP` unchanged.

- [ ] **Step 3: Run i18n checks**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default
bun run i18n:sync
bun test src/features/value-packages/lib/rules.test.ts
```

Expected: sync reports no newly missing keys after edits, tests pass.

- [ ] **Step 4: Commit**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
git add web/default/src/i18n/locales
git commit -m "i18n: add value package translations"
```

---

## Task 12: Full verification, cleanup, and final review

**Files:**
- Inspect and, when verification exposes a defect from Tasks 1-11, modify the exact source file that introduced the defect before the final commit.

- [ ] **Step 1: Run backend focused tests**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
go test ./model ./service ./controller ./middleware -count=1
```

Expected: PASS.

- [ ] **Step 2: Run frontend focused tests**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default
bun test src/hooks/sidebar-data-model.test.ts src/features/value-packages/lib/rules.test.ts src/features/value-packages/components/value-package-card-source.test.ts src/features/wallet/components/value-packages-entry-card-source.test.ts src/features/subscriptions/lib/plan-form-value-package.test.ts src/features/subscriptions/components/value-package-admin-cards-source.test.ts src/features/subscriptions/components/subscriptions-mutate-drawer-value-package-source.test.ts src/features/app-effects/global-entitlement-effects-source.test.ts
```

Expected: PASS.

- [ ] **Step 3: Run frontend typecheck and build**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default
bun run typecheck
bun run build
```

Expected: PASS.

- [ ] **Step 4: Run lint after typecheck and build**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default
bun run lint
```

Expected: PASS. If this command fails because of unrelated pre-existing lint warnings, do not claim PASS; copy the exact output into the final handoff under known pre-existing failures.

- [ ] **Step 5: Manual browser smoke test**

Run `cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages/web/default && bun run dev --host 127.0.0.1 --port 52414`, then use the in-app browser at `http://localhost:52414/`. Verify these flows:

1. Admin opens `Subscription Management` and sees three super-value package config cards.
2. Admin edits 日卡 and sees per-card LDXP payment link, product name, amount, session TTL, concurrency, 5h limit, 7d limit.
3. Ordinary user sees `超值套餐` nav entry.
4. Ordinary user opens `/value-packages` and sees 日卡 / 周卡 / 月卡 cards.
5. A card missing payment link shows disabled purchase state.
6. A paid-but-not-enabled package shows `▶ 启动`.
7. Clicking start shows yellow edge glow.
8. Clicking close removes yellow glow and warns time keeps counting.
9. VIP user without active package shows VIP gold glow and one-time modal.
10. VIP user with active package sees package glow priority over VIP glow.

- [ ] **Step 6: Inspect git diff for scope**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
git status --short
git diff --stat main...HEAD
git diff --name-only main...HEAD
```

Expected: files match this plan; no unrelated dependency lockfiles or generated artifacts outside intended i18n output.

- [ ] **Step 7: Final commit for verification fixes**

After Step 1-6, inspect the working tree. When fixes were made, stage only the planned source areas and commit:

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/spec-value-packages
git status --short
git add model service controller middleware router relay web/default/src
git commit -m "fix: stabilize value package rollout"
```

When `git status --short` is empty, skip this commit.

- [ ] **Step 8: Final handoff**

Report:

- branch name;
- commit range;
- exact verification commands and outcomes;
- known pre-existing failures with exact command output, or `none observed` after the commands in this task;
- whether user wants merge to main, PR, or deployment.
