# 云贝生产服务器垃圾清理计划

**日期：** 2026-07-17
**状态：** 已完成
**范围：** 仅清理云贝生产服务器上可明确再生、未被运行服务使用的 Docker 构建缓存、悬空对象和确认过期的临时资源；不修改业务代码、配置、数据库、用户数据、密钥、持久卷或当前/必要回滚镜像。

## 1. 目标与性能指标

本轮目标是在不中断服务的前提下回收明显无效的磁盘占用，并建立可重复的“盘点 -> 小步清理 -> 反馈验证 -> 决定是否继续”闭环。

验收指标：

1. 根分区从基线 `85 GiB / 145 GiB（59%）` 至少回收 20 GiB，或把 Docker 可回收构建缓存降到 10 GiB 以下；若第一阶段已达到该目标，不为追求更大数字扩大删除范围。
2. `yunbay-new-api`、Caddy、PostgreSQL、Redis、Sub2API、CLI Proxy、LDXP worker/proxy、Grok API/egress/PostgreSQL/Redis 在清理前后均保持原容器身份、运行状态和 restart count，不执行服务重启。
3. 宿主机 `http://127.0.0.1:3000/api/status` 与公网 `https://yunbay.xyz/api/status` 在每个清理节点均为 HTTP 200；最终连续检查至少 5 轮。
4. 不执行 `docker system prune -a --volumes`、`docker volume prune`、全栈 Compose 重建、日志文件直接截断、数据库清理或备份目录批量删除。
5. 保留所有运行容器所用镜像、当前生产标签和至少一个已确认的直接回滚点；本轮默认不删任何带标签的回滚镜像。
6. 清理过程中没有无界等待。单条命令必须自行结束；若 Docker daemon、HTTP 探针或系统负载异常，立即停止后续清理并进入验证。

## 2. 控制系统抽象

- **对象：** 生产宿主机根分区、Docker image/build cache/container/volume 存储和服务运行态。
- **控制器：** 以年龄、引用关系、资源类型和最小回收目标为约束的分阶段清理策略。
- **执行器：** Docker 原生 `builder prune`、`image prune`、经逐项确认的 stopped container 删除，以及本任务产生的临时文件删除。
- **测量：** `df -hT`、`df -i`、`docker system df`、容器 ID/状态/restart count、内外 HTTP 状态、系统 load/memory 和严重日志。
- **环境与扰动：** 正在进行的镜像构建、并发部署、浏览器 worker 高负载、Docker GC I/O、共享镜像层、SSH 断开和生产请求波动。
- **稳定性优先：** 先清理完全可再生的构建缓存；只有反馈证明回收不足且目标对象明确时才进入下一阶段。数据库卷、业务备份和回滚资产不作为第一轮清理对象。

## 3. 只读基线

- 根分区：`145 GiB`，已用 `85 GiB`，可用 `60 GiB`，使用率 `59%`；inode 使用率 `9%`。
- 内存：`7.8 GiB`，available 约 `2.7 GiB`，无 swap。
- Docker images：`70.45 GiB`，其中 `9.319 GiB` 标记为可回收。
- Docker build cache：`55.15 GiB`，Docker 汇总显示约 `31.78 GiB` 可实际回收；共 317 条缓存记录。
- Docker containers：19 个，其中 12 个运行、7 个为 2 至 3 天前退出的旧 Grok 栈容器；退出容器可回收写层仅约 `2.257 MiB`。
- Docker volumes：14 个，仅约 `760.4 MiB` 显示未使用；因卷可能含状态，本轮只盘点不自动删除。
- `/opt/new-api`：约 `9.9 GiB`，其中 backups `8.0 GiB`、releases `1.2 GiB`、logs `411 MiB`、app `236 MiB`；这些默认按业务/审计资产保护。
- `/home/deploy`：约 `3.5 GiB`，其中 `grokcli-2api` `3.2 GiB`、grok backups `208 MiB`；默认不删。
- `/var/log`：约 `1.1 GiB`；当前 deploy 用户无免密 sudo，journal/apt 系统级清理不通过权限绕过实施。
- 当前宿主机与公网 `/api/status` 均为 HTTP 200；systemd failed units 为 0。
- 盘点时 load average 较高，主要来自正在运行的浏览器/自动化进程；清理期间需额外观察，不并行执行镜像构建或部署。

## 4. GitHub 经验与设计依据

实施前已核对以下公开项目的清理实践：

1. Docker 官方文档将 build cache 作为独立资源清理，并明确 builder 之间各自维护缓存：<https://github.com/docker/docs/blob/main/content/manuals/engine/manage-resources/pruning.md>。
2. `jeong-sik/masc` 的生产清理脚本默认 dry-run，以 24 小时年龄过滤清理 dangling images、stopped containers 和 builder cache，同时只报告 volume、不自动删除：<https://github.com/jeong-sik/masc/blob/main/scripts/docker-prune.sh>。
3. `Orderlee/pipeline` 先记录 `df` 与 `docker system df`，用年龄和保留容量限制 builder prune，并明确排除 volumes：<https://github.com/Orderlee/pipeline/blob/main/scripts/docker_prune.sh>。
4. `latentwill/xvision` 在部署前设置磁盘阈值，建议先清 dangling image，再人工识别旧 deploy tag，而不是直接删除全部 tagged images：<https://github.com/latentwill/xvision/blob/main/scripts/deploy-image.sh>。

本轮据此采用：先测量、按年龄过滤、缓存优先、卷永不自动删、带标签镜像人工判定、每阶段闭环验证。

## 5. 实施步骤

### 阶段 A：互斥与反馈基线

1. 再次确认没有 `docker build`、`docker compose`、rsync、部署 watchdog 或有效部署锁；若存在并发变更，停止清理。
2. 记录所有运行容器的 ID、镜像 ID、状态、restart count 和启动时间，形成清理前快照。
3. 记录根盘、Docker 占用、load/memory、宿主机与公网健康探针。
4. 检查 Docker builder prune 支持的年龄/保留容量参数，并列出将受影响的缓存范围。

### 阶段 B：最小缓存清理

1. 仅删除超过 24 小时未使用的 Docker builder cache；保留最近 24 小时缓存，避免影响刚完成的生产构建与短期回滚构建效率。
2. 命令完成后立即记录实际回收量、根盘使用率和 `docker system df`。
3. 核对运行容器身份未变化，执行宿主机与公网健康探针。
4. 若已回收至少 20 GiB或 build cache 可回收量已低于 10 GiB，停止扩大磁盘清理范围。
5. 首轮反馈若未达标，只在系统 load 回落且完整健康复核通过后执行第二次有界缓存清理：年龄阈值收紧到 12 小时，同时保留至少 `8 GiB` builder cache；最近 12 小时构建缓存、全部镜像标签和运行资源继续保留。

### 阶段 C：低风险对象清理（仅在阶段 B 不足时）

1. 清理超过 24 小时的 dangling images；不加 `-a`，不触碰任何 tagged image。
2. 逐个检查 7 个退出容器的镜像、挂载与 Compose 归属。仅删除已由新 Grok 栈取代、没有独有持久数据且超过 48 小时的 stopped containers。
3. 容器删除后如旧网络已无 endpoint，仅删除确认属于废弃旧 Grok 栈的空网络。
4. volumes 继续只报告不删除；带标签的 candidate/release/rollback images 继续保留，除非另有逐项证据和新的计划记录。
5. 每个动作后重复磁盘、容器和 HTTP 反馈验证；任何异常立即停止。

### 阶段 D：最终验收与记录

1. 对比清理前后根盘、Docker images/build cache/containers/volumes。
2. 连续至少 5 轮检查宿主机与公网 `/api/status`，复核所有运行容器健康、ID、restart count 和严重日志。
3. 更新本计划实施记录、仓库统一运维手册和本地唯一服务器连接手册；不创建第二份连接/运维说明。
4. 删除本任务产生的临时清单或探针文件，不删除用户已有本地文件。
5. 只提交本任务自己的文档变更并推送 GitHub `main`；不得暂存或覆盖工作区原有修改和未跟踪文件。

## 6. 停止与回滚

缓存与 dangling 对象删除不可原位恢复，但不会改变运行容器；因此控制重点是限制删除范围，而不是事后恢复。

立即停止条件：

1. 任一生产健康探针非 200，或关键容器 ID/状态/restart count 发生非预期变化。
2. Docker daemon 响应异常、清理命令无界运行、根盘 I/O 导致服务延迟明显上升，或系统可用内存持续不足。
3. 发现并发构建、部署、镜像重标记或需要使用待清理缓存的运维任务。
4. 待删对象与数据库卷、用户数据、secrets、当前镜像、直接回滚点或业务备份存在任何不确定关联。

若只发生构建缓存缺失，后续构建按需重新生成；若服务健康异常，不继续清理，先保存现场、检查 Docker/应用日志并按现有生产手册恢复服务。本轮禁止通过重启数据库、Caddy 或全栈 Compose 作为“清理后的验证手段”。

## 7. 验收偏差说明

- 原定磁盘目标“回收 20 GiB 或可回收缓存低于 10 GB”未严格命中；实测为回收 `17.83 GiB`、剩余可回收缓存 `12.6 GB`。
- 继续命中该数字需要越过已验证的 12 小时保护窗，删除当天构建缓存；此时根盘已经降到 `47%`、可用约 `78 GiB`，继续删除的收益低于构建恢复成本。
- 按本计划既定的稳定性优先和最小实施原则，采用更高优先级的停止条件：根盘低于 `50%`、累计回收超过 `17 GiB`、剩余对象全部处于保护边界且完整健康反馈通过。该偏差已明确记录，不把接近目标冒充为命中原数值目标。

## 8. 实施记录

- [x] 读取唯一服务器连接手册并验证固定 SSH 路径。
- [x] 完成服务器、Docker、业务目录和服务健康的只读基线盘点。
- [x] 核对 GitHub 上 Docker 官方文档及三个公开项目的清理实践。
- [x] 确认最小充分方案为“24 小时年龄过滤的 builder cache 优先，反馈达标即停止”。
- [x] 完成阶段 A 的清理前运行态快照与参数核对：部署锁空闲、无并发 mutation 进程；12 个运行容器均 healthy/restart=0，连续 3 轮内外探针为 200；Docker 支持 `until`、`reserved-space` 和 builder status timeout。
- [x] 完成阶段 B：首轮 24 小时年龄过滤回收 `11,054,096,384` bytes（Docker 报告 `11.08 GB`）；负载回落到 `3.24` 且完整健康复核通过后，第二轮 12 小时年龄过滤再回收 `8,093,089,792` bytes（Docker 报告 `8.093 GB`）。两轮累计实际释放 `19,147,186,176` bytes（约 `17.83 GiB` / `19.17 GB`），根盘 `59% -> 47%`，builder cache `55.15 GB -> 35.97 GB`、可回收量 `31.78 GB -> 12.6 GB`。
- [x] 根据反馈主动不执行阶段 C：最终仅 2 个约 5 小时的 dangling images，仍在 12 小时保护窗；7 个退出容器属于带挂载的旧 Grok 回滚栈，空旧网络与未使用 volumes 均保留。未删除 tagged image、container、network、volume、backup、release、log 或业务文件。
- [x] 完成最终健康、容器不变性与磁盘验收：12 个运行容器 ID、镜像 ID、启动时间和 restart=0 均与基线一致；最终 load 1 分钟值 `1.44`，systemd failed units 为 0，连续 5 轮源站 `/api/status`、公网 `/api/status` 和首页全部 200，五个关键容器严重日志计数均为 0。
- [x] 更新仓库及本地唯一运维记录；本任务没有在服务器创建待清理的临时文件、候选容器或新镜像。
- [x] 本任务计划与脱敏运维记录已提交并普通推送 GitHub `main@535933bb`；暂存和提交均未包含工作区原有修改与未跟踪文件。
