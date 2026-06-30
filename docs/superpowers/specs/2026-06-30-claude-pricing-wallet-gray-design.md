# Claude 模型价格维护与钱包充值灰测设计

**日期：** 2026-06-30
**项目：** 云贝 `new-api` 生产定价与钱包充值体验维护
**状态：** 已完成生产灰测与代码侧上线（2026-06-30）；Claude 真实价格 options 与 LDXP 折扣真实支付闭环保留后续变更窗口
**范围：** `web/default` 前端、系统配置项、模型计费配置、生产灰测验证流程
**生产事实源：** 以 `https://yunbay.xyz` 当前生产服务器 `/opt/new-api/app`、生产数据库 options/channels/users/tokens 只读核验为准；本地工作区当前不完整反映生产钱包模块。

---

## 1. 背景与目标

本次维护有两条主线：

1. **模型价格维护：Claude 也要使用之前 GPT 的价格维护逻辑。**
   - 之前 GPT 已经通过系统设置里的模型定价配置维护了输入价、输出价、缓存价、长上下文/表达式计费等规则。
   - Claude 当前生产配置里已有大量 `ModelRatio` / `CompletionRatio` / `CacheRatio` / `CreateCacheRatio` 条目，也已有前端 `tiered_expr` 表达式编辑器与 Claude preset，但生产实际仍主要依赖 ratio 组合，Claude 4.x/别名/推理档位的维护不够成体系。
   - 本次目标是把 Claude 纳入与 GPT 一样可复制、可批量、可灰测、可回滚的维护路径：优先用表达式计费维护需要按上下文长度、缓存读写、1h cache、fast header 等区别计价的 Claude；对不需要阶梯的 Claude 子模型保留 ratio 维护但要有明确清单。

2. **钱包充值页与快速启动按钮体验调整。**
   - 生产当前已经上线 LDXP / 链动小铺浏览器 Worker 自动充值链路，钱包页顶部有 `Alipay Auto Top-up` 卡片，对应用户看到的“支付宝充值”。本地当前工作区没有这些 LDXP 文件，不能直接以本地旧文件为实现基础。
   - 充值金额按钮当前显示过弱，6 个金额不够醒目；50、100、500 元需要突出折扣：50 元 95 折、100 元 9 折、500 元 85 折，并展示原价、划线价、折扣标签，同时声明“支付手续费自理”。
   - 钱包页下方 “6 API 自带的账户充值” 栏在当前业务里显示未开通，应该移除该栏，只保留“推荐”和“兑换码/立即充值”主链路。
   - 普通用户快速启动页底部按钮不够显眼，需要在不破坏云贝整体科技、未来、先锋、简约风格的前提下增强。
   - 所有改动完成后先在生产上的“检测 001”灰测对象上验证。

---

## 2. 生产只读核验结果

以下只记录可公开、非敏感事实，不记录服务器密钥、cookie、后台密码、支付密钥或 API key。

### 2.1 生产部署形态

- 生产源码目录：`/opt/new-api/app`
- 生产核心容器状态：
  - `yunbay-new-api`：healthy
  - `yunbay-caddy`：healthy
  - `yunbay-postgres`：healthy
  - `yunbay-redis`：healthy
- 生产目录 `.git` 不可信，不应在服务器上依赖 `git pull` / `git status`。
- 生产发布仍应使用非删除式 `rsync` 到 `/opt/new-api/app`，再用生产 env-file 重建 `new-api` 容器。

### 2.2 本地与生产源码差异

生产 `web/default/src/features/wallet/` 比当前本地工作区多出并正在使用以下 LDXP / 自动充值相关文件：

```text
web/default/src/features/wallet/components/ldxp-topup-card.tsx
web/default/src/features/wallet/components/ldxp-payment-dialog.tsx
web/default/src/features/wallet/components/ldxp-payment-dialog-source.test.ts
web/default/src/features/wallet/hooks/use-ldxp-topup.ts
web/default/src/features/wallet/lib/ldxp-topup.ts
web/default/src/features/wallet/lib/ldxp-topup.test.ts
web/default/src/features/wallet/lib/redemption-result.ts
web/default/src/features/wallet/lib/redemption-result.test.ts
```

生产 `web/default/src/features/wallet/index.tsx` 当前已渲染：

```tsx
<LdxpTopupCard ... />
<RechargeFormCard ... />
<SubscriptionPlansCard ... />
<AffiliateRewardsCard ... />
<LdxpPaymentDialog ... />
```

结论：**实施前必须先把生产 wallet 模块相关文件回同步/合并到当前开发工作区，或基于生产文件做补丁。不得直接在当前本地旧版 wallet 文件上改完后整体覆盖生产，否则会误删已上线自动充值链路。**

### 2.3 当前生产 LDXP 充值卡片

生产 `LdxpTopupCard` 当前核心表现：

- 标题：`Alipay Auto Top-up`
- 描述：`Choose a fixed amount and scan the QR code to pay`
- 右上角 badge：`Alipay`
- 金额：来自 `LDXP_TOPUP_AMOUNTS = [10, 20, 30, 50, 100, 500]`，也可被后端 `ldxp_amount_options` 限制。
- 按钮当前只是 `outline` 小按钮，显示纯数字金额。

这正对应用户反馈的“支付宝充值”和“底下 6 个金额太不显眼”。

### 2.4 当前生产钱包布局

生产钱包页当前主体布局是：

```text
WalletStatsCard
└── 两列区域
    ├── 左侧：LdxpTopupCard + RechargeFormCard
    └── 右侧：SubscriptionPlansCard
AffiliateRewardsCard
```

用户要求“账户充值栏显示未开通，把这个栏目去掉，只保留推荐和兑换码”，与当前右侧 `SubscriptionPlansCard` 对应。该卡在普通用户路径里会造成一个未开通/无用栏位，应从普通钱包页移除。

保留项应是：

- 立即充值（LDXP 自动充值卡片，原“支付宝充值”）
- 兑换码（现位于 `RechargeFormCard` 底部）
- 推荐（`AffiliateRewardsCard`）
- 订单历史、余额统计、支付弹窗等已有辅助能力

### 2.5 当前生产充值折扣配置

生产 options 中本次只读未看到 `payment_setting.amount_options` / `payment_setting.amount_discount` 被查出为独立当前值；生产仍存在 legacy/default 支付配置和 LDXP 自己的金额列表。后端默认支付设置代码中是：

```go
AmountOptions: []int{10, 20, 50, 100, 200, 500}
AmountDiscount: map[int]float64{}
```

生产 LDXP 金额列表当前是：

```ts
[10, 20, 30, 50, 100, 500]
```

因此本次 50/100/500 的折扣展示应该在 LDXP 自动充值卡片里独立处理，不依赖普通 `RechargeFormCard` 的 `payment_setting.amount_discount`，以免影响旧在线充值 / Stripe / Waffo 计算逻辑。

后端真实付款金额仍应由 LDXP 后端 session / 商品配置决定。前端折扣展示只展示“本次用户可获得的折扣价/原价/手续费说明”，不能自行篡改入账金额。

### 2.6 当前生产模型定价配置

生产 options 中与 GPT / Claude 相关事实：

- `ModelRatio` 已含 GPT 与 Claude 大量条目，例如：
  - GPT：`gpt-5.4`、`gpt-5.5`、`gpt-5.4-mini`、`gpt-5.4-nano`、`openai/gpt-5.5` 等。
  - Claude：`claude-opus-4-6`、`claude-opus-4-7`、`claude-opus-4-8`、`claude-sonnet-4-5-20250929`、`anthropic/claude-sonnet-4.6`、`claude-haiku-4-5-20251001` 等。
- `CompletionRatio` 已含 Claude 输出倍率条目，Claude 多数为 `5`。
- `CacheRatio` 已含 Claude 缓存读取条目，多数为 `0.1`。
- `CreateCacheRatio` 已含 Claude 缓存创建条目，多数为 `1.25`。
- `billing_setting.billing_mode` / `billing_setting.billing_expr` 当前未在只读输出中显示出生产已有大量 Claude 表达式配置，说明生产 Claude 主要仍是 ratio 模式。

### 2.7 当前代码中的表达式计费能力

项目已经具备完整表达式计费系统：

- 文档：`pkg/billingexpr/expr.md`
- 后端存储：`setting/billing_setting/tiered_billing.go`
- 预扣：`relay/helper/price.go`
- 实际结算：`service/tiered_settle.go`
- 前端编辑器：`web/default/src/features/system-settings/models/tiered-pricing-editor.tsx`
- 模型定价表/抽屉：`web/default/src/features/system-settings/models/model-ratio-visual-editor.tsx`、`model-pricing-sheet.tsx`

表达式变量支持：

```text
p      输入 token
len    完整输入上下文长度，Claude 长上下文分档应使用 len
c      输出 token
cr     cache read
cc     cache create，通用/Claude 5min TTL
cc1h   Claude 1h cache create
img    图片输入
ai/ao  音频输入/输出
```

现有前端 preset 已包含 Claude 示例：

```text
Claude Opus 4.6
Claude Sonnet 4.5
Claude Opus 4.6 Fast
```

这说明“Claude 用 GPT 之前逻辑维护价格”的优选方案不是新建一套 Claude 专属配置系统，而是复用现有 `tiered_expr` 和模型定价表，把 Claude 模型批量切到表达式/统一 snapshot 维护。

### 2.8 灰测对象：检测 001

生产只读查询未发现名为“检测 001”的渠道或分组，但发现以下灰测相关对象：

```text
users.username = jiance001, group = 体验用户
tokens.name = cs001, user_id = 1, group = gpt-plus
tokens.name = 001, user_id = 1, group = gpt-plus
```

因此本设计把“检测 001”解释为生产灰测账号/令牌集合，而不是独立部署环境。实施与验证时应：

1. 不扩散到所有用户可见行为之前，先用 `jiance001` 登录/会话或管理员代查方式验证普通用户钱包与快速启动；
2. 如需 API 灰测，使用生产中已有 `001` / `cs001` 令牌对应能力，但不得在文档或日志中打印 token key；
3. 如果用户后续指出“检测 001”另有专用渠道/环境，则按用户现场反馈调整灰测载体。

---

## 3. 设计原则

### 3.1 生产优先

- 任何实现前都必须先以生产 `/opt/new-api/app` 当前源码为准，尤其是 wallet 模块。
- 本地当前工作区落后于生产 wallet，不允许直接覆盖生产文件。
- 所有生产数据修改都先备份，先灰测，后扩大。

### 3.2 不拆现有体系

- Claude 定价复用已有模型价格维护体系：`ModelRatio` / `CompletionRatio` / `CacheRatio` / `CreateCacheRatio` / `billing_setting.billing_mode` / `billing_setting.billing_expr`。
- 不新建 Claude 专属表或专属配置页。
- 不修改 billing expression 语言本身。

### 3.3 UI 风格

云贝当前前台/快速启动是深色、粒子、玻璃、先锋科技感；后台钱包页是 `base-nova` shadcn 风格。充值页改动应做到：

- 简约，但有更强的金额层级。
- 科技、未来、先锋感来自材质、光边、节奏，而不是堆满霓虹和随机渐变。
- 不引入新 UI 库；继续用现有 `Button`、`Badge`、`Alert`、`TitledCard`、`Skeleton` 等组件。
- 按钮对比度要足够，桌面文案不换行，移动端布局可读。
- 保留 `new-api` / `QuantumNous` 相关归属信息，不修改受保护标识。

### 3.4 安全与数据边界

- 不在文档中记录完整服务器 IP、密钥、cookie、支付 secret、管理员密码、API key。
- 不打印灰测 token key。
- 不做破坏性 schema rollback。
- 不用 `rsync --delete`。

---

## 4. Claude 价格维护设计

### 4.1 总体方案

采用“配置数据维护 + 现有编辑器增强提示 + 灰测验证”的方式：

1. **用表达式维护 Claude 的复杂计费模型。**
   - Sonnet / Opus 这类有长上下文阶梯、缓存读写、1h cache 或 fast header 的模型使用 `tiered_expr`。
   - 表达式必须用 `len` 判断上下文档位，不用 `p` 判断档位，避免 cache 命中导致分档误判。
2. **用 ratio 维护简单 Claude 模型。**
   - Haiku 等无复杂阶梯或当前业务不需要分档的模型可继续 ratio。
3. **把 Claude 别名统一映射到同一价格模板。**
   - 例如 `claude-sonnet-4-5-20250929`、`anthropic/claude-sonnet-4.6` 这类同家族模型，应清楚记录价格来源和模板。
4. **前端编辑器只做小增强。**
   - 保留现有 `TieredPricingEditor` preset。
   - 如有必要新增/调整 Claude preset 文案和模板，但不重做编辑器。
5. **生产变更通过 options 写入。**
   - 改动目标是 options 中的 JSON map，而不是写死在代码中。

### 4.2 Claude 表达式模板

表达式原则来自 `pkg/billingexpr/expr.md`：系数是实际 `$ / 1M tokens`。

#### Claude Sonnet 长上下文模板

适用于 Sonnet 4.x/4.5/4.6 家族，具体模型名单由生产模型广场和当前渠道可用模型决定。

```text
len <= 200000
  ? tier("standard", p * 3 + c * 15 + cr * 0.3 + cc * 3.75 + cc1h * 6)
  : tier("long_context", p * 6 + c * 22.5 + cr * 0.6 + cc * 7.5 + cc1h * 12)
```

要点：

- `len <= 200000` 是分档条件。
- `p` 是文本输入价格。
- `c` 是输出价格。
- `cr` 是 cache read。
- `cc` 是 5 分钟 cache create。
- `cc1h` 是 1 小时 cache create。

#### Claude Opus 基础模板

适用于当前没有长上下文差异、但需要缓存细分的 Opus 4.6/4.7/4.8 家族。具体价格需按生产渠道真实供应商公布价或管理员设定确认；当前编辑器已有示例：

```text
tier("base", p * 5 + c * 25 + cr * 0.5 + cc * 6.25 + cc1h * 10)
```

#### Claude Fast header 模板

适用于需要按 `anthropic-beta` header 中 fast mode 加价的 Opus Fast 场景。当前编辑器已有 request-rule 模板：

```text
tier("base", p * 5 + c * 25 + cr * 0.5 + cc * 6.25 + cc1h * 10)|||when(header("anthropic-beta") has "fast-mode-2026-02-01") * 6
```

### 4.3 价格配置落点

对于切到表达式计费的 Claude 模型，生产 options 应写入：

```json
billing_setting.billing_mode = {
  "claude-sonnet-4-5-20250929": "tiered_expr",
  "anthropic/claude-sonnet-4.6": "tiered_expr"
}

billing_setting.billing_expr = {
  "claude-sonnet-4-5-20250929": "len <= 200000 ? tier(...) : tier(...)"
}
```

同时保留/写入这些模型在 `ModelRatio` / `ModelPrice` 中的可读参考值。现有前端保存逻辑已经对 `tiered_expr` 模型“始终序列化 ratio/price 值”，用于表格展示和兼容，但实际计费以 expression 为准。

### 4.4 管理端 UX

不新增页面，只在已有路径维护：

```text
System Settings -> Models -> Pricing Configuration
```

现有能力：

- 表格扫描所有模型价格；
- 选中行打开侧边编辑器；
- 模式选择 `Per-token` / `Per-request` / `Expression`；
- Expression 下可选 preset、视觉编辑、raw 编辑、request rules。

本次实施应确保：

1. Claude preset 标签清晰，例如 `Claude Sonnet 4.5 / 4.6`，避免只有单一版本导致管理员误用。
2. 文案强调：Claude 长上下文分档使用 `Full input length (len)`。
3. 保存后 `/api/pricing` 能返回 `billing_mode=tiered_expr` 与 `billing_expr`。
4. 使用日志/模型广场能显示动态价格 breakdown。

### 4.5 不做事项

- 不重写计费引擎。
- 不新增数据库表。
- 不把生产模型价格写死到 Go 默认 map 作为唯一来源。
- 不自动全量覆盖管理员在生产手工维护过的所有 Claude 价格；先灰测模型清单，再扩大。

---

## 5. 钱包充值页设计

### 5.1 页面结构

普通用户钱包页调整为一列主流程：

```text
WalletStatsCard
立即充值（LDXP 自动充值卡片，6 个金额大按钮）
兑换码（独立 `RedemptionCodeCard`）
推荐（AffiliateRewardsCard）
订单历史入口 / LDXP 弹窗
```

移除普通用户主页面中的：

```text
SubscriptionPlansCard / 账户充值 / 未开通订阅套餐栏
```

后续如果管理员还需要管理订阅套餐，可保留后台系统设置和相关组件源码，但普通钱包页不渲染该栏。

### 5.2 “支付宝充值”改名为“立即充值”

生产 `LdxpTopupCard`：

```tsx
title={t('Alipay Auto Top-up')}
description={t('Choose a fixed amount and scan the QR code to pay')}
action={<Badge>{t('Alipay')}</Badge>}
```

改为：

- 标题：`立即充值` / `Top up now`
- 描述：`选择金额，扫码后自动到账。支付平台手续费由用户自行承担。`
- badge：可改为 `支付宝扫码`，但标题不要再叫“支付宝充值”。

建议中文体验：

```text
立即充值
选择金额，扫码后自动到账。支付平台手续费由用户自行承担。
支付宝扫码
```

### 5.3 金额卡片设计

当前 6 个金额按钮太弱。新设计为 2x3 或 3x2 响应式金额卡：

- 手机：2 列
- 平板：3 列
- 宽屏：6 列或 3 列大卡，按当前容器宽度选择不拥挤的布局

每个金额卡包含：

```text
¥50
95 折
原价 ¥50
现付 ¥47.50
手续费自理
```

对于无折扣金额：

```text
¥10
标准价
现付 ¥10.00
手续费自理
```

视觉要求：

- 金额字号明显增大，建议 `text-2xl` 到 `text-3xl`。
- 折扣卡使用更强层级，例如高亮边框、微光背景、`Badge`，但保持克制。
- 原价使用删除线：`line-through`。
- 折扣标签写清楚：`95 折` / `9 折` / `85 折`。
- 选 hover/active 有轻微下压或亮边反馈。
- disabled 状态仍清晰，不可点击时不要像主 CTA。

### 5.4 折扣规则

前端展示规则：

```ts
const LDXP_TOPUP_DISCOUNTS = {
  50: 0.95,
  100: 0.9,
  500: 0.85,
}
```

计算：

```text
original = amount
payable = amount * discount
save = original - payable
```

显示金额四舍五入到 2 位：

```text
50 -> 47.50
100 -> 90.00
500 -> 425.00
```

注意：

- 这是 LDXP 自动充值的展示规则，不直接改普通 `payment_setting.amount_discount`，除非实施中确认 LDXP 后端付款金额也读取该配置。
- 如果 LDXP 后端当前创建 session 时仍按 amount 原值或商品链接价格生成订单，那么必须同步生产 `LDXP_TOPUP_PRODUCTS_JSON` 商品映射或后端 LDXP session 金额逻辑，使展示价格与真实支付金额一致。
- 灰测前必须用 50/100/500 分别检查创建的 session `money` 是否与前端展示一致。

### 5.5 手续费声明

在 LDXP 卡片描述和每个金额卡底部都要有短声明：

```text
支付平台手续费由用户自行承担
```

英文：

```text
Payment platform fees are borne by the user.
```

文案要直白，不藏在 tooltip 里。

### 5.6 兑换码保留

`RechargeFormCard` 当前同时包含传统在线充值和兑换码。普通用户目标是只保留“推荐”和“兑换码”之外，再加主“立即充值”。本设计确定采用独立兑换码卡片：

```text
RedemptionCodeCard
```

落地方式：

- 从 `RechargeFormCard` 现有兑换码区域抽出一张只负责兑换码的卡片。
- 普通钱包页渲染 `LdxpTopupCard`、`RedemptionCodeCard`、`AffiliateRewardsCard`。
- 普通钱包页不再渲染 `RechargeFormCard`，避免传统在线充值、支付方式、未开通提示继续出现。
- `RechargeFormCard` 源码可先保留，供后台或后续兼容场景继续引用；本次不做删除式重构。

这样最符合“只保留推荐和兑换码”的要求，也避免通过 prop 隐藏旧区域后残留未开通提示。

### 5.7 订单历史保留

当前 `RechargeFormCard` 顶部有 `Order History` action。移除/拆分后仍需要保留订单历史入口，放在 `LdxpTopupCard` / `Top up now` 卡右上角。

---

## 6. 快速启动按钮增强设计

### 6.1 当前问题

快速启动页底部 `Previous` / 页码 / `Enter dashboard` / `Next` 控件在深色粒子背景下比较轻。普通用户首次进入时，主按钮存在感不够。

### 6.2 改造原则

- 不改变快速启动的页面结构和数据流程。
- 只增强底部 controls 和关键 CTA 的视觉层级。
- 保持当前深色宇宙、玻璃、粒子风格。
- 不做大面积彩虹渐变，不破坏简约感。

### 6.3 具体设计

`Next` / `Enter dashboard` 主按钮增强：

- 使用高对比白色/近白主按钮继续保持；
- 增加更清晰的外发光：例如 `shadow-[0_18px_60px_rgba(255,255,255,0.20)]`；
- 增加细微 ring：`ring-1 ring-white/30`；
- hover 时略微上浮或亮度提高；
- disabled 时不能过亮。

`Enter dashboard` 次按钮增强：

- 保持 glass outline，但增加 border 和背景对比，避免完全融入背景。

移动端：

- 主按钮应至少 `h-11`，文字不换行。
- 底部 controls wrap 后仍保持主按钮靠右或占满一行。

### 6.4 不做事项

- 不改快速启动步骤数量。
- 不改 API key 生成逻辑。
- 不改兑换码逻辑。
- 不引入新动画库。

---

## 7. i18n 设计

新增/变更前端文案必须同步 6 种语言：

```text
en, zh, fr, ja, ru, vi
```

新增 key 建议：

```text
Top up now
Choose an amount, scan the QR code, and funds will arrive automatically.
Payment platform fees are borne by the user.
Alipay QR payment
Standard price
Original price
Pay now
You save {{amount}}
{{discount}} off
Redemption code
```

如果继续使用英文 key 作为 i18n key，应保持 `en.json` key=value；其它语言给出自然翻译。

---

## 8. 灰测与生产上线设计

### 8.1 灰测对象

优先使用生产已发现对象：

```text
普通用户：jiance001
令牌名称：001 / cs001（不打印 token key）
```

### 8.2 灰测步骤

1. 同步代码到生产前，先在本地完成 typecheck/build/test。
2. 生产部署前备份：
   - 被覆盖的 wallet/quick-start/system-settings 文件；
   - 当前 `yunbay-new-api:prod` 镜像 tag；
   - 如修改 options，备份相关 options 行。
3. 非删除式同步到 `/opt/new-api/app`。
4. 重建并重启 `yunbay-new-api`。
5. 使用 `jiance001` 验证：
   - 钱包页出现“立即充值”；
   - 不出现“支付宝充值”作为卡片标题；
   - 6 个金额更明显；
   - 50/100/500 有折扣、原价划线、现付价格和手续费声明；
   - 不出现未开通的“账户充值/订阅套餐”栏；
   - 推荐和兑换码保留；
   - 快速启动底部主按钮更醒目。
6. API 灰测：
   - 用灰测 token 或管理员发起 Claude 测试请求；
   - 验证 `/api/pricing` 中目标 Claude 的 `billing_mode` / `billing_expr`；
   - 验证使用日志中动态价格 breakdown 和 tier 信息。

### 8.3 回滚

- UI 回滚：恢复备份文件并重建容器。
- options 回滚：恢复相关 options 备份行，重启/刷新配置。
- 不删除已产生的充值记录或兑换记录。
- 若新增 DB 字段已经存在但旧代码忽略，不做破坏性 rollback。

---

## 9. 验证清单

### 9.1 本地验证

默认目录：

```bash
cd /Users/ethan/Documents/yunbay/web/default
```

必须运行：

```bash
bun test src/features/wallet/lib/ldxp-topup.test.ts
bun test src/features/wallet/components/ldxp-payment-dialog-source.test.ts
bun test src/features/quick-start/quick-start-page-source.test.ts
bun run typecheck
bun run build
bun run i18n:sync
```

如果 wallet 生产文件回同步后带有更多测试，也一并运行。

后端/计费表达式验证：

```bash
go test ./pkg/billingexpr ./service ./relay/helper -count=1
```

本机没有系统 Go 时使用项目维护文档里的 Go toolchain。

### 9.2 生产只读验证

部署后：

```bash
curl -sS -L -o /dev/null -w "yunbay_status=%{http_code}\n" https://yunbay.xyz/api/status
curl -sS -L -o /dev/null -w "yunbay_wallet=%{http_code}\n" https://yunbay.xyz/wallet
curl -sS -L -o /dev/null -w "yunbay_quick_start=%{http_code}\n" https://yunbay.xyz/quick-start
```

容器：

```bash
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml ps new-api caddy
```

### 9.3 视觉验收

- 金额卡片在 1440px、1024px、390px 宽度下不拥挤。
- 折扣信息一眼能看到。
- 手续费声明可见。
- 快速启动主 CTA 在深色背景下明显。
- 不出现颜色过饱和、模板化 AI 紫、杂乱渐变。

---

## 10. 风险与对策

### 风险 1：本地旧文件覆盖生产 LDXP 链路

对策：实施计划第一步必须回同步/合并生产 wallet 文件，且 diff 确认不删除 LDXP 文件。

### 风险 2：前端折扣展示和后端实际付款金额不一致

对策：灰测时检查 LDXP session 返回的 `money`，确认 50/100/500 与展示价格一致；若不一致，先调整生产 LDXP 商品映射或后端 session 金额逻辑，再开放。

### 风险 3：Claude 表达式误用 `p` 分档导致缓存场景计费错误

对策：所有 Claude 长上下文表达式使用 `len`；新增测试覆盖 cache read 很大但 `len > 200000` 的场景。

### 风险 4：移除普通钱包页订阅/传统充值入口影响旧支付路径

对策：只从普通钱包页不渲染，不删除组件、不删除后端订阅接口。以后如需订阅产品可重新放到独立入口。

### 风险 5：i18n 漏 key

对策：实施后运行 `bun run i18n:sync`，检查 `_sync-report.json`。

---

## 11. 设计结论

本次不应做大规模重构。正确路径是：

1. 以生产 wallet 代码为真实基线，先防止覆盖已上线的 LDXP 自动充值。
2. 把“支付宝充值”产品化为“立即充值”，用更强的金额卡片承接 10/20/30/50/100/500 六档，重点突出 50/100/500 折扣和手续费声明。
3. 普通用户钱包页移除未开通的 `SubscriptionPlansCard` 和传统 `RechargeFormCard` 充值流程，保留推荐和独立兑换码卡。
4. 快速启动只增强 CTA 视觉，不改变流程。
5. Claude 价格维护复用 GPT 已使用的模型价格维护体系，复杂 Claude 使用 `tiered_expr`，长上下文分档统一用 `len`。
6. 所有改动先在生产“检测 001”灰测对象验证，再扩大。

---

## 12. 生产灰测与上线完成记录（2026-06-30）

本维护已在 2026-06-30 完成生产灰测与代码侧上线。此前“等待按实施计划落地”的状态已被本节替换为生产完成事实；但 Claude 真实价格 options 改写和 LDXP 50/100/500 折扣真实支付闭环仍保留为后续生产变更窗口，不能把本次记录误读为这些配置已经全量扩大。

### 12.1 实施基线

灰测分支与工作树：

```text
分支：codex/claude-pricing-wallet-gray
工作树：/Users/ethan/Documents/yunbay/.worktrees/claude-pricing-wallet-gray
```

关键提交：

```text
1ea932fd feat: update Claude pricing presets and wallet gray rollout
0ce58397 fix: restore quick start Codex launcher page
38619d6d fix: align wallet referral card and signup group
1e021bdf fix: restore signup invite and token defaults
434230f3 feat: prepare claude pricing gray rollout presets
83479b48 fix(router): register user group tags route
```

生产基线以服务器 `/opt/new-api/app` 已部署的 LDXP / affiliate / redemption 能力为准；实施时先保留生产自动充值基线，再做 UI 与配置维护。

### 12.2 部署方式与备份

本次生产同步继续遵守非删除式同步原则，没有使用 `rsync --delete`，也没有依赖生产 `.git`。

```text
部署文件列表：/tmp/claude-wallet-gray-files.txt
文件数：110
files sha256：ee754ff804797d43f767b2f53c261a01ca424ad1c3d169cbff0ed8751db36a6e
生产文件备份：/opt/new-api/backups/app-files-before-claude-wallet-20260630040325.tgz
镜像备份：yunbay-new-api:backup-claude-wallet-20260630040325
镜像备份：yunbay-ldxp-browser-worker:backup-claude-wallet-20260630040325
```

生产构建与重启结果：

```text
yunbay-new-api:prod 构建完成
yunbay-ldxp-browser-worker:prod 构建完成
yunbay-new-api Up (healthy)
yunbay-ldxp-browser-worker Up
```

### 12.3 生产 smoke 与“检测 001”灰测

生产 smoke：

```text
yunbay-caddy Up (healthy)
yunbay-new-api Up (healthy)
yunbay-ldxp-browser-worker Up
home=200
status=200
topup_anon=401
ldxp_anon=401
```

生产前端 bundle 脱敏字符串检查：

```text
Top up now=true
Claude Sonnet 4.5 / 4.6=true
Claude Opus 4.6 / 4.7 / 4.8=true
Claude Opus Fast=true
Payment platform fees are borne by the user.=true
Redemption code=true
Wallet and redemption code=true
Use redemption code=true
```

“检测 001”生产普通灰测账号路径已验证；灰测采用临时 access token 写入方式，只输出脱敏字段，执行结束后已恢复原 access token 状态。

```text
topup_success=true
enable_ldxp_topup=true
ldxp_amount_options=[10,20,30,50,100,500]
enable_redemption=true
payment_compliance_confirmed=true
amount=10 session money=10 status=created
cancel_success=true cancel_status=canceled
access_token_restored_null=t
```

说明：灰测只创建并立即取消低额 LDXP session，不进行真实支付；未记录 session id、二维码、卡密或 token。

### 12.4 明确未全量扩大的配置

LDXP 折扣真实支付闭环：

- 已完成前端折扣展示和支付弹窗应付金额显示修正；
- 未擅自修改生产 `LDXP_TOPUP_PRODUCTS_JSON` 的 50/100/500 `money` 为 `47.5/90/425`；
- 因为当前生产商品链接只读核验仍是原价商品，直接修改 `money` 会导致 worker 与后端验单金额不匹配；
- 后续必须先在 LDXP 后台创建或确认折扣商品链接，再把 `product_url` 与 `money` 一起切换，并用 `jiance001` 分别验证 50/100/500。

Claude 真实价格 options：

- 已部署 Claude 价格维护的前端 preset 和表达式维护路径；
- 未在本窗口直接改写生产 options 中的 Claude 价格配置；
- 后续改写前必须明确目标模型清单、备份相关 options 行，并验证 `/api/pricing`、实际小流量请求和 usage log。

### 12.5 后续维护要求

- 钱包页、LDXP 自动充值、兑换码、推荐返利、Quick Start CTA 和 Claude preset 的后续改动都必须基于生产文件，不得用旧本地 wallet 文件覆盖生产；
- 真实折扣支付闭环扩大前，必须确保商品链接金额和 session money 一致；
- Claude 表达式长上下文分档继续使用 `len`，不要用扣除缓存后的 `p` 分档；
- 不要在文档中记录 token、worker token、支付密钥、卡密、完整 session id、完整二维码或后台 cookie。
