# Repository UI Rules

All future UI changes must first read [`docs/UI_DESIGN_SYSTEM.md`](docs/UI_DESIGN_SYSTEM.md).
For official non-home surfaces, also use
[`modurelay-console-frosted`](.agents/skills/modurelay-console-frosted/SKILL.md)
and consult the project-local
[`ui-ux-pro-max`](.agents/skills/ui-ux-pro-max/SKILL.md) search data. The
repository design system and existing product behavior always take priority
over generic skill recommendations. Official `/home` work uses the separate
[`modurelay-home-sylva`](.agents/skills/modurelay-home-sylva/SKILL.md) skill.

Preserve routes, permissions, API payloads, business behavior, custom homepage
and portal modes, and existing test selectors unless the task explicitly
authorizes a behavior change. Keep the official `/home` visual system separate
from custom HTML, iframe, compact, and Station surfaces. Do not add
`transition: all`, unscoped GSAP selectors, unsupported marketing claims, or
visual-only backend changes.
