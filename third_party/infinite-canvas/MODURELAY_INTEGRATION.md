# ModuRelay Integration

This directory vendors the complete source tree of
[`basketikun/infinite-canvas`](https://github.com/basketikun/infinite-canvas)
at commit `d213a74614e0e4bd8a26383d1e1e907249e9c61b`.

The upstream project is licensed under the MIT License. Its original
[`LICENSE`](LICENSE) and copyright notice are retained unchanged.

ModuRelay-specific changes are intentionally limited to the web application:

- build and route the React application below `/infinite-canvas/`;
- resolve runtime config, logo, and local plugin assets below that base path;
- provide navigation back to the ModuRelay console;
- share the ModuRelay language and light/dark preference;
- allow an authenticated user to explicitly reuse an available grouped API key
  or create a dedicated key after selecting one of the account's available
  groups, then store the selected key in Infinite Canvas's existing
  browser-local configuration;
- expose a canvas account center with the current user, balance, concurrency,
  available groups, and masked reusable keys without rendering full secrets;
- fetch third-party model lists through a narrow authenticated ModuRelay
  endpoint when the canvas is served below `/infinite-canvas/`;
- route cross-origin AI provider traffic through `/api/v1/canvas/upstream`,
  including image, text, video, audio, custom model-script, polling, and
  generated-media requests, while preserving standalone direct/local-proxy
  behavior and rejecting private-network targets;
- hand off navigation between the Vue console and React canvas through a
  text-free, reduced-motion-aware frosted-glass double-door transition;
- use the Ant Design 6 `Modal.styles.container` slot required by the locked
  dependency version so the vendored source passes TypeScript validation.

Infinite Canvas projects, assets, and API configuration remain browser-local
unless the upstream application explicitly synchronizes them. This
integration does not claim database-backed cloud synchronization.
