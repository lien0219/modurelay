# Enterprise SCIM acceptance

Phase C2 adds provisioning to the verified C1 SAML boundary
`41b0b31dfd59ca2ddc337b78d2c1bc609bba0a77`, on branch `feature/new-feature`.
The original Phase C baseline is `81e68865463755f3fb684199b3de5a31e8572713`.
Migration 296 is the only new C2 migration; migrations through 295 are retained.
No push, PR, branch switch, deployment or acceptance18081 runtime mutation occurs.

## Local automated matrix

| Area | Result | Evidence |
| --- | --- | --- |
| Connector/token lifecycle and hash-only/show-once | PASS | Service tests inspect generated entropy,32-byte hash input,prefix/expiry,typed errors; realPG rotation up to8 active tokens/revoke/disable |
| Users/Groups HTTP and protocol fixtures | PASS | Auth/media/errors/config/schema/types,limits,filter injection,PUT/PATCH input,version forwarding,Location/ETag,DELETE204,404/501 |
| Typed tenant/source integrity | PASS | Direct SQL rejects foreign connector/group/Team/provider type/member/resource/effective source and automated Owner |
| Concurrent POST x8 | PASS | Exactly1 resource and7 typed409 conflicts; duplicate resource/externalId cannot create another user |
| Concurrent Group PATCH/token operations | PASS | Serialized additions preserve both members; max8 token slots; immediate revoke invalidates subsequent reads/writes |
| Multi-source Membership/Team | PASS | Manual+SCIM and OIDC+SAML+manual+SCIM Team; complete-empty provider claims and Group/member delete remove only own sources |
| Administrative suspension/removal | PASS | active=true does not clear suspension; removal preserves identity history and prior404; invitation cannot revive tombstone; explicit admin activation restores |
| Exact managed role source | PASS | Multiple SCIM connectors retain exact selected source,manual wins,stable fallback after removal; no billing/developer privilege ladder |
| Verified inbox/global profile safety | PASS | Trusted exact verified identity+Workspace domain required; unverified/global alias/external domain denied; existing profile/password unchanged |
| Owner/Billing Owner | PASS | User create/update/deactivate/delete protection including PATCH409; billing ownership never transferred |
| Audit/events/Webhook/inbox | PASS | Transactional scalar mutation/event/outbox,external NULL actor,no payload/token/email; success quiet,Owner/Admin operational alerts |
| Expiry/failure debounce | PASS | Locked expiry read/write/scan regressions; dormant expiry batch100/once-day; actual authenticated HTTP rejections threshold3/15min/fanout; conflict debounce; routine duplicate not security alert |
| Migration/control-plane history | PASS | Isolated295->296 all provider/Team backfill; rerun preserves newer administrative/source state;293->latest historical snapshots unchanged except new membership columns |
| Embedded frontend protocol path | PASS | RED SCIM interception fixed by /scim/v2/ bypass; both modern/legacy focused embed tests |
| Deleted grouped User lifecycle | PASS | DELETE204 removes references and versions Groups; stale Group412; later rename/add/remove succeeds; repeated DELETE no revision change; active=false retains assignments |
| Full frontend/Canvas | PASS | i18n/full lint/typecheck,424files3149tests; build24.69s; Canvas critical7tests; rebuilt Bun bundle16.62s and full embed web0.077s |
| Deleted User with unrelated Group Owner | PASS | Actual HTTP DELETE of ordinary A returns204 while unrelated Owner/Billing Owner B and all B sources remain unchanged; two Groups version once, stale If-Match412, exact A-source removal and one effective Team-removal event; repeat unchanged204 and direct Owner writes409 |
| User email-add retries (R1) | PASS | Protocol and actual HTTP/PostgreSQL RED before correction; 350 successful replay PATCH requests preserve resource/ETag/lastModified/mutation audit/events/outbox; real additions and strict bounds/protection remain covered |
| Backend relevant/PostgreSQL/race/vet/build/lint | PASS | Final post-R1 relevant service6.215s/handler0.081s/repo0.053s/middleware0.044s/migrations0.009s; realPG43.515s; all identity/SAML/SCIM realPG race51.510s; vet/build; pinned2.13.0 reports0issues |
| Dependency audit | PASS | Current govulncheck11module-only advisories,0imported-package/0reachable-function findings; pnpm prod exception checker passed; no SCIM dependency |
| Full default backend suite | PRE-EXISTING | Final c2-after-r1-full-default.log exactly reproduces archived81e688 three PgDumper missing-sh failures |
| Full unit/integration backend suites, final runs | PRE-EXISTING | Post-R1 unit and repeated full integration reproduce exact archived baseline failure tests/packages: unit has3PgDumper plus Ollama stale CAS; integration has3PgDumper; no build errors/timeouts |
| Initial post-R1 external TLS test | FAIL on initial run; resolved on matched full reruns | TestAllProfiles received tls.peet.ws reset/refusal; original mismatch retained; full archived baseline/current integration reruns have no failing TLS test; no TLS source/test change |
| Supplemental integration-tag lint | PRE-EXISTING | Identical command on archived81e688 and current source returns13 identical path/line/diagnostic findings in10 untouched integration-test files; default full lint remains0issues |
| Independent backend/final source review | PASS | Whole-C2 review plus scoped R1 Spec/CodeQuality APPROVE, earlier F1-F3/B1 correction approvals and all11frontend files independently approved; no outstanding Critical/Important source finding |

Core relevant real PostgreSQL command:

```powershell
go test -tags=integration ./internal/repository -run 'TestEnterpriseSCIM|TestEnterpriseIdentity|TestEnterpriseSAML|TestWorkspace' -count=1 -parallel=1
```

Latest core result: PASS28.409s. Focused concurrency/source/schema/expiry race:
PASS12.237s. Additional exact managed connector-role test: PASS6.149s. Root strict
permission/source/service/migration scope: PASS0.125s/0.060s/0.011s. Protocol,
handler,notification and audit tests have separate fixture coverage. Raw logs and
immutable review packages are retained under ignored `.cache/phase-c/`.

Final post-R1 source gates ran sequentially with `GOMAXPROCS=4`:
relevant service/handler/repository/middleware/migration
PASS6.215s/0.081s/0.053s/0.044s/0.009s; real PostgreSQL
identity/SAML/SCIM/Workspace/governance PASS43.515s;
service/handler/repository/middleware race PASS9.370s/1.290s/1.116s/1.084s.
The final PostgreSQL race command includes all SCIM, OIDC identity and SAML
integration tests, including B1 and R1, and passed51.510s. `go vet ./...`,
`go build ./...` and pinned golangci-lint2.13.0 (0issues) PASS.

```powershell
go test ./internal/service ./internal/handler ./internal/repository ./internal/server/middleware ./migrations -run 'Test(SCIM|SAML|Enterprise|Workspace|Governance|DomainEvent|AuditLog)' -count=1 -timeout=5m
go test -tags=integration ./internal/repository -run 'Test(Enterprise|Workspace|Governance)' -count=1 -parallel=1 -timeout=5m
go test -race ./internal/service ./internal/handler ./internal/repository ./internal/server/middleware -run 'Test(SCIM|SAML|Enterprise|Workspace|Governance|DomainEvent|AuditLog)' -count=1 -timeout=5m
go test -race -tags=integration ./internal/repository -run 'TestEnterpriseSCIM|TestEnterpriseIdentity|TestEnterpriseSAML' -count=1 -parallel=1 -timeout=5m
go vet ./...
go build ./...
golangci-lint run --timeout=10m
```

Raw final backend logs use the `c2-after-r1-backend-*` and
`c2-after-r1-full-*` prefixes. The B1 regression failed twice before the
correction with HTTP409, then passed against real PostgreSQL after the fix.
The fix changes only deletion-target reconciliation; full Group Owner guards
remain intact. Whole-C2 review identified R1: replayed email additions appended
duplicates. The correction deduplicates complete supported complex email values,
normalizing `value` by lowercase/trim while retaining exact `type`, `primary` and
`display` equality. Both path forms preserve order, distinct supported attributes,
raw/result20-email limits, primary validation and protected identity behavior.
Historical stored duplicates are preserved. Protocol and real HTTP/PostgreSQL
RED/GREEN,350 successful retries and independent Spec/CodeQuality APPROVE are
retained in `c2-fix-round3-report.md` and `c2-r1-re-review.md`.

Final source matches all56 source hashes in immutable
`c2-release-review-v2.diff`, SHA256
`f6c4aefbe1845d6ed1720de875f85b0fd9da5870b69d5cd0d6fd92ca4f8e0aee`.
Completion documentation is reviewed separately against the final manifest.

The supplemental command below returns exit1 on both the actual archived
`81e688` baseline and final source, with13 identical findings in10 unchanged
integration-test files. The executed baseline comparison is retained in
`c2-integration-tag-lint-baseline-comparison.json`. This is **PRE-EXISTING**;
the required default full lint command above passes with0issues.

```powershell
golangci-lint run --timeout=10m --build-tags integration ./internal/service/... ./internal/repository/...
```

Full final commands are `go test ./... -count=1 -timeout=10m`,
`go test -tags=unit ./... -count=1 -timeout=10m` and
`go test -tags=integration ./... -count=1 -timeout=10m`. Their nonzero results are
**PRE-EXISTING**, not a blanket backend PASS. Both failure test names and failure
packages match the actual archived81e688 runs in the final comparison
`c2-post-r1-delivery-baseline-comparison.json`, including the missing-sh error
and the unit Ollama CAS assertion. The four unique final failures are:

- TestPgDumperHoldsMigrationLockThroughReaderClose
- TestPgDumperReleasesMigrationLockWhenProcessFails
- TestPgDumperReportsUnlockFailureAndDiscardsConnection
- TestOllamaProbeCallback_StaleLongDoesNotOverrideNewShort (unit only)

The first post-R1 integration attempt additionally failed `TestAllProfiles` on
external `https://tls.peet.ws/api/all` TCP reset/refusal. Its original log and
`c2-after-r1-baseline-comparison.json` retain the initial mismatch. Root verified
unchanged TLS Go sources against baseline Git blobs/archive and repeated the
identical full integration command sequentially on actual baseline and final
source with `GOMAXPROCS=4`, taking167.885s/171.047s. Both complete reruns have only
the three identical PgDumper failures, no build errors/timeouts and no failing
TLS test. Raw logs are `baseline-integration-network-recheck.log` and
`current-integration-network-recheck.log`; structured evidence is
`c2-integration-network-recheck.json`. This initial external-network FAIL is
disclosed separately rather than labeled PRE-EXISTING. Existing network skips
remain intact; these runs do not prove live external fingerprint validation.

## Security gate answers

| Question | Answer |
| --- | --- |
| Q1 SAML provider fake OIDC fields? | NO; nullable real protocol fields with DB checks. |
| Q2 Unsigned SAML assertion can log in? | NO; mature-library signature validation and malicious fixtures. |
| Q3 Transient NameID permanent subject? | NO; persistent or explicitly configured stable subject required. |
| Q4 SAML existing-email automatic takeover? | NO; existing account requires authenticated explicit linking. |
| Q5 SAML satisfies Require SSO? | YES; provider/type/Workspace/revision/time assurance. |
| Q6 OIDC still satisfies Require SSO? | YES; same enforcement engine and regression fixtures. |
| Q7 SCIM active=false deletes Global User? | NO; only own attributed sources are deactivated. |
| Q8 SCIM deactivates Owner? | NO;409 protection before source mutation. |
| Q9 SCIM changes Billing Owner? | NO; manual-only billing transfer,deprovision409. |
| Q10 SCIM removal deletes manual source? | NO; source-specific reconciliation. |
| Q11 SCIM removal deletes OIDC/SAML source? | NO; exact provider/connector/Group source keys. |
| Q12 Group takes over same-name manual Team? | NO; explicit same-Workspace existing-Team binding. |
| Q13 Plaintext token can be GET again? | NO; immediate create response only,SHA256 storage. |
| Q14 Bearer payload chooses another Workspace? | NO; unknown control attributes rejected,tenant from credential. |
| Q15 Require SSO blocks SCIM Bearer? | NO; human management is guarded,protocol routes independent. |
| Q16 SAML/SCIM enters Gateway hot path? | NO; existing live tenant admission/billing/principal engine retained. |

## Entra/Okta operator checklist for Phase L

1. Configure retained HTTPS enterprise callback origin and verified Workspace
   domain; create an active connector with the intended non-Owner default role.
2. Copy supplied Base URL and create one-time token. In Entra custom application
   provisioning or Okta SCIM application, enter Base/Tenant URL and bearer secret.
3. Map userName,externalId,active,primary work email,given/family/display name.
   Provision Users before Group membership references; use supported eq filters.
   Bootstrap and explicitly link employee SSO identities through the existing
   authenticated linking flow; SCIM creation does not automatically adopt an
   existing or newly provisioned Global User into a provider subject binding.
4. Synchronize a small directory sample,inspect resources and last-sync/outcomes,
   then explicitly bind each returned Group to an active Workspace Team. Team
   Project Access Grants remain the existing authorization mechanism.
5. Verify disable/deactivate/reactivate/delete,manual/SSO coexistence,admin
   suspension,Owner/Billing Owner rejection and repeated provider retries.
6. Rotate by creating a second active token,update the client,verify sync and
   revoke the old token. Test expiry/failure/conflict notifications with Owner/Admin.
7. Confirm raw credentials/PII absent from logs,audit,inbox/Webhooks and no changes
   to global passwords,other Workspaces,keys,Service Accounts or billing history.

Entra SAML,Okta SAML,Google/other SAML,Entra SCIM and Okta SCIM are all **NOT RUN**.
Browser/operator/light-dark/mobile,production load and deployment are **NOT RUN**,
reserved for Phase L. Local fixtures do not claim vendor certification.

## Known limits and migration risks

IdP-initiated SSO and SLO remain deferred. SCIM supports core schema and bounded
eq filters only; bulk/sort/password/schema extensions unsupported. Primary inbox
cannot change after binding. Connector disable retains existing sourced access;
deprovision explicitly before disable if removal is intended. Large groups use
bounded PATCH batches; Workspace-wide serialization can require client retries.

Historical suspended members conservatively become administratively suspended.
Legacy unbound SCIM attribution is conservatively manual because no connector
evidence exists. Review these control-plane rows after migration. Retain the
existing encryption key for SAML; SCIM credentials cannot be recovered from hash.
