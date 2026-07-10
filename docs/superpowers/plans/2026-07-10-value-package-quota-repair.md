# Value Package Quota Repair Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make day/week/month package balances truthful and visible, fix renewal snapshots, and migrate active legacy zero-total subscriptions without interrupting users.

**Architecture:** Keep `UserSubscription.AmountTotal` as the lifecycle source of truth, add a structured `period_limits` projection built once in the model layer, and render that projection in every default-frontend surface. A dry-run-first GORM maintenance binary performs the approved B2 migration and is shipped beside the server binary.

**Tech Stack:** Go 1.25+, GORM v2, Gin, React 19, TypeScript, i18next, Bun, SQLite tests with production PostgreSQL dry-run/apply.

---

## File map

**Create**

- `model/value_package_periods.go` — lifecycle/stage/short-window period projection.
- `model/value_package_periods_test.go` — day/week/month projection table tests.
- `model/value_package_quota_migration.go` — portable dry-run/apply migration service.
- `model/value_package_quota_migration_test.go` — target filtering, B2 math, atomicity, idempotency.
- `cmd/value-package-quota-migrate/main.go` — operator CLI and manifest hash enforcement.
- `web/default/src/features/value-packages/lib/period-limits.ts` — typed legacy adapter and ordered display model.
- `web/default/src/features/value-packages/lib/period-limits.test.ts` — pure TypeScript behavior tests.
- `web/default/src/features/value-packages/components/value-package-period-list.tsx` — shared period renderer.

**Modify**

- `model/subscription.go` — add `PeriodLimits`, use one renewal helper in both purchase paths.
- `model/value_package_test.go` — order-completion and replay regression tests.
- `controller/subscription.go` — finite lifecycle-total validation.
- `controller/value_package_test.go` — self-state `period_limits` response coverage.
- `controller/order_management_test.go` — admin rows expose the same period projection.
- `Dockerfile` — build and copy the maintenance binary.
- `web/default/src/features/value-packages/types.ts` — structured period types.
- `web/default/src/features/order-management/types.ts` — reuse the shared usage/period types.
- `web/default/src/features/value-packages/components/value-package-card.tsx` — render period rows instead of parallel quota logic.
- `web/default/src/features/order-management/components/value-package-usage-table.tsx` — replace fixed 5h/7d/total cells with ordered period rows.
- `web/default/src/features/order-management/components/value-package-management-page.tsx` — same admin projection.
- `web/default/src/features/subscriptions/lib/plan-form.ts` — value-package total must be positive; month 7d must not exceed total.
- `web/default/src/features/subscriptions/lib/plan-form-value-package.test.ts` — frontend payload/schema regressions.
- `web/default/src/i18n/locales/{en,zh,fr,ru,ja,vi}.json` — period and no-refresh labels.

## Task 1: Add the canonical period projection

**Files:**
- Create: `model/value_package_periods.go`
- Create: `model/value_package_periods_test.go`
- Modify: `model/subscription.go`

- [ ] **Step 1: Write failing table tests for day, week, and month**

Add tests that call an unexported `buildValuePackagePeriodLimits` helper from package `model`:

```go
func TestBuildValuePackagePeriodLimits(t *testing.T) {
	now := int64(1_783_700_000)
	tests := []struct {
		name string
		plan SubscriptionPlan
		want []ValuePackagePeriodLimit
	}{
		{
			name: "day has 5h and non-refreshing 1d lifecycle",
			plan: SubscriptionPlan{PlanKind: SubscriptionPlanKindValuePackage, PackageType: ValuePackageTypeDay, Limit5hAmount: 900},
			want: []ValuePackagePeriodLimit{
				{Kind: ValuePackagePeriodFiveHour, LabelUnit: "hour", LabelValue: 5, Limit: 900, Used: 100, Remaining: 800, Refreshes: true, ResetAt: now + 60},
				{Kind: ValuePackagePeriodLifecycle, LabelUnit: "day", LabelValue: 1, Limit: 2400, Used: 600, Remaining: 1800, Refreshes: false},
			},
		},
		{
			name: "week exposes lifecycle as 7d instead of stage N/A",
			plan: SubscriptionPlan{PlanKind: SubscriptionPlanKindValuePackage, PackageType: ValuePackageTypeWeek, Limit5hAmount: 900},
			want: []ValuePackagePeriodLimit{
				{Kind: ValuePackagePeriodFiveHour, LabelUnit: "hour", LabelValue: 5, Limit: 900, Used: 100, Remaining: 800, Refreshes: true, ResetAt: now + 60},
				{Kind: ValuePackagePeriodLifecycle, LabelUnit: "day", LabelValue: 7, Limit: 4500, Used: 600, Remaining: 3900, Refreshes: false},
			},
		},
		{
			name: "month has 5h stage7d lifecycle30d",
			plan: SubscriptionPlan{PlanKind: SubscriptionPlanKindValuePackage, PackageType: ValuePackageTypeMonth, Limit5hAmount: 900, Limit7dAmount: 5500},
			want: []ValuePackagePeriodLimit{
				{Kind: ValuePackagePeriodFiveHour, LabelUnit: "hour", LabelValue: 5, Limit: 900, Used: 100, Remaining: 800, Refreshes: true, ResetAt: now + 60},
				{Kind: ValuePackagePeriodSevenDayStage, LabelUnit: "day", LabelValue: 7, Limit: 5500, Used: 700, Remaining: 4800, Refreshes: true, ResetAt: now + 120},
				{Kind: ValuePackagePeriodLifecycle, LabelUnit: "day", LabelValue: 30, Limit: 22000, Used: 600, Remaining: 21400, Refreshes: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := UserSubscription{AmountTotal: tt.want[len(tt.want)-1].Limit, AmountUsed: 600}
			details := ValuePackageWindowUsageDetails{Used5h: 100, ResetAt5h: now + 60, Used7d: 700, ResetAt7d: now + 120}
			require.Equal(t, tt.want, buildValuePackagePeriodLimits(&sub, &tt.plan, &details))
		})
	}
}
```

- [ ] **Step 2: Run the test and confirm RED**

Run:

```bash
go test ./model -run '^TestBuildValuePackagePeriodLimits$' -count=1
```

Expected: compile failure because `ValuePackagePeriodLimit` and `buildValuePackagePeriodLimits` do not exist.

- [ ] **Step 3: Implement the focused projection helper**

Create `model/value_package_periods.go`:

```go
package model

const (
	ValuePackagePeriodFiveHour     = "five_hour"
	ValuePackagePeriodSevenDayStage = "seven_day_stage"
	ValuePackagePeriodLifecycle    = "lifecycle"
)

type ValuePackagePeriodLimit struct {
	Kind       string `json:"kind"`
	LabelUnit  string `json:"label_unit"`
	LabelValue int    `json:"label_value"`
	Limit      int64  `json:"limit"`
	Used       int64  `json:"used"`
	Remaining  int64  `json:"remaining"`
	Percent    float64 `json:"percent"`
	Refreshes  bool   `json:"refreshes"`
	ResetAt    int64  `json:"reset_at"`
	Limited    bool   `json:"limited"`
}

func remainingValuePackageQuota(used, limit int64) int64 {
	if limit <= used {
		return 0
	}
	return limit - used
}

func buildValuePackagePeriodLimits(sub *UserSubscription, plan *SubscriptionPlan, details *ValuePackageWindowUsageDetails) []ValuePackagePeriodLimit {
	if sub == nil || plan == nil || details == nil || !plan.IsValuePackage() {
		return nil
	}
	periods := make([]ValuePackagePeriodLimit, 0, 3)
	if plan.Limit5hAmount > 0 {
		periods = append(periods, ValuePackagePeriodLimit{
			Kind: ValuePackagePeriodFiveHour, LabelUnit: "hour", LabelValue: 5,
			Limit: plan.Limit5hAmount, Used: details.Used5h,
			Remaining: remainingValuePackageQuota(details.Used5h, plan.Limit5hAmount),
			Percent: valuePackagePercent(details.Used5h, plan.Limit5hAmount),
			Refreshes: true, ResetAt: details.ResetAt5h,
			Limited: details.Used5h >= plan.Limit5hAmount,
		})
	}
	if plan.PackageType == ValuePackageTypeMonth && plan.Limit7dAmount > 0 {
		periods = append(periods, ValuePackagePeriodLimit{
			Kind: ValuePackagePeriodSevenDayStage, LabelUnit: "day", LabelValue: 7,
			Limit: plan.Limit7dAmount, Used: details.Used7d,
			Remaining: remainingValuePackageQuota(details.Used7d, plan.Limit7dAmount),
			Percent: valuePackagePercent(details.Used7d, plan.Limit7dAmount),
			Refreshes: true, ResetAt: details.ResetAt7d,
			Limited: details.Used7d >= plan.Limit7dAmount,
		})
	}
	labelDays := map[string]int{ValuePackageTypeDay: 1, ValuePackageTypeWeek: 7, ValuePackageTypeMonth: 30}[plan.PackageType]
	if sub.AmountTotal > 0 && labelDays > 0 {
		periods = append(periods, ValuePackagePeriodLimit{
			Kind: ValuePackagePeriodLifecycle, LabelUnit: "day", LabelValue: labelDays,
			Limit: sub.AmountTotal, Used: sub.AmountUsed,
			Remaining: remainingValuePackageQuota(sub.AmountUsed, sub.AmountTotal),
			Percent: valuePackagePercent(sub.AmountUsed, sub.AmountTotal),
			Refreshes: false, Limited: sub.AmountUsed >= sub.AmountTotal,
		})
	}
	return periods
}
```

Add to `ValuePackageUsageSummary` in `model/subscription.go`:

```go
PeriodLimits []ValuePackagePeriodLimit `json:"period_limits"`
```

Set it inside `buildValuePackageUsageSummaryFromDetails`:

```go
summary.PeriodLimits = buildValuePackagePeriodLimits(sub, plan, usageDetails)
```

- [ ] **Step 4: Run focused and existing model tests**

Run:

```bash
go test ./model -run 'ValuePackage|Subscription' -count=1
```

Expected: PASS; existing legacy fields remain unchanged.

- [ ] **Step 5: Commit the projection**

```bash
git add model/value_package_periods.go model/value_package_periods_test.go model/subscription.go
git commit -m "feat: expose value package quota periods"
```

## Task 2: Fix lifecycle validation and idempotent renewal totals

**Files:**
- Modify: `controller/subscription.go`
- Modify: `controller/value_package_test.go`
- Modify: `model/subscription.go`
- Modify: `model/value_package_test.go`
- Modify: `web/default/src/features/subscriptions/lib/plan-form.ts`
- Modify: `web/default/src/features/subscriptions/lib/plan-form-value-package.test.ts`

- [ ] **Step 1: Add failing renewal and validation tests**

Extend `TestValuePackagePurchaseIntentSameLevelExtends` to assert both time and amount:

```go
require.Equal(t, firstEnd+7*valuePackageDaySeconds, extended.EndTime)
require.EqualValues(t, week.TotalAmount*2, extended.AmountTotal)
require.EqualValues(t, originalUsed, extended.AmountUsed)
```

Add a replay assertion around `CompleteValuePackageOrder`:

```go
first, err := CompleteValuePackageOrder(order.TradeNo, "payload-1", PaymentProviderLDXP, PaymentMethodLDXP, true)
require.NoError(t, err)
firstTotal, firstEnd := first.AmountTotal, first.EndTime
replayed, err := CompleteValuePackageOrder(order.TradeNo, "payload-2", PaymentProviderLDXP, PaymentMethodLDXP, true)
require.NoError(t, err)
require.Equal(t, firstTotal, replayed.AmountTotal)
require.Equal(t, firstEnd, replayed.EndTime)
```

Add controller cases asserting enabled value packages reject `total_amount=0` and month `limit_7d_amount > total_amount`.

- [ ] **Step 2: Confirm RED**

```bash
go test ./model ./controller -run 'ValuePackage.*(Extend|Replay|TotalAmount|Plan)' -count=1
```

Expected: renewal total assertion fails and validation requests currently succeed.

- [ ] **Step 3: Extract one renewal helper and use it in both purchase paths**

Add near the purchase actions in `model/subscription.go`:

```go
func extendValuePackageSubscriptionTx(tx *gorm.DB, subscriptionID int, plan *SubscriptionPlan, nowUnix, purchasedEndUnix int64) (*UserSubscription, error) {
	if tx == nil || subscriptionID <= 0 || plan == nil || plan.TotalAmount <= 0 {
		return nil, errors.New("invalid value package extension")
	}
	var existing UserSubscription
	if err := withUpdateLock(tx).Where("id = ?", subscriptionID).First(&existing).Error; err != nil {
		return nil, err
	}
	base := existing.EndTime
	if base < nowUnix {
		base = nowUnix
	}
	existing.EndTime = base + (purchasedEndUnix - nowUnix)
	existing.AmountTotal += plan.TotalAmount
	existing.UpdatedAt = common.GetTimestamp()
	if err := tx.Save(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}
```

Replace both duplicated `ValuePackagePurchaseActionExtend` blocks in `CreateValuePackageSubscriptionFromPlanTx` and `CompleteValuePackageOrder` with this helper. Do not move the successful order-status update outside the existing transaction; callback replay must continue returning `order.UserSubscriptionId` without invoking the helper.

- [ ] **Step 4: Enforce positive totals and coherent monthly stage limits**

In `normalizeAndValidateSubscriptionPlanRequest` after identifying a value package:

```go
if plan.TotalAmount <= 0 {
	return "超值套餐总额度必须大于0"
}
if plan.PackageType == model.ValuePackageTypeMonth && requestedLimit7dAmount > plan.TotalAmount {
	return "7天阶段额度不能大于30天总额度"
}
```

In `getPlanFormSchema`, use `superRefine` so only value packages require a positive total and month stage does not exceed total. Keep ordinary subscription `total_amount=0` valid.

- [ ] **Step 5: Run backend and frontend tests**

```bash
go test ./model ./controller -run 'ValuePackage|SubscriptionPlan' -count=1
cd web/default
bun test src/features/subscriptions/lib/plan-form-value-package.test.ts
```

Expected: PASS.

- [ ] **Step 6: Commit renewal and validation**

```bash
git add model/subscription.go model/value_package_test.go controller/subscription.go controller/value_package_test.go web/default/src/features/subscriptions/lib/plan-form.ts web/default/src/features/subscriptions/lib/plan-form-value-package.test.ts
git commit -m "fix: preserve value package quota on renewal"
```

## Task 3: Build the B2 migration service and guarded CLI

**Files:**
- Create: `model/value_package_quota_migration.go`
- Create: `model/value_package_quota_migration_test.go`
- Create: `cmd/value-package-quota-migrate/main.go`
- Modify: `Dockerfile`

- [ ] **Step 1: Write failing migration tests**

Seed active/inactive/value-package/ordinary rows and assert:

```go
preview, err := PreviewLegacyValuePackageQuotaMigration(db, now)
require.NoError(t, err)
require.Len(t, preview.Rows, 2)
require.EqualValues(t, weekUsed+weekPlan.TotalAmount, preview.Rows[0].NewTotal)
require.EqualValues(t, monthUsed+monthPlan.TotalAmount, preview.Rows[1].NewTotal)
require.EqualValues(t, 0, activeWeek.AmountTotal, "preview must not write")

require.NoError(t, db.Model(&UserSubscription{}).
	Where("id = ?", preview.Rows[0].SubscriptionID).
	Update("amount_used", weekUsed+1234).Error)
previewAfterUsage, err := PreviewLegacyValuePackageQuotaMigration(db, now)
require.NoError(t, err)
require.Equal(t, preview.ManifestHash, previewAfterUsage.ManifestHash,
	"normal consumption must not invalidate the authorization manifest")

applied, err := ApplyLegacyValuePackageQuotaMigration(db, now, preview.ManifestHash)
require.NoError(t, err)
require.Equal(t, 2, applied.Updated)
require.EqualValues(t, weekUsed+1234+weekPlan.TotalAmount, applied.Rows[0].NewTotal)

second, err := ApplyLegacyValuePackageQuotaMigration(db, now, preview.ManifestHash)
require.NoError(t, err)
require.Equal(t, 0, second.Updated)
```

Also change a plan grant and a subscription expiry after separate previews and assert apply rejects both stale stable manifests without writes. Register a GORM update callback that returns `errors.New("forced second update failure")` and assert every target retains `amount_total=0` after the transaction.

- [ ] **Step 2: Confirm RED**

```bash
go test ./model -run '^Test(LegacyValuePackageQuotaMigration|PreviewLegacyValuePackageQuotaMigration)' -count=1
```

Expected: compile failure because the migration API is absent.

- [ ] **Step 3: Implement portable target discovery and manifest hashing**

Define stable output types in `model/value_package_quota_migration.go`:

```go
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
	MigrationNow int64 `json:"migration_now"`
	Rows []LegacyValuePackageQuotaMigrationRow `json:"rows"`
	Skipped map[string]int `json:"skipped"`
	ManifestHash string `json:"manifest_hash"`
	Updated int `json:"updated"`
}

type legacyValuePackageQuotaManifestRow struct {
	SubscriptionID int    `json:"subscription_id"`
	PlanID         int    `json:"plan_id"`
	PackageType    string `json:"package_type"`
	Grant          int64  `json:"grant"`
	EndTime        int64  `json:"end_time"`
}
```

Use GORM `Where`, `Find`, `First`, and transactions only. Sort rows by subscription ID. Build the manifest hash with `common.Marshal` and SHA-256 over only the sorted `[]legacyValuePackageQuotaManifestRow`; never include `MigrationNow`, `AmountUsed`, `OldTotal`, predicted `NewTotal`, `Skipped`, or `Updated`. Apply recomputes that stable target manifest inside its transaction, requires the supplied hash to match, re-queries each ID with `withUpdateLock(tx)` and the same active/end-time/zero-total predicates, loads the plan, then writes:

```go
newTotal := locked.AmountUsed + plan.TotalAmount
result := tx.Model(&UserSubscription{}).
	Where("id = ? AND amount_total = ?", locked.Id, 0).
	Updates(map[string]interface{}{
		"amount_total": newTotal,
		"updated_at": common.GetTimestamp(),
	})
```

For each applied row, populate the returned `AmountUsed` and `NewTotal` from the locked values rather than copying dry-run estimates. Membership, plan ID, package type, grant, or expiry changes require a new dry-run; normal `amount_used` changes do not. If no eligible target remains because the same manifest was already fully applied, return `Updated=0` without another grant so repeated execution is idempotent.

- [ ] **Step 4: Implement the CLI with dry-run as the default**

`cmd/value-package-quota-migrate/main.go` must register flags before `common.InitEnv()` parses them:

```go
var (
	apply = flag.Bool("apply", false, "apply the migration; default is dry-run")
	manifest = flag.String("manifest", "", "required dry-run manifest SHA-256 for --apply")
)

func main() {
	common.InitEnv()
	if err := model.InitDB(); err != nil { log.Fatal(err) }
	defer model.CloseDB()
	now := model.GetDBTimestamp()
	var report *model.LegacyValuePackageQuotaMigrationReport
	var err error
	if *apply {
		if strings.TrimSpace(*manifest) == "" { log.Fatal("--manifest is required with --apply") }
		report, err = model.ApplyLegacyValuePackageQuotaMigration(model.DB, now, strings.TrimSpace(*manifest))
	} else {
		report, err = model.PreviewLegacyValuePackageQuotaMigration(model.DB, now)
	}
	if err != nil { log.Fatal(err) }
	payload, err := common.Marshal(report)
	if err != nil { log.Fatal(err) }
	fmt.Println(string(payload))
}
```

Do not print DSNs, usernames, emails, API keys, or plan titles.

- [ ] **Step 5: Ship the binary in the image**

In the Go builder stage of `Dockerfile`:

```dockerfile
RUN go build -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=$(cat VERSION)'" -o new-api \
 && go build -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=$(cat VERSION)'" -o value-package-quota-migrate ./cmd/value-package-quota-migrate
```

In the runtime stage:

```dockerfile
COPY --from=builder2 /build/value-package-quota-migrate /
```

- [ ] **Step 6: Verify migration tests and binaries**

```bash
go test ./model -run 'LegacyValuePackageQuotaMigration' -count=1
go test ./cmd/value-package-quota-migrate -count=1
go build -o /tmp/value-package-quota-migrate ./cmd/value-package-quota-migrate
test -x /tmp/value-package-quota-migrate
```

Expected: all commands succeed.

- [ ] **Step 7: Commit migration tooling**

```bash
git add model/value_package_quota_migration.go model/value_package_quota_migration_test.go cmd/value-package-quota-migrate/main.go Dockerfile
git commit -m "feat: add guarded value package quota migration"
```

## Task 4: Prove both API surfaces return identical periods

**Files:**
- Modify: `controller/value_package_test.go`
- Modify: `controller/order_management_test.go`

- [ ] **Step 1: Add response assertions**

For a week card, decode `period_limits` and assert:

```go
require.Len(t, periods, 2)
assert.Equal(t, "five_hour", periods[0]["kind"])
assert.Equal(t, "lifecycle", periods[1]["kind"])
assert.Equal(t, float64(7), periods[1]["label_value"])
assert.Equal(t, false, periods[1]["refreshes"])
assert.Equal(t, float64(sub.AmountTotal-sub.AmountUsed), periods[1]["remaining"])
```

Repeat for admin usage rows and add day/month cardinality assertions.

- [ ] **Step 2: Run controller tests**

```bash
go test ./controller -run 'ValuePackage.*(Usage|State|Period)' -count=1
```

Expected: PASS without controller-specific period computation.

- [ ] **Step 3: Commit API coverage**

```bash
git add controller/value_package_test.go controller/order_management_test.go
git commit -m "test: cover value package period responses"
```

## Task 5: Render truthful periods in default frontend

**Files:**
- Create: `web/default/src/features/value-packages/lib/period-limits.ts`
- Create: `web/default/src/features/value-packages/lib/period-limits.test.ts`
- Create: `web/default/src/features/value-packages/components/value-package-period-list.tsx`
- Modify: `web/default/src/features/value-packages/types.ts`
- Modify: `web/default/src/features/order-management/types.ts`
- Modify: `web/default/src/features/value-packages/components/value-package-card.tsx`
- Modify: `web/default/src/features/order-management/components/value-package-usage-table.tsx`
- Modify: `web/default/src/features/order-management/components/value-package-management-page.tsx`

- [ ] **Step 1: Write failing pure TypeScript tests**

Test structured data first and legacy fallback second:

```ts
test('week lifecycle is a non-refreshing seven-day period', () => {
  const periods = getValuePackagePeriodLimits(weekUsage, 'week')
  assert.deepEqual(periods.map((item) => [item.kind, item.labelValue, item.refreshes]), [
    ['five_hour', 5, true],
    ['lifecycle', 7, false],
  ])
  assert.equal(periods[1].remaining, 33_000_000)
})

test('legacy zero total is not replaced with plan total', () => {
  const periods = getValuePackagePeriodLimits({ ...legacyUsage, total_limit: 0 }, 'week')
  assert.equal(periods.some((item) => item.kind === 'lifecycle'), false)
})
```

- [ ] **Step 2: Confirm RED**

```bash
cd web/default
bun test src/features/value-packages/lib/period-limits.test.ts
```

Expected: module-not-found failure.

- [ ] **Step 3: Add shared types and adapter**

```ts
export type ValuePackagePeriodKind =
  | 'five_hour'
  | 'seven_day_stage'
  | 'lifecycle'

export interface ValuePackagePeriodLimit {
  kind: ValuePackagePeriodKind
  label_unit: 'hour' | 'day'
  label_value: number
  limit: number
  used: number
  remaining: number
  percent: number
  refreshes: boolean
  reset_at: number
  limited: boolean
}
```

`getValuePackagePeriodLimits` returns `usage.period_limits` unchanged when present. Its legacy adapter constructs 5h, optional month 7d stage, and lifecycle only from `usage.total_limit`; it never accepts a plan total argument.

- [ ] **Step 4: Add one reusable renderer**

`ValuePackagePeriodList` receives only `periods` and renders label, `remaining / limit`, progress, used amount, and either reset text or `t('Does not refresh')`. Label keys are derived from kind and unit:

```ts
const labelKeys = {
  five_hour: '5-hour remaining',
  seven_day_stage: 'Current 7-day stage remaining',
  lifecycle: {
    1: '1-day total remaining',
    7: '7-day total remaining',
    30: '30-day total remaining',
  },
} as const
```

- [ ] **Step 5: Replace three duplicate display paths**

- `value-package-card.tsx`: replace separate total/5h/7d `LimitProgressRow` calls with the shared renderer.
- `value-package-usage-table.tsx`: replace the three quota columns with one `Quota periods` column containing the ordered list.
- `value-package-management-page.tsx`: make the same replacement; remove `Period7dQuotaCell` and `TotalRemainingCell` from both admin files.

The week row must contain `7-day total remaining`; it must never render `Not applicable` for its lifecycle balance.

- [ ] **Step 6: Add all six locale values**

Add translations for these exact English keys in `en`, `zh`, `fr`, `ru`, `ja`, and `vi`:

```text
Quota periods
5-hour remaining
Current 7-day stage remaining
1-day total remaining
7-day total remaining
30-day total remaining
Does not refresh
```

Use `bun run i18n:sync`, then replace any source-language fallback left in non-English locale files with real translations.

- [ ] **Step 7: Run frontend verification**

```bash
cd web/default
bun test src/features/value-packages/lib/period-limits.test.ts \
  src/features/order-management/components/value-package-usage-table-source.test.ts \
  src/features/order-management/components/value-package-management-page-source.test.ts
bun run i18n:sync
bun run typecheck
bun run build
```

Expected: tests, i18n sync, typecheck, and production build all succeed.

- [ ] **Step 8: Commit frontend quota rendering**

```bash
git add web/default/src/features/value-packages web/default/src/features/order-management web/default/src/i18n/locales web/default/src/features/subscriptions/lib/plan-form.ts web/default/src/features/subscriptions/lib/plan-form-value-package.test.ts
git commit -m "fix: show package lifecycle balances"
```

## Task 6: Complete package-unit verification

**Files:** all files in this plan.

- [ ] **Step 1: Run the package backend suite**

```bash
go test ./model ./controller ./service -run 'ValuePackage|Subscription|BillingSession' -count=1
```

Expected: PASS.

- [ ] **Step 2: Run race-sensitive migration and reservation tests repeatedly**

```bash
go test ./model ./service -run 'LegacyValuePackageQuotaMigration|RealtimeValuePackage' -count=10
```

Expected: PASS in all ten iterations.

- [ ] **Step 3: Inspect the final diff for forbidden fallbacks**

```bash
git diff f7358ed8...HEAD -- model controller web/default | rg 'plan\.TotalAmount.*remaining|Not applicable' || true
```

Expected: no frontend remaining-balance fallback to `plan.TotalAmount`; unrelated legitimate “Not applicable” uses may remain, but no week lifecycle quota path may use it.

- [ ] **Step 4: Record the unit as ready**

```bash
git status --short
git log --oneline --max-count=8
```

Expected: only the pre-existing untracked `docs/superpowers/specs/2026-07-08-sub2api-force-priority-server-design.md` may remain; package changes are committed.
