# Value Package Bugfix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix value package glow conflicts, VIP/package effective billing groups, package quota progress display, quota exhaustion prompts, and sidebar attention animation.

**Architecture:** Keep `users.group` as the persistent identity group, but override relay request context while a value package is enabled so pricing and routing use the package model group. Add a read-only usage summary to the existing `ValuePackageState` response, render quota progress on the existing value package cards, and add CSS-only visual effects for package glow and sidebar attention.

**Tech Stack:** Go 1.22+, Gin, GORM, SQLite/MySQL/PostgreSQL-compatible queries, React 19, TypeScript, Base UI, Tailwind CSS, Bun, node:test.

**Baseline:** This plan is based on GitHub `origin/main` commit `d7123a73345f3d9bbe0aeb5326e4bcd7d22f53ce` plus spec commit `6456abae`. Do not modify `infra/sub2api/frontend/pnpm-lock.yaml`, `infra/sub2api/frontend/package.json`, or `infra/sub2api/backend/go.mod`.

---

## File Structure

- Modify `middleware/value_package.go`: override effective relay user group while a package is active and use a unified quota exhausted prompt.
- Modify `middleware/value_package_test.go`: add regression tests for VIP context override, disabled preference rollback behavior, and exhausted prompt text.
- Modify `model/subscription.go`: add `ValuePackageUsageSummary`, compute usage summary in `GetValuePackageState`, and expose quota exhaustion constants.
- Modify `model/value_package_test.go`: add usage summary and identity group regression tests.
- Modify `web/default/src/features/value-packages/types.ts`: add `ValuePackageUsageSummary` type and optional `usage` on `ValuePackageState`.
- Modify `web/default/src/features/value-packages/components/value-package-card.tsx`: render usage progress bars and exhaustion alert.
- Modify `web/default/src/features/value-packages/components/value-package-card-source.test.ts`: assert progress and prompt source requirements.
- Modify `web/default/src/styles/index.css`: make package glow 淡蓝/blue and add sidebar button pulse CSS.
- Modify `web/default/src/components/layout/types.ts`: add an optional `attention` marker on nav items.
- Modify `web/default/src/hooks/sidebar-data-model.ts`: mark all `/value-packages` sidebar entries with `attention: 'value-packages'`.
- Modify `web/default/src/hooks/sidebar-data-model.test.ts`: verify ordinary and admin value package entries carry the attention marker.
- Modify `web/default/src/components/layout/components/nav-group.tsx`: apply pulse class to attention-marked nav links.
- Create `web/default/src/components/layout/components/nav-group-source.test.ts`: source-level assertion for the pulse class.
- Modify locale files under `web/default/src/i18n/locales/{en,zh,fr,ja,ru,vi}.json` only if `bun run i18n:sync` reports new keys are needed.

---

### Task 1: Backend relay context override for package billing group

**Files:**
- Modify: `middleware/value_package_test.go`
- Modify: `middleware/value_package.go`

- [ ] **Step 1: Write the failing middleware test for VIP users**

Add this test after `TestValuePackageMiddlewareForcesPackageGroup` in `middleware/value_package_test.go`:

```go
func TestValuePackageMiddlewareOverridesUserGroupForPackageBilling(t *testing.T) {
	setupValuePackageMiddlewareTestDB(t)
	user, plan, sub := seedValuePackageMiddlewareState(t, true, 1000, 5000, 1)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("group", model.UserGroupVIP).Error)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUserId, user.Id)
		common.SetContextKey(c, constant.ContextKeyUserGroup, model.UserGroupVIP)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "gpt-plus")
		common.SetContextKey(c, constant.ContextKeyTokenGroup, "gpt-plus")
		c.Next()
	})
	router.Use(ValuePackageEntitlement())
	router.POST("/relay", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_group":                    common.GetContextKeyString(c, constant.ContextKeyUserGroup),
			"using_group":                   common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
			"token_group":                   common.GetContextKeyString(c, constant.ContextKeyTokenGroup),
			"value_package_subscription_id": common.GetContextKeyInt(c, constant.ContextKeyValuePackageSubscriptionId),
		})
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/relay", nil)
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), fmt.Sprintf(`"value_package_subscription_id":%d`, sub.Id))
	require.Contains(t, recorder.Body.String(), `"user_group":"`+plan.ModelGroup+`"`)
	require.Contains(t, recorder.Body.String(), `"using_group":"`+plan.ModelGroup+`"`)
	require.Contains(t, recorder.Body.String(), `"token_group":"`+plan.ModelGroup+`"`)

	var reloaded model.User
	require.NoError(t, model.DB.First(&reloaded, user.Id).Error)
	require.Equal(t, model.UserGroupVIP, reloaded.Group)
}
```

- [ ] **Step 2: Run the new test and verify it fails**

Run:

```bash
go test ./middleware -run TestValuePackageMiddlewareOverridesUserGroupForPackageBilling -count=1
```

Expected: the test fails because `"user_group":"vip"` is still present instead of the package model group.

- [ ] **Step 3: Implement request-scoped effective user group override**

In `middleware/value_package.go`, add this package-level constant near `const valuePackageConcurrencySlotTTL`:

```go
const valuePackageOriginalUserGroupContextKey = "value_package_original_user_group"
```

Replace `applyValuePackageGroupScope` with:

```go
func applyValuePackageGroupScope(c *gin.Context, state *model.ValuePackageState) {
	modelGroup := strings.TrimSpace(state.Plan.ModelGroup)
	if modelGroup == "" {
		return
	}
	originalUserGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	if originalUserGroup != "" {
		c.Set(valuePackageOriginalUserGroupContextKey, originalUserGroup)
	}
	common.SetContextKey(c, constant.ContextKeyUserGroup, modelGroup)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, modelGroup)
	common.SetContextKey(c, constant.ContextKeyTokenGroup, modelGroup)
	common.SetContextKey(c, constant.ContextKeyValuePackageSubscriptionId, state.Subscription.Id)
	common.SetContextKey(c, constant.ContextKeyValuePackagePlanId, state.Plan.Id)
	common.SetContextKey(c, constant.ContextKeyValuePackageModelGroup, modelGroup)
	common.SetContextKey(c, constant.ContextKeyValuePackagePackageType, state.Plan.PackageType)
}
```

- [ ] **Step 4: Add disabled-preference guard test**

Add this test after `TestValuePackageMiddlewareDisabledPreferenceDoesNotForceGroup`:

```go
func TestValuePackageMiddlewareDisabledPreferenceKeepsOriginalUserGroup(t *testing.T) {
	setupValuePackageMiddlewareTestDB(t)
	user, _, _ := seedValuePackageMiddlewareState(t, false, 1000, 5000, 1)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUserId, user.Id)
		common.SetContextKey(c, constant.ContextKeyUserGroup, model.UserGroupVIP)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "gpt-plus")
		common.SetContextKey(c, constant.ContextKeyTokenGroup, "gpt-plus")
		c.Next()
	})
	router.Use(ValuePackageEntitlement())
	router.POST("/relay", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_group":  common.GetContextKeyString(c, constant.ContextKeyUserGroup),
			"using_group": common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
			"token_group": common.GetContextKeyString(c, constant.ContextKeyTokenGroup),
		})
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/relay", nil)
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"user_group":"vip"`)
	require.Contains(t, recorder.Body.String(), `"using_group":"gpt-plus"`)
	require.Contains(t, recorder.Body.String(), `"token_group":"gpt-plus"`)
}
```

- [ ] **Step 5: Run middleware tests for this task**

Run:

```bash
go test ./middleware -run 'TestValuePackageMiddleware(OverridesUserGroupForPackageBilling|DisabledPreferenceKeepsOriginalUserGroup|ForcesPackageGroup|DisabledPreferenceDoesNotForceGroup)' -count=1
```

Expected: all selected middleware tests pass.

- [ ] **Step 6: Commit Task 1**

```bash
gofmt -w middleware/value_package.go middleware/value_package_test.go
git add middleware/value_package.go middleware/value_package_test.go
git commit -m "fix: use package group for value package billing context"
```

---

### Task 2: Backend usage summary for value package progress

**Files:**
- Modify: `model/value_package_test.go`
- Modify: `model/subscription.go`

- [ ] **Step 1: Write failing usage summary tests**

Add these tests near the existing value package usage tests in `model/value_package_test.go`:

```go
func TestGetValuePackageStateIncludesUsageSummary(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3401, UserGroupVIP)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	day.TotalAmount = 200
	day.Limit5hAmount = 100
	day.Limit7dAmount = 150
	require.NoError(t, DB.Save(&day).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-10, now+3600)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("amount_used", int64(80)).Error)
	require.NoError(t, upsertValuePackagePreferenceTx(DB, user.Id, true, sub.Id))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "summary-recent", Quota: 40, CreatedAt: now}))
	require.NoError(t, RecordValuePackageUsage(&ValuePackageUsageRecord{UserId: user.Id, UserSubscriptionId: sub.Id, PlanId: day.Id, PackageType: day.PackageType, ModelGroup: day.ModelGroup, RequestId: "summary-older", Quota: 30, CreatedAt: now - 6*3600}))

	state, err := GetValuePackageState(user.Id)

	require.NoError(t, err)
	require.NotNil(t, state)
	require.NotNil(t, state.Usage)
	require.EqualValues(t, 80, state.Usage.TotalUsed)
	require.EqualValues(t, 200, state.Usage.TotalLimit)
	require.EqualValues(t, 120, state.Usage.TotalRemaining)
	require.InDelta(t, 40.0, state.Usage.TotalPercent, 0.001)
	require.EqualValues(t, 40, state.Usage.Used5h)
	require.EqualValues(t, 100, state.Usage.Limit5h)
	require.InDelta(t, 40.0, state.Usage.Percent5h, 0.001)
	require.EqualValues(t, 70, state.Usage.Used7d)
	require.EqualValues(t, 150, state.Usage.Limit7d)
	require.InDelta(t, 46.666, state.Usage.Percent7d, 0.01)
	require.False(t, state.Usage.Exhausted)
	require.Empty(t, state.Usage.ExhaustedReason)

	var reloaded User
	require.NoError(t, DB.First(&reloaded, user.Id).Error)
	require.Equal(t, UserGroupVIP, reloaded.Group)
}

func TestGetValuePackageStateMarksExhaustedUsageSummary(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3402, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	day.TotalAmount = 100
	day.Limit5hAmount = 1000
	day.Limit7dAmount = 1000
	require.NoError(t, DB.Save(&day).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-10, now+3600)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("amount_used", int64(100)).Error)
	require.NoError(t, upsertValuePackagePreferenceTx(DB, user.Id, true, sub.Id))

	state, err := GetValuePackageState(user.Id)

	require.NoError(t, err)
	require.NotNil(t, state.Usage)
	require.True(t, state.Usage.Exhausted)
	require.Equal(t, ValuePackageExhaustedReasonTotal, state.Usage.ExhaustedReason)
	require.Equal(t, ValuePackageQuotaExhaustedUserMessage, state.Usage.ExhaustedMessage)
}
```

- [ ] **Step 2: Run the new model tests and verify they fail**

Run:

```bash
go test ./model -run 'TestGetValuePackageState(IncludesUsageSummary|MarksExhaustedUsageSummary)' -count=1
```

Expected: compile failure because `ValuePackageState.Usage`, `ValuePackageExhaustedReasonTotal`, and `ValuePackageQuotaExhaustedUserMessage` do not exist yet.

- [ ] **Step 3: Add usage summary types and constants**

In `model/subscription.go`, add these constants after the existing `ValuePackageLevel*` constants:

```go
const (
	ValuePackageExhaustedReasonTotal = "total_quota_exhausted"
	ValuePackageExhaustedReason5h    = "limit_5h_exhausted"
	ValuePackageExhaustedReason7d    = "limit_7d_exhausted"
)

const ValuePackageQuotaExhaustedUserMessage = "当前余额已用完，建议暂停使用，使用 API 或等时间跑完再使用"
```

Add this struct immediately above `type ValuePackageState struct`:

```go
type ValuePackageUsageSummary struct {
	TotalUsed        int64   `json:"total_used"`
	TotalLimit       int64   `json:"total_limit"`
	TotalRemaining   int64   `json:"total_remaining"`
	TotalPercent     float64 `json:"total_percent"`
	Used5h           int64   `json:"used_5h"`
	Limit5h          int64   `json:"limit_5h"`
	Percent5h        float64 `json:"percent_5h"`
	Used7d           int64   `json:"used_7d"`
	Limit7d          int64   `json:"limit_7d"`
	Percent7d        float64 `json:"percent_7d"`
	Exhausted        bool    `json:"exhausted"`
	ExhaustedReason  string  `json:"exhausted_reason"`
	ExhaustedMessage string  `json:"exhausted_message"`
}
```

Change `ValuePackageState` to:

```go
type ValuePackageState struct {
	Preference   UserValuePackagePreference `json:"preference"`
	Subscription *UserSubscription          `json:"subscription,omitempty"`
	Plan         *SubscriptionPlan          `json:"plan,omitempty"`
	Usage        *ValuePackageUsageSummary  `json:"usage,omitempty"`
}
```

- [ ] **Step 4: Add usage summary builder**

In `model/subscription.go`, add these functions after `getValuePackageStateTx` or before `CompleteValuePackageOrder`:

```go
func valuePackagePercent(used int64, limit int64) float64 {
	if limit <= 0 || used <= 0 {
		return 0
	}
	percent := float64(used) * 100 / float64(limit)
	if percent > 100 {
		return 100
	}
	if percent < 0 {
		return 0
	}
	return percent
}

func buildValuePackageUsageSummaryTx(tx *gorm.DB, userId int, sub *UserSubscription, plan *SubscriptionPlan, now int64) (*ValuePackageUsageSummary, error) {
	if tx == nil {
		tx = DB
	}
	if sub == nil || plan == nil || sub.Id <= 0 {
		return nil, nil
	}
	if now <= 0 {
		now = getDBTimestampTx(tx)
	}
	used5h, used7d, err := getValuePackageWindowUsageTx(tx, userId, sub.Id, now)
	if err != nil {
		return nil, err
	}
	totalRemaining := int64(0)
	if sub.AmountTotal > 0 && sub.AmountTotal > sub.AmountUsed {
		totalRemaining = sub.AmountTotal - sub.AmountUsed
	}
	summary := &ValuePackageUsageSummary{
		TotalUsed:      sub.AmountUsed,
		TotalLimit:     sub.AmountTotal,
		TotalRemaining: totalRemaining,
		TotalPercent:   valuePackagePercent(sub.AmountUsed, sub.AmountTotal),
		Used5h:         used5h,
		Limit5h:        plan.Limit5hAmount,
		Percent5h:      valuePackagePercent(used5h, plan.Limit5hAmount),
		Used7d:         used7d,
		Limit7d:        plan.Limit7dAmount,
		Percent7d:      valuePackagePercent(used7d, plan.Limit7dAmount),
	}
	switch {
	case sub.AmountTotal > 0 && sub.AmountUsed >= sub.AmountTotal:
		summary.Exhausted = true
		summary.ExhaustedReason = ValuePackageExhaustedReasonTotal
	case plan.Limit5hAmount > 0 && used5h >= plan.Limit5hAmount:
		summary.Exhausted = true
		summary.ExhaustedReason = ValuePackageExhaustedReason5h
	case plan.Limit7dAmount > 0 && used7d >= plan.Limit7dAmount:
		summary.Exhausted = true
		summary.ExhaustedReason = ValuePackageExhaustedReason7d
	}
	if summary.Exhausted {
		summary.ExhaustedMessage = ValuePackageQuotaExhaustedUserMessage
	}
	return summary, nil
}
```

- [ ] **Step 5: Attach usage summary in state lookup**

In `getValuePackageStateTx`, replace the tail:

```go
state.Subscription = &sub
state.Plan = plan
return state, nil
```

with:

```go
state.Subscription = &sub
state.Plan = plan
usage, err := buildValuePackageUsageSummaryTx(tx, userId, &sub, plan, now)
if err != nil {
	return nil, err
}
state.Usage = usage
return state, nil
```

- [ ] **Step 6: Run model tests for this task**

Run:

```bash
gofmt -w model/subscription.go model/value_package_test.go
go test ./model -run 'TestGetValuePackageState(IncludesUsageSummary|MarksExhaustedUsageSummary)' -count=1
```

Expected: selected model tests pass.

- [ ] **Step 7: Commit Task 2**

```bash
git add model/subscription.go model/value_package_test.go
git commit -m "feat: expose value package usage summary"
```

---

### Task 3: Unified package quota exhaustion prompt

**Files:**
- Modify: `middleware/value_package_test.go`
- Modify: `middleware/value_package.go`
- Modify: `model/value_package_test.go`
- Modify: `model/subscription.go`

- [ ] **Step 1: Write failing middleware prompt test**

In `middleware/value_package_test.go`, add this assertion inside `TestValuePackageMiddlewareRejectsOverRollingWindows` after the first `http.StatusForbidden` check:

```go
	require.Contains(t, recorder.Body.String(), model.ValuePackageQuotaExhaustedUserMessage)
```

Also add the same assertion after the second `http.StatusForbidden` check in that test:

```go
	require.Contains(t, recorder.Body.String(), model.ValuePackageQuotaExhaustedUserMessage)
```

- [ ] **Step 2: Write failing total quota prompt test**

Add this test after `TestPreConsumeValuePackageSubscriptionOnlyAcceptsValuePackageAndIsIdempotent` in `model/value_package_test.go`:

```go
func TestPreConsumeValuePackageSubscriptionUsesUserFacingExhaustedMessage(t *testing.T) {
	setupValuePackageTestDB(t)
	user := createValuePackageUser(t, 3403, UserGroupTiyan)
	day := createValuePackagePlan(t, ValuePackageTypeDay, ValuePackageLevelDay, 1, 3.9)
	day.TotalAmount = 10
	require.NoError(t, DB.Save(&day).Error)
	now := common.GetTimestamp()
	sub := createActiveValuePackageSub(t, user.Id, day, now-10, now+3600)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Update("amount_used", int64(10)).Error)

	_, err := PreConsumeValuePackageSubscription("vp-exhausted-message", user.Id, sub.Id, 1)

	require.Error(t, err)
	require.Contains(t, err.Error(), "subscription quota insufficient")
	require.Contains(t, err.Error(), ValuePackageQuotaExhaustedUserMessage)
}
```

- [ ] **Step 3: Run exhaustion tests and verify they fail**

Run:

```bash
go test ./middleware -run TestValuePackageMiddlewareRejectsOverRollingWindows -count=1
go test ./model -run TestPreConsumeValuePackageSubscriptionUsesUserFacingExhaustedMessage -count=1
```

Expected: both fail because the user-facing message is not returned yet.

- [ ] **Step 4: Use unified message in middleware**

In `middleware/value_package.go`, replace these two blocks:

```go
if state.Plan.Limit5hAmount > 0 && used5h >= state.Plan.Limit5hAmount {
	abortWithOpenAiMessage(c, http.StatusForbidden, fmt.Sprintf("超值套餐 5 小时滚动额度已用尽，已用 %d / 限额 %d", used5h, state.Plan.Limit5hAmount))
	return
}
if state.Plan.Limit7dAmount > 0 && used7d >= state.Plan.Limit7dAmount {
	abortWithOpenAiMessage(c, http.StatusForbidden, fmt.Sprintf("超值套餐 7 天滚动额度已用尽，已用 %d / 限额 %d", used7d, state.Plan.Limit7dAmount))
	return
}
```

with:

```go
if state.Plan.Limit5hAmount > 0 && used5h >= state.Plan.Limit5hAmount {
	abortWithOpenAiMessage(c, http.StatusForbidden, fmt.Sprintf("%s（5 小时：已用 %d / 限额 %d）", model.ValuePackageQuotaExhaustedUserMessage, used5h, state.Plan.Limit5hAmount))
	return
}
if state.Plan.Limit7dAmount > 0 && used7d >= state.Plan.Limit7dAmount {
	abortWithOpenAiMessage(c, http.StatusForbidden, fmt.Sprintf("%s（7 天：已用 %d / 限额 %d）", model.ValuePackageQuotaExhaustedUserMessage, used7d, state.Plan.Limit7dAmount))
	return
}
```

- [ ] **Step 5: Use unified message in value package pre-consume**

In `model/subscription.go`, replace these three error returns inside `PreConsumeValuePackageSubscription`:

```go
return fmt.Errorf("subscription quota insufficient, need=%d", amount)
```

```go
return fmt.Errorf("subscription quota insufficient, 5h rolling limit exceeded, need=%d", amount)
```

```go
return fmt.Errorf("subscription quota insufficient, 7d rolling limit exceeded, need=%d", amount)
```

with these three returns respectively:

```go
return fmt.Errorf("subscription quota insufficient: %s, need=%d", ValuePackageQuotaExhaustedUserMessage, amount)
```

```go
return fmt.Errorf("subscription quota insufficient: %s, 5h rolling limit exceeded, need=%d", ValuePackageQuotaExhaustedUserMessage, amount)
```

```go
return fmt.Errorf("subscription quota insufficient: %s, 7d rolling limit exceeded, need=%d", ValuePackageQuotaExhaustedUserMessage, amount)
```

- [ ] **Step 6: Run exhaustion tests**

Run:

```bash
gofmt -w middleware/value_package.go middleware/value_package_test.go model/subscription.go model/value_package_test.go
go test ./middleware -run TestValuePackageMiddlewareRejectsOverRollingWindows -count=1
go test ./model -run TestPreConsumeValuePackageSubscriptionUsesUserFacingExhaustedMessage -count=1
```

Expected: selected tests pass.

- [ ] **Step 7: Commit Task 3**

```bash
git add middleware/value_package.go middleware/value_package_test.go model/subscription.go model/value_package_test.go
git commit -m "fix: show value package quota exhausted guidance"
```

---

### Task 4: Frontend usage progress and exhausted alert

**Files:**
- Modify: `web/default/src/features/value-packages/types.ts`
- Modify: `web/default/src/features/value-packages/components/value-package-card-source.test.ts`
- Modify: `web/default/src/features/value-packages/components/value-package-card.tsx`

- [ ] **Step 1: Write failing source test for progress UI**

Extend `value package card source keeps required user controls and limit copy` in `web/default/src/features/value-packages/components/value-package-card-source.test.ts` with these assertions:

```ts
  assert.match(source, /Progress/)
  assert.match(source, /Package total limit/)
  assert.match(source, /formatUsageAmount/)
  assert.match(source, /getProgressToneClass/)
  assert.match(source, /当前余额已用完，建议暂停使用，使用 API 或等时间跑完再使用/)
```

- [ ] **Step 2: Run source test and verify it fails**

Run:

```bash
cd web/default
bun test src/features/value-packages/components/value-package-card-source.test.ts
```

Expected: FAIL because `Progress`, `Package total limit`, `formatUsageAmount`, `getProgressToneClass`, and the new prompt are absent.

- [ ] **Step 3: Add frontend usage types**

In `web/default/src/features/value-packages/types.ts`, add this interface above `ValuePackageState`:

```ts
export interface ValuePackageUsageSummary {
  total_used: number
  total_limit: number
  total_remaining: number
  total_percent: number
  used_5h: number
  limit_5h: number
  percent_5h: number
  used_7d: number
  limit_7d: number
  percent_7d: number
  exhausted: boolean
  exhausted_reason: string
  exhausted_message: string
}
```

Change `ValuePackageState` to:

```ts
export interface ValuePackageState {
  preference: UserValuePackagePreference
  subscription?: UserSubscription | null
  plan?: ValuePackagePlan | null
  usage?: ValuePackageUsageSummary | null
}
```

- [ ] **Step 4: Add progress helpers and import**

In `web/default/src/features/value-packages/components/value-package-card.tsx`, change the icon import to include `AlertTriangle`:

```ts
import {
  AlertTriangle,
  Clock,
  Gauge,
  Loader2,
  PauseCircle,
  Play,
  Shield,
} from 'lucide-react'
```

Add this import below the button import:

```ts
import { Progress } from '@/components/ui/progress'
```

Add these helpers after `formatLimitAmount`:

```ts
function clampPercent(value: number): number {
  if (!Number.isFinite(value) || value <= 0) {
    return 0
  }
  return Math.min(100, Math.max(0, value))
}

function formatUsageAmount(amount: number): string {
  return new Intl.NumberFormat().format(Math.max(0, Math.round(amount || 0)))
}

function getProgressToneClass(percent: number): string {
  if (percent >= 100) {
    return '[&_[data-slot=progress-indicator]]:bg-destructive'
  }
  if (percent >= 80) {
    return '[&_[data-slot=progress-indicator]]:bg-warning'
  }
  return '[&_[data-slot=progress-indicator]]:bg-sky-500'
}
```

Add this component before `export function ValuePackageCard`:

```tsx
function LimitProgressRow({
  label,
  used,
  limit,
  percent,
}: {
  label: string
  used: number
  limit: number
  percent: number
}) {
  if (!limit || limit <= 0) {
    return null
  }

  const normalizedPercent = clampPercent(percent)

  return (
    <div className='rounded-xl border border-sky-500/15 bg-sky-500/5 p-3'>
      <div className='flex items-center justify-between gap-3 text-xs'>
        <span className='text-muted-foreground font-medium'>{label}</span>
        <span className='font-semibold tabular-nums'>
          {formatUsageAmount(used)} / {formatUsageAmount(limit)}
        </span>
      </div>
      <Progress
        value={normalizedPercent}
        className={cn('mt-2 h-1.5', getProgressToneClass(normalizedPercent))}
      />
      <div className='text-muted-foreground mt-1 text-right text-[11px] tabular-nums'>
        {normalizedPercent.toFixed(0)}% used
      </div>
    </div>
  )
}
```

- [ ] **Step 5: Render progress and alert in the card**

Inside `ValuePackageCard`, after `displayPrice`, add:

```ts
  const usage = state?.subscription?.plan_id === plan.id ? state.usage : null
  const exhaustedMessage =
    usage?.exhausted_message ||
    '当前余额已用完，建议暂停使用，使用 API 或等时间跑完再使用'
```

After the existing 5-hour / 7-day limit grid, add:

```tsx
        {usage ? (
          <div className='grid grid-cols-1 gap-2'>
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
            />
            <LimitProgressRow
              label={t('7-day limit')}
              used={usage.used_7d}
              limit={usage.limit_7d}
              percent={usage.percent_7d}
            />
          </div>
        ) : null}

        {cardState.kind === 'running' && usage?.exhausted ? (
          <Alert variant='destructive'>
            <AlertTriangle className='size-4' />
            <AlertDescription>{exhaustedMessage}</AlertDescription>
          </Alert>
        ) : null}
```

- [ ] **Step 6: Run frontend card test**

Run:

```bash
cd web/default
bun test src/features/value-packages/components/value-package-card-source.test.ts
```

Expected: PASS.

- [ ] **Step 7: Commit Task 4**

```bash
git add web/default/src/features/value-packages/types.ts web/default/src/features/value-packages/components/value-package-card.tsx web/default/src/features/value-packages/components/value-package-card-source.test.ts
git commit -m "feat: show value package usage progress"
```

---

### Task 5: Blue package glow and sidebar value package pulse

**Files:**
- Modify: `web/default/src/styles/index.css`
- Modify: `web/default/src/components/layout/types.ts`
- Modify: `web/default/src/hooks/sidebar-data-model.ts`
- Modify: `web/default/src/hooks/sidebar-data-model.test.ts`
- Modify: `web/default/src/components/layout/components/nav-group.tsx`
- Create: `web/default/src/components/layout/components/nav-group-source.test.ts`
- Modify: `web/default/src/features/value-packages/components/authenticated-benefit-effects-source.test.ts`

- [ ] **Step 1: Write failing sidebar data tests**

In `web/default/src/hooks/sidebar-data-model.test.ts`, add this helper after `const t = ...`:

```ts
function findValuePackageItem(role: number) {
  const items = buildSidebarData(t, role).navGroups.flatMap((group) => group.items)
  const item = items.find((entry) => 'url' in entry && entry.url === '/value-packages')
  assert.ok(item)
  return item
}
```

Add these tests before the first existing test:

```ts
test('ordinary value package sidebar entry is attention marked', () => {
  const item = findValuePackageItem(1)

  assert.equal('attention' in item ? item.attention : undefined, 'value-packages')
})

test('admin value package sidebar entry is attention marked', () => {
  const item = findValuePackageItem(10)

  assert.equal('attention' in item ? item.attention : undefined, 'value-packages')
})
```

- [ ] **Step 2: Create failing nav group source test**

Create `web/default/src/components/layout/components/nav-group-source.test.ts`:

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
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const sourcePath = new URL('./nav-group.tsx', import.meta.url)

test('nav group applies value package pulse attention class', async () => {
  const source = await readFile(sourcePath, 'utf8')

  assert.match(source, /yunbay-sidebar-value-package-pulse/)
  assert.match(source, /item\.attention === 'value-packages'/)
})
```

- [ ] **Step 3: Extend benefit effects source test for package modifier**

In `web/default/src/features/value-packages/components/authenticated-benefit-effects-source.test.ts`, add this assertion inside the first test:

```ts
  assert.match(source, /yunbay-viewport-benefit-glow--package/)
```

- [ ] **Step 4: Run frontend tests and verify they fail**

Run:

```bash
cd web/default
bun test \
  src/hooks/sidebar-data-model.test.ts \
  src/components/layout/components/nav-group-source.test.ts \
  src/features/value-packages/components/authenticated-benefit-effects-source.test.ts
```

Expected: sidebar-data and nav-group source tests fail because attention markers and pulse class are absent.

- [ ] **Step 5: Add attention type**

In `web/default/src/components/layout/types.ts`, add to `BaseNavItem`:

```ts
  attention?: 'value-packages'
```

- [ ] **Step 6: Mark value package sidebar entries**

In `web/default/src/hooks/sidebar-data-model.ts`, update both `/value-packages` entries.

Ordinary user entry becomes:

```ts
            {
              title: t('Value Packages'),
              url: '/value-packages',
              icon: icons.valuePackages,
              attention: 'value-packages',
            },
```

Admin entry in the `personal` group becomes:

```ts
          {
            title: t('Value Packages'),
            url: '/value-packages',
            icon: icons.valuePackages,
            attention: 'value-packages',
          },
```

- [ ] **Step 7: Apply pulse class in nav group**

In `web/default/src/components/layout/components/nav-group.tsx`, change `SidebarMenuButton` inside `SidebarMenuLink` to:

```tsx
      <SidebarMenuButton
        className={cn(
          item.attention === 'value-packages' &&
            'yunbay-sidebar-value-package-pulse'
        )}
        isActive={checkIsActive(href, item)}
        tooltip={item.title}
        render={<Link to={item.url} onClick={() => setOpenMobile(false)} />}
      >
```

Also add this import near the existing imports:

```ts
import { cn } from '@/lib/utils'
```

- [ ] **Step 8: Change package glow to blue and add pulse CSS**

In `web/default/src/styles/index.css`, replace `.yunbay-viewport-benefit-glow--package` with:

```css
.yunbay-viewport-benefit-glow--package {
  --yunbay-benefit-glow-color: oklch(0.76 0.14 235 / 0.64);
  --yunbay-benefit-glow-soft-color: oklch(0.82 0.1 235 / 0.22);
}
```

Add this CSS after `.yunbay-viewport-benefit-glow--vip`:

```css
.yunbay-sidebar-value-package-pulse {
  position: relative;
  overflow: hidden;
  box-shadow: 0 0 0 0 oklch(0.74 0.14 235 / 0.32);
}

.yunbay-sidebar-value-package-pulse::after {
  content: '';
  position: absolute;
  inset: 2px;
  pointer-events: none;
  border-radius: inherit;
  background: linear-gradient(
    90deg,
    transparent,
    oklch(0.8 0.13 235 / 0.22),
    transparent
  );
  opacity: 0;
  transform: translateX(-48%);
}

@keyframes yunbay-sidebar-value-package-pulse {
  0%,
  100% {
    box-shadow: 0 0 0 0 oklch(0.74 0.14 235 / 0.18);
  }
  50% {
    box-shadow: 0 0 0 4px oklch(0.74 0.14 235 / 0.1);
  }
}

@keyframes yunbay-sidebar-value-package-sheen {
  0% {
    opacity: 0;
    transform: translateX(-48%);
  }
  42% {
    opacity: 1;
  }
  100% {
    opacity: 0;
    transform: translateX(48%);
  }
}

@media (prefers-reduced-motion: no-preference) {
  .yunbay-sidebar-value-package-pulse {
    animation: yunbay-sidebar-value-package-pulse 2.4s ease-in-out infinite;
  }

  .yunbay-sidebar-value-package-pulse::after {
    animation: yunbay-sidebar-value-package-sheen 2.8s ease-in-out infinite;
  }
}

@media (prefers-reduced-motion: reduce) {
  .yunbay-sidebar-value-package-pulse,
  .yunbay-sidebar-value-package-pulse::after {
    animation: none !important;
  }
}
```

- [ ] **Step 9: Run frontend tests for visual changes**

Run:

```bash
cd web/default
bun test \
  src/hooks/sidebar-data-model.test.ts \
  src/components/layout/components/nav-group-source.test.ts \
  src/features/value-packages/components/authenticated-benefit-effects-source.test.ts
```

Expected: selected frontend tests pass.

- [ ] **Step 10: Commit Task 5**

```bash
git add \
  web/default/src/styles/index.css \
  web/default/src/components/layout/types.ts \
  web/default/src/hooks/sidebar-data-model.ts \
  web/default/src/hooks/sidebar-data-model.test.ts \
  web/default/src/components/layout/components/nav-group.tsx \
  web/default/src/components/layout/components/nav-group-source.test.ts \
  web/default/src/features/value-packages/components/authenticated-benefit-effects-source.test.ts
git commit -m "feat: highlight value package entry and blue package glow"
```

---

### Task 6: i18n sync, typecheck, and full regression verification

**Files:**
- Modify locale files only if `bun run i18n:sync` reports missing keys.

- [ ] **Step 1: Run frontend i18n sync**

Run:

```bash
cd web/default
bun run i18n:sync
cat src/i18n/locales/_reports/_sync-report.json
```

Expected: `_sync-report.json` shows `missingCount: 0` for every locale. If new keys were added, the sync script updates locale files; inspect the report and keep all six locales complete.

- [ ] **Step 2: Run focused frontend tests**

Run:

```bash
cd web/default
bun test \
  src/hooks/sidebar-data-model.test.ts \
  src/components/layout/components/nav-group-source.test.ts \
  src/features/value-packages/lib/benefit-effects.test.ts \
  src/features/value-packages/components/value-package-card-source.test.ts \
  src/features/value-packages/components/authenticated-benefit-effects-source.test.ts
```

Expected: all selected frontend tests pass with 0 failures.

- [ ] **Step 3: Run frontend typecheck**

Run:

```bash
cd web/default
bun run typecheck
```

Expected: command exits 0.

- [ ] **Step 4: Run backend regression tests**

Run:

```bash
go test ./model ./service ./controller ./middleware -count=1
```

Expected: all selected backend packages pass.

- [ ] **Step 5: Check protected sub2api files are untouched**

Run:

```bash
git status --short -- \
  infra/sub2api/frontend/pnpm-lock.yaml \
  infra/sub2api/frontend/package.json \
  infra/sub2api/backend/go.mod
```

Expected: no output.

- [ ] **Step 6: Run whitespace diff check**

Run:

```bash
git diff --check
```

Expected: no output.

- [ ] **Step 7: Commit final verification artifacts if locale files changed**

If `bun run i18n:sync` changed locale files, run:

```bash
git add web/default/src/i18n/locales
git commit -m "chore: sync value package translations"
```

If there are no locale changes, do not create an empty commit.

- [ ] **Step 8: Final status summary**

Run:

```bash
git status --short --branch
git log --oneline --decorate -6
```

Expected: branch is ahead of `origin/main` by the spec commit plus implementation commits, with no unstaged changes.
