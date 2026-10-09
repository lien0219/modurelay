# Enterprise data lifecycle architecture — Phase G

Architecture Gate: **APPROVED for implementation**, 2026-10-09. This gate
authorizes source changes and disposable PostgreSQL tests only. Production
export, restore, purge and deployment require the separate Phase L gate.

## Architecture findings

The starting branch is `feature/new-feature`, clean at
`3888ba452d8da926d1d6d234b7641acb6ebcc193`. Phase F
`fe39bcf126fe9bda5b776bbab309b734037295c2` is its ancestor; the intervening
toolchain/dependency repair is retained. Existing migrations end at 299.

Workspace archive is a status mutation with audit/event/outbox; there is no
Owner restore. Budget admission locks Global Users before Workspace and Project;
settlement uses the frozen reservation even after archive or owner transfer.
Phase E evidence already rejects UPDATE and DELETE. Migration 299's allocation
guard is declared for both operations but actually rejects only UPDATE and
returns OLD for DELETE. Its two parent FKs cascade. Usage cleanup and dashboard
retention can delete financial usage; partition DROP bypasses row triggers and
also removes rollups. All these paths must be protected together.

Administrator backup/restore uses whole-database pg_dump and independent backup
retention. It is not a tenant export or tenant restore. Existing delivery
retention has bounded SKIP LOCKED batches, but its legacy fixed durations need
tenant/platform retention predicates. Canvas resources are Global User owned
unless a durable Workspace binding exists; creator membership is not ownership.

## Gate decision and alternatives

Use durable tenant export/deletion jobs, conservative financial retention,
repeatable-read export snapshots, and retained identity/audit tombstones.
Physical tenant-root cascade deletion is rejected because it can erase evidence
or prevent frozen asynchronous settlement. Copying financial evidence into a
new archive and deleting its original graph is rejected for this phase because
it would require a second reconciliation and dedup authority.

Protected historical financial evidence is retained separately and does not
block business closure. It is never deleted, anonymized by rewriting allocation
fields, or detached from its original parents. Organization scopes can purge
explicitly allowlisted business resources and sensitive credentials after
cooling periods while preserving protected metadata and reference parents.
`deleted` denotes completed business closure/eligible resource cleanup with
`protected_evidence_retained=true`; it never denotes partial completion or
physical erasure of financial rows. Only pending obligations/tasks, active legal
or business holds, unknown safety state and concurrent operations block cleanup.
This separation was explicitly confirmed by the user on 2026-10-09.

## Data classification and ownership matrix

Days are platform business defaults, **not claims of legal compliance**. Zero
means indefinite retention and disallows purge. Platform operators must review
applicable law and contract obligations before changing configured floors.

| Resource | Class | Default floor | Ownership/dependencies | Purge eligibility and method | Verification/blockers |
| --- | --- | --- | --- | --- | --- |
| Workspace metadata | operational/audit | retained tombstone | Workspace; Owner and billing principal are Global Users | Never DELETE root; minimize only after complete eligible job | No pending job or financial obligation |
| Projects | operational/audit | retained tombstone | Workspace; usage/audit/grants reference IDs | Archive in bounded batches; preserve IDs | Parent state, default Project and references |
| API key and Service Account metadata | security | 365 days | Project; reservations/usage reference credentials | Revoke; preserve referenced identifiers; never export secrets | No pending admission/settlement; bounded credential batches |
| Usage and tenant rollups | financial | indefinite | Frozen Workspace/Project/key/principal/reservation | Ineligible for Phase G purge; legacy unassigned cleanup remains bounded | DELETE guard; financial partition DROP prevention; reconciliation |
| Wallet, payment, subscription, refund/recovery | financial | indefinite | Global User/billing principal, not tenant membership | Outside tenant purge/export | Workspace B and global funds unchanged |
| Budget reservations/counters/settlement dedup | financial | indefinite | Frozen Workspace/Project/principal/credential | Ineligible; pending holds block deletion | Replayed settlement charges/refunds once |
| Reservation/usage allocation snapshots | financial | indefinite | Original reservation/usage/cost center parents | UPDATE/DELETE prohibited; no trigger bypass | Direct DELETE and parent CASCADE rejected |
| FinOps anomaly evidence/findings | financial/audit | indefinite | Workspace/Project and immutable snapshot | Ineligible | Immutable evidence remains queryable |
| Workspace audit logs | audit | indefinite | Workspace/Project/actor parents retained | Retained, no lifecycle purge | Mutation/event/outbox atomicity |
| Domain events | audit | indefinite | Workspace, delivered outbox/notification/webhook children | Only when configured finite policy and all children eligible | Pending delivery and live leases preserved |
| Notifications | operational | 180 days | Recipient Global User plus explicit event Workspace | Indexed bounded retention after effective floor | No foreign Workspace deletion |
| Webhook deliveries/outbox | operational/audit | 180/indefinite | Explicit Workspace/event/webhook | Completed only, bounded and policy gated | Pending/dead recovery and lease blockers |
| SSO/SCIM/security metadata | security | 365 days | Workspace bindings; Global User separate | Revoke credentials; retain bindings needed by audit | Current security policy; no global identity deletion |
| Export job metadata | audit | indefinite | Explicit Workspace and requesting Owner | Retained bounded listing | No object key, encryption key or token disclosure |
| Export artifacts/download grants | temporary | 7 days / minutes | Server-derived Workspace/job/attempt key | Ciphertext object deletion; expired grants rejected | Prefix, lease fencing, one-use token, retryable cleanup |
| Canvas/media | operational/financial | conservative retain | Explicit durable ownership only | No membership-derived delete; bound async media blocks purge | No known binding means reported retention/blocker |

Policy precedence: indefinite platform protection wins; otherwise effective
days are at least the platform floor and the Workspace requested duration.
Workspace policies cannot lower floors. Existing legacy durations are also
minimums on legacy workers. Cooling starts at request time. Credential revocation
and logical closure do not erase evidence records. Physical deletion of business
configuration additionally requires its effective category deadline; ineligible
configuration is retained and does not delay business closure.

## Export contract and consistency

Owner-only, central RBAC plus live repository recheck. Durable states:
`pending -> running -> completed/failed/cancelled -> expired`. Each worker claim
uses SKIP LOCKED and a fresh token; progress/completion require the same live
token. Expired leases can retry with a bounded attempt cap and backoff.

One read-only PostgreSQL REPEATABLE READ transaction captures every section and
its server timestamp and MVCC visibility boundary. Extraction uses keyset pages and
fixed row/byte/time limits; there is no Workspace write lock during extraction.
Concurrent changes are outside that snapshot. Oversized exports fail explicitly
and never produce a misleading partial completed archive. A retry starts a new
whole snapshot and a new object key, never mixes pages from different snapshots.

The structured ZIP contains explicit allowlisted Workspace, Project, member,
credential metadata, usage/FinOps, allocation, anomaly, audit and identity
sections. Manifest version, Workspace ID, cutoff, counts, exact decimal totals,
checksums and exclusions explain completeness. No `SELECT *` entity dump,
secret field, raw request body or global wallet is exported. Member export is
minimal IDs/roles/status; email/name requires a separately authorized contract.
This archive is a data portability artifact; restoration never extracts it.

Use the existing S3 client factory with separate configured bucket credentials
and fixed `workspace-exports/` namespace. Artifacts are authenticated encrypted
before upload using a persistent operator-provided 32-byte key. HTTP generates
short-lived single-use, hashed download grants; an authenticated Owner redeems
the grant, tenant/status/expiry are rechecked, and the server verifies the whole
decrypted ZIP in bounded memory before returning an attachment. No permanent or
bearer-only object URL is exposed. Downloads have row/byte/time bounds too.
The current format has one operator key and no key-ring lookup. Rotation requires
an approved drain of all old-key artifacts (including indefinite retention and
holds), or a separate future key-ring migration; loss of the key fails closed.

## Archive and controlled restore

Keep existing archive routes and add explicit Workspace/Project restore routes.
Only Owner restores Workspace; Project restore uses its central Project admin
permission. Workspace must be archived, Project's parent must be active, and
no active deletion job may exist. Recheck Global User, active membership,
billing Owner, current SSO/MFA/session policy under the Workspace lock. Only
the scope status changes: no credential, membership, Project grant, billing,
allocation or historical snapshot is re-enabled or rewritten. Existing admitted
work can settle while archive rejects all new admission.

## Deletion state machine and concurrency

Organization only; Personal deletion is a distinct unsupported operation with
an explicit blocker. Dry run returns eligibility, typed blockers, counts,
protected records, explicit purge scope, and earliest purge time. Request
requires Owner, recent strong authentication (actual MFA proof when enrolled
or required), exact Workspace name, a server-issued expiring one-use challenge,
and a persisted rate limit. Request locks Workspace, reruns preflight and
atomically records job, status, audit/event/outbox. No force flag exists.

Workspace states: `active/archived -> pending_deletion -> purging -> deleted`.
Job states: `pending -> running -> completed`, with `blocked/failed` recovery
and `cancelled` only before the first irreversible batch. Cancellation restores
the recorded previous scope status and never revoked credentials. Restore,
archive, request, cancel, worker mutation and admission share the Workspace
lock. Global User lock order remains User before Workspace to avoid inversion.

Preflight explicitly checks pending reservations, unsettled billing/recovery,
async images/video, retained usage/allocation/anomaly evidence, pending exports,
pending webhook/outbox delivery, child scopes, identities and retention dates.
Historical evidence appears in protected_records and retention disclosures,
not blocking_reasons. Policy-ineligible metadata is retained and verified, not
deleted. Credential values are destroyed after grace while security identifiers
remain for the enforced retention period. A simple operator-controlled active
hold blocks irreversible cleanup; complex legal-hold workflows remain deferred.
Unknown financial/task state is a blocker, not proof of absence. Credentials
and Projects are archived/revoked; Global Users and all foreign scopes remain.

Purge claims are bounded. Each checkpoint transaction locks Workspace then job,
verifies live lease token, reruns blockers, executes one indexed allowlisted
batch, and saves phase/cursor/progress together. Completion is permitted only
after every phase is verified empty/terminal. Crashes roll back a batch or leave
a committed checkpoint; takeover resumes it. Failed/blocked jobs remain visible
and never silently mark the Workspace deleted. Owner cancellation races and
stale worker updates return conflicts. Events use stable job/state dedup keys.

## Migration and tenant isolation risks

Migration 300 adds retention policies and financial deletion protection;
301 adds durable jobs/challenges/grants, indexed leases and additive states.
Do not edit historical migrations or rewrite historical usage. Retain composite
tenant predicates/FKs. New job relations use RESTRICT, never tenant-root CASCADE.
Strengthened financial DELETE guards require disposable-schema/database test
teardown rather than deleting evidence rows or disabling triggers. Existing
partition retention must inspect protected data before DDL, retaining financial
partitions and their rollups. Rehearse DDL on supported PostgreSQL versions.

The Global Admin role is not a tenant Owner. All IDs, object keys and storage
attempts are server-derived; numeric Project IDs and UUID job IDs are paired
with trusted Workspace ID. Restore bypasses neither tenant security nor global
identity lifecycle. Unknown Canvas ownership never broadens purge by user ID.

## Implementation plan and gates

- [x] G1: retention floors, immutable/root DELETE protection, all cleanup and
  partition paths, disposable PostgreSQL fixtures; RED/GREEN behavioral tests.
- [x] G2: durable encrypted snapshot exports, cancellation, fenced retries,
  one-use authorized downloads, expiry cleanup, manifest verification.
- [x] G3/G4: central RBAC, secure restore/preflight/challenges, durable deletion
  jobs and resumable allowlisted purge; audit/event/outbox atomicity.
- [x] Functional bilingual responsive UI and typed API integration.
- [x] Actual PostgreSQL concurrency/IDOR/retention/evidence/recovery matrix,
  independent destructive-path review and final security diff review.
- [x] Full backend and relevant race/static gates; pinned lint 2.13.0; full
  frontend/Canvas/media/FinOps E/F/security regressions. Reproduce full-suite
  failures on exact clean 3888ba452 baseline before PRE-EXISTING classification.
- [x] Operational/acceptance docs and roadmap; mark COMPLETE only after required
  gates pass; one local phase-boundary commit, no push/PR/deployment.

Feature rollout and purge kill switches default off. Production backup,
migration/restore rehearsal, object storage ACL preparation, operator approval,
load/manual/provider/deployment acceptance remain Phase L. Complex legal hold,
Phase H global diagnostics, Phase I platform chaos and Phase K UI redesign are
outside this implementation. Phase G's own safety/concurrency tests are required.

The completed local gate records are in
[acceptance](ENTERPRISE_DATA_LIFECYCLE_ACCEPTANCE.md). Native Go1.27.2 vet/build,
compatible lint2.14.0, actual PostgreSQL16/18.1 and relevant race pass. Original
native lint2.13.0 is PRE-EXISTING / TOOLCHAIN BLOCKED; equivalent-source compatible
checking is disclosed separately. The untouched full unit-tag Ollama CAS failure
reproduces on actual starting code and is not called PASS. No Phase G behavior
or protected-data constraint is weakened by these verification boundaries.
