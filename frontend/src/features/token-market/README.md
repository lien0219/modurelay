# Token Market Frontend Architecture

`token-market` is a standalone product surface and domain module. It intentionally does not inherit the dashboard `AppLayout`/sidebar visual system.

## Module boundaries

- `types.ts` — stable frontend/domain contracts. Keep API DTO mapping outside UI components.
- `domain.ts` — pure deterministic rules such as exchange calculations and balance validation. No Vue, network, router, storage, or DOM dependencies.
- `service.ts` — service boundary. The default data source is an isolated in-memory mock; an HTTP repository is available behind `VITE_TOKEN_MARKET_DATA_SOURCE=http` and must only be enabled when the corresponding backend routes are deployed.
- `store.ts` — Pinia orchestration and async state. Components must not call HTTP directly.
- `mockData.ts` — prototype fixtures only. Production code must not infer business rules from fixture values.
- `route.ts` — feature-owned route metadata for the standalone page.
- `__tests__/` — domain/store/component regression tests.

## Integration rules

1. UI components may depend on the Pinia store and exported domain formatting helpers; they must not import Axios or backend endpoints directly.
2. Monetary/Token calculations must not be duplicated in Vue templates or click handlers. Move deterministic calculations to `domain.ts` and cover them with tests.
3. Real exchange execution must become server-authoritative. The frontend quote is display-only; the backend must return a quote ID, expiry, applied rate, source amount, destination amount, and idempotent transaction result.
4. Never trust frontend balances. When HTTP integration lands, `executeExchange` must refresh/replace the wallet snapshot using the server response.
5. Exchange, payment, refund, settlement, and wallet mutations require idempotency keys and immutable ledger records on the backend. The frontend service contract is deliberately shaped so these fields can be added without changing page layout.
6. Token and fiat/platform-balance values must use exact decimal representations on the backend. JavaScript numbers are acceptable only for the current visual prototype; the production HTTP adapter should transport decimal values as strings if backend precision requires it.
7. Product, merchant, cart, order, delivery, and settlement capabilities should become subdomains under this feature rather than being embedded in the landing page.
8. Feature flags and permission guards belong in route/settings contracts, not scattered across components.
9. External image/CDN dependencies are forbidden for the market shell. Approved visual assets belong under `frontend/public/token-market/` or the future asset pipeline.
10. The standalone page may share global fonts/reset utilities, but it must not depend on dashboard layout components or dashboard card design tokens unless explicitly adopted by the Token Market design system.

## Expected backend API surface

The frontend is prepared for these logical endpoints (exact URLs can follow backend conventions):

- bootstrap: wallet snapshot, categories, featured products, featured merchants, activities, exchange configuration
- exchange quote: direction + amount -> signed/expiring quote
- exchange execute: quote ID + idempotency key -> transaction + authoritative wallet snapshot
- search: query/filter/pagination -> product/merchant results
- wallet: balances, holds, ledger entries
- cart/order: cart mutations, checkout quote, order creation, status
- merchant: store profile, listings, availability

## Migration from mock to HTTP

`HttpTokenMarketRepository` already satisfies the repository contract and maps the bootstrap, product, order, wallet, search, exchange quote, and exchange execution DTOs. Enable it only after the backend routes and authorization contract are available. Do not rewrite page event handlers or domain rules to make the HTTP adapter work.

The current backend checkout has no Token Market routes. Checkout quotes, order creation, Token deduction, payment status, refunds/after-sales, merchant onboarding/review, listing publication/inventory, merchant fulfillment, merchant ledger/fees/settlement, messaging, and official reference-price APIs remain integration gaps. The UI keeps these paths visible with loading, empty, failure, and blocked-submit states rather than fabricating balances, deductions, or successful transactions.

## Production gates

Before enabling real funds or Token transfers:

- typecheck, lint, unit tests and build are green;
- exchange quote expiry and replay are tested;
- insufficient balance, duplicate submit and concurrent execution are tested server-side;
- wallet/ledger invariants are verified;
- order/refund/settlement flows have idempotency coverage;
- no secrets, raw upstream credentials, or sensitive merchant inventory are exposed to the browser;
- feature flag defaults to disabled until backend migrations and operator configuration are complete.
