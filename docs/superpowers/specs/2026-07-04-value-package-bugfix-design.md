# 超值套餐 Bug 修复设计规格

> 基线：本规格基于 `origin/main` 当前提交 `d7123a73345f3d9bbe0aeb5326e4bcd7d22f53ce` 的实际代码结构编写。本文只定义本轮 Bug 修复范围、数据流、落点与验收标准，不包含实现代码。

## 1. 修复目标

本轮修复围绕已经上线的“超值套餐”能力，解决四类用户可见问题：

1. **发光条冲突**：日卡、周卡、月卡启用后的页面边框呼吸光应改为淡蓝色，和 VIP 的金色/黄色边框光区分。
2. **套餐启用后的有效分组与倍率**：当用户真实身份是 VIP 时，启用日卡/周卡/月卡后，API 请求应按套餐分组和普通用户倍率消耗套餐额度，而不是继续套用 VIP 的 `0.3` 特殊倍率。
3. **套餐限额进度可视化**：套餐卡片需要展示总额度、5 小时滚动限额、7 天滚动限额的用量进度，有限额才显示对应进度。
4. **侧边栏入口吸引注意**：所有用户侧边栏里的“超值套餐”入口需要有轻量、显眼但不扰民的呼吸动画。

## 2. 当前实现证据

### 2.1 后端套餐模型与状态

实际文件：`model/subscription.go`

当前已经存在：

- `SubscriptionPlan.PlanKind = value_package`
- `PackageType`：`day` / `week` / `month`
- `PackageLevel`：`1` / `2` / `3`
- `ModelGroup`
- `ConcurrencyLimit`
- `Limit5hAmount`
- `Limit7dAmount`
- `LdxpProductUrl` / `LdxpProductName` / `LdxpProductAmount` / `LdxpProductRef` / `LdxpSessionTTLSeconds`
- `UserValuePackagePreference.Enabled`
- `UserValuePackagePreference.ActiveUserSubscriptionId`
- `ValuePackageUsageRecord`，按 `user_subscription_id + request_id` 去重记录套餐消耗

当前 `ValuePackageState` 只返回：

- `preference`
- `subscription`
- `plan`

它没有返回前端展示进度所需的 usage summary，因此前端目前只能展示“限额配置值”，不能展示“已用多少”。

### 2.2 后端套餐 relay 逻辑

实际文件：`middleware/value_package.go`

当前 `ValuePackageEntitlement()` 在用户套餐启用时会：

- 查询 `model.GetActiveValuePackageForRelay(userId)`；
- 通过 `applyValuePackageGroupScope()` 强制设置：
  - `ContextKeyUsingGroup = state.Plan.ModelGroup`
  - `ContextKeyTokenGroup = state.Plan.ModelGroup`
  - `ContextKeyValuePackageSubscriptionId`
  - `ContextKeyValuePackagePlanId`
  - `ContextKeyValuePackageModelGroup`
  - `ContextKeyValuePackagePackageType`
- 检查 5 小时 / 7 天 rolling window；
- 申请 1~2 并发 slot。

当前没有覆盖 `ContextKeyUserGroup`。而实际倍率计算在：

- `relay/helper/price.go` 的 `HandleGroupRatio()`；
- `service/quota.go` 的 audio quota 路径。

这两处都会先用 `relayInfo.UserGroup + relayInfo.UsingGroup` 查 `ratio_setting.GetGroupGroupRatio()`。因此 VIP 用户启用套餐后，虽然 `UsingGroup` 已变成套餐模型组，但 `UserGroup` 仍可能是 `vip`，会继续影响倍率，无法稳定达到“套餐按普通用户倍率”的目标。

### 2.3 后端套餐额度消耗

实际文件：`service/billing_session.go`、`model/subscription.go`

当前 `NewBillingSession()` 在 `relayInfo.ValuePackageSubscriptionId > 0` 时，会强制创建 `SubscriptionFunding`，并携带 `valuePackageSubscriptionId`。这意味着：

- 即使用户设置是 `wallet_only`，启用套餐时也会走套餐订阅 funding；
- 用户钱包余额不会作为主要资金来源下降；
- `UserSubscription.AmountUsed` 会增加；
- `ValuePackageUsageRecord` 会写入/更新。

已有测试 `TestValuePackageBillingIgnoresWalletOnlyPreference` 覆盖了“启用套餐时不消耗钱包余额，消耗套餐额度”。本轮应保留这条语义，并补充 VIP 场景和额度耗尽提示。

### 2.4 前端套餐 UI 与光效

实际文件：

- `web/default/src/features/value-packages/components/value-package-card.tsx`
- `web/default/src/features/value-packages/components/authenticated-benefit-effects.tsx`
- `web/default/src/features/value-packages/lib/benefit-effects.ts`
- `web/default/src/styles/index.css`

当前：

- `getBenefitGlowMode()` 已规定 package glow 优先于 vip glow；
- `AuthenticatedBenefitEffects` 根据 `shouldShowPackageGlow()` 和 `isVipUserGroup()` 渲染 `ViewportBenefitGlow`；
- CSS 中 `.yunbay-viewport-benefit-glow--package` 仍使用接近黄色/金色的 `oklch(... 82 ...)` 色相；
- `.yunbay-viewport-benefit-glow--vip` 也是金黄风格，所以两者视觉上冲突。

### 2.5 侧边栏入口

实际文件：

- `web/default/src/hooks/sidebar-data-model.ts`
- `web/default/src/components/layout/components/nav-group.tsx`
- `web/default/src/components/ui/sidebar.tsx`

当前普通用户和管理员侧边栏都已经有 `/value-packages` 入口。`NavLink`/`NavGroup` 尚没有用于“重点入口”的标记或样式。

## 3. 设计原则

### 3.1 不持久破坏用户真实身份分组

用户希望“启用套餐时切到日卡/周卡/月卡分组，暂停时切回原分组”。在当前架构下，`users.group` 同时承载：

- 用户真实身份：普通 / 体验 / VIP；
- VIP 仪式感弹窗和边框判断；
- 充值、邀请、后台用户管理中的身份展示；
- 部分旧订阅升级/降级逻辑。

因此本轮不应把 `users.group` 持久改成 `day-card`、`week-card` 或 `month-card`。正确做法是：

- **数据库里的 `users.group` 保持不变**，VIP 仍是 VIP，普通用户仍是普通用户；
- 套餐启用时，仅对当前 relay 请求上下文设置“有效计费用户分组”和“有效模型分组”；
- 套餐暂停后，因为 preference disabled，后续请求不再进入套餐 override，自然回落到 TokenAuth/UserAuth 原本写入的 `ContextKeyUserGroup`、`ContextKeyUsingGroup` 和 `ContextKeyTokenGroup`。

这样对用户体验等价于“启用切到套餐、暂停切回原分组”，同时不会破坏 VIP 身份和后台数据。

### 3.2 套餐有效倍率应可配置但默认不走 VIP 特殊倍率

当前普通用户默认是 `体验用户 + gpt-plus = 0.99`，VIP 常规 Plus 是 `gpt-plus = 0.3`。套餐分组一般是管理员配置的独立模型组，例如 `day-card`、`week-card`、`month-card`。

本轮应在套餐启用时覆盖 relay 上下文：

- `ContextKeyUsingGroup = plan.ModelGroup`
- `ContextKeyTokenGroup = plan.ModelGroup`
- `ContextKeyUserGroup = plan.ModelGroup`

这样价格计算会先尝试 `GroupGroupRatio[plan.ModelGroup][plan.ModelGroup]`，没有配置时退回 `GroupRatio[plan.ModelGroup]`，不再使用真实 VIP 用户组的特殊倍率。管理员可以通过套餐模型分组自身的 group ratio，或者后续补充的 group-group ratio，把日卡/周卡/月卡设成和普通用户一致的倍率。

如果后续希望更明确地区分“套餐模型分组”和“套餐计费用户分组”，可以新增 `SubscriptionPlan.BillingUserGroup` 字段；本轮为最小安全修复，优先使用现有 `ModelGroup` 作为有效计费用户组，不新增数据库字段。

### 3.3 套餐额度优先级

启用套餐后，API 请求资金来源应保持当前设计：

1. 套餐开启且有效：走 `PreConsumeValuePackageSubscription()`；
2. 套餐额度不足、总额度用完、5 小时限额用完、7 天限额用完：请求返回明确错误，不自动偷偷改扣钱包；
3. 用户点击“暂停/关闭套餐使用”后：`UserValuePackagePreference.Enabled = false`，后续请求走原来的钱包/API 余额或普通订阅偏好。

这样用户能主动决定何时回到 API 余额，避免套餐用完后静默消耗钱包。

## 4. 后端修复设计

### 4.1 套餐上下文覆盖

落点：`middleware/value_package.go`

修改 `applyValuePackageGroupScope()`：

- 保留当前写入：
  - `ContextKeyUsingGroup`
  - `ContextKeyTokenGroup`
  - `ContextKeyValuePackageSubscriptionId`
  - `ContextKeyValuePackagePlanId`
  - `ContextKeyValuePackageModelGroup`
  - `ContextKeyValuePackagePackageType`
- 新增写入：
  - `ContextKeyUserGroup = state.Plan.ModelGroup`

建议同时在上下文记录原始用户分组，便于日志和后续调试：

- 如果当前已有 `ContextKeyUserGroup`，可在普通 string key 中保存 `value_package_original_user_group`；
- 不影响现有 `constant.ContextKey`，避免新增广泛依赖。

验收：

- VIP 用户启用月卡后，relayInfo 中 `UserGroup` 与 `UsingGroup` 都应为月卡分组；
- 普通用户启用日卡后同样生效；
- 套餐关闭后 middleware 不做 override，原始 `UserGroup` 保持 TokenAuth/UserAuth 写入的 VIP 或普通分组。

### 4.2 套餐用量 summary

落点：`model/subscription.go`、`controller/value_package.go`

新增只读 DTO/结构，不新增表：

```go
type ValuePackageUsageSummary struct {
    TotalUsed       int64   `json:"total_used"`
    TotalLimit      int64   `json:"total_limit"`
    TotalRemaining  int64   `json:"total_remaining"`
    TotalPercent    float64 `json:"total_percent"`

    Used5h          int64   `json:"used_5h"`
    Limit5h         int64   `json:"limit_5h"`
    Percent5h       float64 `json:"percent_5h"`

    Used7d          int64   `json:"used_7d"`
    Limit7d         int64   `json:"limit_7d"`
    Percent7d       float64 `json:"percent_7d"`

    Exhausted       bool    `json:"exhausted"`
    ExhaustedReason string  `json:"exhausted_reason"`
}
```

`ValuePackageState` 增加：

```go
Usage *ValuePackageUsageSummary `json:"usage,omitempty"`
```

计算方式：

- 当 `state.Subscription` 和 `state.Plan` 存在时才计算；
- `TotalUsed = subscription.AmountUsed`；
- `TotalLimit = subscription.AmountTotal`，如果为 `0` 表示不限总额；
- `TotalRemaining = max(TotalLimit - TotalUsed, 0)`，不限总额时可为 `0` 或不显示；
- `Used5h`、`Used7d` 复用现有 `getValuePackageWindowUsageTx()`；
- 百分比 clamp 到 `0~100`；
- exhausted 判断优先级：
  1. `total_quota_exhausted`：总额度有限且已用完；
  2. `limit_5h_exhausted`：5 小时限额有限且已用完；
  3. `limit_7d_exhausted`：7 天限额有限且已用完。

数据库兼容：

- 全部用 GORM 查询或现有 `COALESCE(SUM(quota), 0)` 方式；
- 不新增列，不涉及跨数据库迁移风险；
- 保持 SQLite/MySQL/PostgreSQL 同时兼容。

### 4.3 额度用完错误文案

落点：`middleware/value_package.go`、`service/billing_session.go` 或 `model/subscription.go`

保持 API 语义：套餐启用但额度不足时返回 `403`，不自动回退钱包。

用户可见文案统一为：

```text
当前余额已用完，建议暂停使用，使用 API 或等时间跑完再使用
```

其中“余额”按用户话术理解为当前套餐额度。实现上可使用这句中文作为 relay 错误 message，前端卡片也展示同一句。

需要覆盖三类耗尽场景：

- 总额度 `AmountUsed >= AmountTotal`；
- 5 小时滚动限额 `Used5h >= Limit5h`；
- 7 天滚动限额 `Used7d >= Limit7d`。

对于 rolling window 场景，后端可在日志中保留具体 `已用 / 限额`，但面向用户的主要提示使用统一产品文案。

## 5. 前端修复设计

### 5.1 套餐发光改淡蓝色

落点：`web/default/src/styles/index.css`

调整 `.yunbay-viewport-benefit-glow--package`：

- 主色改为淡蓝 / 天蓝，例如 `oklch(0.78 0.13 235 / 0.62)`；
- 柔光改为更低透明度的蓝色，例如 `oklch(0.82 0.10 235 / 0.20)`；
- VIP 继续保留金色；
- `getBenefitGlowMode()` 保持 package 优先于 vip。

验收：

- 同时 VIP + 套餐启用时页面四周显示蓝色套餐光；
- 只有 VIP 无套餐时显示金色 VIP 光；
- `prefers-reduced-motion: reduce` 下动画停止但静态光效保留或弱化。

### 5.2 套餐卡片限额进度条

落点：

- `web/default/src/features/value-packages/types.ts`
- `web/default/src/features/value-packages/components/value-package-card.tsx`
- 必要时新增 `web/default/src/features/value-packages/components/value-package-limit-progress.tsx`

前端类型 `ValuePackageState` 增加 `usage?: ValuePackageUsageSummary | null`。

卡片展示规则：

- 总额度有限：展示“套餐总额度”进度条；
- `plan.limit_5h_amount > 0`：展示“5 小时限额”进度条；
- `plan.limit_7d_amount > 0`：展示“7 天限额”进度条；
- 无对应限额则不显示进度条，只保留当前“Unlimited/不限额”文本；
- 进度条文案显示 `已用 / 限额` 和百分比；
- 超过 80% 时可用 warning 色，100% 时用 destructive 色；
- 如果 `usage.exhausted === true` 且当前套餐 `running`，卡片内显示醒目的 Alert：

```text
当前余额已用完，建议暂停使用，使用 API 或等时间跑完再使用
```

暂停按钮不自动触发，用户自己决定是否点击“关闭套餐使用”。

### 5.3 侧边栏“超值套餐”呼吸动画

落点：

- `web/default/src/components/layout/types.ts`
- `web/default/src/hooks/sidebar-data-model.ts`
- `web/default/src/components/layout/components/nav-group.tsx`
- `web/default/src/styles/index.css`

建议给 `NavLink` 增加可选字段：

```ts
attention?: 'value-packages'
```

`buildSidebarData()` 中普通用户和管理员的 `/value-packages` item 都设置：

```ts
attention: 'value-packages'
```

`SidebarMenuLink` 渲染时：

- 对 `item.attention === 'value-packages'` 增加 class，例如 `yunbay-sidebar-value-package-pulse`；
- active 状态下仍保留动画，但降低强度，避免和 active 背景冲突；
- 动画包括：
  - 图标轻微蓝紫外发光；
  - 菜单项背景淡蓝呼吸；
  - 可选小光点，不新增文案。

无障碍与性能：

- 遵守 `prefers-reduced-motion: reduce`，关闭循环动画；
- 动画只使用 opacity / box-shadow / transform，避免布局抖动；
- 不影响折叠侧边栏 tooltip 和移动端点击。

### 5.4 i18n

本轮如果只复用已有 key，可不新增翻译。

如果新增以下英文 key，需要同步所有语言：

- `Package quota used up hint`
- `Package total limit`
- `Used {{used}} / {{limit}}`
- `{{percent}}% used`

其中中文必须包含用户指定文案：

```text
当前余额已用完，建议暂停使用，使用 API 或等时间跑完再使用
```

最终需运行 `bun run i18n:sync`，确保 `en/zh/fr/ja/ru/vi` 无 missing/untranslated 报告。

## 6. 测试计划

### 6.1 Go 后端测试

新增或扩展：

1. `middleware/value_package_test.go`
   - VIP 用户启用套餐时：
     - `using_group = plan.ModelGroup`
     - `token_group = plan.ModelGroup`
     - `user_group = plan.ModelGroup`
   - preference disabled 时：
     - 不覆盖 `user_group`
     - 不覆盖 `using_group`
   - rolling limit exhausted 时返回统一提示。

2. `model/value_package_test.go`
   - `GetValuePackageState()` 返回 usage summary：
     - 总额度进度；
     - 5 小时进度；
     - 7 天进度；
     - exhausted reason。
   - `ActivateValuePackage()` / `DeactivateValuePackage()` 不修改数据库 `users.group`，VIP 仍是 VIP。

3. `service/billing_session_test.go`
   - VIP + `wallet_only` + 套餐启用时：
     - 钱包余额不下降；
     - 套餐 `AmountUsed` 上升；
     - `ValuePackageUsageRecord` 写入；
     - relay effective group 不使用 VIP 特殊倍率的上下文由 middleware 测试覆盖。

建议验证命令：

```bash
go test ./model ./service ./controller ./middleware -count=1
```

### 6.2 前端单元/源码测试

新增或扩展：

1. `web/default/src/features/value-packages/components/value-package-card-source.test.ts`
   - 卡片包含 usage 进度条相关源码；
   - exhausted alert 文案存在；
   - 5h/7d 限额只在有限额时展示。

2. `web/default/src/features/value-packages/lib/benefit-effects.test.ts`
   - package glow 仍优先于 vip glow。

3. `web/default/src/features/value-packages/components/authenticated-benefit-effects-source.test.ts`
   - package glow class 指向蓝色 package modifier；
   - VIP modifier 仍存在。

4. `web/default/src/hooks/sidebar-data-model.test.ts`
   - 普通用户 `/value-packages` item 带 `attention: 'value-packages'`；
   - 管理员 `/value-packages` item 也带该标记。

5. 新增或扩展 `nav-group` source test：
   - `SidebarMenuLink` 对 attention item 添加 `yunbay-sidebar-value-package-pulse` class。

建议验证命令：

```bash
cd web/default
bun test \
  src/hooks/sidebar-data-model.test.ts \
  src/features/value-packages/lib/benefit-effects.test.ts \
  src/features/value-packages/components/value-package-card-source.test.ts \
  src/features/value-packages/components/authenticated-benefit-effects-source.test.ts
bun run typecheck
bun run i18n:sync
```

### 6.3 样式验收

手动或浏览器验收：

1. VIP 用户无套餐：边框为金色 VIP glow。
2. VIP 用户启用月卡：边框切换为淡蓝套餐 glow。
3. 普通用户启用日卡：边框为淡蓝套餐 glow。
4. 侧边栏“超值套餐”入口在普通用户和管理员视角都有呼吸动画。
5. `prefers-reduced-motion` 开启时没有循环动画。

## 7. 验收标准

本轮完成后应满足：

- 套餐发光是淡蓝色，VIP 发光是金色，两者同时存在时套餐蓝色优先。
- VIP 用户启用套餐后，请求上下文中的有效 user group、using group、token group 都切到套餐分组，计费不再套用 VIP 的特殊倍率。
- 关闭套餐使用后，不需要额外恢复数据库字段，下一次请求自然回到原普通/VIP 分组。
- 启用套餐期间用户钱包余额不被套餐请求消耗；套餐额度和套餐 usage record 正常增加。
- 套餐总额度、5 小时限额、7 天限额在有配置时显示进度条。
- 套餐额度用完时，卡片显示：`当前余额已用完，建议暂停使用，使用 API 或等时间跑完再使用`。
- 所有用户侧边栏的“超值套餐”入口有明显呼吸动画，且支持 reduced motion。
- 不修改 `infra/sub2api/frontend/pnpm-lock.yaml`、`infra/sub2api/frontend/package.json`、`infra/sub2api/backend/go.mod`。
- 不修改或移除受保护项目身份、版权、许可信息。

## 8. 非目标

- 不重做套餐购买、联动小铺支付、订单完成流程。
- 不新增新的支付渠道。
- 不改变日卡/周卡/月卡的购买覆盖规则。
- 不把套餐分组持久写入 `users.group`。
- 不自动在套餐额度耗尽后改扣钱包；必须由用户手动暂停套餐使用。
- 不修改 `infra/sub2api` 独立项目的依赖清单或构建入口。
