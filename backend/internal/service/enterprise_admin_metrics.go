package service

import (
	"github.com/prometheus/client_golang/prometheus"
	"math"
	"time"
)

var adminMetricsRegistry = prometheus.NewRegistry()
var adminDiagnosticDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "modurelay_enterprise_admin_diagnostics_duration_seconds", Help: "Bounded administrator diagnostic request duration."}, []string{"endpoint"})
var adminWorkerBacklog = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "modurelay_enterprise_admin_worker_backlog", Help: "Last observed exact inventory; NaN for unavailable or capped evidence. Queue inventory does not establish liveness."}, []string{"worker", "state"})
var adminWorkerAvailability = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "modurelay_enterprise_admin_worker_inventory_available", Help: "One when inventory is available and exact, zero otherwise; check observation timestamp."}, []string{"worker", "state"})
var adminWorkerLag = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "modurelay_enterprise_admin_worker_due_lag_seconds", Help: "Last observed due lag; NaN when unavailable; check observation timestamp."}, []string{"worker"})
var adminWorkerObserved = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "modurelay_enterprise_admin_worker_observed_timestamp_seconds", Help: "Database snapshot observation timestamp; on-demand observations are not heartbeats."}, []string{"worker"})
var adminBillingAlertAge = prometheus.NewGauge(prometheus.GaugeOpts{Name: "modurelay_enterprise_admin_billing_pending_alert_age_seconds", Help: "Age of the oldest pending billing alert; NaN when evidence is unavailable or capped. This is not worker due lag."})
var adminOperationTotal = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "modurelay_enterprise_admin_operation_total", Help: "Count of guarded Global Admin operations by fixed action and outcome."}, []string{"action", "outcome"})

func init() {
	adminMetricsRegistry.MustRegister(adminDiagnosticDuration, adminWorkerBacklog, adminWorkerAvailability, adminWorkerLag, adminWorkerObserved, adminBillingAlertAge, adminOperationTotal)
}

// EnterpriseAdminMetricsGatherer exposes only Phase H collectors. The caller
// must independently authorize the scrape and refresh on-demand diagnostics.
func EnterpriseAdminMetricsGatherer() prometheus.Gatherer { return adminMetricsRegistry }

func observeAdminDuration(endpoint string, start time.Time) {
	if adminAllowed(endpoint, "search", "overview", "workspace", "jobs") {
		adminDiagnosticDuration.WithLabelValues(endpoint).Observe(time.Since(start).Seconds())
	}
}

func observeAdminOperation(action, outcome string) {
	if adminAllowed(action, "suspend", "resume", "retry_webhook") && adminAllowed(outcome, "success", "error") {
		adminOperationTotal.WithLabelValues(action, outcome).Inc()
	}
}
func observeAdminWorkers(v *AdminJobsDiagnostics) {
	for _, w := range v.Workers {
		if !adminAllowed(w.Worker, "outbox", "notification", "webhook", "finops", "export", "purge", "scim", "billing_recovery", "async_settlement") {
			continue
		}
		adminWorkerObserved.WithLabelValues(w.Worker).Set(float64(w.ObservedAt.Unix()))
		lag := math.NaN()
		if w.DueLagSeconds != nil && w.Freshness == "fresh" {
			lag = float64(*w.DueLagSeconds)
		}
		adminWorkerLag.WithLabelValues(w.Worker).Set(lag)
		if w.Worker == "billing_recovery" {
			age := math.NaN()
			if w.PendingAlertAge.Available && w.PendingAlertAge.AgeSeconds != nil && w.PendingAlertAge.Freshness == "fresh" {
				age = float64(*w.PendingAlertAge.AgeSeconds)
			}
			adminBillingAlertAge.Set(age)
		}
		for state, c := range w.Counts {
			if !adminAllowed(state, "pending", "running", "completed", "cancelled", "expired", "output_deleted", "recovered", "unleased_running", "live_leases", "expired_leases", "due", "retry_evidence", "dead", "disabled_endpoint_backlog", "materialized_pending", "failed", "blocked", "cooling", "expiring_tokens", "connector_errors", "pending_alerts", "batch_image_pending", "video_pending", "video_running", "video_failed", "video_completed") {
				continue
			}
			value, available := math.NaN(), float64(0)
			if c.Available && !c.Capped && c.Value != nil && c.Freshness == "fresh" {
				value = float64(*c.Value)
				available = 1
			}
			adminWorkerBacklog.WithLabelValues(w.Worker, state).Set(value)
			adminWorkerAvailability.WithLabelValues(w.Worker, state).Set(available)
		}
	}
}
