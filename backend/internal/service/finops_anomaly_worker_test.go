package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type anomalyWorkerProbe struct {
	FinOpsAnomalyRepository
	calls   atomic.Int32
	entered chan struct{}
	once    sync.Once
}

func (r *anomalyWorkerProbe) RunFinOpsAnomalyScan(ctx context.Context, _ time.Time, _ FinOpsAnomalyConfig) (FinOpsAnomalyDetectorStatus, error) {
	r.calls.Add(1)
	r.once.Do(func() { close(r.entered) })
	<-ctx.Done()
	return FinOpsAnomalyDetectorStatus{}, ctx.Err()
}

func TestFinOpsAnomalyWorkerStartIsSingleAndDrainWaits(t *testing.T) {
	r := &anomalyWorkerProbe{entered: make(chan struct{})}
	w := NewFinOpsAnomalyWorker(r, DefaultFinOpsAnomalyConfig(), time.Hour)
	w.Start()
	w.Start()
	select {
	case <-r.entered:
	case <-time.After(time.Second):
		t.Fatal("worker did not enter bounded scan")
	}
	require.Never(t, func() bool { return r.calls.Load() > 1 }, 30*time.Millisecond, time.Millisecond)
	w.Stop()
	w.Stop()
	require.Equal(t, int32(1), r.calls.Load())
}

func TestFinOpsAnomalyWorkerStopBeforeStartAndConcurrentStartStop(t *testing.T) {
	for i := 0; i < 20; i++ {
		r := &anomalyWorkerProbe{entered: make(chan struct{})}
		w := NewFinOpsAnomalyWorker(r, DefaultFinOpsAnomalyConfig(), time.Hour)
		w.Stop()
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); w.Start() }()
		go func() { defer wg.Done(); w.Stop() }()
		wg.Wait()
		require.Zero(t, r.calls.Load())
	}
}
