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
- allow an authenticated user to explicitly create a dedicated ModuRelay API
  key and store it in Infinite Canvas's existing browser-local configuration.
- use the Ant Design 6 `Modal.styles.container` slot required by the locked
  dependency version so the vendored source passes TypeScript validation.

Infinite Canvas projects, assets, and API configuration remain browser-local
unless the upstream application explicitly synchronizes them. This
integration does not claim database-backed cloud synchronization.
