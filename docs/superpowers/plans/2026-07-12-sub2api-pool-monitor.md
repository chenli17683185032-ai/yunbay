# Sub2API 号池监控实施计划

日期：2026-07-12

## 实施步骤

1. **建立分支与基线**
   - 在 GitHub 和本地创建 `codex/sub2api-pool-monitor`。
   - 保留现有无关未跟踪文件，不纳入本任务提交。

2. **确认运行接口**
   - 验证 Sub2API 登录、账号列表、实时账号可用性、渠道监测和主动额度接口。
   - 验证 new-api SMTP 设置仍保存在 PostgreSQL `options` 表，并继续使用现有 Resend SMTP 链路。

3. **实现监控程序**
   - 新增 `deploy/sub2api-monitor/sub2api_pool_monitor.py`。
   - 动态读取容器环境变量和 new-api SMTP options。
   - 计算号池、模型和额度指标，构建告警/恢复邮件。
   - 添加锁、超时、原子状态持久化和敏感信息保护。

4. **实现安装与调度**
   - 新增无交互安装脚本，将程序安装到 `/opt/new-api/monitor/sub2api-pool-monitor/`。
   - 用 deploy 用户 crontab 每 5 分钟运行，避免依赖当前未启用 lingering 的 user systemd。
   - 安装过程使用标记区块幂等更新 crontab，不能覆盖其他任务。

5. **验证**
   - 本地运行 Python 单元测试和语法检查。
   - 生产先执行 `--dry-run`，核对指标与告警原因。
   - 再执行一次 `--test-email`，验证 SMTP 链路。
   - 安装定时任务并手动执行一次正式检查。
   - 确认 7 个生产容器仍在运行，主要健康检查仍为 healthy。

6. **留痕与发布**
   - 在 `docs/yunbay-maintenance.md` 的同一文件追加本次部署记录。
   - 在本地桌面云贝唯一服务器连接/维护手册追加简明运维记录。
   - 只暂存本任务文件，提交并推送 `codex/sub2api-pool-monitor`。

## 回滚

1. 从 deploy 用户 crontab 删除 `YUNBAY SUB2API POOL MONITOR` 标记区块。
2. 删除 `/opt/new-api/monitor/sub2api-pool-monitor/`。
3. 保留或删除状态/日志由运维人员决定；不需要重启任何业务容器。
