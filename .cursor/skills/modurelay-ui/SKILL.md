---
name: modurelay-ui
description: >-
  ModuRelay Vue 3 UI system: refactor pages, dashboards, tables, forms, dialogs,
  sidebar, header, theme, responsive layout, motion (CSS/motion-v/GSAP), and UX
  polish against design-system/MASTER.md. Use for UI refactor, page design,
  Dashboard, table, form, modal, Sidebar, Header, theme, responsive, animation,
  or UX optimization work in this repo.
---

# ModuRelay UI Skill

Project-specific UI skill for the ModuRelay frontend (Vue 3 · TypeScript · Tailwind 3 · Pinia · Vue Router · Vue I18n · Chart.js · motion-v · GSAP).

## When to use

Apply this skill for requests involving:

- UI 重构 / UI refactor
- 页面设计 / page design
- Dashboard
- 表格 / tables
- 表单 / forms
- 弹窗 / modal / dialog / drawer
- Sidebar / Header / AppShell
- 主题 / theme / dark mode
- 响应式 / responsive
- 动画 / animation / motion / GSAP
- UX 优化

## Mandatory workflow

1. **Read** `design-system/MASTER.md` first (source of truth).
2. If a page override exists at `design-system/pages/<page>.md`, apply it **over** MASTER for that page.
3. Read `docs/UI_REFACTOR_PLAN.md` and stay within the relevant phase scope.
4. Inspect existing Vue patterns in `frontend/src/components` and `frontend/src/style.css` before inventing new ones.
5. Implement **one module at a time** (one view family or one primitive).
6. Run in `frontend/`:
   - `pnpm typecheck`
   - `pnpm build`
7. Do not commit or push unless the user explicitly asks.

## Hard constraints (verbatim requirements)

- 先读取 design-system/MASTER.md
- 保持 Vue 3、TypeScript、Pinia、Router 和 i18n
- 不修改后端接口
- 不改变业务数据结构
- 不引入第二套大型 UI 框架
- 优先复用和抽象组件
- 每次只渐进修改一个模块
- 每次运行 typecheck 和 build
- Motion 与 GSAP 不控制同一个属性
- 必须清理 GSAP context、timeline 和 ScrollTrigger
- 必须支持 prefers-reduced-motion

## Stack rules

| Do | Don't |
|----|-------|
| Vue 3 `<script setup>` + TypeScript | Rewrite in React/Svelte |
| Pinia stores for UI state | Parallel ad-hoc global event buses for theme/sidebar |
| Vue Router + existing layouts | New router architecture |
| vue-i18n strings | Hardcoded user-facing copy (unless matching existing local patterns temporarily) |
| Tailwind tokens (`primary`, `dark`, `gray`, existing component classes) | New hex soup / second design kit |
| Extend `style.css` `@layer components` | Import Element Plus / Ant Design Vue / Naive as system |
| Chart.js existing wrappers | Replace chart library casually |

## Visual direction (summary)

- **AI-Native Enterprise Dashboard + Swiss Minimalism + Data-Dense + restrained Bento + Subtle Dimensional Layering**
- Brand accent: existing **teal `primary`** — keep
- Surfaces: slate `dark-*` + gray — keep
- Deprecate duplicate **`accent-*`** (identical to `dark-*`)
- Avoid: purple-pink AI gradients, heavy glass, glow spam, oversized radius, floating every card, emoji icons, low-contrast gray text

## Animation ownership

### CSS / Tailwind

- 普通颜色过渡
- focus
- 简单 hover
- 基础 loading

### motion-v

- 按钮 press
- Modal / Drawer
- Dropdown
- 列表进入退出
- 布局变化
- 页面局部过渡

### GSAP

- Dashboard 分阶段进入
- 数字计数
- SVG 和拓扑线路
- 多元素 Timeline
- 复杂图表首屏动画
- ScrollTrigger 场景

### Forbidden

- 同一元素的 transform 同时由 Motion 和 GSAP 控制
- 表格几十行逐行长时间入场
- 动画阻塞点击
- 组件卸载后保留 Timeline
- 未处理 reduced motion

### GSAP cleanup (required)

In `onBeforeUnmount` (or composable dispose):

- `ctx.revert()` if using `gsap.context`
- `timeline.kill()`
- Kill ScrollTriggers created by the component
- Never leave tweens targeting unmounted DOM

### Reduced motion (required)

- Honor `prefers-reduced-motion: reduce`
- Skip choreography; set final visual state immediately
- Do not leave content at `opacity: 0`

## Component preferences

Reuse before create:

- Layout: `AppLayout`, `AppSidebar`, `AppHeader`, `TablePageLayout`, `AuthLayout`
- Common: `DataTable`, `BaseDialog`, `ConfirmDialog`, `Input`, `Select`, `TextArea`, `Toggle`, `Pagination`, `EmptyState`, `Skeleton`, `LoadingSpinner`, `StatCard`, `Toast`, badges
- Charts: `components/charts/*`
- Icons: `components/icons/Icon.vue` + provider icons — **no emoji as functional icons**

When abstracting: prefer improving an existing common component over a one-off copy in a view.

## Out of scope

- Backend Go services, Ent schemas, API routes, OpenAPI contracts
- Changing request/response field names or business algorithms for styling
- Replacing the design token system with another kit
- Brand compatibility identifiers listed in `BRANDING.md` as frozen

## Implementation checklist

- [ ] Read `design-system/MASTER.md`
- [ ] Scoped to one module / phase
- [ ] Tokens via Tailwind / existing classes
- [ ] No backend or DTO changes
- [ ] Motion ownership respected + GSAP cleaned up
- [ ] `prefers-reduced-motion` handled
- [ ] `pnpm typecheck` and `pnpm build` in `frontend/`
- [ ] No commit unless user requested
