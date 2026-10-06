# Enterprise Workspace Full Roadmap

This document is the durable progress source for the Enterprise Workspace program requested for `feature/new-feature`. It records the current capability audit, architecture decisions, phase boundaries, verification state, risks, and deferred scope. It is updated at every production phase boundary.

## Current baseline

- Branch: `feature/new-feature`
- HEAD at audit: `fcdf84bba fix(ci): resolve frontend audit and golangci-lint failures`
- Recent enterprise baseline: `9ed858729 feat(policies): add hierarchical gateway policy engine`
- Working tree at audit: Phase A changes present on top of `fcdf84bba`; no unrelated files were reset or cleaned.
- Migration ceiling at audit: `293_workspace_governance_teams_project_access.sql`

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
| Verified domains | Missing | No workspace domain model/routes | Phase B |
| Generic OIDC/JIT/SSO discovery/enforcement | Missing | No workspace IdP binding flow | Phase B |
| SAML 2.0 | Missing | No SAML SP model or protocol flow | Phase C |
| SCIM 2.0 | Missing | No SCIM resource/token lifecycle | Phase C |
| Workspace security policy | Missing | Existing policy engine is gateway policy, not identity security policy | Phase D |
| FinOps anomaly detection | Missing | No anomaly finding model/worker | Phase E |
| Cost centers/tags/environment allocation | Missing | Usage snapshots do not expose these dimensions | Phase F |
| Retention/export/deletion lifecycle | Partial | Existing retention workers cover current event/notification/webhook data; enterprise export/deletion is absent | Phase G |
| Enterprise admin diagnostics | Partial | Global workspace list/inspect/status exists; health/backlog/search controls are absent | Phase H |
| Production hardening | Partial | Existing tests cover many billing/policy invariants; full enterprise matrix does not yet exist | Phase I |
| Functional freeze, unified UI, final release gate | Not started | Explicitly deferred until functional phases are complete | J–L |

## Missing Capability Matrix

| Missing capability | Contract | Planned phase | Blocking risk |
| --- | --- | --- | --- |
| Teams and team membership | Workspace membership collection, SCIM Group-compatible shape | A | Cross-tenant member attachment |
| Direct/team project grants | Fixed viewer/developer/admin roles, no owner grant | A | IDOR and accidental loss of existing access |
| Project access mode | Default `all_projects`, explicit `assigned_projects` | A | Backward compatibility |
| Verified domain ownership | Normalized exact domains, hashed DNS token, 409 ownership conflict | B | Account discovery and takeover |
| OIDC | Verified JWT/JWKS, state/nonce/PKCE, SSRF-safe discovery | B | Token forgery and SSRF |
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
| A | Teams, Project Access Grants, restricted project mode | Fixed built-in roles; direct/team grants; default all-project compatibility; owner/admin/billing compatibility retained; no custom roles | 293 | `feat(governance): add workspace teams and project access controls` | Backend unit/service/handler/repository/migration tests; PostgreSQL governance and key isolation integration; full frontend Vitest, lint, typecheck, build; diff review | PASS | Cross-tenant subject IDs are checked in repository transactions and database trigger; project-key, service-account, legacy key list/search, policy, budget, and usage paths recheck project access | Custom roles; browser/manual acceptance; real providers |
| B | Domains, generic OIDC, JIT, discovery, enforcement | Global User plus workspace identity binding; verified domains; SSRF-safe discovery | 294+ | Not started | Not run | NOT RUN | Account linking, token verification, SSRF | SAML/SCIM |
| C | SAML 2.0 and SCIM 2.0 | Mature protocol library; idempotent SCIM resources mapped to teams | 295+ | Not started | Not run | NOT RUN | XML wrapping, lifecycle races | Advanced identity federation |
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

## Deferred scope

STS, AWS-style temporary credentials, workload identity federation, PrivateLink, VPC peering, BYOK/HSM, native SIEM integrations, DLP, automatic remediation, multi-level ERP departments, complex legal hold, generic BPM, arbitrary ABAC, and Kubernetes operators are explicitly outside this Enterprise edition.

## Verification policy

Every phase records targeted tests, full relevant tests, security/diff review, and explicit `PASS`, `FAIL`, `PRE-EXISTING`, or `NOT RUN` classification. A failure is compared with the clean parent baseline before it is classified. No push or PR is part of this workstream.
