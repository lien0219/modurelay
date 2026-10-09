# Enterprise data lifecycle (Phase G)

Workspace business closure and protected evidence retention are separate results.
An Organization can reach `deleted` with a completed deletion job, while its
financial/audit graph remains available to authorized historical queries.
`business_closed=true` does not claim physical erasure or account deletion.
Personal Workspaces use a separate Global User lifecycle and cannot use this flow.

Architecture decisions and classification are in
[the approved architecture](ENTERPRISE_DATA_LIFECYCLE_ARCHITECTURE.md).
Executed verification and release boundaries are in
[the acceptance record](ENTERPRISE_DATA_LIFECYCLE_ACCEPTANCE.md).

## Retention and ownership

`0` means indefinite retention. The effective policy is the platform protection,
then the greater of the platform floor and the tenant's requested duration.
An indefinite policy wins. Missing/unknown policy also retains data. Financial
protection cannot be weakened by a Workspace setting or a finite platform value.
Default days are operational180, security365, temporary7, financial0 and audit0.
These are conservative product policies, not an assertion of legal compliance.
Platform values may be changed only through approved operational policy review;
Phase G does not expose an operator policy management API.

| Resource | Classification | Phase G deletion behavior |
| --- | --- | --- |
| Workspace/Project/Team parents | operational/audit | Logical closure, archived/minimized names and descriptions; IDs retained |
| API keys and Service Accounts | security | Credential values destroyed; keys inactive, accounts disabled; referenced IDs retained |
| OIDC/SAML/SCIM/webhook credentials | security | Secret values/hash material invalidated; scoped provider/connectors disabled; audit bindings retained |
| Invitations | security | Revoked, email minimized, token hash replaced; identifier retained |
| Project access grants | security | Physically removed only beyond effective security retention; otherwise retained under a closed parent |
| Current Project/key/machine allocations and their tag links | operational | Physically removed only beyond effective operational retention; ineligible configurations retained |
| Tenant Usage and financial rollups | financial | Retained; legacy cleanup excludes all tenant attribution and allocation evidence |
| Budget reservations/counters/settlement alerts/dedup | financial | Retained; pending obligations block irreversible cleanup; frozen settlement UPDATE remains possible |
| Usage/reservation allocation snapshots and anomaly evidence | financial/audit | Immutable and retained; original parent relationships preserved |
| Workspace audits and export/deletion metadata | audit | Retained by this lifecycle; no raw audit metadata in exports |
| Tenant domain events/outbox | audit | Indefinite by default; only eligible completed evidence under approved finite policy is cleaned |
| Notifications and completed webhook deliveries | operational | Legacy minimums and effective tenant floor both apply; pending/live deliveries retained |
| Completed export artifacts | temporary | Encrypted objects removed only after current policy eligibility and no active hold |
| Failed/replaced export attempt objects | temporary job residue | Durable ledger cleanup after its safety delay; active holds and live leases still apply |
| Global User/wallet/payment/subscription/refund/Canvas | Global User owned | Outside tenant deletion and export; no membership-derived deletion |
| Bound async image/video jobs | operational/financial | Pending/unknown submission/settlement blocks cleanup; metadata retained |

Physical configuration removal is distinct from immediate access revocation.
Retained grants/configurations cannot reopen admission through a closed
Workspace, an archived Project or revoked credentials. This release retains
policy-ineligible configuration after completion; it does not promise a later
automatic physical sweep of that configuration. Credential revocation does not
wait for the metadata retention deadline.

Existing Usage cleanup remains compatible for genuinely unassigned legacy rows.
Tenant hot settlement markers stay linked to reservations. Allocation parent
FKs are RESTRICT; immutable UPDATE/DELETE and financial TRUNCATE guards remain
enabled. Partition DROP locks the verified attached child and rechecks protected
data before DDL; tenant rollups are not invalidated by legacy partition cleanup.
Event retention uses bounded durable cursors, policy/hold fencing, live lease
checks and a final child recheck before parent DELETE.

## APIs and authorization

All routes are authenticated beneath `/api/v1/workspaces/:id`. A Global Admin
does not acquire tenant ownership. Service/Repository checks use current central
Workspace RBAC, active Global User/membership and current SSO/MFA/session policy.
Project IDs are numeric; job IDs are canonical UUIDs paired with Workspace ID.

| Route | Contract |
| --- | --- |
| GET `/lifecycle` | Owner/Admin policy and deletion status; feature capabilities |
| PUT `/retention/:category` | Owner; days0 or at least the platform floor; transactional audit/event/outbox |
| POST `/restore` | Owner, archived Organization, valid billing Owner, current security and recent proof |
| POST `/projects/:project_id/restore` | Central Project restore permission; active parent and archived child |
| POST/GET `/exports` | Owner-only creation and paginated history |
| GET `/exports/:export_id` | Owner-only tenant-scoped metadata/progress |
| POST `/exports/:export_id/cancel` | Owner-only; fences any old worker token |
| POST `/exports/:export_id/download` | Owner-only, completed/unexpired export; single-use60-second grant |
| POST `/exports/:export_id/download/redeem` | Authenticated Owner; token in POST body; binary ZIP attachment |
| GET `/deletion/preflight` | Owner-only dry run, typed blockers, capped counts, retained/purge scope and earliest date |
| POST `/deletion/challenge` | Owner, current security/recent strong proof; one-use10-minute challenge |
| GET/POST `/deletion` | Status / exact-name plus challenge request; fresh preflight under Workspace lock |
| POST `/deletion/:job_id/cancel` | Owner/recent proof; only before the first irreversible phase/cursor/progress |
| POST `/deletion/:job_id/retry` | Owner/recent proof; failed/blocked job, fresh preflight and feature switch |

Sensitive restore/delete/cancel/retry operations require primary authentication
within10 minutes or trusted recent step-up evidence; an enrolled or required
factor needs actual proof. `totp_enabled=true` is never proof of session MFA.
Request bodies are limited to16 KiB. Challenge issuance is capped at5 per hour;
export creation at4 per day and download grant issuance at20 per minute per
Workspace/actor. Tokens/object keys/worker lease values never appear in job JSON.

## Structured tenant exports

This is a portability ZIP, separate from administrator pg_dump and Global User
backup. Its allowlist includes Workspace, Projects, minimal member IDs/roles/
status, credential metadata, Service Accounts, Usage costs, immutable allocations,
Cost Centers/tags/Project allocation metadata, anomaly evidence, bounded audit
envelopes, identity protocol/status and reservations with frozen allocations.
It does not represent every configuration table or a restorable database backup.
Global profiles/wallets/payment/refunds without durable Workspace ownership,
upstream account credentials, request/response bodies, arbitrary audit metadata,
and all secret fields are excluded. CSV strings defend against formula injection.

A read-only REPEATABLE READ transaction captures all sections and one MVCC
visibility boundary. Concurrent commits beyond it are excluded. The manifest
contains schema_version, Workspace/job IDs, export time, cutoff/snapshot,
consistency text, included sections, exact record counts/total, per-file SHA256,
decimal amount totals and explicit exclusions. Monetary values preserve decimal
precision; totals describe each included section separately.

Extraction uses500-row keyset pages (UUID reservation keyset separately),
100000 total records,64 MiB input/artifact bounds,20-second SQL statements and a
2-minute snapshot deadline. Oversize fails explicitly; no partial completed ZIP
is published. Retries start a new snapshot and attempt-specific object key.

An independent operator-provided32-byte hex key encrypts chunked AES-GCM before
filesystem/object storage. Scope, random salt, counter, length and a required
terminal chunk are authenticated. Only ciphertext enters the OS temp directory.
Object keys are server-derived `workspace-exports/{workspace}/{job}/{attempt}.enc`.
There is no public/presigned object URL. After one-use grant redemption, the
server reads/decrypts in bounded memory, verifies whole ZIP size/hash and rechecks
live membership/security/expiry before returning the attachment. Two downloads
and one export build per process limit shared memory concurrency.

Every attempt has a durable orphan ledger. Cleanup checks one indexed due
candidate per call, postpones retained candidates, fences policy/hold writes,
and locks job before ledger. Object IO has a10-second timeout; DB rollback leaves
the ledger retryable, including an already-deleted object. Lost leases/unknown
commit results do not create a direct object-deletion bypass.

## Archive, restore and deletion

Archive retains records and denies new Gateway/key/machine admission. Previously
accepted tasks retain their frozen billing/attribution context and can settle.
Restore changes only the scope status. It does not reactivate revoked keys,
disabled providers/members, child Projects, SCIM credentials or revoked grants, and
does not rewrite financial history. Restore/delete/admission serialize through
the Workspace lock, with Global User locks preceding Workspace locks.

Workspace deletion states are `active/archived -> pending_deletion -> purging ->
deleted`. Jobs use pending/running/blocked/failed/cancelled/completed, with durable
phase/cursor/progress, attempt cap5, lease60 seconds and token fencing.
Cooling is platform configurable with a database minimum7 days. Cancellation
restores the recorded prior Workspace status only while no irreversible cleanup
has started. Failed retries restart the explicit safe allowlist; blocked retries
retain checkpoints. Worker crashes roll back a batch or leave its committed
checkpoint. No force-delete route exists.

Preflight blockers include unavailable billing Owner, pending budget/quota holds,
nonzero Workspace/Project reserved counters, unresolved settlement alerts,
nonterminal/unknown async media, pending exports and webhook/outbox delivery,
active operator legal/business hold and unsupported Personal lifecycle. Historical
finance/snapshots/audits appear as protected records instead of blockers. Unknown
submission outcome remains blocked even if the task status otherwise looks final.
Counts stop at10001 and declare capping; no HTTP export scan runs synchronously.

Purge runs one explicit200-row phase in a20-second checkpoint transaction,
rechecks blockers, fences Workspace/platform policies, and never physically
deletes financial parents. Completion independently verifies every terminal
resource invariant; cursor exhaustion alone is insufficient. Status, minimized
Workspace identifiers, completion flags and audit/event/outbox commit together.
Stable job/event/state dedup keys prevent repeated lifecycle notifications.

## Operational runbook and release boundary

1. Before migration, take and verify an independent database backup and rehearsal
   restore. Inventory tenant partitions/FKs, pending reservations/tasks, current
   retention policies, holds and audit requirements. Review platform floors with
   the responsible operator; do not invent a financial expiry.
2. Rehearse migrations300/301 on a disposable copy. They add indexes/DDL/FK guards
   and can need an explicit lock window; production migration time/load is not
   measured by local tests. Do not disable triggers or constraints for rehearsal.
3. Keep `data_lifecycle.enabled=false` and `purge_enabled=false` initially. Prepare
   a private S3 bucket/prefix with independent least-privilege credentials,
   production TLS and no automatic bucket expiry that bypasses database holds or
   floors. Set a stable encryption key from approved secret storage.
4. Enable export first after encrypted upload/download and storage ACL checks.
   Enable purge only after retention review, grace configuration, pending-funds
   reconciliation, dry run and authorized operator approval for real-data use.
   Phase G implementation does not authorize production permanent deletion.
5. The configured service starts one process-wide durable worker. Application
   shutdown cancels/joins it; leases permit another instance to resume. Disabling
   export stops its worker and storage downloads; disabling purge stops purge
   claims and new destructive requests/retries. Policies/history/cancellation
   remain accessible according to RBAC. Switch changes currently require restart.
6. For blocked jobs, resolve the named obligation/approved hold through its owning
   subsystem and retry with current Owner proof. Never settle by rewriting frozen
   attribution, clearing reservations, or marking a live task terminal manually.
7. For failed jobs, inspect phase/cursor/failure code and the original resource
   invariants. Retry the safe allowlist only after the cause is repaired. A job
   already purging cannot be restored or cancelled. Rehearse any whole-database
   recovery in a separate environment; portability ZIPs are not restore inputs.
8. Monitor failed/exhausted leases, persistent blocked reasons, oldest pending
   jobs/deliveries/reservations, progress/checkpoint staleness and object cleanup
   errors. SQL job fields provide durable observability; log codes contain no
   secrets. No new high-cardinality metrics labels are introduced. Production
   alert routing and global diagnostics remain Phase H/L work.
9. The current encryption format uses one active key, with no old-key lookup.
   Do not rotate away from a key while retained/held artifacts still need it.
   An approved drain or separately implemented key-ring migration is required.
   Losing the key fails downloads closed; retain independent backups.

Complex legal-hold management, global diagnostics, platform-wide chaos, unified
UI redesign, authenticated browser/provider/load acceptance and production
deployment/export/purge/restore remain Phase H/I/K/L boundaries.
