package service

import (
	"fmt"
	"strconv"
	"strings"
)

const monitoringHTTPGroup = "cluster,namespace,service,source"
const monitoringPodGroup = "cluster,namespace,pod,uid"

func monitoringHTTPGroupFor(q OpsMonitoringQuery) string {
	if q.Pod != "" || q.UID != "" {
		return monitoringHTTPGroup + ",pod,uid"
	}
	return monitoringHTTPGroup
}

// Selectors are built exclusively from quoted exact-match labels. The public
// API never accepts PromQL, a metric name, a regex, or a data-source URL.
func monitoringSelector(q OpsMonitoringQuery, kind string, extra ...string) string {
	labels := []struct{ k, v string }{{"cluster", q.Cluster}, {"namespace", q.Namespace}}
	switch kind {
	case "http":
		labels = append(labels, struct{ k, v string }{"service", q.Service}, struct{ k, v string }{"pod", q.Pod}, struct{ k, v string }{"uid", q.UID}, struct{ k, v string }{"source", q.Source}, struct{ k, v string }{"protocol", q.Protocol}, struct{ k, v string }{"route", q.Route}, struct{ k, v string }{"status_code", q.StatusCode})
	case "pod":
		labels = append(labels, struct{ k, v string }{"pod", q.Pod})
	case "identity":
		labels = append(labels, struct{ k, v string }{"pod", q.Pod}, struct{ k, v string }{"uid", q.UID})
	case "deployment":
		labels = append(labels, struct{ k, v string }{"deployment", q.Deployment})
	}
	parts := append([]string{}, extra...)
	for _, label := range labels {
		if label.v != "" {
			parts = append(parts, label.k+"="+strconv.Quote(label.v))
		}
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// Monitoring targets scrape every 15 seconds. A sample older than 60 seconds
// is missing, even when Prometheus still returns it inside its lookback window.
func monitoringFresh(raw string) string {
	return "(" + raw + ") and (time() - timestamp(" + raw + ") <= 60)"
}

func monitoringFreshRate(raw, fn, window string) string {
	return fn + "(" + raw + "[" + window + "]) and (time() - timestamp(" + raw + ") <= 60)"
}

func monitoringScrapeSelector(q OpsMonitoringQuery) string {
	extra := []string{`job="modurelay-instance"`}
	if q.Service != "" {
		extra = append(extra, "service="+strconv.Quote(q.Service))
	}
	return monitoringSelector(q, "identity", extra...)
}

func monitoringScrapeHealth(q OpsMonitoringQuery) string {
	raw := "up" + monitoringScrapeSelector(q)
	selected := raw
	age := "time() - timestamp(" + raw + ")"
	if q.Deployment != "" {
		membership := " and on (cluster,namespace,pod) (" + monitoringDeploymentPods(q) + ")"
		selected = "(" + selected + ")" + membership
		age = "(" + age + ")" + membership
	}
	// Gate the whole selected Service when any expected target fails or goes
	// stale. Do not present partial replica traffic as a healthy Service total.
	return "(min by (cluster,namespace,service) (" + selected + ") == 1) and on (cluster,namespace,service) (max by (cluster,namespace,service) (" + age + ") <= 60)"
}

func monitoringInstanceCapacity(q OpsMonitoringQuery) string {
	prefix := "modurelay_database_instance_"
	if strings.HasPrefix(q.Metric, "redis_") {
		prefix = "modurelay_redis_instance_"
	}
	selector := monitoringSelector(q, "base")
	value := func(name string) string { return "max by (cluster) (" + monitoringFresh(prefix+name+selector) + ")" }
	expr := value("connections")
	if strings.HasSuffix(q.Metric, "_limit") {
		expr = value("max_connections")
	}
	if strings.HasSuffix(q.Metric, "_usage_percent") {
		expr = "100 * " + value("connections") + " / (" + value("max_connections") + " > 0)"
	}
	up := "up" + monitoringSelector(q, "base", `job="modurelay-cluster"`)
	health := "(min by (cluster) (" + up + ") == 1) and on (cluster) (max by (cluster) (time() - timestamp(" + up + ")) <= 60)"
	return "(" + expr + ") and on (cluster) (" + health + ")"
}

func monitoringHTTPIncrement(q OpsMonitoringQuery, metric, fn, window string, extra ...string) string {
	labels := "cluster,namespace,service,pod,uid,instance,source,method,route,protocol"
	started := metric == "modurelay_http_requests_started_total"
	if started {
		q.Source = ""
	}
	if metric == "modurelay_http_requests_total" {
		labels += ",status_code"
	}
	if strings.HasSuffix(metric, "_bucket") {
		labels += ",le"
	}
	expr := "max by (" + labels + ") (" + monitoringFreshRate(metric+monitoringSelector(q, "http", extra...), fn, window) + ")"
	if q.Deployment != "" {
		expr += " and on (cluster,namespace,pod) (" + monitoringDeploymentPods(q) + ")"
	}
	group := monitoringHTTPGroupFor(q)
	if strings.HasSuffix(metric, "_bucket") {
		group += ",le"
	}
	value := "(sum by (" + group + ") (" + expr + ")) and on (cluster,namespace,service) (" + monitoringScrapeHealth(q) + ")"
	if started {
		value = `label_replace((` + value + `),"source","app","service",".*")`
	}
	return value
}

func monitoringDeploymentPods(q OpsMonitoringQuery) string {
	owners := `label_replace(max by (cluster,namespace,pod,owner_name) (` + monitoringFresh("kube_pod_owner"+monitoringSelector(q, "pod", `owner_kind="ReplicaSet"`)) + `),"replicaset","$1","owner_name","(.*)")`
	deployments := `max by (cluster,namespace,replicaset) (` + monitoringFresh("kube_replicaset_owner"+monitoringSelector(q, "base", `owner_kind="Deployment"`, "owner_name="+strconv.Quote(q.Deployment))) + `)`
	return "max by (cluster,namespace,pod) (" + owners + " * on (cluster,namespace,replicaset) group_left () " + deployments + ")"
}

func monitoringPodFilter(q OpsMonitoringQuery, expr string) string {
	if q.Service != "" {
		s := monitoringSelector(q, "pod", "service="+strconv.Quote(q.Service))
		expr = "(" + expr + ") and on (cluster,namespace,pod) (max by (cluster,namespace,pod) (" + monitoringFresh("modurelay_instance_draining"+s) + "))"
	}
	if q.Deployment != "" {
		expr = "(" + expr + ") and on (cluster,namespace,pod) (" + monitoringDeploymentPods(q) + ")"
	}
	return expr
}

func monitoringPodIdentity(q OpsMonitoringQuery) string {
	return "max by (" + monitoringPodGroup + ") (" + monitoringFresh("kube_pod_info"+monitoringSelector(q, "identity")) + ")"
}

func monitoringCurrentContainers(q OpsMonitoringQuery, expr string) string {
	selector := monitoringSelector(q, "pod", `container!=""`, `container!="POD"`)
	created := "max by (cluster,namespace,pod) (" + monitoringFresh("kube_pod_created"+monitoringSelector(q, "identity")) + ")"
	starts := monitoringFresh("container_start_time_seconds" + selector)
	return "(" + expr + ") and on (cluster,namespace,pod,container,id) ((" + starts + ") >= on (cluster,namespace,pod) group_left () (" + created + "))"
}

func monitoringMaturePodWindow(q OpsMonitoringQuery, expr string) string {
	window, _ := monitoringWindowSeconds(q.Window)
	return "(" + expr + ") and on (" + monitoringPodGroup + ") ((time() - max by (" + monitoringPodGroup + ") (" + monitoringFresh("kube_pod_created"+monitoringSelector(q, "identity")) + ")) >= " + strconv.Itoa(window) + ")"
}

func monitoringPodUsage(q OpsMonitoringQuery, memory bool) string {
	selector := monitoringSelector(q, "pod", `container!=""`, `container!="POD"`)
	metric := monitoringFreshRate("container_cpu_usage_seconds_total"+selector, "rate", q.Window)
	if memory {
		metric = monitoringFresh("container_memory_working_set_bytes" + selector)
	}
	// Exclude previous cgroups left behind by same-name Pod replacement.
	metric = monitoringCurrentContainers(q, metric)
	expr := "sum by (cluster,namespace,pod) (max by (cluster,namespace,pod,container) (" + metric + ")) * on (cluster,namespace,pod) group_left (uid) (" + monitoringPodIdentity(q) + ")"
	if !memory {
		// A rate window crossing a same-name Pod replacement cannot be assigned
		// to the new UID. Wait for one complete window after creation.
		expr = monitoringMaturePodWindow(q, expr)
	}
	return expr
}

func monitoringPodLimit(q OpsMonitoringQuery, memory bool) string {
	if q.effectiveLimits {
		return monitoringEffectivePodLimit(q, memory)
	}
	resource, unit := "cpu", "core"
	if memory {
		resource, unit = "memory", "byte"
	}
	// An unlimited sidecar makes the Pod percentage undefined. Never divide
	// aggregate usage by only the subset of containers that have a limit.
	containers := "max by (" + monitoringPodGroup + ",container) (" + monitoringFresh("kube_pod_container_info"+monitoringSelector(q, "identity")) + ")"
	limits := "max by (" + monitoringPodGroup + ",container) (" + monitoringFresh("kube_pod_container_resource_limits"+monitoringSelector(q, "identity", "resource="+strconv.Quote(resource), "unit="+strconv.Quote(unit))) + ") > 0"
	complete := "count by (" + monitoringPodGroup + ") (" + limits + ") == count by (" + monitoringPodGroup + ") (" + containers + ")"
	fallback := "sum by (" + monitoringPodGroup + ") (" + limits + ") and on (" + monitoringPodGroup + ") (" + complete + ")"
	// Scheduler /metrics/resources accounts for init/sidecar lifecycle and Pod
	// overhead. Its unit labels are plural, unlike kube-state-metrics. Require
	// complete regular-container limits to avoid treating an unlimited sidecar
	// as bounded. Pod-level-only limits require a separate effective-limit feed.
	schedulerUnit := "cores"
	if memory {
		schedulerUnit = "bytes"
	}
	scheduler := "max by (cluster,namespace,pod) (" + monitoringFresh("kube_pod_resource_limit"+monitoringSelector(q, "pod", "resource="+strconv.Quote(resource), "unit="+strconv.Quote(schedulerUnit))) + ") > 0"
	scheduler = "(" + scheduler + ") * on (cluster,namespace,pod) group_left (uid) (" + monitoringPodIdentity(q) + ")"
	scheduler = "(" + scheduler + ") and on (" + monitoringPodGroup + ") (" + complete + ")"
	label := func(expr, source string) string {
		return `label_replace((` + expr + `),"limit_source","` + source + `","pod",".*")`
	}
	return "(" + label(scheduler, "kube_scheduler") + ") or on (" + monitoringPodGroup + ") (" + label(fallback, "regular_container_sum") + ")"
}

// monitoringPromQL returns a fixed expression and, for sample-sensitive values,
// the matching sample-count expression used to mask insufficient data.
func monitoringPromQL(q OpsMonitoringQuery) (string, string, error) {
	if q.Source == "external_ingress" {
		return monitoringExternalIngressPromQL(q)
	}
	countQuery := q
	if strings.HasPrefix(q.Metric, "http_") && strings.HasSuffix(q.Metric, "_rate") {
		countQuery.StatusCode = ""
	}
	count := monitoringHTTPIncrement(countQuery, "modurelay_http_requests_total", "increase", q.Window)
	switch q.Metric {
	case "qps":
		return monitoringHTTPIncrement(q, "modurelay_http_requests_total", "rate", q.Window), "", nil
	case "rpm":
		return monitoringHTTPIncrement(q, "modurelay_http_requests_total", "increase", "1m"), "", nil
	case "request_count":
		return count, "", nil
	case "db_instance_connections", "db_instance_limit", "db_instance_usage_percent", "redis_instance_connections", "redis_instance_limit", "redis_instance_usage_percent":
		return monitoringInstanceCapacity(q), "", nil
	case "request_count_started":
		return monitoringHTTPIncrement(q, "modurelay_http_requests_started_total", "increase", q.Window), "", nil
	case "qps_started":
		return monitoringHTTPIncrement(q, "modurelay_http_requests_started_total", "rate", q.Window), "", nil
	case "rpm_started":
		return monitoringHTTPIncrement(q, "modurelay_http_requests_started_total", "increase", "1m"), "", nil
	case "db_pool_wait_count", "db_pool_wait_duration_ms", "redis_pool_timeout_count":
		metric := map[string]string{"db_pool_wait_count": "modurelay_db_pool_wait_total", "db_pool_wait_duration_ms": "modurelay_db_pool_wait_duration_seconds_total", "redis_pool_timeout_count": "modurelay_redis_pool_timeout_total"}[q.Metric]
		q.Source = ""
		identity := "cluster,namespace,service,pod,uid,instance"
		inner := "max by (" + identity + ") (" + monitoringFreshRate(metric+monitoringSelector(q, "http"), "increase", q.Window) + ")"
		if q.Metric != "redis_pool_timeout_count" {
			family := "modurelay_database_pool_wait_total"
			if q.Metric == "db_pool_wait_duration_ms" {
				family = "modurelay_database_pool_wait_duration_seconds_total"
			}
			pools := "sum by (" + identity + ") (max by (" + identity + ",pool) (" + monitoringFreshRate(family+monitoringSelector(q, "http"), "increase", q.Window) + "))"
			inner = "(" + pools + ") or on (" + identity + ") (" + inner + ")"
		}
		if q.Deployment != "" {
			inner = "(" + inner + ") and on (cluster,namespace,pod) (" + monitoringDeploymentPods(q) + ")"
		}
		value := "sum by (cluster,namespace,service) (" + inner + ")"
		value = "(" + value + ") and on (cluster,namespace,service) (" + monitoringScrapeHealth(q) + ")"
		if q.Metric == "db_pool_wait_duration_ms" {
			value = "1000 * (" + value + ")"
		}
		return value, "", nil
	case "http_p95_ms", "http_p99_ms", "ttft_p95_ms", "ttft_p99_ms":
		metric := "modurelay_http_request_duration_seconds"
		if strings.HasPrefix(q.Metric, "ttft_") {
			metric = "modurelay_http_ttft_seconds"
		}
		quantile := "0.95"
		if strings.Contains(q.Metric, "p99") {
			quantile = "0.99"
		}
		return "1000 * histogram_quantile(" + quantile + ", " + monitoringHTTPIncrement(q, metric+"_bucket", "rate", q.Window) + ")", monitoringHTTPIncrement(q, metric+"_count", "increase", q.Window), nil
	case "http_4xx_count", "http_5xx_count", "http_429_count", "http_4xx_rate", "http_5xx_rate":
		extra := []string{`status_code=~"4.."`}
		if strings.Contains(q.Metric, "5xx") {
			extra = []string{`status_code=~"5.."`}
		}
		if strings.Contains(q.Metric, "429") {
			extra = []string{`status_code="429"`}
		} else if q.Exclude429 {
			extra = append(extra, `status_code!="429"`)
		}
		errors := "(" + monitoringHTTPIncrement(q, "modurelay_http_requests_total", "increase", q.Window, extra...) + " or on (" + monitoringHTTPGroupFor(q) + ") (0 * " + count + "))"
		if strings.HasSuffix(q.Metric, "_rate") {
			return "100 * " + errors + " / (" + count + " > 0)", count, nil
		}
		return errors, "", nil
	case "pod_cpu_cores", "pod_cpu_limit_cores", "pod_cpu_usage_percent", "pod_memory_bytes", "pod_memory_limit_bytes", "pod_memory_usage_percent":
		memory := strings.HasPrefix(q.Metric, "pod_memory")
		value := monitoringPodUsage(q, memory)
		if strings.Contains(q.Metric, "_limit_") {
			value = monitoringPodLimit(q, memory)
		}
		if strings.HasSuffix(q.Metric, "_percent") {
			value = "100 * (" + value + ") / on (" + monitoringPodGroup + ") group_left (limit_source) (" + monitoringPodLimit(q, memory) + ")"
		}
		if q.effectiveLimits && (strings.Contains(q.Metric, "_limit_") || strings.HasSuffix(q.Metric, "_percent")) {
			value = "(" + value + ") or (" + monitoringUnlimitedPodLimit(q, memory) + ")"
		}
		return monitoringPodFilter(q, value), "", nil
	case "cluster_cpu_usage_percent":
		usage := monitoringPodFilter(q, monitoringPodUsage(q, false))
		limit := monitoringPodFilter(q, monitoringPodLimit(q, false))
		// Numerator and denominator contain the exact same UID set. Missing or
		// unlimited pods are excluded from both, and the subset is explicit.
		value := "100 * sum by (cluster) ((" + usage + ") and on (" + monitoringPodGroup + ") (" + limit + ")) / sum by (cluster) ((" + limit + ") and on (" + monitoringPodGroup + ") (" + usage + "))"
		return `label_replace((` + value + `),"aggregation","pods_with_usage_and_limit","cluster",".*")`, "", nil
	case "pod_info":
		return monitoringPodFilter(q, monitoringPodIdentity(q)), "", nil
	case "pod_ready", "pod_pending", "pod_unschedulable", "pod_crash_loop", "pod_restarts", "pod_oom":
		metric, extra := "kube_pod_status_ready", []string{`condition="true"`}
		switch q.Metric {
		case "pod_pending":
			metric, extra = "kube_pod_status_phase", []string{`phase="Pending"`}
		case "pod_unschedulable":
			metric, extra = "kube_pod_status_unschedulable", nil
		case "pod_crash_loop":
			metric, extra = "kube_pod_container_status_waiting_reason", []string{`reason="CrashLoopBackOff"`}
		case "pod_restarts":
			metric, extra = "kube_pod_container_status_restarts_total", nil
		case "pod_oom":
			metric, extra = "kube_pod_container_status_last_terminated_reason", []string{`reason="OOMKilled"`}
		}
		raw := metric + monitoringSelector(q, "identity", extra...)
		value := monitoringFresh(raw)
		if q.Metric == "pod_crash_loop" {
			// Restartable init sidecars can crash while all regular containers
			// remain running. Include their distinct KSM status family.
			value = "(" + value + ") or (" + monitoringFresh("kube_pod_init_container_status_waiting_reason"+monitoringSelector(q, "identity", extra...)) + ")"
		}
		if q.Metric == "pod_restarts" {
			value = monitoringFreshRate(raw, "increase", q.Window)
		}
		value = "sum by (" + monitoringPodGroup + ") (max by (" + monitoringPodGroup + ",container) (" + value + "))"
		if q.Metric == "pod_crash_loop" {
			value = "(" + value + ") or on (" + monitoringPodGroup + ") (" + monitoringRunningPodZero(q) + ")"
		}
		return monitoringPodFilter(q, value), "", nil
	case "pod_oom_count":
		raw := "container_oom_events_total" + monitoringSelector(q, "pod", `container!=""`, `container!="POD"`)
		value := monitoringCurrentContainers(q, monitoringFreshRate(raw, "increase", q.Window))
		// A restart creates another cgroup id within the same Pod UID. Sum
		// their event increases after exporter deduplication, not max across
		// incarnations of the same container name.
		value = "sum by (cluster,namespace,pod) (max by (cluster,namespace,pod,container,id) (" + value + ")) * on (cluster,namespace,pod) group_left (uid) (" + monitoringPodIdentity(q) + ")"
		return monitoringPodFilter(q, monitoringMaturePodWindow(q, value)), "", nil
	case "pod_cpu_throttling_percent":
		makeRate := func(metric string) string {
			rate := monitoringFreshRate(metric+monitoringSelector(q, "pod", `container!=""`, `container!="POD"`), "rate", q.Window)
			return "sum by (cluster,namespace,pod) (max by (cluster,namespace,pod,container) (" + monitoringCurrentContainers(q, rate) + "))"
		}
		value := "100 * " + makeRate("container_cpu_cfs_throttled_periods_total") + " / (" + makeRate("container_cpu_cfs_periods_total") + " > 0) * on (cluster,namespace,pod) group_left (uid) (" + monitoringPodIdentity(q) + ")"
		return monitoringPodFilter(q, monitoringMaturePodWindow(q, value)), "", nil
	case "deployment_cpu_usage_percent", "deployment_memory_usage_percent", "deployment_restarts", "deployment_oom", "deployment_oom_count":
		return monitoringDeploymentAggregate(q)
	case "hpa_target_cpu_utilization_percent", "hpa_target_memory_utilization_percent":
		resource := "cpu"
		if strings.Contains(q.Metric, "memory") {
			resource = "memory"
		}
		return "max by (cluster,namespace,horizontalpodautoscaler) (" + monitoringFresh("kube_horizontalpodautoscaler_spec_target_metric"+monitoringSelector(q, "base", "metric_name="+strconv.Quote(resource), `metric_target_type="utilization"`)) + ")", "", nil
	case "hpa_current_replicas", "hpa_desired_replicas", "hpa_max_replicas", "hpa_at_max":
		makeHPA := func(suffix string) string {
			return "max by (cluster,namespace,horizontalpodautoscaler) (" + monitoringFresh("kube_horizontalpodautoscaler_"+suffix+monitoringSelector(q, "base")) + ")"
		}
		if q.Metric == "hpa_at_max" {
			return makeHPA("status_current_replicas") + " >= bool " + makeHPA("spec_max_replicas"), "", nil
		}
		suffix := map[string]string{"hpa_current_replicas": "status_current_replicas", "hpa_desired_replicas": "status_desired_replicas", "hpa_max_replicas": "spec_max_replicas"}[q.Metric]
		return makeHPA(suffix), "", nil
	case "deployment_desired_replicas", "deployment_available_replicas", "deployment_ready_replicas":
		suffix := map[string]string{"deployment_desired_replicas": "spec_replicas", "deployment_available_replicas": "status_replicas_available", "deployment_ready_replicas": "status_replicas_ready"}[q.Metric]
		return "max by (cluster,namespace,deployment) (" + monitoringFresh("kube_deployment_"+suffix+monitoringSelector(q, "deployment")) + ")", "", nil
	case "scrape_up":
		value := "max by (cluster,namespace,service,pod,uid,instance) (" + monitoringFresh("up"+monitoringScrapeSelector(q)) + ")"
		if q.Deployment != "" {
			value = "(" + value + ") and on (cluster,namespace,pod) (" + monitoringDeploymentPods(q) + ")"
		}
		return value, "", nil
	}
	return "", "", fmt.Errorf("unsupported monitoring metric")
}

// Only fresh, successfully scraped, fully running containers prove that a
// disappearing CrashLoopBackOff reason is a healthy zero. A missing exporter,
// stale identity or a replacement UID never manufactures a recovery.
func monitoringRunningPodZero(q OpsMonitoringQuery) string {
	health := "((" + monitoringFresh("up") + ") == 1)"
	live := func(metric string) string {
		return "(" + monitoringFresh(metric+monitoringSelector(q, "identity")) + ") and on (cluster,job,instance) (" + health + ")"
	}
	running := "count by (" + monitoringPodGroup + ") (max by (" + monitoringPodGroup + ",container) (" + live("kube_pod_container_status_running") + ") == 1)"
	known := "count by (" + monitoringPodGroup + ") (max by (" + monitoringPodGroup + ",container) (" + live("kube_pod_container_info") + "))"
	initKnown := "count by (" + monitoringPodGroup + ") (max by (" + monitoringPodGroup + ",container) (" + live("kube_pod_init_container_info") + "))"
	initCompleteRaw := "kube_pod_init_container_status_terminated_reason" + monitoringSelector(q, "identity", `reason="Completed"`)
	initComplete := "(" + monitoringFresh(initCompleteRaw) + ") and on (cluster,job,instance) (" + health + ")"
	initHealthy := "count by (" + monitoringPodGroup + ") (max by (" + monitoringPodGroup + ",container) ((" + live("kube_pod_init_container_status_running") + ") or (" + initComplete + ")) == 1)"
	initUnknown := "(" + initKnown + ") unless on (" + monitoringPodGroup + ") ((" + initHealthy + ") == on (" + monitoringPodGroup + ") (" + initKnown + "))"
	return "(0 * ((" + running + ") == on (" + monitoringPodGroup + ") (" + known + ")) and on (" + monitoringPodGroup + ") (" + monitoringPodIdentity(q) + ")) unless on (" + monitoringPodGroup + ") (" + initUnknown + ")"
}

func monitoringDeploymentAggregate(q OpsMonitoringQuery) (string, string, error) {
	// Keep the original Pod query contract; this explicit metric family changes
	// aggregation to Deployment. Join the owner chain instead of guessing names.
	owners := `label_replace(max by (cluster,namespace,pod,owner_name) (` + monitoringFresh("kube_pod_owner"+monitoringSelector(q, "pod", `owner_kind="ReplicaSet"`)) + `),"replicaset","$1","owner_name","(.*)")`
	extra := []string{`owner_kind="Deployment"`}
	if q.Deployment != "" {
		extra = append(extra, "owner_name="+strconv.Quote(q.Deployment))
	}
	deployments := `label_replace(max by (cluster,namespace,replicaset,owner_name) (` + monitoringFresh("kube_replicaset_owner"+monitoringSelector(q, "base", extra...)) + `),"deployment","$1","owner_name","(.*)")`
	membership := "max by (cluster,namespace,pod,deployment) (" + owners + " * on (cluster,namespace,replicaset) group_left (deployment) " + deployments + ")"
	aggregate := func(expr string) string {
		return "sum by (cluster,namespace,deployment) ((" + expr + ") * on (cluster,namespace,pod) group_left (deployment) (" + membership + "))"
	}
	if strings.HasSuffix(q.Metric, "usage_percent") {
		memory := strings.Contains(q.Metric, "memory")
		usage, limit := monitoringPodUsage(q, memory), monitoringPodLimit(q, memory)
		return "100 * " + aggregate("("+usage+") and on ("+monitoringPodGroup+") ("+limit+")") + " / " + aggregate("("+limit+") and on ("+monitoringPodGroup+") ("+usage+")"), "", nil
	}
	q.Metric = strings.Replace(q.Metric, "deployment_", "pod_", 1)
	value, _, err := monitoringPromQL(q)
	return aggregate(value), "", err
}

// External ingress is a separate observation source. Never add it to app or
// application-admission counters: the same request may be visible in both.
func monitoringExternalIngressPromQL(q OpsMonitoringQuery) (string, string, error) {
	counter, duration, status := "nginx_ingress_controller_requests", "nginx_ingress_controller_request_duration_seconds", "status"
	if q.externalAdapter == "normalized" {
		counter, duration, status = "modurelay_external_http_requests_total", "modurelay_external_http_request_duration_seconds", "status_code"
	}
	job := q.externalJob
	if job == "" {
		job = "ingress-nginx"
	}
	selector := func(code string) string {
		parts := []string{"job=" + strconv.Quote(job)}
		for _, label := range []struct{ k, v string }{{"cluster", q.Cluster}, {"namespace", q.Namespace}, {"service", q.Service}, {"route", q.Route}, {"protocol", q.Protocol}} {
			if label.v != "" {
				parts = append(parts, label.k+"="+strconv.Quote(label.v))
			}
		}
		if code != "" {
			parts = append(parts, status+"="+strconv.Quote(code))
		}
		return "{" + strings.Join(parts, ",") + "}"
	}
	healthSelector := "{job=" + strconv.Quote(job)
	if q.Cluster != "" {
		healthSelector += ",cluster=" + strconv.Quote(q.Cluster)
	}
	healthSelector += "}"
	up := "up" + healthSelector
	health := "(min by (cluster) (" + up + ") == 1) and on (cluster) (max by (cluster) (time()-timestamp(" + up + ")) <= 60)"
	increment := func(metric, fn, window, extra, code string) string {
		sel := selector(code)
		if extra != "" {
			sel = strings.TrimSuffix(sel, "}") + "," + extra + "}"
		}
		labels := "cluster,namespace,service,instance,controller_pod,ingress,host,path,method," + status + ",route,protocol"
		group := "cluster,namespace,service"
		if strings.HasSuffix(metric, "_bucket") {
			labels += ",le"
			group += ",le"
		}
		value := "sum by (" + group + ") (max by (" + labels + ") (" + monitoringFreshRate(metric+sel, fn, window) + "))"
		return `label_replace(((` + value + `) and on (cluster) (` + health + `)),"source","external_ingress","service",".*")`
	}
	count := increment(counter, "increase", q.Window, "", q.StatusCode)
	switch q.Metric {
	case "qps":
		return increment(counter, "rate", q.Window, "", q.StatusCode), "", nil
	case "rpm":
		return increment(counter, "increase", "1m", "", q.StatusCode), "", nil
	case "request_count":
		return count, "", nil
	case "http_p95_ms", "http_p99_ms":
		quantile := "0.95"
		if q.Metric == "http_p99_ms" {
			quantile = "0.99"
		}
		return "1000 * histogram_quantile(" + quantile + "," + increment(duration+"_bucket", "rate", q.Window, "", "") + ")", increment(duration+"_count", "increase", q.Window, "", ""), nil
	case "http_4xx_count", "http_5xx_count", "http_429_count", "http_4xx_rate", "http_5xx_rate":
		extra := status + `=~"4.."`
		if strings.Contains(q.Metric, "5xx") {
			extra = status + `=~"5.."`
		}
		if strings.Contains(q.Metric, "429") {
			extra = status + `="429"`
		} else if q.Exclude429 {
			extra += "," + status + `!="429"`
		}
		all := increment(counter, "increase", q.Window, "", "")
		errors := "(" + increment(counter, "increase", q.Window, extra, q.StatusCode) + " or on (cluster,namespace,service,source) (0 * " + all + "))"
		if strings.HasSuffix(q.Metric, "_rate") {
			return "100 * " + errors + " / (" + all + " > 0)", all, nil
		}
		return errors, "", nil
	}
	return "", "", fmt.Errorf("unsupported external ingress metric")
}

func monitoringEffectiveLimitSeries(q OpsMonitoringQuery, memory bool, name string) string {
	resource := "cpu"
	if memory {
		resource = "memory"
	}
	selector := monitoringSelector(q, "identity", "resource="+strconv.Quote(resource))
	observed := "max by (" + monitoringPodGroup + ") (" + monitoringFresh("modurelay_kubernetes_pod_limit_observed_timestamp_seconds"+selector) + ")"
	value := "max by (" + monitoringPodGroup + ") (" + monitoringFresh("modurelay_kubernetes_pod_"+name+selector) + ")"
	up := "up" + monitoringSelector(OpsMonitoringQuery{Cluster: q.Cluster}, "base", `job="modurelay-cluster"`)
	health := "min by (cluster) ((" + monitoringFresh(up) + ")) == 1"
	return "(" + value + ") and on (" + monitoringPodGroup + ") ((time() - " + observed + ") <= 120) and on (cluster) (" + health + ")"
}
func monitoringEffectivePodLimit(q OpsMonitoringQuery, memory bool) string {
	computed := "(" + monitoringEffectiveLimitSeries(q, memory, "effective_limit") + ") > 0"
	resource, unit := "cpu", "cores"
	if memory {
		resource, unit = "memory", "bytes"
	}
	scheduler := "max by (cluster,namespace,pod) (" + monitoringFresh("kube_pod_resource_limit"+monitoringSelector(q, "pod", "resource="+strconv.Quote(resource), "unit="+strconv.Quote(unit))) + ")"
	scheduler = "(" + scheduler + ") * on (cluster,namespace,pod) group_left (uid) (" + monitoringPodIdentity(q) + ")"
	// A scheduler that has not adopted Pod-level or in-place resources must not
	// override the actuated API contract. Prefer it only when the values agree.
	scheduler = "(" + scheduler + ") == on (" + monitoringPodGroup + ") (" + computed + ")"
	label := func(value, source string) string {
		return `label_replace((` + value + `),"limit_source","` + source + `","pod",".*")`
	}
	return "(" + label(scheduler, "kube_scheduler") + ") or on (" + monitoringPodGroup + ") (" + label(computed, "kubernetes_api") + ")"
}
func monitoringUnlimitedPodLimit(q OpsMonitoringQuery, memory bool) string {
	unlimited := "0 * ((" + monitoringEffectiveLimitSeries(q, memory, "limit_bounded") + ") == 0)"
	return `label_replace((` + unlimited + `),"limit_state","unlimited","pod",".*")`
}
