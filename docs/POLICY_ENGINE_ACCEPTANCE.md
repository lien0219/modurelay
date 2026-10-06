# Policy Engine Acceptance Runbook

This runbook defines the later manual acceptance for hierarchical policy,
gateway admission, quota accounting, and notifications. It is a test plan,
not evidence that these scenarios have passed. Run it only against an isolated
environment with disposable workspaces, projects, service accounts, groups,
credentials, budgets, and provider fixtures. Do not use production tenants or
credentials.

## Before Testing

- Deploy the candidate backend, frontend, migrations, and Redis configuration
  together; record the image/build ID and database migration version.
- Create at least two workspaces with two projects each, a composite group, a
  dedicated service account, separate test credentials, and test-only budgets.
- Use deterministic provider fixtures or provider accounts approved for
  acceptance. Record which cases use fixtures and which reach a real upstream.
- Capture request ID, policy scope/revision, quota counters or ledger entries,
  selected platform/model, HTTP or protocol result, and relevant event/outbox
  records for each case. Redact tokens and secrets.
- For concurrency cases, reset counters and use synchronized starts. Repeat
  each race case enough times to exercise concurrent admission.
- Record unavailable provider/protocol cases as `NOT RUN`; do not infer a pass
  from unit tests or policy configuration alone.
- For asynchronous image cases, also inject a policy-quota release/finalize
  failure and restart the worker. The current implementation persists the
  reservation ID but has no durable settlement consumer, so this recovery
  scenario is expected to remain `NOT RUN`/a known accounting gap until that
  consumer is implemented.

## Workspace, Project, and Service Account

| Case | Setup and action | Expected result |
| --- | --- | --- |
| A. Parent and child model allowlists | Set Workspace to GPT and Claude; set Project to GPT. Send one Claude request and one GPT request through the Project credential. | Claude is rejected by the Project layer; GPT passes policy admission. A child allowlist cannot reopen a parent-denied model. |
| B. Explicit deny-all | Leave Workspace unrestricted; set Project `allowed_models` to `[]`. Send a request using each otherwise valid credential in that Project. | Every model request is rejected. The API and UI preserve `[]`, distinct from `null`. |
| C. Multi-scope RPM | Configure Workspace RPM 100, Project 50, Service Account 20, and Credential 10. Drive concurrent requests through that Credential. | The effective limit is 10 requests per minute, with no more than 10 admitted in the configured window. Other credentials remain subject to their own and parent counters. |
| D. Workspace and Project request quota | Set Workspace monthly requests to 100 and one Project to 20. Exhaust that Project, then send requests to a second Project in the same Workspace. | Further requests to the exhausted Project are rejected. The second Project can continue while the Workspace has remaining quota. The shared Workspace counter never exceeds its limit. |
| E. Service Account quota | Set a Service Account request quota lower than its Project quota. Submit through two credentials belonging to that Service Account, then through a different Service Account. | The credentials share the Service Account quota; exhaustion blocks that principal. The other Service Account remains eligible subject to its own hierarchy. |
| F. Hard token quota | Set a low hard token limit and send a request whose accounted usage crosses it. | Admission/reservation prevents usage from exceeding the hard limit according to the documented reservation policy; rejection leaves no leaked reservation. |
| G. Streaming settlement | Send a streaming request that completes normally, then cancel a second request after partial output. | Usage is finalized exactly once from reliable observed usage. Partial/cancelled streams follow the documented settlement rule, and no unknown usage is recorded as zero. |
| H. Responses continuation after policy change | Start a Responses conversation, tighten its parent model policy, then submit a continuation referencing the previous response. | The continuation is checked against the current policy revision before provider dispatch. A parent change takes effect without waiting for a long cache TTL. |
| I. Composite platform filtering | Configure a composite group containing OpenAI and Claude candidates and an allowed platform policy containing OpenAI only. | Only OpenAI candidates are eligible. A Claude request is rejected or routed only to an allowed candidate according to the public-model contract; it must not reach Claude. |
| J. Alias and mapping bypass | Add a model alias/prefix or provider mapping that resolves to a model excluded by a parent policy. Submit using both the public alias and its direct name. | Both requests are denied after canonical and resolved-model checks. Aliases and mappings cannot widen access. |
| K. Seedance model allowlist | Allow one exact Seedance model and submit it plus a different Seedance model. | The configured model passes admission; the other is rejected before provider dispatch. Verify normal Seedance create/poll/content behavior for the allowed model. |
| L. Seedance platform deny | Deny the Seedance platform at a parent or child layer and submit an otherwise allowed Seedance model. | The request is rejected before task creation. Existing Seedance account/model routing does not bypass platform policy. |
| M. Seedance polling request count | Create a Seedance task within quota, then poll its status and content until complete. | Task creation consumes the configured logical request quota once. Poll/status/content operations follow their documented accounting contract and do not repeatedly consume create-request quota. |
| N. Grok compatibility | Submit a policy-allowed Grok request, then repeat with a denied model/platform and exercise the supported asynchronous lifecycle. | Allowed traffic follows existing Grok routing and accounting. Denied traffic is blocked before upstream dispatch; status/content polling does not double-charge logical request quota. |
| O. Image generation | Exercise each supported image create/edit endpoint with an allowed request, then with a denied model/platform. | Allowed requests reserve and settle according to the image usage contract; denied requests do not reach the provider or leave quota reservations behind. |
| P. Live/WebSocket | Open a Live/WebSocket session, submit multiple model turns, then close normally and with client cancellation. Also return a provider `2xx` without `Location` and force the Live mapping store to fail. | Each logical turn is admitted against current policy and the documented RPM/request/token rules. Session lifetime and control frames do not bypass limits or cause duplicate settlement. An accepted but uncorrelated create is retained in a recovery-only record and finalized once after the maximum session window; if that record cannot be stored, the request-only fallback is visible and no pending reservation is silently released. |
| Q. Batch logical items | Submit a batch containing 100 logical requests/items. Repeat with a quota below 100 and with a quota at 100. | Accounting charges the documented number of logical items, not only one HTTP batch envelope. Under-quota admission rejects or processes only according to the all-or-nothing contract; no 1000-item envelope bypass is possible. |
| R. Revision conflict | Load one policy revision in two administrator sessions. Save a change from the first session, then attempt a stale save from the second. | The stale update returns `409`; it does not overwrite the first change. The console offers reload before retry. |
| S. Immediate policy update | Keep a credential making requests, update its Workspace policy, and continue requests without restarting services. | New admissions use the updated revision immediately or within the explicitly documented near-real-time bound; behavior does not wait for a long cache TTL. |
| T. Parent tightening | Configure a child policy that allows a model, then tighten the Workspace or Project parent to deny it. | The child cannot reopen access. Existing cached effective data is invalidated or bypassed quickly enough to enforce the parent change. |
| U. Service Account creator removal | Create and configure a Service Account and credential, remove or suspend the creator, then issue a request using the machine credential. | Execution follows the Service Account ownership and status contract, not the former creator's interactive session. Disabled/revoked machine credentials remain rejected. |
| V. Cross-tenant IDOR | As a member of Workspace A, request and mutate Workspace B policy IDs, including Project and Service Account IDs that exist in B. | Reads and writes are denied without disclosing policy contents. No row, audit event, or domain event is changed in Workspace B. |
| W. Quota threshold notification | Configure a quota threshold and drive usage across it. | The threshold notification is emitted once per documented threshold/cooldown and uses the localized policy/quota event labels. |
| X. Webhook on quota exhaustion | Configure an authorized webhook subscription and exhaust a policy quota. | The expected quota-exhausted event is delivered or retried through the outbox; payload contains safe scope/revision metadata and no credential secret. |
| Y. Budget plus policy quota | Configure both a budget and a policy quota. Test once where policy rejects and once where budget admission rejects. | Either gate can reject. A reservation made by an earlier gate is released exactly once if a later gate fails; successful requests settle policy usage, budget, and billing once. |

## Direct API Key Compatibility Gate

This gate is mandatory after every Policy Engine or machine-identity change. It
starts from the legacy user-facing `/keys` flow: the test user does not create
an organization workspace, custom project, or Service Account. `APIKeyService.Create`
must lazily resolve the Personal Workspace and Default Project, create a human
credential with `principal_type=user` semantics, and leave
`service_account_id` `NULL`. A Service Account policy is loaded only for a
credential whose server-resolved `service_account_id` is non-NULL.

The automated integration test is
`TestDirectAPIKeyCompatibilityGate` in
`backend/internal/repository/direct_api_key_compatibility_integration_test.go`.
The listed unit and migration tests are complementary regression evidence; a
real provider or browser result is not implied by those tests.

| # | Required compatibility case | Automated evidence | Live acceptance status |
| --- | --- | --- | --- |
| 1 | Existing users do not need to visit a Workspace page. | `/keys` is an independent authenticated route; direct `Create` lazily provisions the personal tenant. | `NOT RUN` browser flow |
| 2 | Existing users can open the API-key page directly. | `policy-routes.spec.ts` resolves `/keys` without the Workspace route. | `NOT RUN` browser flow |
| 3 | Creating an ordinary API Key succeeds. | `TestDirectAPIKeyCompatibilityGate` creates a key through `APIKeyService.Create`. | `PASS` automated; `NOT RUN` browser |
| 4 | `service_account_id` remains `NULL`. | SQL snapshot and `ExecutionPrincipal` assertions in the integration gate. | `PASS` automated |
| 5 | The Secret creation contract is unchanged. | A supplied custom Secret is returned once and stored unchanged. | `PASS` automated |
| 6 | Legacy group selection remains attached to the key. | The selected `group_id` is persisted and existing group authorization is exercised. | `PASS` automated |
| 7 | Existing API Key model restrictions remain effective. | Legacy Group and API-key restrictions are projected by `TestPolicyAdmissionProjectsLegacyGroupAndCredentialLayers`. | `PASS` automated |
| 8 | Existing Key RPM and quota behavior remains effective. | Direct-key fields are persisted unchanged; existing rate-window repository tests remain the runtime guard. | `PASS` automated |
| 9 | GPT calls continue to work. | Policy admission is covered; no real GPT credential is used by this test suite. | `NOT RUN` real upstream |
| 10 | Claude calls continue to work. | Policy admission is covered; no real Claude credential is used by this test suite. | `NOT RUN` real upstream |
| 11 | Grok calls continue to work. | Grok routing/settlement tests are separate; no real Grok credential is used here. | `NOT RUN` real upstream |
| 12 | Seedance and Infinite Canvas calls continue to work for ordinary Keys. | Direct-key identity and policy admission are covered; no real video/Canvas provider call is made here. | `NOT RUN` real upstream/browser |
| 13 | Usage is attributed to Personal Workspace, Default Project, API Key, and User Execution Principal. | The integration gate queries all tenant, key, user, and `service_account_id` Usage snapshot columns. | `PASS` automated |
| 14 | Billing Principal is correct. | Tenant Usage Billing records the user as `billing_principal_user_id`. | `PASS` automated |
| 15 | Budget attribution and finalization are correct. | The real budget reservation is created with the user actor and finalized exactly once by Usage Billing. | `PASS` automated |
| 16 | A Service Account is not required. | Direct creation completes before any machine identity is created. | `PASS` automated |
| 17 | A Project is not manually required. | `PersonalProject` creates/returns the Default Project inside direct creation. | `PASS` automated |
| 18 | A Workspace is not manually required. | `EnsurePersonalWorkspace` is reached lazily from the direct creation path. | `PASS` automated |
| 19 | Existing historical Key Secrets do not change. | Migration guard forbids API-key rewrites; the direct-key snapshot verifies the stored Secret is unchanged. | `PASS` automated; `NOT RUN` historical production replay |
| 20 | Existing Keys are not migrated into Service Accounts. | Service-account migrations contain no historical `api_keys` or Usage backfill and preserve nullable human attribution. | `PASS` migration tests |
| 21 | Default Policy configuration does not change behavior. | Missing Workspace/Project rows and every nullable limit/allowlist resolve as unrestricted inheritance. | `PASS` automated |
| 22 | An explicit Personal Workspace Policy later limits ordinary Keys. | Workspace model/RPM policy narrows the direct-key effective policy. | `PASS` automated |
| 23 | An explicit Project Policy is inherited by ordinary Keys. | Project model/platform/RPM policy composes with the Workspace layer. | `PASS` automated |
| 24 | Service Account Policy never applies to ordinary Keys. | Direct-key middleware and resolver tests assert no Service Account layer; machine keys still honor it. | `PASS` automated |
| 25 | Disabling or deleting a Service Account does not affect ordinary Keys. | The integration gate disables the machine identity, deletes the no-credential test row through SQL (there is no application delete endpoint), then re-authenticates the unchanged direct Secret successfully. | `PASS` automated for disable and database removal; application delete flow `NOT RUN`/not available |

## Evidence and Exit Criteria

For every case, record `PASS`, `FAIL`, or `NOT RUN`, the build/migration IDs,
the sanitized request ID, and links or identifiers for relevant audit, outbox,
notification, quota, budget, and provider-fixture evidence. A case passes only
when the policy result, provider dispatch decision, counter/reservation state,
and settlement side effects all match the expected result.

Stop the run on any cross-tenant access, provider dispatch after a policy
denial, counter overshoot, leaked/double-finalized reservation, duplicate
settlement, or credential disclosure. Preserve test evidence and report the
case as failed; do not repair test data in a way that hides the failure.

Manual execution status: `NOT RUN` by this document's authoring change.
