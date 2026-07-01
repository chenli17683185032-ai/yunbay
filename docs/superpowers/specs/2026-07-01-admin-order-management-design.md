# 管理员订单管理与邮件核对页面设计

日期：2026-07-01

## 背景

管理员后台需要新增一个订单管理页面，用来集中查看网站成交、联动小铺订单邮件核对、返利归属和返利提现申请。

当前业务中，联动小铺订单会发送购买确认邮件到已配置的运营邮箱。邮件正文包含商品、实付金额、数量、付款时间、小铺单号和购买内容，例如：

```text
感谢购买商品0.1 元测试
实付0.10元
数量:1,
付款时间2026-06-28 03:37:42
单号:LD260628UZJ97P,
以下是您的购买内容:
9470548686742880
```

管理员每天晚上会复查这些邮件。新页面要把复查工作系统化：网站订单已经拿到小铺单号和外部实付金额快照时，系统自动或按管理员点击立即核对邮件；核对通过后在订单行显示邮件核对已通过。

本设计基于前序 LDXP / 联动小铺自动充值设计中的核心数据口径：

- `amount` / 入账金额 / 面值：网站给用户计入的金额或额度对应面值，例如 `500`。
- `money` / 外部实付金额快照：用户在小铺实际支付的金额，例如折扣后 `425`，或手续费由用户承担时 `10.30`。
- 邮件里的 `实付` 字段必须和外部实付金额快照比对，不能和网站入账金额直接比对。

> 重要：当前主工作区可能没有完整 LDXP、返利提现实现文件。实施时必须先对齐生产/LDXP 分支已有模型与接口，避免用本地旧文件覆盖已经上线或灰测的 LDXP 能力。

## 设计目标

1. 管理员能在一个后台页面看到每日、近 7 天、近 30 天和自定义范围的网站成交情况。
2. 管理员能看到成交明细清单，并在每行看到邮件核对状态。
3. 系统自动核对订单的小铺单号和邮件单号、订单外部实付金额和邮件实付金额。
4. 管理员可以点击“立即核对”，让系统马上开始核对当前订单或所有未完成订单，不必等待定时任务。
5. 管理员能看到每笔订单谁拿了返利。
6. 页面底部有返利统计系统，能看到有多少用户有返利，以及用户是否申请提现；如果申请了，可以看到提现申请信息并处理。
7. UI 风格沿用当前 `web/default` 的后台风格：卡片、muted 背景、紧凑数据表、语义色状态徽章、响应式布局。

## 非目标

1. 不做人工强制打勾作为主流程。“立即核对”不是人工直接通过，而是触发系统立刻拉取/扫描邮件并执行自动核对。
2. 不按固定手续费或固定折扣公式从网站入账金额反推出邮件金额。
3. 不将邮件实付金额与网站入账金额直接比较。
4. 不在仓库中写入 QQ 邮箱授权码、IMAP 密码、Worker token、SMTP token 或任何生产密钥。
5. 不改普通支付渠道的既有支付、回调和入账逻辑。Stripe、Waffo、Creem、易支付等没有联动小铺邮件的订单可以展示为“不适用”或在后续版本纳入统一订单台账。
6. 不修改或移除受保护的项目身份、组织身份、版权、品牌、模块路径或许可证信息。

## 信息架构

新增管理员后台菜单：

```text
订单管理
```

建议路由：

```text
/_authenticated/order-management/
```

页面采用上下结构：

```text
订单管理页
├─ 顶部：时间范围与操作区
├─ 图表分析区
│  ├─ KPI 卡片
│  └─ 成交趋势图
├─ 成交明细区
│  └─ 大面积表格 + 立即核对操作
└─ 返利统计区
   ├─ 返利 KPI
   └─ 有返利用户与提现申请表格
```

## 顶部时间范围与操作区

顶部右侧固定展示时间范围切换：

```text
近 7 天 | 近 30 天 | 自定义
```

默认进入页面使用 `近 7 天`。切换到 `近 30 天` 后，顶部 KPI、趋势图、成交明细和返利统计都使用同一个时间范围刷新。

顶部操作按钮：

```text
立即核对未完成订单
拉取最新邮件
导出 CSV
```

按钮含义：

- `立即核对未完成订单`：马上触发系统核对当前筛选范围内未核对、待邮件、异常订单。
- `拉取最新邮件`：马上触发邮件拉取或扫描，不等待后台定时任务。
- `导出 CSV`：导出当前时间范围与筛选条件下的成交明细和核对状态。

## 图表分析区

### KPI 卡片

第一版展示 5 个卡片：

1. `7 天入账` 或当前范围入账金额。
2. `30 天入账`，即使当前选中 7 天，也明确展示 30 天入口或概览，避免管理员看不到月度视角。
3. `外部实付`：小铺/邮件金额口径，即订单 `money` 汇总。
4. `邮件核对通过率`：已通过邮件核对订单数 / 需要邮件核对订单数。
5. `待核对`：当前范围内待邮件、核对中、核对异常的订单数。

金额展示统一使用人民币符号 `¥`，因为该页面服务小铺订单与返利运营，不沿用普通模型计费的 quota / USD 展示口径。

### 成交趋势图

趋势图按天聚合，支持：

- 近 7 天：显示 7 个日点。
- 近 30 天：显示 30 个日点；如果宽度不足，图表内部横向滚动或压缩点间距。
- 自定义：按日期范围显示日聚合；范围过长时仍按日聚合并分页或限制最大导出范围。

图表指标：

1. 网站入账金额：用户实际获得的面值或充值金额。
2. 外部实付金额：小铺/邮件金额快照。
3. 订单数。
4. 邮件核对通过率。

第一版可以用柱状图展示金额，用折线或次轴展示核对率；如果实现复杂，先展示金额双柱和核对率 KPI，后续再加次轴。

## 成交明细区

成交明细位于图表下方，面积要明显大于图表区域，便于管理员看到更多行。

建议表格列：

| 列 | 含义 |
| --- | --- |
| 时间 | 订单创建或支付完成时间 |
| 用户 | 用户 ID、用户名或邮箱简要信息 |
| 网站订单 | 本地 session/order/topup 标识 |
| 入账金额 | 网站给用户计入的面值，例如 `¥500.00` |
| 外部实付 | 创建支付会话时保存的小铺实付快照，例如 `¥425.00` |
| 邮件实付 | 邮件解析出来的 `实付` 金额 |
| 小铺单号 | Worker 或支付会话返回的小铺单号，例如 `LD260628UZJ97P` |
| 邮件核对 | 待邮件、核对中、已核对、金额异常、单号异常等 |
| 操作 | 立即核对、重新核对、查看详情 |
| 返利 | 该订单产生返利时显示返利用户和金额 |

### 邮件核对状态

建议状态：

```text
not_required       不适用
pending            待核对
waiting_mail       待邮件
checking           核对中
verified           已核对
order_mismatch     单号异常
amount_mismatch    金额异常
mail_parse_failed  邮件解析失败
mail_fetch_failed  邮件拉取失败
timeout            核对超时
```

UI 展示建议：

- `已核对`：绿色徽章。
- `核对中`：蓝色徽章，按钮 disabled。
- `待邮件` / `待核对`：橙色徽章。
- 异常：红色徽章，并在行背景使用浅红或浅橙提示。
- `不适用`：灰色徽章。

### “立即核对”行为

“立即核对”不是人工强制通过。点击后系统立刻执行自动核对流程。

单行按钮：

```text
立即核对
重新核对
```

顶部批量按钮：

```text
立即核对未完成订单
```

执行流程：

```text
管理员点击立即核对
        ↓
后端创建或触发核对任务
        ↓
订单状态显示为 checking / 核对中
        ↓
系统立即拉取或扫描最新邮件
        ↓
按小铺单号匹配邮件
        ↓
比对邮件实付金额与订单外部实付金额快照
        ↓
更新订单核对状态
```

单行立即核对只处理当前订单。批量立即核对处理当前筛选范围内的未完成订单，但需要设置一次最多处理数量，避免管理员误点造成长时间 IMAP 扫描或请求超时。

### 核对规则

必须同时满足：

```text
订单小铺单号 == 邮件单号
订单外部实付金额快照 == 邮件实付金额
```

金额比较要求：

1. 使用 decimal 或分为单位的整数比较，不使用 float 直接比较。
2. 比较前统一保留 2 位小数。
3. 邮件金额必须和 `money` / 外部实付快照比较。
4. 不允许用 `amount` / 入账金额与邮件金额比较。
5. 不允许按固定手续费或固定折扣公式临时推导。

示例：

| 入账金额 | 外部实付快照 | 邮件实付 | 结果 |
| --- | ---: | ---: | --- |
| `¥500.00` | `¥425.00` | `¥425.00` | 通过 |
| `¥10.00` | `¥10.30` | `¥10.30` | 通过 |
| `¥10.00` | `¥10.30` | `¥10.00` | 金额异常 |

## 邮件拉取与解析

第一版复用或补齐 LDXP 邮件监听/解析能力。

邮件来源：已配置的运营邮箱，生产密钥通过环境变量、secret 文件或后台加密配置提供，不进入仓库。

解析字段：

| 字段 | 来源 |
| --- | --- |
| 商品名 | `感谢购买商品...` 或 `商品名称` / `购买内容` |
| 实付金额 | `实付10.30元` 或 `支付金额：10.30` |
| 数量 | `数量:1` |
| 付款时间 | `付款时间2026-06-28 03:37:42` |
| 单号 | `单号:LD260628UZJ97P` |
| 购买内容 / 卡密 | `以下是您的购买内容:` 后的 token |

解析要求：

1. 兼容 HTML 邮件和纯文本邮件。
2. 兼容中文冒号、英文冒号、逗号、换行和空格差异。
3. 邮件去重按 `Message-ID`、IMAP UID 和 raw hash。
4. 邮件原文或摘要中的敏感购买内容、卡密、token 在普通表格中脱敏展示；详情中也默认脱敏，必要时仅管理员可查看。
5. 邮件拉取失败只影响核对状态，不影响订单本身入账状态。

## 返利统计区

返利统计区放在页面底部，不只在订单行里展示一个返利字段。

### 返利 KPI

第一版展示：

1. `有返利用户`：存在累计返利记录的用户数。
2. `返利总额`：当前范围或累计口径下返利金额汇总。
3. `已申请提现`：已提交提现申请的人数和金额。
4. `可提现未申请`：有可提现余额但没有待处理提现申请的用户数。

时间范围规则：

- 顶部选择 7 天 / 30 天时，默认返利产生金额按所选时间范围统计。
- 用户累计返利、可提现、已提现可以同时展示累计值；列名必须清楚区分“周期产生”和“累计余额”。

### 返利用户表

建议列：

| 列 | 含义 |
| --- | --- |
| 返利用户 | 邀请人用户 ID、用户名 |
| 周期返利 | 当前时间范围产生的返利 |
| 累计返利 | 历史累计返利 |
| 可提现 | 当前可提现金额 |
| 已提现 | 已打款金额 |
| 提现申请 | 未申请、待处理、已打款、已驳回 |
| 申请信息 | 联系方式、申请金额、申请时间、用户备注、管理员备注 |
| 操作 | 标记已打款、驳回、查看来源订单 |

如果用户申请了提现，行内必须直接显示申请信息，不需要管理员跳转到另一个页面才能判断。

### 返利来源订单

每个返利用户行提供 `查看来源订单`。点击后可以展开或打开详情抽屉，展示返利来自哪些成交订单：

| 字段 | 含义 |
| --- | --- |
| 订单时间 | 成交时间 |
| 被邀请用户 | 产生订单的用户 |
| 订单号 | 本地订单 / 小铺单号 |
| 订单金额 | 外部实付或返利基准金额 |
| 返利比例 | 例如 15% |
| 返利金额 | 该订单给邀请人的返利 |
| 邮件核对 | 该订单是否已邮件核对 |

返利是否可提现应依赖已经确认的返利记录，不应该由前端临时计算。

## 数据来源与模型口径

### LDXP / 小铺订单

优先复用前序 LDXP 数据模型：

```text
LdxpTopupSession
- session_id
- user_id
- amount              网站入账金额/面值
- money               外部实付金额快照
- worker_order_no     Worker 返回小铺单号
- worker_amount       Worker 页面解析金额
- mail_order_no       邮件解析单号
- mail_amount         邮件解析实付金额
- verified_time       核对通过时间
- error_code
- error_message
- topup_id
- redemption_id
```

如果实现分支缺少这些模型，实施前必须先同步/移植 LDXP 模型和迁移，不能重新设计一套不兼容表结构。

### 成交订单

成交统计第一版以 LDXP / 小铺订单为主：

- 成功入账订单用于入账金额统计。
- 已创建但未核对订单用于待核对统计。
- 邮件异常订单用于异常统计。

如果需要把普通 `TopUp`、`SubscriptionOrder` 同时纳入“网站成交”，可以在订单类型列中展示：

```text
ldxp_topup | topup | subscription
```

普通非小铺订单的邮件核对状态显示为 `不适用`。

### 返利与提现

优先复用前序返利模型：

```text
AffiliateCommission
- inviter_user_id
- invitee_user_id
- topup_id
- trade_no
- base_money
- rate
- commission_money
- status
- created_time

AffiliateWithdrawal
- user_id
- amount
- contact
- remark
- status
- admin_remark
- created_time
- processed_time
```

当前主分支如果只有旧的 `aff_quota`、`aff_history_quota`、`aff_count` 字段，则无法完整表达“提现申请信息”。实施计划必须先合并或补齐 `AffiliateCommission` 和 `AffiliateWithdrawal`，并保证历史旧字段不会被破坏。

## 后端 API 设计

建议新增管理员 API 组：

```text
/api/order-management/admin
```

所有接口使用 `middleware.AdminAuth()`。涉及提现处理的接口也可以要求更高权限，按现有项目权限约定决定。

### 获取图表与 KPI

```http
GET /api/order-management/admin/analytics?range=7d
GET /api/order-management/admin/analytics?range=30d
GET /api/order-management/admin/analytics?start_time=...&end_time=...
```

响应包含：

```json
{
  "summary": {
    "site_amount": 8920.0,
    "external_paid_amount": 7611.3,
    "order_count": 188,
    "mail_verified_count": 184,
    "mail_pending_count": 4,
    "mail_error_count": 3,
    "mail_verified_rate": 0.968,
    "affiliate_user_count": 12,
    "affiliate_amount": 612.75,
    "withdrawal_pending_count": 3,
    "withdrawal_pending_amount": 300.0
  },
  "daily": [
    {
      "date": "2026-07-01",
      "site_amount": 1284.0,
      "external_paid_amount": 1127.3,
      "order_count": 31,
      "mail_verified_count": 28,
      "mail_error_count": 3
    }
  ]
}
```

### 获取成交明细

```http
GET /api/order-management/admin/orders?page=1&page_size=20&range=7d&mail_status=pending&keyword=LD260628
```

响应使用项目现有分页结构，items 中包含：

```json
{
  "id": 1,
  "order_type": "ldxp_topup",
  "session_id": "ldxp_xxx",
  "user_id": 1024,
  "username": "user",
  "site_amount": 500.0,
  "external_paid_amount": 425.0,
  "worker_order_no": "LD260628UZJ97P",
  "mail_order_no": "LD260628UZJ97P",
  "mail_paid_amount": 425.0,
  "mail_status": "verified",
  "mail_status_text": "已核对",
  "error_code": "",
  "error_message": "",
  "affiliate": {
    "inviter_user_id": 12,
    "commission_money": 63.75,
    "status": "available"
  },
  "created_time": 1782882180,
  "verified_time": 1782882300
}
```

### 立即核对

单笔订单：

```http
POST /api/order-management/admin/orders/:id/mail-check
```

批量未完成订单：

```http
POST /api/order-management/admin/mail-check
```

请求体：

```json
{
  "range": "7d",
  "scope": "pending",
  "limit": 100
}
```

响应：

```json
{
  "job_id": "mailcheck_...",
  "started": true,
  "affected_count": 7
}
```

任务状态：

```http
GET /api/order-management/admin/mail-check/:job_id
```

如果第一版实现选择同步执行，也必须设置较短超时，并在超时后返回 `started=true` 或 `partial=true`，避免 HTTP 请求长时间挂起。推荐异步任务，因为 IMAP 拉取和邮件解析可能慢或失败。

### 获取返利统计

```http
GET /api/order-management/admin/affiliate-stats?range=7d&page=1&page_size=20&withdrawal_status=pending
```

响应包含 KPI 和用户行：

```json
{
  "summary": {
    "affiliate_user_count": 12,
    "period_commission_amount": 612.75,
    "pending_withdrawal_user_count": 3,
    "pending_withdrawal_amount": 300.0,
    "available_without_withdrawal_user_count": 9
  },
  "items": [
    {
      "user_id": 12,
      "username": "inviter_a",
      "period_commission_amount": 63.75,
      "total_commission_amount": 163.75,
      "available_amount": 63.75,
      "withdrawn_amount": 100.0,
      "withdrawal": {
        "id": 501,
        "withdrawal_id": "affw_xxx",
        "amount": 50.0,
        "contact": "支付宝：138****8888",
        "remark": "",
        "status": "pending",
        "created_time": 1782882000,
        "admin_remark": ""
      }
    }
  ]
}
```

### 处理提现申请

```http
POST /api/order-management/admin/affiliate-withdrawals/:id/paid
POST /api/order-management/admin/affiliate-withdrawals/:id/reject
```

请求体：

```json
{
  "admin_remark": "已通过支付宝打款"
}
```

提现处理必须记录管理员操作审计，至少包含提现 ID、用户 ID、金额、处理动作、管理员 ID、处理时间。

## 前端设计

### 文件结构建议

```text
web/default/src/routes/_authenticated/order-management/index.tsx
web/default/src/features/order-management/index.tsx
web/default/src/features/order-management/api.ts
web/default/src/features/order-management/types.ts
web/default/src/features/order-management/components/order-analytics-cards.tsx
web/default/src/features/order-management/components/order-trend-chart.tsx
web/default/src/features/order-management/components/order-details-table.tsx
web/default/src/features/order-management/components/affiliate-stats-section.tsx
web/default/src/features/order-management/components/mail-check-status-badge.tsx
web/default/src/features/order-management/components/withdrawal-actions.tsx
```

### UI 风格要求

1. 使用当前项目已有 UI 组件和设计 token。
2. 卡片使用白底、细边框、轻阴影或现有 `Card` 风格。
3. 页面背景使用 muted / slate 类浅背景，与当前后台一致。
4. 状态用 `Badge` 或项目现有徽章风格，不使用硬编码鲜艳大色块。
5. 表格密度要高，成交明细区域比图表区域更大。
6. 过滤条与按钮保持紧凑，避免占用明细高度。
7. 适配窄屏：图表卡片换行，成交表格横向滚动，返利统计表格可折叠或横向滚动。

### 国际化

所有新增前端文案必须使用 `t('English key')`，并补齐：

```text
en, zh, fr, ja, ru, vi
```

新增文案包括但不限于：

```text
Order Management
Order analytics
Revenue amount
External paid amount
Mail verification
Verify now
Recheck
Checking...
Pending mail
Amount mismatch
Order number mismatch
Affiliate statistics
Users with rewards
Withdrawal request
Mark as paid
Reject withdrawal
Source orders
```

## 数据库与兼容性

1. 所有新增查询优先使用 GORM，避免数据库特定 SQL。
2. 图表日聚合建议先查询时间范围内必要字段，再在 Go 中按日期聚合，避免 `DATE_TRUNC`、`GROUP_CONCAT`、`STRING_AGG` 等跨数据库差异。
3. 如果必须使用 raw SQL，必须兼容 SQLite、MySQL >= 5.7.8、PostgreSQL >= 9.6。
4. 金额比较使用 decimal 或“分”为单位的整数，不直接用 float 判断相等。
5. JSON marshal/unmarshal 继续使用 `common.Marshal`、`common.Unmarshal` 等项目 wrapper。
6. 迁移不能使用 SQLite 不支持的 `ALTER COLUMN`。

## 权限与审计

1. 页面仅管理员可见。
2. 后端接口使用 `AdminAuth`。
3. 立即核对动作记录操作日志：管理员 ID、触发范围、触发订单、结果数量、失败原因。
4. 提现处理动作记录审计：管理员 ID、提现 ID、用户 ID、金额、动作、备注。
5. 邮件授权码、Worker token、SMTP token 不在日志输出。
6. 邮件正文中的购买内容、卡密、token 默认脱敏。

## 错误处理

### 邮件相关

| 场景 | 处理 |
| --- | --- |
| 邮箱配置缺失 | 顶部操作提示“邮件配置不可用”，订单状态不变 |
| 邮箱连接失败 | 批量任务返回失败原因，订单标记 `mail_fetch_failed` 或保持待核对并显示任务失败 |
| 邮件解析失败 | 保存邮件事件和错误摘要，订单标记 `mail_parse_failed` |
| 找不到邮件 | 标记 `waiting_mail`，允许之后重新核对 |
| 单号不一致 | 标记 `order_mismatch` |
| 金额不一致 | 标记 `amount_mismatch` |
| 重复邮件 | 去重后不重复处理，不重复打勾 |

### 返利提现相关

| 场景 | 处理 |
| --- | --- |
| 提现已处理后重复点击 | 返回状态已变化，前端刷新行 |
| 可提现余额不足 | 禁止创建提现；管理端展示异常但不直接修改余额 |
| 打款失败 | 管理员不点击已打款；可备注失败原因 |
| 驳回提现 | 记录管理员备注，不删除申请记录 |

## 验收标准

1. 管理员后台出现 `订单管理` 菜单，普通用户不可见。
2. 页面顶部可以切换近 7 天和近 30 天，并且 KPI、趋势图、成交明细、返利统计同步刷新。
3. 页面上方展示图表分析，下方展示更大面积成交明细，最底部展示返利统计系统。
4. 成交明细中可以看到入账金额、外部实付金额、邮件实付金额、小铺单号、邮件核对状态。
5. 邮件核对通过条件是：小铺单号一致，外部实付金额快照与邮件实付金额一致。
6. `500` 入账、`425` 外部实付、邮件 `425` 的订单核对通过。
7. `10` 入账、`10.30` 外部实付、邮件 `10.30` 的订单核对通过。
8. `10` 入账、`10.30` 外部实付、邮件 `10.00` 的订单标记金额异常。
9. 点击单行 `立即核对` 后，系统马上开始核对该订单，并显示 `核对中`，结束后更新状态。
10. 点击顶部 `立即核对未完成订单` 后，系统马上处理当前范围未完成订单，不等待定时任务。
11. 底部返利统计能显示有返利用户数量。
12. 底部返利统计能显示用户是否申请提现；如果申请了，可以看到联系方式、申请金额、申请时间、备注和状态。
13. 管理员可以在页面处理待打款提现申请，且处理动作有审计记录。
14. 所有新增前端文案完成六语言 i18n。
15. 不引入数据库方言不兼容问题。
16. 不提交任何邮箱授权码、token 或生产密钥。

## 实施顺序建议

1. 先确认当前实现分支是否已经包含 LDXP session、mail event、affiliate commission、affiliate withdrawal 模型。
2. 如果缺失，先从已确认的 LDXP / 返利提现实现分支或生产基线同步必要模型与接口，避免重新发明不兼容 schema。
3. 写后端单元测试：金额核对、邮件解析、立即核对任务、图表聚合、返利统计。
4. 实现后端查询与立即核对 API。
5. 实现前端页面骨架：顶部时间范围、KPI、图表、成交明细、返利统计。
6. 补齐 i18n。
7. 运行 Go 测试、前端 typecheck/build 和 i18n sync。

## 风险与对策

### 风险 1：金额口径混淆

风险：把网站入账金额和小铺实付金额直接比较，导致折扣或手续费场景误报异常。

对策：核对只使用外部实付金额快照与邮件实付金额；设计、测试和列名都明确区分 `入账金额` 与 `外部实付`。

### 风险 2：本地旧文件覆盖生产 LDXP 能力

风险：当前主工作区可能缺少生产已有的 LDXP 文件，直接开发可能误删或覆盖已上线链路。

对策：实施第一步必须对齐生产/LDXP 分支差异，只做增量页面和 API，不删除现有 LDXP 文件。

### 风险 3：IMAP 或邮件服务不稳定

风险：邮箱授权码失效、IMAP 连接失败、邮件延迟，导致核对按钮短时间无结果。

对策：立即核对采用任务状态；失败显示原因，允许稍后重新核对；邮件拉取失败不影响订单入账状态。

### 风险 4：返利提现金额重复处理

风险：管理员重复点击已打款或并发处理同一提现申请。

对策：提现处理使用事务和状态检查，只允许 pending -> paid/rejected，重复处理返回当前状态。

### 风险 5：图表聚合跨数据库不一致

风险：用数据库日期函数导致 SQLite、MySQL、PostgreSQL 表现不一致。

对策：优先在 Go 中按日期聚合；必要 raw SQL 必须按项目跨数据库规则分支。
