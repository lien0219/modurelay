# Domain events acceptance runbook

This runbook covers reliable events, the Notification Center, workspace webhooks
and budget/video settlement alerts. Run destructive fault injection only on an
isolated test database and controlled receivers. Local development does not
require a production cutover or real-provider spend.

## Preparation

1. Record branch, HEAD and dirty state. Keep `feature/new-feature`, and do not
   reset or replace unrelated work. New migrations are 280–283; 273–279 retain
   their original contents.
2. Use a disposable PostgreSQL/Redis environment. The repository integration
   harness provisions separate containers and replays all embedded migrations.
   Do not replay these migrations against the existing acceptance app merely
   to run tests.
3. Create workspace A with Owner, Admin, Developer, Billing and Viewer users,
   a project with an API key, and unrelated workspace B. Retain endpoint and
   delivery IDs for scoped authorization checks.
4. For external delivery, use a controlled public HTTPS receiver that records
   raw request bytes and headers, verifies the signature, and stores a durable
   event ID receipt. Keep secrets out of screenshots and logs.
5. Verify encryption configuration before creating an endpoint. Run at least two
   app instances sharing the isolated database/Redis for concurrency scenarios.

## Automated checks

Run from `backend` using the project's Go toolchain:

```text
go test ./...
go test -tags=unit ./...
go test -tags=integration ./...
go vet ./...
go build ./...
golangci-lint run --timeout=30m ./...
```

Focused repository coverage includes `TestDomainEvent*`, `TestNotification*`,
`TestWorkspaceWebhook*`, the existing `TestBudget*`, workspace/key isolation and
`TestVideo*` settlement suites. Service coverage includes immutable envelope,
dispatcher replay, HMAC/SSRF/retry and Seedance/Grok recovery tests. Record exact
commands, exit codes and failing test names; a summary from a prior run is not
current verification.

Run from `frontend`:

```text
pnpm typecheck
pnpm lint:check
pnpm test:run
pnpm build
```

Use `package.json` scripts if names change. From the repository root also run
`git diff --check` and the existing frontend/video critical regression target.
Verify that no new `test.skip`, `test.only` or fail-open branch was introduced.

When a full check fails, create a **detached** baseline worktree at the change's
parent commit and run the same failing check with the same environment. A failure
reproduced there is `PRE-EXISTING`; a passing parent is a `REGRESSION` requiring
repair. Record Windows shell/toolchain gaps separately. An unavailable real
receiver/provider check is `NOT RUN`, never PASS.

## Functional scenarios

| Scenario | Action and expected evidence |
| --- | --- |
| A. Workspace suspension | Suspend workspace A as a global administrator. Existing gateway admission rejects new work, active members receive `workspace.suspended`, and their authenticated inbox still reads the notice. Resume and observe its new event. |
| B. Finalized 50% | Set a $100 policy. Finalize $49 then $2. Exactly one 50% transition/event/inbox row/delivery per subscribed endpoint; further finalized spend below 80% adds none. |
| C. Finalized 80% | Move actual spend from $79 to $81. One 80% transition; reservation/release alone does not create it. |
| D. Hard rejection | Exhaust a hard policy, then submit 20 rejected requests. One 100% hard transition for that scope/month/revision. First-ever oversized request also alerts once without ghost counters/reservations. Check the distinct `admission_rejected` reason. |
| E. Signature | Deliver to the controlled receiver. Check canonical headers, version 1 and a valid raw-body HMAC within ±5 minutes. |
| F. Tamper and replay | Change one raw-body byte, timestamp or secret. Verification fails. Reusing an already accepted event ID performs no second receiver action, even with a new valid retry timestamp. |
| G. Transient receiver | Return 500, then 500, then 200. Observe bounded retrying state, scheduled backoff/jitter and eventual success with the same payload, event ID and delivery ID. |
| H. Terminal receiver | Return 400/401/403/404. Delivery becomes dead without endless automatic retry. Manual retry requires `webhook.delivery.retry`, is audited and keeps the original event. |
| I. SSRF | Attempt HTTP, localhost, loopback, private, metadata/link-local, mapped IPv6, reserved/multicast and userinfo URLs. Backend rejects every prohibited case. A controlled public HTTPS destination is accepted. |
| J. Redirect/rebinding | Resolve to a public IP at creation, then private at delivery, or return a redirect to a private URL. The worker blocks the request/redirect and does not contact the private target. |
| K. Rotation | Rotate once, copy the returned secret, and confirm later GET/history contains no secret. Within the 24h grace verify either `v1` digest; after expiry only the new secret works. Repeated retry uses the unchanged body. |
| L. Tenant escape | As workspace B's user, attempt list/get/update/delete/test/rotate/delivery-read/retry with A's known IDs. Return denied/not-found, never data or secret. Repeat with Developer/Billing/Viewer to verify the permission matrix. |
| M. Pending Seedance/Grok | Complete a controlled accepted video whose actual cost exceeds a hard project's remaining capacity. Wallet/Usage/Budget changes roll back, reservation remains pending, and one public pending event/inbox row/delivery persists. Repeat 20 polls. |
| N. Recovered settlement | Increase budget and retry recovery concurrently. One wallet debit, usage row, budget finalization and recovered event; 20 more polls/restarts remain idempotent. Force recovered-event insertion to fail on an isolated DB and prove all money/recovered-state changes roll back together. |
| O. Two workers | Run two dispatchers and two delivery workers against one queue. Claims do not overlap within a lease. Partial fanout replay cannot duplicate inbox/delivery rows. A stale token cannot acknowledge a reclaimed row. |
| P. Crash and lease | Stop a worker after claim and after HTTP response but before success persistence. Expiry reclaims abandoned work. Receiver may see a duplicate; its event ID receipt suppresses a second action. |
| Q. Announcements | Header retains `AnnouncementBell`. Existing unread/read, dropdown and announcement navigation still work independently. |
| R. Resources | Generate existing request/collaboration/chat Resource Center activity. The unified center preserves counts, read actions and original destinations without changing its backend API. |

## Additional state and lifecycle checks

- Register a user, including a rolled-back registration. Successful personal
  creation has one event; rollback has none. Concurrent lazy ensures never
  duplicate it. Existing personal workspaces do not acquire historical notices.
- Create/update/revoke a project key. Event snapshots retain original public
  names and attribution; raw creation secrets are absent in event, audit,
  notification and delivery history. Failed mutations leave no event.
- Suspend/remove a member. Owner/Admin and the affected user receive the action
  notice; unrelated Viewer users do not. A removed member receives no further
  ordinary financial/project notices.
- Increase a policy at already-crossed spend. New revision seed rows exist,
  with no replayed historical alert. A later actual crossing emits under the
  new revision. Run both scope timezones across a month boundary.
- Disable a claimed endpoint before sending. No new request starts once the
  worker observes disable; an already-started request may finish. Re-enable to
  resume eligible work. Changing URL or subscriptions does not rebuild events.
- Create ten endpoints, including disabled endpoints. An eleventh is rejected
  by the backend. Test cooldown and aggregate workspace test limit survive
  repeated clicks, multiple instances and endpoint deletion/recreation.
- Serve a huge/credential-echoing response. Reads remain bounded at 64 KiB;
  history is at most 1024 bytes, escaped in the UI and stripped of secrets.
- Test both locales and light/dark themes on desktop/tablet/430px/390px. Check
  keyboard focus, loading/error/empty/disabled states, delivery pagination,
  filters, one-time secret dismissal and stale responses during workspace/user
  changes. Implementation keys must not appear as user-facing captions.
- Inject 10,000 outbox events. Each claim is bounded and uses the due index;
  concurrency and memory do not grow with the whole queue.
- Expire retained rows in the isolated DB. Cleanup removes eligible history in
  bounded batches and preserves pending/retrying rows and active leases. A
  pending settlement does not lose its dedup state when event history expires.
- Stop the server during processing. Dispatcher and worker cancel and join;
  work without a committed acknowledgement remains recoverable.

## Result recording

For each scenario record PASS, FAIL, PRE-EXISTING or NOT RUN with commit, event
IDs, sanitized evidence and relevant counter/receipt counts. Keep real secrets,
provider credentials and full response bodies out of the report. Record real
external receiver and real upstream acceptance independently from mock/local
integration results.
