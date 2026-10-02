# 环境准备

在安装工具前，先准备可用终端、密钥和网络。无需为阅读文档安装任何客户端。

## Windows

推荐使用 PowerShell。调用 curl 示例时使用 `curl.exe`，并注意 PowerShell 使用 `$env:变量名` 读取环境变量，与 Bash 写法不同。

```powershell
$env:MODEL_GATE_API_KEY = "YOUR_MODEL_GATE_KEY"
curl.exe "https://model-gate.cc/v1/models" -H "Authorization: Bearer $env:MODEL_GATE_API_KEY"
```

## macOS / Linux

```bash
export MODEL_GATE_API_KEY="YOUR_MODEL_GATE_KEY"
curl https://model-gate.cc/v1/models -H "Authorization: Bearer $MODEL_GATE_API_KEY"
```

临时环境变量只在当前终端及其启动的程序中生效。长期保存前，确认配置文件不会上传到代码仓库或云同步服务。

## Node.js 与 Python

部分工具依赖 Node.js 或 Python。以工具官方要求选择受支持版本，从 [Node.js 官方网站](https://nodejs.org/)或 [Python 官方网站](https://www.python.org/)安装，避免来历不明的整合安装包。

```text
node --version
npm --version
python --version
```

不要为了接入网关而关闭客户端的权限审批、安全沙箱或系统防护。

## 网络与凭据

优先使用文档公布的域名。若超时，分别检查 DNS、代理设置和客户端日志；不要把站点密钥粘贴到第三方连通性检测网页。

API 地址与密钥必须属于同一环境，测试账户的密钥不能用于生产地址。
