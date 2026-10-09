# Phase G implementation plan

> Agentic workers: use superpowers:subagent-driven-development for isolated
> implementation tasks and independent review; root owns integration. Read the
> assigned task brief, preserve other edits, and make no intermediate commits.

Goal: production lifecycle functionality with preserved financial references.
Architecture: [approved gate](ENTERPRISE_DATA_LIFECYCLE_ARCHITECTURE.md).
Stack: existing Go/sql.DB/PostgreSQL, S3 factory, Vue/TypeScript and Vitest.
Spec: the attached Phase G requirements and the approved architecture document.

## Global constraints

- Branch `feature/new-feature`; starting HEAD `3888ba452d8da926d1d6d234b7641acb6ebcc193`.
- No push, PR, production export/restore/delete, acceptance runtime modification,
  trigger disabling or historical usage rewrite. One final local commit.
- Workspace Owner and Global Admin remain separate; central RBAC is mandatory.
- Financial/audit retention default indefinite; 0 means never purge. Each
  eligible cleanup is indexed, bounded and guarded by its actual dependencies.
- Export: 500-row keyset pages, 100000 total records, 64 MiB uncompressed ZIP
  input, 2-minute snapshot, 5 attempts, 5-minute lease, 7-day artifact default,
  one-use 60-second download grant. Oversize fails without completed artifact.
- Purge: 200-row batch, 20-second transaction, 7-day minimum cooling, 5 attempts,
  60-second lease. No financial row or tenant parent is physically deleted.
- Complete PostgreSQL, regression, race/static and independent destructive
  review gates before COMPLETE; disclose PASS/FAIL/PRE-EXISTING/NOT RUN precisely.

## Task 1: Retention and financial graph

Files: migration 300; new lifecycle retention model/repository; Usage cleanup,
dashboard partition cleanup, delivery retention; disposable workspace fixtures.
Root owns job schema migration 301 and the other lifecycle source files.

Interface:
```go
type LifecycleRetentionPolicy struct {
 Category string `json:"category"`
 RetentionDays int `json:"retention_days"`
 MinimumDays int `json:"minimum_days"`
 Protected bool `json:"protected"`
}
LifecycleRetention(ctx context.Context, actorID, workspaceID int64) ([]LifecycleRetentionPolicy, error)
UpdateLifecycleRetention(ctx context.Context, actorID, workspaceID int64, category string, days int) ([]LifecycleRetentionPolicy, error)
```

- [x] Add behavioral RED cases for direct snapshot DELETE, parent CASCADE,
  cleanup preserving tenant Usage, and lower-floor rejection.
- [x] Run the new cases against migration 299 and retain actual failing output.
- [x] Add platform/tenant retention tables with explicit category checks and a
  SQL effective-retention function. Protect both allocation snapshot tables,
  tenant usage and reservations from DELETE. Keep frozen settlement UPDATEs.
- [x] Add selection exclusions to every legacy Usage cleanup/partition path;
  preserve affected tenant rollups; respect effective floors in event workers.
- [x] Replace max-ID row-deletion fixture cleanup with disposable database or
  schema isolation. Never disable immutable triggers for cleanup.
- [x] GREEN: real PostgreSQL graph, floor and legacy cleanup tests; relevant
  existing suites; record test command/output and scoped quality/spec review.

## Task 2: Export

Files: migration 301, lifecycle_models.go, lifecycle export service/worker and
repository, chunk encryption, configured DI, lifecycle handler.

Interfaces:
```go
CreateLifecycleExport(context.Context, int64, int64) (*LifecycleExportJob, error)
ListLifecycleExports(context.Context, int64, int64, int, int) ([]LifecycleExportJob, int64, error)
GetLifecycleExport(context.Context, int64, int64, string) (*LifecycleExportJob, error)
CancelLifecycleExport(context.Context, int64, int64, string) error
```

- [x] RED: Owner IDOR, secret exclusion, snapshot concurrent writes,
  cancellation/stale token, corruption/expiry/replayed download.
- [x] Durable claims, paginated allowlisted snapshot ZIP and manifest with
  exact decimal totals; chunk AES-GCM before UploadFile, attempt-specific keys.
- [x] Fenced progress/completion, retry/backoff, cancellation and expiry object
  cleanup, bounded process-wide worker and shutdown.
- [x] POST authenticated authorization then POST authenticated token redemption;
  server verifies bounded decryption and returns an attachment after live checks. No S3 URL exposed.
- [x] GREEN: real PostgreSQL tests plus crypto/service tests and race checks.

## Task 3: Archive/restore and deletion

Files: lifecycle model/repository/service/handler, central Workspace RBAC,
Workspace mutation/read gates, domain event registry/recipient resolver.

- [x] RED: archived history read; unsafe restore rejected; pending work and
  active holds block irreversible cleanup; historical immutable records remain
  intact while business closure completes; stale challenge/cancel/worker races fail.
- [x] Add explicit restore commands. Reuse current security evaluation and
  actual recent MFA proof; check parent, billing Owner and live memberships.
- [x] Add preflight, one-use expiring challenge and exact-name confirmation;
  use Workspace lock to atomically rerun checks, request/cancel and audit/events.
- [x] Add token-fenced purge phases/cursors with stable batch order, bounded
  retry and blocker reporting; completion requires complete phase verification.
- [x] GREEN: PostgreSQL multi-instance, crash/restart/checkpoint rollback,
  tenant/global identity isolation, grace and archive/admission boundary tests.

## Task 4: Functional UI

Files: typed lifecycle API, new WorkspaceLifecyclePanel in existing settings,
Project archive/restore actions, en/zh messages and focused behavioral tests.

- [x] Read design system/Frosted skill and local UX search before UI changes.
- [x] RED: display blockers, exact-name confirmation, cancellation and stale
  Workspace responses; never enable destructive actions from hidden roles.
- [x] Render retention, history/progress, authenticated download, archive/
  restore, preflight/deletion confirmation/status. Preserve existing selectors.
- [x] GREEN: affected Vitest/typecheck/lint/i18n, then full frontend/build.

## Task 5: Integration and delivery

- [x] Independent task spec/quality and final destructive/security review.
- [x] PostgreSQL matrix, migration package, relevant regressions/race, Go vet/
  build, pinned lint 2.13.0, full frontend and Canvas critical tests.
- [x] Run complete backend default/unit/integration commands; reproduce exact
  failures on an archived clean actual starting HEAD before baseline labeling.
- [x] Write lifecycle/runbook/acceptance documents with executed evidence.
- [x] Update roadmap only to the actually verified state; final diff/secret
  review, explicit path staging and authorized local phase-boundary commit.

## Executed verification boundary

See [acceptance](ENTERPRISE_DATA_LIFECYCLE_ACCEPTANCE.md) for actual commands,
results, review closure and release limits. Native Go1.27.2 source is unchanged.
The requested native lint2.13.0 result is PRE-EXISTING / TOOLCHAIN BLOCKED,
not PASS. User-approved equivalent-source lint2.13.0 and the preferred native
compatible lint2.14.0 both pass with the original config/rules. Existing backend
CI already uses2.14.0; compatible CI must pass before PR/release.

Complete default/integration suites pass with process-local Git Bash PATH.
The additional full unit-tag command has one PRE-EXISTING Ollama CAS assertion,
reproduced against actual3888ba452 and current source at count20. All Phase G
targeted, PostgreSQL16/18.1, race, frontend and Canvas gates pass. No test,
assertion, financial FK or immutable protection was removed to obtain a PASS.
Checkboxes record executed work under these disclosed boundaries, not an
assertion that blocked/baseline invocations or live production acceptance passed.
