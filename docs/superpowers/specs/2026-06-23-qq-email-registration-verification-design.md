# QQ 邮箱注册验证设计

日期：2026-06-23
项目：云贝网站 New API
路径：`/Users/ethan/Desktop/云贝/云贝网站/new-api`

## 背景

云贝网站基于 New API 源码运行。当前项目已经内置邮箱验证码能力，包括：

- `GET /api/verification?email=...` 发送邮箱验证码。
- `POST /api/user/register` 在启用邮箱验证时校验 `email` 和 `verification_code`。
- 系统设置中已有 SMTP 配置、邮箱验证开关、邮箱域名白名单。
- 前端注册页已能在 `status.email_verification` 为 true 时显示邮箱和验证码输入框。

本次需求不是改 OIDC SSO，也不是修改 `yunbay` Cloudflare Worker 项目；目标是在 New API 注册流程中强制加入 QQ 邮箱验证码。

## 目标

注册新账号时，用户必须验证 QQ 邮箱。

最终注册流程：

1. 用户打开注册页。
2. 输入用户名、密码、QQ 邮箱。
3. 点击发送验证码。
4. 系统只向 `@qq.com` 邮箱发送验证码。
5. 用户输入验证码。
6. 提交注册。
7. 后端强制校验 QQ 邮箱格式、验证码和邮箱唯一性。
8. 注册成功后将 QQ 邮箱保存到用户 `email` 字段。

## 非目标

本次不做以下内容：

- 不支持 `@foxmail.com`，只允许 `@qq.com`。
- 不引入新的第三方邮件 API。
- 不改登录逻辑。
- 不改密码找回流程，除非现有代码自然复用用户 email。
- 不新增验证码数据库表。
- 不做多实例验证码持久化改造。
- 不修改 Cloudflare 域名/DNS 架构。

## 设计选择

采用方案 B：小改源码，把 QQ 邮箱验证作为云贝注册的强制规则。

具体含义：

- 注册接口不再仅依赖 `EmailVerificationEnabled` 开关判断是否要求邮箱验证码。
- 对普通密码注册，始终要求 `email` 和 `verification_code`。
- `email` 必须是有效邮箱，且域名必须严格等于 `qq.com`。
- 发送验证码接口也必须拒绝非 `qq.com` 邮箱。
- 前端注册页始终显示 QQ 邮箱与验证码输入框，而不是只在 `status.email_verification` 为 true 时显示。

## 后端改动

### QQ 邮箱校验 helper

新增或复用小型 helper，建议放在 `controller/user.go` 或更通用的 `common` 包中。

职责：

- trim 空白。
- 转小写比较域名。
- 使用现有 validator 校验邮箱格式，或在调用方已有格式校验时只做域名检查。
- 只接受单一 `@` 分隔后的域名 `qq.com`。

建议语义：

```text
isQQEmail(email) == true only if email is syntactically valid and domain is qq.com
```

### 发送验证码接口

文件：`controller/misc.go`

函数：`SendEmailVerification`

现状：

- 校验 email 格式。
- 解析 local part 和 domain part。
- 如果启用域名白名单，则按白名单检查。
- 检查别名限制。
- 检查邮箱是否已被占用。
- 生成验证码并发送邮件。

改动：

- 在基础格式校验后，强制要求域名为 `qq.com`。
- 非 QQ 邮箱直接返回 `success: false`。
- 错误信息使用中文明确提示：`请使用 QQ 邮箱注册` 或 `请输入有效的 QQ 邮箱`。
- 保留现有邮箱占用检查、SMTP 发送逻辑、Turnstile 和速率限制中间件。

### 注册接口

文件：`controller/user.go`

函数：`Register`

现状：

- 只有 `common.EmailVerificationEnabled` 为 true 时才要求 `email` 和 `verification_code`。
- 只有邮箱验证开启时才把 `user.Email` 写入 `cleanUser.Email`。

改动：

- 密码注册始终要求 `user.Email` 非空。
- 密码注册始终要求 `user.VerificationCode` 非空。
- `user.Email` 必须是 `@qq.com`。
- 验证码必须通过 `common.VerifyCodeWithKey(user.Email, user.VerificationCode, common.EmailVerificationPurpose)`。
- 注册成功时始终保存 `cleanUser.Email = user.Email`。
- 保留现有用户名、密码、邀请码、默认 token、自动登录相关逻辑。

### 邮箱唯一性

现有 `model.CheckUserExistOrDeleted(user.Username, user.Email)` 会同时检查用户名和邮箱。

设计上保持：

- 一个 QQ 邮箱只能注册一个账号。
- 已删除用户占用行为沿用现有 New API 逻辑。

### 验证码生命周期

本次不改变现有验证码存储方式。

现状：

- 验证码保存在内存 map。
- 默认有效期 `common.VerificationValidMinutes = 10` 分钟。
- 服务重启后验证码失效。
- 多实例部署时，如果请求落到不同实例，验证码可能无法验证。

本次接受这个限制，作为后续增强项。

## 前端改动

主要文件：

- `web/default/src/features/auth/sign-up/components/sign-up-form.tsx`
- `web/default/src/features/auth/constants` 或相关 schema 文件
- 如仍启用 classic 前端，也同步修改 `web/classic/src/components/auth/RegisterForm.jsx`

### 默认前端注册页

现状：

- 只有 `status.email_verification` 为 true 时显示邮箱和验证码输入框。

改动：

- 始终显示 QQ 邮箱输入框。
- 始终显示验证码输入框与发送验证码按钮。
- 文案偏向 QQ 邮箱：
  - Label：`QQ 邮箱`
  - Placeholder：`请输入 QQ 邮箱`
  - 验证码 placeholder：`请输入 QQ 邮箱验证码`
- 提交前校验：
  - QQ 邮箱不能为空。
  - 验证码不能为空。
  - 邮箱必须匹配 `@qq.com`。
- 注册 payload 始终传：
  - `email`
  - `verification_code`

### schema 校验

现有注册 schema 需要确保：

- `email` 对注册表单必填。
- `email` 必须是合法邮箱。
- `email` 必须以 `@qq.com` 结尾。

前端校验只是体验优化；后端校验是最终安全边界。

## 系统配置

虽然代码会强制 QQ 邮箱验证，生产环境仍需要配置 SMTP，否则验证码无法发送。

推荐配置：

```text
SMTPServer = smtp.qq.com
SMTPPort = 465
SMTPAccount = 发信 QQ 邮箱
SMTPFrom = 发信 QQ 邮箱
SMTPToken = QQ 邮箱 SMTP 授权码
SMTPSSLEnabled = true
```

建议同时在后台保持以下配置一致：

```text
EmailVerificationEnabled = true
EmailDomainRestrictionEnabled = true
EmailDomainWhitelist = qq.com
```

但本设计不依赖这些开关保证注册强制性。

## 错误处理

建议错误信息：

| 场景 | 返回提示 |
| --- | --- |
| 邮箱为空 | `请输入 QQ 邮箱` |
| 邮箱格式错误 | `请输入有效的 QQ 邮箱` |
| 非 QQ 邮箱 | `请使用 QQ 邮箱注册` |
| 验证码为空 | `请输入 QQ 邮箱验证码` |
| 验证码错误或过期 | 使用现有 i18n：验证码错误或已过期 |
| 邮箱已占用 | 沿用现有：邮箱地址已被占用 |
| SMTP 未配置 | 沿用现有错误或返回邮件服务未配置 |
| SMTP 发送失败 | 沿用现有错误或返回验证码发送失败 |

## 测试计划

### 后端测试

新增或修改 controller 测试，覆盖：

1. `GET /api/verification` 对非 `@qq.com` 邮箱返回失败。
2. `GET /api/verification` 对合法 `@qq.com` 邮箱进入发送逻辑。
3. `POST /api/user/register` 缺少 email 时失败。
4. `POST /api/user/register` 缺少 verification_code 时失败。
5. `POST /api/user/register` 使用非 `@qq.com` 邮箱时失败。
6. `POST /api/user/register` 使用错误验证码时失败。
7. `POST /api/user/register` 使用正确 QQ 邮箱验证码时成功注册并保存 email。
8. 已占用 QQ 邮箱不能重复注册。

### 前端检查

至少手动或自动确认：

1. 注册页始终显示 QQ 邮箱输入框。
2. 注册页始终显示验证码输入框与发送按钮。
3. 非 QQ 邮箱前端阻止提交或显示错误。
4. 点击发送验证码调用 `/api/verification?email=...`。
5. 注册 payload 包含 `email` 和 `verification_code`。

## 风险与后续增强

### 风险

- 当前验证码保存在进程内存，服务重启会失效。
- 多实例部署时验证码可能在不同实例之间不共享。
- QQ 邮箱 SMTP 可能有发送频率、授权码、安全策略限制。
- 如果生产环境未正确配置 SMTP，用户无法完成注册。

### 后续增强

后续可以考虑：

- 将验证码持久化到数据库或 Redis。
- 注册成功后删除已使用验证码。
- 增加更明确的邮件发送失败提示。
- 增加后台开关，例如 `QQEmailRegistrationRequired`。
- 支持 `foxmail.com`，如果运营上需要。
