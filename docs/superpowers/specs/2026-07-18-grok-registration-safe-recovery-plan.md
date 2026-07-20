# Grok 注册安全恢复计划

## 目标与边界

本轮目标是恢复 Grok 账号注册，同时保持现有 API 持续可用，并避免注册任务再次把服务器资源推到失控状态。

用户已确认：历史版本本地注册曾经成功，不需要 YesCaptcha；因此本轮不把 YesCaptcha 作为前置条件，也不以当前失败的 Solver 试验推断历史路径不可行。

用户进一步确认：历史注册由明确的“注册”触发后才执行，不应恢复成无人值守的持续补量。只有把“注册”作为明确执行指令的独立消息才算触发；讨论、引用或说明该词不算。恢复阶段保持手动触发、单批次、单并发；没有收到触发就不创建邮箱、不启动浏览器。

控制对象是“独立注册工作负载”，API、数据库、Redis 和正常模型请求不回滚、不与注册共享无限资源。所有操作遵循稳定性优先和可重复闭环：先测量历史成功配置，再以单账号/并发 1 复现，确认反馈后才逐级放量。

## 可验证的成功指标

1. `grokcli-2api` 保持 `healthy`，重启次数不增加，OOM 计数不增加；真实 API 并发探针在注册期间仍全部完成。
2. 注册任务默认只允许一个活动批次、一个 session、并发 1、无预取；注册工作负载有明确 CPU、内存、PID 和超时上限。
3. 至少一个真实账号完成注册、认证文件导入并能被 API 使用；浏览器、solver、临时 session 和注册容器均被回收。
4. 任一超时、OOM、异常 PID/内存增长、批次契约错误或 API 探针失败，注册控制器自动停止注册，不重启 API，不扩大并发。
5. 只有连续观察窗口内 API 和宿主机指标稳定，才允许把并发从 1 调到下一个固定档位；本轮不追求批量速度。

## 系统模型与不变量

- 对象：注册浏览器、Turnstile 处理、邮箱/SSO、认证文件导入及其外部网络时延。
- 控制器：单飞注册 runner、批次状态机、资源限制、超时和熔断阈值。
- 执行器：独立注册容器/进程；不得把 solver、Camoufox 和 API worker 放进同一无限 cgroup。
- 测量：容器 health/restart/OOM/PID、CPU/内存、宿主机 load、Redis runner 锁/心跳、注册日志、API 真实 SSE 探针。
- 环境扰动：Turnstile 页面变化、代理出口、邮箱延迟、上游限流和浏览器资源抖动。

不变量：API 服务不因注册失败而重启；注册并发不超过当前档位；批次缺少 `batch_id` 或状态失真时立即停止全部活动注册；所有等待均有硬超时。

## 实施步骤与检查点

### 1. 盘点当前状态与历史证据

- [x] 读取当前生产容器、资源限制、runner 锁/心跳和 API 基线。
- [x] 读取历史备份中的 Compose、脱敏环境变量、部署日志、首个注册 worker 日志和成功导入记录。
- [x] 对比候选提交/镜像的注册引擎、浏览器类型、Turnstile lazy/thread/idle、邮箱 provider、代理出口和 batch 契约。
- [x] 只认定带有真实“注册成功 + auth 导入 + API 可用”证据的版本为历史成功基线。

### 2. 构造隔离 canary

- [x] API 继续使用当前隔离镜像和现有生产容器。
- [x] 注册使用历史成功代码/镜像的独立容器或一次性 runner，不占用 API 端口和进程空间。
- [x] 初始参数固定为 1 个 batch、1 个 session、并发 1、预取 0；资源和时间上限比生产主机预算更严格。
- [x] 先做 dry-run/健康检查，确认失败只会停止注册 runner。

### 3. 单账号真实闭环

- [x] 启动服务器端有界 watchdog，持续采集 API 探针和注册资源。
- [x] 执行一个真实注册；不输出邮箱、cookie、token、API key 或验证码。
- [x] 验证账号认证文件导入、账号状态可用、浏览器退出、runner 锁释放和临时文件清理。
- [x] 注册期间至少多轮执行 API 真实 SSE 并发探针，记录成功率和服务端处理耗时。

### 4. 反馈与逐步放量

- [x] 单账号失败时保留日志和现场摘要，自动停止，不增加并发；先修正历史配置差异。
- [ ] 单账号成功且观察窗口稳定后，最多提升到预先批准的下一个固定档位；每档重新验证资源和 API 指标。
- [ ] 任一指标越过阈值立即降档/停止，禁止自动连续放量。

### 5. 收尾与同步

- [x] 更新本计划和唯一 `docs/yunbay-maintenance.md` 运维记录，写入根因、配置摘要、资源峰值、成功判据和回滚方法。
- [x] 清理 canary 容器、临时挂载、探针和工作区临时文件，但保留可审计备份及可回滚镜像。
- [x] 只提交本轮必要文件，按项目约定 fast-forward 合并并推送 GitHub `main`；不覆盖用户已有改动。

## 回滚/熔断条件

出现 API 5xx/连接失败、API 容器 unhealthy/restart/OOM、宿主机 load 持续异常、注册 cgroup 接近内存/PID 上限、批次状态缺少 `batch_id`、注册硬超时、上游连续失败、配置快照为空或恢复失败时：立即停止注册 runner 和活动浏览器，释放其资源，保留 API 原容器不动；配置快照未验证前不得创建真实注册 session，只有恢复基线后才允许再次单账号试验。

## 当前进度

- [x] 计划文件建立。
- [x] 历史成功版本和配置证据核对。
- [x] 单账号隔离 canary。
- [x] API 并发/资源观察窗口。
- [x] 运维记录、测试和 GitHub `main` 收尾。

## 历史证据反馈（2026-07-18）

- 用户判断成立：YesCaptcha 不是历史成功注册的依赖。历史配置为 `captcha_provider=local`、内联本地 Solver；日志中的 `YesCaptcha` 只是本地 Solver 所兼容的请求协议名称，endpoint 实际为本机 `127.0.0.1:5072`。
- `20260713-225731-fast-register` 备份的注册指标有 6 次 `signup_complete`、6 次 `sso_obtained`、2 次即时 `auth_imported`，均为成功；`auth.json` 同期有 99 个有效结构化认证记录。
- PostgreSQL 任务历史显示，`20260716-turnstile-fa7de8d` 修复浏览器池后先有真实 `2/2`，随后多个 500 账号批次分别成功 255 至 322 个；最后一个成功大批次在 `2026-07-17 21:35 +08:00` 为 `305/500`。
- 从 7 月 18 日 API 资源优先版本切换后，首批变为 `0/13` 且错误全部为本地 CAPTCHA 无解；后续单账号 canary 也未成功。因此不能把后续失败追溯成“历史一直依赖外部 YesCaptcha”。
- 已保留在服务器的最后成功镜像为 `grokcli-2api:20260716-turnstile-fa7de8d`。其 `grok-build-auth` Solver 与当前镜像哈希一致；Turnstile Solver 的逻辑差异只有默认线程数，而运行时均显式传入线程数。下一步用该不可变镜像和历史运行参数做隔离复现，以区分镜像/运行资源/页面状态三类因素。
- 当前生产 API 为 `healthy/restart=0`，约 691 MiB / 53 PID；主机可用内存约 5.1 GiB，生产注册容器数和 Redis 注册键均为 0。

## 用户历史行为补充（2026-07-18）

- 上一版的成功体验以手动“注册”触发为边界；本轮不启用 `GROK2API_REG_AUTO_MAINTAIN`，也不让维护器自行补量。
- 触发器必须校验明确执行意图；包含“注册”一词的解释、引用、复盘或条件说明不得启动 runner。
- 验证码采用本地 Solver 的兼容协议；请求体可以出现协议字段名，但不得配置外部 YesCaptcha endpoint 或密钥。
- 旧 YYDS 共享域名目前返回 `shared_domain_restricted`，不能作为恢复回退；canary 改用历史 MoeMail 配置，仅验证单账号闭环。

## 首轮历史镜像 canary 反馈（2026-07-18）

- canary 全程未重建生产 API。第一次启动发现旧单 worker 仍会依据共享数据库 feature flag 竞选 maintainer leader，立即停止；后续显式使用 `GROK2API_MAINTAINER_LEADER=never` 和独立 Redis 前缀，确认 token maintainer/model health 不再运行。
- 两次前置失败分别是旧管理路由前缀差异和 YYDS 专用 key 未映射到旧兼容参数，均在创建注册 session 前结束，账号总数保持 3737。
- 真正单账号试跑中，本地 Turnstile Solver 约 55 秒成功返回 token，证明本地求解路径仍然可行，不需要外部 YesCaptcha。注册活跃时 5 路真实 SSE 为 5/5，首模型内容约 1.4 至 1.8 秒，API 始终 healthy/restart=0/OOM=0。
- 该 session 随后因 `CreateEmailValidationCode` 返回 `HTTP 200 + 空响应体` 被旧客户端判为失败，未导入账号；同时历史镜像 cgroup 峰值约 1.61 GB / 218 PID，触发软资源保护并自动停止/删除 canary。
- 更早的轻量 `20260713-fast-register-1` 镜像虽然资源小，但历史 `.env` 确实包含一枚 46 字符外部 YesCaptcha key，不能作为用户要求的本地注册方案。

## 协议兼容修正反馈（2026-07-18）

- 新分支 `codex/grok-registration-recovery` 只修改 `XConsoleAuthClient.create_email_validation_code()`：仅该发送验证码 RPC 的 `HTTP 200 + 空 body + 无 gRPC 错误` 视为兼容成功，后续邮箱收码仍是权威反馈；其它 gRPC 调用继续严格校验。
- 新增 4 项回归，覆盖空 body 200、非 200、显式 gRPC 错误和 VerifyEmailValidationCode 不放宽。
- 本地通过：协议兼容 4 项、注册隔离 13 项、Turnstile 池 20 项、API 优先 4 项、账号并发 11 项；Python 编译和 `git diff --check` 通过。
- prompt-cache 测试仅在本机因缺少仓库已声明的 `python-multipart` 依赖而无法导入，将在依赖完整的候选镜像中补跑。

## 第三轮手动 MoeMail canary 反馈（2026-07-18）

- 代码修复提交 `2b43e9e` 补齐 `EmailRegistrationBody` / `RegistrationConfigBody` 的 provider 专用 host 字段；否则从 YYDS 切换到 MoeMail 时会丢失 `moemail_base_url`，错误复用旧 YYDS 地址。相关 55 项回归全部通过，候选镜像为 `grokcli-2api:20260718-registration-host-fields-2b43e9e`（`sha256:f8b76da60f87314ca12ce2cbd075ba1e65e7e36c13fed9aced98ed1f1ce66f00`）。
- canary 运行目录为 `/home/deploy/grok-backups/20260718T134046-history-registration-canary`。只启动 1 个 batch / 1 个 session / 并发 1 / 预取 0；`GROK2API_REG_AUTO_MAINTAIN=0`、独立 Redis 前缀、`restart=no`，没有启动生产注册 worker。
- 使用历史 MoeMail 地址和域名、本地 Turnstile Solver；运行时选择 `captcha_provider=local`，适配器将兼容协议 key 归一为 `local`，没有调用外部 YesCaptcha。YYDS 仅保留为生产配置的原激活槽位，未用于本次创建邮箱；后续手动入口已额外把两个兼容 key 环境变量固定为 `local`。
- 账号闭环成功：注册结果为 `imported=true`，账号总数 `3737 -> 3738`；新认证记录带 refresh token，随后单账号 probe 返回 `ok=true`、`pool_status=normal`、无冷却。邮箱、token、cookie 和 key 未写入计划或控制日志。
- 注册期间资源峰值约 `1.43 GiB / 213 PID`（canary 硬限 `1.5 GiB / 220 PID`、CPU `1.0`）；保护器在连续越线样本后停止并清理 canary。停止原因是 `resource_guard`，不是注册终态失败；导入已在保护器触发前完成。
- 注册活跃期真实 API 探针 `5/5`，随后收尾复验再次 `5/5`；API 容器始终为 `healthy`、`restart=0`、`OOM=0`，最终约 `726 MiB / 53 PID`。canary Redis 键清理为 `0`，注册容器、浏览器和临时 session 均已回收。
- canary 启动前保存的注册配置已通过 API 原路恢复；当前生产仍保持 API 镜像 `grokcli-2api:20260718-registration-isolation-32bb09f`、自动注册关闭。成功一次只证明“单次手动触发可行”，不批准提高并发、批次或恢复无人值守补量。

## GitHub 收尾反馈（2026-07-18）

- 本计划与 Grok 运维章节已作为提交 `745aa1cf` fast-forward 推送到 `yunbay/main`；暂存时只选择本轮两个文件，没有带入工作区原有计划、图像任务记录或 `outputs/`。
- 下一次注册仍是新的显式控制动作：必须收到独立、明确的“注册”执行指令后才可启动，并继续从单账号档位运行。

## 第四轮显式触发执行（2026-07-18 14:53 CST）

目标：响应用户本次明确的“启动注册”指令，只增加一个可用账号；不恢复自动补量，不提高并发，不复用上一轮已清理的临时进程。

- [x] 确认指令具有明确执行意图，并复核 API `healthy/restart=0/OOM=0`、注册容器为 0、宿主机资源余量充足。
- [x] 从不可变候选镜像创建全新隔离 runner：1 batch / 1 session / 并发 1 / 预取 0、`restart=no`、独立 Redis 前缀、本地 Solver、自动维护关闭。
- [x] 启动硬超时和资源 watchdog；越过注册软线、API 异常、批次状态异常或账号导入终态后，只停止并回收注册侧。
- [x] 注册活跃期间执行真实 API 并发探针，确认 API 请求不因注册排队；记录脱敏资源峰值和结果。
- [x] 核对账号总数变化及新账号单独测活，恢复临时配置并清理容器、浏览器、Redis 临时键和锁。
- [x] 把本轮反馈同步到本计划、仓库运维记录和桌面唯一服务器手册，并只提交本轮必要文档到 GitHub `main`。

### 第四轮结果

- 运行目录：`/home/deploy/grok-backups/20260718T145340-history-registration-canary`；候选镜像为 `grokcli-2api:20260718-registration-host-fields-2b43e9e`。只创建一次性 `grok-registration-history-canary`，常驻 `registration-producer` 和 API 均未重启。
- 注册结果为 `imported=true`，账号总数 `3738 -> 3739`；新账号单独测活返回 `ok=true`、`pool_status=normal`、`in_cooldown=false`。
- 注册 cgroup 硬限为 `1 CPU / 1.5 GiB / 220 PID`；观测峰值约 `1.405 GiB / 202 PID`，未触及硬限。单次样本接近内存软线后回落，没有连续越线熔断，也没有 API 排队或 OOM。
- 活跃期间真实 SSE probe 为 `5/5`，5 路均有业务成功、finish 和 `[DONE]`，最慢总耗时约 `4.12s`；API 始终 `healthy / restart=0 / OOM=0`。
- 本次请求使用 `captcha_provider=local` 和本机 Solver，未调用外部 YesCaptcha/YesChatUp。临时 Redis 前缀曾产生 3 个 session/index 键，已按精确前缀删除为 `0`；容器、浏览器、锁和临时启动脚本均已回收。
- 运行时配置自动快照为空，不能视为恢复成功；已将当前状态保留在审计目录，并用上一轮已验证的 YYDS 基线快照手动原样恢复。后续 runner 必须把“快照文件非空 + 恢复回读一致”设为注册前置条件，否则直接停止，不创建 session。

## 第四轮文档收尾

- 本轮恢复结果和运维记录已作为提交 `da401a11` fast-forward 推送到 `yunbay/main`；本地其他未提交文件保持原样。
- 当前生产仍处于 API-only 稳态，注册 worker 已清理；下一次仍需新的独立“注册”执行指令，并先通过非空配置快照与回读校验。

## 第五轮两路受控 canary 计划（2026-07-18）

用户接受的最低安全余量调整为：主机内存至少保留约 37%、CPU 至少保留 25%；本轮将用户在“两路试验”提议后的明确“可以”视为执行授权。该授权只覆盖一次性两账号 canary，不覆盖常驻 producer、自动补量或三路并发。

### 目标与不变量

- 目标：验证 `count=2 / concurrency=2` 是否能在当前 `4 vCPU / 7941 MiB` 主机上稳定完成，并保持 API 可用。
- 注册总 cgroup 硬限为 `2 CPU / 3 GiB / 440 PID`；API 继续保持原 `2 CPU / 2 GiB / 256 PID` 容器和镜像，不重建、不改配置。
- 两个注册 worker 使用适配器允许的最大错峰 10 秒，预取 0；本地 Solver、自动维护关闭、独立 Redis 前缀、`restart=no`。
- 主机 `MemAvailable < 3.0 GiB`、CPU idle `<25%` 连续两次、API unhealthy/restart/OOM、批次状态异常、配置快照为空或恢复回读不一致时，立即只停止注册侧。
- 本轮最多导入 2 个账号；任一熔断允许得到 0/1 个结果，不为追求数量重试或提限。

### 执行检查点

- [x] 用户批准两路阈值，建立本轮计划和成功/停止判据。
- [x] 复核生产基线、无残留注册任务，并审计批次响应结构和上一版 runner。
- [x] 生成最小两路 runner，完成 shell 语法、静态差异和 dry-run 检查；配置快照非空验证发生在创建容器之前。
- [x] 启动两路 canary 和 API 真实并发 probe，持续采样 CPU idle、MemAvailable、注册内存/PID、API health/restart/OOM。
- [x] 核对账号增量和新增账号测活，恢复配置并清理容器、浏览器、锁、Redis 临时键和脚本。
- [x] 更新本计划、仓库运维记录和桌面唯一服务器手册，只提交本轮文档并 fast-forward 推送 GitHub `main`。

### 第五轮实测结果

- 外部 Docker build 运行期间前置锁正确拒绝启动；build 结束且 CPU idle 恢复后，dry-run 通过，确认配置快照非空并且恢复回读一致。
- 两路 canary 运行目录：`/home/deploy/grok-backups/20260718T153251-dual-registration-canary`；硬限 `2 CPU / 3 GiB / 440 PID`。资源峰值约注册侧 `1.572 GiB / 208 PID`、API+注册合计 `2.361 GiB`、主机可用内存最低约 `4.35 GiB`。
- CPU idle 最低 `18%`，连续低于用户批准的 `25%` 阈值，watchdog 以 `host_cpu_guard` 立即停止注册侧；账号数保持 `3739`，没有导入账号，也没有提高并发重试。
- API 活跃期间真实 SSE probe 为 `5/5`，API 始终 `healthy / restart=0 / OOM=0`；配置恢复 `restored_verified`，本轮 6 个 Redis 临时键已删除为 `0`。
- 结论：当前主机在“至少保留 25% CPU”约束下，生产安全并发仍为 **1 路**。两路不是内存装不下，而是 CPU 余量不成立；需要更低的单路 CPU 配额或更多 vCPU 后才能重新评估。

## 第六轮 API 延迟反馈与自适应注册控制器（2026-07-18）

本轮把用户的两个结果目标合成一条可观测闭环：前台请求优先使用有足够新鲜反馈的低 TTFT 账号，后台注册只在宿主机和 API 都有余量时逐档增加并发。注册默认仍从 1 路开始；任何指标越线只降低注册档位或停止注册，不影响 API 容器。

### 目标与硬约束

- API：真实请求的本地 `pick + local` p95 目标小于 `500ms`；账号选择不等待全量探测，不在请求路径同步探测全部账号；上游慢必须在 `up_hdr/up_tok` 中单独可见。
- 注册：主机 `MemAvailable` 不得低于约 `3.0 GiB`（约 37%），CPU idle 不得连续低于 `25%`；API 必须保持 `healthy`、restart/OOM 不增加。
- 注册 cgroup 保留内存、PID、CPU 硬上限；预取固定为 0；批次必须有 `batch_id`，配置快照必须非空且恢复回读一致。
- 自动调档必须有冷却窗口和连续稳定样本；不允许一次性从 1 路跳到高并发，也不允许因注册失败自动重试提档。

### 控制结构

1. **API 延迟反馈**：真实 SSE 首 token 和后台模型探测首 token 分别写入按模型 EWMA；样本带时间新鲜度和成功/失败信息。选路只读取有界 ZSET 窗口，至少有 8 个不同账号反馈才启用快速窗口，每五个请求保留一次全池探索。
2. **注册资源控制器**：每个采样周期读取宿主机 CPU idle、`MemAvailable`、注册 cgroup memory/PID/OOM、API health/restart/OOM，以及 API 本地 TTFT p95。稳定窗口内按 `1 -> 2` 的固定档位试探；任一越线立即停止活动批次并退回 1 路，连续故障则保持停机冷却。
3. **安全执行器**：注册 worker 只创建一个有界 batch，维护心跳和租约；停止、超时、异常和进程退出都调用统一清理。API 与注册使用不同容器和资源配额，注册 worker 不拥有 API 端口或 `new-api` 网络。

### 实施节点

- [ ] 节点 1：在同一个代码分支补充模型探测 TTFT 反馈、延迟样本 TTL/错误惩罚和健康状态字段，增加回归测试。
- [ ] 节点 2：实现资源采样器与自适应注册档位状态机，默认 `1` 路、预取 `0`，补充越线降档、稳定升档、API 异常熔断测试。
- [ ] 节点 3：本地运行定向测试、Python 编译、Compose 校验和静态差异；构建候选镜像但不切换生产。
- [ ] 节点 4：在服务器执行只读基线和隔离 canary；先 1 路稳定窗口，再最多尝试 2 路，CPU idle/内存/API 任一越线立即停止。
- [ ] 节点 5：通过真实 SSE、账号导入、资源峰值和清理验收后，才部署 API 侧候选；自动注册保持关闭，除非新的明确执行指令批准 canary。
- [ ] 节点 6：把结果、镜像摘要、回滚目录和本地/服务器运维记录写回本计划与唯一维护手册，并只提交本轮必要文件到 `main`。

### 失败判据与回滚

- API 本地 p95 连续越过 `500ms`、CPU idle 连续低于 `25%`、`MemAvailable < 3.0 GiB`、注册 cgroup 越过软线、OOM/restart 增加、SSE 失败或批次契约异常：立即停止注册侧，保留 API 容器不动。
- 延迟路由 Redis 不可用时降级为现有轮询，不得把 Redis 故障放大为 API 故障；样本过少或过期时不启用快速窗口。
- 候选部署仅允许在有界 watchdog 和部署锁内重建 `grokcli-2api`；失败在 60 秒内恢复固定旧镜像。不得回滚 PostgreSQL、Redis、账号数据或其它服务。

## 第七轮永久 refresh 失败重试风暴修正（2026-07-19）

注册 canary 前置检查发现，API leader 持续对同一批 20 个已撤销 refresh token 发起续期：单个 API worker 长时间占用约 56% CPU，维护周期重复输出 `invalid_grant`。注册 worker 全程未启动；为立即降低负载，已通过运行时设置暂停 Token 自动续期，不停止 API、不删除账号。

### 根因假设与证据

- PostgreSQL 中 20 条账号均已持久化 `payload.refresh_invalid=true`，对应 `account_pool.enabled=false`，说明永久失败识别和禁用动作本身成功。
- `store/accounts_pg.read_auth_map()` 在缓存未命中时先释放缓存锁再读取数据库。若永久标记事务在这次读取期间提交并执行缓存失效，旧读取仍可能随后把提交前快照重新写回进程缓存；同一维护周期的 `refresh_all_accounts()` 因而再次选择已标记账号。
- PostHog 的 OAuth refresh sweep 同样把无限重试永久失败视为资源与可观测性故障：对 `invalid_grant` 使用有界重试后进入 terminal，后续 sweep 直接跳过。参考：<https://github.com/PostHog/posthog/blob/266a72955d657afc02cccef4c11b56bac56022fe/posthog/models/integration.py#L93-L177>。

### 目标与不变量

- 已提交的账号变更之后，旧数据库快照不得重新进入进程缓存；`refresh_invalid` 账号在任何后台周期都不得再次调用上游 refresh endpoint。
- 修复只收紧 PostgreSQL 全量账号缓存的一致性，不改变账号数据结构、硬删除策略、正常 token 刷新、API 选路或注册参数。
- 自动续期恢复前必须证明：竞争回归可重复通过、永久失败被跳过、API 本地 p95/5 路 SSE 正常、CPU idle 恢复到 25% 以上且无 OOM/restart。
- 注册继续保持关闭；只有 API 维护负载稳定后，才重新执行单路 canary。

### 实施节点

- [x] 节点 1：增加可控线程竞争回归，复现“旧全量读取在失效后回填缓存”，并断言下一次读取看到提交后的 `refresh_invalid`。
- [x] 节点 2：对 `accounts_pg.read_auth_map()` 做最小同步修正，确保数据库读取、缓存发布与失效之间有明确顺序；补充已标记账号不进入 refresh 候选的行为测试。
- [x] 节点 3：运行定向测试、既有 64 项相关回归、Python 编译、Compose 校验和静态差异；确认没有扩大请求路径锁范围到 Redis/上游网络。
- [x] 节点 4：构建候选镜像并使用部署锁、固定回滚镜像和服务器端 watchdog 仅重建 API 容器；其它服务不重启。
- [x] 节点 5：先保持自动续期关闭做真实 SSE/资源基线，再恢复自动续期一次；确认 20 条终态账号为 skipped、CPU 回落、正常账号可刷新。
- [ ] 节点 6：API 稳定后重跑单路注册 canary；任一 guard 越线立即只停止注册侧，随后更新唯一运维手册并提交必要文档。

### 节点 4 部署与停续期基线（2026-07-19）

- 候选镜像 `grokcli-2api:20260719-refresh-cache-c84b1f3` 在服务器端构建；镜像内 66 项回归、Python 编译和 Compose 校验通过。
- 通过 `/var/lock/grokcli-adaptive-deploy.lock` 和独立 watchdog 只重建 `grokcli-2api`。PostgreSQL、Redis、egress 及其它容器未重启；备份目录为 `/home/deploy/grok-backups/20260719T031651-refresh-cache-c84b1f3/`。
- 切换后 API `healthy / restart=0 / OOM=false`，镜像 digest 为 `sha256:129a15c06c29b3ec0aeee1ce4eede985a02f26e079444a5cb589efc5b1b9454e`。宿主机 `MemAvailable` 约 `5.59 GiB`，API 空闲约 `435 MiB / 40 PID`。
- 运行时设置仍为 `token_maintain_enabled=false`，维护线程 `running=false`；环境变量中的 `GROK2API_TOKEN_MAINTAIN=1` 只作启动默认值，不覆盖已持久化的运行时开关。注册自动维护为关闭、注册并发为 1、预取为 0。
- 新鲜 5 路真实 SSE 为 `5/5`：全部 HTTP 200、首数据帧、业务内容和 `[DONE]` 完整；API guard 收到 5 个样本，服务端 `local p95=108ms`、错误率 `0`。该基线满足恢复续期前的 API 可用性门槛。

### 节点 5 首轮恢复观察（2026-07-19）

- 通过运行中的管理 API 恢复 `token_maintain_enabled=true`；注册自动维护仍为关闭，未创建注册容器或邮箱会话。
- 首轮及后续有界批次均为正常账号 `attempted=80 / refreshed=80 / failed=0 / invalidated=0`；`purge(dry_run)` 返回 `already_invalid=20 / would_disable=0`，证明终态账号没有再次写 PG 或请求上游 refresh。
- 维护器运行期间 API 容器约 `0.5 GiB / 55 PID`，短时 CPU 峰值约 `71%`（不足 1 个主机核心）；宿主机 load 约 `0.46`，`MemAvailable` 约 `5.5 GiB`。5 路真实 SSE 再次为 `5/5`，服务端 `local p95=248ms`、错误率 `0`。
- 发现旧维护日志把 `skipped(reason=refresh_invalid)` 计入 `invalidated`，显示为 `deleted=20`；数据库账号数和 `refresh_invalid=20` 未变化，没有实际删除。下一补丁将终态跳过与真实永久失败分开统计，并让无候选空扫使用基础间隔，避免误报和不必要的 30 秒轮询。

### 节点 5 收口与最终候选（2026-07-19）

- 提交 `e7de8cf` 让已标记 `refresh_invalid` 的账号在软清理中直接 `skipped`；提交 `8035dba` 将终态跳过与真实永久失败分开计数；最终提交 `f6e87e2` 让当前空扫结果直接参与等待计算，并把 `terminal_skipped` 纳入健康摘要。服务器端 71 项完整回归及候选容器回归均通过。
- 最终镜像 `grokcli-2api:20260719-refresh-control-f6e87e2`，digest `sha256:4e4cf2954828b7d4bed3c3da035fc77dfc6e43275d63616a77b9181f7deef804`；部署备份 `/home/deploy/grok-backups/20260718T201019Z-refresh-control-f6e87e2/`，切换只重建 API，watchdog 标记为 `success`。最终容器 `f5a89c8daea5504da4220eb928ef0f84eae97665b1cb420b6fe8dbeba579a04e`，`healthy / restart=0 / OOM=false`。
- Token 维护恢复后实测日志为 `refreshed=5 attempted=5 failed=0 deleted=0 terminal_skip=20`；随后空扫为 `refreshed=0 attempted=0 failed=0 deleted=0 terminal_skip=20`，健康摘要 `next_wait=90s`。PostgreSQL 账号总数保持 `3739`，`refresh_invalid=20`、pool `enabled=3682 / disabled=57`，没有删除或重复标记。
- 模型健康后台在重启后的长 sweep 会占用维护锁；为保证 API 优先，运行时已设 `model_health_enabled=false` 且 `running=false`。Token 维护为 `enabled=true / running=true`，注册维护仍 `enabled=false / running=false`。这是有意的低资源稳态，不恢复自动注册。
- 最终维护开启状态下再跑 5 路真实 SSE 为 `5/5`，均有业务内容、finish 和 `[DONE]`；`api_guard` `local p95=334ms`、错误率 `0`。API 容器约 `441 MiB / 49 PID`，宿主机 `MemAvailable` 约 `5.6 GiB`，无 panic/fatal/OOM；Redis 在途租约键为 `0`，注册状态仅保留长期状态键，无活动 runner。
- 节点 6 仍保持未执行：本轮没有新的独立“注册”执行指令，因此不创建邮箱/session、不启动注册容器；安全注册并发继续固定为 1。只有新的明确触发才从单账号 canary 开始。
