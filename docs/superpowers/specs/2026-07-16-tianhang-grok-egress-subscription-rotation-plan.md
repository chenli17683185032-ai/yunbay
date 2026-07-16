# Tianhang Grok 出口订阅轮换计划

**日期：** 2026-07-16
**状态：** 已完成
**范围：** 仅轮换生产 `grok-egress` 使用的 Tianhang 出口；不修改 LDXP 回国代理、New API、Sub2API、本机 Clash、数据库或账号池。

## 1. 目标与性能指标

目标是让生产 Grok 上游流量停止使用旧 Tianhang 静态节点凭据，改由用户本次提供的新订阅快照中经验证的节点承载。

验收指标：

1. 新订阅能被生产同版本 `metacubex/mihomo:v1.19.28` 解析，provider 至少产生一个候选节点。
2. 正式节点必须通过 Cloudflare 连通、出口国家/地区、`cli-chat-proxy.grok.com` 预期响应和真实 Grok 模型请求四层检查。
3. 正式运行配置不再包含旧静态节点的凭据，也不引用此前的订阅地址；服务器仅保存新订阅的私有 provider 快照。
4. `grokcli-2api` 仍只通过 `GROK2API_UPSTREAM_PROXY` 代理模型上游；注册、邮箱、Turnstile、SSO、OIDC、PostgreSQL 和 Redis 保持直连。
5. 仅允许重建 `grok-egress`；正常切换窗口控制在 60 秒内。若实际模型闭环失败，立即在同一窗口恢复旧配置。
6. `grok-egress`、`grokcli-2api`、PostgreSQL、Redis、`yunbay-new-api` 和 Caddy 最终均运行正常，非目标容器的 ID、启动时间和重启次数不变。
7. 订阅地址、节点凭据、API Key、账号令牌和控制器 secret 不进入仓库、Git 历史、公开文档或验证输出。

## 2. 控制系统抽象

- **对象：** `grokcli-2api -> cli-chat-proxy.grok.com` 的上游请求链路。
- **控制器：** Mihomo `grok-egress` 的 provider、节点过滤和 `Grok-Egress` 策略组。
- **执行器：** Docker Compose 对 `grok-egress` 单服务的受控重建。
- **测量：** provider 节点数、健康延迟、出口位置、上游 HTTP 状态、真实模型响应、容器健康和日志中的实际选路节点。
- **环境与扰动：** 订阅失效、节点过期、区域限制、TLS/EOF、DNS 缓存、provider 响应格式变化和在途请求。
- **稳定性优先：** 候选先隔离验证，正式切换保留一键回滚；不借本次轮换改造账号、注册或全局网络。

## 3. 已确认生产基线

- 正式目录：`/home/deploy/grokcli-upstream-v1947`。
- 正式容器：`grokcli-upstream-grok-egress-1`，镜像 `metacubex/mihomo:v1.19.28`。
- 当前实现是单个静态节点，配置目录整体以只读方式挂载到 `/root/.config/mihomo`。
- `grokcli-2api` 只设置 `GROK2API_UPSTREAM_PROXY=http://grok-egress:7890`；没有全局 `HTTP_PROXY`、`HTTPS_PROXY` 或 `GROK2API_XAI_PROXY`。
- 当前服务器配置与本地私有恢复副本 SHA-256 一致。
- New API 渠道没有网络代理，Sub2API 代理表为空；服务器没有系统级 VPN 或透明代理。
- 工作区已有与本任务无关的修改和未跟踪文件，实施时不得暂存、覆盖或删除。

## 4. GitHub 经验与设计依据

实施前已核对 MetaCubeX 官方 GitHub 文档：

- `MetaCubeX/Meta-Docs/docs/config/proxy-providers/index.en.md`
- `MetaCubeX/Meta-Docs/docs/config/proxy-providers/content.en.md`

采用的约束：

1. 官方 provider 同时支持 `http` 和 `file`，两者均可使用独立 `health-check` 和节点过滤。
2. 候选实测发现新订阅 CDN 对生产 VPS 的 IPv4/IPv6 直连均返回 403，更换官方客户端 User-Agent 也无效。
3. 不保留旧静态节点作为 HTTP provider 的下载引导，避免新链路继续依赖旧凭据。
4. 改用官方支持的 `type: file`：受控获取新订阅快照，作为私有 provider 文件与配置一起只读挂载，因此无需改动 Compose 或 HomeDir 权限。
5. provider 使用精确或最小范围过滤；正式策略只包含实际验证通过的节点，不把未测节点自动投入生产。
6. 正式规则保持 `MATCH,Grok-Egress`，候选与生产均从日志确认实际节点，不允许静默落到 `DIRECT`。

## 5. 实施步骤

### 阶段 A：基线与备份

1. 记录目标容器 ID、镜像、启动时间、重启次数、网络、挂载、配置哈希和非目标容器状态。
2. 在 `/home/deploy/grok-backups/` 创建时间戳目录，备份正式 `config.yaml`、Compose 文件、解析后的 Compose 摘要和容器 inspect。
3. 校验备份可读且哈希与当前正式文件一致；在备份完成前不修改生产文件。

### 阶段 B：隔离候选

1. 受控获取用户提供的新订阅响应，保存为本地私有 provider 快照，权限设为 `0600`；仓库计划只记录“新订阅”，不记录明文。
2. 候选使用同一 Mihomo 镜像、独立容器名、新 `file` provider 和现有 Grok Docker 网络，不占用正式名称或端口。
3. 先用宽但受控的美国节点过滤读取节点清单，排除套餐信息、到期提示和非代理条目。
4. 对候选逐项验证：provider 成功、节点延迟、Cloudflare HTTP、出口地区、Grok 上游端点预期状态。
5. 选择通过验证的单个节点并收紧正式过滤；若没有节点通过，停止实施并保留生产原状。

### 阶段 C：生产切换

1. 生成正式私有配置：新 `file` provider、5 分钟健康检查、精确节点过滤和 `MATCH,Grok-Egress`。
2. 保持现有 Compose、镜像、资源限制、网络、健康检查和服务名不变；新 provider 快照作为同一私有目录中的 `0600` 文件只读挂载。
3. 先执行 `docker compose config -q`，再启动一次不接业务名的正式形态候选验证挂载和重启行为。
4. 原子替换服务器 `config.yaml` 并新增私有 provider 快照，然后仅执行 `docker compose ... up -d --no-deps --no-build --force-recreate grok-egress`。
5. 等待 `healthy`，确认 provider 已加载、旧静态节点不在运行配置中。

### 阶段 D：闭环验收

1. 对 `127.0.0.1:3301/health` 连续探测。
2. 使用现有服务器私有 API Key 发起最小真实 `grok-4.5` 请求，不在输出中显示 Key、账号或令牌。
3. 从 Mihomo 日志确认该请求命中新 provider 的已验证节点，而不是 `DIRECT` 或旧节点。
4. 复核 API、PostgreSQL、Redis、New API、Caddy、Sub2API 和 LDXP 代理的容器 ID/启动时间/重启次数，证明未扩大重启范围。
5. 检查切换窗口内 Grok API 与 Mihomo 日志，无新增 `panic`、`fatal`、持续 TLS/EOF、区域 403 或 `all_accounts_failed`。

## 6. 回滚

触发条件：provider 无节点、候选/正式出口不符合区域要求、真实模型请求失败、持续上游 TLS/EOF、切换超过 60 秒仍未恢复，或任何非目标服务异常。

回滚动作：

1. 恢复备份中的旧 `config.yaml` 和 Compose 文件。
2. 仅重建 `grok-egress`。
3. 确认旧静态节点重新出现在实际选路日志中，真实 Grok 请求恢复。
4. 不重启 PostgreSQL、Redis、New API、Caddy、Sub2API 或 LDXP worker。
5. 保留失败候选的脱敏诊断摘要，删除订阅响应、临时凭据和候选容器。

## 7. 本地恢复资料与 Git 收尾

1. 更新 `/Users/ethan/Desktop/云贝/服务器相关/grok-egress-private/config.yaml` 和同目录 Compose 恢复文件，保持 `0600`。
2. 只在 `/Users/ethan/Desktop/云贝/LOCAL_PRIVATE_README.md` 追加本次运维记录；不新建第二份运维手册，不记录订阅明文。
3. 在本计划中循环更新阶段状态、验证结果、生产哈希和回滚点。
4. 只暂存本任务新增的计划文件；不得暂存工作区原有修改。
5. 将本任务提交普通推送到 GitHub `main`，不 force push；清理候选容器、临时目录和本任务产生的敏感临时文件。

## 8. 实施记录

- [x] 完成生产代理清单、数据库代理清单和系统级代理基线检查。
- [x] 确认当前 Grok 静态节点来源于旧 Tianhang 配置，LDXP 回国代理为独立链路。
- [x] 核对 MetaCubeX 官方 GitHub provider 文档与当前只读 HomeDir 的冲突。
- [x] 完成生产备份：`/home/deploy/grok-backups/20260716T124512Z-tianhang-egress-rotation`。
- [x] 确认新订阅网络层响应正常，但生产 VPS 直连 CDN 固定返回 403；方案收敛为新订阅私有 `file` provider 快照。
- [x] 新订阅解析出 2 个美国候选；两者各 5 轮均通过 Cloudflare 204 和 Grok 端点 401。最终精确选择延迟更低的 `直连 US1 x1.0`。
- [x] 完成与生产同镜像、同只读挂载、同健康检查的隔离候选验证；provider 仅 1 个 alive 节点，无宿主机端口。
- [x] 第一次切换后发现生产 `.env` 仍把 `GROK2API_EGRESS_CONFIG_DIR` 指向旧 `mihomo` 目录；容器虽 healthy 但 controller 无新 provider，因此未冒充成功。
- [x] 将 `.env` 的 egress 目录持久改为 `grok-egress-private`，且除该行外的内容 SHA-256 前后一致。
- [x] 第二次切换因新 controller secret 与无鉴权 Compose 健康检查不兼容，45 秒超时后自动回滚到旧静态节点并恢复 healthy；服务没有停在半完成状态。
- [x] 最终配置沿用原有空 controller secret，同构候选健康后正式切换；仅重建 `grok-egress`，6 秒恢复 healthy，restart=0。
- [x] 最终运行时挂载为 `/home/deploy/grokcli-upstream-v1947/grok-egress-private`，旧 `US 33 AI加速 x1.0` 不在正式配置或新 provider 中，配置也不保存订阅 URL。
- [x] 出口为 `88.210.37.217` / 美国洛杉矶；Grok health 20/20、Cloudflare 204、Grok 端点 401、公网 `/api/status` 200。
- [x] 真实 `grok-4.5` 基线在切换前为 HTTP 503 / `all_accounts_failed`，最终切换后为 HTTP 200、1 个 choice、`finish_reason=stop`。
- [x] 正式日志的 Grok 请求均命中 `Grok-Egress[直连 US1 x1.0]`，`DIRECT` 命中 0，provider/configuration/panic/fatal 严重日志 0。
- [x] 最终切换窗口内 Grok API、PostgreSQL、Redis、Caddy、Sub2API、LDXP 代理/worker 和 CLI Proxy 均未重建。`yunbay-new-api` 在本任务最终切换前由并发操作两次替换，最终 healthy/restart=0，不归因于本任务。
- [x] 正式私有文件 SHA-256：`config.yaml` 为 `1cc0bffde89fc1f032fc785e71b8cdc188459adbe46bb625f55ebdfbdf10e7ec`，provider 快照为 `23efb61ab5453fd8d09146a53aebef2a2e7065e705657a56ad4e51265cacee8c`，`.env` 为 `65d41645956aa3883ea0cf97cc348c04101afc3ea970b59df0f50f8dcd20a0e1`。
- [x] 完成本地恢复资料、运维记录和临时资源清理；本计划提交作为 GitHub `main` 收尾。
