# Enterprise Admin Diagnostics and Operations

Phase H extends the existing Global Admin Workspace page. Its scope is
administrator diagnostics and narrowly controlled operations; it does not
grant tenant membership or financial repair authority. The architecture and
query contract are in `ENTERPRISE_ADMIN_DIAGNOSTICS_ARCHITECTURE.md`.

## Authorization and API contract

The administrator router retains its existing authentication, panel rate limit,
compliance and HTTP audit controls. The repository independently checks a live
active nondeleted Global Admin for each entry point. A Workspace Owner/Admin
does not acquire global access. Diagnostic reads do not bootstrap membership,
impersonate the tenant or populate tenant permissions.

| Route under `/api/v1/admin` | Purpose |
| --- | --- |
| `GET /workspaces` | Validated server search, stable bounded pagination |
| `GET /workspaces/:id` | Existing safe Workspace inspection |
| `GET /workspaces/diagnostics/overview` | System inventory, jobs, identity/findings and failure evidence |
| `GET /workspaces/:id/diagnostics` | Structured scoped health, obligations, lifecycle and safe audit |
| `GET /operations/jobs` | Nine worker-family evidence summaries |
| `GET /operations/health` | Operations health and read-only export-key rotation risks |
| `GET /operations/metrics` | Authenticated fixed-label Phase H Prometheus scrape |
| `PATCH /workspaces/:id/status` | Guarded suspend/resume with explicit preconditions |
| `POST /workspaces/:id/webhooks/:webhook_id/deliveries/:delivery_id/retry` | Guarded dead-delivery requeue via existing worker |

Search uses exact positive Workspace/Owner/Billing Owner IDs, escaped name/slug
prefixes, allowlisted status/type/sort/direction, and ordered RFC3339 created/
updated windows. The default page size is20; maximum100 and maximum offset10000.
Prefix length is at most100 runes and paired date ranges at most366 days. Invalid
filters reject rather than silently widen. Owners are shown as IDs, not emails.

Diagnostic responses carry `Cache-Control: no-store`. Secret/token values and
hashes, SAML/OAuth/TOTP/Webhook cryptographic material, arbitrary stored errors,
provider payloads, audit metadata, object-storage keys/URLs and decrypted export
contents are excluded. Errors use stable safe classifications.

The list envelope retains `items,total,page,page_size,pages` (minimum one page),
with additive `total_capped` and `observed_at`. Internal `total_pages` is not a
public field. Each query parameter occurs at most once; raw query size is at
most4KiB. The shared frontend's valid IANA `timezone` metadata is accepted up to
64 bytes and ignored for the RFC3339 timestamp semantics. Other unknown query
fields reject. Diagnostic requests have a ten-second deadline and a fixed
four-request per-instance concurrency budget; a saturated budget returns429
`ADMIN_DIAGNOSTICS_BUSY` with `Retry-After: 1`.

Mutation bodies are at most16KiB, one UTF-8 JSON object with no unknown fields.
Status PATCH retains the Workspace response and adds
`X-Admin-Operation-Receipt-ID`; Webhook retry returns the safe immutable receipt.
If a committed status operation cannot subsequently inspect its result, HTTP502
`ADMIN_OPERATION_RESULT_UNAVAILABLE` includes the receipt ID and
`mutation_committed=true` metadata. Preserve that identity and original UUID;
the response is not evidence that the transaction rolled back.

| Safe error code | Meaning / response |
| --- | --- |
| `UNAUTHORIZED` / `WORKSPACE_FORBIDDEN` | Missing session or denied Global Admin/proof boundary; clear sensitive UI state |
| `WORKSPACE_NOT_FOUND` | Canonical target ID is not available to the authorized read |
| `WORKSPACE_INVALID` / `ADMIN_OPERATION_BODY_TOO_LARGE` | Invalid bounded input or body size; correct input before submitting |
| `WORKSPACE_CONFLICT` / `ADMIN_OPERATION_IDEMPOTENCY_CONFLICT` | Changed state/precondition or changed request under the same UUID; inspect current evidence |
| `STEP_UP_REQUIRED` / `RECENT_AUTH_REQUIRED` / `MFA_REQUIRED` | Complete the existing session-bound proof or sign-in workflow; enrollment and refresh alone do not satisfy it |
| `ADMIN_DIAGNOSTICS_BUSY` / `ADMIN_DIAGNOSTICS_TIMEOUT` | Capacity or request deadline exceeded; an operation timeout may have an ambiguous outcome |
| `ADMIN_DIAGNOSTICS_UNAVAILABLE` / `ADMIN_OPERATION_FAILED` | Fixed safe failure; no SQL/provider text is exposed |
| `ADMIN_OPERATION_RESULT_UNAVAILABLE` | Transaction completed, but follow-up inspection failed; retain the receipt and original request identity |

Sensitive list/inspect/diagnostic/metrics GETs retain HTTP actor/route/action/
status/timing auditing. The two H mutation templates omit raw request bodies
even when input is rejected; bounded reason and scalar action evidence belong
to the atomic Workspace business audit. HTTP auditing stores neither response
payloads nor query strings and is not the transactional operation evidence.

## Evidence, freshness and count semantics

Each section includes observation time, source, availability and coverage.
Matching-row counts inspect at most10001 rows. A capped value10001 means
**at least10001**, not an exact total. Indirect tenant queries can instead cap
an indexed source sample before classifying it; their coverage is explicitly
partial and any reported capped value is a lower bound. Zero matches in an
overflowing source sample do not prove absence. Only a successfully queried
complete empty source may report an exact zero. Unavailable evidence is
null/unknown.

Healthy/degraded/blocked/unknown describes the available evidence, with
freshness separately marked. A current queue observation does not establish a
live worker process. A last successful job is distinct from the last successful
scan. Stale/future/missing scan timestamps never become a healthy heartbeat.
Independent category failures preserve successful categories and expose a
fixed unknown diagnostic. Every issue has a stable code, severity, scope,
observation time, safe reason and recommended action/runbook.

Counts are evidence categories rather than a universally disjoint partition.
Running work can also have a live/expired lease; retry evidence can overlap
pending work; Webhook failed/dead both describe the retained terminal state.
Use each family's source/coverage instead of summing these categories.

| Worker family | Evidence and limits |
| --- | --- |
| Domain Event Outbox | Undelivered inventory, live/expired lease and due lag; retry failures may overlap pending inventory |
| Notification Dispatcher | Shares Outbox evidence; inbox unread count is not processing backlog; independent heartbeat unavailable |
| Webhook Delivery | Pending/delivering/retrying/dead states, next-attempt/finished timestamps and endpoint eligibility; successful jobs do not prove process liveness |
| FinOps Anomaly Detector | Persisted singleton scan/bucket/lag and materialized lease evidence; undiscovered rollup workload is not counted as zero |
| Export Worker | Durable state/lease/progress/completion; completed objects can expire without representing an export failure |
| Purge Worker | Cooling/due/blocked state, checkpoint and lease; retention/settlement blockers remain authoritative |
| SCIM Monitoring | Connector/client sync and token expiry inventory; no persisted expiry-monitor heartbeat or run history |
| Billing Recovery | Pending/recovered settlement-alert evidence; alerts are not a complete charge/recovery queue |
| Async Media Settlement | Scoped SQL batch-image evidence; Redis-only video task/terminal history unavailable without a bounded authoritative source |

Recent failure trend covers the past24 hours of Webhook terminal failures using
their real `finished_at`, grouped into UTC-hour buckets with bounded counts.
Sources without a failure-occurrence timestamp are not added using `updated_at`
as a proxy. Retention can limit visible historical success/failure evidence.

Workspace diagnostics additionally cover status/security-policy summary,
members/projects/machine-identity state, active Billing Owner, pending budget
reservations and unsettled alerts, accepted batch images, findings, identity,
Webhook/lifecycle jobs, retention holds and up to20 audit summaries. Financial
amounts and tenant business payloads are unnecessary to this control plane.

## Safe operations

Supported status transitions are active -> suspended and suspended -> active.
Resume rechecks active Billing Owner membership and a valid active Owner.
Archived/pending-deletion/purging/deleted states cannot be reopened here; revoked
credentials and child scopes are never automatically restored.

Mutation requests require a human Global Admin, mandatory recent authentication,
real MFA proof when enrolled, a targeted confirmation, nonempty reason<=500
runes, canonical nonzero UUID idempotency key and current server precondition.
The reason is included in the Workspace business audit and visible to tenant
users with `audit.read`; keep it factual and do not include secrets or personal
data.
The original-login window is ten minutes; the reused session-bound TOTP step-up
grant has the existing fifteen-minute TTL. Token refresh and MFA enrollment are
not proof. Missing guard/configuration, machine/admin API-key access and stale
versions fail closed regardless of the optional global step-up switch.

Confirmations are `suspend:<workspaceID>`, `resume:<workspaceID>` and
`retry_webhook:<deliveryID>`. Keep the exact `updated_at` returned by the server
for the Workspace precondition; do not truncate its precision. Webhook retries
also compare attempts and last-attempt time. The final request/receipt field
contract is defined by the typed API binding.

One transaction locks actor then Workspace, verifies current role/enrollment
and preconditions, applies the conditional mutation and appends the immutable
receipt, workspace audit and domain event/outbox. An identical actor/key replays
its original receipt; changed input with that key conflicts. Receipts are
synchronous outcomes rather than scheduled jobs. Authentication caches are
invalidated only after successful transaction completion.

Only a scoped dead Webhook delivery with an active Workspace, enabled endpoint,
no active claim and matching attempt token is eligible. At most three successful
operator retries are permitted per delivery. Requeue preserves its original
event/IDs/payload; the existing worker performs the delivery. At-least-once HTTP
delivery remains the established contract, so receivers must deduplicate IDs.

The administrator retry records its own `webhook.administrator_retried` event,
with only previous/result status and the delivery ID. Existing `workspace.updated`
subscriptions retain their original meaning. Retry evidence enters durable
audit/outbox and only explicitly subscribed Webhooks; it does not create default
member inbox notifications. Additive migration304 validates this exact safe
event shape without changing released event validation or subscription rows.

Other jobs provide diagnostics/runbooks only. There is no force purge, floor or
hold override, manual balance reset, direct settlement/release, immutable-trigger
switch, pending Outbox deletion, credential restoration or generic SQL console.

## Export encryption key rotation: diagnosis only

The current export format uses one operator key and no per-object key ID or
old-key lookup. Phase H shows current-instance export/purge/key availability,
bounded active exports, retained object-ledger inventory and holds. This is
partial readiness evidence. It does not verify complete S3 inventory, key
custody/backups, cross-instance key agreement or recovery drills.

Do not replace the active key while retained/held/orphan ciphertext still needs
it. Normal download expiry does not prove ciphertext cleanup; indefinite
retention and holds can extend object life. Stop accepting/exporting before any
approved drain and verify every instance is quiescent. Never erase protected
holds or lower floors merely to satisfy a drain condition.

Before Phase I, approve the actual drain-or-key-ring design, key version/custody,
multi-instance deployment, rollback and recovery runbook. Before Phase L,
execute real-storage old/new-key readability and recovery drills, including
retained/held/orphan objects, interrupted exports, mismatched/lost keys and
rollback. The release gate remains blocked until evidence exists. Phase H has
no rotation endpoint, automatic rotation or diagnostic decryption probe.

## Operator runbook and performance boundary

1. Check source observation time, coverage/capping and freshness before using a
   number. Refresh partial/unknown evidence; a missing heartbeat is an unknown
   liveness fact, not a proven worker outage.
2. Inspect a selected Workspace's obligation and blocker codes before acting.
   Resolve billing/identity/retention through their owning subsystem. A
   portability export is not an input to the existing database backup/restore.
3. For suspend/resume, refresh the exact version, explain the action and confirm
   its target. Complete sign-in/MFA if requested. On conflict refresh and review
   the changed state; do not replay with a new key blindly.
4. For dead Webhook delivery, repair the endpoint/cause and verify receiver
   deduplication before requesting a permitted retry. Preserve the idempotency
   key if the first response is lost. Do not retry a live/succeeded delivery.
5. For blocked purge or export failures, follow `ENTERPRISE_DATA_LIFECYCLE.md`.
   Configuration switches require the existing operator restart procedure;
   diagnostic controls do not toggle an unsupported worker pause state.
6. Review the immutable receipt/audit trail and resulting diagnostic state.
   An HTTP timeout is not evidence that a server mutation rolled back.

Diagnostic SQL has context/statement/lock bounds, indexed capped reads and no
unbounded historical usage aggregation. Concurrent indexes still need IO/WAL/
disk headroom and production rehearsal. Prometheus labels are fixed endpoint,
worker,state,action,outcome values; tenant/user/key/model/email IDs are excluded.
Diagnostic observations are collected outside Gateway admission and settlement.
The dedicated registry includes observation timestamps/availability because
on-demand gauges do not establish current evidence without a recent observation.
Automated/local results and remaining real-provider/storage/browser/production
gates are recorded in `ENTERPRISE_ADMIN_DIAGNOSTICS_ACCEPTANCE.md`.

Migration304's validated event-protocol CHECK scans existing `domain_events`
and needs a transactional table-lock window. Its idempotent PostgreSQL fixture
replay verifies correctness; production heap size, validation/lock duration and
timeout/retry rollout must be rehearsed in Phase I alongside migration303.

The current migration303 candidate contains55 concurrent indexes after an
individual existing-index reuse audit. Its fixed invalid-index recovery
allowlist contains the same55 names. Scoped Webhook success uses a matching
workspace/completion partial index. Nonterminal Webhook classification first
acquires at most10001 delivery sources, deduplicates endpoint keys and shares
the acquisition across its counts and coverage. The global path performs up
to10001 one-row index seeks; scoped paths can perform10003 searches including
empty status branches. Overflowed filtered zero counts remain lower bounds.

Real PostgreSQL fresh-heap EXPLAIN fixtures retain FKs/triggers and use ANALYZE
without VACUUM or planner overrides. Representative combined inventory plans
inspect10001 delivery sources and at most10001 endpoint rows, with local warm
execution32.520–57.161ms and30150–60152 shared-buffer hits. These measurements
do not establish a cold-IO or production SLA. Phase I must rehearse index build
IO/WAL/disk/metadata locks, failed-build restart, cold/fragmented estates and
dead-version maintenance. Request deadlines and sanitized unknown evidence
remain authoritative when a source cannot complete within its bound.

The Phase H read collectors use the following metric contract. The prefix is
`modurelay_enterprise_admin_`; no instance of a tenant, user, credential, model or
email is a label value.

| Metric suffix | Fixed labels | Interpretation |
| --- | --- | --- |
| `diagnostics_duration_seconds` | endpoint: search/overview/workspace/jobs | Histogram of bounded diagnostic service-call duration, including failures |
| `worker_backlog` | worker/state allowlists | Last exact fresh count; unavailable, stale or capped evidence is NaN |
| `worker_inventory_available` | Same worker/state | 1 only when the corresponding count is available, fresh and exact |
| `worker_due_lag_seconds` | Nine worker families | Due-work scheduling lag; unsupported or unavailable evidence is NaN |
| `worker_observed_timestamp_seconds` | Nine worker families | SQL snapshot time; a scrape or job completion is not a process heartbeat |
| `billing_pending_alert_age_seconds` | None | Age of the oldest pending alert; alert age is distinct from worker due lag |
| `operation_total` | action: suspend/resume/retry_webhook; outcome: success/error | Guarded operation calls, including idempotent receipt replay; these are not unique committed transition counts |

Use inventory availability and observation age together before alerting on an
exact backlog. A successful scrape alone does not certify source completeness
or liveness. Workspace-detail reads do not overwrite the global worker gauges;
only global overview/jobs observations update them. Notification shares Outbox
evidence, so their values must not be summed as independent queues.

The incident procedures and key-rotation release checklist are in
[the administrator runbook](ENTERPRISE_ADMIN_DIAGNOSTICS_RUNBOOK.md).

## Migration303 individual index and reuse audit

All names below omit the `enterprise_admin_` prefix. This reviewed set has55
concurrent DDL names and the same55 fixed invalid-index recovery names. Each
row identifies the query it supports and why an existing index is insufficient.
Actual selected-query plans are verified by the PostgreSQL integration tests;
production-size index IO/WAL/storage and cold-plan behavior remain Phase I work.

| New index | Actual query or order requiring it | Existing index reused or overlap retained |
| --- | --- | --- |
| workspace_name_prefix | escaped `lower(name)` literal prefix | No existing expression/pattern index |
| workspace_slug_prefix | escaped `lower(slug)` literal prefix | Raw unique slug remains usable for slug ordering |
| workspace_name_order | list `ORDER BY name,id` | Lowercase pattern order cannot serve raw name order |
| workspace_status | sparse exact status and ID pagination | Workspace PK cannot bound empty sparse status |
| workspace_type | exact type and ID pagination | PK/personal-owner partial does not cover all organizations |
| workspace_owner | exact owner with ID pagination | Existing personal-owner unique index covers personal rows only |
| workspace_created | created range and `ORDER BY created_at,id` | PK order is unrelated to created time |
| workspace_updated | updated range and `ORDER BY updated_at,id` | PK order is unrelated to update time |
| export_state_created | global state counts, sparse failed/terminal subsets | Existing one_active/scope indexes reused for active scoped probes |
| export_attention | latest global pending/running/failed by created/id | State-first index cannot directly merge all states in creation order |
| export_success | global latest completed/expired completion time | Existing due index orders scheduled work, not completion |
| export_claims | global running null/live/expired leases | State index alone could scan all running rows for empty lease subsets |
| export_pending_created | global oldest pending/running creation | Existing due index orders available_at rather than created_at |
| export_errors_global | global retained error code ordered by updated_at | Scoped history uses existing export scope index before cap/filter |
| purge_state_created | global blocked/failed/terminal state counts | Existing one_active reused for active scoped probes |
| purge_attention | latest global pending/running/failed/blocked by creation | State-first index cannot directly merge all four states |
| purge_success | global latest completed completion time | Cancelled completion clocks are deliberately excluded |
| purge_claims | global running null/live/expired leases | State index alone does not bound empty expired subset |
| purge_pending_created | global oldest pending/running/blocked creation | Cooling jobs stay pending without being due |
| purge_errors_global | global retained nonempty failure code by update time | Scoped history uses existing deletion scope index and cap |
| purge_eligibility | sparse time eligibility `GREATEST(available_at,earliest_purge_at)<=now()` | Existing due available_at cannot bound all cooling exclusions; source order uses same expression |
| purge_cooling | pending `GREATEST(earliest_purge_at,available_at)>now()` | Pending-only partial avoids scanning blocked/running future rows |
| webhook_scope_created | newest scoped deliveries by created/id | Existing scope index has webhook_id between workspace and creation |
| webhook_claims | global delivering live/expired locked_at | Existing due next_attempt index cannot bound empty lease subsets |
| webhook_pending_created | global oldest nonterminal creation and bounded creation/workspace/id source seeks; INCLUDE(status,webhook_id) covers classification | Existing due order is scheduling time, not creation; scoped source seeks reuse lifecycle workspace/status/id |
| webhook_dead_scope | independent scoped latest dead finished_at/id | Existing finished index starts status then time, so foreign dead history can dominate scope filter |
| webhook_scope_due | scoped first10001 time-eligible sources by next_attempt/created | Existing global due used for global source; lifecycle workspace/status reused for state inventories |
| webhook_success_scope | exact scoped latest succeeded finished_at/id, including empty tenant beside foreign successes | Existing finished status/time order can inspect foreign history; scoped creation order cannot prove latest completion |
| image_attention_global | global sparse pending OR uncertain submission, oldest creation | Existing status index cannot cover OR error arm; scoped index has workspace leading |
| image_attention_scope | scoped same OR predicate and oldest creation | Existing lifecycle workspace/status lacks uncertain-submission OR arm |
| image_errors_global | global latest retained SQL image error by updated_at | Scoped error probe uses existing workspace-created source cap |
| image_success_global | global latest completed image finished_at | Scoped success uses existing workspace-created source cap |
| outbox_claims | global undelivered locked_until live/expired range | Existing due available_at does not support sparse claim subset |
| outbox_retry | global undelivered nonnull/nonempty error and latest retry scheduling time | Explicit null predicate fixes reproduced COALESCE cardinality misestimate |
| outbox_pending_created | global oldest undelivered creation | Existing due is available_at; scoped domain_events creation plus lateral outbox PK reused |
| finops_pending_scope | scoped incomplete sources and oldest bucket; bounded lease/error samples | Existing due(completed_at,claimed_until,...) reused for global incomplete/claims/completed ordering |
| finops_errors_global | global retained incomplete error by updated_at | Scoped error/claims use capped incomplete source; success uses capped scoped PK source |
| scim_expiry | global active expiring token inventory | Scoped tokens use existing unique scope source before expiry filtering |
| scim_errors_global | global retained connector error by updated_at | Scoped connectors use bounded unique(workspace_id,id) source; no extra scoped error index |
| members_scope_state | sparse active/suspended within workspace | Existing workspace/user uniqueness lacks status leading within scope |
| members_state | sparse global member state | Workspace-leading index cannot bound global empty status |
| projects_scope_state | sparse active/archived within workspace | Existing one_default is a different partial subset |
| projects_state | global project state inventory | Workspace-leading state index cannot bound global empty status |
| service_accounts_scope_state | sparse active/disabled within workspace | Existing workspace/project index has project before status |
| service_accounts_state | global machine identity state inventory | Scoped index cannot bound global empty status |
| identity_state | global active/disabled provider inventory | Existing provider workspace/status reused for scoped inventory/health sample |
| scim_state | global active/disabled connector inventory | Scoped unique/status index cannot bound global empty status |
| scim_scope_state | sparse scoped connector state | Existing unique workspace/id reused for health/sample; status filter otherwise residual |
| billing_recovered | global latest recovered alert recovered_at | Existing PK reservation_id does not order recovery time |
| billing_state | global pending/recovered count and pending_at age | Scoped reservation source plus lateral alert PK is reused with source-cap qualification |
| budget_pending_scope | sparse workspace pending budget reservations | Existing workspace/project/status has project before status |
| budget_reserved | workspace/project nonzero reserved counter subset | Existing counter PK(scope_type,scope_id,period) reused; partial avoids all settled periods; project sources capped |
| findings_scope_state | scoped open/acknowledged/resolved findings | Existing scoped findings index has project before status; existing status index used globally |
| objects_scope | capped scoped export object ledger | Existing cleanup index orders cleanup_after, not workspace |
| active_holds | capped global active holds | Scoped hold PK reused; partial avoids historical inactive holds globally |
