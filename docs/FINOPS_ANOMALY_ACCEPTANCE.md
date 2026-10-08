# FinOps anomaly detection acceptance

This checklist is the Phase E boundary for migration 298 and the local commit
`feat(finops): add workspace anomaly detection and findings`. It records the
checks needed to accept the detector without treating unavailable infrastructure
or real-provider checks as successful.

## Automated checks

Run from `backend` unless noted:

```powershell
go test ./internal/service/... -run 'FinOps|WorkspaceAccess|DomainEvent' -count=1
go test ./internal/repository/... -run 'FinOps|Workspace' -count=1
go test ./internal/handler/... -run 'FinOps|Workspace' -count=1
go test ./migrations -count=1
go test -race ./internal/service/... ./internal/repository/... -run 'FinOps' -count=1
go vet ./...
go build ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.0 run --timeout=30m ./...
```

Run from `frontend`:

```powershell
pnpm run test:run -- src/views/workspace/__tests__/WorkspaceFinopsView.spec.ts
pnpm run check:i18n
pnpm run lint:check
pnpm run typecheck
pnpm run build
```

The complete backend commands (`go test ./...`, unit, and integration tags)
must be compared with the Phase D baseline. A Windows `sh.exe` failure in the
existing backup dumper tests is an environment baseline failure only when the
same command and failure are reproduced on the parent. It must be reported as
`PRE-EXISTING`, never hidden with a skip or relaxed assertion.

## 2026-10-08 execution record

The following commands were run on `feature/new-feature` from the Phase D
boundary `a7e8427a6` after migration 298 and the Phase E implementation were
present:

| Check | Result | Evidence |
| --- | --- | --- |
| `go test ./internal/service/... -run 'FinOps|WorkspaceAccess|DomainEvent' -count=1` | PASS | Service packages completed successfully. |
| `go test ./internal/repository/... -run 'FinOps|Workspace' -count=1` | PASS | Repository packages completed successfully. |
| `go test ./internal/handler/... -run 'FinOps|Workspace' -count=1` | PASS | Handler packages completed successfully. |
| `go test ./migrations -count=1` | PASS | Migration contract tests completed successfully. |
| `go test -race ./internal/service/... ./internal/repository/... -run 'FinOps' -count=1` | PASS | Race run completed; non-integration repository package had no tagged tests to run. |
| `go test -race -tags=integration ./internal/repository -run '^TestFinOpsAnomalyPostgres' -count=1 -timeout=10m` | PASS | PostgreSQL anomaly scan, idempotency/evidence, and lease fencing passed under race instrumentation. |
| `go test -tags=integration ./internal/repository -run '^TestFinOpsAnomalyPostgres' -count=1 -timeout=5m` | PASS | PostgreSQL tenant/project isolation, finding/event idempotency, immutable snapshot, ACK/Resolve concurrency, and lease fencing passed. |
| `go test -tags=integration ./... -count=1 -timeout=20m` | PRE-EXISTING | All packages, including Phase E integration tests, passed except the three existing `backup_pg_dumper` tests; each fails because Windows cannot find `sh.exe`, followed by the expected sqlmock unlock-expectation cascade. The identical failure set was reproduced on the clean Phase D parent. |
| `go test ./... -count=1` | PRE-EXISTING | Same three `backup_pg_dumper`/missing-`sh.exe` failures and no Phase E-only failure. |
| `go vet ./...` / `go build ./...` / pinned `golangci-lint v2.13.0` | PASS | Static checks completed with no new diagnostics; pinned lint reported 0 issues. |
| `pnpm run test:run` | PASS | 430 frontend files and 3199 tests passed. |
| `pnpm run lint:check` / `pnpm run typecheck` / `pnpm run build` / `pnpm run check:i18n` | PASS | Lint, typecheck, production build, and 3 i18n tests passed; existing Vite chunk/dynamic-import warnings are non-failing. |

The combined historical-migration/anomaly selection was also rerun after the
test fixture restored migration 298's event payload contract. It retained only
the two `backup_pg_dumper` tests selected by that narrower expression; no
anomaly event constraint failure remained. PostgreSQL was available for the
integration runs above, so the Phase E database matrix is `PASS`, not `NOT RUN`.

## Database and integration matrix

With PostgreSQL configured, run the migration history through 298 twice and
verify the following cases:

| Case | Expected result |
| --- | --- |
| 1 through 297, then 298 | All migrations apply in order; the second run is idempotent. |
| Workspace A reads Workspace B finding | Not found; no cross-tenant row is returned. |
| Project A route reads Project B finding | Not found; composite tenant scope is enforced. |
| Same bucket scanned twice | One snapshot, one finding, and one detected event. |
| Two workers claim one unit | One lease owner processes it; an expired token cannot complete a newer lease. |
| Update/delete evidence snapshot | Database trigger rejects both operations. |
| Stale ACK/Resolve version | One transition succeeds; the stale request returns conflict. |
| Invalid status transition | Database/service boundary rejects it. |
| Deleted or archived project/key/service account | Historical finding remains readable without exposing a secret. |

If PostgreSQL, migration credentials, or the configured integration harness are
unavailable, mark this matrix `NOT RUN` and retain the reason in the delivery
report.

## Detector table cases

The service table tests cover insufficient samples, zero/constant baselines,
MAD=0, spend/request/unit-cost spikes, sparse high-value spend, absolute and
relative floors, future buckets, NaN/Infinity rejection, deterministic
fingerprints, and severity boundaries. Additional review should confirm that
28-day same-UTC-hour baselines, the 12-sample minimum, five-minute grace, and
candidate/workspace/rollup hard caps remain centralized in
`DefaultFinOpsAnomalyConfig`.

## Manual UI and security checks

When an authenticated acceptance runtime is available:

1. Open Workspace FinOps as owner, billing, developer, and viewer. Confirm the
   list, filters, evidence explanation, and role-appropriate actions.
2. Switch workspaces while list/status/detail requests are pending. Confirm the
   old response cannot populate the new workspace.
3. Exercise acknowledge, resolve, stale-version conflict, empty, API-error, and
   sub-cent evidence states. Confirm the table scrolls inside its container at
   390px and 430px widths and controls remain keyboard reachable.
4. Try foreign workspace/project/finding IDs, direct API-key and service-account
   dimensions, and notification reads. Confirm tenant and project authorization
   is rechecked server-side and no credential secret is present.
5. Inspect the domain-event/outbox stream. Confirm scalar bounded evidence,
   owner/admin/billing recipients, and one notification for a retried bucket.

Authenticated browser, real upstream/provider, production-load, chaos, and
deployment cutover checks are release-level gates. Report them as `NOT RUN` when
they are not executed.
