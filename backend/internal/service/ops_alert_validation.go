package service

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

var opsAlertResourceLabel = regexp.MustCompile(`^[A-Za-z0-9_./:-]{1,253}$`)

func opsAlertSupportsBusinessScope(metric string) bool {
	switch metric {
	case "success_rate", "error_rate", "upstream_error_rate", "group_available_accounts", "group_available_ratio", "group_rate_limit_ratio", "account_rate_limited_count", "account_error_count", "account_error_ratio", "account_temp_unscheduled_count", "overload_account_count":
		return true
	}
	return false
}

func ValidateOpsAlertFilters(metric string, filters map[string]any) error {
	for key, value := range filters {
		switch key {
		case "cluster", "namespace", "service", "deployment", "pod", "uid", "source", "protocol", "route", "status_code":
			if metric == "monitoring_source_unavailable" {
				return fmt.Errorf("filters.%s is not supported by source availability", key)
			}
			v, ok := value.(string)
			if !ok || !opsAlertResourceLabel.MatchString(v) {
				return fmt.Errorf("filters.%s must be a valid resource label", key)
			}
			if !isMonitoringAlertMetric(metric) {
				return fmt.Errorf("filters.%s requires a monitoring metric", key)
			}
			if key == "source" && v != "app" && v != "ingress" && v != "external_ingress" {
				return fmt.Errorf("filters.source must be app, ingress or external_ingress")
			}
		case "platform", "region":
			if key == "region" || !opsAlertSupportsBusinessScope(metric) {
				return fmt.Errorf("filters.%s is not an evaluated dimension for %s", key, metric)
			}
			v, ok := value.(string)
			if !ok || len(v) > 253 {
				return fmt.Errorf("filters.%s must be a string up to 253 characters", key)
			}
		case "exclude_429":
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("filters.exclude_429 must be a boolean")
			}
			if metric != "http_4xx_count" && metric != "http_4xx_rate" {
				return fmt.Errorf("exclude_429 requires a 4xx metric")
			}
		case "min_errors", "group_id", "critical_threshold":
			if key == "group_id" && !opsAlertSupportsBusinessScope(metric) {
				return fmt.Errorf("filters.group_id is not an evaluated dimension for %s", metric)
			}
			v, ok := value.(float64)
			if !ok {
				switch n := value.(type) {
				case int:
					v = float64(n)
					ok = true
				case int64:
					v = float64(n)
					ok = true
				}
			}
			if !ok || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1e12 {
				return fmt.Errorf("filters.%s must be a finite nonnegative number", key)
			}
			if key != "critical_threshold" && v != math.Trunc(v) {
				return fmt.Errorf("filters.%s must be an integer", key)
			}
			if key == "min_errors" && metric != "http_4xx_rate" && metric != "http_5xx_rate" && metric != "http_4xx_count" && metric != "http_5xx_count" && metric != "http_429_count" {
				return fmt.Errorf("min_errors requires an HTTP error metric")
			}
		case "webhook_urls":
			var targets []string
			switch urls := value.(type) {
			case []string:
				targets = urls
			case []any:
				for _, u := range urls {
					str, ok := u.(string)
					if !ok {
						return fmt.Errorf("webhook_urls must contain only strings")
					}
					targets = append(targets, str)
				}
			default:
				return fmt.Errorf("webhook_urls must be an array")
			}
			if len(targets) > 5 {
				return fmt.Errorf("at most 5 webhook URLs are allowed")
			}
			for _, target := range targets {
				if err := ValidateOpsAlertWebhookURL(strings.TrimSpace(target)); err != nil {
					return err
				}
			}
		case "silenced_until":
			raw, ok := value.(string)
			if !ok {
				return fmt.Errorf("silenced_until must be RFC3339")
			}
			if _, err := time.Parse(time.RFC3339, raw); err != nil {
				return fmt.Errorf("silenced_until must be RFC3339")
			}
		default:
			return fmt.Errorf("unsupported alert filter: %s", key)
		}
	}
	if isMonitoringAlertMetric(metric) && metric != "monitoring_source_unavailable" {
		text := func(key string) string { value, _ := filters[key].(string); return value }
		q := OpsMonitoringQuery{Metric: metric, Cluster: text("cluster"), Namespace: text("namespace"), Service: text("service"), Deployment: text("deployment"), Pod: text("pod"), UID: text("uid"), Source: text("source"), Protocol: text("protocol"), Route: text("route"), StatusCode: text("status_code")}
		if _, err := normalizeMonitoringQuery(&q, config.OpsMonitoringConfig{}); err != nil {
			return err
		}
	}
	return nil
}
