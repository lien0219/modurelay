# Workspace Security Policy — Phase D

Phase D extends the existing `workspace_security_policies` table with migration
297. It governs human access to Organization Workspaces and admission of new or
reactivated members. Personal Workspaces, Direct API Key and Service Account
Gateway execution, provider scheduling, media tasks and external SCIM Bearer
authentication retain their separate admission paths.

Architecture findings and the implementation gate are recorded in
[WORKSPACE_SECURITY_POLICY_ARCHITECTURE.md](WORKSPACE_SECURITY_POLICY_ARCHITECTURE.md).
Operational acceptance is in
[WORKSPACE_SECURITY_POLICY_ACCEPTANCE.md](WORKSPACE_SECURITY_POLICY_ACCEPTANCE.md).
Validation and delivery are recorded in
[WORKSPACE_SECURITY_POLICY_DELIVERY.md](WORKSPACE_SECURITY_POLICY_DELIVERY.md).

## Policy model and defaults

`WorkspaceSecurityPolicy` is canonical. `WorkspaceIdentityPolicy` is a
source-compatible alias; there is no separate MFA, invitation or session policy.

| Field | Default | Meaning |
| --- | --- | --- |
| `require_sso` | Existing value; false for new policies | Require current Workspace enterprise assurance after any SSO grace deadline. |
| `sso_grace_until` | Existing value or NULL | UTC SSO-only grace deadline; never waives MFA or session age. |
| `require_mfa` | false | Current human Session has server-verified local TOTP evidence. |
| `session_max_age_seconds` | NULL | Additional limit since original Global Session authentication; integer 900–2592000. |
| `invitation_policy` | `any` | `any`, `verified_domains_only` or `disabled`. |
| `allow_external_members` | true | False requires an exact verified Workspace email domain for new admission across all sources. |
| `workspace_jit_enabled` | true | Additional AND condition for new/reactivated OIDC/SAML JIT membership. |
| `approved_identity_provider_mode` | `any_active` | `any_active` or `selected`. |
| `approved_identity_provider_ids` | [] | Tenant-scoped provider relation; meaningful for `selected`. |
| `revision` | Existing value or 1 | Monotonic optimistic concurrency revision. |

Verified domains reuse `workspace_domains` with `status='verified'` and exact
canonical IDNA matching. Subdomains do not inherit approval. There is no
arbitrary text domain allowlist. Restrictive admission with zero verified
domains fails closed; the UI explains that outcome. Existing active members
remain and the UI reports their external-member count.

## Actual Session authentication

`SessionAuthentication` carries original `AuthMethod`, `AuthenticatedAt` and
`MFASatisfied`. `MFAEnrolled` is a live hint used to choose an enrollment or
verification error; it cannot authorize access. JWT, OptionalJWT and AdminJWT
propagate the complete trusted Session into service context.

Password plus verified local TOTP, enterprise completion plus verified local
TOTP, and an exact-family TOTP Session upgrade establish MFA. TOTP enrollment,
password alone, OAuth alone, OIDC/SAML alone and passkey alone do not. Phase D
does not map IdP AMR/ACR/AuthnContext or WebAuthn assertion flags to Workspace MFA.

The existing `/user/totp/step-up` accepts optional `refresh_token`. Without it,
the existing recent sensitive-operation grant remains. With it, the server
verifies local TOTP, matches access Session ID, user, token version, refresh
family, browser binding, both original authentication clocks, Workspace,
provider/revision and assertion deadline, then atomically consumes the refresh
token and rotates credentials in the same family. Cross-family/changed proof
returns 403 `SESSION_MFA_UPGRADE_INVALID`; replay returns 409
`SESSION_MFA_UPGRADE_REUSED`. Neither clears a valid Global Session with 401.

The upgrade response adds `access_token`, `refresh_token`, `token_type` and
`token_expires_in`; legacy `expires_in` is still the recent-operation grant TTL.
The browser serializes upgrade with ordinary refresh and cross-tab Web Locks,
rejects changed user/family/token snapshots, and adopts only the returned stored
pair. Browser-decoded claims scope UI work and never establish security proof.

## Session age and enterprise assurance

Historical `OIDC*` wire fields remain protocol-independent aliases for OIDC and
SAML, avoiding accidental invalidation of existing sessions. Additive
`OIDCValidUntil` preserves verified SAML Conditions, SubjectConfirmation and
SessionNotOnOrAfter deadlines through completion, JWT, refresh and TOTP upgrade.

Enterprise assurance retains the named 12-hour ceiling. Workspace age may
shorten that lifetime, and assertion expiry may shorten it further:

`original Global authentication age AND enterprise age < 12h AND assertion deadline AND live provider revision`.

NULL general age adds no further Workspace limit. A configured 24-hour general
age does not extend enterprise assurance beyond 12 hours. Refresh and MFA
upgrade change neither original clock. Legacy sessions with missing original
time require reauthentication when a general age limit is configured.

`EvaluateWorkspaceSecurity` composes age, required SSO, tenant/provider/protocol
and revision, provider approval, and actual Session MFA with AND. Workspace RBAC,
membership, user state and Project Access Grants remain additional controls.

## One member admission policy

The pure `WorkspaceMemberAdmissionPolicy.Check` is consumed by one SQL helper,
`checkWorkspaceMemberAdmissionTx`. It loads current policy, verified domains,
member state, administrative flags and provider state under the caller's
Workspace transaction lock. Caller input cannot assert `existing_active` or
override SQL administrative state.

| Entry path | Shared admission checks |
| --- | --- |
| Invitation create | Current invitation mode, exact domain and external-member restrictions before token creation. |
| Invitation accept | Recheck latest admission policy, inviter permissions, matching email and invitation state; evaluate current Session age/SSO/MFA before any token/member/source write. |
| OIDC/SAML JIT | Workspace JIT AND provider JIT AND active/approved provider AND verified email/domain for new/reactivated access. |
| SCIM User create/reactivate | Same current domain/external admission policy; protocol-compliant SCIM denial. Human Session assurance is not applied to Bearer provisioning. |
| Administrative restoration | Same domain/external policy for reactivated human access. Explicit admin authority remains necessary. |

Policy tightening and domain revocation serialize with every entry path.
Rejected invitation acceptance retains the pending token. Rejected JIT/SCIM
admission rolls back Global User, member, source, resource, audit, event and
outbox writes. Existing source attribution is retained. Administrative
suspension/removal always blocks automated JIT/SCIM restoration.

## API, RBAC and recovery

All tenant roles have `workspace_security.read`; only Owner has
`workspace_security.update`. All tenant policy endpoints remain protected by
the current human security evaluator, including read and preview.

| Route, relative to `/api/v1` | Contract |
| --- | --- |
| `GET /workspaces/:id/security-policy` | Policy, revision, safe description, exact verified domains, external-member count and current-session decision. |
| `POST /workspaces/:id/security-policy/preview` | Read-only candidate decision and safe prerequisite reason. Does not mutate policy or revision. |
| `PATCH /workspaces/:id/security-policy` | Owner, recent strong authentication, expected revision and transactional self-lockout/provider prerequisites. |

PATCH requires positive `expected_revision`, retains omitted values, and
accepts explicit NULL to clear age/grace. A stale revision returns 409
`WORKSPACE_SECURITY_POLICY_CONFLICT` without partial writes. Selected providers
must belong to the same Workspace; enforced SSO requires a usable approved
provider, verified domain and Owner identity binding. Provider disable/update
cannot remove the last usable provider while SSO remains required.

Policy edits, including weakening changes, reuse centralized recent strong
authentication. Recognized recent primary methods are password, OIDC, SAML and
passkey; an enrolled user must also have verified MFA or a trusted existing
step-up grant. `RecentAuthenticationProof` carries verified time and actual MFA
strength. A timestamp alone never proves MFA. Policy mutation and break-glass
recheck enrollment under the actor lock; a stale pre-enrollment hint cannot
weaken this requirement. Recent operation proof and durable Session MFA are
distinct. Original authentication age still controls Workspace access after
step-up.

Recent primary authentication is checked within ten minutes. The existing
session-bound TOTP sensitive-operation grant retains its fifteen-minute TTL;
an accepted grant supplies a fresh request-local proof for that operation.
This does not refresh either original Session clock or satisfy required
Workspace Session MFA without the exact-family token upgrade.

Owner break-glass uses the existing Global recovery endpoint, fresh password
and TOTP when enrolled, explicit confirmation, a 10–500-character reason, and
the existing once-per-Workspace 15-minute limit. It relaxes SSO, MFA and general
age only, retaining member admission restrictions and provider approvals. It
does not grant membership or RBAC and adds no weak lost-factor bypass. TOTP
factor loss without recognized strong recovery evidence requires operator
recovery; Phase D does not invent an account takeover mechanism.

TOTP disable checks all active Organization memberships transactionally and
rejects `WORKSPACE_MFA_REQUIRED` when any active Workspace requires the only
recognized factor. SQL failure fails closed. Existing Global Admin factor
requirements still apply. Setup, secret/decrypted prefixes, QR material and
temporary login token prefixes are excluded from logs and tested.

Denied human requests carry only safe string metadata: `workspace_id`,
`requires_sso`, `requires_mfa`, `policy_revision`. Policy unavailable returns
503 `SECURITY_POLICY_UNAVAILABLE`. The frontend retains the valid Global
Session, scopes stale responses to their tenant/navigation/session, offers
enrollment and a separate TOTP proof, or approved SSO/reauthentication. Explicit
password/TOTP/passkey reauthentication failure preserves the current Session.

## Transactions, audit and compatibility

Policy, admission and provider writes serialize on the existing Workspace row;
policy updates also lock the actor with `FOR NO KEY UPDATE`, lock policy and
recheck live factor state. This actor lock excludes factor/lifecycle writes and
permits the `KEY SHARE` locks used by Workspace-first audit foreign keys.
Expected revision prevents lost updates. Live SQL checks avoid stale policy
allow caches; no Gateway auth-cache dependency was added.

Every successful policy mutation commits redacted before/after audit metadata,
`workspace.security_policy.updated`, and transactional outbox together. Existing
SSO enabled/disabled and break-glass events remain. Owner/Admin receive security
notifications; existing Workspace webhook delivery recognizes the new event.
Event payloads use bounded allowlisted scalars, never credentials or claims.

Migration 297 preserves existing SSO/grace/revision/updater values, members,
typed membership sources, users, keys, Service Accounts, usage, budgets and
immutable billing attribution. Approved-provider foreign keys include tenant
identity. The migration runner remains authoritative; raw rerun guards are
also tested. No full-table historical usage rewrite occurs.

Existing refresh-only revocation semantics remain: deleting a refresh family
or revoking refresh tokens does not immediately invalidate an already-signed
access JWT absent a password-derived version change. Current Workspace policy
and provider changes are nevertheless evaluated live on human requests. A full
access-token denylist/session redesign remains outside Phase D.

## Security gate answers

| Gate | Answer and evidence category |
| --- | --- |
| Q1 Enrollment without Session MFA can access required-MFA Workspace? | NO — evaluator, middleware and every-tenant-route tests. |
| Q2 Refresh advances original authentication time? | NO — refresh and upgrade metadata tests. |
| Q3 Workspace A MFA affects Personal Workspace? | NO — Personal/machine evaluator regressions. |
| Q4 Workspace A SSO satisfies Workspace B? | NO — tenant/provider/revision tests. |
| Q5 SSO and MFA require only one condition? | NO — AND evaluator tests. |
| Q6 Unapproved provider satisfies required SSO? | NO — provider selection/live SQL tests. |
| Q7 Pending external invitation survives policy tightening? | NO — latest-policy acceptance and PostgreSQL race tests. |
| Q8 Admission denial consumes invitation token? | NO — transactional rollback tests. |
| Q9 Provider JIT bypasses Workspace JIT=false? | NO — shared admission and OIDC/SAML tests. |
| Q10 SCIM Bearer is blocked by human SSO/MFA? | NO — provisioning separation integration test. |
| Q11 Human SCIM management is protected? | YES — every-tenant-route assurance test. |
| Q12 Direct API Key runtime is affected? | NO — machine/key isolation regressions. |
| Q13 Service Account runtime is affected? | NO — existing machine admission plus human policy separation. |
| Q14 TOTP secret prefixes remain in logs? | NO — captured DEBUG leakage regression. |
| Q15 SCIM/JIT can revive administrative suspension/removal? | NO — source reconciliation and admission tests. |
| Q16 Stale policy cache bypasses new MFA? | NO — authoritative SQL policy on each human gate; no policy cache. |

## Performance and deferred scope

Normal tenant requests retain the existing Workspace access read and add
canonical policy/provider validation as needed. Canonical policy loading uses
one joined query including approved IDs. Policy descriptions also read verified
domains and external-member counts. Admission reads current member/domain and
provider metadata under the existing Workspace lock. Gateway execution gains
no new human-policy query. No production latency/load benchmark was run; test
durations do not establish production overhead. Migration DDL locking and hot
Workspace write contention need production rollout measurement.

Deferred: IdP MFA claim mappings, passkey MFA recognition, arbitrary ABAC,
device/risk/geo/IP trust, IdP-initiated SAML, SLO, SCIM Bulk, cost centers,
anomalies, retention/export/deletion lifecycle, and unified UI redesign.
