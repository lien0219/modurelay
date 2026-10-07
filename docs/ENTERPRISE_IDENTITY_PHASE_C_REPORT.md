# Phase C Enterprise Identity Advanced report

This is the final implementation and verification record for SAML Enterprise SSO
and SCIM Enterprise Provisioning. Phase L retains real-provider, rendered-browser,
operator, load and deployment acceptance.

## Branch

`feature/new-feature`.

## Baseline

`81e68865463755f3fb684199b3de5a31e8572713`.

## C1 Commit

`41b0b31dfd59ca2ddc337b78d2c1bc609bba0a77`,
`feat(saml): add enterprise SAML single sign-on`.

## C2 Commit

The local `feat(scim): add enterprise identity provisioning` boundary.
Resolve its immutable SHA with
`git log -1 --format='%H' --grep='^feat(scim): add enterprise identity provisioning$'`.
A commit cannot embed its own hash; the delivery response records the resolved SHA.

## Working Tree

The delivery controller checks the clean tracked/untracked working tree after
the local C2 boundary and verifies exactly two commits since the baseline with
`git status --porcelain=v1 --untracked-files=all` and
`git rev-list --count 81e68865463755f3fb684199b3de5a31e8572713..HEAD`.
The delivery response records that post-commit result and the actual C2 SHA.
Ignored local evidence remains in `.cache/phase-c/`.
No push, PR, branch switch, deployment or acceptance-runtime mutation occurs.
Port 18081 is untouched.

## Architecture Findings

Phase B already supplies provider-scoped identity bindings, domain proofs,
JIT/linking, role and Team mappings, tenant RBAC, audit/outbox and browser
assurance. Its OIDC-required columns and OIDC-only assurance were the SAML
extension points. Single-valued materialized Membership/Team source fields were
insufficient for simultaneous manual, OIDC, SAML and SCIM management.

The phase reuses the provider parent and stable binding, uses protocol-dependent
SQL integrity, generalizes the shared assurance engine and introduces typed
source attribution. SCIM has its own connector and credential model. Neither
protocol introduces a Project ACL or changes global identity, machine execution,
budget, billing or historical attribution. Detailed pre-implementation decisions
are in ENTERPRISE_SAML.md, ENTERPRISE_SCIM.md and the phase plan.

| Audited area | Final decision / behavior |
| --- | --- |
| 1 Provider polymorphism | One parent; genuine nullable protocol fields and protocol SQL CHECKs |
| 2 SSO assurance | One OIDC/SAML tenant/provider/revision/time enforcement engine |
| 3 Identity binding | Reuse provider+stable-subject binding; no parallel SAML identity table |
| 4 Role mapping | Shared engine; manual role and exact active managed role source retained |
| 5 Team mapping | Shared SSO mappings; SCIM explicit existing-Team binding |
| 6 Membership attribution | Typed manual/OIDC/SAML/SCIM sources and administrative overrides |
| 7 Team attribution | Typed exact provider/connector/Group sources with tenant FKs |
| 8 SSO routes | Shared start/completion, opaque SAML metadata and bounded POST ACS |
| 9 SSO completion | Same-origin one-use exchange, TOTP and existing session issuance |
| 10 Secret encryption | Persistent SecretEncryptor for per-provider SP private keys |
| 11 SSRF | Shared pinned HTTPS transport; blocked networks, no proxy/redirect, bounds |
| 12 RBAC | Central Workspace permissions and locked rechecks; human recent auth |
| 13 Audit/Event/Outbox | Transactional safe summaries, secret/body omission and external NULL actor |
| 14 Team/Project ACL | Existing Team-to-Project Access Grant engine; no alternate SCIM ACL |
| 15 Owner/Billing safety | Manual management; automated Owner and billing deprovision409 |
| 16 JWT/session assurance | Original auth time/type/revision retained; SCIM creates no session |
| 17 Dependencies | Maintained gosaml2/xmldsig crypto; SCIM uses existing dependencies |

## Architecture Gate Decisions

| Architecture gate | Answer |
| --- | --- |
| Q1 SAML reuses provider parent | YES |
| Q2 OIDC-specific columns | Nullable with protocol-dependent database integrity; no dummy values |
| Q3 Phase B data migration | No provider-table move; control-plane schema/backfill only |
| Q4 SAML reuses identity binding | YES, provider plus stable subject |
| Q5 Transient NameID | Rejected as permanent identity; persistent/configured stable subject required |
| Q6 SAML enterprise email proof | Trusted verified signature + configured email attribute + verified Workspace domain |
| Q7 Shared provisioning/mappings/linking/assurance | YES |
| Q8 OIDC-specific type duplication | Protocol-neutral shared aliases/pipeline; stable OIDC implementation retained |
| Q9 SCIM is login provider | NO; independent connector |
| Q10 Token storage | Secure high entropy, SHA256 only, plaintext once |
| Q11 Rotation | Multiple active tokens, up to8, explicit revoke |
| Q12 Tenant identity | Credential-derived connector/Workspace; body cannot choose tenant |
| Q13 Membership source removal | Exact typed source only; other sources and admin authority survive |
| Q14 Team source removal | Exact provider/connector/Group only |
| Q15 SCIM deletes Global User | NO |
| Q16 SCIM always suspends member | NO; only no-live-source state, subject to admin override |
| Q17 SCIM deactivates Owner | NO,409 |
| Q18 SCIM deactivates Billing Owner | NO,409 until manual billing transfer |
| Q19 Group Team semantics | Explicit same-Workspace existing active-Team binding |
| Q20 Require SSO | One shared enforcement engine; independent Bearer provisioning route |

## Provider Polymorphism

OIDC and SAML share `workspace_identity_providers` and
`workspace_user_identities`. OIDC-specific columns are nullable with strict
protocol CHECKs; SAML has genuine IdP configuration and independent encrypted SP
keys. Provider type is immutable. No fake issuer/client ID is written, and Phase B
OIDC control rows do not need a separate config-table migration.

## SAML Architecture

SP-initiated signed HTTP-Redirect requests and HTTP-POST ACS are implemented.

## SAML Metadata

Paste/import XML and HTTPS metadata URL use bounded parsing and the existing
DNS-pinned, no-proxy, no-redirect transport. DTD/XXE, excessive nesting, endpoints,
attributes and certificates are rejected. Public metadata uses opaque provider
IDs and the configured HTTPS origin.

## SAML Crypto / Certificates

gosaml2 v0.12.0 and goxmldsig v1.6.1 perform signature verification,
canonicalization and encrypted-assertion decryption. Multiple IdP signing
certificates support rollover. Per-provider RSA3072 SP private keys use
SecretEncryptor, with current/staged/retained-previous decryption keys and a
15-minute previous-key overlap. Metadata never exposes private material.

## SAML Login Flow

Opaque hashed RelayState freezes tenant/provider/revision/request ID/return path
and browser/link intent, with expiry and single use. Issuer, audience,
Destination, Recipient, InResponseTo, status, subject confirmation, timing,
signature and replay checks precede provisioning. Persistent NameID or an
explicit stable subject is required; transient/email-only subjects do not become
permanent bindings.

## SAML JIT / Linking

Validated signed claims, configured email attribute and verified Workspace domain
provide enterprise email proof for new JIT accounts. Existing-email SSO requires
recent authenticated explicit linking and local TOTP when applicable. Original
browser authentication time survives the same-origin completion exchange and refresh.

## SAML Team / Role Mapping

Role/Team mapping reuses the shared provisioning engine; manual role, Owner and
Billing Owner remain authoritative. Typed attribution retains the exact provider
source when other providers or connectors also manage access.

## Enterprise SSO Assurance

One Require SSO engine accepts matching active OIDC or SAML assurance, including
Workspace, provider, protocol, revision and original authentication time. Revision
or disable invalidates that Workspace proof without deleting global sessions.
Owner recovery remains shared. Human SCIM management observes Require SSO;
external SCIM Bearer traffic uses its separate connector authentication.

## SCIM Architecture

`/scim/v2/:connector_public_id` implements Users, Groups, ServiceProviderConfig,
ResourceTypes and Schemas. Tenant comes exclusively from the matching active
connector/token. Connectors belong only to active organization Workspaces and
use explicit Group-to-Team binding and a non-Owner default role.

## SCIM Connector / Tokens

Tokens contain 32 secure random bytes, have `mrc_scim_` prefixes and are SHA256
only at rest. Plaintext is returned once, under no-store, and never retrievable.
Up to eight active tokens coexist for create-new/reconfigure/revoke-old rotation.
Revoke, disable, revision and wall-clock expiry are rechecked after acquiring the
Workspace transaction lock. No SCIM operation creates a human session.

## SCIM Users

Core User and Group attributes, replacement PUT, idempotent add/replace/remove
PATCH, safe repeated DELETE, opaque resource IDs and ETag/If-Match are supplied.
Filters are bounded allowlisted equality predicates with parameterized SQL;
pagination is 1-based, count at most 100 and offset at most 100000. Responses use
SCIM media/error schemas and configured-origin Location.

Existing Global User resolution needs exact normalized verified inbox evidence
and a verified Workspace domain. Unverified identities, aliases and external
domains are rejected. New users get unknowable bcrypt credentials. Existing
global profile/password and the bound primary inbox are preserved.

## SCIM Groups

Groups map only through an explicit existing active same-Workspace Team.
Display-name equality never takes over a manual Team. Group deletion/removal
deactivates only exact Group sources, retaining the Team and its Project grants.
User active=false retains Group assignments for restoration; DELETE removes
connector-local references and versions affected Groups. Deleting ordinary A
does not reconcile unrelated Owner/Billing Owner B in shared Groups; B's
identity, sources, Teams and billing state remain byte-for-byte unchanged.

## Multi-Source Membership Model

Typed Membership and Team source tables have composite tenant FKs and exclusive
manual/OIDC/SAML/SCIM reference shapes. Any live source can retain membership,
subject to administrative suspension/removal. Manual role wins; otherwise retain
the exact selected managed source before a stable earliest-source fallback.
Billing/developer roles are not ranked numerically.

Complete-empty SSO groups remove only that provider's grants. Missing/incomplete
claims preserve prior attribution. SCIM deprovision affects its own connector
source only. Administrative suspension/removal cannot be undone by provisioning.

## Owner / Billing Safety

Owner requests and Billing Owner deprovision return 409. Global User, personal/
other Workspaces, keys, Service Accounts, sessions and billing ownership survive.

## RBAC

Central Owner/Admin provisioning read/manage/token.rotate permissions are
rechecked under write locks. Recent human auth protects mutations.

## Audit / Events / Notification / Webhook

External sync has a NULL human actor. Audit/event/outbox payloads contain bounded scalar
summaries; raw credentials, assertions, metadata and SCIM PII are omitted.

Ordinary successes retain audit/Webhooks without inbox fanout. Disable, protected
identity conflicts, repeated failure and token-expiry alerts target Owner/Admin.
Failure threshold is three with 15-minute debounce; dormant expiry scans are
bounded to 100 and debounce once per day. Authenticated rejected writes also
record one safe outcome; reads and unauthenticated traffic remain silent.

## Database

Tenant integrity is enforced by the database, including protocol-dependent
provider checks, provider-type constraints and exact resource/member/source
composite foreign keys. Global identity remains separate from Workspace access.

## Migrations

Migration 295 extends genuine protocol provider configuration, encrypted keys,
auth-state/request correlation, replay protection and SAML attribution.
Migration 296 introduces connector/token/resource and typed source tables,
administrative flags and exact selected-source pointers. Tenant integrity is
enforced by the database, including provider-type and exact resource/member FKs.

Backfill is limited to control-plane Membership/provider/Team rows and is
idempotent. Historical suspended rows conservatively gain administrative
suspension. Unbound legacy SCIM is preserved as manual when connector evidence
does not exist. Rerun preserves newer source/admin state. Migrations 294/295 are
not rewritten by C2; Usage/Billing/Key/Service Account history is not rewritten.

## Frontend

Workspace Identity retains OIDC/SAML controls and adds an independently
permission-gated SCIM section: connector/default role/disable, server Base URL,
show-once token/rotation/revoke, explicit Group binding, safe sync summary and
Entra/Okta setup help. English/Chinese copy and notification keys are complete.
All async mutations/finalizers bind to Workspace/connector/tab/permission
generation. Secrets clear on dismissal, context change, invalidation and unmount.
Frosted tokens, existing routes/selectors and shared dialog/auth handling remain.

## Backward Compatibility

Personal Workspaces, Direct Keys, Service Accounts, ExecutionPrincipal, policy,
budget, billing, Seedance, Grok and Canvas keep existing semantics. Only
`/scim/v2/` is added to the embedded frontend bypass so protocol requests reach
the backend. Identity protocols are outside the Gateway hot path.

## Security Review

C1 independent review approved the immutable v4 patch after optional-attribute
serialization, HTTP body omission, stale rotation state and NameID policy fixes.
C2 backend task review identified expiry-after-lock, deleted Group references and
early-outcome gaps. F1/F3 and five lint issues are independently approved.
Deleted-reference repair was narrowed after a B1 regression: deleting ordinary
A must not traverse unrelated Group Owner B. Actual HTTP409 RED was reproduced
twice before the exact-member fix; real PostgreSQL GREEN and final expanded
identity/SAML/SCIM race pass. B1 is independently Spec APPROVE / Code Quality
APPROVED. Whole-C2 review then found R1: repeated User email additions appended
duplicates, changed resource/audit versions and eventually exceeded the limit.
The bounded correction deduplicates complete supported email values using
normalized mailbox values and exact decoded attributes, preserving order,
distinct entries, limits, ownership and historical stored duplicates.
Protocol and real HTTP/PostgreSQL RED preceded correction;350 successful retry
requests now preserve resource/ETag/lastModified/mutation audit/events/outbox.
Independent R1 Spec/CodeQuality APPROVE closes the only whole-review blocker.
The final56 source hashes match the approved v2 manifest; no Critical/Important
source finding remains. Formal Codex Security plugin execution is NOT RUN;
independent source/security reviews are the recorded review method.

## Concurrency Review

Workspace locks serialize manual/SSO/SCIM resources and sources, token/connector
mutations and Group PATCH. Existing email advisory locks prevent duplicate global
accounts. Real PostgreSQL covers eight simultaneous POSTs, source coexistence,
state races, Group updates, token rotation/revoke and migration integrity.

## Dependency Review

SAML dependencies are Apache-2.0 and maintained 2026 releases, compatible with
the repository's Go 1.27. Exact-version advisory checks were recorded for C1.
SCIM adds no dependency. Current govulncheck stream contains 11 module-only
advisories, zero imported-package and zero reachable-function findings; the
production pnpm exception checker passed. These are dated audit results.

## Tests

| Gate | Result | Evidence |
| --- | --- | --- |
| C1 protocol/security/PostgreSQL/race | PASS | Signed/encrypted local IdP, malicious XML/SSRF/replay/state/linking fixtures; final realPG43.498s and race |
| C1 frontend | PASS | Reviewed422files3112tests, i18n/lint/typecheck, build26.71s |
| C2 protocol/HTTP/RBAC/audit/events | PASS | Final post-R1 relevant service6.215s, handler0.081s, repo0.053s, middleware0.044s, migrations0.009s |
| C2 real PostgreSQL/migration/governance | PASS | Final post-R1 identity/SAML/SCIM/Workspace/governance43.515s |
| C2 race | PASS | Final service9.370s/handler1.290s/repo1.116s/middleware1.084s; all SCIM/OIDC identity/SAML realPG race51.510s |
| R1 email retry regression | PASS | Protocol/actual HTTP-PostgreSQL RED/GREEN;350 successful retries, unchanged versions/audits/events, real additions and strict rejection/protection |
| C2 frontend | PASS | 424files3149tests, full i18n/lint/typecheck, production build24.69s |
| Canvas/embed | PASS | Canvas critical7tests, rebuilt Bun bundle16.62s, full embedded web0.077s |
| Go format/vet/build/lint | PASS | gofmt scan clean, vet/build, pinnedgolangci-lint2.13.0 with0issues |
| Current dependencies | PASS | govulncheck11module-only advisories,0imported/0reachable; pnpm production exception checker validated |
| Full default/unit/integration final runs | PRE-EXISTING | Post-R1 default/unit and repeated full integration match actual archived81e688 failure tests, packages and missing-sh/CAS characteristics; no build errors/timeouts |
| Initial post-R1 external TLS test | FAIL on initial run; resolved on matched full reruns | TestAllProfiles received tls.peet.ws TCP reset/refusal; original log/mismatch retained; unchanged TLS sources, final baseline/current integration have no failing TLS test; external fingerprint success is not claimed |
| Supplemental integration-tag lint | PRE-EXISTING | Actual identical archived81e688/current commands return13 identical path/line/diagnostic findings in10 untouched files; required default full lint0issues |
| C2 independent source review | PASS | Whole-C2 review plus R1 Spec/CodeQuality APPROVE; F1-F3/B1 backend correction and all11frontend files independently approved |
| Real providers/browser/operator/load/deployment | NOT RUN | Phase L |

Full backend suites returned nonzero. Default/integration reproduce exactly
three Windows PgDumper missing-sh failures; unit also reproduces the baseline
Ollama stale-long-callback CAS assertion. No strict assertion was relaxed. Raw
commands, baseline matching and the complete test matrix are in
[ENTERPRISE_SAML_ACCEPTANCE.md](ENTERPRISE_SAML_ACCEPTANCE.md) and
[ENTERPRISE_SCIM_ACCEPTANCE.md](ENTERPRISE_SCIM_ACCEPTANCE.md). The SCIM
acceptance document separately answers every required security Q1-Q16.

Final comparison is `c2-post-r1-delivery-baseline-comparison.json`.
Initial post-R1 integration additionally failed `TestAllProfiles` on external
TCP reset/refusal, so the original `c2-after-r1-baseline-comparison.json` correctly
retains `identical=false` for that attempt. Identical complete integration
commands were rerun sequentially on archived baseline and final source, taking
167.885s and171.047s; both retain only the same three PgDumper failures. The TLS
package sources were checked against baseline Git blobs and archive. No assertion,
timeout or skip behavior was changed. Existing external-network skips mean this
does not establish live TLS fingerprint validation. This transient network event
is disclosed separately rather than called a proven PRE-EXISTING application bug.

## External Provider Validation

Entra/Okta/Google SAML and Entra/Okta SCIM: NOT RUN, Phase L. Rendered browser,
light/dark/mobile/keyboard, operator, production load and deployment: NOT RUN.
Local signed/encrypted IdP and SCIM fixtures provide automated protocol coverage.

## Risks

Retain encryption keys and exact HTTPS configuration. Inspect conservatively
backfilled suspension/legacy rows. Workspace serialization and payload limits
require bounded client retries/batches. Disable retains sourced access; explicitly
deprovision first when access removal is intended. Vendor mapping differences
remain an interoperability risk until Phase L.

SCIM preprovisioning preserves the existing explicit SSO-linking boundary.
Employees without a prior usable credential/binding may need mailbox ownership
recovery to authenticate and link initially; provider interoperability and that
operator onboarding flow remain Phase L acceptance items.

## Deferred

IdP-initiated SSO, SLO, POST-only AuthnRequest, SCIM bulk/sort/password/schema
extensions, full UI redesign and Phase D/E/F implementation remain deferred.

## Roadmap

Phase C, C1 and C2 are **COMPLETE** at the two verified local phase boundaries.
Phase A/B remain COMPLETE; Phase D is **NEXT**. No Phase D/E/F functionality is
implemented. Real-provider and production/manual release gates remain Phase L.
