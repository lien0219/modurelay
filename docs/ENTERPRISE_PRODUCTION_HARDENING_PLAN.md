# Phase I Production Hardening Implementation Plan

> Execute in the requested existing `feature/new-feature` checkout. Use independent
> audit/review agents for disjoint domains, with root coordinating migrations,
> shared interfaces and final verification. Preserve source evidence before fixes.

**Goal:** Preserve export readability and financial evidence while making failed
enterprise work durably recoverable and demonstrating the result in isolation.

**Architecture:** Existing SQL money/tenant authorities remain canonical. Add
versioned crypto and explicit rollout controls, repair current authorization and
lease boundaries, and persist recovery evidence before retryable side effects.

**Tech Stack:** Go1.27.2, PostgreSQL16/18.x, Redis, S3-compatible MinIO, Vue/Vitest.
**Spec:** Supplied Phase I task and `ENTERPRISE_PRODUCTION_HARDENING_ARCHITECTURE.md`.

## Global constraints

- Branch `feature/new-feature`; baseline `2e24b086d04c52fb4174e8f995a253bfe62e4989`.
- No push, PR, main/develop modification, deployment or18081 restart.
- Isolated synthetic data/keys only; never production Chaos/load/purge/rotation.
- Exact V1 read compatibility, V1 writes by default, unknown key fail closed.
- Once-effective settlement; immutable attribution; Total = Allocated + Unallocated.
- Preserve routes, permissions, payloads, behavior/selectors; no unrelated UI change.
- Historical migrations1–304 unchanged; new migrations305 onward only if needed.
- No weakened assertions/skips; final lint2.14.0; honest gate classifications.

## Task 1: Versioned export key compatibility and rollout

Files: config `data_lifecycle.go`/tests, service `lifecycle_artifact.go`, new
`lifecycle_keys.go`/tests, `lifecycle_runtime.go`/tests, repository crypto-readiness
adapter/integration tests, migration305, deployment config example and runbook.
Interfaces: explicit legacy/active/Key ID resolver; metadata-only SQL reader
registration and all-approved-reader consensus before V2 claims.

- [ ] Add RED tests: corrupted terminal record produces zero output; fixed old
  ciphertext remains readable; Key ID routing and invalid config fail closed.
- [ ] Run `go test ./internal/config ./internal/service -run 'TestLifecycle|TestDataLifecycle' -count=1`.
- [ ] Implement MRLEX02 with bounded explicit Key ID, separate authenticated
  derivation/header, active/decrypt-only registry, V1 wrappers and whole validation.
- [ ] Add real PostgreSQL readiness tests: both expected readers ready, missing/
  stale/different key bytes/status/inventory denied; unknown peers denied.
- [ ] Run focused unit/PG/race tests; independent security/compatibility review.

## Task 2: Live policy authorization and tenant matrix

Files: `policy_repo.go`, Workspace Project-scoped permissions and relevant
repository/service/middleware integration tests. Reuse locked Workspace/RBAC/
security evaluation rather than adding another identity model.

- [ ] Reproduce direct-repository policy updates after suspend/revoke/archive
  and viewer Project grants with real PostgreSQL.
- [ ] Repair current actor/tenant/grant/security checks and serialization.
- [ ] Verify all role/resource/operation cases and legacy/human/machine parity;
  record existing protections as test-backed evidence.

## Task 3: Financial recovery and async durability

Files: shared gateway/OpenAI billing, usage billing repository and recovery
service, media/cache/task stores and DI, lifecycle preflight, migration306 if
necessary, failure/replay integration tests. Preserve test-compatible constructors.

- [ ] RED: failed tenant billing must leave no zero-cost Usage/allocation snapshot.
- [ ] Persist bounded frozen billing command/evidence, replay via original money
  transaction; reconcile pending retry token and settlement receipt atomically.
- [ ] Persist async attempt before provider send; preserve accepted/unknown
  outcomes across Redis/process loss; never blind-resubmit or refund unknown work.
- [ ] Inject usage/outbox/finalize/Redis failures and replay; verify money,
  reservations, immutable row hashes and allocation conservation across tenants.

## Task 4: FinOps and worker recovery

Files: `workspace_finops_anomaly.go` and related tests; other evidenced worker
repairs only. Interfaces carry claim token through evidence persistence.

- [ ] RED: expired same-token completion and stale-token evidence are denied.
- [ ] Lock and validate live claim before writing snapshot/finding/event; check
  database-clock expiry and affected rows at completion.
- [ ] Verify concurrent claims, takeover, commit/outbox rollback, restart/replay,
  bounded backlog, drain and poison/retry behavior across worker families.

## Task 5: Isolated rehearsal, full verification and local boundary

Files: reproducible integration/rehearsal/benchmark tools and tests, five Phase I
documents and the roadmap. Evidence is under ignored `.cache/phase-i/`.

- [ ] S3 mixed V1/V2/held/retained/orphan/interrupted/corrupt/lost-key recovery.
- [ ] Full historical upgrade/replay, lock and partial-index recovery, financial
  hash preservation and database backup/restore on PostgreSQL16 and18.x.
- [ ] Bounded benchmark before/after: latency percentiles, throughput/errors,
  memory/CPU, connections/locks/worker lag/backlog; disclose scale limits.
- [ ] Full default/integration/unit-tag Go suites; relevant race; migrations;
  vet/build/golangci-lint2.14.0. Reproduce baseline failures with identical command.
- [ ] Full frontend Vitest/lint/typecheck/i18n/build and Canvas critical checks.
- [ ] Final-source Codex Security, dependency audits and independent diff review.
  Record INCOMPLETE/NOT VERIFIED if scan cannot complete.
- [ ] Final docs/roadmap/diff/secret/temporary-artifact review; local
  `fix(enterprise): harden workspace security and recovery`, no push/PR.
- [ ] Mark I COMPLETE/J NEXT only after every necessary local gate is satisfied.
