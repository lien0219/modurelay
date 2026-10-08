# Phase D — Workspace Security Policy delivery

## Branch, baseline, commit and working tree

Branch: `feature/new-feature`. The initially clean baseline was
`6981e17de52d87ba4723b28452ad8e455e0317b5`, the local Phase C2 boundary.
This phase is delivered as the local commit named
`feat(security): add workspace security policies and assurance enforcement`.
Its exact SHA and final working-tree result are recorded by the delivery
response; a commit cannot embed its own SHA. No push, PR or deployment belongs
to this delivery. The manually managed acceptance instance on port 18081 was
not changed.

## Architecture findings and decision

The existing SQL SSO policy, Global Session metadata, Workspace assurance,
membership sources and recovery flow provided the foundation. The missing
controls were complete JWT Session context propagation, actual-session MFA,
general Workspace age, provider approvals, a single transaction-bound admission
policy, and functional policy/recovery UI. The architecture gate chose to extend
those foundations without rewriting billing or machine execution.

See [the architecture gate](WORKSPACE_SECURITY_POLICY_ARCHITECTURE.md),
[the policy contract](WORKSPACE_SECURITY_POLICY.md), and
[the implementation plan](superpowers/plans/2026-10-08-workspace-security-policy.md).

## Security policy model

`WorkspaceSecurityPolicy` is canonical and `WorkspaceIdentityPolicy` remains a
source-compatible alias. Migration 297 extends `workspace_security_policies`.
Defaults retain SSO/grace/revision values and add MFA=false, age=NULL,
invitations=`any`, external members=true, Workspace JIT=true and
providers=`any_active`. Selected providers use a tenant-scoped relation.
Omitted PATCH fields retain their values; explicit null clears age/grace.

## Authentication assurance and MFA semantics

Signed Global Session authentication carries method, original authentication
time and `MFASatisfied`. JWT, OptionalJWT and AdminJWT propagate that complete
trusted snapshot. `TotpEnabled`/`MFAEnrolled` choose recovery guidance and never
manufacture MFA. Password plus verified local TOTP, enterprise completion plus
verified local TOTP, and verified exact-family TOTP upgrade establish MFA.
OIDC/SAML, passkey, password and OAuth alone remain conservatively MFA=false.
IdP AMR/ACR/AuthnContext and WebAuthn assertion-level MFA mapping are deferred.

The optional Session upgrade on `/user/totp/step-up` atomically consumes and
rotates the exact current refresh family. It matches user/version/family/browser
binding, both original clocks, Workspace/provider/revision and assertion
deadline. Cross-family proof and replay are rejected. It preserves the original
Global and enterprise authentication times and does not add a new login clock.
Frontend adoption is serialized with ordinary refresh and cross-tab Web Locks.

Sensitive-operation proof separately carries
`RecentAuthenticationProof{VerifiedAt, MFASatisfied}`. Time-only primary proof
cannot satisfy an enrolled factor. Policy mutation and break-glass recheck live
enrollment under the actor lock. Verified recent MFA does not forge durable
Session MFA, and legacy strong/time assertions remain primary proof only.

## TOTP security

Production logs no longer include encrypted/decrypted TOTP secret prefixes,
setup/QR material or temporary login token prefixes. A real DEBUG logger-capture
regression checks the emitted logs. Transactional TOTP disable checks active
Organization Workspace MFA dependencies and fails closed on SQL errors; Global
Admin factor requirements remain. Session enrollment is not used as access proof.

## Session age, SSO and approved providers

General age is NULL or an integer 900–2592000 seconds and is measured since the
original Global authentication. Refresh and step-up cannot extend it. Enterprise
assurance retains its named 12-hour ceiling; SAML assertion/session deadlines
may shorten it further. Historical OIDC wire names remain compatible with both
OIDC and SAML, with additive `OIDCValidUntil` propagated end to end.

Human access composes original age, required SSO, matching Workspace, active
provider/protocol/revision, selected approval and actual MFA with AND. SSO grace
waives only SSO, never MFA or age. Provider changes and policy approval writes
serialize under Workspace; removing the last usable provider while SSO is
required fails. An unchanged expired grace deadline may be retained; only a
newly changed grace is constrained to the future and seven-day maximum.

## Domains, external members, invitations and JIT

All verified `workspace_domains.normalized_domain` entries form the exact IDNA
domain set. No suffix/subdomain inheritance or arbitrary text allowlist exists.
Zero verified domains with a restrictive policy denies new admission and is
explained in the UI. Tightening preserves existing active members and sources.

One pure `WorkspaceMemberAdmissionPolicy.Check` and one SQL
`checkWorkspaceMemberAdmissionTx` govern Invitation create/accept, OIDC and SAML
JIT creation/reactivation, SCIM User creation/reactivation and administrative
restoration. The helper derives live member/admin/provider state from SQL under
the Workspace transaction; caller hints cannot assert existing active access.

Invitation acceptance rechecks current policy and current human assurance before
any member/source/token write. Denial retains the pending invitation. JIT uses
Workspace JIT AND provider JIT, current approval, and verified email/domain
requirements. Denials roll back user, member, source, resource and audit/event
effects. Administrative suspension/removal wins over automated restoration.

## SCIM integration and machine separation

SCIM Bearer provisioning uses the shared member admission policy and protocol
error envelope, without human SSO/MFA/age. Human connector management is protected
by Workspace security and RBAC. Existing source attribution and effective active
access are retained; reactivation rechecks current admission.

Direct API Key and Service Account runtime retain Credential AND active
Workspace/Project AND Gateway Policy AND quota/budget AND scheduler controls.
Human policy does not enter Gateway caches, billing attribution or task ownership.
Personal Workspaces remain independent.

## Break-glass, RBAC and API

All tenant member roles may read; only Owner may update, with recent strong
authentication. GET/PATCH/preview remain protected by current Workspace security.
PATCH requires positive `expected_revision`; stale revisions return 409 without
partial writes. Selected provider IDs are tenant scoped and enabling controls
rechecks current Owner prerequisites to avoid self-lockout.

Owner recovery requires fresh password and TOTP when enrolled, explicit
confirmation, a 10–500-character reason, active ownership and the existing
15-minute Workspace rate limit. It relaxes SSO/MFA/general age only, preserving
provider approvals, admission restrictions and membership. A stale enrollment
hint or time-only proof cannot bypass the locked recent-factor guard. No weak
lost-factor bypass is introduced.

`SECURITY_POLICY_UNAVAILABLE` is 503 and fails closed. Denial metadata contains
only safe string fields `workspace_id`, `requires_sso`, `requires_mfa` and
`policy_revision`. Recovery and enrollment retain a valid Global Session.
Full route/payload/error contracts are in the policy document.

## Frontend

The Security page is the single policy editor; Identity provides summary/link.
The functional UI covers all fields, read-only roles, prerequisite warnings,
provider pagination, optimistic conflicts, exact unchanged timestamp precision,
tenant/session-scoped stale responses and actionable enrollment/MFA/SSO/age
recovery. Invitation rejection retains its token. EN/ZH strings and existing
selectors/routes are retained. Repository Frosted tokens and UI rules govern the
official non-home surfaces; this phase does not redesign `/home` or portal modes.

## Audit, events, notification and webhooks

Policy mutation commits redacted before/after audit, revision,
`workspace.security_policy.updated` and outbox in one transaction. Legacy SSO
and break-glass events remain. Owner/Admin notification recipients and Workspace
webhooks recognize the new security event. Payloads contain bounded allowlisted
scalar metadata, not credentials or identity claims. Denial does not generate
unbounded audit or tenant-labeled metrics.

## Database, migration and compatibility

Migration 297 preserves historical SSO/grace/revision/updater, users, members,
typed sources, API keys, Service Accounts, usage and immutable billing snapshots.
It adds enum/age checks, tenant/provider composite foreign keys and the additive
login-completion assertion deadline. Real PostgreSQL migration-history tests
check preservation, rerun behavior and cross-tenant constraints. No historical
hot-data rewrite or old migration modification occurs.

Legacy refresh/wire contracts, API-key compatibility and machine-plane behavior
remain tested. Refresh-family deletion still does not immediately revoke an
already signed access JWT absent password-derived token-version change. Live
policy/provider checks apply immediately to subsequent human requests. A global
access-token denylist/session redesign is deferred.

## Concurrency review

Policy revision, provider approval/disable, same-actor factor disable/enable,
invitation policy changes, OIDC/SAML JIT, SCIM create/reactivation, administrative
restoration and domain revocation have real PostgreSQL checks. The expanded
admission test covers all seven source entry paths.

Review reproduced an actual User `FOR UPDATE` / Workspace-first audit foreign-key
deadlock. The actor now uses `FOR NO KEY UPDATE`, preserving exclusive factor
and lifecycle serialization while allowing audit FK `KEY SHARE`. The actual
repository regression has RED/GREEN and race-detector evidence. No Workspace
lock was added to ordinary factor disable that would reverse JIT/SCIM FK order.
Overlapping different-member disable/Owner enable can linearize as
disable-before-enable; ordinary unenrolled members are supported and still need
actual Session MFA to access a required-MFA Workspace.

## Security review and formal scan

Independent authentication and policy/admission reviews trace trusted proof,
shared admission, IDOR, provider revision/approval, recovery and machine-plane
separation. Every tenant handler is enumerated by authorization tests, covering
more than 70 routes and policy-unavailable denial. Legacy key list/search/count
SQL paths also enforce current SAML/MFA/original-age/provider approval.

The durable Codex Security diff scan is
`02d22b4b-ba5a-4e0e-9303-35f114289de5`, with initial working-tree digest
`codex-security-snapshot/v1:sha256:54bff4c75307cefe326d22251b8727fa8e6d2b74c4ed3d7f6825fee092ff2b56`.
Post-review lock, typed-proof and grace changes are separately hashed and reviewed;
the initial digest is never presented as the final source digest. The durable
scan is finalized and indexed, with report.md and SARIF. It retains two medium
initial-snapshot recent-proof findings, both corrected before delivery. The
underlying HTTP SSO recovery enrollment-read window predates Phase D; that
finding is bounded to the newly added MFA/age relaxation sink. No original
external attacker-to-database execution is claimed.

The workbench warns that the working tree changed and keeps its results bound
to the original digest. Its sealed coverage is **partial**: an early checkpoint's
deferred text, `Discovery/validation evidence awaiting normalization; the final
review has not been sealed.`, remains in the generated canonical coverage even
after all six candidate validations, both eligible attack paths, final draft and
successful sealing. This stale checkpoint entry is disclosed as an artifact
limitation; the sealed result is not edited or replaced to manufacture complete
coverage. Independent post-review source, unit, PostgreSQL and race checks verify
the fixes separately; no residual blocker was found in that reviewed scope.
Do not treat scan completion or its two remediated titles as zero findings or
complete final-commit security certification. A future immutable final-commit
formal scan remains a Phase I/L follow-up.

Final 101-file source aggregate SHA-256:
`c613d3eedc2feb5e66de153b64805663975a42c8b82678cd6ee98ac820938c09`.
The snapshot file under `.cache/phase-d/final-source-snapshot.json` records each
file hash. The canonical report is retained at
`C:/Users/Administrator/AppData/Local/Temp/codex-security-scans-slqRV6/modurelay/6981e17de52d87ba4723b28452ad8e455e0317b5_20261008T013219Z_j1e7ql33/report.md`;
the SARIF is its sibling `exports/results.sarif`.
Plugin-returned token measurement has complete measurement coverage:
total 18,988,981; input 18,878,549; cached input 17,297,536; output 110,432.
The reported source is `codex_rollout`, aggregating two threads. This is the
tool's measured accounting, not an estimate of the scan alone or its cost.

Bounded source secret scanning reports no production credential candidates.
Production dependency manifests/locks did not change. Current and exact baseline
pnpm audit have the same 1 moderate/3 low advisories and 0 high/critical; the CI
exception check passes. Govulncheck has the same 11 module-only advisories on
current/baseline, 0 imported-package and 0 reachable-function findings, exit 0.
These results are not a claim of zero dependency advisories.

## Performance review

Security checks run on the human control plane and reuse existing Workspace
access and policy/provider reads. There is no human-policy query added to the
Gateway hot path/cache. Mutation serialization is bounded by the existing
Workspace row. Provider lookup occurs only for relevant enterprise assurance.
No production load/latency benchmark was run; Workspace-lock contention and
large-domain/provider/member sets remain a deployment acceptance measurement.

## Tests and verification boundaries

Evidence logs are retained locally under ignored `.cache/phase-d/`; they are not
committed. The complete final command/result table is appended at closure.

| Gate | Recorded result |
| --- | --- |
| Security/auth/admission/IDOR focused tests | PASS; final root unit service0.271s/handler0.100s/middleware0.053s/migrations0.018s, and independent proof/handler unit0.053s/0.194s. |
| Real PostgreSQL policy/admission/OIDC/SAML/SCIM race | PASS, 89.336s; additional seven-path/domain-revocation and review-fix checks. |
| Relevant service/handler/middleware/repository race | PASS; final broad authentication/assurance unit race service84.536s/handler64.790s/middleware1.395s/repository3.456s/migrations1.064s. |
| Full default and integration backend suites | Raw FAIL only three PgDumper tests requiring unavailable Windows `sh`; identical exact-baseline reproduction, PRE-EXISTING environment. |
| Full unit backend suite | Raw FAIL: same PgDumper cases and baseline-reproduced Ollama CAS; one initial inflight-memory threshold failure separately disclosed below. |
| Full service unit retry | FAIL only baseline-reproduced Ollama CAS; inflight-memory case passed. |
| Inflight memory repeat | Baseline 30/current 30 repetitions PASS. Initial threshold failure is not labeled PRE-EXISTING. |
| Frontend stable full suite | PASS: 430 files / 3193 tests; precedes only the two-file grace precision fix. |
| Frontend grace precision fix | RED 2 expected failures, GREEN 19 Security tests; lint/typecheck/fresh production build PASS. |
| Frontend i18n/full lint/typecheck/build | PASS, with final grace affected checks; Vite chunk advisory retained. |
| Canvas critical and embedded assets | PASS: 7 tests; final Bun production bundle13.27s and `go test -tags=embed ./internal/web -count=1`0.080s. Frontend build finished before Canvas regeneration. |
| Dependency gates | PASS at repository threshold with the unchanged advisories described above. |
| Browser/real IdP/real upstream/load/deployment acceptance | NOT RUN. |

The initial full-unit inflight memory assertion measured 8391064 bytes against
8388608. An identical failure was not established on baseline. Both isolated
30-repeat runs and the current full-service unit retry pass that assertion. It
is recorded as an unreproduced transient with final successful retests, not
silently classified as baseline failure and not fixed by weakening assertions.

## Security gate questions

| Question | Answer |
| --- | --- |
| Q1 `TotpEnabled` without current Session MFA permits access? | NO |
| Q2 Refresh advances `AuthenticatedAt`? | NO |
| Q3 Workspace A MFA changes Personal Workspace access? | NO |
| Q4 Workspace A SSO satisfies Workspace B? | NO |
| Q5 SSO + MFA permits satisfying only one? | NO |
| Q6 Unapproved provider satisfies required SSO? | NO |
| Q7 Pending external invitation survives newly disabled external admission? | NO |
| Q8 Invitation policy denial consumes token? | NO |
| Q9 Provider JIT bypasses `workspace_jit=false`? | NO |
| Q10 SCIM Bearer is blocked by human SSO/MFA? | NO |
| Q11 Human SCIM control plane is protected? | YES |
| Q12 Direct API Key runtime gains human MFA requirements? | NO |
| Q13 Service Account runtime gains human security requirements? | NO |
| Q14 TOTP secret prefix remains in production logs? | NO |
| Q15 SCIM/JIT restores administrative suspension? | NO |
| Q16 Stale policy cache bypasses new MFA? | NO — live SQL, no allow cache |

## Remaining risks and manual acceptance

Browser acceptance, real IdP/TOTP/passkey/provisioning providers and production
load are NOT RUN. Use
[WORKSPACE_SECURITY_POLICY_ACCEPTANCE.md](WORKSPACE_SECURITY_POLICY_ACCEPTANCE.md)
for the complete operator runbook. The 18081 runtime was not redeployed.
Retain a persistent encryption key and validate operational recovery before
enabling restrictive policy. Access-token revocation semantics, external-member
retention, conservative passkey/IdP MFA recognition, and Workspace-lock contention
are explicit boundaries described above, not hidden implementation gaps.

## Roadmap

Phase D implementation, relevant security/verification and diff gates are
complete; the authorized local commit forms its phase boundary. Phase E is NEXT
and remains unimplemented. Formal-scan artifact limits and runtime acceptance
retain the explicit statuses above. FinOps anomalies, retention/export,
advanced diagnostics, arbitrary ABAC, risk/device/geo/IP controls, IdP-initiated
SAML/SLO, SCIM Bulk and unified UI refactoring remain deferred.

## Final supplemental command results

| Command/check | Result and local evidence |
| --- | --- |
| `gofmt -l` for every changed/new Go file | PASS, no output. |
| `go vet ./...` | PASS after all review fixes; `backend-vet-post-review.log`. |
| `go build ./...` | PASS after all review fixes; `backend-build-post-review.log`. |
| Pinned `golangci-lint v2.13.0 run ./...` | PASS, 0 issues; `backend-lint-post-review.log`. |
| Final root focused unit command | PASS; `security-unit-post-review-root.log`. |
| Author affected PostgreSQL / same selection race | PASS24.119s /21.652s; exact commands in `policy-admission-remediation.md`. |
| Independent PostgreSQL five-test command | PASS9.117s, none skipped; `auth-independent-remediation-postgres.log`. |
| Root original-control overlay | Expected behavioral RED reproduced: time-only proof returns nil; `recent-proof-overlay-red-root.log`. Final source remains unchanged. |
| Final bounded secret scan and 101-file hashes | PASS; `secret-scan-final.json`, `final-source-snapshot.json`. |
| Exact source hashes after independent review | PASS, no drift. |
| Formal Codex Security scan | COMPLETE scan lifecycle; sealed coverage PARTIAL; two initial-snapshot medium findings explicitly remediated. |
| Final diff/status/staging boundary | Reviewed before local commit; only explicit Phase D source and documentation are staged. Cache, overlays, scan evidence and generated assets remain unstaged/ignored. |
| Existing acceptance runtime health | Read-only `GET http://127.0.0.1:18081/health` returned200 with status ok. This does not run Phase D acceptance or deploy the new source. |

The full default/unit/integration suites were run serially before the final
review-driven lock/proof/grace corrections; affected tests, actual PostgreSQL,
race, vet/build/lint and frontend affected checks were rerun afterward. No claim
is made that the final 101-file hash passed a second complete three-tag backend
suite. All raw failures and baseline/retry boundaries above remain visible.
