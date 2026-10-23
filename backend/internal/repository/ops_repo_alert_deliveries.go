package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

func enqueueAlertDeliveries(ctx context.Context, tx *sql.Tx, eventID int64, deliveries []*service.OpsAlertDelivery) error {
	for _, d := range deliveries {
		if d == nil {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO ops_alert_deliveries(event_id,channel,transition,target,payload)
   VALUES($1,$2,$3,$4,$5) ON CONFLICT(event_id,channel,transition,target) DO NOTHING`,
			eventID, d.Channel, d.Transition, d.Target, string(d.Payload)); err != nil {
			return err
		}
	}
	return nil
}

func (r *opsRepository) EnsureFiringAlertDeliveries(ctx context.Context, eventID int64, deliveries []*service.OpsAlertDelivery) error {
	if len(deliveries) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM ops_alert_events WHERE id=$1 FOR UPDATE`, eventID).Scan(&status); err != nil {
		return err
	}
	if status != service.OpsAlertStatusFiring {
		return nil
	}
	for _, d := range deliveries {
		if d == nil || d.Transition != "firing" {
			return fmt.Errorf("only firing notification intents can be supplemented")
		}
	}
	if err := enqueueAlertDeliveries(ctx, tx, eventID, deliveries); err != nil {
		return err
	}
	return tx.Commit()
}

const alertDeliveryColumns = `id,event_id,channel,transition,target,status,attempts,last_error,next_attempt_at,created_at,sent_at,payload,lease_token`

func scanAlertDelivery(row opsAlertEventRow) (*service.OpsAlertDelivery, error) {
	var d service.OpsAlertDelivery
	var sent sql.NullTime
	err := row.Scan(&d.ID, &d.EventID, &d.Channel, &d.Transition, &d.Target, &d.Status, &d.Attempts, &d.LastError, &d.NextAttemptAt, &d.CreatedAt, &sent, &d.Payload, &d.LeaseToken)
	if err != nil {
		return nil, err
	}
	if sent.Valid {
		d.SentAt = &sent.Time
	}
	return &d, nil
}

func (r *opsRepository) ClaimAlertDeliveries(ctx context.Context, now time.Time, limit int) ([]*service.OpsAlertDelivery, error) {
	if limit < 1 || limit > 50 {
		limit = 20
	}
	token := uuid.NewString()
	rows, err := r.db.QueryContext(ctx, `UPDATE ops_alert_deliveries SET status='sending',attempts=attempts+1,lease_token=$3 || ':' || id::text,next_attempt_at=$1+INTERVAL '2 minutes'
 WHERE id IN (SELECT d.id FROM ops_alert_deliveries d WHERE d.status IN ('pending','sending') AND d.next_attempt_at <= $1
 AND NOT EXISTS (SELECT 1 FROM ops_alert_deliveries earlier WHERE earlier.event_id=d.event_id AND earlier.channel=d.channel AND earlier.target=d.target AND earlier.id<d.id AND earlier.status IN ('pending','sending'))
 ORDER BY d.next_attempt_at,d.id LIMIT $2 FOR UPDATE OF d SKIP LOCKED)
 RETURNING `+alertDeliveryColumns, now, limit, token)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []*service.OpsAlertDelivery{}
	for rows.Next() {
		d, err := scanAlertDelivery(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, errors.Join(rows.Err(), rows.Close())
}

func (r *opsRepository) CompleteAlertDelivery(ctx context.Context, d *service.OpsAlertDelivery, sendErr error, next time.Time) error {
	status := "sent"
	message := ""
	var sent any = time.Now().UTC()
	if sendErr != nil {
		status = "pending"
		sent = nil
		message = sendErr.Error()
		if len(message) > 1024 {
			message = message[:1024]
		}
		if d.Attempts >= 8 {
			status = "failed"
		}
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `UPDATE ops_alert_deliveries SET status=$3,last_error=$4,next_attempt_at=$5,sent_at=$6,lease_token=''
 WHERE id=$1 AND lease_token=$2 AND status='sending'`, d.ID, d.LeaseToken, status, message, next, sent)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n > 0 {
		_, err = tx.ExecContext(ctx, `INSERT INTO ops_alert_delivery_attempts(delivery_id,lease_token,attempt,status,error) VALUES($1,$2,$3,$4,$5) ON CONFLICT(lease_token) DO NOTHING`, d.ID, d.LeaseToken, d.Attempts, status, message)
		if err != nil {
			return err
		}
	}
	if n > 0 && status == "sent" && d.Channel == "email" {
		_, err = tx.ExecContext(ctx, `UPDATE ops_alert_events SET email_sent=true WHERE id=$1`, d.EventID)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *opsRepository) ListAlertDeliveries(ctx context.Context, eventID int64) ([]*service.OpsAlertDelivery, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+alertDeliveryColumns+` FROM ops_alert_deliveries WHERE ($1::bigint=0 OR event_id=$1) ORDER BY id DESC LIMIT 100`, eventID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []*service.OpsAlertDelivery{}
	for rows.Next() {
		d, err := scanAlertDelivery(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(out))
	byID := map[int64]*service.OpsAlertDelivery{}
	for _, d := range out {
		ids = append(ids, d.ID)
		byID[d.ID] = d
	}
	if len(ids) == 0 {
		return out, nil
	}
	history, err := r.db.QueryContext(ctx, `SELECT a.delivery_id,a.attempt,a.status,a.error,a.completed_at FROM unnest($1::bigint[]) AS d(id)
 CROSS JOIN LATERAL(SELECT delivery_id,attempt,status,error,completed_at FROM ops_alert_delivery_attempts WHERE delivery_id=d.id ORDER BY id DESC LIMIT 20) a`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer func() { _ = history.Close() }()
	for history.Next() {
		var id int64
		var attempt service.OpsAlertDeliveryAttempt
		if err := history.Scan(&id, &attempt.Attempt, &attempt.Status, &attempt.Error, &attempt.CompletedAt); err != nil {
			return nil, err
		}
		if d := byID[id]; d != nil {
			d.AttemptHistory = append(d.AttemptHistory, attempt)
		}
	}
	return out, errors.Join(history.Err(), history.Close())
}
func (r *opsRepository) RetryAlertDelivery(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `UPDATE ops_alert_deliveries SET status='pending',attempts=0,last_error='',next_attempt_at=NOW(),lease_token='' WHERE id=$1 AND status='failed'`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("delivery does not exist or is not failed")
	}
	return nil
}

func (r *opsRepository) TransitionAlertEvent(ctx context.Context, event *service.OpsAlertEvent, deliveries []*service.OpsAlertDelivery) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	// Match creation's lock order (incident before event) so concurrent firing
	// and recovery on the same resource cannot deadlock each other.
	var lockedIncident int64
	err = tx.QueryRowContext(ctx, `SELECT i.id FROM ops_alert_incidents i JOIN ops_alert_events e ON e.incident_id=i.id WHERE e.id=$1 FOR UPDATE OF i`, event.ID).Scan(&lockedIncident)
	if err != nil && err != sql.ErrNoRows {
		return false, err
	}
	var query string
	if event.Status == service.OpsAlertStatusFiring {
		query = `UPDATE ops_alert_events SET severity=$2,metric_value=$3,title=$4 WHERE id=$1 AND status='firing' AND severity<>$2`
		res, e := tx.ExecContext(ctx, query, event.ID, event.Severity, opsNullFloat64(event.MetricValue), event.Title)
		if e != nil {
			return false, e
		}
		n, e := res.RowsAffected()
		if e != nil {
			return false, e
		}
		if n == 0 {
			return false, nil
		}
	} else {
		query = `UPDATE ops_alert_events SET status=$2,resolved_at=$3 WHERE id=$1 AND status='firing'`
		res, e := tx.ExecContext(ctx, query, event.ID, event.Status, event.ResolvedAt)
		if e != nil {
			return false, e
		}
		n, e := res.RowsAffected()
		if e != nil {
			return false, e
		}
		if n == 0 {
			return false, nil
		}
	}
	if event.Status == service.OpsAlertStatusFiring {
		_, err = tx.ExecContext(ctx, `UPDATE ops_alert_incidents SET severity=LEAST(severity,$2) WHERE id=(SELECT incident_id FROM ops_alert_events WHERE id=$1)`, event.ID, event.Severity)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE ops_alert_incidents i SET status='resolved',resolved_at=$2 WHERE i.id=(SELECT incident_id FROM ops_alert_events WHERE id=$1)
 AND NOT EXISTS(SELECT 1 FROM ops_alert_events e WHERE e.incident_id=i.id AND e.status='firing')`, event.ID, event.ResolvedAt)
	}
	if err != nil {
		return false, err
	}
	if err = enqueueAlertDeliveries(ctx, tx, event.ID, deliveries); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func (r *opsRepository) GetAlertResourceState(ctx context.Context, ruleID int64, key string) (*service.OpsAlertRuleState, error) {
	var s service.OpsAlertRuleState
	var last sql.NullTime
	err := r.db.QueryRowContext(ctx, `SELECT rule_id,consecutive_breaches,last_evaluated_at,data_status,rule_revision FROM ops_alert_resource_states WHERE rule_id=$1 AND resource_key=$2`, ruleID, key).Scan(&s.RuleID, &s.ConsecutiveBreaches, &last, &s.DataStatus, &s.RuleRevision)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if last.Valid {
		s.LastEvaluatedAt = last.Time
	}
	return &s, nil
}
func (r *opsRepository) UpsertAlertResourceState(ctx context.Context, key string, s *service.OpsAlertRuleState) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO ops_alert_resource_states(rule_id,resource_key,consecutive_breaches,last_evaluated_at,data_status,rule_revision)
 SELECT $1,$2,$3,$4,$5,$6 FROM ops_alert_rules WHERE id=$1 AND evaluation_revision=$6 FOR SHARE
 ON CONFLICT(rule_id,resource_key) DO UPDATE SET consecutive_breaches=EXCLUDED.consecutive_breaches,last_evaluated_at=EXCLUDED.last_evaluated_at,data_status=EXCLUDED.data_status,rule_revision=EXCLUDED.rule_revision,updated_at=NOW()
 WHERE ops_alert_resource_states.rule_revision<>EXCLUDED.rule_revision OR ops_alert_resource_states.last_evaluated_at IS NULL OR ops_alert_resource_states.last_evaluated_at < EXCLUDED.last_evaluated_at`, s.RuleID, key, s.ConsecutiveBreaches, s.LastEvaluatedAt, s.DataStatus, opsAlertRevision(s.RuleRevision))
	return err
}
func (r *opsRepository) GetResourceAlertEvent(ctx context.Context, ruleID int64, key string, active bool) (*service.OpsAlertEvent, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id,COALESCE(rule_id,0),COALESCE(severity,''),COALESCE(status,''),COALESCE(title,''),COALESCE(description,''),metric_value,threshold_value,dimensions,fired_at,resolved_at,email_sent,created_at
 FROM ops_alert_events WHERE rule_id=$1 AND resource_key=$2 AND (NOT $3::bool OR status='firing') ORDER BY fired_at DESC,id DESC LIMIT 1`, ruleID, key, active)
	event, err := scanOpsAlertEvent(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return event, err
}

func (r *opsRepository) IsAlertResourceSilenced(ctx context.Context, ruleID int64, key string, now time.Time) (bool, error) {
	var yes bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ops_alert_silences WHERE rule_id=$1 AND resource_key=$2 AND until>$3)`, ruleID, key, now).Scan(&yes)
	return yes, err
}

func (r *opsRepository) MarkMissingAlertResources(ctx context.Context, ruleID, ruleRevision int64, seen []string, now time.Time) (bool, error) {
	if seen == nil {
		seen = []string{}
	}
	res, err := r.db.ExecContext(ctx, `UPDATE ops_alert_resource_states SET consecutive_breaches=0,last_evaluated_at=$3,data_status='unavailable',updated_at=NOW()
 WHERE rule_id=$1 AND rule_revision=$4 AND NOT(resource_key=ANY($2::text[])) AND (last_evaluated_at IS NULL OR last_evaluated_at<$3)`, ruleID, pq.Array(seen), now, opsAlertRevision(ruleRevision))
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
