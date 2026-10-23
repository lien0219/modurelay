package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/testutil"
	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
)

type billingReconciliationRepoStub struct {
	opsRepoMock
	snapshot *OpsBillingReconciliation
	err      error
}

func (r *billingReconciliationRepoStub) ReconcileBillingUsage(context.Context, time.Time) error {
	return r.err
}
func (r *billingReconciliationRepoStub) GetBillingReconciliation(context.Context) (*OpsBillingReconciliation, error) {
	return r.snapshot, r.err
}
func TestOpsBillingReconciliationUnknownCannotReportZero(t *testing.T) {
	now := time.Now()
	old := now.Add(-4 * time.Minute)
	zero := int64(0)
	for _, tc := range []struct {
		name      string
		at        *time.Time
		truncated bool
		err       error
		want      string
	}{
		{"fresh", &now, false, nil, "Available"}, {"stale", &old, false, nil, "Unknown"},
		{"never-run", nil, false, nil, "Unknown"}, {"incomplete", &now, true, nil, "Unknown"},
		{"database-error", &now, false, errors.New("db unavailable"), "Unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &OpsService{opsRepo: &billingReconciliationRepoStub{snapshot: &OpsBillingReconciliation{CheckedAt: tc.at, Truncated: tc.truncated, MissingUsageLogs: &zero, AmountMismatches: &zero}, err: tc.err}}
			result, err := svc.GetBillingReconciliation(context.Background())
			require.NoError(t, err)
			require.Equal(t, tc.want, result.State)
			if tc.want == "Unknown" {
				require.Nil(t, result.MissingUsageLogs)
				require.Nil(t, result.AmountMismatches)
			}
		})
	}
}

func TestOpsBillingReconciliationOperationalCoverageRequiresFreshCompleteScan(t *testing.T) {
	now := time.Now()
	old := now.Add(-4 * time.Minute)
	zero := int64(0)
	for _, tc := range []struct {
		name, state string
		checked     *time.Time
		available   bool
	}{
		{"complete", "Available", &now, true}, {"incomplete", "Unknown", &now, false}, {"stale", "Available", &old, false}, {"never", "Available", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &OpsService{opsRepo: &billingReconciliationRepoStub{snapshot: &OpsBillingReconciliation{CheckedAt: &now, OperationalState: tc.state, OperationalCheckedAt: tc.checked, FailedSettlements: &zero, CompensationBacklog: &zero, LedgerMismatches: &zero}}}
			result, err := svc.GetBillingReconciliation(context.Background())
			require.NoError(t, err)
			if tc.available {
				require.Equal(t, "Available", result.OperationalState)
				require.NotNil(t, result.LedgerMismatches)
			} else {
				require.Equal(t, "Unknown", result.OperationalState)
				require.Nil(t, result.FailedSettlements)
				require.Nil(t, result.CompensationBacklog)
				require.Nil(t, result.LedgerMismatches)
			}
		})
	}
}

func TestOpsBillingReconciliationRemotePodObservationGapIsUnknown(t *testing.T) {
	mr := miniredis.RunT(t)
	client := testutil.NewRedisClient(t, mr.Addr())
	a := &OpsMetricsCollector{redisClient: client, instanceID: "pod-a"}
	b := &OpsMetricsCollector{redisClient: client, instanceID: "pod-b"}
	now := time.Now()
	zero := int64(0)
	require.NoError(t, a.publishRuntimeSnapshot(context.Background(), &OpsSystemMetricsSnapshot{CreatedAt: now}))
	require.NoError(t, b.publishRuntimeSnapshot(context.Background(), &OpsSystemMetricsSnapshot{CreatedAt: now, BillingObservationUnpersisted: 4}))
	repo := &billingReconciliationRepoStub{snapshot: &OpsBillingReconciliation{CheckedAt: &now, OperationalCheckedAt: &now, OperationalState: "Available", LedgerMismatches: &zero, FailedSettlements: &zero, CompensationBacklog: &zero}}
	svc := &OpsService{opsRepo: repo, metricsCollector: a}
	result, err := svc.GetBillingReconciliation(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 4, result.ObservationUnpersisted)
	require.Equal(t, "Unknown", result.OperationalState)
	require.Nil(t, result.LedgerMismatches)
}
