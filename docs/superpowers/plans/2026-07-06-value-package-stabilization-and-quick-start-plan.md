# Value Package Stabilization and Quick Start Clipboard Fallback Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 稳定云贝日卡、周卡、月卡链路，使套餐只影响计费权益、不污染模型路由，并修复快速引导 API key 已生成但复制失败时无法衔接第 5 步的问题。

**Architecture:** 后端显式拆分真实用户组、模型路由组、套餐计费组，并让 `/api/value-packages/self` 返回权威计费展示状态；前端只展示后端返回的套餐计费事实，不再从分组倍率表推断。所有套餐消耗路径统一通过 BillingSession/usage record 记账，realtime WebSocket 只做累计预留，不重复扣费，最终结算回写同一条 rolling-window usage record。

**Tech Stack:** Go 1.22+、Gin、GORM、SQLite/MySQL/PostgreSQL-compatible queries、Redis optional concurrency slots、React 19、TypeScript、TanStack Query/Table、Bun test/typecheck/build、i18next。

---

## Execution scope and baseline

- Repository root for this plan: `/Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening`
- Source baseline reviewed: `main` at `4bee2d93a8f6d335596385d590301478b05fcb5c`
- Spec file: `docs/superpowers/specs/2026-07-06-value-package-stabilization-and-quick-start-spec.md`
- This plan covers only source and test changes. Server deployment is a separate explicit user action.

## Files by responsibility

### Backend identity and billing contract

- Modify: `relay/common/relay_info.go` — add explicit billing identity fields and a helper method.
- Modify: `relay/common/relay_info_test.go` — assert `UserGroup`, `UsingGroup`, and billing group are separate.
- Modify: `relay/helper/price.go` — compute group-group billing ratio from billing identity only.
- Modify: `relay/helper/price_test.go` — assert billing ratio uses `month-card:gpt-plus` while routing stays `gpt-plus`.
- Modify: `service/quota.go` — realtime quota calculation uses billing identity and reserve semantics.

### Backend value-package state and usage accounting

- Modify: `model/subscription.go` — add `ValuePackageBillingState`; add helper for usage-record upsert by subscription/request; keep queries DB-compatible.
- Modify: `model/value_package_test.go` — assert billing state and rolling-window usage are consistent.
- Modify: `service/billing_session.go` — extract value-package usage recording helper and keep final settlement idempotent.
- Modify: `service/billing_session_test.go` — assert realtime reservation and final settlement do not double count.
- Modify: `controller/value_package_test.go` — assert `/api/value-packages/self` includes authoritative billing state.

### Backend logs, orders, redemption, affiliate, concurrency

- Modify: `service/log_info_generate.go` — include value-package fields in usage-log `other` payload.
- Modify: `service/task_billing.go` — include value-package billing fields for async task logs.
- Modify: `middleware/value_package.go` — add precise concurrency-slot logs and keep TTL/heartbeat behavior.
- Modify: `middleware/value_package_test.go` — assert slot release, stale recovery, and routing context isolation.
- Modify: `model/order_management_test.go` — cover value-package orders, deletion marks, analytics, and affiliate fields.
- Modify: `controller/order_management_test.go` — cover admin realtime table and deletion endpoint responses.
- Modify: `service/order_mail_check_job_test.go` — assert deleted value-package orders stay excluded from scans.
- Modify: `model/redemption_subscription_test.go` — assert day/week/month redemption defaults active and does not create cash affiliate commission.

### Frontend value-package and API key UI

- Modify: `web/default/src/features/value-packages/types.ts` — add `ValuePackageBillingState`.
- Create: `web/default/src/features/value-packages/lib/billing-display.ts` — normalize backend billing state for UI.
- Create: `web/default/src/features/value-packages/lib/billing-display.test.ts` — test active/inactive billing state.
- Modify: `web/default/src/features/keys/lib/api-key-display.ts` — make `auto` keys show active package ratio.
- Modify: `web/default/src/features/keys/lib/api-key-display.test.ts` — cover `auto · 套餐 1x`.
- Modify: `web/default/src/features/keys/components/api-keys-columns.tsx` — read active package ratio from backend billing state and render it in both `auto` and normal groups.
- Modify: `web/default/src/features/keys/components/api-keys-package-billing-alert.tsx` — align copy with backend billing truth.

### Frontend logs, redemption UI, order-management realtime table

- Modify: `web/default/src/features/usage-logs/types.ts` — add value-package log fields.
- Modify: `web/default/src/features/usage-logs/components/columns/common-logs-columns.tsx` — display `套餐 1x` / `Package 1x` instead of bare `1x`.
- Modify: `web/default/src/features/usage-logs/components/columns/common-logs-columns.test.ts` — assert label text.
- Modify: `web/default/src/features/usage-logs/components/dialogs/details-dialog.tsx` — show package effective ratio and original routing ratio.
- Modify: `web/default/src/features/redemption-codes/components/redemptions-mutate-drawer.tsx` — show an explicit empty state when no enabled value-package plans exist.
- Modify: `web/default/src/features/redemption-codes/components/redemptions-mutate-drawer-source.test.ts` — source-level guard for value-package plan creation UI.
- Modify: `web/default/src/features/order-management/components/value-package-usage-table.tsx` — ensure table labels/refresh match backend rows.
- Modify: `web/default/src/features/order-management/components/value-package-usage-table-source.test.ts` — source-level guard for 5h/7d/total columns.

### Frontend quick-start API key clipboard fallback

- Modify: `web/default/src/features/quick-start/quick-start-api-key.ts` — return `{ name, fullKey, copied }`; do not throw when clipboard copy fails after reveal succeeds.
- Modify: `web/default/src/features/quick-start/quick-start-api-key.test.ts` — assert copy failure resolves with `copied: false`.
- Modify: `web/default/src/features/quick-start/index.tsx` — persist generated key, show copy warning, allow retry copy without creating another key.
- Modify: `web/default/src/features/quick-start/quick-start-page-source.test.ts` — guard warning UI and retry copy flow.
- Modify: `web/default/src/features/quick-start/quick-start-locales.test.ts` — include new quick-start text keys.
- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/zh.json`
- Modify: `web/default/src/i18n/locales/fr.json`
- Modify: `web/default/src/i18n/locales/ja.json`
- Modify: `web/default/src/i18n/locales/ru.json`
- Modify: `web/default/src/i18n/locales/vi.json`

---

## Task 1: Preflight and contract guard

**Files:**

- Read: `docs/superpowers/specs/2026-07-06-value-package-stabilization-and-quick-start-spec.md`
- Verify: git branch/status only

- [ ] **Step 1: Start from a clean implementation worktree**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening
git status --short --untracked-files=all
git branch --show-current
git rev-parse HEAD
```

Expected:

```text
main
4bee2d93a8f6d335596385d590301478b05fcb5c
```

`git status` may show the spec and this plan as documentation-only changes. No Go/TS source files should be dirty before implementation begins.

- [ ] **Step 2: Re-read the behavior contract before touching code**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening
sed -n '1,260p' docs/superpowers/specs/2026-07-06-value-package-stabilization-and-quick-start-spec.md
sed -n '260,560p' docs/superpowers/specs/2026-07-06-value-package-stabilization-and-quick-start-spec.md
```

Expected: the spec includes these exact requirements:

```text
日卡、周卡、月卡只改变用户的套餐计费权益，不改变模型路由组
快速引导中 API key 一旦成功创建并 reveal，即使自动复制失败，也不能阻断第 5 步
```

- [ ] **Step 3: Commit the spec and plan before implementation**

`docs/superpowers/plans` is ignored in this repo, so force-add the plan when committing documentation.

```bash
git add docs/superpowers/specs/2026-07-06-value-package-stabilization-and-quick-start-spec.md
git add -f docs/superpowers/plans/2026-07-06-value-package-stabilization-and-quick-start-plan.md
git commit -m "docs: plan value package stabilization"
```

Expected: one docs-only commit. If the docs are already committed by the executor, this step should produce no new source changes.

---

## Task 2: Stop overloading `RelayInfo.UserGroup` for package billing

**Files:**

- Modify: `relay/common/relay_info.go`
- Modify: `relay/common/relay_info_test.go`
- Modify: `relay/helper/price.go`
- Modify: `relay/helper/price_test.go`
- Modify: `service/quota.go`

- [ ] **Step 1: Replace the current relay-info test with a failing identity contract**

In `relay/common/relay_info_test.go`, replace `TestGenRelayInfoUsesValuePackageGroupOnlyForBillingIdentity` with:

```go
func TestGenRelayInfoKeepsUserGroupAndSetsValuePackageBillingGroup(t *testing.T) {
    gin.SetMode(gin.TestMode)
    recorder := httptest.NewRecorder()
    ctx, _ := gin.CreateTestContext(recorder)
    ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
    common.SetContextKey(ctx, constant.ContextKeyUserGroup, "vip")
    common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "gpt-plus")
    common.SetContextKey(ctx, constant.ContextKeyTokenGroup, "gpt-plus")
    common.SetContextKey(ctx, constant.ContextKeyValuePackageSubscriptionId, 123)
    common.SetContextKey(ctx, constant.ContextKeyValuePackagePlanId, 456)
    common.SetContextKey(ctx, constant.ContextKeyValuePackageModelGroup, "month-card")
    common.SetContextKey(ctx, constant.ContextKeyValuePackagePackageType, "month")

    info, err := GenRelayInfo(ctx, types.RelayFormatOpenAI, &dto.GeneralOpenAIRequest{Model: "gpt-5.5"}, nil)

    require.NoError(t, err)
    require.NotNil(t, info)
    require.Equal(t, "vip", info.UserGroup)
    require.Equal(t, "vip", info.RealUserGroup)
    require.Equal(t, "gpt-plus", info.UsingGroup)
    require.Equal(t, "gpt-plus", info.TokenGroup)
    require.Equal(t, "month-card", info.BillingUserGroup)
    require.Equal(t, "month-card", info.ValuePackageBillingGroup)
    require.Equal(t, "month-card", info.ValuePackageModelGroup)
    require.Equal(t, "month", info.ValuePackagePackageType)
    require.Equal(t, "month-card", info.BillingRatioUserGroup())
}
```

- [ ] **Step 2: Run the failing relay-info test**

```bash
go test ./relay/common -run TestGenRelayInfoKeepsUserGroupAndSetsValuePackageBillingGroup -count=1
```

Expected before implementation: compile failure for missing `RealUserGroup`, `BillingUserGroup`, `ValuePackageBillingGroup`, or assertion failure because `UserGroup` is still `month-card`.

- [ ] **Step 3: Add explicit billing fields and helper method**

In `relay/common/relay_info.go`, add these fields near the existing `UserGroup` and value-package fields:

```go
RealUserGroup            string // 用户真实身份分组，不被套餐覆盖
BillingUserGroup         string // 本次请求用于 group-group 计费倍率的身份
ValuePackageBillingGroup string // day-card/week-card/month-card，仅用于套餐计费
```

Add this method in the same file:

```go
func (info *RelayInfo) BillingRatioUserGroup() string {
    if info == nil {
        return ""
    }
    if group := strings.TrimSpace(info.BillingUserGroup); group != "" {
        return group
    }
    if group := strings.TrimSpace(info.ValuePackageBillingGroup); group != "" {
        return group
    }
    return strings.TrimSpace(info.UserGroup)
}
```

`relay/common/relay_info.go` already imports `strings`, so no new import should be required.

- [ ] **Step 4: Stop assigning package group into `UserGroup`**

In `relay/common/relay_info.go:genBaseRelayInfo`, replace the current block:

```go
userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
if valuePackageGroup := strings.TrimSpace(common.GetContextKeyString(c, constant.ContextKeyValuePackageModelGroup)); valuePackageGroup != "" {
    userGroup = valuePackageGroup
}
```

with:

```go
userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
valuePackageGroup := strings.TrimSpace(common.GetContextKeyString(c, constant.ContextKeyValuePackageModelGroup))
billingUserGroup := userGroup
if valuePackageGroup != "" {
    billingUserGroup = valuePackageGroup
}
```

Then populate the `RelayInfo` literal as:

```go
UserGroup:                  userGroup,
RealUserGroup:              userGroup,
BillingUserGroup:           billingUserGroup,
ValuePackageBillingGroup:   valuePackageGroup,
ValuePackageSubscriptionId: common.GetContextKeyInt(c, constant.ContextKeyValuePackageSubscriptionId),
ValuePackagePlanId:         common.GetContextKeyInt(c, constant.ContextKeyValuePackagePlanId),
ValuePackageModelGroup:     valuePackageGroup,
ValuePackagePackageType:    common.GetContextKeyString(c, constant.ContextKeyValuePackagePackageType),
```

Keep `UsingGroup` and `TokenGroup` populated from their existing context keys.

- [ ] **Step 5: Make price helper use billing identity only for billing ratio**

In `relay/helper/price.go:HandleGroupRatio`, replace:

```go
userGroupRatio, ok := ratio_setting.GetGroupGroupRatio(relayInfo.UserGroup, relayInfo.UsingGroup)
```

with:

```go
userGroupRatio, ok := ratio_setting.GetGroupGroupRatio(relayInfo.BillingRatioUserGroup(), relayInfo.UsingGroup)
```

This keeps distributor routing on `UsingGroup` while applying `month-card:gpt-plus` for billing.

- [ ] **Step 6: Make realtime quota calculation use the same billing identity**

In `service/quota.go:PreWssConsumeQuota`, replace:

```go
userGroupRatio, ok := ratio_setting.GetGroupGroupRatio(relayInfo.UserGroup, relayInfo.UsingGroup)
```

with:

```go
userGroupRatio, ok := ratio_setting.GetGroupGroupRatio(relayInfo.BillingRatioUserGroup(), relayInfo.UsingGroup)
```

- [ ] **Step 7: Add price-helper regression test**

Append this test to `relay/helper/price_test.go`:

```go
func TestHandleGroupRatioUsesBillingUserGroupWithoutMutatingRoutingGroup(t *testing.T) {
    gin.SetMode(gin.TestMode)
    oldGroupRatios := ratio_setting.GroupRatio2JSONString()
    oldGroupGroupRatios := ratio_setting.GroupGroupRatio2JSONString()
    require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"gpt-plus":0.3}`))
    require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"month-card":{"gpt-plus":1}}`))
    t.Cleanup(func() {
        require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldGroupRatios))
        require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(oldGroupGroupRatios))
    })

    recorder := httptest.NewRecorder()
    ctx, _ := gin.CreateTestContext(recorder)
    ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
    info := &relaycommon.RelayInfo{
        UserGroup:                "vip",
        RealUserGroup:            "vip",
        BillingUserGroup:         "month-card",
        ValuePackageBillingGroup: "month-card",
        UsingGroup:               "gpt-plus",
    }

    ratio := HandleGroupRatio(ctx, info)

    require.Equal(t, "vip", info.UserGroup)
    require.Equal(t, "gpt-plus", info.UsingGroup)
    require.True(t, ratio.HasSpecialRatio)
    require.Equal(t, 1.0, ratio.GroupRatio)
    require.Equal(t, 1.0, ratio.GroupSpecialRatio)
}
```

- [ ] **Step 8: Run identity and ratio tests**

```bash
go test ./relay/common ./relay/helper ./service -run 'RelayInfo|HandleGroupRatio|BillingRatio|Wss' -count=1 -timeout=120s
```

Expected: pass.

- [ ] **Step 9: Commit identity refactor**

```bash
git add relay/common/relay_info.go relay/common/relay_info_test.go relay/helper/price.go relay/helper/price_test.go service/quota.go
git commit -m "fix: separate value package billing identity"
```

---

## Task 3: Expose authoritative value-package billing state

**Files:**

- Modify: `model/subscription.go`
- Modify: `model/value_package_test.go`
- Modify: `controller/value_package_test.go`
- Modify: `web/default/src/features/value-packages/types.ts`
- Create: `web/default/src/features/value-packages/lib/billing-display.ts`
- Create: `web/default/src/features/value-packages/lib/billing-display.test.ts`

- [ ] **Step 1: Add failing model test for billing state**

Append to `model/value_package_test.go`:

```go
func TestGetValuePackageStateIncludesAuthoritativeBillingState(t *testing.T) {
    setupValuePackageTestDB(t)
    user := createValuePackageUser(t, 3501, UserGroupVIP)
    month := createValuePackagePlan(t, ValuePackageTypeMonth, ValuePackageLevelMonth, 30, 29.9)
    month.ModelGroup = "month-card"
    require.NoError(t, DB.Save(month).Error)
    now := common.GetTimestamp()
    sub := createActiveValuePackageSub(t, user.Id, month, now-10, now+3600)
    _, err := ActivateValuePackage(user.Id, sub.Id)
    require.NoError(t, err)

    state, err := GetValuePackageState(user.Id)

    require.NoError(t, err)
    require.NotNil(t, state.Billing)
    require.True(t, state.Billing.Active)
    require.Equal(t, "month-card", state.Billing.PackageGroup)
    require.Equal(t, float64(1), state.Billing.EffectiveRatio)
    require.Equal(t, month.Id, state.Billing.PlanId)
    require.Equal(t, month.Title, state.Billing.PlanTitle)
}
```

- [ ] **Step 2: Run failing model test**

```bash
go test ./model -run TestGetValuePackageStateIncludesAuthoritativeBillingState -count=1
```

Expected before implementation: compile failure for missing `Billing` field or `ValuePackageBillingState`.

- [ ] **Step 3: Add backend billing state DTO**

In `model/subscription.go`, near the value-package constants, add:

```go
const ValuePackageEffectiveBillingRatio = 1.0
```

Near `ValuePackageUsageSummary`, add:

```go
type ValuePackageBillingState struct {
    Active             bool    `json:"active"`
    RoutingGroup       string  `json:"routing_group"`
    PackageGroup       string  `json:"package_group"`
    EffectiveRatio     float64 `json:"effective_ratio"`
    OriginalGroupRatio float64 `json:"original_group_ratio"`
    PlanTitle          string  `json:"plan_title"`
    PlanId             int     `json:"plan_id"`
}
```

Add the field to `ValuePackageState`:

```go
Billing *ValuePackageBillingState `json:"billing,omitempty"`
```

- [ ] **Step 4: Populate billing state in `loadValuePackageStateTx`**

Add this helper in `model/subscription.go` near the value-package state helpers:

```go
func buildValuePackageBillingState(pref *UserValuePackagePreference, sub *UserSubscription, plan *SubscriptionPlan) *ValuePackageBillingState {
    if pref == nil || sub == nil || plan == nil || !pref.Enabled || !plan.IsValuePackage() {
        return &ValuePackageBillingState{Active: false}
    }
    packageGroup := strings.TrimSpace(plan.ModelGroup)
    if packageGroup == "" {
        return &ValuePackageBillingState{Active: false}
    }
    return &ValuePackageBillingState{
        Active:         true,
        RoutingGroup:   "",
        PackageGroup:   packageGroup,
        EffectiveRatio: ValuePackageEffectiveBillingRatio,
        PlanTitle:      plan.Title,
        PlanId:         plan.Id,
    }
}
```

In both branches of `loadValuePackageStateTx` where `state.Subscription` and `state.Plan` are set, add:

```go
state.Billing = buildValuePackageBillingState(&state.Preference, state.Subscription, state.Plan)
```

For the branch that creates `state := &ValuePackageState{Preference: *prefPtr, Subscription: sub, Plan: plan}`, add immediately after it:

```go
state.Billing = buildValuePackageBillingState(&state.Preference, state.Subscription, state.Plan)
```

For the main branch after `state.Subscription = &sub` and `state.Plan = plan`, add:

```go
state.Billing = buildValuePackageBillingState(&state.Preference, state.Subscription, state.Plan)
```

- [ ] **Step 5: Add controller test for API response**

Append to `controller/value_package_test.go`:

```go
func TestGetValuePackageSelfIncludesBillingState(t *testing.T) {
    setupValuePackageControllerTest(t)
    plan := seedValuePackageControllerPlan(t, model.ValuePackageTypeMonth, model.ValuePackageLevelMonth)
    user := seedValuePackageControllerUser(t)
    sub := seedValuePackageControllerSubscription(t, user.Id, plan.Id, 1000, 0)
    _, err := model.ActivateValuePackage(user.Id, sub.Id)
    require.NoError(t, err)

    rec := valuePackageControllerRequest(GetValuePackageSelf, http.MethodGet, "/value-packages/self", nil, user.Id)

    require.Equal(t, http.StatusOK, rec.Code)
    var body map[string]interface{}
    require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &body))
    data := body["data"].(map[string]interface{})
    billing := data["billing"].(map[string]interface{})
    require.Equal(t, true, billing["active"])
    require.Equal(t, plan.ModelGroup, billing["package_group"])
    require.Equal(t, float64(1), billing["effective_ratio"])
    require.Equal(t, float64(plan.Id), billing["plan_id"])
}
```

- [ ] **Step 6: Update frontend type**

In `web/default/src/features/value-packages/types.ts`, add:

```ts
export interface ValuePackageBillingState {
  active: boolean
  routing_group?: string
  package_group?: string
  effective_ratio?: number
  original_group_ratio?: number
  plan_title?: string
  plan_id?: number
}
```

Update `ValuePackageState`:

```ts
export interface ValuePackageState {
  preference: UserValuePackagePreference
  subscription?: UserSubscription | null
  plan?: ValuePackagePlan | null
  usage?: ValuePackageUsageSummary | null
  billing?: ValuePackageBillingState | null
}
```

- [ ] **Step 7: Create frontend billing display helper**

Create `web/default/src/features/value-packages/lib/billing-display.ts`:

```ts
import type { ValuePackageState } from '../types'

export function getActiveValuePackageBillingRatio(
  state: ValuePackageState | null | undefined
): number | undefined {
  const ratio = state?.billing?.active ? state.billing.effective_ratio : undefined
  return typeof ratio === 'number' && Number.isFinite(ratio) ? ratio : undefined
}

export function getActiveValuePackageBillingLabel(
  state: ValuePackageState | null | undefined
): string | null {
  if (!state?.billing?.active) return null
  const title = state.billing.plan_title?.trim()
  const group = state.billing.package_group?.trim()
  if (title && group) return `${title} · ${group}`
  return title || group || null
}
```

- [ ] **Step 8: Test frontend billing helper**

Create `web/default/src/features/value-packages/lib/billing-display.test.ts`:

```ts
import assert from 'node:assert/strict'
import test from 'node:test'
import { getActiveValuePackageBillingLabel, getActiveValuePackageBillingRatio } from './billing-display'

test('active billing ratio comes from backend billing state', () => {
  const state = {
    preference: { id: 1, user_id: 1, enabled: true, active_user_subscription_id: 2, created_at: 1, updated_at: 1 },
    billing: { active: true, effective_ratio: 1, plan_title: '月卡', package_group: 'month-card' },
  }

  assert.equal(getActiveValuePackageBillingRatio(state), 1)
  assert.equal(getActiveValuePackageBillingLabel(state), '月卡 · month-card')
})

test('inactive package has no billing ratio', () => {
  const state = {
    preference: { id: 1, user_id: 1, enabled: false, active_user_subscription_id: 0, created_at: 1, updated_at: 2 },
    billing: { active: false },
  }

  assert.equal(getActiveValuePackageBillingRatio(state), undefined)
  assert.equal(getActiveValuePackageBillingLabel(state), null)
})
```

- [ ] **Step 9: Run backend and frontend state tests**

```bash
go test ./model ./controller -run 'ValuePackageState|ValuePackageSelf|BillingState' -count=1 -timeout=180s
cd web/default
bun test src/features/value-packages/lib/billing-display.test.ts
```

Expected: pass.

- [ ] **Step 10: Commit billing-state DTO work**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening
git add model/subscription.go model/value_package_test.go controller/value_package_test.go web/default/src/features/value-packages/types.ts web/default/src/features/value-packages/lib/billing-display.ts web/default/src/features/value-packages/lib/billing-display.test.ts
git commit -m "feat: expose value package billing state"
```

---

## Task 4: Fix quick-start API key generation when clipboard copy fails

**Files:**

- Modify: `web/default/src/features/quick-start/quick-start-api-key.ts`
- Modify: `web/default/src/features/quick-start/quick-start-api-key.test.ts`
- Modify: `web/default/src/features/quick-start/index.tsx`
- Modify: `web/default/src/features/quick-start/quick-start-page-source.test.ts`
- Modify: `web/default/src/features/quick-start/quick-start-locales.test.ts`
- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/zh.json`
- Modify: `web/default/src/i18n/locales/fr.json`
- Modify: `web/default/src/i18n/locales/ja.json`
- Modify: `web/default/src/i18n/locales/ru.json`
- Modify: `web/default/src/i18n/locales/vi.json`

- [ ] **Step 1: Change the clipboard failure test first**

In `web/default/src/features/quick-start/quick-start-api-key.test.ts`, replace `one-click API key creation reports clipboard failure` with:

```ts
test('one-click API key creation returns generated key when clipboard copy fails', async () => {
  const result = await generateAndCopyQuickStartApiKey({
    now: () => 1,
    defaultGroup: 'gpt-plus',
    createApiKey: async () => ({ success: true }),
    searchApiKeys: async ({ keyword }) => ({
      success: true,
      data: { items: [{ id: 7, name: keyword || '' }] },
    }),
    fetchTokenKey: async () => ({
      success: true,
      data: { key: 'key' },
    }),
    copyToClipboard: async () => false,
  })

  assert.equal(result.name, 'yunbay-quick-start-1')
  assert.equal(result.fullKey, 'sk-key')
  assert.equal(result.copied, false)
})
```

- [ ] **Step 2: Run the failing quick-start unit test**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening/web/default
bun test src/features/quick-start/quick-start-api-key.test.ts
```

Expected before implementation: failure because the function still rejects on copy failure and does not return `copied`.

- [ ] **Step 3: Add explicit result type and stop throwing after successful reveal**

In `web/default/src/features/quick-start/quick-start-api-key.ts`, add:

```ts
export type QuickStartApiKeyResult = {
  name: string
  fullKey: string
  copied: boolean
}
```

Change the function signature to:

```ts
export async function generateAndCopyQuickStartApiKey(
  dependencies: QuickStartApiKeyDependencies
): Promise<QuickStartApiKeyResult> {
```

Replace the final copy block:

```ts
const fullKey = key.startsWith('sk-') ? key : `sk-${key}`
if (!(await dependencies.copyToClipboard(fullKey))) {
  throw new Error('Failed to copy the new API key')
}

return { name, fullKey }
```

with:

```ts
const fullKey = key.startsWith('sk-') ? key : `sk-${key}`
const copied = await dependencies.copyToClipboard(fullKey)

return { name, fullKey, copied }
```

Create/search/reveal failures must keep throwing, because the frontend does not have a key without reveal.

- [ ] **Step 4: Add page state for copy status**

In `web/default/src/features/quick-start/index.tsx`, after:

```ts
const [generatedApiKey, setGeneratedApiKey] = useState('')
```

add:

```ts
const [generatedApiKeyCopied, setGeneratedApiKeyCopied] = useState<boolean | null>(null)
```

- [ ] **Step 5: Update retry-copy behavior**

In `handleGenerateApiKey`, replace the existing `generatedApiKey` branch with:

```ts
if (generatedApiKey) {
  const copied = await copyToClipboard(generatedApiKey)
  setGeneratedApiKeyCopied(copied)
  if (copied) {
    toast.success(t('Already copied to clipboard'))
  } else {
    toast.warning(
      t('API key was generated but clipboard copy failed. You can copy it again or continue setup.')
    )
  }
  return
}
```

- [ ] **Step 6: Persist generated key even when copied is false**

In the create-key branch of `handleGenerateApiKey`, replace:

```ts
setGeneratedApiKey(result.fullKey)
toast.success(t('Already copied to clipboard'))
```

with:

```ts
setGeneratedApiKey(result.fullKey)
setGeneratedApiKeyCopied(result.copied)
if (result.copied) {
  toast.success(t('Already copied to clipboard'))
} else {
  toast.warning(
    t('API key was generated but clipboard copy failed. You can copy it again or continue setup.')
  )
}
```

- [ ] **Step 7: Update API key card copy text**

In the API key page paragraph, replace:

```tsx
{generatedApiKey
  ? t('Already copied to clipboard')
  : t(
      'Click generate. The new API key will be copied to your clipboard.'
    )}
```

with:

```tsx
{generatedApiKey
  ? generatedApiKeyCopied === false
    ? t('API key was generated but clipboard copy failed. You can copy it again or continue setup.')
    : t('Already copied to clipboard')
  : t(
      'Click generate. The new API key will be copied to your clipboard.'
    )}
```

- [ ] **Step 8: Add source-level guard for warning text**

In `web/default/src/features/quick-start/quick-start-page-source.test.ts`, add:

```ts
test('quick start keeps generated API key when clipboard copy fails', () => {
  assert.match(pageSource, /generatedApiKeyCopied/)
  assert.match(
    pageSource,
    /API key was generated but clipboard copy failed\. You can copy it again or continue setup\./
  )
  assert.match(pageSource, /setGeneratedApiKey\(result\.fullKey\)/)
  assert.match(pageSource, /setGeneratedApiKeyCopied\(result\.copied\)/)
})
```

- [ ] **Step 9: Add new quick-start translation key to locale test**

In `web/default/src/features/quick-start/quick-start-locales.test.ts`, add this string to `QUICK_START_COMPONENT_KEYS`:

```ts
'API key was generated but clipboard copy failed. You can copy it again or continue setup.',
```

- [ ] **Step 10: Add translations**

Add the same key to each locale JSON under `translation`.

English:

```json
"API key was generated but clipboard copy failed. You can copy it again or continue setup.": "API key was generated but clipboard copy failed. You can copy it again or continue setup."
```

Chinese:

```json
"API key was generated but clipboard copy failed. You can copy it again or continue setup.": "API key 已生成，但自动复制失败。你可以再次复制，或继续下一步配置。"
```

French:

```json
"API key was generated but clipboard copy failed. You can copy it again or continue setup.": "La clé API a été générée, mais la copie automatique a échoué. Vous pouvez réessayer de la copier ou continuer la configuration."
```

Japanese:

```json
"API key was generated but clipboard copy failed. You can copy it again or continue setup.": "API キーは生成されましたが、自動コピーに失敗しました。もう一度コピーするか、設定を続行できます。"
```

Russian:

```json
"API key was generated but clipboard copy failed. You can copy it again or continue setup.": "API-ключ создан, но автоматически скопировать его не удалось. Вы можете повторить копирование или продолжить настройку."
```

Vietnamese:

```json
"API key was generated but clipboard copy failed. You can copy it again or continue setup.": "API key đã được tạo nhưng sao chép tự động thất bại. Bạn có thể sao chép lại hoặc tiếp tục thiết lập."
```

- [ ] **Step 11: Run quick-start tests**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening/web/default
bun test src/features/quick-start/quick-start-api-key.test.ts \
  src/features/quick-start/quick-start-page-source.test.ts \
  src/features/quick-start/quick-start-locales.test.ts
```

Expected: pass.

- [ ] **Step 12: Commit quick-start fix**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening
git add web/default/src/features/quick-start/quick-start-api-key.ts \
  web/default/src/features/quick-start/quick-start-api-key.test.ts \
  web/default/src/features/quick-start/index.tsx \
  web/default/src/features/quick-start/quick-start-page-source.test.ts \
  web/default/src/features/quick-start/quick-start-locales.test.ts \
  web/default/src/i18n/locales/en.json \
  web/default/src/i18n/locales/zh.json \
  web/default/src/i18n/locales/fr.json \
  web/default/src/i18n/locales/ja.json \
  web/default/src/i18n/locales/ru.json \
  web/default/src/i18n/locales/vi.json
git commit -m "fix: keep quick start API key after clipboard failure"
```

---

## Task 5: Show package billing ratio on API keys, including `auto`

**Files:**

- Modify: `web/default/src/features/keys/lib/api-key-display.ts`
- Modify: `web/default/src/features/keys/lib/api-key-display.test.ts`
- Modify: `web/default/src/features/keys/components/api-keys-columns.tsx`
- Modify: `web/default/src/features/keys/components/api-keys-package-billing-alert.tsx`
- Read: `web/default/src/features/value-packages/lib/billing-display.ts`

- [ ] **Step 1: Add failing `auto` display test**

Append to `web/default/src/features/keys/lib/api-key-display.test.ts`:

```ts
test('API key display shows active package billing ratio for auto group', () => {
  const display = getApiKeyDisplayGroup(
    {
      group: 'auto',
      effective_group: '',
      effective_group_ratio: undefined,
      cross_group_retry: true,
    },
    { auto: 1 },
    1
  )

  assert.equal(display.group, 'auto')
  assert.equal(display.ratio, 1)
  assert.equal(display.isEffective, true)
})
```

- [ ] **Step 2: Run failing API key display test**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening/web/default
bun test src/features/keys/lib/api-key-display.test.ts
```

Expected before implementation: failure because `auto` returns no `ratio`.

- [ ] **Step 3: Return active package ratio for every group**

In `web/default/src/features/keys/lib/api-key-display.ts`, replace:

```ts
if (group === 'auto') {
  return { group, isEffective }
}

const ratio = isEffective ? activePackageRatio : groupRatios[group]

return { group, ratio, isEffective }
```

with:

```ts
const ratio = isEffective ? activePackageRatio : groupRatios[group]

return { group, ratio, isEffective }
```

- [ ] **Step 4: Read active package ratio from backend billing state**

In `web/default/src/features/keys/components/api-keys-columns.tsx`, import:

```ts
import { getActiveValuePackageBillingRatio } from '@/features/value-packages/lib/billing-display'
```

Delete the local `getActivePackageBillingRatio` function that reads `plan.model_group` and `groupRatios`.

Replace:

```ts
const activePackageRatio = getActivePackageBillingRatio(
  valuePackageState,
  groupRatios
)
```

with:

```ts
const activePackageRatio = getActiveValuePackageBillingRatio(valuePackageState)
```

- [ ] **Step 5: Render ratio for `auto` group too**

In the `group === 'auto'` branch, replace:

```tsx
<GroupBadge group='auto' />
```

with:

```tsx
<GroupBadge group='auto' ratio={ratio} />
```

Keep the `Cross-group` badge and existing tooltip text.

- [ ] **Step 6: Add source guard for backend billing helper**

Create or extend `web/default/src/features/keys/components/api-keys-columns-source.test.ts` with:

```ts
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const currentDir = dirname(fileURLToPath(import.meta.url))
const source = readFileSync(resolve(currentDir, 'api-keys-columns.tsx'), 'utf8')

test('API key columns read package billing ratio from backend state helper', () => {
  assert.match(source, /getActiveValuePackageBillingRatio\(valuePackageState\)/)
  assert.doesNotMatch(source, /groupRatios\[packageGroup\]/)
  assert.match(source, /<GroupBadge group='auto' ratio=\{ratio\} \/>/)
})
```

- [ ] **Step 7: Run key UI tests**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening/web/default
bun test src/features/keys/lib/api-key-display.test.ts \
  src/features/keys/components/api-keys-columns-source.test.ts \
  src/features/value-packages/lib/billing-display.test.ts
```

Expected: pass.

- [ ] **Step 8: Commit API key display fix**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening
git add web/default/src/features/keys/lib/api-key-display.ts \
  web/default/src/features/keys/lib/api-key-display.test.ts \
  web/default/src/features/keys/components/api-keys-columns.tsx \
  web/default/src/features/keys/components/api-keys-columns-source.test.ts \
  web/default/src/features/keys/components/api-keys-package-billing-alert.tsx
git commit -m "fix: show package billing ratio for api keys"
```

---

## Task 6: Label usage logs as package billing, not bare `1x`

**Files:**

- Modify: `service/log_info_generate.go`
- Modify: `service/task_billing.go`
- Modify: `web/default/src/features/usage-logs/types.ts`
- Modify: `web/default/src/features/usage-logs/components/columns/common-logs-columns.tsx`
- Modify: `web/default/src/features/usage-logs/components/columns/common-logs-columns.test.ts`
- Modify: `web/default/src/features/usage-logs/components/dialogs/details-dialog.tsx`
- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/zh.json`
- Modify: `web/default/src/i18n/locales/fr.json`
- Modify: `web/default/src/i18n/locales/ja.json`
- Modify: `web/default/src/i18n/locales/ru.json`
- Modify: `web/default/src/i18n/locales/vi.json`

- [ ] **Step 1: Change usage-log ratio text test**

In `web/default/src/features/usage-logs/components/columns/common-logs-columns.test.ts`, replace the expected string in `common log token metadata displays 1x when subscription ratio is applied`:

```ts
assert.equal(
  getGroupRatioText({
    group_ratio: 1,
    user_group_ratio: -1,
    subscription_ratio_applied: true,
  }),
  'Package 1x'
)
```

- [ ] **Step 2: Run failing log column test**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening/web/default
bun test src/features/usage-logs/components/columns/common-logs-columns.test.ts
```

Expected before implementation: actual value is `1x`.

- [ ] **Step 3: Include value-package fields in backend log `other` payload**

In `service/log_info_generate.go`, inside the function that appends subscription billing fields, add:

```go
if relayInfo.ValuePackageSubscriptionId != 0 {
    other["value_package_subscription_id"] = relayInfo.ValuePackageSubscriptionId
}
if relayInfo.ValuePackagePlanId != 0 {
    other["value_package_plan_id"] = relayInfo.ValuePackagePlanId
}
if relayInfo.ValuePackageModelGroup != "" {
    other["value_package_model_group"] = relayInfo.ValuePackageModelGroup
}
if relayInfo.ValuePackagePackageType != "" {
    other["value_package_package_type"] = relayInfo.ValuePackagePackageType
}
if relayInfo.PriceData.SubscriptionRatioApplied {
    other["value_package_effective_ratio"] = relayInfo.PriceData.GroupRatioInfo.GroupRatio
}
```

In `service/task_billing.go`, add the same fields when `info` or task private data carries value-package metadata. If the task private data does not yet carry those fields, store only the fields available from `BillingContext` and add a source comment-free test for normal relay logs first.

- [ ] **Step 4: Add frontend log types**

In `web/default/src/features/usage-logs/types.ts`, add to `LogOtherData`:

```ts
value_package_subscription_id?: number
value_package_plan_id?: number
value_package_model_group?: string
value_package_package_type?: string
value_package_effective_ratio?: number
```

- [ ] **Step 5: Return package label from `getGroupRatioText`**

In `web/default/src/features/usage-logs/components/columns/common-logs-columns.tsx`, replace:

```ts
return `${formatRatioCompact(groupRatio)}x`
```

inside the `isSubscriptionRatio` group-ratio branch with:

```ts
const ratioText = `${formatRatioCompact(groupRatio)}x`
return isSubscriptionRatio ? `Package ${ratioText}` : ratioText
```

Keep the plain group ratio `1x` hidden when `subscription_ratio_applied` is false.

- [ ] **Step 6: Label detailed billing rows**

In `web/default/src/features/usage-logs/components/dialogs/details-dialog.tsx`, change the billing breakdown group-ratio row:

```tsx
label: isUserGR ? t('User Exclusive Ratio') : t('Group Ratio'),
```

into:

```tsx
label: other.subscription_ratio_applied === true
  ? t('Package Effective Ratio')
  : isUserGR
    ? t('User Exclusive Ratio')
    : t('Group Ratio'),
```

Inside the subscription billing detail section, after the `Plan` row, add:

```tsx
{other.value_package_model_group && (
  <DetailRow
    label={t('Package Billing Group')}
    value={other.value_package_model_group}
    mono
  />
)}
{other.value_package_effective_ratio != null && (
  <DetailRow
    label={t('Package Effective Ratio')}
    value={`${formatRatio(other.value_package_effective_ratio)}x`}
    mono
  />
)}
{other.original_group_ratio != null && (
  <DetailRow
    label={t('Original Routing Group Ratio')}
    value={`${formatRatio(other.original_group_ratio)}x`}
    mono
  />
)}
```

- [ ] **Step 7: Add translation keys**

Add these keys to every supported locale:

```json
"Package Effective Ratio": "Package Effective Ratio",
"Package Billing Group": "Package Billing Group",
"Original Routing Group Ratio": "Original Routing Group Ratio"
```

Chinese translations:

```json
"Package Effective Ratio": "套餐实际倍率",
"Package Billing Group": "套餐计费组",
"Original Routing Group Ratio": "原始路由组倍率"
```

Use concise native translations for French, Japanese, Russian, and Vietnamese.

- [ ] **Step 8: Run usage-log tests**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening/web/default
bun test src/features/usage-logs/components/columns/common-logs-columns.test.ts
```

Expected: pass.

- [ ] **Step 9: Commit log-label fix**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening
git add service/log_info_generate.go service/task_billing.go \
  web/default/src/features/usage-logs/types.ts \
  web/default/src/features/usage-logs/components/columns/common-logs-columns.tsx \
  web/default/src/features/usage-logs/components/columns/common-logs-columns.test.ts \
  web/default/src/features/usage-logs/components/dialogs/details-dialog.tsx \
  web/default/src/i18n/locales/en.json \
  web/default/src/i18n/locales/zh.json \
  web/default/src/i18n/locales/fr.json \
  web/default/src/i18n/locales/ja.json \
  web/default/src/i18n/locales/ru.json \
  web/default/src/i18n/locales/vi.json
git commit -m "fix: label value package billing in usage logs"
```

---

## Task 7: Fix realtime WebSocket accounting without double counting

**Files:**

- Modify: `relay/common/relay_info.go`
- Modify: `service/quota.go`
- Modify: `service/billing_session.go`
- Modify: `service/billing_session_test.go`
- Modify: `model/subscription.go`
- Modify: `model/value_package_test.go`

- [ ] **Step 1: Add failing realtime double-count test**

Append to `service/billing_session_test.go`:

```go
func TestRealtimeValuePackageReserveDoesNotDoubleCountOnFinalSettle(t *testing.T) {
    setupValuePackageBillingSessionTestDB(t)
    user := model.User{Username: "vp-realtime-user", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: model.UserGroupVIP, Quota: 100000}
    require.NoError(t, model.DB.Create(&user).Error)
    token := model.Token{UserId: user.Id, Key: "vp-realtime-token", Status: common.TokenStatusEnabled, RemainQuota: 100000}
    require.NoError(t, model.DB.Create(&token).Error)
    plan := model.SubscriptionPlan{Title: "Realtime Month", PlanKind: model.SubscriptionPlanKindValuePackage, PackageType: model.ValuePackageTypeMonth, PackageLevel: model.ValuePackageLevelMonth, ModelGroup: "month-card", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: true, TotalAmount: 100000, Limit5hAmount: 100000, Limit7dAmount: 100000, ConcurrencyLimit: 1}
    require.NoError(t, model.DB.Create(&plan).Error)
    now := common.GetTimestamp()
    sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, AmountTotal: 100000, Status: model.UserSubscriptionStatusActive, StartTime: now - 10, EndTime: now + 86400}
    require.NoError(t, model.DB.Create(&sub).Error)

    ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
    relayInfo := &relaycommon.RelayInfo{
        RequestId: "vp-realtime-request",
        UserId: user.Id,
        TokenId: token.Id,
        TokenKey: token.Key,
        TokenUnlimited: true,
        UserQuota: user.Quota,
        UsingGroup: "gpt-plus",
        UserGroup: model.UserGroupVIP,
        BillingUserGroup: "month-card",
        ValuePackageBillingGroup: "month-card",
        OriginModelName: "gpt-realtime",
        IsPlayground: false,
        ValuePackageSubscriptionId: sub.Id,
        ValuePackagePlanId: plan.Id,
        ValuePackageModelGroup: plan.ModelGroup,
        ValuePackagePackageType: plan.PackageType,
        PriceData: types.PriceData{
            ModelRatio: 1,
            QuotaBeforeGroup: 0,
            QuotaToPreConsume: 1,
            GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1, GroupSpecialRatio: -1},
        },
    }
    session, apiErr := NewBillingSession(ctx, relayInfo, 1)
    require.Nil(t, apiErr)
    require.NotNil(t, session)
    relayInfo.Billing = session

    usage := &dto.RealtimeUsage{TotalTokens: 20, InputTokens: 20, InputTokenDetails: dto.InputTokenDetails{TextTokens: 20}}
    require.NoError(t, PreWssConsumeQuota(ctx, relayInfo, usage))
    require.NoError(t, SettleBilling(ctx, relayInfo, 20))

    var reloadedSub model.UserSubscription
    require.NoError(t, model.DB.First(&reloadedSub, sub.Id).Error)
    require.EqualValues(t, 20, reloadedSub.AmountUsed)
    used5h, used7d, err := model.GetValuePackageWindowUsage(user.Id, sub.Id, common.GetTimestamp())
    require.NoError(t, err)
    require.EqualValues(t, 20, used5h)
    require.EqualValues(t, 20, used7d)
}
```

- [ ] **Step 2: Run failing realtime test**

```bash
go test ./service -run TestRealtimeValuePackageReserveDoesNotDoubleCountOnFinalSettle -count=1
```

Expected before implementation: subscription `AmountUsed` is greater than 20 or usage record is not synchronized.

- [ ] **Step 3: Add realtime cumulative quota fields**

In `relay/common/relay_info.go`, add near quota fields:

```go
RealtimeActualQuota   int // realtime WebSocket actual cumulative quota observed so far
RealtimeReservedQuota int // realtime WebSocket funding reservation target already requested
```

These fields are request-local runtime state and do not need JSON tags.

- [ ] **Step 4: Extract usage-record helper in service layer**

In `service/billing_session.go`, replace the body of `func (s *BillingSession) recordValuePackageUsage(actualQuota int) error` with:

```go
func (s *BillingSession) recordValuePackageUsage(actualQuota int) error {
    if s == nil {
        return nil
    }
    return recordValuePackageUsageForRelay(s.relayInfo, actualQuota)
}
```

Add this helper below it:

```go
func recordValuePackageUsageForRelay(relayInfo *relaycommon.RelayInfo, actualQuota int) error {
    if relayInfo == nil || actualQuota < 0 || relayInfo.ValuePackageSubscriptionId <= 0 {
        return nil
    }
    record := &model.ValuePackageUsageRecord{
        UserId:             relayInfo.UserId,
        UserSubscriptionId: relayInfo.ValuePackageSubscriptionId,
        PlanId:             relayInfo.ValuePackagePlanId,
        PackageType:        relayInfo.ValuePackagePackageType,
        ModelGroup:         relayInfo.ValuePackageModelGroup,
        RequestId:          relayInfo.RequestId,
        Quota:              int64(actualQuota),
    }
    if err := model.RecordValuePackageUsage(record); err != nil {
        common.SysLog(fmt.Sprintf("error recording value package usage (userId=%d, subscriptionId=%d, requestId=%s): %s",
            relayInfo.UserId, relayInfo.ValuePackageSubscriptionId, relayInfo.RequestId, err.Error()))
        return err
    }
    return nil
}
```

- [ ] **Step 5: Change `PreWssConsumeQuota` from direct deduction to reservation**

In `service/quota.go:PreWssConsumeQuota`, keep the existing quota calculation, but replace this block:

```go
err = PostConsumeQuota(relayInfo, quota, 0, false)
if err != nil {
    return err
}
logger.LogInfo(ctx, "realtime streaming consume quota success, quota: "+fmt.Sprintf("%d", quota))
return nil
```

with:

```go
if quota <= 0 {
    return nil
}

relayInfo.RealtimeActualQuota += quota
reserveTarget := relayInfo.RealtimeActualQuota
if relayInfo.FinalPreConsumedQuota > reserveTarget {
    reserveTarget = relayInfo.FinalPreConsumedQuota
}
if relayInfo.RealtimeReservedQuota >= reserveTarget {
    return nil
}

if relayInfo.Billing != nil {
    if err := relayInfo.Billing.Reserve(reserveTarget); err != nil {
        return err
    }
    relayInfo.RealtimeReservedQuota = reserveTarget
    if err := recordValuePackageUsageForRelay(relayInfo, reserveTarget); err != nil {
        return err
    }
    logger.LogInfo(ctx, "realtime streaming reserve quota success, quota: "+fmt.Sprintf("%d", reserveTarget))
    return nil
}

err = PostConsumeQuota(relayInfo, quota, 0, false)
if err != nil {
    return err
}
relayInfo.RealtimeReservedQuota += quota
logger.LogInfo(ctx, "realtime streaming consume quota success, quota: "+fmt.Sprintf("%d", quota))
return nil
```

This keeps compatibility for legacy no-BillingSession calls but prevents double counting in the normal relay path.

- [ ] **Step 6: Ensure Reserve updates usage record only through the realtime path**

Do not add usage-record writes to `BillingSession.Reserve` directly. `Reserve` is a generic funding operation; the realtime path calls `recordValuePackageUsageForRelay` after a successful reserve so the rolling table sees the current reserved total.

- [ ] **Step 7: Run realtime accounting tests**

```bash
go test ./service ./model -run 'RealtimeValuePackage|ValuePackageBilling|RollingUsage|RecordValuePackageUsage' -count=1 -timeout=180s
```

Expected: pass. In the new realtime test, final `AmountUsed`, 5h usage, and 7d usage must all equal the final actual quota.

- [ ] **Step 8: Commit realtime accounting fix**

```bash
git add relay/common/relay_info.go service/quota.go service/billing_session.go service/billing_session_test.go model/subscription.go model/value_package_test.go
git commit -m "fix: reserve realtime value package usage without double count"
```

---

## Task 8: Harden concurrency slots and routing context observability

**Files:**

- Modify: `middleware/value_package.go`
- Modify: `middleware/value_package_test.go`

- [ ] **Step 1: Add tests for context isolation and stale recovery**

Append to `middleware/value_package_test.go`:

```go
func TestValuePackageScopeDoesNotOverwriteRoutingOrUserGroups(t *testing.T) {
    setupValuePackageMiddlewareTestDB(t)
    user, plan, sub := seedValuePackageMiddlewareState(t, model.ValuePackageTypeWeek)
    _, err := model.ActivateValuePackage(user.Id, sub.Id)
    require.NoError(t, err)

    gin.SetMode(gin.TestMode)
    router := gin.New()
    router.Use(func(c *gin.Context) {
        common.SetContextKey(c, constant.ContextKeyUserId, user.Id)
        common.SetContextKey(c, constant.ContextKeyUserGroup, model.UserGroupVIP)
        common.SetContextKey(c, constant.ContextKeyUsingGroup, "gpt-plus")
        common.SetContextKey(c, constant.ContextKeyTokenGroup, "gpt-plus")
        c.Next()
    })
    router.Use(ValuePackageGroupScope())
    router.GET("/check", func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{
            "user_group": common.GetContextKeyString(c, constant.ContextKeyUserGroup),
            "using_group": common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
            "token_group": common.GetContextKeyString(c, constant.ContextKeyTokenGroup),
            "package_group": common.GetContextKeyString(c, constant.ContextKeyValuePackageModelGroup),
        })
    })

    recorder := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/check", nil)
    router.ServeHTTP(recorder, req)

    require.Equal(t, http.StatusOK, recorder.Code)
    var body map[string]string
    require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
    require.Equal(t, model.UserGroupVIP, body["user_group"])
    require.Equal(t, "gpt-plus", body["using_group"])
    require.Equal(t, "gpt-plus", body["token_group"])
    require.Equal(t, plan.ModelGroup, body["package_group"])
}
```

- [ ] **Step 2: Run middleware test before logging changes**

```bash
go test ./middleware -run 'ValuePackageScopeDoesNotOverwriteRoutingOrUserGroups|Concurrency|Slot' -count=1 -timeout=120s
```

Expected: routing context isolation test passes after Task 2; concurrency tests keep passing.

- [ ] **Step 3: Add precise logs in concurrency slot code**

In `middleware/value_package.go`, add `common.SysLog` or existing logger calls at these decision points:

```go
common.SysLog(fmt.Sprintf("value package concurrency denied: subscription=%d limit=%d", state.Subscription.Id, normalizeValuePackageConcurrencyLimit(state.Plan.ConcurrencyLimit)))
common.SysLog(fmt.Sprintf("value package concurrency redis acquire error: subscription=%d error=%s", state.Subscription.Id, err.Error()))
common.SysLog(fmt.Sprintf("value package concurrency redis release error: key=%s error=%s", key, err.Error()))
common.SysLog(fmt.Sprintf("value package concurrency redis refresh stopped: key=%s token=%s", key, common.LocalLogPreview(token)))
```

Do not log API keys, token keys, Redis passwords, database credentials, or SSH paths.

- [ ] **Step 4: Keep slot release idempotent**

In `acquireValuePackageRedisSlot`, wrap the returned `release` closure with `sync.Once` so the heartbeat stop channel cannot be closed twice:

```go
var releaseOnce sync.Once
return func() {
    releaseOnce.Do(func() {
        close(stopRefresh)
        if err := releaseValuePackageRedisSlot(key, token, ttlSeconds); err != nil {
            common.SysLog(fmt.Sprintf("value package concurrency redis release error: key=%s error=%s", key, err.Error()))
        }
    })
}, true, nil
```

If `sync` is not already imported in `middleware/value_package.go`, add it to the import list.

- [ ] **Step 5: Run middleware tests**

```bash
go test ./middleware -run 'ValuePackage|Concurrency|Slot|Scope' -count=1 -timeout=180s
```

Expected: pass.

- [ ] **Step 6: Commit concurrency hardening**

```bash
git add middleware/value_package.go middleware/value_package_test.go
git commit -m "test: guard value package routing and concurrency"
```

---

## Task 9: Regression coverage for orders, deletion, redemption, affiliate, and realtime admin table

**Files:**

- Modify: `model/order_management_test.go`
- Modify: `controller/order_management_test.go`
- Modify: `service/order_mail_check_job_test.go`
- Modify: `model/redemption_subscription_test.go`
- Modify: `web/default/src/features/redemption-codes/components/redemptions-mutate-drawer.tsx`
- Modify: `web/default/src/features/redemption-codes/components/redemptions-mutate-drawer-source.test.ts`
- Modify: `web/default/src/features/order-management/components/value-package-usage-table.tsx`
- Modify: `web/default/src/features/order-management/components/value-package-usage-table-source.test.ts`
- Modify: `web/default/src/features/order-management/order-management-source.test.ts`

- [ ] **Step 1: Run current regression targets and record failures**

```bash
go test ./model ./controller ./service -run 'OrderManagement|ValuePackageOrder|MailCheck|Redemption|Affiliate|ValuePackageUsage' -count=1 -timeout=240s
cd web/default
bun test src/features/redemption-codes/lib/redemption-form.test.ts \
  src/features/redemption-codes/components/redemptions-mutate-drawer-source.test.ts \
  src/features/order-management/components/value-package-usage-table-source.test.ts \
  src/features/order-management/order-management-source.test.ts
```

Expected before new source guards: existing tests may pass, and missing source guards will fail once added.

- [ ] **Step 2: Add redemption empty-state source guard**

In `web/default/src/features/redemption-codes/components/redemptions-mutate-drawer-source.test.ts`, add:

```ts
test('redemption drawer explains when no enabled value package plans exist', () => {
  assert.match(source, /No enabled day, week, or month packages are available/)
  assert.match(source, /plan_kind === 'value_package'/)
  assert.match(source, /plan\.enabled/)
})
```

- [ ] **Step 3: Add empty-state UI text**

In `web/default/src/features/redemption-codes/components/redemptions-mutate-drawer.tsx`, in the value-package plan selector area, render this when `valuePackagePlanOptions.length === 0`:

```tsx
<p className='text-muted-foreground rounded-md border border-dashed px-3 py-2 text-xs'>
  {t('No enabled day, week, or month packages are available. Enable a package plan first.')}
</p>
```

Add translations for:

```text
No enabled day, week, or month packages are available. Enable a package plan first.
```

- [ ] **Step 4: Add admin realtime table source guard**

In `web/default/src/features/order-management/components/value-package-usage-table-source.test.ts`, assert these labels and refresh behavior:

```ts
test('value package usage table keeps per-user rolling quota columns', () => {
  assert.match(source, /5h|5 小时|used_5h/)
  assert.match(source, /7d|7 天|used_7d/)
  assert.match(source, /total_remaining/)
  assert.match(source, /refetchInterval:\s*15000/)
})
```

- [ ] **Step 5: Keep the deleted value-package mail-scan regression test green**

`service/order_mail_check_job_test.go` already contains `TestRunBatchMailCheckSkipsDeletedValuePackageOrders`. Verify that it still seeds a value-package plan, creates a deleted value-package order/session, runs the batch mail-check path, and asserts the deleted trade number is not returned to visible order management data. If a Task 9 change breaks the test, fix the production code rather than weakening the assertion.

Run the exact test:

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening
go test ./service -run TestRunBatchMailCheckSkipsDeletedValuePackageOrders -count=1 -timeout=120s
```

Expected: pass.

- [ ] **Step 6: Keep the cash affiliate regression test green**

`model/value_package_test.go` already contains `TestCompleteValuePackageOrderCreatesAffiliateCommissionForInviter`. Verify that the test still asserts one `AffiliateCommission` row is created for an LDXP cash week-card purchase and that retrying completion remains idempotent.

Run the exact test:

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening
go test ./model -run TestCompleteValuePackageOrderCreatesAffiliateCommissionForInviter -count=1 -timeout=120s
```

Expected: pass.

- [ ] **Step 7: Run regression suite**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening
go test ./model ./controller ./service -run 'OrderManagement|ValuePackageOrder|MailCheck|Redemption|Affiliate|ValuePackageUsage' -count=1 -timeout=300s
cd web/default
bun test src/features/redemption-codes/lib/redemption-form.test.ts \
  src/features/redemption-codes/components/redemptions-mutate-drawer-source.test.ts \
  src/features/order-management/components/value-package-usage-table-source.test.ts \
  src/features/order-management/order-management-source.test.ts
```

Expected: pass.

- [ ] **Step 8: Commit regression and UX coverage**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening
git add model/order_management_test.go controller/order_management_test.go service/order_mail_check_job_test.go model/redemption_subscription_test.go \
  web/default/src/features/redemption-codes/components/redemptions-mutate-drawer.tsx \
  web/default/src/features/redemption-codes/components/redemptions-mutate-drawer-source.test.ts \
  web/default/src/features/order-management/components/value-package-usage-table.tsx \
  web/default/src/features/order-management/components/value-package-usage-table-source.test.ts \
  web/default/src/features/order-management/order-management-source.test.ts \
  web/default/src/i18n/locales/en.json \
  web/default/src/i18n/locales/zh.json \
  web/default/src/i18n/locales/fr.json \
  web/default/src/i18n/locales/ja.json \
  web/default/src/i18n/locales/ru.json \
  web/default/src/i18n/locales/vi.json
git commit -m "test: cover value package admin flows"
```

---

## Task 10: Full verification and i18n sync

**Files:**

- Verify only unless tests force a deterministic source fix.

- [ ] **Step 1: Run focused backend suite**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening
go test ./middleware ./relay/common ./relay/helper ./relay/channel/openai ./service ./model ./controller -run 'ValuePackage|BillingRatio|OrderManagement|Redemption|Realtime|Wss|Group|QuickStart' -count=1 -timeout=300s
```

Expected: pass.

- [ ] **Step 2: Run full impacted backend packages**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening
go test ./middleware ./relay/common ./relay/helper ./relay/channel/openai ./service ./model ./controller -count=1 -timeout=300s
```

Expected: pass.

- [ ] **Step 3: Run focused frontend tests**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening/web/default
bun test src/features/quick-start/quick-start-api-key.test.ts \
  src/features/quick-start/quick-start-page-source.test.ts \
  src/features/quick-start/quick-start-locales.test.ts \
  src/features/keys/lib/api-key-display.test.ts \
  src/features/keys/components/api-keys-columns-source.test.ts \
  src/features/value-packages/lib/billing-display.test.ts \
  src/features/usage-logs/components/columns/common-logs-columns.test.ts \
  src/features/redemption-codes/lib/redemption-form.test.ts \
  src/features/redemption-codes/components/redemptions-mutate-drawer-source.test.ts \
  src/features/order-management/components/value-package-usage-table-source.test.ts \
  src/features/order-management/order-management-source.test.ts
```

Expected: pass.

- [ ] **Step 4: Run frontend typecheck, build, and i18n sync**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening/web/default
bun run typecheck
bun run build
bun run i18n:sync
```

Expected: each command exits 0. If `bun run i18n:sync` rewrites locale files deterministically, include those locale diffs in the final verification commit.

- [ ] **Step 5: Scan for forbidden regression patterns**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening
rg -n "userGroup = valuePackageGroup|ContextKeyUsingGroup.*ValuePackage|ContextKeyTokenGroup.*ValuePackage|groupRatios\[packageGroup\]|Failed to copy the new API key" relay service middleware web/default/src || true
```

Expected: no active production source line shows any of these regression patterns. Test fixtures may contain strings that assert the old behavior is gone.

- [ ] **Step 6: Commit verification-only fixes**

If typecheck, build, or i18n sync changed files, commit them:

```bash
git status --short
git add web/default/src/i18n/locales/en.json \
  web/default/src/i18n/locales/zh.json \
  web/default/src/i18n/locales/fr.json \
  web/default/src/i18n/locales/ja.json \
  web/default/src/i18n/locales/ru.json \
  web/default/src/i18n/locales/vi.json
git commit -m "chore: sync value package translations"
```

If there are no changed files, do not create an empty commit.

---

## Task 11: Merge to local and GitHub `main`, without deploying server

**Files:**

- Git operations only.

- [ ] **Step 1: Confirm local history**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening
git log --oneline --decorate -12
git status --short --untracked-files=all
```

Expected: all implementation commits are on local `main` or on the chosen implementation branch ready to merge. Working tree is clean except intentionally untracked scratch files, which must not be committed.

- [ ] **Step 2: Merge into local `main` when implementation branch is used**

If implementation was performed on a `codex/...` branch, merge it to local `main`:

```bash
git switch main
git pull --ff-only origin main
git merge --ff-only codex/value-package-stabilization-quick-start
```

Expected: fast-forward merge succeeds. If the executor implemented directly on `main`, run only:

```bash
git switch main
git pull --ff-only origin main
```

- [ ] **Step 3: Push GitHub `main`**

```bash
git push origin main
```

Expected: push succeeds. This satisfies the user's earlier requirement to sync local and GitHub `main` before any server deployment.

- [ ] **Step 4: Do not deploy server in this task**

Record the pushed SHA:

```bash
git rev-parse HEAD
```

Expected: a single SHA that can be used for later deployment. Stop here unless the user explicitly says to上线服务器.

---

## Task 12: Server rollout runbook for later approval

**Files:**

- Runbook only. Execute only after the user explicitly requests server deployment.

- [ ] **Step 1: Before deployment, record server and local SHAs**

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening
git rev-parse HEAD
```

On the server, after login using the existing Yunbay key location provided by the user, record:

```bash
cd /path/to/yunbay
git rev-parse HEAD
git status --short
```

Do not print `.env`, API keys, payment keys, SSH private keys, database passwords, or cloud credentials.

- [ ] **Step 2: Deploy the exact GitHub `main` SHA**

On the server:

```bash
cd /path/to/yunbay
git fetch origin
git checkout main
git pull --ff-only origin main
git rev-parse HEAD
```

Expected: server SHA equals the SHA pushed in Task 11.

- [ ] **Step 3: Build/restart with existing production commands**

Use the project’s existing server deployment script or service commands. Capture the command names and exit codes in the deployment note. Do not invent a new process manager command when the server already has one.

- [ ] **Step 4: Post-deploy log checks**

Run these checks on the server log directory used by the current deployment:

```bash
grep -E '分组 (day-card|week-card|month-card) 下模型 .* 无可用渠道' -R logs/ || true
grep -E 'value package concurrency denied|超值套餐并发请求数已达上限' -R logs/ | tail -50 || true
grep -E 'subscription_ratio_applied|value_package_effective_ratio|value_package_model_group' -R logs/ | tail -50 || true
```

Expected:

```text
第一条无结果。
第二条只在真实并发请求时出现。
第三条能看到套餐计费标记。
```

- [ ] **Step 5: Manual smoke checks**

Use a non-production-risk test user/order:

```text
1. 开启日/周/月卡后请求 gpt-plus 模型，请求不得报 day-card/week-card/month-card distributor 无渠道。
2. API key 页面显示 gpt-plus 或 auto，同时展示套餐 1x。
3. 使用日志列表显示 Package 1x 或对应中文“套餐 1x”。
4. 日/周/月卡管理员实时表 5h、7d、总额度随请求变化。
5. 快速引导第 4 页模拟剪贴板失败后仍能进入第 5 页并显示 masked generated key。
```

- [ ] **Step 6: Rollback triggers**

Rollback to the recorded previous server SHA if any condition occurs:

```text
日/周/月卡用户再次出现 day-card/week-card/month-card distributor 无渠道错误。
套餐用户无法使用原本 Plus / Pro 模型。
单次请求导致订阅额度明显重复扣减。
realtime 套餐请求出现 usage record 与 AmountUsed 明显不一致。
快速引导生成 key 后丢失 key，无法进入第 5 步。
删除订单后再次被邮件扫描或重新出现在订单列表。
```

---

## Acceptance checklist

- [ ] `RelayInfo.UserGroup` stays the real user group under active day/week/month card.
- [ ] `RelayInfo.UsingGroup` and distributor routing stay on `gpt-plus`, `gpt-pro`, or the resolved `auto` group.
- [ ] `BillingUserGroup` / `ValuePackageBillingGroup` carries `day-card`, `week-card`, or `month-card` only for billing.
- [ ] API key page shows `gpt-plus · Package 1x` or `auto · Package 1x` when a package is active.
- [ ] Usage logs clearly label package billing and include original routing ratio for audit.
- [ ] Realtime WebSocket package requests update `UserSubscription.AmountUsed` and `ValuePackageUsageRecord` without double counting.
- [ ] Admin order management still shows day/week/month orders, deletion marks, realtime table, and affiliate information.
- [ ] User-side quick-start keeps the generated API key when clipboard copy fails after reveal succeeds.
- [ ] `bun run typecheck`, `bun run build`, focused frontend tests, and focused backend tests pass.
- [ ] GitHub `main` is updated before any server deployment.

## Self-review notes

- Spec coverage: R1/R2/R3/R4 are covered by Task 9; R5/R6 are covered by Tasks 2, 5, and 6; R7 is covered by Task 7; R8 is covered by Task 8; R9/R10 are covered by Task 4.
- Placeholder scan: this plan intentionally avoids unresolved placeholders. Each task has concrete files, code snippets, commands, and expected outcomes.
- Type consistency: backend fields use `RealUserGroup`, `BillingUserGroup`, `ValuePackageBillingGroup`, and `ValuePackageBillingState`; frontend fields use `billing`, `effective_ratio`, and helper `getActiveValuePackageBillingRatio` consistently.
