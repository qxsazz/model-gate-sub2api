# v0.2.21 + 群聊 Bot 网站侧交付

2026-11-11，测试环境已部署并完成 HTTPS 验收。按要求直接推送 staging，没有创建 PR，没有更新生产。本文件是部署完成后的交付记录，不改变已验收的运行代码。

## 环境和版本

| 项目 | 已核验结果 |
|---|---|
| 测试地址 | https://staging.model-gate.cc |
| 绑定管理 | https://staging.model-gate.cc/chat-bindings（需登录） |
| 测试版本 | `0.2.21-mg.1`，Go `1.27.4` |
| staging 运行源码 | `7b5d4b524fa010b4111e2395d4c28bcac31deace` |
| 不可变镜像 | `ghcr.io/qxsazz/model-gate-sub2api@sha256:6a11824e3a98e7eb698efd18e2b015dc0b5819c11b127c3dc3aaffd1542b59db` |
| 官方版本 | `v0.2.21`，`10ba8f9b4dc2e6023d4a9879ab77b21841babd10` |
| 正式上游合并提交 | `b0c838a454ff7d983346ed5df4eb4d1efbf45138` |
| 生产版本 | `0.2.20-mg.1`，生产 `/health` 仍为 HTTP 200 |
| 生产分支基线 | `59153037b9ce4804a041ab086e64abd5750c1d49` |

[Deploy Staging 43047411664](https://github.com/qxsazz/model-gate-sub2api/actions/runs/43047411664) 全部成功。应用更新后公开版本接口、健康接口和绑定页面均可访问，未携带专用凭据的 Bot 请求返回 HTTP 401。

## 验证证据

- [后端 CI 43044974708](https://github.com/qxsazz/model-gate-sub2api/actions/runs/43044974708)：全量单元/集成测试、golangci-lint、部署脚本检查通过。
- [前端 CI 43044974743](https://github.com/qxsazz/model-gate-sub2api/actions/runs/43044974743)：413 个测试文件、3264 个测试通过；类型检查、构建、画布检查通过。lint 沿用项目已有的 5 个 warning，无 error。
- [安全扫描 43044974725](https://github.com/qxsazz/model-gate-sub2api/actions/runs/43044974725)：后端、前端、画布三项通过。Go 已从 1.27.3 更新至 1.27.4，修复 `GO-2026-6859`。
- 本地真实 PostgreSQL：空库初始化、从旧版 319 个 SQL 迁移升级、完整 checksum、错误身份/过期/限次/越权解绑、充值退款、零奖励、上海日期、旧事件重绑等验证通过。18 个网站/QQ/微信并发签到请求只产生一条记录和一次奖励。
- 浏览器生产构建：桌面、手机、深色模式共 7 项验证通过，登录返回绑定链接、明确确认后发码、解绑确认均正常；没有未捕获 JS 错误或本地静态资源加载失败。新页面使用现有 MG 金色主题变量。
- 线上迁移：全部 **322** 个完整文件名与 checksum 匹配；原有 **319** 个迁移文件未改动。本环境无需启用历史 checksum 兼容项。实际记录在测试服务器 `/opt/sub2api-staging/.bot-migration-receipt.json`。
- 线上现有充值门槛回归：`minimum_cny=10`，合成流程通过并回滚。
- 线上 Bot HTTPS：专用凭据与 scope、QQ/微信双向绑定、跨标签页账号切换保护、资格、重复消息、越权解绑、解绑重绑与上海日期全部通过。合成账号、订单、回执和签到预算占用均已清理。

## 本地 AstrBot 对接

完整字段、端点、错误格式、示例和重试规则见 [接入协议](bot-integration.md)。必须从可信平台事件读取 `platform + bot_id + group_id + sender_id`，QQ 使用群 `member_openid`，微信使用群 `threadId` 和成员 `senderId`。

需要特别保留的协议约束：

1. 网站预览返回 `account_id` 和 `account_confirmation`。页面核对显示账号后，authorize 提交 `confirm:true` 与原 `account_confirmation`；用户 ID 始终取 JWT。
2. 确认码为 16 位 Base32 字符，页面以 `ABCD-EFGH-IJKL-MNOP` 排版；发送 `/绑定 CODE` 的事件必须来自同一 Bot、群和成员。
3. 签到使用平台原始 `event_id` 与带时区的 RFC3339 `event_at`。重试复用原请求，不重新生成 ID 或时间；收到业务回执后停止自动重试。
4. 签到状态在 `data.status`。错误码先取 `reason`，缺省时取字符串 `code`，以兼容项目既有鉴权/限流中间件。HTTP 5xx/网络错误可有限退避重试原请求；429 遵守 `Retry-After`。
5. 解绑、重绑、充值或退款后，旧事件仍返回原回执；重新判断资格必须发送新的群消息。群回复不展示网站账号、邮箱、余额或奖励金额。

测试专用凭据由测试服务器生成，保存在 `/opt/sub2api-staging/.bot-smoke-token`（0600）；只允许 `mg-synthetic-qq`、`mg-synthetic-wx` 两个实例和 `mg-synthetic-group`。配置快照是同目录 `.bot-clients.json`（0600）。原始凭据未写入仓库、日志或文档。GitHub 当前安装授权不能写入环境 Secrets，因此本次通过服务器安全配置完成接入。

本地团队可通过已有的服务器管理通道读取合成凭据并注入本地 Bot 安全配置。真实群需要另生成专用客户端，明确允许实际 Bot/群 ID，保留合成验收客户端；使用 `configure-staging-bot.py` 更新合并后的客户端数组，再按现有部署流程重建测试应用容器以加载配置。不要直接覆盖现有 scope，也不要复用测试凭据到生产。

本地 AstrBot 的 QQ/微信适配器、真实群消息映射与真实群验收仍由本地端完成。本次没有声称已连接真实 QQ/微信群；生产晋级继续等待真实群验收及后续生产发布指令。
