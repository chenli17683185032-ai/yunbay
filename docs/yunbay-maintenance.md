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
