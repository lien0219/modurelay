package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

const monitoringMaxPoints = 11000
const monitoringMaxSeries = 200
const monitoringMaxTotalPoints = 100000

var ErrOpsMonitoringQuery = errors.New("invalid monitoring query")
var monitoringQuerySlots = make(chan struct{}, 8)

var monitoringTransport = &http.Transport{
	MaxIdleConns: 16, MaxIdleConnsPerHost: 8, IdleConnTimeout: 60 * time.Second,
	TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 10 * time.Second,
	// Do not inherit an HTTP proxy that could bypass the destination check.
	DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if !config.MonitoringAddressAllowed(ip.IP) {
				return nil, errors.New("monitoring address is disallowed")
			}
		}
		var last error
		for _, ip := range ips {
			conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
			if err == nil {
				return conn, nil
			}
			last = err
		}
		if last == nil {
			last = errors.New("monitoring address has no IPs")
		}
		return nil, last
	},
}

func (s *OpsService) monitoringConfig() config.OpsMonitoringConfig {
	c := config.OpsMonitoringConfig{}
	if s != nil && s.cfg != nil {
		c = s.cfg.Ops.Monitoring
	}
	if c.Mode == "" || c.Mode == "auto" {
		c.Mode = "standalone"
		if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
			c.Mode = "kubernetes"
		}
	}
	if c.TimeoutSeconds == 0 {
		c.TimeoutSeconds = 8
	}
	if c.MaxQueryRangeHours == 0 {
		c.MaxQueryRangeHours = 744
	}
	if c.ClusterName == "" {
		c.ClusterName = "modurelay"
	}
	if c.EventsRetentionDays == 0 {
		c.EventsRetentionDays = 30
	}
	return c
}

func (s *OpsService) GetMonitoringCapabilities(ctx context.Context) (*OpsMonitoringCapabilities, error) {
	if s == nil {
		return nil, errors.New("ops service unavailable")
	}
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return nil, err
	}
	c := s.monitoringConfig()
	configured := strings.TrimSpace(c.PrometheusURL) != ""
	sources := []string{"app", "ingress"}
	if c.ExternalIngressAdapter != "" {
		sources = append(sources, "external_ingress")
	}
	return &OpsMonitoringCapabilities{Sources: sources, ExternalIngressAdapter: c.ExternalIngressAdapter, KubernetesResources: c.Mode == "kubernetes" && c.KubernetesResourcesEnabled, Mode: c.Mode, PrometheusConfigured: configured, Kubernetes: c.Mode == "kubernetes", History: configured, MaxRangeSeconds: int64(c.MaxQueryRangeHours) * 3600, MaxPointsPerSeries: monitoringMaxPoints, Metrics: append([]OpsMonitoringMetric{}, opsMonitoringMetrics...), KubernetesEvents: c.Mode == "kubernetes" && c.KubernetesEventsEnabled, ClusterName: c.ClusterName}, nil
}

func monitoringWindowSeconds(window string) (int, error) {
	switch window {
	case "1m":
		return 60, nil
	case "5m":
		return 300, nil
	case "15m":
		return 900, nil
	case "60m":
		return 3600, nil
	}
	return 0, fmt.Errorf("%w: window must be 1m, 5m, 15m or 60m", ErrOpsMonitoringQuery)
}

func normalizeMonitoringQuery(q *OpsMonitoringQuery, c config.OpsMonitoringConfig) (OpsMonitoringMetric, error) {
	q.effectiveLimits = c.KubernetesResourcesEnabled
	var def OpsMonitoringMetric
	for _, m := range opsMonitoringMetrics {
		if m.ID == q.Metric {
			def = m
			break
		}
	}
	if def.ID == "" {
		return def, fmt.Errorf("%w: unsupported metric", ErrOpsMonitoringQuery)
	}
	if q.Window == "" {
		q.Window = "5m"
	}
	if _, err := monitoringWindowSeconds(q.Window); err != nil {
		return def, err
	}
	if q.Source == "" {
		q.Source = "app"
	}
	if q.Source != "app" && q.Source != "ingress" && q.Source != "external_ingress" {
		return def, fmt.Errorf("%w: unsupported source", ErrOpsMonitoringQuery)
	}
	if strings.HasSuffix(q.Metric, "_started") && q.Source != "app" {
		return def, fmt.Errorf("%w: inbound metrics only support app source", ErrOpsMonitoringQuery)
	}
	if (strings.HasPrefix(q.Metric, "db_instance_") || strings.HasPrefix(q.Metric, "redis_instance_")) && (q.Namespace != "" || q.Service != "" || q.Deployment != "" || q.Pod != "" || q.UID != "" || q.Source != "app") {
		return def, fmt.Errorf("%w: database instance metrics support only cluster filtering", ErrOpsMonitoringQuery)
	}
	httpMetric := q.Metric == "qps" || q.Metric == "rpm" || strings.HasSuffix(q.Metric, "_started") || q.Metric == "request_count" || strings.HasPrefix(q.Metric, "http_") || strings.HasPrefix(q.Metric, "ttft_")
	if !httpMetric && (q.Protocol != "" || q.Route != "" || q.StatusCode != "") {
		return def, fmt.Errorf("%w: HTTP dimensions only apply to HTTP metrics", ErrOpsMonitoringQuery)
	}
	if q.Protocol != "" && q.Protocol != "http" && q.Protocol != "sse" && q.Protocol != "websocket" {
		return def, fmt.Errorf("%w: unsupported protocol", ErrOpsMonitoringQuery)
	}
	if q.Route != "" {
		switch q.Route {
		case "responses", "messages", "chat_completions", "embeddings", "images", "audio", "models", "gemini", "other":
		default:
			return def, fmt.Errorf("%w: unsupported route", ErrOpsMonitoringQuery)
		}
	}
	if q.Source == "external_ingress" {
		if !httpMetric || strings.HasPrefix(q.Metric, "ttft_") || strings.HasSuffix(q.Metric, "_started") || q.Pod != "" || q.UID != "" || q.Deployment != "" {
			return def, fmt.Errorf("%w: external ingress supports completed HTTP service metrics only", ErrOpsMonitoringQuery)
		}
		if c.ExternalIngressAdapter == "nginx" && (q.Route != "" || q.Protocol != "") {
			return def, fmt.Errorf("%w: nginx does not export application route/protocol; use a normalized adapter", ErrOpsMonitoringQuery)
		}
		q.externalAdapter, q.externalJob = c.ExternalIngressAdapter, c.ExternalIngressJob
		if q.externalJob == "" {
			q.externalJob = "ingress-nginx"
		}
	}
	if q.StatusCode != "" {
		code, err := strconv.Atoi(q.StatusCode)
		if err != nil || code < 100 || code > 599 || len(q.StatusCode) != 3 {
			return def, fmt.Errorf("%w: status_code must be an HTTP status between 100 and 599", ErrOpsMonitoringQuery)
		}
		if strings.HasSuffix(q.Metric, "_started") || strings.HasSuffix(q.Metric, "_ms") {
			return def, fmt.Errorf("%w: status_code is unavailable for arrivals and histograms", ErrOpsMonitoringQuery)
		}
	}
	if strings.HasSuffix(q.Metric, "_started") && q.Protocol != "" {
		return def, fmt.Errorf("%w: response protocol is unavailable before request completion", ErrOpsMonitoringQuery)
	}
	for _, v := range []string{q.Cluster, q.Namespace, q.Service, q.Deployment, q.Pod, q.UID} {
		if len(v) > 253 || strings.ContainsAny(v, "\x00\r\n") {
			return def, fmt.Errorf("%w: invalid resource filter", ErrOpsMonitoringQuery)
		}
	}
	if q.MinRequests < 0 || q.MinRequests > 1000000 {
		return def, fmt.Errorf("%w: min_requests must be between 0 and 1000000", ErrOpsMonitoringQuery)
	}
	if q.MinRequests == 0 {
		q.MinRequests = 20
	}
	if strings.HasPrefix(q.Metric, "hpa_") && (q.Service != "" || q.Deployment != "" || q.Pod != "" || q.UID != "") {
		return def, fmt.Errorf("%w: HPA metrics support cluster and namespace filters", ErrOpsMonitoringQuery)
	}
	if strings.HasPrefix(q.Metric, "deployment_") && (q.Service != "" || q.Pod != "" || q.UID != "") {
		return def, fmt.Errorf("%w: deployment metrics support cluster, namespace and deployment filters", ErrOpsMonitoringQuery)
	}
	if q.Start.IsZero() {
		if !q.End.IsZero() {
			return def, fmt.Errorf("%w: start and end must be provided together", ErrOpsMonitoringQuery)
		}
		q.StepSeconds = 0
		return def, nil
	}
	if q.End.IsZero() || !q.End.After(q.Start) {
		return def, fmt.Errorf("%w: end must be after start", ErrOpsMonitoringQuery)
	}
	if q.End.Sub(q.Start) > time.Duration(c.MaxQueryRangeHours)*time.Hour {
		return def, fmt.Errorf("%w: requested range exceeds configured limit", ErrOpsMonitoringQuery)
	}
	if q.End.After(time.Now().Add(time.Minute)) {
		return def, fmt.Errorf("%w: end cannot be in the future", ErrOpsMonitoringQuery)
	}
	if q.StepSeconds == 0 {
		q.StepSeconds = 15
		if q.End.Sub(q.Start) > 6*time.Hour {
			q.StepSeconds = 60
		}
		if q.End.Sub(q.Start) > 48*time.Hour {
			q.StepSeconds = 300
		}
		if q.End.Sub(q.Start) > 7*24*time.Hour {
			q.StepSeconds = 900
		}
	}
	if q.StepSeconds < 15 || q.StepSeconds > 86400 || int(q.End.Sub(q.Start).Seconds())/q.StepSeconds+1 > monitoringMaxPoints {
		return def, fmt.Errorf("%w: step must be 15..86400 seconds and at most 11000 points", ErrOpsMonitoringQuery)
	}
	return def, nil
}

// QueryMonitoring deliberately reports data-source failures as Unknown, with
// no raw upstream errors, hostnames, tokens, or response bodies exposed.
func (s *OpsService) QueryMonitoring(ctx context.Context, q OpsMonitoringQuery) (*OpsMonitoringResult, error) {
	if s == nil {
		return nil, errors.New("ops service unavailable")
	}
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return nil, err
	}
	c := s.monitoringConfig()
	def, err := normalizeMonitoringQuery(&q, c)
	if err != nil {
		return nil, err
	}
	if q.evaluationTime.IsZero() {
		q.evaluationTime = time.Now().UTC().Truncate(time.Second)
	}
	out := &OpsMonitoringResult{Metric: q.Metric, Unit: def.Unit, State: OpsMonitoringUnknown, StepSeconds: q.StepSeconds, Series: []OpsMonitoringSeries{}}
	if def.KubernetesOnly && c.Mode != "kubernetes" {
		out.State = OpsMonitoringNotApplicable
		out.Reason = "kubernetes_unavailable"
		return out, nil
	}
	if q.Source == "external_ingress" && c.ExternalIngressAdapter == "" {
		out.State, out.Reason = OpsMonitoringNotApplicable, "external_ingress_not_configured"
		return out, nil
	}
	if strings.TrimSpace(c.PrometheusURL) == "" {
		out.Reason = "prometheus_not_configured"
		return out, nil
	}
	if err := c.Validate(); err != nil {
		out.Reason = "prometheus_configuration_invalid"
		return out, nil
	}
	expr, countExpr, err := monitoringPromQL(q)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(c.TimeoutSeconds)*time.Second)
	defer cancel()
	select {
	case monitoringQuerySlots <- struct{}{}:
		defer func() { <-monitoringQuerySlots }()
	default:
		out.Reason = "query_capacity_exceeded"
		return out, nil
	}
	series, err := fetchMonitoringSeries(ctx, c, q, expr)
	if err != nil {
		out.Reason = "prometheus_query_failed"
		return out, nil
	}
	if countExpr != "" {
		counts, err := fetchMonitoringSeries(ctx, c, q, countExpr)
		if err != nil {
			out.Reason = "prometheus_query_failed"
			return out, nil
		}
		maskMonitoringSamples(series, counts, q.MinRequests)
	}
	out.Series = series
	for _, series := range series {
		for _, point := range series.Points {
			if point.Value != nil {
				out.State = OpsMonitoringAvailable
			}
			if point.State == OpsMonitoringInsufficientData && out.State != OpsMonitoringAvailable {
				out.State = OpsMonitoringInsufficientData
			}
		}
	}
	if out.State == OpsMonitoringUnknown && len(series) > 0 {
		allUnlimited := true
		for _, item := range series {
			for _, point := range item.Points {
				if point.State != OpsMonitoringNotApplicable {
					allUnlimited = false
				}
			}
		}
		if allUnlimited {
			out.State = OpsMonitoringNotApplicable
			out.Reason = "resource_unlimited"
		}
	}
	if out.State == OpsMonitoringUnknown {
		out.Reason = "samples_unavailable"
	}
	if out.State == OpsMonitoringInsufficientData {
		out.Reason = "minimum_requests_not_met"
	}
	return out, nil
}

type monitoringPromResponse struct {
	Status   string   `json:"status"`
	Warnings []string `json:"warnings"`
	Data     struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string   `json:"metric"`
			Value  []json.RawMessage   `json:"value"`
			Values [][]json.RawMessage `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

func fetchMonitoringSeries(ctx context.Context, c config.OpsMonitoringConfig, q OpsMonitoringQuery, expr string) ([]OpsMonitoringSeries, error) {
	u, err := url.Parse(c.PrometheusURL)
	if err != nil {
		return nil, err
	}
	params := url.Values{"query": {expr}, "timeout": {strconv.Itoa(c.TimeoutSeconds) + "s"}}
	endpoint := "query"
	params.Set("time", q.evaluationTime.UTC().Format(time.RFC3339Nano))
	if !q.Start.IsZero() {
		endpoint = "query_range"
		params.Set("start", q.Start.UTC().Format(time.RFC3339Nano))
		params.Set("end", q.End.UTC().Format(time.RFC3339Nano))
		params.Set("step", strconv.Itoa(q.StepSeconds))
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/v1/" + endpoint
	// Prometheus query POST is read-only and avoids proxy URI length limits for
	// the bounded but deliberately explicit resource/freshness expressions.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if c.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.BearerToken)
	}
	client := &http.Client{Transport: monitoringTransport, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("monitoring redirects disabled") }}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("monitoring upstream failed")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(body) > 8*1024*1024 {
		return nil, errors.New("monitoring response too large")
	}
	var raw monitoringPromResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	expected := "vector"
	if !q.Start.IsZero() {
		expected = "matrix"
	}
	if raw.Status != "success" || raw.Data.ResultType != expected || len(raw.Warnings) > 0 {
		return nil, errors.New("monitoring returned incomplete data")
	}
	if len(raw.Data.Result) > monitoringMaxSeries {
		return nil, errors.New("monitoring series limit exceeded")
	}
	out := make([]OpsMonitoringSeries, 0, len(raw.Data.Result))
	total := 0
	for _, result := range raw.Data.Result {
		series := OpsMonitoringSeries{Labels: monitoringSafeLabels(result.Metric), Points: []OpsMonitoringPoint{}}
		values := result.Values
		if expected == "vector" {
			values = [][]json.RawMessage{result.Value}
		}
		if len(values) > monitoringMaxPoints {
			return nil, errors.New("monitoring point limit exceeded")
		}
		for _, v := range values {
			point, err := decodeMonitoringPoint(v)
			if err != nil {
				return nil, err
			}
			if series.Labels["limit_state"] == "unlimited" && point.Value != nil {
				point.Value = nil
				point.State = OpsMonitoringNotApplicable
			}
			series.Points = append(series.Points, point)
		}
		if expected == "matrix" {
			series.Points = fillMonitoringGaps(series.Points, q)
		}
		total += len(series.Points)
		if total > monitoringMaxTotalPoints {
			return nil, errors.New("monitoring total point limit exceeded")
		}
		out = append(out, series)
	}
	sort.Slice(out, func(i, j int) bool { return monitoringSeriesKey(out[i].Labels) < monitoringSeriesKey(out[j].Labels) })
	return out, nil
}

func monitoringSafeLabels(labels map[string]string) map[string]string {
	out := map[string]string{}
	for _, key := range []string{"cluster", "namespace", "service", "deployment", "pod", "uid", "source", "instance", "horizontalpodautoscaler", "node", "limit_source", "limit_state", "aggregation"} {
		if v := labels[key]; v != "" && len(v) <= 253 {
			out[key] = v
		}
	}
	return out
}

func decodeMonitoringPoint(raw []json.RawMessage) (OpsMonitoringPoint, error) {
	point := OpsMonitoringPoint{State: OpsMonitoringUnknown}
	if len(raw) != 2 {
		return point, errors.New("invalid monitoring point")
	}
	var timestamp float64
	var text string
	if err := json.Unmarshal(raw[0], &timestamp); err != nil {
		return point, err
	}
	if err := json.Unmarshal(raw[1], &text); err != nil {
		return point, err
	}
	point.Timestamp = time.UnixMilli(int64(timestamp * 1000)).UTC()
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return point, err
	}
	if !math.IsNaN(value) && !math.IsInf(value, 0) {
		point.Value = &value
		point.State = OpsMonitoringAvailable
	}
	return point, nil
}

func fillMonitoringGaps(points []OpsMonitoringPoint, q OpsMonitoringQuery) []OpsMonitoringPoint {
	indexed := map[int64]OpsMonitoringPoint{}
	for _, point := range points {
		indexed[point.Timestamp.UnixMilli()] = point
	}
	out := make([]OpsMonitoringPoint, 0, int(q.End.Sub(q.Start).Seconds())/q.StepSeconds+1)
	for at := q.Start; !at.After(q.End); at = at.Add(time.Duration(q.StepSeconds) * time.Second) {
		point, ok := indexed[at.UnixMilli()]
		if !ok {
			point = OpsMonitoringPoint{Timestamp: at.UTC(), State: OpsMonitoringUnknown}
		}
		out = append(out, point)
	}
	return out
}

func monitoringSeriesKey(labels map[string]string) string {
	data, _ := json.Marshal(labels)
	return string(data)
}

func maskMonitoringSamples(series, counts []OpsMonitoringSeries, minimum int64) {
	index := map[string]map[int64]*float64{}
	for _, s := range counts {
		values := map[int64]*float64{}
		for _, p := range s.Points {
			values[p.Timestamp.UnixMilli()] = p.Value
		}
		index[monitoringSeriesKey(s.Labels)] = values
	}
	for si := range series {
		for pi := range series[si].Points {
			point := &series[si].Points[pi]
			count := index[monitoringSeriesKey(series[si].Labels)][point.Timestamp.UnixMilli()]
			point.SampleCount = count
			if count == nil {
				point.Value = nil
				point.State = OpsMonitoringUnknown
			} else if *count < float64(minimum) {
				point.Value = nil
				point.State = OpsMonitoringInsufficientData
			}
		}
	}
}
