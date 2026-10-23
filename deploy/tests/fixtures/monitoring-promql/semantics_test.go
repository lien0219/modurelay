package main

import (
	"fmt"
	"github.com/prometheus/prometheus/promql/promqltest"
	"strings"
	"testing"
	"time"
)

func TestMonitoringPromQLSemantics(t *testing.T) {
	q := OpsMonitoringQuery{Metric: "qps", Window: "1m", Source: "app", Cluster: "c", Namespace: "n", Service: "api"}
	expr, _, _ := monitoringPromQL(q)
	traffic := `load 15s
 modurelay_http_requests_total{cluster="c",namespace="n",service="api",pod="a",uid="ua",instance="a:8080",source="app",method="POST",route="messages",protocol="http",status_code="200",job="one"} 0+30x12
 modurelay_http_requests_total{cluster="c",namespace="n",service="api",pod="a",uid="ua",instance="a:8080",source="app",method="POST",route="messages",protocol="http",status_code="200",job="duplicate"} 0+30x12
 modurelay_http_requests_total{cluster="c",namespace="n",service="api",pod="a",uid="ua",instance="a:8080",source="app",method="POST",route="responses",protocol="http",status_code="200",job="one"} 0+15x12
 modurelay_http_requests_total{cluster="c",namespace="n",service="api",pod="b",uid="ub",instance="b:8080",source="app",method="POST",route="responses",protocol="http",status_code="200",job="one"} 0+60x12
 up{cluster="c",namespace="n",service="api",pod="a",uid="ua",instance="a:8080",job="modurelay-instance"} 1+0x12
 up{cluster="c",namespace="n",service="api",pod="b",uid="ub",instance="b:8080",job="modurelay-instance"} 1+0x12
`
	cases := []struct{ name, input string }{
		{"dedup_and_distinct_routes", traffic + "\neval instant at 3m " + expr + "\n {cluster=\"c\",namespace=\"n\",service=\"api\",source=\"app\"} 7\n"},
		{"failed_target_masks_entire_service", strings.Replace(traffic, `job="modurelay-instance"} 1+0x12`, `job="modurelay-instance"} 0+0x12`, 1) + "\neval instant at 3m " + expr + "\n"},
		{"stale_target_masks_entire_service", strings.Replace(traffic, `job="modurelay-instance"} 1+0x12`, `job="modurelay-instance"} 1+0x6`, 1) + "\neval instant at 3m " + expr + "\n"},
	}
	q.Pod = "a"
	podHTTP, _, _ := monitoringPromQL(q)
	cases = append(cases, struct{ name, input string }{"pod_http_preserves_uid", traffic + "\neval instant at 3m " + podHTTP + "\n {cluster=\"c\",namespace=\"n\",service=\"api\",source=\"app\",pod=\"a\",uid=\"ua\"} 3\n"})
	q.Pod = ""
	q.Metric = "http_5xx_count"
	zero, _, _ := monitoringPromQL(q)
	cases = append(cases, struct{ name, input string }{"zero_errors_is_legitimate_zero", traffic + "\neval instant at 3m " + zero + "\n {cluster=\"c\",namespace=\"n\",service=\"api\",source=\"app\"} 0\n"})
	resources := `load 15s
 kube_pod_info{cluster="c",namespace="n",pod="a",uid="ua"} 1+0x40
 kube_pod_created{cluster="c",namespace="n",pod="a",uid="ua"} 0+0x40
 kube_pod_container_info{cluster="c",namespace="n",pod="a",uid="ua",container="one"} 1+0x40
 kube_pod_container_info{cluster="c",namespace="n",pod="a",uid="ua",container="two"} 1+0x40
 kube_pod_container_resource_limits{cluster="c",namespace="n",pod="a",uid="ua",container="one",resource="cpu",unit="core"} 1+0x40
 kube_pod_container_resource_limits{cluster="c",namespace="n",pod="a",uid="ua",container="two",resource="cpu",unit="core"} 1+0x40
 kube_pod_resource_limit{cluster="c",namespace="n",pod="a",resource="cpu",unit="cores"} 4+0x40
 container_cpu_usage_seconds_total{cluster="c",namespace="n",pod="a",container="one",id="one"} 0+7.5x40
 container_cpu_usage_seconds_total{cluster="c",namespace="n",pod="a",container="two",id="two"} 0+7.5x40
 container_start_time_seconds{cluster="c",namespace="n",pod="a",container="one",id="one"} 1+0x40
 container_start_time_seconds{cluster="c",namespace="n",pod="a",container="two",id="two"} 1+0x40
`
	q.Service = ""
	q.Window = "5m"
	q.Metric = "pod_cpu_usage_percent"
	cpu, _, _ := monitoringPromQL(q)
	cases = append(cases, struct{ name, input string }{"scheduler_limit_is_preferred", resources + "\neval instant at 10m " + cpu + "\n {cluster=\"c\",namespace=\"n\",pod=\"a\",uid=\"ua\",limit_source=\"kube_scheduler\"} 25\n"})
	unlimited := strings.Replace(resources, `container="two",resource="cpu",unit="core"} 1+0x40`, `container="two",resource="cpu",unit="core"} 0+0x40`, 1)
	cases = append(cases, struct{ name, input string }{"unlimited_sidecar_is_unknown", unlimited + "\neval instant at 10m " + cpu + "\n"})
	q.Metric = "pod_memory_bytes"
	memory, _, _ := monitoringPromQL(q)
	replacement := `load 15s
 kube_pod_info{cluster="c",namespace="n",pod="same",uid="new"} 1+0x40
 kube_pod_created{cluster="c",namespace="n",pod="same",uid="new"} 500+0x40
 container_memory_working_set_bytes{cluster="c",namespace="n",pod="same",container="main",id="old"} 512+0x40
 container_memory_working_set_bytes{cluster="c",namespace="n",pod="same",container="main",id="new"} 128+0x40
 container_start_time_seconds{cluster="c",namespace="n",pod="same",container="main",id="old"} 100+0x40
 container_start_time_seconds{cluster="c",namespace="n",pod="same",container="main",id="new"} 550+0x40
`
	cases = append(cases, struct{ name, input string }{"replacement_pod_excludes_old_cgroup", replacement + "\neval instant at 10m " + memory + "\n {cluster=\"c\",namespace=\"n\",pod=\"same\",uid=\"new\"} 128\n"})
	q.Metric = "pod_cpu_throttling_percent"
	throttling, _, _ := monitoringPromQL(q)
	throttleInput := `load 15s
 kube_pod_info{cluster="c",namespace="n",pod="same",uid="new"} 1+0x40
 kube_pod_created{cluster="c",namespace="n",pod="same",uid="new"} 100+0x40
 container_cpu_cfs_throttled_periods_total{cluster="c",namespace="n",pod="same",container="main",id="old"} 0+135x40
 container_cpu_cfs_throttled_periods_total{cluster="c",namespace="n",pod="same",container="main",id="new"} 0+15x40
 container_cpu_cfs_periods_total{cluster="c",namespace="n",pod="same",container="main",id="old"} 0+150x40
 container_cpu_cfs_periods_total{cluster="c",namespace="n",pod="same",container="main",id="new"} 0+150x40
 container_start_time_seconds{cluster="c",namespace="n",pod="same",container="main",id="old"} 50+0x40
 container_start_time_seconds{cluster="c",namespace="n",pod="same",container="main",id="new"} 200+0x40
`
	cases = append(cases, struct{ name, input string }{"throttling_excludes_old_cgroup", throttleInput + "\neval instant at 10m " + throttling + "\n {cluster=\"c\",namespace=\"n\",pod=\"same\",uid=\"new\"} 10\n"})
	q = OpsMonitoringQuery{Metric: "pod_crash_loop", Window: "5m", Cluster: "c", Namespace: "n", UID: "current"}
	crash, _, _ := monitoringPromQL(q)
	runningPod := `load 15s
 kube_pod_info{cluster="c",namespace="n",pod="a",uid="current",job="ksm",instance="ksm:8080"} 1+0x40
 kube_pod_container_info{cluster="c",namespace="n",pod="a",uid="current",container="main",job="ksm",instance="ksm:8080"} 1+0x40
 kube_pod_container_status_running{cluster="c",namespace="n",pod="a",uid="current",container="main",job="ksm",instance="ksm:8080"} 1+0x40
 kube_pod_container_status_waiting_reason{cluster="c",namespace="n",pod="a",uid="current",container="main",reason="CrashLoopBackOff",job="ksm",instance="ksm:8080"} 1+0x4 stale
 up{cluster="c",job="ksm",instance="ksm:8080"} 1+0x40
`
	cases = append(cases, struct{ name, input string }{"crash_loop_recovers_to_trusted_zero", runningPod + "\neval instant at 10m " + crash + "\n {cluster=\"c\",namespace=\"n\",pod=\"a\",uid=\"current\"} 0\n"})
	initInfo := ` kube_pod_init_container_info{cluster="c",namespace="n",pod="a",uid="current",container="sidecar",job="ksm",instance="ksm:8080"} 1+0x40
`
	initWaiting := ` kube_pod_init_container_status_waiting_reason{cluster="c",namespace="n",pod="a",uid="current",container="sidecar",reason="CrashLoopBackOff",job="ksm",instance="ksm:8080"} 1+0x40
`
	cases = append(cases, struct{ name, input string }{"crash_loop_init_sidecar_is_visible", runningPod + initInfo + initWaiting + "\neval instant at 10m " + crash + "\n {cluster=\"c\",namespace=\"n\",pod=\"a\",uid=\"current\"} 1\n"})
	cases = append(cases, struct{ name, input string }{"crash_loop_missing_init_state_does_not_recover", runningPod + initInfo + "\neval instant at 10m " + crash + "\n"})
	initCompleted := ` kube_pod_init_container_status_terminated_reason{cluster="c",namespace="n",pod="a",uid="current",container="sidecar",reason="Completed",job="ksm",instance="ksm:8080"} 1+0x40
`
	cases = append(cases, struct{ name, input string }{"crash_loop_completed_init_allows_recovery", runningPod + initInfo + initCompleted + "\neval instant at 10m " + crash + "\n {cluster=\"c\",namespace=\"n\",pod=\"a\",uid=\"current\"} 0\n"})
	failedScrape := strings.Replace(runningPod, `up{cluster="c",job="ksm",instance="ksm:8080"} 1+0x40`, `up{cluster="c",job="ksm",instance="ksm:8080"} 0+0x40`, 1)
	cases = append(cases, struct{ name, input string }{"crash_loop_failed_scrape_does_not_recover", failedScrape + "\neval instant at 10m " + crash + "\n"})
	q.UID = "previous"
	oldUID, _, _ := monitoringPromQL(q)
	cases = append(cases, struct{ name, input string }{"crash_loop_replacement_uid_does_not_recover_old_pod", runningPod + "\neval instant at 10m " + oldUID + "\n"})
	staleRunning := strings.Replace(runningPod, `job="ksm",instance="ksm:8080"} 1+0x40`, `job="ksm",instance="ksm:8080"} 1+0x4`, 1)
	cases = append(cases, struct{ name, input string }{"crash_loop_stale_identity_does_not_recover", staleRunning + "\neval instant at 10m " + crash + "\n"})
	q = OpsMonitoringQuery{Metric: "pod_oom_count", Window: "5m", Cluster: "c", Namespace: "n"}
	oomCount, _, _ := monitoringPromQL(q)
	oomCgroups := `load 15s
 kube_pod_info{cluster="c",namespace="n",pod="oom",uid="current"} 1+0x40
 kube_pod_created{cluster="c",namespace="n",pod="oom",uid="current"} 0+0x40
 container_start_time_seconds{cluster="c",namespace="n",pod="oom",container="main",id="one"} 1+0x40
 container_start_time_seconds{cluster="c",namespace="n",pod="oom",container="main",id="two"} 2+0x40
 container_oom_events_total{cluster="c",namespace="n",pod="oom",container="main",id="one"} 0+1x40
 container_oom_events_total{cluster="c",namespace="n",pod="oom",container="main",id="two"} 0+1x40
`
	cases = append(cases, struct{ name, input string }{"oom_increases_sum_across_cgroup_incarnations", oomCgroups + "\neval instant at 10m " + oomCount + "\n {cluster=\"c\",namespace=\"n\",pod=\"oom\",uid=\"current\"} 40\n"})
	q = OpsMonitoringQuery{Metric: "pod_cpu_usage_percent", Window: "5m", Cluster: "c", Namespace: "n", effectiveLimits: true}
	apiLimits := `
load 15s
 modurelay_kubernetes_pod_effective_limit{cluster="c",namespace="n",pod="a",uid="ua",resource="cpu"} 4+0x40
 modurelay_kubernetes_pod_limit_bounded{cluster="c",namespace="n",pod="a",uid="ua",resource="cpu"} 1+0x40
 modurelay_kubernetes_pod_limit_observed_timestamp_seconds{cluster="c",namespace="n",pod="a",uid="ua",resource="cpu"} 600+0x40
 up{cluster="c",job="modurelay-cluster",instance="app:8080"} 1+0x40
`
	strictLimit, _, _ := monitoringPromQL(q)
	cases = append(cases, struct{ name, input string }{"effective_api_confirms_scheduler_limit", resources + apiLimits + "\neval instant at 10m " + strictLimit + "\n {cluster=\"c\",namespace=\"n\",pod=\"a\",uid=\"ua\",limit_source=\"kube_scheduler\"} 25\n"})
	podOnly := strings.ReplaceAll(resources, `kube_pod_container_resource_limits`, `ignored_container_resource_limits`)
	cases = append(cases, struct{ name, input string }{"pod_level_limit_without_container_limits", podOnly + apiLimits + "\neval instant at 10m " + strictLimit + "\n {cluster=\"c\",namespace=\"n\",pod=\"a\",uid=\"ua\",limit_source=\"kube_scheduler\"} 25\n"})
	oldScheduler := strings.Replace(resources, `resource="cpu",unit="cores"} 4+0x40`, `resource="cpu",unit="cores"} 2+0x40`, 1)
	cases = append(cases, struct{ name, input string }{"outdated_scheduler_uses_actuated_api_limit", oldScheduler + apiLimits + "\neval instant at 10m " + strictLimit + "\n {cluster=\"c\",namespace=\"n\",pod=\"a\",uid=\"ua\",limit_source=\"kubernetes_api\"} 25\n"})
	staleAPI := strings.Replace(apiLimits, `600+0x40`, `0+0x40`, 1)
	cases = append(cases, struct{ name, input string }{"stale_effective_limit_is_unknown", resources + staleAPI + "\neval instant at 10m " + strictLimit + "\n"})
	unlimitedAPI := strings.Replace(apiLimits, `modurelay_kubernetes_pod_effective_limit`, `ignored_effective_limit`, 1)
	unlimitedAPI = strings.Replace(unlimitedAPI, `resource="cpu"} 1+0x40`, `resource="cpu"} 0+0x40`, 1)
	cases = append(cases, struct{ name, input string }{"explicit_unlimited_marker_is_not_numeric_usage", resources + unlimitedAPI + "\neval instant at 10m " + strictLimit + "\n {cluster=\"c\",namespace=\"n\",pod=\"a\",uid=\"ua\",limit_state=\"unlimited\"} 0\n"})
	q = OpsMonitoringQuery{Metric: "http_5xx_rate", Window: "1m", Cluster: "c", Namespace: "n", Service: "api", Source: "external_ingress", externalAdapter: "nginx", externalJob: "edge"}
	external, _, _ := monitoringPromQL(q)
	edge := `load 15s
 nginx_ingress_controller_requests{cluster="c",namespace="n",service="api",instance="edge:10254",job="edge",method="POST",status="200",ingress="api",host="example.test",path="/"} 0+27x12
 nginx_ingress_controller_requests{cluster="c",namespace="n",service="api",instance="edge:10254",job="edge",method="POST",status="502",ingress="api",host="example.test",path="/"} 0+3x12
 up{cluster="c",instance="edge:10254",job="edge"} 1+0x12
`
	cases = append(cases, struct{ name, input string }{"nginx_external_errors_do_not_mix_app", traffic + "\n" + edge + "\neval instant at 3m " + external + "\n {cluster=\"c\",namespace=\"n\",service=\"api\",source=\"external_ingress\"} 10\n"})
	q = OpsMonitoringQuery{Metric: "db_pool_wait_count", Window: "1m", Cluster: "c", Namespace: "n", Service: "api"}
	poolQuery, _, _ := monitoringPromQL(q)
	poolInput := `load 15s
 modurelay_database_pool_wait_total{cluster="c",namespace="n",service="api",pod="a",uid="ua",instance="a:8080",pool="primary"} 0+30x12
 modurelay_database_pool_wait_total{cluster="c",namespace="n",service="api",pod="a",uid="ua",instance="a:8080",pool="advisory_lock"} 0+15x12
 modurelay_database_pool_wait_total{cluster="c",namespace="n",service="api",pod="a",uid="ua",instance="a:8080",pool="read_replica"} 0+45x12
 modurelay_db_pool_wait_total{cluster="c",namespace="n",service="api",pod="a",uid="ua",instance="a:8080"} 0+30x12
 up{cluster="c",namespace="n",service="api",pod="a",uid="ua",instance="a:8080",job="modurelay-instance"} 1+0x12
`
	cases = append(cases, struct{ name, input string }{"all_database_pools_without_legacy_double_count", poolInput + "\neval instant at 3m " + poolQuery + "\n {cluster=\"c\",namespace=\"n\",service=\"api\"} 360\n"})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engine := promqltest.NewTestEngine(t, false, 5*time.Minute, 100000)
			defer engine.Close()
			promqltest.RunTest(t, tc.input, engine)
		})
	}
	fmt.Printf("Executed %d Prometheus engine metric contract cases\n", len(cases))
}
