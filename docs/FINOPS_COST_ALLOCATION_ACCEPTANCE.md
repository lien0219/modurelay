# Phase F acceptance matrix

This document records the local Phase F boundary for migration 299 and the
cost-allocation implementation. Results below come from commands executed on
`feature/new-feature`; unavailable external or manual gates remain `NOT RUN`.

## Automated matrix

Run backend commands from `backend` and frontend commands from `frontend`.

| Area | Command | Result | Evidence |
| --- | --- | --- | --- |
| Migration contract | `go test ./migrations -run FinOpsCostAllocation -count=1` | PASS | Migration 299 tables, tenant constraints, immutable triggers, archive guards, bounded tag JSON, and no-history-rewrite checks. |
| Allocation domain and precedence | `go test ./internal/service -run 'Allocation|Budget' -count=1` | PASS | Environment/tag validation, cardinality and secret-key rejection, precedence, defensive copies, policy revisions, and budget admission tests. |
| Admission and budget repository | `go test ./internal/repository -run 'Allocation|Budget' -count=1` | PASS | Reservation snapshot persistence, retry stability, and budget repository coverage. |
| Control-plane API | `go test ./internal/handler -run 'Allocation|Workspace' -count=1` | PASS | Allocation route registration, bounded workspace/project scopes, handler compilation, and Workspace API tests. |
| PostgreSQL allocation integration | `go test -tags=integration ./internal/repository -run 'Allocation|FinOps' -count=1` | PASS | Cross-tenant rejection, audit/domain event atomicity, reservation and usage snapshots, archive behavior, rollup/report reconciliation, tag overlap, and async video placeholder/settlement. |
| PostgreSQL allocation integration under race | `go test -race -tags=integration ./internal/repository -run 'Allocation|FinOps' -count=1 -timeout=10m` | PASS | Same allocation/FinOps integration matrix passed under race instrumentation. |
| Go static checks | `go vet ./...`; `go build ./...`; pinned `golangci-lint v2.13.0` | PASS | Vet and build exited zero; pinned lint reported `0 issues`. Changed Phase F Go files are gofmt-clean. |
| Frontend focused | `pnpm exec vitest run src/views/workspace/__tests__/WorkspaceAllocation.spec.ts src/views/workspace/__tests__/WorkspaceFinopsView.spec.ts` | PASS | 2 files, 11 tests passed. |
| Frontend full suite | `pnpm run test:run` | PASS | 431 files, 3205 tests passed. Existing test warnings are non-failing. |
| Frontend i18n | `pnpm run check:i18n` | PASS | 1 file, 3 tests passed; English and Simplified Chinese workspace keys match. |
| Frontend lint/typecheck/build | `pnpm run lint:check`; `pnpm run typecheck`; `pnpm run build` | PASS | All exited zero. Build retained existing Vite chunk/dynamic-import advisories only. |
| Canvas/Seedance regression | `pnpm run test:canvas` and focused Go `Seedance|Grok|Video` tests | PASS | Canvas suite: 7 tests; service, handler, and repository media regression packages passed. |
| Diff hygiene | `git diff --check` | PASS | No whitespace errors after documentation and source changes. |

The complete backend commands were also run:

| Command | Result | Evidence |
| --- | --- | --- |
| `go test ./... -count=1` | PRE-EXISTING | Only the three existing `backup_pg_dumper` tests fail because Windows cannot find `sh.exe`; each also reports the expected sqlmock unlock-expectation cascade. All other packages, including Phase F packages, passed. |
| `go test -tags=integration ./... -count=1 -timeout=20m` | PRE-EXISTING | The same three `backup_pg_dumper`/`sh.exe` failures and unlock cascade; no Phase F-only failure. The failure signature matches the recorded Phase E baseline. |

No test was deleted, skipped, or weakened to obtain these results. The
repository-wide `gofmt -l` command still reports the unrelated pre-existing
`backend/internal/pkg/ip/ip_test.go`; no Phase F file is listed.

## Required behavior cases

| Case | Result | Evidence |
| --- | --- | --- |
| Cross-tenant cost-center, tag, project, key, and service-account IDs | PASS | Composite foreign keys plus PostgreSQL tenant-isolation integration and service/repository predicates. |
| API-key → service-account → project → unallocated precedence | PASS | Domain precedence tests and authoritative admission query. |
| Immutable admission and usage snapshots | PASS | Reservation retry drift test, database immutable trigger, and PostgreSQL usage snapshot assertions. |
| Cost-center update/archive preserves history | PASS | Archived configuration is rejected for new traffic while historical snapshot/report rows remain readable. |
| Legacy NULL/unallocated usage | PASS | Report queries union tenant-scoped legacy rows as explicit unallocated evidence without rewriting `usage_logs`. |
| Billing total reconciliation | PASS | Integration report asserts `workspace_total = allocated + unallocated`; tag groups are marked overlapping and excluded from total summation. |
| Budget consistency and idempotent retry | PASS | Reservation snapshot is written with the budget hold; repeated request IDs retain the first allocation and do not double charge. |
| Async video settlement | PASS | Zero-cost video placeholder does not capture evidence; final settlement captures the original reservation allocation once. |
| Concurrent configuration mutation | PASS | Workspace lock and policy revision conflict checks prevent attribution drift. |
| Tag cardinality and scalar bounds | PASS | Service and migration tests enforce the 32-tag cap, normalized key/value bounds, and secret-like key rejection. |
| Timezone and bucket boundary | PASS | Bounded RFC3339 filters, timezone validation, and UTC hourly rollup constraints. |
| Workspace-switch stale response protection | PASS | Focused Workspace allocation UI tests cover stale response isolation and permission visibility. |
| Phase E anomaly compatibility | PASS | Existing anomaly tests and FinOps integration remain green; allocation is layered onto usage/reservation evidence. |

## Release boundaries

Authenticated browser/manual acceptance, real upstream/provider smoke,
production load/chaos, deployment cutover, and formal external security
testing are `NOT RUN`. They remain release-level Phase L/I gates and are not
converted to `PASS` by local tests.
