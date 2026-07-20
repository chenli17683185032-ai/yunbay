# Responses 客户端断流计费闭环修复计划

## 1. 背景与已确认事实

- 生产日志 `125646` 在 `/v1/responses` 流式请求中记录为 `client_gone / context canceled`，new-api 得到的输入、输出 token 与 quota 均为 `0`。
- 同一请求对应的 Sub2API 上游账本在客户端断开后继续完成，记录输入 `6,080`、缓存读取 `91,904`、输出 `182`，说明上游真实使用与成本已经发生。
- 2026-07-19 00:30 至 2026-07-20 00:30 的固定窗口内，用户 `180`、令牌 `284`、渠道 `35`、模型 `gpt-5.6-sol` 共 364 次请求；其中 53 次 `client_gone`，52 次 quota 为 0。
- 正常完成的 311 次 new-api 日志可与 Sub2API usage 逐 token 对齐；断流后的 53 条 new-api 日志也与同一客户端指纹下剩余 53 条上游 usage 构成完整闭环。
- 当前 direct Responses 流处理只为 `response.output_text.delta` 累计本地输出；Codex 工具调用主要产生 function-call 事件。客户端在最终 `response.completed.usage` 前断开时，本地输出构建器为空，最终以零 usage 结算。
- 该请求使用钱包计费、无限额令牌并满足信任额度条件，因此预扣旁路生效；这不是套餐免扣或前端显示问题。

## 2. GitHub 与既有工程经验

- 已核对 `QuantumNous/new-api` GitHub 当前 `main`：direct Responses handler 仍只累计文本 delta，尚无 function-call 断流后的精确 usage 修复，直接升级上游不能解决本问题。
- 已核对同仓库 `chat_via_responses.go`：它会累计工具名和参数用于本地估算，证明工具调用事件必须纳入断流处理；但该估算不知道缓存命中量，不适合作为本项目 tiered billing 的精确结算依据。
- 已核对 GitHub issue 搜索，未发现可以直接复用的已合并修复。
- 已核对本仓库 HTTP 生命周期：上游请求由独立 `http.NewRequest` 创建，没有绑定下游 request context；当前真正终止上游读取的是 `StreamScannerHandler` 在 `client_gone` 时返回并关闭 response body。因此可以在不改供应商适配器的前提下，有界地继续读取最终 usage。

## 3. 控制系统抽象

### 3.1 对象与信号

- 对象：上游 Responses 推理过程。
- 控制器：new-api 流扫描、usage 汇总与 `SettleBilling`。
- 测量：`response.completed.usage` 中的输入、缓存和输出 token。
- 执行器：钱包/订阅与令牌 quota 扣减。
- 环境扰动：客户端提前断开、网络取消、上游延迟、SSE 超时和无 usage 异常。

### 3.2 当前失稳点

客户端断开后测量链路被立即切断；控制器把“没有观测到 usage”误当成“实际 usage 为 0”，执行器随后零结算。信任额度旁路又使系统没有可保留的预扣状态，因此扰动可重复转化为漏扣费。

### 3.3 目标闭环

客户端断开只关闭下游输出，不立即切断上游测量；系统在有限时间内继续读取终态 usage，随后用原冻结计费表达式精确结算。若上游在边界内无法终止，必须关闭资源、保留明确错误观测，不能无限等待。

## 4. 设计方案

### 4.1 最小代码变更

1. 在 `RelayInfo` 增加显式的 `DrainOnClientGone` 行为标志，默认 `false`。
2. 仅在 OpenAI direct Responses 流处理器中启用该标志。
3. `StreamScannerHandler` 在该标志开启时：
   - 首次观察到下游 context 取消后记录 `client_gone`；
   - 停止依赖下游连接的 ping；
   - scanner 与 data handler 继续处理上游 SSE；
   - 等待上游 `[DONE]`、EOF、handler stop、空闲超时或有界 drain 超时；
   - 超时后主动取消内部 context 并关闭 response body，不允许无限占用资源。
4. direct Responses data handler 在下游取消后不再写 `gin.ResponseWriter`，但继续解析 `response.completed` 并提取完整 usage。
5. 结算、表达式、倍率、缓存归一化和日志字段沿用现有路径，不引入另一套价格算法。

### 4.2 明确不采用的方案

- 不把估算输入全部按普通输入收费：缓存命中请求会被显著过度收费。
- 不根据 Sub2API 数据库异步补账：会把 new-api 与具体上游实现、数据库和跨服务事务强耦合。
- 不全局改变所有流式接口：本次证据和可重复路径仅指向 direct Responses，先证明最小闭环。
- 不自动追扣历史用户余额：历史关联虽为高置信度，但没有共享 request ID 的事务合同；追扣需要独立授权和审计规则。

## 5. 测试计划与验收指标

### 5.1 回归测试

- 新增流扫描器测试：默认模式在客户端取消后仍立即停止，证明其他渠道行为不变。
- 新增 drain 模式测试：客户端取消后继续处理后续上游事件，最终状态保持 `client_gone`，且函数在上游终止后退出。
- 新增 direct Responses 测试：先发送工具调用事件，再取消客户端，再发送带缓存 usage 的 `response.completed`；断言完整 prompt/cache/completion usage 被恢复，取消后的事件没有继续写给下游。
- 运行 `go test` 定向包、`go test -race` 风险包和 `git diff --check`。

### 5.2 成功标准

- 正常 `/v1/responses` 流行为和计费不变。
- 客户端断开后，上游在 drain 边界内返回 usage 时，消费日志必须满足 `client_gone` 且 `prompt_tokens > 0`、`quota > 0`。
- tiered billing 必须使用真实 cache token，不得退化为把所有输入按非缓存价格收费。
- 默认流处理路径仍在客户端断开后及时关闭，不扩大到无关渠道。
- drain 有绝对时间边界，不能因客户端不操作而永久挂起。

## 6. 实施节点

- [x] 节点 A：生产证据、GitHub 上游与本地生命周期核查。
- [x] 节点 B：先写失败测试，复现 drain 需求和 direct Responses usage 丢失。
- [x] 节点 C：实现最小 drain 控制与下游写抑制。
- [x] 节点 D：定向测试、race 测试、格式与差异检查。
- [x] 节点 E：提交并推送 GitHub `main`。
- [x] 节点 F：生产备份、构建、最短中断部署和健康检查。
- [x] 节点 G：受控真实断流 canary，验证 `client_gone + 非零 usage + 非零 quota`。
- [x] 节点 H：更新本仓库维护记录与本地唯一服务器连接手册，提交并推送最终记录。
- [x] 节点 I：清理本地和服务器临时工件，确认工作区只保留用户原有未跟踪文件。

## 7. 发布与回滚

- 构建期间旧实例继续服务；全程使用 `/var/lock/yunbay-new-api-deploy.lock`。
- 切换前创建源码、镜像、容器和配置基线备份，启动独立 60 秒 watchdog。
- 只重建标准 `new-api`，不修改或重启 Caddy、PostgreSQL、Redis、Sub2API、CLI Proxy、LDXP 或其它服务。
- 允许短暂重启但目标连续不可用小于 1 分钟；任何健康检查失败立即用固定 rollback 镜像恢复标准实例。
- 回滚恢复本次涉及的既有源码文件，删除本次新增测试文件，并重新标记旧镜像；数据库和业务数据不需要迁移或回滚。

## 8. 状态记录

- 2026-07-20：计划建立；节点 A 完成，尚未修改业务代码或生产状态。
- 2026-07-20：节点 B 完成。新增 direct Responses 断流回归测试；当前代码在客户端取消后关闭 upstream pipe，测试稳定失败为 `io: read/write on closed pipe`，与生产根因一致。
- 2026-07-20：节点 C 完成。仅 direct Responses 显式启用 drain；客户端断开后停止下游写入、继续解析上游终态 usage，空闲与绝对 drain 均有边界且绝对上限为 5 分钟。共享 scanner 的停止通道改为 `sync.Once + close`，消除 race 测试发现的关闭/发送竞争；默认路径隔离测试通过。
- 2026-07-20：节点 D 完成。new-api 全部后端包（显式排除独立依赖不完整的 `infra/sub2api/backend`）普通测试通过，`controller`、`service`、全部 `relay/...` 回归通过；新增 client-gone、drain、绝对超时和 direct Responses usage 恢复测试通过；相关 helper/OpenAI race 测试、`go vet`、`gofmt` 与 `git diff --check` 通过。`go test ./...` 仍仅因仓库既有 Sub2API 独立模块缺少其自身依赖而失败；完整 helper race 仍能看到既有测试并行修改全局 timeout/ping/logger 状态的夹具竞争，本次新增路径的定向 race 为通过。
- 2026-07-20：节点 E 完成。修复与计划提交为 `ec4420eb`，已推送 GitHub `main`；用户原有未跟踪计划与 `outputs/` 未纳入提交。
- 2026-07-20：节点 F 开始。生产部署标记为 `e670b30c`；三份目标源码与 `ec4420eb^` 逐字一致，标准 new-api/Caddy healthy、固定 upstream 三层均为 `new-api:3000`、部署锁空闲、无并发发布进程。已重新记录主机重启后的容器基线，开始准备有界备份、候选构建与单服务切换。
- 2026-07-20：节点 F 完成。成功备份为 `/opt/new-api/backups/responses-client-gone-billing-20260719T175931Z-ec4420eb`；旧镜像固定为 `yunbay-new-api:rollback-responses-billing-20260719T175931Z`，新镜像为 `sha256:ae0a0bd862087d0698719c902ac1954895be8ed03eaa4bfc8315f94bce870d88`。只重建标准 new-api，watchdog=`success`；切换探针在约 14 秒窗口内记录 12 次 502，随后源站、首页、快速启动和公网状态 10 轮共 40 次全部为 200。新容器 healthy/restart=0，启动严重日志为 0；Caddy 三层固定 upstream 与所有受保护服务快照前后完全一致。
- 2026-07-20：节点 G 开始。专用根测试令牌 ID `6` 为启用、无限额且不限制模型；canary 请求仍强制生成函数调用，但测试客户端在最早的 `response.created` 流起始事件到达后即主动断开，不识别或拦截工具调用。只记录新消费日志的非敏感结算字段，不输出或写入进程参数中的令牌明文。
- 2026-07-20：节点 G 完成。真实消费日志 `126475` 为 `client_gone`，但准确记录 prompt `4,468`、completion `528`、quota `5,133`，渠道 `35`、模型 `gpt-5.6-sol`、钱包计费且结算错误为 0；测试客户端 curl 以预期的写管道关闭码 `23` 退出。根测试用户同一批次另有两条并发消费，三条 quota `8,698 + 5,133 + 13,877 = 27,708`，与钱包及 used quota 的批量变化 `27,708` 精确相等，证明本 canary 的 `5,133` 已进入真实扣费闭环。生产 new-api 全程 healthy/restart=0，结束后源站和公网状态 5/5 均为 200。
- 2026-07-20：节点 H 开始。将功能范围、部署身份、真实 canary、短暂中断、回滚点和凭据处理边界写入仓库维护记录及桌面唯一服务器连接手册；生产部署标记继续保持功能提交 `ec4420eb`。
- 2026-07-20：节点 H 完成。仓库维护记录与桌面唯一服务器连接手册均已更新，后者权限保持 `0600`；最终记录作为纯文档提交推送 GitHub `main`，生产 `.yunbay-deploy-sha` 不跟随文档提交，继续保持功能提交 `ec4420eb`。
- 2026-07-20：节点 I 完成。detached 发布日志和钱包批次对账已归档到成功备份；服务器顶层发布包、脚本、状态、日志与 run dir 均已删除，锁空闲、绿实例为 0。清理后新容器 healthy/restart=0，release/rollback 标签与审计备份保留，源站和公网状态 5/5 为 200；本机一次性脚本和发布包已删除，只保留用户原有未跟踪计划与 `outputs/`。
