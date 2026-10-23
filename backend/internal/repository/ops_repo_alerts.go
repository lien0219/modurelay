package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *opsRepository) ListAlertRules(ctx context.Context) ([]*service.OpsAlertRule, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}

	q := `
SELECT
  r.id,
  r.name,
  COALESCE(r.description, ''),
  r.enabled,
  COALESCE(r.severity, ''),
  r.metric_type,
  r.operator,
  r.threshold,
  r.window_minutes,
  r.sustained_minutes,
  r.cooldown_minutes,
  COALESCE(r.notify_email, true),
  COALESCE(r.min_requests, 0),
  r.filters,
  r.last_triggered_at,
  r.created_at,
  r.updated_at,
  COALESCE(s.data_status, rs.data_status, ''),
  r.evaluation_revision
FROM ops_alert_rules r
LEFT JOIN ops_alert_rule_states s ON s.rule_id = r.id
LEFT JOIN LATERAL (
 SELECT CASE WHEN bool_or(data_status='unavailable') THEN 'unavailable'
 WHEN bool_or(data_status='insufficient_data') THEN 'insufficient_data' ELSE 'ok' END AS data_status
 FROM ops_alert_resource_states WHERE rule_id=r.id HAVING COUNT(*) > 0
) rs ON true
ORDER BY r.id DESC`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []*service.OpsAlertRule{}
	for rows.Next() {
		var rule service.OpsAlertRule
		var filtersRaw []byte
		var lastTriggeredAt sql.NullTime
		if err := rows.Scan(
			&rule.ID,
			&rule.Name,
			&rule.Description,
			&rule.Enabled,
			&rule.Severity,
			&rule.MetricType,
			&rule.Operator,
			&rule.Threshold,
			&rule.WindowMinutes,
			&rule.SustainedMinutes,
			&rule.CooldownMinutes,
			&rule.NotifyEmail,
			&rule.MinRequests,
			&filtersRaw,
			&lastTriggeredAt,
			&rule.CreatedAt,
			&rule.UpdatedAt,
			&rule.DataStatus,
			&rule.EvaluationRevision,
		); err != nil {
			return nil, err
		}
		if lastTriggeredAt.Valid {
			v := lastTriggeredAt.Time
			rule.LastTriggeredAt = &v
		}
		if len(filtersRaw) > 0 && string(filtersRaw) != "null" {
			var decoded map[string]any
			if err := json.Unmarshal(filtersRaw, &decoded); err == nil {
				rule.Filters = decoded
			}
		}
		out = append(out, &rule)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *opsRepository) CreateAlertRule(ctx context.Context, input *service.OpsAlertRule) (*service.OpsAlertRule, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}
	if input == nil {
		return nil, fmt.Errorf("nil input")
	}

	filtersArg, err := opsNullJSONMap(input.Filters)
	if err != nil {
		return nil, err
	}

	q := `
INSERT INTO ops_alert_rules (
  name,
  description,
  enabled,
  severity,
  metric_type,
  operator,
  threshold,
  window_minutes,
  sustained_minutes,
  cooldown_minutes,
  min_requests,
  notify_email,
  filters,
  created_at,
  updated_at
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,NOW(),NOW()
)
RETURNING
  id,
  name,
  COALESCE(description, ''),
  enabled,
  COALESCE(severity, ''),
  metric_type,
  operator,
  threshold,
  window_minutes,
  sustained_minutes,
  cooldown_minutes,
  COALESCE(min_requests, 0),
  COALESCE(notify_email, true),
  filters,
  last_triggered_at,
  created_at,
  updated_at,
  evaluation_revision`

	var out service.OpsAlertRule
	var filtersRaw []byte
	var lastTriggeredAt sql.NullTime

	if err := r.db.QueryRowContext(
		ctx,
		q,
		strings.TrimSpace(input.Name),
		strings.TrimSpace(input.Description),
		input.Enabled,
		strings.TrimSpace(input.Severity),
		strings.TrimSpace(input.MetricType),
		strings.TrimSpace(input.Operator),
		input.Threshold,
		input.WindowMinutes,
		input.SustainedMinutes,
		input.CooldownMinutes,
		input.MinRequests,
		input.NotifyEmail,
		filtersArg,
	).Scan(
		&out.ID,
		&out.Name,
		&out.Description,
		&out.Enabled,
		&out.Severity,
		&out.MetricType,
		&out.Operator,
		&out.Threshold,
		&out.WindowMinutes,
		&out.SustainedMinutes,
		&out.CooldownMinutes,
		&out.MinRequests,
		&out.NotifyEmail,
		&filtersRaw,
		&lastTriggeredAt,
		&out.CreatedAt,
		&out.UpdatedAt,
		&out.EvaluationRevision,
	); err != nil {
		return nil, err
	}
	if lastTriggeredAt.Valid {
		v := lastTriggeredAt.Time
		out.LastTriggeredAt = &v
	}
	if len(filtersRaw) > 0 && string(filtersRaw) != "null" {
		var decoded map[string]any
		if err := json.Unmarshal(filtersRaw, &decoded); err == nil {
			out.Filters = decoded
		}
	}

	return &out, nil
}

func (r *opsRepository) UpdateAlertRule(ctx context.Context, input *service.OpsAlertRule) (*service.OpsAlertRule, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}
	if input == nil {
		return nil, fmt.Errorf("nil input")
	}
	if input.ID <= 0 {
		return nil, fmt.Errorf("invalid id")
	}

	filtersArg, err := opsNullJSONMap(input.Filters)
	if err != nil {
		return nil, err
	}

	q := `
UPDATE ops_alert_rules
SET
  evaluation_revision = evaluation_revision + CASE WHEN
    ROW(enabled,severity,metric_type,operator,threshold,window_minutes,sustained_minutes,cooldown_minutes,min_requests,
      COALESCE(filters,'{}'::jsonb) - 'webhook_urls' - 'silenced_until')
    IS DISTINCT FROM
    ROW($4::boolean,$5::varchar,$6::varchar,$7::varchar,$8::numeric,$9::integer,$10::integer,$11::integer,$12::integer,
      COALESCE($14::jsonb,'{}'::jsonb) - 'webhook_urls' - 'silenced_until')
    THEN 1 ELSE 0 END,
  name = $2,
  description = $3,
  enabled = $4,
  severity = $5,
  metric_type = $6,
  operator = $7,
  threshold = $8,
  window_minutes = $9,
  sustained_minutes = $10,
  cooldown_minutes = $11,
  min_requests = $12,
  notify_email = $13,
  filters = $14,
  updated_at = NOW()
WHERE id = $1
RETURNING
  id,
  name,
  COALESCE(description, ''),
  enabled,
  COALESCE(severity, ''),
  metric_type,
  operator,
  threshold,
  window_minutes,
  sustained_minutes,
  cooldown_minutes,
  COALESCE(min_requests, 0),
  COALESCE(notify_email, true),
  filters,
  last_triggered_at,
  created_at,
  updated_at,
  evaluation_revision`

	var out service.OpsAlertRule
	var filtersRaw []byte
	var lastTriggeredAt sql.NullTime

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := tx.QueryRowContext(
		ctx,
		q,
		input.ID,
		strings.TrimSpace(input.Name),
		strings.TrimSpace(input.Description),
		input.Enabled,
		strings.TrimSpace(input.Severity),
		strings.TrimSpace(input.MetricType),
		strings.TrimSpace(input.Operator),
		input.Threshold,
		input.WindowMinutes,
		input.SustainedMinutes,
		input.CooldownMinutes,
		input.MinRequests,
		input.NotifyEmail,
		filtersArg,
	).Scan(
		&out.ID,
		&out.Name,
		&out.Description,
		&out.Enabled,
		&out.Severity,
		&out.MetricType,
		&out.Operator,
		&out.Threshold,
		&out.WindowMinutes,
		&out.SustainedMinutes,
		&out.CooldownMinutes,
		&out.MinRequests,
		&out.NotifyEmail,
		&filtersRaw,
		&lastTriggeredAt,
		&out.CreatedAt,
		&out.UpdatedAt,
		&out.EvaluationRevision,
	); err != nil {
		return nil, err
	}

	if lastTriggeredAt.Valid {
		v := lastTriggeredAt.Time
		out.LastTriggeredAt = &v
	}
	if len(filtersRaw) > 0 && string(filtersRaw) != "null" {
		var decoded map[string]any
		if err := json.Unmarshal(filtersRaw, &decoded); err == nil {
			out.Filters = decoded
		}
	}

	// Keep delivery-only edits from restarting the sustained timer. A changed
	// evaluation contract retires old events explicitly, never as a metric
	// recovery, and stale evaluators cannot write progress for the old revision.
	for _, table := range []string{"ops_alert_rule_states", "ops_alert_resource_states"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE rule_id=$1 AND rule_revision<>$2", input.ID, out.EvaluationRevision); err != nil {
			return nil, err
		}
	}
	// Use the same incident-before-event lock order as event transitions.
	rows, err := tx.QueryContext(ctx, `SELECT i.id FROM ops_alert_incidents i WHERE i.id IN
 (SELECT incident_id FROM ops_alert_events WHERE rule_id=$1 AND status='firing'
 AND COALESCE((dimensions->>'rule_revision')::bigint,1)<>$2) ORDER BY i.id FOR UPDATE`, input.ID, out.EvaluationRevision)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE ops_alert_events SET status='manual_resolved',resolved_at=NOW(),
 dimensions=COALESCE(dimensions,'{}'::jsonb)||'{"resolution_reason":"rule_reconfigured"}'::jsonb
 WHERE rule_id=$1 AND status='firing' AND COALESCE((dimensions->>'rule_revision')::bigint,1)<>$2`, input.ID, out.EvaluationRevision); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE ops_alert_incidents i SET status='resolved',resolved_at=NOW()
 WHERE status='firing' AND id IN (SELECT incident_id FROM ops_alert_events WHERE rule_id=$1)
 AND NOT EXISTS(SELECT 1 FROM ops_alert_events e WHERE e.incident_id=i.id AND e.status='firing')`, input.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *opsRepository) DeleteAlertRule(ctx context.Context, id int64) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("nil ops repository")
	}
	if id <= 0 {
		return fmt.Errorf("invalid id")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, "DELETE FROM ops_alert_rule_states WHERE rule_id = $1", id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM ops_alert_resource_states WHERE rule_id = $1", id); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, "DELETE FROM ops_alert_rules WHERE id = $1", id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

func (r *opsRepository) GetAlertRuleState(ctx context.Context, ruleID int64) (*service.OpsAlertRuleState, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}
	if ruleID <= 0 {
		return nil, nil
	}

	var state service.OpsAlertRuleState
	var lastEvaluatedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
SELECT rule_id, consecutive_breaches, last_evaluated_at, COALESCE(data_status, ''), rule_revision
FROM ops_alert_rule_states
WHERE rule_id = $1`, ruleID).Scan(
		&state.RuleID,
		&state.ConsecutiveBreaches,
		&lastEvaluatedAt,
		&state.DataStatus,
		&state.RuleRevision,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lastEvaluatedAt.Valid {
		state.LastEvaluatedAt = lastEvaluatedAt.Time
	}
	return &state, nil
}

func (r *opsRepository) UpsertAlertRuleState(ctx context.Context, state *service.OpsAlertRuleState) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("nil ops repository")
	}
	if state == nil || state.RuleID <= 0 {
		return fmt.Errorf("invalid alert rule state")
	}

	var lastEvaluatedAt any
	if !state.LastEvaluatedAt.IsZero() {
		lastEvaluatedAt = state.LastEvaluatedAt
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO ops_alert_rule_states (
  rule_id,
  consecutive_breaches,
  last_evaluated_at,
  data_status,
  rule_revision,
  updated_at
) SELECT $1, $2, $3, $4, $5, NOW()
FROM ops_alert_rules WHERE id=$1 AND evaluation_revision=$5 FOR SHARE
ON CONFLICT (rule_id) DO UPDATE SET
  consecutive_breaches = EXCLUDED.consecutive_breaches,
  last_evaluated_at = EXCLUDED.last_evaluated_at,
  data_status = EXCLUDED.data_status,
  rule_revision = EXCLUDED.rule_revision,
  updated_at = NOW()
WHERE ops_alert_rule_states.rule_revision<>EXCLUDED.rule_revision OR ops_alert_rule_states.last_evaluated_at IS NULL OR ops_alert_rule_states.last_evaluated_at < EXCLUDED.last_evaluated_at`,
		state.RuleID,
		state.ConsecutiveBreaches,
		lastEvaluatedAt,
		strings.TrimSpace(state.DataStatus),
		opsAlertRevision(state.RuleRevision),
	)
	return err
}

func (r *opsRepository) ListAlertEvents(ctx context.Context, filter *service.OpsAlertEventFilter) ([]*service.OpsAlertEvent, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}
	if filter == nil {
		filter = &service.OpsAlertEventFilter{}
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	where, args := buildOpsAlertEventsWhere(filter)
	args = append(args, limit)
	limitArg := "$" + itoa(len(args))

	q := `
SELECT
  id,
  COALESCE(rule_id, 0),
  COALESCE(severity, ''),
  COALESCE(status, ''),
  COALESCE(title, ''),
  COALESCE(description, ''),
  metric_value,
  threshold_value,
  dimensions,
  fired_at,
  resolved_at,
  email_sent,
  created_at
FROM ops_alert_events
` + where + `
ORDER BY fired_at DESC, id DESC
LIMIT ` + limitArg

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []*service.OpsAlertEvent{}
	for rows.Next() {
		var ev service.OpsAlertEvent
		var metricValue sql.NullFloat64
		var thresholdValue sql.NullFloat64
		var dimensionsRaw []byte
		var resolvedAt sql.NullTime
		if err := rows.Scan(
			&ev.ID,
			&ev.RuleID,
			&ev.Severity,
			&ev.Status,
			&ev.Title,
			&ev.Description,
			&metricValue,
			&thresholdValue,
			&dimensionsRaw,
			&ev.FiredAt,
			&resolvedAt,
			&ev.EmailSent,
			&ev.CreatedAt,
		); err != nil {
			return nil, err
		}
		if metricValue.Valid {
			v := metricValue.Float64
			ev.MetricValue = &v
		}
		if thresholdValue.Valid {
			v := thresholdValue.Float64
			ev.ThresholdValue = &v
		}
		if resolvedAt.Valid {
			v := resolvedAt.Time
			ev.ResolvedAt = &v
		}
		if len(dimensionsRaw) > 0 && string(dimensionsRaw) != "null" {
			var decoded map[string]any
			if err := json.Unmarshal(dimensionsRaw, &decoded); err == nil {
				ev.Dimensions = decoded
			}
		}
		out = append(out, &ev)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *opsRepository) GetAlertEventByID(ctx context.Context, eventID int64) (*service.OpsAlertEvent, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}
	if eventID <= 0 {
		return nil, fmt.Errorf("invalid event id")
	}

	q := `
SELECT
  id,
  COALESCE(rule_id, 0),
  COALESCE(severity, ''),
  COALESCE(status, ''),
  COALESCE(title, ''),
  COALESCE(description, ''),
  metric_value,
  threshold_value,
  dimensions,
  fired_at,
  resolved_at,
  email_sent,
  created_at
FROM ops_alert_events
WHERE id = $1`

	row := r.db.QueryRowContext(ctx, q, eventID)
	ev, err := scanOpsAlertEvent(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return ev, nil
}

func (r *opsRepository) GetActiveAlertEvent(ctx context.Context, ruleID int64) (*service.OpsAlertEvent, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}
	if ruleID <= 0 {
		return nil, fmt.Errorf("invalid rule id")
	}

	q := `
SELECT
  id,
  COALESCE(rule_id, 0),
  COALESCE(severity, ''),
  COALESCE(status, ''),
  COALESCE(title, ''),
  COALESCE(description, ''),
  metric_value,
  threshold_value,
  dimensions,
  fired_at,
  resolved_at,
  email_sent,
  created_at
FROM ops_alert_events
WHERE rule_id = $1 AND status = $2
ORDER BY fired_at DESC
LIMIT 1`

	row := r.db.QueryRowContext(ctx, q, ruleID, service.OpsAlertStatusFiring)
	ev, err := scanOpsAlertEvent(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return ev, nil
}

func (r *opsRepository) GetLatestAlertEvent(ctx context.Context, ruleID int64) (*service.OpsAlertEvent, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}
	if ruleID <= 0 {
		return nil, fmt.Errorf("invalid rule id")
	}

	q := `
SELECT
  id,
  COALESCE(rule_id, 0),
  COALESCE(severity, ''),
  COALESCE(status, ''),
  COALESCE(title, ''),
  COALESCE(description, ''),
  metric_value,
  threshold_value,
  dimensions,
  fired_at,
  resolved_at,
  email_sent,
  created_at
FROM ops_alert_events
WHERE rule_id = $1
ORDER BY fired_at DESC
LIMIT 1`

	row := r.db.QueryRowContext(ctx, q, ruleID)
	ev, err := scanOpsAlertEvent(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return ev, nil
}

func (r *opsRepository) CreateAlertEvent(ctx context.Context, event *service.OpsAlertEvent) (*service.OpsAlertEvent, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}
	if event == nil {
		return nil, fmt.Errorf("nil event")
	}

	q := `
INSERT INTO ops_alert_events (
  rule_id,
  resource_key,
  incident_id,
  severity,
  status,
  title,
  description,
  metric_value,
  threshold_value,
  dimensions,
  fired_at,
  resolved_at,
  email_sent,
  created_at
) VALUES (
  $1,$12,$13,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW()
)
ON CONFLICT (rule_id, resource_key) WHERE status = 'firing' DO NOTHING
RETURNING
  id,
  COALESCE(rule_id, 0),
  COALESCE(severity, ''),
  COALESCE(status, ''),
  COALESCE(title, ''),
  COALESCE(description, ''),
  metric_value,
  threshold_value,
  dimensions,
  fired_at,
  resolved_at,
  email_sent,
  created_at`

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var currentRevision int64
	if err := tx.QueryRowContext(ctx, `SELECT evaluation_revision FROM ops_alert_rules WHERE id=$1 FOR SHARE`, event.RuleID).Scan(&currentRevision); err != nil {
		return nil, err
	}
	if currentRevision != opsAlertRevision(event.RuleRevision) {
		return nil, fmt.Errorf("alert rule changed during evaluation")
	}
	if event.Dimensions == nil {
		event.Dimensions = map[string]any{}
	}
	event.Dimensions["rule_revision"] = currentRevision
	resourceKey, _ := event.Dimensions["resource_key"].(string)
	incidentResource := resourceKey
	if incidentResource == "" {
		incidentResource = fmt.Sprintf("global:%v:%v", event.Dimensions["platform"], event.Dimensions["group_id"])
	}
	var incidentID int64
	if err := tx.QueryRowContext(ctx, `INSERT INTO ops_alert_incidents(resource_key,severity,opened_at) VALUES($1,$2,$3)
 ON CONFLICT(resource_key) WHERE status='firing' DO UPDATE SET severity=LEAST(ops_alert_incidents.severity,EXCLUDED.severity)
 RETURNING id`, incidentResource, event.Severity, event.FiredAt).Scan(&incidentID); err != nil {
		return nil, err
	}
	if event.Dimensions == nil {
		event.Dimensions = map[string]any{}
	}
	event.Dimensions["incident_id"] = incidentID
	event.Dimensions["incident_key"] = fmt.Sprintf("incident-%d", incidentID)
	dimensionsArg, err := opsNullJSONMap(event.Dimensions)
	if err != nil {
		return nil, err
	}
	row := tx.QueryRowContext(
		ctx,
		q,
		opsNullInt64(&event.RuleID),
		opsNullString(event.Severity),
		opsNullString(event.Status),
		opsNullString(event.Title),
		opsNullString(event.Description),
		opsNullFloat64(event.MetricValue),
		opsNullFloat64(event.ThresholdValue),
		dimensionsArg,
		event.FiredAt,
		opsNullTime(event.ResolvedAt),
		event.EmailSent,
		resourceKey,
		incidentID,
	)
	created, err := scanOpsAlertEvent(row)
	if err == sql.ErrNoRows {
		return nil, nil
	} // Another evaluator already opened this resource.
	if err != nil {
		return nil, err
	}
	if err := enqueueAlertDeliveries(ctx, tx, created.ID, event.Deliveries); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE ops_alert_rules SET last_triggered_at=$2 WHERE id=$1", event.RuleID, event.FiredAt); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return created, nil
}

func (r *opsRepository) UpdateAlertEventStatus(ctx context.Context, eventID int64, status string, resolvedAt *time.Time) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("nil ops repository")
	}
	if eventID <= 0 {
		return fmt.Errorf("invalid event id")
	}
	if strings.TrimSpace(status) == "" {
		return fmt.Errorf("invalid status")
	}

	_, err := r.TransitionAlertEvent(ctx, &service.OpsAlertEvent{ID: eventID, Status: strings.TrimSpace(status), ResolvedAt: resolvedAt}, nil)
	return err
}

func (r *opsRepository) UpdateAlertEventEmailSent(ctx context.Context, eventID int64, emailSent bool) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("nil ops repository")
	}
	if eventID <= 0 {
		return fmt.Errorf("invalid event id")
	}

	_, err := r.db.ExecContext(ctx, "UPDATE ops_alert_events SET email_sent = $2 WHERE id = $1", eventID, emailSent)
	return err
}

type opsAlertEventRow interface {
	Scan(dest ...any) error
}

func (r *opsRepository) CreateAlertSilence(ctx context.Context, input *service.OpsAlertSilence) (*service.OpsAlertSilence, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}
	if input == nil {
		return nil, fmt.Errorf("nil input")
	}
	if input.RuleID <= 0 {
		return nil, fmt.Errorf("invalid rule_id")
	}
	platform := strings.TrimSpace(input.Platform)
	if platform == "" && strings.TrimSpace(input.ResourceKey) == "" {
		return nil, fmt.Errorf("invalid platform")
	}
	if input.Until.IsZero() {
		return nil, fmt.Errorf("invalid until")
	}

	q := `
INSERT INTO ops_alert_silences (
  rule_id,
  resource_key,
  platform,
  group_id,
  region,
  until,
  reason,
  created_by,
  created_at
) VALUES (
  $1,$8,$2,$3,$4,$5,$6,$7,NOW()
)
RETURNING id, rule_id, resource_key, platform, group_id, region, until, COALESCE(reason,''), created_by, created_at`

	row := r.db.QueryRowContext(
		ctx,
		q,
		input.RuleID,
		platform,
		opsNullInt64(input.GroupID),
		opsNullString(input.Region),
		input.Until,
		opsNullString(input.Reason),
		opsNullInt64(input.CreatedBy),
		strings.TrimSpace(input.ResourceKey),
	)

	var out service.OpsAlertSilence
	var groupID sql.NullInt64
	var region sql.NullString
	var createdBy sql.NullInt64
	if err := row.Scan(
		&out.ID,
		&out.RuleID,
		&out.ResourceKey,
		&out.Platform,
		&groupID,
		&region,
		&out.Until,
		&out.Reason,
		&createdBy,
		&out.CreatedAt,
	); err != nil {
		return nil, err
	}
	if groupID.Valid {
		v := groupID.Int64
		out.GroupID = &v
	}
	if region.Valid {
		v := strings.TrimSpace(region.String)
		if v != "" {
			out.Region = &v
		}
	}
	if createdBy.Valid {
		v := createdBy.Int64
		out.CreatedBy = &v
	}
	return &out, nil
}

func (r *opsRepository) IsAlertSilenced(ctx context.Context, ruleID int64, platform string, groupID *int64, region *string, now time.Time) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("nil ops repository")
	}
	if ruleID <= 0 {
		return false, fmt.Errorf("invalid rule id")
	}
	platform = strings.TrimSpace(platform)
	if platform == "" {
		return false, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	q := `
SELECT 1
FROM ops_alert_silences
WHERE rule_id = $1
  AND resource_key = ''
  AND platform = $2
  AND (group_id IS NOT DISTINCT FROM $3)
  AND (region IS NOT DISTINCT FROM $4)
  AND until > $5
LIMIT 1`

	var dummy int
	err := r.db.QueryRowContext(ctx, q, ruleID, platform, opsNullInt64(groupID), opsNullString(region), now).Scan(&dummy)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func scanOpsAlertEvent(row opsAlertEventRow) (*service.OpsAlertEvent, error) {
	var ev service.OpsAlertEvent
	var metricValue sql.NullFloat64
	var thresholdValue sql.NullFloat64
	var dimensionsRaw []byte
	var resolvedAt sql.NullTime

	if err := row.Scan(
		&ev.ID,
		&ev.RuleID,
		&ev.Severity,
		&ev.Status,
		&ev.Title,
		&ev.Description,
		&metricValue,
		&thresholdValue,
		&dimensionsRaw,
		&ev.FiredAt,
		&resolvedAt,
		&ev.EmailSent,
		&ev.CreatedAt,
	); err != nil {
		return nil, err
	}
	if metricValue.Valid {
		v := metricValue.Float64
		ev.MetricValue = &v
	}
	if thresholdValue.Valid {
		v := thresholdValue.Float64
		ev.ThresholdValue = &v
	}
	if resolvedAt.Valid {
		v := resolvedAt.Time
		ev.ResolvedAt = &v
	}
	if len(dimensionsRaw) > 0 && string(dimensionsRaw) != "null" {
		var decoded map[string]any
		if err := json.Unmarshal(dimensionsRaw, &decoded); err == nil {
			ev.Dimensions = decoded
		}
	}
	ev.RuleRevision = 1
	if revision, ok := ev.Dimensions["rule_revision"].(float64); ok {
		ev.RuleRevision = int64(revision)
	}
	return &ev, nil
}

func buildOpsAlertEventsWhere(filter *service.OpsAlertEventFilter) (string, []any) {
	clauses := []string{"1=1"}
	args := []any{}

	if filter == nil {
		return "WHERE " + strings.Join(clauses, " AND "), args
	}

	if filter.RuleID > 0 {
		args = append(args, filter.RuleID)
		clauses = append(clauses, "rule_id = $"+itoa(len(args)))
	}
	if filter.ResourceKey != "" {
		args = append(args, filter.ResourceKey)
		clauses = append(clauses, "resource_key = $"+itoa(len(args)))
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		args = append(args, status)
		clauses = append(clauses, "status = $"+itoa(len(args)))
	}
	if severity := strings.TrimSpace(filter.Severity); severity != "" {
		args = append(args, severity)
		clauses = append(clauses, "severity = $"+itoa(len(args)))
	}
	if filter.EmailSent != nil {
		args = append(args, *filter.EmailSent)
		clauses = append(clauses, "email_sent = $"+itoa(len(args)))
	}
	if filter.StartTime != nil && !filter.StartTime.IsZero() {
		args = append(args, *filter.StartTime)
		clauses = append(clauses, "fired_at >= $"+itoa(len(args)))
	}
	if filter.EndTime != nil && !filter.EndTime.IsZero() {
		args = append(args, *filter.EndTime)
		clauses = append(clauses, "fired_at < $"+itoa(len(args)))
	}

	// Cursor pagination (descending by fired_at, then id)
	if filter.BeforeFiredAt != nil && !filter.BeforeFiredAt.IsZero() && filter.BeforeID != nil && *filter.BeforeID > 0 {
		args = append(args, *filter.BeforeFiredAt)
		tsArg := "$" + itoa(len(args))
		args = append(args, *filter.BeforeID)
		idArg := "$" + itoa(len(args))
		clauses = append(clauses, fmt.Sprintf("(fired_at < %s OR (fired_at = %s AND id < %s))", tsArg, tsArg, idArg))
	}
	// Dimensions are stored in JSONB. We filter best-effort without requiring GIN indexes.
	if platform := strings.TrimSpace(filter.Platform); platform != "" {
		args = append(args, platform)
		clauses = append(clauses, "(dimensions->>'platform') = $"+itoa(len(args)))
	}
	if filter.GroupID != nil && *filter.GroupID > 0 {
		args = append(args, fmt.Sprintf("%d", *filter.GroupID))
		clauses = append(clauses, "(dimensions->>'group_id') = $"+itoa(len(args)))
	}

	return "WHERE " + strings.Join(clauses, " AND "), args
}

func opsNullJSONMap(v map[string]any) (any, error) {
	if v == nil {
		return sql.NullString{}, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return sql.NullString{}, nil
	}
	return sql.NullString{String: string(b), Valid: true}, nil
}

func opsAlertRevision(revision int64) int64 {
	if revision < 1 {
		return 1
	}
	return revision
}
