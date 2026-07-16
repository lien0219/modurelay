# ModuRelay Design System — MASTER

> **Source of Truth** for UI/UX. Generated from project audit + `ui-ux-pro-max` (2026-07-16).  
> Stack: Vue 3 · TypeScript · TailwindCSS 3 · Pinia · Vue Router · Vue I18n · Chart.js · motion-v · GSAP  
> Brand: [BRANDING.md](../BRANDING.md) · Product: Enterprise AI API Gateway / Account Pool / Model Routing

Page overrides (if any) live in `design-system/pages/*.md` and **override** this file for that page only.

---

## 1. Design Principles

1. **Precision over decoration** — Every pixel serves operations: routing, quotas, keys, cost, health.
2. **Density with calm** — Data-dense dashboards; whitespace is structural, not marketing.
3. **One accent** — Teal (`primary`) is the only brand accent. No purple/pink AI gradients.
4. **Token-first** — Prefer Tailwind semantic tokens (`primary-*`, `dark-*`, surface utilities). No arbitrary hex in pages except third-party brand buttons (Stripe / Alipay / WeChat / Airwallex).
5. **Converge, don't fork** — Extend existing `style.css` component classes and Tailwind theme; do not invent a second palette or animation system.
6. **Motion has a job** — CSS for microstates, motion-v for UI chrome, GSAP for choreographed dashboard moments. Never both on the same transform.
7. **Trustworthy & accessible** — WCAG AA contrast, visible focus, `prefers-reduced-motion`, no emoji-as-icons.

---

## 2. Brand Visual Positioning

| Axis | Direction |
|------|-----------|
| Style blend | **AI-Native Enterprise Dashboard** + **Swiss Minimalism** + **Data-Dense Dashboard** + **Bento Layout** (restrained) + **Subtle Dimensional Layering** |
| Keywords | Enterprise · Technical · Precise · Calm · Premium · Trustworthy · Data-dense · Developer-focused |
| Voice | Clear, operational, bilingual-ready (zh/en/ja via i18n) |
| Logo | Temporary mark (`/modurelay-mark.svg`) until formal assets; respect `site_name` / `site_logo` overrides |
| Anti-look | Purple-pink AI gradients, heavy glassmorphism, neon glow, oversized radius, floating every card, marketing hero animation |

**ui-ux-pro-max dials used:** variance 6 · motion 5 · density 9

---

## 3. Light & Dark Mode

| Rule | Implementation |
|------|----------------|
| Strategy | `darkMode: 'class'` on `<html>` |
| Persistence | `localStorage.theme` = `light` \| `dark` |
| Default | System `prefers-color-scheme` when unset |
| Init | Single source of truth (today duplicated in `main.ts`, `AppSidebar`, `KeyUsageView` — converge in Phase 0/1) |
| Surfaces | Light: cool gray canvas + white cards · Dark: `dark-950` canvas + `dark-800/900` elevated |

Do **not** introduce a second theme engine or CSS-variable-only theme that bypasses Tailwind `dark:`.

---

## 4. Semantic Color Tokens

### 4.1 Preserve (Tailwind — already in `tailwind.config.js`)

| Token | Role | Notes |
|-------|------|-------|
| `primary-50…950` | Brand accent / CTA / focus / active nav | Teal family — **keep** |
| `dark-50…950` | Dark surfaces & muted text in dark mode | Slate — **keep** |
| `gray-*` | Light-mode neutrals | Tailwind default — **keep** |
| Status: `emerald` / `amber` / `red` / `blue` (charts only) | Success / warning / danger / info | Prefer semantic aliases below in new code |

### 4.2 Converge / deprecate

| Issue | Decision |
|-------|----------|
| `accent-*` === `dark-*` (identical hex) | **Deprecate `accent-*` in new UI.** Use `dark-*` for surfaces or `gray-*` for light neutrals. Map any `accent` usage → `dark` / `gray` in Phase 0. |
| `badge-purple`, purple KPI icons | **Avoid for brand.** Use `primary` / status colors. Purple only if chart series needs a 5th categorical color. |
| Hardcoded payment brand hex | **Keep** (`.btn-stripe`, `.btn-alipay`, etc.) — third-party identity. |

### 4.3 Semantic aliases (map to Tailwind classes — do not invent new hex)

| Semantic | Light | Dark |
|----------|-------|------|
| `surface-canvas` | `bg-gray-50` | `bg-dark-950` |
| `surface-raised` | `bg-white` | `bg-dark-900` |
| `surface-sunken` | `bg-gray-100` | `bg-dark-800` |
| `surface-overlay` | `bg-white` | `bg-dark-800` |
| `border-default` | `border-gray-200` | `border-dark-700` |
| `border-subtle` | `border-gray-100` | `border-dark-800` |
| `text-primary` | `text-gray-900` | `text-white` / `text-gray-100` |
| `text-secondary` | `text-gray-600` | `text-dark-300` |
| `text-muted` | `text-gray-500` | `text-dark-400` |
| `text-disabled` | `text-gray-400` | `text-dark-500` |
| `action-primary` | `bg-primary-600` text white | same / slightly brighter text |
| `focus-ring` | `ring-primary-500/50` | same |
| `danger` | `red-500/600` | `red-400/500` |
| `success` | `emerald-500/600` | `emerald-400` |
| `warning` | `amber-500/600` | `amber-400` |
| `info` | `primary-500` or `blue-500` (charts) | matching |

Optional future CSS vars (Phase 0) for Chart.js / non-Tailwind consumers:

```css
:root {
  --mr-primary: theme('colors.primary.500');
  --mr-canvas: theme('colors.gray.50');
  --mr-raised: #ffffff;
  --mr-border: theme('colors.gray.200');
  --mr-text: theme('colors.gray.900');
  --mr-text-muted: theme('colors.gray.500');
}
.dark {
  --mr-canvas: theme('colors.dark.950');
  --mr-raised: theme('colors.dark.900');
  --mr-border: theme('colors.dark.700');
  --mr-text: theme('colors.gray.100');
  --mr-text-muted: theme('colors.dark.400');
}
```

---

## 5. Background Hierarchy (Dimensional Layering — subtle)

| Level | Purpose | Light | Dark | Elevation |
|-------|---------|-------|------|-----------|
| L0 Canvas | App shell | `gray-50` | `dark-950` | none |
| L1 Raised | Cards, tables, sidebar | `white` | `dark-900` | border + `shadow-card` |
| L2 Recessed | Inputs, tabs track, table header | `gray-50/100` | `dark-800` | inset border |
| L3 Overlay | Modal, dropdown, toast | `white` | `dark-800` | `shadow-lg` / `shadow-2xl` |
| L4 Scrim | Modal backdrop | `bg-black/50` | `bg-black/60` | blur optional `sm` |

**Mesh / glass:** Keep `bg-mesh-gradient` only if reduced to ≤8% opacity teal wash **or** remove in Phase 1. Prefer Swiss calm canvas over marketing mesh. Limit `.glass` to sticky header only.

---

## 6. Text Color Hierarchy

| Level | Class pattern | Min contrast |
|-------|---------------|--------------|
| Primary | `text-gray-900 dark:text-white` | ≥ 4.5:1 |
| Secondary | `text-gray-600 dark:text-dark-300` | ≥ 4.5:1 body / ≥ 3:1 large |
| Muted / hint | `text-gray-500 dark:text-dark-400` | ≥ 3:1 (labels, meta) |
| Inverse on primary | `text-white` on `primary-500+` | ≥ 4.5:1 |
| Link / active | `text-primary-600 dark:text-primary-400` | ≥ 4.5:1 |

**Forbidden:** `text-gray-400` for body copy; low-contrast gray on gray cards.

---

## 7. Typography

### 7.1 Families

| Role | Recommended | Tailwind mapping | Status |
|------|-------------|------------------|--------|
| UI sans | **IBM Plex Sans** (ui-ux-pro-max: developer enterprise) | `font-sans` | Adopt in Phase 0–1; until then keep system stack |
| Mono / data | **JetBrains Mono** or existing `ui-monospace` stack | `font-mono` | Keys, IDs, tokens, costs, JSON |
| Fallback | System CJK stacks already in config | keep | zh/ja |

Do **not** adopt Fira Code as UI heading (cli default) — too “code editor” for product chrome. Mono is for data cells and code blocks only.

### 7.2 Scale (4px rhythm)

| Token | Size | Weight | Line-height | Use |
|-------|------|--------|-------------|-----|
| `display` | 30px / `text-3xl` | 600–700 | 1.2 | Rare (auth/marketing only) |
| `h1` / page | 24px / `text-2xl` | 700 | 1.25 | `.page-title` |
| `h2` / section | 18px / `text-lg` | 600 | 1.3 | Card / panel titles |
| `h3` | 16px / `text-base` | 600 | 1.4 | Subsections |
| `body` | 14px / `text-sm` | 400–500 | 1.5 | Default UI |
| `body-lg` | 16px / `text-base` | 400 | 1.5 | Forms long copy |
| `caption` | 12px / `text-xs` | 400–500 | 1.4 | Hints, badges, table meta |
| `overline` | 12px / `text-xs` | 600 | 1.3 | `uppercase tracking-wider` section labels |

---

## 8. Spacing System (8px base, 4px half-steps)

| Token | px | Tailwind | Use |
|-------|-----|----------|-----|
| `space-0.5` | 2 | `0.5` | Hairline icon adjust |
| `space-1` | 4 | `1` | Tight badge padding |
| `space-2` | 8 | `2` | Compact gaps |
| `space-3` | 12 | `3` | Control internal |
| `space-4` | 16 | `4` | Default gap |
| `space-5` | 20 | `5` | Card padding compact |
| `space-6` | 24 | `6` | Section / card padding |
| `space-8` | 32 | `8` | Page section gaps |
| `space-10` | 40 | `10` | Large breaks |
| `space-12` | 48 | `12` | Empty states |

**Dashboard density:** Prefer `gap-3`–`gap-4`, card `p-4`–`p-5` (not `p-8`). Table cell `px-4 py-2.5`–`py-3` (dense) vs `py-4` (comfortable).

---

## 9. Radius System

| Token | Value | Tailwind | Use |
|-------|-------|----------|-----|
| `none` | 0 | `rounded-none` | Rare |
| `sm` | 6px | `rounded-md` | Inputs dense, chips |
| `md` | 8px | `rounded-lg` | Buttons sm, table tools |
| `lg` | 12px | `rounded-xl` | **Default** controls, nav links |
| `xl` | 16px | `rounded-2xl` | Cards, modals |
| `full` | 9999 | `rounded-full` | Badges, avatars only |

**Converge:** Reduce new `rounded-4xl` / marketing soft blobs. Bento cards use `rounded-xl`–`rounded-2xl`, not 24px Apple-marketing radius everywhere.

---

## 10. Shadow System

| Token | Existing | Use | Verdict |
|-------|----------|-----|---------|
| `shadow-card` | keep | Default card | **Primary** |
| `shadow-card-hover` | keep | Interactive panels only | Use sparingly |
| `shadow-glass` / `glass-sm` | keep | Header / rare overlays | Limit |
| `shadow-glow` / `glow-lg` | keep file, restrict use | Logo mark only | **Do not** animate glow on cards |
| `shadow-lg` / `2xl` | Tailwind | Dropdown / modal | OK |
| `inner-glow` | keep | Optional dark bevel | Rare |

Elevation = border + one shadow level. No stacked multi-layer “glow stacks.”

---

## 11. Border System

| Token | Class |
|-------|-------|
| Default | `border border-gray-200 dark:border-dark-700` |
| Subtle | `border-gray-100 dark:border-dark-800` |
| Strong | `border-gray-300 dark:border-dark-600` |
| Focus | `border-primary-500` + ring |
| Danger | `border-red-500` |
| Divider | `.divider` / `h-px bg-gray-200 dark:bg-dark-700` |

Global `* { border-gray-200 dark:border-dark-700 }` in `style.css` — preserve.

---

## 12. Iconography

| Rule | Spec |
|------|------|
| Library | Existing `Icon.vue` (Heroicons-style paths) + `@lobehub/icons` for model/provider marks |
| Style | Outline 1.5 stroke; filled only for selected/emphasis |
| Sizes | `xs` 12 · `sm` 16 · `md` 20 · `lg` 24 · `xl` 32 |
| Touch | Hit area ≥ 44×44 CSS px for icon-only buttons (`.btn-icon`) |
| Color | Inherit `currentColor`; status via semantic text colors |
| Forbidden | Emoji as functional icons; mixed stroke weights in one toolbar |

---

## 13. Dashboard 12-Column Grid

```
Desktop (≥1024):  display: grid; grid-template-columns: repeat(12, 1fr); gap: 16px (gap-4)
Tablet (768–1023): 8-col or 2× stacked; gap-4
Mobile (<768):     4-col / single column; gap-3
```

| Span | Use |
|------|-----|
| 3 | KPI tile (4-up) |
| 4 | KPI tile (3-up) / side chart |
| 6 | Half-width chart / table |
| 8 | Primary chart + side rail |
| 12 | Full-width table / trend |

**Bento:** Varied spans (3+3+6, 4+8, 8+4) allowed on Dashboard / Ops only. Admin CRUD tables stay full-width 12.

---

## 14. Page Max Width

| Context | Max width | Padding |
|---------|-----------|---------|
| App main (default) | fluid inside shell | `p-4 md:p-6 lg:p-8` (existing AppLayout) |
| Dense ops / tables | fluid | Prefer `p-4 md:p-6` |
| Auth / legal / setup | `max-w-lg`–`max-w-2xl` centered | `px-4` |
| Marketing Home | content sections `max-w-6xl` | separate from app shell |
| Readable prose | `max-w-3xl` | docs-like |

Do not force `max-w-7xl` on data tables — waste horizontal space.

---

## 15. Table Density

| Mode | Cell padding | Row height target | When |
|------|--------------|-------------------|------|
| Comfortable | `px-4 py-3`–`px-5 py-4` | ~48–56px | Default admin tables today |
| Dense | `px-3 py-2`–`px-4 py-2.5` | ~36–44px | Accounts, usage logs, ops |
| Mobile | Card list via `DataTable` | — | `< lg` |

Rules:
- Sticky header with subtle bottom shadow (existing `.table-wrapper thead.sticky`)
- Zebra optional; prefer hover `bg-gray-50 dark:bg-dark-800/30`
- Mono for IDs / keys / costs
- Virtualize long lists (`@tanstack/vue-virtual` already available)

---

## 16. Form Specs

| Element | Class / pattern |
|---------|-----------------|
| Label | `.input-label` |
| Control | `.input` / `Input.vue` / `Select.vue` / `TextArea.vue` / `Toggle.vue` |
| Hint | `.input-hint` |
| Error | `.input-error` + `.input-error-text` |
| Layout | Single column default; 2-col only ≥`md` for related pairs |
| Validation | Inline on blur/submit; keep focus on first error |
| Required | Text indicator + `aria-required`, not color alone |

Group related settings in one card; avoid nested cards inside cards.

---

## 17. Button Specs

| Variant | Class | Use |
|---------|-------|-----|
| Primary | `.btn .btn-primary` | Main action (prefer solid `primary-600` over strong gradient long-term) |
| Secondary | `.btn .btn-secondary` | Cancel / alternate |
| Ghost | `.btn .btn-ghost` | Tertiary / toolbar |
| Danger | `.btn .btn-danger` | Destructive |
| Success / Warning | existing | Contextual |
| Brand pay | `.btn-stripe` etc. | Payment only |
| Sizes | `.btn-sm` `.btn-md` `.btn-lg` `.btn-icon` | |

Motion: CSS `active:scale-[0.98]` OK; richer press → motion-v. Disabled: `opacity-50` + `cursor-not-allowed`, no press scale.

---

## 18. Card Specs

| Variant | Class | When |
|---------|-------|------|
| Default | `.card` | Panels, charts, settings blocks |
| Interactive | `.card` + mild hover border (avoid default `.card-hover` lift on every tile) | Clickable KPI / nav tiles |
| Stat | `.stat-card` / `StatCard.vue` | Dashboard KPIs — **prefer component over one-off markup** |
| Glass | `.glass-card` | **Deprecated for new UI**; migrate to `.card` |
| Table shell | `.card` inside `TablePageLayout` | Keep |

Bento: one job per cell; no nested card-in-card; border > heavy shadow.

---

## 19. Badge & Status

| Class | Meaning |
|-------|---------|
| `.badge-primary` | Brand / info |
| `.badge-success` | Active / healthy |
| `.badge-warning` | Degraded / pending |
| `.badge-danger` | Error / blocked |
| `.badge-gray` | Neutral / inactive |
| `.badge-purple` | **Legacy — avoid new use** |

Always pair color with text or icon. `StatusBadge.vue` / `PlatformTypeBadge.vue` for domain statuses.

---

## 20. Modal, Drawer, Dropdown

| Pattern | Spec |
|---------|------|
| Modal | `BaseDialog.vue` + `.modal-*` — role=dialog, aria-modal, focus trap, Escape |
| Confirm | `ConfirmDialog.vue` |
| Dropdown | `.dropdown` / `.dropdown-item` — motion-v or existing Vue `<transition>` |
| Drawer | Not standardized yet — when added: right sheet, `max-w-md`/`lg`, same scrim, motion-v slide |
| Z-index | Overlay 50 · Toast 100 · Progress above content |

Converge `.dialog-*` vs `.modal-*` naming in Phase 2 (one primitive).

Scrim ≥ 40–60% black. Reduced motion: existing modal rules in `style.css` — extend to Drawer.

---

## 21. Empty / Loading / Error

| State | Pattern |
|-------|---------|
| Empty | `.empty-state` / `EmptyState.vue` — icon + title + one CTA |
| Loading page | `LoadingSpinner` centered; prefer skeleton for dashboards (`Skeleton.vue`, Ops skeleton) |
| Loading inline | `.spinner` / button spinner |
| Error | Inline alert or toast (`showError`); destructive actions need confirm |
| Partial fail | Keep last good data + error banner |

No blank white flashes: keep AppLayout chrome visible.

---

## 22. Charts

| Rule | Spec |
|------|------|
| Library | Chart.js + vue-chartjs (existing) |
| Theme | Read `document.documentElement.classList.contains('dark')` (as today) → grid/text colors from tokens |
| Series (token usage) | Input `blue-500` · Output `emerald-500` · Cache create `amber-500` · Cache read `cyan-500` · Extra categorical `violet-500` sparingly |
| Brand accent line | `primary-500` for single-metric trends |
| Legend | Bottom or right; wrap on mobile; contrast ≥ 3:1 |
| Empty | Centered muted text in fixed height container (existing) |
| Animation | Chart.js default OK; complex first-paint choreography → GSAP opacity only on wrapper, not double-animating canvas |

---

## 23. Responsive Breakpoints

Align with Tailwind defaults + existing shell behavior:

| Name | Min | Shell behavior |
|------|-----|----------------|
| `sm` | 640 | Header densifies |
| `md` | 768 | 2-col forms |
| `lg` | 1024 | Sidebar visible; table desktop mode |
| `xl` | 1280 | Wider bento |
| `2xl` | 1536 | Optional denser grids |

Mobile: sidebar off-canvas + overlay (existing). Touch targets ≥ 44px. Safe areas: `.safe-top` / `.safe-bottom`.

---

## 24. Accessibility

- Focus visible: `focus:ring-2 focus:ring-primary-500/50 focus:ring-offset-2` (dark offset `dark-900`)
- Keyboard: all interactive controls reachable; modals trap focus (BaseDialog)
- Labels: every input has visible label; icon-only has `aria-label`
- Color not sole status signal
- Live regions for toasts (`Toast.vue`)
- Skip decorative icons with `aria-hidden`
- Target WCAG **AA**
- Test 375 / 768 / 1024 / 1440

---

## 25. Animation Tokens

| Token | Duration | Easing | Owner |
|-------|----------|--------|-------|
| `motion-instant` | 0–80ms | linear | CSS opacity flickers |
| `motion-fast` | 150ms | ease-out | Hover color, focus |
| `motion-base` | 200–250ms | ease-out | Buttons, tabs |
| `motion-enter` | 250–300ms | ease-out | Modal / dropdown enter |
| `motion-exit` | 150–200ms | ease-in | Exit faster than enter |
| `motion-stagger-step` | 40–60ms | — | GSAP dashboard tiles (≤8 items) |
| `motion-count` | 400–800ms | power2.out | GSAP number count-up |

Existing Tailwind keyframes to **keep**: `fade-in`, `slide-up`, `slide-down`, `slide-in-right`, `scale-in`, `shimmer` (skeletons).  
**Deprecate for product UI:** looping `glow` animation on interactive surfaces.

---

## 26. `prefers-reduced-motion`

```css
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 1ms !important;
    transition-duration: 1ms !important;
    scroll-behavior: auto !important;
  }
}
```

Rules:
- Already applied to modals and `NavigationProgress` — **extend globally in Phase 0/7**
- GSAP: check `matchMedia('(prefers-reduced-motion: reduce)')` or `gsap.matchMedia()` → set state immediately, skip timelines
- motion-v: disable / instant transition when reduced
- Never leave opacity:0 content stuck when motion is off

---

## 27. Design Anti-Patterns

1. Purple/pink full-bleed AI gradients  
2. Glassmorphism on every card  
3. Glow shadows as decoration  
4. `rounded-3xl`/`4xl` everywhere  
5. Every card `hover:-translate-y` floating  
6. Stagger-animating 50 table rows  
7. Motion + GSAP both driving `transform` on one node  
8. Low-contrast gray body text  
9. Emoji as nav/status icons  
10. Marketing-site scroll theater inside admin CRUD  
11. Introducing shadcn/Element/Ant as a second system  
12. New hex colors bypassing Tailwind tokens  
13. Duplicate theme init / duplicate modal primitives  
14. Changing API payloads or backend for “prettier UI”  
15. Uncleaned GSAP timelines after unmount  

---

## 28. Animation Ownership (CSS · motion-v · GSAP)

### CSS / Tailwind
- Color / border / shadow transitions  
- Focus rings  
- Simple hover  
- Base loading (spinner, shimmer skeleton)  
- Existing `.modal-*` transitions (until migrated)

### motion-v
- Button press feedback (beyond CSS scale)  
- Modal / Drawer enter-exit  
- Dropdown  
- List enter/exit (short lists)  
- Layout shifts (`layout`)  
- Local page section transitions  

### GSAP
- Dashboard phased entrance (KPI → charts → tables)  
- Number count-up  
- SVG / topology lines  
- Multi-step timelines  
- Complex chart first-paint wrappers  
- ScrollTrigger scenes (rare; Home or Ops storytelling only)

### Forbidden
- Same element `transform` owned by both motion-v and GSAP  
- Long row-by-row table entrance  
- Animations that block clicks (`pointer-events` during 1s+ intros)  
- Timelines surviving component unmount  
- Ignoring `prefers-reduced-motion`  

**Cleanup checklist (GSAP):** `ctx.revert()` / `timeline.kill()` / `ScrollTrigger.getAll().forEach(st => st.kill())` in `onBeforeUnmount`.

---

## 29. Retain / Converge / Discard

| Asset | Decision |
|-------|----------|
| `primary` teal scale | **Retain** — brand |
| `dark` slate scale | **Retain** — dark surfaces |
| `accent` scale | **Converge → deprecate** (duplicate of `dark`) |
| `.btn` / `.input` / `.card` / `.table` / `.badge` / `.sidebar` | **Retain** — refine |
| `.glass` / mesh / glow | **Converge** — reduce scope |
| `fade-in` / `slide-*` / `scale-in` / `shimmer` | **Retain** |
| Looping `glow` keyframes | **Discard** for UI surfaces |
| `StatCard`, `DataTable`, `BaseDialog`, `TablePageLayout` | **Retain** — expand reuse |
| Duplicate `AccountStatsModal` paths | **Converge** |
| gsap / motion-v deps | **Retain** — unused today; adopt per ownership matrix |
| System fonts | **Converge** → IBM Plex Sans + JetBrains Mono |
| Payment brand button hex | **Retain** |

---

## 30. Tailwind Mapping Quick Reference

```text
Canvas          bg-gray-50 dark:bg-dark-950
Card            bg-white dark:bg-dark-800/50 + border + shadow-card
Primary action  bg-primary-600 hover:bg-primary-700 text-white
Focus           focus:ring-2 focus:ring-primary-500/50
Text            text-gray-900 dark:text-white
Muted           text-gray-500 dark:text-dark-400
Nav active      bg-primary-50 text-primary-600 dark:bg-primary-900/20 dark:text-primary-400
Radius control  rounded-xl
Radius card     rounded-2xl
Gap dashboard   gap-4
Pad card dense  p-4
```

---

*End of MASTER. For UI work, also read `docs/UI_REFACTOR_PLAN.md` and `.cursor/skills/modurelay-ui/SKILL.md`.*
