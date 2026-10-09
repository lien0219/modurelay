package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	FinOpsAnomalyDetectorVersion = "v1"

	AnomalyDetectorSpendSpike    = "spend_spike"
	AnomalyDetectorRequestSpike  = "request_spike"
	AnomalyDetectorUnitCostSpike = "unit_cost_spike"

	AnomalyScopeWorkspace      = "workspace"
	AnomalyScopeProject        = "project"
	AnomalyScopePlatform       = "platform"
	AnomalyScopeModel          = "model"
	AnomalyScopeAPIKey         = "api_key"
	AnomalyScopeServiceAccount = "service_account"

	AnomalySeverityLow      = "low"
	AnomalySeverityMedium   = "medium"
	AnomalySeverityHigh     = "high"
	AnomalySeverityCritical = "critical"

	AnomalyStatusOpen         = "open"
	AnomalyStatusAcknowledged = "acknowledged"
	AnomalyStatusResolved     = "resolved"
)

// FinOpsAnomalyConfig is the single source of truth for detector boundaries.
// Values are deliberately conservative: a small monetary fluctuation must not
// create a notification storm, and a sparse baseline never creates a finding.
type FinOpsAnomalyConfig struct {
	DetectorVersion       string
	LookbackDays          int
	Grace                 time.Duration
	MinBaselineSamples    int
	MinObservedRequests   int64
	MinRequestDelta       int64
	MinScore              float64
	SpendAbsoluteFloor    float64
	SpendRelativeFloor    float64
	RequestRelativeFloor  float64
	UnitCostAbsoluteFloor float64
	UnitCostRelativeFloor float64
	CandidateCap          int
	MaxRollupRows         int
	ScanWorkspaceBatch    int
}

func DefaultFinOpsAnomalyConfig() FinOpsAnomalyConfig {
	return FinOpsAnomalyConfig{
		DetectorVersion:       FinOpsAnomalyDetectorVersion,
		LookbackDays:          28,
		Grace:                 5 * time.Minute,
		MinBaselineSamples:    12,
		MinObservedRequests:   20,
		MinRequestDelta:       20,
		MinScore:              3.5,
		SpendAbsoluteFloor:    0.01,
		SpendRelativeFloor:    1.0,
		RequestRelativeFloor:  1.0,
		UnitCostAbsoluteFloor: 0.0001,
		UnitCostRelativeFloor: 1.0,
		CandidateCap:          100,
		MaxRollupRows:         10000,
		ScanWorkspaceBatch:    25,
	}
}

type FinOpsDetectorInput struct {
	WorkspaceID    int64
	ProjectID      int64
	ScopeType      string
	ScopeID        int64
	DimensionType  string
	DimensionValue string

	DetectorType    string
	DetectorVersion string
	WindowStart     time.Time
	WindowEnd       time.Time
	EvaluationTime  time.Time

	ObservedSpend     float64
	ExpectedSpend     float64
	ObservedRequests  int64
	ExpectedRequests  float64
	ObservedUnitCost  float64
	ExpectedUnitCost  float64
	BaselineSpend     []float64
	BaselineRequests  []float64
	BaselineUnitCosts []float64
}

type FinOpsAnomalyDetection struct {
	WorkspaceID     int64     `json:"workspace_id"`
	ProjectID       int64     `json:"project_id,omitempty"`
	ScopeType       string    `json:"scope_type"`
	ScopeID         int64     `json:"scope_id,omitempty"`
	DimensionType   string    `json:"dimension_type"`
	DimensionValue  string    `json:"dimension_value"`
	DetectorType    string    `json:"detector_type"`
	DetectorVersion string    `json:"detector_version"`
	WindowStart     time.Time `json:"window_start"`
	WindowEnd       time.Time `json:"window_end"`

	ObservedSpend    float64 `json:"observed_spend"`
	ExpectedSpend    float64 `json:"expected_spend"`
	SpendDelta       float64 `json:"spend_delta"`
	ObservedRequests int64   `json:"observed_requests"`
	ExpectedRequests float64 `json:"expected_requests"`
	ObservedUnitCost float64 `json:"observed_unit_cost"`
	ExpectedUnitCost float64 `json:"expected_unit_cost"`

	BaselineSampleCount int       `json:"baseline_sample_count"`
	BaselineStart       time.Time `json:"baseline_start"`
	BaselineEnd         time.Time `json:"baseline_end"`
	BaselineMAD         float64   `json:"baseline_mad"`
	RelativeIncrease    float64   `json:"relative_increase"`
	Score               float64   `json:"score"`
	Severity            string    `json:"severity"`
	Fingerprint         string    `json:"fingerprint"`
}

// DetectFinOpsAnomaly evaluates one already-bounded dimension and one detector.
// It has no database or wall-clock side effects, so retries reproduce exactly
// the same evidence and fingerprint.
func DetectFinOpsAnomaly(input FinOpsDetectorInput, cfg FinOpsAnomalyConfig) (FinOpsAnomalyDetection, bool) {
	cfg = normalizeAnomalyConfig(cfg)
	result := FinOpsAnomalyDetection{
		WorkspaceID: input.WorkspaceID, ProjectID: input.ProjectID, ScopeType: input.ScopeType,
		ScopeID: input.ScopeID, DimensionType: input.DimensionType, DimensionValue: input.DimensionValue,
		DetectorType: input.DetectorType, DetectorVersion: detectorVersion(input.DetectorVersion, cfg),
		WindowStart: input.WindowStart.UTC(), WindowEnd: input.WindowEnd.UTC(),
		ObservedSpend: input.ObservedSpend, ExpectedSpend: input.ExpectedSpend,
		SpendDelta:       input.ObservedSpend - input.ExpectedSpend,
		ObservedRequests: input.ObservedRequests, ExpectedRequests: input.ExpectedRequests,
		ObservedUnitCost: input.ObservedUnitCost, ExpectedUnitCost: input.ExpectedUnitCost,
	}
	if input.WindowStart.IsZero() || input.WindowEnd.IsZero() || !input.WindowEnd.After(input.WindowStart) {
		return result, false
	}
	now := input.EvaluationTime
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if input.WindowEnd.After(now.UTC()) {
		return result, false
	}
	if input.WorkspaceID <= 0 || !validAnomalyScope(input.ScopeType) || strings.TrimSpace(input.DimensionValue) == "" {
		return result, false
	}
	if !finiteNonNegative(input.ObservedSpend) || !finiteNonNegative(input.ExpectedSpend) ||
		!finiteNonNegative(input.ObservedUnitCost) || !finiteNonNegative(input.ExpectedUnitCost) || input.ObservedRequests < 0 {
		return result, false
	}
	values := baselineForDetector(input)
	if len(values) < cfg.MinBaselineSamples || !allFiniteNonNegative(values) {
		return result, false
	}
	expected, mad := robustBaseline(values)
	observed := detectorObserved(input)
	if !finiteNonNegative(observed) {
		return result, false
	}
	if input.DetectorType == AnomalyDetectorUnitCostSpike && input.ObservedRequests < cfg.MinObservedRequests {
		return result, false
	}
	if input.DetectorType == AnomalyDetectorRequestSpike && input.ObservedRequests < cfg.MinObservedRequests {
		return result, false
	}
	if !finiteNonNegative(expected) {
		return result, false
	}
	delta := observed - expected
	if delta <= 0 {
		return result, false
	}
	floor := absoluteFloorForDetector(input.DetectorType, cfg)
	relative := relativeIncrease(observed, expected, floor)
	score := robustScore(observed, expected, mad, floor)
	if !finiteNonNegative(score) || score < cfg.MinScore {
		return result, false
	}
	if !passesAnomalyFloors(input, cfg, observed, expected, delta, relative) {
		return result, false
	}
	result.BaselineSampleCount = len(values)
	result.BaselineMAD = mad
	result.RelativeIncrease = relative
	result.Score = score
	result.Severity = anomalySeverity(score, delta, relative, len(values), input.DetectorType)
	result.Fingerprint = FinOpsAnomalyFingerprint(input, result.DetectorVersion)
	switch input.DetectorType {
	case AnomalyDetectorSpendSpike:
		result.ExpectedSpend = expected
		result.SpendDelta = input.ObservedSpend - expected
	case AnomalyDetectorRequestSpike:
		result.ExpectedRequests = expected
	case AnomalyDetectorUnitCostSpike:
		result.ExpectedUnitCost = expected
	}
	return result, true
}

func FinOpsAnomalyFingerprint(input FinOpsDetectorInput, version string) string {
	if version == "" {
		version = FinOpsAnomalyDetectorVersion
	}
	canonical := fmt.Sprintf("%d|%d|%s|%d|%s|%s|%s|%s|%s",
		input.WorkspaceID, input.ProjectID, input.ScopeType, input.ScopeID,
		input.DimensionType, input.DimensionValue, input.DetectorType, version,
		input.WindowStart.UTC().Format(time.RFC3339Nano)+"/"+input.WindowEnd.UTC().Format(time.RFC3339Nano))
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

func normalizeAnomalyConfig(cfg FinOpsAnomalyConfig) FinOpsAnomalyConfig {
	defaults := DefaultFinOpsAnomalyConfig()
	if cfg.DetectorVersion == "" {
		cfg.DetectorVersion = defaults.DetectorVersion
	}
	if cfg.LookbackDays <= 0 {
		cfg.LookbackDays = defaults.LookbackDays
	}
	if cfg.Grace <= 0 {
		cfg.Grace = defaults.Grace
	}
	if cfg.MinBaselineSamples <= 0 {
		cfg.MinBaselineSamples = defaults.MinBaselineSamples
	}
	if cfg.MinObservedRequests <= 0 {
		cfg.MinObservedRequests = defaults.MinObservedRequests
	}
	if cfg.MinRequestDelta <= 0 {
		cfg.MinRequestDelta = defaults.MinRequestDelta
	}
	if cfg.MinScore <= 0 {
		cfg.MinScore = defaults.MinScore
	}
	if cfg.SpendAbsoluteFloor <= 0 {
		cfg.SpendAbsoluteFloor = defaults.SpendAbsoluteFloor
	}
	if cfg.SpendRelativeFloor <= 0 {
		cfg.SpendRelativeFloor = defaults.SpendRelativeFloor
	}
	if cfg.RequestRelativeFloor <= 0 {
		cfg.RequestRelativeFloor = defaults.RequestRelativeFloor
	}
	if cfg.UnitCostAbsoluteFloor <= 0 {
		cfg.UnitCostAbsoluteFloor = defaults.UnitCostAbsoluteFloor
	}
	if cfg.UnitCostRelativeFloor <= 0 {
		cfg.UnitCostRelativeFloor = defaults.UnitCostRelativeFloor
	}
	if cfg.CandidateCap <= 0 {
		cfg.CandidateCap = defaults.CandidateCap
	}
	if cfg.CandidateCap > 1000 {
		cfg.CandidateCap = 1000
	}
	if cfg.MaxRollupRows <= 0 {
		cfg.MaxRollupRows = defaults.MaxRollupRows
	}
	if cfg.MaxRollupRows > 10000 {
		cfg.MaxRollupRows = 10000
	}
	if cfg.ScanWorkspaceBatch <= 0 {
		cfg.ScanWorkspaceBatch = defaults.ScanWorkspaceBatch
	}
	if cfg.ScanWorkspaceBatch > 1000 {
		cfg.ScanWorkspaceBatch = 1000
	}
	return cfg
}

func detectorVersion(value string, cfg FinOpsAnomalyConfig) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return cfg.DetectorVersion
}

func validAnomalyScope(value string) bool {
	switch value {
	case AnomalyScopeWorkspace, AnomalyScopeProject, AnomalyScopePlatform, AnomalyScopeModel, AnomalyScopeAPIKey, AnomalyScopeServiceAccount:
		return true
	default:
		return false
	}
}

func baselineForDetector(input FinOpsDetectorInput) []float64 {
	switch input.DetectorType {
	case AnomalyDetectorRequestSpike:
		return input.BaselineRequests
	case AnomalyDetectorUnitCostSpike:
		return input.BaselineUnitCosts
	default:
		return input.BaselineSpend
	}
}

func detectorObserved(input FinOpsDetectorInput) float64 {
	switch input.DetectorType {
	case AnomalyDetectorRequestSpike:
		return float64(input.ObservedRequests)
	case AnomalyDetectorUnitCostSpike:
		return input.ObservedUnitCost
	default:
		return input.ObservedSpend
	}
}

func absoluteFloorForDetector(detector string, cfg FinOpsAnomalyConfig) float64 {
	if detector == AnomalyDetectorUnitCostSpike {
		return cfg.UnitCostAbsoluteFloor
	}
	if detector == AnomalyDetectorRequestSpike {
		return float64(cfg.MinRequestDelta)
	}
	return cfg.SpendAbsoluteFloor
}

func passesAnomalyFloors(input FinOpsDetectorInput, cfg FinOpsAnomalyConfig, observed, expected, delta, relative float64) bool {
	switch input.DetectorType {
	case AnomalyDetectorRequestSpike:
		return input.ObservedRequests >= cfg.MinObservedRequests && delta >= float64(cfg.MinRequestDelta) && relative >= cfg.RequestRelativeFloor
	case AnomalyDetectorUnitCostSpike:
		return observed >= cfg.UnitCostAbsoluteFloor && delta >= cfg.UnitCostAbsoluteFloor && relative >= cfg.UnitCostRelativeFloor
	case AnomalyDetectorSpendSpike:
		return observed >= cfg.SpendAbsoluteFloor && delta >= cfg.SpendAbsoluteFloor && relative >= cfg.SpendRelativeFloor
	default:
		return false
	}
}

func robustBaseline(values []float64) (float64, float64) {
	copyValues := append([]float64(nil), values...)
	sort.Float64s(copyValues)
	median := percentileMedian(copyValues)
	deviations := make([]float64, len(copyValues))
	for i, value := range copyValues {
		deviations[i] = math.Abs(value - median)
	}
	sort.Float64s(deviations)
	return median, percentileMedian(deviations)
}

func percentileMedian(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	middle := len(values) / 2
	if len(values)%2 == 1 {
		return values[middle]
	}
	return (values[middle-1] + values[middle]) / 2
}

func robustScore(observed, expected, mad, floor float64) float64 {
	if mad > 1e-12 {
		return math.Abs(0.6745 * (observed - expected) / mad)
	}
	scale := math.Max(math.Abs(expected)*0.5, floor)
	if scale <= 0 {
		scale = 1
	}
	return math.Abs(observed-expected) / scale
}

func relativeIncrease(observed, expected, floor float64) float64 {
	denominator := math.Max(expected, floor)
	if denominator <= 1e-12 {
		return 0
	}
	return (observed - expected) / denominator
}

func anomalySeverity(score, delta, relative float64, samples int, detector string) string {
	confidence := 1.0
	if samples < 28 {
		confidence = float64(samples) / 28
	}
	if math.IsInf(relative, 1) || (score >= 8 && delta >= 1 && confidence >= 0.42) {
		return AnomalySeverityCritical
	}
	if score >= 6 || (relative >= 4 && delta >= 0.10) {
		return AnomalySeverityHigh
	}
	if score >= 4.5 || relative >= 2 {
		return AnomalySeverityMedium
	}
	return AnomalySeverityLow
}

func finiteNonNegative(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}

func allFiniteNonNegative(values []float64) bool {
	for _, value := range values {
		if !finiteNonNegative(value) {
			return false
		}
	}
	return true
}

var (
	ErrFinOpsAnomalyNotFound          = infraerrors.NotFound("FINOPS_ANOMALY_NOT_FOUND", "finops anomaly not found")
	ErrFinOpsAnomalyVersionConflict   = infraerrors.Conflict("FINOPS_ANOMALY_VERSION_CONFLICT", "finops anomaly changed concurrently")
	ErrFinOpsAnomalyInvalidTransition = infraerrors.BadRequest("FINOPS_ANOMALY_INVALID_TRANSITION", "invalid finops anomaly status transition")
)

type FinOpsAnomalyFinding struct {
	ID int64 `json:"id"`
	FinOpsAnomalyDetection
	SnapshotID           int64      `json:"snapshot_id"`
	Status               string     `json:"status"`
	FirstDetectedAt      time.Time  `json:"first_detected_at"`
	LastDetectedAt       time.Time  `json:"last_detected_at"`
	AcknowledgedAt       *time.Time `json:"acknowledged_at,omitempty"`
	AcknowledgedByUserID *int64     `json:"acknowledged_by_user_id,omitempty"`
	ResolvedAt           *time.Time `json:"resolved_at,omitempty"`
	ResolvedByUserID     *int64     `json:"resolved_by_user_id,omitempty"`
	ResolutionReason     string     `json:"resolution_reason,omitempty"`
	Version              int64      `json:"version"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type FinOpsAnomalyFilter struct {
	Status        string
	Severity      string
	DetectorType  string
	DimensionType string
	Start         *time.Time
	End           *time.Time
	Page          int
	PageSize      int
}

func (f FinOpsAnomalyFilter) Validate() error {
	if (f.Status != "" && f.Status != AnomalyStatusOpen && f.Status != AnomalyStatusAcknowledged && f.Status != AnomalyStatusResolved) ||
		(f.Severity != "" && f.Severity != AnomalySeverityLow && f.Severity != AnomalySeverityMedium && f.Severity != AnomalySeverityHigh && f.Severity != AnomalySeverityCritical) ||
		(f.DetectorType != "" && f.DetectorType != AnomalyDetectorSpendSpike && f.DetectorType != AnomalyDetectorRequestSpike && f.DetectorType != AnomalyDetectorUnitCostSpike) ||
		(f.DimensionType != "" && !validAnomalyScope(f.DimensionType)) {
		return ErrWorkspaceInvalid
	}
	if f.Start != nil && f.End != nil && !f.End.After(*f.Start) {
		return ErrWorkspaceInvalid
	}
	if f.Start != nil && f.Start.IsZero() || f.End != nil && f.End.IsZero() {
		return ErrWorkspaceInvalid
	}
	if f.Start != nil && f.End != nil && f.End.Sub(*f.Start) > 366*24*time.Hour {
		return ErrWorkspaceInvalid
	}
	return nil
}

func (f FinOpsAnomalyFilter) Normalized() FinOpsAnomalyFilter {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = 50
	}
	if f.PageSize > 100 {
		f.PageSize = 100
	}
	if f.Page > math.MaxInt/f.PageSize {
		f.Page = math.MaxInt / f.PageSize
	}
	if f.Status != "" && f.Status != AnomalyStatusOpen && f.Status != AnomalyStatusAcknowledged && f.Status != AnomalyStatusResolved {
		f.Status = ""
	}
	if f.Severity != "" && f.Severity != AnomalySeverityLow && f.Severity != AnomalySeverityMedium && f.Severity != AnomalySeverityHigh && f.Severity != AnomalySeverityCritical {
		f.Severity = ""
	}
	if f.DetectorType != "" && f.DetectorType != AnomalyDetectorSpendSpike && f.DetectorType != AnomalyDetectorRequestSpike && f.DetectorType != AnomalyDetectorUnitCostSpike {
		f.DetectorType = ""
	}
	if f.DimensionType != "" && !validAnomalyScope(f.DimensionType) {
		f.DimensionType = ""
	}
	return f
}

type FinOpsAnomalyPatch struct {
	Status           string `json:"status"`
	ResolutionReason string `json:"resolution_reason"`
	ExpectedVersion  int64  `json:"expected_version"`
}

type FinOpsAnomalyDetectorStatus struct {
	LastSuccessfulScan  *time.Time `json:"last_successful_scan,omitempty"`
	LastProcessedBucket *time.Time `json:"last_processed_bucket,omitempty"`
	LastFailureCode     string     `json:"last_failure_code,omitempty"`
	LagSeconds          int64      `json:"lag_seconds"`
	CandidateCount      int64      `json:"candidate_count"`
	FindingCount        int64      `json:"finding_count"`
	ScanDurationMS      int64      `json:"scan_duration_ms"`
}

var (
	ErrFinOpsAnomalyLeaseLost      = errors.New("finops anomaly lease lost")
	ErrFinOpsAnomalyRetryExhausted = errors.New("finops anomaly retries exhausted")
	ErrFinOpsAnomalyRollupLimit    = errors.New("finops anomaly rollup input limit exceeded")
)

// FinOpsAnomalyRepository is optional on WorkspaceRepository to keep existing
// focused test doubles source-compatible while production enables Phase E.
type FinOpsAnomalyRepository interface {
	ListFinOpsAnomalies(context.Context, FinOpsScope, FinOpsAnomalyFilter) ([]FinOpsAnomalyFinding, int64, error)
	GetFinOpsAnomaly(context.Context, FinOpsScope, int64) (*FinOpsAnomalyFinding, error)
	TransitionFinOpsAnomaly(context.Context, int64, FinOpsScope, int64, FinOpsAnomalyPatch) (*FinOpsAnomalyFinding, error)
	RunFinOpsAnomalyScan(context.Context, time.Time, FinOpsAnomalyConfig) (FinOpsAnomalyDetectorStatus, error)
	GetFinOpsAnomalyStatus(context.Context) (*FinOpsAnomalyDetectorStatus, error)
}

// FinOpsAnomalyWorker is a single process-wide bounded polling loop. The SQL
// lease is the multi-instance guard; this lifecycle wrapper only owns shutdown.
type FinOpsAnomalyWorker struct {
	repo             FinOpsAnomalyRepository
	cfg              FinOpsAnomalyConfig
	interval         time.Duration
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	lifecycle        sync.Mutex
	started, stopped bool
}

func NewFinOpsAnomalyWorker(repo FinOpsAnomalyRepository, cfg FinOpsAnomalyConfig, interval time.Duration) *FinOpsAnomalyWorker {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &FinOpsAnomalyWorker{repo: repo, cfg: normalizeAnomalyConfig(cfg), interval: interval, ctx: ctx, cancel: cancel}
}

func (w *FinOpsAnomalyWorker) Start() {
	if w == nil || w.repo == nil {
		return
	}
	w.lifecycle.Lock()
	defer w.lifecycle.Unlock()
	if w.started || w.stopped {
		return
	}
	w.started = true
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.scan()
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()
		for {
			select {
			case <-w.ctx.Done():
				return
			case <-ticker.C:
				w.scan()
			}
		}
	}()
}

func (w *FinOpsAnomalyWorker) scan() {
	ctx, cancel := context.WithTimeout(w.ctx, 20*time.Second)
	defer cancel()
	_, _ = w.repo.RunFinOpsAnomalyScan(ctx, time.Now().UTC(), w.cfg)
}

func (w *FinOpsAnomalyWorker) Stop() {
	if w == nil {
		return
	}
	w.lifecycle.Lock()
	w.stopped = true
	w.cancel()
	w.lifecycle.Unlock()
	w.wg.Wait()
}
