package repository

import (
	"context"
	"errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"time"
)

func (r *opsRepository) ReplaceMonitoringPodLimits(ctx context.Context, cluster, namespace string, rows []service.OpsMonitoringPodLimit) error {
	if cluster == "" || len(rows) > 20000 {
		return errors.New("invalid Kubernetes limit snapshot")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Scope deletion by the collector's namespace; never merge two Pod UIDs.
	if namespace == "*" {
		_, err = tx.ExecContext(ctx, "DELETE FROM ops_kubernetes_pod_limits WHERE cluster=$1", cluster)
	} else {
		_, err = tx.ExecContext(ctx, "DELETE FROM ops_kubernetes_pod_limits WHERE cluster=$1 AND namespace=$2", cluster, namespace)
	}
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.Cluster != cluster || row.Namespace == "" || row.Pod == "" || row.UID == "" || row.ObservedAt.IsZero() || (namespace != "*" && row.Namespace != namespace) {
			return errors.New("invalid Kubernetes limit identity")
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO ops_kubernetes_pod_limits(cluster,namespace,pod,uid,resource,limit_value,bounded,observed_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, row.Cluster, row.Namespace, row.Pod, row.UID, row.Resource, row.Limit, row.Bounded, row.ObservedAt)
		if err != nil {
			return err
		}
	}
	// Stale snapshots cannot accumulate after a namespace or cluster is removed.
	_, err = tx.ExecContext(ctx, "DELETE FROM ops_kubernetes_pod_limits WHERE cluster=$1 AND observed_at < NOW() - INTERVAL '1 day'", cluster)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (r *opsRepository) ListMonitoringPodLimits(ctx context.Context, cluster string, since time.Time) ([]service.OpsMonitoringPodLimit, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT cluster,namespace,pod,uid,resource,limit_value,bounded,observed_at FROM ops_kubernetes_pod_limits WHERE cluster=$1 AND observed_at>=$2 ORDER BY namespace,uid,resource LIMIT 20000`, cluster, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []service.OpsMonitoringPodLimit{}
	for rows.Next() {
		var row service.OpsMonitoringPodLimit
		if err := rows.Scan(&row.Cluster, &row.Namespace, &row.Pod, &row.UID, &row.Resource, &row.Limit, &row.Bounded, &row.ObservedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

var _ service.OpsMonitoringLimitsRepository = (*opsRepository)(nil)
