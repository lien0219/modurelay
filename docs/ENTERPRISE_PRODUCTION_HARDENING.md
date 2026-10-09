# Phase I Production Hardening

Phase I hardens the existing Enterprise Workspace implementation around three
failure boundaries: export key compatibility, financial evidence, and recovery
after a process or dependency failure. The canonical source baseline is
`2e24b086d04c52fb4174e8f995a253bfe62e4989` on `feature/new-feature`.

## Scope and invariants

- Existing routes, payloads, permissions, billing principals, and immutable
  attribution snapshots remain authoritative.
- MRLEX01 artifacts remain readable. MRLEX02 is explicit-key-ID, fail-closed,
  and disabled for writes unless the configured reader inventory and compatible
  rollback gate agree.
- A failed billing transaction cannot create a zero-cost tenant usage receipt.
  Recovery replays the frozen command and original attribution rather than
  recomputing ownership from current credentials.
- Provider-start markers and SQL recovery records are written before an async
  image/media request crosses the upstream boundary. Unknown provider outcomes
  are preserved and are never blindly resubmitted or refunded.
- FinOps and dispatcher work is fenced by live leases/tokens and bounded retry
  state. Stale workers cannot commit evidence.

## Implemented areas

`backend/internal/service/lifecycle_keys.go` and the lifecycle configuration
validation add explicit key IDs, versions, statuses, deterministic fingerprints,
and reader-consensus inputs. `backend/internal/service/lifecycle_artifact.go`
keeps the V1 reader and adds authenticated V2 headers with workspace/object
scope binding. `backend/migrations/305_workspace_export_crypto_readers.sql`
stores only metadata and reader liveness; key bytes stay in trusted secret
configuration.

Tenant policy writes now repeat live User, Workspace, Project grant, Service
Account, lifecycle, MFA, SSO, and session-age checks inside the mutation
transaction. Financial recovery uses migrations 306 and 307 for frozen usage,
media attempts, SQL batch recovery, and explicit FinOps terminal failure.
Video creates persist a durable attempt before the provider boundary. Accepted
tasks keep the marker's tenant, reservation, model, dimensions, and allocation
snapshot when the pending billing record is finalized; 4xx rejection is marked
terminal, and stale started attempts are classified as unknown without replay.

## Operational boundary

This phase was implemented and exercised only with synthetic data and
disposable PostgreSQL, Redis, and MinIO/test-provider environments. It did not
deploy or restart the manually managed `18081` acceptance service, rotate real
keys, access production storage, or call real providers. Exports and purge stay
default-off until the release checklist and Phase L gates are approved.

See [the acceptance record](ENTERPRISE_PRODUCTION_HARDENING_ACCEPTANCE.md),
[the release checklist](ENTERPRISE_RELEASE_READINESS.md), and [the key rotation
runbook](ENTERPRISE_KEY_ROTATION_RUNBOOK.md).
