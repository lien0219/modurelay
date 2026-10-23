package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"time"
)

func isMonitoringAlertMetric(metric string) bool {
	if metric == "monitoring_source_unavailable" {
		return true
	}
	for _, candidate := range opsMonitoringMetrics {
		if candidate.ID == metric && candidate.ID != "pod_info" {
			return true
		}
	}
	return false
}

// OpsMonitoringAlertMetricTypes shares the read-only provider's catalog.
func OpsMonitoringAlertMetricTypes() []string {
	out := []string{"monitoring_source_unavailable"}
	for _, candidate := range opsMonitoringMetrics {
		if candidate.ID != "pod_info" {
			out = append(out, candidate.ID)
		}
	}
	return out
}

func OpsMonitoringAlertMetricIsPercentage(metric string) bool {
	for _, candidate := range opsMonitoringMetrics {
		if candidate.ID == metric {
			return candidate.Unit == "percent"
		}
	}
	return false
}

func isRequestAlertMetric(metric string) bool {
	return strings.HasPrefix(metric, "http_") || strings.HasPrefix(metric, "ttft_") || metric == "qps" || metric == "rpm" || metric == "request_count" || metric == "qps_started" || metric == "rpm_started" || metric == "request_count_started"
}
func alertFilterNumber(filters map[string]any, key string) float64 {
	switch v := filters[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	}
	return 0
}
func alertCriticalThreshold(filters map[string]any) (float64, bool) {
	_, present := filters["critical_threshold"]
	return alertFilterNumber(filters, "critical_threshold"), present
}

func monitoringAlertSampleStatus(rule *OpsAlertRule, sample OpsMonitoringAlertSample) alertMetricStatus {
	if sample.Value == nil || math.IsNaN(*sample.Value) || math.IsInf(*sample.Value, 0) {
		return alertMetricUnavailable
	}
	if isRequestAlertMetric(rule.MetricType) && sample.RequestCount < int64(rule.MinRequests) {
		return alertMetricInsufficient
	}
	if (strings.Contains(rule.MetricType, "p95") || strings.Contains(rule.MetricType, "p99") || strings.HasSuffix(rule.MetricType, "_rate")) && sample.RequestCount == 0 {
		return alertMetricInsufficient
	}
	if min := alertFilterNumber(rule.Filters, "min_errors"); min > 0 {
		if sample.ErrorCount == nil {
			return alertMetricUnavailable
		}
	}
	return alertMetricOK
}

func (s *OpsAlertEvaluatorService) evaluateMonitoringAlertRule(ctx context.Context, cfg *OpsAlertRuntimeSettings, rule *OpsAlertRule, start, end, now time.Time, interval time.Duration) bool {
	repo, ok := s.opsRepo.(OpsAlertDeliveryRepository)
	if !ok || s.opsService == nil {
		s.resetRuleState(ctx, rule.ID, now)
		return false
	}
	if !s.prepareRuleEvaluationState(ctx, rule) {
		return false
	}
	// Rules created before dimension validation was tightened must never fall
	// back to a broader query. Keep any active event and expose invalid_rule.
	if err := ValidateOpsAlertFilters(rule.MetricType, rule.Filters); err != nil {
		_, _ = repo.MarkMissingAlertResources(ctx, rule.ID, alertRuleRevision(rule.EvaluationRevision), nil, now)
		s.noteRuleSampleStatus(ctx, rule.ID, now, OpsAlertDataStatusInvalidRule)
		return false
	}
	query := s.opsService.EvaluateMonitoringAlertMetric
	if s.monitoringAlertSamples != nil {
		query = s.monitoringAlertSamples
	}
	samples, err := query(ctx, rule.MetricType, rule.Filters, start, end)
	if err != nil || len(samples) == 0 {
		_, _ = repo.MarkMissingAlertResources(ctx, rule.ID, alertRuleRevision(rule.EvaluationRevision), nil, now)
		s.resetRuleState(ctx, rule.ID, now)
		return false
	}
	overall := OpsAlertDataStatusOK
	seen := make([]string, 0, len(samples))
	evaluated := false
	for _, sample := range samples {
		// Missing identity must not merge unrelated resources into a global alarm.
		if sample.ResourceKey == "" {
			overall = OpsAlertDataStatusUnavailable
			continue
		}
		seen = append(seen, sample.ResourceKey)
		state, err := repo.GetAlertResourceState(ctx, rule.ID, sample.ResourceKey)
		if err != nil {
			overall = OpsAlertDataStatusUnavailable
			continue
		}
		if state == nil || alertRuleRevision(state.RuleRevision) != alertRuleRevision(rule.EvaluationRevision) {
			state = &OpsAlertRuleState{RuleID: rule.ID, RuleRevision: alertRuleRevision(rule.EvaluationRevision)}
		}
		previousAt := state.LastEvaluatedAt
		if !previousAt.IsZero() && interval > 0 && now.Sub(previousAt) < interval {
			continue
		}
		state.LastEvaluatedAt = now
		status := monitoringAlertSampleStatus(rule, sample)
		if status != alertMetricOK {
			state.ConsecutiveBreaches = 0
			if status == alertMetricInsufficient {
				state.DataStatus = OpsAlertDataStatusInsufficient
				if overall == OpsAlertDataStatusOK {
					overall = state.DataStatus
				}
			} else {
				state.DataStatus = OpsAlertDataStatusUnavailable
				overall = state.DataStatus
				state.ConsecutiveBreaches = 0
			}
			_ = repo.UpsertAlertResourceState(ctx, sample.ResourceKey, state)
			continue
		}
		state.DataStatus = OpsAlertDataStatusOK
		if !previousAt.IsZero() && interval > 0 && now.Sub(previousAt) >= 2*interval {
			state.ConsecutiveBreaches = 0
		}
		breached := compareMetric(*sample.Value, rule.Operator, rule.Threshold)
		if min := alertFilterNumber(rule.Filters, "min_errors"); min > 0 && sample.ErrorCount != nil && *sample.ErrorCount < min {
			breached = false
		}
		if breached {
			state.ConsecutiveBreaches++
		} else {
			state.ConsecutiveBreaches = 0
		}
		if err := repo.UpsertAlertResourceState(ctx, sample.ResourceKey, state); err != nil {
			overall = OpsAlertDataStatusUnavailable
			continue
		}
		evaluated = true
		active, err := repo.GetResourceAlertEvent(ctx, rule.ID, sample.ResourceKey, true)
		if err != nil {
			continue
		}
		if active != nil && alertRuleRevision(active.RuleRevision) != alertRuleRevision(rule.EvaluationRevision) {
			continue // The rule changed while this evaluator was querying metrics.
		}
		if !breached {
			if active != nil {
				s.resolveAlertWithDeliveries(ctx, cfg, rule, active, now, *sample.Value)
			}
			continue
		}
		if active != nil {
			s.ensureFiringNotification(ctx, cfg, rule, active, now)
			s.upgradeAlertEvent(ctx, cfg, rule, active, *sample.Value)
			continue
		}
		if state.ConsecutiveBreaches < requiredSustainedBreaches(rule.SustainedMinutes, interval) {
			continue
		}
		latest, err := repo.GetResourceAlertEvent(ctx, rule.ID, sample.ResourceKey, false)
		if err != nil {
			continue
		}
		if latest != nil && alertRuleRevision(latest.RuleRevision) == alertRuleRevision(rule.EvaluationRevision) && rule.CooldownMinutes > 0 && now.Sub(latest.FiredAt) < time.Duration(rule.CooldownMinutes)*time.Minute {
			continue
		}
		dims := map[string]any{}
		for k, v := range sample.Dimensions {
			dims[k] = v
		}
		dims["resource_key"] = sample.ResourceKey
		dims["rule_revision"] = alertRuleRevision(rule.EvaluationRevision)
		dims["window_minutes"] = rule.WindowMinutes
		fingerprint := sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%dm:%s", rule.ID, alertRuleRevision(rule.EvaluationRevision), rule.WindowMinutes, sample.ResourceKey)))
		dims["fingerprint"] = hex.EncodeToString(fingerprint[:])
		// Shared resource key enables incident grouping while all rule events remain intact.
		incident := sha256.Sum256([]byte(sample.ResourceKey))
		dims["incident_key"] = hex.EncodeToString(incident[:])
		ev := &OpsAlertEvent{RuleID: rule.ID, Severity: rule.Severity, Status: OpsAlertStatusFiring, Title: rule.Severity + ": " + rule.Name, Description: buildOpsAlertDescription(rule, *sample.Value, rule.WindowMinutes, "", nil), MetricValue: sample.Value, ThresholdValue: float64Ptr(rule.Threshold), Dimensions: dims, FiredAt: now, CreatedAt: now}
		ev.RuleRevision = alertRuleRevision(rule.EvaluationRevision)
		if critical, configured := alertCriticalThreshold(rule.Filters); configured && compareMetric(*sample.Value, rule.Operator, critical) {
			ev.Severity = "P0"
		}
		ev.Title = ev.Severity + ": " + rule.Name
		ev.Deliveries, err = s.prepareAlertDeliveriesAt(ctx, cfg, rule, ev, "firing", now)
		if err != nil {
			continue
		}
		_, _ = s.opsRepo.CreateAlertEvent(ctx, ev)
	}
	if missing, err := repo.MarkMissingAlertResources(ctx, rule.ID, alertRuleRevision(rule.EvaluationRevision), seen, now); err != nil || missing {
		overall = OpsAlertDataStatusUnavailable
	}
	s.noteRuleSampleStatus(ctx, rule.ID, now, overall)
	return evaluated
}

func (s *OpsAlertEvaluatorService) resolveAlertWithDeliveries(ctx context.Context, cfg *OpsAlertRuntimeSettings, rule *OpsAlertRule, event *OpsAlertEvent, now time.Time, value float64) bool {
	repo, ok := s.opsRepo.(OpsAlertDeliveryRepository)
	if !ok {
		return false
	}
	resolved := *event
	resolved.Status = OpsAlertStatusResolved
	resolved.ResolvedAt = &now
	resolved.MetricValue = &value
	deliveries, err := s.prepareAlertDeliveries(ctx, cfg, rule, &resolved, "resolved")
	if err != nil {
		return false
	}
	updated, err := repo.TransitionAlertEvent(ctx, &resolved, deliveries)
	return err == nil && updated
}
func (s *OpsAlertEvaluatorService) upgradeAlertEvent(ctx context.Context, cfg *OpsAlertRuntimeSettings, rule *OpsAlertRule, event *OpsAlertEvent, value float64) {
	critical, configured := alertCriticalThreshold(rule.Filters)
	if !configured || event.Severity == "P0" || !compareMetric(value, rule.Operator, critical) {
		return
	}
	repo, ok := s.opsRepo.(OpsAlertDeliveryRepository)
	if !ok {
		return
	}
	upgraded := *event
	upgraded.Severity = "P0"
	upgraded.Title = "P0: " + rule.Name
	upgraded.MetricValue = &value
	deliveries, err := s.prepareAlertDeliveries(ctx, cfg, rule, &upgraded, "upgrade")
	if err != nil {
		return
	}
	_, _ = repo.TransitionAlertEvent(ctx, &upgraded, deliveries)
}
