# ModuRelay GHCR 制品发布与手动部署

该方案只负责构建并发布 Docker 镜像，不会从 GitHub Actions 连接任何服务器。

```text
develop 开发
  -> PR 合并到 main
  -> GitHub Actions 构建 linux/amd64 镜像
  -> 推送 GHCR 制品
  -> 流程结束
```

腾讯云或其他服务器不会被自动访问，也不会自动拉取、重启或修改容器。

## 一、镜像地址和拉取命令

最新 `main` 镜像的完整拉取命令：

```bash
docker pull ghcr.io/lien0219/modurelay:main
```

首次发布 `0.0.1` 时还会生成：

```bash
docker pull ghcr.io/lien0219/modurelay:main-v0.0.1
```

每次构建都会生成精确 SHA 标签：

```bash
docker pull ghcr.io/lien0219/modurelay:sha-<完整提交 SHA>
```

## 二、标签规则

每次 `main` 更新都会发布：

```text
ghcr.io/lien0219/modurelay:main
ghcr.io/lien0219/modurelay:sha-<完整提交 SHA>
```

只有 `deploy/IMAGE_VERSION` 发生变化，或者手动执行工作流时，才额外发布版本标签：

```text
ghcr.io/lien0219/modurelay:main-v<IMAGE_VERSION>
```

当前版本文件：

```text
deploy/IMAGE_VERSION
```

当前内容：

```text
0.0.1
```

发布下一版时，把它改为：

```text
0.0.2
```

然后按正常流程合并到 `main`，工作流就会额外生成：

```text
ghcr.io/lien0219/modurelay:main-v0.0.2
```

这样普通代码提交不会覆盖已有的版本标签。

标签用途：

- `main`：始终指向最新主分支镜像，适合测试或手动获取最新版；
- `main-v0.0.1`：人类可读的发布版本，适合版本管理；
- `sha-*`：与某次提交一一对应，最适合生产部署、问题追踪和回滚。

## 三、GitHub Actions 行为

工作流文件：

```text
.github/workflows/publish-main-image.yml
```

触发方式：

- 推送到 `main`；
- 在 Actions 页面手动执行 `workflow_dispatch`。

构建平台：

```text
linux/amd64
```

制品仓库：

```text
ghcr.io/lien0219/modurelay
```

## 四、GitHub 配置

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

镜像版本不放在 Repository Variable 中，而是由仓库文件 `deploy/IMAGE_VERSION` 管理，方便代码审查、历史追踪和回滚。

## 五、验证制品发布

把 `develop` 合并到 `main` 后，进入：

```text
Actions -> Publish Main Image
```

确认 Job：

```text
Build and publish GHCR image
```

第一次成功后，仓库 Packages 中应看到：

```text
main
main-v0.0.1
sha-<本次 main 提交的完整 SHA>
```

后续没有修改 `deploy/IMAGE_VERSION` 的普通提交只会更新 `main` 并新增 `sha-*`，不会覆盖 `main-v0.0.1`。

## 六、设置 GHCR 镜像可见性

GHCR 容器包可能默认是私有的。

进入：

```text
仓库首页 -> Packages -> modurelay -> Package settings
```

按实际需要选择：

- 保持私有：拉取镜像的机器需要登录 GHCR；
- 改为 Public：任何机器都可匿名拉取。

公开仓库不代表对应的 GHCR Package 一定自动公开，需要单独确认 Package 可见性。

## 七、私有镜像拉取

创建只具有 `read:packages` 权限的 GitHub PAT，然后在需要拉取镜像的机器执行：

```bash
echo '<PAT>' | docker login ghcr.io -u lien0219 --password-stdin
docker pull ghcr.io/lien0219/modurelay:main-v0.0.1
```

Token 只保存在实际拉取镜像的机器，不需要放入当前 GitHub Actions 工作流。

## 八、可选的服务器手动更新

仓库仍保留：

```text
deploy/docker-compose.prod.yml
deploy/deploy-main.sh
deploy/production.env.example
```

这些文件只是提供人工部署能力，不会由 GitHub Actions 自动执行。

推荐服务器部署固定版本或固定 SHA：

```bash
cd /www/apps/modurelay/deploy
chmod 700 deploy-main.sh
./deploy-main.sh ghcr.io/lien0219/modurelay:main-v0.0.1
```

或者：

```bash
./deploy-main.sh ghcr.io/lien0219/modurelay:sha-<完整提交 SHA>
```

脚本会：

1. 校验 Compose 和服务器本地 `.env`；
2. 拉取指定镜像；
3. 使用 `--no-deps --force-recreate modurelay` 只更新应用容器；
4. 等待健康检查；
5. 失败时尝试回滚；
6. 不重新创建 PostgreSQL 和 Redis。

也可以完全不用这些部署文件，只把 GHCR 当作镜像制品仓库。

## 九、版本发布建议

正常流程：

```text
feature/fix 分支 -> develop -> PR -> main
```

普通提交不修改 `deploy/IMAGE_VERSION`。

准备正式发布新版本时，再修改版本文件，例如：

```text
0.0.1 -> 0.0.2
```

生产环境推荐优先使用：

```text
ghcr.io/lien0219/modurelay:main-v0.0.2
```

要求与具体提交绝对一致时，使用：

```text
ghcr.io/lien0219/modurelay:sha-<完整提交 SHA>
```
