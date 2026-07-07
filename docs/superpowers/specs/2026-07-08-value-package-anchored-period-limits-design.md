# 超值套餐 7 天固定周期与 7/30 天总限额管理 Spec

Date: 2026-07-08
Status: Draft for review
Scope: 云贝超值套餐（日卡 / 周卡 / 月卡）的 7 天非滚动周期语义、30 天月卡语义、套餐管理表单总限额设置、用户端/后台限额展示与测试覆盖

> 本 spec 以当前 `main` 分支实际代码为准，并覆盖之前“7 天 rolling window”相关表述。后续实现以本文定义的“从开通套餐时刻锚定的固定时间周期”为准。

## 1. 背景

当前超值套餐已经有三类卡：

- 日卡：`package_type = day`
- 周卡：`package_type = week`
- 月卡：`package_type = month`

后端已有字段：

- `SubscriptionPlan.TotalAmount`：套餐总额度，购买后复制到 `UserSubscription.AmountTotal`。
- `SubscriptionPlan.Limit5hAmount`：5 小时限额。
- `SubscriptionPlan.Limit7dAmount`：7 天限额。
- `UserSubscription.StartTime` / `EndTime`：用户实际开通与过期时间。
- `ValuePackageUsageRecord.CreatedAt` / `Quota`：套餐用量记录。
- `ValuePackageQuotaReset`：用户消耗重置次数或管理员重置生成的短窗口重置事件。

当前问题集中在两处：

1. **7 天限额语义错误**：现有后端仍用 `now - 7 days` 或历史用量集合做 rolling 7 天统计，并且手动 reset 会把 7 天统计下界推到 reset 时间。这和现在确认的业务语义冲突。
2. **套餐管理界面表达错误**：后台创建/编辑超值套餐时，`total_amount` 仍显示为通用的 `Received amount`，`limit_7d_amount` 仍显示为裸字段名或 rolling 语义说明。管理员无法清楚设置“周卡 7 天总限额”和“月卡 30 天总限额”，容易以为只能配置日限额或短窗口限额。

## 2. 已确认业务语义

### 2.1 7 天不是 rolling window

“7 day”必须按固定时间周期计算：

```text
anchor = 用户开通该套餐的时间 UserSubscription.StartTime
第 1 个 7 天周期 = [anchor, anchor + 7 * 24h)
第 2 个 7 天周期 = [anchor + 7 * 24h, anchor + 14 * 24h)
...
```

它不是：

```text
[now - 7 days, now]
```

也不是每次请求后重新推迟的滚动恢复窗口。

### 2.2 7 天周期不受手动 reset 影响

用户或管理员的额度 reset 事件不改变 7 天周期：

- 不改变 7 天周期起点；
- 不清空当前 7 天周期内已经发生的 7 天用量；
- 不推迟 7 天恢复时间；
- 不提前开启下一段 7 天周期。

7 天周期只受 `UserSubscription.StartTime`、固定 7 天长度、`UserSubscription.EndTime` 影响。

### 2.3 周卡 7 天到期

周卡本身就是 7 天有效期：

```text
week card duration = 7 * 24h from StartTime
```

因此周卡的“7 天总限额”应理解为整个周卡生命周期内的套餐总额度。周卡到 `StartTime + 7 * 24h` 后过期，不需要在同一个订阅内进入下一段 7 天周期。

### 2.4 日卡不涉及 7 天限额

日卡有效期是 1 天。日卡不展示、不校验、不消耗 `limit_7d_amount` 的 7 天周期能力。即使旧数据里日卡计划意外存在非零 `limit_7d_amount`，运行时也应忽略它，避免日卡被错误套上 7 天规则。

### 2.5 月卡按 30 天处理

月卡的产品语义按固定 30 天计算：

```text
month card duration = 30 * 24h from StartTime
```

不要继续依赖日历月 `AddDate(0, 1, 0)` 造成 28/29/30/31 天不一致。新建或编辑月卡计划时，前后端都应提交/保存为：

```text
duration_unit = day
duration_value = 30
custom_seconds = 0
```

已有活跃订阅不在本次修复中强行缩短或延长，避免改变用户已经购买的有效期；但新购买、重新保存后的套餐计划按 30 天执行。

## 3. 设计选择

### 3.1 方案对比

#### 方案 A：新增 `limit_30d_amount` 等数据库字段

优点：字段命名最直观。

缺点：需要数据库迁移、API DTO 变更、管理表单和用户端展示全链路改造，并且会和已有 `total_amount` 重复。当前需求可以用已有字段表达，不需要增加迁移风险。

#### 方案 B：继续使用 `limit_7d_amount` 表示 7 天总限额，新增 30 天字段

优点：少改一部分旧代码。

缺点：`limit_7d_amount` 本质是窗口限额字段，不适合承载“周卡生命周期总额度”；30 天仍要新增字段，模型会更混乱。

#### 方案 C：使用 `total_amount` 作为卡周期总限额，保留 `limit_7d_amount` 为可选 7 天固定周期限额

优点：不新增数据库字段；总额度、5 小时限额、7 天周期限额职责清楚；兼容旧 API；实现范围可控。

本 spec 采用方案 C。

### 3.2 字段职责

| 字段 | 新语义 | 日卡 | 周卡 | 月卡 |
| --- | --- | --- | --- | --- |
| `total_amount` | 套餐有效期内总额度。UI 按卡类型显示为 1 天 / 7 天 / 30 天总限额。 | 1 天总限额 | 7 天总限额 | 30 天总限额 |
| `limit_5h_amount` | 5 小时短窗口限额。沿用现有固定恢复窗口语义。 | 可用 | 可用 | 可用 |
| `limit_7d_amount` | 可选的 7 天固定周期限额，从 `StartTime` 锚定，不 rolling，不受手动 reset 影响。 | 忽略 | 通常不需要，因 `total_amount` 已覆盖 7 天总额 | 可选，用于“每 7 天最多可用多少”的阶段限额 |

`limit_7d_amount` 不再被称为“7 天总限额”。“总限额”统一由 `total_amount` 承担。

## 4. 后端设计

### 4.1 周期计算 helper

新增或重构一个内部 helper，用订阅开通时间锚定固定窗口：

```go
type valuePackageAnchoredWindow struct {
    Start int64
    End   int64
}

func valuePackageAnchoredWindow(startTime int64, endTime int64, windowSeconds int64, now int64) valuePackageAnchoredWindow
```

行为：

- `startTime <= 0` 或 `windowSeconds <= 0` 时返回空窗口。
- 如果 `now < startTime`，当前窗口从 `startTime` 开始。
- 否则使用整数除法计算当前窗口序号：

```text
index = floor((now - startTime) / windowSeconds)
window_start = startTime + index * windowSeconds
window_end = window_start + windowSeconds
```

- 如果 `endTime > 0 && window_end > endTime`，则 `window_end = endTime`。
- 周卡 `endTime = startTime + 7d` 时，只有一个 7 天窗口，窗口结束即订阅过期。
- 月卡 `endTime = startTime + 30d` 时，7 天窗口边界为 `start+7d`、`start+14d`、`start+21d`、`start+28d`，最后一段可以是 `[start+28d, start+30d)` 的短窗口。

### 4.2 7 天用量计算

`Used7d` 改为只统计当前锚定窗口内正用量：

```text
record.quota > 0
record.created_at >= current_7d_window.Start
record.created_at < current_7d_window.End
record.created_at <= now
```

不再使用：

```text
created_at >= now - 7d
created_at >= lastResetAt
```

也不再调用滚动语义的 `valuePackageRollingUsageDetails(records)` 来计算 7 天限额。

### 4.3 7 天恢复时间

当 `limit_7d_amount > 0` 且当前套餐类型需要 7 天周期时，返回：

```text
reset_at_7d = current_7d_window.End
reset_seconds_7d = max(reset_at_7d - now, 0)
```

这和旧逻辑不同：

- 旧逻辑：`earliest_usage_in_rolling_window + 7d`
- 新逻辑：`StartTime + N * 7d`

因此后续少量请求不会刷新恢复时间；手动 reset 也不会刷新恢复时间。

周卡如果 `reset_at_7d >= EndTime`，前端可以把它当作“到期”，而不是承诺同一个周卡内还会再次恢复。

### 4.4 日卡忽略 7 天限额

后端要提供统一判断：

```go
func valuePackageHas7dWindow(plan *SubscriptionPlan) bool {
    return plan != nil && plan.IsValuePackage() && plan.PackageType != ValuePackageTypeDay && plan.Limit7dAmount > 0
}
```

所有 7 天校验、summary、reset 秒数展示都通过这个判断。日卡即使 `Limit7dAmount > 0` 也不校验。

### 4.5 5 小时限额维持现有短窗口语义

5 小时限额不改成从开通时间固定切片。它继续使用当前已经修过的固定恢复窗口语义：

- 当前 5 小时窗口从窗口内最早有效正用量开始；
- 后续同窗口内小额使用不把恢复时间推迟；
- 用户/管理员 reset 可以清掉 5 小时短窗口统计下界；
- 5 小时 reset 不影响 7 天固定周期和套餐总额度。

因此 `ValuePackageQuotaReset` 后续只作为 5 小时短窗口的下界，不再作为 7 天周期下界。

### 4.6 总额度校验

`UserSubscription.AmountTotal` / `AmountUsed` 继续作为套餐生命周期总额度来源：

```text
if AmountTotal > 0 && AmountUsed + delta > AmountTotal => reject
```

这承担：

- 日卡 1 天总限额；
- 周卡 7 天总限额；
- 月卡 30 天总限额。

手动 reset 不恢复 `AmountUsed`，不增加 `AmountTotal`，不延长 `EndTime`。

### 4.7 预扣与实时 reserve 必须一致

以下两个路径必须使用同一套窗口计算：

- `PreConsumeValuePackageSubscription(...)`
- `ReserveValuePackageUsageToTarget(...)`

特别是 `ReserveValuePackageUsageToTarget` 需要保持 replacement 语义：

- 同一个 `request_id` 已有记录时，窗口校验要使用“替换后的记录集合”；
- 如果旧记录的 `created_at` 不在当前 5 小时窗口或当前 7 天锚定窗口内，提高 target quota 只影响总额度，不应错误计入当前窗口；
- 如果旧记录在当前窗口内，替换后的 quota 要参与当前窗口校验；
- 不能因为旧 request 在 7 天以前，就在当前 7 天窗口里重复计算 target quota。

### 4.8 查询范围

`getValuePackageWindowUsageDetailsTx` 需要加载 `UserSubscription` 和对应 `SubscriptionPlan`，以获得：

- `sub.StartTime`
- `sub.EndTime`
- `plan.PackageType`
- `plan.Limit7dAmount`

查询用量记录时可以用一个保守下界减少数据量：

```text
lower_bound = min(current_7d_window.Start, current_5h_query_start)
```

然后在 Go helper 内按窗口严格过滤，避免 SQL 中混入复杂 DB-specific 逻辑。所有 SQL 继续使用 GORM 和普通比较，保持 SQLite / MySQL / PostgreSQL 兼容。

管理列表 `ListValuePackageManagementRows` 也要改成 per-subscription summary 计算，不再简单用全局 `now - 7d` + `lastResetAt` 去预过滤 7 天记录。

### 4.9 错误文案

后端错误不再出现 `7d rolling limit exceeded`。建议改为：

```text
7d period limit exceeded
```

5 小时错误继续使用：

```text
5h limit exceeded
```

总额度错误继续使用现有用户可见提示：

```text
当前余额已用完，建议暂停使用，使用 API 或等时间跑完再使用
```

## 5. 套餐管理界面设计

### 5.1 卡类型驱动总限额标签

后台套餐创建/编辑抽屉中，`total_amount` 对超值套餐不再显示为通用 `Received amount`。当 `plan_kind = value_package` 时，标签按 `package_type` 动态显示：

| `package_type` | `total_amount` 标签 | 说明文案 |
| --- | --- | --- |
| `day` | `1-day total limit` / `1 天总限额` | 日卡有效期内最多可用额度，0 表示不限总额度。 |
| `week` | `7-day total limit` / `7 天总限额` | 周卡从开通时刻起 7 天内最多可用额度，周卡 7 天后过期。 |
| `month` | `30-day total limit` / `30 天总限额` | 月卡从开通时刻起 30 天内最多可用额度。 |

普通订阅仍可保持当前 `Received amount` 标签，避免影响非超值套餐。

### 5.2 7 天字段改名并条件显示

`limit_7d_amount` 不再用裸字段名展示，也不再描述为 rolling limit。

显示规则：

- 日卡：隐藏，并在提交 payload 时归零，防止旧表单值污染。
- 周卡：默认隐藏或作为高级字段隐藏，因为周卡 7 天总限额已经由 `total_amount` 承担。若为了兼容旧数据必须显示，标签必须是 `7-day period limit`，说明它是额外周期限额，不是总限额。
- 月卡：可显示为可选高级字段 `7-day period limit` / `每 7 天限额`，说明文案必须写清楚：从开通套餐时间开始每 7 天固定重置，不 rolling，不受手动 reset 影响；0 表示不启用这个阶段限额。

本轮最低要求是：管理员必须能清楚设置周卡 `7 天总限额` 和月卡 `30 天总限额`，这两个都落到 `total_amount`。

### 5.3 30 天月卡 duration

`web/default/src/features/subscriptions/constants.ts` 中月卡模板应改为：

```ts
{
  value: 'month',
  durationUnit: 'day',
  durationValue: 30,
}
```

`formValuesToPlanPayload()` 对 `package_type = month` 也必须提交：

```text
duration_unit = day
duration_value = 30
custom_seconds = 0
```

后端保存超值套餐时也要 normalize 月卡 duration，防止绕过前端的 API 请求继续保存成日历月。

### 5.4 后台卡片展示

`ValuePackageAdminCards` 中不要只展示：

```text
limit_5h_amount
limit_7d_amount
```

应至少展示：

- 支付金额；
- 有效期：日卡 1 天、周卡 7 天、月卡 30 天；
- 套餐总限额：按类型显示为 1 天 / 7 天 / 30 天总限额；
- 5 小时限额；
- 如果 `limit_7d_amount > 0 && package_type != day`，展示“每 7 天限额”，不能展示为“总限额”。

这样管理员在列表页就能确认 7 天和 30 天总限额是否配置正确。

### 5.5 用户端展示

用户端套餐卡片进度条也要按同样语义展示：

- `usage.total_*` 行 label 动态显示为 1 天 / 7 天 / 30 天总限额；
- 5 小时限额行保留；
- 7 天周期限额行只在 `limit_7d_amount > 0 && package_type != day` 时展示；
- 7 天 reset 文案使用固定周期结束时间，不使用 rolling 恢复时间。

用户 reset 按钮确认文案要改掉当前“clear 5-hour and 7-day usage windows”的说法，改为：

```text
This will consume 1 reset count and clear your current package's 5-hour usage window. It will not restore total quota, reset the 7-day period limit, or extend expiration.
```

中文语义：

```text
本次会消耗 1 次重置次数并清空当前套餐的 5 小时短窗口用量；不会恢复总额度，不会重置 7 天周期限额，也不会延长有效期。
```

## 6. API 与兼容性

### 6.1 不新增表字段

本轮不新增数据库字段。继续使用：

- `total_amount`
- `limit_5h_amount`
- `limit_7d_amount`
- `duration_unit`
- `duration_value`
- `custom_seconds`

### 6.2 旧数据处理

- 旧日卡计划中如果 `limit_7d_amount > 0`，运行时忽略；编辑保存时前端应提交为 0。
- 旧周卡计划中如果 `total_amount` 已配置，直接作为 7 天总限额。
- 旧周卡计划中如果 `total_amount = 0` 但 `limit_7d_amount > 0`，不能在运行时静默把它当作总额度写入用户订阅，避免隐藏数据迁移。后台编辑页应把这类配置暴露清楚，管理员保存后以 `total_amount` 为准。
- 旧月卡计划如果仍是 `duration_unit = month, duration_value = 1`，编辑保存后改为 `day, 30`。已有活跃订阅的 `EndTime` 不做强制改写。

### 6.3 JSON 与数据库规则

如本轮实现涉及 JSON marshal/unmarshal，必须使用 `common/json.go` 的 wrapper。窗口计算和查询不需要引入数据库方言函数，优先在 Go 内过滤，保持 SQLite / MySQL / PostgreSQL 兼容。

### 6.4 i18n

新增/修改前端文案需要补齐：

- `en`
- `zh`
- `fr`
- `ja`
- `ru`
- `vi`

实现阶段应使用项目 i18n 流程更新 `web/default/src/i18n/locales/*.json`。

## 7. 测试计划

### 7.1 后端单元测试

在 `model/value_package_test.go` 补充以下覆盖：

1. **7 天锚定窗口不是 rolling**
   - 订阅 `StartTime = T`；
   - 在 `T + 1h` 写入用量 A；
   - 在 `T + 7d + 1h` 查询；
   - A 不计入当前 7 天窗口；当前窗口起点为 `T + 7d`。

2. **7 天 reset 时间来自 StartTime 边界**
   - 月卡 `StartTime = T`，`now = T + 3d`；
   - `reset_at_7d = T + 7d`；
   - 在 `T + 6d` 再写入用量后，`reset_at_7d` 仍是 `T + 7d`。

3. **手动 reset 不影响 7 天用量**
   - 在当前 7 天锚定窗口内写入 7 天用量；
   - 创建 `ValuePackageQuotaReset`，`reset_at` 晚于用量；
   - `Used5h` 可被清空或归零；
   - `Used7d` 仍统计 reset 前的 7 天用量。

4. **日卡忽略 7 天限额**
   - 日卡计划设置非零 `Limit7dAmount`；
   - 在总额度足够时，请求不因 `Limit7dAmount` 被拒绝；
   - usage summary 不展示/不标记 7 天 limited。

5. **周卡 7 天总限额由 total_amount 承担**
   - 周卡 `TotalAmount = X`；
   - 累计超过 X 时触发总额度不足；
   - 错误 reason 是 total quota，而不是 7d rolling。

6. **月卡按 30 天 duration**
   - 保存或 normalize 月卡计划后，duration 是 `day/30`；
   - 新购买月卡的 `EndTime = StartTime + 30 * 24h`。

7. **reserve replacement 使用新窗口**
   - 同一 `request_id` 的旧记录在上一段 7 天锚定窗口内；
   - `ReserveValuePackageUsageToTarget` 提高 target quota；
   - 不把旧请求错误计入当前 7 天窗口；
   - 总额度 delta 仍正确增加。

8. **错误文案不再包含 rolling**
   - 触发 7 天周期限额时，错误包含 `7d period limit exceeded`；
   - 不包含 `rolling`。

### 7.2 前端测试

在 `web/default/src/features/subscriptions/lib/plan-form-value-package.test.ts` 或相邻测试中补充：

1. `getValuePackageDuration('week')` 返回 `day/7`。
2. `getValuePackageDuration('month')` 返回 `day/30`。
3. 周卡表单提交时，`total_amount` 转换为 quota units，并作为 7 天总限额保存。
4. 月卡表单提交时，`total_amount` 转换为 quota units，并作为 30 天总限额保存。
5. 日卡提交时，`limit_7d_amount` 被归零或不参与 payload 业务语义。
6. 编辑已有计划时，`total_amount` 从 quota units 回填成展示金额，不依赖 `limit_7d_amount`。

如果 UI label helper 抽成纯函数，应补测试：

- day => `1-day total limit`
- week => `7-day total limit`
- month => `30-day total limit`

### 7.3 验证命令

实现完成后至少运行：

```bash
go test ./model ./middleware ./controller ./service -run 'ValuePackage|Subscription|BillingSession|RealtimeValuePackage|OrderManagement' -count=1
cd /Users/ethan/Documents/yunbay/web/default && bun run typecheck
cd /Users/ethan/Documents/yunbay/web/default && bun run build
git diff --check
```

如果 i18n 有新增 key，再运行项目现有 i18n 同步/检查命令。

## 8. 非目标

本次不做：

- 不新增 `limit_30d_amount` 等数据库字段。
- 不重做普通订阅套餐系统。
- 不改变钱包余额、LDXP 支付、兑换码生成逻辑。
- 不让用户 reset 恢复总额度。
- 不让用户 reset 延长有效期。
- 不把日卡纳入 7 天周期限额。
- 不强制修改已有活跃订阅的 `EndTime`。
- 不删除历史 `ValuePackageUsageRecord` 或 `ValuePackageQuotaReset`。

## 9. 验收标准

修复完成后应满足：

1. 7 天统计从 `UserSubscription.StartTime` 起算，按固定 7 天边界自动切换，不再使用 rolling `now - 7d`。
2. 手动 reset 不影响 7 天周期统计、恢复时间、总额度。
3. 周卡 7 天后过期，周卡总限额通过 `total_amount` 设置和校验。
4. 日卡不展示、不校验 7 天周期限额。
5. 月卡新计划固定 30 天，不再是日历月。
6. 套餐管理界面能明确设置：
   - 日卡 1 天总限额；
   - 周卡 7 天总限额；
   - 月卡 30 天总限额。
7. 管理列表和用户端卡片不再把 `limit_7d_amount` 叫作“总限额”。
8. 后端错误和前端说明不再出现 “rolling 7-day” 语义。
9. 预扣路径和实时 reserve 路径对总额度、5 小时限额、7 天固定周期限额的判断一致。
