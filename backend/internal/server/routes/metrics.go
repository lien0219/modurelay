package routes

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httpmetrics"
	"github.com/Wei-Shaw/sub2api/internal/pkg/lifecycle"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type clusterMetricsSource interface {
	GetClusterMetrics(context.Context) (*service.OpsWindowStats, *service.OpsSystemMetricsSnapshot, error)
}

// RegisterMetricsRoutes exposes one logical cluster scrape target. Every pod
// returns the same shared traffic totals; summing targets would double-count.
func RegisterMetricsRoutes(r *gin.Engine, cfg *config.Config, source clusterMetricsSource) {
	if cfg == nil || !cfg.Ops.Metrics.Enabled {
		return
	}
	token := strings.TrimSpace(cfg.Ops.Metrics.Token)
	expected := sha256.Sum256([]byte(token))
	// This endpoint is local to the scraped instance; it deliberately does not
	// query Redis/DB or change the cluster-wide semantics of /metrics.
	r.GET("/metrics/instance", func(c *gin.Context) {
		provided, valid := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		actual := sha256.Sum256([]byte(provided))
		if token == "" || !valid || subtle.ConstantTimeCompare(actual[:], expected[:]) != 1 {
			c.Header("WWW-Authenticate", "Bearer")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		manager := lifecycle.ManagerFromContext(c.Request.Context())
		if manager == nil {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		snapshot := manager.Snapshot()
		draining := 0
		if snapshot.Draining {
			draining = 1
		}
		var out strings.Builder
		gauge := func(name, help string, value int) {
			fmt.Fprintf(&out, "# HELP %s %s\n# TYPE %s gauge\n%s %d\n", name, help, name, name, value)
		}
		gauge("modurelay_instance_draining", "Whether this instance is draining (1) or accepting traffic (0).", draining)
		gauge("modurelay_instance_active_http_requests", "HTTP handlers still running on this instance, excluding probes and metrics.", snapshot.ActiveHTTP)
		gauge("modurelay_instance_active_websocket_sessions", "Upgraded sessions still running or finalizing on this instance.", snapshot.ActiveWebSockets)
		gauge("modurelay_instance_active_websocket_turns", "Admitted Responses WebSocket turns still running on this instance.", snapshot.ActiveTurns)
		// Instance counters and histograms. Do not copy these onto /metrics;
		// that endpoint returns cluster gauges which must not be summed per pod.
		out.WriteString(httpmetrics.Render())
		if observation, ok := source.(interface{ BillingObservationCounters() map[string]float64 }); ok {
			values := observation.BillingObservationCounters()
			for _, metric := range []struct{ name, kind, help string }{
				{"modurelay_billing_observation_missed_total", "counter", "Billing observations missed on this instance; financial operations remain unaffected."},
				{"modurelay_billing_observation_unpersisted", "gauge", "Missed observations on this instance awaiting durable gap recording."},
				{"modurelay_billing_observation_last_failure_timestamp_seconds", "gauge", "Unix timestamp of the latest missed billing observation on this instance."},
			} {
				if value, present := values[metric.name]; present {
					fmt.Fprintf(&out, "# HELP %s %s\n# TYPE %s %s\n%s %.12g\n", metric.name, metric.help, metric.name, metric.kind, metric.name, value)
				}
			}
		}
		if pools, ok := source.(interface {
			InstanceDatabasePools() []service.OpsDatabasePoolStats
		}); ok {
			stats := pools.InstanceDatabasePools()
			for _, metric := range []struct {
				suffix, kind string
				value        func(service.OpsDatabasePoolStats) float64
			}{
				{"open_connections", "gauge", func(p service.OpsDatabasePoolStats) float64 { return float64(p.Active + p.Idle) }},
				{"max_open_connections", "gauge", func(p service.OpsDatabasePoolStats) float64 { return float64(p.Limit) }},
				{"wait_total", "counter", func(p service.OpsDatabasePoolStats) float64 { return float64(p.WaitCount) }},
				{"wait_duration_seconds_total", "counter", func(p service.OpsDatabasePoolStats) float64 { return p.WaitSeconds }},
			} {
				name := "modurelay_database_pool_" + metric.suffix
				fmt.Fprintf(&out, "# HELP %s Local SQL pool %s; pool role is bounded.\n# TYPE %s %s\n", name, metric.suffix, name, metric.kind)
				for _, pool := range stats {
					if pool.Role != "primary" && pool.Role != "advisory_lock" && pool.Role != "read_replica" {
						continue
					}
					fmt.Fprintf(&out, "%s{pool=%q} %.12g\n", name, pool.Role, metric.value(pool))
				}
			}
		}
		if pools, ok := source.(interface{ InstancePoolCounters() map[string]float64 }); ok {
			values := pools.InstancePoolCounters()
			for _, metric := range []struct{ name, kind, help string }{
				{"modurelay_db_pool_wait_total", "counter", "Cumulative waits for this instance primary SQL pool."},
				{"modurelay_db_pool_wait_duration_seconds_total", "counter", "Cumulative seconds waiting for this instance primary SQL pool."},
				{"modurelay_redis_pool_timeout_total", "counter", "Cumulative Redis pool timeouts for this instance."},
				{"modurelay_db_pool_open_connections", "gauge", "Open connections in this instance primary SQL pool."},
				{"modurelay_db_pool_max_open_connections", "gauge", "Configured maximum connections in this instance primary SQL pool; zero means unlimited."},
				{"modurelay_redis_pool_connections", "gauge", "Open Redis connections from this instance."},
				{"modurelay_redis_pool_max_connections", "gauge", "Configured Redis connection budget for this instance."},
			} {
				if value, present := values[metric.name]; present {
					fmt.Fprintf(&out, "# HELP %s %s\n# TYPE %s %s\n%s %.12g\n", metric.name, metric.help, metric.name, metric.kind, metric.name, value)
				}
			}
		}
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8", []byte(out.String()))
	})
	r.GET("/metrics", func(c *gin.Context) {
		provided, valid := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		actual := sha256.Sum256([]byte(provided))
		if token == "" || !valid || subtle.ConstantTimeCompare(actual[:], expected[:]) != 1 {
			c.Header("WWW-Authenticate", "Bearer")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()
		if source == nil {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		stats, snapshot, err := source.GetClusterMetrics(ctx)
		if err != nil || stats == nil || snapshot == nil {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		var out strings.Builder
		gauge := func(name, help string, value float64) {
			fmt.Fprintf(&out, "# HELP %s %s\n# TYPE %s gauge\n%s %.12g\n", name, help, name, name, value)
		}
		gauge("modurelay_cluster_qps", "Completed requests per second across the cluster over the last 60 seconds.", float64(stats.SuccessCount+stats.ErrorCountTotal)/60)
		gauge("modurelay_cluster_tps", "Input output and cache tokens per second across the cluster over the last 60 seconds.", float64(stats.TokenConsumed)/60)
		gauge("modurelay_cluster_requests_window", "Completed successful and failed requests across the cluster in the last 60 seconds.", float64(stats.SuccessCount+stats.ErrorCountTotal))
		gauge("modurelay_cluster_errors_window", "Failed requests across the cluster in the last 60 seconds.", float64(stats.ErrorCountTotal))
		gauge("modurelay_cluster_tokens_window", "Tokens across the cluster in the last 60 seconds.", float64(stats.TokenConsumed))
		gauge("modurelay_cluster_window_end_timestamp_seconds", "End timestamp of the shared traffic measurement window.", float64(stats.EndTime.Unix()))
		gauge("modurelay_cluster_instances", "Instances with a fresh runtime heartbeat; expired instances are excluded after 45 seconds.", float64(snapshot.InstanceCount))
		gauge("modurelay_cluster_runtime_timestamp_seconds", "Newest runtime heartbeat timestamp.", float64(snapshot.CreatedAt.Unix()))
		if capacitySource, ok := source.(interface {
			DatabaseInstanceCapacity(context.Context) *service.OpsDatabaseCapacity
		}); ok {
			if capacity := capacitySource.DatabaseInstanceCapacity(ctx); capacity != nil {
				for _, item := range []struct {
					prefix string
					value  service.OpsDatabaseInstanceCapacity
				}{
					{"modurelay_database_instance", capacity.PostgreSQL},
					{"modurelay_redis_instance", capacity.Redis},
				} {
					if item.value.State == "Available" && item.value.Used != nil && item.value.Limit != nil && *item.value.Limit > 0 {
						gauge(item.prefix+"_connections", "Shared database-server connections; do not sum across application pods.", float64(*item.value.Used))
						gauge(item.prefix+"_max_connections", "Shared database-server connection capacity from standard server protocol.", float64(*item.value.Limit))
					}
				}
			}
		}
		if limits, ok := source.(interface{ MonitoringPodLimitMetrics(context.Context) string }); ok {
			out.WriteString(limits.MonitoringPodLimitMetrics(ctx))
		}
		if jobs, ok := source.(interface {
			MonitoringJobHeartbeats(context.Context) (map[string]float64, error)
		}); ok {
			if heartbeats, err := jobs.MonitoringJobHeartbeats(ctx); err == nil {
				out.WriteString("# HELP modurelay_ops_job_last_success_timestamp_seconds Last successful shared Ops background job.\n# TYPE modurelay_ops_job_last_success_timestamp_seconds gauge\n")
				for _, job := range []string{"ops_metrics_collector", "ops_alert_evaluator"} {
					if timestamp, present := heartbeats[job]; present {
						fmt.Fprintf(&out, "modurelay_ops_job_last_success_timestamp_seconds{job_name=%q} %.12g\n", job, timestamp)
					}
				}
			}
		}
		for _, metric := range []struct {
			name, help string
			value      *int
		}{
			{"modurelay_cluster_db_connections_active", "Sum of active primary database pool connections across live instances.", snapshot.DBConnActive},
			{"modurelay_cluster_db_connections_idle", "Sum of idle primary database pool connections across live instances.", snapshot.DBConnIdle},
			{"modurelay_cluster_db_connections_limit", "Sum of configured primary database pool connection limits across live instances.", snapshot.DBMaxOpenConns},
			{"modurelay_cluster_redis_connections", "Sum of Redis connections across live instances and Redis nodes.", snapshot.RedisConnTotal},
			{"modurelay_cluster_redis_connections_idle", "Sum of idle Redis connections across live instances and Redis nodes.", snapshot.RedisConnIdle},
			{"modurelay_cluster_goroutines", "Sum of goroutines across live instances.", snapshot.GoroutineCount},
		} {
			if metric.value != nil {
				gauge(metric.name, metric.help, float64(*metric.value))
			}
		}
		if snapshot.CPUUsagePercent != nil {
			gauge("modurelay_cluster_cpu_usage_percent", "Mean CPU utilization percentage across live instances.", *snapshot.CPUUsagePercent)
		}
		if snapshot.MemoryUsedMB != nil {
			gauge("modurelay_cluster_memory_used_bytes", "Sum of memory usage across live instances.", float64(*snapshot.MemoryUsedMB)*1024*1024)
		}
		if snapshot.MemoryTotalMB != nil {
			gauge("modurelay_cluster_memory_limit_bytes", "Sum of reported memory capacity across live instances.", float64(*snapshot.MemoryTotalMB)*1024*1024)
		}
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8", []byte(out.String()))
	})
}
