# Enterprise SSO — Phase B

Enterprise SSO authenticates humans entering an Organization Workspace. Users
remain global identities with Personal and other Workspace memberships. SAML
and SCIM are deferred to Phase C. IdP access is absent from the Gateway API-key
and Service Account credential request path.

## Architecture findings and gate

Existing authentication uses JWT access tokens, atomic Redis refresh rotation,
token families, password fingerprints, optional session binding, password,
TOTP/step-up and separate social OAuth/OIDC. Workspace RBAC checks live tenant
membership; Phase A direct/Team grants control project visibility. Global
administrator is not a Workspace role. Audit, immutable events, transactional
outbox, notifications and Workspace webhooks already exist. Secrets use the
existing AES-GCM encryptor; OIDC reuses public-address/DNS-pinning protections.

The architecture gate is **APPROVED for OIDC foundation only**:

| Question | Decision |
| --- | --- |
| Identity scope/stable key | Workspace + provider + global User + subject; `(workspace_id,provider_id,subject)`, never email. |
| Existing same email | Authenticated explicit link; no automatic adoption. |
| JIT/verified domain | Active organization/provider, enabled JIT, verified exact domain, true verified email, no existing global email/alias identity. |
| Unverified email | No JIT/new link. Existing stable binding may authenticate without changing global email. |
| Domain uniqueness | One non-revoked claim globally, including pending/failed claims; no disclosure of the other Workspace. |
| Team mapping | Provider-attributed grants materialize Phase A Team membership; reconcile only current provider. |
| Owner/Billing Owner | Automatic Owner forbidden; Billing Owner unchanged; manual roles preserved. |
| Role conflicts | Highest explicit priority; ties: admin, billing, developer, viewer. |
| Missing/overage groups | Preserve mapped roles/Teams. Explicit empty groups reconcile. Manual/SCIM/other-provider attribution survives. |
| Session scope | Requested Organization only; A assurance cannot satisfy B; Personal/global login remains usable. |
| Session metadata | Original method/time/MFA plus Workspace, provider/revision, OIDC time. Refresh never advances authentication time. |
| Assurance expiry/disable | Twelve-hour limit; disabled provider/config revision invalidates management assurance. |
| Recovery | Dedicated Owner endpoint; reverified password, account TOTP, confirmation/reason, strict rate limits, atomic audit/event. |
| SSRF/HTTP redirects | Public HTTPS endpoints, DNS validation and IP pinning at each connection, no proxy, no redirects. |
| JWKS | Bounded cache, coalesced unknown-key refresh, 30-second backoff, no expired-key fallback. |
| Multiple providers | Supported; one active default. Sole enforced provider cannot be disabled. |
| Existing membership/migration | Old attribution defaults to manual; no historical usage/billing/API-key rewrite. |

Migration 294 adds domain, provider, binding, mapping, state, completion,
security-policy and recovery-limit tables plus source/provider attribution.
Workspace locking, DB unique/composite constraints and transactional audit/
outbox protect configuration, JIT, subject binding, Team targets and recovery.

## Configuration and secret storage

Set `ENTERPRISE_SSO_REDIRECT_URL` to the application's public callback:

```text
https://relay.example.com/api/v1/auth/sso/callback
```

This is independent of global `OIDC_CONNECT_REDIRECT_URL`; request Host never
supplies a fallback. The frontend and API share this origin. HTTP is permitted
only for the local application's loopback callback; all IdP endpoints remain
public HTTPS. Set and retain `TOTP_ENCRYPTION_KEY`, a 32-byte hex AES key. An
automatically generated ephemeral key cannot enable enterprise SSO. Back up
the retained key separately; replacing it makes existing ciphertext unreadable.

Client secret, nonce and PKCE verifier use existing AES-GCM encryption. GET
returns `has_client_secret`, never plaintext for any role. PATCH requires the
current `revision` and `secret_action`: preserve, replace or remove. Remove
requires public-client method `none`. Secret changes require recent auth.
The effective authentication method and stored secret must agree in the
repository transaction and migration CHECK: public `none` has no ciphertext;
basic/post requires ciphertext. Switching confidential to public requires
explicit removal; switching public to confidential requires replacement.
Invalid combinations leave revision, default-provider selection, audit and
outbox unchanged. The bilingual form explains these conflicts before submit
and retains explicit removal confirmation.
Rotation replaces one secret; it does not promise IdP-side zero downtime. Once a provider has bound subjects, its issuer and client ID are immutable: create a new provider to change the subject namespace.

## Verified domains

Owner/Admin adds a canonical IDNA domain. Creation/regeneration shows the
random verification token once; only SHA-256 is stored. Publish:

```text
Host: _modurelay-verification.example.com
TXT:  modurelay-verification=<value shown once>
```

Manual verification uses the server DNS resolver with a five-second timeout.
The transaction compares the original token hash after lookup, so regeneration
cannot race an old successful check. Regeneration immediately invalidates the
old token. A transient DNS lookup failure preserves an already verified domain;
an authoritative TXT mismatch marks it failed and stops verified-domain
discovery/JIT. Revoke releases the claim. `DOMAIN_ALREADY_CLAIMED` gives no other
tenant details. Automatic periodic DNS reverification is deferred.

## Generic OIDC and presets

Register Authorization Code flow, PKCE S256 and the exact callback. Required
scope is `openid`; defaults are `openid profile email`. Groups are optional.
Use discovery or explicit authorization/token/JWKS endpoints. All configured
and discovered endpoints have the same HTTPS/SSRF validation. Supported token
authentication: `client_secret_basic`, `client_secret_post`, public `none`.

| Preset | Setup |
| --- | --- |
| Microsoft Entra ID | Tenant-specific `https://login.microsoftonline.com/<tenant-id>/v2.0`; Web redirect URI, client ID, secret, optional group claims. |
| Google Workspace | `https://accounts.google.com`; OAuth Web client, redirect URI, organization consent policy, client ID/secret; restrict JIT to verified Workspace domains. |
| Okta | Exact authorization-server issuer such as `https://<organization>.okta.com/oauth2/default`; Web OIDC integration, redirect URI, client ID/secret, optional groups. |
| Generic | Exact issuer spelling, client ID/secret, scopes and bounded claim paths. |

Presets never relax verification. Entra can omit verified-email proof and
emit distributed/overage groups. JIT still requires a true signed boolean at
the configured verified-email path; UPN/domain alone is insufficient. Microsoft
Graph fetching is deferred. Google `hd` is not domain ownership. Test connection
checks discovery, endpoint safety, supported signing algorithms and reachable JWKS; test sign-in verifies credentials and callback. Provider GET includes only the last validation timestamp and a safe result code; edits invalidate that result.
Real provider availability and claim policy require actual provider validation.

## Linking, JIT, role and Team mapping

Existing email returns `OIDC_ACCOUNT_LINK_REQUIRED`. Sign into that account
and explicitly link with recent session/step-up or password plus account TOTP.
The actor ID and intent are stored server-side; callback cannot nominate a
different user. Verified email must match the target global account. Existing
provider subjects cannot be reassigned, even when email changes.

JIT defaults to viewer. User, email identity, Personal Workspace, organization
membership, stable subject and mapped access commit together. Suspended/removed
members are not reactivated. JIT never changes Owner or Billing Owner.

Bounded property paths map `email`, `name`, `email_verified`, `groups`; there is
no expression language or subject/issuer remapping. Accept at most 200 group
values of 512 bytes each. Role/Team mapping is separate. Only same-provider
OIDC roles reconcile; manual roles survive. Missing/overage groups preserve
access, while explicit empty groups reconcile to default role/no provider
grants. Manual, future SCIM and other providers' Team grants remain. Phase A
project access reads the live resulting Team memberships.

## Browser flow, assurance and enforcement

Start creates a ten-minute browser-bound, single-use state with nonce and
encrypted PKCE verifier. Callback atomically consumes matching state, exchanges
code and verifies signature/algorithm, exact issuer, audience, required exp/iat,
nbf, azp and constant-time nonce, then resolves identity/reconciles access.
Raw claims and provider tokens are never persisted or included in events. Expired state/completion records are pruned in bounded batches after one day of retention when new authentication records are created.

Callback stores a two-minute HttpOnly opaque completion cookie and redirects
to `/auth/sso/callback`. Same-origin POST exchange consumes that browser-bound
completion and issues a fresh local token family. Accounts with local TOTP must complete a browser-bound authenticator challenge before consumption/token issuance; IdP authentication alone does not assert local MFA. Tokens never enter URLs.
Return paths are restricted to `/workspaces`, `/dashboard`, `/profile`.

Enforcement is Owner-only. Enabling requires verified domain, active provider,
Owner binding, current successful Workspace OIDC assurance and valid provider
configuration. Sensitive changes require authentication within ten minutes or
recent account TOTP step-up. Rollout grace is bounded to seven days. All human
Workspace management routes, including separate Service Account/Policy routes,
enforce the Workspace policy. Enabling rechecks the requesting Owner's exact
provider/revision, active binding/member/user and verified domain while holding
the Workspace lock used by provider edits; outbound validation cannot race a
configuration change into enabling enforcement.

Legacy `/keys/:id` reads/updates/deletes also enforce human Workspace assurance.
Legacy list/search applies equivalent live policy, grace, Workspace,
provider/revision/status and assurance-age predicates before pagination/counts;
inaccessible Organization keys are absent, and returned Organization secrets
remain masked. A reached grace deadline requires assurance immediately.
Personal keys remain available. Gateway and other Workspace identities remain
independent. Provider revisions/disable and twelve-hour age invalidate assurance.

## Break-glass recovery

Use `/auth/sso?workspace_id=<id>&error=provider_failure` from the failed SSO flow. The Owner recovery section also appears when `required=1`. The global
password session remains available. The active Owner must reverify password,
account TOTP if enabled, confirm and provide a 10–500 character reason. The
endpoint only disables `require_sso`; it cannot grant access to unrelated APIs.
IP/user limits fail closed on Redis errors. A DB limit permits one recovery per
Workspace per fifteen minutes. Audit/event/outbox commit with the policy;
`workspace.sso.break_glass_used` notifies active Owners/Admins and eligible
webhook subscribers. Global administrator override is deferred.

## APIs and RBAC

All paths are under `/api/v1`. Owner/Admin: `identity.read`, `identity.manage`.
Only Owner: `workspace_sso.update`/recovery. Other roles cannot manage identity.
Tenant IDs never widen authorization; repositories repeat RBAC transactionally.

| API | Purpose |
| --- | --- |
| `GET/POST /workspaces/:id/domains` | List/create; return TXT once. |
| `POST /workspaces/:id/domains/:domain_id/verify` | DNS check. |
| `POST /workspaces/:id/domains/:domain_id/regenerate` | Invalidate old token. |
| `DELETE /workspaces/:id/domains/:domain_id` | Revoke. |
| `GET/POST /workspaces/:id/identity-providers` | List/create. |
| `GET/PATCH /workspaces/:id/identity-providers/:provider_id` | Redacted read/revision-checked update. |
| `POST /workspaces/:id/identity-providers/:provider_id/disable` | Sole-provider safety. |
| `POST /workspaces/:id/identity-providers/:provider_id/test` | Discovery/config test. |
| `GET/PUT /workspaces/:id/identity-providers/:provider_id/mappings` | Role/Team mappings. |
| `GET/PATCH /workspaces/:id/security-policy` | Enforcement/grace. |
| `POST /auth/sso/discover` | Domain-based discovery; no account-existence lookup. |
| `GET/POST /auth/sso/start` | Start browser login. |
| `POST /auth/sso/link/start` | JWT-authenticated explicit link. |
| `GET /auth/sso/callback` | Browser/state-bound callback. |
| `POST /auth/sso/exchange` | Same-origin single-use completion. |
| `POST /auth/sso/recover` | JWT Owner recovery outside SSO gate. |

Mappings: `{roles:[{claim_value,role,priority}],teams:[{claim_value,team_id}]}`.
Recovery: `{workspace_id,password,totp_code,reason,confirmed:true}`.

## Security gate answers

These answers describe the implemented contract, source review and local mock/
PostgreSQL regressions. They do not establish real-provider or production
acceptance.

| Gate | Answer |
| --- | --- |
| Q1 Existing email automatically takes over a Global User? | NO; authenticated explicit matching-account linking is required. |
| Q2 Email is the stable identity key? | NO; provider/subject with enforced Workspace scope is the key. |
| Q3 Role mapping automatically grants Owner? | NO; service/repository/SQL reject it. |
| Q4 OIDC automatically changes Billing Owner? | NO; the billing principal remains independent. |
| Q5 Workspace A assurance satisfies Workspace B? | NO; assurance is Workspace/provider/revision bound. |
| Q6 require_sso disables Personal password login? | NO; only the requested Organization control plane is gated. |
| Q7 require_sso blocks API Key/Service Account runtime credentials? | NO; Gateway uses its existing admission path. |
| Q8 Loopback/private issuer can be saved or accessed? | NO; IdP endpoints require public HTTPS. |
| Q9 Public discovery can point JWKS at private addresses? | NO; every endpoint and dial target is validated/pinned. |
| Q10 OIDC state can be replayed? | NO; matching browser proof is atomically consumed once. |
| Q11 Wrong nonce can authenticate? | NO; signature/claims and constant-time nonce checks fail closed. |
| Q12 email_verified=false can JIT? | NO; new users/links require true verified email. |
| Q13 Missing groups remove manual Teams? | NO; missing/overage claims preserve grants. |
| Q14 OIDC removes future SCIM membership? | NO; reconciliation changes only current-provider OIDC attribution. |
| Q15 Broken IdP permanently locks the Owner out? | NO; dedicated password/account-TOTP recovery disables enforcement. |
| Q16 Break-glass is a generic API bypass? | NO; active Owner, proof, confirmation/reason and limits are required; only enforcement is disabled. |
| Q17 Global Admin can read the client secret? | NO; GET never returns plaintext for any actor. |

## Events, troubleshooting and verification boundaries

Domain/provider/mapping/link/JIT/reconciliation/enforcement/recovery mutations
commit audit and outbox together. Failed DNS checks and normal SSO logins do
not notify. Domain verified, provider disabled, enforcement changed and recovery
notify Owners/Admins. Webhook allowlists accept only bounded scalar metadata.

`OIDC_ACCOUNT_LINK_REQUIRED`: explicitly link the existing account.
`SSO_REQUIRED`: reauthenticate with the requested organization's IdP.
`WORKSPACE_CONFLICT`: refresh revisions/satisfy enable or disable prerequisites.
`RECENT_AUTH_REQUIRED`/`STEP_UP_REQUIRED`: sign in again or complete TOTP.
Expired/mismatched state or completion: restart the browser flow. Provider
outage fails closed; use Owner recovery. Encryption configuration must be
repaired by the operator using the retained key.

Evidence is retained in `.cache/phase-b/`. Mock protocol and real PostgreSQL
tests establish local automated behavior. Real Entra/Google/Okta sign-in,
production load and final manual/browser acceptance are **NOT RUN**. Follow
[ENTERPRISE_SSO_ACCEPTANCE.md](ENTERPRISE_SSO_ACCEPTANCE.md).

The history migration test applies every migration through 293 to an isolated
PostgreSQL database, seeds existing Global Users, Personal/Organization scopes,
members, Teams, project grants, Direct/machine keys, Service Accounts, policy,
budget reservations, attributed/legacy usage, rollups, audit/events/outbox, then
applies 294 twice. Historical JSON snapshots remain unchanged apart from added
manual membership-attribution fields. Existing Workspace policies default to
`require_sso=false`; migration 294 never scans or rewrites historical usage.
