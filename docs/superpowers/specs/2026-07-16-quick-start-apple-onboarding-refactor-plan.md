# Yunbay 快速启动苹果式新手引导重构计划

日期：2026-07-16

## 1. 目标与性能指标

本轮在不改变 Yunbay 现有暗色点云、字体层级和品牌视觉的前提下，重构普通用户快速启动流程。目标不是增加更多说明，而是把模型、软件、余额、API Key、CC Switch 导入和控制台接续组成一条可恢复、可验证的闭环。

完成标准：

- 快速启动固定为 5 页：用途、模型、软件、账户准备、导入检查。
- 默认模型优先精确选择 `gpt-5.6-sol`；思考强度作为独立配置固定为 `xhigh`（界面显示“极高”），严禁拼接成不存在的 `gpt-5.6-sol-thinking`；目标模型不可用时明确显示实际回退模型。
- 默认模型必须在模型页首屏可见，并显示推荐/当前选择状态。
- 软件下载只保留 Mac Universal 与 Windows 64 位两个官方 CC Switch GitHub 直链。
- 充值/兑换与 API Key 合并为一页，并根据真实余额和现有快速启动 Key 显示完成状态。
- 已存在的 `yunbay-quick-start-*` Key 可恢复和复用，复查流程不得重复创建 Key，也不得把完整 Key 写入 `localStorage` 或 `sessionStorage`。
- 导入检查区分：模型已选、软件已确认安装、协议已尝试打开、用户确认完成。
- 退出动画在正常设备约 900ms 完成，最大 1500ms 后无条件进入控制台；减少动态效果偏好下使用短淡出。
- 控制台完成引导只在系统公告处理完后出现，不允许两个弹窗叠加。
- 桌面与 390x844 移动端均无内容裁切、底栏遮挡、按钮重叠和重复“进入控制台”。
- 所有新增文案覆盖 en、zh、fr、ja、ru、vi。

## 2. 既有系统与外部参考

### 当前实现

- 快速启动入口：`web/default/src/features/quick-start/index.tsx`
- 快速启动数据：`web/default/src/features/quick-start/quick-start-data.ts`
- API Key 生成：`web/default/src/features/quick-start/quick-start-api-key.ts`
- CC Switch 深链：`web/default/src/features/quick-start/quick-start-cc-switch.ts`
- 翻页控制器：`web/default/src/features/home/components/landing-snap-frame.tsx`
- 系统公告队列：`web/default/src/hooks/use-notifications.ts` 与 `web/default/src/components/layout/components/app-header.tsx`
- 控制台现有设置引导：`web/default/src/features/dashboard/components/overview/overview-dashboard.tsx`

### GitHub 参考

- 官方仓库：`farion1231/cc-switch`
- 2026-07-16 核对的最新版本：`v3.17.0`
- macOS 工作流构建目标：`universal-apple-darwin`
- macOS 资产：`CC-Switch-v3.17.0-macOS.dmg`
- Windows 默认资产：`CC-Switch-v3.17.0-Windows.msi`
- Windows ARM64 虽然存在，但本轮按用户确认只展示一个常用 Windows 入口，不增加架构选择负担。

## 3. 最小充分状态模型

快速启动只保存非敏感、设备相关的短期状态；账户状态继续以服务端为准。

| 状态 | 测量来源 | 完成条件 | 失败/恢复动作 |
| --- | --- | --- | --- |
| 用途 | React 页面状态 | 已有默认用途或用户选择 | 返回用途页 |
| 模型 | 后端 pricing 模型列表 | 存在有效当前模型 | 目标缺失时显示实际回退模型 |
| 下载 | 当前会话非敏感状态 | 用户点击 Mac 或 Windows 下载 | 保留两个下载入口供重试 |
| 余额 | 当前用户数据 | 余额大于 0，或用户明确继续 | 在同页充值/兑换并刷新用户 |
| API Key | 服务端 Key 列表与 reveal 接口 | 恢复或生成有效 Key | 显示错误并允许重试 |
| 软件安装 | 用户确认 | 用户点击“已经安装” | 点击“还没有”返回软件页 |
| 导入尝试 | `ccswitch://` 用户手势 | 已触发协议链接 | 提供重试与重新下载 |
| 导入完成 | 用户确认 | 用户点击“已打开，完成导入” | 不自动把协议启动等同于成功 |
| 控制台接续 | 会话完成摘要 | 公告弹窗关闭后显示完成引导 | 最大等待由公告真实状态控制 |

完整 API Key 仅保留在 React/React Query 内存中，并可通过服务端恢复；会话摘要只记录模型名、平台、完成标志，不记录完整 Key。

## 4. 页面与交互设计

### 第 1 页：用途

- 保留当前三种用途和默认选择。
- 保留现有点云构图和选择动效。

### 第 2 页：模型

- 默认模型精确匹配 `gpt-5.6-sol`，思考强度独立记录为 `xhigh`。
- 将当前选择移动到列表首位，避免默认模型在首屏之外。
- 目标模型标题显示 `GPT 5.6 Sol`，另以“极高思考”状态明确展示 `xhigh`；其它模型保持后端名称，不把思考档位伪装成模型后缀。
- 保留真实费率，不合成前端模型。

### 第 3 页：软件

- 右侧改为上下排列的两个平台下载行：Mac、Windows。
- Mac 显示 `Universal · Intel 与 Apple Silicon`。
- Windows 显示 `Windows 10/11 · 64 位`。
- 下载按钮使用原生链接新标签打开 GitHub 资产，避免当前引导页被替换。
- 点击后记录平台并显示轻量完成状态；不声称网站已经检测到安装成功。

### 第 4 页：账户准备

- 左侧继续显示真实余额、当前模型和用途摘要。
- 右侧按 1、2 两步排列：充值/兑换；生成或复用 API Key。
- 已有余额时第 1 步自动完成，但仍保留充值与兑换入口。
- 已有快速启动 Key 时第 2 步自动恢复；按钮改为复制现有 Key，不再创建重复 Key。
- 剪贴板失败时保留 Key 状态并提供再次复制。

### 第 5 页：准备检查与一键导入

- 使用单页渐进检查，不连续弹三个阻塞式对话框。
- 模型状态由系统自动确认。
- 软件安装仅询问一次，提供“已经安装”和“还没有”两个明确动作。
- “还没有”直接返回软件下载页。
- 确认安装后展示 API、模型和脱敏 Key，以及“一键导入”。
- 触发 CC Switch 后展示“已尝试打开”的恢复面板；用户可以重试、重新下载或确认完成。
- 只有用户确认完成后才启动退出动画。

### CC Switch `xhigh` 兼容边界

- 2026-07-16 按 `v3.17.0` 标签源码复核：公开深链 parser 接收 `config` / `configFormat`，但 Codex provider 构建函数会重新生成 `config.toml`，并把 `model_reasoning_effort` 固定为 `high`。
- `parse_and_merge_config` 只从内嵌配置提取 API Key、endpoint 和 model，不能保留内嵌 TOML 中的 `model_reasoning_effort = "xhigh"`；添加未知查询参数同样会被 parser 丢弃。
- 因此本站可以可靠一键导入 `gpt-5.6-sol`，但在官方 `v3.17.0` 上不能仅靠公开深链保证 `xhigh`。实现与文案不得声称已自动写入极高档；若上游增加 reasoning effort 深链字段，应以运行时复测为准再启用。

### 底部控制器

- 控制器整体水平居中，使用统一毛玻璃底座。
- 上一页为次按钮，进度居中，下一页/进入控制台为唯一主按钮。
- 中间页面不再常驻一个同等级“进入控制台”按钮；保留弱化的“稍后设置”文本入口。
- 主按钮桌面高度约 52px，次按钮至少 44px；移动端保持单行，不覆盖内容。
- 页面内容增加基于 `env(safe-area-inset-bottom)` 的安全区。

### 退出动画

- 整页使用 Web Animations API 驱动非线性 `clip-path + blur + opacity + translate` 三段关键帧，形成纸面被水线吞没并融入点云的效果。
- 动画期间锁定重复点击。
- `prefers-reduced-motion` 下仅执行短淡出。
- 动画完成事件负责导航，另保留一次性定时保险；完成事件和超时只能触发一次。

### 控制台完成引导

- 在 AppHeader 已有公告状态之后排队展示，不修改公告强制阅读规则。
- 展示模型、软件、账户、导入四项完成摘要。
- 操作为“复查设置”和“开始使用”。
- “复查设置”返回 `/quick-start#readiness` 并恢复非敏感会话状态。
- “开始使用”关闭完成引导并留在控制台。

## 5. 预期代码范围

- 重构 `web/default/src/features/quick-start/index.tsx`。
- 更新 `web/default/src/features/quick-start/quick-start-data.ts`。
- 扩展 `web/default/src/features/quick-start/quick-start-api-key.ts`，增加恢复现有快速启动 Key 的纯逻辑。
- 新增一个快速启动会话摘要模块，连接快速启动和 AppHeader 完成引导。
- 新增一个控制台完成引导组件，并在 AppHeader 中按公告状态排队。
- 更新快速启动及 AppHeader 相关单元/源码约束测试。
- 更新六个 locale JSON 和静态 i18n key 清单。
- 不改后端 API、不改数据库、不改支付业务、不改公告已读语义、不改受保护品牌信息。

## 6. 验证计划

### 自动化

- 快速启动数据顺序、默认模型、下载链接和平台数量测试。
- 现有 Key 选择、恢复、复制失败和不重复创建测试。
- CC Switch 深链参数回归测试。
- 会话摘要不包含完整 Key 的测试。
- 控制器唯一主 CTA、无重复控制台按钮和退出超时保险源码测试。
- 完成引导排在公告之后的源码/状态测试。
- `bun run i18n:sync` 并核对六种语言。
- `bun run typecheck`。
- `bun run lint`，若仓库存在既有错误则记录并限定本次文件。
- `bun run build`。

### 浏览器闭环

- 桌面：用途 → 默认模型可见 → Mac/Windows 下载状态 → 账户页 → 安装确认 → 导入尝试 → 完成确认 → 退出动画 → 公告 → 完成引导。
- 移动端 390x844：重复同一流程，确认按钮、长文案、下载行、账户步骤和导入面板不重叠。
- 检查浏览器控制台无新增 error。
- 检查 `prefers-reduced-motion` 降级路径。
- 不在自动化验证中实际下载大型安装包、创建多余 Key、兑换代码或更改公告已读状态；需要副作用的动作使用可逆测试状态或 DOM/链接验证。

## 7. 部署与运维记录

- 本地验证全部通过后提交到当前仓库。
- 推送并合并到 GitHub `main`；只提交本轮文件，不纳入用户已有未跟踪文件。
- 生产部署前备份将覆盖的前端文件和当前镜像。
- 仅重建并重启 Yunbay 前端所在服务，目标中断小于 1 分钟。
- 生产验证 `GET /quick-start`、入口资源、桌面/移动端关键流程和服务健康状态。
- 将部署命令、提交、备份位置、镜像、验证结果追加到现有 `docs/yunbay-maintenance.md`，不新建第二份运维手册。

## 8. 实施节点

| 节点 | 状态 | 验收条件 |
| --- | --- | --- |
| 现状与 GitHub 资产核验 | 已完成 | 已确认官方 Universal Mac 与 Windows x64 资产 |
| 完整计划与状态模型 | 已完成 | 本文件建立并作为唯一实施计划维护 |
| 数据、Key 恢复与会话状态 | 已完成 | 25 个相关单元测试通过 |
| 五页 UI 与苹果式控制器 | 已完成 | 五页结构、居中控制器、移动端内容滚动已实现 |
| 导入确认、退出动画、控制台接续 | 已完成 | 导入需人工确认，退出有一次性超时保险，完成弹层排在强制公告之后 |
| 六语言与自动化验证 | 已完成 | 六语言缺失/未翻译均为 0，49 个快速启动测试、typecheck、涉及文件 ESLint 与生产构建通过 |
| 浏览器桌面/移动端验收 | 已完成 | 1280x720 与 390x844 全流程、公告队列、复查/直接开始和减少动态效果均通过 |
| GitHub main、生产部署与运维记录 | 进行中 | 待提交 main、生产健康验证并追加现有运维记录 |

## 9. 实施验证记录

### 运行时修正

- 浏览器实测发现应用外层的 `filter: blur(0px)` 会让仅使用 `fixed inset-0` 的快速启动根节点相对零高度包含块计算，导致普通 DOM 被裁切，而 WebGL 合成层仍泄漏显示。根节点现显式使用 `100dvh`，点云层改为页内绝对定位，点云与内容可稳定同屏。
- 导入面板和导入结果卡展开后会自动滚入底部控制器上方；减少动态效果时使用即时定位。
- 390x844 下内容滚动视口在底部控制器上方结束，控制器不再覆盖用途卡、下载按钮、账户操作或导入结果。

### 自动化结果

- `bun test src/features/quick-start/*.test.ts`：49 pass / 0 fail。
- `bun run typecheck`：通过。
- 本轮涉及的 TS/TSX/MJS 文件 ESLint：通过。
- `bun run i18n:sync`：en、zh、fr、ja、ru、vi 的 missing、extras、untranslated 均为 0。
- `bun run build`：通过，Rsbuild 生产构建成功。
- `git diff --check`：通过。
- 仓库全量检查的既有边界：全量 lint 仍有 113 errors / 6 warnings，全量 format 检查仍有 61 个既有未格式化文件，版权全量检查仍有 17 个既有文件待更新；本轮文件不在这些失败项中。

### 浏览器闭环

- 桌面 1280x720：用途、默认 `gpt-5.6-sol`、独立“极高思考”、Mac/Windows 官方直链、账户合并页、Key 恢复、安装确认、协议导入、人工确认和控制台跳转均通过。
- CC Switch 协议捕获确认 `model=gpt-5.6-sol`，没有 `gpt-5.6-sol-thinking`，也没有伪造当前上游不支持的 `model_reasoning_effort` 参数。
- 强制公告先出现，关闭公告后才显示完成摘要；“我需要复查一遍”返回 `/quick-start#readiness`，“我不需要，直接开始”关闭摘要并清除待提示状态。
- 移动端 390x844：无横向溢出，页内滚动、两种下载、账户操作、导入面板、结果卡与底部控制器均无重叠。
- `prefers-reduced-motion: reduce`：短淡出分支可进入控制台，测试后已恢复浏览器媒体偏好与视口。
- 退出动画运行时采样：约 300ms 时 `clip-path` 为波浪多边形、`filter` 约 `blur(1.59px)`、`opacity` 约 `0.936`，并存在轻微下沉缩放；减少动态效果约 70ms 时只改变 `opacity`，`clip-path`、`filter`、`transform` 均保持不变。
- 浏览器控制台在本轮闭环期间无新增应用错误；记录中仅保留 mock 修正前的旧错误和主动模拟减少动态效果产生的 Motion 提示。
