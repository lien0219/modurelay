package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const upsertMonitoringEventSQL = `INSERT INTO ops_kubernetes_events
(cluster,namespace,event_uid,pod,pod_uid,reason,event_type,message,source,first_seen_at,last_seen_at,occurrence_count,object_kind,object_name,object_uid)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
ON CONFLICT(cluster,namespace,event_uid) DO UPDATE SET
  first_seen_at=LEAST(ops_kubernetes_events.first_seen_at,EXCLUDED.first_seen_at),
  last_seen_at=GREATEST(ops_kubernetes_events.last_seen_at,EXCLUDED.last_seen_at),
  occurrence_count=GREATEST(ops_kubernetes_events.occurrence_count,EXCLUDED.occurrence_count),
  message=CASE WHEN EXCLUDED.last_seen_at >= ops_kubernetes_events.last_seen_at THEN EXCLUDED.message ELSE ops_kubernetes_events.message END,
  collected_at=NOW()
WHERE ops_kubernetes_events.object_uid=EXCLUDED.object_uid AND ops_kubernetes_events.object_kind=EXCLUDED.object_kind`

func (r *opsRepository) UpsertMonitoringEvents(ctx context.Context, events []service.OpsKubernetesEvent) error {
	if len(events) == 0 {
		return nil
	}
	if len(events) > 10000 {
		return fmt.Errorf("too many Kubernetes events")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, upsertMonitoringEventSQL)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, event := range events {
		if event.ObjectKind == "" {
			event.ObjectKind, event.ObjectName, event.ObjectUID = "Pod", event.Pod, event.PodUID
		}

		if event.Cluster == "" || event.Namespace == "" || event.EventUID == "" || event.ObjectUID == "" || event.ObjectName == "" || event.FirstSeenAt.IsZero() || event.LastSeenAt.IsZero() {
			return fmt.Errorf("invalid Kubernetes event identity")
		}
		if _, err := stmt.ExecContext(ctx, event.Cluster, event.Namespace, event.EventUID, event.Pod, event.PodUID, event.Reason, event.Type, event.Message, event.Source, event.FirstSeenAt, event.LastSeenAt, event.Count, event.ObjectKind, event.ObjectName, event.ObjectUID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *opsRepository) ListMonitoringEvents(ctx context.Context, f service.OpsMonitoringEventsFilter) ([]service.OpsKubernetesEvent, error) {
	args := []any{f.Start, f.End}
	where := []string{"last_seen_at >= $1", "first_seen_at <= $2"}
	for _, filter := range []struct{ column, value string }{{"cluster", f.Cluster}, {"namespace", f.Namespace}, {"pod", f.Pod}, {"pod_uid", f.UID}, {"object_kind", f.ObjectKind}, {"object_name", f.ObjectName}, {"object_uid", f.ObjectUID}} {
		if filter.value != "" {
			args = append(args, filter.value)
			where = append(where, fmt.Sprintf("%s = $%d", filter.column, len(args)))
		}
	}
	limit := f.Limit
	if limit < 1 || limit > 500 {
		limit = 100
	}
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, `SELECT cluster,namespace,event_uid,pod,pod_uid,reason,event_type,message,source,first_seen_at,last_seen_at,occurrence_count,object_kind,object_name,object_uid FROM ops_kubernetes_events WHERE `+strings.Join(where, " AND ")+fmt.Sprintf(" ORDER BY last_seen_at DESC,event_uid DESC LIMIT $%d", len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []service.OpsKubernetesEvent{}
	for rows.Next() {
		var event service.OpsKubernetesEvent
		if err := rows.Scan(&event.Cluster, &event.Namespace, &event.EventUID, &event.Pod, &event.PodUID, &event.Reason, &event.Type, &event.Message, &event.Source, &event.FirstSeenAt, &event.LastSeenAt, &event.Count, &event.ObjectKind, &event.ObjectName, &event.ObjectUID); err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	return out, rows.Err()
}

func (r *opsRepository) DeleteExpiredMonitoringEvents(ctx context.Context, cluster string, before time.Time) error {
	// Bound each cleanup batch to avoid monopolizing the shared database.
	_, err := r.db.ExecContext(ctx, `DELETE FROM ops_kubernetes_events WHERE ctid IN (SELECT ctid FROM ops_kubernetes_events WHERE cluster=$1 AND last_seen_at<$2 ORDER BY last_seen_at LIMIT 5000)`, cluster, before)
	return err
}

var _ service.OpsMonitoringEventsRepository = (*opsRepository)(nil)
