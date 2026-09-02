---
name: modurelay-console-frosted
description: Implement and review ModuRelay official non-home Vue interfaces using the Frosted Precision design system. Use for authentication, setup, public utility, user console, Admin, table, form, chart, responsive, accessibility, and shared layout work. Preserve routes, permissions, APIs, payloads, business behavior, density, and test selectors. Exclude the official /home, custom HTML, custom URL iframe, compact-home, and portal surfaces; use for Station only when the task explicitly includes Station and its branch-specific rules.
---

# ModuRelay Frosted Console

Build calm, precise, developer-first work surfaces with restrained Glass,
stable data surfaces, resilient text, and compact interaction density.

## Establish The Contract

1. Read repository-root `AGENTS.md` and `docs/UI_DESIGN_SYSTEM.md` completely.
2. Inspect the current route, shared layout, affected components, API bindings,
   permissions, feature flags, tests, and existing selectors before editing.
3. For a new visual direction or uncertain component decision, read
   `.agents/skills/ui-ux-pro-max/SKILL.md` and run the smallest relevant local
   search. Treat its output as advice; repository rules win every conflict.
4. Keep the official `/home` under `modurelay-home-sylva`. Do not transfer home
   scene code, cinematic type scale, or vendor assets into console surfaces.
5. Keep custom HTML, custom URL iframe, compact-home, and portal rendering
   untouched. Preserve Station identity and capability boundaries when Station
   is explicitly in scope.

## Apply Frosted Precision

- Use `frontend/src/style.css` semantic tokens. Do not add component-local brand
  hex values.
- Keep the canvas solid. Use Glass for headers, sidebars, authentication focus
  panels, filter strips, dropdowns, dialogs, popovers, and selected KPI accents.
- Keep tables, long forms, charts, and primary work regions on opaque surfaces.
  Do not nest Glass or glassify every card.
- Use Noto Sans SC/system sans for UI text and the existing monospace stack only
  for paths, keys, model IDs, technical labels, and tabular values.
- Use Indigo for primary action, Cyan for technical emphasis, and semantic
  colors only for status. Keep charts within the same palette.
- Target 36px desktop controls, 40-44px touch controls, 44-48px table rows,
  16-20px panel padding, and 16-24px page spacing.
- Keep headings compact. Give flex text children `min-width: 0`; allow long
  tokens to wrap safely; keep badge labels whole; wrap chip collections or use
  an operable `+n` disclosure.
- Use concrete CSS transition properties. Keep console feedback within
  120-240ms and honor reduced motion. Use scoped GSAP only for a real sequence.

## Use UI UX Pro Max Selectively

Run from the repository root on Windows:

```powershell
py -3 .agents/skills/ui-ux-pro-max/scripts/search.py `
  "enterprise glassmorphism" --domain style
py -3 .agents/skills/ui-ux-pro-max/scripts/search.py `
  "badge chip label wraps" --domain ux
py -3 .agents/skills/ui-ux-pro-max/scripts/search.py `
  "trend comparison" --domain chart
```

Use `--design-system` only for a system-wide direction. Do not persist a second
generated design system; `docs/UI_DESIGN_SYSTEM.md` is the source of truth.
Retry an empty search once with a narrower phrase, then use project conventions
and disclose that the local data had no match.

## Implement In Safe Increments

1. Establish representative behavior and screenshots before changing shared
   primitives.
2. Prefer shared semantic classes or an existing component when several pages
   need the same treatment. Keep page-specific corrections local.
3. Preserve route metadata, field names, table columns and ordering, sticky
   behavior, pagination, loading/error/empty states, and all API interaction.
4. Verify light and dark modes at desktop, tablet, 430px, and 390px. Check long
   labels, zoom, keyboard focus, reduced motion, overflow, modal, dropdown,
   loading, empty, disabled, and error states.
5. Run typecheck, changed-file lint, focused tests, the full frontend test suite,
   production build, and `git diff --check`. Report any skipped check exactly.

## Reject These Patterns

- No `transition: all`, unscoped selectors, unsupported claims, visual-only
  backend changes, new icon families, large glow, full-page gradients, obvious
  grids, decorative particles, or blanket Glass.
- No card-inside-card composition for ordinary page sections.
- No hidden or hover-only essential action, color-only status, tiny body text,
  clipped labels, or inaccessible icon-only control.
- No weakening tests, type rules, accessibility checks, or production behavior
  to accommodate a visual change.
