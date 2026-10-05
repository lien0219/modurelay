# Workspace webhooks

Workspace webhooks send committed tenant events to a customer's HTTPS endpoint.
Delivery is asynchronous and **at-least-once**. The HTTP worker does not run on
the business transaction, and receiver failures cannot undo committed billing.

## API and permissions

The settings page is `/workspaces/:workspaceId/webhooks`. API operations live
under `/api/v1/workspaces/:workspaceId/webhooks`:

| Method | Suffix | Permission |
| --- | --- | --- |
| GET | empty | `webhook.read` |
| POST | empty | `webhook.create` |
| PATCH | `/:webhookId` | `webhook.update` |
| DELETE | `/:webhookId` | `webhook.delete` |
| POST | `/:webhookId/rotate` | `webhook.secret.rotate` |
| POST | `/:webhookId/test` | `webhook.test` |
| GET | `/:webhookId/deliveries` | `webhook.delivery.read` |
| POST | `/:webhookId/deliveries/:deliveryId/retry` | `webhook.delivery.retry` |

Permissions extend the existing centralized Workspace RBAC map. Owner/Admin
have all webhook permissions, Developer can read endpoints/delivery history,
Billing can read endpoint metadata, and Viewer has no webhook access. Global
administrator status does not implicitly grant a workspace role or secret
access. Every repository operation scopes endpoint and delivery IDs to the
workspace; knowing another tenant's ID grants no access.

Create accepts `name`, `url` and `event_types`. Update accepts optional `name`,
`url`, `enabled` and `event_types`. An omitted subscription list preserves the
existing list. Create/rotation return `{ webhook, secret }`; subsequent reads
never return either the plaintext or encrypted secret. Secret display is
temporary and clears when leaving the context. There is a backend maximum of
ten endpoints per workspace, including disabled endpoints.

An endpoint test is limited to one per endpoint per minute and ten per workspace
per minute, enforced in PostgreSQL. It queues a real `webhook.test` event through
the signed server delivery pipeline and only targets the selected endpoint. It
never uses browser fetch. Deleting/recreating endpoints cannot reset the
workspace test limit.

Only the workspace-visible producer events listed in
[NOTIFICATIONS.md](NOTIFICATIONS.md) can be subscribed. Internal worker,
provider credential/health and global operator events are excluded.

Service Account subscriptions use the eight concrete event names:
`service_account.created`, `.updated`, `.disabled`, `.enabled`, and
`service_account.credential.created`, `.updated`, `.revoked`, `.rotated`
(expand each shorthand to its complete prefix when submitting `event_types`).
They use the same immutable envelope, HMAC, SSRF checks, lease, retry and
at-least-once delivery. Credential payloads expose safe IDs/names/expiration;
rotation includes `old_credential_id` and `new_credential_id`. Neither raw
secret nor lookup digest is delivered. Global Admin disable/revoke produces the
corresponding scoped tenant event, not platform-private risk notes.
Expiration event names are reserved without an automatic producer in this
release. See [SERVICE_ACCOUNTS.md](SERVICE_ACCOUNTS.md) and the
[manual acceptance runbook](SERVICE_ACCOUNTS_ACCEPTANCE.md).

## Immutable envelope

```json
{
  "id": "evt_00112233445566778899aabbccddeeff",
  "type": "budget.threshold_reached",
  "version": 1,
  "created_at": "2026-10-05T00:00:00Z",
  "workspace_id": 123,
  "project_id": 456,
  "subject": { "type": "project", "id": "456" },
  "data": {
    "scope_type": "project",
    "scope_id": 456,
    "period_start": "2026-10-01",
    "policy_revision": 3,
    "threshold": 80,
    "amount": 100,
    "spent": 81,
    "reserved": 0,
    "reason_code": "finalized_spend"
  }
}
```

Optional actor/project/workspace fields are omitted when not applicable.
Event IDs, delivery IDs and payload snapshots remain stable across retries,
manual retries and crash recovery. An externally breaking payload change requires
a new event version. Additive optional fields may keep version 1.

API key events expose only key ID, public name and project ID. Settlement events
expose public task/request ID, model/platform, estimated/actual amount and reason
code. No raw API key, key hash, provider account ID, credential, proxy or upstream
configuration is serialized.

## Signing protocol

Each attempt sends `Content-Type: application/json` and these headers:

```text
X-ModuRelay-Event-ID: evt_...
X-ModuRelay-Event-Type: budget.threshold_reached
X-ModuRelay-Delivery-ID: 1234
X-ModuRelay-Timestamp: 1791158400
X-ModuRelay-Delivery-Attempt: 1
X-ModuRelay-Signature: v1=<hex HMAC-SHA256>
```

The signature input is the timestamp header, a literal `.` and the exact raw
HTTP request body bytes. The HMAC key is the complete displayed `whsec_...`
string. Verify the raw bytes before parsing JSON or reserializing the envelope.

Receivers should reject timestamps outside a ±5 minute window, compare digests
in constant time, validate the event ID/type against the headers and perform
their business action idempotently by event ID. A fresh timestamp on a retry
does not make the event new. Store the idempotency receipt durably before
returning a success response.

### Python verification example

```python
import hashlib
import hmac
import time

def verify(raw_body: bytes, timestamp: str, signature: str, secret: str) -> bool:
    try:
        if abs(int(time.time()) - int(timestamp)) > 300:
            return False
    except ValueError:
        return False
    expected = hmac.new(
        secret.encode("utf-8"), timestamp.encode("ascii") + b"." + raw_body,
        hashlib.sha256,
    ).digest()
    matched = False
    for part in signature.split(","):
        key, sep, value = part.strip().partition("=")
        if key != "v1" or not sep:
            continue
        try:
            candidate = bytes.fromhex(value)
        except ValueError:
            continue
        matched |= hmac.compare_digest(expected, candidate)
    return matched
```

### Go verification example

```go
func verifyWebhook(secret string, body []byte, timestamp, signature string, now time.Time) bool {
    seconds, err := strconv.ParseInt(timestamp, 10, 64)
    if err != nil || seconds < now.Add(-5*time.Minute).Unix() || seconds > now.Add(5*time.Minute).Unix() {
        return false
    }
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write([]byte(timestamp + "."))
    mac.Write(body)
    expected := mac.Sum(nil)
    valid := false
    for _, part := range strings.Split(signature, ",") {
        value, ok := strings.CutPrefix(strings.TrimSpace(part), "v1=")
        if !ok { continue }
        actual, err := hex.DecodeString(value)
        if err == nil && hmac.Equal(expected, actual) { valid = true }
    }
    return valid
}
```

The Go example uses `crypto/hmac`, `crypto/sha256`, `encoding/hex`, `strconv`,
`strings` and `time`. Production receivers should also limit body size and verify
before acknowledging or performing actions.

## Secrets and rotation

Secrets are generated from cryptographic randomness, displayed once and
encrypted at rest through the project's existing secret encryptor. The service
requires a configured encryption key; it does not store plaintext as fallback.
Keep the instance encryption key stable across restarts and workers.

Rotation is audited and returns the new secret once. The previous encrypted
secret remains valid for a 24 hour grace period. During grace the signature
header contains both `v1=new_digest,v1=old_digest`; the receiver accepts either
matching digest. After expiry only the current secret is used. Retried deliveries
keep their immutable body and use the current signing material at send time.
Update the receiver during grace and retire the old key afterward.

## Delivery, retry and crash recovery

Successful 2xx responses mark the delivery succeeded. Network errors, timeouts,
408, 429 and 5xx retry with delays of 1m, 5m, 15m, 1h, 6h and 24h, capped at 24h,
plus 0–25% jitter. The worker stops after ten attempts. Other responses, including
400/401/403/404 and redirects, are terminal and become dead deliveries.

Owner/Admin may manually retry a dead delivery. This is audited and reuses the
same event and delivery identities; it never rebuilds the payload from current
business state. Pending, retrying, delivering and succeeded rows cannot be
manually requeued. Disabling an endpoint pauses new claims/sends; an HTTP request
already in flight may complete. Re-enabling permits eligible pending work to
continue.

Claims use `FOR UPDATE SKIP LOCKED`, an expiring lease and a fresh token.
Expired work is reclaimable and stale workers cannot acknowledge a new claim.
If HTTP succeeds but the process crashes before persisting success, another
attempt can arrive. This is why receiver-side event ID idempotency is required.
It is not an exactly-once external delivery guarantee.

The client has bounded connection, TLS, response header and total timeouts;
total attempt time is ten seconds. At most 64 KiB of response body is read.
Delivery history stores a sanitized, credential-redacted preview of at most
1024 bytes, HTTP status, attempt/timing fields and bounded error information.

## Outbound security

Endpoints must be public HTTPS URLs without userinfo. Localhost, loopback,
private, link-local/metadata, multicast, reserved/documentation ranges and
IPv4-mapped IPv6 addresses are rejected. A valid hostname is checked at creation
and resolved again on every attempt. All resolved IPs must be allowed and the
transport connects to the validated pinned IP while retaining TLS hostname
verification. Environment proxies and redirects are disabled.

Customers cannot supply Authorization, Cookie, Host or proxy headers. The
platform sends only its protocol headers and HMAC. URL checks and test limits
are backend controls, not UI-only validation.

Succeeded delivery history is retained for 90 days, dead history for 180 days.
Pending/retrying deliveries and active leases are protected from cleanup.
See [DOMAIN_EVENTS_ACCEPTANCE.md](DOMAIN_EVENTS_ACCEPTANCE.md) for the complete
signature, SSRF, retry, concurrency and billing regression runbook.
