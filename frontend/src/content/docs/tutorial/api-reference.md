# API 参考

选择与你的客户端及密钥分组一致的协议。相同域名，不代表可以混用请求格式。

## 地址与协议

| 客户端 / 协议 | Base URL | 常用接口 |
| --- | --- | --- |
| OpenAI 兼容 SDK | `https://model-gate.cc/v1` | `/chat/completions`、`/responses`、`/models` |
| Anthropic SDK / Claude Code | `https://model-gate.cc` | `/v1/messages` |
| Gemini 原生协议 | `https://model-gate.cc` | `/v1beta/models` 及模型动作接口 |

表格中的接口需配合支持对应能力的分组与模型。Codex 使用 Responses 协议；不要用 Chat Completions 的请求体调用 Responses。

## 身份验证

OpenAI 兼容调用使用 Bearer 密钥：

```http
Authorization: Bearer YOUR_MODEL_GATE_KEY
Content-Type: application/json
```

Anthropic 原生请求通常还携带 `anthropic-version`，鉴权可使用 `x-api-key`。SDK 会自动组装协议请求头；具体以所用 SDK 版本为准。

## Responses 请求

```bash
curl https://model-gate.cc/v1/responses \
  -H "Authorization: Bearer $MODEL_GATE_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"YOUR_AVAILABLE_MODEL","input":"Reply with: connected","stream":false}'
```

## Anthropic Messages 请求

```bash
curl https://model-gate.cc/v1/messages \
  -H "x-api-key: $MODEL_GATE_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{"model":"YOUR_AVAILABLE_MODEL","max_tokens":64,"messages":[{"role":"user","content":"Reply with: connected"}]}'
```

## 流式响应

将支持流式的请求设置为 `stream: true`，并使用 SSE 兼容客户端消费响应。不要把流式响应直接当作普通 JSON 解析；不同协议的事件结构不同。

连接中断后先检查使用记录。已经产生输出的请求可能已有费用，重试并不保证免于重复计费。

## 图像与其他能力

平台包含图像相关接口，但不同渠道对同步、异步、多模态输入和返回格式的支持不同。请先确认对应分组的能力与公告，不能只根据模型名字判断。

## 状态码速查

| 状态 | 优先检查 |
| --- | --- |
| 400 | 请求格式、参数、协议是否匹配 |
| 401 | 密钥缺失、拼写、失效或错误环境 |
| 403 | 密钥状态、分组资格、账户权限 |
| 404 | 地址是否重复拼接 `/v1`、端点和模型是否可用 |
| 429 | 账户并发、RPM、上游限流 |
| 5xx | 平台或上游暂时异常，有限次数退避重试 |

完整的错误归因以响应正文为准，详见 [限流与重试](/docs?cat=platform&page=rate-limits)。
