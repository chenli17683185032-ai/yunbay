# Grok Model Price Alias Matching Repair Plan

**Date:** 2026-08-03

**Goal:** 修复 Grok 渠道导入模型后，价格同步无法把上游别名映射到已有官方/OpenRouter 价格的问题；先建立可重复的“导入模型 ID -> 唯一正式型号 -> 价格预览 -> 持久化 -> 实际公开价格目录”闭环，再发布到生产。

**Architecture:** 保留现有保守匹配的三层结构，在字符规范化精确匹配之后增加一层显式、可审计的 xAI 别名映射。别名只来自当前生产上游实际映射及公开 GitHub 项目，不使用模糊包含匹配，不把无公开 token 价格的图像、视频或 Composer 型号伪装成聊天模型价格。

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
- [ ] 运行 `go build ./...` 或仓库既有等价构建检查。
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
- [ ] 提交功能修复与计划 checkpoint。
- [ ] 推送到 `origin/main`；若远端前进，先安全获取并在不覆盖用户文件的前提下整合。
- [ ] 记录功能提交 SHA。

### Task 6: 生产发布

- [ ] 发布前只读基线：部署 SHA、目标容器 ID/镜像/健康/restart/OOM、内外探针、相关 options 指纹。
- [ ] 获取生产部署锁，创建带时间戳的受限备份，至少包含目标源码、部署标记、Compose 摘要和相关 options 导出。
- [ ] 按既有 new-api 发布流程构建候选镜像并先验证候选启动/健康/聚焦价格预览。
- [ ] 固定旧镜像 rollback 标签和新镜像 release 标签。
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
