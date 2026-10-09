# Phase I production hardening architecture

Date: 2026-10-10. Branch: `feature/new-feature`. Starting source:
`2e24b086d04c52fb4174e8f995a253bfe62e4989`. Remote branch matched this SHA;
the starting worktree was clean. Phase H is COMPLETE at `bf4f1c79e`; historical
migrations end at 304. Phase I neither operates port18081 nor activates exports,
purge, new-format writes or production key rotation.

## Architecture findings and gate decision

**APPROVED FOR LOCAL IMPLEMENTATION AND DISPOSABLE REHEARSALS.** Production
release is BLOCKED until the separately documented Phase L manual/external gates.
The supplied Phase I specification authorizes implementation without another
design approval. Existing Phase A–H contracts govern unchanged behavior.

| Priority | Category | Existing protection | Source-backed gap | Remediation | Required evidence |
| --- | --- | --- | --- | --- | --- |
| P0 | Export key compatibility | `lifecycle_artifact.go` authenticates bounded chunks and a terminal record with random salt and object scope; downloads verify checksum before returning | `MRLEX01` has no Key ID; `lifecycle_runtime.go` resolves one `encryption_key`, so replacing it loses retained/held artifact readability | Preserve exact V1 reads and writes; explicit V2 Key ID and authenticated header; active/decrypt-only key registry; no guessing or key destruction | Frozen V1 fixture, mixed versions, wrong/lost/unknown keys, header/tag/truncation, zero output on authentication failure, isolated S3 recovery |
| P0 | Multi-instance key rollout | Exports and purge default off; durable object-attempt ledger | Configuration validation is local only; old binaries cannot read V2; queue emptiness cannot prove safe retirement | V2 writes default off; explicit reader inventory and compatible-rollback declaration; SQL reader heartbeats and exact key/metadata fingerprint consensus checked before every V2 claim | Two readers, missing/stale/mismatched readers, interruption and compatible rollback |
| P1 | Financial integrity | `usage_billing_repo.go` settles money, usage, reservation and events in one deduplicated transaction; immutable allocation triggers; preserved failed holds | `gateway_usage_billing.go:1205` and equivalent OpenAI fallback may insert zero-cost tenant usage after billing failure, freezing wrong allocation evidence and conflicting with later correct retry | Tenant failures never create false zero-cost usage; freeze recoverable actual command/evidence in durable SQL inventory and replay the same transaction | Inject billing/usage/outbox failures; verify no changed money or false snapshot before retry; one effective debit after replay; allocation equality |
| P1 | Async recovery | Frozen payer/pricing/reservation attribution; safe unknown-outcome hold preservation; existing image/video recovery | Redis-only video pending/account binding and image task state can disappear after process/Redis loss; accepted upstream creation may precede persistence | Durable pre-send attempt inventory and SQL-authoritative recovery/cache projection; unknown creation never automatically resubmitted or refunded | Crash at admission/provider-start/accepted/settlement; Redis loss; archive/owner/config changes; preserved financial references |
| P1 | FinOps worker fencing | Unique fingerprints, immutable snapshots and transactional event/outbox; database claims | `workspace_finops_anomaly.go:282` completes without live lease/affected-row checks; `persistAnomalyDetection` has no claim token | Fence evidence/event and completion against the live token using database clock and locked claim; stale/expired worker returns conflict | Expired same-token completion, takeover, stale evidence, outbox rollback, replay and kill/restart |
| P1 | Policy authorization | Service central Workspace/Project checks, optimistic policy revision | `policy_repo.go:UpdatePolicy` does not independently recheck live role/grants/lifecycle/security; Service Account policy update missing from Project-scoped permissions | Reuse Workspace lock/current policy/RBAC and correct scoped permission; changes serialize with revoke/archive | Direct-repository IDOR and suspended/removed/viewer/grant cases; concurrent revoke/update |
| P1 | Migration and performance | Checksums, advisory lock, transactional migrations and `_notx.sql` concurrent-index recovery | 304 CHECK validation and 303 index IO/locks not measured; broad recovery/performance matrix absent | Unchanged history plus additive migrations; isolated16/18 history/replay/lock/invalid-index/backup restore; reproducible bounded workloads | Exact commands, SQL plans, lock/error/latency/resource evidence; no production-QPS claim |

Role, identity and financial boundaries remain separate. Global Admin does not
receive tenant membership. Human SSO/MFA uses the actual current Session and
original authentication age; direct API keys and machine credentials do not
inherit human MFA. Project management grants stay outside model-request hot
paths; runtime keys check their existing execution identity and live tenant
lifecycle. Suspended/removed users, revoked keys and closed parents fail closed.

## Key compatibility and custody

The selected scheme uses a key ring, rather than draining/deleting protected
objects to force rotation. V1 selects only the explicit legacy `encryption_key`;
V2 selects only its bounded header Key ID. A missing key, invalid configuration,
unsupported format or failed authentication yields no user plaintext. Every
artifact keeps independent salt/derived key, chunk counters and authenticated
Workspace/object scope. V2 additionally authenticates the entire header and
uses a separate derivation domain. Old format bytes and cryptographic semantics
remain unchanged.

Only operator-injected secret configuration supplies local key material. A typed
resolver boundary permits a trusted KMS adapter; actual cloud KMS is NOT RUN.
SQL stores instance identifiers, nonsecret fingerprints and reader liveness,
never key material. Logs, API JSON and ZIP manifests must not contain keys.
No automatic removal, re-encryption or retirement of keys is implemented.

Multi-instance promotion requires an operator-approved complete stable instance
inventory, all readers configured with the same historical and active keys,
and a rollback target that supports V1/V2. Database consensus verifies registered
readers; it cannot discover unregistered legacy binaries. Accurate inventory and
the compatible rollback declaration are therefore explicit release controls.
Readers deploy first with V1 writes. V2 writing is opt-in only after their live
registrations agree. Every V2 claim repeats this check; loss of consensus pauses
export creation, preserving queued jobs and objects. Reverting the write flag
to V1 does not remove V2 readers or historical keys.

## Migration risks and tenant isolation risks

New migrations start at305, only for durable state that existing tables cannot
represent. Do not edit released SQL/checksums, scan or rewrite historical usage,
disable constraints, delete protected rows or promise a lossless DOWN rollback.
Locks/timeouts and partial concurrent indexes require measured recovery on
disposable PostgreSQL16 and18.x. Rollback uses compatible binaries plus forward
fixes; database restore is separately verified and does not replace financial
reconciliation.

New recovery records use immutable trusted Workspace/Project/principal and
reservation identity, bounded sanitized data, durable states and token fencing.
Creation-time payer/pricing/allocation snapshots remain authoritative after
Owner or configuration changes. Unknown provider outcome stays held and visible;
absence from Redis is not evidence of no upstream work. Lifecycle preflight
includes unresolved durable obligations; cleanup cannot erase recovery evidence.
External webhooks remain at-least-once with stable Event ID, never exactly-once.
For asynchronous video, a SQL attempt marker is committed before provider
submission. Accepted completion atomically transitions that marker and writes or
refreshes the pending billing record from the marker snapshot; definitive 4xx
rejection is terminal, while stale started attempts become `unknown` without
resubmission or automatic refund.

## Implementation plan

See `ENTERPRISE_PRODUCTION_HARDENING_PLAN.md`. Implement compatibility first,
then evidenced policy/financial/worker repairs, isolated recovery and migration
rehearsals, final security/static/dependency and full regression gates, release
runbooks and the authorized local phase commit. COMPLETE requires observed gate
results; unavailable/incomplete evidence remains explicitly classified.
