//go:build integration

package repository

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseAdminWorkspaceOperationReplayIsAtomic(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, _, _, workspace := workspaceFixture(t)
	admin := enterpriseAdminActor(t)
	now := time.Now().UTC()
	trusted := service.WithSessionAuthentication(ctx, service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: now})
	trusted = service.WithRecentAuthentication(trusted, now)
	svc := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
	key := uuid.NewString()
	in := service.AdminOperationInput{Action: "suspend", Reason: "planned maintenance", Confirmation: "suspend:" + formatID(workspace.ID), IdempotencyKey: key, ExpectedUpdatedAt: workspace.UpdatedAt.Format(time.RFC3339Nano)}
	first, err := svc.AdminOperateWorkspace(trusted, admin.ID, workspace.ID, in)
	require.NoError(t, err)
	require.Equal(t, "suspended", first.ResultStatus)
	replay, err := svc.AdminOperateWorkspace(trusted, admin.ID, workspace.ID, in)
	require.NoError(t, err)
	require.Equal(t, first.ID, replay.ID)
	changed := in
	changed.Reason = "different reason"
	_, err = svc.AdminOperateWorkspace(trusted, admin.ID, workspace.ID, changed)
	require.ErrorIs(t, err, service.ErrAdminOperationIdempotencyConflict)
	var transitions, receipts, audits, events, outbox int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspaces WHERE id=$1 AND status='suspended'`, workspace.ID).Scan(&transitions))
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM enterprise_admin_operations WHERE actor_user_id=$1 AND idempotency_key=$2`, admin.ID, key).Scan(&receipts))
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_audit_logs WHERE workspace_id=$1 AND action='workspace_suspended' AND actor_user_id=$2`, workspace.ID, admin.ID).Scan(&audits))
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_events WHERE workspace_id=$1 AND event_type=$2 AND actor_user_id=$3`, workspace.ID, service.EventWorkspaceSuspended, admin.ID).Scan(&events))
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_event_outbox o JOIN domain_events e ON e.id=o.event_id WHERE e.workspace_id=$1 AND e.event_type=$2 AND e.actor_user_id=$3`, workspace.ID, service.EventWorkspaceSuspended, admin.ID).Scan(&outbox))
	require.Equal(t, 1, transitions)
	require.Equal(t, 1, receipts)
	require.Equal(t, 1, audits)
	require.Equal(t, 1, events)
	require.Equal(t, 1, outbox)

	resume := in
	resume.Action, resume.Status, resume.Confirmation, resume.IdempotencyKey = "resume", "", "resume:"+formatID(workspace.ID), uuid.NewString()
	resume.ExpectedUpdatedAt = first.ResultUpdatedAt.Format(time.RFC3339Nano)
	_, err = svc.AdminOperateWorkspace(trusted, admin.ID, workspace.ID, resume)
	require.NoError(t, err)
}

func TestEnterpriseAdminWorkspaceOperationConcurrentIdenticalKey(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, _, _, workspace := workspaceFixture(t)
	admin := enterpriseAdminActor(t)
	now := time.Now().UTC()
	trusted := service.WithRecentAuthentication(service.WithSessionAuthentication(ctx, service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: now}), now)
	in := service.AdminOperationInput{Action: "suspend", Reason: "concurrent maintenance", Confirmation: "suspend:" + formatID(workspace.ID), IdempotencyKey: uuid.NewString(), ExpectedUpdatedAt: workspace.UpdatedAt.Format(time.RFC3339Nano)}
	before := snapshotAdminOperation(t, workspace.ID, 0)
	start := make(chan struct{})
	type result struct {
		receipt *service.AdminOperationReceipt
		err     error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			receipt, err := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB)).AdminOperateWorkspace(trusted, admin.ID, workspace.ID, in)
			results <- result{receipt: receipt, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	var firstID string
	for got := range results {
		require.NoError(t, got.err)
		require.NotNil(t, got.receipt)
		if firstID == "" {
			firstID = got.receipt.ID
		} else {
			require.Equal(t, firstID, got.receipt.ID)
		}
		require.True(t, got.receipt.ResultUpdatedAt.After(workspace.UpdatedAt))
	}
	requireAdminWriteSet(t, before, snapshotAdminOperation(t, workspace.ID, 0), 1)
	var status string
	var updatedAt time.Time
	require.NoError(t, integrationDB.QueryRow(`SELECT status,updated_at FROM workspaces WHERE id=$1`, workspace.ID).Scan(&status, &updatedAt))
	require.Equal(t, "suspended", status)
	require.True(t, updatedAt.After(workspace.UpdatedAt))
}

func TestEnterpriseAdminRetryWebhookIsDeadOnlyAndCapped(t *testing.T) {
	ctx, _, webhookSvc, webhookRepo, owner, workspace := workspaceWebhookFixture(t)
	admin := enterpriseAdminActor(t)
	now := time.Now().UTC()
	trusted := service.WithRecentAuthentication(service.WithSessionAuthentication(ctx, service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: now}), now)
	endpoint := createWorkspaceWebhookFixture(t, ctx, webhookSvc, owner.ID, workspace.ID)
	event, err := service.NewDomainEvent(service.EventWorkspaceUpdated, workspace.ID, 0, owner.ID, "workspace", formatID(workspace.ID), service.DomainEventData{"status": "updated"})
	require.NoError(t, err)
	delivery, err := webhookRepo.CreateTestDelivery(ctx, owner.ID, workspace.ID, endpoint.ID, event)
	require.NoError(t, err)
	last := now.Add(-time.Minute).Truncate(time.Microsecond)
	_, err = integrationDB.Exec(`UPDATE workspace_webhook_deliveries SET status='dead',attempts=2,last_attempt_at=$2,finished_at=$3,locked_at=NULL,lock_owner=NULL WHERE id=$1`, delivery.ID, last, now)
	require.NoError(t, err)
	r := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
	before := snapshotAdminOperation(t, workspace.ID, delivery.ID)
	envelope := deliveryEnvelope(t, delivery.ID)
	for i := 0; i < 3; i++ {
		in := service.AdminOperationInput{Action: "retry_webhook", Reason: "requeue dead delivery", Confirmation: "retry_webhook:" + formatID(delivery.ID), IdempotencyKey: uuid.NewString(), ExpectedAttempts: 2, ExpectedLastAttemptAt: last.Format(time.RFC3339Nano)}
		receipt, retryErr := r.AdminRetryWebhook(trusted, admin.ID, workspace.ID, endpoint.ID, delivery.ID, in)
		require.NoError(t, retryErr)
		require.Equal(t, "retrying", receipt.ResultStatus)
		var status string
		require.NoError(t, integrationDB.QueryRow(`SELECT status FROM workspace_webhook_deliveries WHERE id=$1`, delivery.ID).Scan(&status))
		require.Equal(t, "retrying", status)
		require.Equal(t, envelope, deliveryEnvelope(t, delivery.ID))
		if i < 2 {
			_, err = integrationDB.Exec(`UPDATE workspace_webhook_deliveries SET status='dead',attempts=2,last_attempt_at=$2,finished_at=$3,locked_at=NULL,lock_owner=NULL WHERE id=$1`, delivery.ID, last, now)
			require.NoError(t, err)
		}
	}
	_, err = integrationDB.Exec(`UPDATE workspace_webhook_deliveries SET status='dead',attempts=2,last_attempt_at=$2,finished_at=$3,locked_at=NULL,lock_owner=NULL WHERE id=$1`, delivery.ID, last, now)
	require.NoError(t, err)
	in := service.AdminOperationInput{Action: "retry_webhook", Reason: "fourth attempt", Confirmation: "retry_webhook:" + formatID(delivery.ID), IdempotencyKey: uuid.NewString(), ExpectedAttempts: 2, ExpectedLastAttemptAt: last.Format(time.RFC3339Nano)}
	denied := snapshotAdminOperation(t, workspace.ID, delivery.ID)
	_, err = r.AdminRetryWebhook(trusted, admin.ID, workspace.ID, endpoint.ID, delivery.ID, in)
	require.ErrorIs(t, err, service.ErrWorkspaceConflict)
	require.Equal(t, denied, snapshotAdminOperation(t, workspace.ID, delivery.ID))
	requireAdminWriteSet(t, before, denied, 3)
}

func TestEnterpriseAdminWorkspaceOperationRollsBackOnAuditFailure(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, _, _, workspace := workspaceFixture(t)
	admin := enterpriseAdminActor(t)
	now := time.Now().UTC()
	trusted := service.WithRecentAuthentication(service.WithSessionAuthentication(ctx, service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: now}), now)
	constraint := "enterprise_test_admin_operation_audit_failure"
	_, err := integrationDB.Exec(`ALTER TABLE workspace_audit_logs ADD CONSTRAINT ` + constraint + ` CHECK (NOT (workspace_id=` + formatID(workspace.ID) + ` AND action='workspace_suspended')) NOT VALID`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.Exec(`ALTER TABLE workspace_audit_logs DROP CONSTRAINT IF EXISTS ` + constraint)
	})
	in := service.AdminOperationInput{Action: "suspend", Reason: "must rollback", Confirmation: "suspend:" + formatID(workspace.ID), IdempotencyKey: uuid.NewString(), ExpectedUpdatedAt: workspace.UpdatedAt.Format(time.RFC3339Nano)}
	_, err = service.NewWorkspaceService(NewWorkspaceRepository(integrationDB)).AdminOperateWorkspace(trusted, admin.ID, workspace.ID, in)
	require.Error(t, err)
	var status string
	require.NoError(t, integrationDB.QueryRow(`SELECT status FROM workspaces WHERE id=$1`, workspace.ID).Scan(&status))
	require.Equal(t, "active", status)
	var receipts int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM enterprise_admin_operations WHERE idempotency_key=$1`, in.IdempotencyKey).Scan(&receipts))
	require.Zero(t, receipts)
}

func formatID(id int64) string {
	return strconv.FormatInt(id, 10)
}
