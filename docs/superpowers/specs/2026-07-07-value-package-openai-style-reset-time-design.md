# Value Package OpenAI-Style Rolling Reset Time Design

Date: 2026-07-07
Status: Draft approved for implementation planning
Scope: 云贝日卡、周卡、月卡的 5 小时 / 7 天滚动流量桶恢复时间展示

## 1. 背景

云贝日卡、周卡、月卡已经有两个滚动流量桶：

- 短周期桶：最近 5 小时内累计 Codex / value-package 用量。
- 长周期桶：最近 7 天内累计 Codex / value-package 用量。

当前系统已经在后端记录 `value_package_usage_records`，并按滚动窗口计算：

```text
used_5h = sum(quota where created_at >= now - 5h)
used_7d = sum(quota where created_at >= now - 7d)
```

限流规则保持不变：

```text
limit_5h > 0 && used_5h >= limit_5h -> 5 小时桶限流
limit_7d > 0 && used_7d >= limit_7d -> 7 天桶限流
```

用户提出的产品目标是：

> 1:1 模仿 OpenAI Plus / Codex 的用量提示，不主展示“下一笔部分恢复多少”，而是显示“还有多久完全恢复 / resets in ...”。

因此，本设计只完善展示和响应字段，不改变计费倍率、模型分组、套餐身份切换、套餐购买、兑换码、订单扫描等已完成逻辑。

## 2. 非目标

本次不做以下事情：

- 不改变 5 小时 / 7 天限流算法。
- 不改为固定周期重置；仍然是滚动窗口。
- 不每天 0 点清空。
- 不按用户当天第一次使用时间创建固定周期。
- 不改模型分组；日卡 / 周卡 / 月卡仍只影响套餐身份和套餐计费，不覆盖 Plus / Pro 路由组。
- 不新增复杂预测，例如“下一笔可恢复多少额度”。
- 不修改 LDXP 支付链路。

## 3. 术语

### 3.1 当前窗口用量

对于窗口 `window_seconds`：

```text
window_start = now - window_seconds
window_usage = usage records where created_at >= window_start
used = sum(window_usage.quota)
```

### 3.2 OpenAI 风格完全恢复时间

本设计中的“恢复时间 / reset time”指：

> 如果用户从现在开始不再继续消耗，该窗口内当前所有用量都滚出窗口的时间。

计算：

```text
reset_at = max(created_at in current window) + window_seconds
reset_seconds = max(0, reset_at - now)
```

也就是说：

- 5 小时桶：`reset_at_5h = 当前 5 小时窗口内最新 usage.created_at + 5h`。
- 7 天桶：`reset_at_7d = 当前 7 天窗口内最新 usage.created_at + 7d`。

这不是“下一笔部分额度恢复时间”。下一笔部分恢复应该是窗口内最早 usage 离开窗口的时间，但本次不主展示它。

## 4. 示例

假设短周期额度是 60 单位 / 5 小时：

| 时间 | 操作 | 最近 5 小时累计 |
| --- | ---: | ---: |
| 09:00 | 用了 20 | 20 |
| 10:30 | 用了 25 | 45 |
| 12:00 | 用了 15 | 60 |
| 13:00 | 再用 | 可能被限流 |

在 13:00：

```text
used_5h = 60
limit_5h = 60
latest_usage_created_at = 12:00
reset_at_5h = 12:00 + 5h = 17:00
reset_seconds_5h = 4h
```

UI 主展示：

```text
5 小时额度已用完
将在 4 小时后恢复
```

注意：14:00 时，09:00 那批 20 会先离开窗口，用户可能恢复一部分可用空间；但 OpenAI 风格 banner 主表达仍是“当前窗口完全恢复时间”，不是部分恢复提醒。

## 5. 后端设计

### 5.1 数据来源

继续使用现有 `value_package_usage_records` 表，不新增表。

每条用量记录至少依赖：

- `user_id`
- `user_subscription_id`
- `plan_id`
- `quota`
- `created_at`
- `request_id`

### 5.2 现有 summary 字段

现有 `ValuePackageUsageSummary` / API response 中已有字段保持兼容：

```json
{
  "used_amount": 0,
  "remaining_amount": 0,
  "total_remaining": 0,
  "used_5h": 0,
  "limit_5h": 0,
  "percent_5h": 0,
  "used_7d": 0,
  "limit_7d": 0,
  "percent_7d": 0,
  "exhausted_reason": "",
  "exhausted_message": ""
}
```

### 5.3 新增 summary 字段

在后端 usage summary 中新增：

```json
{
  "reset_at_5h": 0,
  "reset_seconds_5h": 0,
  "reset_at_7d": 0,
  "reset_seconds_7d": 0,
  "limited_5h": false,
  "limited_7d": false
}
```

字段含义：

| 字段 | 类型 | 含义 |
| --- | --- | --- |
| `reset_at_5h` | Unix seconds | 5 小时桶完全恢复时间。无窗口内用量或不限量时为 0。 |
| `reset_seconds_5h` | number | 距离 5 小时桶完全恢复还剩几秒。无窗口内用量或不限量时为 0。 |
| `reset_at_7d` | Unix seconds | 7 天桶完全恢复时间。无窗口内用量或不限量时为 0。 |
| `reset_seconds_7d` | number | 距离 7 天桶完全恢复还剩几秒。无窗口内用量或不限量时为 0。 |
| `limited_5h` | boolean | `limit_5h > 0 && used_5h >= limit_5h`。 |
| `limited_7d` | boolean | `limit_7d > 0 && used_7d >= limit_7d`。 |

### 5.4 计算规则

伪代码：

```go
func computeReset(now, windowSeconds, limit, used int64, latestCreatedAt int64) (resetAt, resetSeconds int64, limited bool) {
    limited = limit > 0 && used >= limit

    if limit <= 0 {
        return 0, 0, false // 不限量，不显示 reset
    }
    if used <= 0 || latestCreatedAt <= 0 {
        return 0, 0, false // 当前窗口无用量，已完全恢复
    }

    resetAt = latestCreatedAt + windowSeconds
    resetSeconds = resetAt - now
    if resetSeconds < 0 {
        resetSeconds = 0
    }
    return resetAt, resetSeconds, limited
}
```

为了避免多次 DB round-trip，后端应在计算 `sum(quota)` 时同时计算窗口内最新 `created_at`：

```text
SUM(quota), MAX(created_at)
```

必须保持 SQLite / MySQL / PostgreSQL 兼容。可优先使用 GORM 的 `Select("COALESCE(SUM(quota),0) as used, COALESCE(MAX(created_at),0) as latest_created_at")`，避免数据库方言特有语法。

### 5.5 API 覆盖范围

需要返回 reset 字段的接口：

1. 用户端套餐状态接口
   - `GET /api/value-packages/self`
   - 用于用户在日卡 / 周卡 / 月卡卡片、状态 banner 中看到自己的 5h / 7d 用量和恢复时间。

2. 用户切换套餐接口返回
   - `POST /api/value-packages/activate`
   - 切换后应立即返回最新 usage summary，包括 reset 字段。

3. 管理员订单管理套餐用量接口
   - `GET /api/order-management/admin/value-package-usage`
   - 用于管理员表格查看每个日卡 / 周卡 / 月卡用户的 5h / 7d 用量和完全恢复时间。

4. 限流错误响应文案
   - 当中间件因 5h / 7d 桶限流时，错误文案应附带 reset 提示。
   - 示例：
     ```text
     超值套餐额度已用完（5 小时：已用 $60 / $60，将在 2 小时 41 分钟后恢复）
     ```

## 6. 前端设计

### 6.1 类型更新

更新以下 TypeScript 类型：

- `web/default/src/features/value-packages/types.ts`
- `web/default/src/features/order-management/types.ts`

新增字段：

```ts
reset_at_5h: number
reset_seconds_5h: number
reset_at_7d: number
reset_seconds_7d: number
limited_5h: boolean
limited_7d: boolean
```

字段名与后端 JSON 保持 snake_case。

### 6.2 格式化函数

新增或复用一个展示函数：

```ts
formatResetTime(seconds: number, t: TFunction): string
```

推荐展示规则：

| 条件 | 文案 |
| --- | --- |
| limit <= 0 | 不限量 |
| used <= 0 或 reset_seconds <= 0 | 已完全恢复 |
| reset_seconds < 60 | 不到 1 分钟后恢复 |
| < 1 小时 | N 分钟后恢复 |
| < 24 小时 | N 小时 M 分钟后恢复 |
| >= 24 小时 | N 天 M 小时后恢复 |

英文基础 key 使用英文句子，i18n 同步到 `en/zh/fr/ja/ru/vi`。

### 6.3 用户端展示

在用户端日卡 / 周卡 / 月卡当前套餐状态区域展示：

```text
5 小时额度
$42 / $60
将在 3 小时 18 分钟后恢复

7 天额度
$180 / $420
将在 4 天 6 小时后恢复
```

如果达到限制：

```text
5 小时额度已用完
$60 / $60
将在 2 小时 41 分钟后恢复
```

如果没有窗口内用量：

```text
5 小时额度
$0 / $60
已完全恢复
```

如果不限量：

```text
5 小时额度
不限量
```

### 6.4 管理员订单管理表格展示

在现有日卡 / 周卡 / 月卡用户实时用量表中增加恢复时间列或在现有用量单元格内补充副文案。

推荐列：

| 用户 | 套餐 | 状态 | 5 小时用量 | 5 小时恢复 | 7 天用量 | 7 天恢复 | 总剩余 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| lanwenfu | 月卡 | 启用 | $42 / $60 | 3 小时 18 分钟后 | $180 / $420 | 4 天 6 小时后 | $xxx |

继续保持 15 秒自动刷新：

```ts
refetchInterval: 15_000
```

### 6.5 不显示“下一笔恢复多少”

本次不在主 UI 显示：

```text
下一次恢复约 $20
```

原因：用户明确要求模仿 OpenAI Plus / Codex 的“完全恢复 / reset time”体验。

## 7. 中间件限流文案

当前中间件已经能识别 5h / 7d 触顶：

```go
if limit5h > 0 && used5h >= limit5h { ... }
if limit7d > 0 && used7d >= limit7d { ... }
```

改造后：

1. 仍先计算 used。
2. 同时计算 reset seconds。
3. 错误文案附加 reset。

示例：

```text
超值套餐额度已用完（5 小时：已用 $60 / $60，将在 2 小时 41 分钟后恢复）
```

如果 reset 无法计算，降级为旧文案：

```text
超值套餐额度已用完（5 小时：已用 $60 / $60）
```

## 8. 测试计划

### 8.1 后端 model tests

新增 / 更新测试：

1. 窗口内多条 usage 时，`reset_at_5h` 取最新 usage + 5h。
2. 窗口内多条 usage 时，`reset_at_7d` 取最新 usage + 7d。
3. 窗口外 usage 不影响 used，也不影响 reset。
4. 无窗口内 usage 时 reset 为 0。
5. limit 为 0 时 reset 为 0，表示不限量。
6. used 达到 limit 时 `limited_5h` / `limited_7d` 为 true。

### 8.2 后端 controller tests

更新：

- `AdminOrderManagementValuePackageUsage` response 包含 reset fields。
- `GetValuePackageState` / activate 返回包含 reset fields。
- 限流错误文案在可计算 reset 时包含恢复提示。

### 8.3 前端 tests

更新或新增：

- value package card/source test 包含 reset 字段和恢复文案。
- order management usage table source test 包含 `reset_seconds_5h` / `reset_seconds_7d` / 恢复列。
- i18n source / sync 测试确保新增 key 同步到 6 个语言文件。

### 8.4 验证命令

后端 focused：

```bash
go test ./model ./middleware ./controller -run 'ValuePackage|OrderManagement' -count=1 -timeout=300s
```

前端 focused：

```bash
cd web/default
bun test \
  src/features/value-packages/components/value-package-card-source.test.ts \
  src/features/order-management/components/value-package-usage-table-source.test.ts \
  src/features/order-management/order-management-source.test.ts
bun run typecheck
bun run i18n:sync
```

必要时跑更大范围：

```bash
go test ./middleware ./service ./model ./controller -count=1 -timeout=300s
cd web/default && bun run build
```

## 9. 成功标准

实现完成后必须满足：

1. 后端仍按最近 5 小时 / 最近 7 天滚动窗口限流。
2. 用户端能看到 5 小时和 7 天桶的已用 / 上限 / OpenAI 风格完全恢复时间。
3. 管理员订单管理的日卡 / 周卡 / 月卡用户实时用量表能看到每个用户的 5 小时和 7 天完全恢复时间。
4. 达到限流时，API 错误文案包含对应窗口的恢复时间。
5. 无用量显示“已完全恢复”。
6. 不限量显示“不限量”。
7. i18n 六语言同步无缺失。
8. 不改变模型分组、不改变套餐默认启用、不影响已完成的 LDXP / 兑换码 / 返利 / 订单删除逻辑。
