package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCleanupStagesWaitForProducersBeforeConsumers(t *testing.T) {
	var mu sync.Mutex
	var events []string
	record := func(name string) cleanupStep {
		return cleanupStep{name, func() error { mu.Lock(); defer mu.Unlock(); events = append(events, name); return nil }}
	}
	err := runCleanupStages(context.Background(),
		cleanupStage{"producers", true, []cleanupStep{record("producer")}},
		cleanupStage{"usage", false, []cleanupStep{record("usage"), record("errors")}},
		cleanupStage{"sinks", false, []cleanupStep{record("cache"), record("logs"), record("database")}},
	)
	require.NoError(t, err)
	require.Equal(t, []string{"producer", "usage", "errors", "cache", "logs", "database"}, events)
}

func TestCleanupTimeoutDoesNotCloseDependenciesUnderRunningWorker(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	entered := make(chan struct{})
	closed := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- runCleanupStages(ctx,
			cleanupStage{"workers", true, []cleanupStep{{"blocked", func() error { close(entered); <-release; return nil }}}},
			cleanupStage{"infra", false, []cleanupStep{{"db", func() error { closed <- struct{}{}; return nil }}}},
		)
	}()
	<-entered
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("cleanup ignored deadline")
	}
	select {
	case <-closed:
		t.Fatal("closed database while a worker was running")
	default:
	}
}

func TestCleanupFailureIsReportedAndOtherStepsStillRun(t *testing.T) {
	failure := errors.New("flush failed")
	ran := false
	err := runCleanupStages(context.Background(), cleanupStage{"flush", true, []cleanupStep{
		{"failed", func() error { return failure }}, {"next", func() error { ran = true; return nil }},
	}})
	require.ErrorIs(t, err, failure)
	require.True(t, ran)
}

func TestCleanupFailureDoesNotAdvanceToDependencies(t *testing.T) {
	failure := errors.New("queue still running")
	closed := false
	err := runCleanupStages(context.Background(),
		cleanupStage{"queue", false, []cleanupStep{{"queue", func() error { return failure }}}},
		cleanupStage{"database", false, []cleanupStep{{"db", func() error { closed = true; return nil }}}},
	)
	require.ErrorIs(t, err, failure)
	require.False(t, closed)
}
