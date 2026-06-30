# LDXP 卡密兑换与充值统计维护说明

本文记录云贝对接链动发卡网（LDXP）卡密兑换后的业务语义、后台操作约定和上线验证方式。文档只记录可公开维护信息，不包含服务器登录凭据、后台会话、API key、cookie、session 或生产 secret。

## 业务目标

LDXP 发卡网售出的卡密在云贝侧兑换后，需要同时满足：

- 用户获得对应额度；
- 真实付费卡密进入充值记录与充值统计；
- 活动赠送码只增加额度，不污染真实充值统计；
- 管理员可以按批次创建、复制和导出卡密。

## 兑换码类型

`redemptions.kind` 决定兑换码的会计语义：

| kind | 用途 | 兑换后加额度 | 创建 `top_ups` 成功记录 | 计入充值统计 |
| --- | --- | --- | --- | --- |
| `paid_topup` | LDXP / 付费充值卡 | 是 | 是 | 是 |
| `promo_credit` | 赠送额度 / 活动码 | 是 | 否 | 否 |
| `legacy` | 历史兑换码兼容 | 是 | 否 | 否 |

付费充值卡必须满足：

- `quota > 0`
- `amount > 0`
- `money > 0`
- `count_as_topup = true`

赠送码必须满足：

- `quota > 0`
- `amount >= 0`
- `money >= 0`
- `count_as_topup = false`

## 新增字段

`redemptions` 表新增字段：

| 字段 | 说明 |
| --- | --- |
| `kind` | 兑换码类型，例如 `paid_topup`、`promo_credit`、`legacy` |
| `amount` | 充值面额，整数单位沿用系统充值记录语义 |
| `money` | 实付金额 |
| `count_as_top_up` | 是否创建成功充值记录并计入充值统计 |
| `batch_id` | 批次号，用于管理、搜索和导出 |
| `source` | 来源，例如 `ldxp`、`promo`、`manual` |
| `exported_time` | 批次导出时间戳；未导出为 `0` |

> Go JSON 字段使用 `count_as_topup`，GORM 默认数据库列名为 `count_as_top_up`。

## 兑换流程

用户调用兑换接口后，后端在同一个数据库事务中处理：

1. 锁定兑换码并校验状态、过期时间和类型；
2. 给用户增加 `quota`；
3. 标记兑换码已使用；
4. 如果是 `paid_topup` 且 `count_as_top_up=true`，创建一条成功 `TopUp` 记录；
5. 返回原有数字额度结果，并额外带上可选 `redemption` 元信息。

`POST /api/user/topup` 的兼容性约定：

- `data` 保持为数字，兼容 classic 前端和旧调用方；
- 新版 default 前端可以读取可选的 `redemption` 字段展示更准确的成功提示。

## 管理后台操作

default 后台兑换码页支持：

- 选择卡密类型；
- 设置来源、批次、面额、实付金额；
- 创建后复制本次生成的卡密；
- 按批次导出 TXT 或 CSV；
- 导出后写入 `exported_time`。

建议约定：

- LDXP 发卡网售卖卡密使用 `paid_topup` + `source=ldxp`；
- 运营赠送额度使用 `promo_credit` + `source=promo`；
- 不要把赠送码设置成 `count_as_topup=true`。

## 本地验证

后端建议至少运行：

```bash
go test ./model ./controller ./router ./common ./setting/... -count=1
```

default 前端建议至少运行：

```bash
cd web/default
bun test \
  src/features/redemption-codes/lib/export-utils.test.ts \
  src/features/redemption-codes/lib/redemption-form.test.ts \
  src/features/wallet/lib/redemption-result.test.ts
bun run typecheck
bun run build
```

## 生产验证清单

生产同步后至少确认：

```text
yunbay-new-api: running / healthy
yunbay-caddy:   running / healthy
yunbay-postgres running / healthy
yunbay-redis    running / healthy
https://yunbay.xyz/                          200
https://yunbay.xyz/api/status                200
https://yunbay.xyz/console/redemption-codes  200
https://yunbay.xyz/wallet                    200
```

数据库字段可用只读 SQL 验证：

```sql
select column_name
from information_schema.columns
where table_name = 'redemptions'
  and column_name in (
    'kind',
    'amount',
    'money',
    'count_as_top_up',
    'batch_id',
    'source',
    'exported_time'
  )
order by column_name;
```

未登录访问批次导出接口返回 `401` 属于预期；登录管理员后，应由业务逻辑处理批次是否存在，而不是路由 `404`。

## 回滚注意

代码回滚时可以回滚镜像和源码；新增数据库字段通常无需删除，旧代码会忽略多余字段。

不要做破坏性 schema rollback，除非已经确认没有生产数据依赖这些列。
