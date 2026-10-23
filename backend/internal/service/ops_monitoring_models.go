package service

import "time"

type OpsMonitoringState string

const (
	OpsMonitoringAvailable        OpsMonitoringState = "Available"
	OpsMonitoringUnknown          OpsMonitoringState = "Unknown"
	OpsMonitoringInsufficientData OpsMonitoringState = "InsufficientData"
	OpsMonitoringNotApplicable    OpsMonitoringState = "NotApplicable"
)

type OpsMonitoringMetric struct {
	ID             string `json:"id"`
	Unit           string `json:"unit"`
	KubernetesOnly bool   `json:"kubernetes_only"`
}

type OpsMonitoringCapabilities struct {
	Sources                []string              `json:"sources"`
	ExternalIngressAdapter string                `json:"external_ingress_adapter,omitempty"`
	KubernetesResources    bool                  `json:"kubernetes_resources"`
	Mode                   string                `json:"mode"`
	PrometheusConfigured   bool                  `json:"prometheus_configured"`
	Kubernetes             bool                  `json:"kubernetes"`
	History                bool                  `json:"history"`
	MaxRangeSeconds        int64                 `json:"max_range_seconds"`
	MaxPointsPerSeries     int                   `json:"max_points_per_series"`
	Metrics                []OpsMonitoringMetric `json:"metrics"`
	KubernetesEvents       bool                  `json:"kubernetes_events"`
	ClusterName            string                `json:"cluster_name"`
}

type OpsMonitoringQuery struct {
	Metric      string
	Cluster     string
	Namespace   string
	Service     string
	Deployment  string
	Pod         string
	UID         string
	Source      string
	Protocol    string
	Route       string
	StatusCode  string
	Window      string
	Start       time.Time
	End         time.Time
	StepSeconds int
	MinRequests int64
	Exclude429  bool
	// Shared across value/count queries so instant samples align exactly.
	evaluationTime  time.Time
	externalAdapter string
	effectiveLimits bool
	externalJob     string
}

type OpsMonitoringPoint struct {
	Timestamp   time.Time          `json:"timestamp"`
	Value       *float64           `json:"value"`
	State       OpsMonitoringState `json:"state"`
	SampleCount *float64           `json:"sample_count,omitempty"`
}

type OpsMonitoringSeries struct {
	Labels map[string]string    `json:"labels"`
	Points []OpsMonitoringPoint `json:"points"`
}

type OpsMonitoringResult struct {
	Metric      string                `json:"metric"`
	Unit        string                `json:"unit"`
	State       OpsMonitoringState    `json:"state"`
	Reason      string                `json:"reason,omitempty"`
	StepSeconds int                   `json:"step_seconds"`
	Series      []OpsMonitoringSeries `json:"series"`
}

var opsMonitoringMetrics = []OpsMonitoringMetric{
	{"qps", "requests/s", false}, {"rpm", "requests/min", false}, {"request_count", "requests", false},
	{"qps_started", "requests/s", false}, {"rpm_started", "requests/min", false}, {"request_count_started", "requests", false},
	{"db_pool_wait_count", "waits", false}, {"db_pool_wait_duration_ms", "ms", false}, {"redis_pool_timeout_count", "timeouts", false},
	{"db_instance_connections", "connections", false}, {"db_instance_limit", "connections", false}, {"db_instance_usage_percent", "percent", false},
	{"redis_instance_connections", "connections", false}, {"redis_instance_limit", "connections", false}, {"redis_instance_usage_percent", "percent", false},
	{"http_p95_ms", "ms", false}, {"http_p99_ms", "ms", false}, {"ttft_p95_ms", "ms", false}, {"ttft_p99_ms", "ms", false},
	{"http_4xx_count", "requests", false}, {"http_5xx_count", "requests", false}, {"http_429_count", "requests", false},
	{"http_4xx_rate", "percent", false}, {"http_5xx_rate", "percent", false},
	{"pod_cpu_cores", "cores", true}, {"pod_cpu_limit_cores", "cores", true}, {"pod_cpu_usage_percent", "percent", true},
	{"cluster_cpu_usage_percent", "percent", true},
	{"pod_memory_bytes", "bytes", true}, {"pod_memory_limit_bytes", "bytes", true}, {"pod_memory_usage_percent", "percent", true},
	{"pod_info", "info", true}, {"pod_ready", "boolean", true}, {"pod_restarts", "restarts", true}, {"pod_oom", "containers", true},
	{"pod_oom_count", "events", true},
	{"pod_pending", "boolean", true}, {"pod_unschedulable", "boolean", true}, {"pod_crash_loop", "containers", true},
	{"pod_cpu_throttling_percent", "percent", true},
	{"hpa_current_replicas", "replicas", true}, {"hpa_desired_replicas", "replicas", true}, {"hpa_max_replicas", "replicas", true}, {"hpa_at_max", "boolean", true},
	{"deployment_cpu_usage_percent", "percent", true}, {"deployment_memory_usage_percent", "percent", true}, {"deployment_restarts", "restarts", true}, {"deployment_oom", "containers", true}, {"deployment_oom_count", "events", true},
	{"hpa_target_cpu_utilization_percent", "percent", true}, {"hpa_target_memory_utilization_percent", "percent", true},
	{"deployment_desired_replicas", "replicas", true}, {"deployment_available_replicas", "replicas", true}, {"deployment_ready_replicas", "replicas", true},
	{"scrape_up", "boolean", false},
}
