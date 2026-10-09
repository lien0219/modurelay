package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq"
)

type domainEventRetentionStep struct {
	category, candidates, deleteRows string
}

// The candidate page applies the legacy minimum first, then evaluates the
// tenant policy on at most 1000 rows. Persisted keyset cursors prevent an
// indefinite policy or pending child from starving later eligible envelopes.
// Notification tenancy also follows its immutable event when no scope was
// copied into the inbox row by an older writer.
var domainEventRetentionSteps = []domainEventRetentionStep{
	{"notifications", `SELECT n.id::text,n.created_at,
	 (COALESCE(n.workspace_id,e.workspace_id) IS NULL OR
	  (effective_lifecycle_retention_days(COALESCE(n.workspace_id,e.workspace_id),'operational')>0 AND
	   n.created_at<now()-GREATEST(180,effective_lifecycle_retention_days(COALESCE(n.workspace_id,e.workspace_id),'operational'))*interval '1 day'))
	 AND NOT EXISTS(SELECT 1 FROM workspace_lifecycle_holds h WHERE h.workspace_id=COALESCE(n.workspace_id,e.workspace_id) AND h.active)
	 FROM user_notifications n LEFT JOIN domain_events e ON e.id=n.event_id
	 WHERE n.created_at<now()-interval '180 days'
	 AND ($2::timestamptz IS NULL OR (n.created_at,n.id)>($2,$3::bigint))
	 ORDER BY n.created_at,n.id LIMIT $1 FOR UPDATE OF n SKIP LOCKED`,
		`DELETE FROM user_notifications WHERE id=ANY($1::bigint[])`},
	{"webhooks", `SELECT d.id::text,COALESCE(d.finished_at,d.delivered_at,d.created_at),
	 (effective_lifecycle_retention_days(d.workspace_id,'operational')>0 AND
	  COALESCE(d.finished_at,d.delivered_at,d.created_at)<now()-GREATEST(CASE WHEN d.status='succeeded' THEN 90 ELSE 180 END,effective_lifecycle_retention_days(d.workspace_id,'operational'))*interval '1 day')
	 AND NOT EXISTS(SELECT 1 FROM workspace_lifecycle_holds h WHERE h.workspace_id=d.workspace_id AND h.active)
	 FROM workspace_webhook_deliveries d
	 WHERE d.status IN ('succeeded','dead') AND d.lock_owner IS NULL AND d.locked_at IS NULL
	 AND COALESCE(d.finished_at,d.delivered_at,d.created_at)<now()-CASE WHEN d.status='succeeded' THEN interval '90 days' ELSE interval '180 days' END
	 AND ($2::timestamptz IS NULL OR (COALESCE(d.finished_at,d.delivered_at,d.created_at),d.id)>($2,$3::bigint))
	 ORDER BY COALESCE(d.finished_at,d.delivered_at,d.created_at),d.id LIMIT $1 FOR UPDATE OF d SKIP LOCKED`,
		`DELETE FROM workspace_webhook_deliveries WHERE id=ANY($1::bigint[])`},
	{"outbox", `SELECT o.event_id,o.delivered_at,
	 (e.workspace_id IS NULL OR (effective_lifecycle_retention_days(e.workspace_id,'audit')>0 AND
	  o.delivered_at<now()-GREATEST(90,effective_lifecycle_retention_days(e.workspace_id,'audit'))*interval '1 day'))
	 AND NOT EXISTS(SELECT 1 FROM workspace_lifecycle_holds h WHERE h.workspace_id=e.workspace_id AND h.active)
	 FROM domain_event_outbox o JOIN domain_events e ON e.id=o.event_id
	 WHERE o.delivered_at<now()-interval '90 days' AND (o.locked_until IS NULL OR o.locked_until<=now())
	 AND ($2::timestamptz IS NULL OR (o.delivered_at,o.event_id)>($2,$3::text))
	 ORDER BY o.delivered_at,o.event_id LIMIT $1 FOR UPDATE OF o SKIP LOCKED`,
		`DELETE FROM domain_event_outbox WHERE event_id=ANY($1::text[])`},
	{"events", `SELECT e.id,e.created_at,
	 (e.workspace_id IS NULL OR (effective_lifecycle_retention_days(e.workspace_id,'audit')>0 AND
	  e.created_at<now()-GREATEST(180,effective_lifecycle_retention_days(e.workspace_id,'audit'))*interval '1 day'))
	 AND NOT EXISTS(SELECT 1 FROM workspace_lifecycle_holds h WHERE h.workspace_id=e.workspace_id AND h.active)
	 AND NOT EXISTS(SELECT 1 FROM domain_event_outbox o WHERE o.event_id=e.id)
	 AND NOT EXISTS(SELECT 1 FROM user_notifications n WHERE n.event_id=e.id)
	 AND NOT EXISTS(SELECT 1 FROM workspace_webhook_deliveries d WHERE d.event_id=e.id)
	 FROM domain_events e WHERE e.created_at<now()-interval '180 days'
	 AND ($2::timestamptz IS NULL OR (e.created_at,e.id)>($2,$3::text))
	 ORDER BY e.created_at,e.id LIMIT $1 FOR UPDATE OF e SKIP LOCKED`,
		`DELETE FROM domain_events e WHERE e.id=ANY($1::text[])
	 AND NOT EXISTS(SELECT 1 FROM domain_event_outbox o WHERE o.event_id=e.id)
	 AND NOT EXISTS(SELECT 1 FROM user_notifications n WHERE n.event_id=e.id)
	 AND NOT EXISTS(SELECT 1 FROM workspace_webhook_deliveries d WHERE d.event_id=e.id)`},
}

func cleanupDomainEventRetentionClass(ctx context.Context, tx *sql.Tx, step domainEventRetentionStep, limit int) (int64, error) {
	var afterTime sql.NullTime
	var afterID sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT after_time,after_id FROM domain_event_retention_cursors WHERE category=$1 FOR UPDATE SKIP LOCKED`, step.category).Scan(&afterTime, &afterID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	scanLimit := min(limit*4, 1000)
	rows, err := tx.QueryContext(ctx, step.candidates, scanLimit, afterTime, afterID)
	if err != nil {
		return 0, err
	}
	ids := make([]string, 0, limit)
	var lastTime time.Time
	var lastID string
	scanned := 0
	for rows.Next() {
		var eligible bool
		if err = rows.Scan(&lastID, &lastTime, &eligible); err != nil {
			_ = rows.Close()
			return 0, err
		}
		scanned++
		if eligible && len(ids) < limit {
			ids = append(ids, lastID)
		}
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		return 0, err
	}
	if err = rows.Close(); err != nil {
		return 0, err
	}
	var count int64
	if len(ids) > 0 {
		result, deleteErr := tx.ExecContext(ctx, step.deleteRows, pq.Array(ids))
		if deleteErr != nil {
			return 0, deleteErr
		}
		if count, err = result.RowsAffected(); err != nil {
			return 0, err
		}
	}
	if scanned < scanLimit {
		afterTime, afterID = sql.NullTime{}, sql.NullString{}
	} else {
		afterTime, afterID = sql.NullTime{Time: lastTime, Valid: true}, sql.NullString{String: lastID, Valid: true}
	}
	_, err = tx.ExecContext(ctx, `UPDATE domain_event_retention_cursors SET after_time=$2,after_id=$3 WHERE category=$1`, step.category, afterTime, afterID)
	return count, err
}
