# ModuRelay 品牌规范

## 品牌定义

| 项目 | 规范 |
| --- | --- |
| 品牌名称 | `ModuRelay`（不得写作 `Modurelay`、`modu relay` 或全大写） |
| 完整名称 | ModuRelay AI Gateway |
| 中文定位 | 开源多模型接入、路由、账号池、计量与 API 管理平台 |
| English description | An open-source AI gateway for multi-provider routing, account pooling, usage metering and API management. |
| 中文标语 | 一次接入，自由路由所有模型 |
| English tagline | Connect once. Route any model. |
| GitHub repository | `lien0219/modurelay` |

## 目标命名

以下是新增安装与发布产物的目标命名。迁移现有部署前必须提供兼容路径和回滚方案，不得仅因名称统一而破坏用户环境。

| 项目 | 目标名称 |
| --- | --- |
| repository | `modurelay` |
| frontend package | `modurelay-frontend` |
| binary | `modurelay` |
| Docker service | `modurelay` |
| Docker container | `modurelay-server` |
| systemd service | `modurelay.service` |
| install directory | `/opt/modurelay` |
| config directory | `/etc/modurelay` |
| data directory | `/var/lib/modurelay` |
| log directory | `/var/log/modurelay` |
| environment prefix | `MODURELAY_` |

## 用户可见品牌替换规则

- 页面标题、meta description、登录/注册/初始化页面、导航、页脚、关于页和默认说明使用 ModuRelay 品牌。
- 默认 GitHub 链接使用 `https://github.com/lien0219/modurelay`。
- 默认 Logo、favicon 与图片替代文本使用 ModuRelay；正式视觉资产发布前使用标注为临时的文字或字母图标。
- 站点管理员可通过既有 `site_name`、`site_logo` 和 `site_subtitle` 设置覆盖默认展示；新代码必须把 ModuRelay 作为默认值。
- 用户可见部署文档采用 ModuRelay 名称；涉及已有服务、容器、路径或数据卷时须明确迁移步骤。

## 暂时保留的旧标识

以下标识是兼容性保留，不进行无审查替换：

- `backend/go.mod` 及 Go 源码中的 `github.com/Wei-Shaw/sub2api` module/import 路径。
- Ent 迁移历史、数据库表名、字段名、默认数据库名、缓存键和浏览器本地存储键。
- `/v1/*` 等既有 API 路径、WebSocket 子协议、Provider/模型名称和外部协议字段。
- 既有 Docker 服务、容器、镜像、systemd、安装目录、配置目录、数据目录、日志目录和部署脚本中的 `sub2api` 标识，直至提供兼容迁移。
- Docker 镜像名 `weishaw/sub2api`（VersionBadge 回滚提示仍引用，待 ModuRelay 镜像发布后迁移）。
- TOTP issuer `Sub2API`（变更会导致已绑定验证器失效）。
- Setup 向导默认数据库名 `sub2api` 及导入数据格式标识 `sub2api-data` / `sub2api-bundle`。
- `LICENSE`、版权声明、上游仓库链接和上游同步文档中的 Sub2API 信息。
- 合规文档正文中仍含上游项目名称的历史表述（短语与文档链接已指向 ModuRelay 仓库副本）。

## 不允许修改的兼容字段

- Go module 路径和现有 Go imports。
- Ent 数据库迁移历史、已有表名和字段名。
- 已公开兼容的 API 路径与协议字段。
- Provider 名称、模型名称及外部协议字段。
- `LICENSE` 内容和 Git 历史。
- 现网部署使用的二进制路径、systemd 单元名、Docker Compose 服务名与数据卷名（需单独迁移方案）。

## 后续品牌升级清单

1. 发布正式 Logo、深色 Logo、favicon、Apple touch icon 和社交预览图。
2. 为 Docker、二进制、systemd 单元及目录名称设计兼容迁移和回滚方案。
3. 在确认用户升级路径后，逐步引入 `MODURELAY_` 环境变量，并保留现有配置 fallback。
4. 更新发行镜像、发布工作流和安装文档，避免触发上游发布或复用上游镜像标签。
5. 在每次上游同步后复核用户可见品牌、上游版权和兼容性保留项。
6. 官方 README（`README.md` / `README_CN.md` / `README_JA.md`）以 ModuRelay 项目文档为准，仅在归属与上游说明中引用 Sub2API。
