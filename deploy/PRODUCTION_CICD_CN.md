# ModuRelay GHCR 制品发布与手动部署

该方案只负责构建并发布 Docker 镜像，不会从 GitHub Actions 连接任何服务器。

```text
develop 开发
  -> PR 合并到 main
  -> GitHub Actions 构建 linux/amd64 镜像
  -> 推送 ghcr.io/lien0219/modurelay:main
  -> 同时推送 ghcr.io/lien0219/modurelay:sha-<完整提交 SHA>
  -> 流程结束
```

腾讯云或其他服务器不会被自动访问，也不会自动拉取、重启或修改容器。

## 一、GitHub Actions 行为

工作流文件：

```text
.github/workflows/publish-main-image.yml
```

触发方式：

- 推送到 `main`；
- 在 Actions 页面手动执行 `workflow_dispatch`。

每次构建发布两个标签：

```text
ghcr.io/lien0219/modurelay:main
ghcr.io/lien0219/modurelay:sha-<完整提交 SHA>
```

标签用途：

- `main`：始终指向最新的主分支镜像；
- `sha-*`：不可变版本标签，适合上线、回滚和问题追踪。

## 二、GitHub 配置

进入：

```text
Settings -> Secrets and variables -> Actions
```

### Secrets

本工作流不需要配置腾讯云相关 Secret，也不需要手动创建 `GITHUB_TOKEN`。

以下项目均不需要：

```text
TENCENT_SERVER_HOST
TENCENT_SERVER_PORT
TENCENT_SERVER_USER
TENCENT_SERVER_SSH_KEY
TENCENT_SERVER_KNOWN_HOSTS
GHCR_READ_TOKEN
```

GitHub Actions 会自动提供 `GITHUB_TOKEN`，工作流使用它登录 GHCR 并推送当前仓库的容器镜像。

### Variables

只有一个可选变量：

| 名称 | 建议值 | 说明 |
| --- | --- | --- |
| `NPM_CONFIG_REGISTRY` | `https://registry.npmmirror.com` | 前端依赖下载镜像源；不配置时工作流也会使用该默认值 |

不再需要：

```text
ENABLE_PRODUCTION_DEPLOY
TENCENT_DEPLOY_PATH
```

也不需要创建 `production` Environment。

## 三、验证制品发布

把 `develop` 合并到 `main` 后，进入：

```text
Actions -> Publish Main Image
```

确认唯一的 Job：

```text
Build and publish GHCR image
```

执行成功后，进入仓库首页右侧的 Packages，应该能看到：

```text
modurelay
```

镜像标签应包含：

```text
main
sha-<本次 main 提交的完整 SHA>
```

## 四、设置 GHCR 镜像可见性

GHCR 容器包可能默认是私有的。

进入：

```text
仓库首页 -> Packages -> modurelay -> Package settings
```

按实际需要选择：

- 保持私有：拉取镜像的机器需要登录 GHCR；
- 改为 Public：任何机器都可匿名拉取。

公开仓库不代表对应的 GHCR Package 一定自动公开，需要单独确认 Package 可见性。

## 五、手动拉取镜像

### 公开镜像

```bash
docker pull ghcr.io/lien0219/modurelay:main
```

或拉取固定版本：

```bash
docker pull ghcr.io/lien0219/modurelay:sha-<完整提交 SHA>
```

### 私有镜像

创建一个只包含 `read:packages` 权限的 GitHub PAT，然后在需要拉取镜像的机器执行：

```bash
echo '<PAT>' | docker login ghcr.io -u lien0219 --password-stdin
docker pull ghcr.io/lien0219/modurelay:main
```

该 Token 只保存在实际拉取镜像的机器，不需要放入当前 GitHub Actions 工作流。

## 六、可选的服务器手动更新

仓库仍保留：

```text
deploy/docker-compose.prod.yml
deploy/deploy-main.sh
deploy/production.env.example
```

这些文件只是提供人工部署能力，不会由 GitHub Actions 自动执行。

在服务器上更新固定 SHA 镜像：

```bash
cd /www/apps/modurelay/deploy
chmod 700 deploy-main.sh
./deploy-main.sh ghcr.io/lien0219/modurelay:sha-<完整提交 SHA>
```

脚本会：

1. 校验 Compose 和服务器本地 `.env`；
2. 拉取指定 SHA 镜像；
3. 使用 `--no-deps --force-recreate modurelay` 只更新应用容器；
4. 等待健康检查；
5. 失败时尝试回滚；
6. 不重新创建 PostgreSQL 和 Redis。

也可以完全不用这些部署文件，只把 GHCR 当作镜像制品仓库。

## 七、首次从开发 Compose 切到生产 Compose

只有决定在服务器使用生产 Compose 时才需要执行本节。

首先备份：

```bash
cd /www/apps/modurelay/deploy

docker exec sub2api-postgres-dev \
  pg_dump -U "${POSTGRES_USER:-sub2api}" "${POSTGRES_DB:-sub2api}" \
  > "postgres-before-prod-$(date +%Y%m%d-%H%M%S).sql"

tar czf "modurelay-data-before-prod-$(date +%Y%m%d-%H%M%S).tar.gz" \
  data postgres_data redis_data .env
```

严禁执行：

```bash
docker compose down -v
rm -rf data postgres_data redis_data
```

首次切换：

```bash
cd /www/apps/modurelay/deploy

docker compose -f docker-compose.dev.yml down

docker compose \
  --env-file .env \
  -f docker-compose.prod.yml \
  pull

docker compose \
  --env-file .env \
  -f docker-compose.prod.yml \
  up -d
```

检查：

```bash
docker compose --env-file .env -f docker-compose.prod.yml ps
docker inspect modurelay --format '{{.State.Health.Status}}'
curl -fsS http://127.0.0.1:8080/health
```

## 八、日常发布建议

正常流程：

```text
feature/fix 分支 -> develop -> PR -> main
```

合并后只产生镜像制品。实际环境是否升级、何时升级以及升级到哪个 SHA，由运维人员自行决定。

生产环境推荐固定 SHA：

```text
ghcr.io/lien0219/modurelay:sha-<完整提交 SHA>
```

不建议生产环境长期直接使用浮动的 `main` 标签，因为它不利于精确追踪和回滚。
