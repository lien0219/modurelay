package service

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/util/logredact"
)

var monitoringEventBearer = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]+`)
var monitoringEventURLCredentials = regexp.MustCompile(`(?i)(https?://)[^\s/@]+:[^\s/@]+@`)

type OpsKubernetesEvent struct {
	Cluster     string    `json:"cluster"`
	Namespace   string    `json:"namespace"`
	ObjectKind  string    `json:"object_kind"`
	ObjectName  string    `json:"object_name"`
	ObjectUID   string    `json:"object_uid"`
	EventUID    string    `json:"event_uid"`
	Pod         string    `json:"pod"`
	PodUID      string    `json:"pod_uid"`
	Reason      string    `json:"reason"`
	Type        string    `json:"type"`
	Message     string    `json:"message"`
	Source      string    `json:"source"`
	FirstSeenAt time.Time `json:"first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
	Count       int64     `json:"count"`
}

type OpsMonitoringEventsFilter struct {
	Cluster, Namespace, Pod, UID      string
	ObjectKind, ObjectName, ObjectUID string
	Start, End                        time.Time
	Limit                             int
}

type OpsMonitoringEventsResult struct {
	State           OpsMonitoringState   `json:"state"`
	Reason          string               `json:"reason,omitempty"`
	LastCollectedAt *time.Time           `json:"last_collected_at,omitempty"`
	Events          []OpsKubernetesEvent `json:"events"`
}

// Optional port keeps existing repository mocks and integrations compatible.
type OpsMonitoringEventsRepository interface {
	UpsertMonitoringEvents(context.Context, []OpsKubernetesEvent) error
	ListMonitoringEvents(context.Context, OpsMonitoringEventsFilter) ([]OpsKubernetesEvent, error)
	DeleteExpiredMonitoringEvents(context.Context, string, time.Time) error
}

func monitoringEventsJobName(c config.OpsMonitoringConfig) string {
	sum := sha256.Sum256([]byte(c.ClusterName + "|" + c.EventsNamespace))
	return "ops_kubernetes_events_" + hex.EncodeToString(sum[:16])
}

func (s *OpsService) ListMonitoringEvents(ctx context.Context, filter OpsMonitoringEventsFilter) (*OpsMonitoringEventsResult, error) {
	if s == nil {
		return nil, errors.New("ops service unavailable")
	}
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return nil, err
	}
	c := s.monitoringConfig()
	out := &OpsMonitoringEventsResult{State: OpsMonitoringUnknown, Events: []OpsKubernetesEvent{}}
	if c.Mode != "kubernetes" || !c.KubernetesEventsEnabled {
		out.State = OpsMonitoringNotApplicable
		out.Reason = "kubernetes_events_not_configured"
		return out, nil
	}
	for _, value := range []string{filter.Cluster, filter.Namespace, filter.Pod, filter.UID, filter.ObjectKind, filter.ObjectName, filter.ObjectUID} {
		if len(value) > 253 || strings.ContainsAny(value, "\x00\r\n") {
			return nil, fmt.Errorf("%w: invalid event filter", ErrOpsMonitoringQuery)
		}
	}
	if filter.ObjectKind != "" && filter.ObjectKind != "Pod" && filter.ObjectKind != "Deployment" && filter.ObjectKind != "HorizontalPodAutoscaler" && filter.ObjectKind != "ReplicaSet" {
		return nil, fmt.Errorf("%w: unsupported event object kind", ErrOpsMonitoringQuery)
	}
	if filter.Cluster == "" {
		filter.Cluster = c.ClusterName
	}
	if filter.Limit == 0 {
		filter.Limit = 100
	}
	if filter.Limit < 1 || filter.Limit > 500 {
		return nil, fmt.Errorf("%w: event limit must be 1..500", ErrOpsMonitoringQuery)
	}
	if filter.End.IsZero() {
		filter.End = time.Now().UTC()
	}
	if filter.Start.IsZero() {
		filter.Start = filter.End.Add(-time.Hour)
	}
	if !filter.End.After(filter.Start) || filter.End.Sub(filter.Start) > time.Duration(c.MaxQueryRangeHours)*time.Hour {
		return nil, fmt.Errorf("%w: invalid event time range", ErrOpsMonitoringQuery)
	}
	repo, ok := s.opsRepo.(OpsMonitoringEventsRepository)
	if !ok {
		out.Reason = "events_storage_unavailable"
		return out, nil
	}
	events, err := repo.ListMonitoringEvents(ctx, filter)
	if err != nil {
		out.Reason = "events_storage_unavailable"
		return out, nil
	}
	out.Events = events
	if out.Events == nil {
		out.Events = []OpsKubernetesEvent{}
	}
	if filter.Cluster != c.ClusterName {
		out.Reason = "events_cluster_not_local"
		return out, nil
	}
	heartbeats, err := s.opsRepo.ListJobHeartbeats(ctx)
	if err != nil {
		out.Reason = "events_collector_unavailable"
		return out, nil
	}
	for _, heartbeat := range heartbeats {
		if heartbeat == nil || heartbeat.JobName != monitoringEventsJobName(c) {
			continue
		}
		out.LastCollectedAt = heartbeat.LastSuccessAt
		if heartbeat.LastSuccessAt != nil && time.Since(*heartbeat.LastSuccessAt) <= 2*time.Minute && (heartbeat.LastErrorAt == nil || !heartbeat.LastErrorAt.After(*heartbeat.LastSuccessAt)) {
			out.State = OpsMonitoringAvailable
			return out, nil
		}
	}
	out.Reason = "events_collector_unavailable"
	return out, nil
}

// This runs on the existing once-per-minute collector leader. API outages and
// permission failures affect only this job's heartbeat, never request handling.
func (c *OpsMetricsCollector) collectMonitoringEvents() {
	if c == nil || c.cfg == nil || c.opsRepo == nil {
		return
	}
	cfg := (&OpsService{cfg: c.cfg}).monitoringConfig()
	if cfg.Mode != "kubernetes" || !cfg.KubernetesEventsEnabled {
		return
	}
	repo, ok := c.opsRepo.(OpsMonitoringEventsRepository)
	if !ok {
		return
	}
	started := time.Now().UTC()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	events, err := readInClusterMonitoringEvents(ctx, cfg)
	if err == nil {
		err = repo.UpsertMonitoringEvents(ctx, events)
	}
	if err == nil {
		err = repo.DeleteExpiredMonitoringEvents(ctx, cfg.ClusterName, started.Add(-time.Duration(cfg.EventsRetentionDays)*24*time.Hour))
	}
	finished := time.Now().UTC()
	duration := finished.Sub(started).Milliseconds()
	heartbeat := &OpsUpsertJobHeartbeatInput{JobName: monitoringEventsJobName(cfg), LastRunAt: &started, LastDurationMs: &duration}
	if err != nil {
		message := "Kubernetes event collection failed; verify namespace event read permission and API connectivity"
		heartbeat.LastErrorAt = &finished
		heartbeat.LastError = &message
	} else {
		heartbeat.LastSuccessAt = &finished
	}
	heartbeatCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
	defer stop()
	_ = c.opsRepo.UpsertJobHeartbeat(heartbeatCtx, heartbeat)
}

func readInClusterMonitoringEvents(ctx context.Context, c config.OpsMonitoringConfig) ([]OpsKubernetesEvent, error) {
	client, base, token, close, err := monitoringKubernetesAPI(c)
	if err != nil {
		return nil, err
	}
	defer close()
	return fetchKubernetesMonitoringEvents(ctx, client, base+"/events", token, c.ClusterName)
}

// Shared bounded, standard Kubernetes API client. Credentials never leave the
// service account mount and redirects/proxies cannot change the destination.
func monitoringKubernetesAPI(c config.OpsMonitoringConfig) (*http.Client, string, string, func(), error) {
	fail := func(message string) (*http.Client, string, string, func(), error) {
		return nil, "", "", nil, errors.New(message)
	}
	const mount = "/var/run/secrets/kubernetes.io/serviceaccount/"
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT_HTTPS")
	if port == "" {
		port = "443"
	}
	if host == "" {
		return fail("Kubernetes API is unavailable")
	}
	token, err := os.ReadFile(mount + "token")
	if err != nil {
		return fail("Kubernetes service account is unavailable")
	}
	ca, err := os.ReadFile(mount + "ca.crt")
	if err != nil {
		return fail("Kubernetes certificate is unavailable")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return fail("Kubernetes certificate is invalid")
	}
	namespace := c.EventsNamespace
	if namespace == "" {
		namespace = os.Getenv("POD_NAMESPACE")
	}
	if namespace == "" {
		value, err := os.ReadFile(mount + "namespace")
		if err != nil {
			return fail("Kubernetes namespace is unavailable")
		}
		namespace = strings.TrimSpace(string(value))
	}
	if namespace == "" || strings.ContainsAny(namespace, "/\\\x00\r\n") {
		return fail("Kubernetes namespace is invalid")
	}
	transport := monitoringTransport.Clone()
	transport.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("Kubernetes redirects disabled") }}
	base := "https://" + net.JoinHostPort(host, port) + "/api/v1"
	if namespace != "*" {
		base += "/namespaces/" + url.PathEscape(namespace)
	}
	return client, base, strings.TrimSpace(string(token)), transport.CloseIdleConnections, nil
}

type monitoringKubernetesEventList struct {
	Metadata struct {
		Continue string `json:"continue"`
	} `json:"metadata"`
	Items []struct {
		Metadata struct {
			UID               string    `json:"uid"`
			Namespace         string    `json:"namespace"`
			CreationTimestamp time.Time `json:"creationTimestamp"`
		} `json:"metadata"`
		InvolvedObject struct {
			Kind      string `json:"kind"`
			Name      string `json:"name"`
			UID       string `json:"uid"`
			Namespace string `json:"namespace"`
		} `json:"involvedObject"`
		Reason         string    `json:"reason"`
		Type           string    `json:"type"`
		Message        string    `json:"message"`
		FirstTimestamp time.Time `json:"firstTimestamp"`
		LastTimestamp  time.Time `json:"lastTimestamp"`
		EventTime      time.Time `json:"eventTime"`
		Count          int64     `json:"count"`
		Series         struct {
			Count            int64     `json:"count"`
			LastObservedTime time.Time `json:"lastObservedTime"`
		} `json:"series"`
		Source struct {
			Component string `json:"component"`
		} `json:"source"`
		ReportingComponent string `json:"reportingComponent"`
	} `json:"items"`
}

func fetchKubernetesMonitoringEvents(ctx context.Context, client *http.Client, endpoint, token, cluster string) ([]OpsKubernetesEvent, error) {
	out := []OpsKubernetesEvent{}
	continuation := ""
	for page := 0; page < 20; page++ {
		u, err := url.Parse(endpoint)
		if err != nil {
			return nil, err
		}
		q := url.Values{"limit": {"500"}}
		if continuation != "" {
			q.Set("continue", continuation)
		}
		u.RawQuery = q.Encode()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024+1))
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || readErr != nil || len(body) > 4*1024*1024 {
			return nil, errors.New("Kubernetes event response unavailable")
		}
		var list monitoringKubernetesEventList
		if err := json.Unmarshal(body, &list); err != nil {
			return nil, errors.New("Kubernetes event response invalid")
		}
		if len(list.Items) > 500 {
			return nil, errors.New("Kubernetes event page too large")
		}
		for _, item := range list.Items {
			if (item.InvolvedObject.Kind != "Pod" && item.InvolvedObject.Kind != "Deployment" && item.InvolvedObject.Kind != "HorizontalPodAutoscaler" && item.InvolvedObject.Kind != "ReplicaSet") || item.InvolvedObject.UID == "" || item.Metadata.UID == "" {
				continue
			}
			first := item.FirstTimestamp
			if first.IsZero() {
				first = item.EventTime
			}
			if first.IsZero() {
				first = item.Metadata.CreationTimestamp
			}
			if first.IsZero() {
				continue
			}
			last := item.LastTimestamp
			if item.Series.LastObservedTime.After(last) {
				last = item.Series.LastObservedTime
			}
			if last.IsZero() {
				last = first
			}
			if last.Before(first) {
				first = last
			}
			count := item.Count
			if item.Series.Count > count {
				count = item.Series.Count
			}
			if count < 1 {
				count = 1
			}
			source := item.ReportingComponent
			if source == "" {
				source = item.Source.Component
			}
			message := monitoringEventBearer.ReplaceAllString(item.Message, "Bearer [REDACTED]")
			message = monitoringEventURLCredentials.ReplaceAllString(message, "${1}[REDACTED]@")
			message = logredact.RedactText(message, "token", "api_key", "authorization", "secret", "credential")
			runes := []rune(message)
			if len(runes) > 4096 {
				message = string(runes[:4096]) + "…"
			}
			pod, uid := "", ""
			if item.InvolvedObject.Kind == "Pod" {
				pod, uid = item.InvolvedObject.Name, item.InvolvedObject.UID
			}
			out = append(out, OpsKubernetesEvent{ObjectKind: item.InvolvedObject.Kind, ObjectName: item.InvolvedObject.Name, ObjectUID: item.InvolvedObject.UID, Cluster: cluster, Namespace: item.Metadata.Namespace, EventUID: item.Metadata.UID, Pod: pod, PodUID: uid, Reason: truncateString(item.Reason, 253), Type: truncateString(item.Type, 32), Message: message, Source: truncateString(source, 253), FirstSeenAt: first.UTC(), LastSeenAt: last.UTC(), Count: count})
		}
		continuation = list.Metadata.Continue
		if continuation == "" {
			return out, nil
		}
	}
	return nil, errors.New("Kubernetes event page limit exceeded")
}
