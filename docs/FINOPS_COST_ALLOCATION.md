# Phase F FinOps cost allocation

Phase F adds tenant-scoped cost centers, bounded allocation tags, standard
environment dimensions, project defaults, and optional API-key and
Service-account overrides. Allocation is resolved at the trusted gateway
admission boundary after the existing Workspace, Project, credential, access,
policy, quota, and billing-principal checks. The resolved value is copied into
the budget reservation and then into immutable usage evidence.

## Architecture invariants

- A Workspace owns cost centers and controlled tags. A Project owns the
  default allocation policy. API-key and Service-account overrides are scoped
  to the same Project and Workspace.
- Resolution precedence is deterministic:
  **API-key override → Service-account override → Project default →
  explicit unallocated**.
- Every configured candidate is loaded with composite tenant predicates and
  validated before the selected precedence value is applied. Archived or
  malformed centers/tags, foreign IDs, and project-mismatched credentials fail
  closed.
- The client cannot choose billing or allocation by sending a cost-center,
  project, Workspace, tag, or credential ID. Admission derives all ownership
  from the authenticated credential and server-side joins.
- The reservation snapshot is written in the same transaction as the budget
  hold. A retry with the same request/key retains the original snapshot and
  rejects configuration drift; it never overwrites historical attribution.
- Usage allocation snapshots are append-only evidence. Database triggers reject
  updates and deletes. Configuration edits and archives therefore cannot
  rewrite completed usage.
- Legacy tenant-attributed usage without a Phase F snapshot is reported as
  explicit unallocated. Historical rows with no tenant scope remain outside
  Workspace reports; usage_logs is never batch-rewritten.
- Cost-center and environment rollups are low-cardinality hourly aggregates.
  Tag reports expand the immutable snapshot window and set
  overlapping_tags=true; tag groups are not summed into Workspace totals.

## Migration 299 storage

backend/migrations/299_finops_cost_allocation.sql is forward-only and does
not modify an earlier migration. It adds:

- workspace_cost_centers with stable per-Workspace codes, active/archived
  lifecycle, and archive guards;
- workspace_allocation_tags with normalized bounded keys/values, active/
  archived lifecycle, secret-like key rejection, and archive guards;
- project_cost_allocations and its controlled tag join table;
- API-key and Service-account override tables and their tag joins, each with
  composite Workspace/Project foreign keys;
- budget_reservation_allocation_snapshots, bound to the reservation and
  billing principal;
- usage_allocation_snapshots, bound one-to-one to usage_logs, with frozen
  cost-center code/name, environment, tags, source, revision, and actual cost;
- usage_allocation_hourly_rollups keyed by Workspace, Project, UTC hour,
  cost center, and environment, plus supporting scope/time indexes.

The migration also installs the reservation-to-usage capture trigger, the
hourly rollup trigger, immutable snapshot guards, and bounded JSONB tag
validation. Async video zero-cost placeholders defer capture until validated
final settlement; a normal zero-cost usage row remains reportable.

## Allocation values

Environments are normalized to production, staging, development, or testing;
unallocated is reserved for the explicit fallback and legacy rows. Allocation
tags are normalized to lower-case keys and trimmed scalar values, capped at 32
entries, and reject sensitive-looking keys such as secret, token, password,
credential, and api_key. Database checks repeat the bounds so direct SQL
cannot bypass the service contract.

An archived center or tag remains readable through historical snapshots and
reports. It cannot be attached to a new policy. A center/tag still referenced
by an active policy cannot be archived until the reference is removed, which
prevents new traffic from silently moving to another bucket.

## API and permissions

The control-plane endpoints are under /api/v1/workspaces/:id:

- GET/POST/PATCH/DELETE /cost-centers for lifecycle management;
- GET/POST/PATCH/DELETE /allocation-tags for controlled tags;
- GET/PUT /projects/:project_id/allocation for project defaults;
- GET/PUT /projects/:project_id/keys/:key_id/allocation for API-key
  overrides;
- GET/PUT /projects/:project_id/service-accounts/:service_account_id/allocation
  for Service-account overrides; and
- GET /finops/allocation and
  GET /projects/:project_id/finops/allocation for bounded reports.

The implementation reuses the central Workspace access service and existing
permission model: Workspace read/update for center/tag lifecycle, Project
read/update for project and credential configuration, and usage.read for
reports. Owner/Admin can mutate configuration; Billing retains financial
read access; Developer/Viewer remain subject to the existing Workspace and
Project access grants. Every route ID is checked again in the service and
repository transaction, and foreign tenant/project IDs return the bounded
Workspace error. Mutations write the existing audit, domain-event, and
outbox records with scalar allowlisted metadata.

## Reporting and reconciliation

Reports accept bounded RFC3339 from/to windows, a validated timezone,
environment, cost-center, and tag filters. Without tag filters, base
dimensions read the indexed hourly rollup and union tenant-scoped legacy
unallocated rows. Tag filters and tag breakdowns read immutable snapshots so
the tag set cannot drift with current configuration.

For every Workspace or Project report:

workspace_total = allocated + unallocated

Each usage row contributes once to that total. A row carrying multiple tags
may contribute to multiple tag groups for analysis; those groups are explicitly
overlapping and are never treated as an alternate total.

## Billing and async compatibility

Phase F is a reporting attribution dimension. It does not introduce a new
budget boundary or price modifier. Existing wallet debit, budget reservation/
finalization/release, refund, retry, and recovery semantics remain unchanged.
The admission snapshot carries Workspace, Project, billing principal, cost
center, environment, tags, source, and policy revision through synchronous,
streaming/Responses, Anthropic, OpenAI-compatible, Gemini, image, video,
Seedance, Grok, and Canvas paths that already use the common budget boundary.

For async media, the allocation is frozen when the task is admitted and
reserved. Polling, completion, cancellation, and recovery use that frozen
reservation snapshot even if an administrator edits the Project later. A
placeholder does not create a second snapshot, and idempotent settlement
creates exactly one final usage evidence row.

Phase E anomaly evidence remains immutable and its detector algorithm is
unchanged. It continues to consume the existing bounded rollups and does not
enter the Gateway hot path.

## Operational boundaries and deferred work

Migration history must be applied through 299 in order and can be replayed
idempotently by the repository migration runner. The indexed rollup avoids an
unbounded usage_logs GROUP BY; report windows are capped at 366 days and
breakdowns are limited to 100 groups per dimension.

Authenticated browser/manual acceptance, real upstream/provider smoke,
production load/chaos, deployment cutover, and formal external security
testing remain release-level gates (NOT RUN). Retention, export,
archive/restore, and deletion lifecycle are Phase G and are not included in
Phase F.
