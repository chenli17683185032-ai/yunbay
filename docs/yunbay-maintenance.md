# 云贝 new-api 本地维护说明

本文记录云贝 `new-api` 当前的本地维护、验证、同步生产和排障约定。

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
https://sub2api.<server-ip>.sslip.io
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

sub2api.yunbay.xyz, sub2api.<server-ip>.sslip.io {
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
  -e "ssh -i '<private-key-path>' -o IdentitiesOnly=yes -o UserKnownHostsFile='<known-hosts-path>' -o StrictHostKeyChecking=yes" \
  /Users/ethan/Desktop/云贝/云贝网站/new-api/ \
  deploy@<server-host>:/opt/new-api/app/
```

## 生产构建与启动

```bash
ssh -i '<private-key-path>' \
  -o IdentitiesOnly=yes \
  -o UserKnownHostsFile='<known-hosts-path>' \
  -o StrictHostKeyChecking=yes \
  deploy@<server-host> '
set -e
cd /opt/new-api/app
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml build new-api
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml up -d new-api
'
```

若 `Caddyfile` 有变化，再额外执行：

```bash
ssh -i '<private-key-path>' \
  -o IdentitiesOnly=yes \
  -o UserKnownHostsFile='<known-hosts-path>' \
  -o StrictHostKeyChecking=yes \
  deploy@<server-host> '
set -e
cd /opt/new-api/app
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml up -d --force-recreate caddy
'
```

## 生产冒烟检查

```bash
ssh -i '<private-key-path>' \
  -o IdentitiesOnly=yes \
  -o UserKnownHostsFile='<known-hosts-path>' \
  -o StrictHostKeyChecking=yes \
  deploy@<server-host> '
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
- 主页宣传标语已去掉“不封号”。
- macOS 下载入口指向云贝 Codex 构建产物；由于当前没有 Apple Developer ID / notarization，如 Gatekeeper 提示 App 损坏，引导用户使用页面中的 `xattr` 修复命令。

### 关键源码位置

```text
web/default/src/features/quick-start/index.tsx
web/default/src/features/quick-start/quick-start-api-key.ts
web/default/src/features/quick-start/quick-start-api-key.test.ts
web/default/src/features/quick-start/quick-start-data.ts
web/default/src/features/quick-start/quick-start-redemption.ts
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
  src/components/layout/config/public-landing-brand.test.ts \
  src/i18n/public-landing-locales.test.ts
```

2026-06-24 验证结果：

```text
25 tests
25 pass
0 fail
```

类型检查和构建：

```bash
npm run typecheck
npm run build
```

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

## 2026-06-25 使用日志兼容入口维护记录

本节记录 default 主题下使用日志入口的兼容规则。此处只记录公开代码行为，不记录后台账号、cookie、session 或服务器密钥。

### 根因与预期

历史 classic 页面、支付回跳和部分后端返回路径仍可能生成 `/console/log` 或 `/console/log?...` 链接。default 主题会将该入口切到新版使用日志页：

```text
/console/log?... -> /usage-logs/common?...
```

维护要求：

- 旧参数必须保留并转换为新版 usage logs search 参数，不能只跳到空的 `/usage-logs/common`。
- `start_timestamp` / `end_timestamp` 是秒级时间戳，新版 URL 的 `startTime` / `endTime` 是毫秒级时间戳。
- 旧参数 `model_name`、`token_name`、`request_id`、`upstream_request_id` 分别对应新版 `model`、`token`、`requestId`、`upstreamRequestId`。
- 后端已有登录日志 `type=7`，default 前端的 common logs search schema 和筛选器都必须允许 `type=7`，否则旧链接里的登录日志筛选会被丢弃。
- `/usage-logs` 索引路由也要保留参数再跳到 `/usage-logs/common`，因为后端 `ThemeAwarePath` 可能先把 `/console/log?...` 重写为 `/usage-logs?...`。

### 关键源码位置

```text
web/default/src/routes/console/log.tsx
web/default/src/routes/_authenticated/usage-logs/index.tsx
web/default/src/routes/_authenticated/usage-logs/$section.tsx
web/default/src/features/usage-logs/lib/legacy-console-log-route.ts
web/default/src/features/usage-logs/lib/legacy-console-log-route.test.ts
web/default/src/features/usage-logs/constants.ts
web/default/src/features/usage-logs/components/common-logs-filter-bar.tsx
common/constants.go
```

### 本地验证命令

默认前端目录：

```bash
cd /Users/ethan/Desktop/云贝/云贝网站/new-api/web/default
```

兼容参数转换测试：

```bash
node --test src/features/usage-logs/lib/legacy-console-log-route.test.ts
```

类型检查和构建：

```bash
npm run typecheck
npm run build
```

### 回归检查示例

登录 default 主题后打开旧链接：

```text
/console/log?username=root&type=7&channel=123&model_name=gpt-test&token_name=tok&group=grp&request_id=req-1&start_timestamp=1782316800&end_timestamp=1782324000
```

预期最终进入新版 common 使用日志页，并保留筛选条件：

```text
/usage-logs/common?username=root&type=7&channel=123&model=gpt-test&token=tok&group=grp&requestId=req-1&startTime=1782316800000&endTime=1782324000000
```


### 2026-06-25 生产同步补充：消除使用日志 404

生产同步时发现一个额外问题：生产目录曾混入一版尚未完整落地的 Sub2API usage billing 前端改动，common usage logs 会额外请求：

```text
/api/sub2api/billing/self
/api/sub2api/billing/self/stat
```

当前公开仓库基线并没有这些后端路由；生产容器日志显示这些请求返回 404。为避免用户打开使用日志时继续报错，生产处理采用最小回滚策略：将 `web/default/src/features/usage-logs/`、`web/default/src/routes/_authenticated/usage-logs/` 和 `web/default/src/routes/console/log.tsx` 回同步到当前 GitHub 修复版本，使 common usage logs 继续使用已有稳定接口：

```text
/api/log/self
/api/log/self/stat
/api/log
/api/log/stat
```

生产重建后的公开验证结果：

```text
https://yunbay.xyz/                      HTTP 200
前端入口 JS: /static/js/index.a32ecc1640.js
运行中镜像: sha256:c949075ae8e925739...
yunbay-new-api: healthy
生产前端 JS 包含 /api/log
生产前端 JS 不再包含 /api/sub2api/billing
```

后续维护要求：

- 在后端正式补齐 `/api/sub2api/billing*` 控制器和路由前，不要把 common usage logs 切到该接口。
- 如果将来重新启用 Sub2API usage billing 真源，必须同一 PR 内包含后端路由、控制器、权限校验、前端调用和端到端验证；不能只提交前端接口切换。
- 生产目录 `.git` 当前不可作为可信发布源，少量补丁继续使用精确文件 `rsync`，并在同步前创建 `/opt/new-api/backups/...` 备份。

## 结论

当前维护基线是：

- **不要再回到 Channel Console / Cliproxy / Sub2API 深嵌 adapter 方案**；
- 只维护 `new-api` 原生渠道管理 + `sub2api` 独立上游模式；
- 涉及 `Caddyfile` 改动时，要记得 **force-recreate caddy**，不要只靠宿主机文件覆盖后假设容器会自动看到。


## 2026-06-25 控制台收敛与 Windows Codex 下载维护记录

本节记录 2026-06-25 对云贝 default 前端控制台、快速启动第 5 页和 Yunbay Codex Windows 下载入口的维护结果。此处只记录公开代码与公开运维事实，不记录后台密码、SSH 私钥、cookie、session 或 API key。

### 本轮功能收敛结果

- 顶部导航收敛为：`Home / Console / Model Square`。
- 普通用户侧边栏新增 `Dashboard -> /dashboard/models` 入口。
- 普通用户与管理员都移除了 `chat-presets` 聊天小组件，仅保留 `Playground`。
- `/dashboard/overview` 保留用户侧的 `SummaryCards` 用量概览、公告卡片，以及管理员可见的 `PerformanceHealthPanel` 性能健康卡片；继续隐藏 `ApiInfoPanel`、`UptimePanel` 和 `FAQPanel`。
- 快速启动第 5 页主题改为：
  - `Codex one-click launcher`
  - `Codex one-click setup`
- macOS 与 Windows 下载按钮统一为：`Download one-click launcher`。
- Windows 下载不再跳转 Microsoft Store，改为云贝站内静态文件：
  - `/downloads/yunbay-codex-windows-20260625-030300-f5121184b049.exe`
- Windows 卡片新增与 macOS 相同风格的说明块，介绍 Yunbay Codex 的 API Key 导入、`https://yunbay.xyz/v1` 连接、模型供应商管理、连接测试、余额/用量查询与会话管理能力。
- 快速启动第 5 页保留 `CC Switch` 导入卡片，可把当前站点 API、选中模型和已生成 API Key 通过 `ccswitch://v1/import?...` 一键导入为 Codex provider。

### 2026-06-25 回归修复补充

本次补充修复前一轮收敛时误删的两个前端入口：

- 恢复快速启动第 5 页下载区下方的 `CC Switch` 导入卡片：
  - 使用 `quick-start-cc-switch.ts` 生成 `ccswitch://v1/import?...`；
  - API Key 自动补齐 `sk-` 前缀；
  - endpoint 规范化为站点 `/v1`；
  - 导入按钮在缺少 API Key 或模型时禁用并显示原因。
- 恢复控制台概览页卡片组合：
  - `SummaryCards`：保留用量概览；
  - `AnnouncementsPanel`：保留公告；
  - `PerformanceHealthPanel`：管理员可见，保留性能健康；
  - `ApiInfoPanel`、`UptimePanel`、`FAQPanel` 继续不渲染。
- 新增回归测试：
  - `web/default/src/features/quick-start/quick-start-cc-switch.test.ts`
  - `web/default/src/features/quick-start/quick-start-page-source.test.ts`
  - `web/default/src/features/dashboard/components/overview/overview-dashboard-source.test.ts`

### Windows 安装包来源与校验

本轮 Windows 安装包来源：

```text
repo:      chenli17683185032-ai/yunbay-codex
workflow:  Build Windows
run id:    28121301588
artifact:  yunbay-codex-windows
source:    nsis/Yunbay Codex_0.1.0_x64-setup.exe
sha256:    f5121184b0496cd978eb32f97d1def4a2dc7cbb2cc997189ee428fcd8c9fc5da
```

站内静态文件位置：

```text
web/default/public/downloads/yunbay-codex-windows-20260625-030300-f5121184b049.exe
web/default/dist/downloads/yunbay-codex-windows-20260625-030300-f5121184b049.exe
```

### 本轮关键源码位置

```text
web/default/src/features/quick-start/index.tsx
web/default/src/features/quick-start/quick-start-cc-switch.ts
web/default/src/features/quick-start/quick-start-cc-switch.test.ts
web/default/src/features/quick-start/quick-start-page-source.test.ts
web/default/src/features/quick-start/quick-start-data.ts
web/default/src/features/quick-start/quick-start-data.test.ts
web/default/src/features/quick-start/quick-start-locales.test.ts
web/default/src/features/dashboard/components/overview/overview-dashboard-source.test.ts
web/default/src/hooks/sidebar-data-model.ts
web/default/src/hooks/sidebar-data-model.test.ts
web/default/src/hooks/top-nav-link-policy.ts
web/default/src/hooks/top-nav-link-policy.test.ts
web/default/src/hooks/use-top-nav-links.ts
web/default/src/features/dashboard/components/overview/overview-dashboard.tsx
web/default/src/i18n/locales/{en,zh,fr,ja,ru,vi}.json
```

### 本地验证命令与结果

默认前端目录：

```bash
cd /Users/ethan/Documents/yunbay/web/default
```

测试：

```bash
bun test \
  src/features/quick-start/quick-start-cc-switch.test.ts \
  src/features/quick-start/quick-start-page-source.test.ts \
  src/features/quick-start/quick-start-data.test.ts \
  src/features/quick-start/quick-start-locales.test.ts \
  src/features/dashboard/components/overview/overview-dashboard-source.test.ts \
  src/hooks/sidebar-data-model.test.ts \
  src/hooks/top-nav-link-policy.test.ts
```

结果：

```text
24 pass
0 fail
```

类型检查：

```bash
bun run typecheck
```

构建：

```bash
bun run build
```

构建后 Windows 安装包 hash：

```text
f5121184b0496cd978eb32f97d1def4a2dc7cbb2cc997189ee428fcd8c9fc5da
```

### 生产同步要求

- 生产目录 `.git` 仍不可信，不要在服务器上依赖 `git pull`。
- 本轮改动涉及前端源码、locale 和新增 `public/downloads` 大文件，继续使用**非删除式 rsync** 精确同步到 `/opt/new-api/app`。
- 同步后执行：

```bash
cd /opt/new-api/app
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml build new-api
docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml up -d --force-recreate new-api
```

- 本轮未修改 `Caddyfile`，因此无需 `force-recreate caddy`。


### 2026-06-25 生产同步结果

本轮修复已同步到生产环境。公开记录只保留可复现事实，不记录任何 SSH 私钥、API key、cookie、session 或环境变量值。

- GitHub 分支：`codex/fix-usage-logs-stat-null`
- 回归修复提交：`a3634603 fix: restore quick start import and overview panels`
- 生产同步基线：包含 `a3634603` 的分支 HEAD（同步时为 `fabba3b7`）
- 同步方式：非删除式 `rsync` 到 `/opt/new-api/app/`
- 重建方式：`docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml build new-api`
- 重启方式：`docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml up -d --force-recreate new-api`

生产冒烟结果：

```text
yunbay-caddy:   Up / healthy
yunbay-new-api: Up / healthy
https://yunbay.xyz/                                                         200
https://yunbay.xyz/api/status                                              200
https://yunbay.xyz/quick-start                                             200
https://yunbay.xyz/downloads/yunbay-codex-windows-20260625-030300-f5121184b049.exe 200
```

本次生产验证确认：快速启动页与 Windows 下载入口可访问，后端状态接口可访问，`new-api` 与 `caddy` 容器均为 healthy。

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


### 2026-06-27 生产同步与复核结果

本轮用户标签修复已同步到生产环境。公开文档只记录可复现事实，不记录 SSH 私钥、后台密码、cookie、session、token 或服务器 secret。

- GitHub 分支：`codex/user-tags-model-groups`
- 功能修复提交：`0f0ac266 fix: separate admin user tag options`
- 同步方式：非删除式 `rsync --relative --files-from` 同步本次提交涉及的 21 个文件到 `/opt/new-api/app/`
- 重建方式：`docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml build new-api`
- 重启方式：`docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml up -d --force-recreate new-api`

生产复核结果：

```text
yunbay-new-api: running / healthy
http://127.0.0.1:3000/api/status 200
https://yunbay.xyz/              200
https://yunbay.xyz/api/status    200
https://yunbay.xyz/login         200
https://yunbay.xyz/console/user  200
```

后台用户标签接口已用真实后台登录会话验证：

```text
GET /api/user/group-tags
success=true
values=体验用户,vip
labels=体验用户,VIP 用户
```

生产数据复核：

```text
USER_GROUP|1|体验用户|40
USER_GROUP|10|default|1
USER_GROUP|100|default|1
TOKEN_GROUP|gpt-plus|29
TOKEN_GROUP|gpt-pro|2
COMMON_USER_UNEXPECTED_GROUP_COUNT|0
TOKEN_INVALID_MODEL_GROUP_COUNT|0
```

部署后发现 1 个普通用户历史残留 `users.group=gpt-plus`（模型分组误写到用户标签字段）。已按“普通用户无 VIP 条件则归为体验用户”的规则做一次性修正为 `体验用户`，并在服务器 `/opt/new-api/backups/` 下保留 root-only TSV 备份：

```text
user_group_cleanup_20260627_145927.tsv
```

## 2026-06-27 邮件投递切换：Resend SMTP + Cloudflare Routing

### 当前邮件架构

```text
出站系统邮件：yunbay-new-api -> Resend SMTP -> 用户邮箱
入站/回复邮件：support@yunbay.xyz -> Cloudflare Email Routing -> 10256345@qq.com
```

当前生产 SMTP 配置：

```text
SMTPServer=smtp.resend.com
SMTPPort=465
SMTPSSLEnabled=true
SMTPAccount=resend
SMTPFrom=support@yunbay.xyz
SystemName=yunbay
```

`SMTPToken` 为 Resend API Key，只记录为 `SET`，不要写入仓库文档、GitHub issue、PR、聊天记录或公开日志。

### 切换原因

历史上生产使用 `smtp.qq.com` + QQ 个人邮箱作为 SMTP。虽然已修复 587/STARTTLS 配置错误，但 QQ 发信详情仍会暴露 QQ 邮箱昵称，例如 `ninefourteen <...@qq.com>`。为使用正式域名邮箱身份，生产出站邮件已切到 Resend SMTP，发件地址使用 `support@yunbay.xyz`。

Cloudflare Email Routing 已保留为免费入站转发；Cloudflare Email Sending 未使用，因为该能力需要 Workers Paid。

### 已验证

- 线上服务器直连 Resend SMTP 测试通过：`SMTP_TEST_OK from=support@yunbay.xyz to=10256345@qq.com`。
- 云贝应用层密码重置接口测试返回：`http_status=200`、`success=true`。
- `yunbay-new-api` 重启后为 healthy。
- 最近日志未见 SMTP / TLS / AUTH / Resend / 501 / `failed to send` 相关错误。

### 备份与回滚

切换前备份保存在生产服务器：

```text
/root/yunbay-smtp-backups/smtp-before-resend-20260627-114705.tsv
```

回滚时不要在终端或文档中打印备份内容。恢复 `SMTP*` 与 `SystemName` 相关 options 后，需要重启：

```bash
docker restart yunbay-new-api
```

然后重新走密码重置或邮箱验证码接口验证发信。

详细公开说明见：`docs/email-delivery.md`。

## 2026-06-30 全量上线与必读公告生产完成记录

本节记录 2026-06-30 云贝全量上线与必读公告弹窗的生产完成事实。此处只记录可公开复核的信息，不记录 SSH 私钥、后台密码、cookie、session、access token、worker token、支付密钥、完整卡密、完整二维码或完整 session id。

### 代码基线

全量上线基线分支：

```text
codex/full-rollout-no-overlap-clean
```

关键生产基线提交：

```text
83479b48 fix(router): register user group tags route
13925967 fix: restore quick start codex launcher support
59756734 feat: require explicit announcement read confirmation
434230f3 feat: prepare claude pricing gray rollout presets
a8a85812 feat: add affiliate rewards and withdrawal rollout
44b9d0cd feat: roll out ldxp auto topup without worker flow regression
d8f29dcd feat: add paid redemption rollout baseline
41360edd feat: separate user tags from model groups
27d6bd0c fix: preserve rollout baseline fixes
5eeb1035 docs: add full rollout no-overlap plan
23cefff9 fix: normalize check-in reward amounts
8fb008fb fix: preserve legacy usage log filters
```

### 公开侧生产复核

```text
https://yunbay.xyz/            HTTP 200
https://yunbay.xyz/api/status  HTTP 200
https://yunbay.xyz/api/notice  HTTP 200
生产入口 JS: /static/js/index.599262f2f0.js
```

`/api/status` 当前公告形态：

```text
success=true
announcements_enabled=true
announcements_type=list
announcements_count=6
announcement_keys=content,extra,id,publishDate,type
```

生产入口 JS 已包含必读公告相关标记：

```text
I have read
我已阅读
notification-storage
markNoticeRead
markAnnouncementsRead
```

### 完成范围

本次完成口径：

- 全量上线基线已完成生产同步；
- 必读公告弹窗已进入生产 bundle，保持必须点击“我已阅读”才清除未读的语义；
- 用户标签与模型分组分离、卡密兑换、LDXP 自动充值、推荐返利/提现、Claude 灰测预设、Quick Start 恢复、usage logs 稳定修复、签到金额归一化等功能线按全量上线基线收敛；
- `83479b48` 已补上用户标签路由注册；
- Jeepay / Alipay 充值后台配置与钱包流程、Sub2API usage billing 真源仍不属于本轮上线范围。

### 后续要求

- 后续维护必须继续使用非删除式同步策略，避免旧本地文件覆盖生产已上线 wallet / LDXP / Quick Start /公告能力；
- 涉及公告弹窗时必须保留显式已读语义，除非有新的产品需求；
- 涉及 `infra/sub2api/**` 的改动不要混入全量上线回补提交；
- 后续若有生产更正或回滚，只能追加记录，不要改写本节历史。
