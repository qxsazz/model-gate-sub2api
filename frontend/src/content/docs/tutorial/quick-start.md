# 快速开始

本指南帮助你完成 Model-Gate 的首次 API 调用。你只需要一个可用账户和一枚 API Key。

> 示例中的 `mg_sk_example` 仅用于说明格式，请替换为你自己创建的密钥。

## 1. 创建 API Key

登录 Model-Gate 控制台，进入 **API Keys** 页面并创建一枚新密钥。复制后请立即保存；不要把密钥提交到 Git 仓库或粘贴到公开聊天中。

## 2. 设置环境变量

在 PowerShell 中临时设置密钥：

```powershell
$env:MODEL_GATE_API_KEY = "mg_sk_example"
```

在 macOS 或 Linux 中：

```bash
export MODEL_GATE_API_KEY="mg_sk_example"
```

## 3. 查询可用模型

```bash
curl https://model-gate.cc/v1/models \
  -H "Authorization: Bearer $MODEL_GATE_API_KEY"
```

返回结果会列出当前账户可用的模型。实际可用范围以控制台和接口返回为准。

## 4. 发出第一个请求

选择上一步返回的模型标识，并按照对应客户端或 API 协议发起请求。不要直接照抄未经确认的模型名称。

## 下一步

- 阅读 [API Key 安全指南](/docs?cat=tutorial&page=api-key)
- 查看 [API 参考](/docs?cat=tutorial&page=api-reference)
- 按客户端选择 [Claude Code](/docs?cat=clients&page=claude-code) 或 [Codex](/docs?cat=clients&page=codex) 指南
