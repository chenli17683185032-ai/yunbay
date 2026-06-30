# 云贝全量上线防覆盖总设计

**日期：** 2026-06-30
**项目：** 云贝 new-api 维护上线
**状态：** 已完成全量上线生产同步与公开侧复核（2026-06-30）；本文保留为防覆盖设计与上线完成记录
**范围：** 除 Jeepay / Alipay 充值后台配置与钱包流程、Sub2API usage billing 真源之外的所有待全面上线功能

---

## 1. 背景与目标

近期多个功能线分别完成过开发、灰测、热修或生产同步，但它们散落在不同分支和文档记录中。最新支付系统灰测上线时，部分文件存在被旧本地代码覆盖或被功能分支互相覆盖的风险。用户要求：

1. 不只处理 LDXP 自动充值，而是把所有“还没全面上线”的功能统一推进上线。
2. 以下两个功能本轮明确不处理：
   - Jeepay / Alipay 充值后台配置与钱包流程；
   - Sub2API usage billing 真源。
3. 已上线功能和即将上线功能不得交叉影响、不得互相覆盖。
4. LDXP 自动充值的 worker 主支付确认流程不得随意改动，避免“用户已支付但 worker 拿不到支付结果”。
5. LDXP 金额口径按用户最后确认执行：
   - 用户实际支付价进入 `top_ups.money`，作为推荐返利基数；
   - 账面档位进入 `top_ups.amount`，作为到账额度和 VIP 累计口径；
   - 手续费只作为验单容忍或平台外额外成本，不进入到账、VIP、返利。

本次目标是形成一个可审计、可分批、可回滚的全量上线基线：先收敛代码线，再分批放开功能，所有批次都带独立验证与回滚边界。

---

## 2. 明确排除范围

以下内容本轮不实施、不迁移、不灰测、不改生产配置；如果候选分支中包含相关改动，必须在摘取代码时剔除。

### 2.1 Jeepay / Alipay 充值后台配置与钱包流程

排除内容包括但不限于：

- `origin/codex/jeepay-alipay-admin`；
- Jeepay 后台配置页；
- Jeepay / Alipay 钱包充值流程；
- Jeepay 相关后端 controller、model、service、路由、配置项、i18n；
- 将钱包主充值入口切到 Jeepay 的前端变更。

### 2.2 Sub2API usage billing 真源

排除内容包括但不限于：

- `/api/sub2api/billing*`；
- 前端 usage logs 改为请求 Sub2API billing 真源；
- Sub2API billing / reconcile / usage truth adapter；
- `infra/sub2api/**` 当前工作区已有改动；
- 任何把 common usage logs 统计来源切到 Sub2API 的改动。

允许保留的内容：

- 现有 usage logs 空统计修复、筛选兼容、前端稳定显示；
- `Sub2API` 作为普通上游渠道的既有能力；
- 与本轮排除项无关的 `docs/yunbay-maintenance.md` 历史说明。

---

## 3. 当前证据与候选分支

### 3.1 当前工作区事实

当前仓库路径：

```text
/Users/ethan/Documents/yunbay
```

当前分支：

```text
codex/fix-usage-logs-stat-null
```

当前工作区已有未提交改动，执行代码上线时必须使用隔离 worktree 或独立分支，不能 reset、clean 或覆盖这些文件：

```text
/Users/ethan/Documents/yunbay/docs/yunbay-maintenance.md
/Users/ethan/Documents/yunbay/infra/cloudflare-plan.md
/Users/ethan/Documents/yunbay/infra/sub2api/backend/internal/server/middleware/api_key_auth.go
/Users/ethan/Documents/yunbay/infra/sub2api/backend/internal/server/middleware/api_key_auth_google.go
/Users/ethan/Documents/yunbay/infra/sub2api/backend/internal/server/middleware/api_key_auth_google_test.go
/Users/ethan/Documents/yunbay/infra/sub2api/backend/internal/server/middleware/api_key_auth_test.go
/Users/ethan/Documents/yunbay/docs/email-delivery.md
/Users/ethan/Documents/yunbay/docs/superpowers/specs/2026-06-27-user-tags-model-groups-design.md
/Users/ethan/Documents/yunbay/docs/superpowers/specs/2026-06-29-required-announcement-modal-design.md
/Users/ethan/Documents/yunbay/docs/superpowers/specs/2026-06-30-claude-pricing-wallet-gray-design.md
```

### 3.2 候选分支与用途

| 分支 | 本轮用途 | 关键提交 |
|---|---|---|
| `codex/user-tags-model-groups` | 用户标签与模型分组分离、注册策略、邀请统计、模型分组可见性 | `eb513ef8`、`0f0ac266`、`a1a38836` |
| `codex/deploy-ldxp-card-redemption` | LDXP 卡密兑换、paid_topup/promo_credit、批量导出、兑换后刷新修复、paid_topup VIP 热修 | `71aa0e32`、`54cd3e16`、`10dca0ee`、`04e8418f` |
| `codex/ldxp-products-affiliate-withdrawal` | LDXP 自动充值正式商品、worker 稳定修复、取消中断、推荐返利、提现 | `d37ccd21`、`c66516cc`、`16565cd6`、`bb6052b2`、`855743c8`、`c231e7bc` |
| `codex/claude-pricing-wallet-gray` | Claude 定价 preset/灰测、钱包灰测修复、Quick Start 恢复、注册页邀请码/QQ/default token、钱包推荐返利卡片恢复 | `1ea932fd`、`0ce58397`、`38619d6d`、`1e021bdf` |
| `codex/fix-usage-logs-stat-null` | usage logs 稳定基线、签到金额归一化 | `b7e913e0`、`ff849d72` |

### 3.3 已有设计/计划文档

本总设计不替代已有 per-feature 文档，而是作为全量上线协调层。执行时应按本总设计处理排除项、边界和批次；具体实现细节优先参考对应 feature 文档。

| 功能 | 设计文档 | 计划文档 |
|---|---|---|
| 用户标签与模型分组分离 | `/Users/ethan/Documents/yunbay/docs/superpowers/specs/2026-06-27-user-tags-model-groups-design.md` | `/Users/ethan/Documents/yunbay/docs/superpowers/plans/2026-06-27-user-tags-model-groups.md` |
| LDXP 卡密兑换 | `/Users/ethan/Documents/yunbay/docs/superpowers/specs/2026-06-27-ldxp-card-redemption-design.md` | `/Users/ethan/Documents/yunbay/docs/superpowers/plans/2026-06-27-ldxp-card-redemption.md` |
| LDXP 自动充值 worker | `/Users/ethan/Documents/yunbay/docs/superpowers/specs/2026-06-28-ldxp-browser-worker-auto-topup-design.md` | `/Users/ethan/Documents/yunbay/docs/superpowers/plans/2026-06-28-ldxp-browser-worker-auto-topup.md` |
| 必读公告弹窗 | `/Users/ethan/Documents/yunbay/docs/superpowers/specs/2026-06-29-required-announcement-modal-design.md` | `/Users/ethan/Documents/yunbay/docs/superpowers/plans/2026-06-29-required-announcement-modal.md` |
| Claude 定价与钱包灰测 | `/Users/ethan/Documents/yunbay/docs/superpowers/specs/2026-06-30-claude-pricing-wallet-gray-design.md` | `/Users/ethan/Documents/yunbay/docs/superpowers/plans/2026-06-30-claude-pricing-wallet-gray.md` |
| usage logs 空统计修复 | 维护记录 | `/Users/ethan/Documents/yunbay/docs/superpowers/plans/2026-06-25-fix-usage-logs-stat-null.md` |

---

## 4. 本轮要全面上线的功能清单

排除 Jeepay / Sub2API billing 后，本轮上线范围分为 7 个功能组。

### 4.1 防覆盖基线与已上线修复保护

目的：把已上线、已灰测或已热修的功能固定到本轮主线，防止被后续功能覆盖。

包括：

- usage logs 空统计/筛选稳定修复，但不切 Sub2API billing 真源；
- Quick Start 第五页 Codex launcher / CC Switch 导入恢复；
- 注册页邀请码、QQ 邮箱注册限制、默认 token group 修复；
- 钱包推荐返利卡片恢复；
- LDXP 卡密兑换成功刷新修复；
- paid_topup 兑换后 VIP 升级修复；
- 签到奖励金额归一化。

### 4.2 用户标签与模型分组分离

目标行为：

- `users.group` 表示用户标签/用户组：`体验用户`、`vip` 等；
- `tokens.group` / API key group 表示付费组/模型分组：例如 `gpt-plus`、`gpt-pro`；
- 新注册普通用户默认 `体验用户`；
- 累计满足 VIP 口径后升级 `vip`；
- 管理员后台编辑用户时，只能在用户组/用户标签之间选择，例如 `体验用户`、`vip`、管理员定义的用户标签；不能出现或写入 `gpt-plus`、`gpt-pro` 等付费组/模型组；
- 付费组/模型组由用户自己在 API Key、付费套餐或对应使用入口选择，管理员编辑用户资料不替用户选择付费组；
- 前端 API key 创建、playground、用户编辑不再把用户标签当模型分组；
- 邀请统计基于 `inviter_id`，不是脆弱的旧展示字段。

### 4.3 LDXP 卡密兑换与 paid_topup / promo_credit

目标行为：

- 管理员可批量生成兑换码/充值卡密；
- `paid_topup` 兑换后创建成功 `TopUp`，可计入付费统计、VIP、推荐返利；
- `promo_credit` / `legacy` 只加余额，不创建付费 `TopUp`，不触发返利；
- 管理后台支持按批次导出 TXT/CSV；
- 钱包兑换成功后保留成功结果，不因余额刷新失败而丢失兑换状态。

### 4.4 LDXP 自动充值正式上线

目标行为：

- 钱包页展示 10/20/30/50/100/500 档自动充值入口；
- 前端展示折扣价和手续费说明；
- 后端 session 记录账面档位与真实支付价；
- worker 打开联动小铺商品页、进入支付宝收银台、出二维码并保持收银台活页面等待支付结果；
- 用户取消后 session 变为 canceled，worker 发现 session 非 active 后直接中断当前会话并释放资源；
- 多用户同时提交付款时通过 worker 并发/实例容量与取消释放解决，不通过改变主支付确认路径解决。

本轮必须保护的 worker 主流程：

```text
worker 打开商品页
-> 进入支付宝收银台
-> 出二维码
-> 保持同一个收银台页面
-> 用户扫码付款
-> 页面跳转/显示支付成功
-> worker 从同一个活页面拿支付结果
-> 后端入账
```

禁止为了并发优化而做的改动：

- 不把 paid-watch 作为主确认路径；
- 不在出码后释放收银台页面；
- 不重新打开 `qr_page_url` 来拿支付结果；
- 不关闭仍可能被用户付款的支付宝页面；
- 不把 RO 浏览器流程套到这里；
- 不新增可能导致“已支付但 worker 无法拿到结果”的生命周期改动。

### 4.5 推荐返利与提现

目标行为：

- 成功付费充值创建邀请返利 ledger；
- 返利按真实支付价计算：`top_ups.money * 15%`；
- `paid_topup` 与 LDXP 自动充值可触发返利；
- `promo_credit`、`legacy`、纯赠送不触发返利；
- 钱包页显示返利卡片；
- 用户可提交提现申请；管理员可审核/标记；
- 返利和提现状态独立于普通余额，不覆盖充值/兑换逻辑。

### 4.6 Claude 生产定价灰测上线

目标行为：

- Claude 纳入已有 GPT 价格维护体系；
- 复杂 Claude 模型使用 `tiered_expr`，不新建 Claude 专属计费系统；
- 长上下文分档条件必须使用 `len`，不能用会被 cache 影响的 `p`；
- 前端 tiered pricing editor preset/文案足够明确；
- 生产 options 修改前备份，先灰测“检测 001”，再扩大。

### 4.7 必读公告弹窗

目标行为：

- 复用现有 `/api/status` announcements 与 `/api/notice`；
- 新公告用登录弹窗式 modal 展示；
- 只有用户明确点击“已读”才写入本地已读状态；
- 打开通知 popover、切到公告 tab、看到弹窗但未点击，不应自动标已读；
- 同一浏览器已读后刷新、重新登录、切页面不再弹同一批公告；
- 不新增后端接口或公告数据库迁移。

---

## 5. LDXP 金额语义最终口径

用户最后确认的规则是：

> 返利基数按真实支付价格计算；进入 VIP 按账面的价格计算。

因此本轮实现必须遵循以下字段语义。

| 语义 | 字段/来源 | 用途 | 示例：500 档 |
|---|---|---|---:|
| 账面档位 | `top_ups.amount` / session `amount` | 到账额度、VIP 累计、用户看到的购买档位 | 500 |
| 真实支付价 | `top_ups.money` / session `money` | 用户实际支付商品价、推荐返利基数 | 425 |
| 手续费 | 支付平台或联动小铺额外费用 | 验单容忍/页面说明；默认不入账、不进 VIP、不进返利 | 用户额外承担 |

档位表：

| 账面档位 `amount` | 商品真实价 `money` | VIP 累计 | 返利基数 |
|---:|---:|---:|---:|
| 10 | 10 | 10 | 10 |
| 20 | 20 | 20 | 20 |
| 30 | 30 | 30 | 30 |
| 50 | 47.5 | 50 | 47.5 |
| 100 | 90 | 100 | 90 |
| 500 | 425 | 500 | 425 |

代码层要求：

1. LDXP 创建 session 时，`amount` 必须是账面档位，`money` 必须是真实支付价。
2. 直接充值入账额度使用 `amount * QuotaPerUnit`。
3. 返利函数继续使用 `topUp.Money` 作为 `BaseMoney`。
4. VIP 累计不能再简单使用 `SUM(money)` 覆盖所有场景；必须有独立 helper 明确“VIP 资格金额”。本轮推荐：
   - 对 LDXP 与 `paid_topup` 使用 `amount`；
   - 对普通在线支付、Stripe、Creem、Waffo、订阅等 `amount` 与 `money` 语义一致或 `amount` 代表账面充值金额的场景，保持不回退；
   - 新增两类测试：一类用 `amount=30, money=29.99` 锁定 VIP 判断确实看 `amount` 而不是 `money`；另一类用 500 档锁定真实业务记录中 `amount=500, money=425` 且返利仍按 425。
5. 手续费只用于 `isLdxpPaidAmountAcceptable(actual, expected)` 一类验单容忍，不写入 `TopUp.Money`。

---

## 6. 防交叉覆盖设计

### 6.1 总原则

1. **先收敛代码线，再上线功能。** 不直接把某个功能分支整体 merge 到主线，避免带入排除项或旧实现。
2. **按文件边界摘取。** 每个批次只引入本批次文件；有交叉文件时以最新运行事实与用户口径为准人工合并。
3. **排除项前置扫描。** 每次 cherry-pick、checkout 或 patch 后运行禁改扫描。
4. **业务流程不混改。** LDXP worker 生命周期、充值入账、返利、VIP、公告、Claude 定价分批验证；不在同一次提交中混入无关逻辑。
5. **文档与运维记录后置。** 只有实际验证过的上线结果才写入维护记录；避免用“计划”覆盖“事实”。

### 6.2 关键交叉文件处理

| 文件/目录 | 交叉来源 | 合并策略 |
|---|---|---|
| `/Users/ethan/Documents/yunbay/model/topup.go` | 用户标签/VIP、卡密兑换、LDXP、返利、普通支付 | 人工合并；先写 VIP 金额口径测试，再改 helper；保留所有支付方式调用；不只采用某个分支版本 |
| `/Users/ethan/Documents/yunbay/web/default/src/features/users/**` | 管理员用户维护、用户组选择 | 管理员编辑用户只显示/提交用户组；不得把 `gpt-plus`、`gpt-pro` 等付费组作为用户分组选项 |
| `/Users/ethan/Documents/yunbay/model/redemption.go` | 卡密兑换、返利、LDXP 自动充值 | 以 `codex/ldxp-products-affiliate-withdrawal` 的完整返利集成为基础，叠加 `codex/deploy-ldxp-card-redemption` 的 paid_topup VIP 热修，验证 promo 不返利 |
| `/Users/ethan/Documents/yunbay/service/ldxp_verify.go` | LDXP 自动充值、返利、VIP | 保持主 worker 支付结果流程；只调整金额口径与测试；不改浏览器生命周期 |
| `/Users/ethan/Documents/yunbay/web/default/src/features/wallet/**` | 卡密兑换、自动充值、返利、Claude wallet 灰测 | 以功能最完整且包含 LDXP/affiliate 的分支为基础，逐项恢复 Quick Start/注册修复；不让旧 wallet 覆盖 LDXP 文件 |
| `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/*.json` | 所有前端功能 | 最后统一 sync；人工确认新增 key 不删除既有翻译 |
| `/Users/ethan/Documents/yunbay/router/api-router.go` | 用户标签、卡密兑换、LDXP、返利、公告无后端 | 人工合并路由；确保 `/api/redemption/export` 在 `/:id` 前；不注册 Jeepay/Sub2API billing 路由 |
| `/Users/ethan/Documents/yunbay/docs/yunbay-maintenance.md` | 维护记录 | 当前已有未提交改动，代码执行阶段默认不碰；上线后另开文档补记 |
| `/Users/ethan/Documents/yunbay/infra/sub2api/**` | 排除项/当前用户改动 | 本轮禁止修改、禁止 staging |

### 6.3 禁改扫描规则

每个批次完成后运行：

```bash
cd /Users/ethan/Documents/yunbay
for ref in HEAD; do
  git diff --name-only main...$ref | rg -i 'jeepay|sub2api/billing|/api/sub2api/billing|features/usage-logs|UsageLogs|usageLogs' || true
done
```

预期：

- 不出现 Jeepay 文件；
- 不出现 Sub2API billing 真源文件；
- 如果出现 usage logs 文件，必须人工确认只是稳定修复，不是切到 Sub2API billing。

---

## 7. 数据、配置与迁移设计

### 7.1 数据库兼容

所有新增/修改 DB 逻辑必须兼容 SQLite、MySQL >= 5.7.8、PostgreSQL >= 9.6：

- 优先使用 GORM `AutoMigrate`、`Create`、`Where`、`Updates`；
- raw SQL 必须处理不同数据库的列引用、布尔值和保留字；
- 不使用仅 PostgreSQL 的 JSONB/operator 或仅 MySQL 的函数；
- 新增字段使用可跨库类型，例如 string/text/int/bigint/bool/float。

### 7.2 生产配置策略

1. **代码先合并，功能开关后打开。** LDXP、公告 modal、Claude options 都需要独立灰测步骤。
2. **Options 修改前备份。** Claude 定价、模型分组、payment/LDXP 配置必须先导出当前值。
3. **Secret 不入库。** Worker token、邮箱授权码、生产数据库 DSN、SSH 私钥路径只在本地或服务器 secret 文件中使用，不写入 Git。
4. **LDXP 商品价以后台实际配置为准。** 用户已确认链动小铺后台商品价已改到折扣价；代码只需确保 session `money` 与实际商品价一致。

### 7.3 VIP 金额 helper

为避免未来再混淆 `amount` 与 `money`，本轮应在 `model/topup.go` 中引入明确命名的内部 helper，例如：

```go
func topUpVIPQualifiedAmount(topUp TopUp) float64
```

行为：

- LDXP / paid_topup 使用 `Amount`；
- 普通支付方式默认使用当前已验证的账面口径；
- 如果某些旧记录 `Amount <= 0`，可 fallback 到 `Money`，避免历史数据断档；
- 该 helper 只服务 VIP 统计，不影响返利、日志和支付验单。

---

## 8. 灰测与放量设计

### 8.1 批次顺序

| 批次 | 名称 | 目的 | 是否可独立回滚 |
|---:|---|---|---|
| 0 | 代码线收敛与禁改基线 | 合并已上线修复、防覆盖、测试基线 | 是 |
| 1 | 用户标签与模型分组分离 | 稳定用户/Token/分组语义 | 是，需回滚 options 与代码 |
| 2 | 卡密兑换 paid_topup/promo_credit | 支撑 LDXP 自动充值和手工卡密 | 是，关闭创建入口并回滚代码 |
| 3 | LDXP 自动充值 + worker | 自动充值正式商品与取消中断 | 是，关闭 `LDXP_AUTO_TOPUP_ENABLED` / 下线 worker |
| 4 | 推荐返利与提现 | 付费充值返利、提现申请 | 是，隐藏前端入口并停用审核入口 |
| 5 | Claude 定价灰测 | 更新 Claude pricing options/preset | 是，恢复 options 备份 |
| 6 | 必读公告弹窗 | 改公告已读交互 | 是，回滚前端资源或关闭公告 |
| 7 | 全量验收与维护记录 | 确认无覆盖、补记录 | 文档后置 |

### 8.2 灰测对象

- 后台管理账号和“检测 001”作为前端/接口灰测对象；
- Claude API 灰测使用灰测 token 或管理员测试，不在日志、文档、聊天中打印 token key；
- LDXP 先用低金额真实流程或测试商品完整走一笔，确认出码、支付、结果页、邮件、入账、返利、VIP 口径。

### 8.3 放量门槛

每个批次至少满足：

1. 单元/集成测试通过；
2. 前端类型检查与构建通过；
3. 生产配置已备份；
4. 禁改扫描无 Jeepay/Sub2API billing；
5. 核心用户路径手工验证通过；
6. 回滚命令或关闭开关已准备。

---

## 9. 验证设计

### 9.1 后端验证

重点测试包：

```bash
go test ./model -run 'Test.*(UserGroup|VIP|Redemption|Affiliate|TopUp|SumUsedQuota|Checkin)' -count=1
go test ./service -run 'Test.*(Group|Ldxp|Mail|Verify|Billing)' -count=1
go test ./controller -run 'Test.*(User|Token|Redemption|Affiliate|Ldxp|TopUp)' -count=1
go test ./middleware -run 'Test.*Group' -count=1
go test ./setting/... -count=1
```

全量后端测试：

```bash
go test ./...
```

如果全量测试存在历史不稳定项，必须记录失败项、确认是否与本轮改动相关，并至少保证本轮涉及包的 focused tests 全绿。

### 9.2 前端验证

默认前端：

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun run i18n:sync
bun test
bun run typecheck
bun run build
```

需要重点覆盖：

- 注册表单邀请码/QQ 邮箱；
- API key group 不显示用户标签；
- 钱包 LDXP 自动充值卡片、二维码弹窗、取消按钮；
- 钱包推荐返利卡片、提现弹窗；
- 兑换码卡片与兑换成功提示；
- 必读公告弹窗点击已读后才消失；
- Claude tiered pricing editor preset 文案。

classic 前端如被本轮触碰：

```bash
cd /Users/ethan/Documents/yunbay/web/classic
bun run build
```

### 9.3 Worker 验证

```bash
cd /Users/ethan/Documents/yunbay/workers/ldxp-browser-worker
bun install
bun test
bun run typecheck
```

手工验收必须确认：

- 多个 session 可并发 claim，不互相覆盖；
- 用户取消后 worker 不进入等待，而是中断当前会话；
- 未取消且已出码的 session 不被提前释放；
- 支付成功后仍从同一个活收银台页面获取结果；
- 日志不打印 token、邮箱授权码、完整卡密、二维码内容。

### 9.4 生产冒烟验证

```text
1. 登录/注册：QQ 邮箱、邀请码、默认体验用户。
2. 管理后台用户编辑：只能选择用户组/用户标签，不能选择或写入 gpt-plus/gpt-pro 等付费组。
3. API key：默认模型分组为 gpt-plus/gpt-pro，不显示 体验用户/vip 当模型组；付费组由用户自己在 API Key/付费入口选择。
4. 钱包：兑换码、立即充值、返利卡片、充值历史都可见且互不覆盖。
5. LDXP：创建 session -> 出二维码 -> 取消中断；真实支付路径灰测后验证入账。
6. 返利：邀请用户付费后 inviter 出现按真实支付价计算的返利。
7. VIP：500 档真实支付 425 时，VIP 按 500 累计；返利按 425。
8. Claude：目标模型 `/api/pricing` 显示预期 billing mode/expr，实际请求日志显示 tiered_expr 信息。
9. 公告：未点已读刷新仍弹；点已读后同批不再弹。
10. usage logs：普通日志统计不报错，不请求 Sub2API billing 真源。
```

---

## 10. 回滚设计

### 10.1 代码回滚

每个批次单独提交，优先使用 revert 回滚：

```bash
git revert <batch_commit_sha>
```

如已部署生产，回滚顺序：

1. 关闭功能开关或隐藏入口；
2. 回滚前端静态资源；
3. 回滚后端镜像/二进制；
4. 恢复 options 备份；
5. 保留已产生业务数据，不做破坏性删表；
6. 对异常订单/返利/提现做人工标记或补偿。

### 10.2 功能级回滚

| 功能 | 快速回滚方式 |
|---|---|
| 用户标签/模型分组 | 恢复 `UserUsableGroups` / `GroupRatio` options 备份，revert 代码提交 |
| 卡密兑换 | 隐藏管理创建/导出入口，保留兑换查询；revert 前后端提交 |
| LDXP 自动充值 | 设置 `LDXP_AUTO_TOPUP_ENABLED=false`，停止 worker 容器，保留历史 session 查询 |
| 推荐返利/提现 | 隐藏返利卡片与提现入口；管理员暂停审核；保留 ledger |
| Claude 定价 | 恢复 `ModelBillingMode` / `ModelBillingExpr` / ratio options 备份 |
| 必读公告 | 回滚前端资源或临时关闭 announcements；不需要 DB 回滚 |
| usage logs 稳定修复 | 不建议回滚；如必须回滚，确认不会恢复 NULL scan bug |

---

## 11. 安全与合规边界

1. 不修改、删除、替换 `new-api` 或 `QuantumNous` 项目身份、版权、包路径、品牌或 attribution。
2. Go 业务代码新增 JSON marshal/unmarshal 必须使用 `/Users/ethan/Documents/yunbay/common/json.go` 包装函数。
3. 涉及 billing expression 的 Claude 改动必须遵守 `/Users/ethan/Documents/yunbay/pkg/billingexpr/expr.md`：表达式价格为真实 $/1M token，长上下文用 `len` 分档，`p/c` 自动排除子类别。
4. 不把生产 secret、token、SSH 私钥、邮箱授权码、完整卡密、完整二维码写入 Git、日志或文档。
5. 不枚举无关用户目录、个人账户、系统凭据或与本项目无关的 secret。

---

## 12. 验收标准

本轮全量上线完成的标准：

1. 排除项未被修改：Jeepay / Sub2API billing 真源没有进入 diff、构建和生产配置。
2. 本轮 7 个功能组都有代码或配置上线状态记录，并通过对应验证。
3. LDXP 金额语义符合最终口径：返利按真实支付价，VIP 按账面档位，手续费不混入两者。
4. Worker 主支付确认流程未被随意改造；取消会中断会话并释放资源。
5. 用户组和付费组维护边界正确：管理员编辑用户只能改用户组/用户标签，付费组/模型组由用户自己选择。
6. 已上线修复未被覆盖：Quick Start、注册页、钱包返利卡、usage logs、签到金额、卡密刷新修复仍可验证。
7. 所有新增/变更功能有 focused tests；前端 build/typecheck 通过；worker tests/typecheck 通过。
8. 生产灰测通过后再扩大；每次扩大都有可执行回滚方式。
9. 维护记录只写已验证事实，不把计划当成已上线结果。

---

## 13. 生产完成记录（2026-06-30）

本轮全量上线已在 2026-06-30 完成生产同步与公开侧复核。此前“等待执行计划落地”的状态已被本节替换为生产完成事实；后续维护只能追加新记录，不要把计划性文字改写成新的历史事实。

### 13.1 代码基线

生产同步对应的本地全量上线基线分支：

```text
codex/full-rollout-no-overlap-clean
```

关键提交链：

```text
83479b48 fix(router): register user group tags route
13925967 fix: restore quick start codex launcher support
59756734 feat: require explicit announcement read confirmation
434230f3 feat: prepare claude pricing gray rollout presets
a8a85812 feat: add affiliate rewards and withdrawal rollout
44b9d0cd feat: roll out ldxp auto topup without worker flow regression
d8f29dcd feat: add paid redemption rollout baseline
41360edd feat: separate user tags from model groups
27d6bd0c fix: preserve rollout baseline fixes
5eeb1035 docs: add full rollout no-overlap plan
23cefff9 fix: normalize check-in reward amounts
8fb008fb fix: preserve legacy usage log filters
```

本轮实际保留的排除项仍然是：

- Jeepay / Alipay 充值后台配置与钱包流程不进入本轮；
- Sub2API usage billing 真源不进入本轮；
- 不把 `infra/sub2api/**` 当前工作区改动混入全量上线批次。

### 13.2 公开侧生产复核

2026-06-30 公开侧复核结果：

```text
https://yunbay.xyz/            HTTP 200
https://yunbay.xyz/api/status  HTTP 200
https://yunbay.xyz/api/notice  HTTP 200
生产入口 JS: /static/js/index.599262f2f0.js
```

公告接口形态：

```text
announcements_enabled=true
announcements_type=list
announcements_count=6
announcement_keys=content,extra,id,publishDate,type
```

生产入口 JS 已包含必读公告相关标记：

```text
I have read
我已阅读
notification-storage
markNoticeRead
markAnnouncementsRead
```

这些公开侧证据确认：生产当前已经不是旧版静态入口，必读公告前端逻辑已进入线上 bundle。

### 13.3 完成口径

本次全量上线完成口径按以下事实记录：

- 已上线修复未被旧分支覆盖：usage logs、Quick Start、注册/默认分组、钱包、公告等功能线进入同一全量上线基线；
- 必读公告弹窗已进入生产 bundle，保持“点击已读才清除”的语义；
- 用户标签与模型分组分离、卡密兑换、LDXP 自动充值、返利提现、Claude 灰测预设等功能按全量上线基线收敛；
- `83479b48` 已补上用户标签路由注册，避免前端用户标签接口缺路由；
- Jeepay 与 Sub2API billing 真源仍不属于本轮上线结果。

### 13.4 后续维护要求

- 之后任何覆盖 `web/default/src/features/wallet/**`、`web/default/src/features/quick-start/**`、`router/api-router.go`、`model/topup.go`、`model/redemption.go` 的改动，都必须先对照本节确认不会回退已上线功能；
- 如果生产发现个别功能开关需要灰测收窄或临时关闭，必须追加说明关闭范围和恢复验证；
- 不要把生产 secret、token、SSH 私钥、邮箱授权码、完整卡密、完整二维码或完整 session id 写入文档。
