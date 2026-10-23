# Gemini 画布模型列表兼容修复

## 问题与修复

普通 Gemini 上游可能在 `/v1beta/models` 返回 HTTP 200 与 OpenAI 模型列表。画布 Google 渠道消费 `models[].name`，因此此前会把该响应当作没有模型。使用交接文档中的四模型样本，在完整 `GeminiV1BetaListModels` handler 上先复现了失败，再验证修复。

成功响应处理顺序为：识别并规范化列表 → 合并既有 Antigravity 模型 → 执行分组白名单 → 写出响应。service 的原样透传接口不变；生成接口、权限、账号调度、模型映射、定价、计费和数据库不变。

转换前：

```json
{"object":"list","data":[{"id":"gemini-3.1-flash-lite-image","object":"model"},{"id":"gemini-3.1-flash-image-preview","object":"model"},{"id":"gemini-nano-banana-2.1","object":"model"},{"id":"gemini-3-pro-image-preview","object":"model"}]}
```

转换后（未启用白名单或全部命中）：

```json
{"models":[{"name":"models/gemini-3.1-flash-lite-image"},{"name":"models/gemini-3.1-flash-image-preview"},{"name":"models/gemini-nano-banana-2.1"},{"name":"models/gemini-3-pro-image-preview"}]}
```

原生有效 Google 列表优先，并保留方法、模型元数据、分页与未知元数据。冗余的 OpenAI 目录会被移除，避免白名单只过滤 `models` 后仍从 `data` 泄漏模型。OpenAI 转换仅接受显式 `object: "list"` 与数组 `data`；保持别名大小写、统一单个 `models/` 前缀、稳定去重，拒绝不能用于本站 Gemini 模型路径的 ID。不复制任意条目元数据，也不虚构生成能力。合法空数组可转换；非空但全部无效、未知 envelope、无效 JSON 与 HTML 保留原有透传行为，不伪造空列表。

仅 HTTP 200 进入规范化；已有 insufficient-scope fallback 保持原状。响应体改变时使用 JSON Content-Type，并移除旧长度、编码及实体校验头；原有写出函数继续处理响应长度与 hop-by-hop 头。

## 验证

```sh
# 仓库根目录；云端内存受限时串行执行。
GOMEMLIMIT=1200MiB GOGC=50 go -C backend test -p 1 -parallel 1 -tags unit -timeout=15m ./internal/handler -count=1
GOMEMLIMIT=1200MiB GOGC=50 go -C backend test -p 1 -parallel 1 -tags unit ./internal/service -run 'SelectAccountForModelsEndpoints|ForwardListModels|GroupKeyCapabilities' -count=1
GOMEMLIMIT=1200MiB GOGC=50 go -C backend build -p 1 -tags embed ./cmd/server
cd canvas/web
bun test tests/managed-models.test.ts
npm run typecheck
```

后端覆盖四模型样本、原生元数据、空列表、前缀、去重、无效条目、混合 envelope、白名单具体名称/通配符/前缀、没有发现的模型不会被补齐、混合账号去重及原生元数据优先、401/403/429/5xx/HTML、既有 Vertex 和 scope fallback。画布 broker 测试实际经过 `CanvasHandler.Gateway`、路径转换及列表 handler，并校验生图权限保持原值。

画布测试执行实际 `refreshManagedModels` 和配置 store，仅模拟宿主请求与存储：未显式配置时发现四个生图模型；已保存清单时保留选择；未开启生图权限时不出现可用生图模型。测试不读取或写入用户云端状态。

## 发布基准与边界

2026-10-23 核对：生产最近成功部署记录的 `SOURCE_SHA` 为 `44d3c8233a695abfbfb29f8017cf86f3b742263e`，镜像 digest 为 `sha256:5827811733eb47b827b023aa18b4be69ceccdbdfad5af2db6ffb6148a719afa2`；公开版本为 `0.2.17-mg.2`。此为部署记录核对，未直接连接生产容器重新 inspect。

修复基于测试环境已成功部署的 `a40cd80086950d69124aabe52ef66d71955fbd9b`（`0.2.18-mg.1`）。上一轮网关恢复策略位于独立分支，本修复不包含该改动。

测试环境修复前成功发布的镜像 digest 为 `sha256:b874484137ed0a747529fa9a081817c693c6ebbfd733bb0062b38693ec12c4ce`（工作流 `39954867980` 的部署记录）。这只用于记录本次测试发布的基准；执行回退前仍需确认实际部署状态。

测试环境与生产配置不同：测试环境同名 nanobanana 分组为 26，现有管理员画布 Key 为 10；修复前只读 GET `/api/v1/canvas/gateway/10/gemini/models` 已返回原生 Google 列表，内容为上游实际列出的 `gpt-image-*` 模型，并非交接文档的四个 Gemini 模型。生产组号与生图开关不能直接套用于测试环境。四模型兼容、权限拒绝及白名单收敛由回归测试验证；测试部署后另行核对该现有 Key 的实际列表，不能宣称它返回了未发现的四模型。

不清缓存、不清画布状态、不修改 Key、分组或账号配置、不触发付费生成。生产发布需用户后续指示；回退以实际部署前镜像 digest 为准。
