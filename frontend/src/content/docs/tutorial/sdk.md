# SDK 调用示例

使用熟悉的 SDK，在应用中显式设置 MODEL-GATE 的地址与密钥。先从单次、非流式请求验证连接。

## Python · OpenAI 兼容接口

安装依赖：

```bash
python -m pip install openai
```

设置 `MODEL_GATE_API_KEY` 和 `MODEL_GATE_MODEL` 环境变量，模型名必须来自当前密钥分组的可用列表。

```python
import os
from openai import OpenAI

client = OpenAI(
    api_key=os.environ["MODEL_GATE_API_KEY"],
    base_url="https://model-gate.cc/v1",
)
reply = client.chat.completions.create(
    model=os.environ["MODEL_GATE_MODEL"],
    messages=[{"role": "user", "content": "Reply with: connected"}],
)
print(reply.choices[0].message.content)
```

## JavaScript · Responses

```bash
npm install openai
```

```javascript
import OpenAI from "openai";

const client = new OpenAI({
  apiKey: process.env.MODEL_GATE_API_KEY,
  baseURL: "https://model-gate.cc/v1",
});
const reply = await client.responses.create({
  model: process.env.MODEL_GATE_MODEL,
  input: "Reply with: connected",
});
console.log(reply.output_text);
```

此示例使用支持 ESM 的 Node.js 项目，所选模型和分组必须支持 Responses。不同模型的可选参数并不完全相同。

## 生产应用注意事项

- 在服务端保存密钥，不将它暴露到网页源代码或浏览器包中。
- 设置请求超时、重试上限和应用级并发队列。
- 为不同环境使用独立密钥，避免开发任务消耗生产额度。
- 先核对 SDK 与模型支持的参数，再逐步加入工具调用或流式输出。

接口结构可查阅 [OpenAI 官方 API 文档](https://developers.openai.com/api/docs/)。MODEL-GATE 的可用能力以当前分组、模型与实际响应为准。
