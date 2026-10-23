package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// OpsMonitoringPodLimit is a current, authoritative API observation, not a
// time-series store. An absent observation is Unknown; Bounded=false is an
// explicit unlimited limit and is rendered NotApplicable by the query provider.
type OpsMonitoringPodLimit struct {
	Cluster, Namespace, Pod, UID, Resource string
	Limit                                  *float64
	Bounded                                bool
	ObservedAt                             time.Time
}

type OpsMonitoringLimitsRepository interface {
	ReplaceMonitoringPodLimits(context.Context, string, string, []OpsMonitoringPodLimit) error
	ListMonitoringPodLimits(context.Context, string, time.Time) ([]OpsMonitoringPodLimit, error)
}

type monitoringResourceRequirements struct {
	Limits map[string]string `json:"limits"`
}
type monitoringResourceContainer struct {
	Name          string                         `json:"name"`
	RestartPolicy string                         `json:"restartPolicy"`
	Resources     monitoringResourceRequirements `json:"resources"`
}
type monitoringResourceStatus struct {
	Name      string                          `json:"name"`
	Resources *monitoringResourceRequirements `json:"resources"`
}
type monitoringResourcePod struct {
	Metadata struct{ Name, Namespace, UID string } `json:"metadata"`
	Spec     struct {
		Containers     []monitoringResourceContainer   `json:"containers"`
		InitContainers []monitoringResourceContainer   `json:"initContainers"`
		Resources      *monitoringResourceRequirements `json:"resources"`
		Overhead       map[string]string               `json:"overhead"`
	} `json:"spec"`
	Status struct {
		Resources             *monitoringResourceRequirements `json:"resources"`
		ContainerStatuses     []monitoringResourceStatus      `json:"containerStatuses"`
		InitContainerStatuses []monitoringResourceStatus      `json:"initContainerStatuses"`
		Resize                string                          `json:"resize"`
		Conditions            []struct{ Type, Status string } `json:"conditions"`
	} `json:"status"`
}

var monitoringQuantityPattern = regexp.MustCompile(`^([+]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+))([eE][+-]?[0-9]+|[numkKMGTPE]|[KMGTPE]i)?$`)

// Kubernetes quantity grammar; this conversion is exclusively for display and
// Prometheus gauges. No billing or scheduling arithmetic uses these floats.
func monitoringResourceQuantity(raw string) (float64, error) {
	parts := monitoringQuantityPattern.FindStringSubmatch(raw)
	if parts == nil {
		return 0, errors.New("invalid Kubernetes resource quantity")
	}
	value, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0, err
	}
	suffix := parts[2]
	scales := map[string]float64{"": 1, "n": 1e-9, "u": 1e-6, "m": 1e-3, "k": 1e3, "K": 1e3, "M": 1e6, "G": 1e9, "T": 1e12, "P": 1e15, "E": 1e18, "Ki": 1 << 10, "Mi": 1 << 20, "Gi": 1 << 30, "Ti": 1 << 40, "Pi": 1 << 50, "Ei": 1 << 60}
	scale, ok := scales[suffix]
	if !ok && len(suffix) > 1 && (suffix[0] == 'e' || suffix[0] == 'E') {
		exponent, e := strconv.Atoi(suffix[1:])
		if e != nil || exponent < -100 || exponent > 100 {
			return 0, errors.New("resource exponent out of range")
		}
		scale = math.Pow10(exponent)
		ok = true
	}
	value *= scale
	if !ok || math.IsInf(value, 0) || math.IsNaN(value) || value < 0 {
		return 0, errors.New("invalid resource magnitude")
	}
	return value, nil
}

// Follows Kubernetes component-helpers/resource PodLimits: concurrent regular
// containers + restartable init sidecars, the maximum sequential init stage,
// Pod-level override, then overhead. Actuated status limits take precedence
// during in-place resize; unresolved resize without actuated data is Unknown.
func effectiveMonitoringPodLimit(p monitoringResourcePod, resource string) (*float64, bool, error) {
	resizing := p.Status.Resize != ""
	for _, condition := range p.Status.Conditions {
		if (condition.Type == "PodResizePending" || condition.Type == "PodResizeInProgress") && condition.Status == "True" {
			resizing = true
		}
	}
	read := func(limits map[string]string) (float64, bool, error) {
		raw, ok := limits[resource]
		if !ok {
			return 0, false, nil
		}
		v, err := monitoringResourceQuantity(raw)
		return v, v > 0, err
	}
	if p.Status.Resources != nil {
		// Pod status resources are the actuated pod cgroup budget, already including overhead.
		if _, ok := p.Status.Resources.Limits[resource]; ok {
			v, b, e := read(p.Status.Resources.Limits)
			if e != nil || !b {
				return nil, b, e
			}
			return &v, true, nil
		}
	}
	if p.Spec.Resources != nil {
		if _, ok := p.Spec.Resources.Limits[resource]; ok {
			if resizing {
				return nil, false, errors.New("pod resource resize has no actuated limits")
			}
			v, b, e := read(p.Spec.Resources.Limits)
			if e != nil || !b {
				return nil, b, e
			}
			if raw := p.Spec.Overhead[resource]; raw != "" {
				over, e := monitoringResourceQuantity(raw)
				if e != nil {
					return nil, false, e
				}
				v += over
			}
			return &v, true, nil
		}
	}
	status := map[string]*monitoringResourceRequirements{}
	for _, cs := range append(append([]monitoringResourceStatus{}, p.Status.ContainerStatuses...), p.Status.InitContainerStatuses...) {
		status[cs.Name] = cs.Resources
	}
	containerLimit := func(c monitoringResourceContainer) (float64, bool, error) {
		limits := c.Resources.Limits
		if applied := status[c.Name]; applied != nil {
			limits = applied.Limits
		} else if resizing {
			return 0, false, errors.New("container resize has no actuated limits")
		}
		return read(limits)
	}
	regular, sidecars, maxInit := 0.0, 0.0, 0.0
	bounded := len(p.Spec.Containers) > 0
	for _, c := range p.Spec.Containers {
		v, b, e := containerLimit(c)
		if e != nil {
			return nil, false, e
		}
		regular += v
		bounded = bounded && b
	}
	for _, c := range p.Spec.InitContainers {
		v, b, e := containerLimit(c)
		if e != nil {
			return nil, false, e
		}
		bounded = bounded && b
		if c.RestartPolicy == "Always" {
			sidecars += v
			maxInit = math.Max(maxInit, sidecars)
		} else {
			maxInit = math.Max(maxInit, sidecars+v)
		}
	}
	if !bounded {
		return nil, false, nil
	}
	value := math.Max(regular+sidecars, maxInit)
	if raw := p.Spec.Overhead[resource]; raw != "" {
		over, e := monitoringResourceQuantity(raw)
		if e != nil {
			return nil, false, e
		}
		value += over
	}
	if value <= 0 {
		return nil, false, nil
	}
	return &value, true, nil
}

func fetchMonitoringPodLimits(ctx context.Context, client *http.Client, endpoint, token, cluster string) ([]OpsMonitoringPodLimit, error) {
	out := []OpsMonitoringPodLimit{}
	continuation := ""
	at := time.Now().UTC()
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
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024+1))
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || readErr != nil || len(body) > 8*1024*1024 {
			return nil, errors.New("Kubernetes resource response unavailable")
		}
		var list struct {
			Metadata struct {
				Continue string `json:"continue"`
			} `json:"metadata"`
			Items []monitoringResourcePod `json:"items"`
		}
		if json.Unmarshal(body, &list) != nil || len(list.Items) > 500 {
			return nil, errors.New("invalid Kubernetes resource page")
		}
		for _, p := range list.Items {
			if p.Metadata.Name == "" || p.Metadata.Namespace == "" || p.Metadata.UID == "" {
				return nil, errors.New("missing Kubernetes resource identity")
			}
			for _, resource := range []string{"cpu", "memory"} {
				value, bounded, err := effectiveMonitoringPodLimit(p, resource)
				// An unresolved resize/unknown quantity does not invalidate unrelated Pods,
				// but no row is emitted for this resource: it stays Unknown, never N/A.
				if err != nil {
					continue
				}
				out = append(out, OpsMonitoringPodLimit{Cluster: cluster, Namespace: p.Metadata.Namespace, Pod: p.Metadata.Name, UID: p.Metadata.UID, Resource: resource, Limit: value, Bounded: bounded, ObservedAt: at})
			}
		}
		continuation = list.Metadata.Continue
		if continuation == "" {
			return out, nil
		}
	}
	return nil, errors.New("Kubernetes resource page limit exceeded")
}

func monitoringLimitsJobName(c config.OpsMonitoringConfig) string {
	return strings.Replace(monitoringEventsJobName(c), "events", "limits", 1)
}

func (c *OpsMetricsCollector) collectMonitoringPodLimits() {
	if c == nil || c.cfg == nil || c.opsRepo == nil {
		return
	}
	cfg := (&OpsService{cfg: c.cfg}).monitoringConfig()
	if cfg.Mode != "kubernetes" || !cfg.KubernetesResourcesEnabled {
		return
	}
	repo, ok := c.opsRepo.(OpsMonitoringLimitsRepository)
	if !ok {
		return
	}
	started := time.Now().UTC()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client, base, token, close, err := monitoringKubernetesAPI(cfg)
	if err == nil {
		defer close()
		var rows []OpsMonitoringPodLimit
		rows, err = fetchMonitoringPodLimits(ctx, client, base+"/pods", token, cfg.ClusterName)
		if err == nil {
			scope := cfg.EventsNamespace
			if scope == "" {
				scope = "*"
				if at := strings.LastIndex(base, "/namespaces/"); at >= 0 {
					scope, _ = url.PathUnescape(base[at+len("/namespaces/"):])
				}
			}
			err = repo.ReplaceMonitoringPodLimits(ctx, cfg.ClusterName, scope, rows)
		}
	}
	finished := time.Now().UTC()
	duration := finished.Sub(started).Milliseconds()
	hb := &OpsUpsertJobHeartbeatInput{JobName: monitoringLimitsJobName(cfg), LastRunAt: &started, LastDurationMs: &duration}
	if err != nil {
		message := "Kubernetes resource collection failed; verify pod read permissions and API connectivity"
		hb.LastErrorAt = &finished
		hb.LastError = &message
	} else {
		hb.LastSuccessAt = &finished
	}
	hbctx, stop := context.WithTimeout(context.Background(), 3*time.Second)
	defer stop()
	_ = c.opsRepo.UpsertJobHeartbeat(hbctx, hb)
}

// MonitoringPodLimitMetrics reads only the collector's shared DB snapshot;
// scraping never performs Kubernetes API calls or computes pod specifications.
func (s *OpsService) MonitoringPodLimitMetrics(ctx context.Context) string {
	if s == nil || s.opsRepo == nil {
		return ""
	}
	cfg := s.monitoringConfig()
	if cfg.Mode != "kubernetes" || !cfg.KubernetesResourcesEnabled {
		return ""
	}
	repo, ok := s.opsRepo.(OpsMonitoringLimitsRepository)
	if !ok {
		return ""
	}
	hbs, err := s.opsRepo.ListJobHeartbeats(ctx)
	if err != nil {
		return ""
	}
	healthy := false
	for _, hb := range hbs {
		if hb != nil && hb.JobName == monitoringLimitsJobName(cfg) && hb.LastSuccessAt != nil && time.Since(*hb.LastSuccessAt) <= 2*time.Minute && (hb.LastErrorAt == nil || !hb.LastErrorAt.After(*hb.LastSuccessAt)) {
			healthy = true
		}
	}
	if !healthy {
		return ""
	}
	rows, err := repo.ListMonitoringPodLimits(ctx, cfg.ClusterName, time.Now().Add(-2*time.Minute))
	if err != nil {
		return ""
	}
	var out strings.Builder
	for _, row := range rows {
		labels := fmt.Sprintf("cluster=%q,namespace=%q,pod=%q,uid=%q,resource=%q", row.Cluster, row.Namespace, row.Pod, row.UID, row.Resource)
		bounded := 0
		if row.Bounded {
			bounded = 1
		}
		fmt.Fprintf(&out, "modurelay_kubernetes_pod_limit_bounded{%s} %d\nmodurelay_kubernetes_pod_limit_observed_timestamp_seconds{%s} %d\n", labels, bounded, labels, row.ObservedAt.Unix())
		if row.Bounded && row.Limit != nil {
			fmt.Fprintf(&out, "modurelay_kubernetes_pod_effective_limit{%s} %.12g\n", labels, *row.Limit)
		}
	}
	return out.String()
}
