package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

const (
	opsRuntimeInterval = 15 * time.Second
	opsRuntimeTTL      = 45 * time.Second
	// Both keys deliberately share a Redis Cluster slot.
	opsRuntimeMembersKey = "ops:{runtime}:members"
	opsRuntimeSamplesKey = "ops:{runtime}:samples"
)

func (s *OpsService) SetMetricsCollector(collector *OpsMetricsCollector) {
	if s != nil {
		s.metricsCollector = collector
	}
}

func (c *OpsMetricsCollector) runtimeLoop() {
	defer close(c.runtimeDone)
	c.publishRuntimeSample()
	ticker := time.NewTicker(opsRuntimeInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.publishRuntimeSample()
		case <-c.stopCh:
			if c.redisClient != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				_, _ = opsRuntimeRemoveScript.Run(ctx, c.redisClient, []string{opsRuntimeMembersKey, opsRuntimeSamplesKey}, c.instanceID).Result()
				cancel()
			}
			return
		}
	}
}

func (c *OpsMetricsCollector) publishRuntimeSample() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if !c.isMonitoringEnabled(ctx) {
		return
	}
	sys, _ := c.collectSystemStats(ctx)
	active, idle := c.dbPoolStats()
	snapshot := &OpsSystemMetricsSnapshot{
		CreatedAt: time.Now().UTC(), Scope: "instance", InstanceCount: 1,
		CPUUsagePercent: sys.cpuUsagePercent, MemoryUsedMB: sys.memoryUsedMB,
		CPUResourceID: sys.cpuResourceID, MemoryResourceID: sys.memoryResourceID,
		MemoryTotalMB: sys.memoryTotalMB, MemoryUsagePercent: sys.memoryUsagePercent,
		DBConnActive: intPtr(active), DBConnIdle: intPtr(idle), GoroutineCount: intPtr(runtime.NumGoroutine()),
		DBOK: boolPtr(c.checkDB(ctx)), RedisOK: boolPtr(c.checkRedis(ctx)),
	}
	if c.db != nil {
		snapshot.DBMaxOpenConns = intPtr(c.db.Stats().MaxOpenConnections)
	}
	if total, idle, ok := c.redisPoolStats(); ok {
		snapshot.RedisConnTotal, snapshot.RedisConnIdle = intPtr(total), intPtr(idle)
	}
	snapshot.RedisPoolSize = c.redisPoolCapacity(ctx)
	c.runtimeSnapshotMu.Lock()
	c.runtimeSnapshot = snapshot
	c.runtimeSnapshotMu.Unlock()
	if c.redisClient != nil {
		_ = c.publishRuntimeSnapshot(ctx, snapshot)
	}
}

func (c *OpsMetricsCollector) publishRuntimeSnapshot(ctx context.Context, snapshot *OpsSystemMetricsSnapshot) error {
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	return opsRuntimePublishScript.Run(ctx, c.redisClient, []string{opsRuntimeMembersKey, opsRuntimeSamplesKey}, c.instanceID, payload, int(opsRuntimeTTL.Seconds())).Err()
}

// ClusterRuntimeSnapshot returns only fresh samples. A configured Redis outage
// is an error: silently returning this pod would label a partial total as global.
func (c *OpsMetricsCollector) ClusterRuntimeSnapshot(ctx context.Context) (*OpsSystemMetricsSnapshot, error) {
	if c == nil {
		return nil, fmt.Errorf("metrics collector unavailable")
	}
	if c.redisClient == nil {
		c.runtimeSnapshotMu.RLock()
		defer c.runtimeSnapshotMu.RUnlock()
		if c.runtimeSnapshot == nil || time.Since(c.runtimeSnapshot.CreatedAt) >= opsRuntimeTTL {
			return nil, fmt.Errorf("runtime sample unavailable")
		}
		out := *c.runtimeSnapshot
		out.CPUResourceID, out.MemoryResourceID = "", ""
		return &out, nil
	}
	values, err := opsRuntimeReadScript.Run(ctx, c.redisClient, []string{opsRuntimeMembersKey, opsRuntimeSamplesKey}, int(opsRuntimeTTL.Seconds())).StringSlice()
	if err != nil {
		return nil, err
	}
	samples := make([]*OpsSystemMetricsSnapshot, 0, len(values))
	for _, value := range values {
		var sample OpsSystemMetricsSnapshot
		if err := json.Unmarshal([]byte(value), &sample); err != nil {
			return nil, fmt.Errorf("decode runtime sample: %w", err)
		}
		samples = append(samples, &sample)
	}
	if len(samples) == 0 {
		return nil, fmt.Errorf("no active runtime samples")
	}
	return aggregateOpsRuntimeSamples(samples), nil
}

func aggregateOpsRuntimeSamples(samples []*OpsSystemMetricsSnapshot) *OpsSystemMetricsSnapshot {
	out := &OpsSystemMetricsSnapshot{Scope: "cluster", InstanceCount: len(samples)}
	var cpuSum float64
	var cpuCount int
	cpuResources, memoryResources := map[string]bool{}, map[string]bool{}
	for i, sample := range samples {
		if sample.CreatedAt.After(out.CreatedAt) {
			out.CreatedAt = sample.CreatedAt
		}
		cpuID, memoryID := sample.CPUResourceID, sample.MemoryResourceID
		if cpuID == "" {
			cpuID = fmt.Sprint(i)
		}
		if memoryID == "" {
			memoryID = fmt.Sprint(i)
		}
		if sample.CPUUsagePercent != nil && !cpuResources[cpuID] {
			cpuSum += *sample.CPUUsagePercent
			cpuCount++
			cpuResources[cpuID] = true
		}
		// Only sum self-consistent memory pairs; never combine a pod's usage with
		// another pod's capacity. CPU is the mean utilization across active pods.
		if sample.MemoryUsedMB != nil && sample.MemoryTotalMB != nil && !memoryResources[memoryID] {
			addOpsInt64(&out.MemoryUsedMB, sample.MemoryUsedMB)
			addOpsInt64(&out.MemoryTotalMB, sample.MemoryTotalMB)
			memoryResources[memoryID] = true
		}
		addOpsInt(&out.DBConnActive, sample.DBConnActive)
		addOpsInt(&out.DBConnIdle, sample.DBConnIdle)
		addOpsInt(&out.DBMaxOpenConns, sample.DBMaxOpenConns)
		addOpsInt(&out.RedisConnTotal, sample.RedisConnTotal)
		addOpsInt(&out.RedisConnIdle, sample.RedisConnIdle)
		addOpsInt(&out.RedisPoolSize, sample.RedisPoolSize)
		addOpsInt(&out.GoroutineCount, sample.GoroutineCount)
		andOpsBool(&out.DBOK, sample.DBOK)
		andOpsBool(&out.RedisOK, sample.RedisOK)
	}
	if cpuCount > 0 {
		out.CPUUsagePercent = float64Ptr(roundTo1DP(cpuSum / float64(cpuCount)))
	}
	if out.MemoryTotalMB != nil && *out.MemoryTotalMB > 0 {
		out.MemoryUsagePercent = float64Ptr(roundTo1DP(100 * float64(*out.MemoryUsedMB) / float64(*out.MemoryTotalMB)))
	}
	return out
}

func opsHostResourceID() string {
	// Linux boot_id is shared by containers on a node, unlike their hostnames.
	if bootID, err := os.ReadFile("/proc/sys/kernel/random/boot_id"); err == nil {
		return "host:" + strings.TrimSpace(string(bootID))
	}
	host, _ := os.Hostname()
	return "host:" + host
}

func addOpsInt(dst **int, src *int) {
	if src != nil {
		if *dst == nil {
			*dst = intPtr(0)
		}
		**dst += *src
	}
}
func addOpsInt64(dst **int64, src *int64) {
	if src != nil {
		if *dst == nil {
			*dst = new(int64)
		}
		**dst += *src
	}
}
func andOpsBool(dst **bool, src *bool) {
	if src != nil {
		if *dst == nil {
			*dst = boolPtr(true)
		}
		**dst = **dst && *src
	}
}

// GetClusterMetrics uses the same shared usage log window as the admin QPS/TPS
// display. These are already global and must never be summed per pod.
func (s *OpsService) GetClusterMetrics(ctx context.Context) (*OpsWindowStats, *OpsSystemMetricsSnapshot, error) {
	if s == nil {
		return nil, nil, fmt.Errorf("ops service unavailable")
	}
	end := time.Now().UTC().Truncate(5 * time.Second)
	stats, err := s.GetWindowStats(ctx, end.Add(-time.Minute), end)
	if err != nil {
		return nil, nil, err
	}
	if s.metricsCollector == nil {
		return nil, nil, fmt.Errorf("runtime collector unavailable")
	}
	runtime, err := s.metricsCollector.ClusterRuntimeSnapshot(ctx)
	if err != nil {
		return nil, nil, err
	}
	return stats, runtime, nil
}
