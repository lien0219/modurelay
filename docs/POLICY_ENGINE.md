# Policy Engine

This document records the first policy API and console contract. The policy
editor is available under the tenant workspace routes; Gateway enforcement
must be reported separately from the existence of a stored policy value.

## Policy Layers

Applicable restrictions are predicates composed with AND semantics:

```text
Group
  AND Workspace
  AND Project
  AND Service Account (when present)
  AND Credential
```

A child scope may narrow the allowed model/platform set and lower a numeric
limit. It cannot re-open a parent restriction. The Effective Policy response
preserves the source layers for explainability and computes the minimum of the
configured numeric limits. Model and platform patterns are shown by source
layer; the console does not fabricate a materialized intersection.

Direct human API Keys use the four applicable layers:

```text
Group
  AND Personal Workspace
  AND Default Project
  AND API Key restrictions
```

They keep `principal_type=user` semantics with `service_account_id=NULL`; a
Service Account policy is never loaded for them. Missing Personal Workspace or
Default Project policy rows, nullable allowlists, and nullable numeric limits
inherit without adding a restriction. Existing direct-key Secret, Group,
per-key quota/RPM, execution-principal, Usage, billing, and budget behavior is
therefore preserved. A machine credential adds the Service Account layer only
when its server-resolved `service_account_id` is non-NULL.

## Scope Settings

| Scope | Console route | Policy API |
| --- | --- | --- |
| Workspace | `/workspaces/:workspaceId/policy` | `GET/PATCH /workspaces/:workspaceId/policy` |
| Project | `/workspaces/:workspaceId/projects/:projectId/policy` | `GET/PATCH /workspaces/:workspaceId/projects/:projectId/policy` |
| Service Account | `/workspaces/:workspaceId/projects/:projectId/service-accounts/:serviceAccountId/policy` | `GET/PATCH` on the matching nested `/policy` resource |

Project and Service Account pages also query a read-only Effective Policy:

```text
GET /workspaces/:workspaceId/projects/:projectId/effective-policy
GET /workspaces/:workspaceId/projects/:projectId/service-accounts/:serviceAccountId/effective-policy
```

The standard API envelope unwraps `data` to a Policy or Effective Policy
object. A scope without a stored row returns a default Policy at revision `0`,
with nullable allowlists and limits. The first write therefore uses
`expected_revision: 0`.

## Field Semantics

Policy fields are `allowed_models`, `allowed_platforms`, `rpm_limit`,
`daily_request_limit`, `monthly_request_limit`, `daily_token_limit`, and
`monthly_token_limit`. The three allowlist states must remain distinct through
database, API, TypeScript, and UI:

| Value | Meaning |
| --- | --- |
| `null` | This scope adds no restriction; inherit from its parent. |
| `[]` | Deny every model or platform at this scope. |
| Non-empty array | Allow only the listed exact identifiers. |

Numeric fields use `null` to inherit and positive integers for configured
limits. Zero, negative, fractional, and unsafe integer values are invalid. An
empty numeric field is serialized as `null`; the UI never changes `null` into
an empty allowlist.

The current console lists UTC daily and calendar-month request/token fields as
policy settings. A stored value alone does not prove that a live gateway path
enforces it. Release notes and operational acceptance must identify which
protocols have completed admission, reservation, and settlement integration.

## Gateway Settlement Contract

Gateway admission reserves request units and the conservative token estimate
before provider dispatch. A definitive pre-provider failure releases the
reservation. A definitive provider `4xx` consumes the request unit and settles
zero tokens. Successful or asynchronous work finalizes from reliable usage, or
from the immutable create-time estimate when the protocol has no usage field.
Provider transport failures, `5xx` responses, and accepted responses that lack
the correlation data needed for recovery are never treated as safe releases.

Live creation persists its policy reservation and lease in the Live record. If
the provider returns `2xx` without a usable `Location`, the gateway writes a
recovery-only record with a local opaque id and finalizes it after the maximum
session window; it does not retry another account and risk creating a second
session. If that recovery write also fails, the gateway releases the lease and
settles the reservation request-only as the explicit last-resort fallback.
The same reservation fields are round-tripped through Redis so a later Live
observer or recovery path cannot lose tenant or quota attribution.

Async image tasks persist the policy reservation ID with the task, but the
current detached image worker has no startup scanner or durable settlement
queue. If a definitive release or request-only finalization still fails after
the bounded in-process retries, the reservation remains pending for operator
recovery; an async image process restart must not be described as settlement
safe until that recovery consumer exists. Provider-uncertain image outcomes
remain preserved rather than being released as if they were rejected.

## Effective Policy

Effective Policy reports `layers`, `revisions`, and the minimum configured
numeric limits. Each source layer retains its own nullable lists so aliases,
group restrictions, and future matching rules remain attached to their owner.
The console displays model/platform constraints per source, numeric effective
limits from the resolver, and source revisions. It is read-only.

The effective numeric value is the minimum of configured positive values;
unconfigured layers do not participate. If every layer is `null`, the effective
numeric limit is unbounded. Empty model/platform arrays remain a deny-all
predicate and are not interpreted as unrestricted.

## Updates and Permissions

`PATCH` carries all editable fields and `expected_revision`. Revision `0`
creates a missing row; subsequent writes must match the current revision. A
stale update returns `409`, and the console asks the operator to reload before
retrying. Successful writes return the updated Policy with its new revision.

The console uses `policy.read` and the scope-specific mutation permissions
`workspace_policy.update`, `project_policy.update`, and
`service_account_policy.update` from the tenant RBAC map. The server remains
authoritative for scope membership, RBAC, and cross-tenant isolation.
Client-selected IDs do not establish policy ownership.

## Frontend Verification

The focused Vue tests cover tenant-scoped API paths, compare-and-swap revision
payloads, `null` versus `[]`, read-only source-layer display, and the first-write
revision. Chinese and English labels are required for every visible field and
state. The responsive form uses the existing workspace surfaces and semantic
tokens; no shared design system or unrelated route is changed.

The later end-to-end acceptance matrix is maintained in
`docs/POLICY_ENGINE_ACCEPTANCE.md`. It covers policy behavior together with
gateway, quota, budget, provider, notification, webhook, RBAC, and cache
freshness evidence. Its execution status must be reported separately from
unit, integration, and build checks.
