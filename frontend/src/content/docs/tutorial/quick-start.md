# 快速开始

从一枚密钥，到首次 API 调用。MODEL-GATE 将不同模型的接入、分组和用量，归集到同一账户中。

> 请先完成账户准备并确认余额与分组权限。以下请求使用生产地址；测试环境的账户和密钥不与生产环境通用。API 调用可能产生费用。

## 01 · 创建访问密钥

在控制台的 **API 密钥** 页面创建一枚独立密钥。为它命名并选择与目标模型匹配的分组，例如为终端编程和桌面聊天分别创建密钥。

分组决定可用渠道、模型范围和计费倍率。VIP 身份不会自动替换现有密钥的分组，详见 [模型与分组](/docs?cat=platform&page=groups)。

## 02 · 设置凭据

Windows PowerShell：

```powershell
$env:MODEL_GATE_API_KEY = "YOUR_MODEL_GATE_KEY"
```

macOS / Linux：

```bash
export MODEL_GATE_API_KEY="YOUR_MODEL_GATE_KEY"
```

不要将真实密钥写入代码仓库、截图或公开聊天。以上变量仅在当前终端会话中生效；从该终端启动的程序可以读取它。

## 03 · 查询可用模型

Windows 请使用 `curl.exe`，避免旧版 PowerShell 将 `curl` 解释成其他命令：

```powershell
curl.exe "https://model-gate.cc/v1/models" -H "Authorization: Bearer $env:MODEL_GATE_API_KEY"
```

macOS / Linux：

```bash
curl https://model-gate.cc/v1/models \
  -H "Authorization: Bearer $MODEL_GATE_API_KEY"
```

从返回的模型列表中选取一个支持目标协议的模型标识。列表随密钥分组变化，不代表所有列出的模型都支持所有接口。

## 04 · 完成一次调用

下面是 OpenAI Chat Completions 兼容接口的最小请求。将 `YOUR_AVAILABLE_MODEL` 替换为当前分组中支持该接口的模型。

```bash
curl https://model-gate.cc/v1/chat/completions \
  -H "Authorization: Bearer $MODEL_GATE_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"YOUR_AVAILABLE_MODEL","messages":[{"role":"user","content":"Reply with: connected"}],"stream":false}'
```

PowerShell 可以用结构化请求，避免 JSON 引号转义：

```powershell
$body = @{ model = "YOUR_AVAILABLE_MODEL"; messages = @(@{ role = "user"; content = "Reply with: connected" }); stream = $false } | ConvertTo-Json -Depth 5
Invoke-RestMethod -Uri "https://model-gate.cc/v1/chat/completions" -Method Post -Headers @{ Authorization = "Bearer $env:MODEL_GATE_API_KEY" } -ContentType "application/json" -Body $body
```

收到模型回复后，进入 **使用记录** 核对请求时间、密钥、模型和实际费用。若没有成功回复，先查看 [常见问题](/docs?cat=help&page=faq)，不要连续发送大量重试请求。

## 05 · 选择你的工作方式

| 使用场景 | 下一步 |
| --- | --- |
| Claude Code 编程 | [Claude Code 接入](/docs?cat=clients&page=claude-code) |
| Codex CLI 编程 | [Codex 接入](/docs?cat=clients&page=codex) |
| 自建应用或脚本 | [API 参考](/docs?cat=tutorial&page=api-reference) · [SDK 示例](/docs?cat=tutorial&page=sdk) |
| 桌面聊天与编辑器 | [工具选型指南](/docs?cat=tools&page=recommended) |
| 了解成长权益 | [VIP 等级与权益](/docs?cat=membership&page=vip) |
