//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestLifecycleRetentionPostgresTenantAuditDefaultIndefinite(t *testing.T) {
	ctx, _, owner, workspace, project := outboxFixture(t)
	event := outboxTestEvent(t, workspace.ID, project, owner.ID)
	event.CreatedAt = time.Now().UTC().AddDate(0, 0, -400)
	insertOutboxTestEvent(t, ctx, event, "")
	_, err := integrationDB.ExecContext(ctx, `UPDATE domain_event_outbox SET delivered_at=now()-interval '400 days' WHERE event_id=$1`, event.ID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO user_notifications(event_id,recipient_user_id,workspace_id,category,title_key,body_key,created_at) VALUES($1,$2,$3,'project','title','body',now()-interval '400 days')`, event.ID, owner.ID, workspace.ID)
	require.NoError(t, err)
	repo := NewDomainEventOutboxRepository(integrationDB).(service.DomainEventRetentionRepository)
	result, err := repo.Cleanup(ctx, 100)
	require.NoError(t, err)
	require.Equal(t, int64(1), result.Notifications, "operational notifications keep their finite policy")
	require.Zero(t, result.Outbox, "tenant audit envelopes default to indefinite retention")
	require.Zero(t, result.Events)
	var retained int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM domain_events e JOIN domain_event_outbox o ON o.event_id=e.id WHERE e.id=$1`, event.ID).Scan(&retained))
	require.Equal(t, 1, retained)
}

func TestLifecycleRetentionPostgresConcurrentChildCommitKeepsEnvelope(t *testing.T) {
	ctx, _, owner, workspace, project := outboxFixture(t)
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_, err := integrationDB.ExecContext(ctx, `UPDATE platform_retention_policies SET minimum_days=180,default_days=180 WHERE category='audit'`)
	require.NoError(t, err)
	event := outboxTestEvent(t, workspace.ID, project, owner.ID)
	event.CreatedAt = time.Now().UTC().AddDate(0, 0, -400)
	insertOutboxTestEvent(t, ctx, event, "")
	_, err = integrationDB.ExecContext(ctx, `UPDATE domain_event_outbox SET delivered_at=now()-interval '400 days' WHERE event_id=$1`, event.ID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `DELETE FROM domain_event_outbox WHERE event_id=$1`, event.ID)
	require.NoError(t, err)
	// A disposable view gates the candidate statement before its row lock,
	// making a child commit after the statement snapshot deterministic.
	// All original envelope/financial triggers and FK constraints stay enabled.
	_, err = integrationDB.ExecContext(ctx, `CREATE FUNCTION retention_snapshot_gate(at timestamptz) RETURNS timestamptz LANGUAGE plpgsql STABLE AS $$ BEGIN PERFORM pg_advisory_xact_lock(92371522); RETURN at; END $$;
	ALTER TABLE domain_events RENAME TO retained_domain_events;
	CREATE VIEW domain_events AS SELECT id,event_type,event_version,retention_snapshot_gate(created_at) AS created_at,workspace_id,project_id,actor_user_id,subject_type,subject_id,payload,dedupe_key FROM retained_domain_events;`)
	require.NoError(t, err)
	gate, err := integrationDB.Conn(ctx)
	require.NoError(t, err)
	defer func() {
		_, _ = gate.ExecContext(context.Background(), `SELECT pg_advisory_unlock(92371522)`)
		_ = gate.Close()
	}()
	_, err = gate.ExecContext(ctx, `SELECT pg_advisory_lock(92371522)`)
	require.NoError(t, err)
	writer, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = writer.Rollback() }()
	_, err = writer.ExecContext(ctx, `INSERT INTO user_notifications(event_id,recipient_user_id,workspace_id,category,title_key,body_key) VALUES($1,$2,$3,'project','title','body')`, event.ID, owner.ID, workspace.ID)
	require.NoError(t, err)
	type outcome struct {
		counts service.DomainEventRetentionResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		counts, err := NewDomainEventOutboxRepository(integrationDB).(service.DomainEventRetentionRepository).Cleanup(ctx, 100)
		done <- outcome{counts, err}
	}()
	// Wait for the actual statement's advisory gate, then commit the child
	// before allowing cleanup to check and lock its envelope.
	require.Eventually(t, func() bool {
		var blocked int
		err := integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND wait_event='advisory' AND query LIKE '%SELECT e.id,e.created_at%'`).Scan(&blocked)
		return err == nil && blocked > 0
	}, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, writer.Commit())
	_, err = gate.ExecContext(ctx, `SELECT pg_advisory_unlock(92371522)`)
	require.NoError(t, err)
	result := <-done
	require.NoError(t, result.err)
	require.Zero(t, result.counts.Events, "a newly committed child requires its immutable envelope")
	var retained int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM user_notifications n JOIN domain_events e ON e.id=n.event_id WHERE e.id=$1`, event.ID).Scan(&retained))
	require.Equal(t, 1, retained, "retention cannot cascade-delete a new notification")
}

func TestLifecycleRetentionPostgresActiveHoldPreservesEligibleDeliveries(t *testing.T) {
	ctx, _, owner, workspace, project := outboxFixture(t)
	_, err := integrationDB.ExecContext(ctx, `UPDATE platform_retention_policies SET minimum_days=180,default_days=180 WHERE category='audit'`)
	require.NoError(t, err)
	event := outboxTestEvent(t, workspace.ID, project, owner.ID)
	event.CreatedAt = time.Now().UTC().AddDate(0, 0, -400)
	insertOutboxTestEvent(t, ctx, event, "")
	_, err = integrationDB.ExecContext(ctx, `UPDATE domain_event_outbox SET delivered_at=now()-interval '400 days' WHERE event_id=$1`, event.ID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO user_notifications(event_id,recipient_user_id,workspace_id,category,title_key,body_key,created_at) VALUES($1,$2,$3,'project','title','body',now()-interval '400 days')`, event.ID, owner.ID, workspace.ID)
	require.NoError(t, err)
	var webhook int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO workspace_webhooks(workspace_id,name,url,secret_current_encrypted,created_by_user_id) VALUES($1,'Hold','https://example.com/hook','test',$2) RETURNING id`, workspace.ID, owner.ID).Scan(&webhook))
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO workspace_webhook_deliveries(workspace_id,webhook_id,event_id,event_type,payload,status,created_at,finished_at) SELECT workspace_id,$2,id,event_type,payload,'succeeded',created_at,created_at FROM domain_events WHERE id=$1`, event.ID, webhook)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO workspace_lifecycle_holds(workspace_id,code,reason) VALUES($1,'LEGAL_HOLD','Test hold')`, workspace.ID)
	require.NoError(t, err)
	repo := NewDomainEventOutboxRepository(integrationDB).(service.DomainEventRetentionRepository)
	result, err := repo.Cleanup(ctx, 100)
	require.NoError(t, err)
	require.Equal(t, service.DomainEventRetentionResult{}, result, "active holds protect otherwise eligible tenant records")
}

func TestLifecycleRetentionPostgresCursorAdvancesPastIndefiniteAudit(t *testing.T) {
	ctx, s, owner, retained, project := outboxFixture(t)
	finite, err := s.CreateOrganization(ctx, owner.ID, "Finite", "finite-retention")
	require.NoError(t, err)
	var finiteProject int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT id FROM projects WHERE workspace_id=$1 AND is_default`, finite.ID).Scan(&finiteProject))
	_, err = integrationDB.ExecContext(ctx, `UPDATE platform_retention_policies SET minimum_days=180,default_days=180 WHERE category='audit'`)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO workspace_retention_policies(workspace_id,category,retention_days) VALUES($1,'audit',0),($2,'audit',365)`, retained.ID, finite.ID)
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		event := outboxTestEvent(t, retained.ID, project, owner.ID)
		event.CreatedAt = time.Now().UTC().AddDate(0, 0, -600-i)
		insertOutboxTestEvent(t, ctx, event, "")
		_, err = integrationDB.ExecContext(ctx, `UPDATE domain_event_outbox SET delivered_at=now()-interval '600 days'-$2*interval '1 day' WHERE event_id=$1`, event.ID, i)
		require.NoError(t, err)
	}
	event := outboxTestEvent(t, finite.ID, finiteProject, owner.ID)
	event.CreatedAt = time.Now().UTC().AddDate(0, 0, -400)
	insertOutboxTestEvent(t, ctx, event, "")
	_, err = integrationDB.ExecContext(ctx, `UPDATE domain_event_outbox SET delivered_at=now()-interval '400 days' WHERE event_id=$1`, event.ID)
	require.NoError(t, err)
	repo := NewDomainEventOutboxRepository(integrationDB).(service.DomainEventRetentionRepository)
	first, err := repo.Cleanup(ctx, 1)
	require.NoError(t, err)
	require.Zero(t, first.Outbox, "the first bounded page contains only retained audit")
	require.Zero(t, first.Events)
	second, err := repo.Cleanup(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, int64(1), second.Outbox)
	require.Equal(t, int64(1), second.Events, "a finite Workspace cannot be starved by earlier indefinite data")
	var remaining int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM domain_events WHERE id=$1`, event.ID).Scan(&remaining))
	require.Zero(t, remaining)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM domain_events WHERE workspace_id=$1 AND created_at<now()-interval '500 days'`, retained.ID).Scan(&remaining))
	require.Equal(t, 5, remaining)
}

func TestLifecycleRetentionPostgresLegacyEventsAndInboxScopeFallback(t *testing.T) {
	ctx, _, owner, workspace, project := outboxFixture(t)
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO workspace_retention_policies(workspace_id,category,retention_days) VALUES($1,'operational',0)`, workspace.ID)
	require.NoError(t, err)
	tenant := outboxTestEvent(t, workspace.ID, project, owner.ID)
	tenant.CreatedAt = time.Now().UTC().AddDate(0, 0, -400)
	legacy, err := service.NewDomainEvent(service.EventProjectUpdated, 0, 0, owner.ID, "project", fmt.Sprint(project), service.DomainEventData{"status": "active"})
	require.NoError(t, err)
	legacy.CreatedAt = tenant.CreatedAt
	for _, event := range []*service.DomainEvent{tenant, legacy} {
		insertOutboxTestEvent(t, ctx, event, "")
		_, err = integrationDB.ExecContext(ctx, `UPDATE domain_event_outbox SET delivered_at=now()-interval '400 days' WHERE event_id=$1`, event.ID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `INSERT INTO user_notifications(event_id,recipient_user_id,category,title_key,body_key,created_at) VALUES($1,$2,'system','title','body',now()-interval '400 days')`, event.ID, owner.ID)
		require.NoError(t, err)
	}
	result, err := NewDomainEventOutboxRepository(integrationDB).(service.DomainEventRetentionRepository).Cleanup(ctx, 100)
	require.NoError(t, err)
	require.Equal(t, int64(1), result.Notifications)
	require.Equal(t, int64(1), result.Outbox)
	require.Equal(t, int64(1), result.Events)
	var remaining int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM user_notifications WHERE event_id=$1`, tenant.ID).Scan(&remaining))
	require.Equal(t, 1, remaining, "a missing inbox scope must still honor the event's tenant policy")
}
