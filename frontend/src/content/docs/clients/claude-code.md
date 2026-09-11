# Claude Code 接入

你可以通过环境变量让 Claude Code 使用 Model-Gate。配置项名称可能随客户端版本变化，请以当前客户端文档和命令帮助为准。

## 准备工作

先确认你已经创建 API Key，并能通过 `/v1/models` 查询可用模型。

## 配置地址和密钥

将 Anthropic 兼容基础地址设置为 Model-Gate，并通过环境变量提供密钥：

```text
ANTHROPIC_BASE_URL=https://model-gate.cc
ANTHROPIC_AUTH_TOKEN=mg_sk_example
```

不要同时在多个配置文件中放置不同密钥，否则排查时容易误判实际生效来源。

## 验证连接

重新打开终端后启动 Claude Code，执行一个不会修改文件的简单请求。若连接失败，依次检查环境变量、网络、账户可用模型和客户端日志。

## 模型选择

只选择控制台或模型列表接口中真实存在的模型标识。Model-Gate 不在文档中固定承诺某个模型永久可用。
