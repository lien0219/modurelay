//go:build integration

package repository

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func completeLifecycleExportFixture(t *testing.T, ctx context.Context, r *workspaceRepository, actor, workspace int64) *service.LifecycleExportJob {
	t.Helper()
	j, err := r.CreateLifecycleExport(ctx, actor, workspace)
	require.NoError(t, err)
	claim, err := r.ClaimLifecycleExport(ctx)
	require.NoError(t, err)
	require.Equal(t, j.ID, claim.ID)
	var artifact bytes.Buffer
	result, err := r.WriteLifecycleExportSnapshot(ctx, claim, &artifact)
	require.NoError(t, err)
	require.NoError(t, r.FinishLifecycleExport(ctx, claim, result, nil))
	return claim
}

func TestLifecyclePostgresArtifactCleanupHonorsCurrentFloorAndHold(t *testing.T) {
	ctx, r, owner, w := lifecycleFixture(t)
	j := completeLifecycleExportFixture(t, ctx, r, owner.ID, w.ID)
	_, err := integrationDB.Exec(`UPDATE workspace_export_jobs SET completed_at=now()-interval '8 days',expires_at=now()-interval '1 day' WHERE id=$1`, j.ID)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE workspace_export_objects SET cleanup_after=now()-interval '1 second' WHERE export_id=$1`, j.ID)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`INSERT INTO workspace_lifecycle_holds(workspace_id,code,reason) VALUES($1,'LEGAL_HOLD','Retain export evidence')`, w.ID)
	require.NoError(t, err)
	var removed []string
	remove := func(_ context.Context, key string) error { removed = append(removed, key); return nil }
	require.NoError(t, r.CleanupLifecycleExports(ctx, remove))
	require.Empty(t, removed, "an active operator hold protects tenant artifacts")
	_, err = integrationDB.Exec(`UPDATE workspace_lifecycle_holds SET active=false WHERE workspace_id=$1`, w.ID)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE platform_retention_policies SET minimum_days=30,default_days=30 WHERE category='temporary'`)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE workspace_export_objects SET cleanup_after=now()-interval '1 second' WHERE export_id=$1`, j.ID)
	require.NoError(t, err)
	require.NoError(t, r.CleanupLifecycleExports(ctx, remove))
	require.Empty(t, removed, "a raised floor fences an artifact's old expiry")
	_, err = integrationDB.Exec(`UPDATE workspace_export_jobs SET completed_at=now()-interval '31 days' WHERE id=$1`, j.ID)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE workspace_export_objects SET cleanup_after=now()-interval '1 second' WHERE export_id=$1`, j.ID)
	require.NoError(t, err)
	require.NoError(t, r.CleanupLifecycleExports(ctx, remove))
	require.Equal(t, []string{j.ObjectKey}, removed)
	actual, err := r.GetLifecycleExport(ctx, owner.ID, w.ID, j.ID)
	require.NoError(t, err)
	require.Equal(t, "expired", actual.State)
	require.NoError(t, r.CleanupLifecycleExports(ctx, remove))
	require.Len(t, removed, 1, "cleanup cannot repeat a committed deletion")
}

func TestLifecyclePostgresDownloadRechecksExpiryRoleAndLiveSecurity(t *testing.T) {
	ctx, r, owner, w := lifecycleFixture(t)
	j := completeLifecycleExportFixture(t, ctx, r, owner.ID, w.ID)
	grant, err := r.AuthorizeLifecycleDownload(ctx, owner.ID, w.ID, j.ID)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE workspace_export_download_grants SET created_at=now()-interval '2 minutes',expires_at=now()-interval '1 minute' WHERE export_id=$1`, j.ID)
	require.NoError(t, err)
	_, err = r.RedeemLifecycleDownload(ctx, owner.ID, w.ID, j.ID, grant.Token)
	require.Error(t, err)
	grant, err = r.AuthorizeLifecycleDownload(ctx, owner.ID, w.ID, j.ID)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE workspace_members SET role='admin' WHERE workspace_id=$1 AND user_id=$2`, w.ID, owner.ID)
	require.NoError(t, err)
	_, err = r.RedeemLifecycleDownload(ctx, owner.ID, w.ID, j.ID, grant.Token)
	require.ErrorIs(t, err, service.ErrWorkspaceForbidden)
	_, err = integrationDB.Exec(`UPDATE workspace_members SET role='owner' WHERE workspace_id=$1 AND user_id=$2`, w.ID, owner.ID)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`INSERT INTO workspace_security_policies(workspace_id,require_mfa) VALUES($1,true) ON CONFLICT(workspace_id) DO UPDATE SET require_mfa=true`, w.ID)
	require.NoError(t, err)
	_, err = r.RedeemLifecycleDownload(ctx, owner.ID, w.ID, j.ID, grant.Token)
	require.Error(t, err, "grant does not bypass a newly required session factor")
	strong := service.WithSessionAuthentication(ctx, service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: time.Now().UTC(), MFASatisfied: true})
	_, err = r.RedeemLifecycleDownload(strong, owner.ID, w.ID, j.ID, grant.Token)
	require.NoError(t, err, "denied redemption never consumes the grant")
	_, err = integrationDB.Exec(`UPDATE workspace_export_jobs SET expires_at=now()-interval '1 second' WHERE id=$1`, j.ID)
	require.NoError(t, err)
	_, err = r.AuthorizeLifecycleDownload(strong, owner.ID, w.ID, j.ID)
	require.Error(t, err)
}

func TestLifecyclePostgresExportUncompressedAllowlistAndSecretExclusion(t *testing.T) {
	ctx, r, owner, w := lifecycleFixture(t)
	_, err := integrationDB.Exec(`INSERT INTO api_keys(user_id,project_id,key,name,status) SELECT $1,id,'sk-never-export-this','Safe credential label','active' FROM projects WHERE workspace_id=$2 AND is_default`, owner.ID, w.ID)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`INSERT INTO workspace_audit_logs(workspace_id,actor_user_id,action,target_type,target_id,metadata) VALUES($1,$2,'test_sensitive_audit','workspace',$1,'{"private_note":"private-audit-marker"}'::jsonb)`, w.ID, owner.ID)
	require.NoError(t, err)
	_, err = r.CreateLifecycleExport(ctx, owner.ID, w.ID)
	require.NoError(t, err)
	claim, err := r.ClaimLifecycleExport(ctx)
	require.NoError(t, err)
	var artifact bytes.Buffer
	_, err = r.WriteLifecycleExportSnapshot(ctx, claim, &artifact)
	require.NoError(t, err)
	z, err := zip.NewReader(bytes.NewReader(artifact.Bytes()), int64(artifact.Len()))
	require.NoError(t, err)
	files := map[string][]byte{}
	for _, f := range z.File {
		stream, e := f.Open()
		require.NoError(t, e)
		data, e := io.ReadAll(stream)
		require.NoError(t, e)
		require.NoError(t, stream.Close())
		files[f.Name] = data
		for _, secret := range []string{"sk-never-export-this", "private-audit-marker", owner.Email, "totp_secret", "encrypted_client_secret"} {
			require.NotContains(t, string(data), secret, f.Name)
		}
	}
	require.Contains(t, string(files["credentials.json"]), "Safe credential label")
	var manifest struct {
		Sections []string         `json:"included_sections"`
		Counts   map[string]int64 `json:"record_counts"`
		Total    int64            `json:"record_total"`
	}
	require.NoError(t, json.Unmarshal(files["manifest.json"], &manifest))
	var total int64
	for _, section := range manifest.Sections {
		require.Contains(t, files, section)
		total += manifest.Counts[section]
	}
	require.Equal(t, total, manifest.Total)
}

func TestLifecyclePostgresRestoreProjectAndCurrentProof(t *testing.T) {
	ctx, r, owner, w := lifecycleFixture(t)
	s := service.NewWorkspaceService(r)
	p, err := s.CreateProject(ctx, owner.ID, w.ID, service.ProjectInput{Name: "Restorable", Slug: "restorable"})
	require.NoError(t, err)
	require.NoError(t, s.ArchiveProject(ctx, owner.ID, w.ID, p.ID))
	stale := service.WithSessionAuthentication(ctx, service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: time.Now().UTC().Add(-11 * time.Minute)})
	require.ErrorIs(t, s.RestoreProject(stale, owner.ID, w.ID, p.ID), service.ErrRecentAuthenticationRequired)
	require.NoError(t, s.ArchiveWorkspace(ctx, owner.ID, w.ID))
	require.Error(t, s.RestoreProject(ctx, owner.ID, w.ID, p.ID), "archived parent cannot reopen a project")
	_, err = integrationDB.Exec(`UPDATE users SET totp_enabled=true WHERE id=$1`, owner.ID)
	require.NoError(t, err)
	require.Error(t, s.RestoreWorkspace(ctx, owner.ID, w.ID), "enrollment is not current factor proof")
	strong := service.WithSessionAuthentication(ctx, service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: time.Now().UTC(), MFASatisfied: true})
	require.NoError(t, s.RestoreWorkspace(strong, owner.ID, w.ID))
	actual, err := s.GetProject(strong, owner.ID, w.ID, p.ID)
	require.NoError(t, err)
	require.Equal(t, "archived", actual.Status, "workspace restore cannot silently reopen child projects")
	require.NoError(t, s.RestoreProject(strong, owner.ID, w.ID, p.ID))
	var events, outbox int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*),count(o.event_id) FROM domain_events e LEFT JOIN domain_event_outbox o ON o.event_id=e.id WHERE e.workspace_id=$1 AND e.event_type='project.restored'`, w.ID).Scan(&events, &outbox))
	require.Equal(t, 1, events)
	require.Equal(t, 1, outbox)
}

func TestLifecyclePostgresRestoreAndDeletionRaceCannotReopenJob(t *testing.T) {
	ctx, r, owner, w := lifecycleFixture(t)
	s := service.NewWorkspaceService(r)
	require.NoError(t, s.ArchiveWorkspace(ctx, owner.ID, w.ID))
	deliverLifecycleFixtureEvents(t, w.ID)
	challenge, err := r.LifecycleDeletionChallenge(ctx, owner.ID, w.ID)
	require.NoError(t, err)
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); <-start; errs <- s.RestoreWorkspace(ctx, owner.ID, w.ID) }()
	go func() {
		defer wg.Done()
		<-start
		_, e := r.RequestLifecycleDeletion(ctx, owner.ID, w.ID, w.Name, challenge.Token)
		errs <- e
	}()
	close(start)
	wg.Wait()
	close(errs)
	var successes int
	for e := range errs {
		if e == nil {
			successes++
		}
	}
	require.Positive(t, successes)
	j, err := r.GetLifecycleDeletion(ctx, owner.ID, w.ID)
	require.NoError(t, err)
	var status string
	require.NoError(t, integrationDB.QueryRow(`SELECT status FROM workspaces WHERE id=$1`, w.ID).Scan(&status))
	if j != nil {
		require.Equal(t, "pending_deletion", status)
		require.Error(t, s.RestoreWorkspace(ctx, owner.ID, w.ID))
	} else {
		require.Equal(t, "active", status)
	}
}

func TestLifecyclePostgresPurgeVerifiesResourcesBehindCursor(t *testing.T) {
	ctx, r, owner, w := lifecycleFixture(t)
	_, err := integrationDB.Exec(`INSERT INTO api_keys(user_id,project_id,key,name,status) SELECT $1,id,'sk-still-active','Must revoke','active' FROM projects WHERE workspace_id=$2 AND is_default`, owner.ID, w.ID)
	require.NoError(t, err)
	challenge, err := r.LifecycleDeletionChallenge(ctx, owner.ID, w.ID)
	require.NoError(t, err)
	j, err := r.RequestLifecycleDeletion(ctx, owner.ID, w.ID, w.Name, challenge.Token)
	require.NoError(t, err)
	makeLifecycleDeletionDue(t, j.ID)
	claim, err := r.ClaimLifecycleDeletion(ctx)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE workspace_deletion_jobs SET phase='projects',cursor=9223372036854775807 WHERE id=$1`, j.ID)
	require.NoError(t, err)
	require.Error(t, r.RunLifecyclePurgeBatch(ctx, claim, 200))
	actual, err := r.GetLifecycleDeletion(ctx, owner.ID, w.ID)
	require.NoError(t, err)
	require.False(t, actual.BusinessClosed)
	require.NotEqual(t, "completed", actual.State)
}

func TestLifecyclePostgresClosureRetainsPolicyProtectedConfiguration(t *testing.T) {
	ctx, r, owner, w := lifecycleFixture(t)
	s := service.NewWorkspaceService(r)
	p, err := s.CreateProject(ctx, owner.ID, w.ID, service.ProjectInput{Name: "Retention", Slug: "retention"})
	require.NoError(t, err)
	_, err = s.SetProjectAllocation(ctx, owner.ID, w.ID, p.ID, service.AllocationConfig{Environment: "production", Tags: map[string]string{}, PolicyRevision: 1})
	require.NoError(t, err)
	_, err = s.UpdateLifecycleRetention(ctx, owner.ID, w.ID, "operational", 0)
	require.NoError(t, err)
	deliverLifecycleFixtureEvents(t, w.ID)
	challenge, err := r.LifecycleDeletionChallenge(ctx, owner.ID, w.ID)
	require.NoError(t, err)
	j, err := r.RequestLifecycleDeletion(ctx, owner.ID, w.ID, w.Name, challenge.Token)
	require.NoError(t, err)
	makeLifecycleDeletionDue(t, j.ID)
	finishLifecycleDeletion(t, ctx, r, w.ID, owner.ID)
	var retained int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM project_cost_allocations WHERE workspace_id=$1 AND project_id=$2 AND environment='production'`, w.ID, p.ID).Scan(&retained))
	require.Equal(t, 1, retained, "a deletion job cannot override an indefinite tenant policy")
	actual, err := r.GetLifecycleDeletion(ctx, owner.ID, w.ID)
	require.NoError(t, err)
	require.True(t, actual.BusinessClosed, "policy-protected configuration does not prevent operational closure")
}

func TestLifecyclePostgresExpiredLeaseCannotCommitTerminalClosure(t *testing.T) {
	ctx, r, owner, w := lifecycleFixture(t)
	s := service.NewWorkspaceService(r)
	p, err := s.CreateProject(ctx, owner.ID, w.ID, service.ProjectInput{Name: "Lease", Slug: "lease"})
	require.NoError(t, err)
	_, err = s.SetProjectAllocation(ctx, owner.ID, w.ID, p.ID, service.AllocationConfig{Environment: "production", Tags: map[string]string{}, PolicyRevision: 1})
	require.NoError(t, err)
	_, err = s.UpdateLifecycleRetention(ctx, owner.ID, w.ID, "operational", 0)
	require.NoError(t, err)
	deliverLifecycleFixtureEvents(t, w.ID)
	challenge, err := r.LifecycleDeletionChallenge(ctx, owner.ID, w.ID)
	require.NoError(t, err)
	j, err := r.RequestLifecycleDeletion(ctx, owner.ID, w.ID, w.Name, challenge.Token)
	require.NoError(t, err)
	makeLifecycleDeletionDue(t, j.ID)
	claim, err := r.ClaimLifecycleDeletion(ctx)
	require.NoError(t, err)
	for i := 0; i < 80; i++ {
		actual, e := r.GetLifecycleDeletion(ctx, owner.ID, w.ID)
		require.NoError(t, e)
		if actual.Phase == "projects" && actual.Cursor > 0 {
			break
		}
		require.NoError(t, r.RunLifecyclePurgeBatch(ctx, claim, 200))
	}
	// Delay a terminal invariant read, after the worker's initial lease check.
	// This is confined to a disposable database; evidence guards stay enabled.
	_, err = integrationDB.Exec(`ALTER FUNCTION effective_lifecycle_retention_days(bigint,text) RENAME TO fixture_original_retention_days;
	 CREATE FUNCTION effective_lifecycle_retention_days(scope bigint,classification text) RETURNS integer LANGUAGE plpgsql STABLE AS $$ BEGIN IF classification='operational' THEN PERFORM pg_sleep(0.25); END IF; RETURN fixture_original_retention_days(scope,classification); END $$`)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE workspace_deletion_jobs SET lease_expires_at=clock_timestamp()+interval '200 milliseconds' WHERE id=$1`, j.ID)
	require.NoError(t, err)
	require.ErrorIs(t, r.RunLifecyclePurgeBatch(ctx, claim, 200), service.ErrLifecycleLeaseLost)
	actual, err := r.GetLifecycleDeletion(ctx, owner.ID, w.ID)
	require.NoError(t, err)
	require.False(t, actual.BusinessClosed)
	require.Equal(t, "running", actual.State)
	var status string
	require.NoError(t, integrationDB.QueryRow(`SELECT status FROM workspaces WHERE id=$1`, w.ID).Scan(&status))
	require.Equal(t, "purging", status)
}

func lifecycleCompletedImageFixture(t *testing.T, owner, workspace int64) string {
	t.Helper()
	batch := "lifecycle-image-" + uuid.NewString()
	_, err := integrationDB.Exec(`INSERT INTO batch_image_jobs(batch_id,user_id,workspace_id,project_id,billing_principal_user_id,provider,model,status,item_count,estimated_cost,actual_cost,settled_at,finished_at)
	 SELECT $1,$2,$3,id,$2,'gemini_api','image','completed',1,0,0,now(),now() FROM projects WHERE workspace_id=$3 AND is_default`, batch, owner, workspace)
	require.NoError(t, err)
	return batch
}

func TestLifecyclePostgresCleanedImageHistoryPermitsDeletion(t *testing.T) {
	ctx, r, owner, w := lifecycleFixture(t)
	batch := lifecycleCompletedImageFixture(t, owner.ID, w.ID)
	require.NoError(t, NewBatchImageRepository(integrationDB).MarkBatchImageOutputDeleted(ctx, batch, time.Now().UTC()))
	preflight, err := r.LifecyclePreflight(ctx, owner.ID, w.ID)
	require.NoError(t, err)
	require.True(t, preflight.Eligible, "normally cleaned terminal image history must not block business closure")
	challenge, err := r.LifecycleDeletionChallenge(ctx, owner.ID, w.ID)
	require.NoError(t, err)
	j, err := r.RequestLifecycleDeletion(ctx, owner.ID, w.ID, w.Name, challenge.Token)
	require.NoError(t, err)
	makeLifecycleDeletionDue(t, j.ID)
	finishLifecycleDeletion(t, ctx, r, w.ID, owner.ID)
}

func TestLifecyclePostgresLateImageOutputCleanupPermitsPurge(t *testing.T) {
	ctx, r, owner, w := lifecycleFixture(t)
	batch := lifecycleCompletedImageFixture(t, owner.ID, w.ID)
	challenge, err := r.LifecycleDeletionChallenge(ctx, owner.ID, w.ID)
	require.NoError(t, err)
	j, err := r.RequestLifecycleDeletion(ctx, owner.ID, w.ID, w.Name, challenge.Token)
	require.NoError(t, err)
	makeLifecycleDeletionDue(t, j.ID)
	claim, err := r.ClaimLifecycleDeletion(ctx)
	require.NoError(t, err)
	require.NoError(t, r.RunLifecyclePurgeBatch(ctx, claim, 200))
	require.NoError(t, NewBatchImageRepository(integrationDB).MarkBatchImageOutputDeleted(ctx, batch, time.Now().UTC()))
	for i := 0; i < 80; i++ {
		actual, e := r.GetLifecycleDeletion(ctx, owner.ID, w.ID)
		require.NoError(t, e)
		require.NotEqual(t, "blocked", actual.State, "late normal output cleanup must not strand the deletion")
		if actual.State == "completed" {
			require.True(t, actual.BusinessClosed)
			return
		}
		require.NoError(t, r.RunLifecyclePurgeBatch(ctx, claim, 200))
	}
	t.Fatal("purge did not complete after late terminal output cleanup")
}

func TestLifecyclePostgresUnknownImageOutcomesStillBlockDeletion(t *testing.T) {
	for _, scenario := range []struct{ status, errorCode string }{
		{"future_unknown_state", ""}, {"expired", ""}, {"completed", "SUBMIT_OUTCOME_UNKNOWN"}, {"output_deleted", "SUBMIT_OUTCOME_UNKNOWN"},
	} {
		t.Run(scenario.status+scenario.errorCode, func(t *testing.T) {
			ctx, r, owner, w := lifecycleFixture(t)
			batch := lifecycleCompletedImageFixture(t, owner.ID, w.ID)
			_, err := integrationDB.Exec(`UPDATE batch_image_jobs SET status=$2,last_error_code=$3 WHERE batch_id=$1`, batch, scenario.status, scenario.errorCode)
			require.NoError(t, err)
			preflight, err := r.LifecyclePreflight(ctx, owner.ID, w.ID)
			require.NoError(t, err)
			require.False(t, preflight.Eligible)
			require.Contains(t, preflight.BlockingReasons, service.LifecycleBlocker{Code: "PENDING_ASYNC_MEDIA", Reason: "Accepted image/video tasks or unknown submission outcomes require settlement or recovery.", Count: 1})
			challenge, err := r.LifecycleDeletionChallenge(ctx, owner.ID, w.ID)
			require.NoError(t, err)
			_, err = r.RequestLifecycleDeletion(ctx, owner.ID, w.ID, w.Name, challenge.Token)
			require.ErrorIs(t, err, service.ErrLifecycleBlocked)
		})
	}
}

func TestLifecyclePostgresWebhookEnqueueRechecksLockedWorkspaceState(t *testing.T) {
	ctx, _, owner, w := lifecycleFixture(t)
	var webhook int64
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO workspace_webhooks(workspace_id,name,url,secret_current_encrypted,created_by_user_id) VALUES($1,'Lifecycle','https://8.8.8.8/events','fixture-encrypted-secret',$2) RETURNING id`, w.ID, owner.ID).Scan(&webhook))
	_, err := integrationDB.Exec(`INSERT INTO workspace_webhook_subscriptions(webhook_id,event_type) VALUES($1,$2)`, webhook, service.EventWorkspaceDeletionRequested)
	require.NoError(t, err)
	event, err := service.NewDomainEvent(service.EventWorkspaceDeletionRequested, w.ID, 0, owner.ID, "workspace", "fixture", service.DomainEventData{"status": "pending"})
	require.NoError(t, err)
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	require.NoError(t, insertDomainEventTx(ctx, tx, event, ""))
	require.NoError(t, tx.Commit())

	// An in-flight transition holds the same parent lock as purge. A dispatcher
	// must wait, then use the committed state rather than an old endpoint view.
	transition, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = transition.Rollback() }()
	_, err = transition.Exec(`UPDATE workspaces SET status='purging' WHERE id=$1`, w.ID)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- NewWorkspaceWebhookRepository(integrationDB).EnqueueEventDeliveries(ctx, event) }()
	select {
	case err = <-done:
		require.NoError(t, err)
		t.Fatal("webhook dispatcher bypassed the in-flight Workspace transition lock")
	case <-time.After(150 * time.Millisecond):
	}
	require.NoError(t, transition.Commit())
	require.NoError(t, <-done)
	var deliveries int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_webhook_deliveries WHERE workspace_id=$1 AND event_id=$2`, w.ID, event.ID).Scan(&deliveries))
	require.Zero(t, deliveries, "a purging Workspace cannot admit new webhook work even before its endpoint cleanup phase")
}

func TestLifecyclePostgresRetentionRepositoryRechecksCurrentSecurity(t *testing.T) {
	ctx, r, owner, w := lifecycleFixture(t)
	_, err := integrationDB.Exec(`INSERT INTO workspace_security_policies(workspace_id,require_mfa) VALUES($1,true) ON CONFLICT(workspace_id) DO UPDATE SET require_mfa=true`, w.ID)
	require.NoError(t, err)
	_, err = r.LifecycleRetention(ctx, owner.ID, w.ID)
	require.Error(t, err, "direct policy reads must honor the current session's factor requirement")
	_, err = r.UpdateLifecycleRetention(ctx, owner.ID, w.ID, "operational", 365)
	require.Error(t, err, "direct policy changes must not rely only on their HTTP caller's security check")
	strong := service.WithSessionAuthentication(ctx, service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: time.Now().UTC(), MFASatisfied: true})
	_, err = r.LifecycleRetention(strong, owner.ID, w.ID)
	require.NoError(t, err)
	_, err = r.UpdateLifecycleRetention(strong, owner.ID, w.ID, "operational", 365)
	require.NoError(t, err)
}
