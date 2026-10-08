# FinOps anomaly detection

Phase E adds explainable FinOps anomaly findings for Enterprise Workspaces. The
detector reads the existing immutable hourly usage projections and writes an
immutable evidence snapshot plus a workflow finding. It does not rewrite usage,
billing attribution, reservations, or settled costs.

## Architecture gate

The source of truth is the request-time tenant attribution already projected into
`usage_tenant_hourly_rollups` and the service-account projection in
`usage_service_account_hourly_rollups`. Those projections retain the workspace,
project, API key, platform, model, and service-account dimensions captured when
the request was billed. `usage_logs`, budget reservations, billing principals,
and historical attribution remain outside the detector write path.

Each run chooses one completed UTC hour. A five-minute grace period lets late
rollup writes arrive before evaluation. The detector never treats the active
hour as final. A durable cursor advances one hour at a time, while a workspace,
bucket, and detector-version lease bounds parallel work across ModuRelay
instances. Lease tokens prevent an expired worker from completing a newer
worker's lease.

The recurring query reads only the two hourly rollup tables. It discovers at
most 25 workspaces per batch, evaluates at most 100 current-hour candidates per
workspace, and limits the grouped baseline query to 10,000 rollup rows. A
20-second worker context bounds a scan. There is no recurring `usage_logs`
`GROUP BY` or all-dimensions cross product.

The baseline uses up to 28 days of the same UTC hour-of-day. Median and median
absolute deviation (MAD) provide a robust, deterministic expectation. At least
12 historical samples are required. A candidate is rejected when its values or
baseline contain NaN/Infinity, the bucket is incomplete/future, the deviation
score is below the configured threshold, or its absolute and relative floors are
not met.

The detector types are:

* `spend_spike`: current spend versus the spend baseline. The observed spend and
  delta must each be at least `$0.01`, and the relative increase must be at least
  100%.
* `request_spike`: request count versus the request baseline. At least 20
  observed requests, 20 additional requests, and a 100% relative increase are
  required.
* `unit_cost_spike`: spend/request versus the unit-cost baseline. At least 20
  observed requests, a `$0.0001` observed and absolute delta floor, and a 100%
  relative increase are required.

The minimum robust score is 3.5. Severity is derived from score, absolute delta,
relative increase, and sample confidence; the thresholds are centralized in
`DefaultFinOpsAnomalyConfig`. Candidate ordering is spend descending, then
request count, dimension type, and value. This makes the hard cap deterministic
and prevents high-cardinality workspaces from turning into notification storms.

## Storage and idempotency

Migration `298_finops_anomalies.sql` adds:

* `finops_anomaly_detection_leases` for the durable cursor/lease state;
* `finops_anomaly_snapshots` for append-only detector evidence;
* `finops_anomaly_findings` for `open`, `acknowledged`, and `resolved` workflow
  state; and
* `finops_anomaly_detector_status` for low-cardinality operational status.

Snapshots carry the detector version, UTC window, baseline window/statistics,
observed and expected metrics, score, severity, dimension, tenant scope, and a
SHA-256 fingerprint. The snapshot trigger rejects updates and deletes. The
finding trigger keeps evidence columns immutable and enforces monotonic status
transitions. A unique fingerprint on both snapshot and finding tables is the
database-level retry/idempotency boundary. The detector inserts the snapshot,
finding, and `finops.anomaly.detected` event in one transaction.

Projects use a composite workspace/project foreign key. API-key and
service-account dimensions retain the project captured in the hourly rollup;
credential secrets are never copied into snapshots, findings, events, or
notifications. Archived resources remain explainable through their immutable
dimension value and project snapshot.

## API and permissions

Workspace endpoints:

```text
GET   /api/v1/workspaces/:id/finops/anomalies
GET   /api/v1/workspaces/:id/finops/anomalies/:anomaly_id
PATCH /api/v1/workspaces/:id/finops/anomalies/:anomaly_id
GET   /api/v1/workspaces/:id/finops/anomalies/status
```

Project-scoped list, get, and patch endpoints are available under
`/workspaces/:id/projects/:project_id/finops/anomalies`. Filters are bounded and
validated (`status`, `severity`, detector, dimension, UTC start/end, page, and
page size), with deterministic `last_detected_at DESC, id DESC` ordering.

`finops_anomaly.read` and `finops_anomaly.manage` are registered in the central
WorkspaceAccessService. Workspace owner/admin/billing roles can manage findings;
developer/viewer roles can read according to the existing usage/project grant
rules. Assigned-project members use the project route and are rechecked against
the live project grant. Every query binds the URL workspace and project IDs in
SQL, so a foreign numeric ID returns the same bounded not-found response.
PATCH requires the current finding version. Acknowledge and resolve therefore
return a conflict on a stale concurrent update; resolving requires a bounded
reason.

## Events and notifications

The detector reuses the existing domain-event, outbox, notification, and webhook
pipeline:

```text
finops.anomaly.detected
finops.anomaly.acknowledged
finops.anomaly.resolved
```

Payloads contain only bounded scalar evidence. Domain-event dedupe keys include
the fingerprint for detection and finding/version for workflow changes. Inbox
recipients are limited to active Workspace owner, admin, and billing members.
No developer/viewer notification fan-out is added.

## Worker and operations

`ProvideWorkspaceService` starts one process-wide worker and
`StopBootstrapWorker` stops it during server shutdown. The worker performs an
initial bounded scan, polls every five minutes, uses context cancellation, and
waits for its goroutine before returning. Database leases provide multi-instance
coordination; a crash leaves an expiring lease that another instance can claim.

The detector status endpoint reports last successful scan, last processed bucket,
lag, failure code, candidate count, finding count, and scan duration. These
fields are low-cardinality status values; workspace, model, API-key, and
service-account IDs are not metric labels.

## UI

`WorkspaceFinopsView.vue` displays open/high-critical summary values, detector
health, bounded filters, a horizontally scrollable finding table, immutable
evidence detail, explanation text, and acknowledge/resolve actions. The page
uses existing Frosted Precision semantic tokens, preserves the server-reported
values (including sub-cent amounts), guards workspace/detail request races, and
provides English and Simplified Chinese strings.

## Risks and deferred scope

Late usage can change a future rollup after a snapshot is recorded; the snapshot
is intentionally historical evidence and is never silently recomputed. The
hard candidate cap can omit a low-spend dimension in an unusually high-cardinality
hour; the status counters and bounded query limits make that tradeoff visible.

Phase F cost centers, tags, environments, department/ERP allocation, and any
historical usage rewrite remain deferred. AI/LLM remediation, automatic key
blocking, budget changes, model switching, and automatic finding resolution are
outside Phase E. Real-provider, production-load, browser/manual, and deployment
acceptance remain separate release gates.
