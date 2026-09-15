# Model-Gate 前端与 CI/CD 交接记录

更新时间：2026-09-13

## 1. 当前状态

本轮工作已完成用户端高奢视觉统一，并已部署到测试环境供人工验收。

| 项目 | 当前值 |
|---|---|
| GitHub 仓库 | `qxsazz/model-gate-sub2api` |
| 当前分支 | `feature/user-console-luxury-ui` |
| 当前提交 | `8c9436553e6682ed7a2fd9697614628d1c620298` |
| 功能提交 | `729b22b4b` |
| PR | [#5](https://github.com/qxsazz/model-gate-sub2api/pull/5) |
| PR 目标 | `staging` |
| PR 当前状态 | Open，尚未合并到 `staging` |
| 测试环境 | [https://staging.model-gate.cc](https://staging.model-gate.cc) |
| 当前测试页 | [https://staging.model-gate.cc/redeem](https://staging.model-gate.cc/redeem) |

测试环境当前运行镜像：

```text
ghcr.io/qxsazz/model-gate-sub2api@sha256:0d6202feebe7b09b474d5a8b6c09666d589e93c652981fe82090c527e6cac879
```

服务器状态已确认：

```text
sub2api-staging: running / healthy
/health: HTTP 200
/redeem: HTTP 200
```

## 2. 已完成内容

### 前端视觉统一

- 首页 `/home`：保留粒子背景，统一科技 SaaS 按钮与 Model-Gate 品牌文案。
- 用户控制台：统一黑曜石、珍珠白、香槟金配色。
- `/dashboard`：统一卡片、统计、快捷操作和历史区域。
- `/keys`：统一用户端导航与页面表面风格。
- `/usage`：统一表格、筛选、状态和空状态风格。
- `/purchase`：支付主页面完成高奢主题覆盖。注意：用户口中的 `/payment` 主页面在代码中实际路由是 `/purchase`。
- `/redeem`：兑换余额卡改为黑钻卡片，兑换表单、成功提示、失败提示、说明和历史记录已统一。
- MG 品牌资源：`frontend/public/model-gate-mg-luxury.svg`。
- 未修改后端接口、数据库模型、迁移或支付业务逻辑。

### CI/CD 验证

PR #5 的以下检查已通过：

- `CI`
- `Frontend CI`
- `Security Scan`
- `Deploy Staging`

测试部署工作流：

[Deploy Staging #34787890935](https://github.com/qxsazz/model-gate-sub2api/actions/runs/34787890935)

本次部署使用 Docker 在镜像内部完成前端构建，不产生独立前端 artifact。生产晋级时应复用 staging 已验证的镜像 digest，不重新构建。

## 3. 当前需要人工完成的事项

请登录测试环境后检查：

1. `/redeem` 黑钻余额卡的实际比例、字体和移动端表现。
2. `/purchase` 支付主页面的卡片、按钮、支付方式和空状态。
3. `/dashboard`、`/keys`、`/usage` 的侧边栏、顶部栏和页面背景是否统一。
4. 深色模式是否存在文字对比度或边界问题。
5. 浏览器控制台是否出现新增错误。
6. 登录、刷新、退出登录和主要路由跳转是否正常。

当前测试环境使用已有测试账号即可。不要在测试环境输入生产 API Key、生产支付密钥或生产 Token。

## 4. 验收通过后的操作

### 4.1 合并到 staging

视觉验收通过后，合并 PR #5：

```text
feature/user-console-luxury-ui -> staging
```

合并前确认 PR 检查仍然通过。PR 未合并前，生产分支和生产数据库不会受到影响。

### 4.2 创建 staging 到 main 的 PR

在 GitHub 创建：

```text
base: main
compare: staging
```

检查差异只包含已验收的前端、测试、品牌资源和文档内容，然后合并。

### 4.3 手动晋级生产

进入：

```text
Actions -> Deploy Production -> Run workflow
```

选择：

```text
Use workflow from: main
Staging commit SHA: 8c9436553e6682ed7a2fd9697614628d1c620298
Confirm: DEPLOY
```

生产 workflow 会先确认 staging 仍运行同一个镜像 digest，再备份生产 PostgreSQL，并只更新应用容器。它不会主动重建 PostgreSQL 或 Redis。

如果 PR #5 合并后 staging 产生新的合并提交，应优先使用成功 staging 构建记录中实际使用的 SHA；不要凭感觉填写 main 的合并 SHA。

## 5. 数据库与回滚边界

- 本轮没有修改 `backend/**` 或 `backend/migrations/**`。
- 本轮不会新增数据库迁移。
- staging 部署只更新应用容器，PostgreSQL 和 Redis 保持运行。
- 生产部署虽然不重建数据库容器，但应用启动仍会检查迁移状态。
- 应用镜像回滚不会自动回滚已经执行的数据库迁移。
- 如果后续改动包含后端迁移，必须先单独在 staging 验收迁移兼容性，再考虑生产发布。

若 staging 视觉验收不通过，PR #5 暂不合并即可，`staging` 和 `main` 都不会被修改。修复后继续向同一个 PR 推送即可触发新的检查和测试部署。

## 6. 本地继续开发

仓库目录：

```text
C:\model-gate.cc-server\_src\model-gate-sub2api
```

本地前端预览：

```powershell
cd C:\model-gate.cc-server\_src\model-gate-sub2api\frontend
$env:VITE_DEV_PROXY_TARGET='https://staging.model-gate.cc'
$env:VITE_DEV_PORT='4176'
node_modules/.bin/vite.cmd --host 127.0.0.1 --port 4176
```

访问：

```text
http://127.0.0.1:4176/redeem
```

本地验证命令：

```powershell
node_modules/.bin/vitest.cmd run src/views/__tests__/HomeView.compact.spec.ts src/components/home/__tests__/HomeConsolePreview.spec.ts src/components/layout/__tests__/userConsoleLuxuryTheme.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/views/user/__tests__/RedeemView.luxury.spec.ts
node_modules/.bin/eslint.cmd src
node_modules/.bin/vue-tsc.cmd --noEmit
node_modules/.bin/vite.cmd build
```

本轮结果：50 项相关测试通过，ESLint 通过，`vue-tsc` 通过，Vite 生产构建通过。

## 7. 重要文件

```text
frontend/src/views/HomeView.vue
frontend/src/views/user/DashboardView.vue
frontend/src/views/user/KeysView.vue
frontend/src/views/user/UsageView.vue
frontend/src/views/user/PaymentView.vue
frontend/src/views/user/RedeemView.vue
frontend/src/style.css
frontend/public/model-gate-mg-luxury.svg
docs/cicd-release-runbook.md
```

部署细节、镜像规则、服务器目录和生产发布流程以 [cicd-release-runbook.md](./cicd-release-runbook.md) 为准。
