# Codex 接入

为 Codex CLI 配置独立的 MODEL-GATE 服务商。保留原有安全和审批设置，只替换模型、服务地址与凭据来源。

## 准备工作

从 [Codex 官方文档](https://developers.openai.com/codex/)安装客户端。创建支持 Responses 协议的 API Key，并取得该分组实际可用的模型标识。

## 设置密钥

PowerShell：

```powershell
$env:MODEL_GATE_API_KEY = "YOUR_MODEL_GATE_KEY"
```

macOS / Linux：

```bash
export MODEL_GATE_API_KEY="YOUR_MODEL_GATE_KEY"
```

## 配置用户级文件

编辑用户目录的 `~/.codex/config.toml`；Windows 对应 `%USERPROFILE%\.codex\config.toml`。修改前备份原文件，合并以下字段，不要直接覆盖全部已有配置。

```toml
model_provider = "modelgate"
model = "YOUR_AVAILABLE_MODEL"

[model_providers.modelgate]
name = "MODEL-GATE"
base_url = "https://model-gate.cc/v1"
env_key = "MODEL_GATE_API_KEY"
wire_api = "responses"
```

将模型占位符替换为实际标识。不要使用官方保留的服务商名称作为自定义 ID，也不要重复创建同名 TOML 表。

## 验证连接

从设置了环境变量的终端启动 `codex`，先执行只读任务，再在使用记录中核对模型、密钥和费用。

此配置用于 API Key 接入，不是使用 MODEL-GATE 的账号密码登录 Codex，也不会将你的官方订阅转换为本站余额。

## 常见问题

| 现象 | 检查方向 |
| --- | --- |
| 仍连接官方地址 | 用户级配置是否生效、客户端是否需要重新启动 |
| 找不到环境变量 | 是否从同一终端启动、`env_key` 是否完全一致 |
| 404 或协议错误 | `/v1` 是否重复、分组是否支持 Responses |
| 模型不存在 | 用该密钥重新查询模型列表 |

配置字段依据 [官方配置参考](https://developers.openai.com/codex/config-reference/)。桌面客户端与 CLI 的配置生效方式可能不同，请按所用版本核对。
