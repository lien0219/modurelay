// This package runs only in the temporary module created by the contract script.
package main

import (
	"fmt"
	"os"

	"github.com/prometheus/prometheus/promql/parser"
	"gopkg.in/yaml.v3"
)

func monitoringWindowSeconds(window string) (int, error) {
	return map[string]int{"1m": 60, "5m": 300, "15m": 900, "60m": 3600}[window], nil
}

func main() {
	p := parser.NewParser(parser.Options{})
	count := 0
	for _, metric := range opsMonitoringMetrics {
		for _, deployment := range []string{"", "api"} {
			q := OpsMonitoringQuery{Metric: metric.ID, Window: "5m", Source: "app", Cluster: "cluster", Namespace: "namespace", Service: "api", Deployment: deployment, Pod: "api-1", UID: "uid-1"}
			expr, samples, err := monitoringPromQL(q)
			if err != nil {
				panic(err)
			}
			for _, query := range []string{expr, samples} {
				if query == "" {
					continue
				}
				if _, err := p.ParseExpr(query); err != nil {
					panic(fmt.Errorf("%s: %w", metric.ID, err))
				}
				count++
			}
		}
	}
	for _, adapter := range []string{"nginx", "normalized"} {
		for _, metric := range []string{"qps", "rpm", "request_count", "http_p95_ms", "http_p99_ms", "http_4xx_count", "http_5xx_count", "http_429_count", "http_4xx_rate", "http_5xx_rate"} {
			q := OpsMonitoringQuery{Metric: metric, Window: "5m", Source: "external_ingress", externalAdapter: adapter, externalJob: "edge", Cluster: "c", Service: "api"}
			expr, samples, err := monitoringPromQL(q)
			if err != nil {
				panic(err)
			}
			for _, query := range []string{expr, samples} {
				if query != "" {
					if _, err := p.ParseExpr(query); err != nil {
						panic(err)
					}
					count++
				}
			}
		}
	}
	for _, metric := range []string{"pod_cpu_limit_cores", "pod_memory_limit_bytes", "pod_cpu_usage_percent", "pod_memory_usage_percent", "cluster_cpu_usage_percent", "deployment_cpu_usage_percent"} {
		q := OpsMonitoringQuery{Metric: metric, Window: "5m", Source: "app", effectiveLimits: true}
		expr, _, err := monitoringPromQL(q)
		if err != nil {
			panic(err)
		}
		if _, err := p.ParseExpr(expr); err != nil {
			panic(err)
		}
		count++
	}
	if len(os.Args) != 2 {
		panic("expected external monitoring rules YAML path")
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	var rules struct {
		Groups []struct {
			Rules []struct {
				Expr string `yaml:"expr"`
			} `yaml:"rules"`
		} `yaml:"groups"`
	}
	if err := yaml.Unmarshal(data, &rules); err != nil {
		panic(err)
	}
	ruleCount := 0
	for _, group := range rules.Groups {
		for _, rule := range group.Rules {
			if rule.Expr == "" {
				panic("missing rule expression")
			}
			if _, err := p.ParseExpr(rule.Expr); err != nil {
				panic(err)
			}
			ruleCount++
		}
	}
	if ruleCount == 0 {
		panic("no monitoring watchdog rules found")
	}
	fmt.Printf("Parsed %d fixed monitoring queries and %d external watchdog rule expressions with Prometheus v0.315.1\n", count, ruleCount)
}
