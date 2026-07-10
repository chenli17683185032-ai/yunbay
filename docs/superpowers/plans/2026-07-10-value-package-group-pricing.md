# Value Package Group Pricing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make explicitly configured package-group pricing effective across all billing paths while keeping ordinary subscriptions at 1x and making the admin save flow truthful.

**Architecture:** Reuse the special ratio already resolved by `relay/helper/price.go` for `ValuePackageBillingGroup × UsingGroup`; the subscription override selects that frozen special value only for value packages and otherwise selects 1x. A root-only GET/PUT endpoint validates and atomically persists the two coupled ratio maps, while the frontend uses one final success/failure outcome and exposes plan-derived package groups.

**Tech Stack:** Go, GORM, Gin, existing `ratio_setting`, billing snapshots, React Query, React Hook Form, Zod, Bun.

---

## File map

**Create**

- `controller/option_group_ratio.go` — root-only coupled ratio GET/PUT handlers.
- `controller/option_group_ratio_test.go` — validation, transaction, response-snapshot tests.
- `web/default/src/features/system-settings/models/group-ratio-save.test.ts` — business-error and normalized-baseline regressions.

**Modify**

- `types/price_data.go` — persist the resolved subscription-ratio source.
- `model/task.go` — persist the source through asynchronous billing snapshots.
- `service/billing_ratio.go` — resolve configured value-package ratios instead of unconditional 1x.
- `service/billing_ratio_test.go` — model-ratio and tiered-expression matrix.
- `service/billing_session_test.go` — live value-package reserve/settle coverage.
- `service/log_info_generate.go` — text/realtime log source field.
- `service/task_billing.go` — task log source field.
- `service/task_billing_test.go` — task snapshot regression.
- `controller/relay.go` — copy source into `TaskBillingContext`.
- `setting/ratio_setting/group_ratio.go` — nested-map validation.
- `setting/ratio_setting/group_ratio_test.go` — finite/positive nested ratio tests.
- `model/subscription.go` — list distinct enabled value-package billing groups.
- `model/option_test.go` — `UpdateOptionsBulk` rollback behavior.
- `router/api-router.go` — register `/api/option/group-ratios`.
- `web/default/src/features/system-settings/api.ts` — specialized GET/PUT calls.
- `web/default/src/features/system-settings/types.ts` — coupled endpoint DTOs.
- `web/default/src/features/system-settings/models/ratio-settings-card.tsx` — one save outcome and baseline update.
- `web/default/src/features/system-settings/models/group-ratio-form.tsx` — pass known package groups.
- `web/default/src/features/system-settings/models/group-ratio-visual-editor.tsx` — show plan-derived package groups with “default 1x”.
- `web/default/src/i18n/locales/{en,zh,fr,ru,ja,vi}.json` — package-group guidance.

## Task 1: Resolve the correct subscription ratio once

**Files:**
- Modify: `types/price_data.go`
- Modify: `service/billing_ratio.go`
- Modify: `service/billing_ratio_test.go`

- [ ] **Step 1: Replace the old one-X test with a behavior matrix**

Add table cases around a new pure helper:

```go
func TestResolveSubscriptionBillingRatio(t *testing.T) {
	tests := []struct {
		name string
		info *relaycommon.RelayInfo
		wantRatio float64
		wantSource string
	}{
		{
			name: "ordinary subscription stays one x",
			info: &relaycommon.RelayInfo{BillingSource: BillingSourceSubscription, PriceData: types.PriceData{GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 0.3, HasSpecialRatio: true}}},
			wantRatio: 1, wantSource: SubscriptionRatioSourceRegularOneX,
		},
		{
			name: "value package uses explicit nested ratio",
			info: &relaycommon.RelayInfo{BillingSource: BillingSourceSubscription, ValuePackageSubscriptionId: 8, ValuePackageBillingGroup: "week-card", UsingGroup: "gpt-plus", PriceData: types.PriceData{GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 0.45, GroupSpecialRatio: 0.45, HasSpecialRatio: true}}},
			wantRatio: 0.45, wantSource: SubscriptionRatioSourceValuePackageConfigured,
		},
		{
			name: "value package without nested ratio defaults one x",
			info: &relaycommon.RelayInfo{BillingSource: BillingSourceSubscription, ValuePackageSubscriptionId: 8, ValuePackageBillingGroup: "week-card", UsingGroup: "gpt-plus", PriceData: types.PriceData{GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 0.3, GroupSpecialRatio: -1, HasSpecialRatio: false}}},
			wantRatio: 1, wantSource: SubscriptionRatioSourceValuePackageDefaultOneX,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ratio, source := resolveSubscriptionBillingRatio(tt.info)
			assert.Equal(t, tt.wantRatio, ratio)
			assert.Equal(t, tt.wantSource, source)
		})
	}
}
```

Add an invalid `NaN`, `Inf`, `0`, and negative defensive case expecting 1x/default source.

- [ ] **Step 2: Confirm RED**

```bash
go test ./service -run '^TestResolveSubscriptionBillingRatio$' -count=1
```

Expected: compile failure because the helper and source constants do not exist.

- [ ] **Step 3: Add source state and the resolver**

In `types/price_data.go`:

```go
SubscriptionRatioSource string
```

In `service/billing_ratio.go`:

```go
const (
	SubscriptionRatioSourceRegularOneX            = "regular_subscription_1x"
	SubscriptionRatioSourceValuePackageDefaultOneX = "default_1x"
	SubscriptionRatioSourceValuePackageConfigured  = "configured"
)

func resolveSubscriptionBillingRatio(info *relaycommon.RelayInfo) (float64, string) {
	if info == nil || info.ValuePackageSubscriptionId <= 0 || strings.TrimSpace(info.ValuePackageBillingGroup) == "" {
		return 1, SubscriptionRatioSourceRegularOneX
	}
	candidate := info.PriceData.GroupRatioInfo
	if info.PriceData.SubscriptionRatioApplied && info.PriceData.HasOriginalGroupRatioInfo {
		candidate = info.PriceData.OriginalGroupRatioInfo
	}
	if candidate.HasSpecialRatio && candidate.GroupRatio > 0 && !math.IsNaN(candidate.GroupRatio) && !math.IsInf(candidate.GroupRatio, 0) {
		return candidate.GroupRatio, SubscriptionRatioSourceValuePackageConfigured
	}
	return 1, SubscriptionRatioSourceValuePackageDefaultOneX
}
```

Change `subscriptionPreConsumeQuota` and `applySubscriptionBillingRatio` to accept the resolved ratio. `EnsureSubscriptionBillingRatio` resolves it before calculating quota, passes the same value to ratio-based and `tiered_expr` paths, and sets `PriceData.SubscriptionRatioSource`. `restoreOriginalBillingRatio` clears the applied source along with `SubscriptionRatioApplied=false`.

- [ ] **Step 4: Run the service matrix**

```bash
go test ./service -run 'BillingRatio|ValuePackageBilling|TieredSnapshot' -count=1
```

Expected: explicit 0.45 cases pre-consume 45% of group-before quota, missing cases consume 1x, and ordinary subscription tests remain 1x.

- [ ] **Step 5: Commit the resolver**

```bash
git add types/price_data.go service/billing_ratio.go service/billing_ratio_test.go service/billing_session_test.go
git commit -m "fix: apply configured value package group ratios"
```

## Task 2: Freeze and log the ratio source through every billing path

**Files:**
- Modify: `model/task.go`
- Modify: `controller/relay.go`
- Modify: `service/log_info_generate.go`
- Modify: `service/task_billing.go`
- Modify: `service/task_billing_test.go`
- Modify: `service/billing_ratio_test.go`

- [ ] **Step 1: Add failing log and task-snapshot assertions**

Extend text and task tests:

```go
assert.Equal(t, "configured", other["value_package_ratio_source"])
assert.Equal(t, 0.45, other["value_package_effective_ratio"])
```

Create a task with `SubscriptionRatioSource: "configured"`, serialize/deserialize `TaskPrivateData`, and assert the source and ratio survive.

- [ ] **Step 2: Confirm RED**

```bash
go test ./service ./controller -run 'RatioAudit|BillingContext|ValuePackage.*Log' -count=1
```

Expected: missing-field assertions fail.

- [ ] **Step 3: Add the source to async and log snapshots**

In `model.TaskBillingContext`:

```go
SubscriptionRatioSource string `json:"subscription_ratio_source,omitempty"`
```

In `controller/finalizeSuccessfulRelayTask` copy:

```go
SubscriptionRatioSource: relayInfo.PriceData.SubscriptionRatioSource,
```

In both text/realtime and task log builders:

```go
if priceData.SubscriptionRatioApplied && priceData.SubscriptionRatioSource != "" {
	other["value_package_ratio_source"] = priceData.SubscriptionRatioSource
}
```

and for tasks:

```go
if bc.SubscriptionRatioApplied && bc.SubscriptionRatioSource != "" {
	other["value_package_ratio_source"] = bc.SubscriptionRatioSource
}
```

Keep existing original-ratio audit fields; they explain what the wallet path would have used.

- [ ] **Step 4: Run log/task tests repeatedly**

```bash
go test ./service ./controller -run 'BillingRatio|TaskBilling|Generate.*OtherInfo' -count=3
```

Expected: PASS in all iterations.

- [ ] **Step 5: Commit snapshot observability**

```bash
git add model/task.go controller/relay.go service/log_info_generate.go service/task_billing.go service/task_billing_test.go service/billing_ratio_test.go
git commit -m "feat: audit package ratio source"
```

## Task 3: Validate nested ratios and expose package billing groups

**Files:**
- Modify: `setting/ratio_setting/group_ratio.go`
- Create: `setting/ratio_setting/group_ratio_test.go`
- Modify: `model/subscription.go`
- Modify: `model/value_package_test.go`

- [ ] **Step 1: Write failing nested-validation tests**

```go
func TestCheckGroupGroupRatio(t *testing.T) {
	require.NoError(t, CheckGroupGroupRatio(`{"week-card":{"gpt-plus":0.45}}`))
	for _, raw := range []string{
		`{"":{"gpt-plus":1}}`,
		`{"week-card":{"":1}}`,
		`{"week-card":{"gpt-plus":0}}`,
		`{"week-card":{"gpt-plus":-1}}`,
	} {
		require.Error(t, CheckGroupGroupRatio(raw), raw)
	}
}
```

Add a model test with enabled day/week/month plans, duplicate groups, a disabled plan, and a regular subscription. Expected sorted result contains only distinct enabled value-package `ModelGroup` values.

- [ ] **Step 2: Confirm RED**

```bash
go test ./setting/ratio_setting ./model -run 'GroupGroupRatio|ValuePackageBillingGroups' -count=1
```

- [ ] **Step 3: Implement validation with project JSON wrappers**

```go
func CheckGroupGroupRatio(jsonStr string) error {
	parsed := make(map[string]map[string]float64)
	if err := common.UnmarshalJsonStr(jsonStr, &parsed); err != nil { return err }
	for parent, children := range parsed {
		if strings.TrimSpace(parent) == "" { return errors.New("source group must not be empty") }
		for child, ratio := range children {
			if strings.TrimSpace(child) == "" { return errors.New("target group must not be empty") }
			if ratio <= 0 || math.IsNaN(ratio) || math.IsInf(ratio, 0) {
				return fmt.Errorf("inter-group ratio must be finite and greater than 0: %s -> %s", parent, child)
			}
		}
	}
	return nil
}
```

Do not tighten `CheckGroupRatio` from `>=0` to `>0`; the existing global group map intentionally supports free groups.

Add:

```go
func ListEnabledValuePackageBillingGroups() ([]string, error) {
	var groups []string
	err := DB.Model(&SubscriptionPlan{}).
		Where("enabled = ? AND plan_kind = ? AND model_group <> ?", true, SubscriptionPlanKindValuePackage, "").
		Distinct().Pluck("model_group", &groups).Error
	sort.Strings(groups)
	return groups, err
}
```

- [ ] **Step 4: Run focused tests**

```bash
go test ./setting/ratio_setting ./model -run 'GroupGroupRatio|ValuePackageBillingGroups' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit validation and group discovery**

```bash
git add setting/ratio_setting/group_ratio.go setting/ratio_setting/group_ratio_test.go model/subscription.go model/value_package_test.go
git commit -m "feat: validate package group ratio overrides"
```

## Task 4: Add an atomic root-only ratio pair endpoint

**Files:**
- Create: `controller/option_group_ratio.go`
- Create: `controller/option_group_ratio_test.go`
- Modify: `router/api-router.go`
- Create: `model/option_test.go`

- [ ] **Step 1: Write failing controller tests**

Cover:

```go
type groupRatioOptionsResponse struct {
	Success bool `json:"success"`
	Data struct {
		GroupRatio string `json:"group_ratio"`
		GroupGroupRatio string `json:"group_group_ratio"`
		PackageGroups []string `json:"package_groups"`
	} `json:"data"`
}
```

Assertions:

- GET returns current normalized maps and `day-card/week-card/month-card` plan groups.
- PUT with valid maps updates both DB options and both runtime maps.
- invalid nested ratio leaves both DB rows unchanged.
- a forced failure during the second option write rolls back the first.
- no option values appear in audit metadata.

- [ ] **Step 2: Confirm RED**

```bash
go test ./controller ./model -run 'GroupRatioOptions|UpdateOptionsBulkRollback' -count=1
```

- [ ] **Step 3: Implement GET/PUT handlers**

Use DTOs with strings, not `any`:

```go
type groupRatioOptionsUpdateRequest struct {
	GroupRatio      string `json:"group_ratio" binding:"required"`
	GroupGroupRatio string `json:"group_group_ratio" binding:"required"`
}

type groupRatioOptionsSnapshot struct {
	GroupRatio      string   `json:"group_ratio"`
	GroupGroupRatio string   `json:"group_group_ratio"`
	PackageGroups   []string `json:"package_groups"`
}
```

PUT processing order:

```go
func buildGroupRatioOptionsSnapshot() (*groupRatioOptionsSnapshot, error) {
	groups, err := model.ListEnabledValuePackageBillingGroups()
	if err != nil {
		return nil, err
	}
	return &groupRatioOptionsSnapshot{
		GroupRatio:      ratio_setting.GroupRatio2JSONString(),
		GroupGroupRatio: ratio_setting.GroupGroupRatio2JSONString(),
		PackageGroups:   groups,
	}, nil
}

func GetGroupRatioOptions(c *gin.Context) {
	snapshot, err := buildGroupRatioOptionsSnapshot()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "读取分组倍率失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": snapshot})
}

func UpdateGroupRatioOptions(c *gin.Context) {
	var req groupRatioOptionsUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的参数"})
		return
	}
	if strings.TrimSpace(req.GroupRatio) == "" || strings.TrimSpace(req.GroupGroupRatio) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "分组倍率配置不能为空"})
		return
	}
	if err := ratio_setting.CheckGroupRatio(req.GroupRatio); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := ratio_setting.CheckGroupGroupRatio(req.GroupGroupRatio); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := model.UpdateOptionsBulk(map[string]string{
		"GroupRatio":      req.GroupRatio,
		"GroupGroupRatio": req.GroupGroupRatio,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "保存分组倍率失败"})
		return
	}
	snapshot, err := buildGroupRatioOptionsSnapshot()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "读取已保存分组倍率失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": snapshot})
}
```

The response snapshot uses `ratio_setting.GroupRatio2JSONString()` and `GroupGroupRatio2JSONString()` after commit. Register:

```go
optionRoute.GET("/group-ratios", controller.GetGroupRatioOptions)
optionRoute.PUT("/group-ratios", controller.UpdateGroupRatioOptions)
```

The containing route already uses `RootAuth`; do not weaken it to `AdminAuth`.

- [ ] **Step 4: Run endpoint and option tests**

```bash
go test ./controller ./model -run 'GroupRatioOptions|UpdateOptionsBulk' -count=1
```

Expected: PASS; database and runtime snapshots match.

- [ ] **Step 5: Commit the endpoint**

```bash
git add controller/option_group_ratio.go controller/option_group_ratio_test.go router/api-router.go model/option_test.go
git commit -m "feat: save group ratio pair atomically"
```

## Task 5: Make the admin UI use one truthful save outcome

**Files:**
- Modify: `web/default/src/features/system-settings/api.ts`
- Modify: `web/default/src/features/system-settings/types.ts`
- Modify: `web/default/src/features/system-settings/models/ratio-settings-card.tsx`
- Modify: `web/default/src/features/system-settings/models/group-ratio-form.tsx`
- Modify: `web/default/src/features/system-settings/models/group-ratio-visual-editor.tsx`
- Create: `web/default/src/features/system-settings/models/group-ratio-save.test.ts`

- [ ] **Step 1: Write failing pure save-contract tests**

Extract and test a helper:

```ts
export function requireSuccessfulOptionResponse<T extends { success: boolean; message?: string }>(response: T): T {
  if (!response.success) throw new Error(response.message || 'Failed to update setting')
  return response
}
```

Test `success:false` throws even when transport status was 200, and normalized returned maps replace the old baseline only after the full save promise resolves.

- [ ] **Step 2: Confirm RED**

```bash
cd web/default
bun test src/features/system-settings/models/group-ratio-save.test.ts
```

- [ ] **Step 3: Add typed GET/PUT API methods**

```ts
export type GroupRatioOptionsSnapshot = {
  group_ratio: string
  group_group_ratio: string
  package_groups: string[]
}

export type GroupRatioOptionsResponse = {
  success: boolean
  message: string
  data?: GroupRatioOptionsSnapshot
}
```

```ts
export async function getGroupRatioOptions() {
  const res = await api.get<GroupRatioOptionsResponse>('/api/option/group-ratios')
  return requireSuccessfulOptionResponse(res.data)
}

export async function updateGroupRatioOptions(request: { group_ratio: string; group_group_ratio: string }) {
  const res = await api.put<GroupRatioOptionsResponse>('/api/option/group-ratios', request)
  return requireSuccessfulOptionResponse(res.data)
}
```

- [ ] **Step 4: Replace pairwise generic mutations in `saveGroupRatios`**

When either coupled field changed, call the specialized endpoint exactly once. For the remaining group options, call `updateSystemOption` directly and pass every response through `requireSuccessfulOptionResponse`; do not use `useUpdateOption` because it emits per-key success toasts.

Only after every request succeeds:

```ts
groupNormalizedDefaults.current = {
  ...normalized,
  GroupRatio: normalizeJsonString(server.data?.group_ratio ?? normalized.GroupRatio),
  GroupGroupRatio: normalizeJsonString(server.data?.group_group_ratio ?? normalized.GroupGroupRatio),
}
toast.success(t('Group ratios saved successfully'))
await queryClient.invalidateQueries({ queryKey: ['system-options'] })
```

On failure, keep form values and the old baseline, show one error toast, and rethrow so the form does not report success.

- [ ] **Step 5: Expose plan-derived package groups without persisting empty maps**

Fetch `/api/option/group-ratios` with React Query and pass `package_groups` through `GroupRatioForm` to `GroupRatioVisualEditor`.

In the visual editor, merge missing package groups into the displayed source-group list as empty override cards:

```ts
for (const packageGroup of packageGroups) {
  if (!Object.prototype.hasOwnProperty.call(map, packageGroup)) {
    rows.push({ userGroup: packageGroup, overrides: [], isPackageGroup: true })
  }
}
```

Show a `Package billing group` badge and `Default 1x until an override is added`. Do not serialize empty display-only parents; the first added override creates the nested map.

- [ ] **Step 6: Add six-locale UI copy and tests**

Add real translations for:

```text
Package billing group
Default 1x until an override is added
Group ratios saved successfully
Failed to save group ratios
```

Run `bun run i18n:sync` and the focused tests.

- [ ] **Step 7: Run frontend verification**

```bash
cd web/default
bun test src/features/system-settings/models/group-ratio-save.test.ts
bun run i18n:sync
bun run typecheck
bun run build
```

Expected: PASS; no mixed success/error toast path remains in `saveGroupRatios`.

- [ ] **Step 8: Commit the admin flow**

```bash
git add web/default/src/features/system-settings web/default/src/i18n/locales
git commit -m "fix: make group ratio saves consistent"
```

## Task 6: End-to-end billing verification

**Files:** all files in this plan.

- [ ] **Step 1: Run focused backend suites**

```bash
go test ./setting/ratio_setting ./model ./controller ./service ./relay/... -run 'GroupRatio|BillingRatio|ValuePackageBilling|TaskBilling|ResponsesCompact' -count=1
```

Expected: PASS.

- [ ] **Step 2: Run the tiered-expression regression suite**

```bash
go test ./pkg/billingexpr ./relay/helper ./service -run 'Tiered|BillingRatio|Settle' -count=1
```

Expected: PASS and group ratio is applied exactly once after expression evaluation.

- [ ] **Step 3: Run the full affected frontend checks**

```bash
cd web/default
bun test src/features/system-settings
bun run typecheck
bun run build
```

Expected: PASS.

- [ ] **Step 4: Inspect for the removed unconditional override**

```bash
rg -n 'const subscriptionBillingGroupRatio = 1\.0|GroupRatio: +subscriptionBillingGroupRatio' service/billing_ratio.go
```

Expected: no match. A literal default 1 is allowed only inside the explicit resolver branches with named source constants.
