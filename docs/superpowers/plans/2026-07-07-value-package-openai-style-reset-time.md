# Value Package OpenAI-Style Reset Time Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add OpenAI/Codex-style “fully restored in / resets in” timing to day/week/month card 5-hour and 7-day rolling quota buckets for users, admins, and quota-limit errors without changing routing groups, billing multipliers, or rolling-window enforcement.

**Architecture:** The backend remains the authority for rolling quota usage and reset timestamps. `model/subscription.go` will compute `SUM(quota)` and `MAX(created_at)` per window in one query per bucket, return reset fields through existing value-package state/admin endpoints, and expose the same detailed window data to middleware limit messages. The frontend will add a small shared formatter and render reset text under the existing 5-hour/7-day usage bars in both user cards and the admin realtime table.

**Tech Stack:** Go 1.22+, Gin, GORM v2, SQLite/MySQL/PostgreSQL-compatible SQL; React 19, TypeScript, Rsbuild, Base UI, Tailwind, i18next; Bun for frontend scripts.

---

## Scope and Non-Goals

This plan implements the approved spec in `docs/superpowers/specs/2026-07-07-value-package-openai-style-reset-time-design.md`.

In scope:

- Return `reset_at_5h`, `reset_seconds_5h`, `reset_at_7d`, `reset_seconds_7d`, `limited_5h`, and `limited_7d` in value-package usage summaries.
- Use OpenAI-style full-window restore time:
  - `reset_at_5h = latest current-window usage.created_at + 5h`
  - `reset_at_7d = latest current-window usage.created_at + 7d`
- Show restore text under the existing user-side day/week/month package usage bars.
- Show restore text under the existing admin order-management day/week/month realtime usage bars.
- Add restore time to 5-hour/7-day middleware quota-limit errors when computable.
- Keep existing 15-second admin refresh behavior.
- Preserve the LDXP discount fix: `50 -> 47.5`, `100 -> 90`, `500 -> 425`.

Out of scope:

- Do not change the rolling-window quota algorithm.
- Do not switch to fixed reset periods, daily midnight resets, or first-use fixed cycles.
- Do not display “next partial recovery amount” as the primary UI.
- Do not change model routing groups; value packages still must not overwrite Plus/Pro distributor group selection.
- Do not change value-package billing multipliers, default activation, redemption-code behavior, rebates, order deletion, or LDXP payment flow.
- Do not edit protected branding/identity strings defined in `AGENTS.md`.

## File Map

Backend model and tests:

- Modify: `model/subscription.go`
  - Add reset/limited fields to `ValuePackageUsageSummary`.
  - Add a detailed rolling-window helper that returns used quota and latest usage timestamp.
  - Keep `GetValuePackageWindowUsage` API compatible for older callers.
  - Build reset fields in `buildValuePackageUsageSummaryTx`.
- Modify: `model/value_package_test.go`
  - Extend existing usage-window and summary tests with reset-time assertions.
  - Add no-usage and unlimited-window reset cases.

Backend middleware and tests:

- Modify: `middleware/value_package.go`
  - Use detailed window usage for quota-limit checks.
  - Add Chinese reset-duration formatting for OpenAI-compatible API error messages.
- Modify: `middleware/value_package_test.go`
  - Assert 5-hour and 7-day limit errors include restore timing.
  - Preserve routing-group and concurrency tests.

Backend controller tests:

- Modify: `controller/value_package_test.go`
  - Assert user self and activate responses include reset fields through the existing summary payload.
- Modify: `controller/order_management_test.go`
  - Assert admin realtime usage rows include reset fields.

Frontend shared formatter and tests:

- Create: `web/default/src/features/value-packages/lib/reset-time.ts`
  - Export `formatValuePackageResetTime` and `formatValuePackageResetLine`.
- Create: `web/default/src/features/value-packages/lib/reset-time.test.ts`
  - Cover fully restored, less than a minute, minutes, hours/minutes, days/hours, limit reached, and unlimited behavior.

Frontend user package UI and tests:

- Modify: `web/default/src/features/value-packages/types.ts`
  - Add reset and limited fields to `ValuePackageUsageSummary`.
- Modify: `web/default/src/features/value-packages/components/value-package-card.tsx`
  - Show reset text below 5-hour and 7-day progress bars.
  - Do not show reset text for total package limit.
- Modify: `web/default/src/features/value-packages/components/value-package-card-source.test.ts`
  - Assert the source renders reset text and passes reset fields into 5-hour/7-day rows.

Frontend admin table UI and tests:

- Modify: `web/default/src/features/order-management/types.ts`
  - Add reset and limited fields to `OrderManagementValuePackageUsageSummary`.
- Modify: `web/default/src/features/order-management/components/value-package-usage-table.tsx`
  - Show restore text under both 5-hour and 7-day admin usage bars.
- Modify: `web/default/src/features/order-management/components/value-package-usage-table-source.test.ts`
  - Assert the admin table includes reset-time formatting and reset fields.

Frontend i18n:

- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/zh.json`
- Modify: `web/default/src/i18n/locales/fr.json`
- Modify: `web/default/src/i18n/locales/ja.json`
- Modify: `web/default/src/i18n/locales/ru.json`
- Modify: `web/default/src/i18n/locales/vi.json`
  - Add all new formatter keys in every locale.
  - Run `bun run i18n:sync` after edits.

Regression guard:

- Do not modify `service/ldxp_config.go` unless a verification step proves it already changed unexpectedly.
- Verify the retained LDXP discount config before final commit:
  - `50 -> 47.5`
  - `100 -> 90`
  - `500 -> 425`
- If the user’s “200” note points to another future top-up amount, only investigate after this plan’s implementation is green; do not replace the known `500` discount config by accident.

---

## Task 1: Backend detailed rolling-window data and reset fields

**Files:**

- Modify: `model/subscription.go`
- Modify: `model/value_package_test.go`

- [ ] **Step 1: Add failing model assertions for OpenAI-style reset timestamps**

Edit `model/value_package_test.go` in `TestValuePackageRollingUsageWindows`. Keep the existing used-quota assertions, then add assertions that the detailed helper returns the latest current-window usage timestamp plus the window duration.

Use this shape in the test body after the existing usage records are inserted and `now` is defined:

```go
used5h, used7d, err := GetValuePackageWindowUsage(user.Id, sub.Id, now)
require.NoError(t, err)
require.Equal(t, int64(30), used5h)
require.Equal(t, int64(60), used7d)

details, err := GetValuePackageWindowUsageDetails(user.Id, sub.Id, now)
require.NoError(t, err)
require.Equal(t, int64(30), details.Used5h)
require.Equal(t, int64(60), details.Used7d)
require.Equal(t, now-3600, details.Latest5hCreatedAt)
require.Equal(t, now-3600, details.Latest7dCreatedAt)
require.Equal(t, now+4*3600, details.ResetAt5h)
require.Equal(t, int64(4*3600), details.ResetSeconds5h)
require.Equal(t, now+7*24*3600-3600, details.ResetAt7d)
require.Equal(t, int64(7*24*3600-3600), details.ResetSeconds7d)
```

If the existing fixture uses different record timestamps, keep the same expected formula and adapt only the literal timestamps to the fixture:

```go
expectedResetAt5h := latestUsageInCurrent5hWindow + 5*3600
expectedResetSeconds5h := expectedResetAt5h - now
expectedResetAt7d := latestUsageInCurrent7dWindow + 7*24*3600
expectedResetSeconds7d := expectedResetAt7d - now
```

- [ ] **Step 2: Add failing usage summary assertions**

Edit `TestActivateValuePackageReturnsUsageSummary` and/or `TestGetValuePackageStateIncludesUsageSummary` in `model/value_package_test.go`. Insert usage records before loading state, then assert the returned summary includes reset fields and limited flags.

Use this test pattern:

```go
now := GetDBTimestamp()
err = recordValuePackageUsageTx(DB, &ValuePackageUsageRecord{
    UserId:             user.Id,
    UserSubscriptionId: sub.Id,
    PlanId:             plan.Id,
    PackageType:        plan.PackageType,
    ModelGroup:         plan.ModelGroup,
    Quota:              25,
    RequestId:          "reset-summary-a",
    CreatedAt:          now - 2*3600,
    UpdatedAt:          now - 2*3600,
})
require.NoError(t, err)
err = recordValuePackageUsageTx(DB, &ValuePackageUsageRecord{
    UserId:             user.Id,
    UserSubscriptionId: sub.Id,
    PlanId:             plan.Id,
    PackageType:        plan.PackageType,
    ModelGroup:         plan.ModelGroup,
    Quota:              35,
    RequestId:          "reset-summary-b",
    CreatedAt:          now - 30*60,
    UpdatedAt:          now - 30*60,
})
require.NoError(t, err)

state, err := GetValuePackageState(user.Id)
require.NoError(t, err)
require.NotNil(t, state.Usage)
require.Equal(t, int64(60), state.Usage.Used5h)
require.Equal(t, now-30*60+5*3600, state.Usage.ResetAt5h)
require.Equal(t, int64(4*3600+30*60), state.Usage.ResetSeconds5h)
require.False(t, state.Usage.Limited5h)
require.Greater(t, state.Usage.ResetAt7d, int64(0))
require.Greater(t, state.Usage.ResetSeconds7d, int64(0))
```

If `GetValuePackageState` uses DB time later than the captured `now`, replace exact `ResetSeconds*` equality with a safe range:

```go
require.InDelta(t, int64(4*3600+30*60), state.Usage.ResetSeconds5h, 5)
```

- [ ] **Step 3: Add no-usage and unlimited reset assertions**

Add a new test in `model/value_package_test.go`:

```go
func TestValuePackageUsageSummaryResetFieldsForEmptyAndUnlimitedWindows(t *testing.T) {
    setupValuePackageTestDB(t)

    user := createValuePackageUser(t, 3410, UserGroupVIP)
    plan := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
    plan.TotalAmount = 1000
    plan.Limit5hAmount = 0
    plan.Limit7dAmount = 0
    require.NoError(t, DB.Save(&plan).Error)

    now := common.GetTimestamp()
    sub := createActiveValuePackageSub(t, user.Id, plan, now-10, now+86400)

    state, err := ActivateValuePackage(user.Id, sub.Id)
    require.NoError(t, err)
    require.NotNil(t, state.Usage)

    require.Equal(t, int64(0), state.Usage.Limit5h)
    require.Equal(t, int64(0), state.Usage.Limit7d)
    require.Equal(t, int64(0), state.Usage.ResetAt5h)
    require.Equal(t, int64(0), state.Usage.ResetSeconds5h)
    require.Equal(t, int64(0), state.Usage.ResetAt7d)
    require.Equal(t, int64(0), state.Usage.ResetSeconds7d)
    require.False(t, state.Usage.Limited5h)
    require.False(t, state.Usage.Limited7d)
}
```

- [ ] **Step 4: Run model tests and verify they fail for missing fields/helper**

Run:

```bash
go test ./model -run 'TestValuePackageRollingUsageWindows|TestActivateValuePackageReturnsUsageSummary|TestGetValuePackageStateIncludesUsageSummary|TestValuePackageUsageSummaryResetFieldsForEmptyAndUnlimitedWindows' -count=1 -timeout=300s
```

Expected before implementation:

```text
FAIL
undefined: GetValuePackageWindowUsageDetails
state.Usage.ResetAt5h undefined
state.Usage.ResetSeconds5h undefined
state.Usage.Limited5h undefined
```

- [ ] **Step 5: Add reset fields and detailed helper types**

Edit `model/subscription.go`. Extend `ValuePackageUsageSummary` with the new JSON-compatible fields after the existing 5h/7d percent fields:

```go
type ValuePackageUsageSummary struct {
    TotalUsed        int64   `json:"total_used"`
    TotalLimit       int64   `json:"total_limit"`
    TotalRemaining   int64   `json:"total_remaining"`
    TotalPercent     float64 `json:"total_percent"`
    Used5h           int64   `json:"used_5h"`
    Limit5h          int64   `json:"limit_5h"`
    Percent5h        float64 `json:"percent_5h"`
    ResetAt5h        int64   `json:"reset_at_5h"`
    ResetSeconds5h   int64   `json:"reset_seconds_5h"`
    Limited5h        bool    `json:"limited_5h"`
    Used7d           int64   `json:"used_7d"`
    Limit7d          int64   `json:"limit_7d"`
    Percent7d        float64 `json:"percent_7d"`
    ResetAt7d        int64   `json:"reset_at_7d"`
    ResetSeconds7d   int64   `json:"reset_seconds_7d"`
    Limited7d        bool    `json:"limited_7d"`
    Exhausted        bool    `json:"exhausted"`
    ExhaustedReason  string  `json:"exhausted_reason"`
    ExhaustedMessage string  `json:"exhausted_message"`
}
```

Add these helper structs near the existing window usage functions:

```go
type ValuePackageWindowUsageDetails struct {
    Used5h            int64
    Latest5hCreatedAt int64
    ResetAt5h         int64
    ResetSeconds5h    int64
    Used7d            int64
    Latest7dCreatedAt int64
    ResetAt7d         int64
    ResetSeconds7d    int64
}

type valuePackageWindowAggregate struct {
    Used            int64 `gorm:"column:used"`
    LatestCreatedAt int64 `gorm:"column:latest_created_at"`
}
```

- [ ] **Step 6: Implement detailed rolling-window queries**

Still in `model/subscription.go`, keep the public old function and replace the old internal aggregate implementation with this compatible detailed version:

```go
func GetValuePackageWindowUsage(userId int, userSubscriptionId int, now int64) (int64, int64, error) {
    details, err := getValuePackageWindowUsageDetailsTx(DB, userId, userSubscriptionId, now)
    if err != nil {
        return 0, 0, err
    }
    return details.Used5h, details.Used7d, nil
}

func GetValuePackageWindowUsageDetails(userId int, userSubscriptionId int, now int64) (*ValuePackageWindowUsageDetails, error) {
    return getValuePackageWindowUsageDetailsTx(DB, userId, userSubscriptionId, now)
}

func getValuePackageWindowUsageTx(tx *gorm.DB, userId int, userSubscriptionId int, now int64) (int64, int64, error) {
    details, err := getValuePackageWindowUsageDetailsTx(tx, userId, userSubscriptionId, now)
    if err != nil {
        return 0, 0, err
    }
    return details.Used5h, details.Used7d, nil
}

func getValuePackageWindowUsageDetailsTx(tx *gorm.DB, userId int, userSubscriptionId int, now int64) (*ValuePackageWindowUsageDetails, error) {
    if tx == nil {
        tx = DB
    }
    if now <= 0 {
        now = getDBTimestampTx(tx)
    }

    used5h, latest5h, err := getValuePackageWindowAggregateTx(tx, userId, userSubscriptionId, now-5*3600)
    if err != nil {
        return nil, err
    }
    used7d, latest7d, err := getValuePackageWindowAggregateTx(tx, userId, userSubscriptionId, now-7*24*3600)
    if err != nil {
        return nil, err
    }

    details := &ValuePackageWindowUsageDetails{
        Used5h:            used5h,
        Latest5hCreatedAt: latest5h,
        Used7d:            used7d,
        Latest7dCreatedAt: latest7d,
    }
    details.ResetAt5h, details.ResetSeconds5h = computeValuePackageReset(now, 5*3600, used5h, latest5h)
    details.ResetAt7d, details.ResetSeconds7d = computeValuePackageReset(now, 7*24*3600, used7d, latest7d)
    return details, nil
}

func getValuePackageWindowAggregateTx(tx *gorm.DB, userId int, userSubscriptionId int, windowStart int64) (int64, int64, error) {
    var aggregate valuePackageWindowAggregate
    err := tx.Model(&ValuePackageUsageRecord{}).
        Where("user_id = ? AND user_subscription_id = ? AND created_at >= ?", userId, userSubscriptionId, windowStart).
        Select("COALESCE(SUM(quota), 0) AS used, COALESCE(MAX(created_at), 0) AS latest_created_at").
        Scan(&aggregate).Error
    if err != nil {
        return 0, 0, err
    }
    return aggregate.Used, aggregate.LatestCreatedAt, nil
}

func computeValuePackageReset(now int64, windowSeconds int64, used int64, latestCreatedAt int64) (int64, int64) {
    if used <= 0 || latestCreatedAt <= 0 || windowSeconds <= 0 {
        return 0, 0
    }
    resetAt := latestCreatedAt + windowSeconds
    resetSeconds := resetAt - now
    if resetSeconds < 0 {
        resetSeconds = 0
    }
    return resetAt, resetSeconds
}
```

This SQL uses only `COALESCE`, `SUM`, and `MAX`, which are compatible with SQLite, MySQL, and PostgreSQL.

- [ ] **Step 7: Populate summary reset fields and limited flags**

In `buildValuePackageUsageSummaryTx`, replace:

```go
used5h, used7d, err := getValuePackageWindowUsageTx(tx, userId, sub.Id, now)
if err != nil {
    return nil, err
}
```

with:

```go
usageDetails, err := getValuePackageWindowUsageDetailsTx(tx, userId, sub.Id, now)
if err != nil {
    return nil, err
}
used5h := usageDetails.Used5h
used7d := usageDetails.Used7d
limited5h := plan.Limit5hAmount > 0 && used5h >= plan.Limit5hAmount
limited7d := plan.Limit7dAmount > 0 && used7d >= plan.Limit7dAmount
```

Then extend the summary literal:

```go
summary := &ValuePackageUsageSummary{
    TotalUsed:      sub.AmountUsed,
    TotalLimit:     sub.AmountTotal,
    TotalRemaining: totalRemaining,
    TotalPercent:   valuePackagePercent(sub.AmountUsed, sub.AmountTotal),
    Used5h:         used5h,
    Limit5h:        plan.Limit5hAmount,
    Percent5h:      valuePackagePercent(used5h, plan.Limit5hAmount),
    ResetAt5h:      0,
    ResetSeconds5h: 0,
    Limited5h:      limited5h,
    Used7d:         used7d,
    Limit7d:        plan.Limit7dAmount,
    Percent7d:      valuePackagePercent(used7d, plan.Limit7dAmount),
    ResetAt7d:      0,
    ResetSeconds7d: 0,
    Limited7d:      limited7d,
}
if plan.Limit5hAmount > 0 {
    summary.ResetAt5h = usageDetails.ResetAt5h
    summary.ResetSeconds5h = usageDetails.ResetSeconds5h
}
if plan.Limit7dAmount > 0 {
    summary.ResetAt7d = usageDetails.ResetAt7d
    summary.ResetSeconds7d = usageDetails.ResetSeconds7d
}
```

Replace the exhausted switch limit checks with the new booleans:

```go
switch {
case sub.AmountTotal > 0 && sub.AmountUsed >= sub.AmountTotal:
    summary.Exhausted = true
    summary.ExhaustedReason = ValuePackageExhaustedReasonTotal
case limited5h:
    summary.Exhausted = true
    summary.ExhaustedReason = ValuePackageExhaustedReason5h
case limited7d:
    summary.Exhausted = true
    summary.ExhaustedReason = ValuePackageExhaustedReason7d
}
```

- [ ] **Step 8: Run model tests and verify they pass**

Run:

```bash
go test ./model -run 'TestValuePackageRollingUsageWindows|TestActivateValuePackageReturnsUsageSummary|TestGetValuePackageStateIncludesUsageSummary|TestValuePackageUsageSummaryResetFieldsForEmptyAndUnlimitedWindows|TestListActiveValuePackageUsageRowsReturnsRealtimeWindowUsage' -count=1 -timeout=300s
```

Expected:

```text
ok  	github.com/QuantumNous/new-api/model	...
```

- [ ] **Step 9: Commit Task 1**

Run:

```bash
git add model/subscription.go model/value_package_test.go
git commit -m "feat: add value package reset usage fields"
```

Expected:

```text
[main <hash>] feat: add value package reset usage fields
```

---

## Task 2: Middleware quota-limit restore messages

**Files:**

- Modify: `middleware/value_package.go`
- Modify: `middleware/value_package_test.go`

- [ ] **Step 1: Add failing middleware assertions for reset wording**

Edit `middleware/value_package_test.go`. In `TestValuePackageMiddlewareRejectsOverRollingWindows`, after the request is rejected, assert the response body includes the reset message. Use existing response decoding helpers if present; otherwise inspect `rec.Body.String()`.

Add assertions like:

```go
body := rec.Body.String()
require.Contains(t, body, "超值套餐额度已用完")
require.Contains(t, body, "5 小时")
require.Contains(t, body, "将在")
require.Contains(t, body, "后恢复")
```

Add a separate 7-day case if the existing test only covers 5-hour exhaustion:

```go
func TestValuePackageMiddlewareRejectsOverSevenDayWindowWithResetMessage(t *testing.T) {
    setupValuePackageMiddlewareTestDB(t)

    user, plan, sub := seedValuePackageMiddlewareState(t, true, 0, 50, 1)
    now := common.GetTimestamp()
    require.NoError(t, model.RecordValuePackageUsage(&model.ValuePackageUsageRecord{
        UserId:             user.Id,
        UserSubscriptionId: sub.Id,
        PlanId:             plan.Id,
        PackageType:        plan.PackageType,
        ModelGroup:         plan.ModelGroup,
        Quota:              50,
        RequestId:          "vp-7d-reset-limit",
        CreatedAt:          now - int64(2*time.Hour/time.Second),
    }))

    rec := runValuePackageMiddlewareRequest(t, user.Id, "gpt-plus")
    require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
    body := rec.Body.String()
    require.Contains(t, body, "7 天")
    require.Contains(t, body, "将在")
    require.Contains(t, body, "后恢复")
}
```

- [ ] **Step 2: Run middleware tests and verify they fail**

Run:

```bash
go test ./middleware -run 'TestValuePackageMiddlewareRejectsOverRollingWindows|TestValuePackageMiddlewareRejectsOverSevenDayWindowWithResetMessage|TestValuePackageRealtimeRejectsOverRollingWindows' -count=1 -timeout=300s
```

Expected before implementation:

```text
FAIL
Error: "..." does not contain "将在"
```

- [ ] **Step 3: Add duration formatting helper**

Edit `middleware/value_package.go`. Add these helpers near other value-package helper functions:

```go
func formatValuePackageResetDuration(seconds int64) string {
    if seconds <= 0 {
        return "不到 1 分钟"
    }
    if seconds < 60 {
        return "不到 1 分钟"
    }

    minutes := (seconds + 59) / 60
    if minutes < 60 {
        return fmt.Sprintf("%d 分钟", minutes)
    }

    hours := minutes / 60
    remainingMinutes := minutes % 60
    if hours < 24 {
        if remainingMinutes == 0 {
            return fmt.Sprintf("%d 小时", hours)
        }
        return fmt.Sprintf("%d 小时 %d 分钟", hours, remainingMinutes)
    }

    days := hours / 24
    remainingHours := hours % 24
    if remainingHours == 0 {
        return fmt.Sprintf("%d 天", days)
    }
    return fmt.Sprintf("%d 天 %d 小时", days, remainingHours)
}

func formatValuePackageLimitMessage(windowLabel string, used int64, limit int64, resetSeconds int64) string {
    if resetSeconds > 0 {
        return fmt.Sprintf(
            "%s（%s：已用 %d / 限额 %d，将在 %s后恢复）",
            model.ValuePackageQuotaExhaustedUserMessage,
            windowLabel,
            used,
            limit,
            formatValuePackageResetDuration(resetSeconds),
        )
    }
    return fmt.Sprintf(
        "%s（%s：已用 %d / 限额 %d）",
        model.ValuePackageQuotaExhaustedUserMessage,
        windowLabel,
        used,
        limit,
    )
}
```

The file already imports `fmt`; do not add a duplicate import.

- [ ] **Step 4: Use detailed usage in limit checks**

In `ValuePackageEntitlement`, replace:

```go
used5h, used7d, err := model.GetValuePackageWindowUsage(userId, state.Subscription.Id, now)
if err != nil {
    abortWithOpenAiMessage(c, http.StatusInternalServerError, "查询超值套餐用量失败")
    return
}
if state.Plan.Limit5hAmount > 0 && used5h >= state.Plan.Limit5hAmount {
    abortWithOpenAiMessage(c, http.StatusForbidden, fmt.Sprintf("%s（5 小时：已用 %d / 限额 %d）", model.ValuePackageQuotaExhaustedUserMessage, used5h, state.Plan.Limit5hAmount))
    return
}
if state.Plan.Limit7dAmount > 0 && used7d >= state.Plan.Limit7dAmount {
    abortWithOpenAiMessage(c, http.StatusForbidden, fmt.Sprintf("%s（7 天：已用 %d / 限额 %d）", model.ValuePackageQuotaExhaustedUserMessage, used7d, state.Plan.Limit7dAmount))
    return
}
```

with:

```go
usageDetails, err := model.GetValuePackageWindowUsageDetails(userId, state.Subscription.Id, now)
if err != nil {
    abortWithOpenAiMessage(c, http.StatusInternalServerError, "查询超值套餐用量失败")
    return
}
if state.Plan.Limit5hAmount > 0 && usageDetails.Used5h >= state.Plan.Limit5hAmount {
    abortWithOpenAiMessage(c, http.StatusForbidden, formatValuePackageLimitMessage(
        "5 小时",
        usageDetails.Used5h,
        state.Plan.Limit5hAmount,
        usageDetails.ResetSeconds5h,
    ))
    return
}
if state.Plan.Limit7dAmount > 0 && usageDetails.Used7d >= state.Plan.Limit7dAmount {
    abortWithOpenAiMessage(c, http.StatusForbidden, formatValuePackageLimitMessage(
        "7 天",
        usageDetails.Used7d,
        state.Plan.Limit7dAmount,
        usageDetails.ResetSeconds7d,
    ))
    return
}
```

- [ ] **Step 5: Run middleware tests and verify they pass**

Run:

```bash
go test ./middleware -run 'TestValuePackageMiddlewareRejectsOverRollingWindows|TestValuePackageMiddlewareRejectsOverSevenDayWindowWithResetMessage|TestValuePackageRealtimeRejectsOverRollingWindows|TestValuePackagePlaygroundDistributeKeepsRequestedModelGroupWhenPackageActive|TestValuePackageRelayDistributeKeepsTokenModelGroupWhenPackageActive|TestValuePackageScopeDoesNotOverwriteRoutingOrUserGroups' -count=1 -timeout=300s
```

Expected:

```text
ok  	github.com/QuantumNous/new-api/middleware	...
```

- [ ] **Step 6: Commit Task 2**

Run:

```bash
git add middleware/value_package.go middleware/value_package_test.go
git commit -m "feat: show value package reset in limit errors"
```

Expected:

```text
[main <hash>] feat: show value package reset in limit errors
```

---

## Task 3: Controller response coverage

**Files:**

- Modify: `controller/value_package_test.go`
- Modify: `controller/order_management_test.go`

- [ ] **Step 1: Add user endpoint reset response assertions**

In `controller/value_package_test.go`, extend `TestGetValuePackageSelfReturnsCurrentState` after the response is decoded. Add records before the request if the test currently has no usage.

Use existing response struct/JSON helper patterns in the file. If decoding into `model.ValuePackageState`, assert:

```go
require.NotNil(t, response.Data.State.Usage)
assert.Contains(t, rec.Body.String(), "reset_at_5h")
assert.Contains(t, rec.Body.String(), "reset_seconds_5h")
assert.Contains(t, rec.Body.String(), "reset_at_7d")
assert.Contains(t, rec.Body.String(), "reset_seconds_7d")
assert.Contains(t, rec.Body.String(), "limited_5h")
assert.Contains(t, rec.Body.String(), "limited_7d")
assert.GreaterOrEqual(t, response.Data.State.Usage.ResetSeconds5h, int64(0))
assert.GreaterOrEqual(t, response.Data.State.Usage.ResetSeconds7d, int64(0))
```

If the endpoint response does not wrap the state as `response.Data.State`, use the existing decoded field path and keep the exact same assertions against the decoded `Usage` object.

- [ ] **Step 2: Add activate endpoint reset response assertions**

In `TestActivateAndDeactivateValuePackageAPI`, after activating, assert the activation response includes the new fields:

```go
body := rec.Body.String()
assert.Contains(t, body, "reset_at_5h")
assert.Contains(t, body, "reset_seconds_5h")
assert.Contains(t, body, "reset_at_7d")
assert.Contains(t, body, "reset_seconds_7d")
assert.Contains(t, body, "limited_5h")
assert.Contains(t, body, "limited_7d")
```

This keeps the test robust even if the endpoint response wrapper uses generic maps.

- [ ] **Step 3: Add admin realtime usage response assertions**

In `controller/order_management_test.go`, extend `TestAdminOrderManagementValuePackageUsageReturnsActiveUsers`. After the admin endpoint response, assert the JSON includes all reset fields:

```go
body := rec.Body.String()
assert.Contains(t, body, "reset_at_5h")
assert.Contains(t, body, "reset_seconds_5h")
assert.Contains(t, body, "reset_at_7d")
assert.Contains(t, body, "reset_seconds_7d")
assert.Contains(t, body, "limited_5h")
assert.Contains(t, body, "limited_7d")
```

If the test decodes rows, also add decoded assertions:

```go
require.NotEmpty(t, response.Data)
require.NotNil(t, response.Data[0].Usage)
assert.GreaterOrEqual(t, response.Data[0].Usage.ResetSeconds5h, int64(0))
assert.GreaterOrEqual(t, response.Data[0].Usage.ResetSeconds7d, int64(0))
```

- [ ] **Step 4: Run controller tests**

Run:

```bash
go test ./controller -run 'TestGetValuePackageSelfReturnsCurrentState|TestActivateAndDeactivateValuePackageAPI|TestAdminOrderManagementValuePackageUsageReturnsActiveUsers' -count=1 -timeout=300s
```

Expected:

```text
ok  	github.com/QuantumNous/new-api/controller	...
```

- [ ] **Step 5: Commit Task 3**

Run:

```bash
git add controller/value_package_test.go controller/order_management_test.go
git commit -m "test: cover value package reset API fields"
```

Expected:

```text
[main <hash>] test: cover value package reset API fields
```

---

## Task 4: Frontend shared reset-time formatter

**Files:**

- Create: `web/default/src/features/value-packages/lib/reset-time.ts`
- Create: `web/default/src/features/value-packages/lib/reset-time.test.ts`

- [ ] **Step 1: Write failing formatter tests**

Create `web/default/src/features/value-packages/lib/reset-time.test.ts`:

```ts
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  formatValuePackageResetLine,
  formatValuePackageResetTime,
} from './reset-time'

const t = (key: string, values?: Record<string, unknown>) => {
  if (!values) return key
  return Object.entries(values).reduce(
    (text, [name, value]) => text.replace(`{{${name}}}`, String(value)),
    key
  )
}

describe('formatValuePackageResetTime', () => {
  test('returns fully restored for empty reset seconds', () => {
    assert.equal(formatValuePackageResetTime(0, t), 'Fully restored')
    assert.equal(formatValuePackageResetTime(-10, t), 'Fully restored')
  })

  test('formats less than one minute', () => {
    assert.equal(
      formatValuePackageResetTime(45, t),
      'less than 1 minute'
    )
  })

  test('formats minutes with ceiling rounding', () => {
    assert.equal(formatValuePackageResetTime(61, t), '2 minutes')
  })

  test('formats hours and minutes', () => {
    assert.equal(
      formatValuePackageResetTime(3 * 3600 + 15 * 60, t),
      '3 hours 15 minutes'
    )
  })

  test('formats days and hours', () => {
    assert.equal(
      formatValuePackageResetTime(4 * 24 * 3600 + 6 * 3600 + 30, t),
      '4 days 6 hours'
    )
  })
})

describe('formatValuePackageResetLine', () => {
  test('returns unlimited for non-positive limits', () => {
    assert.equal(formatValuePackageResetLine({ limit: 0, resetSeconds: 0, limited: false, t }), 'Unlimited')
  })

  test('returns fully restored for positive limit with no active window usage', () => {
    assert.equal(formatValuePackageResetLine({ limit: 60, resetSeconds: 0, limited: false, t }), 'Fully restored')
  })

  test('returns reset line when not limited', () => {
    assert.equal(
      formatValuePackageResetLine({ limit: 60, resetSeconds: 3600, limited: false, t }),
      'Resets in 1 hour'
    )
  })

  test('returns limit reached reset line when limited', () => {
    assert.equal(
      formatValuePackageResetLine({ limit: 60, resetSeconds: 3600, limited: true, t }),
      'Limit reached · Resets in 1 hour'
    )
  })
})
```

If Node’s test runner rejects JSX/TS path resolution in this repo, keep the same assertions but follow the existing `*-source.test.ts` style in the frontend test directory.

- [ ] **Step 2: Run formatter test and verify it fails**

Run:

```bash
cd web/default
bun test src/features/value-packages/lib/reset-time.test.ts
```

Expected before implementation:

```text
FAIL
Cannot find module './reset-time'
```

- [ ] **Step 3: Implement formatter module**

Create `web/default/src/features/value-packages/lib/reset-time.ts`:

```ts
import type { TFunction } from 'i18next'

type Translate = TFunction | ((key: string, values?: Record<string, unknown>) => string)

function translate(t: Translate, key: string, values?: Record<string, unknown>) {
  return t(key, values as never)
}

export function formatValuePackageResetTime(
  seconds: number | null | undefined,
  t: Translate
): string {
  const rawSeconds = Number(seconds || 0)
  if (!Number.isFinite(rawSeconds) || rawSeconds <= 0) {
    return translate(t, 'Fully restored')
  }

  if (rawSeconds < 60) {
    return translate(t, 'less than 1 minute')
  }

  const minutes = Math.ceil(rawSeconds / 60)
  if (minutes < 60) {
    return translate(t, '{{count}} minutes', { count: minutes })
  }

  const hours = Math.floor(minutes / 60)
  const remainingMinutes = minutes % 60
  if (hours < 24) {
    if (remainingMinutes === 0) {
      return translate(t, '{{count}} hour', { count: hours })
    }
    return translate(t, '{{hours}} hours {{minutes}} minutes', {
      hours,
      minutes: remainingMinutes,
    })
  }

  const days = Math.floor(hours / 24)
  const remainingHours = hours % 24
  if (remainingHours === 0) {
    return translate(t, '{{count}} day', { count: days })
  }
  return translate(t, '{{days}} days {{hours}} hours', {
    days,
    hours: remainingHours,
  })
}

export function formatValuePackageResetLine({
  limit,
  resetSeconds,
  limited,
  t,
}: {
  limit: number | null | undefined
  resetSeconds: number | null | undefined
  limited: boolean | null | undefined
  t: Translate
}): string {
  const numericLimit = Number(limit || 0)
  if (!Number.isFinite(numericLimit) || numericLimit <= 0) {
    return translate(t, 'Unlimited')
  }

  const resetTime = formatValuePackageResetTime(resetSeconds, t)
  if (resetTime === translate(t, 'Fully restored')) {
    return resetTime
  }

  const resetLine = translate(t, 'Resets in {{time}}', { time: resetTime })
  if (limited) {
    return translate(t, 'Limit reached · {{reset}}', { reset: resetLine })
  }
  return resetLine
}
```

- [ ] **Step 4: Run formatter test and verify it passes**

Run:

```bash
cd web/default
bun test src/features/value-packages/lib/reset-time.test.ts
```

Expected:

```text
pass
```

- [ ] **Step 5: Commit Task 4**

Run:

```bash
git add web/default/src/features/value-packages/lib/reset-time.ts web/default/src/features/value-packages/lib/reset-time.test.ts
git commit -m "feat: add value package reset time formatter"
```

Expected:

```text
[main <hash>] feat: add value package reset time formatter
```

---

## Task 5: User value-package card reset display

**Files:**

- Modify: `web/default/src/features/value-packages/types.ts`
- Modify: `web/default/src/features/value-packages/components/value-package-card.tsx`
- Modify: `web/default/src/features/value-packages/components/value-package-card-source.test.ts`

- [ ] **Step 1: Add TypeScript usage summary fields**

Edit `web/default/src/features/value-packages/types.ts`. Update `ValuePackageUsageSummary` to include the new backend fields:

```ts
export interface ValuePackageUsageSummary {
  total_used: number
  total_limit: number
  total_remaining: number
  total_percent: number
  used_5h: number
  limit_5h: number
  percent_5h: number
  reset_at_5h: number
  reset_seconds_5h: number
  limited_5h: boolean
  used_7d: number
  limit_7d: number
  percent_7d: number
  reset_at_7d: number
  reset_seconds_7d: number
  limited_7d: boolean
  exhausted: boolean
  exhausted_reason: string
  exhausted_message: string
}
```

- [ ] **Step 2: Add failing source test checks for reset UI**

Edit `web/default/src/features/value-packages/components/value-package-card-source.test.ts`. Add assertions that the component imports and uses the reset formatter and reset fields:

```ts
assert.match(source, /formatValuePackageResetLine/)
assert.match(source, /resetSeconds\?: number/)
assert.match(source, /limited\?: boolean/)
assert.match(source, /reset_seconds_5h/)
assert.match(source, /reset_seconds_7d/)
assert.match(source, /limited_5h/)
assert.match(source, /limited_7d/)
```

Also assert total-limit usage does not pass reset fields:

```ts
assert.match(source, /label=\{t\('Package total limit'\)\}[\s\S]*?percent=\{usage\.total_percent\}/)
```

- [ ] **Step 3: Run source test and verify it fails**

Run:

```bash
cd web/default
bun test src/features/value-packages/components/value-package-card-source.test.ts
```

Expected before UI implementation:

```text
FAIL
Input does not match regular expression /formatValuePackageResetLine/
```

- [ ] **Step 4: Render reset text in `LimitProgressRow`**

Edit `web/default/src/features/value-packages/components/value-package-card.tsx`.

Add import:

```ts
import { formatValuePackageResetLine } from '../lib/reset-time'
```

If the existing relative path differs because of current imports, use the path from `components/` to `lib/`:

```ts
import { formatValuePackageResetLine } from '../lib/reset-time'
```

Update `LimitProgressRow`:

```tsx
function LimitProgressRow({
  label,
  used,
  limit,
  percent,
  resetSeconds,
  limited,
  showReset = false,
}: {
  label: string
  used: number
  limit: number
  percent: number
  resetSeconds?: number
  limited?: boolean
  showReset?: boolean
}) {
  const { t } = useTranslation()

  if (!Number.isFinite(limit) || limit <= 0) {
    return null
  }

  const progressPercent = clampPercent(percent)
  const resetLine = showReset
    ? formatValuePackageResetLine({
        limit,
        resetSeconds,
        limited,
        t,
      })
    : ''

  return (
    <div className='flex flex-col gap-1.5'>
      <div className='flex items-center justify-between gap-3 text-xs'>
        <span className='text-muted-foreground font-medium'>{label}</span>
        <span className='text-muted-foreground tabular-nums'>
          {formatUsageAmount(used)} / {formatUsageAmount(limit)}
        </span>
      </div>
      <Progress
        value={progressPercent}
        className={cn('h-1.5', getProgressToneClass(progressPercent))}
      />
      {resetLine ? (
        <div
          className={cn(
            'text-xs tabular-nums',
            limited ? 'text-destructive font-medium' : 'text-muted-foreground'
          )}
        >
          {resetLine}
        </div>
      ) : null}
    </div>
  )
}
```

This component already imports `useTranslation` elsewhere; if it is not in scope, add:

```ts
import { useTranslation } from 'react-i18next'
```

but do not duplicate the import if it already exists.

- [ ] **Step 5: Pass reset props for 5-hour and 7-day rows only**

In the same file, update the usage rows:

```tsx
<LimitProgressRow
  label={t('Package total limit')}
  used={usage.total_used}
  limit={usage.total_limit}
  percent={usage.total_percent}
/>
<LimitProgressRow
  label={t('5-hour limit')}
  used={usage.used_5h}
  limit={usage.limit_5h}
  percent={usage.percent_5h}
  resetSeconds={usage.reset_seconds_5h}
  limited={usage.limited_5h}
  showReset
/>
<LimitProgressRow
  label={t('7-day limit')}
  used={usage.used_7d}
  limit={usage.limit_7d}
  percent={usage.percent_7d}
  resetSeconds={usage.reset_seconds_7d}
  limited={usage.limited_7d}
  showReset
/>
```

- [ ] **Step 6: Run user card source test**

Run:

```bash
cd web/default
bun test src/features/value-packages/components/value-package-card-source.test.ts
```

Expected:

```text
pass
```

- [ ] **Step 7: Run TypeScript check for the changed frontend package**

Run:

```bash
cd web/default
bun run typecheck
```

Expected:

```text
No errors
```

If typecheck reports missing required reset fields in frontend fixtures, update those fixtures with:

```ts
reset_at_5h: 0,
reset_seconds_5h: 0,
limited_5h: false,
reset_at_7d: 0,
reset_seconds_7d: 0,
limited_7d: false,
```

- [ ] **Step 8: Commit Task 5**

Run:

```bash
git add web/default/src/features/value-packages/types.ts web/default/src/features/value-packages/components/value-package-card.tsx web/default/src/features/value-packages/components/value-package-card-source.test.ts
git commit -m "feat: show value package reset on user cards"
```

Expected:

```text
[main <hash>] feat: show value package reset on user cards
```

---

## Task 6: Admin order-management realtime table reset display

**Files:**

- Modify: `web/default/src/features/order-management/types.ts`
- Modify: `web/default/src/features/order-management/components/value-package-usage-table.tsx`
- Modify: `web/default/src/features/order-management/components/value-package-usage-table-source.test.ts`

- [ ] **Step 1: Add TypeScript admin summary fields**

Edit `web/default/src/features/order-management/types.ts`. Find `OrderManagementValuePackageUsageSummary` and add the new fields next to the existing 5-hour/7-day fields:

```ts
export interface OrderManagementValuePackageUsageSummary {
  total_used: number
  total_limit: number
  total_remaining: number
  total_percent: number
  used_5h: number
  limit_5h: number
  percent_5h: number
  reset_at_5h: number
  reset_seconds_5h: number
  limited_5h: boolean
  used_7d: number
  limit_7d: number
  percent_7d: number
  reset_at_7d: number
  reset_seconds_7d: number
  limited_7d: boolean
  exhausted: boolean
  exhausted_reason: string
  exhausted_message: string
}
```

If this interface has additional existing fields, preserve them and add only the six new reset/limited fields.

- [ ] **Step 2: Add failing admin table source assertions**

Edit `web/default/src/features/order-management/components/value-package-usage-table-source.test.ts`:

```ts
assert.match(source, /formatValuePackageResetLine/)
assert.match(source, /resetSeconds\?: number/)
assert.match(source, /limited\?: boolean/)
assert.match(source, /reset_seconds_5h/)
assert.match(source, /reset_seconds_7d/)
assert.match(source, /limited_5h/)
assert.match(source, /limited_7d/)
assert.match(source, /Auto-refresh every 15 seconds/)
```

- [ ] **Step 3: Run admin source test and verify it fails**

Run:

```bash
cd web/default
bun test src/features/order-management/components/value-package-usage-table-source.test.ts
```

Expected before implementation:

```text
FAIL
Input does not match regular expression /formatValuePackageResetLine/
```

- [ ] **Step 4: Render reset text under admin usage bars**

Edit `web/default/src/features/order-management/components/value-package-usage-table.tsx`.

Add import:

```ts
import { formatValuePackageResetLine } from '@/features/value-packages/lib/reset-time'
```

Update `WindowQuotaCell`:

```tsx
function WindowQuotaCell({
  used,
  limit,
  percent,
  resetSeconds,
  limited,
}: {
  used: number
  limit: number
  percent: number
  resetSeconds?: number
  limited?: boolean
}) {
  const { t } = useTranslation()
  const remaining = remainingQuota(used, limit)

  if (remaining === null) {
    return <span className='font-semibold'>{t('Unlimited')}</span>
  }

  const resetLine = formatValuePackageResetLine({
    limit,
    resetSeconds,
    limited,
    t,
  })

  return (
    <div className='flex min-w-[160px] flex-col gap-1.5'>
      <div className='font-semibold tabular-nums'>
        {formatQuota(remaining)} / {formatQuota(limit)}
      </div>
      <Progress value={Math.min(Math.max(percent || 0, 0), 100)} />
      <div className='text-muted-foreground text-xs tabular-nums'>
        {t('Used: {{used}} / {{limit}}', {
          used: formatQuota(used || 0),
          limit: formatQuota(limit),
        })}
      </div>
      <div
        className={cn(
          'text-xs tabular-nums',
          limited ? 'text-destructive font-medium' : 'text-muted-foreground'
        )}
      >
        {resetLine}
      </div>
    </div>
  )
}
```

If `cn` is not already imported in this file, add:

```ts
import { cn } from '@/lib/utils'
```

- [ ] **Step 5: Pass reset fields into admin 5-hour and 7-day cells**

In the same file, update the `WindowQuotaCell` calls:

```tsx
<WindowQuotaCell
  used={usage?.used_5h || 0}
  limit={usage?.limit_5h || 0}
  percent={usage?.percent_5h || 0}
  resetSeconds={usage?.reset_seconds_5h || 0}
  limited={usage?.limited_5h || false}
/>
```

and:

```tsx
<WindowQuotaCell
  used={usage?.used_7d || 0}
  limit={usage?.limit_7d || 0}
  percent={usage?.percent_7d || 0}
  resetSeconds={usage?.reset_seconds_7d || 0}
  limited={usage?.limited_7d || false}
/>
```

- [ ] **Step 6: Run admin source test and typecheck**

Run:

```bash
cd web/default
bun test src/features/order-management/components/value-package-usage-table-source.test.ts
bun run typecheck
```

Expected:

```text
pass
No errors
```

If typecheck reports missing reset fields in admin test fixtures, update those fixtures with:

```ts
reset_at_5h: 0,
reset_seconds_5h: 0,
limited_5h: false,
reset_at_7d: 0,
reset_seconds_7d: 0,
limited_7d: false,
```

- [ ] **Step 7: Commit Task 6**

Run:

```bash
git add web/default/src/features/order-management/types.ts web/default/src/features/order-management/components/value-package-usage-table.tsx web/default/src/features/order-management/components/value-package-usage-table-source.test.ts
git commit -m "feat: show value package reset in admin usage table"
```

Expected:

```text
[main <hash>] feat: show value package reset in admin usage table
```

---

## Task 7: Frontend i18n keys

**Files:**

- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/zh.json`
- Modify: `web/default/src/i18n/locales/fr.json`
- Modify: `web/default/src/i18n/locales/ja.json`
- Modify: `web/default/src/i18n/locales/ru.json`
- Modify: `web/default/src/i18n/locales/vi.json`

- [ ] **Step 1: Add i18n keys to all locale files**

Add these keys if absent:

English (`en.json`):

```json
"Fully restored": "Fully restored",
"less than 1 minute": "less than 1 minute",
"{{count}} minutes": "{{count}} minutes",
"{{count}} hour": "{{count}} hour",
"{{hours}} hours {{minutes}} minutes": "{{hours}} hours {{minutes}} minutes",
"{{count}} day": "{{count}} day",
"{{days}} days {{hours}} hours": "{{days}} days {{hours}} hours",
"Resets in {{time}}": "Resets in {{time}}",
"Limit reached · {{reset}}": "Limit reached · {{reset}}"
```

Chinese (`zh.json`):

```json
"Fully restored": "已完全恢复",
"less than 1 minute": "不到 1 分钟",
"{{count}} minutes": "{{count}} 分钟",
"{{count}} hour": "{{count}} 小时",
"{{hours}} hours {{minutes}} minutes": "{{hours}} 小时 {{minutes}} 分钟",
"{{count}} day": "{{count}} 天",
"{{days}} days {{hours}} hours": "{{days}} 天 {{hours}} 小时",
"Resets in {{time}}": "将在 {{time}} 后恢复",
"Limit reached · {{reset}}": "已达上限 · {{reset}}"
```

French (`fr.json`):

```json
"Fully restored": "Entièrement restauré",
"less than 1 minute": "moins d’une minute",
"{{count}} minutes": "{{count}} minutes",
"{{count}} hour": "{{count}} heure",
"{{hours}} hours {{minutes}} minutes": "{{hours}} heures {{minutes}} minutes",
"{{count}} day": "{{count}} jour",
"{{days}} days {{hours}} hours": "{{days}} jours {{hours}} heures",
"Resets in {{time}}": "Réinitialisation dans {{time}}",
"Limit reached · {{reset}}": "Limite atteinte · {{reset}}"
```

Japanese (`ja.json`):

```json
"Fully restored": "完全に回復済み",
"less than 1 minute": "1 分未満",
"{{count}} minutes": "{{count}} 分",
"{{count}} hour": "{{count}} 時間",
"{{hours}} hours {{minutes}} minutes": "{{hours}} 時間 {{minutes}} 分",
"{{count}} day": "{{count}} 日",
"{{days}} days {{hours}} hours": "{{days}} 日 {{hours}} 時間",
"Resets in {{time}}": "{{time}}後に回復",
"Limit reached · {{reset}}": "上限に達しました · {{reset}}"
```

Russian (`ru.json`):

```json
"Fully restored": "Полностью восстановлено",
"less than 1 minute": "менее 1 минуты",
"{{count}} minutes": "{{count}} мин.",
"{{count}} hour": "{{count}} ч.",
"{{hours}} hours {{minutes}} minutes": "{{hours}} ч. {{minutes}} мин.",
"{{count}} day": "{{count}} дн.",
"{{days}} days {{hours}} hours": "{{days}} дн. {{hours}} ч.",
"Resets in {{time}}": "Восстановится через {{time}}",
"Limit reached · {{reset}}": "Лимит достигнут · {{reset}}"
```

Vietnamese (`vi.json`):

```json
"Fully restored": "Đã khôi phục hoàn toàn",
"less than 1 minute": "dưới 1 phút",
"{{count}} minutes": "{{count}} phút",
"{{count}} hour": "{{count}} giờ",
"{{hours}} hours {{minutes}} minutes": "{{hours}} giờ {{minutes}} phút",
"{{count}} day": "{{count}} ngày",
"{{days}} days {{hours}} hours": "{{days}} ngày {{hours}} giờ",
"Resets in {{time}}": "Khôi phục sau {{time}}",
"Limit reached · {{reset}}": "Đã đạt giới hạn · {{reset}}"
```

Keep each JSON object valid and sorted according to the repository’s current locale-file convention.

- [ ] **Step 2: Run i18n sync**

Run:

```bash
cd web/default
bun run i18n:sync
```

Expected:

```text
missing keys: 0
```

If the script prints a different success phrase, accept the repository’s success output as long as it does not report missing keys or invalid JSON.

- [ ] **Step 3: Run frontend formatter/card/table tests together**

Run:

```bash
cd web/default
bun test \
  src/features/value-packages/lib/reset-time.test.ts \
  src/features/value-packages/components/value-package-card-source.test.ts \
  src/features/order-management/components/value-package-usage-table-source.test.ts
```

Expected:

```text
pass
```

- [ ] **Step 4: Commit Task 7**

Run:

```bash
git add web/default/src/i18n/locales/en.json web/default/src/i18n/locales/zh.json web/default/src/i18n/locales/fr.json web/default/src/i18n/locales/ja.json web/default/src/i18n/locales/ru.json web/default/src/i18n/locales/vi.json
git commit -m "chore: translate value package reset labels"
```

Expected:

```text
[main <hash>] chore: translate value package reset labels
```

---

## Task 8: LDXP discount regression guard and focused full verification

**Files:**

- Read/verify only: `service/ldxp_config.go`
- Read/verify only: `service/ldxp_config_test.go`
- Read/verify only: `service/ldxp_verify_test.go`
- Read/verify only: `workers/ldxp-browser-worker/tests/browser-flow.test.ts`

- [ ] **Step 1: Verify the LDXP discount config has not been overwritten**

Run:

```bash
rg -n '"amount":50,"money":47\.5|"amount":100,"money":90|"amount":500,"money":425' service/ldxp_config.go
```

Expected lines include exactly:

```text
"amount":50,"money":47.5
"amount":100,"money":90
"amount":500,"money":425
```

If this fails, restore only the LDXP default config to:

```go
const defaultLdxpProductsJSON = `[
  {"amount":10,"money":10,"product_url":"https://pay.ldxp.cn/item/7nedvm","product_name":"LDXP 10"},
  {"amount":20,"money":20,"product_url":"https://pay.ldxp.cn/item/tgh5wb","product_name":"LDXP 20"},
  {"amount":30,"money":30,"product_url":"https://pay.ldxp.cn/item/4p4ppn","product_name":"LDXP 30"},
  {"amount":50,"money":47.5,"product_url":"https://pay.ldxp.cn/item/5c4yft","product_name":"LDXP 50"},
  {"amount":100,"money":90,"product_url":"https://pay.ldxp.cn/item/sb48mz","product_name":"LDXP 100"},
  {"amount":500,"money":425,"product_url":"https://pay.ldxp.cn/item/y8t52c","product_name":"LDXP 500"}
]`
```

Do not introduce a `200` amount unless there is a separate verified source and a separate user-approved change.

- [ ] **Step 2: Run LDXP focused tests**

Run:

```bash
go test ./service -run 'TestLoadLdxpConfigDefaultProductsUseRealSixLinks|TestVerifyLdxpWorkerPaidFieldsAllowsCardNetworkFee' -count=1 -timeout=300s
```

Expected:

```text
ok  	github.com/QuantumNous/new-api/service	...
```

- [ ] **Step 3: Run focused backend value-package verification**

Run:

```bash
go test ./model ./middleware ./controller -run 'ValuePackage|OrderManagement' -count=1 -timeout=300s
```

Expected:

```text
ok  	github.com/QuantumNous/new-api/model	...
ok  	github.com/QuantumNous/new-api/middleware	...
ok  	github.com/QuantumNous/new-api/controller	...
```

- [ ] **Step 4: Run focused frontend verification**

Run:

```bash
cd web/default
bun test \
  src/features/value-packages/lib/reset-time.test.ts \
  src/features/value-packages/components/value-package-card-source.test.ts \
  src/features/order-management/components/value-package-usage-table-source.test.ts \
  src/features/order-management/order-management-source.test.ts
bun run typecheck
```

Expected:

```text
pass
No errors
```

- [ ] **Step 5: Run production build smoke test**

Run:

```bash
cd web/default
bun run build
```

Expected:

```text
Build completed
```

Accept the repository’s exact success phrase if the command exits `0`.

- [ ] **Step 6: Inspect final diff for forbidden side effects**

Run:

```bash
git diff --stat HEAD~7..HEAD
git diff -- service/ldxp_config.go
git diff -- model/subscription.go middleware/value_package.go web/default/src/features/value-packages/components/value-package-card.tsx web/default/src/features/order-management/components/value-package-usage-table.tsx
```

Expected:

- `service/ldxp_config.go` has no diff unless it was restored to `50=47.5`, `100=90`, `500=425`.
- `model/subscription.go` only adds reset usage details/fields and keeps old function compatibility.
- `middleware/value_package.go` only changes value-package quota-limit message calculation and does not overwrite routing/user groups.
- Frontend diffs only add reset display and i18n keys.

- [ ] **Step 7: Commit any verification-only fixture restorations**

If Step 1 required restoring LDXP config, commit that restoration separately:

```bash
git add service/ldxp_config.go
git commit -m "fix: preserve ldxp discount defaults"
```

If Step 1 did not require changes, skip this commit.

---

## Task 9: Final integration, push, and deployment handoff

**Files:**

- No planned code changes.

- [ ] **Step 1: Run final status check**

Run:

```bash
git status --short --branch
git log --oneline -8
```

Expected:

```text
## main...origin/main [ahead N]
```

and the log includes the existing LDXP fix commit plus this plan’s implementation commits. Current known LDXP fix commit to preserve:

```text
89d67f439e877d9a42632c7b5c9aad4c4144cf19 fix: align ldxp discounted topup amounts
```

- [ ] **Step 2: Run final combined tests**

Run:

```bash
go test ./middleware ./service ./model ./controller -count=1 -timeout=300s
cd web/default
bun run typecheck
bun run build
```

Expected:

```text
ok  	github.com/QuantumNous/new-api/middleware	...
ok  	github.com/QuantumNous/new-api/service	...
ok  	github.com/QuantumNous/new-api/model	...
ok  	github.com/QuantumNous/new-api/controller	...
No errors
Build completed
```

- [ ] **Step 3: Push local main to GitHub main after tests pass**

Run:

```bash
git push origin main
```

Expected:

```text
To github.com:...
   <old>..<new>  main -> main
```

- [ ] **Step 4: Deploy only after push and final local verification**

The earlier user instruction requested deployment after implementation. Use the existing server key location from project instructions:

```text
/Users/ethan/Desktop/云贝
```

Do not print private keys, tokens, cookies, `.env`, DB DSNs, or server secrets. First inspect the existing deployment scripts/docs in the repository and prior shell history rather than inventing commands. A safe deployment sequence must prove:

```bash
git rev-parse HEAD
```

on the server equals the pushed local commit hash before/after build/restart.

- [ ] **Step 5: Post-deploy smoke verification**

After deployment, verify without exposing secrets:

- User value-package self/state endpoint returns the six new reset/limited fields.
- Admin order-management realtime usage endpoint returns the six new reset/limited fields.
- A package user still routes through Plus/Pro distributor groups, not `day-card`, `week-card`, or `month-card` model groups.
- LDXP config on deployed build still maps `50=47.5`, `100=90`, `500=425`.
- Frontend renders reset text under user and admin 5-hour/7-day usage bars.

If deployment is not desired in the current run, stop after Step 3 and report the pushed commit hash.

---

## Self-Review Checklist

- [ ] Spec coverage: reset fields, OpenAI-style latest-usage full restore time, user UI, admin UI, middleware error messages, i18n, and LDXP guard all have tasks.
- [ ] No algorithm drift: rolling-window `SUM(quota)` remains the enforcement source.
- [ ] No model-group drift: middleware routing-group tests are included in verification.
- [ ] No LDXP regression: plan explicitly verifies `50=47.5`, `100=90`, `500=425` and forbids accidental `200` replacement.
- [ ] DB compatibility: aggregate SQL uses `COALESCE`, `SUM`, `MAX`, and GORM; no DB-specific syntax.
- [ ] Frontend i18n: all six locales are listed and `bun run i18n:sync` is required.
- [ ] Frequent commits: every major backend/frontend/i18n unit has its own commit.
