# GitHub 项目脉络与维护台账

> 快照时间：2026-07-11 19:15 +0800（Asia/Shanghai）
>
> 数据来源：本地 `git fetch --all --prune --tags` 后的引用、`git worktree list`、GitHub CLI 的仓库 / PR / issue / workflow 实时数据。
>
> 维护原则：本文件只记录项目管理脉络、分支关系、工作数量和 GitHub 流程，不记录 token、cookie、私钥、后台账号、支付密钥、完整 session id 或任何生产敏感值。

## 1. 仓库概况

| 项目 | 当前值 |
|------|--------|
| GitHub 仓库 | `chenli17683185032-ai/yunbay` |
| 可见性 | Private |
| 默认分支 | `main` |
| 当前本地工作分支 | `main` |
| 本轮代码 / 生产发布基线 SHA | `1bd746f5774847f539c47b54e69faf88fbc757a3` |
| 台账采集时 `main` / `origin/main` SHA | `1bd746f5774847f539c47b54e69faf88fbc757a3`（本台账提交前快照） |
| GitHub issue | `0` 个开放 issue |
| GitHub PR | `5` 个开放 PR，均为 Dependabot、非 Draft、当前可合并 |
| 仓库 workflow 文件 | `8` 个 |
| GitHub Actions workflow | `10` 个：8 个仓库 workflow + 2 个动态 Dependabot workflow |
| Git tag | `0` 个 |

## 2. 当前工作数量盘点

### 2.1 Git 历史、引用与工作树

| 指标 | 数量 / 状态 |
|------|-------------|
| 全引用可达提交数 | `6366` |
| 台账提交前 `HEAD` 提交数 | `6195` |
| 台账提交前 `origin/main` 提交数 | `6195` |
| 台账提交前 `HEAD` 相对 `origin/main` | 领先 `0`，落后 `0` |
| 本地分支 | `46` 个 |
| 远端 head | `12` 个，不含 `origin/HEAD` 符号引用 |
| 本地与远端同名分支 | `6` 个 |
| 仅本地存在的分支 | `40` 个 |
| 仅远端存在的分支 | `6` 个 |
| Git worktree | `40` 个，其中 `3` 个为 detached 验证 worktree |
| tracked 文件 | `4792` 个 |

当前根工作区只有 `1` 个预先存在且受保护的未跟踪文件：

```text
docs/superpowers/specs/2026-07-08-sub2api-force-priority-server-design.md
```

它不得被读取、修改、暂存、提交或清理。此前台账记录的 `infra/sub2api/**` 4 个未提交文件已经进入正式提交并随 `main` 推送，不再属于未提交工作。

### 2.2 GitHub 与文档资产

| 范围 | 数量 | 说明 |
|------|------|------|
| `.github/` 文件 | `19` | PR / issue 模板、安全政策、workflow、Dependabot 配置与本台账 |
| `.github/workflows/` 文件 | `8` | 仓库中可审阅的 workflow |
| GitHub API workflow | `10` | 另含 `Dependabot Updates` 与 `Dependency Graph` 两个动态 workflow |
| `docs/` 文件 | `69` | 安装、维护、OpenAPI、superpowers 计划 / spec / handoff 等 |
| `docs/superpowers/specs/` 文件 | `22` | 包含受保护未跟踪 spec，因此比 Git tracked 数量多 1 |
| `docs/superpowers/plans/` 文件 | `27` | 实施与发布计划 |

### 2.3 本轮 GitHub 与生产同步

本轮 GPT-5.6、生产修复与 sub2api 构建工具链工作已经直接进入并推送 `main`：

```text
1bd746f5 fix: align sub2api image Go toolchain
0ebd4cee fix: align sub2api image pnpm toolchain
a812be65 fix: constrain sub2api verifier cleanup target
608bf6f9 fix: include sub2api frontend workspace config
860ebc5d fix: preserve pinned sub2api frontend lockfile
19ccf72c test: verify sub2api overlay against pinned source
36f4da52 feat: expose gpt-5.6 models in sub2api
9cc5c38e fix: isolate channel pricing overrides
ad123b7e fix: price gpt-5.6 variants accurately
aee0bcb4 feat: recognize gpt-5.6 in sub2api
dd75a825 fix: restrict gpt-5.6 completion ratios
02517192 test: lock gpt-5.6 pricing keys
9f2b5abb feat: add gpt-5.6 pricing defaults
```

远端事实：

- `refs/heads/main` → `1bd746f5774847f539c47b54e69faf88fbc757a3`
- `refs/heads/codex/sub2api-pnpm-build-fix` → 同一 SHA
- `refs/heads/codex/yunbay-production-remediation` → `a812be65db14d51939687333160a57e85d6a9ceb`
- 生产部署标记 → `1bd746f5774847f539c47b54e69faf88fbc757a3`

上述 SHA 与提交数量是本台账提交前的代码 / 生产发布快照。本台账所在的文档提交会使 GitHub `main` 再前进 1 个提交，但不会改变生产部署标记；不要把文档维护提交误当成新的生产发布 SHA。

独立仓库 `bayma888/sub2api-bmai` 本轮未同步，当前 `main` 仍是 `4d676dddd1af9571bbc79f0a7aff7aea077baf19`；当前 GitHub 登录对该仓库只有 `READ` 权限。生产 sub2api 来自本仓库 `infra/sub2api` overlay 对 pinned upstream 的构建，不依赖该 fork。

## 3. 当前开放 PR

| PR | 状态 | Head → Base | 规模 | 当前处理建议 |
|----|------|-------------|------|--------------|
| [#45](https://github.com/chenli17683185032-ai/yunbay/pull/45) | Ready / Mergeable | `dependabot/npm_and_yarn/web/web-minor-and-patch-55117c93be` → `main` | `2` files, `+4/-4` | 3 个 web minor/patch 更新；先跑 Bun install、typecheck、测试和 build |
| [#46](https://github.com/chenli17683185032-ai/yunbay/pull/46) | Ready / Mergeable | `dependabot/npm_and_yarn/web/visactor/react-vchart-2.1.2` → `main` | `1` file, `+1/-1` | `@visactor/react-vchart` major 更新；需与 #48 同批兼容性验证 |
| [#47](https://github.com/chenli17683185032-ai/yunbay/pull/47) | Ready / Mergeable | `dependabot/npm_and_yarn/web/lucide-react-1.23.0` → `main` | `1` file, `+1/-1` | major 更新；重点核对 icon API、bundle 与类型检查 |
| [#48](https://github.com/chenli17683185032-ai/yunbay/pull/48) | Ready / Mergeable | `dependabot/npm_and_yarn/web/visactor/vchart-2.1.2` → `main` | `1` file, `+1/-1` | `@visactor/vchart` major 更新；与 #46 必须锁定同一版本一起验证 |
| [#49](https://github.com/chenli17683185032-ai/yunbay/pull/49) | Ready / Mergeable | `dependabot/npm_and_yarn/web/i18next-26.3.4` → `main` | `1` file, `+1/-1` | major 更新；重点验证六语言初始化、fallback 与翻译测试 |

旧台账中的 Draft PR #1–#7 已全部不再开放，不能继续按旧队列处理。当前 5 个开放 PR 均是 Dependabot PR；`mergeable` 只说明 GitHub 当前无文本冲突，不代表依赖升级已经通过项目测试。

推荐顺序：

1. 先把 #46 与 #48 作为一组验证，避免 React wrapper 与核心图表库版本不一致。
2. 再独立验证 #47 和 #49 的 major 升级。
3. 最后评估 #45 是否与已验证升级重复或冲突。
4. 每个 PR 合并前至少执行 `bun install --frozen-lockfile`、`bun run typecheck`、相关测试与 `bun run build`；不要仅依据 Dependabot 或 GitHub mergeable 状态合并。

## 4. 分支与 worktree 脉络

### 4.1 远端 head

业务 / 维护分支：

```text
main
codex/jeepay-alipay-admin
codex/ldxp-browser-worker-auto-topup
codex/sub2api-pnpm-build-fix
codex/subscription-order-redemption-production
codex/value-package-reset-time
codex/yunbay-production-remediation
```

Dependabot 分支：

```text
dependabot/npm_and_yarn/web/i18next-26.3.4
dependabot/npm_and_yarn/web/lucide-react-1.23.0
dependabot/npm_and_yarn/web/visactor/react-vchart-2.1.2
dependabot/npm_and_yarn/web/visactor/vchart-2.1.2
dependabot/npm_and_yarn/web/web-minor-and-patch-55117c93be
```

### 4.2 本地工作树风险

当前共有 `40` 个 worktree，显著高于旧快照的分支 / 工作区规模。除根工作区外，大部分用于历史功能、灰测、冲突修复或依赖验证；其中以下 3 个是 detached 验证 worktree：

```text
.worktrees/verify-pr31-electron
.worktrees/verify-pr33-dompurify
.worktrees/verify-sub2api-frontend-main
```

维护规则：

- 不因分支已经合并到 `main` 就直接删除对应 worktree；先检查未提交文件、未推送提交、生产证据和 handoff 文档。
- 不使用 `git clean`、`reset --hard` 或批量删除 worktree 来代替审计。
- 对仅本地分支，先执行 `git log origin/main..branch`、`git status` 和 `git worktree list`，确认其内容是否已被其他提交吸收。
- 对仅远端分支，先检查开放 PR、关闭 PR与生产引用，再决定保留、归档或删除。

## 5. GitHub Actions 脉络

| Workflow | 来源 | 主要作用 |
|----------|------|----------|
| `CI` | `.github/workflows/ci.yml` | 主项目及 overlay 校验入口 |
| `Publish Docker image (Multi-arch)` | `.github/workflows/docker-build.yml` | tag / manual 多架构镜像发布 |
| `Publish Docker image (alpha)` | `.github/workflows/docker-image-alpha.yml` | alpha 镜像发布 |
| `Publish Docker image (nightly)` | `.github/workflows/docker-image-nightly.yml` | nightly 镜像发布 |
| `Build Electron App` | `.github/workflows/electron-build.yml` | Windows Electron 构建 |
| `PR Check` | `.github/workflows/pr-check.yml` | PR 元数据与基础检查，不等同完整测试 |
| `Release (Linux, macOS, Windows)` | `.github/workflows/release.yml` | 多平台 release artifacts |
| `Sync Release to Gitee` | `.github/workflows/sync-to-gitee.yml` | 手动同步 release |
| `Dependabot Updates` | GitHub 动态 workflow | Dependabot 更新执行 |
| `Dependency Graph` | GitHub 动态 workflow | 依赖图更新 |

注意：

- GitHub API 的 workflow 数量包含动态 workflow，因此为 `10`；仓库实际可编辑文件仍为 `8`。
- `CI` 已存在，但具体 job 是否因 manifest 缺失而跳过仍需查看当次 Actions 日志，不能只看 workflow 名称。
- 通过 `PR Check` 不代表后端测试、前端构建、数据库兼容或生产冒烟已经通过。
- 创建 PR 时必须使用 `.github/PULL_REQUEST_TEMPLATE.md`；若当前 Git 作者不是历史核心开发者，PR 正文需声明 AI-generated 或 AI-assisted。

## 6. 台账维护规则

- 每次更新 PR、issue、workflow、远端分支或大量 worktree 后，更新快照时间和对应计数。
- 数量必须来自命令或 GitHub API，不能沿用旧快照推断。
- 区分“代码已经 push 到 GitHub”和“GitHub 工作数量台账已经更新”；两者必须分别验证。
- 区分本仓库 overlay 与独立 sub2api fork；生产来源和仓库同步状态要明确记录。
- 不在本文件写入 GitHub token、Docker Hub token、SSH key、生产 cookie、完整账号信息或任何可复用凭据。
- 不删除或替换受保护的项目标识、上游归属、license 和安全政策信息。
