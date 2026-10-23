package service

import (
	"context"
	"errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/billingobserve"
	"time"
)

// OpsBillingReconciliationRepository is an optional read-side capability. It
// never performs deductions, compensation, or Redis balance reads.
type OpsBillingReconciliationRepository interface {
	ReconcileBillingUsage(context.Context, time.Time) error
	GetBillingReconciliation(context.Context) (*OpsBillingReconciliation, error)
}

type OpsBillingFinding struct {
	ID           int64      `json:"id"`
	SettlementID int64      `json:"settlement_id"`
	RequestID    string     `json:"request_id"`
	Kind         string     `json:"kind"`
	ExpectedCost string     `json:"expected_cost"`
	ActualCost   *string    `json:"actual_cost"`
	FirstSeenAt  time.Time  `json:"first_seen_at"`
	LastSeenAt   time.Time  `json:"last_seen_at"`
	ResolvedAt   *time.Time `json:"resolved_at"`
}

type OpsBillingReconciliation struct {
	State                  string                    `json:"state"`
	CheckedAt              *time.Time                `json:"checked_at"`
	WindowStart            *time.Time                `json:"window_start"`
	WindowEnd              *time.Time                `json:"window_end"`
	MissingUsageLogs       *int64                    `json:"missing_usage_logs"`
	AmountMismatches       *int64                    `json:"amount_mismatches"`
	Scanned                int64                     `json:"scanned"`
	Truncated              bool                      `json:"truncated"`
	Findings               []OpsBillingFinding       `json:"findings"`
	OperationalState       string                    `json:"operational_state"`
	OperationalCheckedAt   *time.Time                `json:"operational_checked_at"`
	FailedSettlements      *int64                    `json:"failed_settlements"`
	CompensationBacklog    *int64                    `json:"compensation_backlog"`
	LedgerMismatches       *int64                    `json:"ledger_mismatches"`
	Operations             []OpsBillingOperation     `json:"operations"`
	LedgerFindings         []OpsAccountLedgerFinding `json:"ledger_findings"`
	RecordingGaps          []OpsBillingRecordingGap  `json:"recording_gaps"`
	ObservationUnpersisted int64                     `json:"observation_unpersisted"`
}

type OpsBillingRecordingGap struct {
	InstanceID        string    `json:"instance_id"`
	MissedCount       int64     `json:"missed_count"`
	AcknowledgedCount int64     `json:"acknowledged_count"`
	FirstMissedAt     time.Time `json:"first_missed_at"`
	LastMissedAt      time.Time `json:"last_missed_at"`
}

type OpsBillingOperation struct {
	ID        int64      `json:"id"`
	IntentID  int64      `json:"intent_id"`
	RequestID string     `json:"request_id"`
	Operation string     `json:"operation"`
	State     string     `json:"state"`
	ErrorCode string     `json:"error_code"`
	Attempts  int        `json:"attempts"`
	CreatedAt time.Time  `json:"created_at"`
	CheckedAt *time.Time `json:"checked_at"`
}
type OpsAccountLedgerFinding struct {
	AccountType    string  `json:"account_type"`
	AccountID      int64   `json:"account_id"`
	Field          string  `json:"field"`
	OpeningAmount  string  `json:"opening_amount"`
	ExpectedAmount string  `json:"expected_amount"`
	ActualAmount   *string `json:"actual_amount"`
}
type OpsBillingOperationRepository interface {
	RetryBillingOperation(context.Context, int64, int64, string) error
}

func (s *OpsService) RetryBillingOperation(ctx context.Context, id, actorID int64, note string) error {
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return err
	}
	repo, ok := s.opsRepo.(OpsBillingOperationRepository)
	if !ok {
		return errors.New("billing reconciliation unavailable")
	}
	return repo.RetryBillingOperation(ctx, id, actorID, note)
}

func (s *OpsService) GetBillingReconciliation(ctx context.Context) (*OpsBillingReconciliation, error) {
	unknown := &OpsBillingReconciliation{State: "Unknown", OperationalState: "Unknown", Findings: []OpsBillingFinding{}, RecordingGaps: []OpsBillingRecordingGap{}, ObservationUnpersisted: billingobserve.Pending()}
	if s == nil {
		return unknown, nil
	}
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return nil, err
	}
	repo, ok := s.opsRepo.(OpsBillingReconciliationRepository)
	if !ok {
		return unknown, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	result, err := repo.GetBillingReconciliation(ctx)
	if err != nil || result == nil {
		return unknown, nil
	}
	if result.CheckedAt == nil || time.Since(*result.CheckedAt) > 3*time.Minute || result.Truncated {
		result.State = "Unknown"
		result.MissingUsageLogs, result.AmountMismatches = nil, nil
	} else {
		result.State = "Available"
	}
	if s.metricsCollector != nil {
		runtime, runtimeErr := s.metricsCollector.ClusterRuntimeSnapshot(ctx)
		if runtimeErr != nil {
			result.OperationalState = "Unknown"
		} else if runtime.BillingObservationUnpersisted > result.ObservationUnpersisted {
			result.ObservationUnpersisted = runtime.BillingObservationUnpersisted
		}
	}
	if result.OperationalCheckedAt == nil || time.Since(*result.OperationalCheckedAt) > 3*time.Minute || result.OperationalState != "Available" || result.ObservationUnpersisted > 0 {
		result.OperationalState = "Unknown"
		result.FailedSettlements, result.CompensationBacklog, result.LedgerMismatches = nil, nil, nil
	}
	return result, nil
}

func (c *OpsMetricsCollector) reconcileBillingUsage() {
	repo, ok := c.opsRepo.(OpsBillingReconciliationRepository)
	if !ok {
		return
	}
	// Independent deadline and transaction: a failed audit never changes a
	// gateway settlement, the resource snapshot, or its success heartbeat.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = repo.ReconcileBillingUsage(ctx, time.Now().UTC())
}

type OpsBillingRecordingGapRepository interface {
	AcknowledgeBillingRecordingGap(context.Context, string, int64, string) error
}

func (s *OpsService) AcknowledgeBillingRecordingGap(ctx context.Context, instanceID string, actorID int64, note string) error {
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return err
	}
	repo, ok := s.opsRepo.(OpsBillingRecordingGapRepository)
	if !ok {
		return errors.New("billing observation unavailable")
	}
	return repo.AcknowledgeBillingRecordingGap(ctx, instanceID, actorID, note)
}
func (s *OpsService) BillingObservationCounters() map[string]float64 {
	return billingobserve.Counters()
}

type OpsBillingGapFlusher interface{ FlushBillingRecordingGaps(context.Context) }
