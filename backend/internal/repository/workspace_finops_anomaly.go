package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

const (
	anomalyLeaseDuration    = 2 * time.Minute
	anomalyWorkspaceTimeout = 3 * time.Second
	anomalyFinishTimeout    = 3 * time.Second
	anomalyMaxAttempts      = 5
)

type anomalyCandidate struct {
	scopeType      string
	scopeID        int64
	projectID      int64
	dimensionType  string
	dimensionValue string
	requests       int64
	spend          float64
	serviceAccount bool
}

type anomalyRollupSample struct {
	bucket   time.Time
	requests int64
	spend    float64
}

func anomalyConfigWithDefaults(cfg service.FinOpsAnomalyConfig) service.FinOpsAnomalyConfig {
	defaults := service.DefaultFinOpsAnomalyConfig()
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

func completedAnomalyBucket(now time.Time, grace time.Duration) (time.Time, bool) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	latest := now.Truncate(time.Hour).Add(-time.Hour)
	if now.Sub(latest.Add(time.Hour)) < grace {
		latest = latest.Add(-time.Hour)
	}
	return latest, latest.Add(time.Hour).Add(grace).Before(now) || latest.Add(time.Hour).Add(grace).Equal(now)
}

func (r *workspaceRepository) RunFinOpsAnomalyScan(ctx context.Context, now time.Time, cfg service.FinOpsAnomalyConfig) (service.FinOpsAnomalyDetectorStatus, error) {
	cfg = anomalyConfigWithDefaults(cfg)
	started := time.Now().UTC()
	latestBucket, ready := completedAnomalyBucket(now, cfg.Grace)
	if !ready {
		return service.FinOpsAnomalyDetectorStatus{}, nil
	}
	status, err := r.GetFinOpsAnomalyStatus(ctx)
	if err != nil {
		return service.FinOpsAnomalyDetectorStatus{}, err
	}
	statusValue := *status
	bucket := latestBucket
	if status.LastProcessedBucket != nil {
		bucket = status.LastProcessedBucket.Add(time.Hour)
		if bucket.After(latestBucket) {
			return statusValue, nil
		}
	}
	if err = r.reapAnomalyLeases(ctx, bucket, cfg.DetectorVersion, cfg.ScanWorkspaceBatch); err != nil {
		return statusValue, err
	}
	workspaceIDs, err := r.listAnomalyWorkspaceIDs(ctx, bucket, cfg.DetectorVersion, cfg.ScanWorkspaceBatch)
	if err != nil {
		_ = r.updateAnomalyDetectorStatus(ctx, statusValue, time.Time{}, err)
		return statusValue, err
	}
	var firstErr error
	for _, workspaceID := range workspaceIDs {
		if err := ctx.Err(); err != nil {
			firstErr = errors.Join(firstErr, err)
			break
		}
		claimed, leaseToken, claimErr := r.claimAnomalyLease(ctx, workspaceID, bucket, cfg.DetectorVersion)
		if claimErr != nil {
			if firstErr == nil {
				firstErr = claimErr
			}
			continue
		}
		if !claimed {
			continue
		}
		workspaceFailed := false
		unitCtx, cancelUnit := context.WithTimeout(ctx, anomalyWorkspaceTimeout)
		candidates, scanErr := r.scanAnomalyWorkspace(unitCtx, workspaceID, bucket, cfg)
		statusValue.CandidateCount += int64(len(candidates))
		if scanErr != nil {
			workspaceFailed = true
			cancelUnit()
			finish, cancelFinish := context.WithTimeout(context.WithoutCancel(ctx), anomalyFinishTimeout)
			finishErr := r.finishAnomalyLease(finish, workspaceID, bucket, cfg.DetectorVersion, leaseToken, scanErr)
			cancelFinish()
			if firstErr == nil {
				firstErr = errors.Join(scanErr, finishErr)
			}
			continue
		}
		for _, candidate := range candidates {
			created, createErr := r.detectAnomalyCandidate(unitCtx, workspaceID, bucket, candidate, cfg, leaseToken)
			if createErr != nil {
				workspaceFailed = true
				finish, cancelFinish := context.WithTimeout(context.WithoutCancel(ctx), anomalyFinishTimeout)
				finishErr := r.finishAnomalyLease(finish, workspaceID, bucket, cfg.DetectorVersion, leaseToken, createErr)
				cancelFinish()
				if firstErr == nil {
					firstErr = errors.Join(createErr, finishErr)
				}
				break
			}
			statusValue.FindingCount += int64(created)
		}
		cancelUnit()
		if !workspaceFailed {
			if err := r.finishAnomalyLease(ctx, workspaceID, bucket, cfg.DetectorVersion, leaseToken, nil); err != nil {
				if firstErr == nil {
					firstErr = err
				}
			}
		}
	}
	pending, pendingErr := r.hasPendingAnomalyWorkspace(ctx, bucket, cfg.DetectorVersion)
	if pendingErr != nil {
		firstErr = errors.Join(firstErr, pendingErr)
	}
	failed, failedErr := r.hasFailedAnomalyWorkspace(ctx, bucket, cfg.DetectorVersion)
	if failedErr != nil {
		firstErr = errors.Join(firstErr, failedErr)
	}
	if failed {
		firstErr = errors.Join(firstErr, service.ErrFinOpsAnomalyRetryExhausted)
	}
	// Explicitly failed units are processed, never successfully completed. Their
	// immutable evidence and durable error remain while healthy future work proceeds.
	advanced := pendingErr == nil && !pending && ctx.Err() == nil && (firstErr == nil || failed)
	if advanced {
		if firstErr == nil {
			statusValue.LastSuccessfulScan = timePtr(time.Now().UTC())
		}
		statusValue.LastProcessedBucket = timePtr(bucket)
	}
	lag := time.Since(bucket.Add(time.Hour).Add(cfg.Grace))
	if lag > 0 {
		statusValue.LagSeconds = int64(lag / time.Second)
	}
	statusValue.ScanDurationMS = time.Since(started).Milliseconds()
	if firstErr != nil {
		statusValue.LastFailureCode = anomalyFailureCode(firstErr)
	}
	statusBucket := time.Time{}
	if advanced {
		statusBucket = bucket
	}
	if err := r.updateAnomalyDetectorStatus(ctx, statusValue, statusBucket, firstErr); err != nil {
		if firstErr == nil {
			firstErr = err
		}
	}
	return statusValue, firstErr
}

func timePtr(value time.Time) *time.Time { return &value }

func anomalyFailureCode(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, service.ErrFinOpsAnomalyRetryExhausted) {
		return "retry_exhausted"
	}
	if errors.Is(err, service.ErrFinOpsAnomalyLeaseLost) {
		return "lease_lost"
	}
	if errors.Is(err, service.ErrFinOpsAnomalyRollupLimit) {
		return "rollup_limit"
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	return "database_error"
}

func (r *workspaceRepository) listAnomalyWorkspaceIDs(ctx context.Context, bucket time.Time, detectorVersion string, limit int) ([]int64, error) {
	if limit < 1 || limit > 1000 {
		limit = service.DefaultFinOpsAnomalyConfig().ScanWorkspaceBatch
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT workspace_id FROM usage_tenant_hourly_rollups u WHERE bucket_start=$1
		  AND NOT EXISTS (SELECT 1 FROM finops_anomaly_detection_leases l WHERE l.workspace_id=u.workspace_id AND l.bucket_start=$1 AND l.detector_version=$3 AND (l.completed_at IS NOT NULL OR l.failed_at IS NOT NULL OR l.available_at>clock_timestamp() OR l.claimed_until>clock_timestamp()))
		UNION
		SELECT workspace_id FROM usage_service_account_hourly_rollups u WHERE bucket_start=$1
		  AND NOT EXISTS (SELECT 1 FROM finops_anomaly_detection_leases l WHERE l.workspace_id=u.workspace_id AND l.bucket_start=$1 AND l.detector_version=$3 AND (l.completed_at IS NOT NULL OR l.failed_at IS NOT NULL OR l.available_at>clock_timestamp() OR l.claimed_until>clock_timestamp()))
		ORDER BY workspace_id LIMIT $2`, bucket, limit, detectorVersion)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	ids := make([]int64, 0, limit)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *workspaceRepository) hasPendingAnomalyWorkspace(ctx context.Context, bucket time.Time, detectorVersion string) (bool, error) {
	var pending bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM (
				SELECT workspace_id FROM usage_tenant_hourly_rollups WHERE bucket_start=$1
				UNION
				SELECT workspace_id FROM usage_service_account_hourly_rollups WHERE bucket_start=$1
			) u
			LEFT JOIN finops_anomaly_detection_leases l
			  ON l.workspace_id=u.workspace_id AND l.bucket_start=$1 AND l.detector_version=$2
			WHERE l.completed_at IS NULL AND l.failed_at IS NULL
		)`, bucket, detectorVersion).Scan(&pending)
	return pending, err
}

func (r *workspaceRepository) hasFailedAnomalyWorkspace(ctx context.Context, bucket time.Time, version string) (bool, error) {
	var failed bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM finops_anomaly_detection_leases WHERE bucket_start=$1 AND detector_version=$2 AND failed_at IS NOT NULL)`, bucket, version).Scan(&failed)
	return failed, err
}

func (r *workspaceRepository) reapAnomalyLeases(ctx context.Context, bucket time.Time, version string, limit int) error {
	_, err := r.db.ExecContext(ctx, `WITH expired AS (
	 SELECT workspace_id FROM finops_anomaly_detection_leases
	 WHERE bucket_start=$1 AND detector_version=$2 AND completed_at IS NULL AND failed_at IS NULL AND attempts>=$4
	 AND (claimed_until IS NULL OR claimed_until<=clock_timestamp())
	 ORDER BY workspace_id LIMIT $3 FOR UPDATE SKIP LOCKED)
	 UPDATE finops_anomaly_detection_leases l SET failed_at=clock_timestamp(),claimed_until=NULL,lease_token=NULL,last_error_code='retry_exhausted',updated_at=clock_timestamp()
	 FROM expired e WHERE l.workspace_id=e.workspace_id AND l.bucket_start=$1 AND l.detector_version=$2`, bucket, version, limit, anomalyMaxAttempts)
	return err
}

func (r *workspaceRepository) claimAnomalyLease(ctx context.Context, workspaceID int64, bucket time.Time, version string) (bool, string, error) {
	token := uuid.NewString()
	var claimed int64
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO finops_anomaly_detection_leases(workspace_id,bucket_start,detector_version,lease_token,claimed_until,attempts,last_error_code,updated_at)
		VALUES($1,$2,$3,$4,clock_timestamp()+$5 * interval '1 second',1,NULL,clock_timestamp())
		ON CONFLICT(workspace_id,bucket_start,detector_version) DO UPDATE SET
		 lease_token=$4,claimed_until=clock_timestamp()+$5 * interval '1 second', attempts=finops_anomaly_detection_leases.attempts+1,
		 last_error_code=NULL, updated_at=clock_timestamp()
		WHERE finops_anomaly_detection_leases.completed_at IS NULL
		  AND finops_anomaly_detection_leases.failed_at IS NULL AND finops_anomaly_detection_leases.attempts<5
		  AND finops_anomaly_detection_leases.available_at<=clock_timestamp()
		  AND (finops_anomaly_detection_leases.claimed_until IS NULL OR finops_anomaly_detection_leases.claimed_until<=clock_timestamp())
		RETURNING workspace_id`, workspaceID, bucket, version, token, anomalyLeaseDuration.Seconds()).Scan(&claimed)
	if errors.Is(err, sql.ErrNoRows) {
		return false, "", nil
	}
	return err == nil && claimed == workspaceID, token, err
}

func (r *workspaceRepository) finishAnomalyLease(ctx context.Context, workspaceID int64, bucket time.Time, version, token string, scanErr error) error {
	if scanErr == nil {
		result, err := r.db.ExecContext(ctx, `UPDATE finops_anomaly_detection_leases SET completed_at=clock_timestamp(),claimed_until=NULL,lease_token=NULL,last_error_code=NULL,updated_at=clock_timestamp() WHERE workspace_id=$1 AND bucket_start=$2 AND detector_version=$3 AND lease_token=$4 AND completed_at IS NULL AND failed_at IS NULL AND claimed_until>clock_timestamp()`, workspaceID, bucket, version, token)
		return requireAnomalyLeaseAffected(result, err)
	}
	result, err := r.db.ExecContext(ctx, `UPDATE finops_anomaly_detection_leases SET claimed_until=NULL,lease_token=NULL,last_error_code=$5,
	 available_at=clock_timestamp()+LEAST(attempts*30,300)*interval '1 second',
	 failed_at=CASE WHEN attempts>=5 THEN clock_timestamp() ELSE NULL END,updated_at=clock_timestamp()
	 WHERE workspace_id=$1 AND bucket_start=$2 AND detector_version=$3 AND lease_token=$4 AND completed_at IS NULL AND failed_at IS NULL AND claimed_until>clock_timestamp()`, workspaceID, bucket, version, token, anomalyFailureCode(scanErr))
	return requireAnomalyLeaseAffected(result, err)
}

func requireAnomalyLeaseAffected(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return service.ErrFinOpsAnomalyLeaseLost
	}
	return nil
}

func (r *workspaceRepository) updateAnomalyDetectorStatus(ctx context.Context, status service.FinOpsAnomalyDetectorStatus, bucket time.Time, scanErr error) error {
	lastFailure := status.LastFailureCode
	if scanErr != nil {
		lastFailure = anomalyFailureCode(scanErr)
	}
	var successful, processed any
	if status.LastSuccessfulScan != nil {
		successful = *status.LastSuccessfulScan
	}
	if status.LastProcessedBucket != nil {
		processed = *status.LastProcessedBucket
	} else if !bucket.IsZero() {
		processed = bucket
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO finops_anomaly_detector_status(id,last_successful_scan,last_processed_bucket,last_failure_code,lag_seconds,candidate_count,finding_count,scan_duration_ms,updated_at)
		VALUES(1,$1,$2,$3,$4,$5,$6,$7,now())
		ON CONFLICT(id) DO UPDATE SET last_successful_scan=GREATEST(EXCLUDED.last_successful_scan,finops_anomaly_detector_status.last_successful_scan),last_processed_bucket=GREATEST(EXCLUDED.last_processed_bucket,finops_anomaly_detector_status.last_processed_bucket),
		 last_failure_code=COALESCE(EXCLUDED.last_failure_code,finops_anomaly_detector_status.last_failure_code),lag_seconds=EXCLUDED.lag_seconds,candidate_count=GREATEST(EXCLUDED.candidate_count,finops_anomaly_detector_status.candidate_count),
		 finding_count=GREATEST(EXCLUDED.finding_count,finops_anomaly_detector_status.finding_count),scan_duration_ms=EXCLUDED.scan_duration_ms,updated_at=now()`, successful, processed, nullIfEmpty(lastFailure), status.LagSeconds, status.CandidateCount, status.FindingCount, status.ScanDurationMS)
	return err
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func (r *workspaceRepository) scanAnomalyWorkspace(ctx context.Context, workspaceID int64, bucket time.Time, cfg service.FinOpsAnomalyConfig) ([]anomalyCandidate, error) {
	rows, err := r.db.QueryContext(ctx, `
		WITH tenant_rollups AS MATERIALIZED (
		 SELECT workspace_id,project_id,api_key_id,resolved_platform,model,request_count,actual_cost
		 FROM usage_tenant_hourly_rollups WHERE workspace_id=$1 AND bucket_start=$2 LIMIT ($4+1)
		), service_rollups AS MATERIALIZED (
		 SELECT service_account_id,project_id,request_count,actual_cost
		 FROM usage_service_account_hourly_rollups WHERE workspace_id=$1 AND bucket_start=$2 LIMIT ($4+1)
		), input_overflow AS (
		 SELECT (SELECT count(*) FROM tenant_rollups)+(SELECT count(*) FROM service_rollups)>$4 AS exceeded
		), candidates AS (
		 SELECT 'workspace'::text scope_type, workspace_id scope_id, 'workspace'::text dimension_type, workspace_id::text dimension_value,
		        SUM(request_count)::bigint requests,SUM(actual_cost)::double precision spend,false service_account,NULL::bigint project_id
		 FROM tenant_rollups GROUP BY workspace_id
		 UNION ALL
		 SELECT 'project',project_id,'project',project_id::text,SUM(request_count)::bigint,SUM(actual_cost)::double precision,false,project_id
		 FROM tenant_rollups GROUP BY project_id
		 UNION ALL
		 SELECT 'platform',NULL::bigint,'platform',resolved_platform,SUM(request_count)::bigint,SUM(actual_cost)::double precision,false,NULL::bigint
		 FROM tenant_rollups WHERE trim(resolved_platform)<>'' GROUP BY resolved_platform
		 UNION ALL
		 SELECT 'model',NULL::bigint,'model',model,SUM(request_count)::bigint,SUM(actual_cost)::double precision,false,NULL::bigint
		 FROM tenant_rollups WHERE trim(model)<>'' GROUP BY model
		 UNION ALL
		 SELECT 'api_key',api_key_id,'api_key',api_key_id::text,SUM(request_count)::bigint,SUM(actual_cost)::double precision,false,project_id
		 FROM tenant_rollups GROUP BY api_key_id,project_id
		 UNION ALL
		 SELECT 'service_account',service_account_id,'service_account',service_account_id::text,SUM(request_count)::bigint,SUM(actual_cost)::double precision,true,project_id
		 FROM service_rollups GROUP BY service_account_id,project_id
		), limited AS (
		SELECT scope_type,scope_id,dimension_type,dimension_value,requests,spend,service_account,project_id
		FROM candidates WHERE requests>0 AND NOT (SELECT exceeded FROM input_overflow)
		ORDER BY spend DESC,requests DESC,dimension_type,dimension_value LIMIT $3)
		SELECT limited.*,false FROM limited
		UNION ALL SELECT '',NULL::bigint,'','',0::bigint,0::double precision,false,NULL::bigint,true
		WHERE (SELECT exceeded FROM input_overflow)
		ORDER BY spend DESC,requests DESC,dimension_type,dimension_value`, workspaceID, bucket, cfg.CandidateCap, cfg.MaxRollupRows)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	candidates := make([]anomalyCandidate, 0, cfg.CandidateCap)
	for rows.Next() {
		var c anomalyCandidate
		var scopeID, projectID sql.NullInt64
		var overflow bool
		if err := rows.Scan(&c.scopeType, &scopeID, &c.dimensionType, &c.dimensionValue, &c.requests, &c.spend, &c.serviceAccount, &projectID, &overflow); err != nil {
			return nil, err
		}
		if overflow {
			return nil, service.ErrFinOpsAnomalyRollupLimit
		}
		if scopeID.Valid {
			c.scopeID = scopeID.Int64
		}
		if projectID.Valid {
			c.projectID = projectID.Int64
		}
		if c.requests < 0 || math.IsNaN(c.spend) || math.IsInf(c.spend, 0) || c.spend < 0 || strings.TrimSpace(c.dimensionValue) == "" {
			continue
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return candidates, nil
}

func (r *workspaceRepository) anomalySamples(ctx context.Context, workspaceID int64, bucket time.Time, candidate anomalyCandidate, cfg service.FinOpsAnomalyConfig) ([]anomalyRollupSample, error) {
	start := bucket.Add(-time.Duration(cfg.LookbackDays) * 24 * time.Hour)
	where := "workspace_id=$1 AND bucket_start>=$2 AND bucket_start<$3 AND EXTRACT(HOUR FROM bucket_start AT TIME ZONE 'UTC')=EXTRACT(HOUR FROM $4::timestamptz AT TIME ZONE 'UTC')"
	args := []any{workspaceID, start, bucket, bucket}
	if candidate.serviceAccount {
		where += fmt.Sprintf(" AND service_account_id=$%d", len(args)+1)
		args = append(args, candidate.scopeID)
	} else {
		switch candidate.dimensionType {
		case service.AnomalyScopeProject:
			where += fmt.Sprintf(" AND project_id=$%d", len(args)+1)
			args = append(args, candidate.scopeID)
		case service.AnomalyScopePlatform:
			where += fmt.Sprintf(" AND resolved_platform=$%d", len(args)+1)
			args = append(args, candidate.dimensionValue)
		case service.AnomalyScopeModel:
			where += fmt.Sprintf(" AND model=$%d", len(args)+1)
			args = append(args, candidate.dimensionValue)
		case service.AnomalyScopeAPIKey:
			where += fmt.Sprintf(" AND api_key_id=$%d", len(args)+1)
			args = append(args, candidate.scopeID)
		}
	}
	table := "usage_tenant_hourly_rollups"
	if candidate.serviceAccount {
		table = "usage_service_account_hourly_rollups"
	}
	args = append(args, cfg.MaxRollupRows)
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`WITH source AS MATERIALIZED (
	 SELECT bucket_start,request_count,actual_cost FROM %s WHERE %s LIMIT ($%d+1)
	), input_overflow AS (SELECT count(*)>$%d AS exceeded FROM source), samples AS (
	 SELECT bucket_start,SUM(request_count)::bigint requests,SUM(actual_cost)::double precision spend
	 FROM source WHERE NOT (SELECT exceeded FROM input_overflow) GROUP BY bucket_start)
	 SELECT bucket_start,requests,spend,false FROM samples
	 UNION ALL SELECT 'epoch'::timestamptz,0::bigint,0::double precision,true WHERE (SELECT exceeded FROM input_overflow)
	 ORDER BY bucket_start DESC`, table, where, len(args), len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	samples := make([]anomalyRollupSample, 0, cfg.MaxRollupRows)
	for rows.Next() {
		var item anomalyRollupSample
		var overflow bool
		if err := rows.Scan(&item.bucket, &item.requests, &item.spend, &overflow); err != nil {
			return nil, err
		}
		if overflow {
			return nil, service.ErrFinOpsAnomalyRollupLimit
		}
		if item.requests < 0 || math.IsNaN(item.spend) || math.IsInf(item.spend, 0) || item.spend < 0 {
			continue
		}
		samples = append(samples, item)
	}
	return samples, rows.Err()
}

func (r *workspaceRepository) detectAnomalyCandidate(ctx context.Context, workspaceID int64, bucket time.Time, candidate anomalyCandidate, cfg service.FinOpsAnomalyConfig, leaseToken string) (int, error) {
	samples, err := r.anomalySamples(ctx, workspaceID, bucket, candidate, cfg)
	if err != nil {
		return 0, err
	}
	if len(samples) < cfg.MinBaselineSamples {
		return 0, nil
	}
	spendBaseline := make([]float64, 0, len(samples))
	requestBaseline := make([]float64, 0, len(samples))
	unitCostBaseline := make([]float64, 0, len(samples))
	for _, sample := range samples {
		spendBaseline = append(spendBaseline, sample.spend)
		requestBaseline = append(requestBaseline, float64(sample.requests))
		if sample.requests > 0 {
			unitCostBaseline = append(unitCostBaseline, sample.spend/float64(sample.requests))
		} else {
			unitCostBaseline = append(unitCostBaseline, 0)
		}
	}
	observedUnitCost := 0.0
	if candidate.requests > 0 {
		observedUnitCost = candidate.spend / float64(candidate.requests)
	}
	base := service.FinOpsDetectorInput{
		WorkspaceID: workspaceID, ScopeType: candidate.scopeType, ScopeID: candidate.scopeID,
		ProjectID:     candidate.projectID,
		DimensionType: candidate.dimensionType, DimensionValue: candidate.dimensionValue,
		WindowStart: bucket, WindowEnd: bucket.Add(time.Hour), EvaluationTime: bucket.Add(time.Hour).Add(cfg.Grace),
		ObservedSpend: candidate.spend, ObservedRequests: candidate.requests, ObservedUnitCost: observedUnitCost,
		BaselineSpend: spendBaseline, BaselineRequests: requestBaseline, BaselineUnitCosts: unitCostBaseline,
		DetectorVersion: cfg.DetectorVersion,
	}
	created := 0
	for _, detector := range []string{service.AnomalyDetectorSpendSpike, service.AnomalyDetectorRequestSpike, service.AnomalyDetectorUnitCostSpike} {
		input := base
		input.DetectorType = detector
		if detector == service.AnomalyDetectorRequestSpike {
			input.ExpectedRequests = 0
		}
		if detector == service.AnomalyDetectorUnitCostSpike {
			input.ExpectedUnitCost = 0
		}
		detection, ok := service.DetectFinOpsAnomaly(input, cfg)
		if !ok {
			continue
		}
		inserted, err := r.persistAnomalyDetection(ctx, detection, bucket.Add(-time.Duration(cfg.LookbackDays)*24*time.Hour), bucket, leaseToken)
		if err != nil {
			return created, err
		}
		if inserted {
			created++
		}
	}
	return created, nil
}

func anomalyEventData(d service.FinOpsAnomalyDetection, anomalyID int64, status, reason string) service.DomainEventData {
	data := service.DomainEventData{
		"anomaly_id": strconv.FormatInt(anomalyID, 10), "status": status,
		"scope_type": d.ScopeType, "scope_id": d.ScopeID, "dimension_type": d.DimensionType, "dimension_value": d.DimensionValue,
		"detector_type": d.DetectorType, "detector_version": d.DetectorVersion, "severity": d.Severity,
		"observed_spend": d.ObservedSpend, "expected_spend": d.ExpectedSpend, "spend_delta": d.SpendDelta,
		"observed_requests": d.ObservedRequests, "expected_requests": d.ExpectedRequests,
		"observed_unit_cost": d.ObservedUnitCost, "expected_unit_cost": d.ExpectedUnitCost, "score": d.Score,
		"window_start": d.WindowStart.UTC().Format(time.RFC3339Nano), "window_end": d.WindowEnd.UTC().Format(time.RFC3339Nano),
	}
	if reason != "" {
		data["resolution_reason"] = reason
	}
	return data
}

func (r *workspaceRepository) persistAnomalyDetection(ctx context.Context, d service.FinOpsAnomalyDetection, baselineStart, baselineEnd time.Time, leaseToken string) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	var workspaceID int64
	err = tx.QueryRowContext(ctx, `SELECT workspace_id FROM finops_anomaly_detection_leases WHERE workspace_id=$1 AND bucket_start=$2 AND detector_version=$3 AND lease_token=$4 AND completed_at IS NULL AND failed_at IS NULL AND claimed_until>clock_timestamp() FOR UPDATE`, d.WorkspaceID, d.WindowStart, d.DetectorVersion, leaseToken).Scan(&workspaceID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, service.ErrFinOpsAnomalyLeaseLost
	}
	if err != nil {
		return false, err
	}
	commit := func() error {
		result, err := tx.ExecContext(ctx, `UPDATE finops_anomaly_detection_leases SET updated_at=clock_timestamp() WHERE workspace_id=$1 AND bucket_start=$2 AND detector_version=$3 AND lease_token=$4 AND completed_at IS NULL AND failed_at IS NULL AND claimed_until>clock_timestamp()`, d.WorkspaceID, d.WindowStart, d.DetectorVersion, leaseToken)
		if err = requireAnomalyLeaseAffected(result, err); err != nil {
			return err
		}
		return tx.Commit()
	}
	var snapshotID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO finops_anomaly_snapshots(workspace_id,project_id,scope_type,scope_id,dimension_type,dimension_value,detector_type,detector_version,window_start,window_end,baseline_start,baseline_end,observed_spend,expected_spend,spend_delta,observed_requests,expected_requests,observed_unit_cost,expected_unit_cost,baseline_sample_count,baseline_mad,relative_increase,score,severity,fingerprint)
		VALUES($1,NULLIF($2,0),$3,NULLIF($4,0),$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25)
		ON CONFLICT(fingerprint) DO NOTHING RETURNING id`, d.WorkspaceID, d.ProjectID, d.ScopeType, d.ScopeID, d.DimensionType, d.DimensionValue, d.DetectorType, d.DetectorVersion, d.WindowStart, d.WindowEnd, baselineStart, baselineEnd, d.ObservedSpend, d.ExpectedSpend, d.SpendDelta, d.ObservedRequests, d.ExpectedRequests, d.ObservedUnitCost, d.ExpectedUnitCost, d.BaselineSampleCount, d.BaselineMAD, d.RelativeIncrease, d.Score, d.Severity, d.Fingerprint).Scan(&snapshotID)
	if errors.Is(err, sql.ErrNoRows) {
		if err = tx.QueryRowContext(ctx, `SELECT id FROM finops_anomaly_snapshots WHERE fingerprint=$1`, d.Fingerprint).Scan(&snapshotID); err != nil {
			return false, workspaceError(err)
		}
	} else if err != nil {
		return false, workspaceError(err)
	}
	var findingID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO finops_anomaly_findings(workspace_id,project_id,snapshot_id,fingerprint,status,first_detected_at,last_detected_at,version)
		VALUES($1,NULLIF($2,0),$3,$4,'open',now(),now(),1)
		ON CONFLICT(fingerprint) DO NOTHING RETURNING id`, d.WorkspaceID, d.ProjectID, snapshotID, d.Fingerprint).Scan(&findingID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, commit()
	}
	if err != nil {
		return false, workspaceError(err)
	}
	event, err := service.NewDomainEvent(service.EventFinOpsAnomalyDetected, d.WorkspaceID, d.ProjectID, 0, "finops_anomaly", strconv.FormatInt(findingID, 10), anomalyEventData(d, findingID, service.AnomalyStatusOpen, ""))
	if err != nil {
		return false, err
	}
	if err = InsertDomainEventTx(ctx, tx, event, "finops.anomaly.detected:"+d.Fingerprint); err != nil {
		return false, err
	}
	if err = commit(); err != nil {
		return false, err
	}
	return true, nil
}

const anomalyFindingSelect = `
	f.id,f.workspace_id,f.project_id,s.scope_type,s.scope_id,s.dimension_type,s.dimension_value,s.detector_type,s.detector_version,
	s.window_start,s.window_end,s.observed_spend,s.expected_spend,s.spend_delta,s.observed_requests,s.expected_requests,s.observed_unit_cost,s.expected_unit_cost,
	s.baseline_sample_count,s.baseline_start,s.baseline_end,s.baseline_mad,s.relative_increase,s.score,s.severity,s.fingerprint,
	f.status,f.first_detected_at,f.last_detected_at,f.acknowledged_at,f.acknowledged_by_user_id,f.resolved_at,f.resolved_by_user_id,f.resolution_reason,f.version,f.created_at,f.updated_at,s.id`

func scanFinOpsAnomaly(scanner workspaceScanner) (*service.FinOpsAnomalyFinding, error) {
	var finding service.FinOpsAnomalyFinding
	var projectID, scopeID, ackBy, resolvedBy sql.NullInt64
	var baselineStart, baselineEnd sql.NullTime
	var ackAt, resolvedAt sql.NullTime
	var snapshotID int64
	err := scanner.Scan(&finding.ID, &finding.WorkspaceID, &projectID, &finding.ScopeType, &scopeID, &finding.DimensionType, &finding.DimensionValue,
		&finding.DetectorType, &finding.DetectorVersion, &finding.WindowStart, &finding.WindowEnd, &finding.ObservedSpend, &finding.ExpectedSpend, &finding.SpendDelta,
		&finding.ObservedRequests, &finding.ExpectedRequests, &finding.ObservedUnitCost, &finding.ExpectedUnitCost, &finding.BaselineSampleCount, &baselineStart, &baselineEnd,
		&finding.BaselineMAD, &finding.RelativeIncrease, &finding.Score, &finding.Severity, &finding.Fingerprint, &finding.Status, &finding.FirstDetectedAt, &finding.LastDetectedAt,
		&ackAt, &ackBy, &resolvedAt, &resolvedBy, &finding.ResolutionReason, &finding.Version, &finding.CreatedAt, &finding.UpdatedAt, &snapshotID)
	if err != nil {
		return nil, workspaceError(err)
	}
	if projectID.Valid {
		finding.ProjectID = projectID.Int64
	}
	if scopeID.Valid {
		finding.ScopeID = scopeID.Int64
	}
	if baselineStart.Valid {
		finding.BaselineStart = baselineStart.Time
	}
	if baselineEnd.Valid {
		finding.BaselineEnd = baselineEnd.Time
	}
	if ackAt.Valid {
		finding.AcknowledgedAt = &ackAt.Time
	}
	if ackBy.Valid {
		finding.AcknowledgedByUserID = &ackBy.Int64
	}
	if resolvedAt.Valid {
		finding.ResolvedAt = &resolvedAt.Time
	}
	if resolvedBy.Valid {
		finding.ResolvedByUserID = &resolvedBy.Int64
	}
	finding.Fingerprint = strings.TrimSpace(finding.Fingerprint)
	finding.SnapshotID = snapshotID
	return &finding, nil
}

func anomalyWhere(scope service.FinOpsScope, filter service.FinOpsAnomalyFilter, startArg int) (string, []any, int) {
	where := "f.workspace_id=$1"
	args := []any{scope.WorkspaceID}
	n := startArg
	if scope.ProjectID > 0 {
		n++
		where += fmt.Sprintf(" AND f.project_id=$%d", n)
		args = append(args, scope.ProjectID)
	} else if scope.WorkspaceOnly {
		where += " AND f.project_id IS NULL"
	}
	if filter.Status != "" {
		n++
		where += fmt.Sprintf(" AND f.status=$%d", n)
		args = append(args, filter.Status)
	}
	if filter.Severity != "" {
		n++
		where += fmt.Sprintf(" AND s.severity=$%d", n)
		args = append(args, filter.Severity)
	}
	if filter.DetectorType != "" {
		n++
		where += fmt.Sprintf(" AND s.detector_type=$%d", n)
		args = append(args, filter.DetectorType)
	}
	if filter.DimensionType != "" {
		n++
		where += fmt.Sprintf(" AND s.dimension_type=$%d", n)
		args = append(args, filter.DimensionType)
	}
	if filter.Start != nil {
		n++
		where += fmt.Sprintf(" AND s.window_start >= $%d", n)
		args = append(args, filter.Start.UTC())
	}
	if filter.End != nil {
		n++
		where += fmt.Sprintf(" AND s.window_end <= $%d", n)
		args = append(args, filter.End.UTC())
	}
	return where, args, n
}

func (r *workspaceRepository) ListFinOpsAnomalies(ctx context.Context, scope service.FinOpsScope, filter service.FinOpsAnomalyFilter) ([]service.FinOpsAnomalyFinding, int64, error) {
	if scope.WorkspaceID <= 0 || scope.ProjectID < 0 {
		return nil, 0, service.ErrWorkspaceNotFound
	}
	if err := filter.Validate(); err != nil {
		return nil, 0, err
	}
	filter = filter.Normalized()
	where, args, n := anomalyWhere(scope, filter, 1)
	var total int64
	countQuery := `SELECT count(*) FROM finops_anomaly_findings f JOIN finops_anomaly_snapshots s ON s.id=f.snapshot_id WHERE ` + where
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, workspaceError(err)
	}
	pageSize := filter.PageSize
	offset := (filter.Page - 1) * pageSize
	n++
	args = append(args, pageSize)
	limitArg := n
	n++
	args = append(args, offset)
	query := fmt.Sprintf(`SELECT %s FROM finops_anomaly_findings f JOIN finops_anomaly_snapshots s ON s.id=f.snapshot_id WHERE %s ORDER BY f.last_detected_at DESC,f.id DESC LIMIT $%d OFFSET $%d`, anomalyFindingSelect, where, limitArg, n)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, workspaceError(err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.FinOpsAnomalyFinding, 0, pageSize)
	for rows.Next() {
		item, scanErr := scanFinOpsAnomaly(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *workspaceRepository) GetFinOpsAnomaly(ctx context.Context, scope service.FinOpsScope, anomalyID int64) (*service.FinOpsAnomalyFinding, error) {
	if scope.WorkspaceID <= 0 || anomalyID <= 0 || scope.ProjectID < 0 {
		return nil, service.ErrWorkspaceNotFound
	}
	where := "f.workspace_id=$1 AND f.id=$2"
	args := []any{scope.WorkspaceID, anomalyID}
	if scope.ProjectID > 0 {
		where += " AND f.project_id=$3"
		args = append(args, scope.ProjectID)
	} else if scope.WorkspaceOnly {
		where += " AND f.project_id IS NULL"
	}
	item, err := scanFinOpsAnomaly(r.db.QueryRowContext(ctx, `SELECT `+anomalyFindingSelect+` FROM finops_anomaly_findings f JOIN finops_anomaly_snapshots s ON s.id=f.snapshot_id WHERE `+where, args...))
	return item, err
}

func (r *workspaceRepository) TransitionFinOpsAnomaly(ctx context.Context, actorID int64, scope service.FinOpsScope, anomalyID int64, patch service.FinOpsAnomalyPatch) (*service.FinOpsAnomalyFinding, error) {
	if scope.WorkspaceID <= 0 || anomalyID <= 0 || actorID <= 0 || scope.ProjectID < 0 {
		return nil, service.ErrWorkspaceNotFound
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	where := "f.workspace_id=$1 AND f.id=$2"
	args := []any{scope.WorkspaceID, anomalyID}
	if scope.ProjectID > 0 {
		where += " AND f.project_id=$3"
		args = append(args, scope.ProjectID)
	} else if scope.WorkspaceOnly {
		where += " AND f.project_id IS NULL"
	}
	current, err := scanFinOpsAnomaly(tx.QueryRowContext(ctx, `SELECT `+anomalyFindingSelect+` FROM finops_anomaly_findings f JOIN finops_anomaly_snapshots s ON s.id=f.snapshot_id WHERE `+where+` FOR UPDATE`, args...))
	if err != nil {
		return nil, err
	}
	if patch.ExpectedVersion != current.Version {
		return nil, service.ErrFinOpsAnomalyVersionConflict
	}
	if (current.Status == service.AnomalyStatusOpen && patch.Status != service.AnomalyStatusAcknowledged && patch.Status != service.AnomalyStatusResolved) ||
		(current.Status == service.AnomalyStatusAcknowledged && patch.Status != service.AnomalyStatusResolved) ||
		current.Status == service.AnomalyStatusResolved {
		return nil, service.ErrFinOpsAnomalyInvalidTransition
	}
	projectID := current.ProjectID
	var updated *service.FinOpsAnomalyFinding
	var result sql.Result
	if patch.Status == service.AnomalyStatusAcknowledged {
		result, err = tx.ExecContext(ctx, `UPDATE finops_anomaly_findings SET status='acknowledged',acknowledged_at=now(),acknowledged_by_user_id=$3,version=version+1,updated_at=now() WHERE id=$1 AND workspace_id=$2 AND version=$4`, anomalyID, scope.WorkspaceID, actorID, patch.ExpectedVersion)
	} else {
		result, err = tx.ExecContext(ctx, `UPDATE finops_anomaly_findings SET status='resolved',resolved_at=now(),resolved_by_user_id=$3,resolution_reason=$4,version=version+1,updated_at=now() WHERE id=$1 AND workspace_id=$2 AND version=$5`, anomalyID, scope.WorkspaceID, actorID, strings.TrimSpace(patch.ResolutionReason), patch.ExpectedVersion)
	}
	if err != nil {
		return nil, workspaceError(err)
	}
	if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
		return nil, service.ErrFinOpsAnomalyVersionConflict
	}
	updated, err = scanFinOpsAnomaly(tx.QueryRowContext(ctx, `SELECT `+anomalyFindingSelect+` FROM finops_anomaly_findings f JOIN finops_anomaly_snapshots s ON s.id=f.snapshot_id WHERE f.workspace_id=$1 AND f.id=$2`, scope.WorkspaceID, anomalyID))
	if err != nil {
		return nil, err
	}
	projectPtr := (*int64)(nil)
	if projectID > 0 {
		projectPtr = &projectID
	}
	action := "finops_anomaly_acknowledged"
	eventType := service.EventFinOpsAnomalyAcknowledged
	if patch.Status == service.AnomalyStatusResolved {
		action = "finops_anomaly_resolved"
		eventType = service.EventFinOpsAnomalyResolved
	}
	if err = appendWorkspaceAudit(ctx, tx, scope.WorkspaceID, actorID, projectPtr, action, "finops_anomaly", anomalyID, map[string]any{"status": patch.Status, "resolution_reason": strings.TrimSpace(patch.ResolutionReason)}); err != nil {
		return nil, err
	}
	event, err := service.NewDomainEvent(eventType, scope.WorkspaceID, projectID, actorID, "finops_anomaly", strconv.FormatInt(anomalyID, 10), anomalyEventData(updated.FinOpsAnomalyDetection, anomalyID, patch.Status, strings.TrimSpace(patch.ResolutionReason)))
	if err != nil {
		return nil, err
	}
	if err = InsertDomainEventTx(ctx, tx, event, fmt.Sprintf("finops.anomaly.%s:%d:%d", patch.Status, anomalyID, patch.ExpectedVersion)); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return updated, nil
}

func (r *workspaceRepository) GetFinOpsAnomalyStatus(ctx context.Context) (*service.FinOpsAnomalyDetectorStatus, error) {
	var status service.FinOpsAnomalyDetectorStatus
	var successful, processed sql.NullTime
	var failure sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT last_successful_scan,last_processed_bucket,last_failure_code,lag_seconds,candidate_count,finding_count,scan_duration_ms FROM finops_anomaly_detector_status WHERE id=1`).Scan(&successful, &processed, &failure, &status.LagSeconds, &status.CandidateCount, &status.FindingCount, &status.ScanDurationMS)
	if errors.Is(err, sql.ErrNoRows) {
		return &status, nil
	}
	if err != nil {
		return nil, workspaceError(err)
	}
	if successful.Valid {
		status.LastSuccessfulScan = &successful.Time
	}
	if processed.Valid {
		status.LastProcessedBucket = &processed.Time
	}
	if failure.Valid {
		status.LastFailureCode = failure.String
	}
	return &status, nil
}
