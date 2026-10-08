//go:build integration

package repository

import (
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceAllocationAdmissionSnapshotReportAndTenantIsolation(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, workspaces, owner, workspace := workspaceFixture(t)
	project, err := workspaces.CreateProject(ctx, owner.ID, workspace.ID, service.ProjectInput{Name: "FinOps", Slug: "finops"})
	require.NoError(t, err)
	center, err := workspaces.CreateAllocationCostCenter(ctx, owner.ID, workspace.ID, "core", "Core", "Primary services")
	require.NoError(t, err)
	var allocationAuditCount, allocationEventCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM workspace_audit_logs WHERE workspace_id=$1 AND action='allocation_center_created'`, workspace.ID).Scan(&allocationAuditCount))
	require.Equal(t, 1, allocationAuditCount)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM domain_events WHERE workspace_id=$1 AND event_type='workspace.updated' AND payload->'data'->>'operation'='create'`, workspace.ID).Scan(&allocationEventCount))
	require.Equal(t, 1, allocationEventCount)
	tag, err := workspaces.CreateAllocationTag(ctx, owner.ID, workspace.ID, "team", "platform", "Controlled team tag")
	require.NoError(t, err)
	centerID := center.ID
	config := service.AllocationConfig{CostCenterID: &centerID, Environment: service.AllocationEnvironmentProduction, Tags: map[string]string{tag.Key: tag.Value}, PolicyRevision: 1}
	projectAllocation, err := workspaces.SetProjectAllocation(ctx, owner.ID, workspace.ID, project.ID, config)
	require.NoError(t, err)
	require.Equal(t, int64(1), projectAllocation.Allocation.PolicyRevision)

	// A cost center from another workspace cannot be attached through a
	// workspace-scoped project mutation, even when the caller is an owner in
	// both workspaces.
	other, err := workspaces.CreateOrganization(ctx, owner.ID, "Other FinOps", fmt.Sprintf("other-finops-%d", workspace.ID))
	require.NoError(t, err)
	foreignCenter, err := workspaces.CreateAllocationCostCenter(ctx, owner.ID, other.ID, "foreign", "Foreign", "")
	require.NoError(t, err)
	foreignConfig := config
	foreignConfig.CostCenterID = &foreignCenter.ID
	foreignConfig.PolicyRevision = 2
	_, err = workspaces.SetProjectAllocation(ctx, owner.ID, workspace.ID, project.ID, foreignConfig)
	require.Error(t, err, "cross-tenant allocation must be rejected")

	var keyID int64
	err = integrationDB.QueryRowContext(ctx, `INSERT INTO api_keys(user_id,project_id,key,name,status) VALUES($1,$2,$3,'FinOps','active') RETURNING id`, owner.ID, project.ID, "sk-finops-"+uuid.NewString()).Scan(&keyID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE users SET balance=100 WHERE id=$1`, owner.ID)
	require.NoError(t, err)
	budget := NewBudgetRepository(integrationDB)
	attribution := service.BudgetAttribution{ActorUserID: owner.ID, APIKeyID: keyID, WorkspaceID: workspace.ID, ProjectID: project.ID, BillingPrincipalUserID: owner.ID, Allocation: &service.AllocationSnapshot{CostCenterID: &centerID, Environment: service.AllocationEnvironmentProduction, Tags: map[string]string{tag.Key: tag.Value}, PolicyRevision: 1}}
	reservation, err := budget.Reserve(ctx, attribution, "allocation-"+uuid.NewString(), 2)
	require.NoError(t, err)
	// An idempotent retry returns the first durable snapshot even if the
	// caller's freshly resolved configuration has since changed.
	changedAdmission := attribution
	changedAdmission.Allocation = &service.AllocationSnapshot{Environment: service.AllocationEnvironmentStaging, Tags: map[string]string{}, PolicyRevision: 2}
	retry, err := budget.Reserve(ctx, changedAdmission, reservation.RequestID, 2)
	require.NoError(t, err)
	require.Equal(t, reservation.ID, retry.ID)
	require.Equal(t, service.AllocationEnvironmentProduction, retry.Allocation.Environment)

	account := mustCreateAccount(t, testEntClient(t), &service.Account{Name: "finops-" + uuid.NewString(), Type: service.AccountTypeAPIKey, Platform: service.PlatformOpenAI})
	command := &service.UsageBillingCommand{RequestID: "billing-" + reservation.RequestID, UserID: owner.ID, APIKeyID: keyID, WorkspaceID: workspace.ID, ProjectID: project.ID, BillingPrincipalUserID: owner.ID, BudgetReservationID: reservation.ID, BudgetActualCost: 1, BalanceCost: 1, AccountID: account.ID, AccountType: service.AccountTypeAPIKey, Model: "gpt-5", ResolvedPlatform: service.PlatformOpenAI}
	log := &service.UsageLog{RequestID: command.RequestID, UserID: command.UserID, APIKeyID: command.APIKeyID, AccountID: command.AccountID, WorkspaceID: &command.WorkspaceID, ProjectID: &command.ProjectID, BillingPrincipalUserID: &command.BillingPrincipalUserID, BudgetReservationID: &command.BudgetReservationID, ResolvedPlatform: &command.ResolvedPlatform, Model: command.Model, ActualCost: 1, TotalCost: 1, RateMultiplier: 1, CreatedAt: time.Now().UTC()}
	usage, ok := NewUsageBillingRepository(testEntClient(t), integrationDB).(service.TenantUsageBillingRepository)
	require.True(t, ok)
	result, err := usage.ApplyTenantUsage(ctx, command, log)
	require.NoError(t, err)
	require.True(t, result.Applied)
	// A retry is idempotent and does not add a second snapshot or rollup row.
	result, err = usage.ApplyTenantUsage(ctx, command, log)
	require.NoError(t, err)
	require.False(t, result.Applied)

	var environment, tagJSON string
	var snapshotCenter int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT cost_center_id,environment,allocation_tags::text FROM usage_allocation_snapshots WHERE usage_log_id=(SELECT id FROM usage_logs WHERE request_id=$1 AND api_key_id=$2)`, command.RequestID, keyID).Scan(&snapshotCenter, &environment, &tagJSON))
	require.Equal(t, center.ID, snapshotCenter)
	require.Equal(t, service.AllocationEnvironmentProduction, environment)
	require.JSONEq(t, `{"team":"platform"}`, tagJSON)

	err = workspaces.ArchiveAllocationCostCenter(ctx, owner.ID, workspace.ID, center.ID)
	require.Error(t, err, "a referenced active project configuration cannot be archived")
	// Changing the mutable project policy cannot rewrite the admission evidence.
	next := config
	next.CostCenterID = nil
	next.Environment = service.AllocationEnvironmentStaging
	next.Tags = map[string]string{}
	next.PolicyRevision = 1
	_, err = workspaces.SetProjectAllocation(ctx, owner.ID, workspace.ID, project.ID, next)
	require.NoError(t, err)
	_, err = workspaces.SetProjectAllocation(ctx, owner.ID, workspace.ID, project.ID, service.AllocationConfig{Environment: service.AllocationEnvironmentDevelopment, Tags: map[string]string{}, PolicyRevision: 2})
	require.NoError(t, err)
	require.NoError(t, workspaces.ArchiveAllocationCostCenter(ctx, owner.ID, workspace.ID, center.ID))

	report, err := workspaces.GetAllocationReport(ctx, owner.ID, service.AllocationFilter{WorkspaceID: workspace.ID, ProjectID: project.ID, From: time.Now().Add(-2 * time.Hour).Format(time.RFC3339), To: time.Now().Add(2 * time.Hour).Format(time.RFC3339), Timezone: "UTC"})
	require.NoError(t, err)
	require.InDelta(t, 1, report.WorkspaceTotal, 1e-9)
	require.InDelta(t, 1, report.Allocated, 1e-9)
	require.InDelta(t, 0, report.Unallocated, 1e-9)
	require.NotEmpty(t, report.CostCenters)

	filtered, err := workspaces.GetAllocationReport(ctx, owner.ID, service.AllocationFilter{WorkspaceID: workspace.ID, ProjectID: project.ID, From: time.Now().Add(-2 * time.Hour).Format(time.RFC3339), To: time.Now().Add(2 * time.Hour).Format(time.RFC3339), Timezone: "UTC", TagKey: "team", TagValue: "platform"})
	require.NoError(t, err)
	require.InDelta(t, 1, filtered.WorkspaceTotal, 1e-9)
	require.InDelta(t, 1, filtered.Allocated, 1e-9)
	filtered, err = workspaces.GetAllocationReport(ctx, owner.ID, service.AllocationFilter{WorkspaceID: workspace.ID, ProjectID: project.ID, From: time.Now().Add(-2 * time.Hour).Format(time.RFC3339), To: time.Now().Add(2 * time.Hour).Format(time.RFC3339), Timezone: "UTC", TagKey: "team", TagValue: "other"})
	require.NoError(t, err)
	require.InDelta(t, 0, filtered.WorkspaceTotal, 1e-9)
	require.InDelta(t, 0, filtered.Allocated, 1e-9)

	_, err = integrationDB.ExecContext(ctx, `UPDATE usage_allocation_snapshots SET environment='testing' WHERE usage_log_id=(SELECT id FROM usage_logs WHERE request_id=$1 AND api_key_id=$2)`, command.RequestID, keyID)
	require.Error(t, err, "allocation evidence is immutable")
}

func TestWorkspaceAllocationAsyncVideoPlaceholderCapturesOnSettlement(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, workspaces, owner, workspace := workspaceFixture(t)
	project, err := workspaces.CreateProject(ctx, owner.ID, workspace.ID, service.ProjectInput{Name: "Video FinOps", Slug: "video-finops"})
	require.NoError(t, err)
	center, err := workspaces.CreateAllocationCostCenter(ctx, owner.ID, workspace.ID, "video", "Video", "")
	require.NoError(t, err)
	centerID := center.ID
	_, err = workspaces.SetProjectAllocation(ctx, owner.ID, workspace.ID, project.ID, service.AllocationConfig{CostCenterID: &centerID, Environment: service.AllocationEnvironmentProduction, PolicyRevision: 1})
	require.NoError(t, err)
	var keyID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO api_keys(user_id,project_id,key,name,status) VALUES($1,$2,$3,'Video FinOps','active') RETURNING id`, owner.ID, project.ID, "sk-video-finops-"+uuid.NewString()).Scan(&keyID))
	_, err = integrationDB.ExecContext(ctx, `UPDATE users SET balance=100 WHERE id=$1`, owner.ID)
	require.NoError(t, err)
	budget := NewBudgetRepository(integrationDB)
	attribution := service.BudgetAttribution{ActorUserID: owner.ID, APIKeyID: keyID, WorkspaceID: workspace.ID, ProjectID: project.ID, BillingPrincipalUserID: owner.ID, Allocation: &service.AllocationSnapshot{CostCenterID: &centerID, Environment: service.AllocationEnvironmentProduction, PolicyRevision: 1}}
	reservation, err := budget.Reserve(ctx, attribution, "video-allocation-"+uuid.NewString(), 2)
	require.NoError(t, err)
	account := mustCreateAccount(t, testEntClient(t), &service.Account{Name: "video-finops-" + uuid.NewString(), Type: service.AccountTypeAPIKey, Platform: service.PlatformOpenAI})
	platform := service.PlatformOpenAI
	created := time.Now().UTC()
	requestID := "grok-video:" + uuid.NewString()
	log := &service.UsageLog{RequestID: requestID, UserID: owner.ID, APIKeyID: keyID, AccountID: account.ID, Model: "video-model", VideoCount: 1, ActualCost: 0, TotalCost: 2, RateMultiplier: 1, CreatedAt: created}
	logs := NewUsageLogRepository(testEntClient(t), integrationDB)
	inserted, err := logs.Create(ctx, log)
	require.NoError(t, err)
	require.True(t, inserted)
	var snapshots int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM usage_allocation_snapshots WHERE usage_log_id=(SELECT id FROM usage_logs WHERE request_id=$1 AND api_key_id=$2)`, requestID, keyID).Scan(&snapshots))
	require.Zero(t, snapshots, "a zero-cost async video placeholder must not capture allocation evidence")
	settledLog := *log
	settledLog.ActualCost = 2
	settledLog.WorkspaceID = &workspace.ID
	settledLog.ProjectID = &project.ID
	settledLog.BillingPrincipalUserID = &owner.ID
	settledLog.BudgetReservationID = &reservation.ID
	settledLog.ResolvedPlatform = &platform
	usage, ok := NewUsageBillingRepository(testEntClient(t), integrationDB).(service.VideoUsageBillingRepository)
	require.True(t, ok)
	command := &service.UsageBillingCommand{RequestID: requestID, UserID: owner.ID, APIKeyID: keyID, WorkspaceID: workspace.ID, ProjectID: project.ID, BillingPrincipalUserID: owner.ID, BudgetReservationID: reservation.ID, BudgetActualCost: 2, BalanceCost: 2, AccountID: account.ID, AccountType: service.AccountTypeAPIKey, Model: "video-model", ResolvedPlatform: platform}
	result, err := usage.ApplyVideoUsage(ctx, command, &settledLog)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM usage_allocation_snapshots WHERE usage_log_id=(SELECT id FROM usage_logs WHERE request_id=$1 AND api_key_id=$2)`, requestID, keyID).Scan(&snapshots))
	require.Equal(t, 1, snapshots, "final video settlement must capture the reservation allocation")
	var capturedCenter int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT cost_center_id FROM usage_allocation_snapshots WHERE usage_log_id=(SELECT id FROM usage_logs WHERE request_id=$1 AND api_key_id=$2)`, requestID, keyID).Scan(&capturedCenter))
	require.Equal(t, centerID, capturedCenter)
}
