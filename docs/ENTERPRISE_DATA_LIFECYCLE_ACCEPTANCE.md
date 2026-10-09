# Phase G acceptance record

Date: 2026-10-09. Branch: `feature/new-feature`. Baseline:
`3888ba452d8da926d1d6d234b7641acb6ebcc193`. Go module/toolchain: **1.27.2**,
unchanged. This is a local implementation boundary, not production activation.
The authorized local commit is
`feat(lifecycle): add workspace retention export and deletion lifecycle`.
Resolve its immutable SHA from Git after commit; a commit cannot embed its own SHA.

## Acceptance decision and classification

The Phase G behavior, native vet/build, compatible native lint, PostgreSQL,
relevant race, frontend and Canvas gates pass. No unresolved Phase G regression
or important review finding remains. Raw pinned lint and the additional full
unit-tag suite retain the precisely reproduced baseline failures below; neither
is described as PASS. Local COMPLETE requires the final security artifact seal,
source/diff verification and phase commit as well as these executed results.
Production deletion/export/restore, deployment and port18081 were not operated.

| Gate | Classification | Executed result and evidence under `.cache/phase-g/` |
| --- | --- | --- |
| Native `go vet ./...` | PASS | Exit0; `native-vet-final.log` |
| Native `go build ./...` | PASS | Exit0; `native-build-final.log` |
| Complete default backend | PASS | `go test ./... -count=1 -timeout=15m`; exit0, service171.838s; `backend-default-verified.log` |
| Complete integration backend | PASS | `go test -tags=integration ./... -count=1 -timeout=15m`; exit0, service173.011s; `backend-integration-verified.log` |
| Complete unit-tag backend | PRE-EXISTING (observed FAIL) | `go test -tags=unit ./... -count=1 -timeout=15m`; exit1 with only `TestOllamaProbeCallback_StaleLongDoesNotOverrideNewShort`, line405, `stale long callback must not pass the CAS`; `backend-unit-verified.log` |
| Phase G scoped config/handler/service and PostgreSQL | PASS | `root-scoped-final.log`, `root-postgres-final.log`; final complete integration above also includes all lifecycle tests |
| PostgreSQL18.1 lifecycle/financial graph | PASS | Full final integration, scoped lifecycle and Task1 retention matrix; immutable constraints enabled |
| PostgreSQL16 final lifecycle/retention | PASS | `go test -tags=integration ./internal/repository -run '^(TestLifecycle\|TestDomainEventRetention\|TestRequestLogRetention)' -count=1 -timeout=10m -v`; exit0,20.279s; `postgres16-lifecycle-verified.log` |
| Relevant PostgreSQL race | PASS | `go test -race -tags=integration ./internal/repository -run '^(TestLifecycle\|TestWorkspaceAllocation\|TestVideoUsage\|TestWorkspaceSecurity\|TestFinOpsAnomaly\|TestDomainEventTenantVideo\|TestWorkspaceWebhook)' -count=1 -timeout=10m`; exit0,46.083s; `race-verified.log` |
| Seedance/Grok/video, FinOps E/F, Workspace Security | PASS | Their existing suites execute in the complete default/integration commands; allocation/video/security/anomaly/webhook PostgreSQL suites also execute under the relevant race command |
| Native lint2.13.0 / Go1.27.2 | PRE-EXISTING / TOOLCHAIN BLOCKED | Current and actual baseline fail to import export data version5 with importer maximum4; `lint.log`, `lint-baseline.log` |
| Native lint2.14.0 / Go1.27.2 | PASS | Same original `.golangci.yml`, `run --timeout=30m`; exit0, `0 issues`; `lint-native-2.14-verified.log` |
| Isolated lint2.13.0 / Go1.26.2 | PASS within compatibility boundary | Same source/config/rules, `GOEXPERIMENT=jsonv2`, `run --timeout=30m`; exit0, `0 issues`; `lint-compatibility-serialized.log` |
| Final focused frontend | PASS | Seven relevant files,110 tests; `frontend-all-review-final.log` |
| Complete frontend Vitest | PASS | `pnpm test:run`;433 files/3252 tests, exit0; `frontend-full-verified.log` |
| Frontend lint/typecheck/i18n/build | PASS | `pnpm lint:check`, `pnpm typecheck`, `pnpm build`; exit0; corresponding `frontend-*-verified.log`; locale completeness also executes in focused/full Vitest |
| Canvas critical Seedance suite | PASS | `pnpm test:canvas`;7/7; `canvas-critical-final.log` |
| Independent source/destructive review | PASS within source-review scope | Backend round2 and frontend round3 APPROVED; original findings preserved and closed; root owns executed tests |
| Final security diff review | See sealed report | Canonical local-patch scan, full changed-file accounting, zero surviving candidates; receipts and generated report location are recorded below |
| Manual/browser/provider/storage/load/production | NOT RUN | Explicit release gates below; automated source/crypto/container tests do not establish these results |

Backend test processes prepend `D:\git\Git\usr\bin` to PATH and use `CI=true`.
This enables existing sh-dependent tests and makes unavailable Docker fail
loudly; it changes no business source, test assertion or persistent environment.
PostgreSQL16 uses `SUB2API_TEST_POSTGRES_IMAGE=postgres:16`; default is18.1.
Integration fixtures clone disposable migrated databases, never delete protected
evidence or disable financial triggers/FKs for teardown.

## Baseline and tooling evidence

The initial raw Windows PATH lacks `sh`. The same three PgDumper failures and
follow-on sqlmock unlock expectations reproduce on the actual baseline in
`g1-backup-baseline.log`; process-local PATH configuration resolves them, and
final complete default/integration suites pass. Do not reuse an earlier failed
full-suite log as a final green result.

The extra full unit-tag command includes the unchanged Ollama callback test.
The exact test was executed20 times on current source and the archived actual
starting source with the same native toolchain, tag and environment:

```powershell
go test -tags=unit ./internal/service -run '^TestOllamaProbeCallback_StaleLongDoesNotOverrideNewShort$' -count=20 -v
```

Both exit1 with the identical line405 assertion:
`ollama-current-unit-reproduction.log`, `ollama-baseline-unit-reproduction.log`.
The production Ollama429 service, test and account CAS repository are byte
unchanged from3888ba452. The wall-clock-based five-second reset fixture can
collide with its scheduling reset; this pre-existing boundary needs a separate
deterministic generation/regression repair before declaring the unit-tag suite
green. No test was skipped, retried to manufacture a PASS, or weakened.

Pinned lint2.13.0 has a proven Go export-import compatibility failure, not a
business-code lint finding. The user approved this precise acceptance boundary.
The preferred repair was verified: lint2.14.0 supports the original Go1.27.2
source and reports0 issues. The repository's existing backend CI already pins
v2.14.0 with `go-version-file: backend/go.mod`; no workflow downgrade was needed.
Before PR/release, require that compatible CI combination to run successfully.
Do not label the old native2.13.0 invocation PASS.

`lint-source-equivalence.json` hashes3670 files: zero source/config/rule
differences, and only isolated Go.mod metadata1.27.2→1.26.2. Inventory SHA256:
`110797d94b303ded759e08c257bb35c703ed16ceabc14396911d755bfd3ed760`.
The real Go.mod was not edited. No excludes/checks were removed. Earlier parallel
lint runs hit timeout/runner-lock failures (`lint-native-2.14-final.log`,
`lint-compatibility-verified.log`); serialized reruns passed. Those execution
failures are recovered, not PRE-EXISTING or PASS artifacts.

## Financial, tenant and recovery verification

The final PostgreSQL lifecycle/retention matrix verifies the required22 safety
areas and concurrency boundaries through behavioral tests, not SQL mocks alone:

| Boundary | Relevant executable evidence |
| --- | --- |
| Floors, indefinite policy, active holds | `FloorsFailClosed`, `RepositoryAuthorizationAndFloors`, `ArtifactCleanupHonorsCurrentFloorAndHold`, `ActiveHoldPreservesEligibleDeliveries` |
| Protected finance, no CASCADE/TRUNCATE escape | `AllocationDeleteAndParentCascadeRejected`, `FinancialTruncateRejected`, `TenantSettlementDedupPreserved`, `MixedPartitionNeverDropped` |
| Consistent export, concurrent writes, checksums/totals | `SnapshotExcludesConcurrentCommitAndChecksums`, `ExportScopeFencingAndSnapshot`, `ExportUncompressedAllowlistAndSecretExclusion` |
| Cancellation, expiration, replay/current Owner security | `ExpiredFifthExportAndCancellationCleanup`, `DownloadRechecksExpiryRoleAndLiveSecurity`; service crypto/download tests also exercise truncation, tampering, bounds and path scope |
| Archive/restore, security and admission | `RestoreProjectAndCurrentProof`, `RestoreAndDeletionConfirmation`, `PendingVideoAndHoldBlockButSettlementAfterArchiveSucceeds`; existing Workspace admission/security regressions |
| Historical finance permits completed business closure | `HistoricalFinanceCompletesBusinessClosure`, `ClosureRetainsPolicyProtectedConfiguration`; original Usage/reservation/allocation/anomaly graph and Global User/Workspace B identifiers remain intact |
| Pending/unknown image/video/refund/settlement | `PendingVideoAndHoldBlockButSettlementAfterArchiveSucceeds`, `CleanedImageHistoryPermitsDeletion`, `LateImageOutputCleanupPermitsPurge`, `UnknownImageOutcomesStillBlockDeletion`; existing video/tenant billing and allocation settlement regressions |
| Multi-instance claim, crash/restart, cursors | `PurgeSingleLeaseAndRestart`, `PurgeVerifiesResourcesBehindCursor`, `ExportScopeFencingAndSnapshot`; independent terminal invariant verification |
| Restore/delete, grace, cancel and stale worker conflicts | `RestoreAndDeletionRaceCannotReopenJob`, `RestoreAndDeletionConfirmation`, `ExpiredLeaseCannotCommitTerminalClosure`; challenge replay/recent proof and minimum grace remain enforced |
| Rollback, evidence/event atomicity and dedup | `LateHoldAndCompletionAuditRollback`, `PolicyAuditAndOutboxRollBackTogether`, `HistoricalFinanceCompletesBusinessClosure`; lifecycle event registry/subscription unit tests |
| Retention parent/child and partition writer races | `ConcurrentChildCommitKeepsEnvelope`, `PartitionDropCannotRaceWriter`, `WebhookEnqueueRechecksLockedWorkspaceState` |
| Direct repository RBAC/current MFA | `RetentionRepositoryRechecksCurrentSecurity`, cross-tenant export/grant IDs and tenant-prefixed artifact scope tests |

Names above abbreviate `TestLifecyclePostgres` or
`TestLifecycleRetentionPostgres` prefixes; full tests reside in the lifecycle
integration files. Migration300 replay preserves immutable protection. The full
migration package and migrated PostgreSQL16/18.1 harness both pass.

Retained behavioral RED/GREEN pairs include financial graph/hold/dedup/child
retention, image terminal states, expired terminal leases, lifecycle webhook
visibility, enqueue/purge state fencing and direct retention security. Frontend
RED/GREEN verifies indefinite exports, binary MFA errors, stale decoder context,
polling without draft loss and new-export pagination; existing assertions remain.
See `task1-report.md` and the immutable independent review reports for original
failure evidence and explicit source/runtime attribution.

## Review scope and performance limits

Independent backend source review covers58 files; final frontend review covers
13 files with original review carry-forward only after hash comparison. Both
original reports are preserved. The separate compact backend security discovery
covers58/58 paths,299 citation ranges and81 source bindings, with zero candidates.
Frontend candidate output is empty. Reviewers did not execute tests or operate
external services; all runtime PASS claims above belong to root-run commands.

The final Codex Security terminal scan retains the exact schema-valid supplied
threat model, all final changed source/config/test/docs paths and immutable
receipts. It uses `codex-security:security-diff-scan`; its generated `report.md`
is the primary result. Detailed location: process TEMP under
`codex-security-scans/modurelay/`; `.cache/phase-g/security-final-location.json`
records the exact local scan/report paths. Token usage is unavailable in this
terminal workflow and is not estimated.

The implemented bounds are500-row export keyset pages,100000 records,64MiB,
two-minute MVCC snapshot,20-second statements/checkpoints,200-row purge batches,
five attempts, live-clock token fencing, one process-wide worker, and bounded
download/export concurrency. PostgreSQL experiments exercise contention and
resume correctness; these are not production throughput or migration-lock
measurements. Build warnings about existing large bundles remain visible; no
frontend build threshold was relaxed.

## Migration and operational release gates

Migration300 strengthens allocation parent RESTRICT and financial DELETE/TRUNCATE
guards, while preserving settlement UPDATEs and immutable historical values.
Migration301 adds jobs/object ledger/download grants/challenges/holds/platform
grace and constrained lifecycle states. DDL/index/FK changes need a rehearsed
lock window; no production timing or rollback restore was measured. No historical
migration was changed and no whole-table attribution rewrite was performed.

NOT RUN: authenticated light/dark/mobile/keyboard/browser acceptance; real
provider/upstream smoke; actual S3 IAM/TLS/privacy and Windows temp ACL checks;
production load/chaos; backup restoration, supported deployment migration timing,
encryption-key custody/drain rehearsal, alert routing; production cutover or any
real tenant export/purge/restore. Follow the separate runbook before release.

`data_lifecycle.enabled` and `purge_enabled` default false. Retention defaults
are conservative product policy, not asserted legal compliance. Financial data
cannot be purged by this release; operator-approved floors/holds must be checked
before activating other cleanup. A closed tenant may retain operational/security
configuration until its floor is eligible; Phase G does not promise a subsequent
automatic sweep of that configuration. Ciphertext download expiry is captured
at completion while cleanup applies current floors, so a raised floor can retain
an object longer than its download window. The single encryption key has no
old-key lookup: approved drain or a separate key-ring migration precedes rotation.

Deferred: complex legal-hold management and SIEM; Phase H global diagnostics/
operations; Phase I whole-platform chaos/load and broader legacy async inventory;
Phase K unified visual work; Phase L live manual/provider/storage/deployment/
backup/recovery acceptance. Phase G's own tenant/financial/concurrency safety
checks above were executed and are not deferred.
