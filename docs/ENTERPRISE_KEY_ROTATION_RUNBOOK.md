# Enterprise Export Key Rotation Runbook

This runbook describes a controlled future rotation. It is not a production
rotation record and must not be executed against real objects without an
approved release change.

## Preconditions

- Confirm lifecycle export and purge switches and record the candidate SHA.
- Inventory held, retained, pending, running, interrupted, orphan, and failed
  object attempts from SQL and private storage.
- Preserve the old key in trusted secret custody and verify an isolated restore
  of representative MRLEX01 and MRLEX02 artifacts.
- Configure a unique active key ID/version and a decrypt-only entry for every
  historical key still required by retention or legal/financial protection.
- Register all instances with matching key-ring and inventory fingerprints;
  require live heartbeats and `rollback_compatible=true` before enabling V2.

## Rotation sequence

1. Roll out read-compatible code everywhere with V2 writes disabled.
2. Add the new key as `active`; keep the prior key `decrypt_only`.
3. Re-run the reader-consensus gate and isolated old/new-key download tests.
4. Enable V2 writes through the release control. New artifacts carry an
   explicit key ID/version in the authenticated header.
5. Monitor failed decrypts, unknown-key errors, orphan attempts, and recovery
   backlog. Keep the old key available while any protected object may require
   it.
6. For rollback, disable V2 writes and return to the compatible binary. Do not
   remove the new key until all V2 objects are either retained with custody or
   safely retired under the lifecycle policy.

## Fail-closed rules

Unknown key IDs, missing historical keys, invalid headers, scope mismatch,
truncated ciphertext, and authentication-tag failures return no plaintext.
There is no trial-decryption loop and no key destruction based solely on queue
emptiness. A reader inventory mismatch pauses claims rather than risking an
artifact written for an incompatible rollback target.

## Recovery evidence

Record the key ID/version, reader fingerprints, object ledger counts, backup and
restore identifiers, failed-attempt list, and operator approvals. Never place
key bytes, raw secrets, or decrypted export contents in SQL, logs, Git, tickets,
or acceptance artifacts.
