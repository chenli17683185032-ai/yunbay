# SVIP 身份标识

SVIP 不是用户组（与 VIP 的 `group=vip` 不同），而是纯身份标识：不影响分组、倍率与计费，用于彰显身份与提示管理员。

## 触发条件

用户「有效充值」累计 ≥ **200 元**。持久化与判定的权威阈值是 `model/svip.go` 的 `SVIPThresholdCents = 20_000`；金额常量只用于表达业务含义，不能替代整数分判定。

有效充值以「分」为单位累计在 `users.valid_topup_cents`；`users.valid_topup_history_cents` 记录其中已核算的可靠流水水位，仅用于启动对账，不对外暴露。有效充值只包含三类来源：

| 来源 | 累计位置 | 金额口径 |
| --- | --- | --- |
| 联动小铺充值（卡密兑换分支） | `model/redemption.go` `redeemWithTx`（仅 `paid_topup` 且 `count_as_topup`） | 面值 `Amount`（元） |
| 联动小铺充值（直充分支） | `service/ldxp_verify.go` `directTopUpLdxpSessionTx`（仅新建 TopUp 分支） | 面值 `session.Amount`（元） |
| 超值套餐购买 | `model/subscription.go` `CompleteValuePackageOrder`（仅 pending→success） | 实付 `order.Money`（元） |
| 管理员增加余额（需勾选开关） | `controller/user.go` `ManageUser` `add_quota/add` + `count_as_valid_topup` | `value / QuotaPerUnit`（元） |

**不计入**：Stripe / 易支付 / Creem / Waffo（非人民币口径）、活动赠送码（`promo_credit`）、套餐兑换码开通、签到、邀请返利、管理员「覆盖」额度。

历史数据回填：`model/svip.go` `backfillUserValidTopupCents` 在启动迁移时从 `top_ups` 汇总 provider 为 `ldxp` / `redemption_code` 的 success 流水。首次运行用 `SVIPValidTopupReconcileReceipt` 在同一事务内建立水位；后续启动只把历史总额相对 `valid_topup_history_cents` 的增量补入累计，因此能修复回滚到旧版本期间漏记的充值，同时保留管理员额外计入的金额。历史管理员手动加款没有流水且无法识别，仍不推断回填。单用户汇总或补差溢出会终止并回滚全批次。

## 接口

- `GET /api/user/self` 新增 `valid_topup_cents`、`is_svip`。
- `POST /api/user/svip-celebration/seen`：标记庆祝弹窗已读（写入用户 setting `svip_celebration_seen`，仅 SVIP 可调用）。VIP 与 SVIP 已读接口都只更新 `setting` 列，避免旧用户快照覆盖并发变化的额度或有效充值累计。
- `POST /api/user/manage`：`action=add_quota, mode=add` 新增可选 `count_as_valid_topup`（写入审计日志）。管理员加额度与有效充值累计通过同一条用户行更新原子写入，任一字段失败都不会留下半更新。

## 前端（web/default）

- **黑金卡弹窗**：`features/value-packages/components/svip-celebration-dialog.tsx`，复用重置卡的 motion 翻卡（rotateY 90→0 + spring + `transformPerspective`），达标且未读时在 `authenticated-benefit-effects.tsx` 自动弹出一次；SVIP 弹窗优先于 VIP 升级弹窗。
- **光效**：`styles/index.css` `.yunbay-viewport-benefit-glow--svip`（香槟金主光 + 深金暗晕 + 暗色 vignette，节奏更慢更沉），优先级 svip > package > vip（`benefit-effects.ts` `getBenefitGlowMode`）。卡面流光 `.yunbay-svip-card-shine`。均含 reduced-motion 降级。
- **充值卡提示**：`features/wallet/components/svip-topup-perk-alert.tsx`——「已到 SVIP，充值尊贵再享 75 折」+「充值后管理员核对，将把优惠发送到你的账上」，挂在 `RechargeFormCard` 与 `LdxpTopupCard`。75 折优惠本身由管理员核对后手动发放，系统不做自动折扣。
- **即时状态同步**：钱包每次读取权威 `/api/user/self` 后同步全局登录用户状态；联动小铺直充或付费充值码刚好跨过门槛时，无需整页刷新即可显示庆祝弹窗、黑金光效和充值提示。
- **管理端**：加余额弹窗（`user-quota-dialog.tsx`）「添加」模式下有「计入有效充值」开关；用户列表分组列显示黑金 SVIP 徽章（按 `valid_topup_cents` 判定，阈值常量 `SVIP_THRESHOLD_CENTS` 与后端一致）。

web/classic 未实现 SVIP 展示（与近期功能开发一致，主力前端为 default）。

## 测试

- `model/svip_test.go`：换算、阈值、累计、回填（含来源过滤、初始化凭证、回滚窗口补差、管理员额外累计保留与幂等）。
- `controller/user_svip_test.go`：已读接口权限、管理员开关计入/不计入。
- `web/default/src/features/value-packages/lib/benefit-effects.test.ts`：光效优先级、SVIP 判定、弹窗 seen 逻辑。
- 全新 SQLite 浏览器验收：桌面与 390×844 移动端覆盖庆祝弹窗、充值提示、用户徽章、管理员开关；验证已读持久化、刷新不重复弹出、reduced-motion 动画关闭和浏览器控制台无错误。
