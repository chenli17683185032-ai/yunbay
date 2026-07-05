# 云贝日/周/月卡稳定化与快速引导 API Key 复制兜底 Spec

## 背景

最近对云贝系统的日卡、周卡、月卡能力做了多次紧急修复，包括订单管理纳入日/周/月卡、兑换码入口、返利、管理员实时额度表、套餐默认启用、套餐不污染模型路由组、并发锁 TTL、日志和 API key 页面倍率显示等。

这些修复是在生产 bug 压力下临时追加的，没有经过完整 spec 和实施计划。当前 `main` 已经能通过部分定向测试，但源码审计发现仍有几个结构性风险：

1. 套餐计费身份仍然混入 `RelayInfo.UserGroup`，真实用户身份和计费身份没有分离。
2. Realtime WebSocket 套餐消耗可能只扣订阅额度，不写 5 小时 / 7 天窗口使用记录。
3. API key 页面仍可能在 `auto` key 场景不显示套餐 `1x`。
4. 前端部分套餐倍率展示由前端推断，不是后端 authoritative billing state。
5. 使用日志虽然能显示 `1x`，但没有清晰标识这是套餐计费覆盖后的 `1x`。
6. 套餐并发锁需要更强测试和可观测性。
7. 订单、删除订单、兑换码、返利、管理员实时表需要端到端回归覆盖。
8. 快速引导第 4 页“生成 API key”存在新 bug：API key 已真实生成并可在后台看到，但自动复制失败时页面显示“复制到粘贴板失败”，导致第 5 步无法衔接。

本 spec 目标是把这些行为重新定义清楚，然后交给 implementation plan 分任务修复。

## 目标

### 业务目标

- 日卡、周卡、月卡只改变用户的套餐计费权益，不改变模型路由组。
- 用户购买或兑换日/周/月卡后默认启用套餐；用户可手动关闭。
- 之前已购买且仍有效的日/周/月卡用户，在没有明确手动关闭记录时默认启用。
- 套餐启用时，Plus / Pro 模型仍使用原本模型路由组，例如 `gpt-plus` / `gpt-pro` / `auto` 解析后的组。
- 套餐启用时，计费倍率按套餐契约显示和记录为 `1x`，并保留原始路由组倍率用于审计。
- 管理员能在订单管理页看到日/周/月卡订单、删除测试订单、查看日/周/月卡用户实时 5 小时 / 7 天 / 总剩余额度。
- LDXP 现金购买的日/周/月卡应参与返利。
- 管理员和用户都能明确创建、兑换日/周/月卡兑换码。
- 快速引导中 API key 一旦成功创建并 reveal，即使自动复制失败，也不能阻断第 5 步；页面必须保留 key，并提供“再次复制 / 手动复制 / 继续下一步”。

### 技术目标

- 明确区分真实用户身份、模型路由身份、套餐计费身份。
- 所有套餐扣费路径统一更新订阅额度和 rolling-window usage record。
- 后端提供权威套餐计费状态，前端不再自行推断套餐倍率。
- 关键链路有 failing tests → implementation → passing tests 的回归覆盖。
- 上线前具备明确的日志检查、手动验收和回滚条件。

## 非目标

- 本次不重新设计普通订阅套餐体系。
- 本次不改变 Plus / Pro 模型分组配置，不新增 `day-card` / `week-card` / `month-card` distributor 分组。
- 本次不改变日/周/月卡当前业务倍率契约；默认有效套餐倍率为 `1x`。
- 本次不把免费兑换码、管理员赠送、手工补偿默认计入现金返利。除非后续另立 spec，否则返利只针对现金购买来源。
- 本次不直接上线服务器。上线必须等用户明确说“上线服务器”。

## 术语和身份边界

### `UserGroup`

真实用户身份组，来源于用户记录或用户缓存，例如：

- `体验用户`
- `default`
- `vip`

`UserGroup` 用于权限、可见性、用户身份展示等逻辑。套餐启用后也不能变成 `day-card` / `week-card` / `month-card`。

### `TokenGroup`

API key 保存的模型路由组，例如：

- `gpt-plus`
- `gpt-pro`
- `auto`

API key 页面应展示这个真实 token group，而不是把它替换为套餐组。

### `UsingGroup`

单次请求经 distributor / auto group / playground 解析后的实际模型路由组，例如：

- `gpt-plus`
- `gpt-pro`

`UsingGroup` 用于模型可用性、渠道选择、distributor 调度。它不能是：

- `day-card`
- `week-card`
- `month-card`

出现类似错误即为回归：

```text
分组 week-card 下模型 gpt-5.5 无可用渠道
```

### `ValuePackageBillingGroup`

套餐计费身份，例如：

- `day-card`
- `week-card`
- `month-card`

它只用于套餐计费倍率、套餐额度、5 小时 / 7 天 rolling-window 逻辑，不参与 distributor 渠道选择。

### `BillingUserGroup`

单次请求实际用于计费倍率计算的身份：

- 没有启用套餐时：等于真实 `UserGroup`。
- 启用日/周/月卡时：等于 `ValuePackageBillingGroup`。

该字段用于替代当前把 `RelayInfo.UserGroup` 偷换成套餐组的临时做法。

## 当前审计证据

### 套餐 middleware 已不再改路由组

文件：`/Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening/middleware/value_package.go`

当前 `applyValuePackageGroupScope()` 只设置套餐上下文：

```go
common.SetContextKey(c, constant.ContextKeyValuePackageSubscriptionId, state.Subscription.Id)
common.SetContextKey(c, constant.ContextKeyValuePackagePlanId, state.Plan.Id)
common.SetContextKey(c, constant.ContextKeyValuePackageModelGroup, modelGroup)
common.SetContextKey(c, constant.ContextKeyValuePackagePackageType, state.Plan.PackageType)
```

它不再覆盖 `ContextKeyUserGroup`、`ContextKeyUsingGroup`、`ContextKeyTokenGroup`。这是正确方向。

### `RelayInfo.UserGroup` 仍被偷换成套餐组

文件：`/Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening/relay/common/relay_info.go`

当前逻辑：

```go
userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
if valuePackageGroup := strings.TrimSpace(common.GetContextKeyString(c, constant.ContextKeyValuePackageModelGroup)); valuePackageGroup != "" {
    userGroup = valuePackageGroup
}
```

这会导致 `RelayInfo.UserGroup` 在套餐启用时变成 `month-card` 等套餐组。该做法必须替换为显式 billing identity。

### Realtime WS 路径可能漏写 rolling usage record

文件：`/Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening/service/quota.go`

`PreWssConsumeQuota()` 使用：

```go
err = PostConsumeQuota(relayInfo, quota, 0, false)
```

而 `PostConsumeQuota()` 在订阅分支只调整 `UserSubscription.AmountUsed`，没有同步写 `ValuePackageUsageRecord`。管理员实时表和 5h/7d 限额依赖 `ValuePackageUsageRecord` 聚合，因此 realtime 套餐流量存在窗口统计漏算风险。

### API key `auto` 分组未展示套餐倍率

文件：`/Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening/web/default/src/features/keys/lib/api-key-display.ts`

当前逻辑：

```ts
if (group === 'auto') {
  return { group, isEffective }
}
```

这会让 active package ratio 在 `auto` key 上丢失。

### 快速引导生成 API key 的复制失败被当成整体失败

文件：`/Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening/web/default/src/features/quick-start/quick-start-api-key.ts`

当前链路：

1. 创建 API key。
2. 搜索刚创建的 key。
3. reveal key。
4. 自动复制到剪贴板。
5. 如果复制失败，直接 throw：

```ts
if (!(await dependencies.copyToClipboard(fullKey))) {
  throw new Error('Failed to copy the new API key')
}

return { name, fullKey }
```

因此当 clipboard API 被浏览器权限、iframe、非 HTTPS、Safari/iOS、系统策略等原因拒绝时，虽然 API key 已创建成功，页面仍进入 catch 分支，`setGeneratedApiKey(result.fullKey)` 不执行，第 5 页拿不到 key。

对应页面代码：`/Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening/web/default/src/features/quick-start/index.tsx`

```ts
const result = await generateAndCopyQuickStartApiKey(...)
setGeneratedApiKey(result.fullKey)
toast.success(t('Already copied to clipboard'))
```

当前设计把“生成成功”和“复制成功”强绑定，这是 bug 根因。

## 需求细则

### R1：订单管理必须纳入日/周/月卡订单

- LDXP 日/周/月卡购买订单必须出现在管理员订单管理列表。
- 订单应显示：用户、金额、支付方式、订单号、邮件核验状态、套餐名称、返利信息。
- 列表、统计、趋势、返利统计应使用同一套订单可见性规则。
- 删除订单后，该订单从列表和统计中隐藏。
- 删除订单后，未来邮件扫描不能再次把它扫回来。
- 删除是 deletion mark，不是硬删原始支付/订单数据。

### R2：管理员实时额度表

管理员订单管理界面必须显示一个实时表格，覆盖日卡、周卡、月卡正在使用的用户。

每行至少包含：

- 用户 ID / 用户名
- 套餐类型：日卡 / 周卡 / 月卡
- 套餐名称
- 模型路由组 / 套餐计费组标识
- 5 小时窗口：已用、限额、剩余、百分比
- 7 天窗口：已用、限额、剩余、百分比
- 总额度：已用、总量、剩余
- 到期时间
- 当前是否启用套餐使用

实时性要求：

- 前端自动刷新，当前 15 秒刷新可以接受。
- 后端返回数据必须与 rolling-window enforcement 使用同一数据源。
- 普通 HTTP、stream、realtime WebSocket、异步任务如消耗套餐，都必须能反映到统计中。

### R3：日/周/月卡兑换码

用户端：

- 日卡、周卡、月卡卡片上必须有兑换码输入和兑换按钮。
- 兑换成功后应立即刷新套餐状态。
- 兑换成功后套餐默认启用。
- 兑换失败应显示明确错误。

管理员端：

- 创建兑换码时，必须能选择“余额/充值码”或“日/周/月卡套餐”。
- 选择“日/周/月卡套餐”后，必须能选择具体 enabled 的日卡、周卡、月卡 plan。
- 如果没有 enabled 的日/周/月卡 plan，UI 必须明确提示“没有启用的日/周/月卡套餐，请先启用套餐计划”，不能只显示空下拉。
- 批量创建后应能复制创建出的兑换码。

返利规则：

- LDXP 现金购买日/周/月卡：算返利。
- 免费兑换码 / 管理员赠送 / 手工补偿：默认不算现金返利。
- 如果未来要支持“付费兑换码也算返利”，需要另立需求，明确资金来源、订单号和邀请关系。

### R4：套餐默认启用和手动关闭

- 新购买日/周/月卡后默认启用。
- 兑换日/周/月卡后默认启用。
- 之前已经购买且仍有效的用户，如果没有明确手动关闭记录，默认启用。
- 用户可以手动关闭套餐使用。
- 用户明确手动关闭后，系统不能在普通查询时重新自动打开。
- 对旧数据的 backfill 只能识别“未触碰过的默认偏好记录”，不能覆盖用户真实手动选择。

### R5：套餐不能污染模型路由组

启用日/周/月卡时：

- `TokenGroup` 仍是 API key 原始组，例如 `gpt-plus`、`gpt-pro`、`auto`。
- `UsingGroup` 仍是 distributor 使用的模型路由组，例如 `gpt-plus`、`gpt-pro`。
- `UserGroup` 仍是真实用户身份组，例如 `vip`、`体验用户`。
- 只有 `BillingUserGroup` / `ValuePackageBillingGroup` 可以是 `day-card` / `week-card` / `month-card`。

禁止出现：

```text
分组 day-card 下模型 ... 无可用渠道
分组 week-card 下模型 ... 无可用渠道
分组 month-card 下模型 ... 无可用渠道
```

如果出现，视为 P0 回归。

### R6：套餐计费倍率必须显示为套餐 1x

启用日/周/月卡时：

- 计费应走 subscription funding。
- 即使用户偏好是 wallet only，也应优先使用 active value package。
- 套餐有效计费倍率为 `1x`。
- 日志中必须记录：
  - `billing_source = subscription`
  - `subscription_ratio_applied = true`
  - `value_package_effective_ratio = 1`
  - 原始路由组倍率，例如 `original_group_ratio = 0.3`
  - 原始用户专属倍率，例如 VIP group-group ratio，如果存在
- API key 页面显示真实路由组 + 套餐计费，例如：
  - `gpt-plus · 套餐 1x`
  - `auto · 套餐 1x`
- 日志列表显示“套餐 1x”，不要只显示裸 `1x`。
- 日志详情显示“套餐计费 / 原始倍率 / 实际倍率”。

### R7：套餐额度消耗必须统一记账

所有套餐消耗路径必须满足：

- `UserSubscription.AmountUsed` 的变化与套餐 usage record 总和一致，除非有明确的预扣/退款中间态。
- `ValuePackageUsageRecord` 必须记录每个请求或每个 realtime chunk 的实际套餐消耗。
- 失败请求必须退款或把 usage record 更新为 0。
- 成功请求不能重复计算。
- realtime WebSocket 不能只扣订阅总量而不写 5h/7d usage record。
- 异步任务提交和完成后的差额结算必须维持 subscription amount 与 usage record 一致。

滚动窗口：

- 5 小时和 7 天统计以 `ValuePackageUsageRecord.created_at` 和 `quota` 聚合。
- 长请求跨窗口边界时，默认按记录创建时间归属窗口；如果未来改为结束时间，需要单独迁移和说明。

### R8：并发限制要可恢复、可观测

- 日/周/月卡并发限制只针对套餐启用后的消耗请求。
- Redis 模式下，进程崩溃或请求异常结束后，stale slot 必须能在 TTL 内恢复。
- 正常请求结束必须 release slot。
- 长请求必须 heartbeat refresh slot。
- 生产日志必须能区分：
  - 真实并发达到上限
  - Redis 错误
  - stale slot 被清理
  - release 失败
- 不允许用户单次请求失败后长期卡住“并发请求数已达上限”。

### R9：快速引导 API key 生成和复制必须解耦

当前第 4 页“生成 API key”的正确行为应为：

1. 如果 create API key 失败：显示“创建 API key 失败”，不进入第 5 步。
2. 如果 create 成功但 search/reveal 失败：显示具体错误，不进入第 5 步，因为前端没有拿到 key。
3. 如果 reveal 成功但自动复制失败：
   - 视为“API key 已生成成功”。
   - 必须保存 `generatedApiKey` 到页面状态。
   - 第 4 页应显示“API key 已生成，但自动复制失败，请点击再次复制或手动复制”。
   - 第 5 页必须可以使用该 key 继续 CC Switch / Codex 配置。
   - 不应让用户以为 API key 没创建成功。
4. 如果 reveal 成功且复制成功：显示现有成功状态。
5. 已生成 key 后再次点击按钮，应只尝试复制现有 key，不应重复创建新 key。

推荐返回类型：

```ts
type QuickStartApiKeyResult = {
  name: string
  fullKey: string
  copied: boolean
}
```

不再因为 `copied === false` throw。

UI 状态建议：

- `generatedApiKey: string`
- `generatedApiKeyCopied: boolean | null`
- `apiKeyCopyWarning: string | null`

展示建议：

- 复制成功：`API key is ready` + `Already copied to clipboard`
- 复制失败但生成成功：`API key is ready` + `Copy failed. The key was generated; copy it manually or try again.`
- 按钮：`Copy API key again`
- 可选：显示 mask 后的 key 和一个明确 copy 按钮；不要直接明文常驻显示 full key，除非用户点击 reveal。

### R10：快速引导 Clipboard fallback

当前 copy 工具已提供 `navigator.clipboard.writeText` 和 `document.execCommand('copy')` fallback。

本次不要求重写全站复制工具，但快速引导必须满足：

- 自动复制失败不能影响生成成功状态。
- 手动点击“再次复制”时应再次调用 copy 工具。
- 如果再次复制仍失败，用户仍能继续第 5 步，且能看到明确说明。
- 测试中要覆盖 `copyToClipboard` 返回 false 的场景。

## 推荐方案

### 方案 A：最小修补

只修当前暴露 bug：

- `RelayInfo.UserGroup` 不动。
- API key auto 显示补一下。
- 快速引导复制失败不 throw。

优点：快。

缺点：身份污染和 realtime usage 漏算仍在，后续很可能继续出现新 bug。

### 方案 B：契约优先的稳定化修复（推荐）

按本 spec 和后续 plan 执行：

- 先拆身份边界。
- 后端返回 authoritative billing state。
- 统一套餐 usage accounting。
- 前端所有显示改读后端 billing state。
- 快速引导生成和复制解耦。
- 最后完整回归订单/兑换码/返利/实时表/并发锁。

优点：一次把临时补丁背后的结构问题清掉。

缺点：改动较多，需要严格分任务和测试。

### 方案 C：先 P0/P1，P2 后续

先做：

- 身份拆分。
- 快速引导复制失败。
- API key auto 显示。
- 日志清晰显示。
- realtime usage record。

暂缓：

- redemption empty state。
- 并发锁更多观测。
- classic 前端确认。

优点：更快解决用户能感知的问题。

缺点：订单/兑换/并发仍可能有残余体验问题。

本 spec 推荐方案 B。如果时间压力很大，可按方案 C 的 P0/P1 子集执行，但不能再做无测试的临时补丁。

## 验收标准

### 后端验收

- active value package 请求中：
  - `RelayInfo.UserGroup` 保持真实用户组。
  - `RelayInfo.UsingGroup` 是 `gpt-plus` / `gpt-pro` 等模型路由组。
  - `RelayInfo.BillingUserGroup` 或等价字段是 `day-card` / `week-card` / `month-card`。
- 不再出现 distributor 使用套餐组找模型的错误。
- 普通 HTTP、stream、realtime WS、异步任务套餐消耗都能更新 usage record。
- rolling-window 限额和管理员实时表读取同一 usage record 数据。
- LDXP 现金日/周/月卡订单算返利。
- 删除订单不会再次进入邮件扫描。

### 前端验收

- API key 页面：
  - `gpt-plus` key 显示 `gpt-plus · 套餐 1x`。
  - `auto` key 显示 `auto · 套餐 1x`。
  - 关闭套餐后恢复显示原 token group 倍率。
- 使用日志：
  - 列表显示“套餐 1x”。
  - 详情显示套餐计费来源、原始倍率、实际倍率。
- 管理员订单管理：
  - 能看到日/周/月卡订单。
  - 能删除测试订单。
  - 能看到日/周/月卡用户实时额度表。
- 用户日/周/月卡卡片：
  - 有兑换码入口。
  - 兑换成功后状态刷新并默认启用。
- 管理员兑换码创建：
  - 能选择日/周/月卡套餐。
  - 没有 enabled 套餐时有明确空态说明。
- 快速引导：
  - API key 创建成功但复制失败时，第 4 页显示“已生成但复制失败”。
  - 第 5 页仍可读取生成的 key 并继续配置。
  - 用户能再次点击复制，不会重复创建 key。

### 测试验收

至少执行：

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening
go test ./middleware ./relay/common ./relay/helper ./relay/channel/openai ./service ./model ./controller -count=1 -timeout=300s
```

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening/web/default
bun test src/features/quick-start/quick-start-api-key.test.ts \
  src/features/keys/lib/api-key-display.test.ts \
  src/features/value-packages/lib/billing-display.test.ts \
  src/features/usage-logs/components/columns/common-logs-columns.test.ts \
  src/features/redemption-codes/lib/redemption-form.test.ts \
  src/features/redemption-codes/components/redemptions-mutate-drawer-source.test.ts \
  src/features/order-management/components/value-package-usage-table-source.test.ts \
  src/features/order-management/order-management-source.test.ts
bun run typecheck
bun run build
bun run i18n:sync
```

## 上线前检查

上线前必须确认：

- GitHub `main` 已包含修复。
- 本地 main 与 origin/main 同步。
- 没有未提交生产逻辑文件。
- 服务器上线前先备份或记录当前 SHA。
- 不打印 `.env`、API key、支付密钥、SSH key、数据库密码。

上线后检查日志：

```bash
grep -E '分组 (day-card|week-card|month-card) 下模型 .* 无可用渠道' -R logs/ || true
grep -E 'value package concurrency denied|超值套餐并发请求数已达上限' -R logs/ | tail -50 || true
grep -E 'subscription_ratio_applied|value_package_effective_ratio' -R logs/ | tail -50 || true
```

期望：

- 第一条无结果。
- 并发 denied 只在真实并发时出现。
- 套餐请求日志能看到套餐计费标记。

## 回滚条件

任一情况出现即回滚：

- 日/周/月卡用户再次出现 `day-card` / `week-card` / `month-card` distributor 无渠道错误。
- 套餐用户无法使用原本 Plus / Pro 模型。
- 单次请求重复扣订阅额度。
- realtime 套餐请求出现明显 double count。
- 快速引导生成 key 后丢失 key，无法进入第 5 步。
- 删除订单后再次被邮件扫描或重新出现在订单列表。

## 后续实施计划入口

本 spec 对应实施计划：

`/Users/ethan/Documents/yunbay/.worktrees/ci-xss-hardening/docs/superpowers/plans/2026-07-06-value-package-stabilization-audit-plan.md`

该计划需要补充快速引导 API key 复制兜底任务，建议插入到前端显示任务之前，作为 P1 处理。
