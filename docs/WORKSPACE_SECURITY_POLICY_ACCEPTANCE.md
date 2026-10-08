# Workspace Security Policy manual acceptance

Status: **NOT RUN**. Automated coverage does not establish live provider,
browser, authenticator, accessibility or production performance acceptance.
The manually managed acceptance application on port 18081 was not redeployed
or mutated during Phase D.

Use an isolated test Organization with an Owner, Admin, Developer, Billing,
Viewer, another Organization and Personal Workspace. Configure a verified
domain, two OIDC/SAML providers with explicit linked identities, a local TOTP
factor, and a SCIM connector. Use test credentials; never copy credentials,
assertions, invitation tokens or QR/secret material into reports or logs.

| Case | Action | Expected result |
| --- | --- | --- |
| Default compatibility | Open an Organization with no prior policy and compare existing SSO-enabled Workspace. | New defaults are permissive; old require_sso/grace/revision remain. |
| Actual Session MFA | Enroll TOTP while retaining an earlier password-only Session, then require MFA using a verified Owner Session. | Earlier Session is denied `MFA_REQUIRED`; enrollment alone grants nothing. |
| Enrollment recovery | An unenrolled member opens a protected tenant page and completes setup. | `MFA_ENROLLMENT_REQUIRED`, enrollment then a separate current-Session verification. |
| Exact-family upgrade | Verify TOTP using the current access/refresh pair; try another family, replay, changed Workspace/provider metadata. | Only exact family upgrades; mismatch 403, consumed proof 409; Global Session retained. |
| Upgrade and refresh race | Trigger refresh and upgrade in one tab and two tabs; replace login during a delayed response. | Serialized pair adoption; stale response cannot restore old user/family/tokens. |
| Original clocks | Inspect safe timestamps before/after refresh and MFA upgrade. | Both primary/enterprise times and assertion deadline unchanged. |
| General age | Configure 900 seconds, use older Session, then refresh or verify TOTP. | Reauthentication required; refresh/step-up cannot extend age. |
| Enterprise ceiling | Configure 24-hour general age with require SSO; use enterprise assurance older than 12 hours. | SSO still required; general age cannot extend legacy ceiling. |
| SAML deadline | Use a valid short-lived assertion/session deadline and refresh before/after expiry. | Verified deadline remains and expiry denies Workspace access. |
| AND requirements | Require SSO+MFA+age; satisfy each condition in separate Sessions. | Every condition is necessary. |
| Wrong tenant | Use Workspace A assurance on Workspace B. | Denied; B membership/provider/approval never inferred from A. |
| Provider approval | Select provider A; sign in using active provider B. | `IDENTITY_PROVIDER_NOT_APPROVED`; approved provider sign-in recovers. |
| Provider disable | Try disabling/updating last usable selected provider under required SSO. | Conflict and no partial policy/provider mutation. |
| Security RBAC | Open Security as each role; try PATCH as Admin/Developer/Billing/Viewer. | All can read when assurance permits; only Owner updates. |
| Recent auth | Save/relax policy with old primary proof, enrolled unverified Session, verified recent TOTP grant, and fresh strong login. | Old/unverified proof rejected; recognized recent strong proof succeeds. |
| Policy GET denial | Open Security with insufficient MFA/age/SSO. | Protected GET stays denied; safe recovery UI works then reloads. |
| PATCH omissions | Change one field; omit age/grace; then clear each explicitly with NULL. | Omitted values preserved; explicit NULL clears. |
| Timestamp preservation | Save an unrelated field with an existing grace deadline containing seconds. | Exact existing timestamp retained. |
| Revision conflict | Two Owner tabs edit same revision and save. | One succeeds, stale tab 409 and explicit reload; no silent overwrite. |
| Self-lockout | Enable MFA without factor/proof; enable SSO without approved provider/domain/link. | Rejected before mutation; Owner remains recoverable. |
| Invitation disabled | Disable invitations, create new invitation and accept a previously pending one. | Both denied, pending token unconsumed. |
| Invitation tightening | Create external invitation, disable external members, then accept. | Latest policy rejects and preserves token/member/source state. |
| Exact domain | Test verified domain, case/IDNA normalization, subdomain and revoked domain. | Exact canonical verified domains only; revoked/subdomain rejected. |
| Zero domains | Restrict domains with no verified domain. | UI warning and new admission denied; existing active members retained. |
| Workspace JIT | Provider JIT=true, Workspace JIT=false; test new and inactive OIDC/SAML users. | Shared admission rejects without partial Global User/source creation. |
| Retained active member | Tighten domain/external/JIT settings for existing member. | Existing active membership/source retained; human assurance still required. |
| SCIM admission | Create/reactivate external SCIM User under external/domain restrictions. | Same shared denial with SCIM envelope; no partial resource/member/source state. |
| SCIM Bearer separation | Require SSO+MFA+short age and provision allowed user with Bearer. | Bearer retains independent auth; human Session is irrelevant. |
| Human SCIM management | Use insufficient human assurance for connector/token/group controls. | Current Workspace security policy rejects. |
| Administrative blocks | Suspend/remove member and attempt JIT/SCIM active restoration. | Administrative flags prevail; automation cannot revive access. |
| Administrative restore | Explicitly restore external inactive member under restrictive policy. | Shared admission still applies. |
| Cross-source race | Hold policy/domain write lock while starting every admission path, commit tightening. | Entrants wait then reject; no partial rows/audit/outbox/token consumption. |
| TOTP disable dependency | Disable factor while active in any requiring Workspace; repeat after leaving/relaxing it. | `WORKSPACE_MFA_REQUIRED` while needed; allowed after dependency removed. |
| Owner factor/policy race | Concurrently disable Owner factor and enable MFA. | One wins safely; cannot commit Owner self-lockout. |
| Audit lock concurrency | Concurrent same-actor policy save and provider/member mutation. | No User/Workspace/FK deadlock; complete atomic audit/outbox. |
| Break-glass | Recover with fresh strong Owner proof, confirmation and reason; retry inside rate window. | Auth constraints relaxed only; admission/provider settings retained; bounded retry denied. |
| Recovery refusal | Use non-Owner, wrong password/TOTP, missing confirmation, short reason. | Fail closed without policy/event/limit mutation. |
| Factor material | Exercise setup/verify/disable with DEBUG logging. | No secret/prefix/decrypted/QR/raw proof in logs/audit/events/notifications/webhooks. |
| Policy unavailable | Make policy read unavailable in an isolated environment. | 503 `SECURITY_POLICY_UNAVAILABLE`; no stale cached allow. |
| Legacy keys | List/search/count/detail/mutate Organization project keys using insufficient assurance. | No hidden key leakage or legacy human bypass. |
| Machine runtime | Execute Direct/machine Gateway traffic and Seedance/Grok/Canvas critical paths. | Existing execution policy/budget/quota applies; no human MFA/SSO/age gate. |
| Audit/event/webhook | Save policy, inspect safe old/new audit metadata and delivered event/notification. | One atomic revision/event/outbox; Owner/Admin notified; no credentials. |
| Browser context race | Navigate A→B→A, change user/session, delay old denials/creates/previews. | No stale redirect, secret presentation, confirmation or tenant takeover. |
| Failed reauth | Fail password/TOTP/passkey in explicit reauth mode. | Valid existing Global Session preserved and proof retry remains usable. |
| Responsive/accessibility | Check keyboard, focus, form errors, dialogs, EN/ZH, light/dark and mobile. | Labels/errors actionable, current tenant obvious, no inaccessible controls. |

Record actual PASS/FAIL and evidence per row after executing this runbook. Keep
unavailable real IdP/SCIM/provider checks NOT RUN; automated fixture success is
not live-provider acceptance. Measure normal tenant and admission latency,
Workspace-lock contention and migration DDL duration before production rollout.
