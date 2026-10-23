package service

import "time"

// Ops alert rule/event models.
//
// NOTE: These are admin-facing DTOs and intentionally keep JSON naming aligned
// with the existing ops dashboard frontend (backup style).

const (
	OpsAlertStatusFiring         = "firing"
	OpsAlertStatusResolved       = "resolved"
	OpsAlertStatusManualResolved = "manual_resolved"

	OpsAlertDataStatusOK           = "ok"
	OpsAlertDataStatusInsufficient = "insufficient_data"
	OpsAlertDataStatusInvalidRule  = "invalid_rule"
	// OpsAlertDataStatusUnavailable means the metric could not be evaluated.
	// An empty data_status still means the rule has not been evaluated.
	OpsAlertDataStatusUnavailable = "unavailable"
)

type OpsAlertRule struct {
	EvaluationRevision int64  `json:"evaluation_revision"`
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	Description        string `json:"description"`

	Enabled  bool   `json:"enabled"`
	Severity string `json:"severity"`

	MetricType string  `json:"metric_type"`
	Operator   string  `json:"operator"`
	Threshold  float64 `json:"threshold"`

	WindowMinutes    int `json:"window_minutes"`
	SustainedMinutes int `json:"sustained_minutes"`
	CooldownMinutes  int `json:"cooldown_minutes"`
	// MinRequests gates success_rate, error_rate, and upstream_error_rate.
	// Zero keeps the previous evaluation behavior. Other metrics ignore it.
	MinRequests int `json:"min_requests"`

	NotifyEmail bool `json:"notify_email"`

	// DataStatus is the latest evaluator sample state. It is not stored on the rule row.
	DataStatus string `json:"data_status,omitempty"`

	Filters map[string]any `json:"filters,omitempty"`

	LastTriggeredAt *time.Time `json:"last_triggered_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type OpsAlertRuleState struct {
	RuleRevision        int64
	RuleID              int64
	ConsecutiveBreaches int
	LastEvaluatedAt     time.Time
	DataStatus          string
}

type OpsAlertEvent struct {
	RuleRevision int64  `json:"-"`
	ID           int64  `json:"id"`
	RuleID       int64  `json:"rule_id"`
	Severity     string `json:"severity"`
	Status       string `json:"status"`

	Title       string `json:"title"`
	Description string `json:"description"`

	MetricValue    *float64 `json:"metric_value,omitempty"`
	ThresholdValue *float64 `json:"threshold_value,omitempty"`

	Dimensions map[string]any `json:"dimensions,omitempty"`

	FiredAt    time.Time  `json:"fired_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`

	EmailSent bool      `json:"email_sent"`
	CreatedAt time.Time `json:"created_at"`
	// Deliveries are inserted atomically with the event; never exposed by the API.
	Deliveries []*OpsAlertDelivery `json:"-"`
}

// OpsMonitoringAlertSample represents one resource, never an average of Pod quantiles.
type OpsMonitoringAlertSample struct {
	ResourceKey  string
	Dimensions   map[string]any
	Value        *float64
	RequestCount int64
	ErrorCount   *float64
}

type OpsAlertDeliveryAttempt struct {
	Attempt     int       `json:"attempt"`
	Status      string    `json:"status"`
	Error       string    `json:"error"`
	CompletedAt time.Time `json:"completed_at"`
}

type OpsAlertDelivery struct {
	AttemptHistory []OpsAlertDeliveryAttempt `json:"attempt_history,omitempty"`
	ID             int64                     `json:"id"`
	EventID        int64                     `json:"event_id"`
	Channel        string                    `json:"channel"`
	Transition     string                    `json:"transition"`
	Target         string                    `json:"target"`
	Status         string                    `json:"status"`
	Attempts       int                       `json:"attempts"`
	LastError      string                    `json:"last_error"`
	NextAttemptAt  time.Time                 `json:"next_attempt_at"`
	CreatedAt      time.Time                 `json:"created_at"`
	SentAt         *time.Time                `json:"sent_at,omitempty"`
	Payload        []byte                    `json:"-"`
	LeaseToken     string                    `json:"-"`
}

type OpsAlertSilence struct {
	ResourceKey string `json:"resource_key,omitempty"`
	ID          int64  `json:"id"`

	RuleID   int64   `json:"rule_id"`
	Platform string  `json:"platform"`
	GroupID  *int64  `json:"group_id,omitempty"`
	Region   *string `json:"region,omitempty"`

	Until  time.Time `json:"until"`
	Reason string    `json:"reason"`

	CreatedBy *int64    `json:"created_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type OpsAlertEventFilter struct {
	RuleID      int64
	ResourceKey string
	Limit       int

	// Cursor pagination (descending by fired_at, then id).
	BeforeFiredAt *time.Time
	BeforeID      *int64

	// Optional filters.
	Status    string
	Severity  string
	EmailSent *bool

	StartTime *time.Time
	EndTime   *time.Time

	// Dimensions filters (best-effort).
	Platform string
	GroupID  *int64
}
