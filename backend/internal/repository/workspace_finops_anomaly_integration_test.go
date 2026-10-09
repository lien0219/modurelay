//go:build integration

package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type anomalyWorkspaceFixture struct {
	ctx     context.Context
	repo    *workspaceRepository
	owner   *service.User
	ws      *service.Workspace
	project *service.Project
}

func newAnomalyWorkspaceFixture(t *testing.T) anomalyWorkspaceFixture {
	t.Helper()
	isolateWorkspaceTestFixtures(t)
	ctx := context.Background()
	owner := mustCreateUser(t, testEntClient(t), &service.User{Email: "anomaly-owner-" + uuid.NewString() + "@example.com"})
	repo := NewWorkspaceRepository(integrationDB).(*workspaceRepository)
	workspaces := service.NewWorkspaceService(repo)
	workspace, err := workspaces.CreateOrganization(ctx, owner.ID, "Anomaly", "anomaly-"+uuid.NewString())
	require.NoError(t, err)
	project, err := workspaces.CreateProject(ctx, owner.ID, workspace.ID, service.ProjectInput{Name: "Anomaly Project", Slug: "anomaly-project-" + uuid.NewString()})
	require.NoError(t, err)

	return anomalyWorkspaceFixture{ctx: ctx, repo: repo, owner: owner, ws: workspace, project: project}
}

func anomalyDetectionFixture(workspaceID, projectID int64, detector, dimensionType, dimensionValue string, window time.Time) service.FinOpsAnomalyDetection {
	input := service.FinOpsDetectorInput{
		WorkspaceID: workspaceID, ProjectID: projectID, ScopeType: dimensionType, ScopeID: projectID,
		DimensionType: dimensionType, DimensionValue: dimensionValue, DetectorType: detector,
		DetectorVersion: service.FinOpsAnomalyDetectorVersion, WindowStart: window, WindowEnd: window.Add(time.Hour),
	}
	return service.FinOpsAnomalyDetection{
		WorkspaceID: workspaceID, ProjectID: projectID, ScopeType: dimensionType, ScopeID: projectID,
		DimensionType: dimensionType, DimensionValue: dimensionValue, DetectorType: detector,
		DetectorVersion: service.FinOpsAnomalyDetectorVersion, WindowStart: window, WindowEnd: window.Add(time.Hour),
		ObservedSpend: 10, ExpectedSpend: 1, SpendDelta: 9, ObservedRequests: 100, ExpectedRequests: 10,
		ObservedUnitCost: .1, ExpectedUnitCost: .1, BaselineSampleCount: 12, BaselineMAD: 0,
		RelativeIncrease: 9, Score: 18, Severity: service.AnomalySeverityCritical,
		Fingerprint: service.FinOpsAnomalyFingerprint(input, service.FinOpsAnomalyDetectorVersion),
	}
}

func TestFinOpsAnomalyPostgresScanIsBoundedAndTenantScoped(t *testing.T) {
	fixture := newAnomalyWorkspaceFixture(t)
	ctx, repo, workspace, project := fixture.ctx, fixture.repo, fixture.ws, fixture.project
	_, err := integrationDB.ExecContext(ctx, `DELETE FROM finops_anomaly_detector_status`)
	require.NoError(t, err)

	bucket := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	apiKeyID := int64(700000 + workspace.ID)
	for daysAgo := 1; daysAgo <= 12; daysAgo++ {
		baselineBucket := bucket.Add(-time.Duration(daysAgo) * 24 * time.Hour)
		_, err = integrationDB.ExecContext(ctx, `
			INSERT INTO usage_tenant_hourly_rollups(workspace_id,project_id,bucket_start,api_key_id,resolved_platform,model,request_count,actual_cost)
			VALUES($1,$2,$3,$4,'openai','anomaly-model',10,1)`, workspace.ID, project.ID, baselineBucket, apiKeyID)
		require.NoError(t, err)
	}
	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO usage_tenant_hourly_rollups(workspace_id,project_id,bucket_start,api_key_id,resolved_platform,model,request_count,actual_cost)
		VALUES($1,$2,$3,$4,'openai','anomaly-model',100,10)`, workspace.ID, project.ID, bucket, apiKeyID)
	require.NoError(t, err)

	status, err := repo.RunFinOpsAnomalyScan(ctx, bucket.Add(time.Hour+5*time.Minute), service.DefaultFinOpsAnomalyConfig())
	require.NoError(t, err)
	require.NotNil(t, status.LastProcessedBucket)
	require.True(t, status.LastProcessedBucket.Equal(bucket))
	require.NotNil(t, status.LastSuccessfulScan)
	require.Positive(t, status.CandidateCount)
	require.Positive(t, status.FindingCount)

	var findings, events int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM finops_anomaly_findings WHERE workspace_id=$1`, workspace.ID).Scan(&findings))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM domain_events WHERE workspace_id=$1 AND event_type=$2`, workspace.ID, service.EventFinOpsAnomalyDetected).Scan(&events))
	require.Equal(t, status.FindingCount, int64(findings))
	require.Equal(t, findings, events)
	require.GreaterOrEqual(t, findings, 2, "spend and request detectors should produce evidence")

	items, total, err := repo.ListFinOpsAnomalies(ctx, service.FinOpsScope{WorkspaceID: workspace.ID}, service.FinOpsAnomalyFilter{Page: 1, PageSize: 100})
	require.NoError(t, err)
	require.Equal(t, int64(findings), total)
	require.Len(t, items, findings)
	projectItems, projectTotal, err := repo.ListFinOpsAnomalies(ctx, service.FinOpsScope{WorkspaceID: workspace.ID, ProjectID: project.ID}, service.FinOpsAnomalyFilter{Page: 1, PageSize: 100})
	require.NoError(t, err)
	require.Positive(t, projectTotal)
	require.Less(t, projectTotal, int64(findings), "project scope excludes workspace/platform/model findings")
	require.Len(t, projectItems, int(projectTotal))
}

func TestFinOpsAnomalyPostgresPersistsIdempotentlyAndProtectsEvidence(t *testing.T) {
	fixture := newAnomalyWorkspaceFixture(t)
	ctx, repo, owner, workspace, project := fixture.ctx, fixture.repo, fixture.owner, fixture.ws, fixture.project
	detection := anomalyDetectionFixture(workspace.ID, project.ID, service.AnomalyDetectorSpendSpike, service.AnomalyScopeProject, "project", time.Date(2026, 10, 8, 1, 0, 0, 0, time.UTC))
	claimed, token, err := repo.claimAnomalyLease(ctx, workspace.ID, detection.WindowStart, detection.DetectorVersion)
	require.NoError(t, err)
	require.True(t, claimed)

	created, err := repo.persistAnomalyDetection(ctx, detection, detection.WindowStart.Add(-28*24*time.Hour), detection.WindowStart, token)
	require.NoError(t, err)
	require.True(t, created)
	created, err = repo.persistAnomalyDetection(ctx, detection, detection.WindowStart.Add(-28*24*time.Hour), detection.WindowStart, token)
	require.NoError(t, err)
	require.False(t, created, "fingerprint retry must not duplicate evidence or events")

	items, total, err := repo.ListFinOpsAnomalies(ctx, service.FinOpsScope{WorkspaceID: workspace.ID}, service.FinOpsAnomalyFilter{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	finding := items[0]
	_, err = integrationDB.ExecContext(ctx, `UPDATE finops_anomaly_snapshots SET observed_spend=11 WHERE id=$1`, finding.SnapshotID)
	require.Error(t, err, "evidence snapshots are immutable")
	_, err = integrationDB.ExecContext(ctx, `DELETE FROM finops_anomaly_snapshots WHERE id=$1`, finding.SnapshotID)
	require.Error(t, err, "evidence snapshots cannot be deleted")

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, transitionErr := repo.TransitionFinOpsAnomaly(ctx, owner.ID, service.FinOpsScope{WorkspaceID: workspace.ID, ProjectID: project.ID}, finding.ID, service.FinOpsAnomalyPatch{Status: service.AnomalyStatusAcknowledged, ExpectedVersion: finding.Version})
			errs <- transitionErr
		}()
	}
	close(start)
	wg.Wait()
	var success, conflicts int
	for i := 0; i < 2; i++ {
		switch err := <-errs; {
		case err == nil:
			success++
		case err != nil:
			require.ErrorIs(t, err, service.ErrFinOpsAnomalyVersionConflict)
			conflicts++
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 1, conflicts)

	updated, err := repo.GetFinOpsAnomaly(ctx, service.FinOpsScope{WorkspaceID: workspace.ID, ProjectID: project.ID}, finding.ID)
	require.NoError(t, err)
	require.Equal(t, service.AnomalyStatusAcknowledged, updated.Status)
	require.Equal(t, int64(2), updated.Version)
	updated, err = repo.TransitionFinOpsAnomaly(ctx, owner.ID, service.FinOpsScope{WorkspaceID: workspace.ID, ProjectID: project.ID}, finding.ID, service.FinOpsAnomalyPatch{Status: service.AnomalyStatusResolved, ResolutionReason: "reviewed", ExpectedVersion: updated.Version})
	require.NoError(t, err)
	require.Equal(t, service.AnomalyStatusResolved, updated.Status)
	require.Equal(t, int64(3), updated.Version)
	_, err = repo.TransitionFinOpsAnomaly(ctx, owner.ID, service.FinOpsScope{WorkspaceID: workspace.ID, ProjectID: project.ID}, finding.ID, service.FinOpsAnomalyPatch{Status: service.AnomalyStatusAcknowledged, ExpectedVersion: updated.Version})
	require.ErrorIs(t, err, service.ErrFinOpsAnomalyInvalidTransition)

	otherOwner := mustCreateUser(t, testEntClient(t), &service.User{Email: "anomaly-other-" + uuid.NewString() + "@example.com"})
	workspaces := service.NewWorkspaceService(repo)
	otherWorkspace, err := workspaces.CreateOrganization(ctx, otherOwner.ID, "Other Anomaly", "other-anomaly-"+uuid.NewString())
	require.NoError(t, err)
	otherProject, err := workspaces.CreateProject(ctx, otherOwner.ID, otherWorkspace.ID, service.ProjectInput{Name: "Other Project", Slug: "other-project-" + uuid.NewString()})
	require.NoError(t, err)
	otherDetection := anomalyDetectionFixture(otherWorkspace.ID, otherProject.ID, service.AnomalyDetectorSpendSpike, service.AnomalyScopeProject, "project", time.Date(2026, 10, 8, 2, 0, 0, 0, time.UTC))
	claimed, otherToken, err := repo.claimAnomalyLease(ctx, otherWorkspace.ID, otherDetection.WindowStart, otherDetection.DetectorVersion)
	require.NoError(t, err)
	require.True(t, claimed)
	created, err = repo.persistAnomalyDetection(ctx, otherDetection, otherDetection.WindowStart.Add(-28*24*time.Hour), otherDetection.WindowStart, otherToken)
	require.NoError(t, err)
	require.True(t, created)
	otherItems, otherTotal, err := repo.ListFinOpsAnomalies(ctx, service.FinOpsScope{WorkspaceID: otherWorkspace.ID}, service.FinOpsAnomalyFilter{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), otherTotal)
	require.Len(t, otherItems, 1)
	_, err = repo.GetFinOpsAnomaly(ctx, service.FinOpsScope{WorkspaceID: workspace.ID, ProjectID: project.ID}, otherItems[0].ID)
	require.ErrorIs(t, err, service.ErrWorkspaceNotFound, "foreign workspace finding is hidden")
	_, otherProjectTotal, err := repo.ListFinOpsAnomalies(ctx, service.FinOpsScope{WorkspaceID: workspace.ID, ProjectID: otherProject.ID}, service.FinOpsAnomalyFilter{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Zero(t, otherProjectTotal, "foreign project scope is tenant-bound")
}

func TestFinOpsAnomalyPostgresLeaseTokenFencesExpiredWorker(t *testing.T) {
	fixture := newAnomalyWorkspaceFixture(t)
	ctx, repo, workspace := fixture.ctx, fixture.repo, fixture.ws
	bucket := time.Date(2026, 10, 8, 3, 0, 0, 0, time.UTC)

	start := make(chan struct{})
	type claimResult struct {
		claimed bool
		token   string
		err     error
	}
	results := make(chan claimResult, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			claimed, token, err := repo.claimAnomalyLease(ctx, workspace.ID, bucket, service.FinOpsAnomalyDetectorVersion)
			results <- claimResult{claimed: claimed, token: token, err: err}
		}()
	}
	close(start)
	wg.Wait()
	var winner claimResult
	for i := 0; i < 2; i++ {
		result := <-results
		require.NoError(t, result.err)
		if result.claimed {
			winner = result
		}
	}
	require.True(t, winner.claimed)
	require.NotEmpty(t, winner.token)

	_, err := integrationDB.ExecContext(ctx, `UPDATE finops_anomaly_detection_leases SET claimed_until=now()-interval '1 second' WHERE workspace_id=$1 AND bucket_start=$2 AND detector_version=$3`, workspace.ID, bucket, service.FinOpsAnomalyDetectorVersion)
	require.NoError(t, err)
	claimed, replacementToken, err := repo.claimAnomalyLease(ctx, workspace.ID, bucket, service.FinOpsAnomalyDetectorVersion)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NotEqual(t, winner.token, replacementToken)

	require.ErrorIs(t, repo.finishAnomalyLease(ctx, workspace.ID, bucket, service.FinOpsAnomalyDetectorVersion, winner.token, nil), service.ErrFinOpsAnomalyLeaseLost)
	var completed bool
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT completed_at IS NOT NULL FROM finops_anomaly_detection_leases WHERE workspace_id=$1 AND bucket_start=$2 AND detector_version=$3`, workspace.ID, bucket, service.FinOpsAnomalyDetectorVersion).Scan(&completed))
	require.False(t, completed, "expired worker cannot complete a replacement lease")
	require.NoError(t, repo.finishAnomalyLease(ctx, workspace.ID, bucket, service.FinOpsAnomalyDetectorVersion, replacementToken, nil))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT completed_at IS NOT NULL FROM finops_anomaly_detection_leases WHERE workspace_id=$1 AND bucket_start=$2 AND detector_version=$3`, workspace.ID, bucket, service.FinOpsAnomalyDetectorVersion).Scan(&completed))
	require.True(t, completed)
}
