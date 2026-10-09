//go:build integration

package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

const adminRetryEventType = "webhook.administrator_retried"

type adminOperationSnapshot struct {
	Workspace string
	Delivery  string
	Writes    [4]int
}

func snapshotAdminOperation(t *testing.T, workspaceID, deliveryID int64) adminOperationSnapshot {
	t.Helper()
	var s adminOperationSnapshot
	require.NoError(t, integrationDB.QueryRow(`SELECT to_jsonb(w)::text FROM workspaces w WHERE id=$1`, workspaceID).Scan(&s.Workspace))
	if deliveryID > 0 {
		require.NoError(t, integrationDB.QueryRow(`SELECT to_jsonb(d)::text FROM workspace_webhook_deliveries d WHERE id=$1`, deliveryID).Scan(&s.Delivery))
	}
	require.NoError(t, integrationDB.QueryRow(`SELECT (SELECT count(*) FROM workspace_audit_logs WHERE workspace_id=$1),(SELECT count(*) FROM domain_events WHERE workspace_id=$1),(SELECT count(*) FROM domain_event_outbox o JOIN domain_events e ON e.id=o.event_id WHERE e.workspace_id=$1),(SELECT count(*) FROM enterprise_admin_operations WHERE workspace_id=$1)`, workspaceID).Scan(&s.Writes[0], &s.Writes[1], &s.Writes[2], &s.Writes[3]))
	return s
}

func requireAdminWriteSet(t *testing.T, before, after adminOperationSnapshot, count int) {
	t.Helper()
	for i := range before.Writes {
		require.Equal(t, before.Writes[i]+count, after.Writes[i], "audit/event/outbox/receipt count at index %d", i)
	}
}

func adminOperationContext(ctx context.Context) context.Context {
	now := time.Now().UTC()
	return service.WithRecentAuthentication(service.WithSessionAuthentication(ctx, service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: now}), now)
}

func adminStatusInput(workspace *service.Workspace, action string) service.AdminOperationInput {
	return service.AdminOperationInput{Action: action, Reason: "operator maintenance", Confirmation: action + ":" + formatID(workspace.ID), IdempotencyKey: uuid.NewString(), ExpectedUpdatedAt: workspace.UpdatedAt.Format(time.RFC3339Nano)}
}

func adminDeadDeliveryFixture(t *testing.T) (context.Context, *workspaceRepository, *workspaceWebhookRepository, *service.WorkspaceWebhookService, *service.User, *service.User, *service.Workspace, *service.WorkspaceWebhook, *service.WorkspaceWebhookDelivery, service.AdminOperationInput) {
	t.Helper()
	ctx, _, webhookSvc, webhookRepo, owner, workspace := workspaceWebhookFixture(t)
	admin := enterpriseAdminActor(t)
	endpoint := createWorkspaceWebhookFixture(t, ctx, webhookSvc, owner.ID, workspace.ID)
	event, err := service.NewDomainEvent(service.EventWorkspaceUpdated, workspace.ID, 0, owner.ID, "workspace", formatID(workspace.ID), service.DomainEventData{"status": "updated"})
	require.NoError(t, err)
	delivery, err := webhookRepo.CreateTestDelivery(ctx, owner.ID, workspace.ID, endpoint.ID, event)
	require.NoError(t, err)
	last := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	_, err = integrationDB.Exec(`UPDATE workspace_webhook_deliveries SET status='dead',attempts=2,last_attempt_at=$2,finished_at=$2,last_error='HTTP 500',error_code='delivery_terminal',response_status=500,response_preview='upstream failed',locked_at=NULL,lock_owner=NULL WHERE id=$1`, delivery.ID, last)
	require.NoError(t, err)
	in := service.AdminOperationInput{Action: "retry_webhook", Reason: "requeue dead delivery", Confirmation: "retry_webhook:" + formatID(delivery.ID), IdempotencyKey: uuid.NewString(), ExpectedAttempts: 2, ExpectedLastAttemptAt: last.Format(time.RFC3339Nano)}
	return adminOperationContext(ctx), NewWorkspaceRepository(integrationDB).(*workspaceRepository), webhookRepo, webhookSvc, owner, admin, workspace, endpoint, delivery, in
}

func deliveryEnvelope(t *testing.T, id int64) string {
	t.Helper()
	var envelope string
	require.NoError(t, integrationDB.QueryRow(`SELECT jsonb_build_object('id',id,'workspace_id',workspace_id,'webhook_id',webhook_id,'event_id',event_id,'event_type',event_type,'payload',payload)::text FROM workspace_webhook_deliveries WHERE id=$1`, id).Scan(&envelope))
	return envelope
}

func TestEnterpriseAdminRetryUsesDedicatedEventAndExplicitSubscribers(t *testing.T) {
	ctx, repo, webhookRepo, webhookSvc, owner, admin, w, endpoint, d, in := adminDeadDeliveryFixture(t)
	before := snapshotAdminOperation(t, w.ID, d.ID)
	envelope := deliveryEnvelope(t, d.ID)
	_, err := repo.AdminRetryWebhook(ctx, admin.ID, w.ID, endpoint.ID, d.ID, in)
	require.NoError(t, err)
	var payload []byte
	var eventType string
	require.NoError(t, integrationDB.QueryRow(`SELECT event_type,payload FROM domain_events WHERE workspace_id=$1 AND actor_user_id=$2 AND subject_type='webhook_delivery' AND subject_id=$3`, w.ID, admin.ID, formatID(d.ID)).Scan(&eventType, &payload))
	require.Equal(t, adminRetryEventType, eventType)
	var event service.DomainEvent
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	require.NoError(t, decoder.Decode(&event))
	require.NoError(t, event.Validate(), "durable dispatcher decoding preserves the validated integer ID")
	require.Equal(t, service.DomainEventData{"previous_status": "dead", "status": "retrying", "delivery_id": json.Number(formatID(d.ID))}, event.Data)
	require.Equal(t, envelope, deliveryEnvelope(t, d.ID))
	requireAdminWriteSet(t, before, snapshotAdminOperation(t, w.ID, d.ID), 1)
	recipients, err := NewNotificationRecipientResolver(integrationDB).Resolve(ctx, &event)
	require.NoError(t, err)
	require.Empty(t, recipients, "retry evidence is audit/webhook only")
	require.NoError(t, webhookRepo.EnqueueEventDeliveries(ctx, &event))
	var deliveries int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_webhook_deliveries WHERE event_id=$1`, event.ID).Scan(&deliveries))
	require.Zero(t, deliveries, "workspace.updated subscriptions do not receive administrator retry events")
	subscriber, _, err := webhookSvc.Create(ctx, owner.ID, w.ID, service.CreateWorkspaceWebhookInput{Name: "Retry observer", URL: "https://8.8.8.8/retry", EventTypes: []string{adminRetryEventType}})
	require.NoError(t, err)
	require.NoError(t, webhookRepo.EnqueueEventDeliveries(ctx, &event))
	require.NoError(t, webhookRepo.EnqueueEventDeliveries(ctx, &event))
	var webhookID int64
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*),min(webhook_id) FROM workspace_webhook_deliveries WHERE event_id=$1`, event.ID).Scan(&deliveries, &webhookID))
	require.Equal(t, 1, deliveries)
	require.Equal(t, subscriber.ID, webhookID)
}

func TestEnterpriseAdminRetryReplayBindsWebhookParent(t *testing.T) {
	ctx, repo, _, _, _, admin, w, endpoint, d, in := adminDeadDeliveryFixture(t)
	first, err := repo.AdminRetryWebhook(ctx, admin.ID, w.ID, endpoint.ID, d.ID, in)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE workspace_webhook_deliveries SET status='succeeded',attempts=1,delivered_at=now(),finished_at=now() WHERE id=$1`, d.ID)
	require.NoError(t, err)
	before := snapshotAdminOperation(t, w.ID, d.ID)
	replay, err := repo.AdminRetryWebhook(ctx, admin.ID, w.ID, endpoint.ID, d.ID, in)
	require.NoError(t, err, "identical replay precedes mutable state and attempt guards")
	require.Equal(t, first, replay)
	_, err = repo.AdminRetryWebhook(ctx, admin.ID, w.ID, endpoint.ID+100000, d.ID, in)
	require.ErrorIs(t, err, service.ErrAdminOperationIdempotencyConflict)
	require.Equal(t, before, snapshotAdminOperation(t, w.ID, d.ID))
}

func TestEnterpriseAdminConcurrentChangedTargetKeyRollsBackLoser(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, _, _, a := workspaceFixture(t)
	_, _, _, b := workspaceFixture(t)
	admin := enterpriseAdminActor(t)
	repo := NewWorkspaceRepository(integrationDB).(*workspaceRepository)
	before := []adminOperationSnapshot{snapshotAdminOperation(t, a.ID, 0), snapshotAdminOperation(t, b.ID, 0)}
	gateID := a.ID + 820000000
	_, err := integrationDB.Exec(fmt.Sprintf(`CREATE FUNCTION phase_h_pause_admin_receipt() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_advisory_xact_lock(%d); RETURN NEW; END $$; CREATE TRIGGER phase_h_pause_admin_receipt BEFORE INSERT ON enterprise_admin_operations FOR EACH ROW EXECUTE FUNCTION phase_h_pause_admin_receipt()`, gateID))
	require.NoError(t, err)
	gate, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = gate.Rollback() }()
	_, err = gate.Exec(`SELECT pg_advisory_xact_lock($1)`, gateID)
	require.NoError(t, err)
	raceCtx, cancel := context.WithTimeout(adminOperationContext(ctx), 8*time.Second)
	defer cancel()
	type result struct {
		index   int
		receipt *service.AdminOperationReceipt
		err     error
	}
	results := make(chan result, 2)
	key := uuid.NewString()
	for i, w := range []*service.Workspace{a, b} {
		in := adminStatusInput(w, "suspend")
		in.IdempotencyKey = key
		go func(index int, workspaceID int64, input service.AdminOperationInput) {
			receipt, err := repo.AdminOperateWorkspace(raceCtx, admin.ID, workspaceID, input)
			results <- result{index, receipt, err}
		}(i, w.ID, in)
	}
	defer func() { _ = gate.Rollback(); cancel() }()
	require.Eventually(t, func() bool {
		var waiting int
		err := integrationDB.QueryRow(`SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'INSERT INTO enterprise_admin_operations%'`).Scan(&waiting)
		return err == nil && waiting == 2
	}, 800*time.Millisecond, 5*time.Millisecond, "both transactions must pass receipt prechecks before the unique-index race")
	require.NoError(t, gate.Commit())
	successes, conflicts := 0, 0
	for range 2 {
		got := <-results
		w := []*service.Workspace{a, b}[got.index]
		after := snapshotAdminOperation(t, w.ID, 0)
		if got.err == nil {
			successes++
			require.NotNil(t, got.receipt)
			requireAdminWriteSet(t, before[got.index], after, 1)
			require.True(t, got.receipt.ResultUpdatedAt.After(w.UpdatedAt))
		} else {
			conflicts++
			require.ErrorIs(t, got.err, service.ErrAdminOperationIdempotencyConflict)
			require.Nil(t, got.receipt)
			require.Equal(t, before[got.index], after, "the receipt unique-index loser rolls back all four writes and the target")
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
}

func TestEnterpriseAdminOperationsReloadLiveGlobalAuthority(t *testing.T) {
	for _, tc := range []struct {
		name, actorUpdate   string
		owner               bool
		enrolled, hint, mfa bool
		proofOffset         time.Duration
		want                error
	}{
		{name: "workspace-owner-is-not-global-admin", owner: true, want: service.ErrWorkspaceForbidden},
		{name: "inactive-admin", actorUpdate: "status='disabled'", want: service.ErrWorkspaceForbidden},
		{name: "deleted-admin", actorUpdate: "deleted_at=now()", want: service.ErrWorkspaceForbidden},
		{name: "demoted-admin", actorUpdate: "role='user'", want: service.ErrWorkspaceForbidden},
		{name: "live-enrollment-overrides-false-hint", enrolled: true, want: service.ErrRecentAuthenticationRequired},
		{name: "enrollment-without-proof", enrolled: true, hint: true, want: service.ErrRecentAuthenticationRequired},
		{name: "valid-mfa-proof", enrolled: true, mfa: true},
		{name: "live-unenrolled-overrides-true-hint", hint: true},
		{name: "future-proof", proofOffset: 2 * time.Minute, mfa: true, want: service.ErrRecentAuthenticationRequired},
		{name: "expired-proof", proofOffset: -11 * time.Minute, mfa: true, want: service.ErrRecentAuthenticationRequired},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, repo, _, _, owner, admin, w, endpoint, d, in := adminDeadDeliveryFixture(t)
			if tc.owner {
				admin = owner
			}
			if tc.actorUpdate != "" {
				_, err := integrationDB.Exec(`UPDATE users SET `+tc.actorUpdate+` WHERE id=$1`, admin.ID)
				require.NoError(t, err)
			}
			_, err := integrationDB.Exec(`UPDATE users SET totp_enabled=$2 WHERE id=$1`, admin.ID, tc.enrolled)
			require.NoError(t, err)
			now := time.Now().UTC()
			ctx = service.WithRecentAuthentication(service.WithSessionAuthentication(ctx, service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: now.Add(-time.Hour), MFAEnrolled: tc.hint}), now.Add(tc.proofOffset), tc.mfa)
			before := snapshotAdminOperation(t, w.ID, d.ID)
			var membershipsBefore int
			require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_members WHERE workspace_id=$1 AND user_id=$2`, w.ID, admin.ID).Scan(&membershipsBefore))
			_, retryErr := repo.AdminRetryWebhook(ctx, admin.ID, w.ID, endpoint.ID, d.ID, in)
			_, statusErr := repo.AdminOperateWorkspace(ctx, admin.ID, w.ID, adminStatusInput(w, "suspend"))
			if tc.want != nil {
				require.ErrorIs(t, retryErr, tc.want)
				require.ErrorIs(t, statusErr, tc.want)
				require.Equal(t, before, snapshotAdminOperation(t, w.ID, d.ID))
			} else {
				require.NoError(t, retryErr)
				require.NoError(t, statusErr)
				requireAdminWriteSet(t, before, snapshotAdminOperation(t, w.ID, d.ID), 2)
			}
			var membershipsAfter int
			require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_members WHERE workspace_id=$1 AND user_id=$2`, w.ID, admin.ID).Scan(&membershipsAfter))
			require.Equal(t, membershipsBefore, membershipsAfter, "Global Admin never bootstraps tenant membership")
		})
	}
}

func TestEnterpriseAdminWorkspaceDistinctKeysCommitOneFencedTransition(t *testing.T) {
	for _, actions := range [][2]string{{"suspend", "suspend"}, {"resume", "resume"}, {"suspend", "resume"}} {
		t.Run(actions[0]+"-"+actions[1], func(t *testing.T) {
			isolateWorkspaceTestFixtures(t)
			ctx, _, _, w := workspaceFixture(t)
			admin := enterpriseAdminActor(t)
			repo := NewWorkspaceRepository(integrationDB).(*workspaceRepository)
			ctx, cancel := context.WithTimeout(adminOperationContext(ctx), 8*time.Second)
			defer cancel()
			if actions[0] == "resume" {
				receipt, err := repo.AdminOperateWorkspace(ctx, admin.ID, w.ID, adminStatusInput(w, "suspend"))
				require.NoError(t, err)
				w.Status, w.UpdatedAt = "suspended", receipt.ResultUpdatedAt
			}
			before := snapshotAdminOperation(t, w.ID, 0)
			type result struct {
				receipt *service.AdminOperationReceipt
				err     error
			}
			results := make(chan result, 2)
			start := make(chan struct{})
			for _, action := range actions {
				in := adminStatusInput(w, action)
				go func(input service.AdminOperationInput) {
					<-start
					receipt, err := repo.AdminOperateWorkspace(ctx, admin.ID, w.ID, input)
					results <- result{receipt, err}
				}(in)
			}
			close(start)
			successes, conflicts := 0, 0
			var successful *service.AdminOperationReceipt
			for range 2 {
				got := <-results
				if got.err != nil {
					require.ErrorIs(t, got.err, service.ErrWorkspaceConflict)
					require.Nil(t, got.receipt)
					conflicts++
				} else {
					require.NotNil(t, got.receipt)
					successful = got.receipt
					successes++
				}
			}
			require.Equal(t, 1, successes)
			require.Equal(t, 1, conflicts)
			requireAdminWriteSet(t, before, snapshotAdminOperation(t, w.ID, 0), 1)
			var status string
			var version time.Time
			require.NoError(t, integrationDB.QueryRow(`SELECT status,updated_at FROM workspaces WHERE id=$1`, w.ID).Scan(&status, &version))
			require.Equal(t, successful.ResultStatus, status)
			require.Equal(t, successful.ResultUpdatedAt, version)
			require.True(t, version.After(w.UpdatedAt))
		})
	}
}

func TestEnterpriseAdminWorkspaceOperationRejectsInvalidFencesAndProtectedStates(t *testing.T) {
	for _, state := range []string{"active", "archived", "pending_deletion", "purging", "deleted"} {
		t.Run(state, func(t *testing.T) {
			isolateWorkspaceTestFixtures(t)
			ctx, _, owner, w := workspaceFixture(t)
			admin := enterpriseAdminActor(t)
			repo := NewWorkspaceRepository(integrationDB).(*workspaceRepository)
			if state != "active" {
				if state != "archived" {
					jobID := uuid.NewString()
					_, err := integrationDB.Exec(`INSERT INTO workspace_deletion_jobs(id,workspace_id,requested_by_user_id,previous_status,state,available_at,earliest_purge_at) VALUES($1,$2,$3,'active','pending',now()+interval '7 days',now()+interval '7 days')`, jobID, w.ID, owner.ID)
					require.NoError(t, err)
					_, err = integrationDB.Exec(`UPDATE workspaces SET status='pending_deletion' WHERE id=$1`, w.ID)
					require.NoError(t, err)
					if state == "purging" || state == "deleted" {
						_, err = integrationDB.Exec(`UPDATE workspace_deletion_jobs SET state='running',lease_token=$2,lease_expires_at=now()+interval '5 minutes' WHERE id=$1`, jobID, uuid.NewString())
						require.NoError(t, err)
						_, err = integrationDB.Exec(`UPDATE workspaces SET status='purging' WHERE id=$1`, w.ID)
						require.NoError(t, err)
						if state == "deleted" {
							_, err = integrationDB.Exec(`UPDATE workspace_deletion_jobs SET state='completed',business_closed=true,completed_at=now() WHERE id=$1`, jobID)
							require.NoError(t, err)
						}
					}
				}
				_, err := integrationDB.Exec(`UPDATE workspaces SET status=$2 WHERE id=$1`, w.ID, state)
				require.NoError(t, err)
			}
			before := snapshotAdminOperation(t, w.ID, 0)
			for _, action := range []string{"suspend", "resume"} {
				in := adminStatusInput(w, action)
				if state == "active" && action == "suspend" {
					in.ExpectedUpdatedAt = w.UpdatedAt.Add(-time.Microsecond).Format(time.RFC3339Nano)
				}
				_, err := repo.AdminOperateWorkspace(adminOperationContext(ctx), admin.ID, w.ID, in)
				require.ErrorIs(t, err, service.ErrWorkspaceConflict)
				require.Equal(t, before, snapshotAdminOperation(t, w.ID, 0))
			}
		})
	}
	for _, tc := range []struct {
		name   string
		mutate func(*service.AdminOperationInput)
	}{
		{"wrong-target-confirmation", func(in *service.AdminOperationInput) { in.Confirmation = "suspend:100000" }},
		{"missing-version", func(in *service.AdminOperationInput) { in.ExpectedUpdatedAt = "" }},
		{"invalid-reason", func(in *service.AdminOperationInput) { in.Reason = "\u0085" }},
		{"invalid-key", func(in *service.AdminOperationInput) { in.IdempotencyKey = uuid.Nil.String() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateWorkspaceTestFixtures(t)
			ctx, _, _, w := workspaceFixture(t)
			admin := enterpriseAdminActor(t)
			before := snapshotAdminOperation(t, w.ID, 0)
			in := adminStatusInput(w, "suspend")
			tc.mutate(&in)
			_, err := NewWorkspaceRepository(integrationDB).(*workspaceRepository).AdminOperateWorkspace(adminOperationContext(ctx), admin.ID, w.ID, in)
			require.ErrorIs(t, err, service.ErrWorkspaceInvalid)
			require.Equal(t, before, snapshotAdminOperation(t, w.ID, 0))
		})
	}
}

func TestEnterpriseAdminResumeRequiresLivePayerAndOwner(t *testing.T) {
	for _, mode := range []string{"payer-inactive", "payer-not-member", "owner-inactive", "no-owner"} {
		t.Run(mode, func(t *testing.T) {
			isolateWorkspaceTestFixtures(t)
			ctx, svc, owner, w := workspaceFixture(t)
			payer := workspaceJoin(t, ctx, svc, owner.ID, w.ID, "billing")
			admin := enterpriseAdminActor(t)
			_, err := integrationDB.Exec(`UPDATE workspaces SET billing_owner_user_id=$2,status='suspended' WHERE id=$1`, w.ID, payer.ID)
			require.NoError(t, err)
			query := map[string]string{
				"payer-inactive":   `UPDATE workspace_members SET status='suspended' WHERE workspace_id=$1 AND user_id=$2`,
				"payer-not-member": `DELETE FROM workspace_members WHERE workspace_id=$1 AND user_id=$2`,
				"owner-inactive":   `UPDATE workspace_members SET status='suspended' WHERE workspace_id=$1 AND role='owner'`,
				"no-owner":         `UPDATE workspace_members SET role='admin' WHERE workspace_id=$1 AND role='owner'`,
			}[mode]
			if mode == "payer-inactive" || mode == "payer-not-member" {
				_, err = integrationDB.Exec(query, w.ID, payer.ID)
			} else {
				_, err = integrationDB.Exec(query, w.ID)
			}
			require.NoError(t, err)
			before := snapshotAdminOperation(t, w.ID, 0)
			_, err = NewWorkspaceRepository(integrationDB).(*workspaceRepository).AdminOperateWorkspace(adminOperationContext(ctx), admin.ID, w.ID, adminStatusInput(w, "resume"))
			require.ErrorIs(t, err, service.ErrWorkspaceConflict)
			require.Equal(t, before, snapshotAdminOperation(t, w.ID, 0))
		})
	}
}

func TestEnterpriseAdminRetryRejectsUnsafeStateScopeAndAttemptFences(t *testing.T) {
	for _, mode := range []string{"succeeded", "delivering", "pending", "retrying", "foreign-workspace", "foreign-endpoint", "foreign-delivery", "disabled-endpoint", "inactive-workspace", "claim-owner", "locked-at", "stale-attempts", "stale-last-attempt", "missing-expected-last-attempt", "null-stored-last-attempt", "wrong-confirmation"} {
		t.Run(mode, func(t *testing.T) {
			ctx, repo, webhookRepo, webhookSvc, owner, admin, w, endpoint, d, in := adminDeadDeliveryFixture(t)
			workspaceID, webhookID, deliveryID := w.ID, endpoint.ID, d.ID
			want := service.ErrWorkspaceConflict
			switch mode {
			case "succeeded", "delivering", "pending", "retrying":
				_, err := integrationDB.Exec(`UPDATE workspace_webhook_deliveries SET status=$2 WHERE id=$1`, d.ID, mode)
				require.NoError(t, err)
			case "foreign-workspace":
				_, _, _, foreign := workspaceFixture(t)
				workspaceID = foreign.ID
				want = service.ErrWorkspaceNotFound
			case "foreign-endpoint":
				other := createWorkspaceWebhookFixture(t, ctx, webhookSvc, owner.ID, w.ID)
				webhookID = other.ID
				want = service.ErrWebhookDeliveryNotFound
			case "foreign-delivery":
				_, _, otherOwner, other := workspaceFixture(t)
				otherEndpoint := createWorkspaceWebhookFixture(t, ctx, webhookSvc, otherOwner.ID, other.ID)
				e, err := service.NewDomainEvent(service.EventWebhookTest, other.ID, 0, otherOwner.ID, "workspace", formatID(other.ID), nil)
				require.NoError(t, err)
				otherDelivery, err := webhookRepo.CreateTestDelivery(ctx, otherOwner.ID, other.ID, otherEndpoint.ID, e)
				require.NoError(t, err)
				deliveryID = otherDelivery.ID
				in.Confirmation = "retry_webhook:" + formatID(deliveryID)
				want = service.ErrWebhookDeliveryNotFound
			case "disabled-endpoint":
				_, err := integrationDB.Exec(`UPDATE workspace_webhooks SET enabled=false WHERE id=$1`, endpoint.ID)
				require.NoError(t, err)
			case "inactive-workspace":
				_, err := integrationDB.Exec(`UPDATE workspaces SET status='suspended' WHERE id=$1`, w.ID)
				require.NoError(t, err)
			case "claim-owner":
				_, err := integrationDB.Exec(`UPDATE workspace_webhook_deliveries SET lock_owner='worker' WHERE id=$1`, d.ID)
				require.NoError(t, err)
			case "locked-at":
				_, err := integrationDB.Exec(`UPDATE workspace_webhook_deliveries SET locked_at=now() WHERE id=$1`, d.ID)
				require.NoError(t, err)
			case "stale-attempts":
				in.ExpectedAttempts++
			case "stale-last-attempt":
				in.ExpectedLastAttemptAt = time.Now().UTC().Format(time.RFC3339Nano)
			case "missing-expected-last-attempt":
				in.ExpectedLastAttemptAt = ""
			case "null-stored-last-attempt":
				_, err := integrationDB.Exec(`UPDATE workspace_webhook_deliveries SET last_attempt_at=NULL WHERE id=$1`, d.ID)
				require.NoError(t, err)
			case "wrong-confirmation":
				in.Confirmation = "retry_webhook:100000"
			}
			before := snapshotAdminOperation(t, w.ID, d.ID)
			envelope := deliveryEnvelope(t, d.ID)
			_, err := repo.AdminRetryWebhook(ctx, admin.ID, workspaceID, webhookID, deliveryID, in)
			require.ErrorIs(t, err, want)
			require.Equal(t, before, snapshotAdminOperation(t, w.ID, d.ID))
			require.Equal(t, envelope, deliveryEnvelope(t, d.ID))
		})
	}
}

func TestEnterpriseAdminOperationsRollBackEveryWriteFailure(t *testing.T) {
	for _, operation := range []string{"suspend", "retry_webhook"} {
		for _, table := range []string{"workspace_audit_logs", "domain_events", "domain_event_outbox", "enterprise_admin_operations"} {
			t.Run(operation+"/"+table, func(t *testing.T) {
				ctx, repo, _, _, _, admin, w, endpoint, d, in := adminDeadDeliveryFixture(t)
				before := snapshotAdminOperation(t, w.ID, d.ID)
				envelope := deliveryEnvelope(t, d.ID)
				_, err := integrationDB.Exec(`CREATE FUNCTION phase_h_fail_admin_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'phase_h_injected_insert_failure:` + table + `'; END $$; CREATE TRIGGER phase_h_fail_admin_write BEFORE INSERT ON ` + table + ` FOR EACH ROW EXECUTE FUNCTION phase_h_fail_admin_write()`)
				require.NoError(t, err)
				if operation == "suspend" {
					_, err = repo.AdminOperateWorkspace(ctx, admin.ID, w.ID, adminStatusInput(w, "suspend"))
				} else {
					_, err = repo.AdminRetryWebhook(ctx, admin.ID, w.ID, endpoint.ID, d.ID, in)
				}
				var pgErr *pq.Error
				require.ErrorAs(t, err, &pgErr)
				require.Equal(t, "phase_h_injected_insert_failure:"+table, pgErr.Message, "operation reached the selected INSERT failure point")
				require.Equal(t, before, snapshotAdminOperation(t, w.ID, d.ID), "all target fields/version and full four-write set rollback")
				require.Equal(t, envelope, deliveryEnvelope(t, d.ID))
			})
		}
	}
}

func TestEnterpriseAdminRetryEventMigrationValidatesProtocolAndReplays(t *testing.T) {
	ctx, _, _, _, _, admin, w, _, d, _ := adminDeadDeliveryFixture(t)
	body, err := migrations.FS.ReadFile("304_enterprise_admin_webhook_retry_event.sql")
	require.NoError(t, err)
	for range 2 {
		_, err = integrationDB.Exec(string(body))
		require.NoError(t, err)
	}
	var validated bool
	require.NoError(t, integrationDB.QueryRow(`SELECT convalidated FROM pg_constraint WHERE conrelid='domain_events'::regclass AND conname='domain_events_admin_webhook_retry_safe'`).Scan(&validated))
	require.True(t, validated)
	valid, err := service.NewDomainEvent(adminRetryEventType, w.ID, 0, admin.ID, "webhook_delivery", formatID(d.ID), service.DomainEventData{"previous_status": "dead", "status": "retrying", "delivery_id": int64(d.ID)})
	require.NoError(t, err)
	require.NoError(t, insertDomainEventTx(ctx, integrationDB, valid, ""))
	before := snapshotAdminOperation(t, w.ID, d.ID)
	for _, tc := range []struct {
		name   string
		mutate func(*service.DomainEvent)
	}{
		{"wrong-subject", func(e *service.DomainEvent) { e.Subject.Type = "workspace" }},
		{"wrong-id", func(e *service.DomainEvent) { e.Subject.ID = "100000" }},
		{"wrong-status", func(e *service.DomainEvent) { e.Data["status"] = "dead" }},
		{"wrong-previous", func(e *service.DomainEvent) { e.Data["previous_status"] = "pending" }},
		{"missing-id", func(e *service.DomainEvent) { delete(e.Data, "delivery_id") }},
		{"string-id", func(e *service.DomainEvent) { e.Data["delivery_id"] = formatID(d.ID) }},
		{"fractional-id", func(e *service.DomainEvent) { e.Data["delivery_id"] = json.Number("1.5"); e.Subject.ID = "1.5" }},
		{"oversize-id", func(e *service.DomainEvent) {
			e.Data["delivery_id"] = json.Number("9223372036854775808")
			e.Subject.ID = "9223372036854775808"
		}},
		{"extra-scalar", func(e *service.DomainEvent) { e.Data["name"] = "not-retry-evidence" }},
		{"missing-workspace", func(e *service.DomainEvent) { e.WorkspaceID = nil }},
		{"missing-actor", func(e *service.DomainEvent) { e.ActorUserID = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := *valid
			e.ID = "evt_" + uuid.NewString()
			e.Data = service.DomainEventData{}
			for k, v := range valid.Data {
				e.Data[k] = v
			}
			tc.mutate(&e)
			payload, err := json.Marshal(&e)
			require.NoError(t, err)
			_, err = integrationDB.Exec(`INSERT INTO domain_events(id,event_type,event_version,created_at,workspace_id,project_id,actor_user_id,subject_type,subject_id,payload) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, e.ID, e.Type, e.Version, e.CreatedAt, e.WorkspaceID, e.ProjectID, e.ActorUserID, e.Subject.Type, e.Subject.ID, payload)
			var pgErr *pq.Error
			require.ErrorAs(t, err, &pgErr)
			require.Equal(t, "23514", string(pgErr.Code))
			require.Equal(t, "domain_events_admin_webhook_retry_safe", pgErr.Constraint)
			require.Equal(t, before, snapshotAdminOperation(t, w.ID, d.ID))
		})
	}
}
