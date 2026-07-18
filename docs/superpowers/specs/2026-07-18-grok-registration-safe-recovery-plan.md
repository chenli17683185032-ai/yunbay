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
- [ ] 只提交本轮必要文件，按项目约定 fast-forward 合并并推送 GitHub `main`；不覆盖用户已有改动。

## 回滚/熔断条件

出现 API 5xx/连接失败、API 容器 unhealthy/restart/OOM、宿主机 load 持续异常、注册 cgroup 接近内存/PID 上限、批次状态缺少 `batch_id`、注册硬超时或上游连续失败时：立即停止注册 runner 和活动浏览器，释放其资源，保留 API 原容器不动；只有恢复基线后才允许再次单账号试验。

## 当前进度

- [x] 计划文件建立。
- [x] 历史成功版本和配置证据核对。
- [x] 单账号隔离 canary。
- [x] API 并发/资源观察窗口。
- [ ] 运维记录、测试和 GitHub `main` 收尾。

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
