# ModuRelay UI Refactor Plan

> Scope: frontend visual system only. **No backend / API / data-shape changes. No commits in the audit phase.**  
> Authority: [design-system/MASTER.md](../design-system/MASTER.md) · Brand: [BRANDING.md](../BRANDING.md)

---

## Guiding Constraints

- Vue 3 + `<script setup>` + TypeScript + Pinia + Vue Router + Vue I18n + Tailwind 3
- Do not introduce a second major UI framework (no Element Plus / Ant Design Vue / naive-ui as system replacement)
- Prefer reuse of `frontend/src/style.css` component classes and `components/common/*`
- One module per PR when possible; run `pnpm typecheck` and `pnpm build` in `frontend/` each phase
- motion-v and GSAP must not control the same property on the same node; always clean up GSAP

---

## Inventory Snapshot (Audit)

| Area | Current state |
|------|----------------|
| Tokens | `primary` teal + `dark`/`accent` slate (duplicate) + gray |
| Theme | `class` dark mode; init in `main.ts`, `AppSidebar`, `KeyUsageView` |
| Shell | `AppLayout` + mesh gradient + `AppSidebar` + sticky glass `AppHeader` |
| Components | Strong common set (`DataTable`, `BaseDialog`, `StatCard`, badges, inputs…) |
| Charts | Chart.js wrappers under `components/charts` + Ops charts |
| Motion | Tailwind keyframes present; **gsap / motion-v installed but unused** |
| Pages | ~80 views (admin / user / auth / setup / public) |

---

## Phase 0 — Design Audit & Token Convergence

### Goals
- Freeze visual source of truth (`MASTER.md`)
- Eliminate conflicting tokens (`accent` ≡ `dark`)
- Document retain / converge / discard
- Add semantic CSS variable bridge for Chart.js without breaking classes
- Global `prefers-reduced-motion` baseline

### Pages
- None (config / CSS / docs only)

### Components / files
- `frontend/tailwind.config.js`
- `frontend/src/style.css`
- Optional: `frontend/src/styles/tokens.css` (if vars extracted)
- Docs already created this phase

### Preserve business logic
- All API, stores, i18n keys, feature flags

### Risks
- Mass class rename of `accent-*` if done carelessly
- Chart hard-coded hex drift if vars not adopted consistently

### Acceptance
- [ ] `MASTER.md` reviewed by maintainer
- [ ] Decision recorded: deprecate `accent-*` for new code; migration script or codemod plan listed
- [ ] No new hex introduced in docs that conflict with `primary` / `dark`
- [ ] `git diff --check` clean for doc/token files

### Recommended commit granularity
1. `docs: add design system MASTER and UI refactor plan`
2. `chore(ui): deprecate accent token usage guidelines` (docs-only or comment)
3. Later: `refactor(ui): map accent-* classes to dark-*/gray-*` (code, separate PR)

---

## Phase 1 — Global AppShell, Sidebar, Header

### Goals
- Calm enterprise shell: reduce mesh/glow noise
- Single theme bootstrap
- Sidebar / header density and a11y pass
- Align brand mark treatment with Swiss + subtle elevation (no looping glow)

### Pages
- All authenticated routes via `AppLayout`
- Auth layout parity check (`AuthLayout.vue`)

### Components
- `AppLayout.vue`
- `AppSidebar.vue`
- `AppHeader.vue`
- `AuthLayout.vue`
- Theme helpers (extract `useTheme` composable)
- `VersionBadge.vue`, `LocaleSwitcher.vue`, `AnnouncementBell.vue` (visual only)

### Preserve business logic
- Nav item generation, feature flags, admin/user sections, balance display data, onboarding tour hooks

### Risks
- Sidebar collapse animation regressions on mobile
- Theme flash if init order changes
- Tour selectors (`data-tour`, ids) break if markup moves

### Acceptance
- [ ] One theme init path
- [ ] Mesh/glow limited per MASTER
- [ ] Keyboard: mobile menu, user dropdown, theme toggle labeled
- [ ] Collapsed sidebar still usable (titles/tooltips)
- [ ] `typecheck` + `build` pass

### Recommended commit granularity
1. `refactor(ui): extract useTheme and unify dark class bootstrap`
2. `style(ui): calm AppLayout canvas and header elevation`
3. `style(ui): sidebar density and a11y polish`

---

## Phase 2 — Foundation Component System

### Goals
- Single dialog primitive path; tighten buttons/inputs/badges/cards
- Prefer `StatCard` / shared empty/loading
- Deduplicate obvious twins (e.g. dual `AccountStatsModal` — choose one facade)
- Prepare motion-v adapters for Modal/Dropdown without rewriting all call sites

### Pages
- Indirect (all pages consuming common components)

### Components
- `components/common/*` (`BaseDialog`, `ConfirmDialog`, `Input`, `Select`, `Toggle`, `DataTable`, `Pagination`, `EmptyState`, `Skeleton`, `Toast`, `StatCard`, …)
- `style.css` `@layer components`
- Icon sizing consistency in `Icon.vue`

### Preserve business logic
- Props/events/public component APIs unless backward-compatible aliases provided

### Risks
- Visual regressions across 50+ modals
- Table sticky/scroll regressions in `TablePageLayout` / `DataTable`

### Acceptance
- [ ] `.dialog-*` vs `.modal-*` convergence plan executed or aliased
- [ ] `.glass-card` unused in new code; migration list for old call sites
- [ ] Badge purple not used in new status designs
- [ ] Focus rings visible in light and dark
- [ ] `typecheck` + `build` + critical unit tests (`DataTable`, dialogs)

### Recommended commit granularity
1. `refactor(ui): unify dialog surface tokens`
2. `refactor(ui): button and input density tokens`
3. `refactor(ui): dedupe AccountStatsModal entry`
4. `feat(ui): motion-v wrappers for modal/dropdown (opt-in)`

---

## Phase 3 — Dashboard Reference Pages

### Goals
- Ship the visual north star: **Admin Dashboard** + **User Dashboard** (+ light Ops header consistency)
- 12-col bento, dense KPIs, chart token colors, GSAP phased enter (reduced-motion safe)

### Pages
- `views/admin/DashboardView.vue`
- `views/user/DashboardView.vue` + `components/user/dashboard/*`
- Optional polish: `views/admin/ops/OpsDashboard.vue` header/skeleton only

### Components
- `StatCard.vue` (replace inline KPI cards where practical)
- `components/charts/*`
- New optional: `DashboardGrid.vue` / composable `useDashboardEntrance.ts` (GSAP)

### Preserve business logic
- All stats API calls, date range, granularity, platform quotas, refresh

### Risks
- Chart re-render vs GSAP fighting opacity
- Performance on low-end devices if stagger too heavy

### Acceptance
- [ ] KPI → charts → tables entrance ≤ ~8 staggered items
- [ ] Reduced motion: instant show, no opacity trap
- [ ] Chart colors use shared token helper (no random purple brand wash)
- [ ] Layout reads as one dense composition, not marketing cards
- [ ] `typecheck` + `build`

### Recommended commit granularity
1. `feat(ui): dashboard grid and StatCard adoption (admin)`
2. `feat(ui): user dashboard visual alignment`
3. `feat(ui): GSAP dashboard entrance with reduced-motion`

---

## Phase 4 — Accounts, Models, Channels

### Goals
- Dense table UX for operational core
- Consistent filters/actions bars, status badges, drawers/modals

### Pages
- `views/admin/AccountsView.vue`
- `views/admin/GroupsView.vue` (routing/model groups)
- `views/admin/ChannelsView.vue`
- `views/admin/ChannelMonitorView.vue`
- `views/admin/ProxiesView.vue`
- Related `components/account/*`, `components/admin/account/*`, `components/admin/channel/*`, `components/channels/*`

### Components
- `TablePageLayout`, `DataTable`, bulk action bars, status indicators, import/sync modals (visual only)

### Preserve business logic
- CRUD, bulk ops, sync, quotas, pricing rows, monitors — **API untouched**

### Risks
- Complex column settings / sticky columns break
- Large Accounts table performance

### Acceptance
- [ ] Comfortable vs dense padding aligned with MASTER
- [ ] Filters + table + pagination rhythm consistent
- [ ] Empty/loading/error states use shared components
- [ ] No API contract changes
- [ ] `typecheck` + `build` + existing account/group tests

### Recommended commit granularity
1. `style(ui): AccountsView table density and toolbar`
2. `style(ui): GroupsView and channel pages alignment`
3. `style(ui): monitor/proxies visual consistency`

---

## Phase 5 — API Keys, Users, Billing Surfaces

### Goals
- Trustworthy key management UI (mono, copy affordances, danger zones)
- Users admin + payment/redeem/subscription user flows look coherent

### Pages
- `views/user/KeysView.vue` + `components/keys/*`
- `views/admin/UsersView.vue` + `components/admin/user/*`
- `views/user/UsageView.vue`, `views/admin/UsageView.vue`
- Billing-adjacent: `PaymentView`, `RedeemView` (user/admin), `SubscriptionsView` (user/admin) — visual only

### Components
- Key modals/popovers, user balance modals, usage charts/tables

### Preserve business logic
- Key CRUD, scopes, user balance adjustments, usage queries, payment providers

### Risks
- Payment brand buttons must keep official colors
- Sensitive key reveal UX regressions

### Acceptance
- [ ] Keys use `font-mono` and clear reveal/copy patterns
- [ ] Danger actions use confirm + danger button
- [ ] Billing CTAs still recognizable (Stripe/Alipay/WeChat/Airwallex)
- [ ] `typecheck` + `build` + payment/keys tests green

### Recommended commit granularity
1. `style(ui): KeysView and key modals`
2. `style(ui): UsersView and user admin modals`
3. `style(ui): usage and billing page shells`

---

## Phase 6 — Settings, Payments Admin, System

### Goals
- Settings IA readability (long forms)
- Admin orders/payment dashboard/plans visual alignment
- Ops remaining cards, risk, backup, announcements, affiliates, promo — shell consistency

### Pages
- `views/admin/SettingsView.vue` + settings subcomponents
- `views/admin/orders/*`
- `views/admin/ops/*` (remaining)
- `views/admin/RiskControlView.vue`, `BackupView.vue`, `AnnouncementsView.vue`, `PromoCodesView.vue`
- `views/admin/affiliates/*`
- `views/admin/RedeemView.vue` (if not done in P5)
- Auth/Setup/Home only if brand shell drift remains

### Components
- Settings sections, order tables, ops cards/charts, compliance dialog (visual)

### Preserve business logic
- All settings payloads, payment admin actions, ops metrics APIs

### Risks
- Settings is large — easy to create accidental behavior changes
- Ops charts many — scope creep

### Acceptance
- [ ] Settings sections use consistent card/spacing
- [ ] Admin payment pages match dashboard token language
- [ ] No env/API changes
- [ ] `typecheck` + `build`

### Recommended commit granularity
1. `style(ui): SettingsView section layout`
2. `style(ui): admin payment and orders`
3. `style(ui): ops/risk/system pages pass`

---

## Phase 7 — Motion, Responsive, Accessibility Hardening

### Goals
- Formalize CSS / motion-v / GSAP ownership across adopted surfaces
- Global reduced-motion
- Responsive QA + a11y audit fixes
- Remove leftover anti-patterns (glow loops, unused glass)

### Pages
- Spot-check: Dashboard, Accounts, Keys, Settings, Login, Home
- Mobile sidebar / table card mode

### Components
- motion-v integrations, GSAP composables, focus styles, skip links (if added)

### Preserve business logic
- Everything functional remains

### Risks
- Over-animating late in project
- a11y fixes touching many templates

### Acceptance
- [ ] Reduced-motion verified on Dashboard entrance + modals + progress bar
- [ ] No Motion+GSAP transform conflicts (code review checklist)
- [ ] GSAP cleanup on unmount verified
- [ ] Contrast spot-check light/dark AA
- [ ] 375 / 768 / 1024 / 1440 smoke
- [ ] `typecheck` + `build` + lint on touched files

### Recommended commit granularity
1. `feat(ui): global reduced-motion and motion ownership helpers`
2. `fix(a11y): focus, labels, contrast follow-ups`
3. `chore(ui): remove deprecated glow/glass usages`

---

## Cross-Phase Do-Not-Touch (for now)

| Area | Why |
|------|-----|
| Backend Go / Ent / API routes | Explicit out of scope |
| DB migrations, cache keys, localStorage keys for data | Compatibility |
| Payment provider SDK integration logic | High regression cost |
| OAuth callback flows (logic) | Auth correctness |
| Go module path / Sub2API compatibility identifiers | BRANDING.md |
| Wholesale i18n key renames | Avoid translation churn mid-UI |
| Introducing new UI framework | Conflicts with token converge |

---

## Suggested Overall Sequencing

```text
Phase 0 (docs/tokens) → 1 (shell) → 2 (primitives) → 3 (dashboard north star)
    → 4 (ops core tables) → 5 (keys/users/billing UI) → 6 (settings/system) → 7 (motion/a11y)
```

Parallelization: Phase 4–6 can partially overlap **after** Phase 2 lands, if owners keep PRs module-scoped.

---

## Definition of Done (Program)

- Design tokens mapped to Tailwind; no competing palette
- App shell calm, dense, trustworthy
- Dashboard is the reference implementation
- Core CRUD pages share table/form/dialog patterns
- motion-v + GSAP used with clear ownership and cleanup
- Accessibility and reduced-motion baseline met
- Frontend `typecheck` + `build` green on mainline
