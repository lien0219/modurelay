# Enterprise administrator diagnostic runbook

Use the Global Admin Workspace console and its dedicated administrator API.
Global Admin access does not grant Workspace membership or permission to modify
tenant billing, protected evidence, retention floors or credentials.

## Establish the evidence boundary

1. Check the observation time, availability, freshness, source and coverage.
   Refresh stale evidence. Unavailable counts are unknown; capped10001 counts
   mean at least10001. A current inventory and a last successful job are not a
   process heartbeat.
2. Select one Workspace and review its current status, security policy,
   billing-owner eligibility, reservations, settlement obligations, accepted
   async tasks, retention/holds and safe recent audit summaries.
3. Read the Worker-specific coverage before interpreting lag or errors.
   Notification shares Outbox evidence. FinOps counts only materialized work.
   SCIM client synchronization is distinct from monitor execution. Billing
   alerts and SQL image jobs do not prove complete video settlement coverage.
4. Resolve source outages through their owning subsystem. Keep SQL errors,
   provider responses, tokens, key material and decrypted export data out of
   diagnostic screenshots and incident notes.

## Suspend or resume a Workspace

Refresh the Workspace before choosing the action. Supported transitions are
active to suspended and suspended to active. Enter a bounded reason and the
targeted confirmation displayed by the console. Complete required recent
authentication and enrolled MFA proof. Enrollment alone is insufficient.

The server checks the exact Workspace version and records the receipt, audit
and domain event/outbox atomically. Resume also requires an eligible active
Billing Owner and Owner. Other lifecycle states cannot be reopened here; child
credentials are not restored by this operation.

The required operation reason is part of the Workspace business audit and is
visible to tenant users with the existing `audit.read` permission. Keep it
factual and bounded; do not include credentials, secrets or personal data.

Preserve the idempotency UUID if the response is lost. An HTTP timeout does not
prove rollback. An identical request with that UUID can replay its receipt;
changed input conflicts. Refresh after a conflict and review the new evidence
before authorizing another action.

## Retry a dead Webhook delivery

Repair the endpoint or known delivery cause and confirm that the receiver
deduplicates event/delivery identities. Refresh the selected Workspace. Only
an eligible dead delivery in an active Workspace with an enabled endpoint and
no claim can be retried. The server compares the attempt evidence and limits
operator retries to three for that delivery.

Use the targeted confirmation, reason, recent proof and idempotency UUID.
The existing Worker delivers the unchanged envelope. At-least-once delivery
remains possible. Pending, retrying, delivering or succeeded deliveries cannot
be manually replayed. Disabled endpoints are not automatically reenabled.

The administrator operation emits `webhook.administrator_retried` as separate
scalar evidence. Only endpoints explicitly subscribed to that event receive
it; the requeued delivery retains its original event envelope and identity.

## Other jobs and retention blockers

Follow [the lifecycle runbook](ENTERPRISE_DATA_LIFECYCLE.md) for export or
deletion evidence, [FinOps anomaly documentation](FINOPS_ANOMALY_DETECTION.md)
for detector evidence, and existing identity/billing runbooks for their
respective obligations. Phase H offers diagnosis for these systems. It does
not reset leases, retry exports/purges/settlement, toggle unsupported Worker
pause states, delete Outbox evidence or override holds/floors.

Resolve a blocker through its authoritative business workflow. Never lower a
protected retention floor, release a reservation without settlement evidence,
modify historical allocation or disable immutable evidence triggers to clear
a diagnostic warning.

## Export encryption key rotation readiness

Phase H is read-only. `MRLEX01` identifies the artifact format; it is not a
key ID. The format uses one operator-provided32-byte key without a key ring or
old-key lookup. Replacing it can make retained ciphertext unreadable.

| Evidence | What it establishes | Remaining condition |
| --- | --- | --- |
| Current-instance key availability | This instance loaded a valid key | Custody, backup, other instances and the ability to decrypt older objects remain unverified |
| Export/purge capability flags | Current-instance configuration | Deployment-wide quiescence and storage completeness remain unverified |
| Pending/running exports | Bounded SQL job inventory | Drain every instance and account for interrupted uploads and outstanding leases |
| Retained/indefinite object ledger | Bounded tracked-object inventory | Verify real private storage, retained/held objects, orphan ciphertext and actual cleanup |
| Active holds | Bounded current hold inventory | Preserve hold and protected retention semantics throughout any approved procedure |

Zero inventories, expired download URLs or a loaded key cannot certify safe
rotation. The diagnostic therefore reports blocked/unknown readiness and no
rotation approval. It never fetches/decrypts an artifact as a key probe and
never displays a key, fingerprint or storage path.

**Before entering Phase I**, record and approve a concrete drain or versioned
key-ring design, key custody and backups, all-instance rollout, rollback,
verification and recovery procedure. A drain must stop new exports, finish or
safely fence in-flight work and prove that every ciphertext requiring the old
key is retained with that key or has been safely migrated/removed. Hold and
financial protections remain authoritative. A key-ring approach needs its own
versioned artifact/schema design and migration before production rotation.

**Before Phase L acceptance**, execute old/new-key readability and restoration
drills against real private storage. Include retained and held objects, orphan
uploads, interrupted exports, multi-instance mismatch, missing old keys and
rollback. Preserve drill evidence and custodial recovery access. Local crypto
unit tests and Phase H inventory observations cannot close this release gate.

## Operational limits

Diagnostic requests use bounded SQL/context/lock timeouts outside Gateway.
Prometheus labels are fixed endpoint/worker/state/action/outcome values. Use
the observation timestamp and availability gauges when interpreting the
on-demand metrics. Never add tenant/user/key/model/email labels.

Concurrent-index deployment and rollback recovery need production-scale
rehearsal. Migration304 also validates existing event rows under an ALTER-table
lock; size the maintenance window and rehearse bounded timeout/retry before
production rollout. The [acceptance record](ENTERPRISE_ADMIN_DIAGNOSTICS_ACCEPTANCE.md)
separates executed local gates from real-storage/provider/browser/load and
production checks. Port18081 is outside this phase's authorized runtime work.
