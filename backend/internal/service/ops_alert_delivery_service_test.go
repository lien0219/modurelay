//go:build unit

package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpsWebhookURLRejectsUnsafeDestinations(t *testing.T) {
	for _, target := range []string{"http://example.com/hook", "https://localhost/hook", "https://127.0.0.1/hook", "https://[::1]/hook", "https://10.1.2.3/hook", "https://169.254.169.254/latest/meta-data", "https://100.100.100.200/", "https://192.0.2.1/", "https://user:secret@example.com/", "https://example.com:8443/", "https://example.com/#token"} {
		t.Run(target, func(t *testing.T) { require.Error(t, ValidateOpsAlertWebhookURL(target)) })
	}
	require.NoError(t, ValidateOpsAlertWebhookURL("https://hooks.example.com/alerts"))
	require.False(t, publicWebhookIP(net.ParseIP("::ffff:127.0.0.1")))
	require.False(t, publicWebhookIP(net.ParseIP("64:ff9b::a00:1")))
	require.True(t, publicWebhookIP(net.ParseIP("8.8.8.8")))
}
func TestOpsWebhookSignatureAuthenticatesTimestampAndBody(t *testing.T) {
	payload := []byte(`{"transition":"resolved"}`)
	secret := strings.Repeat("s", 32)
	signature := opsWebhookSignature(secret, "12345", payload)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte("12345."))
	_, _ = mac.Write(payload)
	require.Equal(t, "sha256="+hex.EncodeToString(mac.Sum(nil)), signature)
	require.NotEqual(t, signature, opsWebhookSignature(secret, "12346", payload))
	require.NotEqual(t, signature, opsWebhookSignature(secret, "12345", []byte(`{}`)))
}
func TestAlertDeliveryBackoffIsBounded(t *testing.T) {
	require.Equal(t, time.Minute, alertDeliveryBackoff(1))
	require.Equal(t, 2*time.Minute, alertDeliveryBackoff(2))
	require.Equal(t, 64*time.Minute, alertDeliveryBackoff(200))
}
func TestPrepareAlertDeliveriesSnapshotsEachRecipientAndRecovery(t *testing.T) {
	cfg := defaultOpsEmailNotificationConfig()
	cfg.Alert.Enabled = true
	cfg.Alert.IncludeResolvedAlerts = true
	cfg.Alert.Recipients = []string{"first@example.com", "second@example.com"}
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	repo := &durableAlertTestRepo{stubOpsRepo: &stubOpsRepo{}}
	svc := &OpsAlertEvaluatorService{opsRepo: repo, opsService: &OpsService{settingRepo: &alertSettingsRepo{values: map[string]string{SettingKeyOpsEmailNotificationConfig: string(raw)}}}}
	rule := &OpsAlertRule{ID: 1, Name: "延迟过高", NotifyEmail: true, Severity: "P1", Filters: map[string]any{"webhook_urls": []string{"https://hooks.example.com/a", "https://hooks.example.com/a"}}}
	event := &OpsAlertEvent{RuleID: 1, Status: OpsAlertStatusResolved, Severity: "P0", FiredAt: time.Now()}
	tasks, err := svc.prepareAlertDeliveries(context.Background(), nil, rule, event, "resolved")
	require.NoError(t, err)
	require.Len(t, tasks, 3)
	require.Equal(t, "first@example.com", tasks[0].Target)
	require.Equal(t, "resolved", tasks[0].Transition)
	var payload opsAlertEmailPayload
	require.NoError(t, json.Unmarshal(tasks[0].Payload, &payload))
	require.Contains(t, payload.Subject, "运维恢复")
	require.Contains(t, payload.Subject, "P0")
	api, err := json.Marshal(tasks[0])
	require.NoError(t, err)
	require.NotContains(t, string(api), "body")
	require.NotContains(t, string(api), "lease_token")
}
func TestMonitoringAlertSampleGatesRequestsAndUnknowns(t *testing.T) {
	value := 1.0
	errors := 0.0
	rule := &OpsAlertRule{MetricType: "http_5xx_rate", MinRequests: 10, Filters: map[string]any{"min_errors": float64(2)}}
	sample := OpsMonitoringAlertSample{Value: &value, RequestCount: 9, ErrorCount: &errors}
	require.Equal(t, alertMetricInsufficient, monitoringAlertSampleStatus(rule, sample))
	sample.RequestCount = 10
	// Few errors can recover an event, provided the request sample is adequate.
	require.Equal(t, alertMetricOK, monitoringAlertSampleStatus(rule, sample))
	sample.ErrorCount = nil
	require.Equal(t, alertMetricUnavailable, monitoringAlertSampleStatus(rule, sample))
	sample.Value = nil
	require.Equal(t, alertMetricUnavailable, monitoringAlertSampleStatus(rule, sample))
}
func TestAlertFilterValidationRejectsSilentlyIgnoredScopesAndSecrets(t *testing.T) {
	require.Error(t, ValidateOpsAlertFilters("cpu_usage_percent", map[string]any{"pod": "gateway-0"}))
	require.Error(t, ValidateOpsAlertFilters("http_p99_ms", map[string]any{"pod": "x\"} or up{}"}))
	require.Error(t, ValidateOpsAlertFilters("http_5xx_rate", map[string]any{"exclude_429": true}))
	require.Error(t, ValidateOpsAlertFilters("http_p99_ms", map[string]any{"webhook_secret": "secret"}))
	require.NoError(t, ValidateOpsAlertFilters("http_4xx_rate", map[string]any{"source": "app", "namespace": "default", "exclude_429": true, "min_errors": float64(10)}))
}

type durableAlertTestRepo struct {
	*stubOpsRepo
	OpsAlertDeliveryRepository
	resources   map[string]*OpsAlertRuleState
	events      map[string]*OpsAlertEvent
	created     []*OpsAlertEvent
	transitions []string
	deliveries  map[string]*OpsAlertDelivery
}

func (r *durableAlertTestRepo) EnsureFiringAlertDeliveries(_ context.Context, eventID int64, deliveries []*OpsAlertDelivery) error {
	if r.deliveries == nil {
		r.deliveries = map[string]*OpsAlertDelivery{}
	}
	for _, d := range deliveries {
		key := strconv.FormatInt(eventID, 10) + ":" + d.Channel + ":" + d.Transition + ":" + d.Target
		if _, exists := r.deliveries[key]; !exists {
			r.deliveries[key] = d
		}
	}
	return nil
}

func (r *durableAlertTestRepo) GetAlertResourceState(_ context.Context, _ int64, key string) (*OpsAlertRuleState, error) {
	if v := r.resources[key]; v != nil {
		copy := *v
		return &copy, nil
	}
	return nil, nil
}
func (r *durableAlertTestRepo) UpsertAlertResourceState(_ context.Context, key string, state *OpsAlertRuleState) error {
	if r.resources == nil {
		r.resources = map[string]*OpsAlertRuleState{}
	}
	copy := *state
	r.resources[key] = &copy
	return nil
}
func (r *durableAlertTestRepo) GetResourceAlertEvent(_ context.Context, _ int64, key string, active bool) (*OpsAlertEvent, error) {
	e := r.events[key]
	if active && e != nil && e.Status != OpsAlertStatusFiring {
		return nil, nil
	}
	return e, nil
}
func (r *durableAlertTestRepo) CreateAlertEvent(_ context.Context, event *OpsAlertEvent) (*OpsAlertEvent, error) {
	if r.events == nil {
		r.events = map[string]*OpsAlertEvent{}
	}
	event.ID = int64(len(r.created) + 1)
	r.created = append(r.created, event)
	r.events[event.Dimensions["resource_key"].(string)] = event
	return event, nil
}
func (r *durableAlertTestRepo) TransitionAlertEvent(_ context.Context, event *OpsAlertEvent, _ []*OpsAlertDelivery) (bool, error) {
	r.events[event.Dimensions["resource_key"].(string)] = event
	r.transitions = append(r.transitions, event.Status+":"+event.Severity)
	return true, nil
}
func TestResourceAlertsPersistIsolateUpgradeAndRecover(t *testing.T) {
	repo := &durableAlertTestRepo{stubOpsRepo: &stubOpsRepo{}}
	a, b := 200.0, 1.0
	samples := []OpsMonitoringAlertSample{{ResourceKey: "pod-uid-a", Value: &a, RequestCount: 100}, {ResourceKey: "pod-uid-b", Value: &b, RequestCount: 100}}
	rule := &OpsAlertRule{ID: 5, Name: "latency", MetricType: "http_p99_ms", Operator: ">", Threshold: 100, Severity: "P1", SustainedMinutes: 1, MinRequests: 10, Filters: map[string]any{"critical_threshold": float64(500)}}
	newSvc := func() *OpsAlertEvaluatorService {
		return &OpsAlertEvaluatorService{opsRepo: repo, opsService: &OpsService{}, monitoringAlertSamples: func(context.Context, string, map[string]any, time.Time, time.Time) ([]OpsMonitoringAlertSample, error) {
			return samples, nil
		}}
	}
	now := time.Now().UTC()
	svc := newSvc()
	require.True(t, svc.evaluateMonitoringAlertRule(context.Background(), nil, rule, now.Add(-time.Minute), now, now, time.Minute))
	require.Empty(t, repo.created)
	// New evaluator reads the first evaluator's persisted resource counter.
	svc = newSvc()
	now = now.Add(time.Minute)
	svc.evaluateMonitoringAlertRule(context.Background(), nil, rule, now.Add(-time.Minute), now, now, time.Minute)
	require.Len(t, repo.created, 1)
	require.Equal(t, "pod-uid-a", repo.created[0].Dimensions["resource_key"])
	// Low traffic cannot recover, and an unrelated Pod remains normal.
	samples[0].RequestCount = 1
	a = 0
	now = now.Add(time.Minute)
	svc.evaluateMonitoringAlertRule(context.Background(), nil, rule, now.Add(-time.Minute), now, now, time.Minute)
	require.Empty(t, repo.transitions)
	require.Equal(t, OpsAlertDataStatusInsufficient, repo.resources["pod-uid-a"].DataStatus)
	samples[0].RequestCount = 100
	a = 700
	now = now.Add(time.Minute)
	svc.evaluateMonitoringAlertRule(context.Background(), nil, rule, now.Add(-time.Minute), now, now, time.Minute)
	require.Equal(t, []string{"firing:P0"}, repo.transitions)
	a = 0
	now = now.Add(time.Minute)
	svc.evaluateMonitoringAlertRule(context.Background(), nil, rule, now.Add(-time.Minute), now, now, time.Minute)
	require.Equal(t, []string{"firing:P0", "resolved:P0"}, repo.transitions)
	now = now.Add(time.Minute)
	svc.evaluateMonitoringAlertRule(context.Background(), nil, rule, now.Add(-time.Minute), now, now, time.Minute)
	require.Len(t, repo.transitions, 2)
}

func (r *durableAlertTestRepo) IsAlertResourceSilenced(context.Context, int64, string, time.Time) (bool, error) {
	return false, nil
}

type opsAlertRoundTripper func(*http.Request) (*http.Response, error)

func (f opsAlertRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestOpsWebhookDeliveryStatusSignatureAndTimeout(t *testing.T) {
	t.Setenv("OPS_ALERT_WEBHOOK_SECRET", strings.Repeat("k", 32))
	payload := []byte(`{"transition":"firing"}`)
	for _, code := range []int{200, 429, 500} {
		client := &http.Client{Transport: opsAlertRoundTripper(func(req *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			require.Equal(t, payload, body)
			require.Equal(t, "ops-alert-17", req.Header.Get("Idempotency-Key"))
			require.Equal(t, opsWebhookSignature(strings.Repeat("k", 32), req.Header.Get("X-ModuRelay-Timestamp"), body), req.Header.Get("X-ModuRelay-Signature"))
			return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader("body contains a secret that must not be logged")), Header: make(http.Header)}, nil
		})}
		err := sendOpsAlertWebhookWithClient(context.Background(), "https://hooks.example.com/", payload, "17", client)
		if code == 200 {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
			require.NotContains(t, err.Error(), "secret")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &http.Client{Transport: opsAlertRoundTripper(func(req *http.Request) (*http.Response, error) { return nil, req.Context().Err() })}
	require.Error(t, sendOpsAlertWebhookWithClient(ctx, "https://hooks.example.com/", payload, "17", client))
}
func TestOpsWebhookDialRejectsPrivateIPBeforeConnecting(t *testing.T) {
	client := opsWebhookHTTPClient()
	defer client.CloseIdleConnections()
	transport := client.Transport.(*http.Transport)
	_, err := transport.DialContext(context.Background(), "tcp", "127.0.0.1:443")
	require.ErrorContains(t, err, "forbidden address")
}
func TestDuplicateEvaluationDoesNotAccumulateSustainedEvidence(t *testing.T) {
	now := time.Now().UTC()
	repo := &stubOpsRepo{states: map[int64]*OpsAlertRuleState{1: {RuleID: 1, ConsecutiveBreaches: 1, LastEvaluatedAt: now, DataStatus: "ok"}}}
	evaluator := &OpsAlertEvaluatorService{opsRepo: repo}
	count, updated := evaluator.updateRuleBreaches(context.Background(), 1, now.Add(time.Second), time.Minute, true)
	require.False(t, updated)
	require.Equal(t, 1, count)
	require.Empty(t, repo.upserts)
	require.Equal(t, 3, requiredSustainedBreaches(2, time.Minute))
}

func (r *durableAlertTestRepo) MarkMissingAlertResources(_ context.Context, _ int64, revision int64, seen []string, now time.Time) (bool, error) {
	present := map[string]bool{}
	for _, key := range seen {
		present[key] = true
	}
	missing := false
	for key, state := range r.resources {
		if !present[key] && alertRuleRevision(state.RuleRevision) == alertRuleRevision(revision) {
			state.ConsecutiveBreaches = 0
			state.LastEvaluatedAt = now
			state.DataStatus = OpsAlertDataStatusUnavailable
			missing = true
		}
	}
	return missing, nil
}
