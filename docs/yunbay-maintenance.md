# 云贝 new-api 本地维护说明

本文记录云贝 `new-api` 当前的本地维护、验证、同步生产和排障约定。

## 2026-07-20 Responses 客户端断流计费修复上线

### 故障边界与实现

- 生产证据确认：direct `/v1/responses` 流在客户端提前断开后会停止读取上游，因而错过 `response.completed.usage`；工具调用流又没有可用的文本输出回退，最终会以零 token、零 quota 结算。固定 24 小时窗口内，用户 `180`、令牌 `284`、渠道 `35`、模型 `gpt-5.6-sol` 的 53 次 `client_gone` 中有 52 次 quota 为 0。
- 功能提交 `ec4420ebdf48d5d02d3d040862c12e8c18f2fa6d` 只让 direct Responses 在**客户端已经断开后**继续有界读取上游终态 usage；正常连接的所有文本和工具调用事件仍照常写给客户端。代码没有“识别工具调用后关闭连接”的逻辑。
- 下游断开后不再向失效的 writer 写数据，但继续解析 `response.completed` 的输入、缓存和输出 token，再走原有冻结计费表达式结算。空闲超时与绝对 drain 上限同时生效，绝对上限不超过 5 分钟；其它流式渠道默认行为不变。
- 回归覆盖默认路径隔离、断流后终态消费、绝对超时、工具调用后取消及缓存 usage 精确恢复。new-api 后端包、`controller`、`service`、全部 `relay/...`、新增路径定向 race、`go vet`、`gofmt` 与 `git diff --check` 均通过；全仓既有 `infra/sub2api/backend` 独立依赖缺失不属于本轮失败。

### 固定 upstream 发布结果

- 生产原三文件与 `ec4420eb^` 逐字一致，只同步 `relay/channel/openai/relay_responses.go`、`relay/common/relay_info.go`、`relay/helper/stream_scanner.go`；组合清单 SHA-256 为 `cdfbe606df6a9d4893dbea2f1fab5f1d60122b3a45d2c676baff7a4b4a52d811`。没有数据库迁移、环境变量、Caddy 或业务配置变更。
- 成功备份：`/opt/new-api/backups/responses-client-gone-billing-20260719T175931Z-ec4420eb`。旧镜像 `sha256:b5fb68ea933c2f4f9d7fd98e0aa4da0d4e40a6662f6af56052f024ad58b3d8e4`，回滚标签 `yunbay-new-api:rollback-responses-billing-20260719T175931Z`；新镜像 `sha256:ae0a0bd862087d0698719c902ac1954895be8ed03eaa4bfc8315f94bce870d88`，release 标签 `yunbay-new-api:release-responses-billing-ec4420eb`。
- 部署锁和独立 60 秒 watchdog 全程生效，构建时旧实例持续服务，正式切换只重建标准 `new-api`。最终容器 `dff5be19d6792714bde28982a00d2bbe04f930a9c81d27a72d905ee76400bd1e` 为 `running / healthy / restart=0`，watchdog=`success`，严重启动日志为 0。
- 切换探针在约 14 秒窗口内记录 12 次 502，Caddy 同窗记录 33 次 502，随后独立 10 轮源站、首页、快速启动和公网状态共 40 次全部为 200。Caddy 文件、只读挂载与运行时配置前后哈希一致，upstream 始终只有 `new-api:3000`；Caddy、PostgreSQL、Redis、Sub2API、CLI Proxy、LDXP 与 Grok API 的容器身份、启动时间和 restart count 前后不变，无绿实例。
- `.yunbay-deploy-sha` 已原子更新为功能提交 `ec4420ebdf48d5d02d3d040862c12e8c18f2fa6d`；`.yunbay-source-manifest` 记录三文件清单、新镜像与 `2026-07-19T18:03:06Z` 部署时间。后续纯文档提交不得覆盖这个生产功能标记。

### 真实断流计费 canary

- 使用启用、无限额且无模型限制的根测试令牌 ID `6`，没有使用用户 `180` 的令牌 `284`。请求强制生成函数调用，但测试客户端在最早的 `response.created` 事件到达后立即断开，因此测试触发与工具事件识别无关；curl 以预期的写管道关闭码 `23` 退出。
- 消费日志 `126475` 为 `client_gone`，同时记录渠道 `35`、模型 `gpt-5.6-sol`、prompt `4,468`、completion `528`、quota `5,133`、`billing_source=wallet`，结算错误为 0。新 prompt 的 cache token 为 0；缓存精确恢复由带 `91,904` cached token 的回归测试覆盖。
- 根测试用户同一批次另有两条并发消费；三条 quota `8,698 + 5,133 + 13,877 = 27,708`，与钱包和 used quota 的批量变化 `27,708` 精确相等，证明 canary 的 `5,133` 已进入真实扣费闭环。canary 后 new-api 保持 healthy/restart=0，源站与公网状态 5/5 均为 200。
- 令牌明文只短暂存在于服务器权限 `0600` 的 curl 配置中，没有进入命令行、发布日志或审计结果，测试完成后已删除。审计备份只保留无凭证脚本、请求合同、非敏感结算字段和哈希。
- detached 发布日志和钱包批次对账已固化到成功备份；服务器顶层发布包、脚本、状态、日志与 run dir 以及本机一次性工件均已清理。部署锁空闲、绿实例为 0；release/rollback 标签和审计备份按回滚边界保留。

### 回滚

回滚时获取 `/var/lock/yunbay-new-api-deploy.lock`，从上述成功备份恢复 `existing-files.txt` 中的三份源码及部署标记，把 `yunbay-new-api:rollback-responses-billing-20260719T175931Z` 重标为 `yunbay-new-api:prod`，启动独立 60 秒 watchdog，并且只执行 Compose `--no-deps --force-recreate --no-build new-api`。Caddy upstream 必须始终保持 `new-api:3000`；禁止创建绿实例，禁止修改或重启 Caddy、PostgreSQL、Redis、Sub2API、CLI Proxy、LDXP、Grok API 或其它服务。本轮没有数据库结构变更。

## 2026-07-19 Quick Start 生图模型禁令扩展上线

### 发布范围与验证

- 功能提交 `e670b30cf5845c8cd9b1028ece43103137c79eef` 只把 Prompt 中的主聊天模型禁令扩展为 `gpt-image-2` 或 `gpt-image-1.5` 等生图模型；双 Images 路由、固定使用 `gpt-image-2`、Chat/Responses 禁令、动态 Key 规范化/脱敏和 `outputs/` 原图保存规则保持不变。
- 生产从 `f27f1a0536e876de3a108e1e1e1ac65f86be7d98` 基线只同步 `quick-start-cc-switch.ts` 与对应测试。两文件 SHA-256 为 `9a05322f93f475270840fa16cd085480580550ddab7f1649f66e57a8a41cf238`、`99f3281fc551cae3218bf3713cc180d2bc0dfba6b16a7f9548e5bf7dca53f65c`，组合清单为 `3f9038636f2e96966f8a5081ac8e838fd77c3f9f219b368e1c158f59c608ad49`。
- 上线前 Quick Start `54 pass / 0 fail`，TypeScript、ESLint、Prettier、`git diff --check` 和生产构建全部通过。本轮没有数据库、业务数据、环境变量、Key 或 Caddy 变更。

### 固定 upstream 发布结果

- 成功备份：`/opt/new-api/backups/quick-start-image-warning-20260719T091046Z-e670b30c`。旧镜像 `sha256:e0d981f228cad32ff910160ce9345f44fdc7b9e3ffef48f78da8570e11ae38fc`，回滚标签 `yunbay-new-api:rollback-quick-start-image-warning-20260719T091046Z`；新镜像 `sha256:b5fb68ea933c2f4f9d7fd98e0aa4da0d4e40a6662f6af56052f024ad58b3d8e4`，release 标签 `yunbay-new-api:release-quick-start-image-warning-e670b30c`。
- 构建期间旧实例持续 healthy/200；部署锁和独立 60 秒 watchdog 全程生效，只重建标准 `new-api`。最终容器 `e51d42b9e456391365b410bf785f80c68d8bddd1bdf063f9f5de340f9632c970` 为 `running / healthy / restart=0`，watchdog=`success`。
- 切换探针从 `2026-07-19T09:13:25Z` 到 `09:13:34Z` 记录 9 次 502，`09:13:35Z` 起恢复，窗口约 10 秒。随后独立 10 轮共 40 个宿主机与公网探针全部为 200；new-api/Caddy 严重启动日志均为 0。
- Caddy 文件、只读挂载和运行时配置前后哈希相同，upstream 始终只有 `new-api:3000`，无绿实例；Caddy、PostgreSQL、Redis、Sub2API、CLI Proxy、LDXP proxy/worker 的容器身份、启动时间和 restart count 均未变化。

### 生产反馈与回滚

- 公网入口 `index.cae768f6fd.js` 的 SHA-256 为 `3be9d4a432acaedf568e6151e2239e4ba62c2dcebb345113bd75ff1cfcd0287c`、字节数 `3065573`；Quick Start chunk `4963.27450303c7.js` 的 SHA-256 为 `df13a0ded0cc3ce92931d5575b9e006634dcc240fa225b155e2e04677256d38e`、字节数 `50421`，与本地构建完全一致。新模型禁令存在，旧单模型禁令和旧完整生图地址不存在，Key 与保存规则仍存在。
- `.yunbay-deploy-sha` 已更新为 `e670b30cf5845c8cd9b1028ece43103137c79eef`，`.yunbay-source-manifest` 记录两文件清单、新镜像与部署时间。完整发布日志 SHA-256 为 `67274d630fb8a2e08d2dcf7b9b28ea2c03d7343271afb381d538891538c6d3ed`，服务器和本机临时工件已清理。
- 回滚时从成功备份恢复 `existing-files.txt` 中的两文件，把固定回滚镜像重标为 `yunbay-new-api:prod`，在同一部署锁和独立 watchdog 下只重建标准 `new-api`。Caddy upstream 必须保持 `new-api:3000`；禁止重启或修改 Caddy、PostgreSQL、Redis、Sub2API、CLI Proxy、LDXP 或其它服务。

## 2026-07-19 Quick Start 生图双路由提示上线

### 发布范围与验证

- 功能提交 `f27f1a0536e876de3a108e1e1e1ac65f86be7d98` 把生图 Prompt 固定为：文生图走 `POST /v1/images/generations`，图生图、参考图、局部修改和蒙版走 `POST /v1/images/edits`；继续保留 `gpt-image-2` 主聊天模型禁令、动态 Key 规范化/脱敏和 `outputs/` 原图保存规则。
- 生产从 `9bdb977b689611a828451dfe751e784a9b9943fe` 基线只同步 `quick-start-cc-switch.ts` 与对应测试。两文件 SHA-256 为 `58c8eb7dc418661f99a36b0517234de34bb3195e543e5236ba26c90bba53a1e2`、`c36ce0dac50487459ae67c90a054bb2852da4e09498e28d71220212a53ceebf6`，组合清单为 `11df6ccdf83dfb3a9a3adc210942a24f8a16c02c3b79674686f1831b1b74c463`。
- 上线前 Quick Start 全量 `54 pass / 0 fail`、定向 `9 pass / 0 fail`，TypeScript、ESLint、Prettier、`git diff --check` 和生产构建均通过。本轮没有数据库、业务数据、环境变量、Key 或 Caddy 变更。

### 固定 upstream 发布结果

- 成功备份：`/opt/new-api/backups/quick-start-prompt-20260719T080618Z-f27f1a05`。旧镜像 `sha256:bcca48d016b6c57abf935cba06c4da4623b9549ce60afa720f33b47b0ea1848b`，回滚标签 `yunbay-new-api:rollback-quick-start-prompt-20260719T080618Z`；新镜像 `sha256:e0d981f228cad32ff910160ce9345f44fdc7b9e3ffef48f78da8570e11ae38fc`，release 标签 `yunbay-new-api:release-quick-start-prompt-f27f1a05`。
- 构建期间旧实例保持 healthy/200。部署锁和独立 60 秒 watchdog 全程生效，只重建标准 `new-api`；最终容器 `5cdac5027b0c48751a19cbb2473f450d4a697028d9513fbab21fd11f6400a388` 为 `running / healthy / restart=0`，watchdog=`success`。
- 切换探针从 `2026-07-19T08:08:52Z` 到 `08:09:02Z` 记录 9 次 502，`08:09:03Z` 起恢复，连续窗口约 11 秒。随后独立连续 10 轮共 40 个宿主机与公网探针全部为 200；new-api/Caddy 严重启动日志均为 0。
- Caddy 文件、只读挂载和运行时配置前后哈希相同，upstream 始终只有 `new-api:3000`，无绿实例；Caddy、PostgreSQL、Redis、Sub2API、CLI Proxy、LDXP proxy/worker 的容器身份、启动时间和 restart count 均未变化。

### 生产反馈与回滚

- 公网入口 `index.76b18de830.js` 的 SHA-256 为 `6cc5545a1303bd7d8d2aef152e130ef15e3dbb36792965d21ca59bd2478f0d5a`、字节数 `3065573`；Quick Start chunk `4963.1f05d9fe68.js` 的 SHA-256 为 `a3f85013d3028198915b8326b43df01061f3b564f2adb5c4f5b4a83c23f7df30`、字节数 `50384`，与本地构建完全一致。新 chunk 包含两个相对 Images 路由和 Key/保存规则，不含旧完整生图地址。
- `.yunbay-deploy-sha` 已更新为 `f27f1a0536e876de3a108e1e1e1ac65f86be7d98`，`.yunbay-source-manifest` 记录两文件清单、新镜像和部署时间。完整发布日志 SHA-256 为 `6a229f0eb72fb9f355f2a9ddacb3f10dc892f2d40f8d65b81da0336431ae256e`，服务器顶层临时工件和 run dir 已清理。
- 回滚时从成功备份恢复 `existing-files.txt` 中的两文件，把固定回滚镜像重标为 `yunbay-new-api:prod`，在同一部署锁和独立 watchdog 下只重建标准 `new-api`。Caddy upstream 必须保持 `new-api:3000`；禁止重启或修改 Caddy、PostgreSQL、Redis、Sub2API、CLI Proxy、LDXP 或其它服务。

## 2026-07-19 超值套餐自定义并发上线

### 发布范围与验证

- 功能提交 `9bdb977b689611a828451dfe751e784a9b9943fe` 将超值套餐并发从固定 `1`/`2` 选择器改为正整数输入，管理接口接受任意正整数，内存与 Redis 限流器按保存值执行且不再把大于 `2` 的值截断。
- 生产只同步 `controller/subscription.go`、`middleware/value_package.go`、`subscriptions-mutate-drawer.tsx` 和 `plan-form.ts` 4 个源码文件。生产原文件精确等于 `9bdb977b^`，没有无法解释的漂移；组合清单 SHA-256 为 `42ce05308585010fc067d8cc3d95897126145fea609a8c7c14fca43b97547d94`。
- 上线前重新通过前端 17 项定向测试、`go test ./controller ./middleware -count=1`、`bun run typecheck` 和 `bun run build`。本轮没有数据库迁移、生产套餐写入、计费请求、密钥或环境变量变更。

### 固定 upstream 发布结果

- 成功备份：`/opt/new-api/backups/value-package-concurrency-20260719T071306Z-9bdb977b`。旧镜像 `sha256:f16936c20ddf6f16eefc3c1581efd31b9586f7a1bbf8bad8df7d5e70ca089f8b`，回滚标签 `yunbay-new-api:rollback-value-package-concurrency-20260719T071306Z`；新镜像 `sha256:bcca48d016b6c57abf935cba06c4da4623b9549ce60afa720f33b47b0ea1848b`，release 标签 `yunbay-new-api:release-9bdb977b`。
- 全程持有 `/var/lock/yunbay-new-api-deploy.lock`，构建时旧实例继续服务；切换前启动独立 60 秒 watchdog，只执行 Compose `--no-deps --force-recreate --no-build new-api`。watchdog=`success`，标准容器约 13 秒恢复 healthy，切换探针记录 9 次 502 后连续 22 次 200，502 窗口约 10 秒。
- Caddy 文件、只读挂载与运行时 upstream 前后哈希一致，始终包含 `new-api:3000` 且无绿实例；Caddy、PostgreSQL、Redis、Sub2API、CLI Proxy、LDXP proxy/worker 的容器身份、restart count 和启动时间前后未变。最终 new-api/Caddy 均 healthy、restart=0，new-api `OOMKilled=false`，严重启动日志计数为 0。

### 生产反馈与回滚

- 实际生产订阅管理 chunk 从 `253.241cb7d707.js` 更新为 `253.b199721db6.js`；新 chunk SHA-256 `994f8576ca9a0d2a43d8c0a5cfc83c5dc6b79f267547be661b42d8bb4115a32b` 与本地构建完全一致，包含 `type=number`、`min=1`、`step=1` 的自定义并发输入，不再包含旧 1/2 选择器。
- 发布后额外 5 轮源站 `/api/status`、公网 `/api/status` 和首页共 15 个探针全部为 200；未鉴权管理套餐 GET 保持 HTTP 401。`.yunbay-deploy-sha` 已更新为 `9bdb977b689611a828451dfe751e784a9b9943fe`，`.yunbay-source-manifest` 记录上述 4 文件清单和新镜像。
- 回滚时从成功备份恢复 `existing-files.txt` 中的 4 个文件，把固定回滚镜像重标为 `yunbay-new-api:prod`，在同一部署锁和独立 watchdog 下只重建标准 `new-api`。Caddy upstream 必须保持 `new-api:3000`；禁止重启或修改 Caddy、PostgreSQL、Redis、Sub2API、CLI Proxy、LDXP 或其它服务。

## 2026-07-18 上游错误分类上线

### 变更与安全边界

- 本轮提交 `b119464b6e0739c7514369381056f9dc610c167f` 增加 `edge_blocked`、`auth_rejected`、`route_mismatch`、`upstream_policy`、`rate_limited` 和 `payload_too_large` 六个稳定错误码；`RelayErrorHandler` 按状态码、Content-Type、Cloudflare 标记和正文做保守分类。
- 只有边缘拦截与载荷超限显式跳过重试；429 保持可重试，401/404/策略错误继续走既有跨渠道 failover 和渠道治理逻辑。没有修改计费、渠道选择或 Key 状态。
- 定向 Go 回归、`go test -race ./service ./controller` 与 `git diff --check` 通过。全仓既有 `infra/sub2api/backend` 独立依赖缺失仍未纳入本轮回归结论。

### 首次失败与修正

- 首次发布在切换前主动失败，备份 `/opt/new-api/backups/image-error-classification-20260718T075447Z-b119464b` 状态为 `failed_pre_switch`；部署器把新增的 `service/error_classification.go` 当作既有文件执行存在性断言，因而没有构建、重建容器或切换流量。
- 部署器已拆分 `existing-files.txt` / `new-files.txt`：既有文件只恢复备份，新文件回滚时删除；前置检查增加命名日志和公网 HTTP 200 的三次短重试。该修正没有触碰生产业务文件或 Key。

### 固定 upstream 发布结果

- 成功备份：`/opt/new-api/backups/image-error-classification-20260718T080814Z-b119464b`。旧镜像 `sha256:f95c9dd5de369b8bb21674cb2763c640727f433006363168f3b484e88bb0843c`，新镜像 `sha256:f16936c20ddf6f16eefc3c1581efd31b9586f7a1bbf8bad8df7d5e70ca089f8b`；release 标签 `yunbay-new-api:release-b119464b`，回滚标签 `yunbay-new-api:rollback-image-api-20260718T080814Z`。
- 按 `/var/lock/yunbay-new-api-deploy.lock` 和独立 60 秒 watchdog，只执行 Compose 标准 `new-api` 重建；watchdog=`success`，标准容器约 12 秒恢复 `healthy`，restart=0。切换探针为 22 次 HTTP 200、8 次 HTTP 502，随后恢复连续 200，Caddy upstream 始终为 `new-api:3000`。
- Caddy、PostgreSQL、Redis、Sub2API、CLI Proxy、LDXP proxy/worker 的容器快照前后未变；没有创建 `new-api-green`、临时 Caddyfile、Caddy reload/Admin API 或全栈重启。`.yunbay-deploy-sha` 与 `.yunbay-source-manifest` 已原子更新到 `b119464b`，三文件清单 SHA-256 为 `5d77bdd03ac6081b4b0d4260b5000b04f9b219d8460657fc2cf17f446c096b90`。

### 无计费线上验收

- `/v1/models` HTTP 200 且包含 `gpt-image-2`；Chat/Responses 误用均为 HTTP 400、`invalid_request` 并提示 Images 路径；generations/edits 无效 JSON 均为 HTTP 400。响应保留 Cloudflare `CF-Ray` 与源站 `x-oneapi-request-id`；这些无计费合同探针未执行真实 generation/edit。
- 使用无效上游凭证取得真实 OpenAI 401 响应后，用本轮同一错误处理函数分类为 `auth_rejected`、`skip_retry=false`；没有把该响应注入生产渠道，因此没有触发自动禁用。五个线上无计费探针 Request-ID 在生产 `logs` 中消费记录为 `0`、额度合计为 `0`。
- 旧 Key 对应 token ID `6`、`339`、`394` 复核均为 `status=1`。本轮没有禁用、删除、轮换或切换任何 Key；备用 Key 仍只存本机钥匙串。

### 用户确认的真实生图

- 2026-07-18 16:42（Asia/Shanghai），用户确认真实生图已成功返回。这证明至少一次实际 generation 已穿过 Cloudflare、Caddy、new-api、上游渠道和客户端闭环；未在记录中保存 Key、Prompt、Base64 或图片内容。
- 多图、图生图/编辑和连续 24 小时成功率观测仍未宣称完成，继续按长期计划执行。

回滚时使用本节成功备份中的 `existing-files.txt` / `new-files.txt`、固定 rollback 标签和同一部署锁/watchdog，只重建标准 `new-api`；不要重启 Caddy、数据库、Redis、Sub2API 或其它服务。

## 2026-07-18 gpt-image-2 错误合同修正上线

### 变更与合同验收

- 提交 `0cf8db1be39f909fb73fb63ebf06dc648236ff3c` 将 relay 请求校验错误固定为 HTTP 400、`invalid_request` 并跳过重试；控制器级合同测试覆盖 `/v1/chat/completions` 与 `/v1/responses` 对 `gpt-image-2` 的误用，并断言 Images 路由提示。
- 定向回归 `go test ./common ./controller ./relay/helper ./relay/channel/openai ./setting/ratio_setting ./service ./types` 通过，`git diff --check` 通过。全仓测试的既有失败仍限于 `infra/sub2api/backend` 缺失独立生成依赖，不属于本轮改动。
- 发布前旧 Key 无计费探针为：`/v1/models` HTTP 200 且包含 `gpt-image-2`；Chat/Responses 误用均为 HTTP 500。发布后同一旧 Key 仍为启用状态，模型列表保持 200，两个误用请求均为 HTTP 400、`invalid_request`；generations/edits 的无效 JSON 均返回 JSON HTTP 400。未执行有效生图，不产生计费。

### 固定 upstream 发布结果

- 只同步 `controller/relay.go` 一个源码文件；测试文件、计划、`outputs/` 和其它用户文件未同步生产。生产镜像为 `sha256:f95c9dd5de369b8bb21674cb2763c640727f433006363168f3b484e88bb0843c`，release 标签为 `yunbay-new-api:release-0cf8db1b`。
- 部署备份为 `/opt/new-api/backups/image-api-error-contract-20260718T072829Z-0cf8db1b`，回滚标签为 `yunbay-new-api:rollback-image-api-20260718T072829Z`，旧镜像为 `sha256:2d56fc5524ba82e8d1126f00a5843dc09bf7ed66cb37a4f54569d1803c32b656`。
- 全程持有 `/var/lock/yunbay-new-api-deploy.lock`，Caddy upstream 始终为 `new-api:3000`，没有创建绿实例、修改 Caddy、重启 Caddy、PostgreSQL、Redis、Sub2API、CLI Proxy 或 LDXP。watchdog 在 60 秒硬边界内为 `success`；切换探针为 22 次 200、8 次 502，502 窗口约 8 秒，随后连续恢复 200。
- 发布后 new-api/Caddy/数据库/Redis 仍 healthy，new-api restart=0；Caddy 配置、挂载和运行时配置前后 SHA-256 一致，启动严重日志为 0。

### 凭证与后续边界

- 旧 Key 没有被切换、禁用或删除；数据库状态复核为启用，Redis 缓存未执行任何轮换动作。备用 Key 继续只保存在本机钥匙串，不切换本地客户端。
- 下一阶段只做免费 `/v1/models` 观测和错误分类；真实 generation/edit 仍须单独授权，禁止用自动重试代替计费确认。任何 Key 轮换必须先获得用户明确批准并安排双 Key 并行窗口。

## 2026-07-18 gpt-image-2 图像链路代码修复上线

### 变更与验证

- GitHub `main` 已推送提交 `5a53c82d002ce36b511d54ccaa03ae38655a037a`。本轮把 `gpt-image-2` 纳入图像模型能力与 OpenAI 渠道模型列表，补齐输入/输出/图像计费倍率；普通 Chat/Responses 请求明确拒绝图像模型，渠道测试也强制使用 Images 端点。
- 回归通过：`go test ./common ./controller ./relay/helper ./relay/channel/openai ./setting/ratio_setting ./service ./types`；`git diff --check` 通过。测试覆盖模型识别、端点归一化、Chat/Responses 拒绝、generations/edits 接受及 multipart 默认值。
- 生产只精确同步 5 个源码文件，组合清单 SHA-256 为 `1945c370823133fbb44b3bd4a02155a71e67e1763f85de067ce8f5243e7e2004`；没有同步测试、计划、`outputs/` 或其它用户文件。

### 固定 upstream 部署结果

- 部署前持有 `/var/lock/yunbay-new-api-deploy.lock`，标准 new-api/Caddy、首页和 `/api/status` 均 healthy/200；Caddy 文件、挂载和运行时 upstream 始终为 `new-api:3000`，没有创建绿实例或调用 Caddy Admin API。
- 旧镜像已固定为本次备份中的 rollback tag；新镜像为 `sha256:2d56fc5524ba82e8d1126f00a5843dc09bf7ed66cb37a4f54569d1803c32b656`，生产容器约 12 秒恢复 healthy、restart=0。切换只重建 `new-api`，Caddy、PostgreSQL、Redis、Sub2API、CLI Proxy 和 LDXP 服务未重启。
- 服务器 watchdog 在 60 秒硬边界内完成 `success`；最终源站与公网 `/api/status` 均 200，无 `new-api-green` 或残留部署进程。完整备份、源码快照、镜像标签、探针和 watchdog 证据：`/opt/new-api/backups/image-api-long-term-20260718T054153Z-5a53c82d`。

### Cloudflare 与凭证后续动作

- 安全事件已证实免费计划 Bot Management 托管规则 `Manage AI bots` 误拦 `OpenAI/Python 2.24.0`。最小 canary 已保存：规则 `OpenAI Images API canary`，ID `a2b1f205f40c4cb78791965b091207f7`，只匹配带 Authorization 的 `GET /v1/models`、`POST /v1/images/generations`、`POST /v1/images/edits`，跳过 `http_request_firewall_managed` 与 `http_request_sbfm`，保留限速、DDoS、源站鉴权和日志。初始只跳过 WAF 阶段仍被 SBFM 拦截，补齐后默认 SDK/代理/直连均 200。
- 已创建等价备用 token 并存入本机钥匙串服务 `yunbay.xyz API - gpt-image-2`，官方 OpenAI Python 2.24.0 的 `models.list()` 代理/直连均通过。旧 token 的撤销曾导致本地客户端断连，随后已恢复旧 token 为启用并删除对应 Redis 缓存；当前旧 Key 保持启用，任何再次撤销必须先得到用户确认并安排迁移窗口。
- 后续错误合同修正已在上方 `2026-07-18 gpt-image-2 错误合同修正上线` 中发布；本节保留首轮图像路由修复、Cloudflare canary 与凭证事故的原始证据。
- 回滚代码发布时继续使用上述备份、固定旧镜像和同一部署锁/watchdog，只重建 `new-api`。不要删除备份、卷或数据库数据。

## 2026-07-17 生产服务器 Docker 构建缓存清理（已完成）

### 目标与基线

- 本轮只回收可明确再生、未被运行服务使用的 Docker builder cache；不修改代码、配置、数据库、用户数据、密钥、持久卷、业务备份或带标签回滚镜像。
- 清理前根分区为 `85 GiB / 145 GiB（59%）`，可用约 `60 GiB`，inode 使用率 `9%`。
- Docker images 为 `70.45 GB`，build cache 为 `55.15 GB`，其中汇总可实际回收约 `31.78 GB`；业务目录中 backups `8.0 GiB`、releases `1.2 GiB`，均按回滚/审计资产保护。
- 实施前核对 Docker 官方 prune 文档和 GitHub 上按年龄过滤、先 dry-run/测量、排除 volumes 的生产脚本；完整依据与阶段计划见 `docs/superpowers/specs/2026-07-17-production-server-cleanup-plan.md`。

### 实施与反馈

- 获取并持有 `/var/lock/yunbay-new-api-deploy.lock`，确认无并发 build、Compose、rsync 或部署进程后开始；每轮命令均设 15 分钟硬超时。
- 第一轮只清超过 24 小时未使用的 builder cache，并保留至少 `8 GB` 缓存，实际回收 `11,054,096,384` bytes；根盘 `59% -> 52%`。
- 第一轮清理瞬时 load 上升，立即停止扩大范围；待 load 连续回落且 12 个容器身份/restart count、5 轮内外 HTTP 和严重日志复核通过后，才执行第二轮。
- 第二轮只清超过 12 小时未使用的 builder cache，继续保留至少 `8 GB`，再回收 `8,093,089,792` bytes。
- 两轮累计释放 `19,147,186,176` bytes（约 `17.83 GiB` / `19.17 GB`）；根盘最终 `67 GiB / 145 GiB（47%）`，可用约 `78 GiB`。build cache 从 `55.15 GB` 降为 `35.97 GB`，可回收量从 `31.78 GB` 降为 `12.6 GB`。

### 最终验证与保留边界

- 12 个运行容器的 ID、镜像 ID、启动时间和 restart count 全部未变；没有重启或重建 new-api、Caddy、PostgreSQL、Redis、Sub2API、CLI Proxy、LDXP 或 Grok 服务。
- 最终 load 1 分钟值为 `1.44`，systemd failed units 为 0；源站 `/api/status`、公网 `/api/status` 与首页连续 5 轮全部 200，new-api/Caddy/Sub2API/Grok API/LDXP worker 严重日志计数为 0。
- 最终 2 个 dangling images 仅约 5 小时，仍在 12 小时保护窗；7 个 exited containers 属于带挂载的旧 Grok 回滚栈。为稳定性与回滚能力主动保留这些对象及空旧网络。
- 本轮没有执行 `docker system prune -a`、image/container/network prune、`docker volume prune`、备份/发布包删除、日志截断或 sudo 权限绕过。后续只有磁盘再次超过阈值并逐项确认引用关系时，才单独计划 tagged image、旧栈或备份保留策略。

## 2026-07-16 生产部署文档纠偏计划（进行中）

### 目标与稳定性指标

- 消除把临时绿实例、临时 Caddyfile 和 Caddy Admin API 动态切流写成推荐发布/回滚路径的全部规范性文字。
- 保留历史发布实际发生过的事实，但明确标注相关路径已经废止、只供事故审计，禁止复制执行。
- 将唯一允许的 new-api 发布与回滚路径收敛为：Caddy 始终指向 `new-api:3000`，只用 Compose 有界重建标准 `new-api`，失败时在 1 分钟内回滚旧镜像。
- 文档必须覆盖部署互斥锁、切换前置检查、有界 watchdog、自动回滚条件、成功判据和清理边界，避免任务中断后生产停留在中间态。
- 本次只修正文档，不部署代码、不修改生产配置或业务数据。

### 实施节点

- [x] 全量盘点项目与桌面维护文档中的 `new-api-green`、临时 Caddyfile、Caddy 动态切流和绿实例回滚引用。
- [x] 核对 GitHub 上 Docker Compose/Caddy 的配置、健康检查和有界部署经验，确认语法校验不能替代真实 upstream 连通性反馈。
- [x] 在项目维护手册顶部建立唯一权威的部署/回滚规则，并将 502 事故后的建议从“加强蓝绿检查”改为“废止动态切流”。
- [x] 修正历史发布章节：保留已发生的部署事实，同时为旧蓝绿流程加废止标记，并把所有未来回滚指令改成标准 Compose 重建。
- [ ] 同步修正桌面唯一服务器连接手册，确保与仓库手册没有相互冲突的操作指令。
- [ ] 修正快速启动实施计划，记录 `42f8c7dd` 生产部署已在切流阶段中止，禁止继续把旧路径称为“已验证”。
- [ ] 全局搜索确认不再存在未加废止说明的绿实例/Caddy 动态切流建议；执行 Markdown 差异检查与 `git diff --check`。
- [ ] 只提交本次跟踪文档并推送 GitHub `main`；保留并忽略用户已有未跟踪文件。

### 验收条件

- 任一维护者只阅读项目手册或桌面手册，都只能得到同一条可执行生产路径。
- Caddy upstream 在部署前、中、后均固定为 `new-api:3000`，任何部署脚本不得调用 Caddy Admin API 改写主站 upstream。
- 部署任务被中断、SSH 断开或候选镜像失败时，现网最多经历标准容器的有界重建窗口，不会无限停留在临时 upstream。
- 历史文本中出现 `new-api-green` 时，邻近文字必须明确其为历史事实或已废止反例，不能被理解为当前操作手册。

## new-api 生产发布唯一允许流程（2026-07-16 起强制执行）

### 不变量与禁令

- Caddy 的文件配置和运行时配置在发布前、中、后都必须指向固定 upstream `new-api:3000`。
- 禁止为 new-api 发布或回滚创建 `new-api-green`、临时 Caddyfile，禁止通过 Caddy Admin API、`caddy reload` 或其它方式动态改写主站 upstream。
- 禁止把 `caddy validate` 当作 upstream 可用性验证。Caddy 官方文档明确说明该命令只加载并 provision 配置、不会实际启动配置；它不能替代真实 HTTP 反馈。
- 禁止用自由拼装的 `docker run` 候选容器承担生产流量，也禁止执行全栈 `docker compose down/up`。
- 历史章节中的绿实例和 Caddy 动态切流只用于事故审计；即使某次历史发布成功，也不代表该路径仍获准复用。

### 强制闭环

1. 在同步、构建、重标记镜像或重建容器前，必须使用 `flock -n /var/lock/yunbay-new-api-deploy.lock` 获取单次部署锁；获取失败立即退出，不等待、不并发部署。
2. 前置检查必须同时确认：标准 `yunbay-new-api` 为 healthy、Caddy 文件与运行时 upstream 均为 `new-api:3000`、首页和 `/api/status` 为 200。任一不满足都停止发布并先处理故障。
3. 以当前**正在运行的容器镜像 ID**建立不可变回滚标签并验证标签可解析；不得只信任可变的 `yunbay-new-api:prod` 标签，因为失败的构建可能已经移动该标签。
4. 构建和静态验证期间保持旧容器服务。正式切换只允许执行 Compose 标准服务重建：`up -d --no-deps --force-recreate --no-build new-api`，不修改或重启 Caddy、PostgreSQL、Redis、Sub2API 及其它无关服务。
5. 切换前必须启动独立于 SSH 会话的服务器端 watchdog。watchdog 每秒检查容器健康、宿主机 `/api/status` 和公网 `/api/status`；标准服务在 45 秒内未恢复，或出现 exited/dead/unhealthy，必须自动把固定回滚镜像重新标记为 `prod` 并再次只重建 `new-api`。watchdog 自身最迟 60 秒结束，禁止无界等待。
6. 只有 new-api healthy、Caddy healthy、首页与 `/api/status` 连续 5 次为 200、Caddy 运行时 upstream 仍为 `new-api:3000` 时才算发布成功。随后才能更新 `.yunbay-deploy-sha`、`.yunbay-source-manifest` 和运维记录。
7. SSH 断开、操作者进程退出或验证失败都不得留下中间态；watchdog 必须完成自动回滚和失败退出。清理只限失败候选、临时环境与探针文件，不删除卷、数据库、回滚镜像或用户文件。

### 回滚规则

- 源码需要回退时，先按对应备份的 `existing-files.txt` / `new-files.txt` 恢复，再把预先固定的旧镜像标记为 `yunbay-new-api:prod`。
- 回滚与发布使用同一个部署锁、同一个 60 秒 watchdog 和同一条 Compose 标准服务重建路径；Caddy upstream 始终保持 `new-api:3000`。
- 回滚成功判据与发布相同。未连续通过健康反馈前，不得更新生产标记或宣布完成。

### 参考经验

- Caddy 官方命令文档：`caddy validate` 不会实际启动待验证配置，不能证明 upstream DNS 或 HTTP 可达：<https://github.com/caddyserver/website/blob/ea5f20c9442e9b3a96adae0ce546c8911ddcfb0c/src/docs/markdown/command-line.md#caddy-validate>。
- Suna 的自托管更新器使用 `flock` 保证单飞、有限健康等待，并在新实例失败时保留旧实例：<https://github.com/kortix-ai/suna/blob/f5533c148994e39e295bf153a11a5fa0313299a2/apps/cli/src/self-host/assets/updater.sh>。
- Super Productivity 的服务脚本同时检查容器状态与外部 HTTP，并用锁避免监控重叠：<https://github.com/super-productivity/super-productivity/blob/6f88775ea24a22d15deaa76fe6516e923394e033/packages/super-sync-server/scripts/health-alert.sh>。

## 2026-07-16 生产 502 故障处置与恢复记录（已完成）

### 目标与性能指标

- 首要目标：恢复 `https://yunbay.xyz/` 与 `https://yunbay.xyz/api/status`，公开入口返回 HTTP 200。
- 稳定性优先：先识别故障边界，再执行最小充分恢复动作；不执行全栈 `compose down/up`。
- 中断约束：若需切换或重启，仅操作故障容器，预期额外中断不超过 1 分钟。
- 数据约束：不修改生产业务数据，不输出 secret，不删除现有容器、卷、镜像或备份。

### 闭环排障与恢复步骤

- [x] 从客户端复现故障并确认公开入口返回 502。
- [x] 通过受限 SSH 登录生产服务器，采集主机负载、磁盘、内存和 Docker 状态。
- [x] 检查 `yunbay-caddy`、`yunbay-new-api`、PostgreSQL、Redis 的健康状态和有界日志。
- [x] 对照 Caddy 上游连通性与 new-api 本地健康端点，定位对象、测量链路或执行链路故障。
- [x] 执行最小恢复动作：热重载 Caddy 正式配置，没有重启任何容器。
- [x] 公开入口连续复测，确认首页和 `/api/status` 返回 200，未鉴权 `/v1/models` 返回预期 401。
- [x] 观察容器健康状态和错误日志，确认无持续重启、资源饱和或依赖断连。
- [x] 将根因、恢复动作、停机时长和后续预防措施更新到本节及唯一服务器连接手册。

### 根因、恢复与验证

- 故障窗口：Caddy 首次出现 `lookup new-api-green` 错误为 `2026-07-16 13:53:47 +08:00`；`14:04:17 +08:00` 完成热重载，公开入口恢复，持续约 10 分 30 秒。
- new-api 本体、PostgreSQL、Redis 始终 healthy，宿主机 `127.0.0.1:3000/api/status` 始终返回 200；故障对象仅为 Caddy 运行时路由。
- 正式 `/opt/new-api/app/Caddyfile` 与容器内挂载文件均指向 `new-api:3000`，但 Caddy Admin API 中的运行时配置被留在 `new-api-green:3000`。
- 临时绿实例在 `13:52:39 +08:00` 启动，但其 Docker 网络 DNS 名称中没有 `new-api-green` 别名；Caddy 因此持续收到 Docker DNS `server misbehaving` 并对外返回 502。
- 恢复动作：先验证正式 Caddyfile，再执行 `caddy reload` 热重载，运行时 upstream 立即恢复为 `new-api:3000`；没有重启 Caddy、new-api、数据库、Redis 或其它服务，额外切换中断低于 1 秒。
- 清理动作：确认没有部署进程仍在运行且正式实例持续 healthy 后，删除未承载流量的失败候选容器 `yunbay-new-api-green`，避免后续名称冲突。
- 恢复后 Caddy 为 healthy、restart=0，new-api 为 running/healthy、restart=0；首页与 `/api/status` 连续多轮返回 200，恢复后 Caddy 没有新增代理错误。
- 本事故之后正式废止 blue/green 与 Caddy 动态切流；后续发布和回滚只能使用上方“生产发布唯一允许流程”，Caddy 始终保持 `new-api:3000`。
- 本地工作区已有与本次事故无关的未跟踪文件，本次处置未清理、未覆盖。

## 2026-07-12 模型价格同步手动编辑上线

本轮将模型价格同步预览中的手动编辑能力发布到生产。运维记录只包含可复现的代码、构建和运行状态，不记录 SSH 私钥、后台会话、API Key 或生产环境变量明文。

### 发布内容

- 发布提交：`f0a2b58d8bd75f5c9c61f8bf8f6ef8783f377101`（`feat allow editing synced model prices`）。
- 模型价格同步预览的每一行增加最终价格编辑入口，可修改输入、输出、缓存、图片、音频、推理和 Web 搜索价格。
- 没有官方或 OpenRouter 匹配价格、原先会被跳过的模型，也可以通过手动填写价格转为可保存状态。
- 后端保存时使用前端提交的手动覆盖价格重新生成计费表达式，而不是只修改前端显示值。

### 验证与部署

- 部署前通过：`go test ./service ./controller`、`bun run typecheck`、`git diff --check`。
- 使用提交归档生成独立源码发布包，生产端校验 SHA-256 和 `.yunbay-source-manifest` 后，从独立 release 目录构建候选镜像。
- 候选镜像：`sha256:70bf3b9003f5c38f669c6a62223c121d934e64265f66001956bc0b481b81ce0d`。
- 只切换 `yunbay-new-api`，未重启 Caddy、PostgreSQL、Redis、Sub2API 或 LDXP Worker。
- 使用服务器端独立 watchdog 和有界回滚脚本；本次切换记录的公开入口中断时间为 `8` 秒，未触发回滚。
- 上线后 `yunbay-new-api` 与 `yunbay-caddy` 均为 `healthy`，`https://yunbay.xyz/` 和 `https://yunbay.xyz/api/status` 均返回 HTTP `200`。
- 已从生产实际提供的 JavaScript 资产中确认 `overrides`、`Preview sync`、`web_search` 和 `No OpenRouter match` 标记存在。

### 生产记录与回滚

```text
生产部署标记：f0a2b58d8bd75f5c9c61f8bf8f6ef8783f377101
生产备份目录：/opt/new-api/backups/model-price-manual-edit-20260712T090108Z-f0a2b58d
生产容器镜像：sha256:70bf3b9003f5c38f669c6a62223c121d934e64265f66001956bc0b481b81ce0d
```

回滚时使用上述备份目录中的 `rollback.sh` 和固定回滚镜像；不要执行全栈 `compose down/up`，也不要无限等待旧容器排空连接。生产 `.yunbay-deploy-sha` 与 `.yunbay-source-manifest` 当前均指向本轮发布提交。

## 当前架构结论（2026-06-23）

当前生产目标已经收敛为：

```text
New API = 云贝主系统
Sub2API = 独立上游服务（普通 OpenAI-compatible 渠道）
```

也就是说：

- `new-api` 使用**原生渠道管理**；
- 不再使用 `Channel Console` / `Cliproxy` / `Sub2API 官方深嵌 adapter` 作为主链路；
- `Sub2API` 保留独立后台与独立登录域名；
- `new-api` 只把 `Sub2API` 当作一个普通上游渠道。

## 项目位置

本地项目根目录：

```text
/Users/ethan/Desktop/云贝/云贝网站/new-api
```

服务器资料目录：

```text
/Users/ethan/Desktop/云贝/服务器相关
```

生产源码目录：

```text
/opt/new-api/app
```

生产站点：

```text
https://yunbay.xyz
```

Sub2API 后台入口：

```text
https://sub2api.yunbay.xyz
https://sub2api.13.140.180.223.sslip.io
```

## 当前工作区原则

- 当前工作区长期存在未提交但已同步生产的修复，**不要**执行：
  - `git reset --hard`
  - `git clean -fd`
  - `rsync --delete`
- 同步生产继续使用**非删除式 rsync**。
- 不要输出或记录任何完整 secret、API key、cookie、支付密钥、管理密钥。
- 不要删除或改掉 `new-api` / `QuantumNous` 的归属信息。
- 本地 Docker daemon 不可用时，不要把本地 Docker 构建作为必要验证项。

## 生产核心容器（当前期望）

```text
yunbay-new-api
yunbay-caddy
yunbay-postgres
yunbay-redis
yunbay-sub2api
```

说明：

- `yunbay-cli-proxy-api` 不再是主链路依赖；
- 若生产仍残留该容器或相关文件，应视为历史兼容物，不应继续对外暴露管理入口。

## Caddy 当前期望配置

当前生产 `Caddyfile` 应至少包含：

```caddyfile
yunbay.xyz, www.yunbay.xyz {
    encode zstd gzip
    import security_headers

    reverse_proxy new-api:3000
}

sub2api.yunbay.xyz, sub2api.13.140.180.223.sslip.io {
    encode zstd gzip
    import security_headers

    reverse_proxy sub2api:8080
}
```

当前不应继续保留：

```text
cliproxy.yunbay.xyz
/cliproxy-admin
/v0/management
```

### 重要注意

生产 `docker-compose.prod.yml` 中，`caddy` 使用的是：

```text
./Caddyfile:/etc/caddy/Caddyfile:ro
```

这是**单文件只读挂载**。如果宿主机 `Caddyfile` 被替换 inode（例如 `rsync` 覆盖），运行中的 `caddy` 容器不一定立刻看到新内容。

因此：

- 更新 `Caddyfile` 后，优先执行：

```bash
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml up -d --force-recreate caddy
```

而不是只假设 `reload` 一定能读到新文件。

## 本地 Go 工具

本机没有系统 `go/gofmt` 时，使用：

```bash
/Users/ethan/Desktop/云贝/云贝网站/new-api/.worktrees/sub2api-gateway/.toolchains/go1.25.1/bin/go
/Users/ethan/Desktop/云贝/云贝网站/new-api/.worktrees/sub2api-gateway/.toolchains/go1.25.1/bin/gofmt
```

常用验证：

```bash
cd /Users/ethan/Desktop/云贝/云贝网站/new-api
./.worktrees/sub2api-gateway/.toolchains/go1.25.1/bin/gofmt -w <changed-go-files>
./.worktrees/sub2api-gateway/.toolchains/go1.25.1/bin/go test ./common ./middleware ./relay ./service ./controller ./router ./model -count=1
```

## 前端验证

在默认前端目录：

```bash
cd /Users/ethan/Desktop/云贝/云贝网站/new-api/web/default
npm run typecheck
npm run build
```

## 非删除式同步生产

如果只是同步少量关键文件，优先**精确 rsync 文件列表**；如果必须同步目录，也要明确排除：

```bash
cd /Users/ethan/Desktop/云贝/云贝网站/new-api
rsync -az \
  --exclude='.git/' \
  --exclude='node_modules/' \
  --exclude='web/default/node_modules/' \
  --exclude='web/default/dist/' \
  --exclude='logs/' \
  --exclude='tmp/' \
  --exclude='.DS_Store' \
  -e "ssh -i '/Users/ethan/Desktop/云贝/服务器相关/ssh/newapi_vps_ed25519' -o IdentitiesOnly=yes -o UserKnownHostsFile='/Users/ethan/Desktop/云贝/服务器相关/ssh/known_hosts' -o StrictHostKeyChecking=yes" \
  /Users/ethan/Desktop/云贝/云贝网站/new-api/ \
  deploy@13.140.180.223:/opt/new-api/app/
```

## 生产构建与启动

```bash
ssh -i '/Users/ethan/Desktop/云贝/服务器相关/ssh/newapi_vps_ed25519' \
  -o IdentitiesOnly=yes \
  -o UserKnownHostsFile='/Users/ethan/Desktop/云贝/服务器相关/ssh/known_hosts' \
  -o StrictHostKeyChecking=yes \
  deploy@13.140.180.223 '
set -e
cd /opt/new-api/app
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml build new-api
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml up -d new-api
'
```

若 `Caddyfile` 有变化，再额外执行：

```bash
ssh -i '/Users/ethan/Desktop/云贝/服务器相关/ssh/newapi_vps_ed25519' \
  -o IdentitiesOnly=yes \
  -o UserKnownHostsFile='/Users/ethan/Desktop/云贝/服务器相关/ssh/known_hosts' \
  -o StrictHostKeyChecking=yes \
  deploy@13.140.180.223 '
set -e
cd /opt/new-api/app
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml up -d --force-recreate caddy
'
```

## 生产冒烟检查

```bash
ssh -i '/Users/ethan/Desktop/云贝/服务器相关/ssh/newapi_vps_ed25519' \
  -o IdentitiesOnly=yes \
  -o UserKnownHostsFile='/Users/ethan/Desktop/云贝/服务器相关/ssh/known_hosts' \
  -o StrictHostKeyChecking=yes \
  deploy@13.140.180.223 '
set -e
cd /opt/new-api/app
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml ps new-api caddy
curl -sS -L -o /dev/null -w "yunbay_home=%{http_code}\n" https://yunbay.xyz/
curl -sS -L -o /dev/null -w "yunbay_status=%{http_code}\n" https://yunbay.xyz/api/status
curl -sS -L -o /dev/null -w "sub2api_login=%{http_code}\n" https://sub2api.yunbay.xyz/login
curl -sS -L -o /dev/null -w "sub2api_root=%{http_code}\n" https://sub2api.yunbay.xyz/
'
```

当前期望：

```text
yunbay-new-api healthy
yunbay-caddy healthy
yunbay_home=200
yunbay_status=200
sub2api_login=200
sub2api_root=200
```

## 当前业务要求

### New API

保留：

- 原生渠道管理
- 原生模型 / 价格设置
- 原生日志
- 钱包 / 充值 / 兑换码
- 支付合规确认

不再保留为主链路：

- Channel Console
- Cliproxy 管理入口
- Sub2API 深嵌 billing / reconcile / usage truth adapter

### Sub2API

保留：

- 独立服务
- 独立数据库
- 独立后台登录域名
- `/health`
- `/v1/models`
- `/v1/chat/completions`

角色：

```text
普通 OpenAI-compatible 上游
```

### 新增普通渠道时的建议

在 New API 后台新增普通渠道时：

- 类型：OpenAI-compatible / OpenAI
- Base URL：
  - 若容器网络打通：`http://sub2api:8080`
  - 否则直接用：`https://sub2api.yunbay.xyz`
- Key：Sub2API 的普通 API Key，不是 admin key

## 2026-06-24 快速启动与生产部署维护记录

本节记录 2026-06-24 对云贝网站快速启动流程、下载引导和生产部署方式的维护基线。此处只记录可公开的代码与运维事实，不记录后台密码、SSH 私钥、VNC 密码、API key、cookie 或 session。

### 网站代码变更基线

目标仓库：

```text
chenli17683185032-ai/yunbay
```

当前已推送到 `main` 的相关提交：

```text
da64a13b fix: prefer actual quick start api key group
dd2d4422 fix: use available quick start api key group
cd6b2e65 fix: polish quick start onboarding
592887c1 fix: localize quick start and macos download help
cb7ea3c9 fix: replace macos codex download bundle
```

当前快速启动页面的预期行为：

- 模型页只使用后端真实模型广场 / pricing 数据，不再前端合成 `claude` / `gemini` 等 fallback 模型。
- 模型默认选择 `GPT-5.5`；如果后端模型广场没有 `GPT-5.5`，默认选择后端返回的第一个模型。
- 一键生成 API key 前会实时刷新 `/api/user/self`，再读取 `/api/user/self/groups`。
- API key 分组选择顺序为：
  1. 当前用户真实分组（例如 `plus`），且该分组在当前用户可用分组中；
  2. 站点启用 `default_use_auto_group` 且当前用户可选 `auto` 时，使用 `auto` 并开启 `cross_group_retry`；
  3. 第一个非 `default` 的用户可用分组；
  4. 只有没有其它可用分组时才使用 `default`；
  5. 没有任何可用分组时直接报错，不创建不可用 key。
- 兑换码页面支持在快速启动页内直接粘贴并兑换，不跳转到控制台兑换码页面。
- 新手引导 2~5 页中文文案已补齐，不应回退到英文界面。
- 下载页新增与当前视觉风格协调的 `CC Switch` 小窗口式导入栏，展示：
  1. 当前站点 API endpoint（自动规范化为 `<server>/v1`）；
  2. 当前已选模型；
  3. 当前已生成 API Key 的脱敏值。
- 只要用户点击一次 `一键导入`，浏览器就会直接尝试打开 `ccswitch://v1/import?...`，把 `app=codex`、`name=Yunbay Codex`、`endpoint`、`apiKey`、`model`、`homepage` 与 `enabled=true` 一并传给本地 CC Switch。
- 若尚未生成 API Key，导入按钮禁用并提示先生成 API Key；若没有可用模型，则提示未选择模型。
- 主页宣传标语已去掉“不封号”。
- macOS 下载入口指向云贝 Codex 构建产物；由于当前没有 Apple Developer ID / notarization，如 Gatekeeper 提示 App 损坏，引导用户使用页面中的 `xattr` 修复命令。

### 关键源码位置

```text
web/default/src/features/quick-start/index.tsx
web/default/src/features/quick-start/quick-start-api-key.ts
web/default/src/features/quick-start/quick-start-api-key.test.ts
web/default/src/features/quick-start/quick-start-data.ts
web/default/src/features/quick-start/quick-start-redemption.ts
web/default/src/features/quick-start/quick-start-cc-switch.ts
web/default/src/features/quick-start/quick-start-cc-switch.test.ts
web/default/src/i18n/locales/{en,zh,fr,ru,ja,vi}.json
web/default/src/components/layout/config/public-landing-brand.ts
```

### 本地验证命令

默认前端目录：

```bash
cd /Users/ethan/Desktop/云贝/云贝网站/new-api/web/default
```

快速启动相关测试：

```bash
npx --yes tsx --test \
  src/features/quick-start/quick-start-api-key.test.ts \
  src/features/quick-start/quick-start-data.test.ts \
  src/features/quick-start/quick-start-redemption.test.ts \
  src/features/quick-start/quick-start-locales.test.ts \
  src/features/quick-start/quick-start-cc-switch.test.ts \
  src/components/layout/config/public-landing-brand.test.ts \
  src/i18n/public-landing-locales.test.ts
```

2026-06-25 验证结果：

```text
32 tests
32 pass
0 fail
```

类型检查和构建：

```bash
node /Users/ethan/Documents/yunbay/web/default/node_modules/typescript/bin/tsc -b
node /Users/ethan/Documents/yunbay/web/default/node_modules/@rsbuild/core/bin/rsbuild.js build
```

2026-06-25 验证结果：均通过。

2026-06-24 验证结果：二者均通过。

### 生产部署注意事项

2026-06-24 生产机 `/opt/new-api/app/.git` 被确认是一个失效的 worktree 指针：

```text
gitdir: /Users/ethan/Desktop/云贝/云贝网站/new-api/.git/worktrees/sub2api-gateway
```

因此生产机上不要依赖：

```bash
git pull
git status
git rev-parse
```

当前建议：

- 少量补丁优先使用精确文件列表 `rsync` 到 `/opt/new-api/app`；
- 大范围同步继续使用非删除式 `rsync`，并排除 `.git/`、`node_modules/`、`dist/`、运行日志和数据目录；
- 同步后在生产机执行 `docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml build new-api`；
- 重启时优先使用 `up -d --force-recreate new-api`，确保新镜像被容器实际采用。

2026-06-24 最后一轮 API key 分组补丁只同步了以下 3 个文件：

```text
web/default/src/features/quick-start/index.tsx
web/default/src/features/quick-start/quick-start-api-key.ts
web/default/src/features/quick-start/quick-start-api-key.test.ts
```

生产重建后状态：

```text
yunbay-new-api: healthy
GET https://yunbay.xyz/quick-start: HTTP 200
当前入口 JS: /static/js/index.e8f3d7a543.js
```


## 2026-06-26 快速启动 Windows 安装包恢复与生产同步记录

本节记录 2026-06-26 对 `codex/quick-start-cc-switch-import` 分支的补丁维护与生产同步。记录内容只包含可公开的代码、构建、部署和验证事实，不记录 SSH 私钥、后台密码、API key、cookie、session 或服务器 secret。

### 背景

`a5b68752 fix: simplify yunbay console and codex downloads` 曾把 Windows 下载恢复为站内托管的 Yunbay Codex `.exe`：

```text
/downloads/yunbay-codex-windows-20260625-030300-f5121184b049.exe
```

后续 `codex/quick-start-cc-switch-import` 分支线引入 CC Switch 导入功能时，快速启动 Windows 卡片又回到了 Microsoft Store 下载地址，导致生产页面按钮不再下载用户上传的 Windows 安装包。

本轮采用最小补丁方式修复：只恢复 Windows `.exe` 下载相关资产、数据、页面渲染、i18n 和回归测试，同时保留 CC Switch 一键导入功能；没有整条 cherry-pick 旧提交，也没有触碰后端、钱包、支付、认证、sidebar/top-nav 或 dashboard 行为。

### GitHub 分支与提交

```text
repo:   chenli17683185032-ai/yunbay
branch: codex/quick-start-cc-switch-import
commit: 470337ca fix: restore quick start Windows launcher download
PR:     https://github.com/chenli17683185032-ai/yunbay/pull/2
```

关键文件：

```text
web/default/public/downloads/yunbay-codex-windows-20260625-030300-f5121184b049.exe
web/default/src/features/quick-start/index.tsx
web/default/src/features/quick-start/quick-start-data.ts
web/default/src/features/quick-start/quick-start-data.test.ts
web/default/src/features/quick-start/quick-start-locales.test.ts
web/default/src/i18n/locales/{en,zh,fr,ja,ru,vi}.json
```

Windows 安装包 SHA256：

```text
f5121184b0496cd978eb32f97d1def4a2dc7cbb2cc997189ee428fcd8c9fc5da
```

### 本地验证

验证目录：

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/quick-start-cc-switch-import/web/default
```

执行过的检查：

```bash
bun test src/features/quick-start/quick-start-data.test.ts src/features/quick-start/quick-start-locales.test.ts
bun run typecheck
bun run build
```

结果：

```text
quick-start tests: 13 pass / 0 fail
typecheck: pass
build: pass
```

本地构建产物中的 Windows 安装包 hash 与源文件一致：

```text
f5121184b0496cd978eb32f97d1def4a2dc7cbb2cc997189ee428fcd8c9fc5da
```

### 生产同步方式

生产 `/opt/new-api/app` 不是可信 git checkout，因此本轮没有在服务器上执行 `git pull`。同步方式为：

1. 在生产机备份将要覆盖的前端文件、`Dockerfile`、`docker-compose.prod.yml` 和旧 `yunbay-new-api:prod` 镜像 tag；
2. 使用精确文件列表 `rsync --files-from` 只覆盖本次补丁涉及文件；
3. 在生产机重新构建 `yunbay-new-api:prod`；
4. 仅重启 `yunbay-new-api`，不重启数据库、Redis、Sub2API 或 Caddy。

生产备份目录：

```text
/opt/new-api/backups/quick-start-windows-predeploy-20260626-230034
```

生产构建和重启命令：

```bash
cd /opt/new-api/app
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml build new-api
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml up -d new-api
```

### 生产验证结果

生产源码同步后，`quick-start-data.ts` 中 Windows 下载路径为：

```text
/downloads/yunbay-codex-windows-20260625-030300-f5121184b049.exe
```

生产机文件 hash：

```text
f5121184b0496cd978eb32f97d1def4a2dc7cbb2cc997189ee428fcd8c9fc5da
```


生产容器状态：

```text
yunbay-new-api: healthy
```

生产 HTML 已切换到新入口 bundle：

```text
/static/js/index.87fd8198e3.js
```

生产页面已由用户实际确认 Windows 下载恢复正常。

### 回滚方式

若后续需要回滚本轮生产同步，优先使用备份目录恢复本轮覆盖的前端文件，然后重建并重启 `new-api`：

```bash
cd /opt/new-api/app
cp -a /opt/new-api/backups/quick-start-windows-predeploy-20260626-230034/files/web/default/src/features/quick-start/index.tsx web/default/src/features/quick-start/index.tsx
cp -a /opt/new-api/backups/quick-start-windows-predeploy-20260626-230034/files/web/default/src/features/quick-start/quick-start-data.ts web/default/src/features/quick-start/quick-start-data.ts
cp -a /opt/new-api/backups/quick-start-windows-predeploy-20260626-230034/files/web/default/src/features/quick-start/quick-start-data.test.ts web/default/src/features/quick-start/quick-start-data.test.ts
cp -a /opt/new-api/backups/quick-start-windows-predeploy-20260626-230034/files/web/default/src/features/quick-start/quick-start-locales.test.ts web/default/src/features/quick-start/quick-start-locales.test.ts
cp -a /opt/new-api/backups/quick-start-windows-predeploy-20260626-230034/files/web/default/src/i18n/locales/*.json web/default/src/i18n/locales/
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml build new-api
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml up -d new-api
```

也可回滚到镜像 tag：

```text
yunbay-new-api:prod-pre-quick-start-20260626-230034
```

## 结论

当前维护基线是：

- **不要再回到 Channel Console / Cliproxy / Sub2API 深嵌 adapter 方案**；
- 只维护 `new-api` 原生渠道管理 + `sub2api` 独立上游模式；
- 涉及 `Caddyfile` 改动时，要记得 **force-recreate caddy**，不要只靠宿主机文件覆盖后假设容器会自动看到。


## 2026-06-27 邮件投递切换：Resend SMTP + Cloudflare Routing

当前生产邮件架构：

```text
出站系统邮件：yunbay-new-api -> Resend SMTP -> 用户邮箱
入站/回复邮件：support@yunbay.xyz -> Cloudflare Email Routing -> 10256345@qq.com
```

当前生产 SMTP 摘要：

```text
SMTPServer=smtp.resend.com
SMTPPort=465
SMTPSSLEnabled=true
SMTPAccount=resend
SMTPFrom=support@yunbay.xyz
SystemName=yunbay
```

`SMTPToken` 为 Resend API Key，只记录为 `SET`，不要写入仓库文档、GitHub issue、PR、聊天记录或公开日志。

切换原因：历史 QQ SMTP 发信会在收件端暴露 QQ 昵称；当前出站邮件改为正式域名邮箱身份 `yunbay <support@yunbay.xyz>`。

已验证：

- 线上服务器直连 Resend SMTP 测试通过：`SMTP_TEST_OK from=support@yunbay.xyz to=10256345@qq.com`。
- 云贝应用层密码重置接口测试返回：`http_status=200`、`success=true`。
- `yunbay-new-api` 重启后为 healthy。
- 最近日志未见 SMTP / TLS / AUTH / Resend / 501 / `failed to send` 相关错误。

切换前备份保存在生产服务器：

```text
/root/yunbay-smtp-backups/smtp-before-resend-20260627-114705.tsv
```

回滚时不要在终端或文档中打印备份内容。恢复 `SMTP*` 与 `SystemName` 相关 options 后，重启 `yunbay-new-api` 并重新走密码重置或邮箱验证码接口验证发信。


详细公开说明见：`docs/email-delivery.md`。

## 2026-06-27 用户标签与模型分组后台管理维护

本轮用户体系拆分后，后台需要严格区分两类“分组”：

```text
users.group      = 用户标签 / 用户等级
                  例如：体验用户、vip

tokens.group     = Token 使用的模型分组
abilities.group  = 模型能力所属模型分组
                  例如：gpt-plus、gpt-pro
```

### 管理后台入口约定

- 用户管理页编辑用户时，修改的是 `users.group`，下拉选项必须来自用户标签接口：

```text
GET /api/user/group-tags
```

- 当前云贝用户标签选项：

```text
体验用户
vip（前端展示为 VIP 用户）
```

- Token/API Key 创建、渠道、模型能力、倍率设置仍然使用模型分组接口：

```text
GET /api/group/
GET /api/user/self/groups
```

不要把 `/api/group/` 接到用户管理的 `users.group` 编辑框，否则后台会只显示 `gpt-plus` / `gpt-pro`，并可能把模型分组错误写入用户标签字段。

### 前端维护点

- default 后台用户编辑：`web/default/src/features/users/components/users-mutate-drawer.tsx`
- default 用户标签选项 helper：`web/default/src/features/users/lib/user-group-tags.ts`
- classic 用户列表筛选：`web/classic/src/hooks/users/useUsersData.jsx`
- classic 用户编辑弹窗：`web/classic/src/components/table/users/modals/EditUserModal.jsx`

编辑 admin/root 等历史用户时，如果当前 `users.group` 是 `default` 或其他非标准值，前端会把当前值临时放到下拉第一项，避免打开编辑框后丢失现有值。普通用户应使用 `体验用户` 或 `vip`。

### 生产数据期望

普通用户不应再保留 `default` 用户标签；Token 不应再保留空值、`default`、`体验用户` 等非模型分组值。生产验证可用以下断言思路：

```text
common_default_users = 0
illegal_token_groups = 0
aff_count_mismatch_count = 0
```

其中 token 合法模型分组目前仅为：

```text
gpt-plus
gpt-pro
```

### 本地验证建议

```bash
/Users/ethan/.cache/codex-go/go1.25.1/bin/go test ./controller -run TestGetUserGroupTagsReturnsUserTagsNotModelGroups -count=1
cd web/default && bun test src/features/users/lib/user-group-tags.test.ts
cd web/default && bun run typecheck && bun run build
cd web/classic && bun run build
```

生产同步仍按本文“非删除式同步生产”章节执行，不要在服务器上依赖失效 worktree 的 `git pull`。



## 2026-06-28 LDXP 卡密兑换与充值统计生产维护

本轮上线 LDXP 卡密兑换与充值统计能力，允许管理员创建付费充值卡和赠送额度码，并让真实付费卡密兑换后进入充值统计。公开文档只记录可复现事实，不记录 SSH 私钥、后台密码、cookie、session、token 或服务器 secret。

### 功能范围

- 新增兑换码类型：
  - `paid_topup`：付费充值卡，兑换后增加额度并创建成功 `TopUp` 记录；
  - `promo_credit`：赠送额度码，兑换后只增加额度，不计入真实充值统计。
- `redemptions` 增加批次、来源、面额、实付金额、是否计入充值统计、导出时间等字段。
- default 后台兑换码页支持批次创建、复制本次生成卡密、TXT/CSV 导出。
- 钱包页兑换成功提示会区分付费充值卡与赠送额度码。
- `POST /api/user/topup` 保持 `data` 为数字，同时额外返回可选 `redemption` 元信息，兼容 classic 前端和旧调用方。

详细维护语义见：`docs/ldxp-redemption-cards.md`。

### 部署来源

- GitHub 分支：`codex/deploy-ldxp-card-redemption`
- 部署提交：`54cd3e16 fix: complete redemption card frontend integration`
- 合成基线：`codex/user-tags-model-groups`

本轮没有直接部署单独的 LDXP 功能分支，而是在用户标签 / 模型分组生产修复基础上创建合成部署分支，避免覆盖已经上线的用户标签修复。

### 本地验证

部署前在合成 worktree 中完成：

```bash
go test ./model ./controller ./router ./common ./setting/... -count=1
cd web/default
bun test \
  src/features/redemption-codes/lib/export-utils.test.ts \
  src/features/redemption-codes/lib/redemption-form.test.ts \
  src/features/wallet/lib/redemption-result.test.ts
bun run typecheck
bun run build
git diff --check
```

### 生产同步方式

- 生产目录 `/opt/new-api/app` 不是可信 git checkout，本轮没有在服务器上执行 `git pull`。
- 同步方式：非删除式 `rsync` 到 `/opt/new-api/app/`。
- 构建方式：`docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml build new-api`
- 重启方式：`docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml up -d --force-recreate new-api`

第一次 `rsync` 发现生产部分历史目录归属为 `501:staff`，导致 `deploy` 用户不能设置目录时间，也不能在部分目录写入新前端文件。已仅针对应用源码相关目录修正为 `deploy:deploy`，删除本轮误传的 worktree `.git` 文件，并使用 `--omit-dir-times --no-owner --no-group` 重新同步。最终确认 `/opt/new-api/app/.git` 不存在。

### 备份

部署前保留：

```text
/opt/new-api/backups/ldxp-card-redemption-predeploy-20260628-004211/app-source.tgz
yunbay-new-api:prod-pre-ldxp-20260628-004211
```

源码备份约 477M。镜像备份可用于快速回滚。

### 生产复核结果

最终复核时间：`2026-06-28T01:01:28+08:00`

容器状态：

```text
yunbay-new-api: running / healthy / restart_count=0
yunbay-caddy:   running / healthy / restart_count=0
yunbay-postgres running / healthy / restart_count=0
yunbay-redis    running / healthy / restart_count=0
```

入口验证：

```text
http://127.0.0.1:3000/api/status                  success=true setup=true
https://yunbay.xyz/                               200
https://yunbay.xyz/api/status                     200
https://yunbay.xyz/console/redemption-codes       200
https://yunbay.xyz/wallet                         200
GET /api/redemption/export?batch_id=__missing__   401（未登录预期）
```

数据库字段验证：

```text
amount
batch_id
count_as_top_up
exported_time
kind
money
source
```

字段类型：

```text
kind:character varying
amount:bigint
money:numeric
count_as_top_up:boolean
batch_id:character varying
source:character varying
exported_time:bigint
```

### 回滚提示

优先回滚镜像：

```bash
docker tag yunbay-new-api:prod-pre-ldxp-20260628-004211 yunbay-new-api:prod
cd /opt/new-api/app
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml up -d --force-recreate new-api
```

如需恢复源码，可从上述 `app-source.tgz` 恢复后重新 build/up。新增数据库列通常无需删除，旧代码会忽略多余字段；不要做破坏性 schema rollback，除非已经确认没有生产数据依赖这些列。

## 2026-06-28 兑换成功后前端跳 500 热修

用户反馈：普通用户点击兑换码兑换时，页面跳到 default 前端的 500 错误页，但额度实际上已经成功到账。

### 根因结论

本次不是后端 `POST /api/user/topup` 失败。生产日志显示相关兑换请求返回 `200`，且兑换后的 `GET /api/user/self` 也返回 `200`。截图中的 500 来自 default 前端错误页/错误边界，而不是后端 HTTP 500。

最窄根因在前端成功路径：

- Quick Start 兑换流程把 `POST /api/user/topup` 成功后的用户信息刷新作为同一个成功条件；
- 钱包兑换 hook 也在成功后内部重复调用 `getSelf()`，页面层随后还会再刷新一次；
- 后续刷新或刷新链路的非关键异常可能把一次已经落库成功的兑换表现成前端错误/失败页。

### 修复内容

- `web/default/src/features/quick-start/quick-start-redemption.ts`
  - 兑换接口返回 `success: true` 后即视为兑换成功；
  - 后续 `refreshSelf()` 改为最佳努力，失败时返回 `refreshed: false`，不再把已成功兑换变成失败。
- `web/default/src/features/wallet/hooks/use-redemption.ts`
  - 移除 hook 内部重复 `await getSelf()`；
  - 继续由钱包页面层 `fetchUser()` 负责刷新用户信息，页面层已有错误兜底。
- 新增回归测试：
  - Quick Start 覆盖“兑换成功但刷新用户失败仍返回成功”；
  - 钱包 hook 源码约束确保成功路径不再内联重复 `getSelf()`。

### 本地验证

```bash
cd web/default
bun test \
  src/features/quick-start/quick-start-redemption.test.ts \
  src/features/wallet/hooks/use-redemption-source.test.ts \
  src/features/wallet/lib/redemption-result.test.ts \
  src/features/redemption-codes/lib/export-utils.test.ts \
  src/features/redemption-codes/lib/redemption-form.test.ts
bunx prettier --check \
  src/features/quick-start/quick-start-redemption.ts \
  src/features/quick-start/quick-start-redemption.test.ts \
  src/features/wallet/hooks/use-redemption.ts \
  src/features/wallet/hooks/use-redemption-source.test.ts
bun run typecheck
bun run build
```

验证结果：

```text
16 pass / 0 fail
Prettier matched files pass
TypeScript typecheck pass
Rsbuild production build pass
```

### GitHub 与生产同步

- GitHub 分支：`codex/deploy-ldxp-card-redemption`
- 修复提交：`10dca0ee fix(web): keep redemption success after refresh failure`
- 同步方式：精确文件列表、非删除式 `rsync --files-from` 同步到 `/opt/new-api/app/`
- 构建方式：`docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml build new-api`
- 重启方式：`docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml up -d --force-recreate new-api`

### 生产备份与回滚点

部署前保留：

```text
/opt/new-api/backups/redemption-refresh-fix-predeploy-20260628-175639/changed-files.tgz
yunbay-new-api:prod-pre-redemption-refresh-fix-20260628-175639
```

若需要回滚镜像：

```bash
docker tag yunbay-new-api:prod-pre-redemption-refresh-fix-20260628-175639 yunbay-new-api:prod
cd /opt/new-api/app
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml up -d --force-recreate new-api
```

### 生产复核结果

最终复核时间：`2026-06-28T18:02:37+08:00`

```text
yunbay-new-api: healthy
https://yunbay.xyz/             200
https://yunbay.xyz/api/status   200
https://yunbay.xyz/wallet       200
https://yunbay.xyz/quick-start  200
http://127.0.0.1:3000/api/status 返回 JSON
```

## 2026-06-29 LDXP 自动充值二维码等待动画灰测

本节只记录可公开维护事实，不记录服务器登录凭据、Worker token、邮箱授权码、支付密钥或完整私密连接信息。

### 本轮完成内容

- 钱包 LDXP 支付弹窗在 `created` / `worker_claimed` 状态下显示更醒目的弹出式大转圈等待面板。
- 金额位置保留 30 秒进度动画，给用户明确等待反馈。
- 新增提示文案：`The payment QR code usually appears in about 20 seconds. Please wait.`，并补齐 `en/zh/fr/ja/ru/vi` 翻译。
- 本轮只重建并重启 `new-api`，没有修改或重建 LDXP browser worker。

### VIP 自动升级规则确认

VIP 自动升级按成功充值记录中的实际支付金额累计判断：

```text
sum(top_ups.money where status='success') >= 30.0
```

不要用 `top_ups.amount` 或 `ldxp_topup_sessions.amount` 判断 VIP 阈值。灰测折扣订单可能出现 `amount=30` 但 `money=0.3` 的情况，此时不应自动升级 VIP。

### 生产灰测验证

2026-06-29 灰测验证结果：

```text
yunbay-new-api: healthy
ldxp-browser-worker: running
new-api image: sha256:6632a1ce50ede30f84c897820678c080f32899c9883e5c81e2d177ea3938a036
worker image: sha256:d0596df45239b943f45b9de7881b2ddd96d26f62c7386cbeae2475409f62f55c
served css markers: ldxp-qr-creation-pop / ldxp-qr-creation-pulse / ldxp-qr-creation-spinner
```

回滚点：

```text
backup dir: /opt/new-api/backups/ldxp-ui-popup-spinner-20260629164951
rollback image: yunbay-new-api:pre-ui-popup-spinner-20260629164951
```

详细排障命令见：`docs/ldxp-browser-worker-auto-topup-runbook.md`。

## 2026-06-29 LDXP 正式商品与推荐奖励提现维护

本节只记录可公开维护事实，不记录支付后台账号、SSH 私钥、cookie、session、数据库密码或生产环境变量明文。

### LDXP 正式商品档位

当前 LDXP 自动充值默认商品配置为 **6 个正式档位**，不要加入早先临时讨论过的 `200` 档：

| 档位 | 正式商品链接 |
| ---: | --- |
| 10 | `https://pay.ldxp.cn/item/nzkyrt` |
| 20 | `https://pay.ldxp.cn/item/ka4pg7` |
| 30 | `https://pay.ldxp.cn/item/n8schm` |
| 50 | `https://pay.ldxp.cn/item/5c4yft` |
| 100 | `https://pay.ldxp.cn/item/sb48mz` |
| 500 | `https://pay.ldxp.cn/item/y8t52c` |

维护点：

- 代码默认值：`service/ldxp_config.go`。
- 前端固定金额：`web/default/src/features/wallet/lib/ldxp-topup.ts`。
- 若生产 `LDXP_TOPUP_PRODUCTS_JSON` 显式配置，则以生产 env 为准；修改前先备份 `/opt/new-api/secrets/prod.env`，不要在日志或公开文档输出 env 明文。
- 卡网/链动小铺手续费由用户在支付页承担，不计入云贝业务实付金额。例如 10 档支付宝收银台可显示 `10.30`，但 `ldxp_topup_sessions.money`、`top_ups.money`、VIP 累计和推荐奖励基数仍按 `10.00`。

### 推荐奖励金规则

推荐关系继续复用 new-api 自带邀请体系：

```text
邀请码/邀请链接：users.aff_code、/api/user/aff
邀请关系：users.inviter_id
旧额度奖励：users.aff_quota、/api/user/aff_transfer
新增现金奖励流水：affiliate_commissions
新增提现流水：affiliate_withdrawals
```

现金奖励规则：

- 只在 `top_ups.status='success'` 后创建。
- 奖励基数为 `top_ups.money`（实际支付金额），不是 `top_ups.amount` 或到账 quota。
- 邀请人进账比例为 `15%`，金额按分位四舍五入。
- 每个 `topup_id` 只能创建一条 `affiliate_commissions`，重复回调或重复验证应保持幂等。
- 无邀请人、自邀、邀请人不存在、订单未成功或实付金额小于等于 0 时不发奖励。

### 用户提现流程

用户钱包页会显示：

- 可提现奖励：`available = total_commission - pending_withdrawals - paid_withdrawals`
- 冻结奖励：`pending_withdrawals`
- 已提现奖励：`paid_withdrawals`
- 返佣比例：当前为 `15%`

用户点击 `Apply for Withdrawal` 后提交：

```text
amount  提现金额
contact 支付宝/微信/邮箱等打款联系方式
remark  给管理员的可选备注
```

后端创建 `affiliate_withdrawals.status='pending'` 后，该金额立即计入冻结奖励，不再显示为可提现余额。

### 管理员处理提现

当前提现是人工打款流程：

1. 管理员查看待处理提现。
2. 线下核对用户联系方式并完成实际打款。
3. 打款成功后调用 paid 操作：冻结金额转为已提现。
4. 如果拒绝提现，调用 reject 操作：冻结金额释放回可提现余额。

相关接口：

```text
GET  /api/user/affiliate/summary
POST /api/user/affiliate/withdrawals

GET  /api/user/affiliate/withdrawals
POST /api/user/affiliate/withdrawals/:id/paid
POST /api/user/affiliate/withdrawals/:id/reject
```

排障 SQL 示例：

```sql
select id, username, aff_code, inviter_id
from users
where username in ('<inviter_username>', '<invitee_username>');

select id, user_id, amount, money, trade_no, payment_method, payment_provider, status
from top_ups
where user_id = (select id from users where username = '<invitee_username>')
order by id desc
limit 20;

select id, commission_id, inviter_user_id, invitee_user_id, topup_id,
       base_money, rate, commission_money, status, created_time
from affiliate_commissions
where inviter_user_id = (select id from users where username = '<inviter_username>')
order by id desc
limit 20;

select id, withdrawal_id, user_id, amount, status, created_time, processed_time
from affiliate_withdrawals
where user_id = (select id from users where username = '<inviter_username>')
order by id desc
limit 20;
```

上线验证建议：

```bash
cd /opt/new-api/app
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml ps new-api ldxp-browser-worker
curl -fsS http://127.0.0.1:3000/api/status >/tmp/yunbay-status.json
```

本轮涉及后端 migration 和前端构建，灰测前应确认 `affiliate_commissions`、`affiliate_withdrawals` 表已随应用启动完成迁移。

## 2026-06-29 LDXP 正式商品二维码修复与正式链路恢复

本节只记录可公开维护事实，不记录 SSH 私钥、worker token、生产 env 明文、支付后台账号或二维码内容。

### 问题与根因

正式 10 元商品 `https://pay.ldxp.cn/item/nzkyrt` 灰测时曾出现二维码不返回。生产证据显示：

```text
session 使用正式商品 nzkyrt
worker 已 claim
qr=not_called
wait_cashier_ready 超时
debug snapshot 仍停留在链动小铺商品/过渡页
```

根因不是把 `money` 配成 `10`，也不是要改成 `10.3`。实际链路是：

```text
商品页 -> /shopApi/Pay/order -> /payApi/AlipayPc/pay.html -> excashier.alipay.com
```

旧 worker 把 `payApi` 过渡 URL 当成“收银台 ready”，过早读取页面文本，导致丢掉后续真正跳到支付宝收银台的 popup/页面。

### 修复内容

- Worker 不再用 `payApi`/URL 命中作为二维码 ready 条件；必须等页面文本出现订单号以及金额/付款/收款方标记。
- Worker 和后端金额校验允许用户承担的合理卡网手续费：`actual = configured money + fee`。
- 业务金额不变：10 档仍按 `money=10` 记账，手续费不计入云贝充值金额、VIP 累计或推荐奖励基数。
- 同步了当前线上 worker 运行基线中的 browser prewarm / paid-watch 源码，并补齐 `/api/ldxp/worker/sessions/claim-paid-watch`。无待监听 QR 会话时该接口返回 `record not found`。

### 2026-06-30 支付成功识别回归与队列稳定性修正

后续灰测先出现“第三次二维码不出”的现象。生产证据显示：

```text
前两笔 session 已 worker_claimed 并约 21-22 秒进入 qr_ready
第三笔 session 长时间停留 created，worker_id / worker_order_no / qr_code 均为空
该时间窗口 worker 没有继续 POST /api/ldxp/worker/sessions/claim
```

当时的判断是：worker 并发为 `2`，前两笔“已出二维码、等待用户付款”的会话占满槽位，第三笔不会被 claim。于是临时尝试 `LDXP_RELEASE_SLOT_AFTER_QR=true`：出码后释放主浏览器槽位，再由 paid-watch 重新打开 `qr_page_url` 观察付款结果。

该尝试随后被生产证据证明会破坏白天已经跑通的付款成功识别路径：

```text
已付款订单 LD260630C62RUK：
session 仍停在 qr_ready，topup_id=0，worker_detected_time=0
worker 一直 claim paid-watch，但没有 posted paid result
worker 容器二次打开 qr_page_url 时，从 excashier.alipay.com/standard/auth.htm 跳到 /home/error.htm
```

结论：当前支付宝 QR URL 不能作为可靠的二次打开状态页。正式支付确认必须沿用白天跑通的链路：

```text
worker 打开商品页 -> 出二维码 -> 同一个支付宝收银台页面保持打开 -> 用户扫码付款 -> 页面跳支付成功 -> worker 识别成功页 -> 后端入账
```

当前修正：

- 生产推荐配置和 worker 默认值恢复为 `LDXP_RELEASE_SLOT_AFTER_QR=false`。
- paid-watch 保留为代码能力，但不是当前支付宝链路的主确认路径；除非重新证明 `qr_page_url` 可二次打开并看到成功页，否则不要作为生产确认条件。
- QQ IMAP 邮件只用于事后审计，不是直接充值成功确认条件。
- 队列占槽问题改由“用户取消支付主动断 worker 线”解决，而不是出码后释放收银台页面。

取消支付断线补充：

- 前端取消支付仍调用 `/api/user/ldxp/topup/session/:session_id/cancel`，后端把 `created` / `worker_claimed` / `qr_ready` 改为 `canceled`。
- 新增 worker 内部状态接口 `/api/ldxp/worker/sessions/:session_id/active`。
- worker 在浏览器流程运行期间轮询该接口；如果用户已取消或 session 不再属于该 worker，立即 abort 当前浏览器流程、关闭 browser context，不再继续等待支付宝/二维码/付款结果。
- active-check 临时失败时不误杀订单，只记录 warning 并继续，避免后端瞬时抖动导致真实支付被断。

当前推荐生产设置：

```text
LDXP_RELEASE_SLOT_AFTER_QR=false
LDXP_WORKER_CONCURRENCY=2
LDXP_WORKER_POLL_INTERVAL_MS=2000
```

### 生产验证结果

正式 10 档独立探测（不创建云贝用户充值单、不打印二维码内容）：

```text
product: https://pay.ldxp.cn/item/nzkyrt
cashier host: excashier.alipay.com
cashier amount: 10.30
configured Yunbay money: 10.00
elapsed: about 24.4s
result: QR extracted
```

正式 6 档生产 env 已恢复：

```text
amounts=10,20,30,50,100,500
money=10,20,30,50,100,500
slugs=nzkyrt,ka4pg7,n8schm,5c4yft,sb48mz,y8t52c
```

上线镜像：

```text
new-api image: sha256:e7427b2921cfcc9ee4ad31a7efdbb05448991931dab66249b6332b3b0abb99ba
worker image: sha256:86ac7d873aa1ae7afc596e5cde83733e74180aa60cccd4f36d2262623ad51c97
formal env backup: /opt/new-api/backups/ldxp-formal-products-payapi-fee-fix-20260629234617
new-api rollback image before paid-watch route: yunbay-new-api:pre-ldxp-paid-watch-route-20260629235912
queue-slot fix images: new-api sha256:4e4be67e7f44bb01c80ff0e1611911d554bf2daa30100e9c8d20508719b499f7, worker sha256:619a6a1f8cf274b6b0536bfe6865af9c591c7a8aa4c9ff15a49a17ff7fb672d0
queue-slot backups: /opt/new-api/backups/ldxp-release-slot-rotation-20260630002705, /opt/new-api/backups/ldxp-paidwatch-coalesce-20260630004530
```

验证命令：

```bash
cd /opt/new-api/app
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml ps new-api ldxp-browser-worker
curl -fsS http://127.0.0.1:3000/api/status >/tmp/yunbay-status.json
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml logs --since=10m ldxp-browser-worker
```

如果再次出现二维码不返回，先看 worker 计时字段：

```text
click_purchase_to_cashier
wait_cashier_ready
read_cashier_text
extract_qr
qr=called / qr=not_called
```

不要因为支付宝页显示 `10.30` 就把正式商品配置改成 `money=10.3`；那会污染充值金额、VIP 判断和推荐奖励基数。

## 2026-07-14 New API/Sub2API 并发 429 紧急处置

- 现象：`new-api` 渠道 35（`http://sub2api:8080`）测试及调用返回 `429 Concurrency limit exceeded for user`；同一时段后台重复触发多个模型测试，每个测试约 30 秒。
- 证据：New API 日志出现 `value package concurrency denied: subscription=17 limit=2`；这表示云贝订阅套餐自身并发上限已占满，不是 OpenAI 官方固定限制。代码/后台目前只允许套餐并发配置为 1 或 2；Sub2API 账号自身另有独立并发限制。
- 处置：仅对 `yunbay-new-api` 执行 `docker compose ... up -d --force-recreate new-api`，清理卡住的本地中转请求；未重启 Sub2API、PostgreSQL 或 Redis，未修改计费/套餐数据。
- 验证：重建后 `yunbay-new-api` healthy；本机 `GET http://127.0.0.1:3000/api/status` 返回 `200`；容器内访问 `http://sub2api:8080/health` 返回 `{"status":"ok"}`。
- 后续：停止连续点击“测试渠道”；如需提高并发，应在后台调整对应套餐/订阅策略并同步评估 Sub2API 上游账号并发额度，不能把套餐上限误认为官方额度。

## 2026-07-14 Sub2API 用户并发改为无限

- 生产事实：`/opt/new-api/sub2api/data/config.yaml` 原为 `default.user_concurrency: 5`；Sub2API 数据库 `users.id=1`（渠道 API Key ID=1 所属用户）原 `concurrency=5`。
- 依据：Sub2API 运行代码将 `concurrency <= 0` 定义为不限制，并发 429 文案正来自该用户槽位检查。
- 处置：将生产数据库 `users.id=1.concurrency` 更新为 `0`；将默认配置 `default.user_concurrency` 更新为 `0`；原配置备份保存在服务器 `/opt/new-api/sub2api/data/config.yaml.bak-unlimited-<timestamp>`。
- 重启范围：仅 `yunbay-sub2api`，未重启 PostgreSQL/Redis；容器镜像 digest 未变，重启后 healthy。
- 验证：容器内 `GET http://127.0.0.1:8080/health` 返回 `{"status":"ok"}`。
- 备注：这只解除 Sub2API 用户级并发；New API 自身的 value-package `subscription=17 limit=2` 仍是独立限制，若继续出现 `value package concurrency denied`，需单独处理订阅权益映射/并发策略。

## 2026-07-15 API Key“无限狂暴不中断”钱包回退上线

- GitHub 代码基线：`main@1de11a839ffd76e7904c27ec57b27a154f8d7e3d`，普通 fast-forward 推送，未使用 force。CI run `29367152184` 全部通过：<https://github.com/chenli17683185032-ai/yunbay/actions/runs/29367152184>。
- 功能：API Keys 页新增默认开启的“无限狂暴不中断”开关。请求优先使用超值套餐和套餐倍率；套餐总额度、5 小时或 7 天窗口耗尽后整单切到账户余额，并恢复用户真实 VIP 倍率；套餐额度恢复后的新请求自动切回套餐。历史用户和新用户的 `NULL` 偏好均按开启解释，用户可显式关闭以保留原 `403` 行为。
- 帮助入口：保留闪电图标，另加圆形问号。桌面悬停/键盘聚焦使用 Tooltip，移动端点击使用 Popover，完整说明为“优先使用特惠套餐额度；用尽后按 VIP 等级倍率使用账户余额，套餐额度恢复后自动切回。”
- 验证：完整主项目 Go 包、根包、default typecheck、6 项定向前端测试、定向 ESLint、default/classic 生产构建、六语 i18n 同步及 `git diff --check` 通过。仓库全量 ESLint 仍有 114 个既有错误，均不在本次改动文件；本次两个新增前端文件定向检查为 0 错误。
- 响应式 QA：本地 `1440x900`、`390x844`、`320x720` 均通过；移动端点击问号后 Popover 可见且不会改变开关值，320 宽度下 `scrollWidth=viewportWidth=320`。生产登录态 DOM 复核显示开关 `aria-checked=true`、两个响应式帮助按钮中仅一个可见、完整 `aria-label` 存在且页面无横向溢出。

发布使用 27 文件精确、非删除式 rsync，生产文件组合 SHA-256 为：

```text
ebd9c84b50b037f182b462f7aa6e92f10e049417983234a56b45616b551cb5ed
```

生产命令：

```bash
cd /opt/new-api/app
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml build new-api
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml up -d --no-deps --force-recreate new-api
```

- 第一次切换保护脚本因 GNU `timeout` 不能直接调用 shell 函数而在 `compose up` 前退出，容器未替换，旧服务持续 `healthy/200`。修正为直接调用 `docker compose` 后再次执行，正式切换 12 秒完成。
- 新镜像：`sha256:59375b4c3e6d846ab21f7f8fe0c5b51bac1f0cff5526590218b01aede5abe945`，保留审计标签 `yunbay-new-api:release-1de11a83`。
- 数据库迁移：PostgreSQL 新列 `user_value_package_preferences.wallet_fallback_enabled` 为 `boolean`、可空、默认 `NULL`。上线后 39 条历史偏好记录均为 `NULL`，与业务层“默认开启”语义一致。
- 服务验证：`yunbay-new-api` 为新镜像且 `healthy`；公网 `/`、`/keys`、`/api/status` 均为 HTTP 200；未登录 `PUT /api/value-packages/wallet-fallback` 返回 401，证明路由已注册；启动日志无相关 `panic`、`fatal` 或迁移错误。
- 重启范围：仅 `yunbay-new-api`。Caddy、PostgreSQL、Redis、Sub2API、LDXP worker 的启动时间均未变化。

回滚点：

```text
source backup: /opt/new-api/backups/value-package-wallet-fallback-20260714T205310Z
old image id: sha256:2ad45f2669d6e7f56625da2dfd02ac2b21bf61f10e28145cfdbd8addd879d68c
old image tag: yunbay-new-api:rollback-wallet-fallback-20260714T205310Z
```

镜像回滚只重建 `new-api`：

```bash
cd /opt/new-api/app
docker tag yunbay-new-api:rollback-wallet-fallback-20260714T205310Z yunbay-new-api:prod
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml up -d --no-deps --force-recreate new-api
```

源码回滚从备份目录的 `source/` 恢复原有 24 个文件，并删除部署前不存在的 3 个新增文件。新增数据库列可保留，旧代码会忽略该可空列；禁止为回滚而删除历史偏好数据或重启 PostgreSQL、Caddy、Redis、Sub2API。

## 2026-07-15 LDXP 中国大陆出口付款链路上线

- 事故现象：生产 VPS 直连 `pay.ldxp.cn` 会间歇进入阿里云 ESA 验证页，导致当天多笔 LDXP 会话在生成订单号和付款二维码前失败。香港 IPLC、香港家宽、Firefox 核心和香港 SOCKS5 均不能稳定消除验证；生产同款 Chromium 通过中国大陆出口后恢复正常。
- 最终方案：worker 新增可选 `LDXP_BROWSER_PROXY_SERVER`，只把 Playwright Chromium 流量交给代理；后端 claim/callback、IMAP、`new-api` 及其他容器仍走原网络。生产 worker 与 `yunbay-ldxp-browser-proxy` 共享网络命名空间，通过回环 `socks5://127.0.0.1:7891` 访问 Mihomo，宿主机和应用 bridge 均未发布代理端口。
- 代理控制：Mihomo 固定为 `metacubex/mihomo:v1.19.28`，采用 `rule` 模式和精确节点过滤，只选择已验证的 `影音无限流量a1全球回国1`。provider 每小时刷新，连通性检查每 10 分钟执行；订阅 URL、token 和节点凭据只保存在服务器 `/opt/new-api/secrets/ldxp-browser-proxy.yaml`，权限 `0600`，未进入仓库、`prod.env`、结果 JSON 或日志。
- 代码收敛：撤回当天“worker 失败后返回 LDXP 直购链接”的 Go、React、类型和六语翻译改动，不再让用户自行填写联系方式和兑换卡密。保留 ESA DOM 快速识别，代理失效时 worker 会立即报告 `waf_challenge`，避免等待完整页面超时。
- GitHub 经验核对：实现前参考了 Playwright 项目中“代理 URL 启动前严格解析、拒绝混合凭据”的做法，以及 MetaCubeX 官方 `proxy-provider` 文档中的 `path`、`interval`、`filter` 和 `health-check` 约束。第一次生产形态测试发现 Mihomo `global` 模式未显式选择时会落到 `DIRECT`，因此结果作废并修正为 `rule + MATCH,LDXP-MAINLAND` 后重新验证。
- 代码验证：worker `bun run check` 全部 79/79 通过；非法协议、缺少端口、URL 内嵌凭据、路径污染、ESA 快速失败均有测试。candidate Docker 镜像构建成功，共享 proxy 网络访问 `http://new-api:3000/api/status` 返回 200。
- 出口验证：正式 compose、正式 worker 镜像和正式 sidecar 连续 3/3 打开商品页，三次出口均为 `116.31.164.40`（中国广东汕头电信），HTTP 200、标题“10 刀兑换券 - 链动小铺”、ESA 元素数为 0、联系方式输入框数为 1 且可见。全程未输入联系方式、未点击购买、未创建订单。结构化结果为 `outputs/recharge-incident-20260715/proxy-tests/ntpizza-production-provider-results.json`，SHA-256 `4ca24d8bbc1abcf36022338fb9614b13fd4e8a4253fc26994178d0a01168b78c`。
- 业务闭环：使用现有运维账号创建 10 元未付款会话 `id=217`，状态按 `created -> worker_claimed -> qr_ready` 演进，27 秒生成 3750 字节二维码且存在真实 worker 订单号。随即调用正式取消接口，最终数据库为 `canceled`、`topup_id=0`，没有付款或入账。
- 发布：部署文件组合 SHA-256 为 `43bc5b57e1edaeffb0d5b4c23561512263d1a0cca8127edf97696ab813789faa`，私有 compose SHA-256 为 `c3fe78af2017fd688099495b04f0cf5cf7209e428067b6faee3e738f25933c1f`。worker 切换 6 秒，`new-api` 切换 17 秒；Caddy、PostgreSQL、Redis、Sub2API 均未重启。
- 当前镜像：`yunbay-new-api:prod` 为 `sha256:d2f26c1e92ea718f49dc2242a580f64df49e0fa1b28fd15287a1fcc06fd6372f`；`yunbay-ldxp-browser-worker:prod` 为 `sha256:3fb25c17ad59d2938ce35139b889cf5c1cb1837bc8751eb891378929e622073c`；Mihomo 为 `sha256:e6acd921addecfd59a8e2d38203f88356d635b54de6c0673db0e015139989312`。
- 最终状态：`new-api running/healthy`、proxy `running/healthy`、worker `running`，三者 restart 均为 0；容器内状态探针 10/10 为 200，部署后 worker 无新 `waf_challenge` 或流程错误，线上前端不再包含直购临时文案。candidate 容器、临时镜像、订阅副本和本机敏感临时文件均已删除。
- 容量风险：订阅在上线时标注剩余 `5 GB`、到期 `2026-07-17`。同一 token 续费后 provider 会自动刷新；到期或流量耗尽前必须续费。当前端口健康只证明 Mihomo 正在监听，业务健康仍应以 LDXP 联系方式输入框和 `waf_challenge` 日志为准。

回滚点：

```text
backup: /opt/new-api/backups/ldxp-mainland-proxy-20260715T131333Z
new-api rollback tag: yunbay-new-api:rollback-ldxp-mainland-proxy-20260715T131333Z
worker rollback tag: yunbay-ldxp-browser-worker:rollback-ldxp-mainland-proxy-20260715T131333Z
old new-api image: sha256:53a8348b7a5bdc9aee76788c7e2be11cda0f966e2b06be5c04b9b6446925569d
old worker image: sha256:a22102ce7afdff6f1973818f7cf7b866fa918b7ed771a9f882bd3beddfaea091
```

回滚时先停止正式 worker，恢复备份中的 `docker-compose.prod.yml.before` 和 `source/` 文件，将两个 rollback 标签重新标记为 `:prod`，再按旧 compose 重建 `new-api` 与 `ldxp-browser-worker`。确认旧 worker 已恢复独立 `yunbay-network` 后，才删除 `yunbay-ldxp-browser-proxy`；禁止重启或回滚 PostgreSQL、Redis、Caddy、Sub2API，也不要删除任何充值、入账或兑换历史数据。

## 2026-07-16 快速启动苹果式五页引导上线

本轮重构普通用户快速启动，保留原有暗色点云与非线性视觉，把用途、模型、软件下载、账户准备和安装导入收敛为五页闭环。运维记录只包含可公开复现的代码、构建、部署和健康事实，不记录 SSH 私钥、生产环境变量、API Key、cookie 或用户会话。

### 发布内容

- 发布提交：`6e8ec9399c10a2b844336c3b38c7cae9c4c2a098`（`feat: rebuild quick start onboarding`），已推送 GitHub `main`。
- 默认模型精确使用 `gpt-5.6-sol`；思考强度作为独立目标使用 `xhigh`，界面显示“极高思考”，不存在 `gpt-5.6-sol-thinking` 拼接模型。
- 软件入口只保留 CC Switch 官方 Mac Universal 与 Windows 64 位安装包，版本为 `v3.17.0`，点击后直接打开 GitHub release 资产。
- 充值/兑换和 API Key 合并为同一页；可恢复最新启用的 `yunbay-quick-start-*` Key，完整 Key 不写入浏览器存储。
- 安装确认、协议导入、人工确认和控制台完成摘要组成可复查闭环；完成摘要排在强制公告之后。
- 整页退出使用 Web Animations API 驱动三段波浪裁切、模糊、透明度和位移关键帧；减少动态效果时只执行短淡出。
- 官方 CC Switch `v3.17.0` 的 Codex 深链仍把 `model_reasoning_effort` 固定为 `high`，公开 importer 不能保留内嵌 `xhigh`。本站因此只可靠导入模型、endpoint 和 Key，并要求用户在 CC Switch 中确认“极高”，没有添加无效伪参数。

### 验证

- `bun test src/features/quick-start/*.test.ts`：49 pass / 0 fail。
- `bun run typecheck`、本轮文件 ESLint、Prettier、六语 i18n 同步、`bun run build` 和 `git diff --check` 均通过。
- en、zh、fr、ja、ru、vi 的 missing、extras、untranslated 均为 0；`CC Switch` 在六语中均保留产品专名。
- 浏览器 1280x720 与 390x844 完整走通五页、Key 恢复、导入、强制公告、复查和直接开始；移动端无横向溢出或底栏覆盖。
- 约 300ms 的退出动画运行态采样显示波浪 `clip-path`、约 `blur(1.59px)`、`opacity 0.936` 和轻微下沉缩放；减少动态效果约 70ms 时只有 `opacity` 变化。
- 仓库全量既有边界：lint 仍有 113 errors / 6 warnings，format 检查仍有 61 个既有未格式化文件，版权全量检查仍有 17 个既有文件待更新；本轮文件不在这些失败项中。

### 部署与回滚

生产 `/opt/new-api/app` 仍不是可信 Git checkout。本轮使用提交差异生成 21 文件精确列表，先备份再执行非删除式 rsync；本地与生产文件组合 SHA-256 均为：

```text
599de8b839ca93001228875766b197a430b2b00a656e0b548a4c7fa19cf343f0
```

回滚点：

```text
source backup: /opt/new-api/backups/quick-start-onboarding-20260715T182454Z-6e8ec939
old image: sha256:d2f26c1e92ea718f49dc2242a580f64df49e0fa1b28fd15287a1fcc06fd6372f
rollback tag: yunbay-new-api:rollback-quick-start-20260715T182454Z
```

新版本：

```text
image: sha256:3e38bc6f23e74bd51b19a326dbd63aa84b5efc7b2a029378bbab0220da5e403f
release tag: yunbay-new-api:release-6e8ec939
public entry: /static/js/index.3fdb276058.js
```

构建期间旧容器持续服务。正式切换只执行 `new-api`：

```bash
cd /opt/new-api/app
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml up -d --no-deps --force-recreate --no-build new-api
```

切换 11 秒恢复 healthy，容器 restart count 为 0。Caddy、PostgreSQL、Redis、Sub2API、LDXP worker 和 LDXP proxy 的启动时间均未变化。

上线后验证：内网 `/api/status` 10/10 为 HTTP 200；公网 `/`、`/quick-start`、`/api/status` 均为 HTTP 200；入口 bundle 包含 `gpt-5.6-sol`、CC Switch 官方仓库、`v3.17.0`、`macOS.dmg` 和 `Windows.msi` 标记，且不含 `gpt-5.6-sol-thinking`。新容器启动日志没有 panic 或 fatal；观察到的错误均为上线后真实请求收到的既有上游模型不支持或额度拒绝，与本次启动和快速引导无关。

需要回滚时，从备份 `source/` 恢复 `existing-files.txt` 中的文件，删除 `new-files.txt` 中登记的 4 个本轮新增文件，再把回滚镜像重新标记为 `yunbay-new-api:prod`，最后只重建 `new-api`：

```bash
cd /opt/new-api/app
docker tag yunbay-new-api:rollback-quick-start-20260715T182454Z yunbay-new-api:prod
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml up -d --no-deps --force-recreate --no-build new-api
```

禁止为本轮前端回滚而重启或修改 PostgreSQL、Redis、Caddy、Sub2API、LDXP worker 或 LDXP proxy。

### 发布后指示与实际状态

发布完成后，用户补充要求“先不要上线”。为恢复发布前状态，运维侧只完成了当前容器、回滚镜像和备份目录的只读预检查，尚未执行源码恢复、文件删除、镜像重标记、Compose 重建或任何服务重启。随后用户明确要求不要撤回，以免打断服务器运行，因此立即取消回滚。

取消时生产仍运行新镜像 `sha256:3e38bc6f23e74bd51b19a326dbd63aa84b5efc7b2a029378bbab0220da5e403f`，Compose 服务状态为 `running / healthy`。本次取消回滚没有造成额外服务切换或中断；后续除非用户再次明确要求，不执行该回滚方案。

## 2026-07-16 快速启动页内动效与平台图标无中断上线

本轮保持快速启动五页信息架构和业务状态机不变，只精修页内操作反馈、平台品牌图标和响应式对齐。运维记录不包含 SSH 私钥、生产环境变量、API Key、cookie 或用户会话。

### 发布内容与验证

- 功能发布基线：`a048627a338ca19b2ad7c4930bfae15ed798c61a`（`feat: refine quick start interactions`），已包含于 GitHub `main`；生产标记按设计指向这个实际参与构建的功能提交，后续纯运维文档提交不要求重新构建生产镜像。
- 用途、模型和软件选择使用共享选中底板与弹簧反馈；模型选择后的重排使用 Motion position layout，步骤编号到完成勾选使用交叉淡入、缩放和位移。
- `prefers-reduced-motion: reduce` 下取消空间位移与缩放；完整 reload 后选择前、中间帧和最终帧的 transform 均为 `none`。
- Mac 使用 `SiApple` 品牌标识，Windows 使用 `FaWindows`；下载按钮桌面固定 `224x48`，移动端固定 `316x48`。账户操作和最终控制器也完成等宽、单行与最长法语/俄语文案验收。
- 默认模型继续精确使用 `gpt-5.6-sol`，思考强度独立为 `xhigh`；没有引入 `gpt-5.6-sol-thinking`。
- 自动化结果：快速启动测试 `51 pass / 0 fail`，TypeScript typecheck、涉及文件 ESLint、Prettier、六语 i18n 同步、生产构建和 `git diff --check` 均通过。
- Browser 在 `1280x720` 和 `390x844` 下完成交互、对齐、长文案、滚动区和减少动态效果验收，控制台 error/warn 为 0；本地生产入口为 `/static/js/index.4f358c87c4.js`。

### 历史部署记录（旧蓝绿流程已废止）

> 以下内容只记录当时实际发生的发布过程，不是当前操作手册。临时绿实例、临时 Caddyfile 和 Caddy 动态切流已因 2026-07-16 全站 502 事故被禁止复制执行。

本轮只精确、非删除式同步四个文件。生产与本地逐项 SHA-256 一致，哈希清单组合值为：

```text
fdfc8da280151ff290b7ace8fe1e7f73612867300ef09d4176cd447db1721490
```

回滚点与镜像：

```text
source backup: /opt/new-api/backups/quick-start-motion-20260715T194510Z-a048627a
old image: sha256:3e38bc6f23e74bd51b19a326dbd63aa84b5efc7b2a029378bbab0220da5e403f
rollback tag: yunbay-new-api:rollback-quick-start-motion-20260715T194510Z
new image: sha256:bd931400a56e9ad0b43c776bbffac26075267a91ac2362c596880b655a4857fa
release tag: yunbay-new-api:release-a048627a
standard container: 2f6671300340a284d4572a9d1b05ee6257028fa6dcfeaeecfaa476c2a137a302
```

构建期间旧 `yunbay-new-api` 容器持续 `healthy`，ID 和启动时间未变化。候选绿实例使用当前生产实例解析后的环境和相同挂载，环境临时文件权限为 `0600` 且创建容器后立即删除；绿实例未映射宿主机端口。第一次候选启动证明应用 `/api/status` 已是 200，但直接 `docker run` 没有自动继承 Compose 服务层 HEALTHCHECK，因此没有切流；该候选先被移除，再带与正式服务相同的 HEALTHCHECK 参数重建并达到 Docker `healthy`。

绿实例入口和静态资源验收：

```text
green start_time: 1784168601
entry: /static/js/index.4f358c87c4.js
bundle sha256: 08c82ce58f3adb7fc781366b41beb37bd2a7e147d74303cbd7356a7e2ddca3b7
bundle bytes: 3062829
```

历史事实：该次发布曾用临时 Caddyfile 将 upstream 切到 `new-api-green:3000`，再重建标准实例。`caddy validate` 当时只证明配置可加载，并未证明 upstream 可用；该步骤现已废止。标准实例当时使用的 Compose 命令如下，该命令本身仍属于当前允许的单服务重建动作：

```bash
cd /opt/new-api/app
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml up -d --no-deps --force-recreate --no-build new-api
```

历史上标准实例达到 `healthy` 后曾 reload 正式配置回 `new-api:3000`。正式宿主机 Caddyfile SHA-256 始终为 `655d48c14d94372bf2383af094899fc3e3e8c54fe24b4476ba3ae7f887302388`；Caddy 容器 `8a961ee82d49a452d551e7f36cdcd8931ef844b3b8392f5a3761cc40a753735a` 的 ID、启动时间和 restart count 均未变化。Compose 审计确认 PostgreSQL、Redis、Sub2API、CLI Proxy、LDXP proxy 和 worker 也没有重建或重启。该历史 reload 不是未来发布步骤。

### 上线验收与清理

- 标准实例为 `running / healthy / restart=0`，`start_time=1784168769`；启动日志中 `panic`、`fatal`、迁移错误均为 0。
- 公网 `/`、`/quick-start`、`/api/status` 均为 HTTP 200；连续三次及最终复核均命中标准实例。公网 bundle 的文件名、字节数和 SHA-256 与本地一致，包含 `gpt-5.6-sol`，不含 `gpt-5.6-sol-thinking`。
- 覆盖切绿与标准重建的公网持续探针 `159/159` 为 200。覆盖两次 Caddy reload 的源站探针共 2400 次，全部收到 HTTP 响应：718 次 200、1682 次由同一来源高频访问主动触发的 429，没有连接失败或 5xx；停止高频探针后 200 恢复。
- 一次本机到 SOCKS5/Cloudflare 的 TLS 握手在收到 HTTP 状态前超时；服务端同时段无 5xx、容器无重启，随后最终请求为 200，故记录为客户端链路抖动，不是服务端 HTTP 错误。
- `.yunbay-deploy-sha` 和 `.yunbay-source-manifest` 已原子更新为实际构建的功能提交、新镜像和四文件组合哈希；运维记录另以纯文档提交推送 GitHub `main`。绿实例、临时环境文件、临时 Caddyfile、构建日志与探针日志已清理。

### 回滚

回滚目录的 `existing-files.txt` 记录部署前已存在的两个文件，`new-files.txt` 记录本轮新增的两个动效模块。源码回滚时恢复备份 `source/` 中的既有文件，并删除部署前不存在的新增文件；镜像回滚使用 `yunbay-new-api:rollback-quick-start-motion-20260715T194510Z`。回滚必须遵循本文顶部唯一允许流程：固定旧镜像、保持 Caddy `new-api:3000` 不变，并在部署锁和有界 watchdog 保护下只重建标准 `new-api`。禁止重启 PostgreSQL、Redis、Caddy、Sub2API、CLI Proxy 或 LDXP 服务。本轮没有数据库变更。

## 2026-07-16 CC Switch Provider 与生图提示词两阶段导入上线

本轮在官网原版 CC Switch 单次只支持一个 `resource` 的边界内，把快速启动现有按钮改为可恢复的两阶段动作：首次导入 Provider，随后同一个按钮切换为“继续导入推荐提示词”。Prompt 固定要求图片请求直接使用 `POST /v1/images/generations` 与 `gpt-image-2`，禁止把图片模型设为 Codex 主聊天模型，并要求 Base64 原图先保存到工作区 `outputs/`。按产品要求，Prompt 会写入与 Provider 相同的真实 API Key；运维记录和测试只使用假 Key，未记录任何真实 Key。

### 发布内容与验证

- 功能提交：`ee953c5954f85e61a4004f0df26f9043e696cc10`（`feat: add staged CC Switch prompt import`），已普通 fast-forward 推送 GitHub `main`。
- Provider 与 Prompt 均从内存中的同一个 `effectiveApiKey` 生成并统一规范化 `sk-` 前缀；完整 Key 不写入 local/session storage、日志、toast 或可见页面，但按明确产品要求会经 Base64 Deep Link 进入 CC Switch，并最终以明文存在用户的 `~/.codex/AGENTS.md`。Base64 只是编码，不是加密。
- Prompt Deep Link 只含 `resource=prompt`、`app=codex`、名称、UTF-8 Base64 正文和 `enabled=true`，不混入 Provider 专属字段。现有“重试”继续只重开 Provider。
- 浏览器初测发现英文长标签会把桌面网格列从 230px 撑到 310px；最终使用 `minmax(0, …)` 固定轨道并允许按钮正文在固定 48px 高度内平衡换行。1280x720 与 390x844 下切换前后宽高一致，无横向溢出、控制台错误或底栏遮挡。
- 本地验证：快速启动测试 `53 pass / 0 fail`；TypeScript、涉及文件 ESLint、Prettier、六语 i18n 同步、生产构建与 `git diff --check` 均通过。六种语言 missing、extras、untranslated 均为 0。

### 历史部署记录（旧蓝绿流程已废止）

> 以下内容只记录 `ee953c59` 当时的真实发布结果。绿实例和 Caddy 动态切流不再是获准的发布或回滚路径。

- 精确、非删除式同步 11 个前端源码/测试/i18n 文件，本地与生产组合 SHA-256 均为 `9d66e3710df70ef1a2522224e2e0f75301995a9b8938b3769dc659c97f3dc26c`。
- 回滚目录：`/opt/new-api/backups/ccswitch-prompt-20260716T042219Z-ee953c59`；旧镜像 `sha256:bd931400a56e9ad0b43c776bbffac26075267a91ac2362c596880b655a4857fa`，回滚标签 `yunbay-new-api:rollback-ccswitch-prompt-20260716T042219Z`。
- 新镜像：`sha256:3db9cb74a065b3aec8dbfae5919c038739c96bdf728b43794491b2fb86dbb91a`，发布标签 `yunbay-new-api:release-ee953c59`。候选绿实例达到 Docker `healthy` 后才切流，内部入口为 `/static/js/index.04106f6b05.js`。
- 历史上曾用临时 Caddyfile 把 upstream 改为 `new-api-green:3000`，随后重建标准 `new-api` 并切回；该动态切流步骤现已废止，不能从本记录复制执行。标准实例当时使用 Compose `--no-deps --force-recreate --no-build` 在 2 秒内重建。
- 正式 Caddyfile SHA-256 始终为 `655d48c14d94372bf2383af094899fc3e3e8c54fe24b4476ba3ae7f887302388`。Caddy 容器 ID、启动时间和 restart=0 不变；PostgreSQL、Redis、Sub2API、CLI Proxy、LDXP proxy 与 worker 均未重启。

### 上线验收与清理

- 标准容器 `a891da9bfa9efbdac84822d5a48ce731cb2e3420a97d23e8ad4ace390e45e725` 为 `running / healthy / restart=0`，应用严重启动日志为 0。
- 公网 `/`、`/quick-start`、`/api/status` 各 3/3 为 200；公网入口 `/static/js/index.04106f6b05.js` 的 SHA-256 为 `211fdddf75fef5856d5f1288c8c6b96585d752d9d77d860d4c8ff3cc2196079c`，字节数 `3064077`，与绿实例一致。
- 切换窗口本机公网探针 180 次中 177 次为 200；编号 8、80、152 三次在收到 HTTP 状态前失败。服务器同窗 Caddy 5xx=0、upstream error=0、应用严重日志=0，三个失败点间隔规律，按本机 SOCKS5/Cloudflare 传输链路失败记录，不记为服务端 5xx。最终独立探针 30/30 为 200。
- `.yunbay-deploy-sha` 与 `.yunbay-source-manifest` 已原子更新到功能提交、新镜像、11 文件数量和组合哈希。绿实例、宿主机/容器临时 Caddyfile及本机构建验证临时文件均已清理。

### 回滚

源码回滚从 `/opt/new-api/backups/ccswitch-prompt-20260716T042219Z-ee953c59/source/` 恢复 `existing-files.txt` 中的 11 个文件；镜像回滚使用 `yunbay-new-api:rollback-ccswitch-prompt-20260716T042219Z`。回滚必须保持 Caddy `new-api:3000` 不变，在部署锁和有界 watchdog 保护下只重建标准 `new-api`；不要重启 Caddy、PostgreSQL、Redis、Sub2API、CLI Proxy 或 LDXP 服务。本轮没有数据库迁移。

## 2026-07-16 快速启动确认顺序与独立生图提示词预览上线

本轮修正快速启动第五页的反馈顺序：Provider 确认后先显示与上方一致的圆形勾选和“已确认导入”，再在下方显示由真实 Prompt 构造器生成的脱敏生图设置预览；确认后的自动滚动也先落到完成状态。运维记录不包含真实 API Key、生产环境变量或用户会话。

### 发布内容

- 功能提交：`f7839a9d5236130590df6c8044978a0f1c729382`；后续 `bc7c3075` 只有计划闭环文档，不改变镜像内容。
- 从生产 `ee953c5954f85e61a4004f0df26f9043e696cc10` 精确、非删除式同步 12 个 `web/default` 源码、测试和六语言文件；文件清单 SHA-256 为 `3019659dddd3fb3a266ed24c5190983dfcb80393d867714c2809373eea2fcf05`。
- 生产入口：`/static/js/index.5ef2da020e.js`；bundle SHA-256 为 `fa94acad0ca79a476a0bb475205424124721e39995536865c03f7b866334b94d`，字节数 `3064387`，与本地测试构建完全一致。

### 固定 upstream 发布结果

- 发布全程没有创建绿实例、临时 Caddyfile或调用 Caddy reload/Admin API。Caddy 文件、容器挂载和运行时配置始终指向 `new-api:3000`。
- 最终新镜像：`sha256:b7af515bac4bfc28de155fa08dfd4f15a2e7f6d2d36d74837c9fcd2b9955472c`；release tag：`yunbay-new-api:release-f7839a9d`。
- 标准容器：`2791f8a56983f9556a362a06e99a7bfa683422a1e36ecddbad0436ec77575f3e`，`running / healthy / restart=0`；最终重建 19 秒完成。
- 回滚目录：`/opt/new-api/backups/quick-start-confirm-order-20260716T131718Z-f7839a9d`；旧镜像 `sha256:3db9cb74a065b3aec8dbfae5919c038739c96bdf728b43794491b2fb86dbb91a`；固定回滚标签 `yunbay-new-api:rollback-quick-start-confirm-order-20260716T131718Z`。
- `.yunbay-deploy-sha` 与 `.yunbay-source-manifest` 已原子更新到功能提交、12 文件清单、新镜像和部署时间。没有数据库迁移。

### 反馈、回滚与中断审计

- 第一次远程执行在 SSH banner 前因本机运维代理出口不在服务器白名单而失败，远端脚本没有执行。一次预备备份又因可选容器缺少 Docker health 字段提前退出；两者均未同步源码、构建或切换服务。
- 第一次实际切换中，watchdog 把旧容器在 Compose 重建时的正常 `exited/unhealthy` 瞬态误判为新镜像失败并自动回滚。旧镜像、旧源码、旧部署标记和 `200/200` 全部确认恢复后，watchdog 改为只对已经运行新镜像的容器执行即时 unhealthy 判定。
- 第二次实际切换中新实例已经 healthy/200，但静态验证使用了会被生产压缩器移除的源码字符串，因此主动请求自动回滚。静态门槛随后改为入口文件名、完整 bundle SHA-256、字节数、四条稳定新文案和旧动作文案缺失。
- Caddy 在第二次切换、该次自动回滚和最终成功切换期间共记录 30 个 HTTP 502，其中 27 个为固定 upstream 下标准容器暂不可连接；三个窗口分别约 10 秒、6 秒和 10 秒，主要影响 `/v1/responses`。Caddy lookup/DNS 错误为 0，Caddy restart=0。最终切换后所有探针恢复 200。

### 最终验收与清理

- 脚本内连续 5 轮和独立连续 10 轮均确认：宿主机 `/api/status`、公网 `/`、`/quick-start` 与 `/api/status` 为 HTTP 200。
- new-api 与 Caddy 均 healthy；Caddy、PostgreSQL、Redis、Sub2API、CLI Proxy、LDXP proxy 和 worker 的容器快照在最终切换前后不变，新实例严重启动日志为 0。
- watchdog 最终状态为 `success`，部署锁已释放；`new-api-green` 数量为 0。远端发布归档、失败尝试的重复解压目录和本机临时验证文件均已清理，失败尝试的最小日志与源码备份保留用于审计。

### 回滚

回滚必须继续使用本文顶部唯一允许流程：获取 `/var/lock/yunbay-new-api-deploy.lock`，把 `yunbay-new-api:rollback-quick-start-confirm-order-20260716T131718Z` 重标为 `yunbay-new-api:prod`，从成功回滚目录恢复 `existing-files.txt` 中的 12 个文件，并在独立 watchdog 保护下只执行 Compose `--no-deps --force-recreate --no-build new-api`。Caddy upstream 始终保持 `new-api:3000`；禁止重启或修改 Caddy、PostgreSQL、Redis、Sub2API、CLI Proxy 和 LDXP 服务。

## 2026-07-16 快速启动再次导入悬停详情上线

本轮把 Provider 已确认后的重复大面板和完成条合并：默认只显示圆形完成标记与灰色“再次导入”，悬停或键盘聚焦时按钮变亮，API、模型、脱敏 Key 与思考强度从完成条内弹性展开；移出组合区或失焦后回弹收起。再次导入保持 `importConfirmed=true`，下方生图设置不会消失。

### 发布内容与验证

- 功能提交：`7bc5e4e7b5ad67969bae0961b0a580670d206382`；后续 `f49f680a` 只收尾计划，不改变镜像内容。
- 生产从 `f7839a9d5236130590df6c8044978a0f1c729382` 精确、非删除式同步 `index.tsx` 与对应源码测试两文件；本地和生产逐文件 SHA-256 一致，清单 SHA-256 为 `67e721d3008b56be4e554cce3e586079c2000c8f696cbdf05c64ed9c0ba78a5c`。
- 本地验证为 54 项快速启动测试全部通过，TypeScript、定向 ESLint、Prettier、六语言同步和生产构建通过；1280x720、390x844、键盘及 reduced-motion 浏览器闭环通过。
- 生产主入口 `/static/js/index.2f46b5d28b.js`：SHA-256 `e2d4051761dfc4a567343fca9cf4411edf125aaa2ddc9abaaf00584de8b7b3e3`，字节数 `3064387`。quick-start chunk `/static/js/async/4963.80d86e7db9.js`：SHA-256 `a095fd82e7bdd86a943d0b3e1ea8e45eccc0927e3f2db5c4b4bdb2308f095fea`，字节数 `46803`；生产重新下载结果与本地构建完全一致。

### 固定 upstream 部署结果

- 发布前确认标准 `new-api`/Caddy healthy，Caddy 文件、挂载及运行时 upstream 都只有 `new-api:3000`，无绿实例。锁为 `/var/lock/yunbay-new-api-deploy.lock`，切换使用独立 60 秒 watchdog；全程未创建绿实例、临时 Caddyfile或调用 Caddy reload/Admin API。
- 首次执行完成构建，新镜像 `sha256:0d948194bc0bf45a5f106a9256d8bc3dbf8c922136628c39457d6c1bf7f88d0e` 已固定为 `yunbay-new-api:release-7bc5e4e7`。新容器约 11 秒 healthy，但验证器误把 Rsbuild 的 `4963:"80d86e7db9"` chunk map 当成完整文件名，主动触发 watchdog；旧源码、旧标记和旧镜像均自动恢复，旧实例重新 healthy/200，没有卡住等待人工输入。
- 修正验证器后重新获取锁、建立新备份并复用同一不可变镜像，不重复构建。最终标准容器 `9d893d41557a2a33e32ee2e09ec45350eae349e600c46bf2915f9b06589ae81b` 在约 12 秒内 healthy，restart count 为 0；watchdog 结果为 `success`。
- 成功切换的一秒探针为 8 次 502 后连续 12 次 200。Caddy 日志中的 502 从 `14:51:20.124Z` 到 `14:51:28.615Z`，约 8.49 秒，共 44 个请求：4 个标准服务名在替换瞬间的 Docker DNS 抖动、39 个新进程监听前的 connection refused、1 个旧连接关闭。14:51:29 起持续恢复 200，Caddy 本身没有重启或改配置。
- 最终远端连续 10 轮确认宿主机 `/api/status` 和公网 `/`、`/quick-start`、`/api/status` 全部为 200；新应用严重启动日志为 0。Caddy、PostgreSQL、Redis、Sub2API、CLI Proxy、LDXP proxy 与 worker 的容器 ID、启动时间和 restart count 在最终切换前后完全一致，无 `new-api-green`。
- `.yunbay-deploy-sha` 与 `.yunbay-source-manifest` 已原子更新到功能提交、两文件清单、新镜像和部署时间。本轮没有数据库迁移，没有修改业务数据、生产环境变量、Caddy 或其它服务。

### 回滚与审计

成功回滚目录：

```text
/opt/new-api/backups/quick-start-reimport-hover-20260716T145117Z-7bc5e4e7
```

固定镜像：

```text
release: yunbay-new-api:release-7bc5e4e7
rollback: yunbay-new-api:rollback-quick-start-reimport-20260716T145117Z
old image: sha256:b7af515bac4bfc28de155fa08dfd4f15a2e7f6d2d36d74837c9fcd2b9955472c
new image: sha256:0d948194bc0bf45a5f106a9256d8bc3dbf8c922136628c39457d6c1bf7f88d0e
```

首次验证器失败备份 `/opt/new-api/backups/quick-start-reimport-hover-20260716T144138Z-7bc5e4e7` 及对应 rollback 标签保留用于审计。需要回滚时，必须获取同一部署锁，从成功回滚目录恢复 `existing-files.txt` 中的两文件，把成功 rollback 标签重标为 `yunbay-new-api:prod`，并在独立 watchdog 保护下只执行 Compose `--no-deps --force-recreate --no-build new-api`。Caddy upstream 始终保持 `new-api:3000`；禁止重启或修改 Caddy、PostgreSQL、Redis、Sub2API、CLI Proxy 和 LDXP 服务。

## 2026-07-17 快速启动单一导入条与生图下一步上线

- 功能提交 `44a3e932857d5f1b0a7abc955a1fa08878fe0161` 已发布；后续 `ef54e007` 只收尾计划。Provider 导入动作后同一组件原位收敛为完成条，生图设置立即作为下一步出现；生图 Prompt 导入后才解锁“进入控制台”。再次导入仍可 hover/focus 展开详情，但不会回退完成态或隐藏生图设置。
- 生产从 `7bc5e4e7` 精确、非删除式同步 `index.tsx` 与源码约束测试两文件。文件 SHA-256 分别为 `6ee31a63fdb442d730b57b04e6632024d412497da5500d42afc13f9afdc25916`、`40ac75b97316c5c0d694a1970c39ce9b0c4ef75311ca2031f26f0cb3cf6332`，组合清单 SHA-256 为 `cfa56c2cf171adc9b6e8bb4b55014be292555fa8a4936b5b9e0be305a0eb1f6a`。本轮没有数据库迁移、环境变量或业务数据变更。
- 新镜像为 `sha256:2ab381744eb207bc1246a31682c39ed3a7d569d7663dc17abe193e3b0717fdf5`，release 标签为 `yunbay-new-api:release-44a3e932`。最终标准容器 `ea163ea454d49d374bdbebd708fdd170d11e884c5d49250a074a1cad87ec8588` 为 `running / healthy / restart=0`，watchdog=`success`。
- 生产入口 `index.e4374b86a3.js` 和 quick-start chunk `4963.937f71286d.js` 的 SHA-256 分别为 `cf673d3ea925820fbc18db9e0c6b55e24bdc2d8aa2f896c4ac5d0dc60573b3b3`、`1addc8ec05f0628f7f708f00dc4a93d5b9089ea74a60e603bcc98c710b83b4d9`，字节数分别为 `3064387`、`47329`，与本地已测试构建一致。
- 切换探针在 `2026-07-16T16:08:38Z` 至 `16:08:46Z` 观测到 502，`16:08:47Z` 起恢复 200。Caddy 日志首末 502 相隔约 8.58 秒，共 25 个请求：23 个新进程监听前的 connection refused、1 个旧连接 EOF、1 个旧连接关闭；没有 DNS/lookup 错误，Caddy 未重启、reload 或改写。
- 最终公网 `/`、`/quick-start`、`/api/status` 连续 10 轮共 30 次全部为 200，新应用严重启动日志为 0。Caddy 文件、挂载和运行时哈希前后相同，upstream 全程只有 `new-api:3000`；PostgreSQL、Redis、Sub2API、CLI Proxy、LDXP proxy 与 worker 的容器 ID、启动时间和 restart count 均未变化。

成功回滚点：

```text
backup: /opt/new-api/backups/quick-start-merged-flow-20260716T160439Z-44a3e932
rollback tag: yunbay-new-api:rollback-quick-start-merged-20260716T160439Z
old image: sha256:0d948194bc0bf45a5f106a9256d8bc3dbf8c922136628c39457d6c1bf7f88d0e
```

两文件源包和完整部署日志已归档到成功备份，顶层传输包、脚本、状态、日志、PID 和 run dir 已清理。回滚必须获取 `/var/lock/yunbay-new-api-deploy.lock`，恢复成功备份中的两文件，把上述 rollback 标签重标为 `yunbay-new-api:prod`，启动独立 watchdog，并且只重建标准 `new-api`。Caddy upstream 始终保持 `new-api:3000`；禁止修改或重启 Caddy、PostgreSQL、Redis、Sub2API、CLI Proxy 或 LDXP 服务。

## 2026-07-17 VIP 历史余额返利、活动邮件与 QQ 临时备用 SMTP

### 返利结果

- 活动切点为 `2026-07-17 11:48:13 +08:00`；只计成功的网站余额直充和 root 管理员正向直接加余额，超值套餐、兑换码、非 root 加额与当前钱包显示额均不作为充值证据。
- 用户已手工处理的 VIP ID `212–256` 被强制排除；本次严格名单为 19 人，充值基数 `$1,095`，30% 返利 `$328.50`，原子增加 `164,250,000 quota`。活动幂等标识为 `campaign_vip_recharge_rebate_20260717_v1`，审计错误为 0。
- 用户名 `647` 对应用户 ID `211`：余额直充 `$30`、root 加额 `$10`，返利基数 `$40`、实际返利 `$12`；此前看到的“300 多”是累计消费，不是余额充值。
- 备份目录为 `/opt/new-api/backups/vip-recharge-rebate-20260717T044732Z/`；数据库 dump SHA-256 为 `d89f6bbd79b4b07440c4f6d1c7fdcf57ee53d392f58f593642322c14d2d18b9e`，`pg_restore --list` 已通过。

### 邮件结果

- 固定批次 manifest 为 `ca5295f8ea361b545b4008e25382a74396a2b68fba4009989d2663ad5d1cb600`：当时共 258 个用户、223 个唯一有效邮箱、35 个缺失邮箱、0 个无效、0 个重复。活动开始后的新注册用户不追加进已持久化批次。
- Resend SMTP 首先一次成功 184 封，随后 39 封统一返回 `550 daily email sending quota`。用户明确授权临时切换历史 QQ SMTP，允许显示个人 QQ 发件身份。
- QQ 配置切换只更新 7 个 SMTP/SystemName options；new-api 单独重启并在 14 秒内恢复 healthy。逐封重新登录方式成功 21 封后触发 QQ `535 login frequency limited`，按 QQ 官方建议冷却超过 15 分钟，再以单个 STARTTLS 会话发送剩余 18 封。
- 最终回执为 `sent=223 / pending=0 / failed=0 / sending=0`；223 个 recipient hash 和规范化邮箱均唯一，223 条成功回执都有 `sent_at` 且 `last_error` 为空。完成时间为 `2026-07-17 15:55:30 +08:00`。

### 临时 SMTP 与自动恢复

- 当前生产 SMTP 临时为 `smtp.qq.com:587 + STARTTLS`，注册、验证码和密码重置均走 QQ。切换后已观察到 5 次验证码请求，HTTP 200 为 5/5，应用 `failed to send email` 为 0。
- 原 Resend 与 QQ 配置快照、SHA-256、控制脚本和审计材料保存在上述 `0600/0700` 备份目录，不含凭据的公开文档不记录 token。控制脚本 SHA-256 为 `a56fbae4c3ea0160cf55f85bd57eb80c9213b04aef0d7b0c77c63746b23aa982`。
- `/etc/cron.d/yunbay-resend-restore-20260718` 将在 `2026-07-18 08:05 +08:00` 后恢复原 Resend 快照。每轮有 120 秒硬超时、最多 12 轮；live SMTP 指纹异常时拒绝覆盖并撤销调度，成功后删除 cron 和计数文件，再验证 new-api healthy、源站和公网 `/api/status` 为 200。
- 最终连续 5 轮源站/公网 `/api/status` 均为 200；new-api、PostgreSQL、Redis、Caddy 均为 healthy/restart=0，应用 panic/fatal 为 0。

## 2026-07-18 Grok 注册事故闭环与手动恢复边界

### 事故根因与现行不变量

- 旧 Grok 容器把 API worker、自动注册 producer、Turnstile Solver 和 Camoufox 放在同一 cgroup；批量补量时同时触发内存、CPU、PID 饱和，最终导致 OOM 和 API 长时间排队。正常 API 请求不是根因。
- 生产 API 已固定为 `grokcli-2api:20260718-registration-isolation-32bb09f`，硬限制 `2 CPU / 2 GiB / 256 PID`；`GROK2API_INLINE_SOLVER=0`、`GROK2API_REG_AUTO_MAINTAIN=0`。注册服务不属于 API 故障域，不能用 `docker compose up` 直接恢复自动补量。
- 用户确认历史流程必须收到明确的“注册”执行指令才开始。只有把“注册”作为明确执行指令的独立消息才算触发；讨论、引用或说明该词不算。没有该触发时不创建邮箱、不启动浏览器、不启动注册 worker；当前不放量、不提高并发、不恢复无人值守 producer。

### 受控单账号验证结果

- 手动 canary 运行目录：`/home/deploy/grok-backups/20260718T134046-history-registration-canary`；候选镜像：`grokcli-2api:20260718-registration-host-fields-2b43e9e`。代码修复提交：`2b43e9e`。
- 运行参数固定为 1 batch / 1 session / 并发 1 / 预取 0 / `restart=no`，注册 cgroup 为 `1 CPU / 1.5 GiB / 220 PID`。邮箱使用历史 MoeMail 配置；验证码使用本地 Solver，未调用外部 YesCaptcha。YYDS 只保留生产配置的原激活槽位。
- 真实结果：`imported=true`，账号总数 `3737 -> 3738`；新认证记录含 refresh token，单账号 probe 为 `ok=true`、`pool_status=normal`、无冷却。
- 注册活跃期 cgroup 峰值约 `1.43 GiB / 213 PID`。保护器连续越过内存/PID软线后停止并清理 canary；这是预期熔断，不是账号注册失败。生产 API 全程 `healthy`、`restart=0`、`OOM=0`。
- 注册期间和收尾后各执行 5 路真实 SSE，均为 `5/5` 业务成功；收尾后 API 约 `726 MiB / 53 PID`，canary Redis 前缀键为 `0`，注册容器、浏览器和临时会话均已删除。
- canary 前保存的注册配置已原路恢复；生产仍不运行注册 worker。下一次只有在收到“注册”触发后，才允许重复同一单账号步骤；任一 API health/restart/OOM、内存或 PID 越线立即停止注册侧。

### 配置修复与回滚

- `EmailRegistrationBody` / `RegistrationConfigBody` 补齐 `moemail_base_url`、`cfmail_base_url`；此前 provider 切换会丢失 MoeMail host 并复用旧 YYDS 地址。55 项相关回归全部通过。
- 服务器源码备份：`/home/deploy/grok-backups/20260718T133820-registration-host-fields-2b43e9e`；候选镜像只用于注册侧验证，禁止替换生产 API 镜像。
- 注册侧回滚只停止/删除 canary 或 `grok-registration`，不停止 API、PostgreSQL、Redis、egress。生产 API 回滚继续使用固定 `20260718T040204-32bb09f-registration-isolation` 备份和原镜像标签，遵循 60 秒有界 watchdog。

### 2026-07-18 14:53 显式单账号注册结果

- 本次收到明确的“启动注册”指令后，只运行一次性 canary：`/home/deploy/grok-backups/20260718T145340-history-registration-canary`，候选镜像 `grokcli-2api:20260718-registration-host-fields-2b43e9e`；参数为 1 batch / 1 session / 并发 1 / 预取 0、`restart=no`、本地 Solver、自动维护关闭。
- 注册成功导入 1 个账号，账号总数 `3738 -> 3739`；新账号单独 probe 为 `ok=true`、`pool_status=normal`、不在冷却。未启动常驻 producer，也未重建 API。
- 注册 cgroup 峰值约 `1.405 GiB / 202 PID`，硬限 `1.5 GiB / 220 PID`；API 活跃期间真实 SSE `5/5`，API 全程 `healthy / restart=0 / OOM=0`。本次没有外部 YesCaptcha/YesChatUp 调用。
- canary 产生的 3 个本轮专属 Redis session/index 键已精确删除为 0；注册容器、浏览器、锁和临时脚本均已清理。审计结果和脱敏 probe 输出保留在上述目录。
- 发现运行时配置快照文件为空，未把它当作“已恢复”；已将当时配置另存为受限审计文件，并用上一轮 YYDS 基线快照手动恢复，复核当前配置为 `mail_provider=yyds`、`captcha_provider=local`、本机 Solver。后续任何 runner 必须在创建 session 前验证快照非空且恢复回读一致，否则立即停止。

### 2026-07-18 两路并发阈值试验结果

- 在用户接受“内存约 37%、CPU 约 25% 余量”的前提下，先完成 dry-run，再启动一次性两路 canary；外部 Docker build 运行期间被前置锁拒绝，未与构建并行。
- canary 目录：`/home/deploy/grok-backups/20260718T153251-dual-registration-canary`；注册总限 `2 CPU / 3 GiB / 440 PID`，错峰 10 秒、预取 0、自动维护关闭、本地 Solver。
- 实测注册侧峰值约 `1.572 GiB / 208 PID`，API+注册合计约 `2.361 GiB`，内存明显未到 5 GiB 上限；但主机 CPU idle 最低 `18%`，低于 25% 守门线，watchdog 以 `host_cpu_guard` 停止注册。
- 账号数保持 `3739`，没有导入账号；API 期间 SSE `5/5`，API `healthy / restart=0 / OOM=0`。配置恢复回读为 `restored_verified`，6 个本轮 Redis 临时键已精确清理为 0。
- 当前生产注册并发保持 1。两路若要再次评估，必须先降低单路 CPU 配额或增加 vCPU；禁止仅因内存尚有余量而直接放开两路。

## 2026-07-19 Grok 永久 refresh 失败收敛与最终 API 稳态

### 修复与发布

- 根因闭环：20 个已持久化 `refresh_invalid=true` 的账号此前在维护扫周期中被重复处理；PostgreSQL 全量账号缓存的并发失效竞态已由 `c84b1f3` 修复。随后 `e7de8cf` 让软清理对已标记账号直接跳过，`8035dba` 修正维护指标，`f6e87e2` 让空扫立即使用基础等待间隔。
- 最终镜像：`grokcli-2api:20260719-refresh-control-f6e87e2`，digest `sha256:4e4cf2954828b7d4bed3c3da035fc77dfc6e43275d63616a77b9181f7deef804`。部署备份：`/home/deploy/grok-backups/20260718T201019Z-refresh-control-f6e87e2/`；固定回滚标签记录在该目录 `deploy.meta`。通过 `/var/lock/grokcli-adaptive-deploy.lock` 和独立 watchdog 只重建 `grokcli-2api`，watchdog=`success`。
- 最终 API 容器：`f5a89c8daea5504da4220eb928ef0f84eae97665b1cb420b6fe8dbeba579a04e`，`healthy / restart=0 / OOM=false`。PostgreSQL、Redis、egress、New API、Caddy、Sub2API、CLI Proxy 和 LDXP 容器均未因本轮重启。

### 运行时稳态

- Token 维护已恢复：`token_maintain_enabled=true`、leader `running=true`；正常周期 `refreshed=5 / attempted=5 / failed=0`，随后无候选空扫为 `refreshed=0 / attempted=0 / failed=0 / deleted=0 / terminal_skip=20`，`next_wait=90s`。20 个终态账号不再调用上游 refresh，也不重复写 PG。
- 数据不变量保持：账号总数 `3739`，`refresh_invalid=20`，账号池 `enabled=3682 / disabled=57`；没有硬删除、账号数漂移或新的 invalid_grant 风暴。
- 模型健康后台已明确暂停（`model_health_enabled=false / running=false`），原因是重启后全量严格 sweep 会长时间占用维护锁并挤压 API；这不影响真实请求的账号选择和 SSE 反馈。注册自动维护仍关闭，注册 worker、浏览器和邮箱 session 均未运行。
- 最终维护开启状态下 5 路真实 SSE 为 `5/5`，每路均有业务内容、finish frame 和 `[DONE]`；`api_guard` 服务端 `local p95=334ms`、错误率 `0`。API 容器约 `441 MiB / 49 PID`，宿主机可用内存约 `5.6 GiB`，无 panic/fatal/OOM；Redis 在途租约键为 `0`。

### 以后操作边界

- 不要把 `GROK2API_TOKEN_MAINTAIN=1` 当作运行时开关；以管理 API 返回的持久化 `token_maintain_enabled` 和 leader 状态为准。多 worker 切换后若请求命中非 leader，重复一次幂等开关请求直到 `local_running=true`，确认 leader 已真正接管。
- 不要直接恢复模型健康全量 sweep 或自动注册来“补数据”。注册仍需新的独立“注册”执行指令，先单账号、并发 1、预取 0；两路试验曾触发 CPU idle 守门线，不得只按内存余量提档。
- 回滚只使用上述备份中的固定旧镜像，在同一部署锁和 watchdog 内只重建 `grokcli-2api`；禁止回滚 PostgreSQL/Redis Volume、账号数据或其它服务。
