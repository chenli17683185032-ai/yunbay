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
| GitHub main、生产部署与运维记录 | 已完成 | `6e8ec939` 已推送 main；生产 11 秒切换并通过健康验证；按用户最新指示保持当前版本，未执行回滚 |

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

### GitHub 与生产结果

- 发布提交：`6e8ec9399c10a2b844336c3b38c7cae9c4c2a098`，已推送并与 GitHub `main` 同步。
- 精确同步文件数：21；本地与生产组合 SHA-256 均为 `599de8b839ca93001228875766b197a430b2b00a656e0b548a4c7fa19cf343f0`。
- 生产备份：`/opt/new-api/backups/quick-start-onboarding-20260715T182454Z-6e8ec939`；旧镜像回滚标签：`yunbay-new-api:rollback-quick-start-20260715T182454Z`。
- 新镜像：`sha256:3e38bc6f23e74bd51b19a326dbd63aa84b5efc7b2a029378bbab0220da5e403f`；发布标签：`yunbay-new-api:release-6e8ec939`。
- 仅使用 `--no-deps --force-recreate --no-build` 替换 `yunbay-new-api`，11 秒恢复 healthy；其它生产容器均未重启。
- 上线后内网状态探针 10/10 为 200，公网 `/`、`/quick-start`、`/api/status` 均为 200；公开入口为 `/static/js/index.3fdb276058.js`，实际 bundle 包含目标模型与 CC Switch v3.17.0 资产标记，不含错误 thinking 后缀。
- 发布完成后收到“先不要上线”的补充指示时，已先执行只读回滚预检查；在任何源码恢复、镜像重标记、Compose 重建或服务重启发生前，用户进一步明确要求不要撤回、不要打断服务器运行。因此取消回滚，生产继续运行上述新镜像，预检查没有造成服务中断。

## 10. 第二轮：页内动效、品牌图标与对齐精修

### 10.1 目标与性能指标

本轮不改变五页信息架构、默认模型、下载地址、账户动作或导入语义，只修正用户在生产验收中指出的反馈与视觉稳定性问题。把一次点击抽象为“输入 → 状态变化 → 可见反馈 → 稳态”闭环，优先保证状态可理解和布局稳定，再追求动效表现。

- 用途、模型、软件、账户和准备检查中的页内状态变化必须有可见反馈，不再瞬时换色或突然重排。
- 选择反馈使用统一的非线性弹簧或 `cubic-bezier(0.16, 1, 0.3, 1)`；普通反馈约 180-360ms，模型重排在约 550ms 内稳定，不使用循环或纯装饰动画。
- 模型选择导致列表重排时，卡片必须从旧位置连续移动到新位置，不允许跳帧换位。
- `prefers-reduced-motion: reduce` 下取消位移、缩放和弹簧，只保留不超过 120ms 的必要透明度反馈。
- 下载页 Mac 与 Windows 操作按钮使用相同高度和宽度；同类步骤操作在桌面与 390x844 移动端使用一致对齐轴，无横向溢出、换行挤压或点击后尺寸变化。
- Mac 使用 Apple 品牌标识，不再使用 Lucide 的水果 `Apple` 图标；Windows 同步使用准确的平台品牌标识，保持同一图标体系和视觉尺寸。
- 页面控制器继续保持唯一主操作、居中和稳定尺寸；按钮按下有克制反馈，跨页仍由现有纵向非线性过渡负责，不叠加干扰方向判断的第二套整页动画。

### 10.2 现状测量与最小充分改动

- 跨页控制器已经通过 `LandingSnapFrame` 使用 900ms 纵向滑动，问题不在跨页状态机，不修改共享翻页组件。
- 用途和模型卡当前只依赖 Tailwind `transition-all`；模型选择后还会重新排到首位，缺少布局插值，是“跳动”的主要来源。
- 软件行当前使用 `Apple` 与 `MonitorCog`，前者是水果图标、后者不是 Windows 品牌标识；两行按钮宽度随翻译文案变化。
- 安装确认、Key 状态与导入面板已有部分 `AnimatePresence`，但步骤编号切换为完成状态仍是瞬时替换。
- 新增一个快速启动专用动效模块，集中维护选择底板、选中标记、步骤标记和动效参数；在现有组件中复用，不引入新依赖。
- 用途卡、模型卡和软件行使用共享选择底板与进入/退出标记；模型卡使用 Motion layout 插值完成重排。
- 账户步骤与准备检查使用同一个带交叉淡入和轻微缩放的状态标记；按钮本体保持现有 Base UI 可访问性与键盘行为。
- 下载行改为稳定网格轨道：内容列自适应、操作列固定；移动端按钮占满可用宽度，桌面端同宽。

### 10.3 GitHub 经验参考

- `motiondivision/motion`：沿用项目已经安装的 Motion React，采用 shared layout、layout position 与 `AnimatePresence` 表达连续状态变化。
- `ibelick/motion-primitives`：参考其把动效封装为可复用原语的做法，只抽取本功能需要的最小动效组件，不复制视觉模板。
- `simple-icons/simple-icons`：项目已有 `react-icons` 依赖，Apple 使用 Simple Icons 品牌标识，Windows 使用同一依赖中的 Font Awesome Brands 标识；不新增包或手写近似 SVG。

### 10.4 验证闭环

- 扩展快速启动源码约束测试：品牌图标来源、专用选择动效、模型 layout 重排、稳定下载操作列和 reduced-motion 分支。
- 运行快速启动全部测试、TypeScript typecheck、涉及文件 ESLint/Prettier、六语 i18n 同步检查、生产构建和 `git diff --check`。
- Browser 桌面验收：用途选择反馈、下一页、模型选择并重排、Mac/Windows 下载行对齐、安装确认状态切换、导入面板出现。
- Browser 390x844 验收：最长可见文案下按钮仍对齐，内容不被底栏遮挡，无横向溢出。
- 运行态采样选择动画的中间帧与最终帧，证明位置/透明度/缩放随时间变化且最终无残余 transform；另验证 reduced-motion 不执行空间位移。

### 10.5 发布控制

- 本地闭环通过后只提交本轮文件并推送 GitHub `main`，不纳入现有未跟踪设计稿或 `outputs/`。
- 生产先备份本轮覆盖文件和当前健康镜像；构建期间现有容器继续服务。
- 使用已验证的绿实例与 Caddy graceful reload 路径完成无中断切换；不重启 PostgreSQL、Redis、Caddy、Sub2API 或 LDXP 相关服务。
- 只有绿实例内部健康、目标静态资源可识别且公网持续探针稳定后才切流；异常时保持当前健康生产实例，不执行无界等待。
- 发布结果、镜像、回滚点、探针和真实切流结果继续追加到 `docs/yunbay-maintenance.md` 与桌面现有运维手册。

### 10.6 实施节点

| 节点 | 状态 | 验收条件 |
| --- | --- | --- |
| 生产现状与问题测量 | 已完成 | 功能提交、生产镜像、健康状态和三类视觉根因已确认 |
| GitHub 经验与完整计划 | 已完成 | Motion、Motion Primitives、Simple Icons 参考和本节计划已记录 |
| 页内选择与步骤动效 | 已完成 | 用途、模型、软件和步骤状态均形成连续反馈，减少动态效果可降级 |
| 品牌图标与按钮对齐 | 已完成 | Apple/Windows 标识准确，下载操作列桌面与移动端稳定同宽 |
| 自动化与浏览器闭环 | 已完成 | 测试、typecheck、lint、format、i18n、build 及双视口交互验收通过 |
| GitHub 与无中断生产同步 | 已完成 | main、生产标记和运维记录均已同步；绿实例切流期间无连接失败或 5xx，最终公网与容器健康通过 |

### 10.7 实施验证记录

- 用途、模型和软件状态使用共享选择底板；模型卡启用 position layout 插值，选中原列表末位 `grok-build` 时，运行态可观测到非 `none` 位移矩阵并在约 520ms 后归零，列表不再跳变。
- 用途选择底板从第一张卡移动到第二张卡时，点击返回帧为 `translateX(-3.84px)`、约 90ms 为 `translateX(-0.41px)`，最终 transform 为 `none`；透明度同步收敛到 1。
- 安装确认步骤 `02` 点击后从 `opacity: 0; scale: 0.68; translateY(5px)` 进入，约 50ms 达到 `opacity: 0.922; scale: 0.976; translateY(0.38px)`，最终稳定为勾选且导入面板同步出现。
- `prefers-reduced-motion: reduce` 在完整 reload 后复测：用途选择前、点击帧和最终帧的 transform 均为 `none`；测试结束已清除媒体模拟并恢复默认偏好。
- 桌面下载页两行按钮最终均为 `224x48`；移动端两行按钮均为 `316x48`，`x=37`。Apple 使用品牌标识，Windows 使用平台标识。
- 账户页浏览器验收发现并修复首版横向挤压：步骤说明与三项操作改为上下两层，桌面同类操作列等宽且均为 44px 高，移动端均为 `316x44`；单一 Key 操作移动端也调整为 `316x44`。
- 最长文案补充验收：法语 Mac/Windows 下载按钮均为 `224x48` 且 `scrollWidth == clientWidth == 222`；俄语充值/兑换按钮均为 `176x44`、输入框为 `190x44`，均无文字溢出。
- 俄语移动端最终主按钮初测在对称三列中被压为三行，虽未溢出但不接受；最终页移动布局改为返回键、进度、剩余宽度主按钮后，按钮为 `228x48`，标签单行高度 15px，`scrollWidth == clientWidth == 228`。
- 1280x720 与 390x844 的 `documentScrollWidth` 均不超过视口；移动端内容滚动区底部为 720px、控制器顶部为 734px，导入按钮底部为 673px，没有底栏遮挡。
- 本轮 Browser 控制台按 `:5173` 过滤后 error/warn 均为 0；未点击真实下载、充值、兑换、Key 创建或协议导入。
- `bun test src/features/quick-start/*.test.ts`：51 pass / 0 fail。
- `bun run typecheck`、涉及文件 ESLint、Prettier、`bun run build`、`git diff --check` 均通过；最终生产构建入口为 `dist/static/js/index.4f358c87c4.js`。
- `bun run i18n:sync` 后 en、zh、fr、ja、ru、vi 的 missing、extras、untranslated 均为 0，本轮没有新增可见文案或 locale 变更。
- 本轮发布提交为 `a048627a338ca19b2ad7c4930bfae15ed798c61a`，已与 GitHub `main` 同步；四个精确同步文件在本地与生产逐项 SHA-256 一致，哈希清单组合值为 `fdfc8da280151ff290b7ace8fe1e7f73612867300ef09d4176cd447db1721490`。
- 发布前回滚点为 `/opt/new-api/backups/quick-start-motion-20260715T194510Z-a048627a`；旧镜像 `sha256:3e38bc6f23e74bd51b19a326dbd63aa84b5efc7b2a029378bbab0220da5e403f` 保留为 `yunbay-new-api:rollback-quick-start-motion-20260715T194510Z`。
- 构建期间旧生产容器 ID、启动时间和健康状态均未变化。新镜像为 `sha256:bd931400a56e9ad0b43c776bbffac26075267a91ac2362c596880b655a4857fa`，审计标签为 `yunbay-new-api:release-a048627a`。
- 绿实例使用当前生产容器解析后的环境与相同挂载，未映射宿主机端口；补齐 Compose 服务层 HEALTHCHECK 后达到 `healthy`，入口 `/static/js/index.4f358c87c4.js` 的 SHA-256 为 `08c82ce58f3adb7fc781366b41beb37bd2a7e147d74303cbd7356a7e2ddca3b7`。
- 临时 Caddyfile 仅把唯一 upstream 从 `new-api:3000` 改为 `new-api-green:3000`，容器内 `caddy validate` 通过后执行 graceful reload。绿实例承载流量时只用 Compose `--no-deps --force-recreate --no-build` 重建标准 `new-api`，标准实例 `healthy` 后验证正式 Caddyfile并 graceful reload 切回。
- 正式宿主机 Caddyfile SHA-256 始终为 `655d48c14d94372bf2383af094899fc3e3e8c54fe24b4476ba3ae7f887302388`；Caddy 容器 ID、启动时间和重启计数未变化。PostgreSQL、Redis、Sub2API、CLI Proxy、LDXP proxy 和 worker 也未重启。
- 切换窗口公网持续探针 `159/159` 为 HTTP 200。源站高频探针共 2400 次，全部收到 HTTP 响应：718 次 200、1682 次由探针自身触发的 429，没有连接失败或 5xx；停止高频探针后限流恢复。最终公网 `/`、`/quick-start`、`/api/status` 均为 200，`start_time=1784168769`，bundle 文件名、字节数和 SHA-256 与本地一致。
- 一次本机到代理/Cloudflare 的 TLS 握手在收到 HTTP 状态前超时；同一时段服务端无 5xx、容器无重启，随后固定出口最终复核再次为 200，因此按客户端链路抖动记录，不记为服务端中断。
- 生产 `.yunbay-deploy-sha` 与 `.yunbay-source-manifest` 已原子更新到 `a048627a` 和新镜像；绿实例、临时环境文件、临时 Caddyfile、构建日志及探针日志均已清理，回滚目录和镜像标签保留。

## 11. 第三轮：单按钮两阶段导入 Provider 与生图提示词

### 11.1 目标与性能指标

在官网原版 CC Switch 只支持单一 `resource` 的约束下，把现有导入操作综合为一个可恢复的两阶段控制器：第一次用户点击导入 Provider，按钮随后切换为“继续导入推荐提示词”；第二次用户点击导入并启用 Prompt。稳定性优先，不尝试由一次点击连续拉起两个自定义协议。

完成标准：

- 第一阶段保持现有 `resource=provider` URL、模型、endpoint、真实 API Key 与 `enabled=true` 不变。
- Provider Deep Link 发出后，使用既有 `importAttempted` 作为反馈量切换同一按钮，不新增第二个按钮或敏感持久化状态。
- 第二阶段使用 `resource=prompt`、`app=codex`、固定名称、UTF-8 Base64 正文与 `enabled=true`。
- Prompt 正文严格使用用户指定的生图规则，并把当前内存中的同一个规范化真实 API Key 写入 `API Key为：` 后；不得使用占位符、脱敏值或另一把 Key。
- 完整 API Key 不写入 `localStorage`、`sessionStorage`、日志、toast、测试快照或可见页面；但按用户明确要求，它会经 Deep Link 传给 CC Switch 并最终以明文存在 `~/.codex/AGENTS.md`。
- 页面刷新后，既有会话摘要可恢复到 Prompt 阶段；现有“重试”继续只重开 Provider，避免丢失恢复路径。
- 新增界面文案覆盖 en、zh、fr、ja、ru、vi，桌面与 390x844 移动端按钮不溢出、不跳动。

### 11.2 协议与安全边界

- 2026-07-16 已复核 `farion1231/cc-switch` 主分支：parser 对 `provider` 与 `prompt` 分开分派；Prompt 的 `content` 由 Base64 解码并校验 UTF-8，`enabled=true` 会保存后启用。
- Base64 不是加密；Provider URL 与 Prompt URL 都会携带完整 API Key。实现只能避免额外泄露面，不能把用户要求写入 `AGENTS.md` 的 Key 变成密文。
- 第二阶段仍需一次明确用户点击和 CC Switch 的独立确认；网页不声称两项已被客户端自动完成。

### 11.3 最小充分实现

- 扩展 `quick-start-cc-switch.ts`：增加提示词名称、由 API Key 生成精确正文、UTF-8 Base64 编码和 Prompt URL 构造器，不增加依赖。
- 扩展 URL 单元测试：解码 Prompt 后验证固定规则、完整同款 Key、`enabled=true`，并验证 Prompt URL 不含 Provider 专属参数。
- 调整 `index.tsx`：增加 Prompt handler；导入面板根据 `importAttempted` 在同一个稳定尺寸按钮内切换 handler、图标和标签。
- 更新源码约束测试、六语言 locale 与静态 key；不修改后端、数据库或 CC Switch 客户端。

### 11.4 验证闭环

- 自动化：`bun test src/features/quick-start/*.test.ts`、`bun run i18n:sync`、`bun run typecheck`、涉及文件 ESLint/Prettier、`bun run build`、`git diff --check`。
- 协议捕获：使用测试 Key 捕获两次 URL，确认第一阶段是 Provider，第二阶段解码后 Prompt 含同一个 Key；验证输出不打印真实生产 Key。
- 浏览器：1280x720 与 390x844 验证首次点击后同按钮切换、长翻译无溢出、现有确认和 Provider 重试路径仍可用；不在自动化中真正改写本机 `~/.codex/AGENTS.md`。

### 11.5 实施节点

| 节点 | 状态 | 验收条件 |
| --- | --- | --- |
| 上游协议、现有状态与安全边界核验 | 已完成 | 已确认原版 CC Switch 需两次用户手势及明文 Key 边界 |
| 完整计划与最小状态模型 | 已完成 | 本节已写入既有单一计划文件 |
| Prompt 构造、阶段切换与测试 | 已完成 | 同一按钮按 `importAttempted` 切换且两次 URL 使用同一 Key |
| 六语言与工程验证 | 已完成 | 六语言无缺失，测试、类型、lint、format 与生产构建通过 |
| 桌面与移动端浏览器验收 | 已完成 | 两种视口切换稳定、参数正确、Provider 重试可用 |
| GitHub main、生产部署与运维记录 | 已完成 | 功能提交已推送 main，绿实例部署完成并追加既有运维手册 |

### 11.6 实施验证记录

- `quick-start-cc-switch.ts` 新增纯函数构造生图 Prompt；Provider 与 Prompt 都使用同一个 `effectiveApiKey`，并统一补全 `sk-` 前缀。Prompt URL 只携带 `resource=prompt`、`app=codex`、名称、Base64 正文和 `enabled=true`，不重复添加 Provider 专属参数。
- 同一按钮使用既有 `importAttempted` 切换处理函数：初始打开 Provider，随后显示“继续导入推荐提示词”；现有“重试”仍重新打开 Provider，刷新后不需要持久化完整 Key。
- 浏览器初测发现英文长文案会把桌面右侧网格列从 230px 撑到 310px；网格轨道改为 `minmax(0, …)`，按钮允许固定 48px 高度内平衡换行后，切换前后尺寸一致。
- 使用测试 Key 捕获两条 Deep Link：第一条为 Provider 且 `apiKey=sk-test-import-key`；第二条为 Prompt，Base64 解码后包含 `API Key为：sk-test-import-key`、固定图片端点、`gpt-image-2` 限制和 `outputs/` 保存规则。测试没有读取或输出真实生产 Key。
- 1280x720 与 390x844 浏览器验收均通过：按钮切换前后宽高一致、没有横向溢出、控制台无错误；现有人工确认和 Provider 重试操作保持可用。
- `bun test src/features/quick-start/*.test.ts`：53 pass / 0 fail；`bun run typecheck`、涉及文件 ESLint、Prettier 与 `bun run build` 均通过。
- `bun run i18n:sync` 后 en、zh、fr、ja、ru、vi 的 missing、extras、untranslated 均为 0；六个 locale 各只新增两行，没有排序噪声。
- 功能提交 `ee953c5954f85e61a4004f0df26f9043e696cc10` 已普通 fast-forward 推送 GitHub `main`。生产精确同步 11 个文件，组合 SHA-256 为 `9d66e3710df70ef1a2522224e2e0f75301995a9b8938b3769dc659c97f3dc26c`。
- 新镜像为 `sha256:3db9cb74a065b3aec8dbfae5919c038739c96bdf728b43794491b2fb86dbb91a`；绿实例 healthy 后经 Caddy graceful reload 承载流量，标准 `new-api` 仅用 Compose `--no-deps --force-recreate --no-build` 在 2 秒内重建，再平滑切回。Caddy、PostgreSQL、Redis、Sub2API、CLI Proxy、LDXP proxy 和 worker 均未重启。
- 最终公网 `/`、`/quick-start`、`/api/status` 各 3/3 为 200；入口 `/static/js/index.04106f6b05.js`，bundle SHA-256 `211fdddf75fef5856d5f1288c8c6b96585d752d9d77d860d4c8ff3cc2196079c`、字节数 `3064077`。
- 切换窗口本机公网探针 180 次中 177 次为 200，编号 8/80/152 的三次在 HTTP 状态前失败；同窗 Caddy 5xx=0、upstream error=0、应用严重日志=0，且三次间隔规律，按本机代理/Cloudflare 传输链路失败记录。最终独立探针 30/30 为 200。
- 生产回滚目录为 `/opt/new-api/backups/ccswitch-prompt-20260716T042219Z-ee953c59`；旧镜像标签 `yunbay-new-api:rollback-ccswitch-prompt-20260716T042219Z`，新镜像标签 `yunbay-new-api:release-ee953c59`。部署标记和 11 文件 manifest 已原子更新，绿实例与临时 Caddyfile 已清理。

## 12. 第四轮：第五步向下推进与生图设置预览

### 12.1 目标与性能指标

把第五页抽象为单向状态链 `Provider 待导入 -> Provider 待确认 -> Provider 已确认 -> 生图设置待导入`。用户完成的动作留在上方，下一动作只在其下方出现，消除第二阶段回到旧按钮位置造成的时间倒置感。

完成标准：

- 上方 CC Switch Provider 面板只承担 Provider 首次导入和重新导入，不再按 `importAttempted` 切换为 Prompt 动作。
- Provider Deep Link 发出后，紧随其后的确认框继续提供“已打开”和 Provider 重试。
- 用户确认 Provider 后，确认框通过一次非线性布局动画收敛为与上方三个准备步骤同宽、同密度的完成行；不改变父级宽度、不产生横向跳动。
- 收敛完成行下方展开一个与 Provider 导入面板同级的大面板，明确展示即将写入 Codex 的生图提示词，并提供“一键导入生图设置”。
- 提示词预览展示完整规则，但 API Key 只显示掩码；实际 `resource=prompt` Deep Link 仍使用与 Provider 相同的内存中真实 Key。
- 中文按钮严格显示“一键导入生图设置”；新增文案覆盖 en、zh、fr、ja、ru、vi。
- 不修改 CC Switch 客户端、后端、数据库、Prompt 协议构造器或 Key 持久化边界。

### 12.2 现状测量与根因

- 当前 `CCSwitchImportPanel` 接收 `providerImportAttempted`，首次 Provider Deep Link 发出后把同一个上方面板按钮切换为 Prompt handler，导致视觉焦点逆流到已完成动作之前。
- 当前下方状态框无论确认前后都保留大块说明与 Provider 重试按钮；确认成功后没有收敛，也没有承载明确的下一阶段。
- Prompt 构造器已有唯一规则来源和同 Key 测试，可直接复用；新增预览只做展示层的掩码替换，不能另写一份容易漂移的提示词常量。

### 12.3 GitHub 经验参考

- `farion1231/cc-switch` 当前主分支 `f6e37ed9`：Deep Link parser 仍按单一 `resource` 分派 `provider` 与 `prompt`，因此保留两次明确用户动作。
- `motiondivision/motion`：沿用项目已有 `layout` 与 `AnimatePresence`，用布局插值表达确认框从待确认态收敛到完成态，以及 Prompt 面板在后方进入；不引入新动画库。
- `shadcn-ui/ui` 当前主分支 `6a5e6da7`：继续复用现有 Button 组合、Lucide 图标与项目语义样式，不新增或重装 UI 组件。

### 12.4 最小充分实现

- `CCSwitchImportPanel` 恢复 Provider-only props 与固定按钮：未尝试时“一键导入”，已尝试时“再次导入”，两者都调用 Provider handler。
- 下方 Provider 状态容器启用 Motion layout；确认前保留现有说明、确认和重试，确认后渲染紧凑完成行并移除冗余 Provider 重试。
- 新增快速启动专用 Prompt 预览组件：从现有 `buildQuickStartImagePrompt` 生成展示文本，再将真实 Key 替换为 `maskQuickStartApiKey` 结果；面板正文允许换行和滚动，但在桌面与移动端均不撑破页面。
- Prompt 面板只在 `importConfirmed` 后出现，操作按钮使用 `Sparkles` 图标和“一键导入生图设置”，继续调用现有 `handleImportPromptToCCSwitch`。
- 源码测试明确约束 Prompt 按钮位于下方确认分支之后，且 Provider 面板内不存在 Prompt handler；协议单测继续证明 Provider 与 Prompt 使用同一测试 Key。

### 12.5 验证闭环

- 自动化：快速启动全量测试、`bun run i18n:sync`、`bun run typecheck`、涉及文件 ESLint/Prettier、`bun run build`、`git diff --check`。
- 动画采样：Provider 确认前后记录状态框高度与中间帧，确认最终与三个准备步骤同宽，且高度收敛、无残余 transform。
- 桌面 1280x720：顺序必须是 Provider 面板、确认/完成行、Prompt 大面板；下一动作不回到上方。
- 移动端 390x844：提示词长文可读、可滚动，按钮不溢出、不被底部控制器遮挡，页面无横向滚动。
- 捕获测试 Deep Link：Provider 和 Prompt 使用同一假 Key，Prompt 预览只出现掩码，浏览器控制台不输出真实 Key。

### 12.6 发布控制

- 本地闭环通过后只提交本轮相关文件，推送 GitHub `main`，不纳入用户现有未跟踪设计稿或 `outputs/`。
- 2026-07-16 的 `42f8c7dd` 生产尝试已在临时 upstream 切换阶段中止，并引发公开入口 502；绿实例、临时 Caddyfile 和 Caddy 动态切流从此废止，不能再作为发布或回滚路径。
- 生产 Caddy 文件与运行时 upstream 必须始终保持 `new-api:3000`。发布和回滚只允许在部署锁与独立 60 秒 watchdog 保护下，用 Compose `--no-deps --force-recreate --no-build` 有界重建标准 `new-api`；45 秒内未 healthy 或出现 exited/dead/unhealthy 时自动恢复固定旧镜像并再次只重建标准服务。
- 发布结果、提交、镜像、回滚点、探针与切流结果追加到 `docs/yunbay-maintenance.md` 和桌面现有运维手册。

### 12.7 实施节点

| 节点 | 状态 | 验收条件 |
| --- | --- | --- |
| 现状、协议与 GitHub 经验核验 | 已完成 | 已确认逆序根因、原版 CC Switch 单 resource 限制与可复用 Motion 路径 |
| 完整计划与状态链 | 已完成 | 本节已追加到既有单一计划文件 |
| Provider 完成收敛与 Prompt 大面板 | 已完成 | 下一动作只向下出现，预览脱敏且实际导入保持真实同 Key |
| 六语言与工程验证 | 已完成 | 六语言无缺失，测试、类型、lint、format 与构建通过 |
| 桌面与移动端浏览器验收 | 已完成 | 顺序、动画、长文预览、溢出和 Deep Link 捕获均通过 |
| GitHub main、生产部署与运维记录 | 已中止 | `42f8c7dd` 已在 main；生产尝试在旧动态切流阶段中止，当前不得沿用该发布路径 |

### 12.8 实施验证记录

- `CCSwitchImportPanel` 已恢复为 Provider-only：首次显示“一键导入”，尝试后显示“再次导入”，两个状态都只调用 Provider handler；Prompt handler 只存在于下方 `ImageSettingsImportPanel`。
- Provider 确认前状态框为 `600x130`；确认点击后 Motion layout 中间帧高度约 `86.42px` 且存在布局 transform，约 320ms 收敛为 `600x86` 并清除 transform。移动端完成行为 `358x86`，与上方准备步骤完全同宽同高。
- Prompt 大面板在 DOM 与视觉位置上都排在 Provider 完成行之后；桌面为 `600x348`，移动端为 `358x418`。移动端正文区域仅纵向滚动，`scrollWidth == clientWidth == 316`，页面 `scrollWidth == viewport == 390`。
- 中文操作文案为“一键导入生图设置”；六语言 `i18n:sync` 报告的 missing、extras、untranslated 均为 0。
- Prompt 预览复用实际 Prompt 构造器并只传入掩码 Key；桌面与移动端可见正文均不包含完整测试 Key。捕获的 Provider URL 与 Prompt Base64 解码正文都包含同一个 `sk-test-browser-import-key`，Prompt 仍为 `resource=prompt`、`enabled=true` 并保留图片端点、模型限制和 `outputs/` 规则。
- 1280x720 与 390x844 浏览器控制台 error/warning 均为 0；两张截图已保存到本次 Codex 可视化目录，没有写入项目 `outputs/`。
- `bun test src/features/quick-start/*.test.ts`：54 pass / 0 fail；`bun run typecheck`、涉及文件 ESLint、Prettier、`bun run build`、`git diff --check` 均通过。生产构建入口为 `dist/static/js/index.48c85fa5d3.js`。
- `42f8c7dd` 的生产尝试没有完成：临时绿实例缺少预期网络别名，Caddy 动态 upstream 无法解析并对外返回 502。事故恢复后 Caddy 已回到固定 `new-api:3000`，失败候选已清理；本节的代码与浏览器结果仍有效，但不能把该次尝试记录为已发布。

## 13. 第五轮：确认反馈优先、圆形完成标记与固定 upstream 发布

### 13.1 目标与性能指标

把第五页的完成反馈闭环固定为 `Provider 导入 -> 用户确认 -> 圆形完成反馈 -> 生图设置动作`。状态变化先被用户看见，再出现下一动作，避免自动滚动直接跳过完成反馈。

- `importConfirmed=true` 后，Provider 状态行左侧必须显示与上方三个准备步骤完全相同的 `36x36` 圆形勾选标记；同一组件、同一 class、同一完成动画，不允许做近似副本。
- Provider 完成行在 DOM、视觉坐标和键盘阅读顺序上都必须位于 `ImageSettingsImportPanel` 上方。
- 确认动作后的自动滚动目标必须是 Provider 完成行，而不是后续生图设置面板；生图设置作为下一动作在其下方展开。
- 桌面 `1280x720` 与移动端 `390x844` 无横向溢出、底栏遮挡或状态切换引起的宽度变化；减少动态效果时仍保持同一顺序，只取消空间动画。
- 不修改 Provider/Prompt Deep Link、真实 Key 边界、默认模型、思考强度、后端、数据库或六语言文案。

### 13.2 现状、运行时与最小充分模型

- 用户截图中的生产页面仍是旧链路：上方面板按钮切换为“继续导入推荐提示词”，确认结果卡落在其下方且没有完成标记。
- GitHub `main@42f8c7dd` 已把 Prompt 独立为下方面板，并在确认行复用 `QuickStartStepMarker`；但自动滚动仍直接指向 Prompt 面板，且标记 class 仍使用 `rounded-xl`，没有把“圆形且与上方严格同源”写成稳定约束。
- 最小改动只涉及 `index.tsx` 与源码约束测试：上方准备步骤和 Provider 完成行统一为 `rounded-full`；确认后滚动到 `importStatusRef`；移除不再需要的 Prompt ref；强化顺序、同 class 和滚动目标断言。
- 不新增组件、依赖、状态、翻译 key 或持久化字段。

### 13.3 GitHub 经验参考

- `motiondivision/motion`：继续使用已有 `AnimatePresence`、layout 与 reduced-motion 分支，让完成行先收敛、下一面板后进入，不叠加第二套状态机。
- `shadcn-ui/ui`：复用现有 Button 与 Lucide `Check` 组合，通过同一个语义组件表达完成状态，不手写第二套 SVG。
- `farion1231/cc-switch`：Provider 与 Prompt 仍是两个独立 `resource`，因此 UI 继续保留两次明确用户动作，只调整反馈顺序。
- `kortix-ai/suna` 与 `super-productivity/super-productivity` 的部署脚本经验：使用单飞锁、有界健康反馈和失败自动回滚；不再动态改写 Caddy upstream。

### 13.4 验证闭环

- 源码测试先约束：完成标记与 `ReadinessRow` 使用同一圆形 class；Provider 状态位置小于 Prompt 面板位置；`importConfirmed` 滚动目标为 `importStatusRef`，源码中不存在 `promptPanelRef`。
- 自动化：快速启动全量测试、`bun run typecheck`、涉及文件 ESLint/Prettier、`bun run i18n:sync`、`bun run build`、`git diff --check`。
- Browser 桌面与移动端走通 `Provider 尝试 -> 已打开 -> 完成行 + Prompt 面板`；测量两个完成标记均为 `36x36`、`border-radius: 9999px`，并确认完成行 `top < Prompt panel top`。
- 检查页面身份、非空、无框架错误层、控制台 error/warn、截图、交互状态与 reduced-motion；不实际修改本机 `~/.codex/AGENTS.md`。

### 13.5 发布闭环

- 2026-07-16 用户已明确说“部署”，本轮生产发布获准开始；仍必须在构建期间保持旧服务，正式切换只允许有 watchdog 保护的标准 `new-api` 单服务重建。
- 先只读确认生产标准实例 healthy、Caddy 文件与运行时 upstream 都是 `new-api:3000`，并固定当前运行镜像为不可变回滚标签。
- 获取 `/var/lock/yunbay-new-api-deploy.lock` 后同步精确文件并构建；构建期间旧标准容器继续服务。发布前启动独立于 SSH 的 60 秒 watchdog。
- 正式切换只重建标准 `new-api`，不创建绿实例、不生成临时 Caddyfile、不调用 Caddy Admin API或 `caddy reload`。45 秒内未恢复时 watchdog 自动标回旧镜像并重建。
- 成功标准：new-api/Caddy healthy，Caddy upstream 仍为 `new-api:3000`，首页与 `/api/status` 连续 5 次 200，新 bundle 和目标 DOM 可识别；随后更新生产标记与既有运维手册并清理临时文件。

### 13.6 实施节点

| 节点 | 状态 | 验收条件 |
| --- | --- | --- |
| 截图、源码、事故与 GitHub 经验核验 | 已完成 | 已确认生产旧 UI、main 已有部分修复及动态切流废止边界 |
| 完整计划与控制指标 | 已完成 | 本节已追加到唯一实施计划文件 |
| 圆形完成标记、反馈顺序与测试 | 已完成 | 同源圆形标记、状态优先滚动和顺序断言通过 |
| 自动化与双视口浏览器验收 | 已完成 | 工程检查、交互、测量、截图和 reduced-motion 通过 |
| GitHub main | 已完成 | 本轮三个文件已随 `f7839a9d` 普通推送 main |
| 固定 upstream 生产发布 | 已完成 | `f7839a9d` 已按固定 upstream 流程上线，最终实例 healthy，回滚点与运维记录已保留 |

### 13.7 实施验证记录

- Provider 确认后的 `QuickStartStepMarker` 与上方准备步骤统一使用 `size-9 rounded-full text-[11px]`；源码测试锁定两处同源 class。完成行在源码、DOM 查询顺序和视觉坐标上都位于 `ImageSettingsImportPanel` 之前。
- 确认后的自动滚动目标已改为 `importStatusRef`，同时移除 `promptPanelRef`；用户先看到圆形完成反馈，再向下看到生图设置动作。
- 独立 Prompt 面板继续由 `buildQuickStartImagePromptPreview()` 生成，预览包含 `/v1/images/generations`、`gpt-image-2` 与 `outputs/` 规则，只显示掩码 Key；旧“继续导入推荐提示词”按钮数量为 0，“一键导入生图设置”按钮数量为 1。
- 桌面 `1280x720` 和移动端 `390x844` 均已通过：完成标记为 `36x36` 完整圆形，完成行在 Prompt 面板上方，无横向溢出。减少动态效果下同一交互顺序通过，页面 `scrollWidth=viewportWidth=1280`；开发模式仅有 Motion 检测到 reduced-motion 后输出的预期说明性 warning，控制台 error 为 0。
- `bun test src/features/quick-start/*.test.ts`：54 pass / 0 fail；`bun run typecheck`、定向 ESLint、六语言 i18n、Prettier、`bun run build` 与 `git diff --check` 均通过。最新本地生产入口为 `dist/static/js/index.5ef2da020e.js`。
- 功能与计划提交 `f7839a9d5236130590df6c8044978a0f1c729382` 已普通 fast-forward 推送 GitHub `main`；提交范围只有本轮计划、快速启动实现与源码测试。
- 本轮最初按用户要求暂停生产；收到明确“部署”指令后才进入第 13.8 节的固定 upstream 发布流程。

### 13.8 生产执行计划

- 目标功能基线固定为 `f7839a9d5236130590df6c8044978a0f1c729382`，不把后续纯文档提交当作镜像内容标记；从生产当前 `ee953c59` 到目标共精确同步 12 个 `web/default` 源码、测试和六语言文件，不同步仓库脏工作区或项目文档。
- 先获取 `/var/lock/yunbay-new-api-deploy.lock` 并完成只读前置检查：标准 new-api/Caddy healthy、文件和运行时 upstream 均为 `new-api:3000`、首页与状态接口为 200、当前运行镜像可解析。
- 在锁内备份 12 个目标文件及当前部署标记，以运行镜像 ID创建不可变 rollback tag；非删除式同步经过提交归档的精确文件，并校验本地/生产组合 SHA-256。
- 服务器构建 `yunbay-new-api:prod` 时旧标准容器继续服务；构建成功后建立 release tag，静态核对镜像内新 bundle 与目标源码标记。
- 切换前启动独立于 SSH 的 60 秒 watchdog；只执行 `docker compose ... up -d --no-deps --force-recreate --no-build new-api`。45 秒内未 healthy 或 HTTP 未恢复时，watchdog 自动把 rollback tag 重标为 `prod` 并只重建标准服务。
- 发布成功须满足：new-api/Caddy healthy、Caddy upstream 仍为 `new-api:3000`、首页和 `/api/status` 连续 5 次 200、生产 bundle 含新提示词面板与圆形完成标记行为所对应的代码；随后原子更新部署标记，追加仓库与桌面唯一运维手册并清理临时发布文件。

### 13.9 生产执行与验证记录

- 生产从 `ee953c5954f85e61a4004f0df26f9043e696cc10` 精确、非删除式同步 12 个 `web/default` 文件到功能提交 `f7839a9d5236130590df6c8044978a0f1c729382`；文件清单 SHA-256 为 `3019659dddd3fb3a266ed24c5190983dfcb80393d867714c2809373eea2fcf05`。构建期间旧容器始终承担现网流量。
- 最终新镜像为 `sha256:b7af515bac4bfc28de155fa08dfd4f15a2e7f6d2d36d74837c9fcd2b9955472c`，release tag 为 `yunbay-new-api:release-f7839a9d`；标准容器 `2791f8a56983f9556a362a06e99a7bfa683422a1e36ecddbad0436ec77575f3e` 为 `running / healthy / restart=0`。
- 成功回滚目录为 `/opt/new-api/backups/quick-start-confirm-order-20260716T131718Z-f7839a9d`；旧运行镜像 `sha256:3db9cb74a065b3aec8dbfae5919c038739c96bdf728b43794491b2fb86dbb91a` 已固定为 `yunbay-new-api:rollback-quick-start-confirm-order-20260716T131718Z`。本轮没有数据库迁移。
- 最终标准服务重建 19 秒完成；watchdog 状态为 `success`，没有触发最终回滚。Caddy 文件、挂载和运行时 upstream 始终为 `new-api:3000`；Caddy 容器 ID、启动时间和 restart=0 不变，PostgreSQL、Redis、Sub2API、CLI Proxy 与 LDXP 服务快照前后完全一致。
- 生产入口为 `/static/js/index.5ef2da020e.js`，SHA-256 `fa94acad0ca79a476a0bb475205424124721e39995536865c03f7b866334b94d`，字节数 `3064387`，与本地已测试构建完全一致；四条新界面文案存在，旧“Continue importing the recommended prompt”文案不存在。
- 独立验收中，源站 `/api/status`、公网 `/`、`/quick-start` 与 `/api/status` 连续 10 轮均为 200；新实例严重启动日志为 0，部署锁已释放，绿实例数量为 0，部署归档和重复解压目录已清理。
- 首次正式 SSH 因本机 Clash 临时落到未放行出口而在 banner 前失败，远端脚本没有执行。随后一次预备备份因可选容器没有 Docker health 字段提前退出，也没有同步、构建或切换。
- 第一次切换时 watchdog 把旧容器在 Compose 替换过程中的 `exited/unhealthy` 瞬态误判为新镜像失败，自动恢复旧镜像、旧源码和旧标记；修正为只有当前容器已使用新镜像时才即时判定 unhealthy。第二次切换中新实例已 healthy/200，但静态验证要求了被生产压缩器移除的源码字符串，因此主动请求自动回滚；最终改用入口、完整 bundle 哈希、字节数、稳定文案和旧文案缺失作为证据。
- Caddy 在第二次切换、该次回滚和最终切换窗口共记录 30 个 HTTP 502，其中 27 个为标准容器重建期间的短暂连接失败；三个窗口分别约 10 秒、6 秒和 10 秒，主要影响 `/v1/responses`。没有 `new-api-green` DNS/lookup 错误，Caddy 没有重启；最终窗口之后全部探针恢复 200。该事实保留为后续进一步缩短固定 upstream 重建时间的基线。

## 14. 第六轮：Provider 完成条与悬停展开详情

### 14.1 目标与性能指标

把 Provider 已确认后的两个重复视觉对象综合为一个稳定的组合块：默认只显示圆形完成标记、完成说明和灰色“再次导入”按钮；用户把鼠标移入按钮或用键盘聚焦时，按钮恢复为亮白主样式，API、模型、脱敏 Key 与思考强度从完成条内部向下弹性展开；离开组合块或焦点移到块外后自动收起。

- `importConfirmed=false` 时继续显示现有 CC Switch 大面板和待确认反馈，不改变首次导入闭环。
- `importConfirmed=true` 时原 CC Switch 大面板必须消失，只保留一个 Provider 完成组合块；生图设置面板继续位于其下方。
- 完成组合块默认高度收敛，左侧继续使用与准备步骤同源的 `36x36` 圆形勾选标记；“再次导入”默认为低对比度灰色，不与下方生图主操作竞争。
- 鼠标进入再次导入按钮或键盘聚焦按钮后，按钮在约 200-320ms 内非线性过渡为亮白样式；详情以 `height: 0 -> auto`、透明度和轻微纵向位移组合展开，弹簧在约 550ms 内稳定。
- 鼠标离开整个组合块或焦点移到块外后，详情沿相反路径弹性收起；在详情区域内查看内容不能意外触发收起。
- 点击“再次导入”仍打开同一个 Provider Deep Link，但 `importConfirmed` 必须保持为 `true`，不能闪回待确认面板或隐藏生图设置。
- `prefers-reduced-motion: reduce` 下取消弹簧、位移和缩放，仅使用不超过 80ms 的透明度/高度反馈；移动端无悬停时保持紧凑完成态，键盘/辅助输入仍可展开。
- 桌面 `1280x720` 与移动端 `390x844` 均不得出现横向溢出、按钮错位、详情裁切、底栏遮挡或展开导致的宽度变化。

### 14.2 最小充分控制模型

- 被控对象：Provider 完成组合块的详情高度、透明度、按钮背景与文字颜色。
- 控制输入：组合块的 `pointerenter` / `pointerleave`，以及再次导入按钮的 `focus` / 组合块的 `blur`。
- 测量状态：一个仅存在于 React 内存的布尔值 `providerDetailsExpanded`；不写入快速启动 session，也不影响业务完成状态。
- 稳定状态：`importConfirmed=true` 是业务稳态；详情展开只是瞬时显示状态。再次导入只重放执行器，不修改业务稳态。
- 扰动与降级：触屏设备可能没有 hover，键盘焦点可能在组合块内部移动，系统可能要求减少动态效果；分别以紧凑默认态、`relatedTarget` 包含关系检查和 reduced-motion 分支处理。

### 14.3 GitHub 经验参考

- `motiondivision/motion`：沿用项目现有 `AnimatePresence` 与 spring transition，以 `height: 0 -> auto` 配合 overflow clipping 完成内容展开/收起；不新增动画库。
- `shadcn-ui/ui`：继续复用项目 Button、`aria-expanded`、`aria-controls` 与 `focus-visible` 语义，保持鼠标和键盘等价反馈。
- `farion1231/cc-switch`：再次导入仍使用既有 `resource=provider` Deep Link；本轮不改协议参数、Provider 数据或 Prompt 资源顺序。

### 14.4 最小实施范围

- `index.tsx`：抽取 Provider 四项指标为小型复用组件；把首次导入大面板限制为未确认态；在确认分支加入灰色再次导入按钮、可访问性属性和 Motion 详情区域。
- 拆分 Provider Deep Link 执行器与状态转换：首次/待确认重试继续写入 `importConfirmed=false`，已确认再次导入只保持 `importConfirmed=true` 并更新同一 session。
- `quick-start-page-source.test.ts`：约束确认后只有一个组合块、详情顺序、hover/focus/reduced-motion 控制、`aria-expanded`，以及已确认再次导入不调用 `setImportConfirmed(false)`。
- 不新增翻译 key、依赖、持久化字段，不改后端、数据库、默认模型、Prompt 构造器、生产配置或运维服务。

### 14.5 验证闭环

- 自动化：快速启动全量测试、`bun run typecheck`、涉及文件 ESLint/Prettier、`bun run i18n:sync`、`bun run build`、`git diff --check`。
- Browser 桌面：走到 Provider 已确认，确认大面板已合并；采样按钮默认灰色、hover 后亮白、详情展开中间帧与最终高度，移出后回到紧凑高度；点击再次导入后完成标记和生图面板保持可见。
- Browser 键盘与 reduced-motion：Tab 聚焦可展开、焦点移出可收起；减少动态效果下无弹簧 transform，顺序和功能不变。
- Browser 移动端：`390x844` 下默认紧凑、焦点展开可读，组合块和生图面板无横向溢出或底栏遮挡；不实际修改本机 CC Switch 或 `~/.codex/AGENTS.md`。

### 14.6 发布控制与实施节点

- 初始阶段按用户当时指示只完成本地闭环并普通推送 GitHub `main`；收到后续明确“部署”指令后，才进入第 14.8 节生产流程。
- 代码提交只包含本轮计划、快速启动实现和对应测试；生产只精确同步两个 `web/default` 文件，继续保留并排除工作区现有其它改动、旧规格文档和 `outputs/`。

| 节点 | 状态 | 验收条件 |
| --- | --- | --- |
| 现状、交互边界与 GitHub 经验核验 | 已完成 | 已确认重复面板根因、稳定态与可复用 Motion/Button 路径 |
| 完整计划与控制指标 | 已完成 | 本节已追加到既有唯一计划文件 |
| 完成组合块、悬停详情与再次导入状态 | 已完成 | 默认紧凑、hover/focus 展开、移出收起且业务确认态不回退 |
| 自动化与双视口浏览器验收 | 已完成 | 工程检查、动效采样、键盘、reduced-motion 与双视口布局均通过 |
| GitHub main | 已完成 | `7bc5e4e7` 已普通推送 main，只包含本轮三个相关文件 |
| 生产部署 | 已完成 | `7bc5e4e7` 已按固定 upstream 流程上线；最终实例 healthy，回滚点与三处运维记录已保留 |

### 14.7 实施验证记录

- Provider 首次/待确认导入继续使用 `openCCSwitchProviderImport(false)`；完成条“再次导入”独立使用 `openCCSwitchProviderImport(true)`。浏览器拦截测试捕获到一个 `resource=provider`、`model=gpt-5.6-sol` 的 Deep Link，点击后 DOM 仍为 `confirmed`、生图设置面板仍可见，session 中 `importAttempted` 与 `importConfirmed` 均保持 `true`。
- `importConfirmed=true` 后原 CC Switch 大面板不再渲染；默认完成组合条为 `600x86`，按钮为 `112x44`、低对比度灰色，详情 DOM 不存在。悬停展开约 90ms 时详情高度 `64.52px`、透明度 `0.747`、纵向位移约 `-2.50px`；最终组合条为 `600x172`，按钮变为纯白，详情高度 `86px` 且 transform 清除。
- 鼠标移出组合块约 90ms 时详情收至 `24.69px`、透明度 `0.290`、纵向位移约 `-5.70px`，最终回到 `600x86` 且详情卸载；展开和收起全过程 `documentWidth=1280`。
- 键盘聚焦“再次导入”可展开，焦点移到下方生图按钮后按同一弹簧路径收起。`prefers-reduced-motion: reduce` 下展开/收起全程 `transform: none`，只保留约 80ms 的高度与透明度反馈；浏览器仅记录 Motion 对模拟 reduced-motion 的预期说明性 warning，应用 error 为 0。
- 移动端 `390x844` 默认组合条为 `358x146`，灰色按钮为 `324x44`；展开后组合条为 `358x415`，四项详情为 `324x269`。动画完成后活动滚动区自动最小滚动 `264px`，详情底部为 `687px`、底栏顶部为 `773px`，完整留出 `86px` 间距；页面和活动滚动区宽度均为 `390px`。桌面同一引导只产生 `2px` 的最小位置校正。
- `bun test src/features/quick-start/*.test.ts`：54 pass / 0 fail；`bun run typecheck`、涉及文件 ESLint、Prettier、`bun run i18n:sync`、`git diff --check` 与 `bun run build` 均通过。六语言 missing、extras、untranslated 均为 0，最终生产构建入口为 `dist/static/js/index.2f46b5d28b.js`。
- 功能、测试与本节计划提交 `7bc5e4e7` 已普通 fast-forward 推送 GitHub `main`；提交范围只有本轮三个目标文件，工作区既有运维手册改动、旧规格文档和 `outputs/` 均未纳入。
- 用户授权前没有触碰生产；授权后仅按第 14.8 节精确同步两文件、构建镜像并有界重建标准 `new-api`，没有修改或重启 Caddy、数据库、Redis、Sub2API、CLI Proxy 或 LDXP 服务。

### 14.8 生产执行计划

- 用户已明确发送“部署”，本轮生产发布自此获准。目标功能基线固定为 `7bc5e4e7`；`f49f680a` 只收尾计划，不作为镜像功能标记。
- 生产当前功能基线预期为 `f7839a9d`。精确同步范围仅为 `web/default/src/features/quick-start/index.tsx` 与 `web/default/src/features/quick-start/quick-start-page-source.test.ts`；不使用删除式同步，不上传本地脏工作区、计划文档、运维手册或 `outputs/`。
- 先获取非阻塞部署锁 `/var/lock/yunbay-new-api-deploy.lock` 并只读确认：标准 `new-api` 与 Caddy healthy、当前镜像可解析、正式 Caddyfile/挂载/运行时 upstream 都是 `new-api:3000`、公网首页与 `/api/status` 为 200、没有 `new-api-green`。
- 锁内建立带 UTC 时间和 `7bc5e4e7` 标记的源码备份，记录部署前存在文件、当前 `.yunbay-deploy-sha` 与 `.yunbay-source-manifest`；把当前运行镜像固定为不可变 rollback tag。同步后逐文件核对本地与生产 SHA-256。
- 在服务器构建 `yunbay-new-api:prod`，构建期间旧标准容器继续承担流量；构建成功后固定 `yunbay-new-api:release-7bc5e4e7`，并在切换前核对镜像内入口、bundle 哈希/字节数和目标稳定标记。
- 正式切换前启动独立于 SSH 的 60 秒 watchdog。只允许执行 `docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml up -d --no-deps --force-recreate --no-build new-api`；不创建绿实例、不修改或 reload Caddy、不重启 PostgreSQL、Redis、Sub2API、CLI Proxy、LDXP proxy 或 worker。
- 新标准容器 45 秒内未达到目标镜像、Docker healthy 和容器内 HTTP 200，或最终公网验证失败时，watchdog/发布脚本必须把固定旧镜像重新标记为 `:prod`、恢复两文件源码与部署标记，并再次只重建标准 `new-api`；不能留下等待人工输入的中间态。
- 发布成功须同时满足：`new-api`/Caddy healthy，Caddy upstream 仍为 `new-api:3000`，其它服务容器 ID、启动时间和 restart count 未变化，公网 `/`、`/quick-start`、`/api/status` 连续 5 次为 200，新 bundle 与本地构建一致且包含本轮稳定标记。随后原子更新部署标记、追加仓库与桌面唯一运维手册并清理临时传输、构建和 watchdog 文件；源码备份与 rollback/release 镜像标签保留。

### 14.9 生产执行记录

- 首次执行在固定 upstream 流程内完成两文件备份、精确同步与镜像构建；构建期间旧容器 ID、镜像、启动时间和 healthy 状态均未变化。新镜像 `sha256:0d948194bc0bf45a5f106a9256d8bc3dbf8c922136628c39457d6c1bf7f88d0e` 已固定为 `yunbay-new-api:release-7bc5e4e7`。
- 首次标准服务切换后，新容器约 11 秒达到 healthy 并提供与本地完全一致的 `index.2f46b5d28b.js`：SHA-256 `e2d4051761dfc4a567343fca9cf4411edf125aaa2ddc9abaaf00584de8b7b3e3`、字节数 `3064387`。应用、镜像和主入口均通过，失败发生在后续静态验证器。
- 静态验证器错误地从主入口搜索完整 `4963.<hash>.js`；Rsbuild 实际使用 chunk map `4963:"80d86e7db9"`，因此脚本无法组装 quick-start chunk 文件名并主动请求 watchdog 回滚。该失败是验证脚本解析错误，不是应用健康或资源内容错误。
- watchdog 已把旧镜像重新标记为 `:prod`、恢复两文件源码与部署标记，并只重建标准 `new-api`。回滚后生产重新运行旧镜像 `sha256:b7af515bac4bfc28de155fa08dfd4f15a2e7f6d2d36d74837c9fcd2b9955472c` 且 healthy；`/`、`/quick-start`、`/api/status` 均为 200，Caddy runtime 仍只有 `new-api:3000`，无绿实例。
- Caddy、PostgreSQL、Redis、Sub2API、CLI Proxy、LDXP proxy 与 worker 的容器 ID、启动时间和 restart count 与首次切换前逐项一致。首次切换与自动回滚的 1 秒公网探针各记录 9 次 502 与 9 次 200；没有 Caddy DNS/lookup 改写或依赖服务重启。
- 第二次执行将 chunk map 解析修正为 `4963:"<hash>" -> 4963.<hash>.js`，重新获取部署锁、建立新备份并启动独立 watchdog，复用上述不可变 release 镜像且不重复构建。标准 `new-api` 约 12 秒达到 healthy，完整验证于 24 秒内完成，watchdog 结果为 `success`。
- 最终运行容器为 `9d893d41557a2a33e32ee2e09ec45350eae349e600c46bf2915f9b06589ae81b`，镜像为 `sha256:0d948194bc0bf45a5f106a9256d8bc3dbf8c922136628c39457d6c1bf7f88d0e`，restart count 为 0。生产标记已原子更新到完整功能提交 `7bc5e4e7b5ad67969bae0961b0a580670d206382`；两文件清单 SHA-256 为 `67e721d3008b56be4e554cce3e586079c2000c8f696cbdf05c64ed9c0ba78a5c`。
- 最终主入口 `/static/js/index.2f46b5d28b.js` 的 SHA-256 为 `e2d4051761dfc4a567343fca9cf4411edf125aaa2ddc9abaaf00584de8b7b3e3`、字节数 `3064387`；quick-start chunk `/static/js/async/4963.80d86e7db9.js` 的 SHA-256 为 `a095fd82e7bdd86a943d0b3e1ea8e45eccc0927e3f2db5c4b4bdb2308f095fea`、字节数 `46803`，并包含两个再次导入标记和三个详情标记。二者均与本地测试构建完全一致。
- 成功切换的 1 秒公网探针记录 8 次 502、随后 12 次连续 200；Caddy 原始日志的 502 窗口为 `14:51:20.124Z` 至 `14:51:28.615Z`，约 8.49 秒，共 44 个请求：4 个 Docker DNS `lookup new-api` 瞬态、39 个新进程监听前的 connection refused、1 个旧连接关闭。Caddy 容器、文件、挂载和运行时 upstream 均未修改，最终仍只有 `new-api:3000`。
- 最终远端 10 轮均确认宿主机 `/api/status` 与公网 `/`、`/quick-start`、`/api/status` 为 200；本机重新下载的两个生产 bundle 哈希和字节数均与本地一致。Caddy、PostgreSQL、Redis、Sub2API、CLI Proxy、LDXP proxy 与 worker 的容器 ID、启动时间和 restart count 在最终切换前后逐项一致，应用严重启动日志为 0，无绿实例。
- 成功回滚目录为 `/opt/new-api/backups/quick-start-reimport-hover-20260716T145117Z-7bc5e4e7`；固定旧镜像标签为 `yunbay-new-api:rollback-quick-start-reimport-20260716T145117Z`，旧镜像 ID 为 `sha256:b7af515bac4bfc28de155fa08dfd4f15a2e7f6d2d36d74837c9fcd2b9955472c`。首次验证器失败备份 `/opt/new-api/backups/quick-start-reimport-hover-20260716T144138Z-7bc5e4e7` 与对应 rollback 标签一并保留用于审计。

## 15. 第七轮：合并 Provider 导入反馈并恢复生图设置下一步

### 15.1 目标与性能指标

把第五页 Provider 导入段收敛为一个随状态原位变化的控制对象，消除截图中“将当前设置导入 CC Switch”和“CC Switch 打开了吗”上下两个独立容器。用户点击 Provider 一键导入后，应立即看到同一容器收敛成完成条，并在其正下方看到生图设置导入，不再经过额外确认卡。

- 初始态只显示一个完整 Provider 导入容器：真实 API、当前模型、脱敏 Key、思考强度和“一键导入”。
- 点击“一键导入”后，同一容器以 layout/spring 动画原位收敛为“圆形完成勾 + 完成文案 + 灰色再次导入”，不得同时残留第二张确认卡。
- “再次导入”保持上一轮交互：默认灰色；hover/focus 时变亮并在同一容器内向下展开 Provider 详情；移出/失焦后弹性收起；点击只重新打开 Provider Deep Link，不回退完成态、不隐藏生图设置。
- Provider 完成条收敛后，`ImageSettingsImportPanel` 必须紧邻其下方进入，继续展示由真实 Prompt 构造器生成的脱敏预览，并保留“一键导入生图设置”。
- 状态改变后自动滚动到生图面板的最近可见位置，使桌面 `1280x720` 和移动端 `390x844` 都能同时辨认上方 Provider 完成条与下方生图设置入口。
- 不修改 `buildQuickStartCCSwitchImportURL`、`buildQuickStartCCSwitchPromptImportURL`、Prompt 正文、真实 Key 边界、默认 `gpt-5.6-sol`、`xhigh`、后端、数据库或翻译文本。

### 15.2 最小充分状态模型

- `softwareConfirmed && effectiveApiKey`：渲染唯一的 Provider 导入容器。
- `importAttempted=false`：完整 Provider 配置和首次导入动作。
- `importAttempted=true`：Provider 导入动作已经发出，容器进入完成条；生图设置面板立即成为下一控制对象。
- `providerDetailsExpanded=true`：只影响完成条内部详情高度与再次导入按钮视觉，不改变 `importAttempted`，也不改变生图面板可见性。
- `importConfirmed=true`：生图设置 Deep Link 已发出，完整两阶段导入完成，底部“进入控制台”才可用；继续复用现有持久化字段，不新增 session schema 或迁移。
- 旧会话中 `importConfirmed=true` 但 `importAttempted=false` 时，将 Provider 视为已完成以兼容历史数据；旧会话中只有 `importAttempted=true` 时显示 Provider 完成条与生图设置，并继续等待用户导入生图设置。

### 15.3 GitHub 经验参考

- `motiondivision/motion@61833240`：继续复用项目已有 `AnimatePresence`、layout 与 spring transition，让同一个视觉对象在展开态和完成态之间连续变形，减少并行卡片造成的认知跳跃。
- `farion1231/cc-switch@f6e37ed9`：项目文档明确 Provider 与 Prompts 都支持 `ccswitch://` Deep Link 导入；因此保留两个串行的真实动作，但在 Provider 动作发出后立刻把 Prompts 作为下一步显示。
- 继续使用现有 shadcn Button、Lucide 图标和 `QuickStartStepMarker`，不引入新组件库或第二套动画配置。

### 15.4 最小实施

1. 将父组件中独立的 Provider 面板、确认状态面板合并为一个 `CCSwitchImportPanel` 调用和一个外层 ref；移除“CC Switch 打开了吗 / 已打开 / 重试”渲染分支及无用 handler。
2. 让 `CCSwitchImportPanel` 自己根据 `imported` 在完整态与完成态之间做 `AnimatePresence`/layout 变形；完成态复用圆形 `QuickStartStepMarker`，内部保留 hover/focus 详情。
3. 首次 Provider 导入只写入 `importAttempted=true`；再次导入不回退 `importConfirmed`。生图设置按钮发出 Prompt Deep Link 后才写入 `importConfirmed=true` 并开放进入控制台。
4. 恢复生图面板 ref，并在 Provider 状态完成后滚动到该面板的 `nearest` 位置；DOM 顺序固定为 Provider 完成条在前、生图面板在后。
5. 更新源码约束测试，明确只有一个 Provider 容器、没有独立打开确认卡、首次导入直接显示 Prompt、Prompt 导入后才可完成、再次导入不回退状态。

### 15.5 验证闭环

- 先运行快速启动全量测试，确认 Provider/Prompt URL、Prompt 内容、会话兼容和源码约束全部通过。
- 运行 TypeScript、定向 ESLint、Prettier、六语言同步检查、生产构建与 `git diff --check`。
- Browser 在 `1280x720` 与 `390x844` 从干净 session 走通：首次 Provider 导入 -> 单一完成条 -> 生图预览可见 -> 再次导入 hover/focus 展开与收起 -> 生图 Deep Link。
- 验收容器数量、DOM/视觉先后、按钮对齐、完成圆圈尺寸、横向溢出、底栏遮挡、控制台 error/warn、键盘焦点与 reduced-motion。
- 本轮先只完成本地代码、测试和 GitHub `main`；没有新的明确“部署”指令前，不同步生产文件、不构建服务器镜像、不停止、重建或重启任何生产服务。

### 15.6 实施节点

| 节点 | 状态 | 验收条件 |
| --- | --- | --- |
| 截图、运行时与历史实现核验 | 已完成 | 已定位双容器来源和生图面板被额外确认态阻断的问题 |
| GitHub 经验与完整闭环计划 | 已完成 | 本节已写入唯一计划文件，固定状态、反馈、验证和发布边界 |
| 单容器状态机与 Prompt 下一步 | 已完成 | Provider 原位收敛，Prompt 紧邻出现，无独立确认卡 |
| 自动化与双视口浏览器验收 | 已完成 | 工程检查、交互、布局、键盘和 reduced-motion 全通过 |
| GitHub main | 已完成 | `44a3e932` 已普通推送 main，仅包含计划、实现和源码约束测试 |
| 生产部署 | 已完成 | `44a3e932` 已按固定 upstream 闭环发布，watchdog 成功，30/30 公网探针为 200 |

### 15.7 实施验证记录

- 父层只保留一个 `CCSwitchImportPanel`。初始态显示 Provider 配置与“一键导入”；动作发出后，同一组件通过 `AnimatePresence`/layout 原位收敛为圆形完成勾、完成文案和灰色“再次导入”。源码和浏览器中均不存在“CC Switch 打开了吗 / 已打开 / 重试”独立确认卡。
- `importAttempted` 现在控制 Provider 完成条和 `ImageSettingsImportPanel` 可见性；`importConfirmed` 只在 Prompt Deep Link 发出后写入并解锁“进入控制台”。历史 `importConfirmed=true` 会归一为 Provider 已完成，旧 session 无需迁移。
- 浏览器隔离捕获到两个 `resource=provider` 和一个 `resource=prompt` 的 `ccswitch://` 动作；全程用测试 Key 拦截 `window.open`，没有启动或修改本机 CC Switch，也没有写入真实 `~/.codex/AGENTS.md`。Prompt 仍由实际构造器生成，预览包含图片端点、`gpt-image-2`、禁止主聊天模型和 `outputs/` 规则，仅显示脱敏 Key。
- 桌面 `1280x720` 完成态只有一个 `600x86` Provider 条和一个 `600x348` 生图面板，二者分别位于 `top=130` 与 `top=228`，生图面板底部为 `576px`；页面宽度与视口同为 `1280px`。Prompt 动作前“进入控制台”禁用，动作后启用；生图按钮转灰但继续明确显示“一键导入生图设置”，不与 Provider 的“再次导入”混淆。
- Provider 详情展开后为四项真实 Provider 数据；按钮由低对比灰色变为亮白。移出/失焦后详情卸载，并在退出动画完成时把生图面板恢复到最近可见位置。一次完整展开/收起采样前后位置完全一致：Provider `top=146 / bottom=232 / height=86`，Prompt `top=244 / bottom=592 / height=348`。
- 移动端 `390x844` 完成态 Provider 为 `358x146`，圆形完成标记 `36x36`，再次导入按钮 `324x44`；生图面板为 `358x418`，底部 `704px`，活动滚动区底部 `720px`，底栏从 `773px` 开始。页面、body 与活动滚动区宽度均为 `390px`，无横向溢出或底栏遮挡。
- hover 与 focus 事件均验证可展开，pointer leave 与焦点移出均验证可收起；`prefers-reduced-motion: reduce` 下仍保持 1 个 Provider、1 个 Prompt 和完整展开/收起状态，宽度 `390px`。浏览器 error 为 0；唯一 warning 是 Motion 对主动模拟 reduced-motion 的说明性提示。
- `bun test src/features/quick-start/*.test.ts` 为 54 pass / 0 fail；`bun run typecheck`、定向 ESLint、Prettier、`bun run i18n:sync`、`git diff --check` 与 `bun run build` 全部通过。六语言 missing、extras、untranslated 均为 0。
- 最终本地生产入口为 `dist/static/js/index.e4374b86a3.js`，SHA-256 `cf673d3ea925820fbc18db9e0c6b55e24bdc2d8aa2f896c4ac5d0dc60573b3b3`、字节数 `3064387`；quick-start chunk 为 `dist/static/js/async/4963.937f71286d.js`，SHA-256 `1addc8ec05f0628f7f708f00dc4a93d5b9089ea74a60e603bcc98c710b83b4d9`、字节数 `47329`。
- 功能、测试与本节计划已作为 `44a3e932` 普通 fast-forward 推送 GitHub `main`；提交范围只有三个本轮目标文件，工作区既有运维手册改动、旧规格文档和 `outputs/` 均未纳入。
- 本轮没有连接、同步、构建或重启生产服务器；现网继续运行上一轮健康版本。开发验收复用现有 `http://127.0.0.1:5173/quick-start`，没有停止本地后端或前端进程。

### 15.8 生产执行计划

- 用户已明确发送“部署”，本轮生产发布自此获准。功能基线固定为 `44a3e932857d5f1b0a7abc955a1fa08878fe0161`；后续纯计划提交不参与镜像功能身份。
- 预期生产基线为 `7bc5e4e7b5ad67969bae0961b0a580670d206382`。精确同步范围仍只有 `web/default/src/features/quick-start/index.tsx` 与 `web/default/src/features/quick-start/quick-start-page-source.test.ts`；不做删除式同步，不上传本地运维手册、旧规格文档、`outputs/` 或其它脏工作区内容。
- 先获取非阻塞部署锁 `/var/lock/yunbay-new-api-deploy.lock` 并只读确认：标准 `new-api` 与 Caddy healthy，当前容器镜像可解析，正式 Caddyfile、挂载和运行时 upstream 都只有 `new-api:3000`，公网 `/`、`/quick-start`、`/api/status` 为 200，且不存在 `new-api-green`。
- 锁内建立带 UTC 时间与 `44a3e932` 标记的成功回滚目录，备份两个源文件、存在文件清单、`.yunbay-deploy-sha` 与 `.yunbay-source-manifest`；把切换前正在运行的镜像 ID固定为不可变 rollback tag。同步后逐文件核对本地与生产 SHA-256，并记录组合 manifest。
- 在服务器构建 `yunbay-new-api:prod`，构建期间旧标准容器继续提供服务；成功后固定 `yunbay-new-api:release-44a3e932`，并在切换前核对镜像内入口、主 bundle、quick-start chunk 的哈希/字节数及单 Provider/单 Prompt 稳定标记。
- 正式切换前启动独立于 SSH 会话、最迟 60 秒结束的 watchdog。只允许执行 `docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml up -d --no-deps --force-recreate --no-build new-api`；不创建绿实例、不修改或 reload Caddy、不重启 PostgreSQL、Redis、Sub2API、CLI Proxy、LDXP proxy 或 worker。
- 新标准容器 45 秒内未达到目标镜像、Docker healthy、容器内/宿主机 HTTP 200，或最终固定 upstream、静态身份、公网探针、依赖服务不变量任一失败时，watchdog/发布脚本必须自动把固定旧镜像重新标记为 `:prod`，恢复两文件源码和部署标记，并再次只重建标准 `new-api`；禁止留下等待人工输入的中间态。
- 发布成功须同时满足：`new-api`/Caddy healthy，Caddy upstream 仍只有 `new-api:3000`，其它服务容器 ID、启动时间和 restart count 未变化，公网 `/`、`/quick-start`、`/api/status` 连续 10 轮为 200，生产 bundle 与本地测试构建身份一致且包含本轮稳定标记。随后原子更新部署标记、追加仓库与桌面唯一运维手册，并清理传输包、顶层脚本、构建日志、探针和 watchdog 临时目录；源码备份、审计证据和 release/rollback 镜像标签保留。

### 15.9 生产结果

- 生产从 `7bc5e4e7b5ad67969bae0961b0a580670d206382` 精确同步上述两文件到功能提交 `44a3e932857d5f1b0a7abc955a1fa08878fe0161`。生产文件 SHA-256 分别为 `6ee31a63fdb442d730b57b04e6632024d412497da5500d42afc13f9afdc25916`、`40ac75b97316c5c0d694a1970c39ce9b0c0b4ef75311ca2031f26f0cb3cf6332`，组合清单 SHA-256 为 `cfa56c2cf171adc9b6e8bb4b55014be292555fa8a4936b5b9e0be305a0eb1f6a`。
- 构建期间旧标准容器持续服务。新镜像 `sha256:2ab381744eb207bc1246a31682c39ed3a7d569d7663dc17abe193e3b0717fdf5` 固定为 `yunbay-new-api:release-44a3e932`；切换前镜像 `sha256:0d948194bc0bf45a5f106a9256d8bc3dbf8c922136628c39457d6c1bf7f88d0e` 固定为 `yunbay-new-api:rollback-quick-start-merged-20260716T160439Z`。
- 只用 Compose 强制重建标准 `new-api`。最终容器 `ea163ea454d49d374bdbebd708fdd170d11e884c5d49250a074a1cad87ec8588` 为 `running / healthy / restart=0`；独立 watchdog 结果为 `success`，生产标记已原子更新到功能提交、两文件清单和新镜像。
- 切换从 `2026-07-16T16:08:37Z` 开始，探针在 `16:08:38Z` 至 `16:08:46Z` 连续观测到 502，`16:08:47Z` 起恢复 200。Caddy 同窗记录 25 个 502，首末时间为 `16:08:38.129Z` 与 `16:08:46.711Z`，窗口约 8.58 秒：23 个 connection refused、1 个旧连接 EOF、1 个旧连接关闭；没有 DNS/lookup 错误，低于 1 分钟稳定性上限。
- 生产入口 `/static/js/index.e4374b86a3.js` 的 SHA-256 为 `cf673d3ea925820fbc18db9e0c6b55e24bdc2d8aa2f896c4ac5d0dc60573b3b3`、字节数 `3064387`；quick-start chunk `/static/js/async/4963.937f71286d.js` 的 SHA-256 为 `1addc8ec05f0628f7f708f00dc4a93d5b9089ea74a60e603bcc98c710b83b4d9`、字节数 `47329`，与本地已测试构建完全一致。
- 公网 `/`、`/quick-start`、`/api/status` 独立连续 10 轮共 30 次全部为 200；新应用启动后日志中 `panic/fatal/error/unhealthy` 为 0。Caddy 文件、挂载和运行时哈希前后相同，主站 upstream 全程只有 `new-api:3000`。
- Caddy、PostgreSQL、Redis、Sub2API、CLI Proxy、LDXP proxy 与 worker 的容器 ID、启动时间和 restart count 在切换前后完全一致。本轮没有数据库迁移，没有修改业务数据、环境变量或其它服务。
- 成功备份为 `/opt/new-api/backups/quick-start-merged-flow-20260716T160439Z-44a3e932`。两文件源包和完整部署日志已归档到该目录，源包无 AppleDouble 条目；顶层传输包、脚本、状态、日志、PID 和 run dir 已清理，release/rollback 镜像与审计证据保留。

## 16. 生图完成条与底栏收敛补充计划（2026-07-17）

### 16.1 目标与性能指标

在第五页补齐最后一个缺失的反馈闭环：用户点击“一键导入生图设置”后，当前生图详情面板必须原位收敛为与 Provider 完成态同源的小条，并显示圆形对号；与此同时，底部包含“上一步 / 05 / 05 / 进入控制台”的长条必须通过非线性 layout 动画缩短，最终只保留底部居中的“进入控制台”。

- 生图完成态继续使用现有 `importConfirmed`，不新增 session 字段、计时状态或后端契约；刷新后若该字段为 `true`，页面直接恢复两个连续完成条和收敛后的主行动。
- 生图面板必须是同一个视觉对象从详情态变为完成态，不允许先消失再插入另一张卡；完成条使用 `QuickStartStepMarker step='05' complete`，与 Provider 的第 `04` 步对号、尺寸、圆角和弹簧参数保持一致。
- 完成条保留低优先级灰色“再次导入”动作，用于 CC Switch 未响应时重放 Prompt Deep Link；再次导入不得回退 `importConfirmed`、展开旧详情或改变底栏完成态。
- 底栏组件在状态切换前后必须保持同一 React/Motion 实例。长条外壳从当前最大 `34rem` 非线性收敛到能容纳六语言按钮文案的紧凑宽度；上一页、页码和未完成提示按同一时序淡出，现有主按钮从右侧平滑移动到中央并成为唯一控件。
- 常规动效使用项目已有 spring；`prefers-reduced-motion: reduce` 禁止空间位移和弹簧，只保留约 80ms 的透明度/尺寸反馈。任何状态下不得出现横向溢出、底栏遮挡、按钮文字越界或双重“进入控制台”。

### 16.2 现状与反馈证据

- 当前 `ImageSettingsImportPanel` 无论 `imported` 为何都保留完整终端预览，只把按钮改灰并替换图标，因此业务已完成但视觉对象没有进入稳定完成态。
- 当前 `QuickStartControls` 在最终页始终渲染三列长条；`canFinish=true` 只解除主按钮禁用，没有移除上一页与页码，也没有改变外壳宽度。
- `LandingSnapFrame` 把 `controlsComponent` 当作 React 组件类型渲染，而 Quick Start 的闭包组件会随 `importConfirmed` 改变引用；直接在现状上加 layout 会因组件类型更换而丢失前后测量，不能保证连续收敛。
- Provider 已验证的完成态提供最小充分范式：同一 `motion.div` 使用 `layout`、`AnimatePresence mode='popLayout'`、`QUICK_START_SPRING_TRANSITION` 和 `QuickStartStepMarker`，桌面完成高度约 `86px`，移动端约 `146px`。

### 16.3 GitHub 与本地模式参考

- 继续沿用本计划第 15.3 节核验过的 `motiondivision/motion@61833240`：稳定组件边界内用 `layout` 测量尺寸/位置，用 `AnimatePresence` 协调退出内容；本轮不引入新的动画库或自制时间轴。
- 继续沿用 `farion1231/cc-switch@f6e37ed9` 的 Prompt Deep Link 能力；只重放现有 `resource=prompt` 执行器，不修改 Prompt 正文、Base64 边界、API Key 传递或 `gpt-image-2` 规则。
- 组件、图标和样式继续使用当前 Button、Lucide 与 Quick Start Motion helper；不安装新 shadcn 组件，不修改主题或全局 CSS。

### 16.4 最小充分动态模型

- 控制输入：`importConfirmed`、最终页 `isFinalPage`、`reducedMotion`。
- 被控对象：生图面板高度/内容、底栏外壳宽度、上一页与页码可见性、主按钮位置。
- 稳定状态 A：`importConfirmed=false`，生图详情可见，长底栏保留返回能力和禁用的最终主行动。
- 稳定状态 B：`importConfirmed=true`，Provider 与生图均为完成条，底栏只有居中的“进入控制台”；再次导入只重放执行器，不离开状态 B。
- 反馈测量：DOM 状态标记、完成标记数量、面板/底栏 bounding box、动画中间帧宽度、最终中心偏差、session 恢复、捕获的 Deep Link、控制台 error/warn 和横向溢出。
- 稳定性优先：先保证刷新可恢复、按钮仍可操作、底栏不卸载和 reduced-motion 正确，再优化弹性幅度；不以更快动画换取布局抖动。

### 16.5 最小实施

1. 保留 `LandingSnapFrame` 现有 `controlsComponent` API，并新增可选 `controlsElement` 插槽；容器用 `cloneElement` 只注入翻页 API，Quick Start 传入模块级稳定 slot 和动态完成 props，让同一个 `QuickStartControls` 实例持续更新且不影响首页既有调用。
2. 给 `ImageSettingsImportPanel` 增加 `reducedMotion`，把根节点改为带 layout 的 Motion 容器；用 `AnimatePresence mode='popLayout'` 在终端预览和第 `05` 步完成条之间原位变形。
3. 完成条复用灰色“再次导入”按钮；未完成态继续显示真实 Prompt 脱敏预览和白色主按钮。根节点增加明确的 `ready / confirmed` 数据状态，供测试与浏览器反馈读取。
4. 把 `QuickStartControls` 外层改为可测量的 Motion 容器。最终完成前保持原三列布局；完成后条件卸载上一页与页码、移除辅助提示、切为单列紧凑宽度，让现有主按钮随父层 layout 移到中央。
5. 新增“Image settings imported”“Setup complete. You can now enter the dashboard.”两条六语言文案，并更新 quick-start locale 白名单与源码约束测试。

### 16.6 验证闭环

- 自动化：快速启动全量测试、TypeScript、定向 ESLint、Prettier、六语言 sync、`git diff --check` 与生产构建。
- 桌面 `1280x720`：从干净 session 走到 Prompt 导入，采样详情态、收敛中间帧和稳定态；确认第 `04`、`05` 完成条顺序一致，底栏宽度单调缩短，最终按钮中心偏差不超过 1px。
- 移动端 `390x844`：确认完成条与按钮文字不溢出，滚动区和 safe-area 不遮挡，完成态无横向滚动；长法语/俄语按钮文案仍装入稳定容器。
- 交互：再次导入捕获到同一个 `resource=prompt` Deep Link，DOM 与 session 均继续为 confirmed；刷新恢复完成态，按钮仍能进入控制台。
- 无障碍：键盘可聚焦再次导入和进入控制台；reduced-motion 下 transform 为 none、状态反馈仍完整；浏览器 console error 为 0。
- 本轮只完成本地代码、验证和 GitHub `main`。没有新的明确“部署”指令前，不连接生产、不传文件、不构建或重启任何生产服务。

### 16.7 实施节点

| 节点 | 状态 | 验收条件 |
| --- | --- | --- |
| 截图、源码与运行边界核验 | 已完成 | 已定位生图未收敛和底栏组件边界不稳定两处根因 |
| 完整闭环计划 | 已完成 | 状态、动效、恢复入口、反馈测量和生产边界已固定 |
| 生图完成条与稳定底栏实现 | 已完成 | 同一视觉对象收敛，主按钮连续移到中央 |
| 自动化与六语言检查 | 已完成 | 测试、类型、Lint、格式、i18n、构建全通过 |
| 双视口与 reduced-motion 浏览器验收 | 已完成 | 中间帧、最终对齐、键盘、刷新恢复和溢出通过 |
| GitHub main | 已完成 | 功能提交 `a6e524db` 已普通推送 main |
| 生产部署 | 已完成 | 固定 upstream 单服务切换成功，watchdog=success，生产标记已更新 |

### 16.8 实施验证与发布基线

- 最终实现没有用 Context 包裹五页内容。`LandingSnapFrame` 保留原 `controlsComponent`，新增向后兼容的 `controlsElement`；模块级 `LandingSnapControlsElement` 注入翻页 API，Quick Start 的稳定 slot 只接收动态完成 props。Motion 控件实例在 `importConfirmed` 切换前后不重挂载，且 React 19 `react-hooks/refs` 检查通过。
- 桌面 `1280x720` 完成前生图面板为 `600x348`、底栏为 `544px`；完成后生图面板收敛为 `600x86`，底栏收敛为 `304px`，主按钮与底栏中心均为 `x=640`。Provider 与生图两个完成条均使用同源圆形勾，DOM 顺序和视觉顺序一致，横向溢出为 0。
- 移动端 `390x844` 刷新恢复后 Provider 与生图完成条均为 `358x146`，底栏为 `304px` 且中心为 `x=195`；生图完成条与底栏之间保留约 `27px`，横向溢出为 0。法语、俄语长文案和 reduced-motion 已在上一轮浏览器轨迹中通过，本轮稳定 slot 改造不改变其渲染或动画参数。
- “再次导入”继续重放 `resource=prompt`，不回退 `importConfirmed`；刷新后直接恢复两个连续完成条和单一“进入控制台”。浏览器测试仅在测试页面临时拦截 `window.open`，没有启动或修改本机 CC Switch，测试标签页结束时清理。
- `bun test src/features/quick-start/*.test.ts` 为 `54 pass / 0 fail`；`bun run typecheck`、定向 ESLint、Prettier、`bun run i18n:sync`、`git diff --check` 与 `bun run build` 全部通过。
- 本地生产入口 `dist/static/js/index.58250d789d.js` 的 SHA-256 为 `86e3b28ff39082fdb0d56db4830e4d3d08d6417fe9ec0f7c0c34762589e2f08b`、字节数 `3065573`；Quick Start chunk `dist/static/js/async/4963.d9f8a87c33.js` 的 SHA-256 为 `2ec156b89b34f56a6e11c27f8a490178149483164d018cade9330da5750f3aed`、字节数 `50341`。
- 用户已明确发送“快点部署了”。发布仍遵循固定 `new-api:3000` upstream：先推送功能提交，再精确同步本轮 10 个源码/测试/翻译文件；构建期间旧标准实例持续服务，切换前固定当前运行镜像并启动独立 60 秒 watchdog，只执行 Compose `--no-deps --force-recreate --no-build new-api`。禁止修改或重启 Caddy、PostgreSQL、Redis、Sub2API、CLI Proxy 和 LDXP 服务。

### 16.9 生产结果

- 功能提交 `a6e524db3fda07da8184092fee5b9970fa8d0cab` 已普通 fast-forward 推送 GitHub `main`，随后从生产基线 `44a3e932857d5f1b0a7abc955a1fa08878fe0161` 精确同步 10 个源码、测试和翻译文件。生产组合清单 SHA-256 为 `7b53c1b8c1852158200451f25fd5b3246fec2e3d7b4d5ae8393f2b34572c3594`，没有数据库迁移、环境变量或业务数据变更。
- 构建期间旧标准容器保持 `healthy / restart=0` 并持续提供 200。新镜像 `sha256:a9a3c1791f56400170fa96e63a6cb5f0414e7b00a1eac43e9a3cec618b1ffd25` 固定为 `yunbay-new-api:release-a6e524db`；最终标准容器 `52c86e2fd6e435288c4e91975da39ebcc743c1f08da4d2dace18f5d80cf918e8` 为 `running / healthy / restart=0`，独立 watchdog 结果为 `success`。
- 切换探针在 `2026-07-16T17:58:47Z` 至 `17:58:55Z` 的约 9 秒窗口内记录 8 次 502，随后恢复并持续为 200，低于 1 分钟约束。发布脚本内置 10 轮固定探针全部通过，收尾再独立复测 5 轮 `/`、`/quick-start`、`/api/status` 全部为 200；新应用严重启动日志为 0。
- 生产入口 `index.58250d789d.js` 的 SHA-256 为 `86e3b28ff39082fdb0d56db4830e4d3d08d6417fe9ec0f7c0c34762589e2f08b`、字节数 `3065573`；Quick Start chunk `4963.d9f8a87c33.js` 的 SHA-256 为 `2ec156b89b34f56a6e11c27f8a490178149483164d018cade9330da5750f3aed`、字节数 `50341`，与本地已测试构建完全一致，并包含本轮完成条与底栏收敛标记。
- Caddy 文件、只读挂载和运行时配置哈希前后完全一致，upstream 始终只有 `new-api:3000`，无绿实例。Caddy、PostgreSQL、Redis、Sub2API、CLI Proxy、LDXP proxy 与 worker 的容器 ID、启动时间和 restart count 均未变化。
- 成功回滚目录为 `/opt/new-api/backups/quick-start-completion-collapse-20260716T175451Z-a6e524db`；固定回滚标签为 `yunbay-new-api:rollback-quick-start-completion-20260716T175451Z`，旧镜像为 `sha256:2ab381744eb207bc1246a31682c39ed3a7d569d7663dc17abe193e3b0717fdf5`。源包、部署脚本、构建日志、探针和 watchdog 证据已归档，服务器顶层传输包、脚本、状态文件、日志和临时 run dir 已清理。

## 17. 生图请求端点提示重写计划（2026-07-18）

### 17.1 目标与性能指标

把快速启动导入 CC Switch 的生图提示词重写为明确的双路由规则：文生图走 `POST /v1/images/generations`；图生图、参考图、局部修改和蒙版请求走 `POST /v1/images/edits`。

- 两个端点均保持用户指定的相对形式，不再在提示词中写完整域名；用户文本中的排版制表符归一为普通中文句号，不写入隐藏 Tab。
- 原始提示词、页面脱敏预览和 `resource=prompt` Deep Link 解码内容必须继续来自同一个 `buildQuickStartImagePrompt()`，避免三个副本漂移。
- `API Key为：` 后继续插入 `normalizeQuickStartApiKey()` 的动态结果；调用方传入已有 `sk-` 前缀时不重复添加，没有前缀时仍自动补齐。
- 不改变 `gpt-image-2`、主聊天模型禁令、API Key 脱敏、`outputs/`、4K 处理规则或 CC Switch Deep Link 参数。
- 本轮不新增 UI 文案 key、不改布局、动效、会话状态或后端契约；生产环境需等待新的明确部署指令。

### 17.2 GitHub 经验与事实依据

- `farion1231/cc-switch` 当前 README 明确 Prompts 可跨应用同步，并支持 `ccswitch://` Deep Link 一键导入提示词；因此继续复用现有 `resource=prompt` 和 Base64 内容链路，不引入第二种导入方式。
- `openai/openai-openapi` 的 OpenAPI 定义同时包含图片生成与 `POST /images/edits`（`operationId: createImageEdit`），与用户指定的 `/v1/images/generations`、`/v1/images/edits` 客户端路径一致。
- 当前项目已经用精确字符串测试锁定完整提示词，并对预览脱敏与 Deep Link 解码内容分别验证；本轮沿用这些现有反馈点，不新增抽象。

### 17.3 最小充分动态模型

- 输入：规范化后的 API Key。
- 控制器：`buildQuickStartImagePrompt()` 拼接唯一提示词正文。
- 输出 A：真实完整提示词；输出 B：替换为脱敏 Key 的页面预览；输出 C：Base64 编码后写入 CC Switch Deep Link 的内容。
- 稳定条件：A、B、C 均以新的双路由句开头，以原有“并保留原始图片。”结尾；动态 Key、模型禁令和保存规则保持不变。
- 扰动边界：换行、Base64 编解码和 Key 脱敏不得改变两个端点、请求类型映射或文本顺序，最终正文不得包含完整 `https://yunbay.xyz/v1/images/generations` 旧地址或隐藏 Tab。

### 17.4 最小实施与验证闭环

1. 先更新 `quick-start-cc-switch.test.ts`：用最新完整正文替换精确期望，并分别断言预览与 Deep Link 解码内容包含两个相对端点、请求类型映射和动态 Key，且不再包含完整旧地址。
2. 只修改 `buildQuickStartImagePrompt()` 的模板字符串，用用户最新双路由首段替换旧首段，保留动态 Key 插值与第二行保存规则。
3. 运行 Quick Start 全量测试、TypeScript、定向 ESLint、Prettier、`git diff --check` 和生产构建；确认只有计划、构造器和测试三个目标文件发生本轮变化。
4. 将实现结果回写本节，显式暂存本轮文件，提交并普通推送 GitHub `main`；保留现有无关运维文档、旧规格和 `outputs/`，不连接生产服务器。

### 17.5 实施节点

| 节点 | 状态 | 验收条件 |
| --- | --- | --- |
| 现状与 GitHub 依据核验 | 已完成 | 已确认单一构造链路、CC Switch Prompt Deep Link 和 `/images/edits` 规范 |
| 完整闭环计划 | 已完成 | 目标、边界、反馈点、发布边界已写入唯一计划文件 |
| 测试约束与最小实现 | 已完成 | 三种输出均使用最新双路由正文，Key 与保存规则不变 |
| 自动化与构建 | 已完成 | 测试、类型、Lint、格式、diff、构建全部通过 |
| GitHub main | 已完成 | 实现提交 `f27f1a05` 已普通推送 main |
| 生产部署 | 已授权，进行中 | 固定 upstream、旧实例持续服务、watchdog 保护下只切换 new-api |

### 17.6 实施验证记录

- `buildQuickStartImagePrompt()` 当前首段精确为：`文生图请求端点走POST /v1/images/generations；图生图、参考图、局部修改、蒙版请求端点走POST /v1/images/edits。`，随后继续保留 `gpt-image-2`、主聊天模型禁令和动态 Key 插值；第二行原图保存与 4K 规则未变。
- Key 继续经过 `normalizeQuickStartApiKey()`：无前缀输入自动补 `sk-`，已有前缀不重复添加；页面预览仍只显示脱敏 Key。完整正文、预览和 Deep Link 解码结果都不包含旧完整生图域名、上一版末尾追加句或隐藏 Tab。
- 精确字符串、脱敏预览和 Base64 解码三层测试先失败后通过；定向测试为 `9 pass / 0 fail`，Quick Start 全量为 `54 pass / 0 fail`。
- `bun run typecheck`、定向 ESLint、Prettier、`git diff --check` 与 `bun run build` 全部通过；生产构建生成主入口 `index.76b18de830.js` 和 Quick Start chunk `4963.1f05d9fe68.js`。
- 本轮目标差异仅为本计划、`quick-start-cc-switch.ts` 和对应测试；既有 `docs/yunbay-maintenance.md`、旧规格与 `outputs/` 保持原状。本轮没有连接或修改生产服务器。
- 计划、实现与测试已作为 `f27f1a05` 普通 fast-forward 推送 GitHub `main`；该实现轮结束时生产尚未授权，后续授权与执行边界见第 17.7 节。

### 17.7 生产执行计划（2026-07-19）

- 用户已明确发送“部署”，生产授权自本节生效。功能身份固定为 `f27f1a0536e876de3a108e1e1e1ac65f86be7d98`；当前 GitHub `main` 为 `98b450248b7320a7e36d043505cdf5cc578caa48`，目标两个 Quick Start 文件在功能提交之后没有再次变化。
- 本地复用已通过全部门槛的当前 `main` 构建：主入口 `index.76b18de830.js`，SHA-256 `6cc5545a1303bd7d8d2aef152e130ef15e3dbb36792965d21ca59bd2478f0d5a`、字节数 `3065573`；Quick Start chunk `4963.1f05d9fe68.js`，SHA-256 `a3f85013d3028198915b8326b43df01061f3b564f2adb5c4f5b4a83c23f7df30`、字节数 `50384`。
- 生产前置检查必须确认当前部署标记和两目标文件基线、标准 `new-api`/Caddy healthy、Caddy 文件/挂载/运行时 upstream 都只有 `new-api:3000`、公网 `/`、`/quick-start`、`/api/status` 为 200、部署锁空闲且无绿实例。任一条件不满足立即停止，不同步源码。
- 生产目录不是可信 Git checkout。本轮只从已提交的当前 `main` 精确打包、非删除式同步 `web/default/src/features/quick-start/quick-start-cc-switch.ts` 与对应测试；锁内先备份两文件和部署标记，并把当前正在运行容器的不可变镜像 ID固定为 rollback tag。
- 构建 `yunbay-new-api:prod` 期间旧标准容器持续服务。新镜像须先固定 release tag，并在镜像内验证双路由文案、动态 Key 片段、旧完整生图域名缺失，再进入切换阶段。
- 正式切换前启动独立于 SSH、最迟 60 秒结束的 watchdog；只执行 Compose `up -d --no-deps --force-recreate --no-build new-api`。目标镜像 45 秒内未 healthy/200，或固定 upstream、静态资产、服务不变量任一失败时，自动恢复两文件、部署标记和旧镜像，并再次只重建标准 `new-api`。
- 成功判据：生产 bundle 与本地入口和 Quick Start chunk 的文件名、SHA-256、字节数一致；双路由文案存在、旧完整地址不存在；宿主机和三个公网入口连续 10 轮为 200；new-api/Caddy healthy，严重启动日志为 0，其它服务容器 ID、启动时间和 restart count 不变。
- 成功后原子更新生产标记和两文件 manifest，追加仓库及桌面唯一运维手册，清理服务器和本地传输包、脚本、状态文件与临时运行目录；保留源码备份、release/rollback 镜像和审计证据。本轮不修改数据库、环境变量、Caddy 或其它服务。
