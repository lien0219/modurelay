# ModuRelay 生产环境 CI/CD

该方案实现以下流程：

```text
develop 开发
  -> PR 合并到 main
  -> GitHub Actions 构建 linux/amd64 镜像
  -> 推送 ghcr.io/lien0219/modurelay:main
  -> 同时推送 ghcr.io/lien0219/modurelay:sha-<完整提交 SHA>
  -> SSH 登录腾讯云
  -> 拉取不可变 SHA 镜像
  -> 只重新创建 ModuRelay 应用容器
  -> PostgreSQL 和 Redis 保持运行
```

自动部署默认关闭。`main` 推送后会先构建并发布镜像；只有仓库变量 `ENABLE_PRODUCTION_DEPLOY=true` 时才会执行腾讯云部署。

## 一、GitHub 配置

进入仓库：

```text
Settings -> Secrets and variables -> Actions
```

### Repository secrets

| 名称 | 必填 | 说明 |
| --- | --- | --- |
| `TENCENT_SERVER_HOST` | 是 | 腾讯云服务器公网 IP 或域名 |
| `TENCENT_SERVER_PORT` | 否 | SSH 端口，默认 `22` |
| `TENCENT_SERVER_USER` | 是 | SSH 用户，例如 `root` 或独立部署用户 |
| `TENCENT_SERVER_SSH_KEY` | 是 | 对应服务器公钥的私钥全文 |
| `TENCENT_SERVER_KNOWN_HOSTS` | 是 | 服务器 SSH 主机指纹，禁止关闭严格校验 |
| `GHCR_READ_TOKEN` | 私有镜像时必填 | 具有 `read:packages` 权限的 GitHub PAT；公开镜像可留空 |

在本地可信环境生成 `known_hosts` 内容：

```bash
ssh-keyscan -p 22 -H <服务器地址>
```

核对服务器真实指纹后，把完整输出保存为 `TENCENT_SERVER_KNOWN_HOSTS`。

### Repository variables

| 名称 | 建议值 | 说明 |
| --- | --- | --- |
| `ENABLE_PRODUCTION_DEPLOY` | 初始为 `false` | 首次人工切换成功后改成 `true` |
| `TENCENT_DEPLOY_PATH` | `/www/apps/modurelay/deploy` | 服务器部署目录 |
| `NPM_CONFIG_REGISTRY` | `https://registry.npmmirror.com` | 可选的前端依赖镜像源 |

建议创建 GitHub `production` Environment，并按需开启审批保护。工作流的部署 Job 使用该 Environment。

## 二、服务器首次准备

首次切换会停止旧开发栈并启动生产栈，因此 PostgreSQL 和 Redis 会在这一次切换中重启。之后的自动发布只更新应用容器。

### 1. 备份当前数据

假设当前目录为 `/www/apps/modurelay/deploy`：

```bash
cd /www/apps/modurelay/deploy

docker compose -f docker-compose.dev.yml ps

docker exec sub2api-postgres-dev \
  pg_dump -U "${POSTGRES_USER:-sub2api}" "${POSTGRES_DB:-sub2api}" \
  > "postgres-before-prod-$(date +%Y%m%d-%H%M%S).sql"

tar czf "modurelay-data-before-prod-$(date +%Y%m%d-%H%M%S).tar.gz" \
  data postgres_data redis_data .env
```

不要执行：

```bash
docker compose down -v
rm -rf data postgres_data redis_data
```

### 2. 准备生产文件

把以下文件放到部署目录：

```text
docker-compose.prod.yml
deploy-main.sh
production.env.example
```

保留原来的 `.env`。首次可以对照模板补齐必要字段：

```bash
cd /www/apps/modurelay/deploy
chmod 600 .env
nano .env
```

至少确认：

```text
POSTGRES_PASSWORD
JWT_SECRET
TOTP_ENCRYPTION_KEY
```

如果宿主机没有代理服务，应让大小写两套代理变量保持为空。如果宿主机实际使用 HTTP `7891`、SOCKS5 `7892`，按 `production.env.example` 的示例分别配置，不要把错误协议和端口混用。

### 3. 登录 GHCR

镜像为公开包时可直接拉取。私有包需要：

```bash
echo '<PAT>' | docker login ghcr.io -u lien0219 --password-stdin
```

PAT 只需要 `read:packages`。

### 4. 停止旧开发栈并启动生产栈

确保当前目录仍然包含原来的 `data/`、`postgres_data/` 和 `redis_data/`：

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

这里没有使用 `-v`，绑定目录数据不会被删除。

检查：

```bash
docker compose --env-file .env -f docker-compose.prod.yml ps
docker inspect modurelay --format '{{.State.Health.Status}}'
curl -fsS http://127.0.0.1:8080/health
```

同时确认数据仍然存在、管理员可以登录、模型请求正常。

## 三、首次验证 GitHub Actions

先保持：

```text
ENABLE_PRODUCTION_DEPLOY=false
```

把 `develop` 合并到 `main` 后，检查 Actions 中 `Build and Deploy Main Image`：

1. `build` Job 成功；
2. GHCR 中存在 `main` 标签；
3. GHCR 中存在 `sha-<完整提交 SHA>` 标签；
4. `deploy` Job 因变量未开启而跳过。

然后手动测试服务器更新：

```bash
cd /www/apps/modurelay/deploy
chmod 700 deploy-main.sh
./deploy-main.sh ghcr.io/lien0219/modurelay:sha-<完整提交 SHA>
```

脚本会：

1. 校验 Compose 和 `.env`；
2. 拉取指定 SHA 镜像；
3. 使用 `--no-deps --force-recreate modurelay` 只更新应用容器；
4. 等待健康检查；
5. 失败时尝试回滚到更新前镜像；
6. PostgreSQL、Redis 不重启。

验证成功后，将仓库变量改为：

```text
ENABLE_PRODUCTION_DEPLOY=true
```

## 四、日常发布

以后按正常分支流程：

```text
feature/fix 分支 -> develop -> PR -> main
```

每次 `main` 更新会发布两个标签：

```text
ghcr.io/lien0219/modurelay:main
ghcr.io/lien0219/modurelay:sha-<完整提交 SHA>
```

服务器实际部署使用 SHA 标签，而不是浮动的 `main` 标签，确保发布过程可追溯、可回滚。

## 五、故障排查

### 应用更新状态

```bash
cd /www/apps/modurelay/deploy
cat .deployed-image
docker inspect modurelay --format 'Image={{.Config.Image}} Status={{.State.Status}} Health={{.State.Health.Status}} OOMKilled={{.State.OOMKilled}}'
docker logs --tail 300 modurelay
```

### 确认数据库和 Redis 没有被重启

```bash
docker inspect modurelay-postgres --format 'StartedAt={{.State.StartedAt}} RestartCount={{.RestartCount}}'
docker inspect modurelay-redis --format 'StartedAt={{.State.StartedAt}} RestartCount={{.RestartCount}}'
```

### 手动回滚

```bash
cd /www/apps/modurelay/deploy
./deploy-main.sh ghcr.io/lien0219/modurelay:sha-<上一个正常提交 SHA>
```

### 查看系统资源

```bash
free -h
df -h
docker stats --no-stream
sudo journalctl -k --since '24 hours ago' | grep -Ei 'oom|out of memory|killed process'
```
