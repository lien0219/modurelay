---
name: modurelay-home-sylva
description: Adapt and maintain the verified ThreeUI Sylva Living Green scene inside the official ModuRelay /home experience. Use when work touches HomeView.vue, HomeHeroScene.vue, the vendored Sylva runtime or assets, homepage Three.js behavior, responsive scene framing, reduced motion, WebGL fallback, or Sylva source-integrity checks. Do not use for custom HTML, custom URL iframe, compact-home, portal, or Station surfaces.
---

# Adapt ModuRelay Home Sylva

## Establish the contract

1. Read the repository-root `AGENTS.md` and `docs/UI_DESIGN_SYSTEM.md` before editing UI.
2. Read [references/sylva-source.md](references/sylva-source.md) before touching the scene host or vendored files.
3. Inspect the current `frontend/src/views/HomeView.vue`, `frontend/src/components/home/HomeHeroScene.vue`, `frontend/src/utils/threeRuntime.ts`, and focused homepage tests.
4. Preserve routes, permissions, public settings, API payloads, business copy, feature flags, test selectors, and all non-official-home modes.

## Preserve the verified source

1. Treat `frontend/public/threeui/sylva/inner-green-3d.html` and its `inner-green-assets` directory as byte-exact vendor files.
2. Fetch registered source only from the URLs recorded in the reference when a refresh is explicitly required.
3. Verify every registered SHA-256 before using refreshed source or assets.
4. Never reconstruct Sylva from screenshots, previews, descriptions, or filenames.
5. Keep ThreeUI, Three.js, image, and Lexend license notices beside the vendored files.

## Adapt only the host boundary

1. Keep ModuRelay on Vue 3; do not add React, React DOM, or `@designcodeio/threeui`.
2. Load the canonical document from a same-origin sandboxed iframe owned by `HomeHeroScene.vue`.
3. Keep the canonical document unchanged. Apply scene-only presentation CSS after load so only its verified `#scene` canvas is visible.
4. Keep all ModuRelay navigation, headings, factual product copy, endpoints, and actions in `HomeView.vue`, outside the vendor document.
5. Forward pointer coordinates through the host when the iframe is presentation-only and cannot receive pointer events directly.
6. Use the existing page progress only to frame or grade the outer scene. Do not rewrite the authored shaders, geometry, motion, or renderer internals.

## Keep the homepage production-ready

1. Use the official homepage semantic tokens and restrained Obsidian Indigo, Indigo, and Cyan treatment around the Living Green scene.
2. Keep the first viewport focused on ModuRelay, a factual AI gateway value proposition, a small CTA set, and the real compatible endpoint.
3. On 430px and 390px layouts, keep text and actions before the visual focal point with no horizontal overflow or canvas overlap.
4. Preserve a useful static fallback when WebGL, iframe access, source loading, or context creation fails.
5. Respect reduced motion and browser visibility behavior. Avoid additional idle animation, persistent particles, glow, or scroll effects outside the authored renderer.
6. Tear down parent listeners, timers, polling, and the iframe document on unmount. Treat iframe removal as the lifecycle boundary for the authored document.
7. Do not add `transition: all`, unscoped GSAP selectors, unsupported claims, visual-only backend changes, or cross-surface styles.

## Verify the result

1. Recompute the registered source and asset hashes.
2. Run focused homepage tests, frontend typecheck, targeted lint, the full frontend test suite, the production build, and `git diff --check`.
3. Inspect the official `/home` in light and dark modes at desktop, tablet, 430px, and 390px widths.
4. Check canvas pixels, initial load, pointer response, reduced motion, resize, visibility changes, WebGL failure, context loss, console errors, and unmount cleanup.
5. Confirm custom HTML, custom URL iframe, compact-home, portal, and Station modes are unchanged.
