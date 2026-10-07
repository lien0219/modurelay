# Enterprise Workspace Full Roadmap

This document is the durable progress source for the Enterprise Workspace program requested for `feature/new-feature`. It records the current capability audit, architecture decisions, phase boundaries, verification state, risks, and deferred scope. It is updated at every production phase boundary.

## Current baseline

- Branch: `feature/new-feature`
- HEAD at Phase B audit: `828ccb8419807d2af89fc2adbb654909686634b0 feat(governance): add workspace teams and project access controls`
- Recent enterprise baseline: Phase A governance on top of `fcdf84bba` and the policy/Service Account/notification foundations.
- Phase B worktree: implementation changes on the audited baseline; the final continuation inherited those changes without staged/committed Phase B files, kept the existing branch and did not reset, clean or discard unrelated changes.
- Migration ceiling: 293 at the Phase B audit; Phase B adds `294_enterprise_identity_oidc.sql`.

## Current Enterprise Capability Matrix

| Capability | Current state | Evidence | Phase |
| --- | --- | --- | --- |
| Global users and personal workspaces | Implemented | `273_workspace_foundation.sql`, `workspace_repo.go` | Baseline |
| Organization workspaces | Implemented | `WorkspaceService.CreateOrganization`, workspace routes | Baseline |
| Workspace membership/RBAC | Implemented | `workspace_access.go`, owner/billing lifecycle guards | Baseline |
| Invitations | Implemented | workspace invitation mutation and acceptance flow | Baseline |
| Projects and project-scoped keys | Implemented | migration 273, workspace/project routes | Baseline |
| Billing principal and usage attribution | Implemented | workspace FinOps/billing repositories and snapshots | Baseline |
| Budgets and reservations | Implemented | migrations 275, 280 and budget repositories | Baseline |
| Audit, domain events, outbox, notifications | Implemented | migrations 280, 281 and existing workers | Baseline |
| Workspace webhooks | Implemented | migration 281 and workspace webhook service | Baseline |
| Service Accounts and credential rotation | Implemented | migrations 284–289 and service account service | Baseline |
| Hierarchical gateway policies and quota reservations | Implemented | migrations 290–292, policy resolver/admission | Baseline |
| Workspace Teams | Implemented | `293_workspace_governance_teams_project_access.sql`, `WorkspaceService`, `/workspaces/:id/teams`, bilingual Teams view | Phase A |
| Project Access Grants | Implemented | Tenant-scoped direct/team grants, fixed viewer/developer/admin roles, project routes and UI | Phase A |
| Explicit restricted project-access mode | Implemented | `workspaces.project_access_mode`, default `all_projects`, project filtering and legacy key-read parity | Phase A |
| Verified domains | Implemented | Migration 294, canonical IDNA/global claim uniqueness, one-time hashed TXT, transactional DNS check/audit/outbox | Phase B |
| Generic OIDC and provider presets | Implemented | Unified Authorization Code/PKCE, browser state/completion, JWT/JWKS, HTTPS/DNS pinning, redacted encrypted secrets | Phase B |
| Linking/JIT and role/Team mappings | Implemented | Global User/subject binding, explicit link, verified-email/domain JIT, source-aware reconciliation and live Phase A grants | Phase B |
| Discovery, assurance, SSO enforcement/recovery | Implemented | Tenant route and legacy key management gates, revision/age invalidation, locked Owner enable gate, password/TOTP recovery | Phase B |
| SAML 2.0 | Missing | No SAML SP model or protocol flow | Phase C |
| SCIM 2.0 | Missing | No SCIM resource/token lifecycle | Phase C |
| Workspace security policy | Foundation implemented; broader controls deferred | Phase B `workspace_security_policies` has SSO/grace; Phase D adds MFA, session/domain/invitation controls | Phase B / D |
| FinOps anomaly detection | Missing | No anomaly finding model/worker | Phase E |
| Cost centers/tags/environment allocation | Missing | Usage snapshots do not expose these dimensions | Phase F |
| Retention/export/deletion lifecycle | Partial | Existing retention workers cover current event/notification/webhook data; enterprise export/deletion is absent | Phase G |
| Enterprise admin diagnostics | Partial | Global workspace list/inspect/status exists; health/backlog/search controls are absent | Phase H |
| Production hardening | Partial | Existing tests cover many billing/policy invariants; full enterprise matrix does not yet exist | Phase I |
| Functional freeze, unified UI, final release gate | Not started | Explicitly deferred until functional phases are complete | J–L |

## Missing Capability Matrix

| Missing capability | Contract | Planned phase | Blocking risk |
| --- | --- | --- | --- |
| SAML | Mature library, signature/audience/destination/clock validation | C | XML signature wrapping |
| SCIM | Idempotent provisioning/deprovisioning and owner/billing safety | C | Lifecycle invariant breakage |
| Security policy | MFA/SSO/session/domain/invitation controls | D | Human control-plane bypass |
| Anomaly/cost allocation | Immutable snapshots and bounded aggregation | E–F | Historical billing drift |
| Lifecycle/export/admin | Retention floors, tenant-owned exports, controlled emergency actions | G–H | Data loss and operator overreach |
| Cross-module hardening | Tenant matrix, concurrency, chaos/recovery, rehearsal | I | Production integrity |

## Architecture Risks and gates

1. **Project authorization is currently workspace-role based.** Phase A must make the access snapshot authoritative for every project-scoped read/write path, including raw SQL key/service-account repositories.
2. **Backward compatibility is a P0.** The database default and all code paths must preserve `all_projects` until an owner/admin explicitly chooses restriction.
3. **Team and grant subject IDs are tenant-scoped.** Every mutation must validate subject ownership while holding the workspace lock; route IDs alone are never sufficient.
4. **Owner and billing-owner invariants already span user lifecycle triggers.** Team/grant mutations must reuse those invariants and never add an alternate owner path.
5. **Control plane and gateway plane stay separate.** Teams, SSO, SCIM, and security settings must not enter the model-request hot path; gateway admission reads only the existing tenant/principal/policy/budget projections.
6. **Secrets remain outside durable control-plane payloads.** OIDC, SAML, SCIM, export, and domain verification secrets require the existing encryption/hash/show-once patterns.
7. **Historical usage is immutable.** New cost dimensions are snapshots on new writes; no full-table historical rewrite is allowed.

## Phase plan and status

| Phase | Scope | Architecture decision | Migration | Commit | Tests | Status | Risks | Deferred |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| A | Teams, Project Access Grants, restricted project mode | Fixed built-in roles; direct/team grants; default all-project compatibility; owner/admin/billing compatibility retained; no custom roles | 293 | `828ccb8419807d2af89fc2adbb654909686634b0` | Backend unit/service/handler/repository/migration tests; PostgreSQL governance and key isolation integration; full frontend Vitest, lint, typecheck, build; diff review | COMPLETE | Cross-tenant subject IDs are checked in repository transactions and database trigger; project-key, service-account, legacy key list/search, policy, budget, and usage paths recheck project access | Custom roles; browser/manual acceptance; real providers |
| B | Verified domains, OIDC/presets, linking/JIT/mappings, discovery, assurance/enforcement/recovery | Global User; provider/subject binding; no email adoption or Owner/Billing Owner escalation; SSRF-safe protocol; source-aware reconciliation; control-plane/Gateway separation | 294 | `feat(sso): add verified domains and OIDC enterprise SSO`; resolve SHA below | Mock/protocol/PostgreSQL/concurrency/race PASS; full frontend 421 files/3103 tests plus lint/typecheck/build PASS; Go full suites have identical parent-baseline failures; source/security/diff review PASS | COMPLETE | Retained encryption key; provider claim differences; real-provider/load/manual gates not run | SAML/SCIM; Graph overage fetching; periodic DNS health; full Phase D security policy |
| C | SAML 2.0 and SCIM 2.0 | Shared SSO/provider binding and typed provisioning sources; see Phase C plan | 295–296 | C1 local boundary below | C1 complete; C2 in progress | IN PROGRESS | Lifecycle races, admin suspension | IdP-initiated SSO, SLO; real IdPs in Phase L |
| C1 | SAML 2.0 Enterprise SSO | Nullable protocol fields with DB integrity; gosaml2 v0.12.0; shared JIT/link/mapping/completion/assurance | 295 | `feat(saml): add enterprise SAML single sign-on` | Complete | COMPLETE | Certificate/key retention and IdP interoperability | IdP-initiated SSO, SLO |
| C2 | SCIM 2.0 Enterprise Provisioning | Independent hash-token connector; attributed Membership/Team sources; explicit Group binding | 296 | Not committed | Not started | IN PROGRESS | Cross-source removal and Owner/Billing safety | Real Entra/Okta provisioning in Phase L |
| D | Workspace security policy | Control-plane middleware with explicit assurance context | 296+ | Not started | Not run | NOT RUN | Break-glass and API-key separation | Arbitrary ABAC |
| E | Advanced FinOps anomalies | Immutable usage snapshots plus bounded detector jobs | 297+ | Not started | Not run | NOT RUN | False positives, cardinality | AI remediation |
| F | Cost centers, tags, environment allocation | New-write dimensions and explicit legacy NULLs | 298+ | Not started | Not run | NOT RUN | Historical attribution drift | ERP tree |
| G | Retention, export, archive/restore, deletion lifecycle | Tenant-owned jobs, retention floors, resumable purge | 299+ | Not started | Not run | NOT RUN | Data loss, legal retention | Complex legal hold |
| H | Admin diagnostics and operations | Global Admin remains outside tenant membership | 300+ | Not started | Not run | NOT RUN | High-cardinality metrics, emergency actions | SIEM integration |
| I | Production hardening | Lifecycle/isolation/concurrency/chaos/migration rehearsal gates | 301+ | Not started | Not run | NOT RUN | Recovery and billing integrity | New business features |
| J | Functional freeze | Only bugs, UI, docs, release blockers after gate | None expected | Not started | Not run | NOT RUN | Scope creep | — |
| K | Unified UI/UX | Frosted Precision for official non-home surfaces after functionality freeze | None expected | Not started | Not run | NOT RUN | Visual regressions | Home Sylva changes |
| L | Final release gate/manual acceptance | Full backend/frontend/security/migration/performance gate | None expected | Not started | Not run | NOT RUN | Real-provider availability | — |

## Phase A execution boundary

Phase A is complete only when the plan's acceptance matrix is evidenced by tests and a local commit. The implementation includes backend authorization, raw SQL and legacy key-list parity, tenant isolation, audit/outbox atomicity, API routes, bilingual minimal UI, and roadmap evidence. Real-provider and final browser acceptance remain deferred to Phase L.

## Phase A verification record

- **PASS** — targeted Go tests for `internal/service`, `internal/handler`, and `migrations` passed; the focused governance repository tests also passed.
- **PASS** — PostgreSQL integration: `go test ./internal/repository -tags=integration -run 'TestWorkspace(RestrictedModeHidesLegacyOrganizationKeysWithoutProjectGrant|GovernanceTenantScopedAccessAndAtomicEvents)$' -count=1`, including cross-tenant grant rejection, team/direct grant precedence, restricted project listing, legacy key-list hiding, and atomic audit/event/outbox checks.
- **PASS** — `pnpm run check:i18n`; English and Chinese workspace keys are complete and the Chinese file contains no replacement `?` characters.
- **PASS** — focused Workspace governance/API/component Vitest coverage (22 tests in the Phase A run).
- **PASS** — full frontend Vitest (`1021` suites, `3024` tests), `pnpm run lint:check`, `pnpm run typecheck`, `pnpm run build`, and `git diff --check`.
- **PRE-EXISTING** — the complete `internal/repository` package command is blocked by the existing `backup_pg_dumper` tests because Windows cannot find `sh` (`exec: "sh": executable file not found in %PATH%`); the cleanup/unlock follow-on expectation is a consequence of that process-start failure. The same parent-baseline environment failure is recorded here rather than treated as a Phase A regression.
- **NOT RUN** — authenticated browser/manual acceptance, external provider smoke tests, load/chaos testing, and final release acceptance remain Phase L work.

The local Phase A commit is created only after this verification record and the final diff/security review are current. No push or PR is part of this phase.

## Phase B execution boundary

Phase B is **COMPLETE** at the local phase-boundary commit. Phase C (SAML/SCIM)
is **NEXT**. Final release/manual acceptance remains Phase L; this phase does
not establish real-provider or production acceptance.

### Architecture decisions and migration

- Keep Global User and Personal Workspace. Identity binds Workspace/provider/
  subject to the existing User; authenticated explicit linking is required for
  an existing email. No auto Owner or Billing Owner mutation.
- Use the existing JWT/Redis token family, local TOTP/step-up, AES-GCM,
  tenant RBAC, Phase A project grants and audit/event/outbox/notification/webhook
  systems. Refresh preserves original authentication and assurance times.
- Provider revisions invalidate assurance; enabling SSO rechecks the requesting
  Owner's exact provider/revision/binding under the Workspace lock. Every human
  tenant route and legacy Organization key management/list/search enforce SSO;
  Gateway Direct/Service Account credentials retain their existing path.
- Claim mapping is bounded; explicit role priority is deterministic. Missing/
  overage groups preserve grants; complete empty groups reconcile only current
  provider attribution. Manual, SCIM and other-provider grants survive.
- Migration `294_enterprise_identity_oidc.sql` adds control-plane tables and
  manual-default source attribution, composite tenant constraints, one active
  default provider, global active-domain uniqueness, auth-method/secret CHECK,
  single-use encrypted state/completion and Owner recovery limits. No historical
  usage, billing, key or Service Account rewrite.
- Operational/session/security setup and all architecture/security gate answers:
  [ENTERPRISE_SSO.md](ENTERPRISE_SSO.md). Deferred manual matrix:
  [ENTERPRISE_SSO_ACCEPTANCE.md](ENTERPRISE_SSO_ACCEPTANCE.md).

### Verification record (2026-10-07)

Evidence is retained locally under `.cache/phase-b/`; test credentials are
isolated fixture values, and these local artifacts are not committed.

| Check | Result | Evidence |
| --- | --- | --- |
| Mock OIDC/service/handler/middleware/refresh protocol | PASS | ID-token signature/claims, discovery, PKCE/state/nonce, JWKS rotation/backoff/cache, local MFA, same-origin exchange, account linking, tenant routes and refresh metadata; full package runs and `identity-race-final.log` |
| Enterprise PostgreSQL matrix | PASS | `go test -tags=integration ./internal/repository -run '^TestEnterpriseIdentity' -count=1 -timeout=5m`; 28 integration tests, `identity-postgres-final.log` |
| Full migration history and preservation | PASS | Historical migrations through 293, seeded Phase A/Gateway/billing rows, 294 applied twice and exact historical snapshots; `identity-migration-history.log` |
| Database concurrency/race | PASS | Simultaneous domain claims, provider revision edits, state consumption, competing subject links, coherent role/Team reconciliation and 8-callback JIT; `identity-postgres-concurrency-race-final.log` |
| Legacy key and provider rollback boundaries | PASS | Search/list/counts, wrong Workspace/method, expired assurance, grace deadline; rejected provider edits preserve defaults/revision/audit/events/outbox; `identity-additional-boundaries.log` |
| `go test ./... -count=1 -timeout=10m` | PRE-EXISTING | Only three PgDumper missing-`sh` failures; exact baseline reproduction in `pgdumper-baseline.log`; final `backend-default-reviewed-final.log` |
| `go test -tags=unit ./... -count=1 -timeout=10m` | PRE-EXISTING | Same three PgDumper failures plus Ollama stale-callback CAS failure; full identical-command `828ccb841` baseline has the same failure set, no difference; `backend-unit-reviewed-final.log` / `backend-unit-baseline.log` |
| `go test -tags=integration ./... -count=1 -timeout=10m` | PRE-EXISTING | Only the same three PgDumper failures; full baseline integration has the identical failure set; final `backend-integration-reviewed-final.log` / `.cache/backend-integration-baseline.log` |
| `go vet ./...`, `go build ./...` | PASS | `backend-vet-reviewed-final.log`, `backend-build-reviewed-final.log` |
| Relevant Go race | PASS | `CGO_ENABLED=1 go test -race -p=2 ./internal/service ./internal/handler ./internal/server/middleware -run 'TestEnterprise|TestWorkspaceSSO|TestRefreshToken' -count=1 -timeout=5m`; `identity-race-final.log` |
| Pinned golangci-lint 2.13.0 | PASS | Official checksum-verified tool, `0 issues`, `backend-lint-reviewed-final.log` |
| Full frontend Vitest | PASS | 421 files / 3103 tests; `frontend-vitest-reviewed-final.log` |
| Frontend lint/typecheck/production build | PASS | `frontend-{lint,typecheck,build}-reviewed-final.log`; existing Vite build warnings remain non-failing |
| Canvas critical regression | PASS | `pnpm run test:canvas`, 7 Seedance video tests; `canvas-final.log` |
| Seedance/Grok/Gateway regression | PASS | Included in the full backend/frontend runs; model-credential runtime remains separate from human SSO |
| Formatting, final diff and secret review | PASS | gofmt, `git diff --check`, explicit phase-file review; no real private key/token or focused/skipped-test/bypass additions |
| Source security/concurrency review | PASS | Independent read-only review; all blocking findings corrected and re-read, minor form issue corrected; no outstanding source blocker. This is not a formal Codex Security plugin scan. |
| Real Entra / Google Workspace / Okta | NOT RUN | Actual tenant/client registrations and provider claims require real-provider validation |
| Final browser/manual, load/chaos/release | NOT RUN | Deferred acceptance document and Phase L gate |

Earlier resource-contention-sensitive current runs failed additional cancel/
lease/WS-frame/heap timing checks. Exact targeted reruns and the final complete
serial backend runs pass those checks. They are not labeled PRE-EXISTING;
only the freshly reproduced identical parent failures above receive that label.

### Commit SHA and deferred risks

- Parent SHA: `828ccb8419807d2af89fc2adbb654909686634b0`.
- Phase-boundary message: `feat(sso): add verified domains and OIDC enterprise SSO`.
- Authoritative Phase B commit SHA is resolved from Git metadata using:

  ```sh
  git log feature/new-feature -1 --format=%H --fixed-strings --grep="feat(sso): add verified domains and OIDC enterprise SSO"
  ```

  A file cannot embed the immutable SHA of its own containing commit without
  changing that SHA. This lookup and the delivery report identify the exact
  local boundary without a separate metadata commit.
- Real IdP differences remain unverified. In particular, Entra JIT needs signed
  verified-email proof; UPN or domain possession alone does not relax it. Graph
  overage expansion, periodic DNS health/reverification, pending-domain claim
  reclamation/advanced operator tooling, IdP-side zero-downtime secret rotation,
  Global Admin emergency override and full Phase D policy remain deferred.
- Retain/back up the encryption key and exact callback configuration. Production
  load, Workspace-lock contention, migration rehearsal, rendered responsive/
  keyboard states and Owner recovery drill remain final acceptance work.
- Local commit only; no push, PR or acceptance deployment is part of Phase B.

## Deferred scope

STS, AWS-style temporary credentials, workload identity federation, PrivateLink, VPC peering, BYOK/HSM, native SIEM integrations, DLP, automatic remediation, multi-level ERP departments, complex legal hold, generic BPM, arbitrary ABAC, and Kubernetes operators are explicitly outside this Enterprise edition.

## Verification policy

Every phase records targeted tests, full relevant tests, security/diff review, and explicit `PASS`, `FAIL`, `PRE-EXISTING`, or `NOT RUN` classification. A failure is compared with the clean parent baseline before it is classified. No push or PR is part of this workstream.

## Phase C1 completion record (2026-10-07)

C1 is COMPLETE at the verified local commit `feat(saml): add enterprise SAML single sign-on`.
The immutable C1 SHA will be recorded by C2; resolve the current boundary with
`git log --all --format='%H %s' --grep='^feat(saml): add enterprise SAML single sign-on$'`.
C2 is IN PROGRESS; D/E/F have not started.

Migration 295 extends the provider parent with genuine nullable protocol fields,
strong SQL checks, separate encrypted SP keys, opaque public identity and request/replay correlation.
OIDC/SAML reuse stable binding, JIT/linking, role/Team mappings, completion/MFA,
original-time assurance, Require SSO and Owner recovery. Per-provider RSA3072 keys
support stage/promote and bounded previous-key decryption overlap. gosaml2 v0.12.0
and goxmldsig v1.6.1 own crypto. Metadata and ACS are bounded; HTTPS/DNS pinning,
XML wrapping/unsigned/replay and configured-origin/browser proofs are tested.

PASS: relevant backend and real PostgreSQL identity/governance (43.498s), race,
full frontend 422 files / 3112 tests, lint/typecheck/i18n/build (26.71s final build),
Go vet/build, pinned golangci-lint v2.13.0 (0 issues), current dependency audit,
secret/diff review and independent C1 spec/quality APPROVAL against v4.
Review findings F0-F3 (optional-attribute serialization, HTTP audit body omission,
rotation async state, unspecified NameID request policy) are fixed and independently rechecked.

PRE-EXISTING: exact archived 81e688 baseline and final default/unit/integration
commands reproduce three PgDumper Windows missing-sh failures. Unit also reproduces
Ollama stale-callback CAS. The full parallel integration run had a Phase B grace-boundary
failure that passed targeted and final serial full reruns; it is not labeled PRE-EXISTING.
Final serial integration has only the identical baseline failures.

govulncheck: 0 reachable/import-package vulnerabilities; 11 required-module advisories
in uncalled code. Production pnpm audit exception checker passed.
NOT RUN: external Entra/Okta/Google SAML, rendered browser/operator/load/deployment.
Deferred: IdP-initiated SSO, SLO, POST-only AuthnRequest; real-provider acceptance Phase L.
No historical Usage/Billing/Key/Service Account rewrite or acceptance runtime change.
Operational details: ENTERPRISE_SAML.md; evidence/boundaries: ENTERPRISE_SAML_ACCEPTANCE.md.
