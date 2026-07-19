# 超值套餐自定义并发实施计划

**日期：** 2026-07-17
**状态：** 功能已完成，生产部署进行中
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
| I | 提交并普通推送 GitHub `main` | 仅暂存本任务文件，远端 main 指向功能提交 | 已完成，`9bdb977b` 已推送 |

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

- 功能阶段已经完成本地代码、验证和 GitHub `main` 推送；2026-07-19 用户随后明确指令“部署”，现已授权执行生产发布。
- 生产部署身份固定为功能提交 `9bdb977b`。经 `git diff 9bdb977b..46930c8c -- <四个生产源码文件>` 复核，功能提交之后这些文件没有变化；只同步下方生产源码清单，不把后续文档或其它功能提交宣称为本次发布内容。
- 工作区已有旧规格文档和 `outputs/` 均不属于本任务，不修改、不暂存、不清理；部署完成后只在既有 `docs/yunbay-maintenance.md` 中追加本次运维记录。
- 所有测试反馈、最终提交和推送结果继续维护在本文件，不另开计划文件。
- 功能、测试与本计划已作为 `9bdb977b` 普通 fast-forward 推送 GitHub `main`；提交范围仅包含本任务 9 个文件。

## 11. 实施记录

- 2026-07-17：完成代码链路、运行时限流结构、GitHub 同类实现和 shadcn Input 规范调研；确认问题同时存在于 UI、Zod、管理接口和运行时归一化四层。
- 2026-07-17：增加前端表单/源码、管理接口、内存限流和 Redis 限流回归测试，固定自定义值、非法值和实际槽位行为。
- 2026-07-17：完成 UI、Zod、管理接口和运行时限流的最小修改；保留非法历史值回退为 `1` 的稳定性保护。
- 2026-07-17：完成定向与完整包测试、类型检查、生产构建、变更文件 Lint/格式检查，以及桌面/390px 窄屏浏览器闭环；临时数据库实测保存值为 `5` 后已删除。
- 2026-07-17：功能提交 `9bdb977b` 已普通 fast-forward 推送 GitHub `main`；既有运维手册改动、旧规格文档和 `outputs/` 未纳入提交，生产未部署。
- 2026-07-19：用户明确授权部署；确认 `main`/`origin/main` 均为 `46930c8c`，并确认 `9bdb977b` 之后本次 4 个生产源码文件没有变化，开始按固定 upstream 流程执行生产发布。

## 12. 生产部署闭环计划（2026-07-19）

### 12.1 发布对象与最小范围

部署身份：`9bdb977b689611a828451dfe751e784a9b9943fe`。

只同步以下生产源码文件：

- `controller/subscription.go`
- `middleware/value_package.go`
- `web/default/src/features/subscriptions/components/subscriptions-mutate-drawer.tsx`
- `web/default/src/features/subscriptions/lib/plan-form.ts`

测试文件、计划、运维文档、`outputs/`、未跟踪文件以及 `9bdb977b` 之后其它提交的文件都不进入生产源码包。本次无数据库迁移，不修改套餐数据、密钥、环境变量、Caddy 或其它服务配置。

### 12.2 控制对象、反馈与不变量

```text
本地已验证源码设定
  -> SHA-256 清单测量
  -> 生产独立 release 目录构建
  -> 候选镜像静态测量
  -> 标准 new-api 容器一次有界重建
  -> 容器健康 + 源站/公网 HTTP + 前端资产反馈
  -> 成功固化标记，或 watchdog 自动恢复旧源码/旧镜像
```

稳定性不变量：

- Caddy 文件、挂载与运行时 upstream 全程只能是 `new-api:3000`。
- 不创建 `new-api-green`，不调用 Caddy Admin API、`caddy reload`，不修改或重启 Caddy。
- 不重启或修改 PostgreSQL、Redis、Sub2API、CLI Proxy、LDXP 或其它无关服务。
- 构建期间旧容器持续服务；只有镜像构建、静态检查和回滚材料都通过后才切换。
- 切换前固定当前运行镜像为不可变 rollback 标签，不能只信任可变 `prod` 标签。
- SSH 断开不影响独立服务器 watchdog；新服务 45 秒内未 healthy，或在新镜像运行后退出、dead/unhealthy，自动恢复旧源码与旧镜像并只重建标准 `new-api`。watchdog 最迟 60 秒结束。

### 12.3 节点、测量与状态

| 节点 | 动作 | 成功判据 | 状态 |
| --- | --- | --- | --- |
| P0 | 固定部署身份与生产文件清单 | 4 个文件在 `9bdb977b..HEAD` 无差异；本地 SHA-256 清单生成 | 已完成，组合清单 `42ce05308585010fc067d8cc3d95897126145fea609a8c7c14fca43b97547d94` |
| P1 | 更新本计划并同步 GitHub `main` | 本节提交并普通 fast-forward 推送；不纳入用户未跟踪文件 | 已完成，`50cf6324` 已推送 |
| P2 | 重新运行本地定向回归 | 前端相关测试、Go controller/middleware 测试、typecheck、生产构建通过 | 已完成，17/17、两个 Go 包、typecheck、build 均通过 |
| P3 | 生产只读前置检查 | 部署锁空闲；标准 new-api/Caddy healthy；首页和 `/api/status` 为 200；无绿实例；固定 upstream；磁盘足够 | 待执行 |
| P4 | 建立发布归档与自动回滚材料 | 备份 4 个既有文件、生产标记和容器快照；固定旧镜像 rollback 标签；脚本和清单校验通过 | 待执行 |
| P5 | 同步并构建候选镜像 | 服务器文件 SHA-256 等于本地清单；构建成功；候选镜像/前端资产静态检查通过；旧实例仍 healthy | 待执行 |
| P6 | 有界切换标准 `new-api` | 独立 60 秒 watchdog 已启动；Compose 仅执行 `--no-deps --force-recreate --no-build new-api` | 待执行 |
| P7 | 生产反馈验收 | new-api/Caddy healthy 且 restart=0；首页、源站/公网 `/api/status` 连续 5 次 200；upstream 不变；无严重启动日志 | 待执行 |
| P8 | 功能资产与合同验收 | 生产 JS 资产包含本次数值输入特征；管理 API 保留鉴权合同；不写生产套餐、不触发计费 | 待执行 |
| P9 | 固化与记录 | 原子更新 `.yunbay-deploy-sha`/manifest；仓库和桌面唯一手册记录镜像、备份、探针及回滚路径 | 待执行 |
| P10 | 提交、推送与清理 | 运维记录普通 fast-forward 推送 `main`；只清本任务临时传输物，保留备份/回滚镜像及用户文件 | 待执行 |

### 12.4 停止、回滚与成功条件

切换前停止条件：任一基线异常、锁被占用、生产源码与已记录基线存在无法解释的漂移、固定 upstream 不成立、备份或旧镜像标签不可验证、候选构建/静态检查失败。此时不重建任何生产容器。

切换后自动回滚条件：标准 new-api 在 45 秒内未 healthy，运行新镜像后出现 exited/dead/unhealthy，或源站/公网 `/api/status` 未恢复。自动回滚恢复 4 个源码文件与旧生产标记，把固定 rollback 镜像重标为 `prod`，并使用同一 Compose 命令只重建 `new-api`。

部署成功必须同时满足：watchdog=`success`；new-api/Caddy healthy；new-api restart=0；连续 5 轮首页、源站和公网状态反馈均为 HTTP 200；Caddy upstream 始终为 `new-api:3000`；其它受保护容器身份和 restart count 不变；生产前端资产能证明自定义并发输入代码已进入实际 bundle；没有生产套餐数据写入。
