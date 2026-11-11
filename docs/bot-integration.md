# ModelGate v0.2.21 群聊接入协议

此文档交付网站侧实现。AstrBot 的 QQ/微信命令解析、真实群事件映射及真实群验收由本地端完成。测试环境为 `https://staging.model-gate.cc`；生产晋级仍需 PR 审核和真实 Bot 验收。

## 版本与保留范围

- 原生产：`59153037b9ce4804a041ab086e64abd5750c1d49`，`0.2.20-mg.1`。
- 官方源码：`v0.2.21`，`10ba8f9b4dc2e6023d4a9879ab77b21841babd10`。
- 正式合并提交：`b0c838a454ff7d983346ed5df4eb4d1efbf45138`，保留真实上游父提交。
- 新版本：`0.2.21-mg.1`。Go 更新至 `1.27.4`，修复扫描发现的 `GO-2026-6859`。
- MG 主题、画布、VIP、成就、充值退款、网关恢复与 Gemini thinking/signature 定制保留。新增页面复用现有 AppLayout、card、button 与主题色变量；个人资料增加群聊绑定入口。
- 最终部署 SHA、镜像 digest、CI 与验收记录见本次发布交付记录。

## 身份和鉴权

身份键严格为 `platform + bot_id + group_id + sender_id`，所有字段必须来自可信适配器事件；不能从命令文本中取值。平台只能是 `qq_official` 或 `wechat_native`。QQ 使用群事件的 `member_openid`；微信使用当前 Bot 实例、群 `threadId` 和成员 `senderId`。私聊不允许借用群身份。

Bot 请求使用 `Authorization: Bearer <专用凭据>`，凭据前缀 `mgbot_`。网站 JWT、管理员 JWT、用户 API Key 均不能用于 Bot 接口。每个客户端配置允许的平台、Bot ID、群 ID；不支持通配符。Bot 凭据不能访问用户/管理员接口。服务端只保存凭据 SHA-256。

环境变量：

| 变量 | 含义 |
|---|---|
| `BOT_INTEGRATION_ENABLED` | 默认 `false`；显式开启才接受 Bot 请求 |
| `BOT_INTEGRATION_PUBLIC_URL` | 网站 HTTPS origin，用于返回绑定链接，不能带路径或凭据 |
| `BOT_INTEGRATION_CLIENTS_JSON` | 客户端数组；每项 `id`、`token_sha256`、`scopes` |

客户端示例结构（哈希为占位，不能直接用于部署）：

```json
[{"id":"local-astrbot","token_sha256":"<64位SHA256>","scopes":[{"platform":"qq_official","bot_id":"<实际实例ID>","group_ids":["<实际群ID>"]}]}]
```

`tools/create_bot_credential.py` 将随机 256 位服务凭据写入权限 0600 的 `bot-token`，配置写入 `clients.json`；输出目录权限 0700，拒绝覆盖。将原始凭据通过本地 Bot 的安全配置注入；将客户端数组通过 GitHub 环境 Secret `BOT_INTEGRATION_CLIENTS_JSON` 注入网站；也可由有权限的运维通过测试服务器的配置脚本更新。不要把原始凭据放入仓库、issue、PR、群聊或文档。轮换时可配置不同客户端 ID 的新凭据，迁移后删除旧客户端。

```sh
python tools/create_bot_credential.py --client-id staging-synthetic \
  --scopes-file deploy/fixtures/chat-bot/scopes.json \
  --output-dir /secure/modelgate-bot-staging
gh secret set BOT_INTEGRATION_CLIENTS_JSON --env model-gate-staging \
  < /secure/modelgate-bot-staging/clients.json
gh secret set BOT_SMOKE_TOKEN --env model-gate-staging \
  < /secure/modelgate-bot-staging/bot-token
```

测试凭据仅允许 `mg-synthetic-qq` / `mg-synthetic-wx` 和 `mg-synthetic-group`，不能直接用于真实群。真实联调需另建显式 scope 的客户端，合并到客户端数组后更新 Secret，保留自动验收客户端。生产默认关闭，不能复用测试凭据。

## API 和绑定流程

所有路径以 `/api/v1` 开头，使用项目统一 `{ "code": 0, "message": "success", "data": ... }` 响应包装。错误使用实际 HTTP 状态码及 `reason`，不要通过中文消息判断业务状态。请求必须为单个 JSON 对象，最大 8 KiB，拒绝未定义字段，尤其是外部 `user_id`、邮箱和余额。

| 方法 / 路径 | 身份 | 用途 |
|---|---|---|
| `POST /integrations/bot/binding-challenges` | Bot | 从群事件生成 10 分钟绑定挑战 |
| `GET /user/chat-bindings/challenges/:id` | 网站 JWT | 展示当前账号与群身份，额外提供的只读预览接口 |
| `POST /user/chat-bindings/challenges/:id/authorize` | 网站 JWT | `{"confirm":true,"account_confirmation":"<预览返回值>"}`，显式确认后生成一次性码 |
| `POST /integrations/bot/bindings/confirm` | Bot | 从同一身份的 `/绑定 CODE` 消费确认码 |
| `GET /user/chat-bindings` | 网站 JWT | 当前用户有效绑定列表 |
| `DELETE /user/chat-bindings/:id` | 网站 JWT | 只能解绑自己拥有的记录 |
| `POST /integrations/bot/checkin` | Bot | 从群 `/签到` 事件触发共同签到逻辑 |

1. `/绑定` 的真实消息产生挑战请求：

```json
{"identity":{"platform":"qq_official","bot_id":"mg-synthetic-qq","group_id":"mg-synthetic-group","sender_id":"synthetic-local-member"},"event_id":"platform-message-bind-001"}
```

响应 `data` 包含 `challenge_id`、`binding_url`、`expires_at`、`replayed`。重复事件返回原挑战，过期后必须使用新的群消息。Bot 只需发送返回的 HTTPS 链接。

2. 用户打开 `/chat-bindings/authorize/:id`，登录后会回到该路径。预览还返回当前 JWT 的 `account_id`（仅用于页面核对）和 `account_confirmation`；它绑定当前显示账号与挑战，authorize 必须原样带回，防止另一个标签页切换登录账号后误绑。页面显示当前账号、平台、Bot、群和成员身份；用户勾选确认并点击后调用 authorize，返回 `{"code":"ABCD-EFGH-IJKL-MNOP","expires_at":"..."}`。码有 80 位随机熵，数据库仅存 SHA-256；刷新页面不会恢复明文。重新授权会替换旧码，不能延长原挑战期限或重置失败次数。

3. Bot 收到原群、原成员发出的 `/绑定 CODE` 后提交：

```json
{"identity":{"platform":"qq_official","bot_id":"mg-synthetic-qq","group_id":"mg-synthetic-group","sender_id":"synthetic-local-member"},"event_id":"platform-message-confirm-002","code":"ABCD-EFGH-IJKL-MNOP"}
```

成功返回 `{"status":"bound","request_key":"bot:<SHA256>","replayed":false}`。一次性码与绑定创建在同一事务内。最多 5 次不同事件的错误尝试，网络重复同一事件不会额外扣次数。码只能绑定挑战已授权的网站账号；同一身份只能有一个有效绑定，必须先在网站解绑才能更换账号。

## 签到、幂等与错误

```json
{"identity":{"platform":"wechat_native","bot_id":"mg-synthetic-wx","group_id":"mg-synthetic-group","sender_id":"synthetic-local-member"},"event_id":"platform-message-checkin-003","event_at":"2026-11-11T16:00:00+08:00"}
```

`event_at` 必须是平台原始事件时间，包含时区，重试保留原值；不能换成重试发生时的时间。服务器按上海时间判断日期，不能由客户端提交签到日期。只有首次请求会校验当前日期；跨日重放已经完成的事件仍返回原回执。

`request_key = "bot:" + SHA256(JSON数组[operation,platform,bot_id,group_id,sender_id,event_id])`，固定 68 字符，由网站生成。`operation` 为 `challenge`、`confirm`、`checkin`。不用客户端自己计算或传入 request_key。

签到 `data` 只包含 `status`、`request_key`、`replayed`、`checkin_date`，不包含网站用户 ID、邮箱、余额、VIP 等级或奖励金额。

| status | 群回复建议 |
|---|---|
| `checked_in` | 签到成功 |
| `already_checked_in` | 今天已签到 |
| `not_bound` | 请先发送 /绑定 |
| `ineligible` | 暂未满足签到条件，请到网站查看 |
| `account_unavailable` | 当前账号不可用，请到网站查看 |
| `date_changed` | 日期已变化，请重新发送 /签到 |
| `checkin_unavailable` | 暂时无法签到，请稍后重新发送 /签到 |

同一事件的成功或业务拒绝都会保存回执。绑定、资格、现金策略变化后，重放仍返回原状态并标记 `replayed:true`，必须新发一条消息才能重新判断。回执保存原绑定和原用户，即使之后解绑并绑定其他账号，旧事件也不会签到到新账号。同一事件修改 `event_at` 或确认码返回 `409 BOT_EVENT_CONFLICT`。

首次事件复用 SQL `achievement_checkin`：实时核验有效线上充值本金净额（默认最低 10 CNY），退款扣除，赠送、管理员加余额、余额消费不替代本金。拒绝时不修改签到、连续天数或奖励。现金开关关闭、非现金白名单、预算不足允许沿用网站的零奖励成功签到。网站、QQ、微信共用配置/用户锁、每日唯一约束与奖励债务抵扣；签到和 Bot 回执在同一个事务提交，再执行现有余额及鉴权缓存失效。

常见非业务错误：401 `BOT_UNAUTHORIZED`、403 `BOT_SCOPE_FORBIDDEN`、400 `BOT_REQUEST_INVALID` / `BOT_IDENTITY_INVALID` / `BOT_BINDING_CODE_INVALID`、409 `BOT_IDENTITY_ALREADY_BOUND` / `BOT_BINDING_CHALLENGE_CLAIMED` / `BOT_BINDING_CHALLENGE_USED` / `BOT_BINDING_ACCOUNT_CHANGED` / `BOT_EVENT_CONFLICT`、410 `BOT_BINDING_CHALLENGE_EXPIRED`。超限为 429 并带 `Retry-After`，客户端每分钟 240 次、身份每分钟 30 次；Redis 不可用时 503，禁止绕过限流。网络错误或 HTTP 5xx 可有限退避重试原请求；收到业务回执后停止自动重试。确认失败的重放标记在错误 `metadata` 内。

## 迁移与验收

新迁移只有：官方 `249_add_usage_logs_api_key_cost_index_notx.sql`、官方 `250_openai_cache_write_ttl.sql` 和本次 `260_chat_bot_bindings.sql`。原有两个编号为 249、两个编号为 250 的迁移按完整文件名分别识别；不要重命名、改内容或只按数字判断执行情况。完整文件名、原始 SHA-256 和运行器去首尾空白后的 SHA-256 记录在 `bot-upgrade-migrations.json`。其中已存在于迁移运行器的历史 checksum 兼容白名单一并列出，不增加新的例外。测试服务器按此清单逐项核验，并将实际完整记录保存至 `/opt/sub2api-staging/.bot-migration-receipt.json`，不会重写历史迁移记录。

260 新建 `chat_bindings`、`chat_binding_challenges`、`chat_bot_events` 及事务函数。事件回执不能直接按时间删除；将来清理必须保留足以阻止历史事件重放到新用户的墓碑。解绑只撤销有效绑定。

```sh
cd backend
CI=true go test -tags integration ./internal/repository -run 'TestChat|TestAchievement|TestVIP' -count=1
go test -tags unit ./internal/config ./internal/server/middleware ./internal/handler ./internal/service -run 'TestBot|TestChat' -count=1
```

集成测试在真实 PostgreSQL 上覆盖空库初始化、旧版 319 个 SQL 迁移升级与完整 checksum、跨平台身份、过期/限次/单次消费、越权解绑、退款、零奖励、上海午夜、旧事件重绑定、18 个并发网站/QQ/微信请求只发一次奖励。资格边界测试同时复用现有 VIP、成就、补签及退款测试。

合成事件可用 `python tools/bot_event_fixture.py qq_official --checkin` 或 `wechat_native` 生成；发送后保存整个 JSON，重试不可重新生成事件 ID 或时间。去掉 `--checkin` 生成绑定挑战事件。真实联调时应将这些字段替换为适配器可信事件，不得用测试 ID 冒充真实群身份。

部署流程使用 `verify-staging-bot.py` 在测试服务器创建两个短期合成用户，通过真实 HTTPS 完成 QQ/微信绑定与签到，验证隔离鉴权、资格、重放和解绑重绑。测试账号 JWT 仅在测试服务器内生成和使用；结束清理合成账号、订单、回执与预算占用。脚本拒绝生产 origin，不输出 token/确认码。默认由测试服务器生成合成客户端，原始凭据保存在 `/opt/sub2api-staging/.bot-smoke-token`（0600），配置快照在 `.bot-clients.json`（0600）；流水线通过 SSH 让服务器读取并验收，凭据不离开服务器。已有客户端配置不会被覆盖。也支持上述环境 Secrets 注入；显式覆盖客户端数组时必须同时提供匹配的合成测试凭据。当前 GitHub 安装授权不能写入环境 Secrets，故本次使用服务器安全配置方式。

本地验收还需覆盖：真实 QQ 群 member_openid、微信 threadId/senderId、Bot 重启后重复消息、群切换、网站登录返回链接、真实群中的最小化回复。该部分不能用合成测试替代。
