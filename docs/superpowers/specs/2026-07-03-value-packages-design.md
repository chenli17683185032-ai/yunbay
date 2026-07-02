# 超值套餐与会员仪式感设计规格

> 基线：本规格基于 `main` / `origin/main` 当前提交 `8d2be058cdf94de615a3fa5671fcdb90280c5d5b` 的实际代码结构编写。本文只定义需求、边界与建议落点，不包含实现代码。

## 1. 背景与目标

当前项目已有一套“订阅套餐”能力，但它更像通用订阅余额包：套餐在钱包页内展示，订单可走多种支付方式，购买后创建 `user_subscriptions` 并可通过 `billing_preference` 参与扣费。这个能力无法完整承载“日卡 / 周卡 / 月卡”这种独立模型分组套餐：

- 套餐需要独立于钱包充值页面，和钱包平级展示为“超值套餐”。
- 套餐需要固定三种等级：日卡、周卡、月卡。
- 套餐购买应默认走联动小铺付款，复用现有联动小铺充值支付体验；第一版不走余额、Stripe、Creem、易支付。
- 套餐需要只影响“当前模型分组 / 权益开关”，不改变 `users.group` 用户身份分组。
- 套餐需要后台可配置并发、5 小时限额、7 天限额，且限额用管理员能理解的“美元 Token 额度”输入。
- 套餐启动 / 关闭要有明显视觉状态：页面四周淡黄色呼吸边缘光。
- 充值满 30 元自动升 VIP 的现有逻辑需要补“仪式感”：一次性会员卡弹窗 + VIP 边框光效。

目标是把现有订阅系统升级为可支持上述“超值套餐”的产品规格，同时不破坏当前钱包充值、API 余额、VIP 升级、订阅扣费和三数据库兼容性。

## 2. main 现状证据

### 2.1 后端订阅模型与订单

实际文件：`model/subscription.go`

现有 `SubscriptionPlan` 字段包括：

- 基础展示：`Title`、`Subtitle`、`PriceAmount`、`Currency`
- 时长：`DurationUnit`、`DurationValue`、`CustomSeconds`
- 状态排序：`Enabled`、`SortOrder`
- 支付配置：`AllowBalancePay`、`StripePriceId`、`CreemProductId`、`WaffoPancakeProductId`
- 购买限制与用户分组：`MaxPurchasePerUser`、`UpgradeGroup`
- 额度：`TotalAmount`、`QuotaResetPeriod`、`QuotaResetCustomSeconds`

现有 `UserSubscription` 字段包括：

- `UserId`、`PlanId`
- `AmountTotal`、`AmountUsed`
- `StartTime`、`EndTime`、`Status`
- `Source`
- `LastResetTime`、`NextResetTime`
- `UpgradeGroup`、`PrevUserGroup`

现有订单完成流程 `CompleteSubscriptionOrder` 会：

1. 读取 `SubscriptionOrder`；
2. 调 `CreateUserSubscriptionFromPlanTx` 创建 `UserSubscription`；
3. 调 `upsertSubscriptionTopUpTx` 写入一条 `TopUp`；
4. 调 `MaybeUpgradeUserToVIPTx` 尝试升级 VIP；
5. 更新订单成功状态并记录日志。

这说明：现有订阅购买金额已经能进入 `TopUp` 统计，并参与 30 元 VIP 升级逻辑；但现有订阅创建会按 `plan.UpgradeGroup` 修改 `users.group`，这与本次“套餐只改变模型分组，不改变用户身份分组”的新要求冲突，必须为超值套餐关闭或绕开 `UpgradeGroup` 修改用户身份的行为。

### 2.2 后端订阅 API

实际文件：`router/api-router.go`、`controller/subscription.go`

现有用户订阅路由：

- `GET /api/subscription/plans`
- `GET /api/subscription/self`
- `PUT /api/subscription/self/preference`
- `POST /api/subscription/balance/pay`
- `POST /api/subscription/epay/pay`
- `POST /api/subscription/stripe/pay`
- `POST /api/subscription/creem/pay`
- `POST /api/subscription/waffo-pancake/pay`

现有管理员订阅路由：

- `GET /api/subscription/admin/plans`
- `POST /api/subscription/admin/plans`
- `PUT /api/subscription/admin/plans/:id`
- `PATCH /api/subscription/admin/plans/:id`
- `POST /api/subscription/admin/bind`
- `GET /api/subscription/admin/users/:id/subscriptions`
- `POST /api/subscription/admin/users/:id/subscriptions`
- `POST /api/subscription/admin/user_subscriptions/:id/invalidate`
- `DELETE /api/subscription/admin/user_subscriptions/:id`

`GetSubscriptionSelf` 当前返回：

- `billing_preference`
- `subscriptions`：当前活跃订阅
- `all_subscriptions`：含过期订阅

这说明超值套餐可以复用一部分订阅查询结构，但还缺少“当前套餐是否启用使用中”“套餐模型分组”“5 小时 / 7 天限额”“并发”等状态。

### 2.3 现有联动小铺 / LDXP 充值链路

实际文件：

- `model/ldxp_topup.go`
- `controller/ldxp_topup.go`
- `service/ldxp_session.go`
- `web/default/src/features/wallet/components/ldxp-topup-card.tsx`
- `web/default/src/features/wallet/hooks/use-ldxp-topup.ts`

现有 LDXP 会话模型 `LdxpTopupSession` 包括：

- 用户与金额：`UserId`、`Amount`、`Money`
- 商品：`ProductUrl`、`ProductName`
- 状态机：`created`、`worker_claimed`、`qr_ready`、`worker_paid`、`verified`、`redeemed`、`success` 等
- 工作者与二维码：`WorkerId`、`QrCode`、`QrPageUrl`
- 订单核验：`WorkerOrderNo`、`WorkerAmount`、`MailOrderNo`、`MailAmount`
- 结算关联：`TopupId`、`RedemptionId`

现有用户 LDXP 路由：

- `POST /api/user/ldxp/topup/session`
- `GET /api/user/ldxp/topup/session/:session_id`
- `POST /api/user/ldxp/topup/session/:session_id/cancel`

现有服务入口 `service.CreateLdxpTopupSession(userID, amount, cfg)` 只接受充值金额 `amount`，再从 `cfg.Products[amount]` 找商品链接和商品名称。

因此，“套餐默认走联动小铺付款，复用充值金额支付途径”不能直接把现有函数原样用于套餐订单：需要在 LDXP 会话中区分业务类型，或新增套餐版 LDXP session 创建入口，确保付款成功后开通套餐而不是增加钱包余额。

### 2.4 现有 Waffo Pancake 订阅支付

实际文件：`controller/subscription_payment_waffo_pancake.go`

当前订阅已有 `POST /api/subscription/waffo-pancake/pay`：

- 使用 `SubscriptionOrder` 创建订单；
- 要求 `plan.WaffoPancakeProductId`；
- 通过 `service.CreateWaffoPancakeCheckoutSession` 创建 hosted checkout；
- 回调按 `WAFFO_PANCAKE_SUB-` 前缀分发到订阅完成逻辑。

这是另一条在线支付能力，不等同于本次用户所说“联动小铺付款”。本次第一版套餐支付应以现有 LDXP 浏览器 worker 充值链路为优先复用对象，而不是继续扩展 Waffo Pancake / Stripe / Creem / Epay。

### 2.5 现有扣费偏好与 API 余额逻辑

实际文件：

- `dto/user_settings.go`
- `common/str.go`
- `service/billing_session.go`
- `model/subscription.go`

用户设置 `dto.UserSetting` 当前有 `BillingPreference`，可为：

- `subscription_first`
- `wallet_first`
- `subscription_only`
- `wallet_only`

`service.NewBillingSession` 当前按 `BillingPreference` 决定优先用订阅还是钱包；默认是 `subscription_first`。`PreConsumeUserSubscription` 当前只按用户活跃订阅、结束时间和剩余额度选择，不按“套餐类型 / 模型分组 / 启动开关”过滤。

这说明：如果把日卡 / 周卡 / 月卡直接塞进现有活跃订阅列表，而不增加“当前是否启用套餐”和“套餐分组过滤”，它会自动参与订阅扣费，无法满足“关闭套餐使用后走 API 余额、套餐时间继续计算”的要求。

### 2.6 用户分组与模型分组现状

实际文件：

- `model/user.go`
- `model/token.go`
- `setting/user_usable_group.go`
- `setting/ratio_setting/group_ratio_defaults_test.go`
- `middleware/distributor.go`
- `relay/helper/price.go`

当前用户身份分组：

- `model.UserGroupDefault = "default"`
- `model.UserGroupTiyan = "体验用户"`
- `model.UserGroupVIP = "vip"`

当前默认模型分组：

- `model.DefaultModelGroup = "gpt-plus"`
- `setting/user_usable_group.go` 默认可用模型分组是 `gpt-plus` 和 `gpt-pro`
- `setting/ratio_setting/group_ratio_defaults_test.go` 显示默认组倍率：
  - `gpt-plus: 0.3`
  - `gpt-pro: 0.4`
  - `体验用户` 对 `gpt-plus` 是 `0.99`，对 `gpt-pro` 是 `1.32`

这与用户的业务描述一致：普通用户在 Plus 基础上约 `3.3x`，VIP 用户按 Plus `1x`。因此本次超值套餐应该改变“当前使用的模型分组 / token group / request group”，不应该改变 `users.group`。`users.group` 继续承担用户身份和倍率角色。

### 2.7 前端钱包与订阅页面现状

实际文件：

- 用户钱包路由：`web/default/src/routes/_authenticated/wallet/index.tsx`
- 钱包 Feature：`web/default/src/features/wallet/index.tsx`
- 钱包内订阅卡：`web/default/src/features/wallet/components/subscription-plans-card.tsx`
- 管理员订阅路由：`web/default/src/routes/_authenticated/subscriptions/index.tsx`
- 管理员订阅 Feature：`web/default/src/features/subscriptions/index.tsx`
- 管理员订阅表单：`web/default/src/features/subscriptions/lib/plan-form.ts`、`web/default/src/features/subscriptions/components/subscriptions-mutate-drawer.tsx`
- 侧边导航：`web/default/src/hooks/sidebar-data-model.ts`、`web/default/src/hooks/use-sidebar-data.ts`

当前钱包布局里，`Wallet` 先展示 `WalletStatsCard`，再用两列布局展示：

- 左侧：`LdxpTopupCard`、`RechargeFormCard`
- 右侧：`SubscriptionPlansCard`
- 下方：`AffiliateRewardsCard`

这与新要求的用户界面顺序不一致。用户要求普通用户的钱包界面顺序是：

1. 先看到套餐卡片 / 或套餐入口；
2. 然后是充值入口；
3. 再往下是兑换码入口和邀请。

同时用户后续确认：超值套餐应单独新开一个与钱包平级的页面；入口方案为主导航新增“超值套餐”，钱包页也保留醒目入口卡片跳转。

### 2.8 管理员订阅表单现状

实际文件：`web/default/src/features/subscriptions/lib/plan-form.ts`、`web/default/src/features/subscriptions/components/subscriptions-mutate-drawer.tsx`

当前管理员表单已经能配置：

- 标题、副标题、价格
- 总额度 `total_amount`，表单中用 `quotaUnitsToDollars` / `parseQuotaFromDollars` 在美元展示值与内部 quota 间转换
- 用户升级分组 `upgrade_group`
- 购买上限
- 时长
- 重置周期
- 启用状态
- 余额兑换开关
- Stripe / Creem / Waffo Pancake 商品 ID

当前缺少：

- 套餐类型 / 等级（日卡、周卡、月卡）
- 套餐模型分组
- 套餐启用后的并发限制
- 5 小时限额
- 7 天限额
- 权益说明
- 联动小铺商品链接或 LDXP 商品配置
- 是否属于“超值套餐”的区分字段

### 2.9 VIP 自动升级现状

实际文件：

- `model/topup.go`
- `model/topup_vip_upgrade_test.go`
- `model/redemption_topup_test.go`
- `model/subscription.go`
- `service/ldxp_verify.go`

当前已有：

- `const VIPUpgradeThresholdMoney = 30.0`
- `MaybeUpgradeUserToVIPTx`
- `MaybeUpgradeUserToVIP`
- `topUpVIPQualifiedAmount`

`MaybeUpgradeUserToVIPTx` 只升级普通角色且当前分组为空、`default` 或 `体验用户` 的用户；不会覆盖管理员、root、特殊分组或已是 VIP 的用户。

`topUpVIPQualifiedAmount` 优先使用 `TopUp.Amount`，没有 `Amount` 时使用 `TopUp.Money`。测试 `TestMaybeUpgradeUserToVIPUsesAmountForDiscountedLDXPTopup` 已覆盖 LDXP 折扣充值时按账面金额计入 VIP 的逻辑。

`CompleteSubscriptionOrder` 已调用 `upsertSubscriptionTopUpTx` 和 `MaybeUpgradeUserToVIPTx`，因此订阅订单金额当前也会进入 VIP 升级统计。

当前缺少的是前端“会员仪式感”状态：没有“弹窗只出现一次”的持久标记，也没有 VIP 边框呼吸光。

## 3. 非目标

第一版不做以下事情：

- 不重做所有订阅系统，只在现有订阅 / LDXP / 钱包能力上加超值套餐所需字段和流程。
- 不引入 Stripe、Creem、Epay、余额支付作为超值套餐首选支付路径。它们可保持现状用于普通订阅或钱包充值，但超值套餐第一版只走联动小铺 / LDXP。
- 不改变 `users.group` 的身份含义。VIP 仍通过现有 30 元有效充值逻辑升级；日卡 / 周卡 / 月卡不通过 `UpgradeGroup` 改用户分组。
- 不暂停套餐倒计时。启动 / 关闭只控制是否使用套餐权益；有效期从付款成功开始不可暂停。
- 不把套餐关闭理解为退款、冻结或延长有效期。
- 不删除或替换现有项目标识、版权和品牌信息。

## 4. 术语与状态

### 4.1 套餐类型

固定三种套餐类型：

| 类型 | 等级 | 默认模型分组建议 | 说明 |
| --- | ---: | --- | --- |
| 日卡 | 1 | `day-card` | 最低等级，短期使用 |
| 周卡 | 2 | `week-card` | 中等级 |
| 月卡 | 3 | `month-card` | 最高等级 |

等级顺序固定为：`日卡 < 周卡 < 月卡`。

### 4.2 用户身份分组 vs 模型分组

- 用户身份分组：`users.group`，例如 `体验用户`、`vip`。它决定用户身份和倍率规则。
- 模型分组：请求选择的 token group / using group，例如 `gpt-plus`、`gpt-pro`、未来的 `day-card`、`week-card`、`month-card`。它决定走哪个渠道分组与模型分组。

超值套餐只改变模型分组和套餐权益，不改变 `users.group`。

### 4.3 套餐实例状态

套餐实例建议至少包含：

- `active`：已付款且未过期；是否正在使用由单独开关决定。
- `expired`：已过期。
- `cancelled`：管理员取消。

用户使用状态建议单独表示：

- `package_enabled = true`：用户当前启用套餐使用，模型分组自动切到套餐分组，套餐并发和限额生效。
- `package_enabled = false`：用户关闭套餐使用，模型分组回到 API 余额分组，套餐时间继续倒计时。

不要把 `package_enabled=false` 写成 `cancelled` 或 `paused`，避免误解为倒计时暂停。

## 5. 用户侧信息架构

### 5.1 新增“超值套餐”页面

新增一个与钱包平级的普通用户页面：`超值套餐`。

建议前端落点：

- 新路由：`web/default/src/routes/_authenticated/value-packages/index.tsx`
- 新 Feature：`web/default/src/features/value-packages/`
- 侧边栏入口：`web/default/src/hooks/sidebar-data-model.ts`
- 图标映射：`web/default/src/hooks/use-sidebar-data.ts`

导航要求：

- 普通用户：能在侧边栏看到“超值套餐”。
- 管理员：也能在个人或通用区域看到“超值套餐”，同时管理员订阅管理入口继续保留。
- 钱包页：在充值入口上方放一个醒目的“超值套餐”入口卡片，点击跳转到新页面。

入口方案按用户确认采用 C：主导航新增“超值套餐”，钱包页也放醒目入口卡片。

### 5.2 钱包页调整

钱包页仍负责：

- 联动小铺 / LDXP 充值；
- 常规充值；
- 兑换码；
- 邀请 / 分销；
- 跳转“超值套餐”的入口。

钱包页展示顺序按用户要求调整为：

1. 超值套餐入口卡片；
2. 充值入口（联动小铺 / 常规充值）；
3. 兑换码入口；
4. 邀请 / 分销。

当前 `Wallet` 把 `SubscriptionPlansCard` 放在右侧，本次不应继续把日卡 / 周卡 / 月卡作为钱包右栏普通订阅卡展示。钱包里只保留跳转卡，真正套餐卡在“超值套餐”页面。

## 6. 超值套餐页面行为

### 6.1 三张卡片

页面固定展示三张主卡片：

- 日卡
- 周卡
- 月卡

每张卡片展示：

- 标题
- 介绍 / 副标题
- 价格
- 有效期
- 模型分组
- 并发限制
- 5 小时限额
- 7 天限额
- 权益说明
- 当前状态
- 剩余时间
- 操作按钮

### 6.2 卡片状态机

| 状态 | 条件 | 主按钮 | 说明 |
| --- | --- | --- | --- |
| 未购买 | 用户没有该类型有效套餐 | 购买 | 展示介绍和价格 |
| 已付款未启用 | 用户有该类型有效套餐，但 `package_enabled=false` | `▶ 启动` | 付款成功后立即进入此状态，倒计时已开始 |
| 使用中 | 用户有该类型有效套餐，且 `package_enabled=true` | 关闭使用 | 页面出现套餐黄色边缘光，模型分组切到套餐分组 |
| 已过期 | 当前套餐实例已过期 | 购买 | 恢复购买状态，可显示最近一次过期记录 |
| 被更高套餐覆盖 | 低级套餐被高级套餐购买覆盖 | 购买或置灰历史 | 剩余时间不折算、不顺延 |

付款成功后，购买按钮必须变为 `▶ 启动`，不能自动让用户误以为倒计时尚未开始。倒计时从付款成功开始。

### 6.3 启动操作

点击 `▶ 启动` 时同时触发：

1. 套餐使用开关打开；
2. 当前模型分组切换到该套餐配置的模型分组；
3. 套餐权益生效，包括并发限制、5 小时限额和 7 天限额；
4. 页面四周出现淡黄色边缘呼吸光。

启动后系统应提示：

- 当前已启用日卡 / 周卡 / 月卡；
- API 请求将优先使用对应套餐模型分组；
- 如需使用 API 余额，可关闭套餐使用。

### 6.4 关闭使用

点击“关闭使用”时同时触发：

1. 套餐使用开关关闭；
2. 套餐黄色边缘光消失；
3. 当前模型分组切回 API 余额分组；
4. 套餐权益暂停参与请求；
5. 套餐有效期继续倒计时。

关闭前或关闭后必须提示：

> 停止使用只会切回 API 余额分组，日卡 / 周卡 / 月卡的有效期仍会继续计算，无法暂停或顺延。

文案不能使用“暂停计时”“冻结套餐”这类容易误导的表达。

### 6.5 倒计时规则

- 付款成功时立刻记录 `start_time` 和 `end_time`。
- 倒计时无法暂停、无法打断。
- 启动 / 关闭只影响使用状态，不影响 `end_time`。
- 周卡和月卡关闭使用期间，时间仍继续计算。

## 7. 购买与覆盖规则

### 7.1 等级规则

等级固定为：

```text
日卡(1) < 周卡(2) < 月卡(3)
```

### 7.2 同级购买

同级购买允许，并直接叠加时间。

示例：

- 月卡还剩 20 天，再买一个月卡 30 天，则新的结束时间为当前结束时间 + 30 天，即剩余约 50 天。
- 如果同级套餐已过期，则从付款成功时间重新开始计算。

### 7.3 低级买高级

低级未过期时允许购买高级，但付款前必须弹确认框。

确认框文案方向：

- 标题：`将直接覆盖当前日卡`
- 内容：`你当前仍有未过期的日卡。购买月卡后，当前日卡会立即失效，剩余时间不会折算或顺延。是否确认购买？`
- 按钮：`取消` / `确认购买并覆盖`

确认后：

- 低级套餐立即失效或标记为被覆盖；
- 低级套餐剩余时间不折算、不顺延；
- 高级套餐从付款成功开始计时；
- 付款成功后高级套餐进入“已付款未启用”状态，按钮显示 `▶ 启动`。

### 7.4 高级买低级

高级套餐未过期时禁止购买低级套餐。

示例：

- 月卡未过期：不能购买日卡或周卡。
- 周卡未过期：不能购买日卡。

按钮可显示为禁用，并提示：

> 当前已有更高等级套餐未过期，暂不能购买低等级套餐。

### 7.5 购买失败和支付取消

- 支付会话创建失败：不创建有效套餐，不改变当前套餐状态。
- 用户取消 LDXP 支付会话：保持原状态。
- LDXP worker 失败 / 超时：不创建有效套餐，订单保留失败状态供管理后台排查。
- 支付成功但开通套餐失败：必须可重试结算，避免用户付款后权益丢失。

## 8. 支付设计

### 8.1 第一版只走联动小铺 / LDXP

超值套餐第一版支付只支持联动小铺 / LDXP。它应复用现有充值链路的用户体验：

1. 用户点击套餐购买；
2. 系统创建 LDXP 支付会话；
3. worker 拉取商品链接并生成二维码 / 支付页；
4. 用户完成付款；
5. 邮件或 worker 结果核验成功；
6. 系统开通 / 续费 / 覆盖套餐；
7. 金额写入有效充值统计，用于 VIP 升级；
8. 不增加钱包余额。

### 8.2 不得把套餐付款当作普通充值

当前 `service.CreateLdxpTopupSession` 只以充值金额查找商品，并最终可能关联 `TopupId` / `RedemptionId` 完成普通充值。超值套餐实现时必须增加业务类型区分，避免套餐付款成功后给用户加钱包余额。

建议新增或扩展：

- LDXP 会话业务类型：`purpose = topup | value_package`
- 套餐关联：`subscription_plan_id` 或 `value_package_plan_id`
- 订单号：能关联到 `SubscriptionOrder` 或新的套餐订单表
- 成功结算分支：
  - `topup`：保持当前充值逻辑；
  - `value_package`：创建 / 更新套餐实例，写入 `TopUp` 统计金额，但 `TopUp.Amount` 不增加用户余额。

如果复用 `SubscriptionOrder`，`PaymentProvider` 应记录为 `ldxp`，`PaymentMethod` 应记录为 `ldxp`，并在完成订单时走套餐专用创建逻辑。

### 8.3 套餐商品配置

后台需要允许为每个套餐配置联动小铺商品信息：

- 商品链接 / 商品 URL
- 商品名称
- 付款金额
- 可选商品 ID 或外部商品标识

用户已说明“联动小铺链接还没建”，所以后台字段必须允许先空着保存；用户端购买按钮在商品配置不完整时应禁用并提示“套餐商品暂未开放”。

## 9. 模型分组与 API 余额切换

### 9.1 不改变 `users.group`

超值套餐不得通过 `SubscriptionPlan.UpgradeGroup` 把用户身份分组改成日卡 / 周卡 / 月卡。原因：

- `users.group` 当前承担普通用户、体验用户、VIP 等身份分组；
- 现有倍率逻辑依赖 `UserGroup` 与 `UsingGroup` 组合；
- 用户已确认日卡 / 周卡 / 月卡只改变模型分组，不改变用户分组。

因此超值套餐计划中的 `upgrade_group` 应为空，或新增专用字段取代 `UpgradeGroup`。

### 9.2 套餐模型分组

每个套餐配置一个模型分组：

- 日卡：默认 `day-card`
- 周卡：默认 `week-card`
- 月卡：默认 `month-card`

这些模型分组需要进入后台可配置项，并且管理员必须能配置对应渠道 / 模型 / 倍率 / 可用性。

### 9.3 API 请求如何切换

当前 API token 有 `Token.Group`，默认空值会被规范化为 `gpt-plus`。Playground 请求可带 `group` 字段，并通过 `middleware.Distribute` 选择 `UsingGroup`。

超值套餐需要新增“当前套餐使用状态”对 API 请求的影响：

- 当 `package_enabled=true` 且套餐未过期：请求默认使用套餐模型分组。
- 当 `package_enabled=false`：请求回到 API 余额分组，默认仍是现有 `gpt-plus` / 用户 token 配置。
- 用户手动关闭套餐使用时，不应再用套餐订阅额度扣费。

实现上需要明确一个优先级：

1. 安全和 token model limits 仍必须生效；
2. 套餐启用时，默认模型分组改为套餐分组；
3. 如果客户端显式传了 `group`，是否允许覆盖套餐分组需要产品固定。

本规格建议：套餐启用期间，普通用户请求强制使用套餐模型分组；不允许通过请求体显式 `group` 绕到 `gpt-plus` / `gpt-pro` 使用 API 余额。需要使用 API 余额时，用户必须先在“超值套餐”页面关闭套餐使用。这样与用户“关闭日卡使用即可切换到 API 余额”的表达一致。

### 9.4 与现有 `billing_preference` 的关系

当前 `billing_preference` 是订阅 / 钱包扣费偏好。超值套餐的“启用 / 关闭”不应直接复用四种 `billing_preference` 字符串，否则会影响普通订阅或未来订阅余额包。

建议新增专用用户设置字段，例如：

- `active_value_package_subscription_id`
- `value_package_enabled`
- `api_balance_model_group`（可选，默认由 token group 或 `gpt-plus` 决定）

当用户点击启动：设置 `value_package_enabled=true` 并绑定当前套餐实例。

当用户点击关闭：设置 `value_package_enabled=false`，保留当前套餐实例和倒计时。

## 10. 限额与并发

### 10.1 后台可配置字段

管理员必须可以为每个套餐配置：

- 模型分组
- 并发限制
- 5 小时限额
- 7 天限额

并发限制要求：

- 值域：`1` 到 `2`
- 必填
- 只在套餐启用使用中时生效
- 关闭套餐使用 / 切回 API 余额后，不受套餐并发限制

限额要求：

- 后台输入单位为“美元 Token 额度”；
- 例如 5 小时限额输入 `100`，用户端显示 `$100 Token`；
- 例如 7 天限额输入 `500`，用户端显示 `$500 Token`；
- 后端可以继续使用内部 quota 计算，但所有后台表单和用户展示使用美元 Token 额度；
- 存储时应记录内部 quota 值或同时记录美元显示值，但转换必须沿用当前 `parseQuotaFromDollars` / `quotaUnitsToDollars` 这类模式，避免管理员直接接触内部 quota。

### 10.2 5 小时 / 7 天限额含义

- 5 小时限额：从当前请求时间向前滚动 5 小时，统计套餐启用期间使用该套餐权益消耗的 quota，不得超过配置额度。
- 7 天限额：从当前请求时间向前滚动 7 天，统计套餐启用期间使用该套餐权益消耗的 quota，不得超过配置额度。
- 两个限制同时存在，任一超限则拒绝套餐路径请求。
- 关闭套餐后走 API 余额，不计入套餐限额；但历史套餐使用量仍保留，用于用户再次启动后继续滚动统计。

### 10.3 与现有 `TotalAmount` / `QuotaResetPeriod` 的关系

当前 `SubscriptionPlan.TotalAmount` 是总额度，`QuotaResetPeriod` 是周期重置；它不能直接表达“5 小时限额 + 7 天限额”两个同时存在的滚动窗口。

建议新增专用字段，不要把 5 小时 / 7 天限额塞进 `QuotaResetPeriod`：

- `limit_5h_amount`：内部 quota，后台按美元 Token 输入转换；
- `limit_7d_amount`：内部 quota，后台按美元 Token 输入转换；
- 或者命名为 `limit_5h_quota` / `limit_7d_quota`，前端字段显示为 dollars。

`TotalAmount` 可继续用于“套餐总额度”或对超值套餐设为 `0` 表示无限；但本次需求的关键限制是滚动 5 小时和 7 天限额。

## 11. 页面边框光效

### 11.1 套餐黄色边缘呼吸光

用户澄清：黄色呼吸灯不是局部的“十字”或某个图标，而是整个页面视角的四周边缘光效。

视觉要求：

- 从页面四周散发淡黄色呼吸光；
- 不遮挡内容；
- 不影响点击、滚动、输入；
- 动画柔和，不能像报警灯；
- 在 `prefers-reduced-motion` 下应降级为静态边框光或关闭动画。

实现建议：

- 在 app root 或 body 上根据状态加 class，例如 `value-package-active`；
- 用 `::before` 或独立 fixed overlay 实现；
- 设置 `pointer-events: none`；
- 使用 inset shadow / radial-gradient / mask 制造边缘光；
- z-index 低于弹窗但高于页面背景。

### 11.2 VIP 会员边缘光

VIP 光效与套餐光效不同：

- VIP：金色 / 黑金的尊贵边框呼吸光；
- 套餐：淡黄色套餐使用中光效。

用户确认光效优先级方案 A：

1. 如果用户是 VIP 但未启用套餐：显示 VIP 金色 / 黑金边框光；
2. 如果用户是 VIP 且启用日卡 / 周卡 / 月卡：套餐黄色光效优先；
3. 关闭套餐使用后：恢复 VIP 金色 / 黑金光效。

## 12. 管理员后台

### 12.1 管理入口

管理员必须有完整管理入口，不能出现“并发设置不了”这种情况。

建议在现有 `Subscription Management` 中扩展“超值套餐”配置，或新增 admin tab：

- 普通订阅计划；
- 超值套餐（日卡 / 周卡 / 月卡）。

因为当前后端和前端已有 `SubscriptionPlan` 管理，优先建议在现有订阅管理中增加类型字段和筛选 tab，而不是新做一套完全独立的管理系统。这样可复用：

- `web/default/src/features/subscriptions/api.ts`
- `web/default/src/features/subscriptions/types.ts`
- `web/default/src/features/subscriptions/lib/plan-form.ts`
- `web/default/src/features/subscriptions/components/subscriptions-mutate-drawer.tsx`
- 后端 `controller/subscription.go`

### 12.2 管理员可配置项

每个超值套餐必须可配置：

- 套餐类型：日卡 / 周卡 / 月卡
- 套餐等级：1 / 2 / 3（可由类型自动带出，不建议管理员任意改乱）
- 标题
- 介绍
- 价格
- 有效期
- 联动小铺商品链接 / 商品名称 / 商品标识
- 模型分组
- 并发限制（1~2）
- 5 小时限额（美元 Token 额度）
- 7 天限额（美元 Token 额度）
- 权益说明
- 排序
- 启用 / 禁用

字段校验：

- 类型必须是 `day` / `week` / `month` 中之一；
- 同一类型建议只允许一个启用套餐；
- 并发限制必须是 1 或 2；
- 5 小时和 7 天限额不能为负；
- 7 天限额应大于或等于 5 小时限额；
- 模型分组不能为空；
- 价格不能为负；
- 有效期必须大于 0。

### 12.3 与现有字段的兼容

建议字段映射：

- `Title` / `Subtitle`：继续用于标题和介绍；
- `PriceAmount` / `Currency`：继续用于价格；
- `DurationUnit` / `DurationValue` / `CustomSeconds`：继续用于有效期；
- `Enabled` / `SortOrder`：继续用于上下架和排序；
- `TotalAmount`：可保留为总额度，第一版超值套餐可设为 `0` 表示不限总额；
- `UpgradeGroup`：超值套餐必须为空，不用于日卡 / 周卡 / 月卡。

需要新增字段承担：套餐类型、模型分组、并发、5 小时限额、7 天限额、权益说明、LDXP 商品配置。

## 13. VIP 会员仪式感旁支功能

### 13.1 触发条件

现有 VIP 升级逻辑保持：有效充值满 30 元自动升级 VIP。

有效金额来源包括：

- 普通充值成功的 `TopUp`；
- LDXP 充值成功的 `TopUp`；
- 兑换码中 `CountAsTopUp=true` 的 paid topup；
- 订阅 / 超值套餐付款成功后写入的 `TopUp`。

这与现有 `model/topup.go` 和 `model/subscription.go` 的设计一致。超值套餐付款成功后必须写入 `TopUp`，但不得增加钱包余额。

### 13.2 弹窗动效

当用户首次获得 VIP 权益时，弹出尊贵奢华的会员卡图片弹窗：

- 样式：黑金 / 金色 / 会员卡质感；
- 文案：`恭喜你获得会员权益`；
- 可以显示简短副文案：例如 `已为你切换至会员组，享受更优模型倍率`；
- 按钮：`我知道了`；
- 弹窗只出现一次。

对于之前已经升级会员的存量用户，也需要补上这个仪式感：他们下次登录或刷新时，如果是 VIP 且未看过该弹窗，也弹一次。

### 13.3 弹窗只出现一次

建议在 `dto.UserSetting` 增加持久标记，例如：

- `VipUpgradeModalSeen bool json:"vip_upgrade_modal_seen,omitempty"`

逻辑：

- 如果用户 `users.group == vip` 且 `vip_upgrade_modal_seen=false`，前端显示弹窗；
- 用户关闭弹窗后调用 API 写入 `vip_upgrade_modal_seen=true`；
- 之后不再显示。

如果要区分“本次刚升级”和“存量补仪式感”，可增加后端响应字段，但第一版不强制；前端只要根据 VIP + seen flag 即可覆盖两类用户。

### 13.4 VIP 边框灯效

VIP 用户应长期显示 VIP 专属边框呼吸灯效，除非当前启用了日卡 / 周卡 / 月卡套餐，此时套餐黄色光效优先。

状态优先级：

1. 套餐使用中：套餐黄色边缘光；
2. VIP 且套餐未使用中：VIP 黑金 / 金色边缘光；
3. 非 VIP 且套餐未使用中：无全局边缘光。

## 14. 数据模型建议

### 14.1 扩展 `SubscriptionPlan`

为减少系统分裂，建议扩展现有 `SubscriptionPlan`，而不是新建完全独立的 `ValuePackagePlan`。新增字段建议：

| 字段 | 类型建议 | 说明 |
| --- | --- | --- |
| `plan_kind` | `varchar(32)` | `subscription` 或 `value_package`，默认 `subscription` |
| `package_type` | `varchar(16)` | `day` / `week` / `month`，普通订阅为空 |
| `package_level` | `int` | day=1, week=2, month=3 |
| `model_group` | `varchar(64)` | 套餐启用后使用的模型分组 |
| `concurrency_limit` | `int` | 1~2 |
| `limit_5h_amount` | `bigint` | 内部 quota；后台以美元 Token 输入 |
| `limit_7d_amount` | `bigint` | 内部 quota；后台以美元 Token 输入 |
| `benefits` | `text` | 权益说明，可存 JSON 数组或换行文本 |
| `ldxp_product_url` | `text` | 联动小铺商品链接 |
| `ldxp_product_name` | `text` | 联动小铺商品名称 |
| `ldxp_product_ref` | `varchar(128)` | 可选外部商品 ID |

默认值要求：

- `plan_kind` 默认 `subscription`，保证现有数据兼容；
- `package_type` 默认空；
- `package_level` 默认 0；
- `model_group` 默认空；
- `concurrency_limit` 默认 1；
- `limit_5h_amount` / `limit_7d_amount` 默认 0，表示不限制或未配置，具体含义需在 UI 明确；
- LDXP 商品字段默认空。

### 14.2 扩展 `UserSubscription` 或新增状态表

现有 `UserSubscription` 可继续代表用户购买的套餐实例，但需要表示：

- 是否被覆盖；
- 当前是否启用使用；
- 当前使用的模型分组。

建议不要把“当前启用使用”存到每条历史订阅上，而是存用户设置或新建用户套餐状态表，因为同一用户同一时间只能启用一个套餐。

推荐新增表：`user_value_package_preferences`

| 字段 | 类型建议 | 说明 |
| --- | --- | --- |
| `id` | int | 主键 |
| `user_id` | int unique | 用户 |
| `enabled` | bool | 是否启用套餐使用 |
| `active_user_subscription_id` | int | 当前启用的套餐实例 |
| `api_balance_group` | varchar(64) | 关闭套餐后默认 API 余额分组，可选 |
| `created_at` | bigint | 创建时间 |
| `updated_at` | bigint | 更新时间 |

也可以把这些字段放进 `users.setting`，但并发和后端请求路径需要频繁读，独立表更清晰、便于加索引和行锁。

### 14.3 记录被覆盖状态

`UserSubscription.Status` 当前有 `active`、`expired`、`cancelled` 语义。为低级套餐被高级覆盖，建议增加状态：

- `covered`

或新增字段：

- `covered_by_subscription_id`
- `covered_time`

推荐新增状态 `covered` + 可选 `covered_by_subscription_id`，便于展示历史和追踪。

### 14.4 滚动限额统计

5 小时 / 7 天限额需要基于实际用量滚动统计。现有 `SubscriptionPreConsumeRecord` 只记录预扣请求，不包含结算后最终 delta 的完整窗口统计语义，且没有套餐类型字段。

可选方案：

1. 复用现有日志 / quota data 表按 user + subscription + time 聚合；
2. 新增 `value_package_usage_records` 表，写入每次套餐结算后的实际 quota、时间、subscription_id、plan_id、model_group；
3. 扩展 `SubscriptionPreConsumeRecord`，增加 `PlanId`、`ModelGroup`、`SettledAmount`、`SettledAt`。

推荐方案 2：新增专用 usage records，边界最清楚，便于滚动窗口查询和测试。

### 14.5 三数据库兼容

所有新增字段和表必须兼容：

- SQLite
- MySQL >= 5.7.8
- PostgreSQL >= 9.6

要求：

- 优先使用 GORM AutoMigrate；
- SQLite 新增字段走 `ALTER TABLE ... ADD COLUMN` 模式，参考 `model/main.go` 的 `ensureSubscriptionPlanTableSQLite`；
- 不使用 JSONB、PostgreSQL-only operator、MySQL-only function；
- `benefits` 如需结构化，存 `TEXT`，通过 `common.Marshal` / `common.Unmarshal` 处理；
- raw SQL 中涉及保留字继续使用 `commonGroupCol` / `commonKeyCol` 等现有模式。

## 15. API 设计建议

### 15.1 用户端 API

建议新增超值套餐专用 API，避免污染现有通用订阅 API：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/value-packages/plans` | 获取三张套餐卡及当前用户状态 |
| `GET` | `/api/value-packages/self` | 获取当前有效套餐、启用状态、剩余时间、限额使用情况 |
| `POST` | `/api/value-packages/:plan_id/ldxp/session` | 创建套餐 LDXP 付款会话 |
| `GET` | `/api/value-packages/ldxp/session/:session_id` | 查询套餐付款会话 |
| `POST` | `/api/value-packages/ldxp/session/:session_id/cancel` | 取消套餐付款会话 |
| `POST` | `/api/value-packages/activate` | 启动指定套餐实例 |
| `POST` | `/api/value-packages/deactivate` | 关闭套餐使用，切回 API 余额 |
| `POST` | `/api/value-packages/purchase-intent` | 可选：检查购买覆盖 / 降级规则，返回是否需要确认 |

如果为了减少路由，也可放在 `/api/subscription/value-packages/*` 下；但不建议继续把购买按钮接到 `/api/subscription/waffo-pancake/pay`，因为支付方式和语义不同。

### 15.2 管理员 API

现有管理员订阅计划 API 可以扩展，或新增筛选：

- `GET /api/subscription/admin/plans?plan_kind=value_package`
- `POST /api/subscription/admin/plans`
- `PUT /api/subscription/admin/plans/:id`

如果新增专用路由，建议：

- `GET /api/value-packages/admin/plans`
- `POST /api/value-packages/admin/plans`
- `PUT /api/value-packages/admin/plans/:id`
- `PATCH /api/value-packages/admin/plans/:id`

无论采用哪种，返回 DTO 必须包含所有管理员可配置项，尤其并发、5 小时限额、7 天限额、LDXP 商品配置。

### 15.3 VIP 弹窗 API

建议扩展现有 `GET /api/user/self` 返回：

- `group` 已有；
- `setting.vip_upgrade_modal_seen` 或等价字段；
- 可选派生字段：`should_show_vip_upgrade_modal`。

新增或复用用户设置更新 API：

- `PUT /api/user/setting`：写入 `vip_upgrade_modal_seen=true`

如果现有 `UpdateUserSetting` 已支持完整 setting 更新，前端要避免覆盖其他 setting 字段。

## 16. 后端业务流程

### 16.1 购买前检查

创建支付会话前执行：

1. 读取目标套餐计划；
2. 校验 `plan_kind=value_package`、`enabled=true`；
3. 校验 LDXP 商品配置完整；
4. 查询用户未过期套餐实例；
5. 根据等级规则判断：
   - 同级：允许；
   - 低级买高级：需要客户端已确认覆盖；
   - 高级买低级：拒绝；
6. 创建待支付订单和 LDXP session。

### 16.2 付款成功结算

LDXP 核验成功后执行：

1. 锁定订单；
2. 确保幂等，已成功则直接返回；
3. 重新读取当前用户有效套餐并执行等级规则；
4. 同级购买：把现有同级有效套餐 `end_time` 延长；
5. 低级买高级：把低级套餐标记 `covered`，创建高级套餐；
6. 写入 `TopUp`，金额计入 VIP 升级；
7. 调 `MaybeUpgradeUserToVIPTx`；
8. 标记订单成功；
9. 不增加用户钱包 quota；
10. 返回前端后卡片显示 `▶ 启动`。

### 16.3 启动套餐

启动时：

1. 校验套餐实例属于当前用户；
2. 校验未过期、未取消、未被覆盖；
3. 写入用户套餐偏好 `enabled=true` 和 active subscription id；
4. 后续 API 请求默认使用套餐模型分组；
5. 并发 / 限额开始对套餐请求生效。

### 16.4 关闭套餐

关闭时：

1. 写入用户套餐偏好 `enabled=false`；
2. 不修改 `user_subscriptions.end_time`；
3. 不修改 `user_subscriptions.status`；
4. 后续 API 请求回到 API 余额分组。

### 16.5 过期处理

现有 `ExpireDueSubscriptions` 会把过期 active 订阅标记为 `expired`，并处理 `UpgradeGroup` 回退。超值套餐的 `UpgradeGroup` 应为空，因此不应触发用户身份回退。

当当前启用的套餐过期：

- 后端请求路径应自动视为未启用套餐；
- 用户偏好中的 `enabled` 可以保留 false 或由定时任务清理；
- 前端显示已过期，按钮恢复购买；
- 全局套餐黄色边缘光消失；
- 如果用户是 VIP，恢复 VIP 边缘光。

## 17. 前端实现建议

### 17.1 default 前端优先

本次用户界面讨论均发生在 `web/default` 的本地页面中，第一实现目标应为 default 前端。

建议新增：

- `web/default/src/features/value-packages/index.tsx`
- `web/default/src/features/value-packages/api.ts`
- `web/default/src/features/value-packages/types.ts`
- `web/default/src/features/value-packages/components/value-package-card.tsx`
- `web/default/src/features/value-packages/components/value-package-payment-dialog.tsx`
- `web/default/src/features/value-packages/components/value-package-status-banner.tsx`
- `web/default/src/features/value-packages/hooks/use-value-packages.ts`
- `web/default/src/features/value-packages/lib/rules.ts`
- `web/default/src/routes/_authenticated/value-packages/index.tsx`

建议改造：

- `web/default/src/features/wallet/index.tsx`：替换当前订阅右栏为超值套餐跳转卡，按新顺序调整布局。
- `web/default/src/hooks/sidebar-data-model.ts`：添加普通用户和管理员可见入口。
- `web/default/src/hooks/use-sidebar-data.ts`：补图标，例如 `BadgePercent`、`Crown` 或 `Sparkles`。
- `web/default/src/features/subscriptions/*`：补管理员字段。
- `web/default/src/styles/index.css`：加入全局边缘光样式。
- `web/default/src/i18n/locales/{en,zh,fr,ja,ru,vi}.json`：新增 UI 文案翻译。

### 17.2 管理员表单

现有 `plan-form.ts` 已把 `total_amount` 在 dollars 和 quota 间转换。新增 `limit_5h_amount`、`limit_7d_amount` 应复用同一转换思路：

- 表单值：美元 Token 额度数字；
- API payload：内部 quota；
- 编辑回填：内部 quota 转美元 Token 额度。

表单 UI 分组建议：

1. 基础信息；
2. 套餐类型；
3. 有效期和价格；
4. 联动小铺商品配置；
5. 模型分组和并发；
6. 5 小时 / 7 天限额；
7. 权益说明；
8. 上架状态。

### 17.3 VIP 弹窗和全局光效

建议新增全局状态组件，挂在 authenticated layout 内：

- 读取 `getSelf` 或新增 profile API；
- 读取 value package self 状态；
- 判断光效优先级；
- 负责展示 VIP 弹窗；
- 负责给 root/body 加 class。

落点可在：

- `web/default/src/components/layout/components/authenticated-layout.tsx`

或新增：

- `web/default/src/features/membership/vip-upgrade-celebration.tsx`
- `web/default/src/features/app-effects/global-border-glow.tsx`

### 17.4 classic 前端

仓库仍有 `web/classic`，并存在 classic 订阅和充值组件。第一版可先保证 default 前端完整；如果项目发布仍支持 classic，需要至少做到：

- 不破坏 classic 构建；
- classic 后台不因新增字段报错；
- 若 classic 仍面向用户，需要补相同入口或明确隐藏超值套餐。

## 18. 测试计划

### 18.1 后端单元测试

新增或扩展测试：

- `model/subscription.go` 等级规则：
  - 同级购买叠加时间；
  - 日卡买月卡需要确认覆盖；
  - 月卡未过期禁止买日卡；
  - 覆盖后低级剩余时间不折算。
- LDXP 套餐订单：
  - 成功付款创建套餐但不增加钱包余额；
  - 写入 `TopUp` 并计入 VIP 升级；
  - 重复回调幂等；
  - 支付失败不创建套餐。
- 启动 / 关闭：
  - 启动只改使用状态，不改 `users.group`；
  - 关闭只改使用状态，不改 `end_time`；
  - 过期后自动不再作为启用套餐。
- 限额：
  - 5 小时窗口超限拒绝；
  - 7 天窗口超限拒绝；
  - 关闭套餐后 API 余额请求不计入套餐限额。
- 并发：
  - 并发限制 1；
  - 并发限制 2；
  - 关闭套餐后不受套餐并发限制。
- VIP 弹窗状态：
  - VIP 且未看过返回应显示；
  - 看过后不再显示；
  - 存量 VIP 也显示一次。

### 18.2 三数据库兼容测试

至少覆盖 SQLite 自动迁移；如果 CI 支持 MySQL / PostgreSQL，应增加迁移和基础 CRUD 测试。

重点检查：

- 新字段默认值；
- SQLite `ensureSubscriptionPlanTableSQLite` 能为旧表补字段；
- raw SQL 没有数据库方言问题；
- JSON 文本字段用 `common.Marshal` / `common.Unmarshal`。

### 18.3 前端测试

建议测试：

- `value-packages/lib/rules.ts`：购买规则、状态机、剩余时间显示；
- 卡片状态：未购买 / 已付款未启用 / 使用中 / 过期 / 禁止低级购买；
- 低级买高级确认弹窗；
- 关闭使用提示文案；
- 钱包页排序：超值套餐入口在充值前；
- VIP 与套餐光效优先级；
- 管理员表单 dollars ↔ quota 转换。

运行命令按项目约定使用 Bun：

```bash
cd web/default
bun run lint
bun run test
bun run build
```

后端建议：

```bash
go test ./model ./service ./controller ./middleware
```

实现时可按改动范围进一步缩小或扩大测试命令。

## 19. 迁移与发布注意事项

- 新增字段必须有安全默认值，保证旧订阅计划仍按普通订阅运行。
- 旧 `SubscriptionPlan` 需要默认 `plan_kind=subscription`。
- 超值套餐上线前，管理员需要创建或补齐三张套餐，并配置 LDXP 商品链接。
- 商品链接没建好时，用户端应展示“暂未开放”而不是发起失败支付。
- 现有钱包充值和现有订阅订单不能受影响。
- VIP 存量用户弹窗只出现一次，不能每次登录重复打扰。
- 全局边缘光必须不挡点击，避免影响支付弹窗、侧边栏、下拉框等交互。

## 20. 已确认决策汇总

- 新增“超值套餐”用户页面，与钱包平级。
- 主导航新增“超值套餐”，钱包页也放醒目跳转入口。
- 页面固定三张卡：日卡、周卡、月卡。
- 付款成功后按钮变为 `▶ 启动`，不是自动暂停倒计时。
- 套餐倒计时从付款成功开始，无法暂停或打断。
- 启动 / 关闭只控制灯效、模型分组切换、权益开关。
- 关闭使用时切回 API 余额分组，但套餐时间继续计算。
- 套餐支付第一版只走联动小铺 / LDXP，并复用充值支付体验。
- 套餐付款金额计入 30 元 VIP 升级有效金额，但不增加钱包余额。
- 同级购买叠加时间。
- 低级买高级允许，但付款前提示会覆盖低级剩余时间。
- 高级买低级禁止。
- 套餐只改变模型分组，不改变 `users.group`。
- 后台限额输入使用“美元 Token 额度”。
- 并发限制后台必须可设置，范围 1~2。
- 套餐使用中显示整个页面四周淡黄色边缘呼吸光。
- VIP 显示黑金 / 金色边缘呼吸光；套餐光效优先于 VIP 光效。
- VIP 弹窗只出现一次，存量 VIP 也补一次。

## 21. 实现前必须再次核对的代码点

写实现计划前应再次核对以下文件，确保没有在其他分支或新提交中变化：

- `model/subscription.go`
- `model/main.go`
- `model/topup.go`
- `model/ldxp_topup.go`
- `service/ldxp_session.go`
- `service/ldxp_verify.go`
- `service/billing_session.go`
- `service/funding_source.go`
- `middleware/distributor.go`
- `router/api-router.go`
- `controller/subscription.go`
- `controller/ldxp_topup.go`
- `web/default/src/features/wallet/index.tsx`
- `web/default/src/features/wallet/components/subscription-plans-card.tsx`
- `web/default/src/features/subscriptions/lib/plan-form.ts`
- `web/default/src/features/subscriptions/components/subscriptions-mutate-drawer.tsx`
- `web/default/src/hooks/sidebar-data-model.ts`
- `web/default/src/hooks/use-sidebar-data.ts`
