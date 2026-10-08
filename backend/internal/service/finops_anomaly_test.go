package service

import (
	"math"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestDetectFinOpsAnomalyRequiresRobustBaselineAndFloors(t *testing.T) {
	now := time.Date(2026, 10, 8, 13, 0, 0, 0, time.UTC)
	cfg := DefaultFinOpsAnomalyConfig()

	tests := []struct {
		name        string
		kind        string
		observed    float64
		baseline    []float64
		requests    int64
		wantFinding bool
	}{
		{name: "insufficient samples", kind: AnomalyDetectorSpendSpike, observed: 20, baseline: []float64{1, 1, 1}, requests: 100},
		{name: "tiny spend floor", kind: AnomalyDetectorSpendSpike, observed: 0.0001, baseline: repeatedFloats(12, 0.00001), requests: 100},
		{name: "constant baseline large spike", kind: AnomalyDetectorSpendSpike, observed: 10, baseline: repeatedFloats(12, 1), requests: 100, wantFinding: true},
		{name: "high value sparse spend still qualifies", kind: AnomalyDetectorSpendSpike, observed: 10, baseline: repeatedFloats(12, 1), requests: 1, wantFinding: true},
		{name: "request spike", kind: AnomalyDetectorRequestSpike, observed: 100, baseline: repeatedFloats(12, 10), requests: 100, wantFinding: true},
		{name: "unit cost spike", kind: AnomalyDetectorUnitCostSpike, observed: 0.20, baseline: repeatedFloats(12, 0.05), requests: 100, wantFinding: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := FinOpsDetectorInput{
				WorkspaceID:      1,
				ScopeType:        AnomalyScopeWorkspace,
				DimensionType:    AnomalyScopeWorkspace,
				DimensionValue:   "1",
				DetectorType:     tt.kind,
				WindowStart:      now.Add(-time.Hour),
				WindowEnd:        now,
				EvaluationTime:   now.Add(5 * time.Minute),
				ObservedSpend:    tt.observed,
				ObservedRequests: tt.requests,
				ObservedUnitCost: func() float64 {
					if tt.kind == AnomalyDetectorUnitCostSpike {
						return tt.observed
					}
					return 0
				}(),
				BaselineSpend:     tt.baseline,
				BaselineRequests:  tt.baseline,
				BaselineUnitCosts: tt.baseline,
			}
			got, ok := DetectFinOpsAnomaly(input, cfg)
			require.Equal(t, tt.wantFinding, ok)
			if ok {
				require.NotEmpty(t, got.Fingerprint)
				require.NotEmpty(t, got.Severity)
				require.GreaterOrEqual(t, got.Score, cfg.MinScore)
				require.False(t, got.WindowEnd.After(now))
			}
		})
	}
}

func TestDetectFinOpsAnomalyRejectsNonFiniteAndIncompleteBuckets(t *testing.T) {
	cfg := DefaultFinOpsAnomalyConfig()
	base := repeatedFloats(cfg.MinBaselineSamples, 1)
	for _, observed := range []float64{math.NaN(), math.Inf(1)} {
		input := FinOpsDetectorInput{
			DetectorType:      AnomalyDetectorSpendSpike,
			WindowStart:       time.Now().UTC().Add(-time.Hour),
			WindowEnd:         time.Now().UTC().Add(time.Hour),
			ObservedSpend:     observed,
			ObservedRequests:  100,
			BaselineSpend:     base,
			BaselineRequests:  base,
			BaselineUnitCosts: base,
		}
		_, ok := DetectFinOpsAnomaly(input, cfg)
		require.False(t, ok)
	}
}

func TestFinOpsAnomalyFingerprintIsDeterministicAndSeverityUsesAbsoluteDelta(t *testing.T) {
	cfg := DefaultFinOpsAnomalyConfig()
	input := FinOpsDetectorInput{
		WorkspaceID:       11,
		ProjectID:         13,
		ScopeType:         AnomalyScopeModel,
		ScopeID:           0,
		DimensionType:     AnomalyScopeModel,
		DimensionValue:    "model-a",
		DetectorType:      AnomalyDetectorSpendSpike,
		DetectorVersion:   FinOpsAnomalyDetectorVersion,
		WindowStart:       time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC),
		WindowEnd:         time.Date(2026, 10, 8, 13, 0, 0, 0, time.UTC),
		EvaluationTime:    time.Date(2026, 10, 8, 13, 5, 0, 0, time.UTC),
		ObservedSpend:     100,
		ObservedRequests:  100,
		ObservedUnitCost:  1,
		BaselineSpend:     repeatedFloats(cfg.MinBaselineSamples, 10),
		BaselineRequests:  repeatedFloats(cfg.MinBaselineSamples, 10),
		BaselineUnitCosts: repeatedFloats(cfg.MinBaselineSamples, 1),
	}
	first, ok := DetectFinOpsAnomaly(input, cfg)
	require.True(t, ok)
	second, ok := DetectFinOpsAnomaly(input, cfg)
	require.True(t, ok)
	require.Equal(t, first.Fingerprint, second.Fingerprint)
	require.Equal(t, AnomalySeverityCritical, first.Severity)
	require.InDelta(t, 90, first.SpendDelta, 0.000001)
}

func repeatedFloats(n int, value float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = value
	}
	return out
}

func TestFinOpsAnomalyWorkflowErrorsExposeStableHTTPSemantics(t *testing.T) {
	require.Equal(t, 404, infraerrors.Code(ErrFinOpsAnomalyNotFound))
	require.Equal(t, 409, infraerrors.Code(ErrFinOpsAnomalyVersionConflict))
	require.Equal(t, 400, infraerrors.Code(ErrFinOpsAnomalyInvalidTransition))
}
