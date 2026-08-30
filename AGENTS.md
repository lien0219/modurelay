# Repository UI Rules

All future UI changes must first read [`docs/UI_DESIGN_SYSTEM.md`](docs/UI_DESIGN_SYSTEM.md).

Preserve routes, permissions, API payloads, business behavior, custom homepage
and portal modes, and existing test selectors unless the task explicitly
authorizes a behavior change. Keep the official `/home` visual system separate
from custom HTML, iframe, compact, and Station surfaces. Do not add
`transition: all`, unscoped GSAP selectors, unsupported marketing claims, or
visual-only backend changes.
