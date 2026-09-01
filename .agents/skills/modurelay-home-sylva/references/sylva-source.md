# Sylva Living Green Source

## Registered component

- Component: `SylvaHero`
- Variant: `living-green`
- Runtime: Full HTML, DOM/CSS, and local Three.js
- Source revision: `SHA-256 05f359ce157a`
- Canonical HTML: `https://threeui.com/landing-pages/inner-green-3d.html`
- Registered bundle: `https://threeui.com/source-code/sylva-hero.json`

The source renders a procedural moss-root world with pale flowers, ferns, drifting pollen, a landing butterfly, responsive framing, pointer interaction, and reduced-motion behavior. Read the complete registered bundle and canonical document before changing the host.

## Configured ThreeUI usage

The copied prompt selected these values:

```tsx
<SylvaHero
  variant="living-green"
  headingFont="lexend"
  bodyFont="lexend"
  headingWeight="300"
  bodyWeight="300"
  primaryColor="#ffffff"
  headingSize={63}
  bodySize={16.5}
  headingLetterSpacing={-0.006}
/>
```

ModuRelay does not run this React wrapper. Preserve the selected Living Green document and adapt only its Vue host. Keep ModuRelay typography, semantic tokens, copy, and actions outside the vendor document.

## Registered text files

| Path | Role | SHA-256 |
| --- | --- | --- |
| `src/shaders/landing-pages/LandingPages.tsx` | Component host | `d34af7b5bf8239835102e34de46e7cae69ab29f875acffd3050209ba64b684f5` |
| `src/shaders/landing-pages/pageTypography.ts` | Controls source | `809cc65797d531cd3b3ca5a56815d55d24b3ee8d293e4e4bad6fdfe6c83244cc` |
| `src/shaders/landing-pages/pageRecipes.ts` | Controls source | `c9d9849cc255bac2d1d938d088c50917f84916f1c516d2bbb27fcfd803523233` |
| `public/landing-pages/inner-green-3d.html` | Canonical source | `69c3694bd63f44ef9f007ebe4dac57a83e4402e0cdf6b54dd10b96dd4f05e197` |
| `public/landing-pages/inner-green-assets/three.min.js` | Three.js runtime | `8a5f7249903b54d30f79f708699d2fed2d6a1d0741a4cd41377d1f01bb5a2271` |
| `src/shaders/threeui.css` | Shared style | `efe4447139f1358dd8e9be68edf6fa46cbefbd1de423a4d6c439ca61d2c8eccf` |

## Registered binary assets

| Path | MIME | Bytes | SHA-256 |
| --- | --- | ---: | --- |
| `public/landing-pages/inner-green-assets/card-ecostove.jpg` | `image/jpeg` | 290988 | `70ce084084902bc502f00c366405b661ecdff90dee95d363b36a6e146829e433` |
| `public/landing-pages/inner-green-assets/card-ethos.jpg` | `image/jpeg` | 316720 | `337627390f499b3ae272cec9e2f83c817694a82f42e1aa10a7b26a2c7d679dff` |
| `public/landing-pages/inner-green-assets/lexend-latin.woff2` | `font/woff2` | 39692 | `1ec8f6ee2750554b4bc59ff0b507d316a82a7ba37e0e5bebc41d3bd9b9faad46` |

## ModuRelay runtime layout

- Canonical document: `frontend/public/threeui/sylva/inner-green-3d.html`
- Runtime and binary assets: `frontend/public/threeui/sylva/inner-green-assets/`
- License notices: `frontend/public/threeui/sylva/licenses/`
- Vue host: `frontend/src/components/home/HomeHeroScene.vue`
- Official page: `frontend/src/views/HomeView.vue`

Keep the canonical document byte-exact. The Vue host may inject presentation CSS after load to hide authored Sylva page chrome while preserving the exact `#scene` renderer. Do not expose Sylva navigation, marketing copy, buttons, or links inside ModuRelay.

## Licensing

- ThreeUI Community application and ThreeUI-authored scene assets: MIT, copyright 2026 Meng To.
- Bundled Three.js runtime: MIT with its upstream header retained.
- Lexend: SIL Open Font License 1.1, copyright 2018 The Lexend Project Authors, Reserved Font Name `RevReading Lexend`.
