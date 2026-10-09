package service

import (
	"context"
	"sync"
	"time"
)

type SCIMTokenExpiryRepository interface {
	NotifyExpiringTokens(context.Context, int) error
}

// Expiry monitoring is independent of client traffic. Each scan is bounded and
// repository transactions use the same workspace lock as token mutations.
type SCIMTokenExpiryMonitor struct {
	repo             SCIMRepository
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	lifecycle        sync.Mutex
	started, stopped bool
}

func NewSCIMTokenExpiryMonitor(repo SCIMRepository) *SCIMTokenExpiryMonitor {
	return &SCIMTokenExpiryMonitor{repo: repo}
}
func ProvideSCIMTokenExpiryMonitor(repo SCIMRepository) *SCIMTokenExpiryMonitor {
	m := NewSCIMTokenExpiryMonitor(repo)
	m.Start()
	return m
}
func (m *SCIMTokenExpiryMonitor) runBatch(ctx context.Context) error {
	if r, ok := m.repo.(SCIMTokenExpiryRepository); ok {
		return r.NotifyExpiringTokens(ctx, 100)
	}
	return nil
}
func (m *SCIMTokenExpiryMonitor) Start() {
	if m == nil || m.repo == nil {
		return
	}
	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	if m.started || m.stopped {
		return
	}
	m.started = true
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			batchCtx, batchCancel := context.WithTimeout(ctx, 20*time.Second)
			_ = m.runBatch(batchCtx)
			batchCancel()
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
func (m *SCIMTokenExpiryMonitor) Stop() {
	if m == nil {
		return
	}
	m.lifecycle.Lock()
	m.stopped = true
	if m.cancel != nil {
		m.cancel()
	}
	m.lifecycle.Unlock()
	m.wg.Wait()
}
