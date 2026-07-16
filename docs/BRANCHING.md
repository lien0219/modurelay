# ModuRelay 分支协作规范

## 仓库远程关系

- `origin`：ModuRelay 二开仓库（`https://github.com/lien0219/modurelay.git`）。
- `upstream`：Sub2API 上游仓库（`https://github.com/Wei-Shaw/sub2api.git`）。

## 分支职责

| 分支 | 职责 | 二开规则 |
| --- | --- | --- |
| `upstream-main` | `upstream/main` 的镜像分支 | 必须与上游保持一致，不允许二开修改 |
| `main` | ModuRelay 稳定发布分支 | 仅接收经过验收的发布内容 |
| `develop` | ModuRelay 日常集成开发分支 | 汇集功能、修复和上游同步内容 |
| `feature/*` | 普通功能开发分支 | 从 `develop` 创建并合并回 `develop` |
| `fix/*` | 普通问题修复分支 | 从 `develop` 创建并合并回 `develop` |
| `hotfix/*` | 生产紧急修复分支 | 用于紧急修复；完成后按发布流程回合并 |

## 标准开发流程

### 普通功能

`develop` → `feature/xxx` → 开发和测试 → 合并回 `develop` → 验收 → 合并到 `main`

### 品牌开发

`develop` → `feature/branding` → 修改品牌 → 前后端构建验证 → 合并回 `develop`

## 分支命名规范

分支名使用小写英文和连字符，并通过前缀表达工作类型：

```text
feature/custom-provider
feature/branding
feature/tenant-management
fix/login-redirect
hotfix/payment-callback
```

## Commit Message 规范

采用 Conventional Commits。格式为：`<type>: <中文说明>`。

| 类型 | 用途 | 中文示例 |
| --- | --- | --- |
| `feat:` | 新功能 | `feat: 新增自定义供应商配置` |
| `fix:` | 缺陷修复 | `fix: 修复登录后跳转地址丢失问题` |
| `docs:` | 文档变更 | `docs: 补充分支协作规范` |
| `refactor:` | 重构 | `refactor: 拆分订阅解析服务` |
| `chore:` | 杂项维护 | `chore: 更新本地开发工具配置` |
| `test:` | 测试 | `test: 增加租户管理接口测试` |
| `build:` | 构建系统或依赖 | `build: 更新前端构建依赖` |
| `ci:` | 持续集成 | `ci: 增加后端测试检查` |

## Pull Request 规范

- PR 必须从 `feature/*` 或 `fix/*` 分支提交到 `develop`。
- `main` 不直接接受日常功能提交。
- `upstream-main` 不接受任何二开提交。
- PR 描述必须包含修改内容、影响范围、测试结果和截图。
- 不允许在包含未提交修改时同步上游。

## 上游同步流程

同步前先创建备份分支或标签。执行以下命令：

```bash
git fetch upstream
git switch upstream-main
git reset --hard upstream/main
git push --force-with-lease origin upstream-main

git switch develop
git merge upstream-main
git push origin develop
```

- `upstream-main` 必须与 `upstream/main` 保持一致。
- 不允许把 `develop` 或 `main` 合并进 `upstream-main`。
- 发生冲突时在 `develop` 上处理。

## 发布流程

`develop` → 测试通过 → `main` → 创建 ModuRelay 自己的版本标签

不要直接推送上游全部 tags，也不要误触发上游发布标签。

## 禁止事项

- 禁止在 `upstream-main` 开发。
- 禁止直接 force push `main`。
- 禁止无审查地全局替换 `sub2api`。
- 禁止删除 `LICENSE` 和上游版权声明。
- 禁止随意修改数据库迁移历史。
- 禁止直接修改已经对外兼容的 API 路径。
