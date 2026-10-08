# Phase D architecture findings and gate (2026-10-08)

Baseline: `feature/new-feature`, `6981e17de52d87ba4723b28452ad8e455e0317b5`, initially clean. No push, PR or acceptance deployment is authorized. The complete Phase D user specification is the binding scope; implementation continues after this gate without another approval pause.

## Current Security Policy Architecture

Migration 294 owns `workspace_security_policies`, initially SSO/grace/revision/updater only. `EnterpriseIdentityService` and its SQL repository already own the read/update API. Extend this table and use one canonical `WorkspaceSecurityPolicy`; keep `WorkspaceIdentityPolicy` as a source-compatible alias. Do not add independent MFA/session/invitation settings tables.

## Current Authentication Assurance

`SessionAuthentication` carries original authentication method/time and `MFASatisfied`. `WorkspaceAssurance` carries tenant/provider/revision/authentication/assertion expiry. Signed JWT and refresh-family metadata already preserve authentication time and MFA. Historical OIDC wire fields also carry SAML. Retain these wire names and document their protocol-independent meaning; do not invalidate old tokens by renaming claims. Preserve assertion `ValidUntil` end to end.

The JWT middleware currently seeds session context from enterprise assurance and loses local authentication time/MFA. Fix context propagation before enforcement. Enrollment status may select an actionable error; it can never establish authentication assurance.

## Current MFA Semantics

Password+verified TOTP and enterprise completion with verified local TOTP produce MFA evidence. Enrollment alone, password alone, OAuth alone and OIDC/SAML alone do not. Workspace MFA is satisfied only by actual evidence for the current signed Session. A TOTP step-up currently creates only a short Redis sensitive-operation grant, so an additive Session upgrade must rotate the exact authenticated refresh family and preserve all original times and tenant assurance. A token from another family cannot supply that proof.

## Current TOTP Semantics

TOTP setup/enable/disable are Global User security routes. Debug logs include secret/decrypted prefixes; remove them and add a leakage regression. Disabling must atomically lock the user, check active organization requirements and update the factor. A precheck outside the write transaction is insufficient.

## Current Passkey Semantics

WebAuthn config and login ceremony require user verification, but the existing handler returns only User and does not propagate assertion flags into session evidence. Gate decision: Phase D recognizes verified local TOTP as Workspace MFA; passkey login retains conservative `MFASatisfied=false` until assertion-level evidence is explicitly propagated and tested. Passkey is not inferred to be MFA from branding or enrollment. IdP AMR/ACR/AuthnContext mapping is likewise not recognized in this phase.

## Current Session Lifetime

Enterprise assurance has a legacy 12-hour ceiling. Keep that ceiling in one named validity function and allow Workspace session age to further constrain access since the original Global Session authentication. `NULL` adds no general Workspace age limit. Positive values are 900 through 2592000 seconds. Assertion expiry, provider revision and the 12-hour enterprise ceiling remain additional AND conditions. Refresh and TOTP step-up never restart either authentication clock.

## Current Recent-Auth / Step-up

Identity/provider and SCIM management use the existing session-bound TOTP grant/recent authentication guard. Reuse one guard for security changes and carry a typed `RecentAuthenticationProof{VerifiedAt, MFASatisfied}` in service context. A time-only proof is primary authentication and cannot satisfy an enrolled factor. Policy updates and break-glass recheck enrollment under the actor lock and require actual Session MFA or separately verified recent MFA. This proof never changes durable Session MFA or either original authentication clock. Strong recent authentication and Owner permission protect all policy edits, including weakening settings.

## Current Workspace Access Enforcement

Workspace, Service Account, gateway-policy management and legacy Organization key management already call the enterprise SSO gate. Replace its internal decision with one Workspace Security Evaluator, retaining Personal and machine-runtime separation. Cover every tenant handler and legacy key list/detail/mutation path, and carry a complete session snapshot to repository reads. Global profile, MFA enrollment, logout and recovery routes remain usable.

## Current Invitation Flow

Create checks permission/role/email and generates a hash-only token. Accept checks token/email/inviter permission/state under the Workspace lock, but neither checks security policy. Both must use current transactional member admission. Acceptance additionally evaluates Session age, approved SSO and actual MFA before membership/source/token mutation. Denial rolls back and preserves pending token. Disabled invitations block creation and acceptance; restrictive domains are exact verified Workspace domains.

## Current JIT Flow

OIDC and SAML share `CompleteIdentityLogin`. Existing verified-domain/JIT checks focus on new Global User creation; inactive membership restoration and an existing Global User need equivalent admission checks. Workspace JIT AND provider JIT AND active/approved provider AND existing verified-email/domain requirements apply to new or reactivated membership. Already-active members are retained when the policy tightens; admin suspension/removal always blocks automated restoration.

## Current SCIM Provisioning Flow

Connector Bearer requests already derive tenant and recheck token/connector under Workspace locks. New SCIM resources and inactive-to-active restoration need the same member admission rules as invitations/JIT. SCIM errors use SCIM conflict/invalid-value envelopes. SCIM Bearer is never subject to human SSO/MFA/session age. Existing external SCIM resources remain; relevant new/changed admission uses current policy.

## Current Domain Model

Reuse exact canonical IDNA `workspace_domains.normalized_domain`, `status='verified'`. All verified domains constitute enterprise domains in Phase D; no arbitrary text allowlist or optional selected-domain subsystem is needed. `allow_external_members=false` blocks new admission outside that set for every source. Revocation takes effect on the next transaction without suspending existing members. Zero verified domains is a valid fail-closed policy with a visible warning.

## Current Provider Approval Model

Providers are tenant-owned OIDC/SAML records with active status and revision. Add `workspace_security_approved_providers` with composite tenant/provider FK. Modes are `any_active` and `selected`. Selected approval constrains required SSO and new JIT; historical bindings remain. Disabling/updating the last usable approved provider under enforced SSO must conflict under the same Workspace lock.

## Current Break-glass

Existing Owner-only recovery requires recent strong global authentication, a bounded reason, rate limits and transactional audit/event/outbox. Extend recovery to a bounded relaxation of authentication requirements (SSO/MFA/general age) only; preserve member/provisioning restrictions and provider relations. If the existing account cannot supply strong recovery evidence, fail closed. No new weak factor-loss bypass is introduced.

## Current Cache / Session Invalidation

Policy is authoritative in SQL on each human control-plane request; use no stale-allow policy cache. Policy updates and admission share Workspace row serialization, provider revisions are checked live, and expected revision prevents lost updates. JWT refresh preserves metadata; TokenVersion/session revocation retains existing semantics. Human policy never enters gateway auth cache or scheduling.

## Risks and tenant isolation

- Session metadata loss, enrollment-as-MFA, token-family mixing and refresh/step-up clock extension are security blockers.
- Three independently implemented domain gates permit bypass. A single repository transaction helper calls a single pure admission policy for invitations, OIDC/SAML JIT, SCIM and explicit administrative restoration.
- Caller-provided `existing_active` is not trusted; the helper derives member state and administrative flags from SQL. Tightening does not rewrite existing membership sources.
- Concurrent same-actor factor disable, provider disable and admission must serialize. Policy mutation follows the lifecycle order User (`FOR NO KEY UPDATE`), Workspace, then Policy; factor disable locks User and reads Workspace requirements without acquiring Workspace locks. Workspace-first audited writes may take User `KEY SHARE` through foreign keys. `NO KEY UPDATE` remains exclusive against factor/lifecycle modification while allowing those FK locks, avoiding the PostgreSQL deadlock demonstrated by an actual audited-mutation regression. Different-member overlapping disable/enable can linearize as disable-before-enable; existing unenrolled members are supported and still cannot access a required-MFA Workspace without real Session proof.
- API/relations/errors never authorize a provider or user from another Workspace. Mutation audit/events/outbox are atomic and strictly redacted. No denial audit storm or tenant/user metric labels.

## Phase D Architecture Gate

Proceed with one canonical policy, one human evaluator and one member admission policy. Choose conservative local TOTP assurance and backward-compatible enterprise wire fields. Defaults remain MFA=false, age=NULL, invitations=any, external=true, JIT=true, providers=any_active. All constraints compose with AND; policies cannot expand provider permissions or revive admin-blocked identities. Central RBAC exposes read to all member roles and update to Owner only. PATCH preserves omitted fields, explicitly accepts null age/grace, and requires `expected_revision` at the HTTP boundary.

## Migration Plan

Add migration `297_workspace_security_policy.sql`; do not alter 294–296. Add policy columns/defaults/bounded enum and age checks, tenant-scoped provider FK relation and additive safe event fields. Retain existing SSO/grace/revisions and every existing membership/source. Do not rewrite usage, billing, API keys, Service Accounts, async tasks or immutable attribution. Test complete migration history, defaults, cross-tenant FK rejection and preserved hot-data snapshots using real PostgreSQL; compare failures with exact baseline 6981e17.

## Post-implementation review decisions

The policy writer accepts an unchanged elapsed SSO grace deadline, whether omitted
or explicitly submitted unchanged. Only a newly changed deadline must be in the
future and within seven days; live SSO enforcement continues after expiry. The
functional editor preserves the exact original timestamp when its displayed
value is unchanged, preventing unrelated edits from truncating grace precision.

The formal security scan retains its original working-tree snapshot identity.
Review-driven lock, typed-proof and grace fixes are separate remediation deltas
with fresh source hashes and focused verification. They do not silently replace
the initial scan digest. Browser, real-provider, deployment and load acceptance
remain explicit follow-up gates.
