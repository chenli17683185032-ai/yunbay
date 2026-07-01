# 模型广场选中模型价格同步设计

## 背景

当前项目已经有上游价格同步能力，也曾引入过不可见的 OpenAI 官方 API 定价同步渠道，但实际价格不够准确。模型广场涉及的模型不只 OpenAI/GPT，还包括 Claude、Gemini 等多家模型。管理员需要在模型广场/模型计价界面中，对已经存在的模型执行可控的一键价格同步：同时参考 OpenRouter 与官方价格来源，逐项比较后采用更高价格，并推送到模型计价渠道。

## 目标

1. 管理员在模型广场已有模型列表里勾选模型后，点击“同步选中模型价格”。
2. 后端只处理前端提交且模型广场已存在的模型；未勾选或模型广场不存在的模型一律不新增、不更新。
3. 对每个模型、每个计价维度，比较 OpenRouter 与官方价格，取更高者作为最终价格。
4. 先预览差异，再应用；应用时后端重新计算，不信任前端提交的价格。
5. 将最终价格写入表达式计费配置，并尽量维护旧版倍率字段作为展示和回退兼容。
6. 覆盖缓存相关价格，降低漏算 cache read/cache write/cache write 1h 的风险。

## 非目标

1. 不自动把 OpenRouter 中存在但模型广场没有的模型加入模型广场。
2. 不在第一版把 `web_search` 等工具价格塞进 token 计费表达式；工具价格后续单独维护。
3. 不做复杂模糊匹配导致误更新；模型匹配宁可跳过，也不猜测写错。
4. 不改变现有品牌、项目身份、授权、登录等无关逻辑。

## 价格来源与维度

### 来源

- OpenRouter：使用管理员选择的 OpenRouter 渠道的可用 key 请求模型列表，读取模型价格字段。
- 官方价格：复用已有官方/第三方官方化来源能力（当前已有 `models.dev` 转换入口），并保留未来扩展官方预设的接口边界。
- 当前配置：读取现有模型广场 `model.GetPricing()` 与计费配置，用于预览 diff。

### 标准化维度

所有来源先统一为“USD / 1M tokens”的价格：

| 标准维度 | OpenRouter 字段 | 计费表达式变量 | 说明 |
| --- | --- | --- | --- |
| input | `prompt` | `p` | 输入文本基础价格 |
| output | `completion` | `c` | 输出文本基础价格 |
| cache_read | `input_cache_read` | `cr` | 缓存读取 |
| cache_write | `input_cache_write` | `cc` | 通用/Claude 5 分钟缓存写入 |
| cache_write_1h | `input_cache_write_1h` | `cc1h` | Claude 1 小时缓存写入 |
| image_input | `image` | `img` | 图片输入 token |
| audio_input | `audio` | `ai` | 音频输入 token |
| audio_output | 暂无稳定字段 | `ao` | 音频输出，保留接口 |
| reasoning | `internal_reasoning` | 暂不写表达式 | 第一版仅展示/跳过 |
| web_search | `web_search` | 不写表达式 | 后续走工具价格体系 |

OpenRouter 返回值是 USD/token 字符串，写入表达式前乘以 `1_000_000`。

## 模型匹配规则

输入模型列表来自管理员勾选项。后端执行以下过滤与匹配：

1. 先与 `model.GetPricing()` 的 `ModelName` 求交集，不在模型广场的模型直接跳过。
2. 对每个保留模型做保守候选匹配：
   - 精确等于本地模型名；
   - OpenRouter id 去掉常见 provider 前缀后等于本地模型名，例如 `openai/gpt-4.1` → `gpt-4.1`；
   - Claude 常见点号/连字符别名归一，例如 `anthropic/claude-sonnet-4.5` 与 `claude-sonnet-4-5`；
   - Google 等常见 provider 前缀按同样规则去前缀。
3. 若存在多个候选，只接受确定性最高的精确或归一匹配；无法确定则跳过并在预览中提示。

## 后端 API

沿用现有 `/api/ratio_sync` 管理域，新增：

### `POST /api/ratio_sync/model_price/preview`

请求：

```json
{
  "openrouter_channel_id": 12,
  "models": ["claude-sonnet-4-5", "gpt-4.1"]
}
```

响应包含每个模型的：当前价格、OpenRouter 价格、官方价格、最终价格、表达式、状态、跳过原因与按维度来源说明。

### `POST /api/ratio_sync/model_price/apply`

请求同 preview。后端重新抓取并计算，不使用前端预览里的价格。成功后写入选中且可匹配模型的配置，并返回本次实际写入摘要。

## 写入策略

主写入：

- `billing_setting.billing_mode`：对应模型设为 `tiered_expr`。
- `billing_setting.billing_expr`：生成如 `tier("base", p * 3 + c * 15 + cr * 0.3 + cc * 3.75 + cc1h * 6)` 的表达式。

兼容写入：

- `ModelRatio`：当 input 价格存在时，按 `input_price * QuotaPerUnit / 1_000_000` 反推旧版输入倍率。
- `CompletionRatio`：当 input/output 均存在且 input > 0 时，写 `output / input`。
- `CacheRatio`：当 input/cache_read 均存在且 input > 0 时，写 `cache_read / input`。
- `CreateCacheRatio`：当 input/cache_write 均存在且 input > 0 时，写 `cache_write / input`。
- `ImageRatio`、`AudioRatio`、`AudioCompletionRatio` 同理尽量维护。

表达式是结算主路径，旧版倍率主要用于旧路径、展示和回退。生成后必须调用 billingexpr 编译校验，并做非负 smoke check。

## 前端流程

在管理员“模型计费/模型广场”现有表格中新增勾选能力和批量按钮：

1. 管理员勾选一个或多个模型。
2. 点击“同步选中模型价格”。
3. 弹窗选择 OpenRouter 渠道，点击“预览同步”。
4. 弹窗展示 diff 表：模型、当前、官方、OpenRouter、最终、状态。
5. 点击“应用同步”后调用 apply；成功后刷新模型计价数据与系统配置。

UI 文案使用 `t()`，补全 en/zh/fr/ja/ru/vi locale。

## 错误处理

- 未选择模型：前端禁用按钮并提示。
- 未选择 OpenRouter 渠道：不允许预览/应用。
- 渠道不可用或无 key：后端返回错误，前端 toast 展示。
- 某模型无匹配价格：该模型在预览中标记跳过，不影响其他模型。
- 表达式校验失败：该模型跳过，apply 不写入。
- 全部跳过：apply 返回成功响应但 `applied_count = 0`，前端提示没有可更新模型。

## 测试策略

后端：

1. OpenRouter 价格解析覆盖 prompt/completion/cache read/cache write/cache write 1h/image/audio/reasoning/web_search。
2. 模型匹配覆盖精确、去 provider 前缀、Claude 点号别名、不确定跳过。
3. 合并逻辑覆盖按维度取高价、缺失维度 fallback、未选中/不在模型广场跳过。
4. 表达式生成和编译覆盖 cache 变量。
5. apply 写入使用 `model.UpdateOptionsBulk()`，并保证不写未选模型。

前端：

1. 选中模型后按钮可用，未选中禁用。
2. 预览弹窗显示后端 diff 和跳过状态。
3. 应用成功后刷新数据并清空选择。
4. i18n 同步无缺失 key。

## 验收标准

1. 管理员只能同步勾选且模型广场已存在的模型。
2. 对每个维度最终价等于 OpenRouter 与官方价中的较高值。
3. cache read/cache write/cache write 1h 能进入表达式，不会被旧逻辑漏算。
4. apply 后模型进入 `tiered_expr` 计费模式，表达式可编译。
5. 未匹配模型不会被新增或写入。
6. 前端构建和相关测试通过；若本地缺少 Go/Bun 等工具，交付说明必须记录实际验证边界。
