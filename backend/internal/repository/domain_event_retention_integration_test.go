//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestDomainEventRetentionKeepsPendingRetryAndActiveLeases(t *testing.T) {
	ctx, _, owner, workspace, projectID := outboxFixture(t)
	// This disposable database opts into finite audit and 90-day operational
	// floors to exercise the existing terminal-state and lease boundaries.
	_, err := integrationDB.ExecContext(ctx, `UPDATE platform_retention_policies SET minimum_days=90,default_days=90 WHERE category IN ('operational','audit')`)
	require.NoError(t, err)
	repo, ok := NewDomainEventOutboxRepository(integrationDB).(service.DomainEventRetentionRepository)
	require.True(t, ok, "SQL outbox must provide retention cleanup")
	var webhookID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO workspace_webhooks(workspace_id,name,url,secret_current_encrypted,created_by_user_id) VALUES($1,'Retention','https://example.com/hook','encrypted-test-value',$2) RETURNING id`, workspace.ID, owner.ID).Scan(&webhookID))
	types := []struct {
		status string
		days   int
		active bool
		want   bool
	}{
		{"succeeded", 91, false, false}, {"succeeded", 89, false, true}, {"dead", 181, false, false}, {"dead", 179, false, true},
		{"pending", 200, false, true}, {"retrying", 200, false, true}, {"delivering", 200, true, true}, {"succeeded", 200, true, true},
	}
	var keepEvents, removeEvents []string
	for _, tc := range types {
		event := outboxTestEvent(t, workspace.ID, projectID, owner.ID)
		event.CreatedAt = time.Now().UTC().AddDate(0, 0, -tc.days)
		insertOutboxTestEvent(t, ctx, event, "")
		_, err := integrationDB.ExecContext(ctx, `INSERT INTO workspace_webhook_deliveries(workspace_id,webhook_id,event_id,event_type,payload,status,created_at,finished_at,delivered_at,lock_owner,locked_at) SELECT workspace_id,$2,id,event_type,payload,$3,created_at,CASE WHEN $3 IN('succeeded','dead') THEN created_at ELSE NULL END,CASE WHEN $3='succeeded' THEN created_at ELSE NULL END,CASE WHEN $4 THEN 'active-lease' ELSE NULL END,CASE WHEN $4 THEN now() ELSE NULL END FROM domain_events WHERE id=$1`, event.ID, webhookID, tc.status, tc.active)
		require.NoError(t, err)
		if tc.want {
			keepEvents = append(keepEvents, event.ID)
		} else {
			removeEvents = append(removeEvents, event.ID)
		}
	}
	// Inbox retention and published outbox retention have independent lifetimes.
	oldInbox := outboxTestEvent(t, workspace.ID, projectID, owner.ID)
	oldInbox.CreatedAt = time.Now().UTC().AddDate(0, 0, -181)
	insertOutboxTestEvent(t, ctx, oldInbox, "")
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO user_notifications(event_id,recipient_user_id,workspace_id,project_id,category,title_key,body_key,data,created_at) SELECT id,$2,workspace_id,project_id,'project','title','body','{}',created_at FROM domain_events WHERE id=$1`, oldInbox.ID, owner.ID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE domain_event_outbox SET delivered_at=now()-interval '91 days' WHERE event_id=$1`, oldInbox.ID)
	require.NoError(t, err)
	activeOutbox := outboxTestEvent(t, workspace.ID, projectID, owner.ID)
	activeOutbox.CreatedAt = time.Now().UTC().AddDate(0, 0, -200)
	insertOutboxTestEvent(t, ctx, activeOutbox, "")
	_, err = integrationDB.ExecContext(ctx, `UPDATE domain_event_outbox SET delivered_at=now()-interval '91 days',locked_until=now()+interval '1 hour',lock_token='active' WHERE event_id=$1`, activeOutbox.ID)
	require.NoError(t, err)
	result, err := repo.Cleanup(ctx, 100)
	require.NoError(t, err)
	require.Equal(t, int64(1), result.Notifications)
	require.Equal(t, int64(2), result.WebhookDeliveries)
	require.Equal(t, int64(1), result.Outbox)
	for _, eventID := range keepEvents {
		var count int
		require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM workspace_webhook_deliveries WHERE event_id=$1`, eventID).Scan(&count))
		require.Equal(t, 1, count, eventID)
	}
	for _, eventID := range removeEvents {
		var count int
		require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM workspace_webhook_deliveries WHERE event_id=$1`, eventID).Scan(&count))
		require.Zero(t, count, eventID)
	}
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM domain_event_outbox WHERE event_id=$1`, activeOutbox.ID).Scan(&count))
	require.Equal(t, 1, count, "active lease must never be collected")
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM domain_events WHERE id=$1`, oldInbox.ID).Scan(&count))
	require.Zero(t, count, "unreferenced old immutable envelopes can be collected")
}

func TestDomainEventRetentionDeletesOnlyOneBoundedBatch(t *testing.T) {
	ctx, _, owner, workspace, projectID := outboxFixture(t)
	repo, ok := NewDomainEventOutboxRepository(integrationDB).(service.DomainEventRetentionRepository)
	require.True(t, ok)
	for i := 0; i < 4; i++ {
		event := outboxTestEvent(t, workspace.ID, projectID, owner.ID)
		insertOutboxTestEvent(t, ctx, event, "")
		_, err := integrationDB.ExecContext(ctx, `INSERT INTO user_notifications(event_id,recipient_user_id,category,title_key,body_key,created_at) VALUES($1,$2,'system','title','body',now()-interval '181 days')`, event.ID, owner.ID)
		require.NoError(t, err)
	}
	result, err := repo.Cleanup(ctx, 2)
	require.NoError(t, err)
	require.Equal(t, int64(2), result.Notifications)
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM user_notifications WHERE recipient_user_id=$1`, owner.ID).Scan(&count))
	require.Equal(t, 2, count, fmt.Sprint(result))
}

func TestDomainEventRetentionHonorsContextCancellation(t *testing.T) {
	repo, ok := NewDomainEventOutboxRepository(integrationDB).(service.DomainEventRetentionRepository)
	require.True(t, ok)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := repo.Cleanup(ctx, 100)
	require.ErrorIs(t, err, context.Canceled)
}

func TestDomainEventMigrationSeedsHistoryWithoutNotificationStorm(t *testing.T) {
	ctx, _, _, workspace, projectID := outboxFixture(t)
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO workspace_budget_policies(workspace_id,amount) VALUES($1,100)`, workspace.ID)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO project_budget_policies(project_id,amount) VALUES($1,100)`, projectID)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO budget_counters(scope_type,scope_id,period_start,spent) VALUES('workspace',$1,'2026-10-01',95),('project',$2,'2026-10-01',85)`, workspace.ID, projectID)
	require.NoError(t, err)
	var before, after int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT count(*) FROM domain_events`).Scan(&before))
	migration, err := migrations.FS.ReadFile("280_domain_events_notifications.sql")
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		_, err = tx.ExecContext(ctx, string(migration))
		require.NoError(t, err)
	}
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT count(*) FROM domain_events`).Scan(&after))
	require.Equal(t, before, after, "historical crossed-state seed must not create events")
	var thresholds, full int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT count(*) FROM budget_alert_transitions WHERE period_start='2026-10-01' AND ((scope_type='workspace' AND scope_id=$1) OR (scope_type='project' AND scope_id=$2)) AND policy_revision=1`, workspace.ID, projectID).Scan(&thresholds))
	require.Equal(t, 4, thresholds)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT count(*) FROM budget_alert_transitions WHERE threshold=100 AND ((scope_type='workspace' AND scope_id=$1) OR (scope_type='project' AND scope_id=$2))`, workspace.ID, projectID).Scan(&full))
	require.Zero(t, full)
}
