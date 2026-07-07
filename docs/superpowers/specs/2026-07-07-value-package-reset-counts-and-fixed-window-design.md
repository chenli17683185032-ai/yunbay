# 超值套餐固定恢复窗口与重置次数功能 Spec

Date: 2026-07-07
Status: Draft for review
Scope: 云贝日卡 / 周卡 / 月卡的恢复时间语义修正、用户重置次数、独立后台“超值套餐管理”页面

> Supersedes note: 本 spec 明确修正 `docs/superpowers/specs/2026-07-07-value-package-openai-style-reset-time-design.md` 中使用 `MAX(created_at) + window` 表示恢复时间的语义。后续实现应以本文的 `MIN(created_at) + window` 和 reset event 下界为准。

## 1. 背景

云贝已经上线日卡、周卡、月卡（本文统一称“超值套餐”），并已支持：

- 用户购买 / 兑换后默认启用；
- 套餐只影响套餐身份和套餐计费，不改变 Plus / Pro 模型 distributor 路由组；
- 后端记录 `value_package_usage_records`；
- 管理员订单管理页可以看到日卡 / 周卡 / 月卡用户实时 5 小时、7 天、总额度用量；
- 用户端套餐卡片显示 5 小时、7 天用量和恢复时间。

最近实现的恢复时间使用了 `MAX(created_at) + window` 作为“完全恢复时间”。这个语义会造成一个用户可见 bug：

```text
09:00 用户用了较多额度 -> 显示约 5 小时后恢复
11:00 用户只聊了一句话 -> 恢复时间又被推到接近 5 小时
```

用户明确要求不要出现这种行为。恢复时间不应被同一窗口内后续小额使用不断刷新成满窗口。

同时，用户提出新增功能：

> 给所有日卡、周卡、月卡用户增加一个充值功能。管理员可以在订单管理后台手动调整充值次数。订单管理里单开一个“超值套餐管理”页面，不要和现在订单管理页混在一起。用户有了次数后，可以用 1 次重置一次使用额度。

本 spec 同时定义这两个点。

## 2. 目标

### 2.1 固定恢复窗口目标

- 5 小时 / 7 天窗口内，恢复时间不能被后续新用量不断推迟到满窗口。
- 当前窗口的恢复时间应由窗口内“最早仍有效的正用量”决定。
- 当最早那批用量滚出窗口后，恢复时间才自然切换到下一批仍有效用量。
- 限流统计仍是 rolling window，不改成固定周期、不改成每日 0 点清空。
- 不改变模型分组、倍率、并发限制、订单和支付链路。

### 2.2 重置次数目标

- 所有日卡 / 周卡 / 月卡用户都可以拥有“额度重置次数”。
- 管理员在独立后台页面“超值套餐管理”里能查看、搜索、调整用户重置次数。
- 用户端能看到自己的剩余重置次数。
- 用户有剩余次数时，可以点击按钮消耗 1 次，重置当前启用超值套餐的短周期使用额度。
- 重置操作必须可审计、可追踪、并发安全，不能因为双击或并发请求扣错次数。

## 3. 非目标

本次不做以下事情：

- 不改变 Plus / Pro distributor 模型路由组。
- 不把 `day-card` / `week-card` / `month-card` 当作模型渠道分组。
- 不改变套餐默认倍率契约。
- 不重新设计钱包余额、普通充值或 LDXP 支付链路。
- 不让“重置次数”延长日卡 / 周卡 / 月卡有效期。
- 不让“重置次数”恢复或增加套餐总额度。
- 不物理删除历史 `value_package_usage_records`。
- 不把重置次数和普通钱包余额混为同一种资产。
- 不把现有订单管理列表继续塞更多复杂 UI；新增独立页面承载超值套餐管理。

## 4. 术语

### 4.1 超值套餐

本文指 `SubscriptionPlan.PlanKind = value_package` 且 `PackageType` 为：

- `day`
- `week`
- `month`

### 4.2 短周期桶

当前系统已有两个滚动桶：

```text
5h bucket: 最近 5 小时内套餐用量
7d bucket: 最近 7 天内套餐用量
```

### 4.3 总额度

`UserSubscription.AmountTotal` 和 `UserSubscription.AmountUsed` 代表套餐生命周期内总额度。

本 spec 中“重置一次使用额度”**不重置总额度**。

### 4.4 重置次数

一个只属于超值套餐用户的非负整数余额：

```text
value_package_reset_count
```

用户每成功执行一次短周期额度重置，余额减 1。

### 4.5 重置事件

一次成功的用户重置操作。重置事件不删除历史用量，只改变之后窗口计算的起点。

## 5. 固定恢复窗口设计

### 5.1 当前错误语义

当前实现使用：

```text
reset_at_5h = MAX(created_at in current 5h window) + 5h
reset_at_7d = MAX(created_at in current 7d window) + 7d
```

这会导致同一窗口内用户后续每发一条消息，`MAX(created_at)` 变新，恢复时间被刷新。

### 5.2 新语义

恢复时间改为：

```text
reset_at_5h = MIN(created_at in effective 5h window) + 5h
reset_at_7d = MIN(created_at in effective 7d window) + 7d
```

其中只统计正用量：

```text
quota > 0
```

并且需要考虑重置事件，见第 6 节。

### 5.3 示例

假设 5 小时额度是 60 单位：

| 时间 | 操作 | 5h 窗口内用量 | 恢复时间 |
| --- | ---: | ---: | --- |
| 09:00 | 用 20 | 20 | 14:00 |
| 10:30 | 用 25 | 45 | 14:00，不变 |
| 12:00 | 用 15 | 60 | 14:00，不变 |
| 13:00 | 查询 | 60 | 14:00 |
| 14:00 后 | 09:00 那批离开窗口 | 40 | 15:30 |

解释：

- 09:00 是当前窗口内最早仍有效用量，所以完全恢复点最初是 14:00。
- 10:30、12:00 的用量不会把恢复点重新推成 15:30 或 17:00。
- 到 14:00，09:00 那批离开窗口后，当前最早仍有效用量变成 10:30，恢复点自然切换到 15:30。

### 5.4 UI 文案

已有“还有多久恢复 / Fully restored / Resets in ...”文案可以保留。

但产品语义应从“最后一次用量后完全恢复”改为：

> 当前窗口下一次完全恢复倒计时。

UI 不需要展示复杂解释，避免干扰。必要时在帮助说明中解释：

```text
额度按滚动窗口计算；后续少量使用不会把当前恢复时间重新刷新到满窗口。
```

### 5.5 后端计算

`GetValuePackageWindowUsageDetails` 应返回：

```go
type ValuePackageWindowUsageDetails struct {
    Used5h int64
    Earliest5hCreatedAt int64
    ResetAt5h int64
    ResetSeconds5h int64

    Used7d int64
    Earliest7dCreatedAt int64
    ResetAt7d int64
    ResetSeconds7d int64
}
```

兼容性选择：

- 如果已有字段名 `Latest5hCreatedAt` / `Latest7dCreatedAt` 只被内部测试使用，应改名为 `Earliest...`，避免误导。
- 如果存在外部依赖，应保留旧字段但不再用于 reset 计算，并新增 `Earliest...`。当前 evidence 显示这些字段未暴露为 JSON，优先改名。

SQL / GORM 聚合应从：

```sql
SUM(quota), MAX(created_at)
```

改为：

```sql
SUM(quota), MIN(created_at)
```

要求兼容 SQLite / MySQL / PostgreSQL：

```go
Select("COALESCE(SUM(quota), 0) AS used, COALESCE(MIN(created_at), 0) AS earliest_created_at")
```

### 5.6 和限流的关系

限流仍然使用窗口内累计量：

```text
used_5h >= limit_5h
used_7d >= limit_7d
```

只改变 reset time 展示和限流错误文案中的倒计时，不改变是否限流。

## 6. 重置次数功能设计

### 6.1 业务语义

用户点击“重置额度”并成功后：

1. 剩余重置次数减 1。
2. 当前启用超值套餐的 5 小时桶从重置时间重新开始统计。
3. 当前启用超值套餐的 7 天桶从重置时间重新开始统计。
4. 历史用量记录保留，用于审计、总额度、日志展示。
5. 套餐总额度不恢复。
6. 套餐有效期不延长。

也就是窗口统计从：

```text
created_at >= now - window_seconds
```

变为：

```text
created_at >= max(now - window_seconds, last_reset_at)
```

### 6.2 示例

假设用户月卡：

```text
5h 限额：60
7d 限额：600
总额度：1000
当前 amount_used：300
重置次数：1
```

用户最近 5h 已用 60，被限流。

用户点击“重置额度”：

```text
重置次数：0
5h used：0
7d used：0
总 amount_used：仍然 300
总剩余额度：仍然 700
有效期：不变
```

之后用户继续使用，新的 `value_package_usage_records` 正常写入，5h / 7d 从 reset 时间后重新累计。

### 6.3 不删除历史 usage

禁止通过删除或改写 `value_package_usage_records` 来实现重置。

原因：

- 会破坏账务审计；
- 会影响历史日志；
- 可能导致总额度和真实消耗对不上；
- 难以解释管理员看到的历史消耗。

正确做法是新增 reset ledger，窗口查询时把 reset 时间作为下界。

## 7. 数据模型设计

### 7.1 用户重置次数余额

推荐在 `UserValuePackagePreference` 增加字段：

```go
ResetCount int `json:"reset_count" gorm:"default:0"`
```

理由：

- 该表已经按用户保存超值套餐偏好；
- 管理员调整的是“用户的超值套餐重置次数余额”，不是某个订单的字段；
- 用户可能换日卡 / 周卡 / 月卡，次数仍属于该用户的超值套餐权益池。

迁移要求：

- SQLite / MySQL / PostgreSQL 都必须支持；
- 默认值为 `0`；
- 旧用户迁移后不自动获得次数，除非管理员手动加。

### 7.2 重置事件表

新增表：

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
```

`Source` 枚举：

```text
user_consume_count    用户消耗重置次数
admin_grant_adjust    管理员调整次数
admin_manual_reset    管理员直接替用户重置（预留，本轮可不做按钮）
```

最小实现中，窗口计算只需要 `user_consume_count` 产生的 reset event。管理员调整次数只写偏好余额和审计日志，不一定写 reset event。

### 7.3 管理员调整日志

建议新增表或复用现有 manage audit。

最小方案：复用已有管理审计日志，记录：

```text
action = value_package.reset_count.adjust
user_id
old_count
new_count
delta
reason
admin_id
```

如果当前管理审计难以查询，可新增 ledger：

```go
type ValuePackageResetCountLedger struct {
    Id int
    UserId int
    Delta int
    BeforeCount int
    AfterCount int
    Source string
    CreatedByUserId int
    CreatedAt int64
    Note string
}
```

推荐采用 ledger 表，因为后续需要追踪充值次数来源和管理员误操作。

## 8. 后端服务设计

### 8.1 获取用户超值套餐状态

现有 `GET /api/value-packages/self` 的 response 应新增：

```json
{
  "preference": {
    "reset_count": 2
  },
  "usage": {
    "used_5h": 0,
    "reset_seconds_5h": 0,
    "used_7d": 0,
    "reset_seconds_7d": 0
  }
}
```

前端可直接显示剩余次数。

### 8.2 用户消耗重置次数

新增接口：

```text
POST /api/value-packages/reset-quota
```

鉴权：普通登录用户。

请求体：

```json
{
  "user_subscription_id": 123
}
```

也可不传，默认使用当前启用套餐。推荐允许不传：

```json
{}
```

服务逻辑：

1. 开启数据库事务。
2. 锁定用户 `UserValuePackagePreference` 或使用原子条件更新。
3. 确认 `ResetCount > 0`。
4. 确认存在当前启用的 active value package subscription。
5. 若请求指定 `user_subscription_id`，必须等于当前启用套餐。
6. 将 `ResetCount` 减 1。
7. 插入 `ValuePackageQuotaReset`：
   - `Source = user_consume_count`
   - `ResetAt = now`
   - `CreatedByUserId = current user id`
8. 返回新的 `ValuePackageState`，包含最新 usage summary 和 reset_count。

错误：

| 场景 | 文案建议 |
| --- | --- |
| 没有登录 | 沿用 auth 错误 |
| 没有有效超值套餐 | `当前没有可重置的超值套餐` |
| 没有重置次数 | `重置次数不足` |
| 套餐未启用 | `请先启用超值套餐后再重置额度` |
| 并发扣减失败 | `重置次数不足或状态已变化，请刷新后重试` |

并发安全要求：

- 不能出现两次并发请求都看到 `ResetCount=1` 并都成功。
- 推荐 SQL/GORM 条件更新：
  ```text
  WHERE user_id = ? AND reset_count > 0
  UPDATE reset_count = reset_count - 1
  ```
  再检查 rows affected。
- 或事务内 `SELECT ... FOR UPDATE`，但必须兼容 SQLite/MySQL/PostgreSQL。

### 8.3 窗口用量计算纳入 reset

`GetValuePackageWindowUsageDetails` 查询窗口时，需要先获取当前订阅最后一次 reset：

```text
last_reset_at = MAX(reset_at)
WHERE user_id = ?
  AND user_subscription_id = ?
  AND source = 'user_consume_count'
```

然后：

```text
start_5h = max(now - 5h, last_reset_at)
start_7d = max(now - 7d, last_reset_at)
```

窗口 usage 查询：

```text
created_at >= start_5h AND quota > 0
created_at >= start_7d AND quota > 0
```

reset time 聚合：

```text
MIN(created_at) + window_seconds
```

当 reset 后当前窗口没有新用量：

```text
used_5h = 0
reset_at_5h = 0
reset_seconds_5h = 0
```

UI 显示 `已完全恢复`。

### 8.4 管理员超值套餐管理 API

新增管理员 API group 或复用现有 `/api/order-management/admin`：

推荐路由：

```text
GET  /api/order-management/admin/value-packages/users
POST /api/order-management/admin/value-packages/users/:user_id/reset-count
GET  /api/order-management/admin/value-packages/users/:user_id/resets
```

#### 8.4.1 列表接口

`GET /api/order-management/admin/value-packages/users`

查询参数：

```text
package_type=day|week|month|all
keyword=<user id / username>
active=active|expired|all
page=1
page_size=20
```

返回每行：

```json
{
  "user_id": 1,
  "username": "alice",
  "display_name": "Alice",
  "package_type": "month",
  "plan_title": "月卡",
  "subscription_id": 123,
  "subscription_status": "active",
  "start_time": 1780000000,
  "end_time": 1782592000,
  "enabled": true,
  "reset_count": 2,
  "usage": {
    "total_used": 300,
    "total_remaining": 700,
    "used_5h": 0,
    "limit_5h": 60,
    "reset_seconds_5h": 0,
    "used_7d": 0,
    "limit_7d": 600,
    "reset_seconds_7d": 0
  },
  "last_reset_at": 1780001000
}
```

#### 8.4.2 调整次数接口

`POST /api/order-management/admin/value-packages/users/:user_id/reset-count`

请求体：

```json
{
  "mode": "set",
  "value": 3,
  "reason": "补偿用户"
}
```

支持模式：

```text
set    设置为指定非负值
add    在当前值基础上增加 value
subtract 在当前值基础上减少 value，但不能低于 0
```

响应：

```json
{
  "user_id": 1,
  "old_count": 1,
  "new_count": 3,
  "delta": 2
}
```

要求：

- 只有管理员可调用；
- `new_count` 不能小于 0；
- 写管理审计和/或 ledger；
- 不要求用户当前必须有 active package，但后台列表应优先展示有历史或当前超值套餐的用户。

#### 8.4.3 reset 历史接口

`GET /api/order-management/admin/value-packages/users/:user_id/resets`

返回：

- 用户消耗次数 reset 记录；
- 管理员调整次数 ledger；
- 可分页。

本接口可作为第二阶段。如果首版页面只需要显示最近 reset 时间，列表接口可以先内联 `last_reset_at`。

## 9. 前端设计

### 9.1 独立“超值套餐管理”页面

不要把新功能继续塞进现有订单管理首页。新增一个页面：

```text
/order-management/value-packages
```

入口位置：

- 管理员侧边栏：订单管理下新增子项，或与订单管理同级新增“超值套餐管理”。
- 现有订单管理首页可以保留“跳转到超值套餐管理”的按钮，但不直接承载所有功能。

页面模块：

1. 统计卡片
   - 当前 active 日卡人数
   - 当前 active 周卡人数
   - 当前 active 月卡人数
   - 总剩余 reset 次数
2. 筛选栏
   - 关键词
   - 套餐类型
   - active / expired / all
3. 用户表格
   - 用户 ID / 用户名
   - 套餐类型 / 计划名称
   - 启用状态
   - 有效期
   - reset_count
   - 5h 用量 / 恢复时间
   - 7d 用量 / 恢复时间
   - 总剩余额度
   - 最近重置时间
   - 操作：调整次数
4. 调整次数弹窗
   - mode: set / add / subtract
   - value
   - reason
   - 确认后刷新列表

### 9.2 用户端入口

在用户端日卡 / 周卡 / 月卡卡片或当前套餐状态区域增加：

```text
剩余重置次数：N
[重置额度]
```

按钮规则：

- `reset_count <= 0`：按钮 disabled，提示“没有可用重置次数”。
- 没有当前 active package：隐藏或 disabled。
- 套餐未启用：提示“请先启用套餐”。
- 点击后弹确认框：
  ```text
  本次会消耗 1 次重置次数，清空当前套餐的 5 小时和 7 天短周期用量；不会恢复总额度或延长有效期。是否继续？
  ```
- 成功后 toast：
  ```text
  已重置 5 小时和 7 天用量额度
  ```
- 失败时展示后端错误。

### 9.3 充值功能的首版边界

用户说“增加一个充值功能”。本 spec 首版把“充值”定义为：

- 用户端可看到次数；
- 管理员可手动给用户加次数；
- 用户可消耗次数重置短周期桶。

是否允许用户自己用 LDXP / 钱包购买重置次数，属于第二阶段。原因：

- 需要新增商品、金额、订单、返利、退款、重复扫描规则；
- 当前用户明确提到“次数我可以在后台手动调整”，先满足后台发放和用户消耗。

如果后续要做付费购买重置次数，应另写 spec。

## 10. i18n

新增 UI 文案必须走 `web/default/src/i18n/locales/{en,zh,fr,ja,ru,vi}.json`。

关键文案：

```text
Value Package Management
Reset count
Adjust reset count
Remaining reset count
Reset quota
Reset 5-hour and 7-day quota
No reset count available
Reset count is insufficient
Please enable a value package before resetting quota
This will consume 1 reset count and clear your current package's 5-hour and 7-day usage windows. It will not restore total quota or extend expiration.
```

## 11. 测试计划

### 11.1 后端 model 测试

必须新增或更新：

1. `GetValuePackageWindowUsageDetails` 使用窗口内最早正用量计算 reset：
   - `now-4h` 用 50；
   - `now-2h` 用 1；
   - reset 应为 `now+1h`，不能是 `now+3h`。
2. 5h 和 7d 都使用 `MIN(created_at)`。
3. `quota = 0` 不影响 reset。
4. 用户 reset 后：
   - reset 前 usage 不再计入 5h / 7d；
   - reset 后新 usage 正常计入；
   - 总额度不变化。
5. 并发消耗 reset_count：
   - `reset_count = 1` 时两个请求最多一个成功。
6. 管理员调整：
   - set / add / subtract；
   - subtract 不得低于 0；
   - 写 ledger / audit。

### 11.2 后端 controller 测试

覆盖：

- 用户 `POST /api/value-packages/reset-quota` 成功；
- reset_count 不足失败；
- 没有 active package 失败；
- 管理员列表返回 reset_count、usage、last_reset_at；
- 管理员调整次数返回 old/new/delta；
- 权限不足无法调用管理员接口。

### 11.3 middleware / 限流测试

覆盖：

- reset 前 5h 达限会限流；
- 消耗 1 次 reset 后，同样用户立即可通过限流检查；
- 后续使用重新累计；
- 错误文案中的 reset_seconds 不会因窗口内后续小额使用被推回满 5 小时。

### 11.4 前端测试

覆盖：

- 用户套餐卡片展示剩余 reset_count；
- reset_count 为 0 时按钮 disabled；
- 点击 reset 调用正确 API；
- 成功后刷新 state；
- 管理页面存在独立路由，不混在现有订单管理首页；
- 管理页面表格包含 reset_count、5h、7d、总额度、最近重置；
- 调整次数弹窗调用管理员 API。

## 12. 迁移与兼容

### 12.1 数据迁移

需要自动迁移：

- `user_value_package_preferences.reset_count`；
- `value_package_quota_resets`；
- 可选 `value_package_reset_count_ledgers`。

默认：

```text
reset_count = 0
```

旧用户不会自动获得次数。

### 12.2 旧 API 兼容

新增字段是 additive，不破坏旧前端：

- `preference.reset_count`；
- `usage.reset_at_5h` / `reset_seconds_5h` 语义修正但字段保留；
- 管理员新接口独立新增。

### 12.3 回滚

如果出现严重问题：

- 可以隐藏前端 reset 按钮；
- 后端不删除 reset 表；
- 窗口查询可临时忽略 reset event；
- `reset_count` 字段保留不影响旧逻辑。

## 13. 安全与审计

- 管理员调整次数必须记录操作者、目标用户、旧值、新值、原因。
- 用户消耗 reset 次数必须记录 reset event。
- 不允许用户给自己增加次数。
- 不允许负数次数。
- 所有接口使用现有 session / `New-Api-User` 鉴权规则。
- 错误响应不得泄露其他用户数据。

## 14. 验收标准

### 14.1 固定恢复窗口验收

给定：

```text
09:00 用 50
11:00 用 1
当前时间 11:00 后
```

5 小时恢复时间应接近：

```text
14:00
```

而不是：

```text
16:00
```

也就是后续小额使用不能把倒计时刷新成满 5 小时。

### 14.2 用户 reset 验收

给定用户：

```text
active 月卡
reset_count = 1
5h used 已达限
7d used 有历史值
amount_used = 300
```

用户点击 reset 后：

```text
reset_count = 0
5h used = 0
7d used = 0
amount_used = 300 不变
可以继续使用套餐
```

### 14.3 管理员页面验收

管理员进入独立页面：

```text
/order-management/value-packages
```

可以：

- 搜索用户；
- 看日卡 / 周卡 / 月卡类型；
- 看 5h / 7d / 总额度；
- 看 reset_count；
- 手动 set / add / subtract reset_count；
- 调整后用户端立即看到新次数。

## 15. 实施顺序建议

1. 先修复恢复时间语义：`MAX(created_at)` -> `MIN(created_at)`，补回归测试。
2. 增加 reset_count 字段和 reset event / ledger 表。
3. 实现窗口查询纳入 `last_reset_at`。
4. 实现用户 reset API 和服务层事务。
5. 实现管理员列表和调整 API。
6. 实现用户端按钮和文案。
7. 实现独立管理员“超值套餐管理”页面。
8. 跑后端 / 前端 / i18n / build 验证。
9. 合并 main、推 GitHub。
10. 用户确认后再部署服务器。
