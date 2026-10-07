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
	repo   SCIMRepository
	cancel context.CancelFunc
	wg     sync.WaitGroup
	start  sync.Once
	stop   sync.Once
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
	m.start.Do(func() {
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
	})
}
func (m *SCIMTokenExpiryMonitor) Stop() {
	m.stop.Do(func() {
		if m.cancel != nil {
			m.cancel()
		}
		m.wg.Wait()
	})
}
