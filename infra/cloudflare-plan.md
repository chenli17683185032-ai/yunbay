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
