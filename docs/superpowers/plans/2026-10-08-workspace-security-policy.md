# Workspace Security Policy Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans for the coupled core and superpowers:dispatching-parallel-agents for disjoint implementation/review work. Steps use checkbox syntax. User scope explicitly authorizes uninterrupted implementation and one final local commit; no intermediate push, PR or deployment.

**Goal:** Enforce actual human Session assurance and one member admission policy across all Workspace entry paths.

**Architecture:** Extend the existing SQL policy. The pure security/admission decisions consume authoritative policy and trusted session/provider metadata; SQL writes serialize under the Workspace lock. JWT/TOTP upgrades retain family, original time and enterprise expiry. Machine runtime remains independent.

**Tech Stack:** Go, PostgreSQL, Redis, Gin, Vue 3, TypeScript, Vitest, existing TOTP/WebAuthn and identity systems.

**Spec:** `docs/WORKSPACE_SECURITY_POLICY_ARCHITECTURE.md` and the complete user Phase D attachment.

## Global Constraints

- Branch `feature/new-feature`, baseline `6981e17de52d87ba4723b28452ad8e455e0317b5`; preserve unrelated changes; local commit only.
- Require MFA means current Session verified local TOTP, never enrolled status. OIDC/SAML/passkey alone are not granted MFA.
- Invitation/JIT/SCIM/manual restoration use one transactional admission helper and exact verified domains.
- Age is NULL or 900–2592000 seconds; legacy enterprise ceiling remains 12h and assertion expiry may shorten it.
- PATCH preserves omissions; explicit null clears age/grace; HTTP requires positive expected revision.
- Defaults: MFA=false, age=NULL, invitations=any, external=true, JIT=true, providers=any_active.
- Security read all tenant roles, update Owner only plus recent strong authentication. Fail closed on unavailable policy.
- Existing members/sources are retained. Automated paths never override admin blocks. No hot-data rewrite.
- Follow UI_DESIGN_SYSTEM.md, local Frosted/UI UX skills; functional UI with EN/ZH and preserved selectors.

## Task 1: Canonical policy, evaluator, migration and transactional update

Files: `backend/internal/service/workspace_security_policy.go`, evaluator tests, `enterprise_identity.go`, `enterprise_identity_service.go`, `workspace_access.go`; `backend/internal/repository/workspace_security_policy.go`, `enterprise_identity_repo.go`; migration297 and tests; `workspace_handler.go`, `workspace_sso.go`; domain event/recovery/notification files.

Interfaces: `WorkspaceSecurityPolicy`, source-compatible alias `WorkspaceIdentityPolicy`, `WorkspaceSecurityPolicyPatch`, `WorkspaceSecurityDecision`; `EnterpriseIdentityService.PatchSecurityPolicy(ctx,actorID,workspaceID,patch)`, existing `CheckWorkspaceAccess` delegates the evaluator; repository `PatchSecurityPolicy(ctx,workspaceID,actorID,patch)` and `loadWorkspaceSecurityPolicyTx(ctx,tx,workspaceID)`.

- [x] Write RED tests for enrolled-but-unverified MFA, AND SSO+MFA+age, null/expiry, wrong tenant/provider/revision, defaults, strict PATCH and stale revisions.
- [x] Run focused `go test ./internal/service ./internal/handler ./migrations -run 'TestWorkspaceSecurity' -count=1` and record actual missing behavior.
- [x] Implement canonical types and pure evaluator; preserve old OIDC wire names. No enrollment value can set MFA.
- [x] Implement migration and SQL load/update with User/Workspace/Policy lock order, expected revision, owner/current-provider/MFA self-lockout checks and atomic redacted audit/event/outbox.
- [x] Replace existing HTTP policy PATCH and central enforcement; preserve old internal method only as a wrapper using the same mutation path.
- [x] Test provider-last-approved disable/update and bounded recovery. Run relevant service/handler/migration tests.

## Task 2: Trusted session context, TOTP upgrade and factor dependency

Files: JWT/optional JWT middleware and tests, auth/refresh models, session authentication helpers, TOTP service/handler/wiring, new user factor repository and tests.

Interfaces consumed: canonical policy fields and require-MFA error contract. Produces complete `WithSessionAuthentication` context and separate enrollment/error hint; existing `POST /user/totp/step-up` accepts optional `refresh_token` and returns additive token pair for exact current family, never a client-supplied MFA flag. Frontend adopts returned credentials atomically.

- [x] RED tests cover context MFA/original times, exact-family upgrade, rejected cross-family/replayed refresh, preserved expiry/time/revision, secret log leakage, workspace factor-disable dependencies.
- [x] Implement server-verified TOTP upgrade preserving Session family and assurance; retain ordinary recent step-up behavior when no refresh token is supplied.
- [x] Remove secret/prefix/QR logging. Implement atomic user-lock dependency check/write and fail closed on SQL errors.
- [x] Run relevant auth/TOTP/refresh/middleware tests and factor-disable/enable concurrency PostgreSQL tests.

## Task 3: One transactional admission policy for every member source

Files: `backend/internal/service/workspace_member_admission.go`, pure admission tests; repository shared helper and tests; `workspace_mutation.go`, identity JIT sections, SCIM Users sections.

Interfaces consumed: `loadWorkspaceSecurityPolicyTx`, canonical fields. Produces `checkWorkspaceMemberAdmissionTx(ctx,tx,workspaceID,request)` deriving existing-active/admin flags from SQL and invoking one pure evaluator. Source values identify invitation create/accept, JIT, SCIM and explicit administrative restore.

- [x] RED tests for default/disabled/verified invitation and tightened pending tokens; same external/domain rejection in Invitation/OIDC/SAML/SCIM/manual restoration; workspace/provider JIT AND; retained active sources/admin blocks.
- [x] Wire helper before every membership/source/global-user mutation that grants new access, under existing Workspace row locks.
- [x] Invitation acceptance evaluates human session independently before consuming token; SCIM returns protocol-compliant errors and never requires human MFA.
- [x] Run real PostgreSQL policy/admission/provider races, IDOR and source-regression tests.

## Task 4: Functional Security UI and assurance recovery

Files: new `WorkspaceSecurityView.vue` and tests, WorkspaceFrame/router/API/store utilities, Identity/Invitations views and tests, TOTP API/modal/session adoption, HTTP access-error handling, EN/ZH modules.

Interfaces consumed: full policy GET/PATCH plus `expected_revision`, backend decision and verified domains/external counts; optional step-up token pair. Produces one policy editor, read-only member views, policy-aware invitations and scoped actionable MFA/SSO/age error flows.

- [x] Read UI design system/local skills and consult search data; write failing UI/API tests for fields, permissions, conflict, warnings and stale A→B→A responses.
- [x] Implement all fields with null-preserving API semantics, safe approved-provider pagination and confirmation. Identity retains summary/link and existing public selectors at the editor.
- [x] Implement policy-aware invitations without dropping rejected acceptance token; route error handling scopes redirects to the still-current Workspace/auth context.
- [x] Adopt server TOTP token upgrade without inventing browser assurance. Complete EN/ZH and run relevant Vitest, i18n, lint and typecheck.

## Task 5: Security review, complete verification and delivery

Files: operational/acceptance/report documentation; roadmap and existing enterprise docs. Local logs under ignored `.cache/phase-d/`; never commit credentials/cache.

- [x] Review each disjoint task's immutable patch and requirements; correct security/quality findings and rerun affected checks.
- [x] Run backend default/unit/integration, real PostgreSQL migration/concurrency and relevant race, gofmt/vet/build/pinned lint; full frontend Vitest/lint/typecheck/build and Canvas critical regression.
- [x] Run exact failing tests on archived baseline6981e17 before PRE-EXISTING classification. Do not weaken assertions or turn NOT RUN into PASS.
- [x] Run source/secret/dependency/tenant review; attempt available Codex Security capability preflight and document any blocker honestly.
- [x] Record all Q1–Q16 answers, performance boundaries, migration risks, manual acceptance NOT RUN and deferred Phase E+ scope.
- [ ] Final diff/status/check; force-add only explicit ignored documentation paths, stage only reviewed task files; local commit `feat(security): add workspace security policies and assurance enforcement`.
- [ ] Verify commit/working tree, roadmap Phase D COMPLETE and E NEXT; final self-contained report links exact commit and evidence.

## Verification boundary and final Git steps

All implementation and relevant post-review checks above are complete. The full
three-tag backend suites ran before the last narrow review fixes; fresh affected
unit/real-PG/race/vet/build/lint and frontend/build/embed checks verify those fixes.
Full-suite baseline failures, the initial memory transient and formal sealed
coverage PARTIAL are recorded in the delivery document. Browser/provider/load
acceptance is NOT RUN. The two final Git checkboxes are intentionally pending in
this pre-commit plan; the delivery response records the actual commit and clean
working-tree result after this document is staged, without rewriting the commit
to embed its own identity.
