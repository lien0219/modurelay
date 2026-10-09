package handler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestTenantUsageIsPersistedBeforeReturningToHandler(t *testing.T) {
	for _, protocol := range []string{"gateway", "openai"} {
		t.Run(protocol, func(t *testing.T) {
			pool := service.NewUsageRecordWorkerPoolWithOptions(service.UsageRecordWorkerPoolOptions{WorkerCount: 1, QueueSize: 2, TaskTimeout: time.Minute, OverflowPolicy: config.UsageRecordOverflowPolicySync})
			t.Cleanup(pool.Stop)
			started, unblock := make(chan struct{}), make(chan struct{})
			t.Cleanup(func() { close(unblock) })
			pool.Submit(func(context.Context) { close(started); <-unblock })
			<-started
			// A non-nil handle marks this as money-critical work. The repository is
			// not exercised here; the handler must choose its synchronous fallback
			// before a volatile worker queue can retain the task.
			ctx := service.WithBudgetReservation(context.Background(), service.NewBudgetReservationHandle(service.NewBudgetService(nil), "frozen-reservation"))
			var persisted atomic.Bool
			task := func(context.Context) { persisted.Store(true) }
			if protocol == "gateway" {
				(&GatewayHandler{usageRecordWorkerPool: pool}).submitUsageRecordTask(ctx, task)
			} else {
				(&OpenAIGatewayHandler{usageRecordWorkerPool: pool}).submitUsageRecordTask(ctx, task)
			}
			require.True(t, persisted.Load(), "accepted tenant billing must reach SQL before being left behind a volatile worker queue")
		})
	}
}
