# Sub2 Relay Validity-Only Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Sub2 a validity-only relay layer: keep API key generation, group/account routing, channel/health/control-plane surfaces, and subscription active/expiry checks, while removing Sub2-side billing enforcement from relay hot paths so New API remains the billing authority.

**Architecture:** Keep the existing router/controller/service layout, but split the work into two tightly bounded changes: (1) shrink authentication middleware so it only performs identity, group, IP, and subscription-validity checks; (2) remove duplicated billing admission gates from every relay handler so requests flow directly into routing, account selection, failover, and streaming. Preserve the control-plane endpoints and all existing relay semantics that are unrelated to billing. Reuse `SubscriptionService.GetActiveSubscription(...)` as the only remaining subscription lookup and do not introduce any new rate limiter, Redis hot check, or alternative billing path.

**Tech Stack:** Go 1.26.4, Gin, Ent, existing `handler` / `middleware` / `service` layers, unit tests via `go test`.

---

## 文件结构与职责

### 必改文件

- `infra/sub2api/backend/internal/server/middleware/api_key_auth.go`
  - 保留 API key 提取、用户状态、分组状态、IP 限制。
  - 仅对 subscription-type group 保留 `GetActiveSubscription(...)` 检查。
  - 删除 billing admission、余额、quota、RPM、usage window、window maintenance、`CheckBillingEligibility(...)`。
  - 保留 `ContextKeyAPIKey`、`ContextKeyUser`、`ContextKeyUserRole`、`ContextKeySubscription`、`ContextKeyOpsFallbackAPIKey`、`TouchLastUsed(...)`。

- `infra/sub2api/backend/internal/server/middleware/api_key_auth_google.go`
  - 与标准 middleware 保持同一套“有效性-only”语义。
  - 继续输出 Google 风格错误 envelope。
  - 删除 billing admission、余额、quota、RPM、usage window、window maintenance。

- `infra/sub2api/backend/internal/handler/gateway_handler.go`
  - 删除 `Messages`、`CountTokens`、`Usage`、fallback retry 中的 billing gate。
  - 保留内容审核、sticky session、account selection、failover、streaming。
  - 保留 `billingErrorDetails(...)` 辅助函数，只有在最终没有调用点时再清理。

- `infra/sub2api/backend/internal/handler/gateway_handler_responses.go`
  - 删除 Responses 入口的 billing gate。
  - 保留请求体解析、内容审核、failover、stream-aware error handling。

- `infra/sub2api/backend/internal/handler/gateway_handler_chat_completions.go`
  - 删除 Chat Completions 入口的 billing gate。
  - 保留协议转换与流式处理。

- `infra/sub2api/backend/internal/handler/openai_chat_completions.go`
  - 删除 OpenAI-compatible chat completions 入口的 billing gate。
  - 保留路由选择、账号选择、stream、failover。

- `infra/sub2api/backend/internal/handler/openai_gateway_handler.go`
  - 删除 Responses / Chat Completions / WebSocket 入口中的 billing gate。
  - 保留 SSE / WebSocket / failover / stream aware error 行为。

- `infra/sub2api/backend/internal/handler/openai_embeddings.go`
  - 删除 embeddings 入口的 billing gate。

- `infra/sub2api/backend/internal/handler/openai_images.go`
  - 删除 images 入口的 billing gate。
  - 保留现有图像生成并发控制，不新增新的节流策略。

- `infra/sub2api/backend/internal/handler/gemini_v1beta_handler.go`
  - 删除 Gemini 入口的 billing gate。
  - 保留 Google / Gemini 错误包装与 antigravity 分支。

- `infra/sub2api/backend/internal/server/api_contract_test.go`
  - 锁住控制面 surface：`/api/v1/keys`、`/api/v1/groups/available`、`/api/v1/channels/available`、`/api/v1/channel-monitors`、`/health`。

- `infra/sub2api/backend/internal/server/routes/gateway_test.go`
  - 增加最小 route smoke test，确认 relay 入口没有因为重构丢失。

- `infra/sub2api/backend/internal/server/middleware/api_key_auth_test.go`
  - 增加 active subscription 放行、missing/expired subscription 拒绝、零余额不再阻断 relay 的回归测试。

- `infra/sub2api/backend/internal/server/middleware/api_key_auth_google_test.go`
  - 增加 Google 兼容路径的 active subscription、expired/missing subscription、零余额不再阻断的回归测试。

- `infra/sub2api/backend/internal/handler/gateway_handler_billing_error_test.go`
  - 保留 helper 单元测试。
  - 将“relay admission 会返回余额/配额/RPM 错误”的断言改成“不应再因为 billing gate 拒绝”的断言。

### 明确不改的文件

- `infra/sub2api/backend/internal/service/billing_cache_service.go`
- `infra/sub2api/backend/internal/service/subscription_service.go`
- `infra/sub2api/backend/internal/repository/user_subscription_repo.go`
- `infra/sub2api/backend/internal/handler/available_channel_handler.go`
- `infra/sub2api/backend/internal/handler/channel_monitor_user_handler.go`
- `infra/sub2api/backend/internal/handler/api_key_handler.go`
- `infra/sub2api/backend/internal/server/routes/user.go`
- `infra/sub2api/backend/internal/server/routes/common.go`

---

## Task 1: 先把“保留什么 / 删除什么”钉死在测试里

**Files:**
- `infra/sub2api/backend/internal/server/middleware/api_key_auth_test.go`
- `infra/sub2api/backend/internal/server/middleware/api_key_auth_google_test.go`
- `infra/sub2api/backend/internal/server/routes/gateway_test.go`
- `infra/sub2api/backend/internal/server/api_contract_test.go`
- `infra/sub2api/backend/internal/handler/gateway_handler_billing_error_test.go`

- [ ] **Step 1: 为标准 middleware 写一个“有效订阅放行、零余额不拦”的测试**

  目标测试名：

  ```go
  func TestAPIKeyAuthWithSubscription_AllowsActiveSubscriptionWithoutBillingChecks(t *testing.T)
  ```

  测试要点：
  - 构造 `StatusActive` 的 `User`、`APIKey`、`Group`。
  - `Group.SubscriptionType = service.SubscriptionTypeSubscription`。
  - 让 `subscriptionService.GetActiveSubscription(...)` 返回一个 `StatusActive` 且 `ExpiresAt > time.Now()` 的订阅。
  - 把 `User.Balance = 0`，并把订阅窗口数值设成会触发旧逻辑失败的值。
  - 断言 middleware 仍然通过，`ContextKeyAPIKey`、`ContextKeyUser`、`ContextKeySubscription` 都存在。
  - 断言 `TouchLastUsed(...)` 仍然被调用。

- [ ] **Step 2: 为标准 middleware 写一个“订阅缺失 / 过期就拒绝”的测试**

  目标测试名：

  ```go
  func TestAPIKeyAuthWithSubscription_RejectsMissingOrExpiredSubscription(t *testing.T)
  ```

  测试要点：
  - 构造 subscription-type group。
  - 让 `subscriptionService.GetActiveSubscription(...)` 返回 `service.ErrSubscriptionNotFound`。
  - 断言状态码是 `403 Forbidden`。
  - 断言错误码 / 消息明确表达“当前订阅不可用”。
  - 再补一个过期订阅分支：repo 返回已过期订阅时仍应被当作不可用。

- [ ] **Step 3: 为 Google middleware 写同样的有效性-only 测试**

  目标测试名：

  ```go
  func TestApiKeyAuthWithSubscriptionGoogle_AllowsActiveSubscriptionWithoutBillingChecks(t *testing.T)
  ```

  测试要点：
  - 使用 `x-goog-api-key` 或 `Authorization: Bearer` 触发 `APIKeyAuthWithSubscriptionGoogle(...)`。
  - 构造 `PlatformGemini` 的 subscription-type group。
  - 返回 active 且未过期订阅。
  - 断言上下文中可读到 `ContextKeyAPIKey`、`ContextKeyUser`、`ContextKeySubscription`。
  - 再补一个 `GetActiveSubscription(...)` 返回 not found 的测试，断言 Google 风格错误保持不变。

- [ ] **Step 4: 给路由和 contract 加 smoke test，锁住控制面和 relay 面**

  目标测试名：

  ```go
  func TestGatewayRoutesAndContractsKeepControlPlaneSurfaces(t *testing.T)
  ```

  测试要点：
  - `api_contract_test.go` 里至少覆盖以下路径：
    - `POST /api/v1/keys`
    - `GET /api/v1/keys`
    - `GET /api/v1/groups/available`
    - `GET /api/v1/channels/available`
    - `GET /api/v1/channel-monitors`
    - `GET /health`
  - `gateway_test.go` 里至少覆盖以下 relay 路径不为 404：
    - `/v1/messages`
    - `/v1/messages/count_tokens`
    - `/v1/responses`
    - `/v1/chat/completions`
    - `/v1/embeddings`
    - `/v1/images/generations`
    - `/v1/images/edits`
    - `/backend-api/codex/responses`

- [ ] **Step 5: 先跑现有测试，确认 baseline 里哪些地方仍在依赖 billing gate**

  Run:

  ```bash
  cd /Users/ethan/Documents/yunbay/infra/sub2api/backend
  GO_BIN="go"
  "$GO_BIN" test ./internal/server/middleware -run 'TestAPIKeyAuthWithSubscription|TestApiKeyAuthWithSubscriptionGoogle' -count=1
  "$GO_BIN" test ./internal/server -run 'TestAPIContracts|TestGatewayRoutes' -count=1
  "$GO_BIN" test ./internal/handler -run 'TestGatewayHandlerBilling|TestBillingErrorDetails' -count=1
  ```

  Expected:
  - 当前代码里仍会暴露旧 billing gate 相关失败点；
  - 这些失败点将作为后续最小修改的目标。

---

## Task 2: 收敛 middleware，只保留“有效性检查”

**Files:**
- `infra/sub2api/backend/internal/server/middleware/api_key_auth.go`
- `infra/sub2api/backend/internal/server/middleware/api_key_auth_google.go`

- [ ] **Step 1: 把标准 middleware 收敛到四层校验**

  只保留以下顺序：

  1. API key 提取与查找；
  2. key / user / user active / group active / IP 限制；
  3. subscription-type group 的 `GetActiveSubscription(...)`；
  4. 写入上下文并 `TouchLastUsed(...)`。

  需要删除的具体分支：
  - `skipBilling`；
  - `ValidateAndCheckLimits(...)`；
  - `DoWindowMaintenance(...)`；
  - `Balance <= 0`；
  - `QuotaExhausted` / `QuotaExpired`；
  - `RateLimit` / `RPM` 检查；
  - `CheckBillingEligibility(...)` 的整个前置逻辑。

  目标结构参考：

  ```go
  if apiKey.Group != nil && apiKey.Group.IsSubscriptionType() && subscriptionService != nil {
      subscription, err := subscriptionService.GetActiveSubscription(
          c.Request.Context(),
          apiKey.User.ID,
          apiKey.Group.ID,
      )
      if err != nil {
          AbortWithError(c, http.StatusForbidden, "SUBSCRIPTION_NOT_FOUND", "No active subscription found for this group")
          return
      }
      c.Set(string(ContextKeySubscription), subscription)
  }
  ```

- [ ] **Step 2: 把 Google middleware 收敛到同一语义，但保留 Google 风格错误包装**

  目标结构同样只保留：
  - API key 提取；
  - user / group / IP；
  - `GetActiveSubscription(...)`；
  - `TouchLastUsed(...)`。

  需要删除：
  - 余额检查；
  - quota 检查；
  - RPM 检查；
  - window maintenance；
  - `CheckBillingEligibility(...)` / `ValidateAndCheckLimits(...)`。

  错误返回仍然走 Google envelope：

  ```go
  c.JSON(status, gin.H{
      "error": gin.H{
          "code":    status,
          "message": message,
          "status":  googleapi.HTTPStatusToGoogleStatus(status),
      },
  })
  ```

- [ ] **Step 3: 清理不再需要的 import 和局部变量**

  在两个 middleware 文件里删除只服务于旧 billing gate 的内容，例如：
  - `errors.Is` 对 quota / RPM 分支的依赖；
  - `subscriptionService.ValidateAndCheckLimits(...)` 相关变量；
  - `needsMaintenance`、`maintenanceCopy`；
  - `skipBilling`；
  - 多余的 `strconv` / `time` / billing error helper 调用。

  保留：
  - `SetOpsFallbackAPIKey(...)`；
  - `setGroupContext(...)`；
  - `TouchLastUsed(...)`；
  - `ContextKeySubscription` 写入。

- [ ] **Step 4: 跑 middleware 单测，验证旧 billing gate 已不再参与鉴权**

  Run:

  ```bash
  cd /Users/ethan/Documents/yunbay/infra/sub2api/backend
  GO_BIN="go"
  "$GO_BIN" test ./internal/server/middleware -run 'TestAPIKeyAuthWithSubscription|TestApiKeyAuthWithSubscriptionGoogle|TestRequireGroupAssignment' -count=1
  ```

  Expected:
  - active subscription 通过；
  - missing / expired subscription 拒绝；
  - 零余额不再阻断请求；
  - Google 兼容路径保持错误格式不变。

---

## Task 3: 删除所有 relay handler 中的 billing admission

**Files:**
- `infra/sub2api/backend/internal/handler/gateway_handler.go`
- `infra/sub2api/backend/internal/handler/gateway_handler_responses.go`
- `infra/sub2api/backend/internal/handler/gateway_handler_chat_completions.go`
- `infra/sub2api/backend/internal/handler/openai_chat_completions.go`
- `infra/sub2api/backend/internal/handler/openai_gateway_handler.go`
- `infra/sub2api/backend/internal/handler/openai_embeddings.go`
- `infra/sub2api/backend/internal/handler/openai_images.go`
- `infra/sub2api/backend/internal/handler/gemini_v1beta_handler.go`
- `infra/sub2api/backend/internal/handler/gateway_handler_billing_error_test.go`

- [ ] **Step 1: 在每个 relay entrypoint 删除 `CheckBillingEligibility(...)`**

  具体位置要逐个清除：
  - `GatewayHandler.Messages`
  - `GatewayHandler.CountTokens`
  - `GatewayHandler.Responses`
  - `GatewayHandler.ChatCompletions`
  - `OpenAIChatCompletions`
  - `OpenAIGatewayHandler.Responses`
  - `OpenAIGatewayHandler.ChatCompletions`
  - `OpenAIGatewayHandler.WebSocket`
  - `OpenAIGatewayHandler.Embeddings`
  - `OpenAIGatewayHandler.Images`
  - `GeminiV1BetaModels`

  删除后这些入口只保留：
  - context 读取；
  - 内容审核；
  - session hash；
  - account selection；
  - failover；
  - stream aware / websocket aware error handling。

- [ ] **Step 2: 删除 `Messages` fallback 中的二次 billing 检查**

  这段 fallback 逻辑当前会在切换到 fallback group 前再次调用 `CheckBillingEligibility(...)`。
  需要改成只检查：
  - fallback group 是否存在；
  - platform / subscription-type 是否满足 fallback 语义；
  - 允许本次 retry 的普通路由条件。

  不要引入新的计费判断，也不要把 fallback 变成新的 rejection gate。

- [ ] **Step 3: 保留 relay 语义，不动与 billing 无关的逻辑**

  明确保留：
  - 请求体解析；
  - 内容审核；
  - sticky session；
  - account selection / scheduler / routing；
  - failover；
  - Streaming / SSE / WebSocket 写法；
  - 图像生成既有并发控制；
  - `Retry-After` 只保留给真实的上游 / 协议错误，不再由 billing gate 触发。

- [ ] **Step 4: 清理因移除 billing gate 而失效的 import 与局部变量**

  常见会被删掉的内容：
  - `subscription` 仅为 billing gate 准备的局部变量；
  - `requestCtx` 中专门给 billing gate 用的分支；
  - `billingErrorDetails(...)` 的调用点；
  - `strconv` 等只用于 billing 的 `Retry-After` 头设置。

  `billingErrorDetails(...)` 函数本身先保留，等全部调用点都清空后再决定是否删除。

- [ ] **Step 5: 跑 relay handler 单测，验证现在不会再因为 billing 拒绝**

  Run:

  ```bash
  cd /Users/ethan/Documents/yunbay/infra/sub2api/backend
  GO_BIN="go"
  "$GO_BIN" test ./internal/handler -run 'TestGateway.*|TestOpenAI.*|TestGemini.*|TestBillingErrorDetails' -count=1
  ```

  Expected:
  - 订阅有效但余额 / quota 为 0 的请求仍可进入 relay；
  - 订阅失效的请求在 middleware 阶段就被拒绝；
  - handler 不再因为 billing error 返回 403 / 429 / Retry-After。

---

## Task 4: 锁住控制面 contract，避免误删 surface

**Files:**
- `infra/sub2api/backend/internal/server/api_contract_test.go`
- `infra/sub2api/backend/internal/server/routes/gateway_test.go`
- `infra/sub2api/backend/internal/handler/available_channel_handler_test.go`
- `infra/sub2api/backend/internal/handler/channel_monitor_user_handler_test.go`

- [ ] **Step 1: 在 contract test 中明确保住控制面接口**

  需要覆盖的路径至少包括：
  - `POST /api/v1/keys`
  - `GET /api/v1/keys`
  - `GET /api/v1/groups/available`
  - `GET /api/v1/channels/available`
  - `GET /api/v1/channel-monitors`
  - `GET /health`

  如果已有断言，补到更明确；如果没有，就加最小 smoke path 断言，不扩大范围。

- [ ] **Step 2: 给 `available-channel` 和 `channel-monitor` 保留只读视图测试**

  这两个 handler 本来就是控制面读接口，不应依赖 relay billing gate。

  如果重构影响到了它们，只补测试，不改业务逻辑。

- [ ] **Step 3: 再跑一轮 contract + relay 的组合测试**

  Run:

  ```bash
  cd /Users/ethan/Documents/yunbay/infra/sub2api/backend
  GO_BIN="go"
  "$GO_BIN" test ./internal/server -run 'TestAPIContracts|TestGatewayRoutes' -count=1
  "$GO_BIN" test ./internal/handler -run 'TestGateway|TestOpenAI|TestGemini|TestAvailableChannel|TestChannelMonitorUser' -count=1
  ```

  Expected:
  - 控制面不变；
  - relay 路径没有因为删 billing gate 而消失；
  - 用户可见的渠道 / 监控接口仍可正常工作。

---

## Task 5: 最终回归与收尾

**Files:**
- 主要是测试文件；仅在残余 import 或死代码显然无调用时再做最小清理。

- [ ] **Step 1: 做一轮完整回归**

  Run:

  ```bash
  cd /Users/ethan/Documents/yunbay/infra/sub2api/backend
  GO_BIN="go"
  "$GO_BIN" test ./internal/server/middleware ./internal/server ./internal/handler ./internal/service -count=1
  ```

  Expected:
  - 新的 middleware 测试通过；
  - relay handler 测试通过；
  - contract / route smoke test 通过；
  - service 层不需要为本次变更额外修改。

- [ ] **Step 2: 手工核对最终边界**

  必须确认：
  1. API key 还能创建；
  2. key 仍能落到正确的 group；
  3. group 仍能映射到正确的 upstream account pool；
  4. `/health` 仍然活着；
  5. `/api/v1/channels/available` 与 `/api/v1/channel-monitors` 仍可提供控制面信息；
  6. relay 请求不再因为 Sub2 自己的 billing 逻辑被拒绝；
  7. 只有“订阅失效”这种最小有效性问题会在 Sub2 被挡住。

- [ ] **Step 3: 写清回滚顺序**

  如果线上出现问题，按这个顺序回滚：
  1. 先回滚 `api_key_auth.go` 与 `api_key_auth_google.go`；
  2. 再回滚各 relay handler 中删除的 billing gate；
  3. 保留控制面接口；
  4. 重启服务即可恢复。

---

## 验收标准

以下条件全部满足时，这个实施计划才算完成：

1. Sub2 relay 热路径不再调用 `CheckBillingEligibility(...)`；
2. Sub2 不再因为余额、quota、RPM、subscription usage window 这类计费条件拒绝 relay 请求；
3. 订阅型分组仍然要求 active 且未过期的订阅；
4. API key / user / group / IP 的基础鉴权仍然存在；
5. API key 生成、分组路由、账号池分发、渠道状态、健康检查仍然可用；
6. `/api/v1/keys`、`/api/v1/groups/available`、`/api/v1/channels/available`、`/api/v1/channel-monitors`、`/health` 不被误删；
7. 没有新增 stream limiter；
8. 没有新增重 Redis 热检查；
9. 没有把 Sub2 重新做成完整计费网关；
10. 成功请求的协议形态保持不变，只有拒绝条件收敛。

---

## 参考规范

- `docs/superpowers/specs/2026-06-25-sub2-relay-validity-only-design.md`
- `docs/yunbay-maintenance.md`
- `infra/sub2api/README.md`
