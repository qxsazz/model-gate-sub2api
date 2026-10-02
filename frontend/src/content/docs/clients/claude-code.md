# Claude Code 接入

用独立密钥连接 Claude Code，将终端编程的用量与其他应用分开管理。

## 准备工作

按 [Claude Code 官方文档](https://code.claude.com/docs/en/overview)安装客户端，创建支持 Anthropic Messages 协议的 API Key。选择实际支持目标模型的分组。

## Windows PowerShell

```powershell
$env:ANTHROPIC_BASE_URL = "https://model-gate.cc"
$env:ANTHROPIC_AUTH_TOKEN = "YOUR_MODEL_GATE_KEY"
$env:ANTHROPIC_MODEL = "YOUR_AVAILABLE_MODEL"
claude
```

## macOS / Linux

```bash
export ANTHROPIC_BASE_URL="https://model-gate.cc"
export ANTHROPIC_AUTH_TOKEN="YOUR_MODEL_GATE_KEY"
export ANTHROPIC_MODEL="YOUR_AVAILABLE_MODEL"
claude
```

模型占位符必须替换。当前终端中设置的环境变量会传递给从该终端启动的 Claude Code；重新打开另一个终端不会自动继承这些临时设置。

## 验证连接

先发送简单、只读的请求，收到回复后在 MODEL-GATE 使用记录中核对。不要在首次验证时运行大规模代码生成或自动化任务。

## 配置冲突

如果你曾使用其他网关、配置切换工具或企业托管配置，检查实际生效的地址与凭据来源。不要同时保留多套互相冲突的密钥，修改配置前先备份。

## 排查顺序

1. 检查 Base URL，不手动重复追加 `/v1/messages`。
2. 核对密钥与环境是否匹配。
3. 确认分组及模型支持 Anthropic Messages。
4. 检查用量、并发与上游状态。

更多字段参见 [官方环境变量说明](https://code.claude.com/docs/en/env-vars)。本站 API Key 接入与 Claude 官方账号订阅是不同的计费体系。
