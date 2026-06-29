# LDXP Browser Worker 自动充值 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在云贝钱包页提供 10/20/30/50/100/500 元固定金额入口，由服务器端浏览器 Worker 打开联动小铺商品链接、生成支付宝二维码、等待支付结果页、结合 QQ 邮箱购买成功邮件核验订单，并在核验通过后自动调用云贝可信卡密兑换完成充值。

**Architecture:** 后端负责 session 状态机、用户 API、内部 Worker API、邮件事件落库、订单核验和最终入账；独立 Node/Playwright Worker 负责打开联动小铺前台、回传二维码、提取支付结果页，并通过 IMAP 读取 `10256345@qq.com` 收到的联动小铺购买成功邮件后回传邮件事件。前端只展示固定金额按钮、二维码弹窗和轮询状态，不接触联动小铺登录态、IMAP 授权码、Worker token 或完整卡密。

**Tech Stack:** Go 1.25.1、Gin、GORM v2、SQLite/MySQL/PostgreSQL 兼容、React 19、TypeScript、Rsbuild、Base UI/shadcn-style 组件、Tailwind CSS、Bun、Node.js、Playwright、imapflow、mailparser、Docker Compose。

---

## 0. Context / 已确认上下文

- Spec：`/Users/ethan/Documents/yunbay/docs/superpowers/specs/2026-06-28-ldxp-browser-worker-auto-topup-design.md`。
- Spec commits：
  - `ca1ee365 docs: add ldxp browser worker auto topup spec`
  - `43e4ae43 docs: add ldxp implementation prerequisites`
- 当前仓库：`/Users/ethan/Documents/yunbay`。
- GitHub remote：`https://github.com/chenli17683185032-ai/yunbay.git`。
- 当前写计划时分支：`codex/fix-usage-logs-stat-null`，本地相对 `origin/codex/fix-usage-logs-stat-null` ahead 5。
- 远端 `main` 写计划时为 `5a2586e5 docs: record yunbay quick start deployment`。
- 当前工作区有其它未提交改动；执行本计划时必须使用隔离分支或 worktree，不要覆盖无关文件。
- 生产凭据、服务器连接信息、Cloudflare、邮箱和支付平台资料以本机 `/Users/ethan/Desktop/云贝` 文件夹为准；不得把密码、授权码、Token、私钥或完整连接密钥写入 Git、日志、截图、PR、issue 或聊天。
- 生产环境已知事实来自既有 spec：Docker Compose + PostgreSQL + Redis + Caddy，应用目录 `/opt/new-api/app`，配置 secret 文件 `/opt/new-api/secrets/prod.env`。本计划只引用路径和变量名，不记录 secret 明文。
- 所有金额按钮初期都可绑定 `https://pay.ldxp.cn/item/n4aqh8` 做测试；正式上线前必须替换为每个金额对应的正式商品链接，或保持功能关闭。
- QQ 邮箱链路：`support@yunbay.xyz -> Cloudflare Email Routing -> 10256345@qq.com`。
- 本计划依赖“链动发卡网卡密兑换”能力：`paid_topup` 卡密兑换成功时创建成功充值记录。相关计划在 `/Users/ethan/Documents/yunbay/docs/superpowers/plans/2026-06-27-ldxp-card-redemption.md`，当前本地已有完成分支 `/Users/ethan/Documents/yunbay/.worktrees/ldxp-card-redemption`，HEAD `8f4b4a9b fix: complete redemption card frontend integration`。

---

## 1. File Structure / 变更文件清单

### 1.1 后端模型与迁移

- Create: `/Users/ethan/Documents/yunbay/model/ldxp_topup.go`
  - `LdxpTopupSession` GORM 模型。
  - `LdxpMailEvent` GORM 模型。
  - 状态常量、金额配置结构、查询/更新 helper。
  - 表名固定为 `ldxp_topup_sessions`、`ldxp_mail_events`。
- Modify: `/Users/ethan/Documents/yunbay/model/main.go`
  - `AutoMigrate` 和 fast migration 列表加入 `&LdxpTopupSession{}`、`&LdxpMailEvent{}`。
- Create: `/Users/ethan/Documents/yunbay/model/ldxp_topup_test.go`
  - SQLite 内存库测试模型迁移、唯一 session、邮件去重、状态更新、过期查询。

### 1.2 后端业务服务

- Create: `/Users/ethan/Documents/yunbay/service/ldxp_config.go`
  - 从环境变量解析功能开关、商品链接、联系邮箱、Worker token、超时、并发限制。
  - Go 业务代码解析 JSON 必须使用 `/Users/ethan/Documents/yunbay/common/json.go` 的 `common.UnmarshalJsonStr`。
- Create: `/Users/ethan/Documents/yunbay/service/ldxp_session.go`
  - 创建 session、claim worker job、记录二维码、记录 Worker 结果、取消/过期处理。
- Create: `/Users/ethan/Documents/yunbay/service/ldxp_mail.go`
  - 邮件解析纯函数、邮件事件落库、订单号/card/金额匹配。
- Create: `/Users/ethan/Documents/yunbay/service/ldxp_verify.go`
  - Worker 结果 + 邮件事件交叉核验。
  - 调用 `model.Redeem(cardKey, userID)` 完成 `paid_topup` 入账。
  - 幂等防重：同一 `session_id`、`worker_order_no`、`mail_order_no`、`worker_card_key` 只能成功入账一次。
- Create: `/Users/ethan/Documents/yunbay/service/ldxp_*_test.go`
  - 分别覆盖 config、session 状态机、mail parser、verify/redeem 幂等。

### 1.3 后端 HTTP API

- Create: `/Users/ethan/Documents/yunbay/controller/ldxp_topup.go`
  - 用户 API：创建、查询、取消 session。
  - 内部 Worker API：claim job、回传二维码、回传支付结果、回传错误、回传邮件事件。
  - 管理 API：查询 session 列表、重试核验、标记人工处理。
- Modify: `/Users/ethan/Documents/yunbay/router/api-router.go`
  - 注册 `/api/user/ldxp/topup/*` 用户路由。
  - 注册 `/api/ldxp/worker/*` 内部 Worker 路由，使用 Worker token 校验。
  - 注册 `/api/ldxp/admin/*` 管理路由，使用 `middleware.AdminAuth()`。
- Create: `/Users/ethan/Documents/yunbay/controller/ldxp_topup_test.go`
  - 覆盖鉴权、创建 session、查询状态、Worker token 拒绝、二维码回传、结果回传。

### 1.4 前端 default 钱包页

- Modify: `/Users/ethan/Documents/yunbay/web/default/src/features/wallet/types.ts`
  - 增加 LDXP 配置、session、状态响应类型。
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/features/wallet/api.ts`
  - 增加 `createLdxpTopupSession`、`getLdxpTopupSession`、`cancelLdxpTopupSession`。
- Create: `/Users/ethan/Documents/yunbay/web/default/src/features/wallet/hooks/use-ldxp-topup.ts`
  - 封装创建、轮询、取消、超时处理。
- Create: `/Users/ethan/Documents/yunbay/web/default/src/features/wallet/lib/ldxp-topup.ts`
  - 固定金额、状态文案、轮询策略、过期判断纯函数。
- Create: `/Users/ethan/Documents/yunbay/web/default/src/features/wallet/lib/ldxp-topup.test.ts`
  - Node 测试固定金额、状态映射、轮询停止条件。
- Create: `/Users/ethan/Documents/yunbay/web/default/src/features/wallet/components/ldxp-topup-card.tsx`
  - 展示 10/20/30/50/100/500 六个金额按钮。
- Create: `/Users/ethan/Documents/yunbay/web/default/src/features/wallet/components/ldxp-payment-dialog.tsx`
  - 展示二维码、订单状态、倒计时、取消按钮、成功/失败结果。
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/features/wallet/index.tsx`
  - 在 `RechargeFormCard` 附近挂载 LDXP 充值卡片和弹窗。
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/en.json`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/zh.json`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/fr.json`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/ja.json`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/ru.json`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/vi.json`
  - 新增 UI 文案翻译，并运行 `bun run i18n:sync`。

### 1.5 Worker 服务

- Create directory: `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/`
- Create: `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/package.json`
- Create: `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/tsconfig.json`
- Create: `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/Dockerfile`
- Create: `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/src/config.ts`
- Create: `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/src/backend.ts`
- Create: `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/src/browser-flow.ts`
- Create: `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/src/mail-poller.ts`
- Create: `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/src/mail-parser.ts`
- Create: `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/src/redact.ts`
- Create: `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/src/index.ts`
- Create: `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/tests/*.test.ts`
  - Worker 单元测试只用 HTML fixture 和邮件 fixture，不真实付款。

### 1.6 部署与运维

- Do not modify: `/Users/ethan/Documents/yunbay/docker-compose.yml`
  - 本计划使用 Worker 目录内的 compose example；根 compose 不纳入本轮，避免影响现有本地开发服务。
- Create: `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/docker-compose.example.yml`
  - 展示 Worker 与后端联调方式。
- Create: `/Users/ethan/Documents/yunbay/docs/ldxp-browser-worker-auto-topup-runbook.md`
  - 生产配置、上线、回滚、排障、手工处理流程。
- Do not modify: `/Users/ethan/Documents/yunbay/docs/yunbay-maintenance.md`
  - 当前该文件已有未提交改动；本计划不触碰，避免覆盖无关维护文档。

---

## 2. Runtime Contracts / 合同定义

### 2.1 后端配置项

后端读取以下环境变量：

```bash
LDXP_AUTO_TOPUP_ENABLED=false
LDXP_CONTACT_EMAIL=support@yunbay.xyz
LDXP_TOPUP_PRODUCTS_JSON='[{"amount":10,"money":10,"product_url":"https://pay.ldxp.cn/item/n4aqh8","product_name":""},{"amount":20,"money":20,"product_url":"https://pay.ldxp.cn/item/n4aqh8","product_name":""},{"amount":30,"money":30,"product_url":"https://pay.ldxp.cn/item/n4aqh8","product_name":""},{"amount":50,"money":50,"product_url":"https://pay.ldxp.cn/item/n4aqh8","product_name":""},{"amount":100,"money":100,"product_url":"https://pay.ldxp.cn/item/n4aqh8","product_name":""},{"amount":500,"money":500,"product_url":"https://pay.ldxp.cn/item/n4aqh8","product_name":""}]'
LDXP_WORKER_TOKEN_FILE=/run/secrets/ldxp_worker_token
LDXP_SESSION_TTL_SECONDS=1200
LDXP_QR_TTL_SECONDS=300
LDXP_WORKER_JOB_TIMEOUT_SECONDS=900
LDXP_MAIL_MATCH_WINDOW_SECONDS=1800
LDXP_REQUIRE_MAIL_MATCH=true
LDXP_DEBUG_SNAPSHOT_DIR=/opt/new-api/logs/ldxp-worker/snapshots
```

规则：

- `LDXP_AUTO_TOPUP_ENABLED=false` 时前端不展示自动充值入口，后端创建 API 返回功能未开启。
- `LDXP_TOPUP_PRODUCTS_JSON` 必须正好包含 `10/20/30/50/100/500` 六个金额；测试期可以全部指向同一链接，正式期必须逐个换成真实金额商品链接。
- `LDXP_WORKER_TOKEN_FILE` 优先级高于 `LDXP_WORKER_TOKEN`；生产推荐 secret file，不推荐把 token 直接写 env。
- 回复、日志、错误消息中只输出 token 文件是否存在，不输出 token 内容。

### 2.2 Worker 配置项

Worker 读取以下环境变量：

```bash
BACKEND_BASE_URL=http://yunbay-new-api:3000
LDXP_WORKER_TOKEN_FILE=/run/secrets/ldxp_worker_token
LDXP_WORKER_ID=ldxp-worker-1
LDXP_WORKER_POLL_INTERVAL_MS=2000
LDXP_WORKER_CONCURRENCY=2
LDXP_BROWSER_HEADLESS=true
LDXP_BROWSER_SNAPSHOT_DIR=/app/snapshots
LDXP_BROWSER_TIMEOUT_MS=900000
LDXP_CONTACT_EMAIL=support@yunbay.xyz
QQ_IMAP_HOST=imap.qq.com
QQ_IMAP_PORT=993
QQ_IMAP_SECURE=true
QQ_IMAP_USER=10256345@qq.com
QQ_IMAP_PASSWORD_FILE=/run/secrets/qq_imap_password
QQ_IMAP_MAILBOX=INBOX
QQ_IMAP_POLL_INTERVAL_MS=10000
```

规则：

- Worker 使用 `X-LDXP-Worker-Token` 调用后端内部 API。
- Worker 日志必须对卡密、邮箱授权码、Worker token、二维码内容做脱敏。
- Worker 失败截图和 HTML 快照写入 `LDXP_BROWSER_SNAPSHOT_DIR`，只回传相对路径和摘要，不回传完整 HTML。

### 2.3 Session 状态机

```text
created -> worker_claimed -> qr_ready -> worker_paid -> verified -> redeemed -> success
created -> canceled
qr_ready -> canceled
created/worker_claimed/qr_ready/worker_paid -> expired
worker_claimed/qr_ready -> worker_failed
worker_paid -> mail_timeout
worker_paid -> verify_failed
verified -> redeem_failed
```

前端展示状态映射：

```text
created / worker_claimed              => 正在创建支付二维码
qr_ready                              => 请扫码支付
worker_paid                           => 已支付，正在核验邮件
verified / redeemed                   => 已核验，正在入账
success                               => 充值成功
canceled                              => 已取消
expired                               => 支付超时
worker_failed / mail_timeout / verify_failed / redeem_failed => 充值失败，请联系客服处理
```

### 2.4 用户 API

```http
POST /api/user/ldxp/topup/session
Content-Type: application/json

{"amount":10}
```

成功响应：

```json
{
  "success": true,
  "message": "",
  "data": {
    "session_id": "ldxp_260628_abcdef123456",
    "amount": 10,
    "money": 10,
    "status": "created",
    "expires_at": 1782580000,
    "poll_interval_ms": 2000
  }
}
```

查询：

```http
GET /api/user/ldxp/topup/session/ldxp_260628_abcdef123456
```

查询成功响应：

```json
{
  "success": true,
  "message": "",
  "data": {
    "session_id": "ldxp_260628_abcdef123456",
    "amount": 10,
    "money": 10,
    "status": "qr_ready",
    "qr_code": "data:image/png;base64,...",
    "worker_order_no": "LD260628UZJ97P",
    "expires_at": 1782580000,
    "error_code": "",
    "error_message": ""
  }
}
```

取消：

```http
POST /api/user/ldxp/topup/session/ldxp_260628_abcdef123456/cancel
```

### 2.5 Worker API

```http
POST /api/ldxp/worker/sessions/claim
X-LDXP-Worker-Token: secret
Content-Type: application/json

{"worker_id":"ldxp-worker-1"}
```

二维码回传：

```http
POST /api/ldxp/worker/sessions/{session_id}/qr
X-LDXP-Worker-Token: secret
Content-Type: application/json

{
  "worker_id":"ldxp-worker-1",
  "worker_order_no":"LD260628UZJ97P",
  "worker_amount":0.10,
  "worker_product_name":"0.1 元测试",
  "qr_code":"data:image/png;base64,...",
  "qr_page_url":"https://excashier.alipay.com/standard/auth.htm?payOrderId=..."
}
```

结果回传：

```http
POST /api/ldxp/worker/sessions/{session_id}/result
X-LDXP-Worker-Token: secret
Content-Type: application/json

{
  "worker_id":"ldxp-worker-1",
  "worker_order_no":"LD260628UZJ97P",
  "worker_amount":0.10,
  "worker_product_name":"0.1 元测试",
  "worker_card_key":"redacted-in-logs-only",
  "worker_status_text":"已付款",
  "worker_success_url":"https://pay.ldxp.cn/order/result/LD260628UZJ97P"
}
```

邮件事件回传：

```http
POST /api/ldxp/worker/mail-events
X-LDXP-Worker-Token: secret
Content-Type: application/json

{
  "message_id":"mail-message-id",
  "imap_uid":"uidvalidity:uid",
  "raw_hash":"sha256hex",
  "mail_from":"联动小铺 <no-reply@example.invalid>",
  "mail_to":"support@yunbay.xyz",
  "subject":"购买成功通知",
  "received_time":1782580000,
  "order_no":"LD260628UZJ97P",
  "amount":0.10,
  "product_name":"0.1 元测试",
  "card_key":"redacted-in-logs-only",
  "paid_time":1782580000,
  "body_excerpt":"脱敏摘要"
}
```

---

## 3. Task 0: 实施前置核验、隔离分支与凭据边界

**Files:**
- Read only: `/Users/ethan/Documents/yunbay/.git/config`
- Read only: `/Users/ethan/Documents/yunbay/docs/superpowers/specs/2026-06-28-ldxp-browser-worker-auto-topup-design.md`
- Read only paths list only: `/Users/ethan/Desktop/云贝`

- [ ] **Step 1: 查看 GitHub yunbay 项目状态**

Run:

```bash
cd /Users/ethan/Documents/yunbay
git fetch origin
git remote -v
git branch --show-current
git status --short
git branch -vv
git log --oneline --decorate --graph --max-count=30 --all
git ls-remote --heads origin
```

Expected:

- 能看到 `origin https://github.com/chenli17683185032-ai/yunbay.git`。
- 明确当前工作分支、远端 `main`、当前分支与 upstream 的 ahead/behind。
- 如果工作区存在无关未提交改动，不在原工作区直接实现。

- [ ] **Step 2: 创建隔离 worktree 或隔离分支**

推荐 worktree：

```bash
cd /Users/ethan/Documents/yunbay
git worktree add /Users/ethan/Documents/yunbay/.worktrees/ldxp-browser-worker-auto-topup -b codex/ldxp-browser-worker-auto-topup HEAD
cd /Users/ethan/Documents/yunbay/.worktrees/ldxp-browser-worker-auto-topup
git status --short
```

Expected:

```text
# git status --short 输出为空，或只包含执行者明确带入的计划内文件
```

- [ ] **Step 3: 只列出本地凭据文件路径，不输出 secret 内容**

Run:

```bash
python3 - <<'PY'
from pathlib import Path
root = Path.home() / "Desktop" / "云贝"
for p in sorted(root.rglob("*")):
    if p.is_file() and any(k in str(p) for k in ("服务器", "邮箱", "Cloudflare", "联动", "支付", "SMTP", "IMAP", "ssh")):
        print(p)
PY
```

Expected:

- 输出只包含文件路径。
- 实施者只打开当前步骤直接需要的文件读取连接方式、IMAP 授权值、服务器 SSH key 路径等。
- 不把实际密码、授权码、Token、私钥内容复制到计划、commit、日志或聊天。

- [ ] **Step 4: Commit checkpoint**

本任务只做核验和分支准备，不产生代码 commit。

---

## 4. Task 1: 合入或确认 paid_topup 卡密兑换前置能力

**Files:**
- Existing plan: `/Users/ethan/Documents/yunbay/docs/superpowers/plans/2026-06-27-ldxp-card-redemption.md`
- Expected backend files after prerequisite: `/Users/ethan/Documents/yunbay/model/redemption.go`, `/Users/ethan/Documents/yunbay/model/topup.go`, `/Users/ethan/Documents/yunbay/controller/redemption.go`, `/Users/ethan/Documents/yunbay/controller/user.go`
- Expected frontend files after prerequisite: `/Users/ethan/Documents/yunbay/web/default/src/features/wallet/lib/redemption-result.ts`

- [ ] **Step 1: 检查当前代码是否已有 paid_topup 能力**

Run:

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/ldxp-browser-worker-auto-topup
rg -n "RedemptionKindPaidTopUp|CreateRedemptionTopUpTradeNo|PaymentProviderRedemptionCode|count_as_topup|paid_topup" model controller web/default/src/features/wallet
```

Expected if prerequisite already present:

- `model/redemption.go` 定义 `RedemptionKindPaidTopUp`。
- `model/topup.go` 定义 `PaymentProviderRedemptionCode`。
- `model.Redeem` 返回结构化结果并在 `paid_topup/count_as_topup` 成功兑换时创建 `TopUp` 成功记录。

- [ ] **Step 2: 如果缺失，先执行既有卡密兑换计划**

Run one of the following approved paths:

```bash
# Preferred if local completed branch is available and clean:
git merge --no-ff codex/ldxp-card-redemption
```

or execute the existing task plan from scratch:

```bash
sed -n '1,260p' /Users/ethan/Documents/yunbay/docs/superpowers/plans/2026-06-27-ldxp-card-redemption.md
```

Expected:

- 兑换码模型、管理端导出、钱包兑换提示、paid top-up 充值统计能力全部存在。
- 不重复实现本计划已经覆盖的 Worker 自动支付部分。

- [ ] **Step 3: 运行前置能力测试**

Run:

```bash
go test ./model ./controller -run 'Redemption|TopUp|Payment' -count=1
cd web/default
bun run typecheck
```

Expected:

- Go 测试通过。
- Frontend typecheck 通过。

- [ ] **Step 4: Commit prerequisite if it was merged or implemented**

Run:

```bash
git status --short
git add model controller router i18n web/default/src/features/redemption-codes web/default/src/features/wallet web/default/src/i18n docs/superpowers/plans/2026-06-27-ldxp-card-redemption.md
git commit -m "feat: support paid topup redemption cards"
```

Expected:

- 如果 Step 2 没有产生改动，则跳过 commit。
- 如果产生改动，commit 只包含 paid_topup 前置能力，不包含自动充值 Worker 代码。

---

## 5. Task 2: 后端 LDXP 数据模型与迁移

**Files:**
- Create: `/Users/ethan/Documents/yunbay/model/ldxp_topup.go`
- Modify: `/Users/ethan/Documents/yunbay/model/main.go`
- Create: `/Users/ethan/Documents/yunbay/model/ldxp_topup_test.go`

- [ ] **Step 1: 写失败测试**

Create `/Users/ethan/Documents/yunbay/model/ldxp_topup_test.go` with tests that assert:

```go
func TestLdxpTopupSessionModelMigratesAndPersists(t *testing.T)
func TestLdxpTopupSessionIDIsUnique(t *testing.T)
func TestLdxpMailEventDedupesByRawHash(t *testing.T)
func TestGetClaimableLdxpSessionSkipsExpiredAndNonCreated(t *testing.T)
func TestUpdateLdxpSessionStatusPersistsWorkerFields(t *testing.T)
```

Use the existing SQLite test pattern from `/Users/ethan/Documents/yunbay/model/redemption_topup_test.go` after Task 1, or from other model tests if Task 1 was already merged.

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./model -run Ldxp -count=1
```

Expected:

- Compile FAIL because `LdxpTopupSession` and `LdxpMailEvent` do not exist yet.

- [ ] **Step 3: Implement models and helpers**

Create `/Users/ethan/Documents/yunbay/model/ldxp_topup.go` with these public contracts:

```go
package model

import "gorm.io/gorm"

const (
    LdxpStatusCreated       = "created"
    LdxpStatusWorkerClaimed = "worker_claimed"
    LdxpStatusQrReady       = "qr_ready"
    LdxpStatusWorkerPaid    = "worker_paid"
    LdxpStatusVerified      = "verified"
    LdxpStatusRedeemed      = "redeemed"
    LdxpStatusSuccess       = "success"
    LdxpStatusCanceled      = "canceled"
    LdxpStatusExpired       = "expired"
    LdxpStatusWorkerFailed  = "worker_failed"
    LdxpStatusMailTimeout   = "mail_timeout"
    LdxpStatusVerifyFailed  = "verify_failed"
    LdxpStatusRedeemFailed  = "redeem_failed"
)

type LdxpTopupSession struct {
    Id                 int            `json:"id"`
    SessionId          string         `json:"session_id" gorm:"type:varchar(64);uniqueIndex"`
    UserId             int            `json:"user_id" gorm:"index"`
    Amount             int64          `json:"amount"`
    Money              float64        `json:"money"`
    ProductUrl         string         `json:"product_url" gorm:"type:text"`
    ProductName        string         `json:"product_name" gorm:"type:text"`
    ContactEmail       string         `json:"contact_email" gorm:"type:varchar(255)"`
    Status             string         `json:"status" gorm:"type:varchar(64);index"`
    WorkerId           string         `json:"worker_id" gorm:"type:varchar(128);index"`
    QrCode             string         `json:"qr_code" gorm:"type:text"`
    QrPageUrl          string         `json:"qr_page_url" gorm:"type:text"`
    QrReadyTime        int64          `json:"qr_ready_time" gorm:"bigint"`
    WorkerOrderNo      string         `json:"worker_order_no" gorm:"type:varchar(64);index"`
    WorkerAmount       float64        `json:"worker_amount"`
    WorkerProductName  string         `json:"worker_product_name" gorm:"type:text"`
    WorkerCardKey      string         `json:"worker_card_key" gorm:"type:varchar(255);index"`
    WorkerStatusText   string         `json:"worker_status_text" gorm:"type:varchar(64)"`
    WorkerSuccessUrl   string         `json:"worker_success_url" gorm:"type:text"`
    WorkerDetectedTime int64          `json:"worker_detected_time" gorm:"bigint"`
    MailMessageId      string         `json:"mail_message_id" gorm:"type:varchar(255)"`
    MailOrderNo        string         `json:"mail_order_no" gorm:"type:varchar(64);index"`
    MailAmount         float64        `json:"mail_amount"`
    MailProductName    string         `json:"mail_product_name" gorm:"type:text"`
    MailCardKey        string         `json:"mail_card_key" gorm:"type:varchar(255);index"`
    MailFrom           string         `json:"mail_from" gorm:"type:varchar(255)"`
    MailTo             string         `json:"mail_to" gorm:"type:varchar(255)"`
    MailSubject        string         `json:"mail_subject" gorm:"type:text"`
    MailReceivedTime   int64          `json:"mail_received_time" gorm:"bigint"`
    VerifiedTime       int64          `json:"verified_time" gorm:"bigint"`
    RedeemedTime       int64          `json:"redeemed_time" gorm:"bigint"`
    TopupId            int            `json:"topup_id" gorm:"index"`
    RedemptionId       int            `json:"redemption_id" gorm:"index"`
    ErrorCode          string         `json:"error_code" gorm:"type:varchar(64)"`
    ErrorMessage       string         `json:"error_message" gorm:"type:text"`
    DebugSnapshotPath  string         `json:"debug_snapshot_path" gorm:"type:text"`
    CreatedTime        int64          `json:"created_time" gorm:"bigint;index"`
    UpdatedTime        int64          `json:"updated_time" gorm:"bigint"`
    ExpiredTime        int64          `json:"expired_time" gorm:"bigint;index"`
    DeletedAt          gorm.DeletedAt `gorm:"index"`
}

type LdxpMailEvent struct {
    Id               int            `json:"id"`
    MessageId        string         `json:"message_id" gorm:"type:varchar(255);index"`
    ImapUid          string         `json:"imap_uid" gorm:"type:varchar(128);index"`
    RawHash          string         `json:"raw_hash" gorm:"type:varchar(128);uniqueIndex"`
    MailFrom         string         `json:"mail_from" gorm:"type:varchar(255)"`
    MailTo           string         `json:"mail_to" gorm:"type:varchar(255)"`
    Subject          string         `json:"subject" gorm:"type:text"`
    ReceivedTime     int64          `json:"received_time" gorm:"bigint;index"`
    OrderNo          string         `json:"order_no" gorm:"type:varchar(64);index"`
    Amount           float64        `json:"amount"`
    ProductName      string         `json:"product_name" gorm:"type:text"`
    CardKey          string         `json:"card_key" gorm:"type:varchar(255);index"`
    PaidTime         int64          `json:"paid_time" gorm:"bigint"`
    BodyExcerpt      string         `json:"body_excerpt" gorm:"type:text"`
    MatchedSessionId string         `json:"matched_session_id" gorm:"type:varchar(64);index"`
    Processed        bool           `json:"processed" gorm:"default:false"`
    ErrorMessage     string         `json:"error_message" gorm:"type:text"`
    CreatedTime      int64          `json:"created_time" gorm:"bigint"`
    DeletedAt        gorm.DeletedAt `gorm:"index"`
}
```

Also implement helpers:

```go
func (LdxpTopupSession) TableName() string
func (LdxpMailEvent) TableName() string
func InsertLdxpTopupSession(session *LdxpTopupSession) error
func GetLdxpTopupSessionBySessionId(sessionId string) (*LdxpTopupSession, error)
func GetLdxpTopupSessionForUser(sessionId string, userId int) (*LdxpTopupSession, error)
func ClaimNextLdxpTopupSession(workerId string, now int64) (*LdxpTopupSession, error)
func SaveLdxpTopupSession(session *LdxpTopupSession) error
func InsertLdxpMailEvent(event *LdxpMailEvent) error
func GetLdxpMailEventByOrderNo(orderNo string) (*LdxpMailEvent, error)
func MarkExpiredLdxpTopupSessions(now int64) (int64, error)
```

Implementation constraints:

- Use GORM methods, not raw DB-specific SQL, except lock hints already used elsewhere.
- Use `common.UsingPostgreSQL` branch only if quoting is required.
- `ClaimNextLdxpTopupSession` must transactionally select one `created` session whose `expired_time > now`, mark it `worker_claimed`, set `worker_id`, set `updated_time`, and return it.
- If no row exists, return `gorm.ErrRecordNotFound`.

- [ ] **Step 4: Add migration entries**

Modify `/Users/ethan/Documents/yunbay/model/main.go`:

- In `migrateDB()` `DB.AutoMigrate(...)`, add:

```go
&LdxpTopupSession{},
&LdxpMailEvent{},
```

- In `migrateDBFast()` migration list, add:

```go
{&LdxpTopupSession{}, "LdxpTopupSession"},
{&LdxpMailEvent{}, "LdxpMailEvent"},
```

- [ ] **Step 5: Run tests**

Run:

```bash
go test ./model -run Ldxp -count=1
go test ./model -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
git add model/ldxp_topup.go model/ldxp_topup_test.go model/main.go
git commit -m "feat: add ldxp topup session models"
```

---

## 6. Task 3: 后端 LDXP 配置解析与安全脱敏 helper

**Files:**
- Create: `/Users/ethan/Documents/yunbay/service/ldxp_config.go`
- Create: `/Users/ethan/Documents/yunbay/service/ldxp_config_test.go`

- [ ] **Step 1: 写失败测试**

Create tests for:

```go
func TestLoadLdxpConfigDisabledByDefault(t *testing.T)
func TestLoadLdxpConfigParsesSixProducts(t *testing.T)
func TestLoadLdxpConfigRejectsMissingAmounts(t *testing.T)
func TestReadLdxpSecretPrefersFile(t *testing.T)
func TestRedactLdxpSecretMasksCardAndQr(t *testing.T)
```

Test products JSON must contain exactly `10,20,30,50,100,500`.

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./service -run LdxpConfig -count=1
```

Expected: compile FAIL because config functions do not exist yet.

- [ ] **Step 3: Implement config loader**

Create `/Users/ethan/Documents/yunbay/service/ldxp_config.go` with public contracts:

```go
type LdxpProductConfig struct {
    Amount      int64   `json:"amount"`
    Money       float64 `json:"money"`
    ProductURL  string  `json:"product_url"`
    ProductName string  `json:"product_name"`
}

type LdxpConfig struct {
    Enabled                 bool
    ContactEmail            string
    Products                map[int64]LdxpProductConfig
    WorkerToken             string
    SessionTTLSeconds       int64
    QrTTLSeconds            int64
    WorkerJobTimeoutSeconds int64
    MailMatchWindowSeconds  int64
    RequireMailMatch        bool
    DebugSnapshotDir        string
}

func LoadLdxpConfig() (*LdxpConfig, error)
func ReadLdxpSecret(envName string, fileEnvName string) string
func RedactLdxpValue(value string) string
func ValidateLdxpProducts(products []LdxpProductConfig) (map[int64]LdxpProductConfig, error)
```

Implementation constraints:

- Use `common.GetEnvOrDefault*` helpers.
- Use `common.UnmarshalJsonStr` to parse `LDXP_TOPUP_PRODUCTS_JSON`.
- `LoadLdxpConfig` must not log secret values.
- `RedactLdxpValue` returns empty for empty input, first 4 + `...` + last 4 for normal strings, and `data:image/png;base64,[redacted]` for QR data URLs.

- [ ] **Step 4: Run tests**

Run:

```bash
go test ./service -run LdxpConfig -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add service/ldxp_config.go service/ldxp_config_test.go
git commit -m "feat: load ldxp topup configuration"
```

---

## 7. Task 4: 后端 session 状态机服务

**Files:**
- Create: `/Users/ethan/Documents/yunbay/service/ldxp_session.go`
- Create: `/Users/ethan/Documents/yunbay/service/ldxp_session_test.go`

- [ ] **Step 1: 写失败测试**

Create tests for:

```go
func TestCreateLdxpTopupSessionRejectsDisabled(t *testing.T)
func TestCreateLdxpTopupSessionRejectsUnsupportedAmount(t *testing.T)
func TestCreateLdxpTopupSessionPersistsCreatedState(t *testing.T)
func TestRecordLdxpQrMovesSessionToQrReady(t *testing.T)
func TestRecordLdxpWorkerResultMovesSessionToWorkerPaid(t *testing.T)
func TestCancelLdxpSessionOnlyOwnerAndCancelableStates(t *testing.T)
func TestPublicLdxpSessionViewDoesNotExposeCardKey(t *testing.T)
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./service -run LdxpSession -count=1
```

Expected: compile FAIL because session service does not exist yet.

- [ ] **Step 3: Implement session service contracts**

Create `/Users/ethan/Documents/yunbay/service/ldxp_session.go` with:

```go
type LdxpCreateSessionRequest struct {
    Amount int64 `json:"amount"`
}

type LdxpSessionPublicView struct {
    SessionID      string  `json:"session_id"`
    Amount         int64   `json:"amount"`
    Money          float64 `json:"money"`
    Status         string  `json:"status"`
    QRCode         string  `json:"qr_code,omitempty"`
    WorkerOrderNo  string  `json:"worker_order_no,omitempty"`
    ExpiresAt      int64   `json:"expires_at"`
    PollIntervalMs int     `json:"poll_interval_ms"`
    ErrorCode      string  `json:"error_code,omitempty"`
    ErrorMessage   string  `json:"error_message,omitempty"`
}

type LdxpWorkerQrPayload struct {
    WorkerID          string  `json:"worker_id"`
    WorkerOrderNo     string  `json:"worker_order_no"`
    WorkerAmount      float64 `json:"worker_amount"`
    WorkerProductName string  `json:"worker_product_name"`
    QRCode            string  `json:"qr_code"`
    QRPageURL         string  `json:"qr_page_url"`
}

type LdxpWorkerResultPayload struct {
    WorkerID          string  `json:"worker_id"`
    WorkerOrderNo     string  `json:"worker_order_no"`
    WorkerAmount      float64 `json:"worker_amount"`
    WorkerProductName string  `json:"worker_product_name"`
    WorkerCardKey     string  `json:"worker_card_key"`
    WorkerStatusText  string  `json:"worker_status_text"`
    WorkerSuccessURL  string  `json:"worker_success_url"`
}

func CreateLdxpTopupSession(userID int, amount int64, cfg *LdxpConfig) (*LdxpSessionPublicView, error)
func GetLdxpSessionPublicView(sessionID string, userID int) (*LdxpSessionPublicView, error)
func CancelLdxpTopupSession(sessionID string, userID int) error
func ClaimLdxpTopupSession(workerID string, cfg *LdxpConfig) (*model.LdxpTopupSession, error)
func RecordLdxpQr(sessionID string, payload LdxpWorkerQrPayload) error
func RecordLdxpWorkerResult(sessionID string, payload LdxpWorkerResultPayload) (*model.LdxpTopupSession, error)
func RecordLdxpWorkerError(sessionID string, workerID string, code string, message string, snapshotPath string) error
```

Implementation constraints:

- Session ID format: `ldxp_` + 24 lowercase alphanumeric chars from existing random helper, or UUID without dashes prefixed with `ldxp_`.
- User public view never includes `WorkerCardKey` or `MailCardKey`.
- QR code can be returned to frontend while status is `qr_ready`; if session is terminal, omit QR code.
- Cancel allowed only in `created`, `worker_claimed`, `qr_ready`.
- `RecordLdxpWorkerResult` in this task only saves the Worker result and moves the session to `worker_paid`. Task 6 wires automatic verify/redeem after the verifier exists.

- [ ] **Step 4: Run tests**

Run:

```bash
go test ./service -run LdxpSession -count=1
```

Expected: PASS. These tests assert persistence and public-view behavior only; automatic verify/redeem wiring is covered in Task 6.

- [ ] **Step 5: Commit**

Run:

```bash
git add service/ldxp_session.go service/ldxp_session_test.go
git commit -m "feat: manage ldxp topup sessions"
```

---

## 8. Task 5: 邮件解析与邮件事件落库

**Files:**
- Create: `/Users/ethan/Documents/yunbay/service/ldxp_mail.go`
- Create: `/Users/ethan/Documents/yunbay/service/ldxp_mail_test.go`

- [ ] **Step 1: 写邮件 fixture 和失败测试**

Create tests for:

```go
func TestParseLdxpMailExtractsOrderAmountAndCard(t *testing.T)
func TestParseLdxpMailHandlesHtmlBody(t *testing.T)
func TestUpsertLdxpMailEventDedupesRawHash(t *testing.T)
func TestMatchLdxpMailEventToWorkerSessionRequiresOrderNo(t *testing.T)
func TestMatchLdxpMailEventRejectsMismatchedCard(t *testing.T)
```

Use sanitized fixture text based on the screenshot facts:

```text
订单号：LD260628UZJ97P
商品名称：0.1 元测试
支付金额：0.10
卡密账号：abcd1234-card-key
付款时间：2026-06-28 03:10:00
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./service -run LdxpMail -count=1
```

Expected: compile FAIL because mail parser does not exist yet.

- [ ] **Step 3: Implement parser and event service**

Create `/Users/ethan/Documents/yunbay/service/ldxp_mail.go` with:

```go
type LdxpParsedMail struct {
    MessageID    string
    ImapUID      string
    RawHash      string
    From         string
    To           string
    Subject      string
    ReceivedTime int64
    OrderNo      string
    Amount       float64
    ProductName  string
    CardKey      string
    PaidTime     int64
    BodyExcerpt  string
}

func ParseLdxpMailText(input string) (*LdxpParsedMail, error)
func NormalizeLdxpMailBody(input string) string
func HashLdxpMailRaw(raw []byte) string
func SaveLdxpMailEvent(parsed *LdxpParsedMail) (*model.LdxpMailEvent, error)
func TryMatchLdxpMailEvent(event *model.LdxpMailEvent) (*model.LdxpTopupSession, error)
```

Parsing rules:

- Order regex accepts `订单号`、`订单编号`、`订单` followed by `LD` + uppercase letters/digits.
- Card regex accepts `卡密账号`、`卡密`、`兑换码` followed by a non-space token or next line token.
- Amount regex accepts `支付金额`、`金额` with optional `¥` and `元`.
- Product name regex accepts `商品名称`、`商品名`、`购买内容`.
- `BodyExcerpt` must be limited to 500 runes and pass `RedactLdxpValue` for card-like values.

Implementation constraints:

- Store duplicate `raw_hash` as one event; if duplicate arrives, return existing event.
- `TryMatchLdxpMailEvent` finds sessions by `worker_order_no == event.OrderNo` in `worker_paid` or `verify_failed` state and calls `TryVerifyAndRedeemLdxpSession`.

- [ ] **Step 4: Run tests**

Run:

```bash
go test ./service -run LdxpMail -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add service/ldxp_mail.go service/ldxp_mail_test.go
git commit -m "feat: parse and store ldxp mail events"
```

---

## 9. Task 6: 核验与入账幂等服务

**Files:**
- Create: `/Users/ethan/Documents/yunbay/service/ldxp_verify.go`
- Create: `/Users/ethan/Documents/yunbay/service/ldxp_verify_test.go`

- [ ] **Step 1: 写失败测试**

Create tests for:

```go
func TestVerifyLdxpSessionRequiresWorkerAndMailOrderMatch(t *testing.T)
func TestVerifyLdxpSessionRequiresCardMatch(t *testing.T)
func TestVerifyLdxpSessionRequiresAmountMatchWhenConfigured(t *testing.T)
func TestVerifyLdxpSessionRedeemsPaidTopupCard(t *testing.T)
func TestVerifyLdxpSessionIsIdempotent(t *testing.T)
func TestVerifyLdxpSessionStoresRedeemFailure(t *testing.T)
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./service -run LdxpVerify -count=1
```

Expected: compile FAIL because verifier does not exist yet.

- [ ] **Step 3: Implement verifier**

Create `/Users/ethan/Documents/yunbay/service/ldxp_verify.go` with:

```go
type LdxpVerifyResult struct {
    Verified     bool
    Redeemed     bool
    Status       string
    ErrorCode    string
    ErrorMessage string
}

func TryVerifyAndRedeemLdxpSession(sessionID string) (*LdxpVerifyResult, error)
func VerifyLdxpSessionFields(session *model.LdxpTopupSession, event *model.LdxpMailEvent) error
func RedeemLdxpSessionCard(session *model.LdxpTopupSession) error
```

Verification rules:

- `session.WorkerOrderNo` must be non-empty.
- Matching `LdxpMailEvent` must exist by same order number.
- `session.WorkerOrderNo == event.OrderNo`.
- `session.WorkerCardKey == event.CardKey`.
- If `session.Money > 0` and `event.Amount > 0`, absolute difference must be `<= 0.01`.
- `session.WorkerStatusText` must contain `已付款` or `成功`.
- If already `success`, return success without redeeming again.
- If status is `verified` or `redeemed` and `redemption_id/topup_id` are populated, return success without redeeming again.
- Call `model.Redeem(session.WorkerCardKey, session.UserId)` only after all checks pass.
- Save `redemption_id/topup_id` if returned by Task 1 prerequisite result; if Task 1 result does not expose `topup_id`, save `redemption_id` and leave `topup_id=0`.
- On redeem error, set `redeem_failed`, keep order/card/mail fields for admin audit.

- [ ] **Step 4: Wire Task 4 and Task 5 calls**

Modify `/Users/ethan/Documents/yunbay/service/ldxp_session.go` and `/Users/ethan/Documents/yunbay/service/ldxp_mail.go` so:

- `RecordLdxpWorkerResult` calls `TryVerifyAndRedeemLdxpSession(sessionID)` after saving worker result.
- `TryMatchLdxpMailEvent` calls `TryVerifyAndRedeemLdxpSession(matched.SessionId)` after attaching mail fields.

- [ ] **Step 5: Run tests**

Run:

```bash
go test ./service -run 'Ldxp(Session|Mail|Verify|Config)' -count=1
go test ./model ./service -run Ldxp -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
git add service/ldxp_verify.go service/ldxp_verify_test.go service/ldxp_session.go service/ldxp_mail.go
git commit -m "feat: verify and redeem ldxp topup sessions"
```

---

## 10. Task 7: 后端 HTTP API 与路由

**Files:**
- Create: `/Users/ethan/Documents/yunbay/controller/ldxp_topup.go`
- Modify: `/Users/ethan/Documents/yunbay/router/api-router.go`
- Create: `/Users/ethan/Documents/yunbay/controller/ldxp_topup_test.go`

- [ ] **Step 1: 写失败测试**

Create controller tests for:

```go
func TestCreateLdxpTopupSessionRequiresUser(t *testing.T)
func TestCreateLdxpTopupSessionReturnsConfiguredAmounts(t *testing.T)
func TestGetLdxpTopupSessionRequiresOwner(t *testing.T)
func TestCancelLdxpTopupSession(t *testing.T)
func TestWorkerClaimRequiresToken(t *testing.T)
func TestWorkerQrUpdateRequiresToken(t *testing.T)
func TestWorkerResultTriggersVerify(t *testing.T)
func TestWorkerMailEventIngestRequiresToken(t *testing.T)
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./controller -run LdxpTopup -count=1
```

Expected: compile FAIL because controller functions do not exist yet.

- [ ] **Step 3: Implement controller**

Create `/Users/ethan/Documents/yunbay/controller/ldxp_topup.go` with functions:

```go
func CreateLdxpTopupSession(c *gin.Context)
func GetLdxpTopupSession(c *gin.Context)
func CancelLdxpTopupSession(c *gin.Context)
func WorkerClaimLdxpTopupSession(c *gin.Context)
func WorkerRecordLdxpQr(c *gin.Context)
func WorkerRecordLdxpResult(c *gin.Context)
func WorkerRecordLdxpError(c *gin.Context)
func WorkerRecordLdxpMailEvent(c *gin.Context)
func AdminListLdxpTopupSessions(c *gin.Context)
func AdminRetryLdxpTopupVerify(c *gin.Context)
```

Controller rules:

- User routes use `c.GetInt("id")` from `middleware.UserAuth()`.
- Worker routes compare `X-LDXP-Worker-Token` with `service.LoadLdxpConfig().WorkerToken` using constant-time compare from `crypto/subtle`.
- Do not log full request body for Worker result or mail event because it may contain card key.
- For JSON bind, `ShouldBindJSON` is acceptable as Gin binding; manual marshal/unmarshal in new Go code must use `common.Marshal` / `common.Unmarshal`.
- Public API response must use `common.ApiSuccess` / `common.ApiError` patterns already present in repo.

- [ ] **Step 4: Register routes**

Modify `/Users/ethan/Documents/yunbay/router/api-router.go`:

Inside authenticated user group:

```go
selfRoute.POST("/ldxp/topup/session", middleware.CriticalRateLimit(), controller.CreateLdxpTopupSession)
selfRoute.GET("/ldxp/topup/session/:session_id", controller.GetLdxpTopupSession)
selfRoute.POST("/ldxp/topup/session/:session_id/cancel", controller.CancelLdxpTopupSession)
```

Inside API router root, outside user auth but with Worker token in handler:

```go
ldxpWorkerRoute := apiRouter.Group("/ldxp/worker")
{
    ldxpWorkerRoute.POST("/sessions/claim", controller.WorkerClaimLdxpTopupSession)
    ldxpWorkerRoute.POST("/sessions/:session_id/qr", controller.WorkerRecordLdxpQr)
    ldxpWorkerRoute.POST("/sessions/:session_id/result", controller.WorkerRecordLdxpResult)
    ldxpWorkerRoute.POST("/sessions/:session_id/error", controller.WorkerRecordLdxpError)
    ldxpWorkerRoute.POST("/mail-events", controller.WorkerRecordLdxpMailEvent)
}
```

Admin routes:

```go
ldxpAdminRoute := apiRouter.Group("/ldxp/admin")
ldxpAdminRoute.Use(middleware.AdminAuth())
{
    ldxpAdminRoute.GET("/sessions", controller.AdminListLdxpTopupSessions)
    ldxpAdminRoute.POST("/sessions/:session_id/retry", controller.AdminRetryLdxpTopupVerify)
}
```

- [ ] **Step 5: Run tests**

Run:

```bash
go test ./controller -run LdxpTopup -count=1
go test ./router ./controller ./service ./model -run Ldxp -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
git add controller/ldxp_topup.go controller/ldxp_topup_test.go router/api-router.go
git commit -m "feat: expose ldxp topup api"
```

---

## 11. Task 8: Worker 服务脚手架与后端客户端

**Files:**
- Create all files under `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/`

- [ ] **Step 1: 创建 Worker package**

Run:

```bash
mkdir -p /Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/src /Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/tests
cd /Users/ethan/Documents/yunbay/workers/ldxp-browser-worker
cat > package.json <<'JSON'
{
  "name": "yunbay-ldxp-browser-worker",
  "private": true,
  "type": "module",
  "scripts": {
    "build": "tsc -p tsconfig.json",
    "start": "node dist/index.js",
    "test": "node --test dist/tests/*.test.js",
    "check": "tsc -p tsconfig.json && node --test dist/tests/*.test.js"
  },
  "dependencies": {
    "imapflow": "latest",
    "mailparser": "latest",
    "playwright": "latest",
    "pino": "latest"
  },
  "devDependencies": {
    "@types/node": "latest",
    "typescript": "latest"
  }
}
JSON
cat > tsconfig.json <<'JSON'
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "strict": true,
    "esModuleInterop": true,
    "outDir": "dist",
    "rootDir": ".",
    "types": ["node"]
  },
  "include": ["src/**/*.ts", "tests/**/*.ts"]
}
JSON
```

- [ ] **Step 2: 写失败测试**

Create `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/tests/config.test.ts` and `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/tests/redact.test.ts` covering:

```ts
import test from 'node:test'
import assert from 'node:assert/strict'
import { loadConfigFromEnv } from '../src/config.js'
import { redactValue } from '../src/redact.js'

test('loadConfigFromEnv requires backend base url and token', () => {
  assert.throws(() => loadConfigFromEnv({}), /BACKEND_BASE_URL/)
})

test('redactValue masks card keys and qr data urls', () => {
  assert.equal(redactValue(''), '')
  assert.equal(redactValue('abcd1234efgh5678'), 'abcd...5678')
  assert.equal(redactValue('data:image/png;base64,AAAA'), 'data:image/png;base64,[redacted]')
})
```

- [ ] **Step 3: Run test to verify it fails**

Run:

```bash
cd /Users/ethan/Documents/yunbay/workers/ldxp-browser-worker
bun install
bun run build
```

Expected: compile FAIL because `src/config.ts` and `src/redact.ts` do not exist yet.

- [ ] **Step 4: Implement config/redact/backend client**

Create:

- `src/config.ts` with `loadConfigFromEnv(env = process.env)` that reads all Worker env vars and file secrets.
- `src/redact.ts` with `redactValue`.
- `src/backend.ts` with:

```ts
export interface ClaimedSession {
  session_id: string
  amount: number
  money: number
  product_url: string
  product_name: string
  contact_email: string
}

export async function claimSession(config: WorkerConfig): Promise<ClaimedSession | null>
export async function postQr(config: WorkerConfig, sessionId: string, payload: WorkerQrPayload): Promise<void>
export async function postResult(config: WorkerConfig, sessionId: string, payload: WorkerResultPayload): Promise<void>
export async function postError(config: WorkerConfig, sessionId: string, payload: WorkerErrorPayload): Promise<void>
export async function postMailEvent(config: WorkerConfig, payload: WorkerMailEventPayload): Promise<void>
```

Backend client rules:

- Use global `fetch` from Node 22.
- Send `X-LDXP-Worker-Token`.
- Treat HTTP 404 from claim as no job.
- Throw on non-2xx for other calls.
- Never log full QR data or card key.

- [ ] **Step 5: Build and test**

Run:

```bash
cd /Users/ethan/Documents/yunbay/workers/ldxp-browser-worker
bun run build
bun run test
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
git add workers/ldxp-browser-worker
git commit -m "feat: scaffold ldxp browser worker"
```

---

## 12. Task 9: Worker Playwright 商品页、二维码、结果页流程

**Files:**
- Create: `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/src/browser-flow.ts`
- Create: `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/tests/browser-flow.test.ts`
- Create fixtures under `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/tests/fixtures/`

- [ ] **Step 1: 写 HTML fixture 和失败测试**

Create fixtures:

- `item-page.html` contains contact input placeholder `请输入联系方式方便查询订单`, `支付宝`, `立即购买`.
- `cashier-page.html` contains order no `LD260628UZJ97P`, amount `0.10 元`, and an image or canvas QR node.
- `result-page.html` contains `已付款`, order no `LD260628UZJ97P`, card key `abcd1234-card-key`.

Create tests for pure extraction helpers:

```ts
import test from 'node:test'
import assert from 'node:assert/strict'
import { extractOrderNo, extractAmount, extractCardKey, isPaidResultText } from '../src/browser-flow.js'

test('extracts order no from cashier text', () => {
  assert.equal(extractOrderNo('订单号 LD260628UZJ97P 金额 0.10 元'), 'LD260628UZJ97P')
})

test('extracts paid result and card key', () => {
  const text = '支付成功 已付款 订单号 LD260628UZJ97P 卡密账号 abcd1234-card-key'
  assert.equal(isPaidResultText(text), true)
  assert.equal(extractCardKey(text), 'abcd1234-card-key')
})
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd /Users/ethan/Documents/yunbay/workers/ldxp-browser-worker
bun run build
```

Expected: compile FAIL because browser-flow exports do not exist.

- [ ] **Step 3: Implement browser flow**

Create `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/src/browser-flow.ts` with:

```ts
export interface BrowserFlowInput {
  sessionId: string
  productUrl: string
  contactEmail: string
  expectedAmount: number
  expectedProductName?: string
}

export interface BrowserQrResult {
  worker_order_no: string
  worker_amount: number
  worker_product_name: string
  qr_code: string
  qr_page_url: string
}

export interface BrowserPaidResult {
  worker_order_no: string
  worker_amount: number
  worker_product_name: string
  worker_card_key: string
  worker_status_text: string
  worker_success_url: string
}

export function extractOrderNo(text: string): string
export function extractAmount(text: string): number
export function extractCardKey(text: string): string
export function isPaidResultText(text: string): boolean
export async function runBrowserFlow(input: BrowserFlowInput, callbacks: { onQr(result: BrowserQrResult): Promise<void> }, config: WorkerConfig): Promise<BrowserPaidResult>
```

Playwright behavior:

- Launch Chromium headless according to `LDXP_BROWSER_HEADLESS`.
- `page.goto(productUrl, { waitUntil: 'domcontentloaded' })`.
- Fill contact input by placeholder text or first visible text/email input.
- Click Alipay payment option if present.
- Click `立即购买`.
- Wait for cashier URL or QR node.
- Extract order number and amount from page text.
- Convert QR `img[src]` data URL directly, or screenshot QR element as `data:image/png;base64,...`.
- Call `callbacks.onQr` exactly once.
- Wait for URL containing `/order/result/` or page text containing paid markers.
- Extract final order number, amount, status text, card key and result URL.
- On failure, take screenshot and HTML snapshot under snapshot directory, then throw an error containing only redacted values.

- [ ] **Step 4: Run tests and build**

Run:

```bash
cd /Users/ethan/Documents/yunbay/workers/ldxp-browser-worker
bun run build
bun run test
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add workers/ldxp-browser-worker/src/browser-flow.ts workers/ldxp-browser-worker/tests/browser-flow.test.ts workers/ldxp-browser-worker/tests/fixtures
git commit -m "feat: automate ldxp browser payment flow"
```

---

## 13. Task 10: Worker IMAP 邮件轮询与邮件解析

**Files:**
- Create: `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/src/mail-parser.ts`
- Create: `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/src/mail-poller.ts`
- Create: `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/tests/mail-parser.test.ts`

- [ ] **Step 1: 写失败测试**

Create `mail-parser.test.ts`:

```ts
import test from 'node:test'
import assert from 'node:assert/strict'
import { parseLdxpMailBody, hashRawMail } from '../src/mail-parser.js'

test('parseLdxpMailBody extracts order/card/amount', () => {
  const parsed = parseLdxpMailBody('订单号：LD260628UZJ97P\n支付金额：0.10 元\n商品名称：0.1 元测试\n卡密账号：abcd1234-card-key')
  assert.equal(parsed.order_no, 'LD260628UZJ97P')
  assert.equal(parsed.amount, 0.10)
  assert.equal(parsed.product_name, '0.1 元测试')
  assert.equal(parsed.card_key, 'abcd1234-card-key')
})

test('hashRawMail is stable sha256 hex', () => {
  assert.equal(hashRawMail(Buffer.from('abc')).length, 64)
})
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd /Users/ethan/Documents/yunbay/workers/ldxp-browser-worker
bun run build
```

Expected: compile FAIL because mail parser does not exist.

- [ ] **Step 3: Implement parser and poller**

Create `src/mail-parser.ts` with:

```ts
export interface ParsedMailEvent {
  message_id: string
  imap_uid: string
  raw_hash: string
  mail_from: string
  mail_to: string
  subject: string
  received_time: number
  order_no: string
  amount: number
  product_name: string
  card_key: string
  paid_time: number
  body_excerpt: string
}

export function hashRawMail(raw: Buffer): string
export function parseLdxpMailBody(body: string): Pick<ParsedMailEvent, 'order_no' | 'amount' | 'product_name' | 'card_key'>
export function makeBodyExcerpt(body: string): string
```

Create `src/mail-poller.ts` with:

```ts
export async function pollMailboxOnce(config: WorkerConfig): Promise<number>
export async function runMailPoller(config: WorkerConfig, signal: AbortSignal): Promise<void>
```

Poller behavior:

- Connect to QQ IMAP with `imapflow` using host/port/secure/user/password from config.
- Open configured mailbox.
- Search recent unseen or recent messages first; if server flags are unreliable, search messages since now minus `LDXP_MAIL_MATCH_WINDOW_SECONDS` equivalent from Worker config.
- Parse raw message with `mailparser`.
- Build `ParsedMailEvent` and call `postMailEvent`.
- Do not mark messages deleted.
- It is acceptable to mark seen only after backend accepts event.
- On parser failure, log redacted subject/from and continue.

- [ ] **Step 4: Run tests and build**

Run:

```bash
cd /Users/ethan/Documents/yunbay/workers/ldxp-browser-worker
bun run build
bun run test
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add workers/ldxp-browser-worker/src/mail-parser.ts workers/ldxp-browser-worker/src/mail-poller.ts workers/ldxp-browser-worker/tests/mail-parser.test.ts
git commit -m "feat: poll ldxp purchase emails"
```

---

## 14. Task 11: Worker 主循环、并发控制与 Dockerfile

**Files:**
- Create: `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/src/index.ts`
- Create: `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/Dockerfile`
- Create: `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/docker-compose.example.yml`

- [ ] **Step 1: Implement main loop**

Create `src/index.ts`:

- Load config.
- Start mail poller in background.
- Loop every `LDXP_WORKER_POLL_INTERVAL_MS`.
- Claim at most `LDXP_WORKER_CONCURRENCY` sessions concurrently.
- For each session, call `runBrowserFlow`; post QR through callback; post result on success; post error on failure.
- Handle SIGINT/SIGTERM with `AbortController`.
- Logs use `pino` and redaction helper.

- [ ] **Step 2: Create Dockerfile**

Create `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/Dockerfile`:

```Dockerfile
FROM mcr.microsoft.com/playwright:latest
WORKDIR /app
COPY package.json tsconfig.json ./
RUN corepack enable && corepack prepare bun@latest --activate && bun install
COPY src ./src
COPY tests ./tests
RUN bun run build
ENV NODE_ENV=production
CMD ["node", "dist/src/index.js"]
```

If the base image does not include Bun in the implementation environment, replace the Bun install line with npm install while preserving package-lock generated by the executor. Record that change in the commit message.

- [ ] **Step 3: Create local compose example**

Create `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/docker-compose.example.yml` with:

```yaml
services:
  ldxp-worker:
    build:
      context: .
    environment:
      BACKEND_BASE_URL: http://host.docker.internal:3000
      LDXP_WORKER_ID: ldxp-worker-local
      LDXP_WORKER_TOKEN_FILE: /run/secrets/ldxp_worker_token
      QQ_IMAP_PASSWORD_FILE: /run/secrets/qq_imap_password
      QQ_IMAP_USER: 10256345@qq.com
      LDXP_CONTACT_EMAIL: support@yunbay.xyz
      LDXP_BROWSER_SNAPSHOT_DIR: /app/snapshots
    secrets:
      - ldxp_worker_token
      - qq_imap_password
    volumes:
      - ./snapshots:/app/snapshots
secrets:
  ldxp_worker_token:
    file: ./secrets/ldxp_worker_token
  qq_imap_password:
    file: ./secrets/qq_imap_password
```

- [ ] **Step 4: Verify Worker build**

Run:

```bash
cd /Users/ethan/Documents/yunbay/workers/ldxp-browser-worker
bun run check
docker build -t yunbay-ldxp-browser-worker:local .
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add workers/ldxp-browser-worker/src/index.ts workers/ldxp-browser-worker/Dockerfile workers/ldxp-browser-worker/docker-compose.example.yml workers/ldxp-browser-worker/package.json workers/ldxp-browser-worker/tsconfig.json
git commit -m "feat: run ldxp worker service"
```

---

## 15. Task 12: 前端 API、hook 与纯函数测试

**Files:**
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/features/wallet/types.ts`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/features/wallet/api.ts`
- Create: `/Users/ethan/Documents/yunbay/web/default/src/features/wallet/hooks/use-ldxp-topup.ts`
- Create: `/Users/ethan/Documents/yunbay/web/default/src/features/wallet/lib/ldxp-topup.ts`
- Create: `/Users/ethan/Documents/yunbay/web/default/src/features/wallet/lib/ldxp-topup.test.ts`

- [ ] **Step 1: 写失败测试**

Create `/Users/ethan/Documents/yunbay/web/default/src/features/wallet/lib/ldxp-topup.test.ts`:

```ts
import { describe, expect, it } from 'bun:test'
import { LDXP_TOPUP_AMOUNTS, isLdxpTerminalStatus, getLdxpStatusMessageKey } from './ldxp-topup'

describe('ldxp topup helpers', () => {
  it('uses fixed allowed amounts', () => {
    expect(LDXP_TOPUP_AMOUNTS).toEqual([10, 20, 30, 50, 100, 500])
  })

  it('detects terminal status', () => {
    expect(isLdxpTerminalStatus('success')).toBe(true)
    expect(isLdxpTerminalStatus('qr_ready')).toBe(false)
    expect(isLdxpTerminalStatus('verify_failed')).toBe(true)
  })

  it('maps status to message keys', () => {
    expect(getLdxpStatusMessageKey('qr_ready')).toBe('Scan with Alipay to pay')
    expect(getLdxpStatusMessageKey('success')).toBe('Recharge successful')
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun test src/features/wallet/lib/ldxp-topup.test.ts
```

Expected: FAIL because helper file does not exist.

- [ ] **Step 3: Extend frontend types and API**

Modify `types.ts`:

```ts
export type LdxpTopupStatus =
  | 'created'
  | 'worker_claimed'
  | 'qr_ready'
  | 'worker_paid'
  | 'verified'
  | 'redeemed'
  | 'success'
  | 'canceled'
  | 'expired'
  | 'worker_failed'
  | 'mail_timeout'
  | 'verify_failed'
  | 'redeem_failed'

export interface LdxpTopupSession {
  session_id: string
  amount: number
  money: number
  status: LdxpTopupStatus
  qr_code?: string
  worker_order_no?: string
  expires_at: number
  poll_interval_ms?: number
  error_code?: string
  error_message?: string
}

export type LdxpTopupSessionResponse = ApiResponse<LdxpTopupSession>
```

Modify `api.ts`:

```ts
export async function createLdxpTopupSession(amount: number): Promise<LdxpTopupSessionResponse> {
  const res = await api.post('/api/user/ldxp/topup/session', { amount })
  return res.data
}

export async function getLdxpTopupSession(sessionId: string): Promise<LdxpTopupSessionResponse> {
  const res = await api.get(`/api/user/ldxp/topup/session/${encodeURIComponent(sessionId)}`)
  return res.data
}

export async function cancelLdxpTopupSession(sessionId: string): Promise<ApiResponse> {
  const res = await api.post(`/api/user/ldxp/topup/session/${encodeURIComponent(sessionId)}/cancel`)
  return res.data
}
```

- [ ] **Step 4: Implement helper and hook**

Create `lib/ldxp-topup.ts` with fixed amounts/status functions.

Create `hooks/use-ldxp-topup.ts` with:

```ts
export function useLdxpTopup(options: { onSuccess?: () => Promise<void> | void })
```

Hook behavior:

- `start(amount)` creates session and opens dialog state.
- Poll every `session.poll_interval_ms || 2000` while status is non-terminal.
- Stop on `success`, call `onSuccess` to refresh user balance and billing history.
- Stop on failure/cancel/expired.
- Expose `session`, `loading`, `error`, `start`, `cancel`, `reset`.

- [ ] **Step 5: Run tests**

Run:

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun test src/features/wallet/lib/ldxp-topup.test.ts
bun run typecheck
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
git add web/default/src/features/wallet/types.ts web/default/src/features/wallet/api.ts web/default/src/features/wallet/hooks/use-ldxp-topup.ts web/default/src/features/wallet/lib/ldxp-topup.ts web/default/src/features/wallet/lib/ldxp-topup.test.ts
git commit -m "feat: add ldxp topup frontend state"
```

---

## 16. Task 13: 前端固定金额卡片与支付弹窗

**Files:**
- Create: `/Users/ethan/Documents/yunbay/web/default/src/features/wallet/components/ldxp-topup-card.tsx`
- Create: `/Users/ethan/Documents/yunbay/web/default/src/features/wallet/components/ldxp-payment-dialog.tsx`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/features/wallet/index.tsx`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/features/wallet/types.ts`

- [ ] **Step 1: Extend `TopupInfo`**

Modify `TopupInfo` in `types.ts`:

```ts
/** Whether LDXP browser-worker automatic top-up is enabled */
enable_ldxp_topup?: boolean
/** Fixed LDXP amount options from backend */
ldxp_amount_options?: number[]
```

Modify backend `GetTopUpInfo` in `/Users/ethan/Documents/yunbay/controller/topup.go` to include:

```go
"enable_ldxp_topup": ldxpCfg.Enabled,
"ldxp_amount_options": service.GetLdxpAmountOptions(ldxpCfg),
```

Add `GetLdxpAmountOptions(cfg *LdxpConfig) []int64` in `service/ldxp_config.go` returning sorted configured amounts.

- [ ] **Step 2: Create UI components**

`ldxp-topup-card.tsx` requirements:

- Render only when `topupInfo?.enable_ldxp_topup === true`.
- Render exactly six buttons in order `10/20/30/50/100/500` unless backend returns a stricter list.
- Each button calls `onStart(amount)`.
- Use existing `TitledCard`, `Button`, `Badge` style from wallet components.

`ldxp-payment-dialog.tsx` requirements:

- Open when hook has active session.
- If status is `qr_ready` and `qr_code` exists, render `<img src={qr_code} alt={t('Alipay payment QR code')} />`.
- Show amount, order number if present, countdown from `expires_at`, status message.
- Show Cancel button for non-terminal statuses except `worker_paid/verified/redeemed`.
- On `success`, show `Recharge successful` and a Close button.
- On failure statuses, show safe `error_message` if present and a Close button.

- [ ] **Step 3: Wire wallet page**

Modify `index.tsx`:

- Import `useLdxpTopup`, `LdxpTopupCard`, `LdxpPaymentDialog`.
- Instantiate hook with `onSuccess: fetchUser`.
- Render LDXP card near recharge card.
- Render payment dialog at page root.

- [ ] **Step 4: Run frontend verification**

Run:

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun run typecheck
bun run build
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add controller/topup.go service/ldxp_config.go web/default/src/features/wallet/components/ldxp-topup-card.tsx web/default/src/features/wallet/components/ldxp-payment-dialog.tsx web/default/src/features/wallet/index.tsx web/default/src/features/wallet/types.ts
git commit -m "feat: add ldxp topup wallet UI"
```

---

## 17. Task 14: 前端 i18n 补齐

**Files:**
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/en.json`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/zh.json`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/fr.json`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/ja.json`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/ru.json`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/vi.json`

- [ ] **Step 1: Add keys used by new components**

Add translations for these English keys:

```text
LinkPay automatic recharge
Pay by scanning the Alipay QR code generated from the linked store
Start recharge
Scan with Alipay to pay
Creating payment QR code
Payment detected, verifying email
Recharge successful
Recharge failed
Payment expired
Cancel payment
Close
Alipay payment QR code
Order number
Payment amount
Waiting for payment
Verifying order
The recharge session was canceled
The payment window expired. Please start again.
Please contact support with this order number.
```

- [ ] **Step 2: Run sync**

Run:

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun run i18n:sync
```

Expected:

- No missing translation keys for `en`, `zh`, `fr`, `ja`, `ru`, `vi`.

- [ ] **Step 3: Run i18n-adjacent tests**

Run:

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun test src/features/wallet/lib/ldxp-topup.test.ts
bun run typecheck
```

Expected: PASS.

- [ ] **Step 4: Commit**

Run:

```bash
git add web/default/src/i18n/locales/en.json web/default/src/i18n/locales/zh.json web/default/src/i18n/locales/fr.json web/default/src/i18n/locales/ja.json web/default/src/i18n/locales/ru.json web/default/src/i18n/locales/vi.json
git commit -m "feat: translate ldxp topup UI"
```

---

## 18. Task 15: 本地端到端联调，不真实支付

**Files:**
- Modify: `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/src/browser-flow.ts`
- Create: `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/src/mock-flow.ts`
- Modify: `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/src/index.ts`

- [ ] **Step 1: Add mock mode**

Add Worker env:

```bash
LDXP_WORKER_MOCK_MODE=false
```

When `LDXP_WORKER_MOCK_MODE=true`, Worker must:

- Claim sessions normally from backend.
- Post QR payload using a small deterministic test QR data URL.
- Post result payload with a deterministic order number derived from session ID.
- Post matching mail event.
- Use a test card key supplied by env `LDXP_WORKER_MOCK_CARD_KEY`.

- [ ] **Step 2: Run backend and frontend locally**

Run from repo root:

```bash
go test ./model ./service ./controller -run Ldxp -count=1
cd web/default
bun run typecheck
bun run build
```

Expected: PASS.

- [ ] **Step 3: Run Worker mock check**

Run:

```bash
cd /Users/ethan/Documents/yunbay/workers/ldxp-browser-worker
bun run check
```

Expected: PASS.

- [ ] **Step 4: Manual local smoke**

Use a local admin account and test user:

1. Create a `paid_topup` test card in Yunbay admin UI.
2. Start backend with `LDXP_AUTO_TOPUP_ENABLED=true` and product JSON pointing to the fixed test link.
3. Start Worker with `LDXP_WORKER_MOCK_MODE=true` and `LDXP_WORKER_MOCK_CARD_KEY` set to the generated test card key.
4. Open wallet page.
5. Click `10`.
6. Confirm QR dialog appears.
7. Wait for mock result.
8. Confirm user quota increased and one top-up record exists.

- [ ] **Step 5: Commit mock mode**

Run:

```bash
git add workers/ldxp-browser-worker/src/mock-flow.ts workers/ldxp-browser-worker/src/index.ts workers/ldxp-browser-worker/src/config.ts workers/ldxp-browser-worker/tests
git commit -m "test: add ldxp worker mock flow"
```

---

## 19. Task 16: 真实测试商品 POC，仍不正式开放

**Files:**
- No code changes expected unless selectors need adjustment.
- If selectors need adjustment, modify `/Users/ethan/Documents/yunbay/workers/ldxp-browser-worker/src/browser-flow.ts` and add fixture coverage.

- [ ] **Step 1: Prepare secrets from local desktop folder**

Use `/Users/ethan/Desktop/云贝` to retrieve only:

- QQ IMAP authorization value for `10256345@qq.com`.
- Production or staging backend worker token.
- Test user/admin credentials if needed.

Do not echo these values. Store in local untracked files under `workers/ldxp-browser-worker/secrets/` for local compose, and add `workers/ldxp-browser-worker/secrets/` to `.git/info/exclude` if needed.

- [ ] **Step 2: Run Worker against real test product**

Start backend with:

```bash
LDXP_AUTO_TOPUP_ENABLED=true
LDXP_CONTACT_EMAIL=support@yunbay.xyz
LDXP_TOPUP_PRODUCTS_JSON='[{"amount":10,"money":0.10,"product_url":"https://pay.ldxp.cn/item/n4aqh8","product_name":"0.1 元测试"},{"amount":20,"money":0.10,"product_url":"https://pay.ldxp.cn/item/n4aqh8","product_name":"0.1 元测试"},{"amount":30,"money":0.10,"product_url":"https://pay.ldxp.cn/item/n4aqh8","product_name":"0.1 元测试"},{"amount":50,"money":0.10,"product_url":"https://pay.ldxp.cn/item/n4aqh8","product_name":"0.1 元测试"},{"amount":100,"money":0.10,"product_url":"https://pay.ldxp.cn/item/n4aqh8","product_name":"0.1 元测试"},{"amount":500,"money":0.10,"product_url":"https://pay.ldxp.cn/item/n4aqh8","product_name":"0.1 元测试"}]'
```

Important:

- This is test mode: `money=0.10` for all amounts.
- Do not give real 10/20/30/50/100/500 credit for this test unless the imported card keys are also deliberate test cards.

- [ ] **Step 3: Execute one full payment test**

1. Create one low-value `paid_topup` card and import it to the test LDXP product.
2. User clicks `10` in Yunbay.
3. Worker generates QR.
4. Scan and pay the small test amount.
5. Worker sees result page.
6. Worker mail poller posts matching email.
7. Backend verifies order/card/amount and redeems card.
8. Frontend shows success.

Expected database facts:

```sql
select session_id,status,worker_order_no,mail_order_no,verified_time,redeemed_time,error_code from ldxp_topup_sessions order by id desc limit 1;
select order_no,card_key,matched_session_id,processed from ldxp_mail_events order by id desc limit 1;
select payment_provider,status from top_ups order by id desc limit 1;
```

Expected:

- Session status `success`.
- Worker order equals mail order.
- Mail event processed true.
- Top-up provider `redemption_code`, status `success`.

- [ ] **Step 4: Commit selector fixes if any**

Run only if code changed:

```bash
git add workers/ldxp-browser-worker/src/browser-flow.ts workers/ldxp-browser-worker/tests/fixtures workers/ldxp-browser-worker/tests/browser-flow.test.ts
git commit -m "fix: stabilize ldxp browser selectors"
```

---

## 20. Task 17: 部署配置和 runbook

**Files:**
- Create: `/Users/ethan/Documents/yunbay/docs/ldxp-browser-worker-auto-topup-runbook.md`
- Do not modify: `/Users/ethan/Documents/yunbay/docker-compose.yml` in this plan.
- Production compose file is on server `/opt/new-api/app/docker-compose.prod.yml`; do not commit production secrets.

- [ ] **Step 1: Write runbook**

Create `/Users/ethan/Documents/yunbay/docs/ldxp-browser-worker-auto-topup-runbook.md` covering:

- Required feature branch/commit.
- Required prerequisite: paid_topup redemption support deployed.
- Required production secret names, without values:
  - `LDXP_WORKER_TOKEN`
  - `QQ_IMAP_PASSWORD`
  - product JSON
  - contact email
- How to generate Worker token locally without printing it in logs:

```bash
python3 - <<'PY' > /tmp/ldxp_worker_token
import secrets
print(secrets.token_urlsafe(48))
PY
chmod 600 /tmp/ldxp_worker_token
```

- Docker compose service shape for `ldxp-worker`.
- Smoke test sequence.
- Rollback:
  1. Set `LDXP_AUTO_TOPUP_ENABLED=false`.
  2. Restart backend.
  3. Stop `ldxp-worker` container.
  4. Manually process sessions in `worker_paid`, `verified`, `redeem_failed`.
- Manual support flow for failed sessions.
- Log redaction rules.

- [ ] **Step 2: Verify committed compose examples stay secret-free**

Run:

```bash
rg -n '/Users/ethan/Desktop/云贝|[Q]Q_IMAP_PASSWORD=|[L]DXP_WORKER_TOKEN=|[S]ESSION_SECRET=' workers/ldxp-browser-worker/docker-compose.example.yml docs/ldxp-browser-worker-auto-topup-runbook.md || true
```

Expected:

- No output.
- Compose examples reference secret file names such as `/run/secrets/qq_imap_password`, not local desktop credential paths or literal secret values.

- [ ] **Step 3: Verify docs contain no secret values**

Run:

```bash
rg -n 'BEGIN [A-Z ]*PRIVATE KEY|[密]码为|[授]权码为|[P]OSTGRES_PASSWORD=|[R]EDIS_PASSWORD=|[S]ESSION_SECRET=|[Q]Q_IMAP_PASSWORD=[^$]|[L]DXP_WORKER_TOKEN=[^$]' docs/ workers/ldxp-browser-worker docker-compose.yml || true
```

Expected:

- No secret values are printed.
- References to variable names are acceptable.

- [ ] **Step 4: Commit**

Run:

```bash
git add docs/ldxp-browser-worker-auto-topup-runbook.md workers/ldxp-browser-worker/docker-compose.example.yml
git commit -m "docs: add ldxp worker deployment runbook"
```

---

## 21. Task 18: 全量验证、PR 准备与上线门禁

**Files:**
- All files touched by previous tasks.

- [ ] **Step 1: Backend full verification**

Run:

```bash
go test ./model ./service ./controller ./router -count=1
go test ./... -run Ldxp -count=1
```

Expected: PASS.

- [ ] **Step 2: Worker verification**

Run:

```bash
cd /Users/ethan/Documents/yunbay/workers/ldxp-browser-worker
bun run check
docker build -t yunbay-ldxp-browser-worker:local .
```

Expected: PASS.

- [ ] **Step 3: Frontend verification**

Run:

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun test src/features/wallet/lib/ldxp-topup.test.ts
bun run i18n:sync
bun run typecheck
bun run build
```

Expected: PASS.

- [ ] **Step 4: Sensitive info scan**

Run:

```bash
cd /Users/ethan/Documents/yunbay
rg -n 'BEGIN [A-Z ]*PRIVATE KEY|[密]码为|[授]权码为|[P]OSTGRES_PASSWORD=|[R]EDIS_PASSWORD=|[S]ESSION_SECRET=|[Q]Q_IMAP_PASSWORD=[^$]|[L]DXP_WORKER_TOKEN=[^$]|10256345.*授[权]码' . \
  -g '!web/node_modules/**' \
  -g '!web/default/node_modules/**' \
  -g '!workers/ldxp-browser-worker/node_modules/**' \
  -g '!workers/ldxp-browser-worker/secrets/**' \
  -g '!logs/**' || true
```

Expected:

- No actual secret values in tracked files.

- [ ] **Step 5: Git diff review**

Run:

```bash
git status --short
git diff --stat origin/main...HEAD
git diff --check
git log --oneline --decorate --max-count=20
```

Expected:

- Diff only contains planned feature files.
- `git diff --check` passes.

- [ ] **Step 6: Production deploy gate**

Before enabling production:

- Confirm GitHub `yunbay` branch to deploy.
- Confirm `paid_topup` prerequisite is deployed and migrated.
- Confirm formal product links for 10/20/30/50/100/500 are configured. If all six still point to `https://pay.ldxp.cn/item/n4aqh8`, keep production feature disabled or test-only.
- Confirm `support@yunbay.xyz` still routes to `10256345@qq.com` and IMAP authorization works.
- Confirm Worker token and QQ IMAP password are stored as Docker secrets or server secret files, not committed env files.
- Confirm one small real test payment succeeds before opening to users.

- [ ] **Step 7: Final commit or PR**

If all tasks are already committed separately, no final squash is required. If the branch contains uncommitted verification doc updates:

```bash
git add docs/ldxp-browser-worker-auto-topup-runbook.md
git commit -m "docs: record ldxp topup verification"
```

When creating PR, follow `/Users/ethan/Documents/yunbay/.github/PULL_REQUEST_TEMPLATE.md` and compare current git user with historical authors as required by `/Users/ethan/Documents/yunbay/AGENTS.md`.

---

## 22. Self-Review / 计划覆盖检查

Spec requirement coverage:

- 固定金额 10/20/30/50/100/500：Task 3 config validation + Task 12/13 frontend fixed buttons。
- 商品链接可全部先绑定 `https://pay.ldxp.cn/item/n4aqh8`：Task 2 config contract + Task 16 test POC。
- Worker 内部打开购买链接、填写云贝邮箱、生成支付宝二维码：Task 9 browser flow。
- 二维码传回云贝前台：Task 7 Worker QR API + Task 13 dialog。
- 用户扫码支付后 Worker 自动跳转卡密页：Task 9 result page wait/extract。
- 邮件核验：Task 5 backend parser + Task 10 Worker IMAP poller。
- 订单号与邮箱订单号一致才核验：Task 6 verifier。
- 卡密一致才入账：Task 6 verifier。
- 使用云贝 paid_topup 卡密兑换入账：Task 1 prerequisite + Task 6 redeem。
- 生产 SMTP/QQ 邮箱/Cloudflare 路由事实：Task 0 credentials boundary + Task 17 runbook；本实现只需要 IMAP 读取购买成功邮件，不依赖 SMTP 发信。
- 实施前先查看 GitHub `yunbay` 项目：Task 0 Step 1。
- 服务器生产密钥在本地桌面 `云贝` 文件夹：Task 0 Step 3 + Task 16 Step 1 + Task 17 secret handling。
- 不泄露 secret：Task 0、Task 17、Task 18 sensitive scan。
- SQLite/MySQL/PostgreSQL 兼容：Task 2 GORM-only model design + tests。
- Go JSON wrapper 规则：Task 3 config parser明确使用 `common.UnmarshalJsonStr`。
- 前端 Bun：Task 12/13/14/18 使用 `bun`。
- i18n：Task 14 覆盖六种前端语言。

Known execution notes:

- 真实支付 POC 只能用测试商品和低价值卡密，不能在所有正式金额仍指向测试商品时开放生产自动入账。
- Worker selector 以运行时页面为准；如果联动小铺页面改版，Task 16 要通过 fixture 增补把修复固化。
- Worker 邮箱轮询读取的是 QQ IMAP，不改变既有 SMTP 发信配置；`support@yunbay.xyz` 通过 Cloudflare 路由到 QQ 邮箱即可满足核验链路。

---

## 23. Execution Status Update / 执行状态更新（2026-06-29 09:20 CST）

> 本节是执行过程中的滚动状态记录，用于反映真实实施与灰测过程中产生的补充工作。原计划中的详细 checkbox 保持作为施工步骤参考；本节作为当前事实源，用于判断“做到哪一步、还剩什么”。

### 23.1 当前总体进度

- **主线开发状态：约 85% 完成。** 后端模型/状态机/API、Worker、前端钱包入口与支付弹窗、生产灰测配置均已落地。
- **当前卡点：真实灰测闭环还未用“修复后的新订单”完成最终确认。** 最新一次失败 `LD260629IK87P7` 已定位为 Worker 在链动小铺 SPA 结果页白屏/未渲染时过早读取文本导致；修复已部署，需用户重新发起一笔新订单验证。
- **上线门禁仍未完成：** runbook 文件未落地、全量测试/敏感信息扫描/PR 准备未完成，生产仍应保持 `LDXP_ALLOWED_USERNAMES=jiance001` 灰测范围，不应全量开放。
- **生产当前灰测策略：** 只允许账号 `jiance001`；`LDXP_REQUIRE_MAIL_MATCH=false`，后台识别支付成功后直接走网站充值入账；邮件链路保留为后续审计，不阻塞到账。

### 23.2 原计划 Task 完成状态

| Task | 原计划内容 | 当前状态 | 备注 |
|---|---|---:|---|
| Task 0 | 前置核验、隔离分支、凭据边界 | ✅ 已完成 | 使用隔离 worktree `/Users/ethan/Documents/yunbay/.worktrees/ldxp-browser-worker-auto-topup`；未输出 secret 明文。 |
| Task 1 | paid_topup 卡密兑换前置能力 | ✅ 已完成 | 已具备 `paid_topup` 入账/VIP 升级相关能力；后续灰测追加了“不走卡密、直接充值入账”的替代路径。 |
| Task 2 | LDXP 数据模型与迁移 | ✅ 已完成 | `LdxpTopupSession`、`LdxpMailEvent` 及迁移、测试已实现。 |
| Task 3 | LDXP 配置解析与脱敏 helper | ✅ 已完成 | 追加了 `LDXP_ALLOWED_USERNAMES`、`LDXP_REQUIRE_MAIL_MATCH` 等灰测/直接入账配置。 |
| Task 4 | 后端 session 状态机服务 | ✅ 已完成 | 追加修复：取消订单后 Worker 迟到回传 QR/error 时，同 worker 的迟到回传改为 no-op，避免把已取消订单写成失败。 |
| Task 5 | 邮件解析与邮件事件落库 | ✅ 已完成 / 生产暂不阻塞 | IMAP/邮件事件能力保留；生产灰测按用户要求不等待邮件匹配，邮件仅用于后续审计。 |
| Task 6 | 核验与入账幂等服务 | ✅ 已完成并变更路径 | 原计划是“Worker + 邮件 + 卡密兑换”；灰测中追加实现 `LDXP_REQUIRE_MAIL_MATCH=false` 时的 Worker paid fields 校验 + `top_ups` 直接成功充值 + VIP 升级 + 幂等防重。 |
| Task 7 | 后端 HTTP API 与路由 | ✅ 已完成 | 用户 API、Worker API、Admin retry/list 等已实现；生产已用于灰测。 |
| Task 8 | Worker 服务脚手架与后端客户端 | ✅ 已完成 | Worker claim、QR/result/error/mail callback 已实现。 |
| Task 9 | Worker Playwright 商品页、二维码、结果页流程 | ✅ 已完成，仍需性能优化 | 已能生成二维码；追加了手动跳转 race、无卡密 paid result、结果页 paid marker 等待、结果页无订单号 fallback。二维码仍约 20+ 秒，已追加阶段 timing 日志等待下一笔定位。 |
| Task 10 | Worker IMAP 邮件轮询 | ✅ 已实现 / 生产暂禁用 | 生产当前 `LDXP_REQUIRE_MAIL_MATCH=false`，IMAP 不作为入账前置。 |
| Task 11 | Worker 主循环、并发控制、Dockerfile | ✅ 已完成 | 生产部署 `LDXP_WORKER_CONCURRENCY=1`、`LDXP_RELEASE_SLOT_AFTER_QR=false`；追加阶段耗时 diagnostics 日志。 |
| Task 12 | 前端 API、hook 与纯函数测试 | ✅ 已完成 | 钱包页可创建、轮询、取消 LDXP session。 |
| Task 13 | 前端固定金额卡片与支付弹窗 | ✅ 已完成 | 曾因入口/二维码展示问题做过联调修正；当前生产钱包页已可打开支付宝自动充值弹窗。 |
| Task 14 | 前端 i18n 补齐 | ✅ 基本完成 | 仍需最终跑一次前端全量 build/i18n 检查作为上线门禁。 |
| Task 15 | 本地端到端联调，不真实支付 | 🟡 部分完成 | 本地容器/前端曾启动供自测；后续用户要求进入生产灰测，本地 mock/E2E 未作为最终门禁收尾。 |
| Task 16 | 真实测试商品 POC | 🟡 进行中 | 生产灰测已开始并发现/修复问题；还缺“修复后新订单支付成功并直接到账”的确认。 |
| Task 17 | 部署配置和 runbook | 🟡 部分完成 | 生产配置/部署已做；正式 runbook 文件 `docs/ldxp-browser-worker-auto-topup-runbook.md` 尚未创建。 |
| Task 18 | 全量验证、PR 准备与上线门禁 | ⏳ 未完成 | 需跑后端/前端/Worker 全量测试、敏感信息扫描、diff review、PR 模板准备。 |

### 23.3 计划外补充工作清单

以下事项不在最初计划的主路径中，是根据真实灰测反馈和用户新的产品要求追加完成或追加中的工作：

1. **入账路径从“卡密兑换”扩展为“网站直接充值入账”。**
   - 背景：用户要求“别走卡密入账，直接走网站的充值入账吧，不填卡密”。
   - 结果：新增 `LDXP_REQUIRE_MAIL_MATCH=false` 模式；Worker 识别支付成功后，后端直接创建成功 `top_ups` 记录并给用户加 quota。
   - 关键防护：仍校验 worker order no、金额、paid 状态文本、重复订单；不是无条件入账。

2. **邮件核验从“入账前置条件”降级为“后续审计材料”。**
   - 背景：用户要求“后台识别到支付成功，就直接充值到账，邮件核对用于后面的订单审计”。
   - 结果：生产灰测关闭邮件阻塞：`LDXP_REQUIRE_MAIL_MATCH=false`。
   - 保留：邮件解析/落库/匹配能力仍在，后续可用于审计或重新打开强校验模式。

3. **生产灰测 allowlist。**
   - 背景：用户要求“不全面上线，只用我的 `jiance001` 这个账号进行灰测”。
   - 结果：生产配置 `LDXP_ALLOWED_USERNAMES=jiance001`，前端入口只对该账号开放。

4. **直接充值也计入 VIP 升级阈值。**
   - 背景：充值应等价网站充值，而不是普通赠送 quota。
   - 结果：直接 LDXP topup 成功后会走 `MaybeUpgradeUserToVIPTx`，达到阈值时刷新 VIP 用户组缓存。

5. **Worker 支持“支付成功但无卡密”的结果页。**
   - 背景：直接网站充值模式不再依赖卡密。
   - 结果：新增 `extractOptionalCardKey()`，无卡密不再导致 Worker 失败。

6. **修复 `LDXP_RELEASE_SLOT_AFTER_QR` 生产环境偏差。**
   - 发现：实际 worker env 一度为 `LDXP_RELEASE_SLOT_AFTER_QR=true`，导致出二维码后不继续等支付结果。
   - 结果：生产已强制为 `false`，Worker 会继续等待支付完成。

7. **取消订单后的 Worker 迟到回传 no-op。**
   - 发现：用户取消/前端超时后，Worker 仍可能迟到回传 QR 或 error，旧逻辑会把已取消订单写成错误状态。
   - 结果：`RecordLdxpQr` / `RecordLdxpWorkerError` 对“同 worker + 已取消 session”的迟到回传返回 nil，不污染最终状态。

8. **联动小铺“立即跳转”页面优化。**
   - 发现：购买后可能先出现“如页面未自动跳转支付页，请点击下方按钮跳转”。
   - 结果：Worker 不再固定等待 popup；改为 popup、当前页 cashier ready、手动跳转三路 race。

9. **支付结果页 SPA 白屏/未渲染防护。**
   - 发现：订单 `LD260629IK87P7` 支付后失败，生产快照标题为“订单详情 - 链动小铺”但截图白屏；Worker 过早从空白结果页提取订单号导致 `worker_failed`。
   - 结果：`waitForPaidResult` 现在不仅等 `/order/result/` URL，还要等 `已付款/支付成功` 等 paid marker；结果页缺订单号时复用收银台阶段已拿到的 worker order no。

10. **二维码慢的阶段耗时 diagnostics。**
    - 现状：二维码仍约 20+ 秒，盲目优化无效。
    - 结果：生产 Worker 已追加 timings 日志，下一笔订单会记录 `browser_launch`、`product_goto`、`click_purchase_to_cashier`、`wait_cashier_ready`、`extract_qr`、`post_qr_callback` 等阶段耗时，用于定位瓶颈。

11. **生产直接热修与备份流程。**
    - 已做备份目录包括：
      - `/opt/new-api/backups/ldxp-result-fallback-20260629091147`
      - `/opt/new-api/backups/ldxp-prod-index-timings-20260629091400`
    - 注意：生产 `workers/ldxp-browser-worker/src/index.ts` 做过最小 timing 补丁；不要直接用本地含 mock-mode 的 `index.ts` 覆盖生产，除非同步其所有依赖并重新 review。

### 23.4 最近一次故障与修复状态

- 故障订单：`LD260629IK87P7`
- 用户看到：`Worker failed, please contact support`
- DB/日志结论：QR 已回传，失败发生在支付后结果页读取阶段。
- 生产快照结论：链动小铺订单详情 SPA 结果页白屏/未渲染，导致 Worker 提取订单号失败。
- 已修复：
  - paid result 必须等待 paid marker；
  - 结果页无订单号时 fallback 到收银台订单号；
  - 空白结果页不会误判成功；
  - 对应 Worker 测试已通过。
- 待验证：用户重新发起一笔新订单，确认支付后自动直接到账。

### 23.5 已执行过的关键验证

- Worker 本地验证：

```bash
cd /Users/ethan/Documents/yunbay/.worktrees/ldxp-browser-worker-auto-topup/workers/ldxp-browser-worker
npm run check
```

最近结果：

```text
# tests 54
# pass 54
# fail 0
```

- 后端局部验证已覆盖：
  - `service` LDXP session late callback no-op；
  - `service/controller` LDXP 相关测试；
  - direct topup / VIP upgrade 相关测试。

- 生产部署验证：
  - `yunbay-new-api` healthy；
  - `yunbay-ldxp-browser-worker` up；
  - worker dist 中已确认包含 `buildPaidResultFromText`、`waitForPaidMarker`、`extractOrderNoOrFallback`、`timings`。

### 23.6 下一步任务队列

1. **等待/执行一笔新的生产灰测订单。**
   - 用户使用 `jiance001` 在 `https://yunbay.xyz/wallet` 新建订单并支付。
   - 验证目标：支付完成后无需邮件，订单直接变 `success`，quota 到账，必要时 VIP 升级生效。

2. **读取新订单 timing 日志，定位二维码 20+ 秒原因。**
   - 若瓶颈在浏览器冷启动：考虑持久 browser/context 或预热。
   - 若瓶颈在商品页/收银台加载：考虑更精确 selector、提前点击跳转、减少等待条件。
   - 若瓶颈在后端轮询：调整前端轮询间隔或状态推送策略。

3. **补齐正式 runbook。**
   - 创建 `/Users/ethan/Documents/yunbay/.worktrees/ldxp-browser-worker-auto-topup/docs/ldxp-browser-worker-auto-topup-runbook.md`。
   - 记录灰测/回滚/生产 secret 名称/日志排查步骤，不写 secret 明文。

4. **做全量上线门禁。**
   - 后端：`go test ./model ./service ./controller ./router -count=1`，必要时跑更大范围。
   - Worker：`npm run check`。
   - 前端：`bun run build`、i18n sync/check。
   - 安全：敏感信息扫描、`git diff --check`、diff review。

5. **决定是否保持 direct topup 为正式路径。**
   - 如果正式采用：更新 spec/runbook，把“邮件 + 卡密兑换”从入账前置改为审计/可选强校验。
   - 如果恢复强校验：重新启用 `LDXP_REQUIRE_MAIL_MATCH=true` 并完成 IMAP 生产验证。

6. **PR/提交整理。**
   - 当前 worktree 存在多类混合改动，不能整体无脑提交。
   - 需要按后端、Worker、前端、docs 分组 review/stage。
   - 创建 PR 时遵守 `.github/PULL_REQUEST_TEMPLATE.md` 和 AGENTS.md 的 AI-assisted 说明要求。
