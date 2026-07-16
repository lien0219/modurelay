# ModuRelay Custom Changelog

## Unreleased

### Added
- 新增 ModuRelay 品牌体系
- 新增二开分支和上游同步规范
- 新增前端集中品牌配置 `frontend/src/config/brand.ts`
- 新增临时品牌图标 `frontend/public/modurelay-mark.svg`
- 新增 `BRANDING.md` 与 `NOTICE.md`
- 重写 `README.md` / `README_CN.md` / `README_JA.md` 为 ModuRelay 官方说明文档

### Changed
- 用户可见默认品牌由 Sub2API 调整为 ModuRelay（前端标题、登录相关布局、引导文案、设置默认值、GitHub 链接）
- 后端默认站点名与合规确认短语调整为 ModuRelay
- 前端包名调整为 `modurelay-frontend`
- 移除指向上游促销站点的代理广告外链
- 移除 README 中上游赞助商广告、邀请码与推广邮箱，改为指向上游原仓库查看赞助信息

### Fixed
- 修复品牌改造引入的空 Vue 模板 ESLint 错误与 Go 文件 UTF-8 BOM
- 对齐 API contract 默认 `site_name` 断言为 ModuRelay
