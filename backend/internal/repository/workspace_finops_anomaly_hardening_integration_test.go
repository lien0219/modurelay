//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func assertAnomalyEvidenceCounts(t *testing.T, fixture anomalyWorkspaceFixture, want int) {
	t.Helper()
	var snapshots, findings, events, outbox int
	require.NoError(t, integrationDB.QueryRowContext(fixture.ctx, `SELECT
	 (SELECT count(*) FROM finops_anomaly_snapshots WHERE workspace_id=$1),
	 (SELECT count(*) FROM finops_anomaly_findings WHERE workspace_id=$1),
	 (SELECT count(*) FROM domain_events WHERE workspace_id=$1 AND event_type='finops.anomaly.detected'),
	 (SELECT count(*) FROM domain_event_outbox o JOIN domain_events e ON e.id=o.event_id WHERE e.workspace_id=$1 AND e.event_type='finops.anomaly.detected')`, fixture.ws.ID).Scan(&snapshots, &findings, &events, &outbox))
	require.Equal(t, want, snapshots)
	require.Equal(t, want, findings)
	require.Equal(t, want, events)
	require.Equal(t, want, outbox)
}

func expireAnomalyClaim(t *testing.T, fixture anomalyWorkspaceFixture, bucket time.Time) {
	t.Helper()
	_, err := integrationDB.ExecContext(fixture.ctx, `UPDATE finops_anomaly_detection_leases SET claimed_until=clock_timestamp()-interval '1 second' WHERE workspace_id=$1 AND bucket_start=$2 AND detector_version=$3`, fixture.ws.ID, bucket, service.FinOpsAnomalyDetectorVersion)
	require.NoError(t, err)
}

func TestFinOpsAnomalyPostgresExpiredSameTokenCannotFinish(t *testing.T) {
	fixture := newAnomalyWorkspaceFixture(t)
	for i, failure := range []error{nil, errors.New("synthetic failure")} {
		bucket := time.Date(2026, 10, 9, i, 0, 0, 0, time.UTC)
		claimed, token, err := fixture.repo.claimAnomalyLease(fixture.ctx, fixture.ws.ID, bucket, service.FinOpsAnomalyDetectorVersion)
		require.NoError(t, err)
		require.True(t, claimed)
		expireAnomalyClaim(t, fixture, bucket)
		require.ErrorIs(t, fixture.repo.finishAnomalyLease(fixture.ctx, fixture.ws.ID, bucket, service.FinOpsAnomalyDetectorVersion, token, failure), service.ErrFinOpsAnomalyLeaseLost, "expiry must revoke success and failure writes even without takeover")
		var completed, tokenUnchanged bool
		require.NoError(t, integrationDB.QueryRowContext(fixture.ctx, `SELECT completed_at IS NOT NULL,lease_token=$4 FROM finops_anomaly_detection_leases WHERE workspace_id=$1 AND bucket_start=$2 AND detector_version=$3`, fixture.ws.ID, bucket, service.FinOpsAnomalyDetectorVersion, token).Scan(&completed, &tokenUnchanged))
		require.False(t, completed)
		require.True(t, tokenUnchanged)
	}
}

func TestFinOpsAnomalyPostgresExpiredWorkerCannotPersistEvidence(t *testing.T) {
	fixture := newAnomalyWorkspaceFixture(t)
	bucket := time.Date(2026, 10, 9, 3, 0, 0, 0, time.UTC)
	claimed, token, err := fixture.repo.claimAnomalyLease(fixture.ctx, fixture.ws.ID, bucket, service.FinOpsAnomalyDetectorVersion)
	require.NoError(t, err)
	require.True(t, claimed)
	expireAnomalyClaim(t, fixture, bucket)
	detection := anomalyDetectionFixture(fixture.ws.ID, fixture.project.ID, service.AnomalyDetectorSpendSpike, service.AnomalyScopeProject, "project", bucket)
	created, err := fixture.repo.persistAnomalyDetection(fixture.ctx, detection, bucket.Add(-28*24*time.Hour), bucket, token)
	require.ErrorIs(t, err, service.ErrFinOpsAnomalyLeaseLost)
	require.False(t, created)
	var count int
	require.NoError(t, integrationDB.QueryRowContext(fixture.ctx, `SELECT count(*) FROM finops_anomaly_snapshots WHERE workspace_id=$1`, fixture.ws.ID).Scan(&count))
	require.Zero(t, count)
}

func TestFinOpsAnomalyPostgresDiscoverySkipsLiveClaims(t *testing.T) {
	fixture := newAnomalyWorkspaceFixture(t)
	bucket := time.Date(2026, 10, 9, 4, 0, 0, 0, time.UTC)
	_, err := integrationDB.ExecContext(fixture.ctx, `INSERT INTO usage_tenant_hourly_rollups(workspace_id,project_id,bucket_start,api_key_id,resolved_platform,model,request_count,actual_cost) VALUES($1,$2,$3,991,'openai','bounded',10,1)`, fixture.ws.ID, fixture.project.ID, bucket)
	require.NoError(t, err)
	claimed, _, err := fixture.repo.claimAnomalyLease(fixture.ctx, fixture.ws.ID, bucket, service.FinOpsAnomalyDetectorVersion)
	require.NoError(t, err)
	require.True(t, claimed)
	ids, err := fixture.repo.listAnomalyWorkspaceIDs(fixture.ctx, bucket, service.FinOpsAnomalyDetectorVersion, 1)
	require.NoError(t, err)
	require.NotContains(t, ids, fixture.ws.ID, "a live first claim must not occupy every bounded discovery page")
}

func TestFinOpsAnomalyPostgresRawRollupLimitRejectsPartialEvidence(t *testing.T) {
	fixture := newAnomalyWorkspaceFixture(t)
	bucket := time.Date(2026, 10, 9, 5, 0, 0, 0, time.UTC)
	_, err := integrationDB.ExecContext(fixture.ctx, `INSERT INTO usage_tenant_hourly_rollups(workspace_id,project_id,bucket_start,api_key_id,resolved_platform,model,request_count,actual_cost) VALUES($1,$2,$3,991,'openai','bounded',10,1),($1,$2,$3,992,'openai','bounded',10,1)`, fixture.ws.ID, fixture.project.ID, bucket)
	require.NoError(t, err)
	cfg := service.DefaultFinOpsAnomalyConfig()
	cfg.MaxRollupRows = 1
	candidates, err := fixture.repo.scanAnomalyWorkspace(fixture.ctx, fixture.ws.ID, bucket, cfg)
	require.Error(t, err, "the raw input limit must apply before aggregate output limits")
	require.Empty(t, candidates)

	_, err = fixture.repo.anomalySamples(fixture.ctx, fixture.ws.ID, bucket.Add(24*time.Hour), anomalyCandidate{dimensionType: service.AnomalyScopeWorkspace}, cfg)
	require.Error(t, err, "one grouped baseline bucket still exceeds a one-row raw input limit")
}

func TestFinOpsAnomalyPostgresStatusCannotRegressOnDelayedWorker(t *testing.T) {
	fixture := newAnomalyWorkspaceFixture(t)
	_, err := integrationDB.ExecContext(fixture.ctx, `DELETE FROM finops_anomaly_detector_status`)
	require.NoError(t, err)
	older := time.Date(2026, 10, 9, 6, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	require.NoError(t, fixture.repo.updateAnomalyDetectorStatus(context.Background(), service.FinOpsAnomalyDetectorStatus{LastProcessedBucket: &newer, LastSuccessfulScan: &newer}, newer, nil))
	require.NoError(t, fixture.repo.updateAnomalyDetectorStatus(context.Background(), service.FinOpsAnomalyDetectorStatus{LastProcessedBucket: &older, LastSuccessfulScan: &older}, older, nil))
	status, err := fixture.repo.GetFinOpsAnomalyStatus(fixture.ctx)
	require.NoError(t, err)
	require.True(t, status.LastProcessedBucket.Equal(newer))
	require.True(t, status.LastSuccessfulScan.Equal(newer))
}

func TestFinOpsAnomalyPostgresTakeoverAndCrashReplayKeepOriginalEvidence(t *testing.T) {
	fixture := newAnomalyWorkspaceFixture(t)
	bucket := time.Date(2026, 10, 9, 7, 0, 0, 0, time.UTC)
	claimed, staleToken, err := fixture.repo.claimAnomalyLease(fixture.ctx, fixture.ws.ID, bucket, service.FinOpsAnomalyDetectorVersion)
	require.NoError(t, err)
	require.True(t, claimed)
	expireAnomalyClaim(t, fixture, bucket)
	claimed, currentToken, err := fixture.repo.claimAnomalyLease(fixture.ctx, fixture.ws.ID, bucket, service.FinOpsAnomalyDetectorVersion)
	require.NoError(t, err)
	require.True(t, claimed)
	detection := anomalyDetectionFixture(fixture.ws.ID, fixture.project.ID, service.AnomalyDetectorSpendSpike, service.AnomalyScopeProject, "project", bucket)
	created, err := fixture.repo.persistAnomalyDetection(fixture.ctx, detection, bucket.Add(-28*24*time.Hour), bucket, staleToken)
	require.ErrorIs(t, err, service.ErrFinOpsAnomalyLeaseLost)
	require.False(t, created)
	assertAnomalyEvidenceCounts(t, fixture, 0)
	created, err = fixture.repo.persistAnomalyDetection(fixture.ctx, detection, bucket.Add(-28*24*time.Hour), bucket, currentToken)
	require.NoError(t, err)
	require.True(t, created)
	var originalEventID string
	var originalPayload []byte
	require.NoError(t, integrationDB.QueryRowContext(fixture.ctx, `SELECT id,payload FROM domain_events WHERE dedupe_key=$1`, "finops.anomaly.detected:"+detection.Fingerprint).Scan(&originalEventID, &originalPayload))

	// Evidence committed, but the worker disappeared before lease completion.
	expireAnomalyClaim(t, fixture, bucket)
	claimed, replayToken, err := fixture.repo.claimAnomalyLease(fixture.ctx, fixture.ws.ID, bucket, service.FinOpsAnomalyDetectorVersion)
	require.NoError(t, err)
	require.True(t, claimed)
	detection.ObservedSpend = 99 // A replay cannot replace creation-time evidence.
	created, err = fixture.repo.persistAnomalyDetection(fixture.ctx, detection, bucket.Add(-28*24*time.Hour), bucket, replayToken)
	require.NoError(t, err)
	require.False(t, created)
	assertAnomalyEvidenceCounts(t, fixture, 1)
	var replayEventID string
	var replayPayload []byte
	var spend float64
	require.NoError(t, integrationDB.QueryRowContext(fixture.ctx, `SELECT id,payload FROM domain_events WHERE dedupe_key=$1`, "finops.anomaly.detected:"+detection.Fingerprint).Scan(&replayEventID, &replayPayload))
	require.NoError(t, integrationDB.QueryRowContext(fixture.ctx, `SELECT observed_spend FROM finops_anomaly_snapshots WHERE fingerprint=$1`, detection.Fingerprint).Scan(&spend))
	require.Equal(t, originalEventID, replayEventID)
	require.JSONEq(t, string(originalPayload), string(replayPayload))
	require.Equal(t, float64(10), spend)
	require.ErrorIs(t, fixture.repo.finishAnomalyLease(fixture.ctx, fixture.ws.ID, bucket, service.FinOpsAnomalyDetectorVersion, currentToken, nil), service.ErrFinOpsAnomalyLeaseLost)
	require.NoError(t, fixture.repo.finishAnomalyLease(fixture.ctx, fixture.ws.ID, bucket, service.FinOpsAnomalyDetectorVersion, replayToken, nil))
	_, err = fixture.repo.persistAnomalyDetection(fixture.ctx, detection, bucket.Add(-28*24*time.Hour), bucket, replayToken)
	require.ErrorIs(t, err, service.ErrFinOpsAnomalyLeaseLost, "completed claims cannot authorize even the duplicate path")
}

func TestFinOpsAnomalyPostgresEvidenceWriteFailuresRollbackTogether(t *testing.T) {
	for _, table := range []string{"finops_anomaly_snapshots", "finops_anomaly_findings", "domain_events", "domain_event_outbox"} {
		t.Run(table, func(t *testing.T) {
			fixture := newAnomalyWorkspaceFixture(t)
			bucket := time.Date(2026, 10, 9, 8, 0, 0, 0, time.UTC)
			claimed, token, err := fixture.repo.claimAnomalyLease(fixture.ctx, fixture.ws.ID, bucket, service.FinOpsAnomalyDetectorVersion)
			require.NoError(t, err)
			require.True(t, claimed)
			_, err = integrationDB.ExecContext(fixture.ctx, `CREATE FUNCTION phase_i_anomaly_write_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic write failure'; END $$`)
			require.NoError(t, err)
			_, err = integrationDB.ExecContext(fixture.ctx, fmt.Sprintf(`CREATE TRIGGER phase_i_anomaly_write_failure BEFORE INSERT ON %s FOR EACH ROW EXECUTE FUNCTION phase_i_anomaly_write_failure()`, table))
			require.NoError(t, err)
			detection := anomalyDetectionFixture(fixture.ws.ID, fixture.project.ID, service.AnomalyDetectorSpendSpike, service.AnomalyScopeProject, "project", bucket)
			created, err := fixture.repo.persistAnomalyDetection(fixture.ctx, detection, bucket.Add(-28*24*time.Hour), bucket, token)
			require.Error(t, err)
			require.False(t, created)
			assertAnomalyEvidenceCounts(t, fixture, 0)
			_, err = integrationDB.ExecContext(fixture.ctx, fmt.Sprintf(`DROP TRIGGER phase_i_anomaly_write_failure ON %s`, table))
			require.NoError(t, err)
			created, err = fixture.repo.persistAnomalyDetection(fixture.ctx, detection, bucket.Add(-28*24*time.Hour), bucket, token)
			require.NoError(t, err)
			require.True(t, created)
			assertAnomalyEvidenceCounts(t, fixture, 1)
		})
	}
}

func TestFinOpsAnomalyPostgresTransactionCrossingExpiryRollsBackEvidence(t *testing.T) {
	fixture := newAnomalyWorkspaceFixture(t)
	bucket := time.Date(2026, 10, 9, 9, 0, 0, 0, time.UTC)
	claimed, token, err := fixture.repo.claimAnomalyLease(fixture.ctx, fixture.ws.ID, bucket, service.FinOpsAnomalyDetectorVersion)
	require.NoError(t, err)
	require.True(t, claimed)
	_, err = integrationDB.ExecContext(fixture.ctx, `CREATE FUNCTION phase_i_slow_anomaly_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_sleep(0.6); RETURN NEW; END $$;
	 CREATE TRIGGER phase_i_slow_anomaly_write BEFORE INSERT ON finops_anomaly_snapshots FOR EACH ROW EXECUTE FUNCTION phase_i_slow_anomaly_write()`)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(fixture.ctx, `UPDATE finops_anomaly_detection_leases SET claimed_until=clock_timestamp()+interval '0.3 seconds' WHERE workspace_id=$1 AND bucket_start=$2`, fixture.ws.ID, bucket)
	require.NoError(t, err)
	detection := anomalyDetectionFixture(fixture.ws.ID, fixture.project.ID, service.AnomalyDetectorSpendSpike, service.AnomalyScopeProject, "project", bucket)
	started := time.Now()
	created, err := fixture.repo.persistAnomalyDetection(fixture.ctx, detection, bucket.Add(-28*24*time.Hour), bucket, token)
	require.GreaterOrEqual(t, time.Since(started), 600*time.Millisecond, "the live transaction must reach the delayed INSERT before expiry")
	require.ErrorIs(t, err, service.ErrFinOpsAnomalyLeaseLost)
	require.False(t, created)
	assertAnomalyEvidenceCounts(t, fixture, 0)
}

func TestFinOpsAnomalyPostgresPoisonBackoffDoesNotStarveHealthyWorkspace(t *testing.T) {
	poison := newAnomalyWorkspaceFixture(t)
	healthy := newAnomalyWorkspaceFixture(t)
	bucket := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	_, err := integrationDB.ExecContext(poison.ctx, `DELETE FROM finops_anomaly_detector_status`)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(poison.ctx, `INSERT INTO usage_tenant_hourly_rollups(workspace_id,project_id,bucket_start,api_key_id,resolved_platform,model,request_count,actual_cost)
	 VALUES($1,$2,$5,991,'openai','bounded',10,1),($1,$2,$5,992,'openai','bounded',10,1),($3,$4,$5,993,'openai','bounded',10,1)`, poison.ws.ID, poison.project.ID, healthy.ws.ID, healthy.project.ID, bucket)
	require.NoError(t, err)
	cfg := service.DefaultFinOpsAnomalyConfig()
	cfg.MaxRollupRows, cfg.ScanWorkspaceBatch = 1, 1
	now := bucket.Add(time.Hour + cfg.Grace)
	_, err = poison.repo.RunFinOpsAnomalyScan(poison.ctx, now, cfg)
	require.ErrorIs(t, err, service.ErrFinOpsAnomalyRollupLimit)
	claimed, _, err := poison.repo.claimAnomalyLease(poison.ctx, poison.ws.ID, bucket, cfg.DetectorVersion)
	require.NoError(t, err)
	require.False(t, claimed, "failed work must retain a durable retry delay")
	status, err := poison.repo.RunFinOpsAnomalyScan(poison.ctx, now, cfg)
	require.NoError(t, err)
	require.Nil(t, status.LastProcessedBucket, "pending delayed work still prevents pretending the bucket succeeded")
	var complete bool
	require.NoError(t, integrationDB.QueryRowContext(poison.ctx, `SELECT completed_at IS NOT NULL FROM finops_anomaly_detection_leases WHERE workspace_id=$1 AND bucket_start=$2`, healthy.ws.ID, bucket).Scan(&complete))
	require.True(t, complete, "healthy work after the poisoned first ID must progress")
	for attempt := 2; attempt <= anomalyMaxAttempts; attempt++ {
		_, err = integrationDB.ExecContext(poison.ctx, `UPDATE finops_anomaly_detection_leases SET available_at=clock_timestamp()-interval '1 second' WHERE workspace_id=$1 AND bucket_start=$2`, poison.ws.ID, bucket)
		require.NoError(t, err)
		status, err = poison.repo.RunFinOpsAnomalyScan(poison.ctx, now, cfg)
		require.Error(t, err)
	}
	var failed bool
	var attempts int
	require.NoError(t, integrationDB.QueryRowContext(poison.ctx, `SELECT completed_at IS NOT NULL,failed_at IS NOT NULL,attempts FROM finops_anomaly_detection_leases WHERE workspace_id=$1 AND bucket_start=$2`, poison.ws.ID, bucket).Scan(&complete, &failed, &attempts))
	require.False(t, complete)
	require.True(t, failed)
	require.Equal(t, anomalyMaxAttempts, attempts)
	require.True(t, status.LastProcessedBucket.Equal(bucket))
	require.Nil(t, status.LastSuccessfulScan, "a terminal failed unit is never a successful scan")
	require.Equal(t, "retry_exhausted", status.LastFailureCode)
	assertAnomalyEvidenceCounts(t, poison, 0)
	status, err = poison.repo.RunFinOpsAnomalyScan(poison.ctx, now.Add(time.Hour), cfg)
	require.NoError(t, err)
	require.True(t, status.LastProcessedBucket.Equal(bucket.Add(time.Hour)))
	require.Equal(t, "retry_exhausted", status.LastFailureCode, "terminal failure evidence survives future cursor progress")
}

func TestFinOpsAnomalyPostgresFifthWorkerCrashBecomesVisibleFailure(t *testing.T) {
	fixture := newAnomalyWorkspaceFixture(t)
	bucket := time.Date(2026, 10, 9, 11, 0, 0, 0, time.UTC)
	_, err := integrationDB.ExecContext(fixture.ctx, `DELETE FROM finops_anomaly_detector_status`)
	require.NoError(t, err)
	claimed, _, err := fixture.repo.claimAnomalyLease(fixture.ctx, fixture.ws.ID, bucket, service.FinOpsAnomalyDetectorVersion)
	require.NoError(t, err)
	require.True(t, claimed)
	_, err = integrationDB.ExecContext(fixture.ctx, `UPDATE finops_anomaly_detection_leases SET attempts=5,claimed_until=clock_timestamp()-interval '1 second' WHERE workspace_id=$1 AND bucket_start=$2`, fixture.ws.ID, bucket)
	require.NoError(t, err)
	status, err := fixture.repo.RunFinOpsAnomalyScan(fixture.ctx, bucket.Add(time.Hour+5*time.Minute), service.DefaultFinOpsAnomalyConfig())
	require.ErrorIs(t, err, service.ErrFinOpsAnomalyRetryExhausted)
	require.Nil(t, status.LastSuccessfulScan)
	require.True(t, status.LastProcessedBucket.Equal(bucket))
	var failed bool
	require.NoError(t, integrationDB.QueryRowContext(fixture.ctx, `SELECT failed_at IS NOT NULL FROM finops_anomaly_detection_leases WHERE workspace_id=$1 AND bucket_start=$2`, fixture.ws.ID, bucket).Scan(&failed))
	require.True(t, failed)
}
