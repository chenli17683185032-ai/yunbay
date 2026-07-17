# 超值套餐自定义并发实施计划

**日期：** 2026-07-17
**状态：** 功能与验证完成，待提交推送
**目标分支：** `main`
**范围：** 超值套餐管理表单、订阅套餐管理接口、超值套餐运行时并发限流及对应测试

## 1. 目标与成功指标

管理员应能在超值套餐编辑抽屉中直接输入任意正整数并发值，点击保存后由后端持久化，并由内存或 Redis 并发限流器按同一个值执行。

成功指标：

- 输入 `3`、`5` 等正整数时，前端校验通过，请求载荷保留原值，后端创建/更新成功。
- 保存后重新读取套餐，`concurrency_limit` 与输入值一致，不被回退或截断为 `2`。
- 运行时前 N 个请求可获得并发槽，第 N+1 个请求被拒绝；释放槽位后可再次获得。
- `0`、负数、小数或不可解析值不能形成有效套餐配置。
- 既有并发 `1`、`2` 的行为保持不变；不引入数据库迁移、依赖或新翻译键。

## 2. 现状与根因

当前链路有三处独立的上限，任何一处未修改都会导致闭环失效：

1. `subscriptions-mutate-drawer.tsx` 使用只包含 `1`、`2` 的 `Select`，管理员无法输入其它值。
2. `plan-form.ts` 使用 Zod `.max(2)`，即使绕过控件也无法提交更高值。
3. `controller/subscription.go` 只接受 `1` 或 `2`；`middleware/value_package.go` 又会把大于 `2` 的值归一化为 `2`。

数据库字段已经是通用整数，内存计数器与 Redis ZSET/Lua 限流结构也天然支持任意正整数，因此不需要迁移或重建限流模型。

## 3. 控制结构

```text
管理员输入设定值
  -> 前端正整数校验
  -> 管理接口正整数校验
  -> subscription_plans.concurrency_limit 状态
  -> 内存/Redis 并发槽执行器
  -> 请求放行或 429 反馈
  -> 管理员按实际容量继续调整设定值
```

稳定性优先级：保留 `<= 0` 时运行时回退为 `1` 的防御性归一化，防止历史脏数据关闭限流；仅移除人为的上限 `2`。管理入口仍拒绝非正值，避免把回退逻辑当成正常配置路径。

## 4. GitHub 与组件经验参考

- [QuantumNous/new-api](https://github.com/QuantumNous/new-api) 上游当前没有超值套餐 `concurrency_limit` 管理实现，本功能属于云贝扩展，不能直接照搬上游补丁。
- [PrefectHQ/prefect](https://github.com/PrefectHQ/prefect/blob/main/ui-v2/src/components/work-pools/work-pool-queue-form/work-pool-queue-form.tsx) 的工作队列并发配置采用数值 `Input`，而非枚举选择器；这与“并发是正整数设定值”的领域模型一致。
- [shadcn/ui Input](https://ui.shadcn.com/docs/components/base/input) 支持直接透传原生 `type="number"`、`min`、`step` 属性；项目已安装 Base UI 版 `Input`，无需新增组件或依赖。

## 5. 实施步骤与验证反馈

| 节点 | 动作 | 验证方式 | 状态 |
| --- | --- | --- | --- |
| A | 建立本计划并固定范围、成功指标和回滚边界 | 计划文件存在且只描述本任务 | 已完成 |
| B | 增加前端 schema/payload 自定义并发测试 | 测试先证明 `5` 可通过且小数/非正数失败 | 已完成，17/17 通过 |
| C | 增加管理接口持久化自定义并发测试 | 创建或更新为 `5` 后数据库读取仍为 `5`；`0` 被拒绝 | 已完成，controller 包通过 |
| D | 增加内存与 Redis 限流测试 | 限额 `3` 时前三次成功、第四次失败 | 已完成，middleware 包通过 |
| E | 把枚举 `Select` 改为正整数 `Input` | 可键盘输入，`min=1`、`step=1`，表单错误仍显示 | 已完成并通过浏览器验收 |
| F | 放宽前后端与运行时限制 | 只接受正整数且不再把大于 `2` 的值截断 | 已完成并通过接口/运行时测试 |
| G | 完成本地闭环验证 | 定向测试、Go 测试、typecheck、生产构建全部通过 | 已完成 |
| H | 浏览器验收 | 编辑抽屉显示数值输入且可输入自定义值，布局无重叠 | 已完成 |
| I | 提交并普通推送 GitHub `main` | 仅暂存本任务文件，远端 main 指向功能提交 | 进行中 |

## 6. 预计修改文件

- `web/default/src/features/subscriptions/components/subscriptions-mutate-drawer.tsx`
- `web/default/src/features/subscriptions/lib/plan-form.ts`
- `web/default/src/features/subscriptions/lib/plan-form-value-package.test.ts`
- `controller/subscription.go`
- `controller/value_package_test.go`
- `middleware/value_package.go`
- `middleware/value_package_test.go`
- 本计划文件

如验证表明无需修改某个预计文件，则不为满足清单而制造改动。

## 7. 测试矩阵

前端：

- 自定义正整数通过 schema。
- `0`、负数和小数被 schema 拒绝。
- payload 保留自定义并发值。
- TypeScript 类型检查与生产构建通过。

后端：

- 管理接口接受并持久化大于 `2` 的正整数。
- 管理接口拒绝 `0` 或负数。
- 内存限流器按自定义值放行和拒绝。
- Redis 限流器按自定义值放行和拒绝。
- 相关 controller 与 middleware 测试包无回归。

界面：

- 桌面与窄视口中字段尺寸稳定、无溢出或遮挡。
- 数值输入可输入 `5`，保存动作可被触发；不对生产数据执行写入验收。

## 8. 验证结果

- `bun test`：两个相关测试文件共 17 项全部通过。
- `go test ./controller ./middleware -count=1`：两个完整 Go 包通过。
- `bun run typecheck`：通过。
- `bun run build`：生产构建通过。
- 变更文件级 ESLint：通过。
- 变更文件级 Prettier 与 `git diff --check`：通过。
- 全仓 `bun run lint`：基线仍有 113 个错误和 6 个警告，集中在本任务未修改的既有组件与 hooks；本次变更文件没有新增 Lint 问题。
- 生产构建浏览器闭环：在临时 SQLite 副本中把 `concurrency_limit` 从 `2` 输入为 `5` 并点击保存，页面显示“更新成功”，数据库读取为 `5`。
- 响应式验收：桌面抽屉无重叠；390 x 844 视口下并发输入宽 357px，左右边界为 17px/374px，完全位于 390px 视口内。
- 临时数据库与浏览器测试标签页已清理；原始 `one-api.db` 未写入测试套餐或测试密码。

## 9. 风险、边界与回滚

- 高并发设定会放大上游容量与成本压力，但这是管理员明确配置的业务值；本次不擅自增加未提出的上限。
- Go `int`、数据库整数和 Redis Lua `tonumber` 对正常管理值足够；异常超大值仍会在请求解析或存储边界失败。
- 不修改普通订阅行为、额度规则、支付配置、数据库结构、生产配置或服务拓扑。
- 回滚只需恢复本次代码文件；没有数据迁移。已保存的大于 `2` 的值若回滚旧代码，会被旧运行时截断为 `2`，因此回滚前应先把相关套餐值改回 `1` 或 `2`。

## 10. 交付与部署边界

- 本轮先完成本地代码、验证和 GitHub `main` 推送。
- 用户本条指令没有明确要求“部署”，因此不连接生产、不上传文件、不构建或重启生产服务。
- 工作区已有 `docs/yunbay-maintenance.md`、旧规格文档和 `outputs/` 变更均不属于本任务，不修改、不暂存、不清理。
- 所有测试反馈、最终提交和推送结果继续维护在本文件，不另开计划文件。

## 11. 实施记录

- 2026-07-17：完成代码链路、运行时限流结构、GitHub 同类实现和 shadcn Input 规范调研；确认问题同时存在于 UI、Zod、管理接口和运行时归一化四层。
- 2026-07-17：增加前端表单/源码、管理接口、内存限流和 Redis 限流回归测试，固定自定义值、非法值和实际槽位行为。
- 2026-07-17：完成 UI、Zod、管理接口和运行时限流的最小修改；保留非法历史值回退为 `1` 的稳定性保护。
- 2026-07-17：完成定向与完整包测试、类型检查、生产构建、变更文件 Lint/格式检查，以及桌面/390px 窄屏浏览器闭环；临时数据库实测保存值为 `5` 后已删除。
