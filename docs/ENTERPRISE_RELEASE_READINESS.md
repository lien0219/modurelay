# Enterprise Release Readiness

This checklist is a release boundary for Phase I. It does not authorize a
deployment or a change to the `18081` acceptance runtime.

Current gate decision: **BLOCKED**. The Phase I focused implementation gates
have evidence, while the formal security scan is `INCOMPLETE / NOT VERIFIED`
because its canonical coverage is partial. Production backup/restore,
migration lock/load/chaos, key-rotation, provider/storage/KMS, deployment and
manual acceptance remain open.

## Required preflight

- [ ] Confirm the exact candidate SHA, migration head, image digest, and clean
      reviewed diff.
- [ ] Take a PostgreSQL backup and restore it into an isolated database; compare
      schema, immutable usage/allocation rows, reservations, and recovery rows.
- [ ] Rehearse migrations 1 through 307 on PostgreSQL 16 and 18.x with lock,
      timeout, partial-failure, restart, and invalid-index recovery evidence.
- [ ] Verify Redis loss, process restart, worker takeover, and provider unknown
      outcome recovery with disposable dependencies.
- [ ] Complete the formal security scan and resolve every reportable P0/P1
      finding. The recorded scan `79ed5fa4-aecc-4130-8b2d-d6996ddd6fe9` has
      0 reportable findings in the reviewed Phase I surfaces, but partial
      coverage means this gate remains `INCOMPLETE / NOT VERIFIED`.
- [ ] Run the frontend Vitest, lint, typecheck, i18n, production build, and
      Canvas critical checks.

## Key and rollout order

1. Deploy a binary that reads MRLEX01 and MRLEX02 while V2 writes remain off.
2. Register every instance with the same key-ring and inventory fingerprint.
3. Verify live reader heartbeats, rollback compatibility, and old-key
   readability in isolated storage.
4. Enable V2 writes only through an explicit release control after the consensus
   gate passes. Keep old decrypt-only keys for the full protected-object floor.
5. If a mismatch appears, disable V2 writes and roll back to a binary that can
   read both formats. Never delete a key because a queue is empty.

## Worker and failure procedure

- Drain new work, let durable leases expire or be explicitly fenced, and record
  the final pending/live/terminal counts.
- Stop one worker at a time; verify another worker can reclaim due work without
  duplicating evidence or settlement.
- Preserve unknown upstream outcomes and frozen financial commands for replay.
- After restart, compare settlement fingerprints, allocation totals, hold state,
  outbox identities, and immutable snapshots before reopening traffic.

## Blocking conditions

Block release for any key-ring mismatch, missing historical key, failed
authentication tag, changed immutable attribution, duplicate settlement,
unfenced stale worker, failed backup restore, unknown migration state, formal
security scan gap, or unclassified full-suite regression. Real provider,
production storage/KMS, browser/manual, deployment, and production load gates
remain Phase L requirements even when local gates pass.
