# 必读公告登录弹窗式确认设计

**日期：** 2026-06-29
**项目：** 云贝 default 前端公告展示改造
**状态：** 已完成实现并同步生产（2026-06-30）；本文保留为设计依据与生产验收补充记录
**范围：** `/Users/ethan/Documents/yunbay/web/default` 前端；不做后端 schema、接口或生产数据迁移

---

## 1. 背景

用户要求：网站公告改成类似登录弹窗的形式，公告必须点击“已读”后才能消掉。

当前 default 前端已经有通知系统：

- 顶部铃铛 popover 展示 `/api/notice` 的 Notice 和 `/api/status` 的 Announcements。
- `useNotifications()` 在打开 popover 或切换到 Announcements tab 时会自动标记已读。
- 已读状态保存在浏览器 localStorage 的 `notification-storage` 中。

这和需求冲突：用户只是打开/看到弹层，还没有明确点击“已读”，公告就可能被消掉。

---

## 2. 当前生产事实

本设计以 2026-06-29 对生产公开接口的只读核验为准，不假设本地 mock 数据能代表生产。

核验接口：

```bash
curl -fsSL https://yunbay.xyz/api/status
curl -fsSL https://yunbay.xyz/api/notice
```

生产响应要点：

```json
{
  "api_status": {
    "success": true,
    "announcements_enabled": true,
    "announcements_type": "list",
    "announcements_count": 6,
    "announcement_keys": ["content", "extra", "id", "publishDate", "type"],
    "announcement_ids": [6, 5, 4, 3, 2, 1],
    "announcement_types": ["default", "warning", "default", "default", "default", "default"]
  },
  "api_notice": {
    "success": true,
    "notice_type": "str",
    "notice_length": 14,
    "notice_sha256": "43e6bd47d346a6b379464bb5156c967b3bd9afbd238f2b89954347b2b0081752"
  }
}
```

结论：

1. 生产公告来源有两个：
   - `/api/notice`：单条字符串 Notice。
   - `/api/status`：`announcements_enabled` 加 `announcements[]` 列表。
2. 生产 Announcements 已有稳定数字 `id`，当前前端用 `id:${id}` 作为已读 key 的优先路径是正确的。
3. 生产 Notice 没有 id，只能继续用完整 trim 后内容作为已读签名；内容变更后应重新弹出。
4. 生产公告字段只有 `content`、`extra`、`id`、`publishDate`、`type`，实现不能依赖不存在的 `title`、`link`、`priority`、`force` 字段。
5. 不需要后端新增“公告已读”接口；按当前生产规模，浏览器本地已读状态足够满足“本设备上点已读后不再弹”。

---

## 3. 目标

### 3.1 用户体验目标

1. 用户进入网站后，只要存在未读 Notice 或 Announcements，就自动出现居中弹窗。
2. 弹窗视觉风格接近现有登录/系统 Dialog：居中、遮罩、圆角卡片、固定底部操作区。
3. 弹窗不能通过以下方式关闭：
   - 点击遮罩。
   - 按 Esc。
   - 点击右上角关闭按钮。
   - 打开顶部铃铛后自动清未读。
4. 用户必须点击明确按钮：`我已阅读` / `I have read`，才会写入已读状态并关闭弹窗。
5. 同一浏览器内，已读后刷新、重新登录、切页面都不再弹同一批公告。
6. 生产管理员修改 Notice 内容或新增/修改 Announcement 后，用户应再次看到弹窗。

### 3.2 工程目标

1. 兼容当前生产接口，不改接口、不改数据库、不写一次性迁移。
2. 复用现有公告读取、Markdown 渲染、颜色点、日期展示和 localStorage store。
3. 改掉当前“打开即已读”的隐式行为，改成“显式确认才已读”。
4. 保留顶部通知铃铛作为手动查看入口。
5. 加入最小测试，覆盖“未读计算”和“显式确认才标记已读”的核心逻辑。
6. 全量补齐新文案的 `en`、`zh`、`fr`、`ja`、`ru`、`vi` 翻译。

---

## 4. 非目标

本次不做：

1. 后端保存每个用户的公告已读状态。
2. 管理后台新增“强制弹窗”“仅登录用户可见”等公告配置字段。
3. 多设备已读同步。
4. 按公告类型决定是否强制弹窗。
5. classic 前端同步改造。
6. 删除或重命名任何受保护的项目标识、组织标识、版权文本或 package metadata。

---

## 5. 现有实现概览

相关文件：

```text
/Users/ethan/Documents/yunbay/web/default/src/hooks/use-notifications.ts
/Users/ethan/Documents/yunbay/web/default/src/stores/notification-store.ts
/Users/ethan/Documents/yunbay/web/default/src/components/notification-popover.tsx
/Users/ethan/Documents/yunbay/web/default/src/components/layout/components/app-header.tsx
/Users/ethan/Documents/yunbay/web/default/src/components/layout/components/public-header.tsx
/Users/ethan/Documents/yunbay/web/default/src/components/ui/dialog.tsx
/Users/ethan/Documents/yunbay/web/default/src/components/dialog.tsx
/Users/ethan/Documents/yunbay/web/default/src/lib/api.ts
/Users/ethan/Documents/yunbay/web/default/src/hooks/use-status.ts
```

当前关键行为：

- `getNotice()` 请求 `/api/notice`。
- `useStatus()` 请求 `/api/status`，并从 localStorage `status` 提供 placeholder。
- `useNotifications()` 聚合 Notice 和 Announcements。
- `notification-store.ts` 持久化：
  - `lastReadNotice`
  - `readAnnouncementKeys`
  - 目前未被使用的 `closedUntilDate`
- `handleOpenPopover()` 当前会调用 `markNoticeRead()`，导致打开通知即清 Notice 未读。
- `handleTabChange('announcements')` 当前会调用 `markAnnouncementsAsRead()`，导致切到公告 tab 即清公告未读。

---

## 6. 设计方案

### 6.1 新增必读公告 Modal

新增组件：

```text
/Users/ethan/Documents/yunbay/web/default/src/components/required-announcement-dialog.tsx
```

职责：

1. 接收 `open`、`notice`、`announcements`、`activeTab`、`onTabChange`、`loading`、`onConfirmRead`。
2. 用现有 Dialog 系统渲染居中弹窗。
3. `showCloseButton={false}`。
4. 对 `onOpenChange(false)` 进行拦截：外部关闭请求不改变状态。
5. 底部只有主按钮 `I have read`。
6. 内容区使用 tabs：Notice / Timeline。
7. 内容过长时内部滚动，不让页面主体滚动承担阅读。

弹窗打开条件由 hook 决定，不由组件自己读取接口。

### 6.2 显式已读动作

`useNotifications()` 改为暴露显式动作：

```ts
confirmRead: () => void
requiredDialogOpen: boolean
```

行为：

1. `confirmRead()`：
   - 如果当前 Notice 未读，调用 `markNoticeRead(noticeContent)`。
   - 只对当前未读 Announcement keys 调用 `markAnnouncementsRead(unreadAnnouncementKeys)`。
2. `handleOpenPopover()`：只打开 popover，不标记已读。
3. `handleTabChange()`：只切 tab，不标记已读。
4. `requiredDialogOpen`：
   - `loading === false`
   - 且 `unreadCount > 0`
   - 且存在实际可展示内容

### 6.3 未读计算

为了可测试和减少 hook 内复杂度，提取纯函数：

```text
/Users/ethan/Documents/yunbay/web/default/src/hooks/notification-model.ts
```

职责：

- `hashString(input: string): string`
- `getAnnouncementKey(item: Record<string, unknown>): string`
- `getUnreadNotificationState(...)`

`getAnnouncementKey()` 保持当前兼容策略：

1. 有 `id` 时返回 `id:${id}`。
2. 无 `id` 时基于 `publishDate`、`content`、`extra`、`type`、`title`、`link` 的 JSON fingerprint hash。

生产当前有 `id`，所以已读状态稳定；保留 hash fallback 是为了兼容后台历史导入或本地非生产数据。

### 6.4 顶部铃铛保留但不自动消未读

`NotificationPopover` 继续存在：

- 未读 badge 继续显示。
- 手动打开只查看内容。
- footer 的 `Close` 只关闭 popover，不写已读。
- 可选：把 footer 主按钮改成 `Mark all as read`，调用同一个 `confirmRead()`。这样用户在铃铛里也能显式确认。

本设计推荐保留 `Close` 并新增/替换一个明确按钮：

- 有未读：显示 `Mark all as read`，点击后写已读并关闭 popover。
- 无未读：显示 `Close`。

这样既满足“必须点已读才能消掉”，又不让用户只能等自动弹窗处理。

### 6.5 Header 接入

在以下两个 header 里接入必读公告弹窗：

```text
/Users/ethan/Documents/yunbay/web/default/src/components/layout/components/app-header.tsx
/Users/ethan/Documents/yunbay/web/default/src/components/layout/components/public-header.tsx
```

接入方式：

1. 继续使用同一个 `useNotifications()` 实例。
2. 如果 `showNotifications === true`，渲染：
   - `NotificationPopover`
   - `RequiredAnnouncementDialog`
3. 如果某个页面显式传 `showNotifications={false}`，不强行弹公告。这保留现有页面控制语义，避免登录页、特殊流程或嵌入页被公告阻断。

### 6.6 i18n

新增英文 key，按项目现有 flat JSON 约定补齐六种语言：

```text
I have read
Mark all as read
Please read these announcements before continuing.
Unread system announcements
```

已有 key 继续复用：

```text
System Announcements
Notice
Timeline
Latest platform updates and notices
Close
Loading...
No announcements at this time
No system announcements
```

需要按 `i18n-translate` 工作流运行：

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun run i18n:sync
```

---

## 7. 状态与数据流

```mermaid
flowchart TD
  A[App/Public Header mounts] --> B[useNotifications]
  B --> C[/api/notice via getNotice]
  B --> D[/api/status via useStatus]
  C --> E[noticeContent]
  D --> F[announcements list]
  E --> G[getUnreadNotificationState]
  F --> G
  H[notification-storage localStorage] --> G
  G --> I{unreadCount > 0?}
  I -- yes --> J[RequiredAnnouncementDialog open]
  I -- no --> K[No blocking dialog]
  J --> L[User clicks I have read]
  L --> M[markNoticeRead / markAnnouncementsRead]
  M --> H
  M --> G
  G --> N[unreadCount = 0]
  N --> O[Dialog closes]
```

已读本地存储仍为：

```json
{
  "state": {
    "lastReadNotice": "trim 后 Notice 全文",
    "readAnnouncementKeys": ["id:6", "id:5"],
    "closedUntilDate": null
  },
  "version": 0
}
```

`closedUntilDate` 当前没有调用方。本次不依赖它，也不需要迁移或删除它，避免影响用户已有 localStorage。

---

## 8. 边界情况

1. **接口还在 loading**：不弹，避免先用旧 placeholder 错弹后又刷新闪烁。
2. **`announcements_enabled=false`**：不展示 Announcements，只看 Notice。
3. **Notice 为空，Announcements 为空**：不弹。
4. **Notice 内容修改**：`lastReadNotice !== noticeContent`，重新弹。
5. **生产 Announcement id 不变但内容修改**：当前 key 为 `id:${id}`，不会重新弹。生产后台如果要让旧 id 的内容修改也重新通知，应新增一条 Announcement 或改变 id。此行为沿用当前前端已读设计，避免把既有生产用户全部重复打扰。
6. **Announcement 无 id**：hash fallback 会让内容变化后重新弹。
7. **多个 header 不应同时存在**：正常路由只会使用 app header 或 public header。若未来同页同时挂载两个 header，可能出现两个 hook 实例；届时应上移为全局 provider。本次不为未出现的结构增加 provider。
8. **不同设备**：localStorage 不同步，同一用户在另一设备仍需点已读。本次设计接受这一点。
9. **用户清理浏览器数据**：会重新弹，符合本地状态设计。
10. **管理员在生产临时关闭公告**：前端随 `/api/status` 更新后不再弹 Announcements；Notice 仍由 `/api/notice` 控制。

---

## 9. 测试策略

### 9.1 纯函数测试

新增测试：

```text
/Users/ethan/Documents/yunbay/web/default/src/hooks/notification-model.test.ts
```

覆盖：

1. 有 id 的 Announcement key 为 `id:<id>`。
2. 无 id 时内容变化导致 hash key 变化。
3. Notice 内容和 `lastReadNotice` 相等时不未读。
4. Notice 内容不同且非空时未读。
5. 生产形态 announcements `[id:6..1]` 能得出 6 个未读 keys。
6. 一部分 announcement keys 已读时，只返回剩余未读 keys。

### 9.2 源码行为测试

由于当前项目没有 React DOM 测试栈，沿用已有 source test 风格，新增或扩展源码断言：

```text
/Users/ethan/Documents/yunbay/web/default/src/hooks/use-notifications-source.test.ts
/Users/ethan/Documents/yunbay/web/default/src/components/required-announcement-dialog-source.test.ts
```

覆盖：

1. `handleOpenPopover` 不调用 `markNoticeRead` 或 `markAnnouncementsRead`。
2. `handleTabChange` 不调用 `markAnnouncementsRead`。
3. `confirmRead` 调用 `markNoticeRead` 和 `markAnnouncementsRead`。
4. 必读弹窗 `showCloseButton={false}`。
5. 必读弹窗忽略 `onOpenChange(false)`。
6. 必读弹窗包含 `I have read` 按钮。

### 9.3 构建验证

执行：

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun test src/hooks/notification-model.test.ts src/hooks/use-notifications-source.test.ts src/components/required-announcement-dialog-source.test.ts
bun run i18n:sync
bun run typecheck
bun run build
```

如果 `bun test` 对现有 `node:test` 风格有兼容问题，改用项目已有测试运行方式前必须先记录失败输出，再做最小调整；不能跳过验证。

---

## 10. 生产上线与回滚

### 10.1 上线前确认

上线前再次只读确认生产接口形态：

```bash
curl -fsSL https://yunbay.xyz/api/status
curl -fsSL https://yunbay.xyz/api/notice
```

确认：

- `announcements_enabled` 仍为 boolean。
- `announcements` 仍为数组。
- 每条公告至少有 `content`，生产当前有 `id`。
- `/api/notice` 的 `data` 仍为 string。

### 10.2 上线后冒烟

浏览器验证：

1. 清除 localStorage 的 `notification-storage`。
2. 打开 `https://yunbay.xyz/`。
3. 看到居中必读公告弹窗。
4. 点击遮罩，弹窗不关闭。
5. 按 Esc，弹窗不关闭。
6. 点击顶部铃铛不应导致公告变已读。
7. 点击 `我已阅读`，弹窗关闭。
8. 刷新页面，不再弹同一批公告。
9. 修改后台 Notice 或新增 Announcement 后，重新出现弹窗。

### 10.3 回滚

本次仅前端变更。若上线后发现阻断性问题：

1. 回滚前端静态构建到上一版本。
2. 不需要数据库回滚。
3. 不需要清理用户 localStorage；旧版本仍兼容已有 `notification-storage` 字段。

---

## 11. 验收标准

1. 生产现有 1 条 Notice 和 6 条 Announcements 在首次访问时能触发必读弹窗。
2. 弹窗必须点击 `我已阅读` 才会关闭。
3. 打开通知铃铛或切换 Notice/Timeline tab 不会自动清未读。
4. 点击 `我已阅读` 后，`notification-storage.state.lastReadNotice` 和 `readAnnouncementKeys` 被写入。
5. 刷新后同一批公告不再弹。
6. 新增生产 Announcement 后对应 `id:<new id>` 未读，弹窗重新出现。
7. `/api/status` 和 `/api/notice` 接口无需改动。
8. `bun run typecheck` 和 `bun run build` 通过。
9. 新增 i18n key 在 `en`、`zh`、`fr`、`ja`、`ru`、`vi` 中全部存在。
10. 不修改、删除、替换任何受保护的项目/组织标识和版权头。

---

## 12. 生产完成记录（2026-06-30）

本功能已在 2026-06-30 随全量上线批次同步到生产环境。此前“等待实现”的状态已被本节替换为生产完成事实；后续若公告接口或弹窗行为再次调整，只能追加新记录，不要改写本节历史。

### 12.1 代码与功能基线

关键实现提交：

```text
59756734 feat: require explicit announcement read confirmation
```

该提交位于全量上线基线分支：

```text
codex/full-rollout-no-overlap-clean
```

关键实现文件包括：

```text
web/default/src/components/required-announcement-dialog.tsx
web/default/src/components/required-announcement-dialog-source.test.ts
web/default/src/hooks/notification-model.ts
web/default/src/hooks/notification-model.test.ts
web/default/src/hooks/use-notifications.ts
web/default/src/hooks/use-notifications-source.test.ts
web/default/src/components/notification-popover.tsx
web/default/src/components/notification-popover-source.test.ts
web/default/src/components/layout/components/app-header.tsx
web/default/src/components/layout/components/public-header.tsx
web/default/src/i18n/locales/{en,zh,fr,ja,ru,vi}.json
```

生产行为基线：

- 未读 Notice / Announcements 会触发登录弹窗式必读公告；
- 用户必须点击“我已阅读”/`I have read` 才写入已读状态；
- 打开通知 popover 或切换公告 tab 不自动清未读；
- 不新增后端接口、不修改公告数据库 schema；
- 已读状态继续使用浏览器 localStorage 的 `notification-storage`。

### 12.2 公开侧生产复核

2026-06-30 公开侧复核结果：

```text
https://yunbay.xyz/            HTTP 200
https://yunbay.xyz/api/status  HTTP 200
https://yunbay.xyz/api/notice  HTTP 200
生产入口 JS: /static/js/index.599262f2f0.js
```

`/api/status` 当前公告形态：

```text
success=true
announcements_enabled=true
announcements_type=list
announcements_count=6
announcement_keys=content,extra,id,publishDate,type
```

生产入口 JS 已包含以下必读公告相关标记：

```text
I have read
我已阅读
notification-storage
markNoticeRead
markAnnouncementsRead
```

### 12.3 后续维护要求

- 后续公告行为变更必须保留“显式点击已读才清未读”的产品语义，除非另有明确需求；
- 如果改动公告接口字段，必须同步更新本设计中的生产接口事实；
- 如果生产临时关闭公告，应追加记录关闭原因、时间、回滚方式和恢复验证结果；
- 不要把后台 cookie、session、access token、管理员账号细节写入本文。
