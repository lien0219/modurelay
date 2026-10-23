package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type monitoringHeartbeatRepo struct {
	opsRepoMock
	heartbeats []*OpsJobHeartbeat
}

func (r *monitoringHeartbeatRepo) ListJobHeartbeats(context.Context) ([]*OpsJobHeartbeat, error) {
	return r.heartbeats, nil
}
func TestOpsMonitoringHeartbeatWhitelist(t *testing.T) {
	now := time.Now()
	secret := "must never be exported"
	repo := &monitoringHeartbeatRepo{heartbeats: []*OpsJobHeartbeat{
		{JobName: "ops_alert_evaluator", LastSuccessAt: &now, LastError: &secret},
		{JobName: "other_job", LastSuccessAt: &now}, nil,
	}}
	s := &OpsService{opsRepo: repo}
	got, err := s.MonitoringJobHeartbeats(context.Background())
	require.NoError(t, err)
	require.Equal(t, map[string]float64{"ops_alert_evaluator": float64(now.Unix()), "ops_metrics_collector": 0}, got)
}
