package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFinOpsAnomalyDomainEventsUseBoundedScalarEvidence(t *testing.T) {
	data := DomainEventData{
		"anomaly_id": "42", "status": AnomalyStatusOpen, "detector_type": AnomalyDetectorSpendSpike,
		"detector_version": FinOpsAnomalyDetectorVersion, "dimension_type": AnomalyScopeModel, "dimension_value": "model-a",
		"severity": AnomalySeverityHigh, "observed_spend": 12.5, "expected_spend": 2.5, "spend_delta": 10.0,
		"observed_requests": int64(42), "expected_requests": 12.0, "observed_unit_cost": 0.3, "expected_unit_cost": 0.2,
		"score": 7.1, "window_start": "2026-10-08T01:00:00Z", "window_end": "2026-10-08T02:00:00Z",
	}
	for _, eventType := range []string{EventFinOpsAnomalyDetected, EventFinOpsAnomalyAcknowledged, EventFinOpsAnomalyResolved} {
		event, err := NewDomainEvent(eventType, 11, 13, 7, "finops_anomaly", "42", data)
		require.NoError(t, err)
		require.True(t, IsWorkspaceVisibleEvent(eventType))
		category, title, body := notificationPresentation(eventType)
		require.Equal(t, "finops", category)
		require.Equal(t, "notifications.finops_anomaly.title", title)
		require.Equal(t, "notifications.finops_anomaly.body", body)
		_, err = event.MarshalPayload()
		require.NoError(t, err)
	}
	_, err := NewDomainEvent(EventFinOpsAnomalyDetected, 11, 0, 0, "finops_anomaly", "42", DomainEventData{"secret": "must-not-pass"})
	require.Error(t, err)
}
