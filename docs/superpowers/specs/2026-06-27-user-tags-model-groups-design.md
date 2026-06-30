# 用户标签与模型分组分离设计 Spec

**日期：** 2026-06-27
**项目：** 云贝 new-api 生产配置与注册/邀请/充值规则修复
**状态：** 已完成生产同步与复核（2026-06-27），并已纳入 2026-06-30 全量上线基线；本文保留为设计依据与生产验收补充记录
**范围：** 后端用户注册、用户标签、模型分组、倍率配置、充值升级、邀请关系、注册页文案与输入框、历史数据迁移

---

## 1. 背景

当前系统把两个概念混在了一起：

1. **用户标签/用户等级**：例如 `体验用户`、`vip`，用于决定用户享受什么价格。
2. **模型分组/Token 分组**：例如 `gpt-plus`、`gpt-pro`，用于决定 token 请求访问哪个模型池。

生产环境中已经出现以下问题：

- 新注册用户默认进入 `default`，不符合业务预期。
- 用户标签会被自动加入可用模型分组，导致前台可能看到 `default`、`体验用户` 等不该出现的模型分组选项。
- 注册页没有可见邀请码输入框，但系统会从 URL/localStorage 隐式提交 `aff_code`。
- 一些账号存在邀请人数/邀请关系异常，用户不知道邀请关系来源。
- 邮箱字段文案没有明确提示“仅支持 QQ 邮箱”。
- 未来开启充值后，需要累计充值满 30 元自动升级为 VIP。

本 spec 的目标是把用户身份标签和模型分组彻底拆开，让所有用户只选择 Plus/PRO 两个模型分组，但根据用户标签使用不同计费倍率。

---

## 2. 已审计的生产事实

生产环境只读审计结果如下，以下信息不包含服务器密钥、IP、token 或其他敏感内容。

### 2.1 生产部署形态

- 应用容器：`yunbay-new-api`
- PostgreSQL 容器：`yunbay-postgres`
- Redis 容器：`yunbay-redis`
- 生产数据库：PostgreSQL 15，数据库名 `new_api`
- 部署目录：`/opt/new-api/app`

### 2.2 生产 options 当前状态

当前 `GroupRatio`：

```json
{
  "gpt-plus": 0.3,
  "gpt-pro": 0.4,
  "体验用户": 1
}
```

当前 `GroupGroupRatio`：

```json
{
  "vip": {
    "edit_this": 0.9
  },
  "体验用户": {
    "gpt-plus": 1,
    "gpt-pro": 1
  }
}
```

当前 `UserUsableGroups`：

```json
{
  "gpt-plus": "默认分组",
  "gpt-pro": "",
  "体验用户": ""
}
```

当前 `TopupGroupRatio`：

```json
{
  "svip": 1,
  "vip": 1
}
```

### 2.3 生产用户与 token 状态

`users.group` 分布：

```text
default   普通用户 21 个
体验用户  普通用户 19 个
default   管理员 1 个
default   root 1 个
```

`tokens.group` 分布：

```text
gpt-plus  27 个
空值       6 个
default    3 个
gpt-pro    2 个
体验用户    1 个
```

`abilities.group` 当前只包含：

```text
gpt-plus
gpt-pro
```

`top_ups` 当前无记录。

### 2.4 生产邀请关系状态

当前真实邀请关系以 `users.inviter_id` 为准：

```text
实际 users.inviter_id 邀请总数: 15
users.aff_count 总和: 0
不一致邀请人用户: 5 个
```

明细：

```text
inviter_id=16 -> 7 个 invitee
inviter_id=11 -> 5 个 invitee
inviter_id=6  -> 1 个 invitee
inviter_id=14 -> 1 个 invitee
inviter_id=17 -> 1 个 invitee
```

结论：`aff_count` 已经不能作为邀请人数的唯一可信来源。展示和校验应以 `inviter_id` 实时统计为准。

---

## 3. 目标

### 3.1 业务目标

1. 新注册普通用户默认成为 `体验用户`。
2. 用户累计成功充值金额达到 30 元后，自动升级为 `vip`。
3. 所有用户只选择两个模型分组：
   - `gpt-plus`，展示为 `Plus 模型分组`
   - `gpt-pro`，展示为 `PRO 模型分组`
4. 用户标签和模型分组彻底分离：
   - `users.group` 表示用户标签。
   - `tokens.group` / `abilities.group` 表示模型分组。
5. 体验用户和 VIP 访问同样的模型分组，但使用不同倍率。
6. 注册页显示可见的邀请码输入框，用户可以修改或清空邀请码。
7. 注册页邮箱文案改为“邮箱（仅支持 QQ 邮箱）”。
8. 后端默认限制注册邮箱只允许 `@qq.com`。
9. 邀请人数展示以 `users.inviter_id` 实时统计为准。

### 3.2 非目标

本次不做以下事情：

1. 不新增 `Experience`、`trial`、`svip` 等新用户等级。
2. 不新增除 `gpt-plus`、`gpt-pro` 之外的模型分组。
3. 不重构整个 billing expression 系统。
4. 不改动模型渠道、模型能力表的业务结构。
5. 不删除项目受保护品牌、作者、项目元信息。
6. 不直接清空历史 `inviter_id` 关系。
7. 不在本 spec 阶段修改生产数据或业务代码。

---

## 4. 术语与数据归属

### 4.1 用户标签

用户标签表示用户身份和价格等级，存储在：

```text
users.group
```

本次目标只使用：

```text
体验用户
vip
```

含义：

| 用户标签 | 含义 |
|---|---|
| `体验用户` | 新注册默认标签，使用较高倍率 |
| `vip` | 累计成功充值满 30 元后的标签，使用基础倍率 |

### 4.2 模型分组

模型分组表示 token 请求访问哪个模型池，存储或关联在：

```text
tokens.group
abilities.group
```

本次目标只使用：

```text
gpt-plus
gpt-pro
```

展示名：

| 内部值 | 前台展示 |
|---|---|
| `gpt-plus` | `Plus 模型分组` |
| `gpt-pro` | `PRO 模型分组` |

### 4.3 明确禁止混用

以下值是用户标签，不允许作为模型分组选项展示：

```text
体验用户
vip
default
```

以下值是模型分组，不允许作为用户标签自动写入 `users.group`：

```text
gpt-plus
gpt-pro
```

---

## 5. 目标配置

### 5.1 `UserUsableGroups`

目标配置：

```json
{
  "gpt-plus": "Plus 模型分组",
  "gpt-pro": "PRO 模型分组"
}
```

说明：

- 所有普通用户和 VIP 用户都只看到这两个模型分组。
- 不再在这里配置 `体验用户`、`vip`、`default`。

### 5.2 `GroupRatio`

目标配置：

```json
{
  "gpt-plus": 0.3,
  "gpt-pro": 0.4
}
```

说明：

- `gpt-plus` 基础倍率为 `0.3`。
- `gpt-pro` 基础倍率为 `0.4`。
- 这个基础倍率也是 VIP 用户最终使用的倍率。

### 5.3 `GroupGroupRatio`

目标配置：

```json
{
  "体验用户": {
    "gpt-plus": 0.99,
    "gpt-pro": 1.32
  }
}
```

说明：

- 当前代码中的 `GroupGroupRatio` 是“最终倍率覆盖值”，不是乘法因子。
- `0.99 = 0.3 * 3.3`。
- `1.32 = 0.4 * 3.3`。
- `vip` 不需要写入 `GroupGroupRatio`，因为 VIP 没有特殊覆盖，直接走 `GroupRatio`。

如果后台为了可读性必须显式配置 VIP，也只能写最终倍率：

```json
{
  "体验用户": {
    "gpt-plus": 0.99,
    "gpt-pro": 1.32
  },
  "vip": {
    "gpt-plus": 0.3,
    "gpt-pro": 0.4
  }
}
```

但推荐不写 `vip` 覆盖，避免以后改基础倍率时出现重复配置不同步。

### 5.4 特殊可见模型分组

目标配置：

```json
{}
```

说明：

- 本次不需要做“部分用户可见，部分用户不可见”的模型分组。
- 所有用户都可见 `gpt-plus` 和 `gpt-pro`。
- 用户差异只体现在倍率，不体现在可见分组。

---

## 6. 倍率规则

最终倍率矩阵：

| 用户标签 | 模型分组 | 基础倍率 | 用户标签覆盖 | 最终倍率 |
|---|---|---:|---:|---:|
| `vip` | `gpt-plus` | `0.3` | 无 | `0.3` |
| `vip` | `gpt-pro` | `0.4` | 无 | `0.4` |
| `体验用户` | `gpt-plus` | `0.3` | `0.99` | `0.99` |
| `体验用户` | `gpt-pro` | `0.4` | `1.32` | `1.32` |

要求：

1. 计费逻辑先检查 `GroupGroupRatio[userTag][usingGroup]`。
2. 如果存在覆盖值，使用覆盖值作为最终倍率。
3. 如果不存在覆盖值，使用 `GroupRatio[usingGroup]` 作为最终倍率。
4. 不把 `GroupGroupRatio` 当成乘法系数。

---

## 7. 后端设计

### 7.1 新注册用户默认标签

涉及文件：

```text
/Users/ethan/Documents/yunbay/controller/user.go
/Users/ethan/Documents/yunbay/model/user.go
/Users/ethan/Documents/yunbay/controller/oauth.go
/Users/ethan/Documents/yunbay/controller/github.go
```

设计要求：

1. 普通邮箱注册成功时，`users.group` 必须为 `体验用户`。
2. OAuth/GitHub 等第三方注册入口也必须兜底为 `体验用户`。
3. `model.User.Insert()` 和 `model.User.InsertWithTx()` 在写入前，如果 `Group` 为空，也要兜底为 `体验用户`。
4. 不依赖数据库默认值作为唯一保障。
5. 管理员手动创建特殊用户时，如果显式传入了其他合法 group，不被兜底覆盖。

### 7.2 用户可用模型分组

涉及文件：

```text
/Users/ethan/Documents/yunbay/service/group.go
```

当前问题：

```go
if _, ok := groupsCopy[userGroup]; !ok {
    groupsCopy[userGroup] = "用户分组"
}
```

这段逻辑会把用户标签自动塞进模型分组列表。

目标行为：

1. `GetUserUsableGroups("体验用户")` 只返回 `gpt-plus` 和 `gpt-pro`。
2. `GetUserUsableGroups("vip")` 只返回 `gpt-plus` 和 `gpt-pro`。
3. 不再自动把 `userGroup` 加进可用模型分组。
4. 仍然保留特殊可见分组的加减能力，供未来需要时使用。

### 7.3 鉴权上下文命名和语义

涉及文件：

```text
/Users/ethan/Documents/yunbay/middleware/auth.go
```

设计要求：

1. 明确区分：
   - `userTag`：来自 `users.group`，例如 `体验用户`、`vip`。
   - `tokenGroup`：来自 `tokens.group`，例如 `gpt-plus`、`gpt-pro`。
   - `usingGroup`：本次请求实际使用的模型分组。
2. 鉴权时用 `userTag` 检查该用户是否有权使用 `tokenGroup`。
3. 请求上下文中：
   - `ContextKeyUserGroup` 保持用户标签语义。
   - `ContextKeyUsingGroup` 保持模型分组语义。
4. 不再通过变量复用让 `userGroup = tokenGroup` 这种写法混淆语义。

### 7.4 充值满 30 元升级 VIP

涉及文件候选：

```text
/Users/ethan/Documents/yunbay/model/topup.go
/Users/ethan/Documents/yunbay/controller/topup.go
/Users/ethan/Documents/yunbay/service/quota.go
```

具体落点以现有充值成功写入链路为准。

设计要求：

1. 每次充值成功后，统计该用户所有成功充值记录的累计金额。
2. 累计金额 `>= 30` 元时，将普通用户从 `体验用户`、`default` 或空 group 升级为 `vip`。
3. 升级逻辑只处理普通用户，不处理 root、管理员或其他特殊角色。
4. 已经是 `vip` 的用户不重复处理。
5. 如果用户是未来自定义特殊标签，不覆盖。
6. 逻辑必须幂等，支付回调重复触发不会产生错误状态。

### 7.5 注册邮箱限制

涉及文件：

```text
/Users/ethan/Documents/yunbay/controller/user.go
```

设计要求：

1. 邮箱注册只允许 `@qq.com`。
2. 域名比较应大小写不敏感。
3. 以下邮箱允许：

```text
123456@qq.com
ABC@qq.com
```

4. 以下邮箱拒绝：

```text
abc@gmail.com
abc@163.com
abc@outlook.com
abc@foxmail.com
abc@qq.com.cn
```

5. 拒绝时返回清晰错误文案：

```text
仅支持 QQ 邮箱注册
```

### 7.6 邀请码校验

涉及文件：

```text
/Users/ethan/Documents/yunbay/controller/user.go
/Users/ethan/Documents/yunbay/model/user.go
```

设计要求：

1. `aff_code` 为空：正常注册，不绑定邀请人。
2. `aff_code` 非空且有效：正常注册，绑定 `inviter_id`。
3. `aff_code` 非空但无效：注册失败，返回“邀请码无效”。
4. 不再静默忽略无效邀请码。
5. 注册用户不能邀请自己。因为注册前用户尚未存在，正常注册路径不会出现自邀；如果未来存在绑定邀请人的后置接口，该接口需要显式防自邀。

### 7.7 邀请人数统计

涉及文件候选：

```text
/Users/ethan/Documents/yunbay/model/user.go
/Users/ethan/Documents/yunbay/controller/user.go
```

设计要求：

1. 前台展示邀请人数时，以 `users.inviter_id` 实时统计为准。
2. `aff_count` 可以保留为历史兼容字段或缓存字段，但不能作为唯一真实来源。
3. 用户详情、用户列表、邀请相关 API 如果展示邀请人数，应统一使用同一种统计方式。
4. 统计时排除软删除用户。

---

## 8. 前端设计

### 8.1 新版注册页

涉及文件：

```text
/Users/ethan/Documents/yunbay/web/default/src/features/auth/sign-up/components/sign-up-form.tsx
/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/en.json
/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/zh.json
/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/fr.json
/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/ja.json
/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/ru.json
/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/vi.json
```

设计要求：

1. 邮箱 label 改为：

```text
Email (QQ mailbox only)
```

中文展示：

```text
邮箱（仅支持 QQ 邮箱）
```

2. 新增可见邀请码输入框。
3. 邀请码字段 label：

```text
Invitation code (optional)
```

中文展示：

```text
邀请码（可选）
```

4. 邀请码 placeholder：

```text
Leave blank if you do not have one
```

中文展示：

```text
如无邀请码可留空
```

5. 如果 URL 中存在 `?aff=xxx`，邀请码输入框预填 `xxx`。
6. 如果 localStorage 中存在旧 `aff`，邀请码输入框可以预填该值。
7. 用户手动清空邀请码输入框后：
   - 注册请求不提交 `aff_code`。
   - 清理 localStorage 中旧的 `aff`。
8. 用户手动修改邀请码后，以输入框当前值为准。
9. 注册提交时不要再无感使用隐藏 `aff_code`。

### 8.2 经典注册页

涉及文件：

```text
/Users/ethan/Documents/yunbay/web/classic/src/components/auth/RegisterForm.jsx
```

设计要求：

1. 邮箱 label 改为：

```text
邮箱（仅支持 QQ 邮箱）
```

2. 邮箱 placeholder 改为：

```text
请输入 QQ 邮箱地址
```

3. 新增可见邀请码输入框。
4. URL/localStorage 邀请码只作为预填值，用户可改可清空。
5. 用户清空后不提交 `aff_code`，并清理 localStorage 中旧 `aff`。

### 8.3 模型分组展示

涉及文件候选：

```text
/Users/ethan/Documents/yunbay/web/default/src/**
/Users/ethan/Documents/yunbay/web/classic/src/**
```

具体文件以现有 token 创建页/分组选择组件为准。

设计要求：

1. 模型分组下拉框只展示 API 返回的可用模型分组。
2. API 返回配置应只有：

```text
Plus 模型分组
PRO 模型分组
```

3. 前端不硬编码展示 `体验用户`、`vip`、`default`。
4. 如果后端配置异常返回用户标签，前端不做本次防御性过滤；本次从后端根源修复。

---

## 9. 历史数据迁移设计

生产数据迁移必须在代码部署完成并验证服务正常后执行。执行前必须备份数据库。

### 9.1 备份要求

至少备份以下表：

```text
users
tokens
options
top_ups
```

推荐整库备份，方便回滚。

### 9.2 迁移普通用户 `default` 到 `体验用户`

PostgreSQL 迁移 SQL：

```sql
UPDATE users
SET "group" = '体验用户'
WHERE "group" = 'default'
  AND role = 1
  AND deleted_at IS NULL;
```

说明：

- 只迁移普通用户。
- 不动 root 和管理员。

### 9.3 迁移历史 token 分组

PostgreSQL 迁移 SQL：

```sql
UPDATE tokens
SET "group" = 'gpt-plus'
WHERE "group" IS NULL
   OR "group" = ''
   OR "group" IN ('default', '体验用户');
```

说明：

- `default`、空值、`体验用户` 都不是模型分组。
- 统一迁到 `gpt-plus`，保证旧 token 继续可用。
- 已经是 `gpt-plus` 或 `gpt-pro` 的 token 不变。

### 9.4 更新 options

目标 `UserUsableGroups`：

```json
{
  "gpt-plus": "Plus 模型分组",
  "gpt-pro": "PRO 模型分组"
}
```

目标 `GroupRatio`：

```json
{
  "gpt-plus": 0.3,
  "gpt-pro": 0.4
}
```

目标 `GroupGroupRatio`：

```json
{
  "体验用户": {
    "gpt-plus": 0.99,
    "gpt-pro": 1.32
  }
}
```

目标特殊可见配置：

```json
{}
```

### 9.5 校准 `aff_count`

PostgreSQL 校准 SQL：

```sql
WITH actual AS (
  SELECT inviter_id, COUNT(*) AS actual_count
  FROM users
  WHERE inviter_id IS NOT NULL
    AND inviter_id <> 0
    AND deleted_at IS NULL
  GROUP BY inviter_id
)
UPDATE users u
SET aff_count = actual.actual_count
FROM actual
WHERE u.id = actual.inviter_id;
```

将无实际邀请人的用户 `aff_count` 归零：

```sql
UPDATE users u
SET aff_count = 0
WHERE NOT EXISTS (
  SELECT 1
  FROM users invitee
  WHERE invitee.inviter_id = u.id
    AND invitee.deleted_at IS NULL
);
```

说明：

- 这只是把历史字段校准到当前真实关系。
- 后续展示仍以 `inviter_id` 实时统计为准。

---

## 10. 验收标准

### 10.1 注册验收

1. 不填邀请码，使用 `@qq.com` 邮箱注册成功。
2. 新注册用户的 `users.group` 为 `体验用户`。
3. 使用非 `@qq.com` 邮箱注册失败，提示“仅支持 QQ 邮箱注册”。
4. 填有效邀请码注册成功，并正确写入 `inviter_id`。
5. 填无效邀请码注册失败，提示“邀请码无效”。
6. URL/localStorage 中存在旧邀请码时，注册页展示为可见预填值。
7. 用户清空邀请码后，注册请求不提交 `aff_code`。

### 10.2 模型分组验收

1. `体验用户` 可用模型分组只包含 `gpt-plus` 和 `gpt-pro`。
2. `vip` 可用模型分组只包含 `gpt-plus` 和 `gpt-pro`。
3. 前台 token 创建页只展示 `Plus 模型分组` 和 `PRO 模型分组`。
4. 前台不展示 `default`、`体验用户`、`vip` 作为模型分组选项。
5. `tokens.group = gpt-plus` 的 token 可以正常请求。
6. `tokens.group = gpt-pro` 的 token 可以正常请求。

### 10.3 计费验收

1. `体验用户 + gpt-plus` 最终倍率为 `0.99`。
2. `体验用户 + gpt-pro` 最终倍率为 `1.32`。
3. `vip + gpt-plus` 最终倍率为 `0.3`。
4. `vip + gpt-pro` 最终倍率为 `0.4`。
5. 不出现把 `3.3` 当最终倍率直接计费的情况。

### 10.4 充值升级验收

1. 体验用户累计成功充值 `29.99` 元，不升级。
2. 体验用户累计成功充值 `30` 元，升级为 `vip`。
3. 体验用户累计成功充值超过 `30` 元，升级为 `vip`。
4. 已经是 `vip` 的用户再次充值，仍保持 `vip`。
5. root、管理员不会被充值升级逻辑改成 `vip`。
6. 重复支付回调不会造成重复异常或错误状态。

### 10.5 邀请统计验收

1. 邀请人数展示与 `users.inviter_id` 实时统计一致。
2. `aff_count` 与 `inviter_id` 不一致时，展示仍然正确。
3. 软删除用户不计入邀请人数。
4. 生产历史邀请关系不会被上线过程直接清空。

---

## 11. 测试策略

### 11.1 后端单元测试

建议覆盖：

1. `service.GetUserUsableGroups("体验用户")` 不自动追加 `体验用户`。
2. `service.GetUserUsableGroups("vip")` 不自动追加 `vip`。
3. 注册默认用户标签为 `体验用户`。
4. 非 `@qq.com` 邮箱注册失败。
5. 有效邀请码注册成功。
6. 无效邀请码注册失败。
7. 充值累计满 30 后升级 `vip`。
8. 管理员/root 不被充值升级逻辑修改。
9. 邀请人数统计基于 `inviter_id`。
10. 倍率解析满足四种最终倍率组合。

### 11.2 后端兼容性测试

由于项目要求 SQLite、MySQL、PostgreSQL 同时兼容，新增 DB 逻辑必须遵守：

1. 优先使用 GORM 查询与更新。
2. 避免 PostgreSQL-only SQL 写入业务代码。
3. 如果迁移脚本使用 PostgreSQL SQL，仅作为生产一次性运维脚本，不写入通用迁移逻辑。
4. 业务代码中 JSON marshal/unmarshal 使用 `/Users/ethan/Documents/yunbay/common/json.go` 包装函数。

### 11.3 前端测试

新版前端至少验证：

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun run build
```

如修改 i18n key，执行：

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun run i18n:sync
```

经典前端按现有脚本执行 build 或 lint，具体命令以 `/Users/ethan/Documents/yunbay/web/classic/package.json` 为准。

---

## 12. 上线顺序

1. 本地实现代码并完成测试。
2. 构建前后端产物。
3. 部署代码到生产。
4. 确认 `yunbay-new-api` 正常启动。
5. 备份生产数据库。
6. 更新 options 配置。
7. 迁移普通用户 `default -> 体验用户`。
8. 迁移历史 token 的非法模型分组到 `gpt-plus`。
9. 校准 `aff_count`。
10. 执行生产回归验证。

---

## 13. 回滚方案

### 13.1 代码回滚

如果新逻辑导致注册、鉴权或计费异常，回滚到上一版稳定镜像或上一版部署 commit。

### 13.2 配置回滚

上线前导出以下 options：

```text
UserUsableGroups
GroupRatio
GroupGroupRatio
GroupSpecialUsableGroup
TopupGroupRatio
```

如果配置更新导致异常，按备份恢复这些 options。

### 13.3 数据回滚

上线前导出：

```sql
SELECT id, username, email, "group", aff_count, inviter_id
FROM users;
```

以及：

```sql
SELECT id, user_id, name, "group"
FROM tokens;
```

如迁移后出现问题，按备份恢复相关字段。

---

## 14. 风险与处理

### 14.1 旧 token 分组迁移风险

风险：旧 token 原本 group 为空或 `default`，迁到 `gpt-plus` 后会使用 Plus 模型组。
处理：这是本次明确选择的兼容策略，因为 `gpt-plus` 是所有用户可用的基础模型分组。迁移前备份 `tokens`，需要时可按 token 粒度恢复。

### 14.2 邀请关系历史真实性风险

风险：当前生产有 15 条 `inviter_id` 关系，部分可能来自旧隐藏邀请码逻辑。
处理：本次不直接清空历史关系。先修复新增逻辑，再导出历史邀请明细给业务确认；只有明确认定误绑定的记录才单独清理。

### 14.3 邮箱限制影响现有用户

风险：后端限制 `@qq.com` 只影响新注册，不影响现有非 QQ 邮箱用户登录。
处理：注册校验只放在注册路径，不放在登录路径。

### 14.4 倍率配置误填风险

风险：误把 `3.3` 写入 `GroupGroupRatio` 会导致体验用户按 3.3 倍计费。
处理：上线前检查 `GroupGroupRatio` 必须是最终倍率 `0.99` 和 `1.32`，并通过计费测试确认。

---

## 15. 最终决策摘要

1. `users.group` 只作为用户标签使用：`体验用户`、`vip`。
2. `tokens.group` 和 `abilities.group` 只作为模型分组使用：`gpt-plus`、`gpt-pro`。
3. 新注册普通用户默认 `体验用户`。
4. 累计成功充值满 30 元自动升级 `vip`。
5. 所有用户都只看到 `Plus 模型分组` 和 `PRO 模型分组`。
6. VIP 访问 Plus/PRO 使用基础倍率 `0.3` / `0.4`。
7. 体验用户访问 Plus/PRO 使用最终倍率 `0.99` / `1.32`。
8. 注册页新增可见的邀请码输入框。
9. 无效邀请码注册失败，不再静默忽略。
10. 邀请人数以 `inviter_id` 实时统计为准。
11. 注册邮箱文案改为“邮箱（仅支持 QQ 邮箱）”。
12. 后端注册默认只允许 `@qq.com` 邮箱。
13. 历史普通用户 `default` 迁移到 `体验用户`。
14. 历史 token 的空值、`default`、`体验用户` 分组迁移到 `gpt-plus`。

---

## 16. 生产完成记录（2026-06-27 / 2026-06-30）

本功能已在 2026-06-27 完成生产同步与复核，并在 2026-06-30 全量上线基线中继续保留。此前“等待实现计划与落地”的状态已被本节替换为生产完成事实；后续如果用户标签、模型分组或注册策略再次变化，只能追加新记录，不要改写本节历史。

### 16.1 生产同步基线

首次生产同步记录：

```text
分支：codex/user-tags-model-groups
功能修复提交：0f0ac266 fix: separate admin user tag options
维护记录提交：a1a38836 docs: record user tag deployment verification
同步方式：非删除式 rsync --relative --files-from，同步本次提交涉及的 21 个文件到 /opt/new-api/app/
重建方式：docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml build new-api
重启方式：docker compose --env-file /opt/new-api/secrets/prod.env -f docker-compose.prod.yml up -d --force-recreate new-api
```

2026-06-30 全量上线保留基线：

```text
41360edd feat: separate user tags from model groups
83479b48 fix(router): register user group tags route
```

`83479b48` 补上用户标签路由注册，避免前端用户标签接口缺路由。

### 16.2 生产复核结果

2026-06-27 生产复核结果：

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

部署后发现 1 个普通用户历史残留 `users.group=gpt-plus`。已按“普通用户无 VIP 条件则归为体验用户”的规则做一次性修正为 `体验用户`，并在生产服务器 `/opt/new-api/backups/` 下保留 root-only TSV 备份：

```text
user_group_cleanup_20260627_145927.tsv
```

### 16.3 后续维护要求

- 用户管理页编辑 `users.group` 时必须使用用户标签接口 `GET /api/user/group-tags`；
- API Key / Token / 模型能力继续使用模型分组，不能把 `体验用户` / `vip` 当模型分组；
- 任何调整注册、邀请码、默认 token group、充值升级 VIP 的改动，都必须重新验证 `users.group` 与 `tokens.group` 语义没有混用；
- 不要在文档中记录后台 cookie、session、access token、数据库连接串或备份 TSV 内容。
