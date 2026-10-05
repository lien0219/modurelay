//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func outboxFixture(t *testing.T) (context.Context, *service.WorkspaceService, *service.User, *service.Workspace, int64) {
	t.Helper()
	isolateWorkspaceTestFixtures(t)
	ctx, workspaceService, owner, workspace := workspaceFixture(t)
	var projectID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT id FROM projects WHERE workspace_id=$1 AND is_default`, workspace.ID).Scan(&projectID))
	// Claim reads the shared queue across all workspaces. Archive existing
	// pending events, including those emitted by earlier committed user fixtures.
	_, err := integrationDB.ExecContext(ctx, `UPDATE domain_event_outbox SET delivered_at=now() WHERE delivered_at IS NULL`)
	require.NoError(t, err)
	return ctx, workspaceService, owner, workspace, projectID
}

func outboxTestEvent(t *testing.T, workspaceID, projectID, ownerID int64) *service.DomainEvent {
	t.Helper()
	event, err := service.NewDomainEvent(service.EventProjectUpdated, workspaceID, projectID, ownerID, "project", fmt.Sprint(projectID), service.DomainEventData{"status": "active", "project_id": projectID})
	require.NoError(t, err)
	return event
}

func insertOutboxTestEvent(t *testing.T, ctx context.Context, event *service.DomainEvent, dedupe string) {
	t.Helper()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()
	require.NoError(t, insertDomainEventTx(ctx, tx, event, dedupe))
	require.NoError(t, tx.Commit())
}

func TestDomainEventOutboxCommitAndRollbackAreAtomic(t *testing.T) {
	ctx, _, owner, workspace, projectID := outboxFixture(t)
	for _, commit := range []bool{true, false} {
		t.Run(fmt.Sprint(commit), func(t *testing.T) {
			event := outboxTestEvent(t, workspace.ID, projectID, owner.ID)
			tx, err := integrationDB.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer tx.Rollback()
			_, err = tx.ExecContext(ctx, `UPDATE workspaces SET name=$2 WHERE id=$1`, workspace.ID, event.ID)
			require.NoError(t, err)
			require.NoError(t, insertDomainEventTx(ctx, tx, event, ""))
			var invisible int
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM domain_events WHERE id=$1`, event.ID).Scan(&invisible))
			require.Zero(t, invisible, "uncommitted events are invisible outside the business transaction")
			if commit {
				require.NoError(t, tx.Commit())
			} else {
				require.NoError(t, tx.Rollback())
			}
			var name string
			var events, outbox int
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT name FROM workspaces WHERE id=$1`, workspace.ID).Scan(&name))
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM domain_events WHERE id=$1`, event.ID).Scan(&events))
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM domain_event_outbox WHERE event_id=$1`, event.ID).Scan(&outbox))
			if commit {
				require.Equal(t, event.ID, name)
				require.Equal(t, 1, events)
				require.Equal(t, 1, outbox)
			} else {
				require.NotEqual(t, event.ID, name)
				require.Zero(t, events)
				require.Zero(t, outbox)
			}
		})
	}
}

func TestDomainEventDatabaseEnvelopeIsImmutableAndV1(t *testing.T) {
	ctx, _, owner, workspace, projectID := outboxFixture(t)
	event := outboxTestEvent(t, workspace.ID, projectID, owner.ID)
	insertOutboxTestEvent(t, ctx, event, "")
	for _, query := range []string{
		`UPDATE domain_events SET payload=jsonb_set(payload,'{data,status}','"changed"') WHERE id=$1`,
		`UPDATE domain_events SET event_type='project.archived' WHERE id=$1`,
		`UPDATE domain_events SET workspace_id=NULL,project_id=NULL WHERE id=$1`,
		`UPDATE domain_events SET created_at=now()-interval '1 day' WHERE id=$1`,
	} {
		_, err := integrationDB.ExecContext(ctx, query, event.ID)
		require.Error(t, err, "durable event envelope must reject updates")
	}
	var payload []byte
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT payload FROM domain_events WHERE id=$1`, event.ID).Scan(&payload))
	var stored service.DomainEvent
	require.NoError(t, json.Unmarshal(payload, &stored))
	require.Equal(t, 1, stored.Version)
	require.Equal(t, "active", stored.Data["status"])
	for _, data := range []string{`{"credential":"sk-private-provider-secret"}`, `{"status":{"credential":"sk-private-provider-secret"}}`} {
		_, err := integrationDB.ExecContext(ctx, `INSERT INTO domain_events(id,event_type,event_version,created_at,workspace_id,project_id,actor_user_id,subject_type,subject_id,payload) SELECT $2,event_type,event_version,created_at,workspace_id,project_id,actor_user_id,subject_type,subject_id,jsonb_set(jsonb_set(payload,'{id}',to_jsonb($2::text)),'{data}',$3::jsonb) FROM domain_events WHERE id=$1`, event.ID, "evt_"+uuid.NewString(), data)
		require.Error(t, err, "raw SQL must obey the closed scalar payload schema")
	}
}

func TestDomainEventOutboxDuplicateTransitionKeepsFirstPayload(t *testing.T) {
	ctx, _, owner, workspace, projectID := outboxFixture(t)
	original := outboxTestEvent(t, workspace.ID, projectID, owner.ID)
	dedupe := "outbox-test:" + uuid.NewString()
	insertOutboxTestEvent(t, ctx, original, dedupe)
	repeated := outboxTestEvent(t, workspace.ID, projectID, owner.ID)
	repeated.Data["status"] = "changed"
	insertOutboxTestEvent(t, ctx, repeated, dedupe)
	var id string
	var payload []byte
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT id,payload FROM domain_events WHERE dedupe_key=$1`, dedupe).Scan(&id, &payload))
	require.Equal(t, original.ID, id)
	require.Contains(t, string(payload), `"active"`)
	require.NotContains(t, string(payload), `"changed"`)
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM domain_event_outbox WHERE event_id IN($1,$2)`, original.ID, repeated.ID).Scan(&count))
	require.Equal(t, 1, count)
}

type repeatFanoutRecipients struct{ ids []int64 }

func (r repeatFanoutRecipients) Resolve(context.Context, *service.DomainEvent) ([]int64, error) {
	return r.ids, nil
}

type failAfterDurableWebhookFanout struct {
	service.DomainEventWebhookEnqueuer
	failed bool
}

func (r *failAfterDurableWebhookFanout) EnqueueEventDeliveries(ctx context.Context, event *service.DomainEvent) error {
	if err := r.DomainEventWebhookEnqueuer.EnqueueEventDeliveries(ctx, event); err != nil {
		return err
	}
	if !r.failed {
		r.failed = true
		return errors.New("crash after durable consumer accepted event")
	}
	return nil
}

func TestDomainEventDispatcherRetryProducesOneInboxAndDelivery(t *testing.T) {
	ctx, _, owner, workspace, projectID := outboxFixture(t)
	event := outboxTestEvent(t, workspace.ID, projectID, owner.ID)
	insertOutboxTestEvent(t, ctx, event, "")
	var webhookID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO workspace_webhooks(workspace_id,name,url,secret_current_encrypted,created_by_user_id) VALUES($1,'Test','https://example.com/hook','encrypted-test-value',$2) RETURNING id`, workspace.ID, owner.ID).Scan(&webhookID))
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO workspace_webhook_subscriptions(webhook_id,event_type) VALUES($1,$2)`, webhookID, event.Type)
	require.NoError(t, err)
	repo := NewDomainEventOutboxRepository(integrationDB)
	notifications := NewNotificationRepository(integrationDB)
	webhooks := &failAfterDurableWebhookFanout{DomainEventWebhookEnqueuer: NewWorkspaceWebhookRepository(integrationDB)}
	dispatcher := service.NewDomainEventDispatcher(repo, notifications, repeatFanoutRecipients{ids: []int64{owner.ID, owner.ID}}, webhooks)
	require.Error(t, dispatcher.ProcessBatch(ctx))
	var inboxCount, deliveryCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM user_notifications WHERE event_id=$1`, event.ID).Scan(&inboxCount))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM workspace_webhook_deliveries WHERE event_id=$1`, event.ID).Scan(&deliveryCount))
	require.Equal(t, 1, inboxCount)
	require.Equal(t, 1, deliveryCount)
	_, err = integrationDB.ExecContext(ctx, `UPDATE domain_event_outbox SET available_at=now() WHERE event_id=$1`, event.ID)
	require.NoError(t, err)
	require.NoError(t, dispatcher.ProcessBatch(ctx))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM user_notifications WHERE event_id=$1`, event.ID).Scan(&inboxCount))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM workspace_webhook_deliveries WHERE event_id=$1`, event.ID).Scan(&deliveryCount))
	require.Equal(t, 1, inboxCount)
	require.Equal(t, 1, deliveryCount)
	var published bool
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT delivered_at IS NOT NULL FROM domain_event_outbox WHERE event_id=$1`, event.ID).Scan(&published))
	require.True(t, published)
}

func TestDomainEventOutboxConcurrentClaimsAndExpiredFence(t *testing.T) {
	ctx, _, owner, workspace, projectID := outboxFixture(t)
	for i := 0; i < 2; i++ {
		insertOutboxTestEvent(t, ctx, outboxTestEvent(t, workspace.ID, projectID, owner.ID), "")
	}
	repoA, repoB := NewDomainEventOutboxRepository(integrationDB), NewDomainEventOutboxRepository(integrationDB)
	claims := make(chan []service.DomainEventOutboxRecord, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, repo := range []service.DomainEventOutboxRepository{repoA, repoB} {
		wg.Add(1)
		go func(r service.DomainEventOutboxRepository) {
			defer wg.Done()
			items, err := r.Claim(ctx, 1, time.Minute)
			claims <- items
			errs <- err
		}(repo)
	}
	wg.Wait()
	close(claims)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var all []service.DomainEventOutboxRecord
	for items := range claims {
		require.Len(t, items, 1)
		all = append(all, items...)
	}
	require.NotEqual(t, all[0].EventID, all[1].EventID)
	original := all[0]
	_, err := integrationDB.ExecContext(ctx, `UPDATE domain_event_outbox SET locked_until=now()-interval '1 second' WHERE event_id=$1`, original.EventID)
	require.NoError(t, err)
	require.ErrorIs(t, repoA.Ack(ctx, original.EventID, original.LockToken), service.ErrDomainEventLeaseLost)
	reclaimed, err := repoB.Claim(ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, reclaimed, 1)
	require.Equal(t, original.EventID, reclaimed[0].EventID)
	require.NotEqual(t, original.LockToken, reclaimed[0].LockToken)
	require.Equal(t, 2, reclaimed[0].Attempts)
	require.ErrorIs(t, repoA.Retry(ctx, original.EventID, original.LockToken, 0, errors.New("old owner")), service.ErrDomainEventLeaseLost)
	require.NoError(t, repoB.Ack(ctx, reclaimed[0].EventID, reclaimed[0].LockToken))
	require.NoError(t, repoA.Ack(ctx, all[1].EventID, all[1].LockToken))
}

func TestDomainEventOutboxTenThousandEventsStayInBoundedBatches(t *testing.T) {
	// An earlier committed user fixture also leaves a personal-workspace event
	// in the shared queue. Reproduce that setup even when this test runs alone.
	isolateWorkspaceTestFixtures(t)
	existingOwner := mustCreateUser(t, testEntClient(t), &service.User{})
	var existingPending int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_event_outbox o JOIN domain_events e ON e.id=o.event_id WHERE o.delivered_at IS NULL AND e.actor_user_id=$1`, existingOwner.ID).Scan(&existingPending))
	require.Equal(t, 1, existingPending)
	ctx, _, owner, workspace, projectID := outboxFixture(t)
	prefix := "evt_" + uuid.NewString() + "_"
	_, err := integrationDB.ExecContext(ctx, `WITH scope AS (SELECT $1::text AS prefix,$2::bigint AS workspace_id,$3::bigint AS project_id,$4::bigint AS actor_id) INSERT INTO domain_events(id,event_type,event_version,created_at,workspace_id,project_id,actor_user_id,subject_type,subject_id,payload) SELECT prefix||n,'project.updated',1,now(),workspace_id,project_id,actor_id,'project',project_id::text,jsonb_build_object('id',prefix||n,'type','project.updated','version',1,'created_at',now(),'workspace_id',workspace_id,'project_id',project_id,'actor_user_id',actor_id,'subject',jsonb_build_object('type','project','id',project_id::text),'data',jsonb_build_object('status','active')) FROM scope CROSS JOIN generate_series(1,10000)n`, prefix, workspace.ID, projectID, owner.ID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO domain_event_outbox(event_id) SELECT id FROM domain_events WHERE id LIKE $1`, prefix+"%")
	require.NoError(t, err)
	// This is the same partial-index shape as Claim; it must remain available
	// even on installations with a long archive of already-published rows.
	var indexDefinition string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT indexdef FROM pg_indexes WHERE tablename='domain_event_outbox' AND indexname='domain_event_outbox_due'`).Scan(&indexDefinition))
	require.Contains(t, indexDefinition, "available_at")
	require.Contains(t, indexDefinition, "delivered_at IS NULL")
	_, err = integrationDB.ExecContext(ctx, `ANALYZE domain_event_outbox`)
	require.NoError(t, err)
	var queryPlan []byte
	require.NoError(t, integrationDB.QueryRowContext(ctx, `EXPLAIN (FORMAT JSON) SELECT event_id FROM domain_event_outbox WHERE delivered_at IS NULL AND available_at<=now() AND (locked_until IS NULL OR locked_until<now()) ORDER BY available_at,created_at,event_id LIMIT 100 FOR UPDATE SKIP LOCKED`).Scan(&queryPlan))
	require.Contains(t, string(queryPlan), "domain_event_outbox_due", "actual claim plan must use the partial due index")
	repo := NewDomainEventOutboxRepository(integrationDB)
	total, batches := 0, 0
	for {
		items, err := repo.Claim(ctx, 100, time.Minute)
		require.NoError(t, err)
		if len(items) == 0 {
			break
		}
		require.LessOrEqual(t, len(items), 100)
		for _, item := range items {
			require.NoError(t, repo.Ack(ctx, item.EventID, item.LockToken))
		}
		total += len(items)
		batches++
	}
	require.Equal(t, 10000, total)
	require.Equal(t, 100, batches)
}
