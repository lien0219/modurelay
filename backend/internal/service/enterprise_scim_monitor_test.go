package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type scimExpiryProbe struct {
	SCIMRepository
	batch int
	calls int
}

func (r *scimExpiryProbe) NotifyExpiringTokens(_ context.Context, batch int) error {
	r.batch = batch
	r.calls++
	return nil
}
func TestSCIMExpiryMonitorUsesBoundedBatchAndStops(t *testing.T) {
	r := &scimExpiryProbe{}
	m := NewSCIMTokenExpiryMonitor(r)
	require.NoError(t, m.runBatch(context.Background()))
	require.Equal(t, 100, r.batch)
	require.Equal(t, 1, r.calls)
	m.Start()
	m.Stop()
	m.Stop()
}

type scimStoppedProbe struct {
	SCIMRepository
	calls atomic.Int32
}

func (r *scimStoppedProbe) NotifyExpiringTokens(context.Context, int) error {
	r.calls.Add(1)
	return nil
}

func TestSCIMExpiryMonitorStopBeforeStartPreventsLaunch(t *testing.T) {
	r := &scimStoppedProbe{}
	m := NewSCIMTokenExpiryMonitor(r)
	m.Stop()
	m.Start()
	require.Never(t, func() bool { return r.calls.Load() > 0 }, 30*time.Millisecond, time.Millisecond)
}

func TestSCIMExpiryMonitorConcurrentStartStop(t *testing.T) {
	for i := 0; i < 20; i++ {
		r := &scimStoppedProbe{}
		m := NewSCIMTokenExpiryMonitor(r)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); m.Start() }()
		go func() { defer wg.Done(); m.Stop() }()
		wg.Wait()
		m.Stop()
	}
}
