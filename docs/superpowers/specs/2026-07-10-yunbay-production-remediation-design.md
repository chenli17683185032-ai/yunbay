# 云贝生产问题端到端修复设计 Spec

Date: 2026-07-10
Status: Written spec awaiting user review
Branch: `codex/yunbay-production-remediation`
Scope: 超值套餐余量与历史数据、分组计价、生产稳定性与部署卫生、new-api/sub2api GPT-5.6 支持

## 1. 背景与调查口径

本设计基于 2026-07-10 对云贝生产运行态、数据库、公开 API、容器、日志和当前 `main` 源码的联合调查。证据冲突时按以下顺序判断：

```text
生产运行行为 > 生产网络响应 > 运行容器配置 > 生产数据库状态 > 本地当前源码 > 历史说明
```

调查确认生产关键业务源码与本地提交 `f7c311541ce952ca5f146e101486ad19be5eee6e` 一致，但生产部署标记和伪 worktree 指针已经失真。因此本轮不能只根据 `/opt/new-api/app/.git` 或 `.yunbay-deploy-sha` 判断真实版本，必须同时记录构建源码哈希、镜像 ID 和运行容器 ID。

本轮包含四个边界明确、可以独立测试和回滚的实施单元：

1. 超值套餐余量语义和历史 `amount_total=0` 数据修复；
2. 超值套餐分组倍率真正进入预扣、结算和日志；
3. Caddy PID 泄漏、LDXP 空队列日志风暴及部署卫生；
4. new-api 与 sub2api 的 GPT-5.6 精确支持。

四个单元共享一次发布流程，但不共享不可逆状态。Caddy、应用镜像、倍率配置、套餐数据迁移和模型 channel 配置分别备份、验证和回滚。

## 2. 目标

### 2.1 超值套餐

- 日卡用户能看到 5 小时余量和 1 天总余量。
- 周卡用户能看到 5 小时余量和 7 天总余量；7 天总量不刷新，不能再显示“不适用”。
- 月卡用户能看到 5 小时余量、当前 7 天阶段余量和 30 天总余量。
- 页面展示、API 返回、预扣校验和最终结算使用同一份订阅额度事实。
- 新购买和同套餐续期都保存正确的总额度。
- 历史活跃超值套餐在不突然中断服务的前提下迁移到有限、可见、可执行的总额度。

### 2.2 分组计价

- 管理后台保存失败必须被识别，不能出现部分保存后仍提示成功。
- 普通钱包计费继续使用当前全局/用户组倍率规则。
- 普通订阅继续保持历史 `1x`，避免本轮无意改变现有产品承诺。
- 超值套餐可以通过“套餐计费组 × 实际 token group”显式配置倍率。
- 未显式配置的超值套餐组合继续使用 `1x`。
- 预扣、流式结算、非流式结算、任务结算和日志中的倍率一致。

### 2.3 稳定性

- Caddy 不再因健康检查子进程成为 zombie 而耗尽 PID。
- LDXP Worker 在没有待处理任务时安静等待，不再每约 2 秒记录一次错误。
- 部署产物能证明来自哪个 Git 提交，不携带 `.git` 伪指针和 macOS AppleDouble 文件。

### 2.4 GPT-5.6

- new-api 和 sub2api 正确认识 `gpt-5.6`、`gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-5.6-luna`。
- `gpt-5.6` 明确等价于 `gpt-5.6-sol`。
- 静态 fallback 价格和生产动态价格与官方价格一致。
- 未知 GPT-5.6 变体不能静默降级为 `gpt-5.4`。
- 代码支持与生产公开模型分离：只有实际 smoke test 通过的模型才加入生产 channel。

## 3. 非目标

- 不删除、替换或改写 new-api / QuantumNous 的标识、归属、模块路径或元数据。
- 不重构整个订阅系统或引入新的通用配额 DSL。
- 不改变 5 小时窗口和月卡锚定 7 天阶段窗口已经确认的时间语义。
- 不把普通订阅从历史 `1x` 切换为用户组倍率。
- 不自动为现有生产倍率表写入非 `1x` 套餐倍率。
- 不在本轮重构 `gpt-image-2` 的 Chat Completions/Images 路由；该问题单独记录。
- 不引入 GPT-5.6 Pro、multi-agent、PTC、显式缓存控制或大规模 Responses API 改造。
- 不把生产密钥、环境变量、数据库备份或用户明细提交到 GitHub。

## 4. 已确认的生产证据

### 4.1 套餐数据

生产计划当前主要额度为：

| 套餐 | `total_amount` | `limit_5h_amount` | `limit_7d_amount` |
| --- | ---: | ---: | ---: |
| 日卡 | 24,000,000 | 9,000,000 | 0 |
| 周卡 | 45,000,000 | 9,000,000 | 0 |
| 月卡 | 220,000,000 | 9,000,000 | 55,000,000 |

运行语义已经是：

- `total_amount`：套餐生命周期总额度；
- `limit_5h_amount`：所有超值套餐的 5 小时短窗口；
- `limit_7d_amount`：月卡的 7 天阶段额度，不是周卡总额度。

生产当时有 14 个活跃周卡订阅、4 个活跃月卡订阅，没有活跃日卡样本。活跃周卡/月卡的 `user_subscriptions.amount_total` 均为 `0`，运行 API 因而返回 `total_limit=0` 和 `total_remaining=0`。部分周卡 `amount_used` 已超过计划的 45,000,000，证明历史 `0` 实际上让总额度校验失效。

### 4.2 分组倍率

管理员保存的 `GroupRatio` 和 `GroupGroupRatio` 已进入数据库并被运行进程读取。钱包请求能使用配置倍率，超值套餐日志却始终记录：

```text
value_package_effective_ratio = 1
```

根因是 `service/billing_ratio.go` 的 `subscriptionBillingGroupRatio = 1.0` 覆盖所有订阅资金来源，而不是前端缓存未刷新。

### 4.3 Caddy 与 LDXP

调查时 Caddy 容器约有 9,469 个 zombie，`pids.current=9482`、`pids.max=9483`，健康检查已出现：

```text
/bin/sh: can't fork: Resource temporarily unavailable
```

Caddy 是容器 PID 1，容器没有 init/reaper；每 30 秒的 `CMD-SHELL` 健康检查长期产生未回收子进程。

LDXP Worker 24 小时约 42,718 行错误，主要是正常空队列被后端返回为 `success:false / record not found`。Worker 已经把 HTTP 404 定义为“暂无任务”。

### 4.4 GPT-5.6

new-api 静态模型定义只完整覆盖到 GPT-5.5。生产 option 曾通过价格同步部分加入 `gpt-5.6`/`gpt-5.6-sol`，但 alias 与 sol 配置不完全一致，也没有 terra/luna。

sub2api 生产动态价格文件已有四个 GPT-5.6 名称，但运行源码没有对应 alias、transform 和白名单；未知 `gpt-5*` 会落入 GPT-5.4 fallback，存在静默模型降级风险。

## 5. 设计决策总览

采用已批准的“针对根因端到端修复”方案：

| 主题 | 决策 |
| --- | --- |
| 周卡展示 | 7 天值来自订阅生命周期总额度，不来自 `limit_7d_amount`。 |
| 历史套餐 | 采用一次性用户友好迁移：`new amount_total = amount_used_at_migration + plan.total_amount`。 |
| 同套餐续期 | 延长 `end_time`，同时累加本次计划 `total_amount`，必须与订单幂等性处于同一事务。 |
| 普通订阅倍率 | 保持历史 `1x`。 |
| 超值套餐倍率 | 显式读取 `GroupGroupRatio[ValuePackageBillingGroup][UsingGroup]`，缺失或无效时为 `1x`。 |
| ratio 保存 | 后端提供受限、事务化的组合保存；前端检查 HTTP 与业务成功状态。 |
| Caddy | 通过生产 compose override 设置 `init: true` 并重建容器。 |
| LDXP 空队列 | `gorm.ErrRecordNotFound` 映射为 HTTP 404；其他错误保持失败。 |
| GPT-5.6 | 精确模型表、精确定价、未知变体拒绝；不使用 GPT-5.4 兜底。 |

## 6. 超值套餐余量设计

### 6.1 字段职责保持不变

不新增额度数据库列：

| 字段 | 日卡 | 周卡 | 月卡 |
| --- | --- | --- | --- |
| `UserSubscription.AmountTotal` | 1 天总额度 | 7 天总额度 | 30 天总额度 |
| `UserSubscription.AmountUsed` | 生命周期已用 | 生命周期已用 | 生命周期已用 |
| `SubscriptionPlan.Limit5hAmount` | 5 小时窗口 | 5 小时窗口 | 5 小时窗口 |
| `SubscriptionPlan.Limit7dAmount` | 忽略 | 忽略 | 当前锚定 7 天阶段窗口 |

生命周期剩余统一计算为：

```text
remaining = max(amount_total - amount_used - in_flight_reservation, 0)
```

如果现有响应只统计已结算值，则保留现有并发预留语义；本轮不能另造一个与实际拦截不同的前端算法。

### 6.2 规范化周期响应

用户端状态接口和后台用量接口由同一个后端 builder 生成规范化周期数组，并保留旧字段以兼容 classic frontend 和旧客户端：

```json
{
  "period_limits": [
    {
      "kind": "five_hour",
      "label_unit": "hour",
      "label_value": 5,
      "limit": 9000000,
      "used": 1000000,
      "remaining": 8000000,
      "refreshes": true,
      "reset_at": 1780000000
    },
    {
      "kind": "lifecycle",
      "label_unit": "day",
      "label_value": 7,
      "limit": 45000000,
      "used": 12000000,
      "remaining": 33000000,
      "refreshes": false,
      "reset_at": 0
    }
  ]
}
```

数组规则：

- 日卡：`five_hour` + `lifecycle(1 day)`；
- 周卡：`five_hour` + `lifecycle(7 day)`；
- 月卡：`five_hour` + 可选 `seven_day_stage` + `lifecycle(30 day)`；
- 月卡 `limit_7d_amount=0` 时不返回 `seven_day_stage`；
- 日卡、周卡即使旧计划误存了非零 `limit_7d_amount`，也不返回阶段 7 天项；
- `refreshes=false` 的生命周期项不显示倒计时或“即将刷新”，只显示到期时间之外的剩余额度；
- 额度为 0 是真实数值，不等于“不适用”；只有该周期能力不存在时才省略对应项。

后端返回原始数值和结构化单位，前端负责 i18n 文案，不根据套餐标题猜类型。

### 6.3 前端展示

`web/default` 使用 `period_limits` 渲染，不再用 `shouldExposeValuePackage7dPeriodLimit()` 把“7 天”统一解释成月卡阶段限额。

展示顺序固定：短周期 → 阶段周期 → 生命周期。用户端卡片和后台列表共用周期 label/format helper：

```text
日卡：5 小时剩余 | 1 天总剩余
周卡：5 小时剩余 | 7 天总剩余
月卡：5 小时剩余 | 当前 7 天阶段剩余 | 30 天总剩余
```

旧 API 响应没有 `period_limits` 时保留兼容 adapter，但 adapter 只能使用 API 中的订阅快照值，不能用当前 plan 总额度伪造剩余。否则会出现“前端显示有限、后端实际无限”。

所有新增标签进入 `web/default/src/i18n/locales/{en,zh,fr,ru,ja,vi}.json`，并运行 `bun run i18n:sync`。不得把中文直接写死在组件中。

### 6.4 计划校验

新建或编辑超值套餐时：

- `total_amount` 必须大于 0；
- `limit_5h_amount` 必须大于等于 0；
- 日卡/周卡运行时忽略 `limit_7d_amount`，管理表单隐藏或清零该字段；
- 月卡允许 `limit_7d_amount=0` 表示没有阶段限额；
- 月卡非零 7 天阶段额度不得大于 30 天总额度；
- 后端执行同样校验，不能只依赖前端。

### 6.5 新购买与续期

新购买创建订阅时继续快照：

```text
AmountTotal = plan.TotalAmount
AmountUsed  = 0
```

同套餐续期必须在订单完成的幂等事务内执行：

```text
EndTime     = max(existing.EndTime, now) + purchased_duration
AmountTotal = existing.AmountTotal + purchased_plan.TotalAmount
```

如果同一支付回调重放，不能重复延长时间或重复增加额度。计划后续被管理员改价/改额度，不追溯修改已经购买的订阅快照。

不同套餐升级/排队激活继续遵循现有订单状态机；本轮不借机重写升级规则。

## 7. 历史套餐迁移设计

### 7.1 目标集合

迁移只选择执行时满足全部条件的记录：

```text
subscription.status = active
subscription.end_time > migration_now
subscription.amount_total = 0
plan.plan_kind = value_package
plan.total_amount > 0
plan.package_type in (day, week, month)
```

已过期、已取消、普通订阅、计划缺失或计划总额度非正数的记录不写入，并在 dry-run 报告中分类计数。

### 7.2 B2 用户友好语义

每条目标订阅在事务中读取/锁定后计算：

```text
old_total = 0
used_at_migration = current amount_used
grant = current plan.total_amount
new_total = used_at_migration + grant
```

因此部署后真实可用余额正好等于一次完整套餐总额度，原 `end_time` 不改变，5 小时和月卡 7 天阶段窗口也不重置。此兼容赠送只发生一次；之后续期按第 6.5 节累加正常购买额度。

该选择优先避免因修复历史 bug 使活跃用户突然停用。它不声称重建历史契约余额，而是明确记录为一次性迁移赠送。

### 7.3 工具与幂等性

迁移实现为仓库内、使用 GORM 的维护命令，复用项目数据库初始化和模型，不内嵌生产 DSN。命令支持：

```text
dry-run（默认）：只读，输出摘要和脱敏清单
apply：要求提供刚生成的 manifest 哈希和明确确认参数
```

要求：

- 使用 GORM 事务和参数化条件，兼容 SQLite、MySQL、PostgreSQL；
- 不直接使用数据库专属 JSON、锁或 update 表达式而没有 fallback；
- apply 再次检查每行仍为 active、未过期、`amount_total=0`；
- 对每行使用当前锁内 `amount_used` 计算 `new_total`，避免 dry-run 后的新消费导致余额偏少；
- 重复 apply 时已迁移行不再匹配，结果为 0，不重复赠送；
- 任一目标行更新失败则事务回滚；
- manifest 不包含用户名、邮箱、token 或密钥，只记录订阅 ID、计划 ID、套餐类型、旧/新额度、迁移时间和结果；
- 生产 apply 前另存数据库原始目标行，回滚按每行原值恢复 `amount_total`，不使用“统一减去 plan total”的猜测算法。

### 7.4 迁移与发布顺序

1. 备份目标行和数据库；
2. 用待发布提交运行 dry-run，核对目标数、各套餐数、计划异常数和新增总额度；
3. 构建/验证待发布应用；
4. 执行带 manifest 哈希的 apply；
5. 立即查询每行 `amount_total - amount_used = grant`；
6. 发布使用规范化周期响应的应用；
7. 从真实 API 抽样验证日/周/月语义。

如果应用发布失败，可以回滚应用并恢复备份的 `amount_total`；`amount_used` 不回退，避免抹掉真实消费。

## 8. 分组计价设计

### 8.1 术语

- `UsingGroup`：本次请求实际选择的 token/channel group，例如 `gpt-plus`。
- `ValuePackageBillingGroup`：当前超值套餐计划的计费组，例如 `day-card`、`week-card`、`month-card`。
- `GroupGroupRatio[parent][child]`：父计费身份对实际 group 的显式倍率。

### 8.2 倍率解析

倍率 resolver 只做一件事：根据资金来源返回最终 group ratio。

```text
wallet:
  保持现有用户组/实际 group 解析结果

regular subscription:
  1.0

value package:
  if GroupGroupRatio[ValuePackageBillingGroup][UsingGroup] exists
     and ratio is finite and ratio > 0:
       configured ratio
  else:
       1.0
```

不能使用 `0` 表示免费；免费语义继续由现有专门字段处理。NaN、Infinity、负值或 0 在保存时被拒绝，在运行时防御性回退 `1x` 并记录限频 warning。

resolver 不从套餐标题推导 group，也不使用普通用户的 `User.Group` 覆盖 `ValuePackageBillingGroup`。

### 8.3 单一结算快照

请求选定资金来源后立即解析一次倍率，并把以下数据冻结到现有 `RelayInfo`/billing snapshot：

```text
billing source
value package subscription id
value package plan id
value package billing group
using group
effective group ratio
ratio source: configured | default_1x | regular_subscription_1x
```

预扣、实时 reserve、非流式结算、流式结算、WebSocket、异步任务重算和退款都使用冻结值，不在响应结束时重新读取可能已经变化的全局配置。

对于 `tiered_expr`，表达式仍按 `pkg/billingexpr/expr.md` 计算 group 前成本；冻结倍率只在表达式结果之后乘一次。不得把 group ratio 隐式塞入表达式系数，也不得同时在表达式和结算层重复相乘。

日志继续保留原始钱包倍率，并新增或规范化：

```text
value_package_billing_group
value_package_effective_ratio
value_package_ratio_source
original_group_ratio
```

### 8.4 管理后台数据源

ratio 页面可配置的父组集合为：

```text
现有普通用户组
∪ 当前所有超值套餐计划的非空 ModelGroup
```

因此 `day-card/week-card/month-card` 不要求伪装成普通 `GroupRatio` child 才能出现在编辑器中。页面明确标记“套餐计费组”，未配置单元格显示“默认 1x”，但不自动向数据库写入大量 1。

### 8.5 原子保存

default frontend 使用一个受管理员权限保护的 ratio 组合保存接口。请求只接受白名单键：

```text
GroupRatio
GroupGroupRatio
```

处理顺序：

1. 用 `common.Unmarshal*` 解析并规范化两个 map；
2. 验证键名、有限数值和正数约束；
3. 在 GORM 事务中更新两条 option；
4. 事务成功后更新内存 setting cache；
5. 如果 cache 更新出现意外错误，立即从数据库重新加载，响应失败并记录错误；
6. 返回服务器重新读取后的规范化快照。

接口整体成功或整体失败，不能只保存前半部分。旧的单 option API 保留兼容，但本页面不再逐项调用它。

前端 mutation 必须同时检查：

```text
HTTP 2xx
response.success === true
返回快照与提交的 normalized maps 一致
```

只有三项均成立才更新 `groupNormalizedDefaults.current` 并显示成功 toast。失败时保留用户编辑内容，提供重试，不伪装成成功。

## 9. 运行稳定性设计

### 9.1 Caddy PID 回收

生产本地 compose 含环境相关配置，不整体提交。仓库新增一个不包含秘密的 production override：

```yaml
services:
  caddy:
    init: true
```

部署使用：

```text
docker compose -f docker-compose.prod.yml -f <tracked-caddy-override> config
docker compose -f docker-compose.prod.yml -f <tracked-caddy-override> up -d --force-recreate caddy
```

`init: true` 让 Docker 注入 init/reaper，回收健康检查退出进程。第一阶段保留现有 Host header 健康检查，避免同时修改两项变量；只有在 init 验证后仍异常，才单独调整 healthcheck。

验证至少跨越两个健康检查周期：

- 新容器中 PID 1 的父子关系正确；
- zombie 为 0 或不增长；
- `pids.current` 稳定；
- 容器变为 healthy；
- 站点首页、`/api/status`、`/api/notice` 和 sub2api 反代均为 200；
- 证书、Caddy 配置和持久卷挂载未变化。

### 9.2 LDXP 空队列

`WorkerClaimLdxpTopupSession` 仅在：

```go
errors.Is(err, gorm.ErrRecordNotFound)
```

时返回 HTTP 404，无错误日志。Worker 现有 backend client 把 404 转为 `null`，按原轮询间隔等待。

参数错误、鉴权错误、数据库连接错误、事务错误仍保留原状态和错误日志，不能被吞成空队列。付费观察 claim 路径只有在测试证明存在相同空队列噪声时才采用同样语义，不做无证据扩张。

### 9.3 部署卫生

发布包通过显式文件清单或 `rsync --exclude` 生成，至少排除：

```text
.git
.worktrees
._*
.DS_Store
node_modules
临时测试/覆盖率输出
本地 secret/env 文件
```

生产目录不再放置指向开发机绝对路径的 `.git` 文件。部署成功后原子写入 `.yunbay-deploy-sha`，同时记录镜像 digest。AppleDouble 文件只在备份后从应用发布目录清理，不扫描或删除无关用户目录。

## 10. GPT-5.6 支持设计

### 10.1 官方模型与价格事实

支持的正式名称：

```text
gpt-5.6       -> gpt-5.6-sol
gpt-5.6-sol   -> gpt-5.6-sol
gpt-5.6-terra -> gpt-5.6-terra
gpt-5.6-luna  -> gpt-5.6-luna
```

官方标准价格（美元 / 1M token）：

| 模型 | Input | Cached input | Cache write | Output |
| --- | ---: | ---: | ---: | ---: |
| `gpt-5.6` / `gpt-5.6-sol` | 5.00 | 0.50 | 6.25 | 30.00 |
| `gpt-5.6-terra` | 2.50 | 0.25 | 3.125 | 15.00 |
| `gpt-5.6-luna` | 1.00 | 0.10 | 1.25 | 6.00 |

new-api 当前 ratio 体系以 $2 / 1M input 为基准，因此默认值应满足：

| 模型 | ModelRatio | CompletionRatio | CacheRatio | CreateCacheRatio |
| --- | ---: | ---: | ---: | ---: |
| `gpt-5.6` / `sol` | 2.5 | 6 | 0.1 | 1.25 |
| `terra` | 1.25 | 6 | 0.1 | 1.25 |
| `luna` | 0.5 | 6 | 0.1 | 1.25 |

如果运行时价格同步提供更新值，动态配置仍可覆盖默认值；alias 与 sol 必须作为同一价格族更新，不能一项新一项旧。

### 10.2 new-api

更新范围：

- 默认模型 ratio、completion ratio、cache read/write ratio；
- GPT-5 completion ratio 的 prefix fallback，先匹配 5.6 精确家族再匹配泛化 GPT-5；
- OpenAI/Codex 相关模型目录或 channel model discovery 的静态列表；
- 管理后台价格展示和同步测试；
- alias 一致性测试。

不把 `gpt-5.6-pro` 添加为模型。Pro 是推理模式，而不是独立模型 slug。

### 10.3 sub2api

选择性移植上游已经验证的模型识别部分：

- OpenAI 模型常量；
- `openai_model_alias.go` 精确 alias；
- `openai_codex_transform.go` 精确目标映射；
- backend/frontend 模型白名单；
- billing/pricing service 的四个静态 fallback；
- 相应单元测试。

不能照搬上游把 5.6 fallback 到 5.4 的错误价格。动态价格缺失时使用第 10.1 节绝对价格；模型名称未知时返回明确的 unsupported model 错误。

特别是：

```text
gpt-5.6-pro
gpt-5.6-unknown
```

不得通过泛化 `gpt-5* -> gpt-5.4` 路径静默成功。

### 10.4 生产 channel 暴露

代码支持不自动等于生产可用。部署后按以下顺序操作：

1. 不改 channel 列表先部署代码；
2. 对 alias/sol 做最小非流式 smoke test，确认实际响应模型和日志；
3. 分别探测 terra、luna；
4. 只有成功的模型才加入目标 channel 的 model allowlist；
5. 再通过 yunbay 公共入口做一次极小请求；
6. 失败时只回滚该模型的 channel 暴露，不回滚其他已验证模型。

smoke test 记录请求 ID、入口模型、sub2api 规范化模型、上游响应模型和计价条目，不记录请求正文、账户凭据或 token。

### 10.5 官方参考

- <https://developers.openai.com/api/docs/guides/latest-model.md>
- <https://developers.openai.com/api/docs/guides/upgrading-to-gpt-5p6-sol.md>
- <https://developers.openai.com/api/docs/pricing.md>

## 11. 错误处理与可观测性

### 11.1 套餐

- 计划总额度非法：保存时返回结构化 validation error。
- 历史订阅总额度为 0：迁移前在管理端标记数据异常，不能用 plan 值静默伪装。
- 总额度耗尽：继续使用现有额度不足错误类型，并在响应中携带真实生命周期周期项。
- 续期事务失败：订单不得被标记为已完成，不能只延长时间或只增加额度。

### 11.2 倍率

- 保存验证失败：两条 option 均不改变。
- 运行期缺失配置：默认 `1x`，日志写 `ratio_source=default_1x`。
- 运行期非法配置：防御性 `1x` + 限频 warning；管理 API 仍返回配置异常，促使修复。

### 11.3 模型

- 正式 GPT-5.6：精确映射。
- 未知 GPT-5.6 变体：明确 4xx unsupported model，不重试到 5.4。
- 上游账户不具备模型权限：保留上游错误并不公开该 channel 模型。

## 12. 测试设计

所有 bugfix 先写失败测试并确认 RED，再做最小实现确认 GREEN。

### 12.1 套餐后端

- 日卡周期数组只有 5h + 1d lifecycle；
- 周卡周期数组只有 5h + 7d lifecycle，7d `refreshes=false`；
- 月卡含 5h + 7d stage + 30d lifecycle；
- 月卡没有阶段限额时省略 7d stage；
- `amount_total=0` 不伪造成 plan total；
- 新购快照 total；
- 同计划续期同时延时和累加 total；
- 重放完成回调不重复累加；
- 迁移 dry-run 不写库；
- apply 只改目标集合、使用锁内 `amount_used`、重复执行为 0；
- SQLite 单元测试覆盖迁移行为；实现只使用三种数据库共有的 GORM API、不包含方言 raw SQL，现有数据库矩阵继续验证 MySQL/PostgreSQL，生产 dry-run/apply 另验证实际 PostgreSQL 路径。

### 12.2 套餐前端

- 日/周/月渲染的 label、顺序、剩余值和刷新文案；
- 周卡不再出现“不适用”；
- 生命周期 0 剩余显示 0，不显示“不适用”；
- 旧 API adapter 不使用 plan total 伪造余额；
- 六种 locale key 完整；
- TypeScript typecheck 和 production build。

### 12.3 倍率

表驱动覆盖：

| 资金来源 | 显式套餐配置 | 期望 |
| --- | --- | ---: |
| wallet | 不适用 | 现有解析值 |
| regular subscription | 任意 | 1.0 |
| value package | 有效值 0.3 | 0.3 |
| value package | 缺失 | 1.0 |
| value package | 非法值 | 1.0 + warning |

同时覆盖 ratio 计价与 `tiered_expr`、预扣与结算、流式/非流式、任务重算和退款。断言倍率只乘一次，日志快照一致。

组合保存接口覆盖：全部成功、第一项验证失败、第二项验证失败、数据库第二次写失败时整体回滚、HTTP 200 + `success:false` 的前端处理、成功后 normalized baseline 更新。

### 12.4 稳定性

- controller 无候选任务返回 404；
- 真实数据库错误不是 404；
- Worker backend client 对 404 返回 `null` 且不 warning；
- `docker compose ... config` 验证 Caddy override 合并后 `init: true`；
- 线上以两个以上健康检查周期验证 PID 不增长。

### 12.5 GPT-5.6

- new-api 四个名字的默认 ratio 精确值；
- alias/sol 完全一致；
- terra/luna 不落到泛化 GPT-5 错价；
- sub2api alias、transform、backend/frontend whitelist；
- 动态价格存在与缺失两种路径；
- 未知 5.6 变体明确失败；
- smoke test 证明实际转发模型没有降级。

### 12.6 验证命令范围

至少执行：

```text
根 Go 相关包测试 + go test ./...
web/default: bun run i18n:sync、typecheck、相关测试、production build
LDXP Worker: bun run check
compose override: docker compose config
```

当前主仓库没有完整 sub2api backend `go.mod` 和 frontend manifest，不能把“manifest 缺失所以跳过”当作验证。实现计划必须建立一个记录来源提交/哈希的完整临时 sub2api 树，覆盖本仓库定制文件后运行：

```text
backend: go test ./...
frontend: bun install --frozen-lockfile、相关测试、build
```

生产构建必须使用同一份已测试完整树。

## 13. 发布、备份与回滚

### 13.1 发布前备份

备份到带时间和待发布 SHA 的目录：

- 当前 production compose 与 Caddy 配置；
- 当前 new-api/sub2api/Worker 镜像 digest 和容器 inspect；
- option 中 `GroupRatio`、`GroupGroupRatio` 和 GPT 价格相关键；
- 目标 `user_subscriptions` 与对应 `subscription_plans` 行；
- 目标 channel 的模型 allowlist；
- 当前 `.yunbay-deploy-sha` 和公开接口基线。

备份文件权限收紧，不进入 Git。

### 13.2 分阶段发布

```text
Phase 1: 单独重建 Caddy（init=true）并验证 PID/HTTP
Phase 2: 运行套餐迁移 dry-run，人工核对摘要
Phase 3: 构建并验证 new-api/default frontend/sub2api
Phase 4: 执行带 manifest 哈希的套餐迁移 apply
Phase 5: 发布 new-api，验证套餐与倍率默认 1x
Phase 6: 发布 sub2api，先不扩 channel allowlist
Phase 7: GPT-5.6 分模型 smoke 与按成功结果暴露
Phase 8: 观察日志、额度和 PID，再更新 deploy SHA
```

每个 phase 有独立停止条件；上一阶段未通过时不进入下一阶段。

### 13.3 回滚

- Caddy：恢复原 compose 文件/override 并重建；如果 `init: true` 已证明稳定，可作为安全修复保留。
- new-api/sub2api：按 digest 恢复上一镜像，不从可变 tag 猜版本。
- 套餐数据：按备份逐行恢复旧 `amount_total`；不回退 `amount_used`。
- ratio：恢复两条 option 后触发进程重载并通过管理 API 核对。
- channel：恢复原 allowlist。
- 部署标记：只有运行态已回到旧版本后才写回旧 SHA。

## 14. 验收标准

### 14.1 Bug 1

- 真实周卡管理/用户 API 返回 7 天 lifecycle limit、used、remaining，`refreshes=false`。
- default frontend 显示“7 天总剩余”，不显示“不适用”。
- 日卡测试和构造样本显示 5 小时 + 1 天总剩余。
- 月卡显示 5 小时 + 当前 7 天阶段 + 30 天总剩余。
- 抽样迁移订阅的 `amount_total - amount_used` 等于该计划一次完整 `total_amount`。
- 新续期订单只累加一次时间和额度。

### 14.2 Bug 2

- 默认未配置时超值套餐仍为 `1x`。
- 设置 `week-card -> gpt-plus = X` 后，新请求预扣、结算和日志均为 X。
- 其他套餐/实际 group 不受该单元格影响。
- 普通订阅仍为 `1x`，钱包仍按原倍率。
- 模拟第二项保存失败时数据库两项均保持原值，前端不提示成功。

### 14.3 Bug 3

- Caddy zombie 清零且跨多个健康检查周期不增长。
- Caddy healthy，四个公开基线入口正常。
- LDXP 空闲轮询不再产生 `record not found` error 风暴。
- 部署目录没有 `.git` 伪指针和 `._*` 发布垃圾，部署 SHA 与镜像源码一致。

### 14.4 Bug 4

- 四个 GPT-5.6 名称在 new-api/sub2api 均有精确映射和正确价格。
- `gpt-5.6` 与 `gpt-5.6-sol` 行为、价格一致。
- 未知 5.6 变体不会运行成 5.4。
- 生产只公开实际 smoke 成功的模型，日志能证明实际上游模型。

## 15. 实施边界与后续项

本 spec 通过后，实施计划按上述四个单元拆成可独立审查的 TDD 批次，但使用一个最终发布分支。以下发现只登记，不进入本轮实现：

- `gpt-image-2` 被错误送入 Codex ChatGPT Chat Completions 路径；
- 生产备份目录长期容量策略；
- 对所有普通订阅重新启用用户组倍率；
- 彻底替换 Caddy shell healthcheck。

这些事项需要各自的运行证据、产品语义和回滚设计，不能作为本轮“顺手修改”。
