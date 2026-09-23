# ModuRelay Notice

ModuRelay 是基于 [Sub2API](https://github.com/Wei-Shaw/sub2api)（仓库：`Wei-Shaw/sub2api`）独立维护的衍生开源项目。

## 关系说明

- ModuRelay **不是** Sub2API 官方版本，也不代表上游官方立场。
- ModuRelay 与 Sub2API 及其维护者之间 **不存在** 官方隶属、合作或背书关系。
- 不得声称 ModuRelay 已获得上游或任何 AI 供应商的官方授权。

## 许可证与版权

- 本仓库继续保留并遵循根目录 [LICENSE](LICENSE)（GNU LGPL v3）及其中上游版权说明。
- 不得删除 `LICENSE`、上游版权声明或本文件中的归属说明。
- 上游项目请访问：<https://github.com/Wei-Shaw/sub2api>

无限画布功能包含以下 MIT 许可项目的原版 React 应用及 Vue 3 适配版本：

- 项目：[`basketikun/infinite-canvas`](https://github.com/basketikun/infinite-canvas)
- 参考提交：`d213a74614e0e4bd8a26383d1e1e907249e9c61b`
- Copyright (c) 2026 basketikun
- 许可证：MIT License。其版权声明和许可声明保留在本仓库的发布源码
  `third_party/infinite-canvas/LICENSE` 中；发布源代码或其实质性部分时必须一并保留。

原版 React 应用仅针对同源子路径、导航、主题、语言和显式网关连接进行了集成；此前的
Vue 3、Pinia、Vue Flow 版本仍作为 ModuRelay 适配实现保留。以上均不表示原作者与
ModuRelay 存在官方合作或背书关系。

## ModuRelay 修改范围

相对于上游，ModuRelay 的修改主要包括：

- 品牌与界面默认展示（见 [BRANDING.md](BRANDING.md)）
- 官方说明文档（`README.md` / `README_CN.md` / `README_JA.md`）
- 协作与上游同步文档（`docs/BRANCHING.md`、`UPSTREAM.md`、`CUSTOM_CHANGELOG.md`）
- 部署配置与后续功能扩展（以实际代码与变更为准）

本说明不声明或编造任何商标、公司主体或商业授权信息。赞助商推广内容属于上游项目时，不会作为 ModuRelay 赞助关系转载。
