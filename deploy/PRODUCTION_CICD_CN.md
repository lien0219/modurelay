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

只有 `deploy/IMAGE_VERSION` 发生变化时，才额外发布版本标签：

```text
ghcr.io/lien0219/modurelay:main-v<IMAGE_VERSION>
```

当前版本文件：

```text
deploy/IMAGE_VERSION
```

发布下一版时，把它改为新的 SemVer，例如：

```text
0.0.2
```

然后按正常流程合并到 `main`。生产镜像成功后，工作流会串行生成：

```text
ghcr.io/lien0219/modurelay:main-v0.0.2
Git Tag v0.0.2
GitHub Release ModuRelay 0.0.2
```

普通代码提交不会覆盖已有的版本标签。工作流会在推送前检查
`main-v<IMAGE_VERSION>` 是否已存在；已存在时直接失败，禁止覆盖生产回退点。
手动执行镜像工作流只允许选择 `main`，用于补跑尚未生成版本标签的构建，不能从
其他分支发布生产镜像，也不能重发已有版本。生产镜像成功后才会自动创建同提交的
Git Tag 和 GitHub Release，避免镜像与 Release 并发发布产生竞态。

标签用途：

- `main`：始终指向最新主分支镜像，适合测试或手动获取最新版；
- `main-v0.0.1`：人类可读的发布版本，适合版本管理；
- `sha-*`：与某次提交一一对应，最适合生产部署、问题追踪和回滚。

## 三、GitHub Actions 行为

工作流文件：

```text
.github/workflows/publish-main-image.yml
.github/workflows/release.yml
```

触发方式：

- 推送到 `main`；
- 在 Actions 页面选择 `main` 后手动执行 `workflow_dispatch`。

普通提交只构建 `main` 和 `sha-*` 镜像。检测到 `deploy/IMAGE_VERSION` 变化时，
`publish-main-image.yml` 先发布不可变 `main-v*` 镜像，然后调用可复用的
`release.yml` 创建同提交 `v*` Tag、GitHub Release、独立二进制和
`checksums.txt`。直接推送已有 `v*` Tag 或手动运行 Release 工作流仍作为故障
恢复入口。

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
镜像任务仅申请 `packages: write`，自动 Tag/Release 任务仅申请
`contents: write` 和 `packages: read`，均使用 GitHub 自动签发的短期 Token。
如果组织策略限制 Actions 写入仓库内容，或仓库对 `v*` 配置了 Tag ruleset，需在
GitHub 设置中允许本工作流创建 Tag 和 Release，否则镜像成功后自动发布会在创建
Tag 时失败。

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

### 生产版本回退

管理后台仅从公开 GHCR Package 读取严格匹配 `main-vX.Y.Z` 的标签，并展示
最近三个低于当前版本的候选项。选择候选版本后，在生产宿主机的 `deploy`
目录执行后台给出的完整命令，例如：

```bash
./deploy-main.sh ghcr.io/lien0219/modurelay:main-v0.2.9
```

应用容器不会挂载 Docker Socket，也不会在容器内替换自身二进制。脚本会先
记录当前镜像，拉取目标镜像并只重建应用服务；目标健康检查失败时自动恢复
原镜像。镜像回退不会回退数据库，且数据库迁移仅向前执行，因此执行前必须：

1. 确认目标版本兼容当前数据库结构；
2. 保留已验证可恢复的数据库备份；
3. 记录操作者、目标镜像、开始时间和健康检查结果；
4. 不执行 `docker compose down -v`。

### GitHub Release 与生产镜像的边界

`.github/workflows/release.yml` 只发布 ModuRelay 的独立二进制压缩包和校验文件，
不再构建 DockerHub/GHCR 镜像，因此不会继续创建
`ghcr.io/lien0219/sub2api`。生产镜像只由
`.github/workflows/publish-main-image.yml` 发布到：

```text
ghcr.io/lien0219/modurelay
```

历史上已经创建的 `sub2api` Package 不会被工作流自动删除；确认没有服务器继续
引用后，需在 GitHub Package settings 中单独人工删除。删除不属于发布或回退流程。

Release 工作流会校验：Tag 必须是 `vMAJOR.MINOR.PATCH`、版本必须与
`deploy/IMAGE_VERSION` 一致、发布提交必须属于 `main`，并且对应的
`main-v<version>` 生产镜像已经存在，且生产镜像的 OCI revision 必须与
发布提交完全一致。自动流程随后在该提交创建 Tag 并生成 Release；同版本 Tag 或
镜像已指向其他提交时直接失败。仓库为私有时，独立二进制部署若要使用
后台在线更新/回退，运行环境必须配置可读取该私有仓库 Release 的
`UPDATE_GITHUB_TOKEN`；Docker 回退候选来自公开 GHCR，不依赖源码仓库公开性。

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

合并到 `main` 后无需再手工创建 Tag：等待 `Publish Main Image` 完成，即可同时
得到 `main-v0.0.2` Package、`v0.0.2` Tag 和 `ModuRelay 0.0.2` Release。

生产环境推荐优先使用：

```text
ghcr.io/lien0219/modurelay:main-v0.0.2
```

要求与具体提交绝对一致时，使用：

```text
ghcr.io/lien0219/modurelay:sha-<完整提交 SHA>
```
