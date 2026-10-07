# Enterprise SCIM 2.0 provisioning

SCIM manages organization Workspace membership and attributed Team grants. It is
an independent provisioning connector, never an SSO provider or a browser login.
OIDC/SAML still authenticate people; Require SSO protects the human management
API while connector Bearer requests use their own authenticated protocol routes.

## Connector and credential lifecycle

Owner/Admin manage connectors through `/api/v1/workspaces/:id/scim-connectors`.
Permissions are centralized as `provisioning.read`, `provisioning.manage` and
`provisioning.token.rotate`. Mutations require recent human authentication and
recheck Workspace permission under the same transaction lock as the write.
Personal Workspaces cannot create connectors.

Each connector has an opaque `public_endpoint_id`, revision, active/disabled
status, default non-Owner role, explicit Group mode and safe operational state.
The Base URL is `https://<configured-origin>/scim/v2/<public_endpoint_id>`; the
origin comes only from retained `enterprise_sso.redirect_url` configuration,
never HTTP Host or an upstream callback. Configure HTTPS before creation.

Credentials use `mrc_scim_` plus 32 cryptographically random bytes encoded as
base64url. The plaintext appears only in the immediate token-create response.
Storage retains SHA-256, a display prefix and lifecycle timestamps. GET/list,
audit, events, inbox notifications and Webhooks never return the secret. Up to
eight active credentials coexist for rotation. Create a replacement, change the
directory configuration, verify synchronization, then revoke the old token.
Optional expiry must be in the future and within 366 days. Revoke/disable/expiry
is revalidated inside resource transactions; authentication has no token cache.

Disable stops authentication and further synchronization. Existing attributed
access stays in place until explicitly deprovisioned or administratively changed.
This prevents an accidental directory outage from becoming a bulk deprovision.

## Protocol contract

All endpoints require an active connector, active unexpired matching token and
active organization Workspace. Use `Authorization: Bearer <one-time-secret>`;
tenant and connector are derived exclusively from the authenticated credential.
Requests accept `application/scim+json` or compatible `application/json` and
responses use `application/scim+json`. Errors use the SCIM Error schema, HTTP
status as a string, optional `scimType` and a safe detail. Human control errors
retain the platform envelope and stable reason codes.

| Endpoint suffix | Supported operations |
| --- | --- |
| `/ServiceProviderConfig` | GET |
| `/ResourceTypes`, `/ResourceTypes/User`, `/ResourceTypes/Group` | GET |
| `/Schemas`, `/Schemas/<User-or-Group-URN>` | GET |
| `/Users`, `/Groups` | GET list, POST create |
| `/Users/:id`, `/Groups/:id` | GET, PUT, PATCH, DELETE |

Patch, equality filters and ETags are supported. Bulk, sorting, password changes,
enterprise schema extensions and `.search` POST are not supported. Capability
discovery reports these limits honestly. Unknown/control attributes are rejected;
payload fields cannot choose Workspace, internal user/member IDs, role or Owner.

List responses use `ListResponse`, `totalResults`, 1-based `startIndex`,
`itemsPerPage` and `Resources`. Default count is 100; count zero returns total
without resources. Count is limited to 100 and offset to 100,000. Only one of
each parameter is accepted. Supported `eq` filters are `id` and `externalId`,
plus User `userName` and Group `displayName`. Values must be JSON quoted strings;
compound predicates, sorting and other operators return an explicit error.
All SQL predicates are selected from a closed column map and parameterized.

Resource IDs are stable random opaque values, independent of global database
IDs. Meta exposes the configured resource URL, creation/modification timestamps
and `W/"<revision>"`. Optional If-Match accepts one quoted positive revision;
stale mutation returns 412. PUT replaces supported writable attributes. PATCH
executes against the current resource under the mutation lock, not an earlier
HTTP snapshot. Revisions increase only for real changes. DELETE tombstones the
resource and repeated DELETE returns 204. Tombstoned IDs/externalIds are retained.
User DELETE also removes its exact connector-local Group references and increments
affected live Group revisions/timestamps. Later Group updates do not encounter
ghost references; active=false retains assignments for reactivation.

PATCH accepts case-insensitive add/replace/remove and common Entra/Okta no-path
object forms. User paths include active, userName, externalId, displayName, name,
name.givenName, name.familyName, emails and allowlisted email equality selectors.
Group paths include displayName, externalId, members and
`members[value eq "<opaque-user-id>"]`. Duplicate additions/absent removals are
idempotent. Removing required userName/displayName is rejected. Removing active
restores default true, subject to the authoritative suspension checks below.

## Users and global identity safety

User attributes are schemas, id, externalId, userName, active, name, displayName
and emails. Exactly one primary inbox is required, except a single email is
implicitly primary. Existing Global User adoption requires normalized exact
inbox matching, a verified Workspace domain and a previously verified email
identity. Evidence may be a verified global email identity, or a verified exact
email at link in this Workspace from an active OIDC/SAML provider. Aliases,
arbitrary external domains and unverified existing identities are not adopted.

Absent users receive an unknowable random bcrypt password and a verified email
identity whose provenance is authenticated domain-scoped provisioning. No known
password is returned. Existing global email/name/password remain unchanged;
SCIM names and secondary emails update only the resource. The bound primary
inbox is immutable. Normal email-ownership recovery remains the existing flow.
Provisioning does not create an OIDC/SAML subject binding. Provisioned accounts,
including new SCIM-created accounts, follow the shared authenticated explicit
SSO-linking boundary. When no prior login credential/binding exists, mailbox
ownership recovery supplies the normal authenticated bootstrap before linking.

Deactivation and DELETE remove only that SCIM membership source and its Team
sources. They never delete Global Users, Personal/other Workspace memberships,
sessions, keys, Service Accounts, historical usage or billing snapshots.
All requests targeting an Owner are rejected with 409. Billing Owner
deactivation/deletion is rejected with 409 until an authorized person transfers
billing ownership. Connector roles allow viewer/developer/admin/billing only.

## Typed source attribution

Migration 296 adds `workspace_membership_sources` and
`workspace_team_membership_sources`. Manual, OIDC, SAML and SCIM sources coexist;
typed nullable provider/connector/resource references have exclusivity checks and
composite tenant foreign keys. OIDC/SAML source type must match provider type.
Legacy single-value columns remain compatible materialized views of the result.

An active source keeps Workspace membership active unless an administrator
suspension/tombstone overrides it. Manual role wins; otherwise an existing active
managed role source is retained, then the earliest stable active source is chosen.
There is no invented numeric ranking between incomparable billing/developer
roles. Removing any source affects that exact source only. OIDC/SAML complete
empty groups remove only the corresponding provider grants; incomplete/missing
group claims retain them. SCIM active=true cannot clear administrative suspension.

Manual removal records administrative suspension/removal and removes the manual source,
preserving identity/resource history. Explicit administrator activation can clear
the tombstone. Removed actors retain the prior tenant 404 boundary; suspension
without removal retains 403. Insert provenance covers owner constructors and invitations. A
prebound SSO login can restore source-deprovisioned access while still respecting
administrative suspension; explicit identity linking requires active membership.

## Groups, Teams and project authorization

A SCIM Group has stable opaque ID, externalId, displayName and connector-local
User references. Owner/Admin explicitly binds it to an existing active Team in
the same Workspace. Unmapped groups store resources without granting access.
Matching displayName never takes over a manual Team. Rename/delete never
renames/deletes that Team or changes its Project Access Grants.

Group members contribute exact connector/Group Team sources. Group delete,
member removal and User deactivation remove only those sources. Other manual,
OIDC, SAML or SCIM Group sources stay. User reactivation restores its existing
Group assignments while administrative suspension still prevents effective
admission. Project access uses the existing Team to Project Access Grant engine;
SCIM introduces no alternate ACL and no Gateway protocol hot-path dependency.

User `PATCH add emails` preserves existing order and skips an already-present
supported complex email value. Comparison lowercases/trims `value` using the
existing repository normalization; `type`, `primary` and `display` retain exact
decoded equality. Distinct fields remain distinct, and conflicting primary
values still fail validation. Raw incoming and resulting resources each retain
the 20-email bound. Identical pathful/pathless retries preserve resource contents,
ETag, lastModified and mutation audit/event/outbox records. Existing stored
duplicates are not rewritten by this correction.

## Bounds, concurrency and operations

Workspace locks serialize manual/SSO/SCIM source writes, resource PATCH, connector
configuration and token revoke/disable. Token/connector revision, wall-clock expiry and
Workspace state are rechecked after taking the lock. PostgreSQL transaction-start
time is not used for that expiry boundary or the after-lock expiry scan. Email uniqueness uses the
existing advisory-lock discipline. Resource requests have a 20-second deadline.

Request bodies are at most 256 KiB, JSON depth 16 and 16,000 tokens; duplicate
keys/case variants and trailing values are rejected. PATCH has at most 32
operations, 200 incoming members per operation and 5,000 stored Group members.
POST/PUT also bound incoming members. String/email attributes and lists are
bounded independently. Large complete replacement may reach payload limits
before the stored-member ceiling; use bounded PATCH batches.

Existing Redis rate limiting applies at 600 requests/minute/IP before auth,
300/connector and 150/token after auth. Keys contain scalar IDs or IP hashes,
never bearer values. Missing/down Redis fails closed with 503; excess returns
429 and Retry-After. All protocol responses use no-store.

Audit and outbox writes are transactional scalar summaries, without raw payload
or email attributes. External sync has no human actor. Successes are silent in
the inbox but retain audit/Webhooks. Connector disabled, protected-identity
conflict, repeated failure (threshold three, 15-minute debounce) and impending
token expiry notify Owner/Admin. Every authenticated write admitted by the rate
limits records exactly one outcome, including media/JSON/schema/attribute/ID/
precondition rejection. Reads, unauthenticated traffic and limit rejections are
excluded. Reporting failure never changes a committed mutation's HTTP result.
Last-used writes are debounced five minutes;
successful last-sync writes one minute. Expiry is detected during sync and by a
bounded background scan independent of client traffic, with once-daily alerts.

## Migration and compatibility

`296_enterprise_identity_scim.sql` changes only identity/control-plane tables.
Existing suspended members conservatively become administratively suspended.
Backfill includes stable provider bindings and every identity Team grant.
Legacy unbound SCIM rows have no connector evidence and are preserved as manual
sources rather than inventing provenance. Rerun does not reapply initial
suspension or override newer source state. Existing migrations 294/295 are not
edited. Historical Usage/Billing/Key/Service Account data is not rewritten.

SCIM adds no dependency. Go uses the existing crypto/rand, SHA-256, bcrypt,
PostgreSQL and Redis components. SAML dependencies remain documented separately.
Local protocol, real PostgreSQL/source/concurrency and final verification evidence
is recorded in [ENTERPRISE_SCIM_ACCEPTANCE.md](ENTERPRISE_SCIM_ACCEPTANCE.md).
Real Entra/Okta provisioning and operator/browser/load/deployment acceptance are
**NOT RUN**, reserved for Phase L.
