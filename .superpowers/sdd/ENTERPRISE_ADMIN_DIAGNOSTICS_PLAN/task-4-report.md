# Phase H Task 4 report

Status: IMPLEMENTED; final review complete.

The Global Admin Workspace console now consumes the reviewed typed search,
overview, jobs, diagnostics and guarded-operation contracts. The page keeps
the existing administrator route and selectors, does not change the Workspace
store or impersonate a tenant, and does not render raw JSON or server error
text.

The Task 4 fix round addresses all three Important findings in
`task-4-final-review.md`:

- The visible advanced filter group now exposes workspace ID, owner and
  billing-owner IDs, workspace type, created/updated local date ranges, all
  supported status values, and all supported sort values. IDs are sent only
  when positive integers; local date inputs are converted to RFC3339 UTC at
  submit time.
- Workspace detail renders typed worker evidence for backlog/due lag, stored
  scan lag, oldest pending and pending-alert age, successful and failed
  timestamps, failure code/source/coverage/freshness, job source/observation
  timestamps, blockers, worker issues and runbooks. Rotation readiness remains
  read-only and shows capabilities, bounded inventories, prerequisites and
  risks without key material or decryption probes.
- Overview and jobs AbortControllers are retained and participate in the same
  cancellation and generation fence as search/detail reads. Refresh, auth
  revocation, selection changes and unmount cancel in-flight diagnostic reads.
- Browser session changes now clear the workspace result page and its
  pagination as well as detail, overview and job evidence. Inspecting a new
  Workspace clears the prior detail immediately; actions render only when the
  detail's Workspace ID matches the current selection, and submission repeats
  that target check.

Guarded suspend/resume and server-eligible dead Webhook retry retain captured
workspace/delivery targets, exact preconditions, bounded reason,
target-specific confirmation, UUID idempotency and captured-session proof
headers. The opt-in TOTP child contract sanitizes proof failures and ignores
late cancelled results while preserving existing callers' defaults.

Fresh focused and final-source evidence:

- Workspace Admin component behavior: `13/13`, including session-state clearing
  and the pending A-to-B inspection target-binding regression.
- Admin transport: `3/3`.
- TOTP child behavior: `2/2`.
- Webhook subscription regression: `19/19`.
- Locale completeness: `3/3`.
- Full frontend Vitest: `436/436` files and `3271/3271` tests, exit 0.
- `pnpm run lint:check`, `pnpm run typecheck`, and `pnpm run build`: exit 0.
- Canvas Seedance regression: `7/7`; i18n check: exit 0.
- Backend default and integration suites, native vet/build and golangci-lint
  2.14.0: exit 0 on the final backend source fence.
- Independent architecture, backend and frontend reviews found no remaining
  blocking issue after the final UI fixes. The Codex Security diff scan remains
  incomplete at discovery (0/40); it is not represented as a completed clean
  scanner result.

No authenticated browser, 18081 runtime, real provider, S3/IAM/TLS,
production, load/chaos, or key-drain/recovery rehearsal was run. Export-key
rotation remains a diagnostic-only blocked/unknown readiness result. Before
Phase I, the drain-or-key-ring, custody, all-instance rollout, rollback and
recovery plan must be approved; before Phase L, real private-storage
old/new-key readability and recovery drills must cover retained/held/orphan
objects, interrupted exports, key mismatch/loss and rollback.
