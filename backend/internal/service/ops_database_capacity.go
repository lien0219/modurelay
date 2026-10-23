package service

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/dbpoolstats"
)

// Shared database-instance gauges are sampled once by the collector leader.
// They are never summed across application replicas.
type OpsDatabaseInstanceCapacity struct {
	State  string                        `json:"state"`
	Source string                        `json:"source"`
	Used   *int64                        `json:"used"`
	Limit  *int64                        `json:"limit"`
	Node   string                        `json:"node,omitempty"`
	Nodes  []OpsDatabaseInstanceCapacity `json:"nodes,omitempty"`
}
type OpsDatabasePoolStats struct {
	Instance    string  `json:"instance"`
	Role        string  `json:"role"`
	Active      int     `json:"active"`
	Idle        int     `json:"idle"`
	Limit       int     `json:"limit"`
	WaitCount   int64   `json:"wait_count"`
	WaitSeconds float64 `json:"wait_seconds"`
}
type OpsDatabaseCapacity struct {
	PostgreSQL          OpsDatabaseInstanceCapacity `json:"postgresql"`
	Redis               OpsDatabaseInstanceCapacity `json:"redis"`
	PostgreSQLPoolLimit *int                        `json:"postgresql_pool_limit"`
	RedisPoolLimit      *int                        `json:"redis_pool_limit"`
	DatabasePools       []OpsDatabasePoolStats      `json:"database_pools,omitempty"`
}

// max_connections controls ordinary client backend slots. WAL senders have a
// separate max_wal_senders limit; background and parallel workers are not clients.
const postgresCapacitySQL = `SELECT (SELECT COUNT(*) FROM pg_stat_activity WHERE backend_type = 'client backend'), current_setting('max_connections')::bigint`

func (c *OpsMetricsCollector) collectDatabaseCapacity(ctx context.Context) *OpsDatabaseCapacity {
	out := &OpsDatabaseCapacity{PostgreSQL: OpsDatabaseInstanceCapacity{State: "Unknown", Source: "PostgreSQL client backend slots / max_connections (includes reserved client slots)"}, Redis: OpsDatabaseInstanceCapacity{State: "Unknown", Source: "Redis INFO clients"}}
	if c == nil {
		return out
	}
	if c.db != nil {
		queryCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		var used, limit int64
		err := c.db.QueryRowContext(queryCtx, postgresCapacitySQL).Scan(&used, &limit)
		cancel()
		if err == nil && used >= 0 && limit > 0 {
			out.PostgreSQL.State = "Available"
			out.PostgreSQL.Used = &used
			out.PostgreSQL.Limit = &limit
		}
	}
	out.Redis = c.collectRedisInstanceCapacity(ctx)
	out.DatabasePools = c.databasePoolStats()
	return out
}

func (c *OpsMetricsCollector) databasePoolStats() []OpsDatabasePoolStats {
	if c == nil {
		return nil
	}
	out := []OpsDatabasePoolStats{}
	for role, stats := range dbpoolstats.Snapshot(c.db) {
		out = append(out, OpsDatabasePoolStats{Instance: c.instanceID, Role: role, Active: stats.InUse, Idle: stats.Idle, Limit: stats.MaxOpenConnections, WaitCount: stats.WaitCount, WaitSeconds: stats.WaitDuration.Seconds()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Role < out[j].Role })
	return out
}

func (s *OpsService) InstanceDatabasePools() []OpsDatabasePoolStats {
	if s == nil || s.metricsCollector == nil {
		return nil
	}
	return s.metricsCollector.databasePoolStats()
}
func parseRedisInstanceCapacity(raw string) OpsDatabaseInstanceCapacity {
	out := OpsDatabaseInstanceCapacity{State: "Unknown", Source: "Redis INFO clients"}
	for _, line := range strings.Split(raw, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil || n < 0 {
			continue
		}
		switch key {
		case "connected_clients":
			out.Used = &n
		case "maxclients":
			if n > 0 {
				out.Limit = &n
			}
		}
	}
	if out.Used != nil && out.Limit != nil {
		out.State = "Available"
	}
	return out
}

// InstancePoolCounters reads process-owned monotonically increasing counters.
// Callers must attach the instance scrape identity and rate/increase each series
// before aggregation; summing raw counters then differentiating is incorrect.
func (s *OpsService) InstancePoolCounters() map[string]float64 {
	if s == nil || s.metricsCollector == nil {
		return nil
	}
	c := s.metricsCollector
	out := map[string]float64{}
	if c.db != nil {
		stats := c.db.Stats()
		out["modurelay_db_pool_wait_total"] = float64(stats.WaitCount)
		out["modurelay_db_pool_wait_duration_seconds_total"] = stats.WaitDuration.Seconds()
		out["modurelay_db_pool_open_connections"] = float64(stats.OpenConnections)
		if stats.MaxOpenConnections > 0 {
			out["modurelay_db_pool_max_open_connections"] = float64(stats.MaxOpenConnections)
		}
	}
	c.appendRedisPoolCounters(out)
	return out
}

func databaseInstanceUsage(capacity OpsDatabaseInstanceCapacity) (float64, alertMetricStatus) {
	if capacity.State != "Available" || capacity.Used == nil || *capacity.Used < 0 || capacity.Limit == nil || *capacity.Limit <= 0 {
		return 0, alertMetricUnavailable
	}
	return 100 * float64(*capacity.Used) / float64(*capacity.Limit), alertMetricOK
}

// DatabaseInstanceCapacity returns the shared leader-collected gauge, never a
// sum across application replicas. Exporters omit unavailable or stale values.
func (s *OpsService) DatabaseInstanceCapacity(ctx context.Context) *OpsDatabaseCapacity {
	if s == nil || s.opsRepo == nil {
		return nil
	}
	snapshot, err := s.opsRepo.GetLatestSystemMetrics(ctx, 1)
	if err != nil || snapshot == nil || snapshot.CreatedAt.IsZero() || time.Since(snapshot.CreatedAt) > 3*time.Minute {
		return nil
	}
	return snapshot.Capacity
}

func usesLocalDatabaseCapacity(rule *OpsAlertRule) bool {
	if rule == nil || (rule.MetricType != "db_instance_usage_percent" && rule.MetricType != "redis_instance_usage_percent") {
		return false
	}
	for _, key := range []string{"cluster", "namespace", "service", "deployment", "pod", "uid", "source"} {
		if raw, ok := rule.Filters[key].(string); ok && raw != "" {
			return false
		}
	}
	return true
}
