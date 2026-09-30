# CI/CD 发布操作手册

本文档适用于 `qxsazz/model-gate-sub2api` 仓库的日常发布、测试环境验收和生产环境晋级。

## 1. 当前架构

```text
feature / upgrade
        |
        v
Pull Request -> staging
        |
        +-> CI 检查
        +-> Docker 构建完整镜像
        +-> 推送 GHCR
        +-> 自动部署 staging
        |
        v
人工验收 staging
        |
        v
Pull Request -> main
        |
        v
Actions -> Deploy Production -> Run workflow
        |
        +-> 根据 staging commit SHA 找到镜像
        +-> 解析镜像 digest
        +-> 确认 staging 正在运行相同 digest
        +-> 备份生产 PostgreSQL
        +-> 只更新 production 应用容器
        +-> 健康检查
```

核心原则：

- 前端在 Docker 构建流程内部完成构建，不单独发布前端 artifact。
- staging 和 production 使用同一个不可变镜像 digest。
- 服务器不执行 `git pull`、源码构建或前端构建。
- 部署脚本只更新应用容器，不主动重建 PostgreSQL 和 Redis。
- 生产发布必须由人工在 GitHub Actions 中触发。

### 1.1 MG 后台版本入口

后台侧栏的版本入口显示当前运行版本，刷新只重新查询运行版本。该入口不检测上游最新版本，也不提供下载二进制、原地更新或原地回滚按钮。旧的上游更新检查缓存不会参与托管模式的版本查询。

“部署入口”打开本仓库的 [Deploy Production workflow](https://github.com/qxsazz/model-gate-sub2api/actions/workflows/deploy-production.yml)，打开链接本身不会执行部署。发布仍须按第 4、5 节完成 staging 验收及生产晋级。

托管模式下，已通过管理员鉴权的以下请求返回 HTTP `409`，错误 reason 为 `DEPLOYMENT_MANAGED`：

| 方法 | API | 行为 |
|---|---|---|
| `POST` | `/api/v1/admin/system/update` | 拒绝原地更新 |
| `POST` | `/api/v1/admin/system/rollback` | 拒绝本地备份和指定版本回滚 |
| `GET` | `/api/v1/admin/system/rollback-versions` | 不查询上游可回滚版本 |

`GET /api/v1/admin/system/check-updates` 返回运行版本、`deployment_mode: "managed"` 和部署链接；`has_update` 为 `false`，`latest_version` 与运行版本相同。这表示部署由 CI/CD 托管，不代表上游没有新版本。

品牌配置的默认值与自定义规则见 [MG 品牌默认值](mg-brand-defaults.md)。

## 2. 环境信息

| 项目 | Staging | Production |
|---|---|---|
| GitHub Environment | `model-gate-staging` | `model-gate-production` |
| 服务器目录 | `/opt/sub2api-staging` | `/opt/sub2api` |
| Compose 项目 | `sub2api-staging` | `sub2api` |
| 应用容器 | `sub2api-staging` | `sub2api` |
| PostgreSQL 容器 | `sub2api-staging-postgres` | `sub2api-postgres` |
| Redis 容器 | `sub2api-staging-redis` | `sub2api-redis` |
| 应用端口 | `127.0.0.1:8081` | `127.0.0.1:8080` |
| 访问域名 | `https://staging.model-gate.cc` | `https://model-gate.cc` |
| API 域名 | 同应用域名 | `https://api.model-gate.cc` |

镜像仓库：

```text
ghcr.io/qxsazz/model-gate-sub2api
```

## 3. 镜像命名规则

### 3.1 日常 CI/CD 镜像

staging 构建使用 Git 提交 SHA 作为标签：

```text
ghcr.io/qxsazz/model-gate-sub2api:staging-<commit_sha>
```

例如：

```text
ghcr.io/qxsazz/model-gate-sub2api:staging-0fd44c6b230cf7ca4d75da26bd7e115e0f626de8
```

实际部署使用镜像 digest，而不是普通标签：

```text
ghcr.io/qxsazz/model-gate-sub2api@sha256:<digest>
```

生产不会重新构建镜像，也不会重新使用 `main` 重新打包，而是复用已经在 staging 验收过的 digest。

### 3.2 正式版本发布镜像

Release workflow 仍然支持版本标签，例如：

```text
ghcr.io/qxsazz/model-gate-sub2api:0.2.4
ghcr.io/qxsazz/model-gate-sub2api:0.2.4-amd64
ghcr.io/qxsazz/model-gate-sub2api:0.2.4-arm64
ghcr.io/qxsazz/model-gate-sub2api:latest
```

正式版本标签用于版本发布和兼容现有发布流程，不用于新的 staging 到 production 晋级流程。

## 4. 日常前端发布流程

### 4.1 创建功能分支

从最新 `staging` 创建分支：

```text
feature/frontend-<description>
```

前端改动应集中在：

```text
frontend/**
```

如果 PR 标记为前端改动，前端边界检查会拒绝同时修改 `backend/**`。

### 4.2 创建 staging PR

将功能分支创建 Pull Request，目标分支选择：

```text
staging
```

等待以下检查完成：

- `CI / test`
- `CI / golangci-lint`
- `Frontend CI / checks`
- `Security Scan / backend-security`
- `Security Scan / frontend-security`

`Deploy Staging` 会在 PR 事件触发后：

1. 使用 PR 提交构建完整 Docker 镜像。
2. 将镜像推送到 GHCR。
3. 上传 staging Compose 和部署脚本。
4. 保持 staging PostgreSQL 和 Redis 运行。
5. 只更新 staging 应用容器。
6. 检查 `/health` 和首页。

### 4.3 验收 staging

基础检查：

```text
https://staging.model-gate.cc/
https://staging.model-gate.cc/health
```

建议检查：

- 登录和退出登录。
- 测试管理员账号和普通测试账号。
- 用户信息和权限显示。
- 主要前端页面和路由跳转。
- 页面刷新和静态资源加载。
- 浏览器控制台是否出现新增错误。
- 测试 API 请求和错误提示。

测试环境不得使用生产账号、生产 API Key、生产渠道密钥或生产 Token。

## 5. 生产发布流程

### 5.1 创建 staging 到 main 的 PR

在 GitHub 创建：

```text
base: main
compare: staging
```

等待检查完成，并确认没有冲突后合并。

合并到 `main` 本身不会部署生产，只是让已验收的版本进入生产分支，并使生产 workflow 出现在 `main` 上。

### 5.2 记录 staging 构建 SHA

打开对应的 `Deploy Staging` 运行记录，找到成功的：

```text
Deploy Staging / build
```

记录该运行使用的 staging commit SHA。

不要默认填写 `main` 的合并提交 SHA。正常情况下应该填写成功 staging 构建对应的功能分支或升级分支提交 SHA。

### 5.3 手动运行 production workflow

进入：

```text
Actions -> Deploy Production -> Run workflow
```

填写：

```text
Use workflow from: main
source_sha: <成功 staging 构建使用的提交 SHA>
confirm: DEPLOY
```

点击 `Run workflow` 后，workflow 会：

1. 校验 `confirm` 必须是精确的 `DEPLOY`。
2. 使用 `staging-<source_sha>` 查找 GHCR 镜像。
3. 解析不可变 digest。
4. 通过 SSH 确认 staging 当前运行相同 digest。
5. 备份生产 PostgreSQL。
6. 上传 production 部署覆盖文件和部署脚本。
7. 只重建 `sub2api` 应用容器。
8. 检查 production 应用健康状态。

## 6. 数据库影响说明

### 6.1 本次部署不会重建数据库容器

生产部署使用：

```text
docker compose up -d --no-deps --force-recreate sub2api
```

该命令只针对应用服务，不主动重建：

```text
sub2api-postgres
sub2api-redis
```

### 6.2 应用启动会检查数据库迁移

应用启动时会连接 PostgreSQL，并执行迁移检查：

- 获取 PostgreSQL advisory lock。
- 检查 `schema_migrations`。
- 校验已应用迁移的 checksum。
- 执行当前镜像中尚未应用的迁移。

因此，“数据库容器不重建”不等于“数据库完全不被访问”。

当前纯前端/CI/CD 发布没有新增 `backend/migrations/**`，因此不会产生新的数据库结构迁移。

以后上游升级或后端改动包含数据库迁移时，必须在 staging 验收后再发布 production，并确认：

- 迁移是否可重复执行。
- 是否存在破坏性数据修改。
- 旧应用是否兼容新数据库结构。
- 数据库备份是否成功。
- 失败后是否需要人工恢复数据库。

应用镜像回滚不会自动回滚已经执行的数据库迁移。

## 7. 发布后检查清单

### 7.1 GitHub Actions

- [ ] `Deploy Production` 显示成功。
- [ ] `Resolve the staging image digest` 成功。
- [ ] `Verify staging and deploy production` 成功。
- [ ] 没有 SSH、GHCR、Compose 或健康检查错误。

### 7.2 服务器

```bash
docker ps
docker inspect sub2api --format '{{.Config.Image}}|{{.State.Status}}|{{.State.Health.Status}}'
docker inspect sub2api-postgres --format '{{.State.Status}}|{{.State.Health.Status}}'
docker inspect sub2api-redis --format '{{.State.Status}}|{{.State.Health.Status}}'
```

确认：

- [ ] production 应用镜像 digest 与 staging 一致。
- [ ] `sub2api` 为 `running` 且 `healthy`。
- [ ] PostgreSQL 为 `running` 且 `healthy`。
- [ ] Redis 为 `running` 且 `healthy`。
- [ ] PostgreSQL 和 Redis 的启动时间没有因发布而更新。

### 7.3 HTTP 和域名

```text
https://model-gate.cc/
https://model-gate.cc/health
https://api.model-gate.cc/health
```

确认首页、健康接口、登录页面、静态资源和主要 API 请求正常。

### 7.4 发布记录和备份

发布记录目录：

```text
/opt/sub2api/releases/
```

生产备份目录：

```text
/opt/sub2api/backups/
```

每次生产发布应至少存在：

- 一个新的 PostgreSQL dump。
- 一个新的 `deploy-<timestamp>.env` 发布记录。
- 记录中的目标镜像 digest。
- 记录中的旧镜像引用。

## 8. 失败处理

如果应用容器启动失败或健康检查失败，部署脚本会尝试使用部署前的旧应用镜像恢复服务。

需要注意：

- 自动回滚只恢复应用镜像。
- 自动回滚不会恢复数据库结构或数据。
- 回滚后仍需检查 `/health`、首页、生产域名和应用日志。
- 不要使用 `latest` 作为回滚目标。
- 不要删除 PostgreSQL、Redis、`data` 或备份目录。

常用只读检查：

```bash
docker logs --since 10m sub2api
docker compose --project-name sub2api \
  --env-file /opt/sub2api/.env \
  -f /opt/sub2api/docker-compose.yml \
  -f /opt/sub2api/docker-compose.cicd.yml config --quiet
```

当前脚本没有独立的人工回滚命令。人工回滚前必须先确认目标镜像的完整 digest，并由维护者执行和验证，不能直接把版本标签当作不可变回滚依据。

人工回滚也沿 CI/CD 镜像流程处理，不使用后台原地回滚 API。若通过现有 `Deploy Production` workflow 晋级旧镜像，需先让 staging 运行目标旧镜像并重新验收，再填写该镜像成功 staging 构建对应的 `source_sha`；workflow 仍会校验 staging 当前 digest 与目标一致。回滚前必须确认旧应用兼容当前数据库结构，不能把镜像回滚视为数据库恢复。

## 9. 当前已知边界

- 私人仓库免费方案不依赖 GitHub Environment 审批人，生产发布依靠手动 workflow 和 `DEPLOY` 确认。
- `staging` 分支没有配置与 `main` 同等严格的保护规则，发布前需要人工确认 PR 和检查结果。
- 当前部署脚本检查服务器本机健康接口；生产域名和 Caddy 需要额外进行发布后检查。
- 当前没有自动化数据库备份恢复演练。
- 当前上游检查 workflow 可以创建升级 PR，但上游升级仍需人工验收 staging 后才能进入 main 和 production。

## 10. 日常最短操作流程

```text
1. 从 staging 创建 feature / upgrade 分支
2. 创建 PR 到 staging
3. 等待 CI 和 Deploy Staging 通过
4. 验收 https://staging.model-gate.cc
5. 创建 staging -> main PR
6. 检查通过后合并
7. Actions -> Deploy Production -> Run workflow
8. Use workflow from 选择 main
9. 填写成功 staging 构建的 source_sha
10. 填写 DEPLOY
11. 运行完成后检查生产 digest、容器、数据库、Redis 和域名
```
