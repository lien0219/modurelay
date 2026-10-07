# Phase C Enterprise Identity Implementation Plan

> Agentic workers: execute tasks in order with Superpowers implementation and review workflows. The user's requested branch and two phase-boundary commits override temporary-branch and per-task-commit defaults.

**Goal:** Add SAML 2.0 SSO and SCIM 2.0 provisioning without changing Global User, machine credential, billing, or historical usage semantics.

**Architecture:** Extend the existing identity-provider parent with nullable protocol columns and protocol-dependent PostgreSQL checks. OIDC and SAML share stable subject binding, provisioning, mappings, login completion, JWT assurance and Require SSO. SCIM is an independent authenticated connector and manages only attributed Workspace/Team sources.

**Tech stack:** Go 1.27, PostgreSQL, existing Gin/Redis/Vue services; gosaml2 v0.12.0 and goxmldsig v1.6.1 for SAML cryptography.

**Spec:** The Phase C requirements provided on 2026-10-07; protocol details and gate decisions are persisted in ENTERPRISE_SAML.md and ENTERPRISE_SCIM.md.

## Global constraints

- Stay on feature/new-feature; no push, PR, reset, clean, forced checkout or temporary branch.
- Baseline is 81e68865463755f3fb684199b3de5a31e8572713; preserve unrelated changes.
- C1 and C2 have separate verified local commits; do not begin D/E/F.
- Do not modify migration 294 or rewrite historical Usage/Billing/Key/Service Account rows.
- Owner and Billing Owner remain manually managed. Existing-email SSO requires explicit authenticated linking.
- Provider type, workspace, revision and original authentication time must all match assurance.
- SCIM credentials are high entropy, hash-only and shown once. SCIM cannot issue human sessions.
- Every source removal affects only that source; administrator suspension wins over provisioning activation.
- SCIM Groups bind explicitly to Workspace Teams; no display-name takeover or alternate Project ACL.
- SAML signatures/canonicalization/encryption come from maintained libraries; no custom XML DSig.
- API/UI preserve existing selectors, permissions, OIDC payload compatibility and semantic design tokens.
- Use local protocol fixtures and real PostgreSQL/concurrency tests. Real IdPs and Phase L manual acceptance remain NOT RUN.

## Task 1: C1 provider persistence and shared SSO core

Files: enterprise_identity_service.go, enterprise_identity.go, enterprise_oidc.go, enterprise_identity_repo.go, enterprise_saml_repo.go, migration 295, backend identity/migration tests.

Interfaces: EnterpriseIdentityProviderInput gains Type and SAML config; parent returns SAML config and opaque PublicID. GetProvider's internal secret result selects the independent encrypted SAML SP-key field for SAML. Protocol-neutral aliases retain old OIDC names. Auth states gain Protocol and RequestID. SAMLReplayRepository adds scoped public-provider lookup and atomic response/assertion replay consumption.

- [x] Add failing protocol/persistence tests for mixed providers, no dummy fields, tenant scope, redaction, revisions, single default, protocol-specific states and completion.
- [x] Create idempotent migration 295 with nullable OIDC fields, SAML config and independent encrypted SP keys, strict protocol CHECKs, SAML membership source, state request correlation and replay tables.
- [x] Reuse existing provider CRUD, scoped transactions, identity binding and mapping pipeline; derive source and signup attribution from validated provider type.
- [x] Generalize assurance checks and completion persistence while preserving OIDC API and refresh fields.
- [x] Run identity/migration/real PostgreSQL tests, review task diff and security invariants.

Representative invariant test:

```go
provider.Type = "saml"
assurance.AuthMethod = "saml"
require.NoError(t, identity.CheckWorkspaceAccess(ctx, workspaceID, "organization", "human", assurance))
assurance.AuthMethod = "oidc"
require.ErrorIs(t, identity.CheckWorkspaceAccess(ctx, workspaceID, "organization", "human", assurance), ErrSSORequired)
```

## Task 2: C1 SAML protocol, metadata, keys and browser flow

Files: enterprise_saml*.go, auth_enterprise_saml.go, auth_enterprise_sso.go, auth_enterprise_completion.go, auth routes, workspace SAML control routes, Go modules and protocol fixture tests.

- [x] Write failing tests for metadata limits, XXE, trusted certificates, subject stability and signed local IdP fixtures.
- [x] Implement metadata XML/URL import with the existing pinned HTTPS transport and bounded XML parser; validate certificate count, key types and validity.
- [x] Generate per-provider RSA SP key/certificate, encrypt private material with SecretEncryptor and expose only public metadata.
- [x] Build signed HTTP-Redirect AuthnRequests; bind opaque single-use RelayState to browser, workspace, provider revision, request ID, return path and link intent.
- [x] Validate POST ACS with strict size limits, XML roundtrip/signature validation, issuer/destination/audience/recipient/request/timing/status checks, encrypted assertions and atomic ID replay checks.
- [x] Convert verified assertions to common enterprise claims. Persistent NameID or explicit stable attribute is required; email is trusted only within signed verification and Workspace-domain provisioning rules.
- [x] Reuse same-origin completion exchange, TOTP and original-time JWT/refresh assurance; account-linking and Owner recovery remain shared.
- [x] Test signature wrapping, unsigned/invalid/mismatched/expired responses, replay, browser state, JIT/linking and OIDC regressions.

## Task 3: C1 functional frontend and phase gate

Files: workspace API/types, enterpriseSSO API, WorkspaceIdentityView.vue, SAML provider component, bilingual locales, frontend tests, ENTERPRISE_SAML.md, ENTERPRISE_SAML_ACCEPTANCE.md, ENTERPRISE_SSO.md and roadmap.

- [x] Preserve OIDC controls/selectors and add provider-type selection and SAML fields; reuse JIT and mappings.
- [x] Display/copy SP entity ID, ACS and metadata URLs. Add metadata import and certificate lifecycle controls with permissions, loading/error states and visible labels.
- [x] Run focused and full frontend tests, i18n, lint, typecheck and production build; run backend C1 relevant tests/race/PostgreSQL and dependency/security/diff review.
- [x] Record evidence and deferred external/manual gates, mark C1 COMPLETE/C2 IN PROGRESS and commit feat(saml): add enterprise SAML single sign-on.

## Task 4: C2 typed source attribution and SCIM persistence

Files: migration 296, workspace source repositories, enterprise SCIM repository/service files, workspace_mutation.go and shared identity reconciliation.

- [x] Add failing manual+SSO+SCIM and administrator-suspension regression tests.
- [x] Create scoped connector/token/user/group and typed membership/Team-source tables with composite tenant FKs and bounded control-plane backfill.
- [x] Reconcile effective Member/Team state from live sources, preserving manual role and explicit administrator suspension.
- [x] Upgrade OIDC/SAML reconciliation so complete-empty groups cannot delete another protocol or SCIM source.
- [x] Implement locked, atomic SCIM resource mutations, normalized verified-email adoption/creation, opaque IDs, source-only deprovision and Owner/Billing Owner 409 protection.
- [x] Verify eight-way duplicate POST, state races, Group PATCH and token rotation/revoke on real PostgreSQL.

## Task 5: C2 SCIM protocol and control plane

Files: SCIM service/parser/handler/router files, central provisioning permissions, app wiring and event/notification/webhook integration.

- [x] Implement authenticated connector-specific Users/Groups/config/schema/type endpoints and SCIM error/media contracts.
- [x] Implement bounded allowlisted eq filters, 1-based pagination, replacement PUT, idempotent add/replace/remove PATCH and safe repeated DELETE.
- [x] Support If-Match and revision versions; serialize resource mutations under the Workspace lock.
- [x] Add connector/token rate limits with explicit fail-closed Redis behavior, bounded payload/operation/member limits and debounced last-used tracking.
- [x] Add human RBAC and Require SSO control-plane gates, one-time tokens, coexistence rotation, expiry/revoke and safe event summaries/important notifications.
- [x] Verify tenant IDOR, injection, giant payload, secret leakage and no global-user/session/credential mutation.

## Task 6: C2 frontend, full verification and delivery

Files: SCIM API/component, WorkspaceIdentityView.vue, bilingual locales, frontend tests, ENTERPRISE_SCIM.md, ENTERPRISE_SCIM_ACCEPTANCE.md, ENTERPRISE_WORKSPACES.md and roadmap.

- [x] Add connector/default-role/group binding, Base URL, create/rotate/revoke token and last-sync/error UI. Token is displayed only in immediate create result and cleared on dismissal/unmount.
- [x] Run C2 protocol/PostgreSQL/concurrency/race and source regression tests; perform independent security/code review.
- [x] Run gofmt, default/unit/integration full backend suites, vet/build/pinned golangci-lint, dependency audit and full frontend i18n/lint/typecheck/Vitest/build plus Canvas critical regression.
- [x] Compare exact failures against archived 81e688 baseline; only reproduced matches are PRE-EXISTING. Correct regressions.
- [x] Review final diff/secrets; record all PASS/FAIL/PRE-EXISTING/NOT RUN evidence and security Q1-Q16.
- [x] Mark Phase C COMPLETE, Phase D NEXT and commit feat(scim): add enterprise identity provisioning; verify clean tree and two local commits at this delivery boundary.

## Execution record

Local evidence and active task ledger live under .cache/phase-c/. They are not release artifacts and are not committed. Exact phase SHAs are resolved from Git commit messages and reported at delivery; C2 roadmap records the immutable C1 SHA.

Final C2 source is the independently reviewed v2 source after R1. Whole-phase
review plus scoped R1 approval closes all Critical/Important findings. Latest
post-R1 relevant/PostgreSQL/race/vet/build/default lint pass; final full-suite
failure sets match the actual archived baseline. The initial external TLS
reset/refusal and its mismatch are preserved separately, with full matched
baseline/current reruns. Supplemental integration-tag lint has13 findings
reproduced exactly on baseline. The report and acceptance docs retain these
boundaries; real providers/browser/operator/load/deployment remain Phase L.
