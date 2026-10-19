# 模型网关用户错误提示

本变更只改善失败说明，不减少故障次数，也不修改调度、重试、粘性、冷却、配额或计费。未部署生产。

## 输出边界

`handler.GatewayClientErrorMessages` 只注册在模型网关路由链，位于 body limit、鉴权和 `OpsErrorLoggerMiddleware` 之前。下游逻辑和运维采集先处理原始载荷，最后才改写发往客户端的 JSON 字符串。包装器向业务代码返回原始写入字节数与 Size，避免错误文案长度影响“是否已经输出”的判断。

`service.RewriteGatewayClientError` 使用结构化错误码、已有 Ops 语义状态、计费错误 reason 和现有上下文／容量识别器生成提示，不把新文案送回原始错误分类。未知错误采用保守模板，未知 code 和原始 message 不会被拼进提示。

只替换已经存在的 message，不新增字段。HTTP 状态、type/code/status、Retry-After、request_id、SSE 事件名和顺序保留；原有脱敏与容量兼容码转换仍在此前完成。已识别的会话安全屏蔽保留专门文案。

显式错误透传规则中的 CustomMessage 优先：service 共享透传函数以及 OpenAI、通用 Gateway、Gemini 的 failover 规则出口登记展示豁免。仅透传原始上游 body 而没有显式自定义文案的规则仍适用统一模板。WS 每轮重置展示豁免，避免影响下一轮。

## 覆盖入口

| 路径／协议 | 覆盖情况 |
|---|---|
| `/v1` 的 Responses、Chat Completions、Messages、Embeddings 等模型接口 | JSON 错误、鉴权／计费错误、SSE 错误 |
| `/responses`、`/backend-api/codex` 及已有根路径别名 | 共用相同边界 |
| `/v1beta` Gemini native | 已有 Google JSON／SSE error envelope |
| `/antigravity/v1`、`/antigravity/v1beta` | 对应协议的已有错误 envelope |
| OpenAI native／passthrough SSE | 现有脱敏和兼容转换完成后，改写 error／response.failed |
| Anthropic-compatible SSE | 既有 error 对象的 message |
| OpenAI WS ingress | 现有下行结构化错误消息与本地策略错误帧；成功帧不变 |
| 图片、音视频、异步任务等 | 经上述共享 HTTP／SSE 网关链的即时错误自动覆盖；不改写成功响应内部的异步任务结果 |

SSE 支持 LF／CRLF、跨 Write 分片和多行 data JSON。只替换 JSON message 字符串 token，保留原有 data 行、注释、id/retry 行及其他内容。不会增加终止事件或重放模型输出。

## 保留原行为／不覆盖

- 管理后台接口和反向代理／CDN 错误页面。
- 账单信息和用量查询（`/sub2api/billing`、`/usage`）保持原有提示。
- WS close code、close reason 和关闭帧；没有结构化错误帧时不新增。
- 独立媒体通道、Live sideband 等没有经过共享出口的错误。
- 正常模型输出，以及成功响应内部的错误描述、工具数据和异步任务状态。
- 无效 JSON、未知 envelope、无 message 的错误对象。
- 已显式固定 Content-Length 或已有 Content-Encoding 的载荷：为保留响应头与传输正确性不改写。
- SSE 未闭合帧在显式 Flush 时保留原字节；无法再安全重写已经发送的片段。
- 超出保护上限的未闭合帧保留原传输行为，避免无界缓存。

## 示例

HTTP 503，机器字段保留：

```json
{"error":{"type":"api_error","message":"【服务暂时不可用｜503】服务暂时不可用。请稍后在当前会话重试，可重新发送上一条消息或输入“请重试上一条请求”；若持续失败，可尝试新建会话并补充必要上下文。"}}
```

HTTP 200 内已有可信语义 503 的 SSE：

```text
event: response.failed
data: {"type":"response.failed","response":{"error":{"code":"server_error","message":"【服务暂时不可用｜流内错误 503】服务暂时不可用。请稍后在当前会话重试，可重新发送上一条消息或输入“请重试上一条请求”；若持续失败，可尝试新建会话并补充必要上下文。"}}}
```

没有可信数字状态时只显示类别和允许列表中的 code，不把 server_error 自动当作 503。用户 Key 错误提示检查本地 Key；上游认证／额度错误不会提示用户换 Key 或充值本地账户；参数、权限和策略问题不会建议盲目重试或切换会话。

## 验证

回归测试包括分类模板、不可变输入、机器字段、敏感文案、SSE 分片与 framing、原始 Ops 证据、管理员文案豁免，以及真实 native／passthrough 转发的改写前后对照。使用本地模拟上游，不调用付费推理。

需要 Go 1.27.2 及项目依赖。此环境使用 `/workspace/.onboarding/env.sh` 配置工具链。在 `backend` 目录执行：

```sh
go test ./internal/service ./internal/handler ./internal/server/routes -run '^TestGatewayClientError' -count=1
go test ./internal/handler ./internal/server/routes ./internal/server/middleware -count=1
go test ./internal/service -run 'TestGatewayClientError|TestApplyErrorPassthroughRule|Test.*HandleErrorResponse|TestOpenAIStreaming|TestHandleStreamingResponseCyberPolicy|TestStreamingPassthroughCyberPolicy|TestHandleSSEToJSON|TestHandleNonStreamingResponse' -count=1
```

2026-10-19 验证结果：新增专项用例通过；handler、routes、鉴权 middleware 整包回归通过；service 错误透传、流式、非流式相关回归通过。最后新增的分类优先级保护也已通过对应专项及相关路由验证。`git diff --check` 通过。

独立 WS 测试使用本地 httptest 服务，检查 JSON 错误提示已改写、关闭码和关闭原因仍保持原值；native／passthrough 对照测试使用 HTTPUpstream stub，检查上游只调用一次、内部错误和其他输出不变。未执行付费推理，未连接生产修改配置或部署。
