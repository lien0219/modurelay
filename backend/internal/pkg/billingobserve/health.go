// Package billingobserve retains observation gaps until PostgreSQL can persist
// them. Export its process counters independently: a hard crash before the next
// successful flush can otherwise lose an outage interval.
package billingobserve

import (
	"sync"
	"sync/atomic"
	"time"
)

type Gap struct {
	InstanceID    string    `json:"instance_id"`
	MissedCount   int64     `json:"missed_count"`
	FirstMissedAt time.Time `json:"first_missed_at"`
	LastMissedAt  time.Time `json:"last_missed_at"`
	persisted     int64
}

var mu sync.Mutex
var gaps = map[string]Gap{}
var missed atomic.Int64
var lastFailure atomic.Int64

func Failed(instanceID string) {
	now := time.Now().UTC()
	mu.Lock()
	gap := gaps[instanceID]
	gap.InstanceID = instanceID
	if gap.MissedCount == 0 {
		gap.FirstMissedAt = now
	}
	gap.MissedCount++
	gap.LastMissedAt = now
	gaps[instanceID] = gap
	mu.Unlock()
	missed.Add(1)
	lastFailure.Store(now.Unix())
}
func Snapshot(instanceID string) Gap { mu.Lock(); defer mu.Unlock(); return gaps[instanceID] }
func PendingGaps() []Gap {
	mu.Lock()
	defer mu.Unlock()
	result := []Gap{}
	for _, gap := range gaps {
		if gap.MissedCount > gap.persisted {
			result = append(result, gap)
		}
	}
	return result
}
func Persisted(instanceID string, count int64) {
	mu.Lock()
	defer mu.Unlock()
	gap, exists := gaps[instanceID]
	if !exists {
		return
	}
	if count > gap.MissedCount {
		count = gap.MissedCount
	}
	if count > gap.persisted {
		gap.persisted = count
		gaps[instanceID] = gap
	}
}
func Pending() int64 {
	mu.Lock()
	defer mu.Unlock()
	var total int64
	for _, gap := range gaps {
		total += gap.MissedCount - gap.persisted
	}
	return total
}
func Counters() map[string]float64 {
	return map[string]float64{
		"modurelay_billing_observation_missed_total":                   float64(missed.Load()),
		"modurelay_billing_observation_unpersisted":                    float64(Pending()),
		"modurelay_billing_observation_last_failure_timestamp_seconds": float64(lastFailure.Load()),
	}
}
