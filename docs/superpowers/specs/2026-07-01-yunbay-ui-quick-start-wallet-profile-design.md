# Yunbay 前端 UI 收敛与快速启动优化设计

日期：2026-07-01

## 背景

本轮优化聚焦 Yunbay 普通用户的首次使用、日常充值、个人资料与签到路径。当前前端已经具备快速启动五页流程、钱包充值/兑换码、个人资料页、签到日历、侧边栏模块自定义等能力，但在普通用户视角下仍存在几类体验问题：

1. 快速启动第 5 页同时展示 macOS、Windows 与 CC Switch 内容，信息密度偏高，用户不容易按自己的场景选择。
2. macOS 一键启动说明需要更明确地限定 Apple Silicon / M 系列芯片，并移除“复制终端安装命令”这种高风险、低必要操作，只保留下载后的修复命令。
3. 普通用户首页/导航仍可能暴露排行榜、文档、关于等非核心栏目，需要收敛为更直接的控制台体验。
4. 钱包菜单与充值卡片存在“立即充值 / 添加资金 / 兑换码”入口分散，以及在线充值未启用时出现“请联系管理员”类提示的问题。
5. 普通用户个人资料页显示“个性化设置左侧边栏显示内容”，对普通用户来说配置含义偏后台化，应隐藏。
6. 签到入口当前在个人资料页右侧，视觉优先级不够高；普通用户容易错过每日签到奖励。

本轮目标是在不改后端权限模型、不改支付/签到业务接口、不删除受保护项目信息的前提下，完成一轮前端 UI 和信息架构收敛。

## 设计目标

### 1. 快速启动第 5 页改成三张可展开选择卡片

第 5 页保留原有“Codex 一键启动/配置”主题，但右侧内容改为三个纵向排列的小卡片：

```text
我是新手（macOS，只支持 M 系列新品，Intel 芯片请下载 CCSwitch）
我是新手（Windows）
我有 CCSwitch
```

交互要求：

- 三张卡片一列展示，每张卡片都可点击展开。
- 点击未展开卡片时展开对应内容。
- 点击已展开卡片时收起。
- 点击另一张卡片时切换展开内容。
- 展开和收起必须有动画，避免直接跳变；优先使用项目已有的 `motion` 依赖完成 `height`、`opacity`、`y` 过渡。
- 动画只作用于当前页局部内容，不改变 `LandingSnapFrame` 的页面滚动/翻页机制。

默认状态建议：

- 默认不展开任何卡片，让用户先做场景选择。
- 如果后续人工验收认为空状态引导弱，可在实现阶段调整为默认展开 macOS，但本设计以“默认不展开”为基线。

### 2. macOS 展开内容收敛

macOS 展开内容复用当前 macOS 下载卡片的核心能力：

- 显示 `macOS` 平台标题。
- 显示下载说明。
- 显示下载一键启动软件按钮。
- 显示 Gatekeeper/损坏提示修复区。
- 保留 `复制修复命令`。

必须删除：

- `复制终端安装命令` 按钮。
- `terminalInstallCommand` 在 UI 中的所有入口。

必须新增或强化说明：

- macOS 一键启动软件只支持 Apple Silicon / M 系列芯片。
- Intel 芯片用户请使用 CC Switch 路径。

数据层建议：

- `CodexDownloadCard` 可以保留 `terminalInstallCommand` 字段用于兼容，但不再为 macOS 数据提供该字段，或者保留字段但不在 UI 渲染。
- 推荐从数据源移除 macOS `terminalInstallCommand`，让测试明确约束“UI 不再依赖一行安装命令”。

### 3. Windows 展开内容复用现有卡片

Windows 展开内容复用当前 Windows 下载卡片：

- 显示 `Windows` 平台标题。
- 显示下载说明。
- 显示下载一键启动软件按钮。
- 保留当前 Windows 使用说明区：下载安装、打开 Yunbay Codex、粘贴 API Key、保存启用、测试连接与管理会话。

本轮不修改 Windows 安装包来源、不改文件名、不改下载路径。

### 4. CC Switch 展开内容复用当前导入卡片

`我有 CCSwitch` 展开后展示当前第 5 页底部的 CC Switch 导入卡片能力：

- 展示配置 API 地址。
- 展示当前选中模型。
- 展示已生成 API Key 的脱敏值。
- 点击按钮尝试打开 `ccswitch://v1/import?...`。
- 如果还未生成 API Key 或未选择模型，保留现有 disabled/提示逻辑。

本轮不修改 CC Switch URL 构造逻辑。

## 普通用户导航收敛

### 1. 隐藏排行榜、文档、关于

普通用户登录后的导航/菜单中不显示：

- 排行榜 / `Rankings`
- 文档 / `Docs` / `Documentation`
- 关于 / `About`

实施边界：

- 优先处理普通用户侧边栏数据模型。
- 如果登录后顶部/首页导航复用公共导航配置，也要在登录态普通用户视角过滤这些入口。
- 不影响管理员需要的管理入口。
- 不影响未登录公开页的品牌展示，除非该入口明确出现在普通用户登录后页面。

### 2. 侧边栏钱包入口合并

普通用户侧边栏的钱包分组只保留一个主入口：

```text
Wallet
- Wallet / Top up
```

删除普通用户侧边栏中的单独兑换码入口：

```text
Redeem codes -> /wallet?section=redeem
```

原因：

- 钱包页本身已经包含充值与兑换码功能。
- 分散入口会让“立即充值”和“添加资金”像两个不同流程。
- 合并后普通用户只需要进入钱包页即可完成充值或兑换码操作。

管理员侧边栏中的充值卡、兑换码管理等后台功能不在本轮合并范围内。

## 钱包充值卡片文案收敛

目标文件：

- `/Users/ethan/Documents/yunbay/web/default/src/features/wallet/components/recharge-form-card.tsx`

当前在线充值未启用时会显示类似：

```text
Online topup is not enabled. Please use redemption code or contact administrator.
```

本轮要求：

- 不再显示“管理员未启动充值 / 请联系管理员”类提示。
- 在线充值未启用时，只隐藏在线充值区或让该区域自然为空。
- 仍展示可用能力：订单历史、兑换码、Creem 产品、订阅套餐、推广奖励等。
- 如果没有任何在线支付方式，但兑换码可用，用户应直接看到兑换码区域，不被“管理员未启用充值”提示打断。

实现策略：

- 保留 `hasAnyTopup`、`hasConfigurableTopup`、`redemptionEnabled` 等判断。
- 将 `hasAnyTopup ? ... : <Alert ...>` 的 Alert 分支移除或改为 `null`。
- 只删除提示展示，不改支付方式计算、不改支付确认逻辑。

## 个人资料页收敛

### 1. 普通用户隐藏侧边栏个性化设置卡片

目标文件：

- `/Users/ethan/Documents/yunbay/web/default/src/features/profile/index.tsx`
- `/Users/ethan/Documents/yunbay/web/default/src/features/profile/components/sidebar-modules-card.tsx`

当前个人资料页在右侧渲染 `SidebarModulesCard`，对应 UI：

- 标题：`Sidebar Personal Settings`
- 描述：`Customize sidebar display content`

本轮要求：

- 普通用户不显示该卡片。
- 管理员/超级管理员保留该卡片，前提是权限没有显式禁用。
- 如果后端用户权限里 `permissions.sidebar_settings === false`，继续隐藏。

建议条件：

```text
canConfigureSidebar = user.role >= ROLE.ADMIN && permissions.sidebar_settings !== false
```

这样普通用户即使后端没有显式禁用 `sidebar_settings`，前端也不会显示该后台化设置。

### 2. 签到卡片提升优先级

目标文件：

- `/Users/ethan/Documents/yunbay/web/default/src/features/profile/index.tsx`
- `/Users/ethan/Documents/yunbay/web/default/src/features/profile/components/checkin-calendar-card.tsx`

布局调整：

- `CheckinCalendarCard` 从右侧 sticky 栏中移出。
- 放到 `ProfileHeader` 下方、账号设置网格之前。
- 作为更宽的横向卡片展示，让用户进入个人资料页后优先看到签到。

视觉调整：

- 未签到状态更醒目：更强的主按钮、更突出的奖励文案、轻量渐变/强调边框/图标提示。
- 已签到状态降低紧迫感：显示已签到标签、今日奖励，并可自动折叠月历详情。
- 保留现有折叠/展开能力。
- 保留 Turnstile 安全校验逻辑。
- 保留当前 `checkedToday` 后自动折叠的逻辑。

建议文案：

- 标题继续使用 `Daily Check-in`。
- 未签到说明继续使用或强化 `Check in daily to receive random quota rewards`。
- 按钮可继续用 `Check in now`，中文翻译显示为“立即签到”。
- 不新增复杂活动文案，避免翻译范围扩大。

## 国际化设计

本项目 `web/default` 前端文案必须走 i18n。

新增或调整的用户可见文案需要覆盖：

- `en`
- `zh`
- `fr`
- `ru`
- `ja`
- `vi`

预计新增 key：

- `I am new to macOS (M-series only; Intel Macs should use CC Switch)`
- `I am new to Windows`
- `I already have CC Switch`
- `Only Apple Silicon / M-series Macs are supported. Intel Mac users should use CC Switch instead.`

如果实现时选择复用更短文案，可以减少新增 key，但不得出现未翻译的 `t()` key。

验证方式：

- 更新 quick-start locale 覆盖测试。
- 运行 `bun run i18n:sync`。

## 涉及文件

### 必改

- `/Users/ethan/Documents/yunbay/web/default/src/features/quick-start/index.tsx`
- `/Users/ethan/Documents/yunbay/web/default/src/features/quick-start/quick-start-data.ts`
- `/Users/ethan/Documents/yunbay/web/default/src/features/quick-start/quick-start-data.test.ts`
- `/Users/ethan/Documents/yunbay/web/default/src/features/quick-start/quick-start-locales.test.ts`
- `/Users/ethan/Documents/yunbay/web/default/src/hooks/sidebar-data-model.ts`
- `/Users/ethan/Documents/yunbay/web/default/src/hooks/sidebar-data-model.test.ts`
- `/Users/ethan/Documents/yunbay/web/default/src/features/wallet/components/recharge-form-card.tsx`
- `/Users/ethan/Documents/yunbay/web/default/src/features/profile/index.tsx`
- `/Users/ethan/Documents/yunbay/web/default/src/features/profile/components/checkin-calendar-card.tsx`
- `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/en.json`
- `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/zh.json`
- `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/fr.json`
- `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/ru.json`
- `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/ja.json`
- `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/vi.json`

### 可能需要改

- `/Users/ethan/Documents/yunbay/web/default/src/components/layout/config/public-landing-nav.config.ts`
- `/Users/ethan/Documents/yunbay/web/default/src/components/layout/components/public-navigation.tsx`
- `/Users/ethan/Documents/yunbay/web/default/src/hooks/use-top-nav-links.ts`
- `/Users/ethan/Documents/yunbay/web/default/src/i18n/static-keys.ts`

这些文件只有在确认普通用户登录态仍能看到排行榜、文档、关于入口时才修改。

### 不改

- 后端支付接口。
- 后端签到接口。
- 后端权限模型。
- Windows/macOS 下载文件本身。
- 受保护项目信息、品牌、版权、module path、README、license header。

## 测试方案

采用小范围 TDD：先改或新增能表达目标行为的测试，再改实现。

### 1. 快速启动数据测试

文件：

- `/Users/ethan/Documents/yunbay/web/default/src/features/quick-start/quick-start-data.test.ts`

覆盖：

- macOS 下载卡仍存在。
- Windows 下载卡仍存在。
- macOS `quarantineFixCommand` 仍存在。
- macOS 不再要求或不再暴露 `terminalInstallCommand`。
- macOS 支持范围说明 key 存在。

### 2. 快速启动翻译测试

文件：

- `/Users/ethan/Documents/yunbay/web/default/src/features/quick-start/quick-start-locales.test.ts`

覆盖：

- 三个折叠卡片标题在所有 locale 中有翻译。
- macOS M 系列限定说明在所有 locale 中有翻译。
- 中文 locale 不回落英文。

### 3. 侧边栏数据测试

文件：

- `/Users/ethan/Documents/yunbay/web/default/src/hooks/sidebar-data-model.test.ts`

覆盖普通用户：

- 不显示 `Redeem codes` 单独入口。
- 不显示排行榜、文档、关于入口。
- 仍显示 `Quick Start`、`Playground`、`Dashboard`、`Usage Logs`、`API Keys`、`Wallet / Top up`、`Profile`。

覆盖管理员：

- 管理员保留后台管理入口。
- 本轮不因普通用户收敛误删管理员管理能力。

### 4. 个人资料/签到轻量测试

如果项目现有测试环境不适合直接渲染 React 组件，优先添加源代码约束或抽取纯逻辑测试：

- 个人资料页侧边栏配置卡片只在管理员条件下渲染。
- `CheckinCalendarCard` 在 `ProfileHeader` 后优先出现，不再放在右侧 sticky 栏里。

如果实现中能用现有测试栈稳定渲染，则可补组件测试；否则不引入大范围测试基础设施。

## 验证命令

实现完成后至少执行：

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun test src/features/quick-start/quick-start-data.test.ts
bun test src/features/quick-start/quick-start-locales.test.ts
bun test src/hooks/sidebar-data-model.test.ts
bun run i18n:sync
bun run typecheck
```

如个人资料/签到新增了专用测试，也一并执行对应 `bun test` 命令。

如果时间允许，再执行：

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun run build
```

## 验收标准

1. 快速启动第 5 页只显示三个一列的可展开选择卡片。
2. 三个卡片可展开、可收起、可切换，动画自然。
3. macOS 展开内容只保留下载按钮和复制修复命令，不再显示复制终端安装命令。
4. macOS 展开内容明确提示只支持 M 系列 / Apple Silicon，Intel Mac 用户使用 CC Switch。
5. Windows 展开内容保留现有下载与使用说明。
6. CC Switch 展开内容保留现有一键导入能力。
7. 普通用户导航中不显示排行榜、文档、关于。
8. 普通用户钱包侧边栏只保留一个钱包/充值入口，不再单独显示兑换码入口。
9. 钱包页在线充值未启用时不显示“管理员未启动充值 / 请联系管理员”类提示。
10. 普通用户个人资料页不显示“Sidebar Personal Settings / Customize sidebar display content”卡片。
11. 管理员仍可看到侧边栏个性化设置卡片，除非后端权限显式禁用。
12. 签到卡片在个人资料页更靠前、更醒目，未签到时主操作明显。
13. i18n 同步后无新增缺失 key。
14. `bun run typecheck` 通过。

## 非目标

本轮明确不做：

1. 不改后端接口、数据库、权限模型。
2. 不新增支付渠道。
3. 不新增签到奖励规则。
4. 不改下载文件和下载产物来源。
5. 不重做整个个人资料页视觉设计。
6. 不新增复杂首页品牌改版。
7. 不删除或替换 `new-api`、`QuantumNous` 相关受保护项目信息。

## 风险与处理

### 风险 1：快速启动第 5 页内容变高导致小屏溢出

处理：

- 展开区域设置合理的最大高度和内部滚动。
- 保持三张选择卡片本身紧凑。
- 只在展开内容中滚动，不破坏整页 snap 体验。

### 风险 2：普通用户导航入口来源不止一处

处理：

- 先以 `sidebar-data-model.ts` 为准。
- 再检查顶部导航和公共导航配置是否在登录态复用。
- 只过滤普通用户登录态入口，不误伤公开页。

### 风险 3：隐藏侧边栏个性化设置影响已有普通用户自定义配置

处理：

- 只隐藏配置入口，不清空用户已有 `sidebar_modules`。
- `useSidebarConfig` 仍按已有数据过滤侧边栏。
- 如果用户过去配置过侧边栏，配置仍可继续生效，只是不再允许普通用户在前端修改。

### 风险 4：签到卡片更显眼但已签到后打扰用户

处理：

- 未签到时突出。
- 已签到后保留状态但自动折叠详情，降低视觉噪音。

## 实施顺序建议

1. 更新快速启动数据测试和 locale 测试，先看到失败。
2. 更新侧边栏数据测试，先看到失败。
3. 实现快速启动第 5 页折叠卡片和 macOS 文案/按钮收敛。
4. 实现普通用户导航与钱包菜单收敛。
5. 实现钱包充值未启用提示隐藏。
6. 实现个人资料页普通用户隐藏侧边栏个性化设置。
7. 调整签到卡片位置和样式。
8. 补全 i18n 并运行 `bun run i18n:sync`。
9. 运行单测、typecheck，必要时运行 build。

