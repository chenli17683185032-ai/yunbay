# gpt-image-2 图像链路长期修复实施计划

## 目标与验收指标

- 默认 OpenAI Python/Node SDK 不设置自定义 User-Agent 时，访问 `https://yunbay.xyz/v1` 不再被 Cloudflare 误拦截。
- `GET /v1/models` 返回 200 且包含 `gpt-image-2`。
- `POST /v1/images/generations` 与 `POST /v1/images/edits` 都能到达源站并按图像链路处理。
- `gpt-image-2` 不得进入 `/v1/chat/completions` 或 `/v1/responses`。
- 失败响应可区分边缘拦截、源站鉴权、路由错误、上游拒绝、限流和载荷超限。
- Cloudflare 变更可回滚；应用发布中断时间小于 60 秒。
- 不在代码、计划、运维记录或日志中写入完整 API Key。

## 控制模型与风险

- 对象：Cloudflare -> Caddy -> new-api -> sub2api -> OpenAI 图片上游的完整请求链。
- 控制器：边缘 Skip 规则、源站鉴权/限流、图像模型路由、客户端请求头与错误分类。
- 测量：CF-Ray、源站 Request-ID、渠道/账号 ID、HTTP 状态、Content-Type、模型和端点。
- 扰动：SDK 默认 UA、代理/直连、Cloudflare Bot/WAF 规则、大型 multipart、上游 4xx/429/超时。
- 稳定性优先：先建立不计费的模型列表闭环，再做单次受控生图；403 不重试，429/5xx 才退避重试。

## 实施节点

### 0. 基线与凭证安全

- [x] 复现默认 OpenAI SDK 的 Cloudflare 403。
- [x] 用同一 Key 的 curl 复现 200，并确认模型列表包含 `gpt-image-2`。
- [x] 通过 SDK 覆盖 User-Agent 复现成功。
- [x] 记录 Cloudflare Security Event 的实际规则 ID/产品。
- [ ] 轮换已出现在本地任务/指令中的旧 Key，并改为受控密钥来源。

基线证据（2026-07-18，已脱敏）：Cloudflare Security Analytics 事件命中规则集
`Cloudflare Bot Management rules for all plans`（ruleset `3e677e63d4e9479382576f3fa66279e7`），规则
`Manage AI bots`（rule `7bd01eeccb6b420fa0be30264603a5cb`），动作 `block`。事件中的 UA 为
`OpenAI/Python 2.24.0`，因此这是边缘产品拦截，不是源站鉴权；同一时段另有 Browser Integrity Check
事件。当前 `yunbay.xyz` 安全规则页显示自定义规则、速率限制规则和可配置托管规则均为 0 条。

放行条件：边缘拦截来源可证据化，旧 Key 不再继续使用。

### 1. Cloudflare API 边缘规则

- [ ] 为 `/v1/models`、`/v1/images/generations`、`/v1/images/edits` 建立最小范围 canary Skip 规则。
- [ ] 只跳过实际误拦截的产品，保留 DDoS、源站鉴权、速率限制和日志。
- [ ] 若命中免费版不可 Skip 的 Bot Fight Mode，采用替代架构并记录决策。
- [ ] 用默认 SDK、curl、代理和直连进行矩阵验证。

放行条件：默认 SDK 的 `models.list()` 200，Cloudflare 不再返回 text/plain 403。

待确认的 canary 草稿（尚未保存）：规则名 `OpenAI Images API canary`；表达式为
`(http.host eq "yunbay.xyz" and len(http.request.headers["authorization"]) gt 0 and ((http.request.method eq "GET" and http.request.uri.path eq "/v1/models") or (http.request.method eq "POST" and http.request.uri.path in {"/v1/images/generations" "/v1/images/edits"})))`；动作 `skip`，仅跳过阶段 `http_request_firewall_managed`，并保留匹配日志。Cloudflare 生成的回滚/部署接口为该 zone 的
`PUT /zones/<zone_id>/rulesets/phases/http_request_firewall_custom/entrypoint`，Authorization 令牌由环境变量提供，禁止写入仓库。

### 2. 图像路由契约

- [x] 审计 new-api 的模型能力元数据、渠道选择和 Images 端点映射。
- [x] 强制 `gpt-image-2` 走 generations/edits，不进入 chat/responses。
- [x] 确认 new-api 到 sub2api 的内部路径和模型映射。
- [ ] 让废弃 `cliproxy` 和错误 `/openai/v1` 路径返回明确 JSON 错误，不再返回后台 HTML 200。

放行条件：日志能证明端点、模型、渠道和上游账号一致。当前代码回归覆盖模型识别、渠道测试端点归一化、Chat/Responses 拒绝、Images 接受和默认图像参数；Sub2API 已有 `/v1/images/generations`、`/v1/images/edits` 与 `gpt-image-2` 路由测试。

### 3. 客户端与错误契约

- [ ] 在项目自有客户端提供可配置稳定 User-Agent 兜底；不把伪装 curl 作为长期依赖。
- [ ] 透传/记录 CF-Ray 与 Request-ID（脱敏）。
- [ ] 统一 edge_blocked/auth_rejected/route_mismatch/upstream_policy/rate_limited/payload_too_large 分类。
- [ ] 只对 429/可恢复 5xx 重试，避免重复生图扣费。

放行条件：错误响应能指导下一步，不再出现无上下文的通用 PermissionDeniedError。

### 4. 大图、长连接与计费稳定性

- [ ] 对齐 Cloudflare、Caddy、new-api、sub2api 的请求体和超时限制。
- [ ] 校验多图 multipart 大小，支持必要的流式心跳或异步恢复。
- [ ] 加入请求幂等 ID，断线重试不得重复扣费。
- [ ] 保留 Base64 原图，再生成派生尺寸。

放行条件：标准参考图载荷在限制内，长请求不会静默断开。

### 5. 测试与观测

- [ ] 添加 Python/Node/curl 的端到端合同探针。
- [ ] 添加 generations/edits、单图/多图、代理/直连和错误矩阵。
- [ ] 每分钟免费探测 models；每日一次受控低成本 generation/edit。
- [ ] 建立边缘 403、图片成功率、错误分类和重复扣费告警。

放行条件：连续 24 小时无边缘误拦截，图片成功率达到 99%。

### 6. Canary 发布与运维收尾

- [ ] 先发布源站代码/测试，再启用 Cloudflare canary 规则。
- [ ] 观察 30 分钟后逐步放量，失败自动回滚。
- [ ] 生产部署后更新唯一 `docs/yunbay-maintenance.md` 运维记录。
- [ ] 轮换旧 Key，合并 GitHub `main`，清理临时输出和本地工作文件。

## 变更边界

本轮不修改受保护的项目归属信息，不把 `gpt-image-2` 用于聊天端点，不在仓库写入真实凭证；Cloudflare 控制台/API 变更必须先导出当前规则并保留回滚版本。
