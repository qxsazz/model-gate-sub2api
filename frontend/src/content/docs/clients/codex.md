# Codex 接入

本指南说明如何让兼容 OpenAI API 的 Codex 客户端通过 Model-Gate 发起请求。

## 获取凭据

在 Model-Gate 控制台创建独立 API Key，例如专门用于本机 Codex 的密钥。

## 配置 API 地址

将客户端的 OpenAI 兼容 Base URL 指向：

```text
https://model-gate.cc/v1
```

将 API Key 设置为环境变量或 Codex 支持的安全凭据配置，示例值为 `mg_sk_example`。

## 选择模型

先请求 `/v1/models` 或查看控制台，再填写当前账户可用的模型标识。不要根据旧截图猜测模型名。

## 验证配置

启动 Codex 后先执行只读任务。如果收到认证错误，检查密钥和 Base URL；如果收到模型错误，重新确认模型列表；如果收到限流错误，降低并发并稍后重试。
