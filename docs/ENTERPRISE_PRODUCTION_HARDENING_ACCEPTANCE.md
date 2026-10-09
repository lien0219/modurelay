# Phase I Acceptance Record

Date: 2026-10-10. Branch: `feature/new-feature`. Starting source:
`2e24b086d04c52fb4174e8f995a253bfe62e4989`. All failure injection used
synthetic data and disposable dependencies.

## Results

| Area | Result | Evidence |
| --- | --- | --- |
| Key-ring config and MRLEX01/MRLEX02 unit behavior | PASS | `crypto-unit-final`, `crypto-config-green`, `crypto-final-focused` |
| PostgreSQL reader consensus and fail-closed claims | PASS | `crypto-readiness-pg`, `crypto-postgres16` |
| MinIO mixed-key, truncation, orphan, hold and retained-object recovery | PASS | `crypto-minio-recovery-2` |
| Tenant live authorization and project grants | PASS | `tenant-final-focused`, `tenant-green-live-policy`, `tenant-green-policy-atomicity`, `tenant-race-policy-runtime` |
| FinOps leases, fencing, retry exhaustion, dispatcher and SCIM lifecycle | PASS | `worker-finops-green-pg-2`, `worker-service-race`, `worker-recovery-regressions-pg`, `worker-admin-finops-pg-3` |
| Failed billing leaves no false usage receipt | PASS | `financial-unit-green-01`, `financial-sql-green-01` |
| Video/image SQL recovery after Redis loss | PASS | `financial-media-current-02` |
| Video provider attempt marker and frozen completion | PASS | `TestVideoMediaAttemptCompletionUsesFrozenAdmissionSnapshot`, `TestVideoMediaAttemptRejectionIsTerminal`, `TestVideoMediaAttemptRecoveryMarksUnknownWithoutResubmission` (focused PostgreSQL integration run, exit 0) |
| Batch SQL recovery and unreleased-hold detection | PASS | latest `TestBatchSQLRecoveryFindsAcceptedJobsAndFailedUnreleasedHolds` run, exit 0 |
| Affected package compile | PASS | `compile-all-current`; `go vet` and `go build` exit 0 |
| Full default Go suite | PRE-EXISTING / TOOLCHAIN BLOCKED | Three `backup_pg_dumper` tests require `sh`, absent from the Windows PATH; the same failure is present in the baseline receipt |
| Full unit-tag Go suite | PRE-EXISTING + TRANSIENT | The baseline Ollama CAS failure reproduces; the full run also observed one Grok cancellation subtest failure, but the exact focused test passed 5/5 immediately afterward. No Phase I diff touches that credential path. |
| Full integration-tag Go suite | PASS | `integration-full-sh-fixed.json`, exit code 0; all packages completed with Git `sh.exe` on PATH |
| Real providers, production S3/IAM/KMS, load/chaos, browser/manual acceptance | NOT RUN | Phase L release gates |
| Formal Codex Security scan | INCOMPLETE / NOT VERIFIED | Standard scan `79ed5fa4-aecc-4130-8b2d-d6996ddd6fe9` completed with 0 reportable findings for the reviewed Phase I surfaces, but canonical coverage is `partial` over the 5860-file repository inventory; worker-slot capability was `unknown` |

The default-suite failures are environment/baseline classifications, not
converted to passes and not hidden by changing assertions. The unit-tag Grok
failure was transient: the same focused test passed five consecutive times
after the full run. No new Phase I failure was found in the affected service,
handler, repository recovery, key, tenant, or worker gates.

## Release interpretation

The three requested controls have focused local evidence. Production release
remains blocked until the formal security coverage is complete, migration and
backup/restore rehearsal is recorded, and the external Phase L gates are
recorded.
