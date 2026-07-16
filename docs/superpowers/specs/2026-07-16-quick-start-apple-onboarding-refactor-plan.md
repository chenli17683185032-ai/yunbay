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
