# 云贝控制台与 Codex 一键配置设计

日期：2026-06-25

## 背景

当前云贝网站已经完成普通用户快速启动流和 Yunbay Codex 的 macOS 下载接入，但还存在三类体验不一致问题：

1. 控制台顶部导航、侧边栏和概览页仍保留较多原始后台视角入口。
2. 快速启动第 5 页仍以“下载 Codex”为主题，Windows 按钮仍指向 Microsoft Store，而不是云贝自己的 Yunbay Codex 安装包。
3. 生产维护文档虽然已经记录了 2026-06-24 的快速启动改造基线，但尚未覆盖本次控制台收敛、Windows 安装包来源和新的生产同步验收点。

本轮目标是在不触碰后端权限逻辑和受保护项目信息的前提下，完成一轮聚焦型前端定制，并把 Windows 安装包纳入云贝站点自己的静态下载目录。

## 设计目标

### 1. 顶部导航收敛

顶部导航最终只显示：

- `Home` -> `/`
- `Console` -> `/dashboard`
- `Model Square` -> `/pricing`

不显示：

- `Rankings`
- `Docs`
- `About`

实现上仍然尊重后端 `HeaderNavModules` 对 `home`、`console`、`pricing` 的启停控制，但前端对 `rankings/docs/about` 进行额外过滤，不再渲染。

### 2. 控制台左侧侧边栏调整

普通用户侧边栏改为：

```text
Getting Started
- Quick Start

AI Usage
- Playground

Data
- Dashboard
- Usage Logs

API
- API Keys

Wallet
- Wallet / Top up
- Redeem codes

Account
- Profile
```

关键约束：

- 新增普通用户 `Dashboard -> /dashboard/models` 入口。
- 隐藏普通用户聊天小组件，也就是移除 `type: 'chat-presets'` 项。
- 不新增后端数据权限逻辑；普通用户仍只通过现有接口查看自己可见的数据。

管理员侧边栏保留现有管理能力，但同样隐藏聊天小组件；原 `Chat` 分组改为 `AI Usage`，仅保留 `Playground`。

### 3. 控制台概览页收敛

`/dashboard/overview` 的顶部 setup guide / recommended actions 区域继续保留；底部卡片区域只保留公告卡片：

- 保留 `AnnouncementsPanel`
- 隐藏 `SummaryCards`
- 隐藏 `ApiInfoPanel`
- 隐藏 `UptimePanel`
- 隐藏 `FAQPanel`
- 隐藏 `PerformanceHealthPanel`

本轮只修改组合逻辑，不删除已有组件文件。

### 4. 快速启动第 5 页改为 Codex 一键配置

第 5 页主题从“Download Codex”调整为：

- eyebrow：`Codex 一键启动软件`
- title：`Codex 一键配置`
- description：`下载 Codex 一键启动软件，并连接到你的云贝 API Key。`

下载按钮目标：

- macOS：继续使用站内已有 Yunbay Codex macOS zip
- Windows：切换为云贝站内托管的 Yunbay Codex Windows NSIS 安装包

按钮文案统一为：

- `Download one-click launcher`

### 5. Windows 下载来源与说明块

Windows 下载正式来源固定为 GitHub Actions 产物，而不是仓库树中的二进制文件：

- 仓库：`chenli17683185032-ai/yunbay-codex`
- Workflow：`Build Windows`
- Run ID：`28121301588`
- Artifact：`yunbay-codex-windows`
- 默认文件：`nsis/Yunbay Codex_0.1.0_x64-setup.exe`
- SHA256：`f5121184b0496cd978eb32f97d1def4a2dc7cbb2cc997189ee428fcd8c9fc5da`

该 `.exe` 下载后固定复制到：

```text
/Users/ethan/Documents/yunbay/web/default/public/downloads/yunbay-codex-windows-20260625-030300-f5121184b049.exe
```

Windows 卡片新增与 macOS 风格一致的说明块，内容包括：

- 该安装包用于 Yunbay Codex 的一键启动
- 用户在快速启动页粘贴 Yunbay API Key
- 自动写入自定 API 配置并连接 `https://yunbay.xyz/v1`
- 支持模型供应商管理、连接测试、余额/用量查询与 Codex 会话管理

### 6. 测试与验证

实现必须采用 TDD 思路，优先补充和更新这些前端测试：

- `web/default/src/features/quick-start/quick-start-data.test.ts`
- `web/default/src/hooks/sidebar-data-model.test.ts`
- 新增 `web/default/src/hooks/top-nav-link-policy.test.ts`

验证范围包括：

- Windows 下载链接已经切换为 `/downloads/...exe`
- 侧边栏不再渲染 `chat-presets`
- 顶部导航只剩 `Home / Console / Model Square`
- 概览页底部只剩公告卡片
- `bun run typecheck` 与 `bun run build` 通过
- `dist/downloads` 中的 Windows `.exe` hash 与源文件一致

## 非目标

本轮明确不做：

1. **不做翻译专项**：不新增整套 locale 覆盖要求，不把文案适配扩展成多语言治理任务。
2. **不修改后端权限逻辑**：普通用户数据隔离继续依赖现有接口和角色控制。
3. **不读取或输出无关密钥**：部署前不扫描桌面“云贝”文件夹；部署阶段只读取必要连接配置，且不在日志和回复中回显 secret。
4. **不删除受保护信息**：不得删除或替换 `new-api` / `QuantumNous` 相关标识、README、LICENSE、copyright header、module path 等。

## 涉及文件

### 必改

- `/Users/ethan/Documents/yunbay/web/default/src/features/quick-start/quick-start-data.ts`
- `/Users/ethan/Documents/yunbay/web/default/src/features/quick-start/index.tsx`
- `/Users/ethan/Documents/yunbay/web/default/src/features/quick-start/quick-start-data.test.ts`
- `/Users/ethan/Documents/yunbay/web/default/src/hooks/sidebar-data-model.ts`
- `/Users/ethan/Documents/yunbay/web/default/src/hooks/sidebar-data-model.test.ts`
- `/Users/ethan/Documents/yunbay/web/default/src/hooks/use-top-nav-links.ts`
- `/Users/ethan/Documents/yunbay/web/default/src/features/dashboard/components/overview/overview-dashboard.tsx`
- `/Users/ethan/Documents/yunbay/web/default/public/downloads/yunbay-codex-windows-20260625-030300-f5121184b049.exe`
- `/Users/ethan/Documents/yunbay/docs/yunbay-maintenance.md`

### 推荐新增

- `/Users/ethan/Documents/yunbay/web/default/src/hooks/top-nav-link-policy.ts`
- `/Users/ethan/Documents/yunbay/web/default/src/hooks/top-nav-link-policy.test.ts`

### 可能最小适配

- `/Users/ethan/Documents/yunbay/web/default/src/features/quick-start/quick-start-locales.test.ts`

## 验收标准

1. 顶部导航只显示 `Home / Console / Model Square`。
2. 普通用户侧边栏存在 `/dashboard/models` 入口，且普通用户和管理员都不再显示聊天小组件。
3. `/dashboard/overview` 底部只显示公告卡片。
4. 快速启动第 5 页主题变为“Codex 一键配置”，macOS 与 Windows 两个按钮都显示“下载一键启动软件”。
5. Windows 按钮下载本站静态文件 `/downloads/yunbay-codex-windows-20260625-030300-f5121184b049.exe`，不再跳转 Microsoft Store。
6. 构建后 `web/default/dist/downloads/yunbay-codex-windows-20260625-030300-f5121184b049.exe` 存在，且 SHA256 与源 artifact 一致。
7. 本地维护文档补充本次变更、验证命令、生产同步步骤和冒烟检查结果。
