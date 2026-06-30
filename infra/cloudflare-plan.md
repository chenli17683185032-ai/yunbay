# Cloudflare DNS 前置设置计划

已在 Cloudflare 中看到的域名：

- `tokenopen.top`
- `yunbay.xyz`

用户口头输入的是 `yunbau.xyz`，与 Cloudflare 里的 `yunbay.xyz` 不一致。执行 DNS 变更前必须确认目标域名。

如果确认目标是 `yunbay.xyz`，建议新增：

| Type | Name | Content | Proxy | TTL |
| --- | --- | --- | --- | --- |
| A | `@` | `13.140.180.223` | Proxied | Auto |
| CNAME | `www` | `yunbay.xyz` | Proxied | Auto |

说明：

- 当前 `yunbay.xyz` 只有邮件路由相关 `MX/TXT` 记录，没有站点 `A/CNAME` 记录。
- `@` 记录用于根域访问 `https://yunbay.xyz`。
- `www` 记录用于 `https://www.yunbay.xyz`，后续可在反代里统一跳转到根域或同样代理 New API。

## 2026-06-27 邮件路由与发信现状

当前 `yunbay.xyz` 的邮件职责拆分如下：

```text
Cloudflare Email Routing：入站收信 / 回复转发
Resend SMTP：出站系统邮件
```

Cloudflare 路由规则当前应至少包含：

```text
support@yunbay.xyz -> 10256345@qq.com
```

说明：

- Cloudflare Email Routing 只负责收信转发，不负责云贝网站出站验证码 / 密码重置邮件。
- Cloudflare Email Sending 当前未使用，因为发送到任意收件人需要 Workers Paid。
- 云贝生产出站 SMTP 已切换到 Resend：`smtp.resend.com:465`，发件地址 `support@yunbay.xyz`。
- 不要把 Resend API Key、Cloudflare Token 或 SMTP Token 写入本文件。
