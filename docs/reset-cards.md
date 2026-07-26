# 重置卡兑换与赠送维护说明

> 2026-07-26 新增。覆盖：重置卡兑换码、套餐开通赠送重置卡、前端赠卡动画。

## 业务目标

- 把「重置卡」并入兑换码系统：管理员可批量生成重置卡兑换码，用户在「充值」或「超值套餐」任一兑换入口输入即可到账。
- 超值套餐可配置「开通赠送重置卡张数」：每次成交（新开、续费、升级，含兑换码开通与 LDXP 购买）都会按套餐配置赠送。
- 用户开通套餐或兑换重置卡时，前端弹出卡片动画告知「赠送 N 张重置卡，在您额度用完的时候可以使用」。

## 数据模型

- 重置卡余额沿用既有 `user_value_package_preferences.reset_count`（按用户维度，跨套餐共享），消费入口不变（`POST /api/value-packages/reset-quota`）。
- `redemptions` 新增列 `reset_card_count int default 0`（AutoMigrate 覆盖三库）。
- `subscription_plans` 新增列 `gift_reset_count int not null default 0`：
  - MySQL/PG 走 AutoMigrate；
  - SQLite 在 `model/main.go` 的 `ensureSubscriptionPlanTableSQLite`（建表 SQL + required 列清单）和 `model/subscription.go` 的 `ensureSubscriptionPlanValuePackageColumnsTx` 中补列。
- `subscription_orders` 新增列 `gift_reset_count int not null default 0`：创建 LDXP 套餐订单时快照套餐赠卡数；MySQL/PG 走 AutoMigrate，SQLite 由 `ensureSubscriptionOrderTableSQLite` 为旧表补列。
- 台账 `value_package_reset_count_ledgers` 新增两个 Source：
  - `redemption`：兑换重置卡兑换码发放；
  - `plan_gift`：开通/续费套餐按套餐配置赠送。

## 兑换码类型

`redemptions.type` 现有三种：

| type | 说明 | 关键字段 |
| --- | --- | --- |
| `quota` | 余额/充值码 | `quota`、`kind`、`amount`、`money` |
| `subscription` | 套餐开通码 | `plan_id` |
| `reset_card` | 重置卡码 | `reset_card_count`（1-100，创建时默认 1） |

创建校验在 `model/redemption.go` `validateRedemptionPlanForCreate`：`reset_card` 强制清零 `quota/plan_id/amount/money/count_as_topup`，kind 固定 `promo_credit`；张数越界返回 `redemption.reset_card_count_invalid`（i18n 三语已加）。

## 兑换与赠送流程

- 唯一兑换入口不变：`POST /api/user/topup` → `model.Redeem` → `redeemWithTx`。
  - `reset_card` 分支：CAS 领码后在同事务内 `grantValuePackageResetCountTx` 加卡并写台账（Source=`redemption`）。
  - `subscription` 分支：套餐为超值套餐且 `gift_reset_count > 0` 时，经 `ensureValuePackagePreferenceAfterPurchaseTx`（`model/subscription.go`）统一赠卡（Source=`plan_gift`）。
- LDXP 创建支付订单时把套餐赠卡数写入 `subscription_orders.gift_reset_count`；`CompleteValuePackageOrder` 使用订单快照收口到同一 `ensureValuePackagePreferenceAfterPurchaseTx`。因此新开/续费/升级每次成交都赠，且支付期间管理员改配置不会造成实际到账与前端提示不一致。
- 响应契约：`data` 对 `subscription`/`reset_card` 返回完整 `RedeemResult`（新增 `reset_card_count`、`gift_reset_count` 字段），`quota` 仍返回裸数字（兼容 classic 主题）。

## 管理后台操作

- 兑换码管理 → 新建：兑换类型选「重置卡」，填写每码张数（1-100）与生成数量即可批量生成；列表类型徽章显示「重置卡兑换码」，额度列显示张数。
- 套餐编辑抽屉（订阅管理 → 超值套餐配置区）：新增「开通赠送重置卡」数字字段。
- 超值套餐管理页（`/order-management/value-packages`）：新增「开通赠送重置卡」设置卡，可直接为各套餐改赠卡张数（走 `PATCH /api/subscription/admin/plans/:id` 局部更新，避免覆盖套餐其他并发修改）。
- 后端校验：仅超值套餐可配置赠卡（普通订阅套餐强制 0），上限 100（`model.MaxSubscriptionPlanGiftResetCount`）。

## 前端交互

- 赠卡动画组件 `web/default/src/components/reset-card-gift-dialog.tsx`（motion/react 翻卡入场），三处触发：
  1. 兑换重置卡码成功（两个兑换入口共用 `use-redemption.ts`）；
  2. 兑换套餐码成功且套餐配置了赠卡（读响应 `gift_reset_count`）；
  3. LDXP 购买支付成功轮询到 `success`（读取服务端返回的订单 `gift_reset_count` 快照，`use-value-packages.ts`）。
- 文案 i18n key：`You received {{count}} reset card(s)!` / `Bonus: {{count}} reset card(s)!` / `You can use it to reset your package quota when it runs out.`（zh 已翻译）。

## 本地验证

```bash
go test ./model/ ./controller/
cd web/default && bun run typecheck
cd web/default && bun test ./src/features/redemption-codes/lib/redemption-form.test.ts ./src/features/subscriptions/lib/plan-form-value-package.test.ts
```

模型层测试：`model/redemption_reset_card_test.go`（兑换发放、二次兑换拒绝、坏码回滚、套餐赠卡、续期赠卡、普通套餐不赠、创建校验矩阵）。

## 回滚注意

- 新列均带默认值，回滚旧版本代码不需要删列；已发放的 `reset_count` 与台账不受影响。
- 若需停用赠送：把各套餐 `gift_reset_count` 置 0 即可，无需回滚代码。
- 兑换码类型对旧版本代码不可识别（`normalizeRedemptionType` 返回空 → 兑换报“无效的兑换码”），回滚前应先禁用未使用的重置卡码。
