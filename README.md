<div align="center">

![new-api](/web/default/public/logo.png)

# New API

🍥 **Next-Generation LLM Gateway and AI Asset Management System**

<p align="center">
  <a href="./README.zh_CN.md">简体中文</a> |
  <a href="./README.zh_TW.md">繁體中文</a> |
  <strong>English</strong> |
  <a href="./README.fr.md">Français</a> |
  <a href="./README.ja.md">日本語</a>
</p>

<p align="center">
  <a href="https://raw.githubusercontent.com/Calcium-Ion/new-api/main/LICENSE">
    <img src="https://img.shields.io/github/license/Calcium-Ion/new-api?color=brightgreen" alt="license">
  </a><!--
  --><a href="https://github.com/Calcium-Ion/new-api/releases/latest">
    <img src="https://img.shields.io/github/v/release/Calcium-Ion/new-api?color=brightgreen&include_prereleases" alt="release">
  </a><!--
  --><a href="https://hub.docker.com/r/CalciumIon/new-api">
    <img src="https://img.shields.io/badge/docker-dockerHub-blue" alt="docker">
  </a><!--
  --><a href="https://goreportcard.com/report/github.com/Calcium-Ion/new-api">
    <img src="https://goreportcard.com/badge/github.com/Calcium-Ion/new-api" alt="GoReportCard">
  </a>
</p>

<p align="center">
  <a href="https://trendshift.io/repositories/20180" target="_blank">
    <img src="https://trendshift.io/api/badge/repositories/20180" alt="QuantumNous%2Fnew-api | Trendshift" style="width: 250px; height: 55px;" width="250" height="55"/>
  </a>
  <br>
  <a href="https://hellogithub.com/repository/QuantumNous/new-api" target="_blank">
    <img src="https://api.hellogithub.com/v1/widgets/recommend.svg?rid=539ac4217e69431684ad4a0bab768811&claim_uid=tbFPfKIDHpc4TzR" alt="Featured｜HelloGitHub" style="width: 250px; height: 54px;" width="250" height="54" />
  </a><!--
  --><a href="https://www.producthunt.com/products/new-api/launches/new-api?embed=true&utm_source=badge-featured&utm_medium=badge&utm_campaign=badge-new-api" target="_blank" rel="noopener noreferrer">
    <img src="https://api.producthunt.com/widgets/embed-image/v1/featured.svg?post_id=1047693&theme=light&t=1769577875005" alt="New API - All-in-one AI asset management gateway. | Product Hunt" style="width: 250px; height: 54px;" width="250" height="54" />
  </a>
</p>

<p align="center">
  <a href="#-quick-start">Quick Start</a> •
  <a href="#-key-features">Key Features</a> •
  <a href="#-deployment">Deployment</a> •
  <a href="#-documentation">Documentation</a> •
  <a href="#-help-support">Help</a>
</p>

</div>

## 📝 Project Description

> [!IMPORTANT]
> - This project is intended solely for lawful and authorized AI API gateway, organization-level authentication, multi-model management, usage analytics, cost accounting, and private deployment scenarios.
> - Users must lawfully obtain upstream API keys, accounts, model services, and interface permissions, and must comply with upstream terms of service and applicable laws and regulations.
> - Users should ensure their use complies with upstream terms of service and applicable laws and regulations.
> - When providing generative AI services to the public, users should comply with applicable regulatory requirements and fulfill all filing, licensing, content safety, real-name verification, log retention, tax, and upstream authorization obligations required by their jurisdiction.

---

## 🤝 Trusted Partners

<p align="center">
  <em>No particular order</em>
</p>

<p align="center">
  <a href="https://www.cherry-ai.com/" target="_blank">
    <img src="./docs/images/cherry-studio.png" alt="Cherry Studio" height="80" />
  </a><!--
  --><a href="https://github.com/iOfficeAI/AionUi/" target="_blank">
    <img src="./docs/images/aionui.png" alt="Aion UI" height="80" />
  </a><!--
  --><a href="https://bda.pku.edu.cn/" target="_blank">
    <img src="./docs/images/pku.png" alt="Peking University" height="80" />
  </a><!--
  --><a href="https://www.compshare.cn/?ytag=GPU_yy_gh_newapi" target="_blank">
    <img src="./docs/images/ucloud.png" alt="UCloud" height="80" />
  </a><!--
  --><a href="https://www.aliyun.com/" target="_blank">
    <img src="./docs/images/aliyun.png" alt="Alibaba Cloud" height="80" />
  </a><!--
  --><a href="https://io.net/" target="_blank">
    <img src="./docs/images/io-net.png" alt="IO.NET" height="80" />
  </a>
</p>

---

## 🙏 Special Thanks

<p align="center">
  <a href="https://www.jetbrains.com/?from=new-api" target="_blank">
    <img src="https://resources.jetbrains.com/storage/products/company/brand/logos/jb_beam.png" alt="JetBrains Logo" width="120" />
  </a>
</p>

<p align="center">
  <strong>Thanks to <a href="https://www.jetbrains.com/?from=new-api">JetBrains</a> for providing free open-source development license for this project</strong>
</p>

---

## 🚀 Quick Start

### Using Docker Compose (Recommended)

```bash
# Clone the project
git clone https://github.com/QuantumNous/new-api.git
cd new-api

# Edit docker-compose.yml configuration
nano docker-compose.yml

# Start the service
docker-compose up -d
```

<details>
<summary><strong>Using Docker Commands</strong></summary>

```bash
# Pull the latest image
docker pull calciumion/new-api:latest

# Using SQLite (default)
docker run --name new-api -d --restart always \
  -p 3000:3000 \
  -e TZ=Asia/Shanghai \
  -v ./data:/data \
  calciumion/new-api:latest

# Using MySQL
docker run --name new-api -d --restart always \
  -p 3000:3000 \
  -e SQL_DSN="root:123456@tcp(localhost:3306)/oneapi" \
  -e TZ=Asia/Shanghai \
  -v ./data:/data \
  calciumion/new-api:latest
```

> **💡 Tip:** `-v ./data:/data` will save data in the `data` folder of the current directory, you can also change it to an absolute path like `-v /your/custom/path:/data`

</details>

---

🎉 After deployment is complete, visit `http://localhost:3000` to start using!

> [!WARNING]
> When operating this project as a public generative AI service or API resale service, users should first complete all required filing, licensing, content safety, real-name verification, log retention, tax, payment, and upstream authorization obligations.

📖 For more deployment methods, please refer to [Deployment Guide](https://docs.newapi.pro/en/docs/installation)

---

## 📚 Documentation

<div align="center">

### 📖 [Official Documentation](https://docs.newapi.pro/en/docs) | [![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/QuantumNous/new-api)

</div>

**Quick Navigation:**

| Category | Link |
|------|------|
| 🚀 Deployment Guide | [Installation Documentation](https://docs.newapi.pro/en/docs/installation) |
| ⚙️ Environment Configuration | [Environment Variables](https://docs.newapi.pro/en/docs/installation/config-maintenance/environment-variables) |
| 🛠️ Yunbay Maintenance | [README maintenance audit/log](#maintenance-audit-log) · [Repository maintenance notes, production sync notes, and usage logs compatibility](./docs/yunbay-maintenance.md) · [GitHub project context and PR queue](./.github/README.md) |
| 📡 API Documentation | [API Documentation](https://docs.newapi.pro/en/docs/api) |
| ❓ FAQ | [FAQ](https://docs.newapi.pro/en/docs/support/faq) |
| 💬 Community Interaction | [Communication Channels](https://docs.newapi.pro/en/docs/support/community-interaction) |

---

<a id="maintenance-audit-log"></a>

## 🛠️ 维护审计与持续维护日志

本节是项目的**最终维护日志入口**。所有维护人员在完成一次维护、修复、上线、回滚、审计或生产同步后，都必须在本节末尾追加记录；**只能追加，不能复写、覆盖、删除或替换前面的历史内容**。如果之前的记录有误，请新增一条“更正记录”说明原因和正确结论，不要直接改写旧记录。

### 维护记录追加规则

每次维护完成后，请追加一条记录，并至少包含：

- 日期与时区：使用 `YYYY-MM-DD`，默认按 `Asia/Shanghai` 记录。
- 操作人：Git 作者、维护人或负责账号。
- 分支与提交：记录维护所在分支、关键 commit hash；未提交时写明工作区状态。
- 维护范围：列出涉及的模块、页面、接口、配置、部署目标或文档。
- 验证结果：记录实际执行过的测试、构建、冒烟检查或生产验证命令及结果摘要。
- 生产状态：说明是否已同步生产、是否需要重启服务、是否有回滚点。
- 后续事项：记录遗留风险、待合并分支、待清理 worktree、待补测试或待复核项。

> 维护完成后的硬性要求：**先验证，再记录；记录时追加到本节末尾；不要为了“整理格式”覆盖旧日志。**

### 历史版本审计基线（2026-06-30）

本次审计覆盖本地仓库当前可达的全部提交历史与本地/远端引用，审计基线如下：

- 仓库状态：`git rev-parse --is-shallow-repository` 返回 `false`，当前不是浅克隆。
- 审计范围：`git log --all`、`git for-each-ref refs/heads refs/remotes`、`git worktree list`、`git fsck --full --no-reflogs`。
- 全引用提交数：`6066` 个提交；当前分支提交数：`5937` 个提交。
- 历史范围：从 `2023-04-22` 的 `4cbef078`（`Initial commit`）到 `2026-06-30` 的 `83479b48`（`fix(router): register user group tags route`）。
- 年度分布：`2023: 833`、`2024: 1154`、`2025: 2925`、`2026: 1154`。
- 远端引用：`origin` 当前有 `10` 个 head、`0` 个 tag，本地 `origin/*` 与远端 head 已对齐。
- 本地 tag：`0` 个。
- 历史主题分布（按提交主题粗分类）：`feat: 1385`、`fix: 1332`、`docs: 162`、`refactor: 246`、`chore: 261`、其他历史提交 `2580`。
- 主要变更热点（按历史触达提交粗分类）：后端 Go `2978`、relay/provider 适配 `1237`、model/db `697`、i18n `436`、docs/README `413`、tests `239`、infra/deploy `181`、`web/default` `203`。
- 结构结论：历史上项目从上游基础能力演进到当前多前端、多 provider relay、数据库模型、生产部署与云贝本地维护并行的状态；当前维护需要同时关注后端兼容性、默认前端、i18n、生产同步文档和多个并行 Codex worktree/分支。
- 风险结论：本地存在多个尚未合并到当前分支的 `codex/*` 维护分支与 worktree；删除、reset、clean、覆盖同步前必须确认对应分支是否已经合并、推送或另行备份。
- 悬空提交：`git fsck` 发现 `5` 个 dangling commit（均为 2026-06-25 到 2026-06-29 的本地维护相关提交）。这些提交不可直接视为当前产品基线；如需恢复，必须先新建分支保存，再审查 diff。

### 持续维护日志

| 日期 | 操作人 | 分支 / 提交 | 维护范围 | 验证结果 | 生产状态 | 后续事项 |
|------|--------|-------------|----------|----------|----------|----------|
| 2026-06-30 | Codex | `codex/fix-usage-logs-stat-null` / `ff849d72`；全引用最新 `83479b48` | 审计全部可达 Git 历史、本地/远端引用、worktree、悬空提交；在 `README.md` 建立追加式最终维护日志 | 已执行 Git 历史统计、引用核对、worktree 清点、`git fsck --full --no-reflogs`；本次仅文档维护，未运行后端/前端构建 | 未同步生产；无服务重启 | 以后每次维护都必须在本表末尾追加记录；不要复写旧记录；合并或清理 `codex/*` 分支前先确认对应功能和生产状态 |
| 2026-06-30 | Codex | `codex/fix-usage-logs-stat-null` / `0a420c3c` | 补提交已在生产执行并验证过的邮件链路维护文档：`docs/email-delivery.md`、`docs/yunbay-maintenance.md`、`infra/cloudflare-plan.md` | 已执行目标文档 diff 核对、敏感信息模式检查、`git diff --check`；提交 `docs: record yunbay email delivery setup` | 记录对应生产状态：Resend SMTP 出站、Cloudflare Email Routing 入站、应用发信验证、`yunbay-new-api` healthy；本次提交本身仅补仓库文档 | 后续邮件配置变更继续追加记录；不要写入 Resend API Key、SMTP Token、Cloudflare Token 或备份文件内容 |
| 2026-06-30 | Codex | `codex/full-rollout-no-overlap-clean` / `83479b48`；公告实现 `59756734` | 补记全量上线与必读公告弹窗生产完成事实，修正旧设计文档中的“等待实现/落地”状态 | 已复核生产公开入口 `/`、`/api/status`、`/api/notice` 均为 HTTP 200；生产入口 JS `/static/js/index.599262f2f0.js` 包含 `I have read`、`我已阅读`、`notification-storage`、`markNoticeRead`、`markAnnouncementsRead` | 已完成生产同步；本次仓库维护只补文档事实，不重新部署服务 | 后续覆盖 wallet、Quick Start、公告、路由或分组逻辑前必须对照全量上线记录；Jeepay 与 Sub2API billing 仍不属于本轮完成范围 |
| 2026-06-30 | Codex | 用户标签 `a1a38836` / `0f0ac266`；Claude 钱包灰测 `1ea932fd` | 补提交用户标签与模型分组分离设计、Claude 价格与钱包灰测设计，并修正旧状态为已生产同步/已生产灰测 | 已核对用户标签生产同步记录、用户标签接口与生产数据复核；已核对 Claude 钱包灰测记录、生产 smoke、bundle 脱敏字符串检查和 `jiance001` 灰测摘要 | 用户标签功能已同步生产并纳入全量上线；Claude 钱包改动已完成代码侧上线和灰测，但 Claude 真实价格 options 与 LDXP 折扣真实支付闭环保留后续变更窗口 | 后续不要把用户标签和模型分组混用；Claude/LDXP 扩大前必须备份 options/商品映射并追加维护记录 |
| 2026-06-30 | Codex | `codex/fix-usage-logs-stat-null` / 本次 GitHub 文档维护提交；GitHub 仓库 `chenli17683185032-ai/yunbay` | 重新盘点 Git / GitHub 工作数量，并新增 `.github/README.md` 作为 GitHub 项目脉络、PR 队列和 workflow 维护入口 | 已执行 `git fetch --all --prune --tags`、`gh repo view`、`gh pr list`、`gh issue list`、分支/提交计数与 workflow 文件审阅；结果：全引用 `6071` 提交、当前 HEAD `5942` 提交、`origin/main` `5922` 提交、开放 Draft PR `7` 个、开放 issue `0` 个、workflow `7` 个、本地分支 `18` 个、远端 head `10` 个 | 本次仅维护 GitHub / README 文档，未同步生产、未重启服务；`infra/sub2api/**` 4 个未提交代码文件继续排除 | 推送当前分支前确认本地领先远端当前分支 `5` 个文档/维护提交；处理 PR 时优先看 `.github/README.md` 的队列关系，尤其 `#5` 冲突和 `#6 → #7` 依赖 |
| 2026-07-11 | Codex | 代码 / 生产发布基线 `1bd746f5`；本行所在台账提交为后续文档提交；GitHub 仓库 `chenli17683185032-ai/yunbay` | 补齐 GPT-5.6 / 生产修复发布后的 GitHub 工作数量维护，替换 6 月 30 日已失效的 PR、分支、worktree 与 workflow 快照 | 已执行实时 `git fetch --all --prune --tags`、Git 引用 / worktree 计数、`gh pr list`、`gh issue list`、`gh workflow list` 与远端 SHA 核对；台账提交前快照为：全引用 `6366` 提交，`main` / `origin/main` 各 `6195`，开放 PR `5`、issue `0`，本地分支 `46`、远端 head `12`、worktree `40`，仓库 workflow `8`、含动态 Dependabot workflow 共 `10` | 本次只更新 GitHub 台账文档；文档提交会让 `main` 前进但生产仍保持部署 SHA `1bd746f5`，未重启服务；独立 `bayma888/sub2api-bmai` fork 未同步且当前账号只有读取权限 | 后续按 `.github/README.md` 的 Dependabot PR 分组验证；先联动验证 #46 / #48，再处理 #47、#49 和 #45；清理 worktree 前逐一审计未提交 / 未推送内容 |

---

## ✨ Key Features

> For detailed features, please refer to [Features Introduction](https://docs.newapi.pro/en/docs/guide/wiki/basic-concepts/features-introduction)

### 🎨 Core Functions

| Feature | Description |
|------|------|
| 🎨 New UI | Modern user interface design |
| 🌍 Multi-language | Supports Simplified Chinese, Traditional Chinese, English, French, Japanese |
| 🔄 Data Compatibility | Fully compatible with the original One API database |
| 📈 Data Dashboard | Visual console and statistical analysis |
| 🔒 Permission Management | Token grouping, model restrictions, user management |

### 💰 Authorized Usage Accounting and Billing

- ✅ Internal top-up and quota allocation for lawful authorized scenarios (EPay, Stripe)
- ✅ Organization-level per-request, usage-based, and cache-hit cost accounting
- ✅ Cache billing statistics for OpenAI, Azure, DeepSeek, Claude, Qwen, and supported models
- ✅ Flexible billing policies for internal management or authorized enterprise customers

### 🔐 Authorization and Security

- 😈 Discord authorization login
- 🤖 LinuxDO authorization login
- 📱 Telegram authorization login
- 🔑 OIDC unified authentication
- 🔍 Key quota query usage (with [new-api-key-tool](https://github.com/Calcium-Ion/new-api-key-tool))

### 🚀 Advanced Features

**API Format Support:**
- ⚡ [OpenAI Responses](https://docs.newapi.pro/en/docs/api/ai-model/chat/openai/create-response)
- ⚡ [OpenAI Realtime API](https://docs.newapi.pro/en/docs/api/ai-model/realtime/create-realtime-session) (including Azure)
- ⚡ [Claude Messages](https://docs.newapi.pro/en/docs/api/ai-model/chat/create-message)
- ⚡ [Google Gemini](https://doc.newapi.pro/en/api/google-gemini-chat)
- 🔄 [Rerank Models](https://docs.newapi.pro/en/docs/api/ai-model/rerank/create-rerank) (Cohere, Jina)

**Intelligent Routing:**
- ⚖️ Channel weighted random
- 🔄 Automatic retry on failure
- 🚦 User-level model rate limiting

**Format Conversion:**
- 🔄 **OpenAI Compatible ⇄ Claude Messages**
- 🔄 **OpenAI Compatible → Google Gemini**
- 🔄 **Google Gemini → OpenAI Compatible** - Text only, function calling not supported yet
- 🚧 **OpenAI Compatible ⇄ OpenAI Responses** - In development
- 🔄 **Thinking-to-content functionality**

**Reasoning Effort Support:**

<details>
<summary>View detailed configuration</summary>

**OpenAI series models:**
- `o3-mini-high` - High reasoning effort
- `o3-mini-medium` - Medium reasoning effort
- `o3-mini-low` - Low reasoning effort
- `gpt-5-high` - High reasoning effort
- `gpt-5-medium` - Medium reasoning effort
- `gpt-5-low` - Low reasoning effort

**Claude thinking models:**
- `claude-3-7-sonnet-20250219-thinking` - Enable thinking mode

**Google Gemini series models:**
- `gemini-2.5-flash-thinking` - Enable thinking mode
- `gemini-2.5-flash-nothinking` - Disable thinking mode
- `gemini-2.5-pro-thinking` - Enable thinking mode
- `gemini-2.5-pro-thinking-128` - Enable thinking mode with thinking budget of 128 tokens
- You can also append `-low`, `-medium`, or `-high` to any Gemini model name to request the corresponding reasoning effort (no extra thinking-budget suffix needed).

</details>

---

## 🤖 Model Support

> For details, please refer to [API Documentation - Gateway Interface](https://docs.newapi.pro/en/docs/api)

| Model Type | Description | Documentation |
|---------|------|------|
| 🤖 OpenAI-Compatible | OpenAI compatible models | [Documentation](https://docs.newapi.pro/en/docs/api/ai-model/chat/openai/createchatcompletion) |
| 🤖 OpenAI Responses | OpenAI Responses format | [Documentation](https://docs.newapi.pro/en/docs/api/ai-model/chat/openai/createresponse) |
| 🎨 Midjourney-Proxy | [Midjourney-Proxy(Plus)](https://github.com/novicezk/midjourney-proxy) | [Documentation](https://doc.newapi.pro/api/midjourney-proxy-image) |
| 🎵 Suno-API | [Suno API](https://github.com/Suno-API/Suno-API) | [Documentation](https://doc.newapi.pro/api/suno-music) |
| 🔄 Rerank | Cohere, Jina | [Documentation](https://docs.newapi.pro/en/docs/api/ai-model/rerank/creatererank) |
| 💬 Claude | Messages format | [Documentation](https://docs.newapi.pro/en/docs/api/ai-model/chat/createmessage) |
| 🌐 Gemini | Google Gemini format | [Documentation](https://docs.newapi.pro/en/docs/api/ai-model/chat/gemini/geminirelayv1beta) |
| 🔧 Dify | ChatFlow mode | - |
| 🎯 Custom upstream | Supports configuring legally authorized upstream endpoints | - |

### 📡 Supported Interfaces

<details>
<summary>View complete interface list</summary>

- [Chat Interface (Chat Completions)](https://docs.newapi.pro/en/docs/api/ai-model/chat/openai/createchatcompletion)
- [Response Interface (Responses)](https://docs.newapi.pro/en/docs/api/ai-model/chat/openai/createresponse)
- [Image Interface (Image)](https://docs.newapi.pro/en/docs/api/ai-model/images/openai/post-v1-images-generations)
- [Audio Interface (Audio)](https://docs.newapi.pro/en/docs/api/ai-model/audio/openai/create-transcription)
- [Video Interface (Video)](https://docs.newapi.pro/en/docs/api/ai-model/audio/openai/createspeech)
- [Embedding Interface (Embeddings)](https://docs.newapi.pro/en/docs/api/ai-model/embeddings/createembedding)
- [Rerank Interface (Rerank)](https://docs.newapi.pro/en/docs/api/ai-model/rerank/creatererank)
- [Realtime Conversation (Realtime)](https://docs.newapi.pro/en/docs/api/ai-model/realtime/createrealtimesession)
- [Claude Chat](https://docs.newapi.pro/en/docs/api/ai-model/chat/createmessage)
- [Google Gemini Chat](https://docs.newapi.pro/en/docs/api/ai-model/chat/gemini/geminirelayv1beta)

</details>

---

## 🚢 Deployment

> [!TIP]
> **Latest Docker image:** `calciumion/new-api:latest`

### 📋 Deployment Requirements

| Component | Requirement |
|------|------|
| **Local database** | SQLite (Docker must mount `/data` directory)|
| **Remote database** | MySQL ≥ 5.7.8 or PostgreSQL ≥ 9.6 |
| **Container engine** | Docker / Docker Compose |

### ⚙️ Environment Variable Configuration

<details>
<summary>Common environment variable configuration</summary>

| Variable Name | Description | Default Value |
|--------|------|--------|
| `SESSION_SECRET` | Session secret (required for multi-machine deployment) | - |
| `CRYPTO_SECRET` | Encryption secret (required for Redis) | - |
| `SQL_DSN` | Database connection string | - |
| `REDIS_CONN_STRING` | Redis connection string | - |
| `RELAY_IDLE_CONN_TIMEOUT` | Idle keep-alive timeout for relay HTTP clients, seconds. Defaults to Go standard library behavior; set `0` to disable | `90` |
| `STREAMING_TIMEOUT` | Streaming timeout (seconds) | `300` |
| `STREAM_SCANNER_MAX_BUFFER_MB` | Max per-line buffer (MB) for the stream scanner; increase when upstream sends huge image/base64 payloads | `64` |
| `MAX_REQUEST_BODY_MB` | Max request body size (MB, counted **after decompression**; prevents huge requests/zip bombs from exhausting memory). Exceeding it returns `413` | `32` |
| `AZURE_DEFAULT_API_VERSION` | Azure API version | `2025-04-01-preview` |
| `ERROR_LOG_ENABLED` | Error log switch | `false` |
| `PYROSCOPE_URL` | Pyroscope server address | - |
| `PYROSCOPE_APP_NAME` | Pyroscope application name | `new-api` |
| `PYROSCOPE_BASIC_AUTH_USER` | Pyroscope basic auth user | - |
| `PYROSCOPE_BASIC_AUTH_PASSWORD` | Pyroscope basic auth password | - |
| `PYROSCOPE_MUTEX_RATE` | Pyroscope mutex sampling rate | `5` |
| `PYROSCOPE_BLOCK_RATE` | Pyroscope block sampling rate | `5` |
| `HOSTNAME` | Hostname tag for Pyroscope | `new-api` |

📖 **Complete configuration:** [Environment Variables Documentation](https://docs.newapi.pro/en/docs/installation/config-maintenance/environment-variables)

</details>

### 🔧 Deployment Methods

<details>
<summary><strong>Method 1: Docker Compose (Recommended)</strong></summary>

```bash
# Clone the project
git clone https://github.com/QuantumNous/new-api.git
cd new-api

# Edit configuration
nano docker-compose.yml

# Start service
docker-compose up -d
```

</details>

<details>
<summary><strong>Method 2: Docker Commands</strong></summary>

**Using SQLite:**
```bash
docker run --name new-api -d --restart always \
  -p 3000:3000 \
  -e TZ=Asia/Shanghai \
  -v ./data:/data \
  calciumion/new-api:latest
```

**Using MySQL:**
```bash
docker run --name new-api -d --restart always \
  -p 3000:3000 \
  -e SQL_DSN="root:123456@tcp(localhost:3306)/oneapi" \
  -e TZ=Asia/Shanghai \
  -v ./data:/data \
  calciumion/new-api:latest
```

> **💡 Path explanation:**
> - `./data:/data` - Relative path, data saved in the data folder of the current directory
> - You can also use absolute path, e.g.: `/your/custom/path:/data`

</details>

<details>
<summary><strong>Method 3: BaoTa Panel</strong></summary>

1. Install BaoTa Panel (≥ 9.2.0 version)
2. Search for **New-API** in the application store
3. One-click installation

📖 [Tutorial with images](./docs/BT.md)

</details>

### ⚠️ Multi-machine Deployment Considerations

> [!WARNING]
> - **Must set** `SESSION_SECRET` - Otherwise login status inconsistent
> - **Shared Redis must set** `CRYPTO_SECRET` - Otherwise data cannot be decrypted

### 🔄 Channel Retry and Cache

**Retry configuration:** `Settings → Operation Settings → General Settings → Failure Retry Count`

**Cache configuration:**
- `REDIS_CONN_STRING`: Redis cache (recommended)
- `MEMORY_CACHE_ENABLED`: Memory cache

---

## 🔗 Related Projects

### Upstream Projects

| Project | Description |
|------|------|
| [One API](https://github.com/songquanpeng/one-api) | Original project base |
| [Midjourney-Proxy](https://github.com/novicezk/midjourney-proxy) | Midjourney interface support |

### Supporting Tools

| Project | Description |
|------|------|
| [new-api-key-tool](https://github.com/Calcium-Ion/new-api-key-tool) | Key quota query tool |
| [new-api-horizon](https://github.com/Calcium-Ion/new-api-horizon) | New API high-performance optimized version |

---

## 💬 Help Support

### 📖 Documentation Resources

| Resource | Link |
|------|------|
| 📘 FAQ | [FAQ](https://docs.newapi.pro/en/docs/support/faq) |
| 💬 Community Interaction | [Communication Channels](https://docs.newapi.pro/en/docs/support/community-interaction) |
| 🐛 Issue Feedback | [Issue Feedback](https://docs.newapi.pro/en/docs/support/feedback-issues) |
| 📚 Complete Documentation | [Official Documentation](https://docs.newapi.pro/en/docs) |

### 🤝 Contribution Guide

Welcome all forms of contribution!

- 🐛 Report Bugs
- 💡 Propose New Features
- 📝 Improve Documentation
- 🔧 Submit Code

---

## 📜 License

This project is licensed under the [GNU Affero General Public License v3.0 (AGPLv3)](./LICENSE).

Additional terms under AGPLv3 Section 7 apply. Modified versions must preserve
the author attribution notice `Frontend design and development by New API
contributors.` in the appropriate legal notices and in any prominent about,
legal, footer, or attribution location presented by the user interface.

Modified versions that present a user interface must also preserve a visible
link to the original project: <https://github.com/QuantumNous/new-api>.

This is an open-source project developed based on [One API](https://github.com/songquanpeng/one-api) (MIT License).

If your organization's policies do not permit the use of AGPLv3-licensed software, or if you wish to avoid the open-source obligations of AGPLv3, please contact us at: [support@quantumnous.com](mailto:support@quantumnous.com)

---

## 🌟 Star History

<div align="center">

[![Star History Chart](https://api.star-history.com/svg?repos=Calcium-Ion/new-api&type=Date)](https://star-history.com/#Calcium-Ion/new-api&Date)

</div>

---

<div align="center">

### 💖 Thank you for using New API

If this project is helpful to you, welcome to give us a ⭐️ Star！

**[Official Documentation](https://docs.newapi.pro/en/docs)** • **[Issue Feedback](https://github.com/Calcium-Ion/new-api/issues)** • **[Latest Release](https://github.com/Calcium-Ion/new-api/releases)**

<sub>Built with ❤️ by QuantumNous</sub>

</div>
