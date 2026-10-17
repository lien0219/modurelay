package service

import (
	"context"
	"database/sql"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/require"
)

// Stop must wait even if an in-progress repository call has not returned yet.
type shutdownMonitoringSettings struct {
	SettingRepository
	entered       chan struct{}
	release       <-chan struct{}
	collectorOnly bool
}

func (s *shutdownMonitoringSettings) GetValue(ctx context.Context, key string) (string, error) {
	if key != SettingKeyOpsMonitoringEnabled {
		return "", nil
	}
	deadline, _ := ctx.Deadline()
	if !s.collectorOnly || time.Until(deadline) > 7*time.Second {
		s.entered <- struct{}{}
		<-s.release
	}
	return "false", nil
}

type shutdownUnusedOpsRepository struct{ OpsRepository }

func TestOpsMetricsStopWaitsForDBCollectorAfterRuntimeLoopStops(t *testing.T) {
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	settings := &shutdownMonitoringSettings{entered: make(chan struct{}, 1), release: release, collectorOnly: true}
	collector := &OpsMetricsCollector{
		opsRepo: &shutdownUnusedOpsRepository{}, settingRepo: settings, db: &sql.DB{},
	}
	collector.Start()
	t.Cleanup(func() { unblock(); collector.Stop() })
	awaitOpsShutdownSignal(t, settings.entered)
	done := make(chan struct{})
	go func() { collector.Stop(); close(done) }()
	awaitOpsShutdownSignal(t, collector.runtimeDone)
	assertOpsStopPending(t, done, 40*time.Millisecond)
	unblock()
	awaitOpsShutdownSignal(t, done)
	collector.Stop()
}

func TestOpsAggregationStopWaitsForBothCurrentJobs(t *testing.T) {
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	settings := &shutdownMonitoringSettings{entered: make(chan struct{}, 2), release: release}
	aggregation := &OpsAggregationService{opsRepo: &shutdownUnusedOpsRepository{}, settingRepo: settings}
	aggregation.Start()
	t.Cleanup(func() { unblock(); aggregation.Stop() })
	awaitOpsShutdownSignal(t, settings.entered)
	awaitOpsShutdownSignal(t, settings.entered)
	first, second := make(chan struct{}), make(chan struct{})
	go func() { aggregation.Stop(); close(first) }()
	go func() { aggregation.Stop(); close(second) }()
	assertOpsStopPending(t, first, 40*time.Millisecond)
	assertOpsStopPending(t, second, 40*time.Millisecond)
	unblock()
	awaitOpsShutdownSignal(t, first)
	awaitOpsShutdownSignal(t, second)
}

func TestOpsProducersStopBeforeStartAndConcurrentStartStop(t *testing.T) {
	for _, kind := range []string{"metrics", "aggregation"} {
		t.Run(kind, func(t *testing.T) {
			for i := 0; i < 50; i++ {
				var start, stop func()
				if kind == "metrics" {
					collector := &OpsMetricsCollector{cfg: &config.Config{}}
					start, stop = collector.Start, collector.Stop
				} else {
					aggregation := &OpsAggregationService{}
					start, stop = aggregation.Start, aggregation.Stop
				}
				if i == 0 {
					stop()
					start()
					stop()
					continue
				}
				var calls sync.WaitGroup
				calls.Add(3)
				go func() { defer calls.Done(); start() }()
				go func() { defer calls.Done(); stop() }()
				go func() { defer calls.Done(); stop() }()
				done := make(chan struct{})
				go func() { calls.Wait(); close(done) }()
				awaitOpsShutdownSignal(t, done)
			}
		})
	}
}

type shutdownSingleRunSchedule struct{ used atomic.Bool }

func (s *shutdownSingleRunSchedule) Next(now time.Time) time.Time {
	if s.used.Swap(true) {
		return now.Add(time.Hour)
	}
	return now.Add(10 * time.Millisecond)
}

func TestOpsCleanupStopWaitsBeyondReloadTimeoutWithoutHoldingConfigLock(t *testing.T) {
	for _, retired := range []bool{false, true} {
		name := "current cron"
		if retired {
			name = "cron retired by reload"
		}
		t.Run(name, func(t *testing.T) {
			svc := &OpsCleanupService{started: true}
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			c := cron.New()
			c.Schedule(&shutdownSingleRunSchedule{}, cron.FuncJob(func() {
				close(entered)
				<-release
				// Real cleanup jobs read effective configuration under this lock.
				_ = svc.snapshotEffective()
			}))
			svc.cron = c
			c.Start()
			t.Cleanup(func() { unblock(); svc.Stop() })
			awaitOpsShutdownSignal(t, entered)
			if retired {
				svc.mu.Lock()
				svc.stopCronLocked() // same path used when Reload replaces a cron
				svc.mu.Unlock()
				require.Nil(t, svc.cron)
			}
			first, second := make(chan struct{}), make(chan struct{})
			go func() { svc.Stop(); close(first) }()
			go func() { svc.Stop(); close(second) }()
			wait := 40 * time.Millisecond
			if !retired {
				wait += opsCleanupCronStopTimeout
			}
			assertOpsStopPending(t, first, wait)
			assertOpsStopPending(t, second, 40*time.Millisecond)
			unblock()
			awaitOpsShutdownSignal(t, first)
			awaitOpsShutdownSignal(t, second)
		})
	}
}

func TestOpsCleanupStopWithoutStart(t *testing.T) {
	svc := &OpsCleanupService{}
	svc.Stop()
	svc.Start()
	svc.Stop()
	require.True(t, svc.stopped)
	require.Nil(t, svc.cron)
}

func awaitOpsShutdownSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatal("background worker did not reach the expected lifecycle boundary")
	}
}

func assertOpsStopPending(t *testing.T, done <-chan struct{}, duration time.Duration) {
	t.Helper()
	select {
	case <-done:
		t.Fatal("Stop returned while a background job was still using dependencies")
	case <-time.After(duration):
	}
}
