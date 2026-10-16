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
