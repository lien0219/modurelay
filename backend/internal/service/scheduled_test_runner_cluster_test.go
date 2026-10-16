package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type scheduledPlanScanStub struct {
	ScheduledTestPlanRepository
	scans int
}

func (r *scheduledPlanScanStub) ListDue(context.Context, time.Time) ([]*ScheduledTestPlan, error) {
	r.scans++
	return nil, nil
}

func TestScheduledTestsSkipPeerAndResumeAfterRelease(t *testing.T) {
	cache := &fakeLeaderLockCache{}
	ctx := context.Background()
	acquired, err := cache.TryAcquireLeaderLock(ctx, scheduledTestLeaderLockKey, "peer", time.Minute)
	require.NoError(t, err)
	require.True(t, acquired)
	repo := &scheduledPlanScanStub{}
	svc := NewScheduledTestRunnerService(repo, nil, nil, nil, nil)
	svc.SetLeaderLock(cache, nil)
	svc.runDue(ctx)
	require.Zero(t, repo.scans, "another pod must not scan or execute upstream probes")
	require.NoError(t, cache.ReleaseLeaderLock(ctx, scheduledTestLeaderLockKey, "peer"))
	svc.runDue(ctx)
	svc.runDue(ctx)
	require.Equal(t, 2, repo.scans, "each completed scan must release leadership")
}

func TestScheduledTestsSingleInstanceAndUnavailableCoordination(t *testing.T) {
	repo := &scheduledPlanScanStub{}
	svc := NewScheduledTestRunnerService(repo, nil, nil, nil, nil)
	svc.runDue(context.Background())
	require.Equal(t, 1, repo.scans)
	svc.SetLeaderLock(&fakeLeaderLockCache{acquireErr: context.DeadlineExceeded}, nil)
	svc.runDue(context.Background())
	require.Equal(t, 1, repo.scans, "coordination failure must not create duplicate probes")
}
