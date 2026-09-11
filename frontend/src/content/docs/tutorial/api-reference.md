# API 参考

Model-Gate 提供兼容常见 AI 客户端的 API 入口。本文只描述稳定的接入约定；具体模型能力以 `/v1/models` 和控制台显示为准。

## 基础地址

```text
https://model-gate.cc
```

OpenAI 兼容客户端通常将 Base URL 配置为：

```text
https://model-gate.cc/v1
```

## 身份验证

```http
Authorization: Bearer mg_sk_example
Content-Type: application/json
```

## 可用接口

平台支持的兼容入口包括 `/v1/models`、`/v1/messages` 和 `/v1beta/models`。不同协议的请求体并不相同，请使用与你的客户端匹配的入口。

## 错误处理

- `401`：凭据缺失、格式不正确或已失效。
- `403`：当前账户或密钥无权执行该请求。
- `429`：请求过快或达到并发限制，应遵循退避策略。
- `5xx`：服务或上游暂时异常，可进行有限次数重试。

## 安全建议

日志中不要记录完整密钥或包含敏感内容的请求正文。排查问题时使用请求时间、状态码和脱敏后的请求标识。
