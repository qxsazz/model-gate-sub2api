# Cursor 接入

Cursor 可通过 OpenAI 兼容配置连接 Model-Gate。不同版本的设置入口可能略有差异。

## 打开模型设置

在 Cursor 设置中找到模型提供商或 OpenAI API 配置区域，启用自定义 API 地址。

## 填写连接信息

```text
Base URL: https://model-gate.cc/v1
API Key: mg_sk_example
```

模型名称必须来自当前账户的可用模型列表。

## 测试请求

保存设置后发起一个短对话。若 Cursor 自带的连接测试与实际对话结果不同，以实际请求响应和客户端日志为准。

## 排查建议

- 确认没有被系统代理或其他扩展改写请求地址。
- 检查 Base URL 是否重复包含 `/v1`。
- 避免在屏幕共享或截图时暴露完整 API Key。
