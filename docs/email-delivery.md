# 云贝邮件投递与邮箱路由

本文记录云贝生产环境邮件投递链路。本文可提交到仓库；不要在本文中写入 Resend API Key、SMTP 密码、Cloudflare Token 或任何完整 secret。

## 当前结论（2026-06-27）

云贝邮件分为两条链路：

```text
出站系统邮件：yunbay-new-api -> Resend SMTP -> 用户邮箱
入站/回复邮件：support@yunbay.xyz -> Cloudflare Email Routing -> 10256345@qq.com
```

当前不再使用个人 QQ 邮箱作为出站 SMTP 发件身份，避免收件端显示 QQ 昵称（例如历史昵称 `ninefourteen`）。

## 出站 SMTP

生产环境当前使用 Resend SMTP：

```text
SMTPServer=smtp.resend.com
SMTPPort=465
SMTPSSLEnabled=true
SMTPAccount=resend
SMTPFrom=support@yunbay.xyz
SystemName=yunbay
```

说明：

- `SMTPToken` 是 Resend API Key，只允许保存在生产数据库 / 受控 secret 存储 / 密码管理器中；不要写入仓库文档。
- `SMTPFrom` 使用 `support@yunbay.xyz`，用户收到邮件时应显示类似 `yunbay <support@yunbay.xyz>`。
- Resend API Key 在 SMTP 中作为密码使用，用户名固定为 `resend`。
- 当前项目已有 SMTP 发信能力，因此无需为 Resend 单独改后端代码。

## 入站/回复路由

Cloudflare Email Routing 保留用于收信和用户回复转发：

```text
support@yunbay.xyz -> 10256345@qq.com
```

如果保留 `admin@yunbay.xyz -> 10256345@qq.com`，它只作为管理收信别名，不是系统邮件发件配置的必要项。

## Cloudflare 与 Resend 的职责边界

- Cloudflare Email Routing：负责 `support@yunbay.xyz` 收信并转发到 QQ；免费可用。
- Resend SMTP：负责云贝系统出站邮件（验证码、密码重置等）。
- Cloudflare Email Sending：当前未使用；该能力需要 Workers Paid，不作为当前免费邮件方案。

## 生产验证记录（2026-06-27）

已完成两层验证：

1. 线上服务器直连 Resend SMTP 发送测试邮件：

```text
SMTP_TEST_OK from=support@yunbay.xyz to=10256345@qq.com
```

2. 通过云贝应用自己的密码重置接口触发发信：

```text
GET /api/reset_password
http_status=200
success=true
```

验证后 `yunbay-new-api` 容器为 healthy，最近应用日志没有出现 SMTP、TLS、AUTH、Resend、501 或 `failed to send` 相关错误。

## 生产备份与回滚

切换到 Resend 前，已在服务器保存 SMTP options 备份：

```text
/root/yunbay-smtp-backups/smtp-before-resend-20260627-114705.tsv
```

如需回滚：

1. 先确认 Resend 故障是否为服务商、域名验证、额度或 API Key 问题；
2. 必要时从上述备份恢复 `SMTP*` 与 `SystemName` 相关 options；
3. 恢复后重启 `yunbay-new-api`；
4. 再通过密码重置或邮箱验证码接口做应用层发信测试。

不要把备份文件内容贴到聊天、GitHub issue、PR 或公开文档中，因为里面可能包含历史 SMTP token。

## 后续可选优化

当前 `common/email.go` 直接拼接邮件头：

```text
From: %s <%s>
```

后续可改为 Go 标准库 `net/mail.Address.String()` 生成 `From`，以便中文或特殊字符显示名也符合 RFC 格式。若未来需要把回复地址与发件地址分离，可再增加 `Reply-To` 配置。
