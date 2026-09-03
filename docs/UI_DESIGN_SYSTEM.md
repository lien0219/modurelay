# ModuRelay UI Design System v2

This is the visual contract for ModuRelay. The product should feel premium,
developer-first, enterprise-ready, calm, precise, modern, physical, fast,
consistent, and accessible. The system is shared by the console and the
official `/home`, while each surface keeps its own information architecture.

**ALL FUTURE UI CHANGES MUST FOLLOW THIS DESIGN SYSTEM.**

## ModuRelay Design Principles

- Make structure easy to scan. Density is useful when hierarchy remains clear.
- Prefer evidence, labels, and real product behavior over decorative claims.
- Use material, spacing, edge contrast, and typography to create hierarchy.
- Keep interaction feedback quick, interruptible, and keyboard accessible.
- Preserve existing routes, permissions, APIs, payloads, and business meaning.
- Keep the official home distinct from custom HTML, iframe, compact, and portal modes.

## Brand Themes

### Mist Indigo Light

The light environment uses a cool, restrained hierarchy rather than a white
wall: `#F5F7FB` background, `#FFFFFF` surface, `#F0F2F7` soft surface, and
`#EEF2FF` primary-soft surface. Brand primary is `#4F46E5`, with `#4338CA`
hover and `#3730A3` active. Cyan `#0891B2` is a technical accent, not a
replacement for status colors.

### Obsidian Indigo Dark

The dark environment uses depth instead of pure black: `#090C12` deep
background, `#0B0D12` page background, `#131720` surface, and `#191E28`
raised surface. Brand primary is `#6366F1` with `#818CF8` light/hover and
`#4F46E5` active. Cyan `#22D3EE` is reserved for technical highlights.

## Semantic Tokens

The source of truth is `frontend/src/style.css`. New components should use
semantic tokens or the existing `--mr-*` compatibility aliases, never a new
component-local brand hex value.

Core token groups:

- Background: `--color-bg`, `--color-bg-deep`, `--color-bg-subtle`
- Surfaces: `--color-surface`, `--color-surface-soft`,
  `--color-surface-raised`, `--color-surface-overlay`
- Brand: `--color-primary`, `--color-primary-hover`,
  `--color-primary-active`, `--color-primary-soft`,
  `--color-primary-border`, `--color-primary-ring`
- Accent: `--color-accent`, `--color-accent-soft`
- Text: `--color-text-primary`, `--color-text-secondary`,
  `--color-text-muted`, `--color-text-disabled`
- Borders: `--color-border`, `--color-border-subtle`,
  `--color-border-strong`
- Status: `--color-success`, `--color-warning`, `--color-danger`,
  `--color-info`
- Shadows: `--shadow-xs`, `--shadow-sm`, `--shadow-md`, `--shadow-lg`,
  `--shadow-overlay`
- Glass: `--glass-bg`, `--glass-bg-strong`, `--glass-border`,
  `--glass-highlight`, `--glass-shadow`
- Motion: `--motion-fast`, `--motion-base`, `--motion-slow`,
  `--ease-standard`, `--ease-enter`, `--ease-exit`

## Surface Hierarchy

Use the smallest surface distinction that makes a region scannable:

1. Background for page canvas and low-emphasis bands.
2. Surface for primary panels, tables, forms, and stable content.
3. Soft surface for grouped controls, code blocks, and secondary regions.
4. Raised surface for active cards, menus, and focused work areas.
5. Overlay surface only for dialogs, popovers, toasts, and floating tools.

Ordinary business cards are stable surfaces, not automatically Glass. Glass is
a layering tool for headers, overlays, filters, and selected product previews.

## Typography And Spacing

Use the existing system and `Noto Sans SC` where available. Console headings
favor 600-700 weight, compact line height, and readable numeric alignment.
Body text should remain readable at normal zoom; muted text must not carry the
only meaning of a value. The official home may use a larger hero title, but
compact panels and console pages must not use hero-scale type.

Use the existing spacing scale consistently. Prefer a small number of clear
gaps over arbitrary one-off values. Keep body copy around 520-620px wide,
preserve generous section breathing room on the official home, and keep
console controls dense enough for repeated operations.

## Radius, Borders, And Shadows

- Controls and compact panels: usually 8-12px.
- Cards and stable surfaces: usually 12-18px.
- Use larger radii only for an existing product convention or a deliberate
  marketing object.
- Light mode uses a thin border and a very weak contact shadow.
- Dark mode relies primarily on cool gray borders and subtle inner highlights.
- Hover may increase border contrast and add at most a small shadow or
  `translateY(-1px)`/`translateY(-2px)`.
- Do not use persistent glow, large tilt, or scale-up hover effects.

## Glass

Use `glass`, `glass-panel`, and `glass-popover` only where layering helps.
Provide a stable fallback background before `backdrop-filter`, keep blur around
12-22px, and keep saturation restrained. Text and focus states must remain
legible over both themes. Do not turn every card into transparent Glass.

## Frosted Precision Console

Official non-home product surfaces use **Frosted Precision**: restrained
glassmorphism over a calm, dense enterprise workbench. This direction connects
to the official `/home` through the same Indigo, Cyan, smoked material, fine
borders, and technical typography, while deliberately reducing spectacle once
the user enters a task-focused surface.

Apply the material hierarchy consistently:

1. Use a stable Mist or Obsidian canvas for the page.
2. Use Glass for navigation, authentication focus panels, filter strips,
   dropdowns, dialogs, popovers, and selected KPI accents.
3. Use opaque surface tokens for tables, long forms, charts, and primary work
   regions.
4. Never nest Glass panels or make every business card transparent.

The default material balance is approximately 20-30% Glass for user pages and
less for dense Admin pages. Authentication may use one stronger Glass panel as
the visual bridge from `/home`. Custom HTML, custom URL iframe, compact-home,
portal, and Station information architecture remain isolated; apply this
visual direction to Station only when the Station task explicitly includes it.

### Console Typography And Density

- Use `Noto Sans SC`, the existing system sans stack, and normal letter spacing
  for headings, labels, forms, and body text.
- Use the existing monospace stack only for API paths, keys, model IDs,
  technical labels, and tabular values.
- Keep page titles around 24px, section titles 16-18px, body and controls 14px,
  and metadata no smaller than 12px.
- Prefer 36px desktop controls, 40-44px touch controls, 44-48px table rows,
  16-20px panel padding, and 16-24px page rhythm.
- User surfaces use comfortable compact density. Admin and monitoring surfaces
  may be denser when scanning and target sizes remain accessible.

### Resilient Text And Compact Controls

- Allow headings to wrap naturally. `text-wrap: balance` is an enhancement,
  not a guarantee about the final line.
- Give flex/grid text children `min-width: 0`; let long URLs, keys, and
  identifiers use `overflow-wrap: anywhere`.
- Keep compact badge and chip labels on one line when practical. Bound only
  unpredictable values and provide an operable full-value disclosure when
  truncation is unavoidable.
- Wrap chip collections or provide a keyboard- and touch-operable `+n`
  disclosure. Do not hide overflow values in a fixed-height row.
- Preserve stable control dimensions during loading, validation, and async
  updates. Badge meaning must include text, icon, or shape rather than color
  alone.
- Let wide data tables scroll inside their table container. Do not compress
  identifiers and actions until they become unreadable.

### Console Charts

- Use Indigo for the primary series, Cyan for the secondary series, neutral
  gray for context, and semantic colors only for semantic state.
- Use line charts for time trends and horizontal or grouped bars for ranked
  comparisons. Use doughnut charts only for a small number of categories.
- Do not distinguish series by hue alone; add direct labels, line styles,
  point shapes, or a readable legend.
- Keep the plot on a stable surface. Glass is appropriate for chart tooltips
  and controls, not behind dense plotting detail.
- Provide an adjacent table, labels, or a concise text summary for important
  values that cannot be recovered without pointer hover.

## Buttons And Forms

Primary actions use Indigo. Secondary actions use a neutral surface and a
visible border. Ghost actions are low emphasis. Danger, warning, success, and
provider payment actions retain their independent semantic or provider colors.
Loading buttons keep their dimensions, disable duplicate activation, and show
an accessible state without layout shift.

Inputs use a surface background, a visible border, readable placeholders, and
an Indigo border plus a low-opacity ring on focus. Errors combine red border,
text, and an icon or other non-color signal. Disabled fields remain readable
and clearly inactive.

## Cards, Tables, And Data

Cards gain hierarchy from edge contrast, spacing, and surface level. Tables
stay high density: headers are slightly separated, row dividers are quiet,
and hover changes the surface without moving rows. Preserve existing columns,
ordering, sticky behavior, pagination, and business actions.

Charts use Indigo as the primary series, Cyan as the secondary series, neutral
gray for context, and semantic colors for status. Avoid rainbow palettes.
Tooltips use the overlay material and all important chart meaning must also be
available as text or labels.

## Status Colors

Status meaning is independent from the brand:

- Success: green, with text/icon/shape support.
- Warning: amber, with text/icon/shape support.
- Danger: red, with text/icon/shape support.
- Info: blue or Cyan, with text/icon/shape support.
- Primary Indigo is never a substitute for success, warning, or danger.

## Motion And GSAP Rules

CSS handles simple hover, focus, and feedback. GSAP handles a coordinated
sequence or a scoped marketing reveal. Use `transform` and `opacity`; do not
animate layout properties when a transform can express the same result.
Declare concrete transition properties. Never add `transition: all`.

GSAP code must:

- start after mount;
- use a component root or element references for selector scope;
- use a timeline for a coordinated sequence;
- use `gsap.matchMedia()` for responsive and reduced-motion conditions;
- call `ctx.revert()`/`mm.revert()` during unmount or mode changes;
- avoid accumulating timelines and kill or replace interruptible motion.

Target timings are 120-160ms fast, 180-240ms base, and 320-480ms slow.
Official home first-load storytelling may reach 700-1000ms when it remains
non-blocking. Console actions should usually complete within 300ms.

## Theme Switching

Use the existing dark class, local storage, system preference initialization,
and single theme source. A progressive View Transition or reveal effect may be
added only when feature-detected and must not flash, blank, or shift the page.
Reduced motion uses an immediate or short fade transition.

## Official Homepage Brand Rules

The default `/home` is the only major surface allowed to change information
architecture. It is a developer-first AI API gateway story: clear headline,
small CTA set, factual integration code, routing/failover, observability,
workflow, and a minimal footer. Use real ModuRelay endpoints and capabilities;
do not invent SLA, market leadership, unlimited availability, providers, or
unsupported routes.

The home hero follows strong typography plus one Relay Core object. It should
not be a wall of particles, visible grid, floating feature cards, or gradients.
The Relay Core is an original ModuRelay object using restrained obsidian,
smoked glass, metal, Indigo reflection, and a weak Cyan rim. It needs contact
shadow, roughness, light falloff, and a static fallback. Three.js must dispose
geometry, materials, textures, renderer, listeners, and RAF work, pause when
hidden, and become static or nearly static under reduced motion.

Marketing advertising sections, advertising navigation, advertising copy, and
advertising-only visual code do not belong on the official home. Advertising
business, admin, storage, and API capabilities remain untouched.

## Accessibility And Responsive Behavior

Every actionable control needs a visible `:focus-visible` state and a keyboard
path. Do not rely on hover or color alone. Preserve readable contrast and keep
important text above tertiary contrast. Use real labels/roles for tabs,
navigation, dialogs, and status surfaces.

At 430px and 390px widths, the home order is text, CTA, then Relay Core. Check
for no horizontal overflow, clipped labels, overlapping controls, or a 3D
canvas covering text. Verify light and dark at desktop, tablet, and mobile
sizes. Respect `prefers-reduced-motion` by disabling idle 3D, parallax,
particle movement, large translations, and scroll-linked effects while keeping
all content and actions available.

## Prohibited Patterns

- Resend, Linear, or Vercel source, assets, fonts, DOM, copy, or 3D objects.
- Pure-black pages, neon cyberpunk treatment, full-page gradients, or glowing
  borders.
- Blanket Glass cards, rainbow charts, persistent particles, or obvious grids.
- `transition: all`, unscoped GSAP selectors, or animations that block input.
- Backend/API/database changes made only to support visual work.
- Removing tests, weakening lint/type rules, or hiding accessibility failures.

## New UI Checklist

- Does the surface use semantic tokens and the correct theme hierarchy?
- Are brand and status colors separated?
- Are radius, border, shadow, and Glass choices intentional?
- Are focus, loading, disabled, error, empty, and reduced-motion states present?
- Are text labels and routes factual and preserved?
- Is motion scoped, interruptible, performant, and cleaned up?
- Has desktop, tablet, mobile, light, dark, and keyboard behavior been checked?

## Review Checklist

Before handoff, run typecheck, lint, focused tests, the full frontend test
suite, production build, and `git diff --check`. Inspect the rendered home,
auth, dashboard, keys, usage, model plaza, admin, Station, long table, form,
modal, drawer, dropdown, popover, toast, loading, empty, and error states.
Record any environment or CI limitation instead of claiming it passed.
