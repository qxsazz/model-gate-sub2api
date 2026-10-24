# 官方 Sub2API v0.2.18 兼容合并

## 来源与范围

- 官方仓库：<https://github.com/Wei-Shaw/sub2api>
- 稳定版：<https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.18>
- 上游提交：`ac1b8f127`（以 release tag 为准，不直接追随开发中的 main）。
- MG 合并前提交：`25da21418`，已记录的上游基线为 `0.2.17`。
- 升级分支：`upgrade/upstream-0.2.18`。

## 二开兼容处理

- 保留 MG 品牌与 README、画布及私有素材审核、VIP/签到权益、分组计价、托管发布限制；不使用上游安装器或二进制覆盖 MG。
- Gemini Chat 兼容流保留 MG 的思考预算、思考/正文独立增量及签名处理，并整合官方客户端断连后的用量结算状态。
- 账号用量组件整合 Vertex 配额进度条；补齐官方 Gemini 地址识别函数和凭据 URL 类型，仅恢复用量分类所需能力，不引入已移除的上游显式缓存配置入口。
- 去掉自动合并在 package.json 和 pnpm 锁文件中重复插入的 follow-redirects override，保留双方安全版本约束及 DOMPurify 3.4.17 更新。
- 保留已有 VIP `246_vip_bonus_and_discount_cap.sql` 的文件名及内容。官方新增 `246_enable_gemini_image_generation_groups.sql` 按独立完整文件名记录，不覆盖已应用迁移。
- 增加 MG 思考流与断连计费联合回归、迁移记录与分组权限回归、官方/第三方 Gemini URL 分类回归。

## 行为变化

- Vertex service_account 支持总额、每日和每周配额。
- Gemini/Antigravity 旧分组通过新增迁移开启生图；composite 分组保留原有开关状态，不自动开启。使用混合分组的 Gemini 生图前需核对该开关。
- Grok 4.7 Fast / Build Fast 按官方 Fast 价卡结算，基础价为普通 Grok 4.7 的两倍；MG 的分组和用户倍率继续适用。
- 异步图片下载拒绝内网、回环地址及重定向至这些地址，非图片响应判为失败。

## 验证

- 前端：首轮 388 个文件、2,879 条用例通过；补齐缺失工具函数后，重跑的 7 个文件及新增分类测试共 8 个文件、110 条用例全部通过。合计 396 个文件、2,989 条用例通过。
- 前端 ESLint：通过；仅有未改动的 ParticleNetwork.vue 中 5 条未使用变量警告。新增工具函数、测试和类型另行 lint 通过。
- 后端单元测试：所有包通过；服务层在资源受限时单独以 `-p 1 -parallel 1` 重跑，通过。
- 后端集成测试：所有包通过；使用 PostgreSQL 18.1 和 Redis 8.4 的独立测试容器。服务层串行重跑通过 8,497 条用例及子用例。
- 新增迁移回归：通过，验证两个 246 迁移均已记录、重复执行幂等，以及 composite/Grok 原有权限不变。
- 画布：TypeScript 检查及生产构建通过；画布当前没有可执行的 Bun 测试文件，其后端权限/归档测试由 Go 测试覆盖。
- 上游版本报告工具：4 条测试通过。
- 部署脚本：Compose 安全、Gateway 环境、运行资源、simple mode 环境和 Caddy 缓存/SSE 检查全部通过。
- 主前端生产构建：通过（`NODE_OPTIONS=--max-old-space-size=2560 pnpm run build`）。
- 完整嵌入式后端：通过 `go build -p 1 -tags embed`，同时包含主站及画布；`go test -tags embed ./internal/web` 通过。

初次并行检查触发云工作区 8 GiB 内存上限，相关进程被系统终止；改为串行并限制 Go/Node 内存后重跑。该资源错误未被当作测试通过。测试日志位于云工作区 `/workspace/.onboarding/upstream-0.2.18/`。

验证源码后将 MG 版本设置为 `0.2.18-mg.1`，独立的上游同步基线更新为 `0.2.18`。

## 发布边界

本分支用于源码兼容合并和本地验证。生产发布仍遵循 `docs/cicd-release-runbook.md` 的 staging 验收、PR 检查和人工晋级流程。
