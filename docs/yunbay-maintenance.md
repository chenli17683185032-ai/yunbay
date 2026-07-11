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
## 2026-07-12 Sub2API 号池与模型可用性邮件监控

生产服务器已安装独立监控脚本：

```text
/opt/new-api/monitor/sub2api-pool-monitor/sub2api_pool_monitor.py
```

运行方式：deploy 用户 crontab 每 5 分钟执行一次。配置文件位于：

```text
/home/deploy/.config/yunbay/sub2api-monitor.env
```

配置文件权限为 `0600`，只保存收件地址、状态文件和锁文件路径。Sub2API 管理凭据从运行容器环境动态读取；SMTP 配置从 new-api PostgreSQL `options` 表动态读取，不在脚本、仓库或日志中复制 SMTP Token。

监控口径：

- 号池可用率只统计 `active && schedulable` 的账号；
- 账号可用状态读取 `/api/v1/admin/ops/account-availability`；
- 模型可用状态复用 `/api/v1/admin/channel-monitors` 的渠道监测结果；
- OAuth / setup-token 账号额度读取主动 usage 接口；
- 号池或模型可用率低于 80%、任一额度窗口使用率达到 80%，或监控接口/数据出现突发异常时发送邮件；
- 异常持续期间每 5 分钟重复发送，全部恢复后发送一次恢复邮件。

部署验证：

- `--dry-run` 成功读取生产实时数据；
- `--test-email` 已通过现有 Resend SMTP 链路发送测试邮件；
- 首次正式检查已发送告警邮件；
- 安装后 `yunbay-new-api`、`yunbay-sub2api`、Caddy、PostgreSQL、Redis 与 CLI Proxy 均保持 healthy，未重启业务容器。

常用命令：

```bash
set -a
. /home/deploy/.config/yunbay/sub2api-monitor.env
set +a

# 手动检查并按规则发信
/opt/new-api/monitor/sub2api-pool-monitor/sub2api_pool_monitor.py

# 只查看报告，不发信、不改状态
/opt/new-api/monitor/sub2api-pool-monitor/sub2api_pool_monitor.py --dry-run

# 查看定时日志
tail -n 200 /opt/new-api/monitor/sub2api-pool-monitor/monitor.log

# 查看定时配置
crontab -l
```

回滚时只删除 crontab 中 `BEGIN/END YUNBAY SUB2API POOL MONITOR` 标记区块，并删除 `/opt/new-api/monitor/sub2api-pool-monitor/`；不需要重启任何业务服务。
