# ModuRelay Upstream 说明

- Upstream project：Wei-Shaw/sub2api
- Upstream branch：`upstream/main`
- Mirror branch：`upstream-main`
- Development branch：`develop`
- Stable branch：`main`
- Branding branch：`feature/branding`
- 当前二开项目：ModuRelay

本项目是独立维护的衍生项目。本项目不代表上游官方，也未获得上游背书。

更多对外说明见：

- [README.md](README.md) / [README_CN.md](README_CN.md)
- [NOTICE.md](NOTICE.md)
- [BRANDING.md](BRANDING.md)
- [docs/BRANCHING.md](docs/BRANCHING.md)

## 上游同步命令

同步前请先创建备份分支或标签，并确认工作区没有未提交修改。

```bash
git fetch upstream
git switch upstream-main
git reset --hard upstream/main
git push --force-with-lease origin upstream-main

git switch develop
git merge upstream-main
git push origin develop
```

## 冲突解决规则

- `upstream-main` 仅镜像 `upstream/main`，不得在此分支处理冲突或提交二开内容。
- 将 `upstream-main` 合并到 `develop` 时发生冲突，必须在 `develop` 上解决。
- 不允许将 `develop` 或 `main` 合并进 `upstream-main`。
- 解决冲突后应完成构建和相关测试，再提交合并结果。

## 最近同步记录

| 日期 | 上游 Commit | ModuRelay 分支 | 操作人 | 结果 | 备注 |
| --- | --- | --- | --- | --- | --- |
| 2026-07-16 | eb2b8632ded614bf991d7d36abfa38b513ad8c2d | feature/branding | 初始化 | 已记录 | 初始化二开仓库同步记录 |
