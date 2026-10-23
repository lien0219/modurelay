package service

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"
)

// EvaluateMonitoringAlertMetric shares the dashboard's metric contract and
// resource identities. Unavailable samples remain nil and cannot resolve an
// existing incident. It performs no writes on the request/billing path.
func (s *OpsService) EvaluateMonitoringAlertMetric(ctx context.Context, metric string, filters map[string]any, start, end time.Time) ([]OpsMonitoringAlertSample, error) {
	if err := ValidateOpsAlertFilters(metric, filters); err != nil {
		return nil, err
	}
	if metric == "monitoring_source_unavailable" {
		return s.evaluateMonitoringSourceHealth(ctx)
	}
	text := func(key string) string { value, _ := filters[key].(string); return value }
	var window string
	switch duration := end.Sub(start); {
	case duration <= time.Minute:
		window = "1m"
	case duration <= 5*time.Minute:
		window = "5m"
	case duration <= 15*time.Minute:
		window = "15m"
	default:
		window = "60m"
	}
	q := OpsMonitoringQuery{Metric: metric, Cluster: text("cluster"), Namespace: text("namespace"), Service: text("service"), Deployment: text("deployment"), Pod: text("pod"), UID: text("uid"), Source: text("source"), Protocol: text("protocol"), Route: text("route"), StatusCode: text("status_code"), Window: window, MinRequests: 1, evaluationTime: end.UTC().Truncate(time.Second)}
	q.Exclude429, _ = filters["exclude_429"].(bool)
	result, err := s.QueryMonitoring(ctx, q)
	if err != nil {
		return nil, err
	}
	if result.State == OpsMonitoringNotApplicable {
		return nil, errors.New("monitoring metric is not applicable")
	}
	if result.State == OpsMonitoringUnknown && len(result.Series) == 0 {
		return nil, errors.New("monitoring metric is unavailable")
	}
	counts := map[string]float64{}
	errorsByResource := map[string]float64{}
	for _, series := range result.Series {
		if len(series.Points) > 0 && series.Points[0].SampleCount != nil {
			counts[monitoringSeriesKey(series.Labels)] = *series.Points[0].SampleCount
		}
	}
	if isRequestAlertMetric(metric) {
		// Histogram quantiles and rate samples already carry the exact count
		// used to mask them. In particular, other HTTP traffic must never make
		// one successful TTFT sample pass a rule's minimum-request threshold.
		usesSampleCount := strings.HasPrefix(metric, "ttft_") || metric == "http_p95_ms" || metric == "http_p99_ms" || strings.HasSuffix(metric, "_rate")
		if !usesSampleCount {
			countQuery := q
			countQuery.Metric = "request_count"
			if metric == "qps_started" || metric == "rpm_started" || metric == "request_count_started" {
				countQuery.Metric = "request_count_started"
			}
			countResult, err := s.QueryMonitoring(ctx, countQuery)
			if err != nil {
				return nil, err
			}
			for _, series := range countResult.Series {
				if len(series.Points) > 0 && series.Points[0].Value != nil {
					counts[monitoringSeriesKey(series.Labels)] = *series.Points[0].Value
				}
			}
		}
		if strings.HasSuffix(metric, "_rate") {
			errorQuery := q
			errorQuery.Metric = strings.TrimSuffix(metric, "_rate") + "_count"
			errorResult, err := s.QueryMonitoring(ctx, errorQuery)
			if err != nil {
				return nil, err
			}
			for _, series := range errorResult.Series {
				if len(series.Points) > 0 && series.Points[0].Value != nil {
					errorsByResource[monitoringSeriesKey(series.Labels)] = *series.Points[0].Value
				}
			}
		}
	}
	out := make([]OpsMonitoringAlertSample, 0, len(result.Series))
	for _, series := range result.Series {
		key := monitoringSeriesKey(series.Labels)
		dimensions := map[string]any{}
		for k, v := range series.Labels {
			dimensions[k] = v
		}
		identity := map[string]string{}
		for k, v := range series.Labels {
			if k != "limit_source" && k != "aggregation" {
				identity[k] = v
			}
		}
		sample := OpsMonitoringAlertSample{ResourceKey: monitoringSeriesKey(identity), Dimensions: dimensions, RequestCount: int64(math.Floor(counts[key]))}
		if len(series.Points) > 0 {
			sample.Value = series.Points[0].Value
		}
		if value, ok := errorsByResource[key]; ok {
			sample.ErrorCount = &value
		}
		if strings.HasSuffix(metric, "_count") && strings.HasPrefix(metric, "http_") {
			sample.ErrorCount = sample.Value
		}
		out = append(out, sample)
	}
	return out, nil
}

func (s *OpsService) evaluateMonitoringSourceHealth(ctx context.Context) ([]OpsMonitoringAlertSample, error) {
	if s == nil {
		return nil, errors.New("ops service unavailable")
	}
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return nil, err
	}
	c := s.monitoringConfig()
	if strings.TrimSpace(c.PrometheusURL) == "" {
		return nil, errors.New("prometheus is not configured")
	}
	if err := c.Validate(); err != nil {
		return nil, errors.New("monitoring configuration is invalid")
	}
	select {
	case monitoringQuerySlots <- struct{}{}:
		defer func() { <-monitoringQuerySlots }()
	default:
		return nil, errors.New("monitoring query capacity exceeded")
	}
	queryCtx, cancel := context.WithTimeout(ctx, time.Duration(c.TimeoutSeconds)*time.Second)
	defer cancel()
	series, err := fetchMonitoringSeries(queryCtx, c, OpsMonitoringQuery{evaluationTime: time.Now().UTC().Truncate(time.Second)}, "vector(1)")
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	value := float64(0)
	if err != nil || len(series) != 1 || len(series[0].Points) != 1 || series[0].Points[0].Value == nil || *series[0].Points[0].Value != 1 {
		value = 1
	}
	return []OpsMonitoringAlertSample{{ResourceKey: "prometheus", Dimensions: map[string]any{"source": "prometheus"}, Value: &value}}, nil
}
