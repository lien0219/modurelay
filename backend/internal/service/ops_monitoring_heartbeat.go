package service

import "context"

// MonitoringJobHeartbeats exposes only timestamps used by the independent
// Prometheus watchdog. Error text, database details and secrets stay private.
func (s *OpsService) MonitoringJobHeartbeats(ctx context.Context) (map[string]float64, error) {
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return nil, err
	}
	heartbeats, err := s.opsRepo.ListJobHeartbeats(ctx)
	if err != nil {
		return nil, err
	}
	result := map[string]float64{"ops_metrics_collector": 0, "ops_alert_evaluator": 0}
	for _, heartbeat := range heartbeats {
		if heartbeat == nil {
			continue
		}
		if _, allowed := result[heartbeat.JobName]; allowed && heartbeat.LastSuccessAt != nil {
			result[heartbeat.JobName] = float64(heartbeat.LastSuccessAt.Unix())
		}
	}
	return result, nil
}
