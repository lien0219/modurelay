package config

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// OpsMonitoringConfig is operator-managed only. Neither the URL nor credentials
// are accepted by the monitoring API or returned in its capability response.
type OpsMonitoringConfig struct {
	Mode                       string `mapstructure:"mode"`
	PrometheusURL              string `mapstructure:"prometheus_url"`
	BearerToken                string `mapstructure:"bearer_token" json:"-"`
	TimeoutSeconds             int    `mapstructure:"timeout_seconds"`
	MaxQueryRangeHours         int    `mapstructure:"max_query_range_hours"`
	ClusterName                string `mapstructure:"cluster_name"`
	KubernetesResourcesEnabled bool   `mapstructure:"kubernetes_resources_enabled"`
	ExternalIngressAdapter     string `mapstructure:"external_ingress_adapter"`
	ExternalIngressJob         string `mapstructure:"external_ingress_job"`
	KubernetesEventsEnabled    bool   `mapstructure:"kubernetes_events_enabled"`
	EventsNamespace            string `mapstructure:"events_namespace"`
	EventsRetentionDays        int    `mapstructure:"events_retention_days"`
}

func (c OpsMonitoringConfig) Validate() error {
	switch c.ExternalIngressAdapter {
	case "", "nginx", "normalized":
	default:
		return fmt.Errorf("ops.monitoring.external_ingress_adapter must be nginx or normalized")
	}
	if len(c.ExternalIngressJob) > 253 || strings.ContainsAny(c.ExternalIngressJob, "\x00\r\n") {
		return fmt.Errorf("invalid external ingress scrape job")
	}

	if len(c.ClusterName) > 253 || strings.ContainsAny(c.ClusterName, "\x00\r\n") {
		return fmt.Errorf("ops.monitoring.cluster_name is invalid")
	}
	if len(c.EventsNamespace) > 253 || strings.ContainsAny(c.EventsNamespace, "/\\\x00\r\n") {
		return fmt.Errorf("ops.monitoring.events_namespace is invalid")
	}
	if c.EventsRetentionDays < 0 || c.EventsRetentionDays > 366 {
		return fmt.Errorf("ops.monitoring.events_retention_days must be between 0 and 366")
	}
	switch c.Mode {
	case "", "auto", "standalone", "kubernetes":
	default:
		return fmt.Errorf("ops.monitoring.mode must be auto, standalone or kubernetes")
	}
	if c.TimeoutSeconds < 0 || c.TimeoutSeconds > 30 {
		return fmt.Errorf("ops.monitoring.timeout_seconds must be between 0 and 30")
	}
	if c.MaxQueryRangeHours < 0 || c.MaxQueryRangeHours > 24*366 {
		return fmt.Errorf("ops.monitoring.max_query_range_hours must be between 0 and 8784")
	}
	if strings.TrimSpace(c.PrometheusURL) == "" {
		return nil
	}
	u, err := url.Parse(c.PrometheusURL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("ops.monitoring.prometheus_url must be an HTTP(S) base URL without credentials, query or fragment")
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && !MonitoringAddressAllowed(ip) {
		return fmt.Errorf("ops.monitoring.prometheus_url cannot target link-local, multicast or unspecified addresses")
	}
	return nil
}

// Private networks and loopback are intentional: Prometheus commonly runs in
// the same cluster or on localhost. Link-local cloud metadata is never allowed.
func MonitoringAddressAllowed(ip net.IP) bool {
	return ip != nil && !ip.IsUnspecified() && !ip.IsMulticast() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast()
}
