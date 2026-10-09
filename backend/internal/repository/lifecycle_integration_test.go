//go:build integration

package repository

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func lifecycleFixture(t *testing.T) (context.Context, *workspaceRepository, *service.User, *service.Workspace) {
	t.Helper()
	isolateWorkspaceTestFixtures(t)
	ctx := context.Background()
	owner := mustCreateUser(t, testEntClient(t), &service.User{Email: "lifecycle-" + uuid.NewString() + "@example.com"})
	repo := NewWorkspaceRepository(integrationDB).(*workspaceRepository)
	workspace, err := repo.CreateOrganization(ctx, owner.ID, "Lifecycle", "lifecycle-"+uuid.NewString())
	require.NoError(t, err)
	ctx = service.WithSessionAuthentication(ctx, service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: time.Now().UTC()})
	deliverLifecycleFixtureEvents(t, workspace.ID)
	return ctx, repo, owner, workspace
}

func makeLifecycleDeletionDue(t *testing.T, id string) {
	t.Helper()
	_, e := integrationDB.Exec(`UPDATE workspace_deletion_jobs SET created_at=now()-interval '8 days',earliest_purge_at=now()-interval '1 day',available_at=now()-interval '1 day' WHERE id=$1`, id)
	require.NoError(t, e)
}
func finishLifecycleDeletion(t *testing.T, ctx context.Context, r *workspaceRepository, w, a int64) {
	t.Helper()
	j, e := r.ClaimLifecycleDeletion(ctx)
	require.NoError(t, e)
	require.NotNil(t, j)
	for i := 0; i < 100; i++ {
		current, x := r.GetLifecycleDeletion(ctx, a, w)
		require.NoError(t, x)
		if current.State == "completed" {
			return
		}
		require.NoError(t, r.RunLifecyclePurgeBatch(ctx, j, 200))
	}
	t.Fatal("purge did not terminate")
}

func TestLifecyclePostgresHistoricalFinanceCompletesBusinessClosure(t *testing.T) {
	ctx, repo, owner, w := lifecycleFixture(t)
	ws := service.NewWorkspaceService(repo)
	other, e := repo.CreateOrganization(ctx, owner.ID, "Other tenant", "foreign-"+uuid.NewString())
	require.NoError(t, e)
	var foreignKey int64
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO api_keys(user_id,project_id,key,name,status) SELECT $1,id,'sk-foreign-private','foreign','active' FROM projects WHERE workspace_id=$2 AND is_default RETURNING id`, owner.ID, other.ID).Scan(&foreignKey))
	p, e := ws.CreateProject(ctx, owner.ID, w.ID, service.ProjectInput{Name: "Finance", Slug: "finance"})
	require.NoError(t, e)
	center, e := ws.CreateAllocationCostCenter(ctx, owner.ID, w.ID, "core", "Core", "")
	require.NoError(t, e)
	_, e = ws.SetProjectAllocation(ctx, owner.ID, w.ID, p.ID, service.AllocationConfig{CostCenterID: &center.ID, Environment: "production", Tags: map[string]string{}, PolicyRevision: 1})
	require.NoError(t, e)
	var key int64
	e = integrationDB.QueryRow(`INSERT INTO api_keys(user_id,project_id,key,name,status) VALUES($1,$2,'sk-secret-lifecycle','key','active') RETURNING id`, owner.ID, p.ID).Scan(&key)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`UPDATE users SET balance=100 WHERE id=$1`, owner.ID)
	require.NoError(t, e)
	budget := NewBudgetRepository(integrationDB)
	a := service.BudgetAttribution{ActorUserID: owner.ID, APIKeyID: key, WorkspaceID: w.ID, ProjectID: p.ID, BillingPrincipalUserID: owner.ID, Allocation: &service.AllocationSnapshot{CostCenterID: &center.ID, Environment: "production", Tags: map[string]string{}, PolicyRevision: 1}}
	res, e := budget.Reserve(ctx, a, uuid.NewString(), 2)
	require.NoError(t, e)
	account := mustCreateAccount(t, testEntClient(t), &service.Account{Name: "lifecycle-provider", Type: service.AccountTypeAPIKey, Platform: service.PlatformOpenAI})
	cmd := budgetBillingCommand(a, res, 1)
	cmd.AccountID = account.ID
	cmd.AccountType = service.AccountTypeAPIKey
	cmd.Model = "gpt-5"
	cmd.ResolvedPlatform = service.PlatformOpenAI
	log := &service.UsageLog{RequestID: cmd.RequestID, UserID: owner.ID, APIKeyID: key, AccountID: account.ID, WorkspaceID: &w.ID, ProjectID: &p.ID, BillingPrincipalUserID: &owner.ID, BudgetReservationID: &res.ID, ResolvedPlatform: &cmd.ResolvedPlatform, Model: cmd.Model, ActualCost: 1, TotalCost: 1, RateMultiplier: 1, CreatedAt: time.Now().UTC()}
	_, e = NewUsageBillingRepository(testEntClient(t), integrationDB).(service.TenantUsageBillingRepository).ApplyTenantUsage(ctx, cmd, log)
	require.NoError(t, e)
	var before string
	require.NoError(t, integrationDB.QueryRow(`SELECT row_to_json(s)::text FROM usage_allocation_snapshots s WHERE workspace_id=$1`, w.ID).Scan(&before))
	deliverLifecycleFixtureEvents(t, w.ID)
	pre, e := repo.LifecyclePreflight(ctx, owner.ID, w.ID)
	require.NoError(t, e)
	require.True(t, pre.Eligible)
	require.EqualValues(t, 1, pre.ProtectedRecords["usage_logs"])
	challenge, e := repo.LifecycleDeletionChallenge(ctx, owner.ID, w.ID)
	require.NoError(t, e)
	job, e := repo.RequestLifecycleDeletion(ctx, owner.ID, w.ID, w.Name, challenge.Token)
	require.NoError(t, e)
	_, e = budget.Reserve(ctx, a, uuid.NewString(), 1)
	require.Error(t, e, "pending deletion rejects new admission")
	makeLifecycleDeletionDue(t, job.ID)
	finishLifecycleDeletion(t, ctx, repo, w.ID, owner.ID)
	closed, e := repo.GetLifecycleDeletion(ctx, owner.ID, w.ID)
	require.NoError(t, e)
	require.True(t, closed.BusinessClosed)
	require.True(t, closed.ProtectedEvidenceRetained)
	var after string
	require.NoError(t, integrationDB.QueryRow(`SELECT row_to_json(s)::text FROM usage_allocation_snapshots s WHERE workspace_id=$1`, w.ID).Scan(&after))
	require.JSONEq(t, before, after)
	var keyValue, status string
	var balance string
	require.NoError(t, integrationDB.QueryRow(`SELECT key,status FROM api_keys WHERE id=$1`, key).Scan(&keyValue, &status))
	require.Equal(t, "inactive", status)
	require.NotEqual(t, "sk-secret-lifecycle", keyValue)
	require.NoError(t, integrationDB.QueryRow(`SELECT key,status FROM api_keys WHERE id=$1`, foreignKey).Scan(&keyValue, &status))
	require.Equal(t, "active", status)
	require.Equal(t, "sk-foreign-private", keyValue)
	require.NoError(t, integrationDB.QueryRow(`SELECT balance::text FROM users WHERE id=$1`, owner.ID).Scan(&balance))
	require.Contains(t, balance, "99")
	_, e = integrationDB.Exec(`DELETE FROM usage_allocation_snapshots WHERE workspace_id=$1`, w.ID)
	require.Error(t, e)
	_, e = integrationDB.Exec(`DELETE FROM usage_logs WHERE workspace_id=$1`, w.ID)
	require.Error(t, e)
	_, e = integrationDB.Exec(`DELETE FROM workspaces WHERE id=$1`, w.ID)
	require.Error(t, e)
}

func TestLifecyclePostgresLateHoldAndCompletionAuditRollback(t *testing.T) {
	ctx, r, owner, w := lifecycleFixture(t)
	challenge, e := r.LifecycleDeletionChallenge(ctx, owner.ID, w.ID)
	require.NoError(t, e)
	j, e := r.RequestLifecycleDeletion(ctx, owner.ID, w.ID, w.Name, challenge.Token)
	require.NoError(t, e)
	makeLifecycleDeletionDue(t, j.ID)
	_, e = integrationDB.Exec(`INSERT INTO workspace_lifecycle_holds(workspace_id,code,reason) VALUES($1,'BUSINESS_HOLD','Pending contractual review')`, w.ID)
	require.NoError(t, e)
	claim, e := r.ClaimLifecycleDeletion(ctx)
	require.NoError(t, e)
	require.NoError(t, r.RunLifecyclePurgeBatch(ctx, claim, 200))
	blocked, e := r.GetLifecycleDeletion(ctx, owner.ID, w.ID)
	require.NoError(t, e)
	require.Equal(t, "blocked", blocked.State)
	require.Zero(t, blocked.Progress)
	require.Zero(t, blocked.Attempts)
	require.Equal(t, "BUSINESS_HOLD", blocked.BlockingReasons[0].Code)
	_, e = integrationDB.Exec(`UPDATE workspace_lifecycle_holds SET active=false WHERE workspace_id=$1`, w.ID)
	require.NoError(t, e)
	_, e = r.RetryLifecycleDeletion(ctx, owner.ID, w.ID, j.ID)
	require.NoError(t, e)
	claim, e = r.ClaimLifecycleDeletion(ctx)
	require.NoError(t, e)
	require.NotNil(t, claim)
	for i := 0; i < 80; i++ {
		current, x := r.GetLifecycleDeletion(ctx, owner.ID, w.ID)
		require.NoError(t, x)
		if current.Phase == "projects" && current.Cursor > 0 {
			break
		}
		require.NoError(t, r.RunLifecyclePurgeBatch(ctx, claim, 200))
	}
	_, e = integrationDB.Exec(`CREATE FUNCTION reject_lifecycle_completed_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.event_type='workspace.deletion.completed' THEN RAISE EXCEPTION 'test final evidence failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_lifecycle_completed_event BEFORE INSERT ON domain_events FOR EACH ROW EXECUTE FUNCTION reject_lifecycle_completed_event()`)
	require.NoError(t, e)
	require.Error(t, r.RunLifecyclePurgeBatch(ctx, claim, 200))
	current, e := r.GetLifecycleDeletion(ctx, owner.ID, w.ID)
	require.NoError(t, e)
	require.Equal(t, "running", current.State)
	require.False(t, current.BusinessClosed)
	var state string
	require.NoError(t, integrationDB.QueryRow(`SELECT status FROM workspaces WHERE id=$1`, w.ID).Scan(&state))
	require.Equal(t, "purging", state)
	_, e = integrationDB.Exec(`DROP TRIGGER reject_lifecycle_completed_event ON domain_events; DROP FUNCTION reject_lifecycle_completed_event()`)
	require.NoError(t, e)
	require.NoError(t, r.RunLifecyclePurgeBatch(ctx, claim, 200))
	current, e = r.GetLifecycleDeletion(ctx, owner.ID, w.ID)
	require.NoError(t, e)
	require.Equal(t, "completed", current.State)
}

func TestLifecyclePostgresExpiredFifthExportAndCancellationCleanup(t *testing.T) {
	ctx, r, owner, w := lifecycleFixture(t)
	j, e := r.CreateLifecycleExport(ctx, owner.ID, w.ID)
	require.NoError(t, e)
	old, e := r.ClaimLifecycleExport(ctx)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`UPDATE workspace_export_jobs SET attempts=5,lease_expires_at=now()-interval '1 second' WHERE id=$1`, j.ID)
	require.NoError(t, e)
	claim, e := r.ClaimLifecycleExport(ctx)
	require.NoError(t, e)
	require.Nil(t, claim)
	failed, e := r.GetLifecycleExport(ctx, owner.ID, w.ID, j.ID)
	require.NoError(t, e)
	require.Equal(t, "failed", failed.State)
	j, e = r.CreateLifecycleExport(ctx, owner.ID, w.ID)
	require.NoError(t, e)
	claim, e = r.ClaimLifecycleExport(ctx)
	require.NoError(t, e)
	require.NoError(t, r.CancelLifecycleExport(ctx, owner.ID, w.ID, j.ID))
	var artifact bytes.Buffer
	_, e = r.WriteLifecycleExportSnapshot(ctx, claim, &artifact)
	require.ErrorIs(t, e, service.ErrLifecycleLeaseLost)
	var removed []string
	remove := func(_ context.Context, key string) error { removed = append(removed, key); return nil }
	require.NoError(t, r.CleanupLifecycleExports(ctx, remove))
	require.Empty(t, removed, "delayed cleanup excludes still possible in-flight uploads")
	_, e = integrationDB.Exec(`UPDATE workspace_export_objects SET cleanup_after=now()-interval '1 second'`)
	require.NoError(t, e)
	require.NoError(t, r.CleanupLifecycleExports(ctx, remove))
	require.NoError(t, r.CleanupLifecycleExports(ctx, remove))
	require.ElementsMatch(t, []string{old.ObjectKey, claim.ObjectKey}, removed)
}

func TestLifecyclePostgresPendingVideoAndHoldBlockButSettlementAfterArchiveSucceeds(t *testing.T) {
	for _, platform := range []string{service.PlatformSeedance, service.PlatformGrok} {
		t.Run(platform, func(t *testing.T) {
			isolateWorkspaceTestFixtures(t)
			ctx, budget, a := budgetFixture(t)
			ctx = service.WithSessionAuthentication(ctx, service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: time.Now().UTC()})
			repo := NewWorkspaceRepository(integrationDB).(*workspaceRepository)
			ws := service.NewWorkspaceService(repo)
			res, e := budget.Reserve(ctx, a, uuid.NewString(), 1)
			require.NoError(t, e)
			deliverLifecycleFixtureEvents(t, a.WorkspaceID)
			pre, e := repo.LifecyclePreflight(ctx, a.BillingPrincipalUserID, a.WorkspaceID)
			require.NoError(t, e)
			require.False(t, pre.Eligible)
			require.Equal(t, "PENDING_BUDGET_RESERVATIONS", pre.BlockingReasons[0].Code)
			require.NoError(t, ws.ArchiveWorkspace(ctx, a.BillingPrincipalUserID, a.WorkspaceID))
			account := mustCreateAccount(t, testEntClient(t), &service.Account{Name: "async-provider", Type: service.AccountTypeAPIKey, Platform: platform})
			cmd := budgetBillingCommand(a, res, 1)
			cmd.RequestID = "grok-video:" + platform + ":" + uuid.NewString()
			cmd.AccountID = account.ID
			cmd.AccountType = service.AccountTypeAPIKey
			cmd.Model = "video"
			cmd.ResolvedPlatform = platform
			log := &service.UsageLog{RequestID: cmd.RequestID, UserID: a.ActorUserID, APIKeyID: a.APIKeyID, AccountID: account.ID, WorkspaceID: &a.WorkspaceID, ProjectID: &a.ProjectID, BillingPrincipalUserID: &a.BillingPrincipalUserID, BudgetReservationID: &res.ID, ResolvedPlatform: &platform, Model: cmd.Model, VideoCount: 1, ActualCost: 1, TotalCost: 1, RateMultiplier: 1, CreatedAt: time.Now().UTC()}
			_, e = NewUsageBillingRepository(testEntClient(t), integrationDB).(service.VideoUsageBillingRepository).ApplyVideoUsage(ctx, cmd, log)
			require.NoError(t, e)
			_, e = integrationDB.Exec(`INSERT INTO workspace_lifecycle_holds(workspace_id,code,reason) VALUES($1,'LEGAL_HOLD','Approved investigation')`, a.WorkspaceID)
			require.NoError(t, e)
			deliverLifecycleFixtureEvents(t, a.WorkspaceID)
			pre, e = repo.LifecyclePreflight(ctx, a.BillingPrincipalUserID, a.WorkspaceID)
			require.NoError(t, e)
			require.False(t, pre.Eligible)
			require.Equal(t, "LEGAL_HOLD", pre.BlockingReasons[0].Code)
			_, e = integrationDB.Exec(`UPDATE workspace_lifecycle_holds SET active=false WHERE workspace_id=$1`, a.WorkspaceID)
			require.NoError(t, e)
			pre, e = repo.LifecyclePreflight(ctx, a.BillingPrincipalUserID, a.WorkspaceID)
			require.NoError(t, e)
			require.True(t, pre.Eligible)
		})
	}
}

func TestLifecyclePostgresSnapshotExcludesConcurrentCommitAndChecksums(t *testing.T) {
	ctx, r, owner, w := lifecycleFixture(t)
	job, e := r.CreateLifecycleExport(ctx, owner.ID, w.ID)
	require.NoError(t, e)
	claim, e := r.ClaimLifecycleExport(ctx)
	require.NoError(t, e)
	require.Equal(t, job.ID, claim.ID)
	tx, e := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, e)
	defer func() { _ = tx.Rollback() }()
	_, e = tx.Exec(`LOCK TABLE projects IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, e)
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() { _, x := r.WriteLifecycleExportSnapshot(ctx, claim, &out); done <- x }()
	deadline := time.Now().Add(5 * time.Second)
	observed := false
	for time.Now().Before(deadline) {
		var n int64
		require.NoError(t, integrationDB.QueryRow(`SELECT progress FROM workspace_export_jobs WHERE id=$1`, job.ID).Scan(&n))
		if n >= 1 {
			observed = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.True(t, observed, "snapshot acquired before project writer commits")
	_, e = tx.Exec(`INSERT INTO projects(workspace_id,name,slug,created_by_user_id) VALUES($1,'Concurrent project','concurrent',$2)`, w.ID, owner.ID)
	require.NoError(t, e)
	require.NoError(t, tx.Commit())
	require.NoError(t, <-done)
	z, e := zip.NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()))
	require.NoError(t, e)
	files := map[string][]byte{}
	for _, f := range z.File {
		stream, x := f.Open()
		require.NoError(t, x)
		data, x := io.ReadAll(stream)
		require.NoError(t, x)
		require.NoError(t, stream.Close())
		files[f.Name] = data
	}
	require.NotContains(t, string(files["projects.json"]), "Concurrent project")
	var manifest struct {
		Checksums    map[string]string `json:"checksums"`
		RecordCounts map[string]int64  `json:"record_counts"`
		WorkspaceID  int64             `json:"workspace_id"`
	}
	require.NoError(t, json.Unmarshal(files["manifest.json"], &manifest))
	require.Equal(t, w.ID, manifest.WorkspaceID)
	for name, want := range manifest.Checksums {
		sum := sha256.Sum256(files[name])
		require.Equal(t, want, hex.EncodeToString(sum[:]), name)
	}
	require.EqualValues(t, 1, manifest.RecordCounts["projects.json"])
}

func deliverLifecycleFixtureEvents(t *testing.T, w int64) {
	t.Helper()
	_, err := integrationDB.Exec(`UPDATE domain_event_outbox SET delivered_at=now(),locked_until=NULL,lock_token=NULL WHERE event_id IN (SELECT id FROM domain_events WHERE workspace_id=$1)`, w)
	require.NoError(t, err)
}

func TestLifecyclePostgresRestoreAndDeletionConfirmation(t *testing.T) {
	ctx, repo, owner, w := lifecycleFixture(t)
	workspaces := service.NewWorkspaceService(repo)
	require.NoError(t, workspaces.ArchiveWorkspace(ctx, owner.ID, w.ID))
	require.NoError(t, workspaces.RestoreWorkspace(ctx, owner.ID, w.ID))
	deliverLifecycleFixtureEvents(t, w.ID)
	preflight, err := repo.LifecyclePreflight(ctx, owner.ID, w.ID)
	require.NoError(t, err)
	require.True(t, preflight.Eligible)
	require.True(t, preflight.ProtectedEvidenceRetained)
	challenge, err := repo.LifecycleDeletionChallenge(ctx, owner.ID, w.ID)
	require.NoError(t, err)
	_, err = repo.RequestLifecycleDeletion(ctx, owner.ID, w.ID, "wrong", challenge.Token)
	require.Error(t, err)
	job, err := repo.RequestLifecycleDeletion(ctx, owner.ID, w.ID, w.Name, challenge.Token)
	require.NoError(t, err)
	require.Equal(t, "pending", job.State)
	require.Error(t, workspaces.RestoreWorkspace(ctx, owner.ID, w.ID))
	_, err = repo.RequestLifecycleDeletion(ctx, owner.ID, w.ID, w.Name, challenge.Token)
	require.Error(t, err)
	require.NoError(t, repo.CancelLifecycleDeletion(ctx, owner.ID, w.ID, job.ID))
	actual, err := workspaces.GetWorkspace(ctx, owner.ID, w.ID)
	require.NoError(t, err)
	require.Equal(t, "active", actual.Status)
	var n int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_events WHERE workspace_id=$1 AND event_type='workspace.deletion.cancelled'`, w.ID).Scan(&n))
	require.Equal(t, 1, n)
}

func TestLifecyclePostgresExportScopeFencingAndSnapshot(t *testing.T) {
	ctx, repo, owner, w := lifecycleFixture(t)
	other, err := repo.CreateOrganization(ctx, owner.ID, "Other", "other-"+uuid.NewString())
	require.NoError(t, err)
	job, err := repo.CreateLifecycleExport(ctx, owner.ID, w.ID)
	require.NoError(t, err)
	_, err = repo.GetLifecycleExport(ctx, owner.ID, other.ID, job.ID)
	require.Error(t, err)
	claimed, err := repo.ClaimLifecycleExport(ctx)
	require.NoError(t, err)
	require.Equal(t, job.ID, claimed.ID)
	var artifact bytes.Buffer
	result, err := repo.WriteLifecycleExportSnapshot(ctx, claimed, &artifact)
	require.NoError(t, err)
	require.Positive(t, result.Records)
	require.NotContains(t, artifact.String(), "totp_secret")
	require.NoError(t, repo.FinishLifecycleExport(ctx, claimed, result, nil))
	require.Error(t, repo.FinishLifecycleExport(ctx, claimed, result, nil))
	grant, err := repo.AuthorizeLifecycleDownload(ctx, owner.ID, w.ID, job.ID)
	require.NoError(t, err)
	_, err = repo.RedeemLifecycleDownload(ctx, owner.ID, other.ID, job.ID, grant.Token)
	require.Error(t, err)
	_, err = repo.RedeemLifecycleDownload(ctx, owner.ID, w.ID, job.ID, grant.Token)
	require.NoError(t, err)
	_, err = repo.RedeemLifecycleDownload(ctx, owner.ID, w.ID, job.ID, grant.Token)
	require.Error(t, err)
}

func TestLifecyclePostgresPurgeSingleLeaseAndRestart(t *testing.T) {
	ctx, repo, owner, w := lifecycleFixture(t)
	challenge, err := repo.LifecycleDeletionChallenge(ctx, owner.ID, w.ID)
	require.NoError(t, err)
	job, err := repo.RequestLifecycleDeletion(ctx, owner.ID, w.ID, w.Name, challenge.Token)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE workspace_deletion_jobs SET created_at=now()-interval '8 days',earliest_purge_at=now()-interval '1 day',available_at=now()-interval '1 day' WHERE id=$1`, job.ID)
	require.NoError(t, err)
	var wg sync.WaitGroup
	claimed := make(chan *service.LifecycleDeletionJob, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			j, e := repo.ClaimLifecycleDeletion(ctx)
			if e == nil && j != nil {
				claimed <- j
			}
		}()
	}
	wg.Wait()
	close(claimed)
	var claim *service.LifecycleDeletionJob
	count := 0
	for j := range claimed {
		claim = j
		count++
	}
	require.Equal(t, 1, count)
	require.NoError(t, repo.RunLifecyclePurgeBatch(ctx, claim, 1))
	require.Error(t, repo.CancelLifecycleDeletion(ctx, owner.ID, w.ID, job.ID))
	_, err = integrationDB.Exec(`UPDATE workspace_deletion_jobs SET lease_expires_at=now()-interval '1 second',available_at=now()-interval '1 second' WHERE id=$1`, job.ID)
	require.NoError(t, err)
	replacement, err := repo.ClaimLifecycleDeletion(ctx)
	require.NoError(t, err)
	require.NotNil(t, replacement)
	require.NotEqual(t, claim.LeaseToken, replacement.LeaseToken)
	require.Error(t, repo.RunLifecyclePurgeBatch(ctx, claim, 1))
	for i := 0; i < 80; i++ {
		current, e := repo.GetLifecycleDeletion(ctx, owner.ID, w.ID)
		require.NoError(t, e)
		if current.State == "completed" {
			break
		}
		require.NoError(t, repo.RunLifecyclePurgeBatch(ctx, replacement, 1))
	}
	completed, err := repo.GetLifecycleDeletion(ctx, owner.ID, w.ID)
	require.NoError(t, err)
	require.Equal(t, "completed", completed.State)
	require.True(t, completed.BusinessClosed)
	require.True(t, completed.ProtectedEvidenceRetained)
	var userCount, eventCount int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM users WHERE id=$1 AND deleted_at IS NULL`, owner.ID).Scan(&userCount))
	require.Equal(t, 1, userCount)
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_events WHERE workspace_id=$1 AND event_type='workspace.deletion.completed'`, w.ID).Scan(&eventCount))
	require.Equal(t, 1, eventCount)
	data, _ := json.Marshal(completed)
	require.NotContains(t, string(data), "lease_token")
}
