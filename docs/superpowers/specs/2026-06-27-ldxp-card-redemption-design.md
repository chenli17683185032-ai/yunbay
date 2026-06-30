# 链动发卡网卡密兑换与充值统计设计 Spec

日期：2026-06-27

## 1. 背景

云贝当前希望把外部发卡网用于余额充值：用户在云贝商品卡或钱包页点击购买后，进入链动发卡网支付并获得卡密，再回到云贝填写兑换码刷新余额。经过确认，链动平台没有开放商户 API，所有交易、发卡和订单管理只能通过链动平台自身页面完成。因此本设计不再追求“链动支付后自动回调云贝”，而是把链动发卡网定位为外部销售渠道：

```text
云贝生成可信卡密 -> 管理员导入链动发卡网 -> 用户付款购买卡密 -> 用户回云贝兑换 -> 云贝按卡密类型处理余额和统计
```

这样云贝不依赖链动接口、回调、页面轮询或非公开前端接口。链动只负责收款和发卡；云贝仍然是卡密真实性、额度发放、充值统计和审计记录的唯一可信系统。

## 2. 已确认事实

1. 链动平台没有开放 API，不能可靠调用创建订单、查询订单、取卡密或支付回调。
2. 链动平台是发卡网，可以由管理员把云贝预先生成的卡密导入商品库存。
3. 云贝已有兑换码模型和兑换入口：
   - 后端模型：`/Users/ethan/Documents/yunbay/model/redemption.go`
   - 用户兑换接口：`POST /api/user/topup`
   - 管理员兑换码接口：`/api/redemption/*`
   - 默认前端钱包兑换入口：`/Users/ethan/Documents/yunbay/web/default/src/features/wallet/*`
   - classic 前端也使用同一个 `POST /api/user/topup`，且期望响应 `data` 仍是数字额度。
4. 云贝已有充值订单模型 `TopUp`，可用于记录真正应计入充值金额的成功充值记录：`/Users/ethan/Documents/yunbay/model/topup.go`。
5. 当前普通兑换码只会增加用户额度，不会区分“付费充值卡密”和“赠送/优惠码”，也不会在兑换时创建 `TopUp` 充值记录。

## 3. 目标

### 3.1 业务目标

1. 管理员可以在云贝批量创建“充值卡密”和“优惠/赠送码”。
2. 管理员可以把生成的卡密导出，并手动导入链动发卡网商品库存。
3. 用户在链动发卡网购买卡密后，回到云贝钱包页输入兑换码。
4. 云贝根据数据库中记录的卡密类型处理兑换：
   - 充值卡密：增加额度，并计入充值金额。
   - 优惠/赠送码：可以增加额度，但不计入充值金额。
5. 充值卡密兑换成功后创建成功充值记录，便于钱包订单历史、管理员充值历史、付费统计和后续对账。
6. 优惠/赠送码不得触发充值金额统计、邀请返利或任何按付费充值计算的权益。
7. 所有判断以云贝数据库为准，不以卡密前缀、用户提交金额或链动页面信息为准。
8. 保留现有兑换码兼容性，历史兑换码默认按旧逻辑处理，不 retroactively 计入充值。

### 3.2 用户体验目标

1. 钱包页继续保留“兑换码/卡密”输入框。
2. `TopUpLink` 可以继续指向链动发卡网商品页，作为“购买卡密”入口。
3. 用户兑换充值卡密后看到明确提示，例如“充值卡兑换成功，已增加对应额度”。
4. 用户兑换优惠/赠送码后看到与充值卡不同的提示，例如“优惠码兑换成功，已增加赠送额度”。
5. 兑换成功后刷新当前用户信息和余额。
6. classic 前端不因后端响应结构变更而损坏。

## 4. 非目标

本轮明确不做：

1. 不接入链动 API，因为平台没有开放 API。
2. 不抓取、模拟或依赖链动买家前端接口、订单页、支付轮询或页面结构。
3. 不实现支付成功自动回调云贝。
4. 不让用户或前端提交金额、支付状态、商品类型来决定是否入账。
5. 不修改链动平台商品、订单、支付方式或库存管理逻辑。
6. 不实现完整优惠券系统，例如下次充值折扣、订阅抵扣、限定商品可用券等；本轮只预留 `coupon` 类型，不做消费场景。
7. 不删除或替换项目受保护标识、包路径、版权信息、README、LICENSE 或相关 attribution。
8. 不改变已有在线支付网关 Stripe、Creem、Waffo、Waffo Pancake、Epay 的流程。

## 5. 术语

### 5.1 云贝兑换码

云贝后端生成并保存到 `redemptions` 表中的可信凭证。用户输入后，云贝通过数据库验证是否存在、是否启用、是否已使用、是否过期。

### 5.2 链动卡密

导入链动发卡网商品库存的字符串。为了安全和可审计，链动卡密必须来源于云贝兑换码，不允许由链动平台随机生成后再让云贝按前缀猜测类型。

### 5.3 充值卡密

用户通过链动发卡网付款购买的充值凭证。兑换后：

```text
增加用户额度
创建成功 TopUp 记录
计入充值金额
可进入付费用户统计
按业务规则决定是否触发邀请返利
```

### 5.4 优惠/赠送码

活动、补偿或人工赠送的兑换码。兑换后可以增加额度，但：

```text
不创建付费充值记录
不计入充值金额
不触发邀请返利
不进入按充值金额计算的权益
```

### 5.5 批次

一次批量生成的卡密集合。批次用于导出、导入链动、审计和避免重复导出。一个批次只允许一种类型、一个面值、一个来源和一套统计规则。

## 6. 卡密类型设计

新增兑换码业务类型字段 `kind`，使用字符串枚举。

| kind | 用途 | 是否增加额度 | 是否计入充值金额 | 是否创建 TopUp | 前端可新建 |
| --- | --- | --- | --- | --- | --- |
| `legacy` | 历史兑换码兼容类型 | 是 | 否 | 否 | 否 |
| `paid_topup` | 付费充值卡密 | 是 | 是 | 是 | 是 |
| `promo_credit` | 优惠/赠送额度码 | 是 | 否 | 否 | 是 |
| `coupon` | 未来优惠券权益码预留 | 本轮不实现 | 否 | 否 | 否 |

规则：

1. 现有兑换码迁移默认 `kind = legacy`。
2. 新增管理后台创建时默认选择 `promo_credit`，管理员必须显式选择 `paid_topup` 才计入充值金额。
3. `paid_topup` 必须配置 `amount > 0`、`money > 0`、`count_as_topup = true`。
4. `promo_credit` 必须配置 `count_as_topup = false`，`amount` 和 `money` 默认为 `0`。
5. `coupon` 仅作为未来扩展值，本轮不允许创建和兑换为权益。

## 7. 数据模型设计

### 7.1 扩展 `Redemption`

在 `/Users/ethan/Documents/yunbay/model/redemption.go` 的 `Redemption` 模型新增字段：

```go
Kind         string  `json:"kind" gorm:"type:varchar(32);default:'legacy';index"`
Amount       int64   `json:"amount" gorm:"default:0"`
Money        float64 `json:"money" gorm:"default:0"`
CountAsTopUp bool    `json:"count_as_topup" gorm:"default:false"`
BatchId      string  `json:"batch_id" gorm:"type:varchar(64);default:'';index"`
Source       string  `json:"source" gorm:"type:varchar(32);default:'manual';index"`
ExportedTime int64   `json:"exported_time" gorm:"bigint;default:0"`
```

字段语义：

- `Kind`：卡密业务类型。
- `Amount`：充值面值，沿用 `TopUp.Amount` 的语义，用于“充值了多少金额/数量”。
- `Money`：实际支付统计金额，沿用 `TopUp.Money` 的语义；优惠/赠送码为 `0`。
- `CountAsTopUp`：是否计入充值金额和付费统计。
- `BatchId`：同批次导出的标识，例如 `LDXP-20260627-10-001`。
- `Source`：来源渠道，例如 `ldxp`、`manual`、`promo`。
- `ExportedTime`：导出时间，`0` 表示尚未导出或未标记导出。

### 7.2 数据库兼容性

1. 迁移使用 GORM `AutoMigrate` 或现有迁移模式。
2. 字段使用通用类型，不使用 JSONB、数组类型、数据库专属枚举或数据库专属函数。
3. 不引入 raw SQL；如不得不用 raw SQL，必须按项目规范兼容 SQLite、MySQL 和 PostgreSQL。
4. Boolean 字段读写通过 GORM 处理；不得手写数据库专属布尔字面量。
5. 旧数据默认值：

```text
kind = legacy
amount = 0
money = 0
count_as_topup = false
batch_id = ''
source = manual
exported_time = 0
```

### 7.3 是否新增批次表

本轮 MVP 不强制新增 `RedemptionBatch` 表，避免扩大迁移和后台页面范围。批次信息先以 `Redemption.BatchId`、`Name`、`Source`、`ExportedTime` 承载。

如果后续需要批次级库存、导出次数、备注、CSV 下载历史和链动商品映射，可新增独立表：

```go
type RedemptionBatch struct {
    Id           int
    BatchId      string
    Name         string
    Kind         string
    Source       string
    Amount       int64
    Money        float64
    Quota        int
    Count        int
    CreatedTime  int64
    ExportedTime int64
    ExpiredTime  int64
}
```

但该表不属于本轮必需实现。

## 8. 后端业务设计

### 8.1 创建卡密

复用现有 `POST /api/redemption/` 创建入口，扩展请求字段：

```json
{
  "name": "链动10元充值卡",
  "quota": 500000,
  "count": 100,
  "expired_time": 0,
  "kind": "paid_topup",
  "amount": 10,
  "money": 10,
  "count_as_topup": true,
  "source": "ldxp",
  "batch_id": "LDXP-20260627-10-001"
}
```

创建规则：

1. 保持现有合规确认检查。
2. 保持现有 `name`、`quota`、`count`、`expired_time` 校验。
3. `count` 上限初期保持现有 `100`，避免一次生成过多导致 UI、日志或导出压力；如运营需要更大批次，后续单独引入配置项。
4. 如果 `kind` 为空，按 `promo_credit` 处理新建请求；历史数据库行仍是 `legacy`。
5. 如果 `kind = paid_topup`：
   - `amount > 0`
   - `money > 0`
   - `quota > 0`
   - `count_as_topup` 必须为 `true`
   - `source` 建议为 `ldxp`
6. 如果 `kind = promo_credit`：
   - `quota > 0`
   - `count_as_topup` 必须为 `false`
   - `amount`、`money` 可为 `0`
7. `batch_id` 可由管理员填写；为空时服务端自动生成。
8. 生成的卡密继续使用高强度随机值，不使用可预测递增编号。
9. 响应保持兼容：`data` 仍返回 `[]string` 卡密列表；可以额外返回 `batch_id` 顶层字段或在未来新增专用响应结构，但不得破坏现有前端依赖。

### 8.2 卡密导出

MVP 支持两种导出方式：

1. 创建成功后前端直接下载/复制本次 `data` 中的卡密列表。
2. 兑换码列表页支持按 `batch_id` 搜索或筛选，再导出选中卡密。

推荐新增导出接口：

```text
GET /api/redemption/export?batch_id=<batch>&format=txt
GET /api/redemption/export?batch_id=<batch>&format=csv
```

导出格式：

- `txt`：一行一个卡密，适合导入链动发卡网。
- `csv`：包含 `key,name,kind,amount,money,quota,batch_id,source,expired_time`，适合本地审计。

导出规则：

1. 仅管理员可导出。
2. 导出不改变兑换状态。
3. 导出成功后可把同批次 `ExportedTime` 从 `0` 更新为当前时间，用于审计和避免误重复导出。
4. 不在普通用户接口返回批量卡密。

### 8.3 用户兑换

现有 `POST /api/user/topup` 保持入口不变。

内部建议把 `model.Redeem(key, userId)` 从只返回 `quota int` 扩展为返回结构化结果：

```go
type RedeemResult struct {
    Quota        int
    Kind         string
    Amount       int64
    Money        float64
    CountAsTopUp bool
    BatchId      string
    Source       string
    RedemptionId int
}
```

控制器响应必须保持 classic 前端兼容：

```json
{
  "success": true,
  "message": "",
  "data": 500000,
  "redemption": {
    "kind": "paid_topup",
    "amount": 10,
    "money": 10,
    "count_as_topup": true,
    "batch_id": "LDXP-20260627-10-001",
    "source": "ldxp"
  }
}
```

兼容规则：

1. `data` 仍为数字额度，classic 前端继续可用。
2. default 前端可以读取可选 `redemption` 字段展示更准确文案。
3. 如果前端不识别 `redemption` 字段，仍显示旧的兑换成功提示。

### 8.4 充值卡密入账

兑换逻辑必须在同一个数据库事务内完成：

```text
锁定 redemption
校验存在、启用、未过期、未使用
增加用户 quota
标记 redemption 已使用
如果 count_as_topup = true：创建 TopUp 成功记录
写入日志
提交事务
```

充值卡密创建 `TopUp` 记录的建议字段：

```text
UserId = 当前兑换用户
Amount = redemption.Amount
Money = redemption.Money
TradeNo = RDM<redemption.Id>U<userId>
PaymentMethod = redemption_code
PaymentProvider = redemption_code
CreateTime = 当前时间
CompleteTime = 当前时间
Status = success
```

规则：

1. `TradeNo` 必须不包含完整卡密，避免在订单历史中泄露可兑换凭证。
2. `TradeNo` 必须唯一。由于一个兑换码只能兑换一次，使用 `redemption.Id` 足够稳定。
3. `TopUp` 创建和兑换码状态变更必须在同一事务内，避免“额度已加但统计未记”或“统计已记但卡未使用”。
4. 如果重复兑换，必须在锁内检测 `Status != enabled` 并拒绝，不得重复创建 `TopUp`。
5. 如果 `TopUp` 创建失败，整个兑换事务回滚。

### 8.5 优惠/赠送码入账

`kind = legacy` 或 `kind = promo_credit` 的兑换码沿用原有额度增加逻辑，但不创建 `TopUp` 记录：

```text
增加用户 quota
标记兑换码已使用
记录兑换日志
不计入充值金额
不触发邀请返利
```

### 8.6 邀请返利与付费统计

本轮实现必须保证：

1. 只有 `CountAsTopUp = true` 且成功创建 `TopUp` 的兑换才可被视为付费充值。
2. `promo_credit`、`legacy`、未来 `coupon` 不得触发邀请返利。
3. 如果现有邀请返利逻辑只监听 `TopUp` 成功记录，则只为 `paid_topup` 创建 `TopUp` 即可自然隔离。
4. 如果后续存在直接读取 redemption 统计的代码，必须显式过滤 `count_as_topup = true`。

### 8.7 日志与审计

兑换日志需要区分类型：

- `paid_topup`：`通过充值卡密充值 <quota>，兑换码ID <id>，批次 <batch_id>`
- `promo_credit`：`通过优惠/赠送码兑换 <quota>，兑换码ID <id>，批次 <batch_id>`
- `legacy`：保持旧文案或追加 `legacy` 标识。

管理员创建审计日志需要记录：

```text
name
count
quota
kind
amount
money
count_as_topup
source
batch_id
```

日志不得输出完整卡密列表。

## 9. 前端设计

### 9.1 管理员兑换码创建表单

在 default 前端 `/redemption-codes` 创建抽屉中新增字段：

1. 卡密类型：
   - `充值卡密（计入充值）` -> `paid_topup`
   - `优惠/赠送码（不计入充值）` -> `promo_credit`
2. 充值面值：`amount`
3. 统计金额：`money`
4. 来源渠道：`source`
   - 默认 `manual`
   - 可选 `ldxp`
   - 可手动输入或下拉选择
5. 批次号：`batch_id`
   - 可为空，由服务端生成
   - 管理员可填写，例如 `LDXP-20260627-10-001`
6. 是否计入充值：`count_as_topup`
   - `paid_topup` 自动为是，不允许关闭
   - `promo_credit` 自动为否，不允许打开

交互规则：

1. 选择 `paid_topup` 时，`amount` 和 `money` 必填且大于 `0`。
2. 选择 `promo_credit` 时，隐藏或禁用 `amount/money/count_as_topup`，默认 `0/false`。
3. 创建成功后显示一个“导出本批卡密”操作，至少支持复制一行一个卡密。
4. 所有新增文案必须走 `useTranslation()` 和 locale 文件，支持 en、zh、fr、ja、ru、vi。

### 9.2 管理员兑换码列表

列表新增可选列：

```text
类型
面值
统计金额
是否计入充值
来源
批次
导出时间
```

最小实现可以先展示：

```text
类型
面值/统计金额
批次
来源
```

搜索扩展：

1. 现有搜索继续支持按 `id` 和 `name` 搜索。
2. 推荐支持按 `batch_id` 前缀搜索，便于导出和对账。

批量操作：

1. 支持复制选中卡密，一行一个。
2. 推荐支持下载选中卡密 TXT。
3. 不允许普通用户看到未兑换卡密列表。

### 9.3 钱包兑换体验

钱包页继续使用现有兑换输入框。default 前端识别 `redemption` 元信息后显示不同提示：

- `paid_topup`：

```text
充值卡兑换成功！已增加：{{quota}}
```

- `promo_credit`：

```text
优惠码兑换成功！已增加：{{quota}}
```

- `legacy` 或无 `redemption` 字段：

```text
兑换成功！已增加：{{quota}}
```

`TopUpLink` 文案可调整为更贴近发卡网：

```text
需要卡密？去购买
```

但不强制修改设置项含义，仍复用当前 `TopUpLink`。

### 9.4 classic 前端兼容

classic 前端当前读取：

```js
const { success, message, data } = res.data
renderQuota(data)
user.quota + data
```

因此后端必须保持 `data` 为数字额度。classic 前端可以不做任何改动；如果后续要增强提示，再单独适配 `redemption` 字段。

## 10. 链动发卡网运营流程

### 10.1 充值卡密商品

建议链动发卡网按面值拆分商品：

```text
10 元云贝充值卡：只导入 paid_topup amount=10 的卡密
20 元云贝充值卡：只导入 paid_topup amount=20 的卡密
活动赠送码：只导入 promo_credit 的卡密
```

禁止把充值卡密和优惠/赠送码混入同一个链动商品库存。

### 10.2 标准操作步骤

```text
1. 管理员在云贝后台创建兑换码批次
2. 选择 kind=paid_topup 或 promo_credit
3. 设置 quota、amount、money、source=ldxp、batch_id
4. 生成后导出 TXT
5. 管理员登录链动平台
6. 把 TXT 导入对应商品库存
7. 用户在链动商品页付款购买
8. 用户复制卡密回云贝钱包兑换
9. 云贝按 kind 和 count_as_topup 自动处理入账和统计
```

### 10.3 对账方式

因为链动没有 API，云贝无法自动知道“已售出但未兑换”的卡密。对账采用人工/半人工方式：

1. 链动后台导出销售订单。
2. 云贝按 `batch_id` 查询卡密使用情况。
3. 管理员比对：
   - 链动已售出且云贝已兑换：正常。
   - 链动已售出但云贝未兑换：用户尚未回来兑换，通常无需处理。
   - 用户持码兑换失败：按卡密在云贝后台查询状态、批次和使用人。

## 11. 安全设计

1. 卡密必须由云贝生成，不得由链动生成后再录入云贝。
2. 卡密必须高强度随机，不允许递增编号。
3. 业务类型以数据库字段为准，不以卡密前缀为准。
4. 用户提交兑换请求时只提交 `key`，不得提交金额、类型、来源或支付状态。
5. 兑换必须使用事务和行锁，防止并发重复兑换。
6. `TopUp.TradeNo` 不得包含完整卡密。
7. 日志、审计、错误信息不得批量输出完整卡密。
8. 导出接口仅管理员可用。
9. `promo_credit` 和 `legacy` 不得触发邀请返利或付费统计。
10. 删除兑换码时沿用现有管理员权限；已使用码不建议物理删除，避免对账困难。

## 12. 涉及文件

### 12.1 后端必改

- `/Users/ethan/Documents/yunbay/model/redemption.go`
- `/Users/ethan/Documents/yunbay/controller/redemption.go`
- `/Users/ethan/Documents/yunbay/controller/user.go`
- `/Users/ethan/Documents/yunbay/model/topup.go`
- `/Users/ethan/Documents/yunbay/model/main.go`
- `/Users/ethan/Documents/yunbay/i18n/keys.go`
- `/Users/ethan/Documents/yunbay/i18n/locales/zh-CN.yaml`
- `/Users/ethan/Documents/yunbay/i18n/locales/en.yaml`

### 12.2 前端 default 必改

- `/Users/ethan/Documents/yunbay/web/default/src/features/redemption-codes/types.ts`
- `/Users/ethan/Documents/yunbay/web/default/src/features/redemption-codes/lib/redemption-form.ts`
- `/Users/ethan/Documents/yunbay/web/default/src/features/redemption-codes/components/redemptions-mutate-drawer.tsx`
- `/Users/ethan/Documents/yunbay/web/default/src/features/redemption-codes/components/redemptions-columns.tsx`
- `/Users/ethan/Documents/yunbay/web/default/src/features/redemption-codes/components/data-table-bulk-actions.tsx`
- `/Users/ethan/Documents/yunbay/web/default/src/features/wallet/types.ts`
- `/Users/ethan/Documents/yunbay/web/default/src/features/wallet/hooks/use-redemption.ts`
- `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/en.json`
- `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/zh.json`
- `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/fr.json`
- `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/ja.json`
- `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/ru.json`
- `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/vi.json`

### 12.3 classic 前端

classic 前端本轮原则上不改，因为后端保持 `data` 为数字额度。如果后续希望展示不同兑换类型文案，再改：

- `/Users/ethan/Documents/yunbay/web/classic/src/components/topup/index.jsx`

### 12.4 推荐新增测试

- `/Users/ethan/Documents/yunbay/model/redemption_topup_test.go`
- `/Users/ethan/Documents/yunbay/web/default/src/features/redemption-codes/lib/redemption-form.test.ts`
- `/Users/ethan/Documents/yunbay/web/default/src/features/wallet/hooks/use-redemption.test.ts`

## 13. 测试策略

### 13.1 后端单元测试

覆盖：

1. `paid_topup` 兑换成功后增加用户额度。
2. `paid_topup` 兑换成功后创建一条 `TopUp` 成功记录。
3. `paid_topup` 的 `TopUp.Amount/Money/Status/UserId` 正确。
4. `paid_topup` 重复兑换不会重复加额度，不会重复创建 `TopUp`。
5. `promo_credit` 兑换成功后增加用户额度，但不创建 `TopUp`。
6. `legacy` 兑换保持旧行为，不创建 `TopUp`。
7. 过期、禁用、已使用兑换码仍按旧规则失败。
8. `paid_topup` 创建请求缺少 `amount/money` 时失败。
9. `promo_credit` 请求强行设置 `count_as_topup=true` 时失败。
10. `TradeNo` 不包含完整兑换码。

### 13.2 后端集成/兼容测试

1. SQLite 下自动迁移成功。
2. GORM 查询不使用数据库专属语法。
3. `POST /api/user/topup` 响应中 `data` 仍是数字。
4. 新增 `redemption` 元信息存在且字段正确。
5. 管理员创建接口响应仍可被现有前端读取为 `string[]`。

### 13.3 前端测试

1. 创建表单选择 `paid_topup` 时要求 `amount/money > 0`。
2. 创建表单选择 `promo_credit` 时不允许计入充值。
3. 兑换 hook 收到 `paid_topup` 元信息时显示充值卡文案。
4. 兑换 hook 收到 `promo_credit` 元信息时显示优惠码文案。
5. 兑换响应没有 `redemption` 字段时保持旧文案。
6. i18n 静态 key 检查通过。

### 13.4 手工验收

1. 管理员创建 `paid_topup` 批次并复制导出卡密。
2. 用户兑换其中一个卡密，余额增加。
3. 用户充值历史出现一条成功充值记录。
4. 管理员充值历史能看到对应记录。
5. 同一卡密再次兑换失败。
6. 创建 `promo_credit` 批次并兑换，余额增加但充值历史不新增记录。
7. 钱包页购买链接仍可指向链动商品页。

## 14. 上线顺序

1. 备份数据库。
2. 部署后端模型字段和兑换逻辑。
3. 运行迁移，确认历史兑换码默认 `legacy` 且不计入充值。
4. 部署 default 前端管理表单和钱包兑换提示。
5. 配置或确认 `TopUpLink` 指向链动发卡网商品页。
6. 管理员创建小批量测试卡密：
   - 1 个 `paid_topup`
   - 1 个 `promo_credit`
7. 手动兑换验证余额和充值历史。
8. 再创建正式链动批次并导入链动。

## 15. 回滚方案

### 15.1 代码回滚

回滚代码后，新增字段可保留在数据库中，不影响旧代码读取核心字段。旧代码会继续按普通兑换码处理，但不会创建 `TopUp` 记录。

### 15.2 数据回滚

如果需要撤销某个误创建批次：

1. 未导入链动且未使用：管理员可禁用或删除该批次兑换码。
2. 已导入链动但未使用：优先在链动下架对应商品或清空库存，再在云贝禁用该批次。
3. 已兑换：不得直接删除，应通过管理员人工调整用户额度和记录审计日志处理。

### 15.3 配置回滚

如果链动销售暂时停用：

1. 清空或改回 `TopUpLink`。
2. 在链动平台下架对应商品。
3. 云贝已生成但未售出的卡密保持禁用或不导出。

## 16. 风险与处理

### 16.1 用户买到卡密但不回来兑换

这是链动无 API 模式的天然限制。云贝只在兑换时入账，不知道链动已售未兑状态。处理方式是保留链动订单导出与云贝批次查询，必要时客服人工引导用户兑换。

### 16.2 管理员把错误类型卡密导入错误商品

例如把 `promo_credit` 导入 10 元充值卡商品。处理方式：

1. 运营流程要求一个链动商品只导入一个类型和面值的批次。
2. 云贝导出文件名包含 `batch_id/kind/amount`。
3. 列表页展示 `kind/amount/batch_id`，便于导入前复核。

### 16.3 重复导入同一批卡密

同一张卡密只能兑换一次。重复导入会导致后购买者兑换失败。处理方式：

1. 使用 `ExportedTime` 标记已导出。
2. 导出已导出批次时给管理员二次确认。
3. 推荐链动后台按批次导入后立即记录商品与批次关系。

### 16.4 统计口径混乱

只有 `paid_topup` 创建 `TopUp`，优惠/赠送码不创建 `TopUp`。所有充值金额统计以后以 `TopUp` 为准，避免从 redemption 直接求和。

### 16.5 邀请返利被赠送码刷取

`promo_credit` 和 `legacy` 不创建 `TopUp` 且 `count_as_topup=false`。如返利逻辑存在独立入口，必须明确检查 `count_as_topup`。

## 17. 验收标准

1. 管理员可以创建 `paid_topup` 卡密，必须填写面值和统计金额。
2. 管理员可以创建 `promo_credit` 卡密，且不能计入充值金额。
3. 创建接口返回卡密列表，管理员可以复制或导出用于导入链动。
4. 用户兑换 `paid_topup` 后余额增加，并新增成功充值记录。
5. 用户兑换 `promo_credit` 后余额增加，但充值记录不新增。
6. 历史 `legacy` 兑换码仍可按旧逻辑兑换，不计入充值金额。
7. `POST /api/user/topup` 的 `data` 字段保持数字，classic 前端不破坏。
8. default 前端能根据 `redemption.kind` 显示不同成功提示。
9. 同一卡密无法重复兑换。
10. 充值卡密的 `TopUp.TradeNo` 不包含完整卡密。
11. 所有新增前端文案完成 en、zh、fr、ja、ru、vi 翻译。
12. 后端测试、前端类型检查、前端构建通过。

## 18. 最终决策摘要

本方案将链动发卡网作为外部“收款 + 发卡”渠道，而不是支付网关。云贝预先生成带类型和统计属性的兑换码，管理员导入链动商品库存。用户付款后拿到卡密，回云贝兑换。云贝按数据库中的 `kind` 和 `count_as_topup` 判断是否增加余额、是否写入充值记录、是否计入充值金额。该设计避免依赖链动非开放接口，保持现有兑换入口兼容，并为后续批次审计和人工对账留下清晰路径。
