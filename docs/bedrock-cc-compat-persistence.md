# Bedrock CC compatibility persistence

`features_config.bedrock_cc_compat` is a channel-level **boolean**, independent
of the platform maps used by web search and the Codex image bridge. The switch
is displayed in the Anthropic section. Removing that section writes `false`.
Execution additionally requires an Anthropic account whose type is `bedrock`.

## Cause and data path

The form wrote `{ "anthropic": true/false }`, whereas form hydration and
`Channel.IsBedrockCCCompatEnabled` required a boolean. The API's untyped JSON
map, repository serialization and PostgreSQL JSONB preserved the object. Cache
reloads preserved the same mismatch; cache invalidation was not the cause.
Upstream `72c112164e6245812a2f1aef75e5ddd33ea16556` corrected the read side;
ModuRelay already had that change but retained the incompatible writer.

The corrected path is:

1. Form submissions explicitly write `true` or `false`, checked by the request type.
2. Admin create/update bind the feature map without dropping `false`.
3. Repository JSON serialization canonicalizes this key before storing JSONB.
4. Detail, list and cache batch reads all use the same canonicalization.
5. Channel updates rebuild the local cache and publish Redis invalidation to peers.
6. Form hydration displays the boolean; it also understands the known legacy
   shape for responses from an older server.
7. The gateway applies CC conversion only after selecting a Bedrock account.
   The original request header remains intact for beta policy and failover;
   the Bedrock forwarder filters beta tokens in its outgoing request.

## Compatibility and rollout

No schema migration or bulk rewrite is needed. Only the exact single-key object
`{ "anthropic": true }` or `{ "anthropic": false }` is recognized. Unknown keys,
multi-key objects, strings, arrays and null are disabled; unrelated feature keys
are preserved. A missing key remains missing and means disabled.

Reading an old row changes only the in-memory representation. The next normal
save persists a boolean. This also accepts submissions from a previously open
editor. There are no read-time database writes or changes to model mapping,
pricing, billing basis or group semantics.

After upgrading, a historical `{ "anthropic": true }` starts working as the
administrator originally requested, exclusively on Bedrock accounts. This is an
intentional behavior change. Upgrade all serving instances together: old
binaries do not normalize old rows and lack the new account guard. Redis
invalidation is best effort with the existing 10-minute local cache TTL as a
fallback. A rollback to the old gateway also restores its mixed-account bug.

Use a read-only inventory against the intended database before rollout:

```sql
BEGIN READ ONLY;
SELECT count(*) AS total_channels,
       count(*) FILTER (WHERE jsonb_typeof(features_config->'bedrock_cc_compat') = 'object') AS object_values,
       count(*) FILTER (WHERE jsonb_typeof(features_config->'bedrock_cc_compat') = 'boolean') AS boolean_values
FROM channels;
SELECT id, features_config->'bedrock_cc_compat' AS bedrock_cc_compat
FROM channels
WHERE features_config ? 'bedrock_cc_compat'
ORDER BY id;
COMMIT;
```

On 2026-10-14, the local acceptance database had zero channel rows. Production
was not inspected or modified. Tests seed both legacy values in an isolated
PostgreSQL database and verify that reads leave stored objects untouched until
the next save.

## Regression coverage

- Vue component interaction through the real channels API binding: create on/off,
  page remount, refresh, repeated edit, disabling/removing Anthropic, strict legacy
  hydration, and preservation of other configuration/mapping/pricing/group values.
- Repository JSON read/write round trips and non-mutation of caller-owned maps.
- Real Gin admin endpoints, PostgreSQL JSONB, list/detail responses, local cache
  refresh and cross-instance Redis invalidation, including lazy legacy repair.
- Gateway request conversion for mixed account types, disabled/inactive/missing
  channel configurations, actual Bedrock request construction for both streaming
  and non-streaming requests, beta filtering and Anthropic fallback preservation.

Gateway tests use a recorded HTTP upstream. Live AWS Bedrock credentials and
real provider acceptance are outside this local verification; no production
deployment is performed.
