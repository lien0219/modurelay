# Enterprise SSO acceptance

Final manual/browser acceptance and real Entra, Google Workspace and Okta
validation are **NOT RUN** in Phase B. No acceptance deployment is requested.
Record actual PASS/FAIL evidence when an approved environment is available.

## Setup

- Apply all historical migrations through 293, then 294. Preserve existing
  Users, membership, Teams, project grants, keys, Service Accounts, policy and
  immutable billing snapshots. Configure retained encryption key/callback.
- Prepare two Organizations, Personal Workspace, all five roles, recovery
  Owner with password/TOTP, bound and unlinked same-email accounts.
- Register exact callback/scopes/claims with each real IdP.

## Manual checklist

| Area | Required checks | Status |
| --- | --- | --- |
| Domains | IDNA/canonicalization, one-time TXT, regeneration race, verify/recheck/revoke, tenant conflict privacy. | NOT RUN |
| Providers | Generic/presets, discovery/manual endpoints, basic/post/none, explicit removal/replacement and cross-field guidance, revision conflict, redaction, sole-provider disable. | NOT RUN |
| Protocol | Code+PKCE, state/nonce/issuer/audience/signature/exp/iat/nbf/azp, replay/expiry, JWKS rotation, token-free redirect. | NOT RUN |
| SSRF | HTTP/private/metadata/mapped IPv6, malicious discovery endpoint, redirects, DNS rebinding rejected. | NOT RUN |
| Linking/JIT | Existing email not adopted; authenticated matching link; verified-domain/email gates; one user under concurrency; inactive member not reactivated. | NOT RUN |
| Roles/Teams | Owner forbidden, Billing Owner unchanged, deterministic priority, missing/overage groups preserved, explicit empty reconciled, manual/SCIM/other providers survive. | NOT RUN |
| Discovery | Unknown/existing account same domain gives identical provider response; no member/account-existence data; rate limits. | NOT RUN |
| Enforcement/session | Owner tested binding with locked revision recheck; grace deadlines; all tenant routes including Service Account/Policy and legacy `/keys` read/update/delete/list/search with filtered counts; A cannot satisfy B; original time on refresh; revision/disable/age invalidation. | NOT RUN |
| Recovery | SSO recovery section, active Owner, password/TOTP, confirmation/reason, all limits, atomic policy/audit/event/notification/webhook. | NOT RUN |
| Frontend/RBAC | English/Chinese, responsive/keyboard, role visibility, stale Workspace switching, cross-tenant IDs denied. | NOT RUN |
| Compatibility | Personal/other Workspace, Direct Key, Service Account credential, policy AND semantics, budget/billing, Seedance/Grok/Canvas ownership. | NOT RUN |

## Recovery drill

Enable only after successful Owner link/sign-in. Simulate provider outage.
Sign in globally, open `/auth/sso?workspace_id=<id>&error=provider_failure` and its Owner recovery section, reverify password/TOTP and confirm
with reason. Verify enforcement disabled, audit/outbox/security notification
and webhook, non-Owner denial, rate limits and subsequent normal access.
Restore IdP, reauthenticate and test before enabling again.

## Automated boundaries

Roadmap/local `.cache/phase-b/` evidence records automated tests separately.
Windows failures must be reproduced with identical commands on `828ccb841`
before classifying PRE-EXISTING. Mock PASS never establishes real-IdP PASS.
Production load, periodic DNS reverification, SAML/SCIM, Phase D session policy
and final release/manual acceptance remain deferred.
