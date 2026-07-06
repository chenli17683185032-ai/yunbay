# Value Package OpenAI-Style Reset Time — Unfinished Task Plan / Handoff

Date: 2026-07-07
Branch: `codex/value-package-reset-time`
Worktree: `/Users/ethan/Documents/yunbay/.worktrees/value-package-reset-time`
Base main commit when paused: `5bf5a087 docs: plan value package reset time implementation`
Current paused HEAD: `b0377dc63818dc8a9d7588fddb03ade6fa2e4e1f`
Status: **Paused by user request; implementation incomplete; do not deploy this branch as final production work.**

## 1. User request that caused pause

User requested:

> 暂停任务吧。把当前的任务进展和后续还需要做的任务，写成一份详细的“任务未完成计划”，然后把这个未完成的分支也上传到 GitHub 上去。

Action taken:

- Stopped further implementation after Task 1 model work.
- Closed active subagent review before it completed.
- Did not start Task 2 middleware work.
- Did not merge to local `main`.
- Did not deploy to server.
- This handoff document records exact state and remaining work.

## 2. Important project constraints to preserve

- Reply/work context is 云贝 project.
- Do not expose server secrets, SSH private keys, tokens, cookies, `.env`, or DSNs.
- Server key location from project instructions, if/when deployment resumes:
  - `/Users/ethan/Desktop/云贝`
- Frontend package manager: Bun.
- Backend DB compatibility: SQLite, MySQL, PostgreSQL.
- JSON marshal/unmarshal in Go business code must use `common/json.go` wrappers; this paused work did not add JSON marshal calls.
- Do not modify/delete protected project identifiers from `AGENTS.md`.
- Do not overwrite LDXP discount fix:
  - `50 -> 47.5`
  - `100 -> 90`
  - `500 -> 425`
- User mentioned “50、100、200”; current verified previous fix is actually `50/100/500`. Treat `200` as needing separate clarification, not as permission to replace `500=425`.

## 3. Specs and plans already committed before implementation

### Spec

`/Users/ethan/Documents/yunbay/docs/superpowers/specs/2026-07-07-value-package-openai-style-reset-time-design.md`

Commit:

```text
6b8aea77 docs: specify value package reset time display
```

Spec intent:

- Keep rolling-window enforcement.
- Do not switch to fixed reset cycles.
- Show OpenAI/Codex-style full restore time:
  - `reset_at_5h = latest current-window usage.created_at + 5h`
  - `reset_at_7d = latest current-window usage.created_at + 7d`
- Do not display partial next-recovery as primary UI.
- Do not change model groups, billing multipliers, default activation, redemption codes, rebates, order deletion, or LDXP flow.

### Implementation plan

`/Users/ethan/Documents/yunbay/docs/superpowers/plans/2026-07-07-value-package-openai-style-reset-time.md`

Commit:

```text
5bf5a087 docs: plan value package reset time implementation
```

Planned tasks:

1. Backend model reset fields and helper.
2. Middleware quota-limit restore messages.
3. Controller response coverage.
4. Frontend reset-time formatter.
5. User value-package card reset display.
6. Admin order-management realtime table reset display.
7. Frontend i18n keys.
8. LDXP guard and focused verification.
9. Final integration, push, and deployment handoff.

## 4. Work completed before pause

### Completed Task 1: Backend model reset fields and helper

Commits on paused branch:

```text
9dcbafb45fbd feat: add value package reset usage fields
b0377dc63818 fix: ignore zero value package usage for reset time
```

Files changed:

- `model/subscription.go`
- `model/value_package_test.go`

#### 4.1 Backend model fields added

`ValuePackageUsageSummary` now includes:

```go
ResetAt5h      int64 `json:"reset_at_5h"`
ResetSeconds5h int64 `json:"reset_seconds_5h"`
Limited5h      bool  `json:"limited_5h"`
ResetAt7d      int64 `json:"reset_at_7d"`
ResetSeconds7d int64 `json:"reset_seconds_7d"`
Limited7d      bool  `json:"limited_7d"`
```

#### 4.2 Detailed rolling-window helper added

`model/subscription.go` now has:

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

func GetValuePackageWindowUsageDetails(userId int, userSubscriptionId int, now int64) (*ValuePackageWindowUsageDetails, error)
```

Old helper compatibility preserved:

```go
func GetValuePackageWindowUsage(userId int, userSubscriptionId int, now int64) (int64, int64, error)
func getValuePackageWindowUsageTx(tx *gorm.DB, userId int, userSubscriptionId int, now int64) (int64, int64, error)
```

#### 4.3 Reset calculation behavior

Current implementation computes reset from positive usage records only:

```text
reset_at = latest positive current-window usage.created_at + windowSeconds
reset_seconds = max(0, reset_at - now)
```

Reason for positive-only filtering:

- `ValuePackageUsageRecord.Quota` allows zero.
- Refund/revoke reservation paths can update usage quota to `0`.
- A newer `quota=0` record should not delay the visible “fully restored” time because it contributes no usage.
- The query therefore filters `quota > 0` while keeping the plan-required select expression:

```sql
COALESCE(SUM(quota), 0) AS used,
COALESCE(MAX(created_at), 0) AS latest_created_at
```

Since quota is non-negative, filtering `quota=0` does not change `SUM(quota)`, only fixes `MAX(created_at)`.

#### 4.4 Summary behavior

`buildValuePackageUsageSummaryTx` now uses detailed helper and computes:

```go
limited5h := plan.Limit5hAmount > 0 && usageDetails.Used5h >= plan.Limit5hAmount
limited7d := plan.Limit7dAmount > 0 && usageDetails.Used7d >= plan.Limit7dAmount
```

Summary reset fields are filled only when corresponding limit is enabled:

```text
limit_5h > 0 -> show reset_at_5h/reset_seconds_5h
limit_7d > 0 -> show reset_at_7d/reset_seconds_7d
limit <= 0   -> reset fields stay 0
```

`ExhaustedReason` now uses `limited5h` / `limited7d` for the rolling-window cases.

#### 4.5 Tests added/updated

`model/value_package_test.go` now covers:

- `TestValuePackageRollingUsageWindows`
  - old `GetValuePackageWindowUsage` compatibility.
  - detailed helper usage and reset for 5h/7d.
- `TestValuePackageWindowUsageDetailsIgnoresZeroQuotaForReset`
  - newer zero-quota record does not delay reset time.
- `TestActivateValuePackageReturnsUsageSummary`
  - summary returns reset fields and limited flags.
- `TestGetValuePackageStateIncludesUsageSummary`
  - state summary returns reset fields and limited flags.
- `TestValuePackageUsageSummaryResetFieldsForEmptyAndUnlimitedWindows`
  - unlimited + empty window reset stays zero and limited flags false.
- Existing `TestListActiveValuePackageUsageRowsReturnsRealtimeWindowUsage` was included in verification.

## 5. Review status at pause

### Completed review before fix

Spec review passed for first Task 1 commit `9dcbafb45fbd`.

Code-quality review found one Important issue:

- `quota=0` usage record could incorrectly delay reset time.

This was fixed in:

```text
b0377dc63818 fix: ignore zero value package usage for reset time
```

### Review interrupted by user pause

After `b0377dc63818`, a second spec review was started for final range `5bf5a087..b0377dc63818`, but the user requested pause before the reviewer finished. The active reviewer was shut down.

Therefore, when resuming, redo both reviews for Task 1 final range:

```bash
git diff --stat 5bf5a087..b0377dc63818
git diff 5bf5a087..b0377dc63818 -- model/subscription.go model/value_package_test.go
```

Recommended review checklist:

- Only `model/subscription.go` and `model/value_package_test.go` changed for Task 1.
- Public helper signature matches plan.
- Old helper signatures still compile.
- SQL remains cross-DB compatible.
- `quota > 0` filtering is intentional and tested.
- Summary reset fields are suppressed when corresponding limit <= 0.
- Tests include zero-quota reset regression.

## 6. Verification already run before pause

### Baseline before implementation

From worktree `/Users/ethan/Documents/yunbay/.worktrees/value-package-reset-time`:

```bash
go test ./model ./middleware ./controller -run 'ValuePackage|OrderManagement' -count=1 -timeout=300s
```

Result:

```text
ok github.com/QuantumNous/new-api/model
ok github.com/QuantumNous/new-api/middleware
ok github.com/QuantumNous/new-api/controller
```

LDXP guard:

```bash
go test ./service -run 'TestLoadLdxpConfigDefaultProductsUseRealSixLinks|TestVerifyLdxpWorkerPaidFieldsAllowsCardNetworkFee' -count=1 -timeout=300s
rg -n '"amount":50,"money":47\.5|"amount":100,"money":90|"amount":500,"money":425' service/ldxp_config.go
```

Result included:

```text
ok github.com/QuantumNous/new-api/service
service/ldxp_config.go:42  {"amount":50,"money":47.5,...}
service/ldxp_config.go:43  {"amount":100,"money":90,...}
service/ldxp_config.go:44  {"amount":500,"money":425,...}
```

Frontend source-test baseline:

```bash
cd web/default
bun test \
  src/features/value-packages/components/value-package-card-source.test.ts \
  src/features/order-management/components/value-package-usage-table-source.test.ts \
  src/features/order-management/order-management-source.test.ts
```

Result:

```text
5 pass
0 fail
```

### Task 1 verification after zero-quota fix

```bash
go test ./model -run 'TestValuePackageWindowUsageDetailsIgnoresZeroQuotaForReset|TestValuePackageRollingUsageWindows|TestActivateValuePackageReturnsUsageSummary|TestGetValuePackageStateIncludesUsageSummary|TestValuePackageUsageSummaryResetFieldsForEmptyAndUnlimitedWindows|TestListActiveValuePackageUsageRowsReturnsRealtimeWindowUsage' -count=1 -timeout=300s
```

Result:

```text
ok github.com/QuantumNous/new-api/model
```

Worker also reported:

```bash
go test ./model -run 'ValuePackage' -count=1 -timeout=300s
```

Result:

```text
ok github.com/QuantumNous/new-api/model
```

## 7. Remaining unfinished tasks

### Task 1 still needs final review gate

Before continuing, redo Task 1 final review for range:

```text
5bf5a087..b0377dc63818
```

Required:

1. Spec compliance review.
2. Code quality review.
3. Fix any Critical/Important issue before Task 2.

Do not assume the interrupted review passed.

### Task 2: Middleware quota-limit restore messages

Files to modify:

- `middleware/value_package.go`
- `middleware/value_package_test.go`

Implementation intent:

- Replace middleware use of `model.GetValuePackageWindowUsage(...)` with `model.GetValuePackageWindowUsageDetails(...)`.
- Add reset duration formatting helper for Chinese API error messages.
- On 5h limit:

```text
超值套餐额度已用完（5 小时：已用 X / 限额 Y，将在 Z 后恢复）
```

- On 7d limit:

```text
超值套餐额度已用完（7 天：已用 X / 限额 Y，将在 Z 后恢复）
```

- If `ResetSeconds* <= 0`, downgrade to existing old message without restore time or use “不到 1 分钟” consistently.
- Preserve model/distributor group behavior. Do not reintroduce day-card/week-card/month-card model group routing bugs.

Tests to update/add:

- `TestValuePackageMiddlewareRejectsOverRollingWindows`
  - assert body includes `将在` and `后恢复` for 5h and 7d cases.
- Keep/pass routing regression tests:
  - `TestValuePackagePlaygroundDistributeKeepsRequestedModelGroupWhenPackageActive`
  - `TestValuePackageRelayDistributeKeepsTokenModelGroupWhenPackageActive`
  - `TestValuePackageScopeDoesNotOverwriteRoutingOrUserGroups`

Suggested verification:

```bash
go test ./middleware -run 'TestValuePackageMiddlewareRejectsOverRollingWindows|TestValuePackageRealtimeRejectsOverRollingWindows|TestValuePackagePlaygroundDistributeKeepsRequestedModelGroupWhenPackageActive|TestValuePackageRelayDistributeKeepsTokenModelGroupWhenPackageActive|TestValuePackageScopeDoesNotOverwriteRoutingOrUserGroups' -count=1 -timeout=300s
```

Suggested commit:

```text
feat: show value package reset in limit errors
```

### Task 3: Controller response coverage

Files to modify:

- `controller/value_package_test.go`
- `controller/order_management_test.go`

Likely no controller code needed because `Usage` struct already carries new JSON fields.

Tests:

- `TestGetValuePackageSelfReturnsCurrentState`
  - response includes `reset_at_5h`, `reset_seconds_5h`, `limited_5h`, `reset_at_7d`, `reset_seconds_7d`, `limited_7d`.
- `TestActivateAndDeactivateValuePackageAPI`
  - activation response includes the same fields.
- `TestAdminOrderManagementValuePackageUsageReturnsActiveUsers`
  - admin realtime usage response includes the same fields.

Verification:

```bash
go test ./controller -run 'TestGetValuePackageSelfReturnsCurrentState|TestActivateAndDeactivateValuePackageAPI|TestAdminOrderManagementValuePackageUsageReturnsActiveUsers' -count=1 -timeout=300s
```

Suggested commit:

```text
test: cover value package reset API fields
```

### Task 4: Frontend reset-time formatter

Files to create:

- `web/default/src/features/value-packages/lib/reset-time.ts`
- `web/default/src/features/value-packages/lib/reset-time.test.ts`

Functions:

```ts
formatValuePackageResetTime(seconds, t): string
formatValuePackageResetLine({ limit, resetSeconds, limited, t }): string
```

Expected behavior:

- `limit <= 0` -> `Unlimited`
- `resetSeconds <= 0` -> `Fully restored`
- `<60s` -> `less than 1 minute`
- minutes/hours/days formatting.
- limited -> `Limit reached · Resets in {{time}}`
- not limited -> `Resets in {{time}}`

Verification:

```bash
cd web/default
bun test src/features/value-packages/lib/reset-time.test.ts
```

Suggested commit:

```text
feat: add value package reset time formatter
```

### Task 5: User value-package card reset display

Files to modify:

- `web/default/src/features/value-packages/types.ts`
- `web/default/src/features/value-packages/components/value-package-card.tsx`
- `web/default/src/features/value-packages/components/value-package-card-source.test.ts`

Implementation:

- Add new fields to `ValuePackageUsageSummary` TypeScript interface.
- Import shared formatter.
- Update `LimitProgressRow` to accept:

```ts
resetSeconds?: number
limited?: boolean
showReset?: boolean
```

- Show reset text under only 5h/7d rows, not total limit row.

Verification:

```bash
cd web/default
bun test src/features/value-packages/components/value-package-card-source.test.ts
bun run typecheck
```

Suggested commit:

```text
feat: show value package reset on user cards
```

### Task 6: Admin order-management realtime table reset display

Files to modify:

- `web/default/src/features/order-management/types.ts`
- `web/default/src/features/order-management/components/value-package-usage-table.tsx`
- `web/default/src/features/order-management/components/value-package-usage-table-source.test.ts`

Implementation:

- Add new fields to `OrderManagementValuePackageUsageSummary`.
- Import shared formatter from `@/features/value-packages/lib/reset-time`.
- Add reset line below each 5h/7d admin usage bar.
- Keep `Auto-refresh every 15 seconds` behavior and existing `refetchInterval: 15_000` logic unchanged.

Verification:

```bash
cd web/default
bun test src/features/order-management/components/value-package-usage-table-source.test.ts
bun run typecheck
```

Suggested commit:

```text
feat: show value package reset in admin usage table
```

### Task 7: i18n six-language sync

Files to modify:

- `web/default/src/i18n/locales/en.json`
- `web/default/src/i18n/locales/zh.json`
- `web/default/src/i18n/locales/fr.json`
- `web/default/src/i18n/locales/ja.json`
- `web/default/src/i18n/locales/ru.json`
- `web/default/src/i18n/locales/vi.json`

Keys from implementation plan:

```json
"Fully restored"
"less than 1 minute"
"{{count}} minutes"
"{{count}} hour"
"{{hours}} hours {{minutes}} minutes"
"{{count}} day"
"{{days}} days {{hours}} hours"
"Resets in {{time}}"
"Limit reached · {{reset}}"
```

Verification:

```bash
cd web/default
bun run i18n:sync
bun test \
  src/features/value-packages/lib/reset-time.test.ts \
  src/features/value-packages/components/value-package-card-source.test.ts \
  src/features/order-management/components/value-package-usage-table-source.test.ts
```

Suggested commit:

```text
chore: translate value package reset labels
```

### Task 8: LDXP guard and focused verification

Must verify before final integration:

```bash
rg -n '"amount":50,"money":47\.5|"amount":100,"money":90|"amount":500,"money":425' service/ldxp_config.go

go test ./service -run 'TestLoadLdxpConfigDefaultProductsUseRealSixLinks|TestVerifyLdxpWorkerPaidFieldsAllowsCardNetworkFee' -count=1 -timeout=300s

go test ./model ./middleware ./controller -run 'ValuePackage|OrderManagement' -count=1 -timeout=300s

cd web/default
bun run typecheck
bun run build
```

Do not modify `service/ldxp_config.go` unless the guard fails and the only change is restoring:

```text
50=47.5, 100=90, 500=425
```

### Task 9: Final integration / push / deploy

This paused branch is currently not merged to `main`.

When implementation is complete and verified:

1. Push feature branch or merge into local `main` per user instruction at that time.
2. If merging to `main`, run final tests after merge.
3. Push `main` only after all tests pass.
4. Deploy only if user explicitly resumes deployment or confirms deploy.

Final verification should include:

```bash
go test ./middleware ./service ./model ./controller -count=1 -timeout=300s
cd web/default
bun run typecheck
bun run build
```

Post-deploy smoke requirements:

- User value-package state endpoint returns six new reset/limited fields.
- Admin realtime usage endpoint returns six new reset/limited fields.
- Frontend user package cards show reset text under 5h/7d usage bars.
- Admin table shows reset text under 5h/7d usage bars.
- Package users still route through Plus/Pro distributor groups, not day/week/month-card model groups.
- LDXP config still maps `50=47.5`, `100=90`, `500=425`.

## 8. Current branch commit list

As of pause, branch `codex/value-package-reset-time` contains:

```text
b0377dc6 fix: ignore zero value package usage for reset time
9dcbafb4 feat: add value package reset usage fields
5bf5a087 docs: plan value package reset time implementation
6b8aea77 docs: specify value package reset time display
89d67f43 fix: align ldxp discounted topup amounts
```

`origin/main` was behind local documentation and implementation commits when work began. Verify again before resuming:

```bash
git fetch origin
git status --short --branch
git log --oneline --decorate --graph --max-count=12 --all
```

## 9. Resume instructions

Recommended resume sequence:

1. Checkout worktree branch:

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/value-package-reset-time
git status --short --branch
```

2. Confirm it is clean.
3. Re-run Task 1 verification:

```bash
go test ./model -run 'TestValuePackageWindowUsageDetailsIgnoresZeroQuotaForReset|TestValuePackageRollingUsageWindows|TestActivateValuePackageReturnsUsageSummary|TestGetValuePackageStateIncludesUsageSummary|TestValuePackageUsageSummaryResetFieldsForEmptyAndUnlimitedWindows|TestListActiveValuePackageUsageRowsReturnsRealtimeWindowUsage' -count=1 -timeout=300s
```

4. Redo Task 1 final spec/code review for range `5bf5a087..b0377dc63818`.
5. Continue from Task 2 in `docs/superpowers/plans/2026-07-07-value-package-openai-style-reset-time.md`.
6. Keep committing one task at a time.
7. Do not deploy until user explicitly says to resume deployment.
