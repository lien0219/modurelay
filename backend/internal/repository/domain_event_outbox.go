package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

type domainEventSQL interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

// insertDomainEventTx persists a domain event and its dispatch row on the
// caller's transaction. dedupeKey is required for retriable transitions; an
// empty key means the event ID itself is the idempotency identity.
func insertDomainEventTx(ctx context.Context, q domainEventSQL, event *service.DomainEvent, dedupeKey string) error {
	if event == nil {
		return errors.New("domain event is nil")
	}
	payload, err := event.MarshalPayload()
	if err != nil {
		return err
	}
	dedupeKey = strings.TrimSpace(dedupeKey)
	if dedupeKey == "" {
		_, err = q.ExecContext(ctx, `INSERT INTO domain_events(id,event_type,event_version,created_at,workspace_id,project_id,actor_user_id,subject_type,subject_id,payload) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, event.ID, event.Type, event.Version, event.CreatedAt, event.WorkspaceID, event.ProjectID, event.ActorUserID, event.Subject.Type, event.Subject.ID, payload)
		if err != nil {
			return err
		}
		_, err = q.ExecContext(ctx, `INSERT INTO domain_event_outbox(event_id) SELECT id FROM domain_events WHERE id=$1 ON CONFLICT(event_id) DO NOTHING`, event.ID)
		return err
	}
	_, err = q.ExecContext(ctx, `INSERT INTO domain_events(id,event_type,event_version,created_at,workspace_id,project_id,actor_user_id,subject_type,subject_id,payload,dedupe_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT(dedupe_key) DO NOTHING`, event.ID, event.Type, event.Version, event.CreatedAt, event.WorkspaceID, event.ProjectID, event.ActorUserID, event.Subject.Type, event.Subject.ID, payload, dedupeKey)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx, `INSERT INTO domain_event_outbox(event_id) SELECT id FROM domain_events WHERE dedupe_key=$1 ON CONFLICT(event_id) DO NOTHING`, dedupeKey)
	return err
}

// InsertDomainEventTx is exported for repositories in other packages while
// retaining one implementation of the SQL contract.
func InsertDomainEventTx(ctx context.Context, q domainEventSQL, event *service.DomainEvent, dedupeKey string) error {
	return insertDomainEventTx(ctx, q, event, dedupeKey)
}

func (r *domainEventOutboxRepository) Claim(ctx context.Context, limit int, lease time.Duration) ([]service.DomainEventOutboxRecord, error) {
	if limit <= 0 {
		limit = 100
	} else if limit > 1000 {
		limit = 1000
	}
	if lease <= 0 {
		lease = time.Minute
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	token := uuid.NewString()
	rows, err := tx.QueryContext(ctx, `
		WITH due AS (
			SELECT o.event_id FROM domain_event_outbox o
			WHERE o.delivered_at IS NULL AND o.available_at<=now()
			  AND (o.locked_until IS NULL OR o.locked_until<now())
			ORDER BY o.available_at,o.created_at,o.event_id LIMIT $1 FOR UPDATE SKIP LOCKED
		), claimed AS (
			UPDATE domain_event_outbox o SET attempts=o.attempts+1,locked_until=now()+$2 * interval '1 second',lock_token=$3
			FROM due WHERE o.event_id=due.event_id RETURNING o.event_id,o.attempts,o.locked_until,o.lock_token
		)
		SELECT c.event_id,e.event_type,e.payload,c.attempts,c.lock_token,c.locked_until
		FROM claimed c JOIN domain_events e ON e.id=c.event_id ORDER BY c.event_id`, limit, lease.Seconds(), token)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.DomainEventOutboxRecord, 0, limit)
	for rows.Next() {
		var item service.DomainEventOutboxRecord
		if err = rows.Scan(&item.EventID, &item.EventType, &item.Payload, &item.Attempts, &item.LockToken, &item.LockedUntil); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	return out, tx.Commit()
}

func (r *domainEventOutboxRepository) Ack(ctx context.Context, eventID, lockToken string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE domain_event_outbox SET delivered_at=now(),locked_until=NULL,lock_token=NULL,last_error=NULL WHERE event_id=$1 AND lock_token=$2 AND delivered_at IS NULL AND locked_until>now()`, strings.TrimSpace(eventID), strings.TrimSpace(lockToken))
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return service.ErrDomainEventLeaseLost
	}
	return err
}

func (r *domainEventOutboxRepository) Retry(ctx context.Context, eventID, lockToken string, delay time.Duration, reason error) error {
	if delay < 0 {
		delay = 0
	}
	message := ""
	if reason != nil {
		message = service.DomainEventFailureCode(reason)
	}
	result, err := r.db.ExecContext(ctx, `UPDATE domain_event_outbox SET available_at=now()+$3 * interval '1 second',locked_until=NULL,lock_token=NULL,last_error=$4 WHERE event_id=$1 AND lock_token=$2 AND delivered_at IS NULL AND locked_until>now()`, strings.TrimSpace(eventID), strings.TrimSpace(lockToken), delay.Seconds(), message)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return service.ErrDomainEventLeaseLost
	}
	return err
}

// Cleanup removes one bounded batch per retention class. Pending dispatch and
// delivery rows keep their envelopes, and live leases are never collected.
func (r *domainEventOutboxRepository) Cleanup(ctx context.Context, limit int) (service.DomainEventRetentionResult, error) {
	var counts service.DomainEventRetentionResult
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return counts, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `SET LOCAL statement_timeout='20s'; SET LOCAL lock_timeout='1s'; LOCK TABLE platform_retention_policies,workspace_retention_policies,workspace_lifecycle_holds IN SHARE MODE`); err != nil {
		return counts, err
	}
	for i, step := range domainEventRetentionSteps {
		count, cleanupErr := cleanupDomainEventRetentionClass(ctx, tx, step, limit)
		if cleanupErr != nil {
			return service.DomainEventRetentionResult{}, cleanupErr
		}
		switch i {
		case 0:
			counts.Notifications = count
		case 1:
			counts.WebhookDeliveries = count
		case 2:
			counts.Outbox = count
		case 3:
			counts.Events = count
		}
	}
	if err := tx.Commit(); err != nil {
		return service.DomainEventRetentionResult{}, err
	}
	return counts, nil
}

type domainEventOutboxRepository struct{ db *sql.DB }

func NewDomainEventOutboxRepository(db *sql.DB) service.DomainEventOutboxRepository {
	return &domainEventOutboxRepository{db: db}
}
