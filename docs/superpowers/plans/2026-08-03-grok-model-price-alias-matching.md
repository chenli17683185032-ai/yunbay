# Grok Model Price Alias Matching Repair Plan

**Date:** 2026-08-03

**Goal:** 让生产 `Grok heavy` 渠道导入的 22 个模型全部进入可验证的计价闭环：聊天/Composer 按 token，图片按输入图、输出张数与分辨率，视频按输入媒体、输出时长与分辨率计费；同时修复别名、阻止自动同步覆盖人工价格，并通过真实请求、账单日志、公开价格目录和生产 options 四路反馈证明结果。

**Architecture:** 保留现有保守匹配的三层结构，在字符规范化精确匹配之后使用显式、可审计的 xAI 别名映射。同步层只负责能由 token 价格源证明的模型，并保护现有人工配置；媒体模型使用能够读取请求参数的固定/表达式计费路径，结算必须与实际请求的张数、输入媒体、时长和分辨率绑定。任何仅在价格页展示但无法在真实请求结算中执行的配置都不视为完成。

**Tech Stack:** Go 1.22+、现有 `service/model_price_sync.go`、`pkg/billingexpr`、PostgreSQL 生产、Docker、GitHub `main`。

---

## 0. 控制目标与性能指标

### 0.1 动态控制结构

- **对象：** 渠道导入后的 Grok 模型目录与模型价格配置。
- **控制器：** `MatchRequestedModelPrices`、`FindCanonicalPriceForModel` 和价格同步 preview/apply 流程。
- **测量：** 单元测试、`/api/pricing`、生产 options、容器健康和价格表达式。
- **执行器：** 显式模型别名表、价格同步 API、单容器受控替换。
- **环境与扰动：** xAI、OpenRouter、models.dev 和第三方 Grok 上游使用不同模型 ID；滚动别名会随时间变化；部分媒体模型不是 token 计价。

### 0.2 稳定性优先约束

- 精确匹配优先于别名匹配；已有同名 OpenRouter 条目不能被别名候选抢占。
- 别名只在目标价格 ID 唯一时成立；多个候选继续返回 `multiple_openrouter_matches`。
- 不引入前缀/包含/编辑距离等模糊匹配。
- 不给无公开 token 价格的图像、视频或 Composer 模型生成虚假表达式。
- 不修改现有价格，直到 preview 给出明确型号、来源和最终值。
- 生产发布只替换 `yunbay-new-api`，不重启 PostgreSQL、Redis、Caddy、Sub2API、Grok 或其它服务；可用性中断必须小于 60 秒。

### 0.3 验收指标

- `grok-4.5-latest`、`grok`、`grok-build-latest` 可唯一映射到 `grok-4.5` 价格。
- `grok-build` 可唯一映射到 `grok-build-0.1` 价格。
- `grok-4.20-reasoning` / `grok-4.20-non-reasoning` 可从 models.dev 的 `-0309-` 正式型号取得价格。
- `grok-latest` 仍优先匹配 OpenRouter 自身的 `~x-ai/grok-latest`，同时可合并 `grok-4.5` 官方高上下文价格。
- 未知别名和无价格模型仍被跳过，不产生错误计费配置。
- 聚焦 Go 测试、完整 `go test ./service`、构建和生产探针通过。
- 生产渠道的 22 个模型逐项都有明确状态：有效价格、显式别名继承，或因端点不支持而被安全拒绝；不存在静默落入默认倍率的模型。
- `grok-composer-2.5-fast`、`grok-composer`、`composer-2.5` 使用同一组可审计的 `$2/M input`、`$4/M output`、`$0.4/M cache read` 价格。
- `grok-imagine-image` 按输出张数计 `$0.02/张`，输入图按 `$0.002/张`；`grok-imagine-image-quality` 的 1K/2K 输出分别为 `$0.05/$0.07/张`，输入图按 `$0.01/张`。
- `grok-imagine-video` 按 480p/720p 输出分别计 `$0.05/$0.07/秒`，输入视频 `$0.01/秒`、输入图 `$0.002/张`；`grok-imagine-video-1.5` 按 480p/720p/1080p 输出分别计 `$0.08/$0.14/$0.25/秒`，输入图 `$0.01/张`。
- 图片 `n > 1`、2K 图片、视频不同时长/分辨率的预扣与结算均不会少收；非法或缺失关键参数不能静默按最低价结算。
- `grok-build` 精确保持人工价 `$8/M input`、`$41/M output`、`$1/M cache read`，后续自动同步 preview/apply 均不能覆盖。

---

## 1. 已确认事实与外部经验

### 1.1 生产只读诊断

- 生产渠道 `Grok heavy` 于 2026-08-02 创建，类型为 xAI，并一次导入 22 个 Grok/Composer 型号。
- 公开 `/api/pricing` 显示：
  - `grok-4.5`、`grok-4.3`、`grok-build-0.1` 和带 `-0309-` 的 Grok 4.20 型号已有价格；
  - `grok-4.5-latest`、`grok`、`grok-build-latest`、无日期 Grok 4.20 别名仍落到未匹配默认倍率 `37.5`；
  - 多个 Imagine/Composer 型号没有公开 token 价格。
- 当前生产容器均健康，本轮诊断没有写数据库、改配置或重启服务。

### 1.2 GitHub 与公开价格源

- `QuantumNous/new-api` 上游默认倍率目前只覆盖到 Grok 3，不能直接解决新别名。
- `Wei-Shaw/sub2api` 的 `backend/internal/pkg/xai/models.go` 明确维护以下实际映射：
  - `grok`、`grok-latest`、`grok-4.5-latest` -> `grok-4.5`
  - `grok-build` -> `grok-build-0.1`
  - `grok-build-latest` -> `grok-4.5`
  - `grok-composer`、`composer-2.5` -> `grok-composer-2.5-fast`
  - 无日期 Grok 4.20 reasoning/non-reasoning -> 对应 `-0309-` 型号
- `truefoundry/models` 的 `grok-4.5-latest.yaml` 记录了同一滚动别名和 xAI 官方价格来源。
- `models.dev` 当前给出 Grok 4.5、4.3、Build 0.1 和带日期 Grok 4.20 的价格；OpenRouter 当前给出 `x-ai/grok-4.5`、`~x-ai/grok-latest`、`x-ai/grok-build-0.1`、`x-ai/grok-4.3` 和 Grok 4.20。

### 1.3 重要边界

- Grok Imagine 图像/视频型号在 models.dev 中存在但 `cost` 为空；现有同步表达式按 token 计价，不能据此生成按图/按秒价格。
- Composer 正式型号当前没有 models.dev/OpenRouter 价格。代码可认识别名目标，但在价格源出现前仍应跳过。
- 生产现有 `grok-build` 有人工价格，不在本轮自动改价范围内。

---

## 2. 文件与变更范围

- Modify: `/Users/ethan/Documents/yunbay/service/model_price_sync.go`
  - 添加显式、可审计的 Grok 价格别名目标。
  - 保持精确规范化匹配优先，再尝试别名目标。
  - 复用唯一候选查找，保持多候选拒绝语义。
- Modify: `/Users/ethan/Documents/yunbay/service/model_price_sync_test.go`
  - 添加 OpenRouter 别名匹配、官方价格别名查找、精确优先和未知模型拒绝测试。
- Modify continuously: `/Users/ethan/Documents/yunbay/docs/superpowers/plans/2026-08-03-grok-model-price-alias-matching.md`
  - 本文件是本任务唯一实施计划；每个节点循环更新状态和证据。
- Modify after actual deployment: `/Users/ethan/Documents/yunbay/docs/yunbay-maintenance.md`
  - 追加脱敏生产发布记录。
- Modify after actual deployment: `/Users/ethan/Desktop/云贝/服务器相关/yunbay-new-api-vps-连接信息.md`
  - 在唯一有效本地连接手册追加部署、回滚和最终状态。

不修改受保护品牌/归属信息，不触碰工作区中既有未跟踪副本文件，不改前端、不改数据库 schema。

---

## 3. 实施步骤

### Task 1: 建立失败测试

- [x] 在 `service/model_price_sync_test.go` 添加表驱动测试，覆盖本次生产导入的可证明别名。
- [x] 证明 `grok-latest` 的同名规范化条目优先于别名目标，避免产生两个 OpenRouter 候选。
- [x] 证明 `grok-composer` 在正式目标没有价格时仍返回空价格。
- [x] 运行 `go test ./service -run 'TestModelPriceSync_.*Grok.*Alias' -count=1 -v`，确认修改前失败。

### Task 2: 最小别名匹配实现

- [x] 在 service 内添加显式 Grok alias -> canonical target 表，内容与已验证上游映射一致。
- [x] 抽取或添加唯一规范化候选查找 helper。
- [x] 在 OpenRouter 匹配中执行：原始精确 -> 字符规范化唯一 -> alias target 唯一。
- [x] 在官方价格查找中执行同样优先级。
- [x] 未命中和多候选状态保持现有返回协议。
- [x] 运行聚焦测试并确认通过。

### Task 3: 回归与静态验证

- [x] 运行 `gofmt` 处理实际修改的 Go 文件。
- [x] 运行 `go test ./service -count=1`。
- [x] 运行与模型价格接口相关的 controller 测试。
- [x] 运行 `go test ./...`；若仓库既有环境/外部依赖导致失败，记录精确失败并证明与本改动无关。
- [x] 运行 `go build ./...` 或仓库既有等价构建检查。
- [x] 检查 `git diff --check` 和定向 diff，确保无无关代码变化。

### Task 4: 本地真实价格源预演

- [x] 使用当前 OpenRouter 和 models.dev 公共响应执行测试/临时只读预览。
- [x] 验证以下最终高价策略：
  - Grok 4.5 家族至少为 input `$4/M`、output `$12/M`、cache read `$0.6/M`（覆盖 >200k 上下文风险）；
  - Grok 4.20 reasoning/non-reasoning 至少为 input `$2.5/M`、output `$5/M`、cache read `$0.4/M`；
  - 不为无价格媒体/Composer 型号生成表达式。
- [x] 将实际结果写回本计划 checkpoint。

### Task 5: GitHub main 闭环

- [x] 再次确认只暂存本任务文件，保留所有用户未跟踪文件。
- [x] 提交功能修复与计划 checkpoint。
- [x] 推送到 `origin/main`；若远端前进，先安全获取并在不覆盖用户文件的前提下整合。
- [x] 记录功能提交 SHA。

### Task 6: 生产发布

- [x] 发布前只读基线：部署 SHA、目标容器 ID/镜像/健康/restart/OOM、内外探针、相关 options 指纹。
- [x] 获取生产部署锁，创建带时间戳的受限备份，至少包含目标源码、部署标记、Compose 摘要和相关 options 导出。
- [x] 按既有 new-api 发布流程构建候选镜像并先验证候选启动/健康/聚焦价格预览。
- [x] 固定旧镜像 rollback 标签和新镜像 release 标签。
- [ ] 启动独立不超过 60 秒的 watchdog，只替换 `yunbay-new-api`；失败自动恢复旧镜像和部署标记，不能等待人工操作。
- [ ] 记录实际中断窗口并确认小于 60 秒。

### Task 7: 生产价格应用与反馈验证

- [ ] 通过现有 root-only 价格同步 API 先 preview 本次缺价的可证明聊天模型别名。
- [ ] 核对每个 alias 的目标、官方/OpenRouter 来源、最终价格和表达式后，再 apply。
- [ ] 首批只应用：`grok-4.5-latest`、`grok`、`grok-build-latest`、`grok-4.20-reasoning`、`grok-4.20-non-reasoning`。
- [ ] 不自动覆盖已有人工价的 `grok-build`，不应用 Imagine/Composer 无来源价格。
- [ ] 读回 `/api/pricing` 与 options，确认上述五个模型不再使用默认 `37.5`，且表达式和倍率一致。
- [ ] 验证 new-api、Caddy、PostgreSQL、Redis、Sub2API、Grok 服务健康，严重日志与 Caddy 502 无新增持续异常。

### Task 8: 运维记录与最终清理

- [ ] 更新仓库唯一维护记录 `docs/yunbay-maintenance.md`。
- [ ] 更新桌面唯一连接手册，不新建第二份文档，不记录密钥/令牌。
- [ ] 提交并推送运维记录到 GitHub `main`。
- [ ] 清理本任务的服务器发布包、候选容器/镜像标签、临时脚本和本地临时文件；保留成功审计备份、release/rollback 镜像。
- [ ] 最终确认本地 `main == origin/main`，工作区仅剩任务开始前已有的用户文件。

---

## 4. 回滚策略

- **代码回滚：** 恢复发布备份中的 service 源码和部署标记，将固定 rollback 镜像重新标为生产镜像，仅 force-recreate `yunbay-new-api`。
- **配置回滚：** 从发布前相关 options 导出恢复这五个模型的 ratio/cache/completion/billing mode/billing expression 条目；不覆盖整张 options 表。
- **守护条件：** 容器未 running/healthy、内外探针失败、价格接口异常或启动严重日志出现时自动回滚。
- **禁止动作：** 不恢复整个数据库，不重启 PostgreSQL/Redis/Caddy/Sub2API/Grok，不删除发布审计备份，不更改其它模型价格。

---

## 5. Checkpoints

### Checkpoint 0 - Discovery complete

- [x] 已确认本次导入渠道、时间和完整模型列表。
- [x] 已确认未匹配默认倍率的具体模型。
- [x] 已核对 OpenRouter、models.dev、上游 new-api 和相似 GitHub 项目。
- [x] 已读取 `pkg/billingexpr/expr.md`，确认表达式价格单位与结算规则。
- [x] 已确认工作区存在大量无关未跟踪副本，后续全部避开。
- [x] 已创建本任务唯一全量计划，正式编码尚未开始。

### Checkpoint 1 - Failing regression reproduced

- [x] 主工作区测试被既有未跟踪 `* 2.go` 文件的重复定义阻塞，未删除或改名这些用户文件。
- [x] 已从当前 `HEAD` 创建临时隔离 worktree，仅同步本任务测试改动。
- [x] 隔离红测证明：Grok alias OpenRouter 匹配失败、官方价格查找失败、`grok-latest` 最终 input 仍为 `$2/M` 而不是保守的 `$4/M`。

### Checkpoint 2 - Minimal matcher green

- [x] 新增显式 alias 表和唯一候选 helper，没有引入模糊字符串匹配。
- [x] 新增测试全部通过，`grok-latest` 精确 OpenRouter ID 优先级保持不变。
- [x] 现有全部 `TestModelPriceSync*` 测试通过。
- [x] `git diff --check` 通过，主工作区与隔离 worktree 的 service 源码/测试逐字一致。

### Checkpoint 3 - Live source feedback corrected

- [x] 当前 models.dev 响应约 3.45 MiB，低于 10 MiB 上限；直接解析得到 298 个可计价键。
- [x] 实时数据证明 models.dev 同时提供无前缀和 `xai/` 前缀的等价键；原 alias 实现会把它们误判为多候选。
- [x] 新增持久回归测试复现双键失败，并修正为 alias 目标精确键优先、供应商前缀规范化次之。
- [x] 当前 OpenRouter + models.dev 临时预演为 6 个 alias 可同步、1 个 Composer 跳过；价格断言符合 Task 4。
- [x] 临时实时测试只用于反馈测量，已删除，不把外网依赖加入常规测试套件。
- [x] `go test ./...` 中主项目已执行的 common/controller/model/relay/router/service/setting 等包通过；总命令仅因根包缺少 `web/classic/dist` 和嵌入的 `infra/sub2api` 非主模块依赖而失败，均为本改动前既有结构问题。

### Checkpoint 4 - GitHub main synchronized

- [x] 功能、测试与本计划已作为提交 `3205e0165e4bde92ec692ecfe5eb77d736e7171e` 合并到 `main`。
- [x] 已推送到 `origin/main`，本地 `HEAD` 与远端完全一致。
- [x] 未跟踪的 `* 2.*`、`outputs/` 和 `tmp/` 等用户文件保持原样，未纳入提交。

### Checkpoint 5 - Candidate image validated

- [x] 发布基线再次确认生产源码哈希为 `5a05ad72...`、旧镜像为 `sha256:3b6fb4bf...`，new-api 和依赖容器均 `healthy / restart=0 / OOM=false`。
- [x] 成功审计备份为 `/opt/new-api/backups/grok-price-alias-20260803T091418Z-3205e016.hBnnhhxs`，相关 10 个价格 option 行、旧源码、部署标记和 Compose/Caddy 基线已受限保存并通过指纹检查。
- [x] 候选镜像 `sha256:4998c275...` 已完成前后端和 Go 生产构建，镜像 label 中的单文件哈希与 `753ee800...` 一致。
- [x] 不接入 Caddy 的候选容器 preview 返回 `requested=5 / syncable=5 / skipped=0`，Grok 4.5 别名为 `$4/$12/$0.6`，Grok 4.20 别名为 `$2.5/$5/$0.4`。
- [x] root 管理字段原为非 NULL 空 `CHAR(32)`；候选验证使用一次性随机 access token，验证后已恢复为原精确空值，临时凭证文件已删除。

### Checkpoint 6 - Alias release completed

- [x] `3205e0165e4bde92ec692ecfe5eb77d736e7171e` 已部署为生产镜像 `sha256:4998c275ad865d5e02d2f9200888ed2fabc6db02211305638f7b43f8cfd6d9cc`。
- [x] 生产 `yunbay-new-api` 健康，`restart=0`、`OOM=false`；审计备份保留在 `/opt/new-api/backups/grok-price-alias-20260803T091418Z-3205e016.hBnnhhxs`。
- [x] 第一阶段聊天别名已脱离默认倍率；反馈测量同时暴露出媒体/Composer 仍未覆盖、`grok-imagine-video-1.5` 被错误写成 token 价，以及 `grok-build` 人工价被同步覆盖。
- [x] 由于反馈表明“名称匹配成功”不等于“真实单位计费正确”，控制目标已扩展为本文件下述第二阶段，旧阶段不再作为最终交付判定。

---

## 6. 第二阶段：22 个模型完整计价闭环（当前实施）

### Task 9: 建立生产模型与计费路径矩阵

- [x] 只读导出渠道 45 的完整 22 个模型，以及 `ModelRatio`、`CompletionRatio`、`CacheRatio`、`ModelPrice`、`ModelBillingMode`、`ModelBillingExpr` 当前值。
- [x] 对每个模型标注请求端点、上游实际型号、单位、价格来源、预扣路径、结算路径和当前缺陷。
- [x] 精确识别并单独保存 `grok-build` 的人工配置；禁止用整个 options 快照回滚覆盖其它管理员变更。
- [x] 验证当前图片、编辑和视频路由能否把计价所需的 `n`、输入图数量、`resolution`、`duration` 传到预扣与结算；不能只依据 DTO 猜测。

### Task 10: 先写失败测试，定义最小充分模型

- [x] Composer 三个名称都命中 `$2/$4/$0.4`，同时仍保持显式别名和唯一候选语义。
- [x] 自动价格同步 preview/apply 对已有人工配置的 `grok-build` 返回保护状态，且不修改其五类计费 option。
- [x] `grok-imagine-video-1.5` 不再由 token 价格同步器生成 `p/c/img` 表达式。
- [x] 图片测试覆盖：`n=1`、`n>1`、1K、2K、输入图数量、字段缺失及不支持的值。
- [x] 视频测试覆盖：480p/720p/1080p、不同时长、输入图/输入视频，以及字段缺失/非法值。
- [x] 测试同时断言预扣倍率、固定价结算输入和冻结 Task 倍率，防止仅价格目录显示正确。

### Task 11: 最小实现聊天/Composer 与同步保护

- [x] 为 Composer 正式型号建立本地、可审计价格条目，并复用现有显式别名映射；不伪装成 `grok-build` 型号。
- [x] 同步逻辑只更新当前真正缺价、且未被人工锁定的模型；对现有 tiered/manual 配置采用明确的跳过原因。
- [x] 将媒体模型从 token 同步候选中排除，清除产生错误 token 计价的根因，而不是只修一次生产数据。
- [x] 聚焦测试、`go test ./service`、`git diff --check` 通过。

### Task 12: 图片真实单位计费闭环

- [x] 选择并证明最小路径：现有固定价格预扣会读取 `ImagePriceRatio`，因此把整次请求价折算为单一倍率，无需引入媒体 billing expression。
- [x] JSON 图片生成保留并转发 `n`、`resolution`/等价质量字段；图片编辑保留输入图数量及计价上下文。
- [x] 对缺失值采用与上游一致且可证明的默认值；对未知图片分辨率按 2K 保守计费，禁止低价兜底。
- [x] 将 Grok 图片模型纳入正确 API 类型的价格目录；生产公开目录与真实结算一致性留待发布反馈验证。

### Task 13: 视频端点与按秒计费闭环

- [x] 复用现有任务框架接通 xAI/Sub2API 的 `/v1/videos/generations`，保存冻结计费快照并复用失败退款与轮询结算。
- [x] 保留并验证 `duration`、`resolution`、输入图/视频元数据；缺少输入视频时长或非法参数在预扣前拒绝。
- [x] 旧的 `/v1/chat/completions` 错误调用不能触发媒体计费；对视频模型经聊天端点请求返回明确错误。
- [ ] 用受控本地/候选上游请求证明 480p/720p/1080p 和不同时长的金额，不能仅靠表达式单测宣称完成。

### Task 14: 回归、构建与 GitHub main

- [x] 在隔离 worktree 执行聚焦测试，避开但不删除主工作区用户的 `* 2.go` 副本。
- [ ] 受影响包、`go test ./service`、controller/relay/task 测试和 `git diff --check` 已通过；生产构建留待候选镜像阶段完成。
- [ ] 对 22 个模型生成本地机器可读验收矩阵，确认无默认倍率、无错误 token 媒体表达式、无少收路径。
- [ ] 只暂存本任务文件，提交并推送 GitHub `main`；保留用户未跟踪文件。

### Task 15: 低中断生产发布、精确配置修复与反馈

- [ ] 发布前创建新的受限审计备份，单独导出 22 个模型相关 option 键和值，并记录容器/探针基线。
- [ ] 候选镜像先完成离线健康、价格 preview、图片/视频计费 smoke test；失败不得切流。
- [ ] 用独立 watchdog 仅替换 `yunbay-new-api`，60 秒内自动判定成功或恢复旧镜像；记录实际中断窗口。
- [ ] 精确清除 `grok-imagine-video-1.5` 错误 token 配置，精确恢复 `grok-build` `$8/$41/$1`，再应用其余缺价模型；不整体覆盖 options。
- [ ] 读回完整 22 项并执行至少一条不产生外部付费副作用的计费预演；允许时再以最小媒体请求核对实际账单。
- [ ] 验证 new-api、Caddy、PostgreSQL、Redis、Sub2API、Grok 容器健康及持续错误/502 基线。
- [ ] 更新 `docs/yunbay-maintenance.md` 和桌面唯一连接手册，提交推送；清理本任务临时脚本/clone/worktree，保留备份和用户文件。

## 7. 第二阶段回滚与失败闭锁

- **配置原子性：** 每次只更新目标模型对应 map 项；写后立即读回并比对。任一项异常则仅恢复本批次修改的键，不替换整张 options 表。
- **计费闭锁：** 未能证明真实结算的媒体模型不得标记为已覆盖；若端点尚不能工作，则显式拒绝该端点，避免以错误低价接受请求。
- **发布回滚：** watchdog 在容器不健康、内部/外部探针失败或计费 smoke test 不一致时自动恢复固定 rollback 镜像，无需等待人工操作。
- **稳定性判定：** 先保证聊天现有业务和人工 `grok-build` 价格不回归，再开放图片，最后开放视频；每一层通过反馈后才扩大下一层。

### Checkpoint 7 - Full-pricing scope established

- [x] 已读取计费表达式设计文档和 `karpathy-guidelines`，明确表达式输出单位、请求参数读取与结算约束。
- [x] 已核对 xAI 当前官方媒体价格，以及 `Wei-Shaw/sub2api`、`chenyme/grok2api` 的 Grok/Composer 型号映射经验。
- [x] 已确认 8 个完全缺价模型、1 个错误 token 媒体配置和 1 个被覆盖的人工价格；第二阶段全量计划已在正式编码前写入本文件。
- [x] 生产 22 模型计费矩阵与端点能力测量完成：文本/Composer 使用 token 同步，图片使用固定基础价结合请求倍率，视频复用异步 Task 计费快照与失败退款。
- [x] 图片现有固定价格链路会在预扣后通过 `ImagePriceRatio` 与 `OtherRatio("n")` 结算；实现需把输入图成本折算进单张倍率，避免输入成本被 `n` 重复放大。
- [x] xAI 图片适配当前丢失 `size` 且编辑请求丢失 multipart 文件；视频兼容路径 `/v1/videos/generations`、xAI TaskAdaptor 和 `openai-video` 端点分类均缺失。
- [x] Sub2API 视频默认值已测量为 8 秒/480p，允许 1-15 秒；图片未知分辨率按 2K 保守计费，视频非法值在转发前拒绝，防止最低价兜底。

### Checkpoint 8 - Red tests next

- [x] 已在 `/private/tmp/yunbay-grok-full-price.ShG46v` 干净隔离 worktree 中建立 Composer、同步保护、图片媒体参数、xAI 视频任务和端点分类失败测试。
- [x] 红测分别证明：Composer `syncable=0/3`、保护模型仍产生 option 更新、图片缺少输入计数/size、xAI TaskAdaptor 与 `/v1/videos/generations` 路由缺失。
- [x] 主工作区用户的 `* 2.*`、`outputs/`、`tmp/` 均未进入测试 worktree，未删除或改写。

### Checkpoint 9 - Minimal implementation green

- [x] 纯 Grok 媒体价目与归一化函数通过测试，覆盖图片输出张数/分辨率/输入图和视频时长/分辨率/输入媒体。
- [x] 同步保护和 Composer 本地价通过测试；`grok-build` 与 6 个媒体模型即使被 override 也不会产生 option 更新。
- [x] 图片请求与 xAI multipart 转发通过测试，Grok 图片把整次请求价折算进 `ImagePriceRatio` 且不再重复乘 `n`。
- [x] xAI 视频 TaskAdaptor、创建/轮询/内容路由和端点分类通过测试，冻结倍率为 `请求总美元价 / ModelPrice`。
- [x] 受影响包 `common`、`dto`、`relay/helper`、`relay/channel/xai`、`relay/channel/task/xai`、`router`、`setting/ratio_setting`、`service`、`relay`、`controller` 全部通过，`git diff --check` 通过。
