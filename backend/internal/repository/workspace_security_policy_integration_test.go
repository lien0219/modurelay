//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

// Historical migration rerun tests must restore the latest event allowlist.
func restoreWorkspaceSecurityMigration(t *testing.T) {
	t.Helper()
	body, err := migrations.FS.ReadFile("297_workspace_security_policy.sql")
	require.NoError(t, err)
	_, err = integrationDB.Exec(string(body))
	require.NoError(t, err)
	phaseE, err := migrations.FS.ReadFile("298_finops_anomalies.sql")
	require.NoError(t, err)
	_, err = integrationDB.Exec(string(phaseE))
	require.NoError(t, err)
}

func workspaceSecurityActorContext(ctx context.Context) context.Context {
	return service.WithSessionAuthentication(ctx, service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: time.Now().UTC(), MFASatisfied: true, MFAEnrolled: true})
}

func TestWorkspaceSecurityPolicyRevisionRBACDefaultsAndNullPatch(t *testing.T) {
	ctx, repo, owner, w, _, _ := enterpriseIdentityFixture(t)
	actor := workspaceSecurityActorContext(ctx)
	policy, err := repo.GetPolicy(actor, w.ID, owner.ID)
	require.NoError(t, err)
	require.False(t, policy.RequireMFA)
	require.Nil(t, policy.SessionMaxAgeSeconds)
	require.Equal(t, "any", policy.InvitationPolicy)
	require.True(t, policy.AllowExternalMembers)
	require.True(t, policy.WorkspaceJITEnabled)
	require.Equal(t, "any_active", policy.ApprovedIdentityProviderMode)
	_, err = integrationDB.Exec(`UPDATE users SET totp_enabled=true,totp_secret_encrypted='fixture-ciphertext' WHERE id=$1`, owner.ID)
	require.NoError(t, err)
	enabled := true
	age := 3600
	updated, err := repo.PatchSecurityPolicy(actor, w.ID, owner.ID, service.WorkspaceSecurityPolicyPatch{ExpectedRevision: policy.Revision, RequireMFA: &enabled, SessionMaxAgeSeconds: service.NullableSecurityValue[int]{Present: true, Value: &age}})
	require.NoError(t, err)
	require.Equal(t, policy.Revision+1, updated.Revision)
	require.True(t, updated.RequireMFA)
	_, err = repo.PatchSecurityPolicy(actor, w.ID, owner.ID, service.WorkspaceSecurityPolicyPatch{ExpectedRevision: policy.Revision})
	require.ErrorIs(t, err, service.ErrWorkspaceSecurityPolicyConflict)
	updated, err = repo.PatchSecurityPolicy(actor, w.ID, owner.ID, service.WorkspaceSecurityPolicyPatch{ExpectedRevision: updated.Revision, SessionMaxAgeSeconds: service.NullableSecurityValue[int]{Present: true}})
	require.NoError(t, err)
	require.Nil(t, updated.SessionMaxAgeSeconds)
	require.True(t, updated.RequireMFA)
	var audits, events, outbox int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_audit_logs WHERE workspace_id=$1 AND action='workspace.security_policy.updated'`, w.ID).Scan(&audits))
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_events WHERE workspace_id=$1 AND event_type='workspace.security_policy.updated'`, w.ID).Scan(&events))
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_event_outbox o JOIN domain_events e ON e.id=o.event_id WHERE e.workspace_id=$1 AND e.event_type='workspace.security_policy.updated'`, w.ID).Scan(&outbox))
	require.Equal(t, 2, audits)
	require.Equal(t, audits, events)
	require.Equal(t, events, outbox)
	ws := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
	admin := workspaceJoin(t, actor, ws, owner.ID, w.ID, "admin")
	_, err = repo.PatchSecurityPolicy(actor, w.ID, admin.ID, service.WorkspaceSecurityPolicyPatch{ExpectedRevision: updated.Revision})
	require.ErrorIs(t, err, service.ErrWorkspaceForbidden)
	require.NoError(t, service.CheckWorkspacePermission(&service.WorkspaceAccess{Workspace: w, Member: &service.WorkspaceMember{Role: "viewer", Status: "active"}}, "workspace_security.read"))
}

func TestWorkspaceSecurityPolicyConcurrentRevisionAndSelfLockout(t *testing.T) {
	ctx, repo, owner, w, _, _ := enterpriseIdentityFixture(t)
	actor := workspaceSecurityActorContext(ctx)
	enabled := true
	_, err := repo.PatchSecurityPolicy(actor, w.ID, owner.ID, service.WorkspaceSecurityPolicyPatch{ExpectedRevision: 1, RequireMFA: &enabled})
	require.ErrorIs(t, err, service.ErrMFARequired, "session proof without enrolled factor cannot enable")
	_, err = integrationDB.Exec(`UPDATE users SET totp_enabled=true,totp_secret_encrypted='fixture-ciphertext' WHERE id=$1`, owner.ID)
	require.NoError(t, err)
	unauthenticated := service.WithSessionAuthentication(ctx, service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: time.Now(), MFAEnrolled: true})
	_, err = repo.PatchSecurityPolicy(unauthenticated, w.ID, owner.ID, service.WorkspaceSecurityPolicyPatch{ExpectedRevision: 1, RequireMFA: &enabled})
	require.ErrorIs(t, err, service.ErrMFARequired, "enrollment never authenticates current session")
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, e := repo.PatchSecurityPolicy(actor, w.ID, owner.ID, service.WorkspaceSecurityPolicyPatch{ExpectedRevision: 1, RequireMFA: &enabled})
			results <- e
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	successes, conflicts := 0, 0
	for e := range results {
		if e == nil {
			successes++
		} else {
			require.ErrorIs(t, e, service.ErrWorkspaceSecurityPolicyConflict)
			conflicts++
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
}

func TestWorkspaceSecurityRecentProofUsesLockedEnrollment(t *testing.T) {
	ctx, repo, owner, w, _, _ := enterpriseIdentityFixture(t)
	_, err := integrationDB.Exec(`UPDATE users SET totp_enabled=true,totp_secret_encrypted='fixture-ciphertext' WHERE id=$1`, owner.ID)
	require.NoError(t, err)
	// An internal caller cannot omit the live hint to weaken the recent-auth guard.
	actor := service.WithSessionAuthentication(ctx, service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: time.Now()})
	disabled := "disabled"
	patch := service.WorkspaceSecurityPolicyPatch{ExpectedRevision: 1, InvitationPolicy: &disabled}
	_, err = repo.PatchSecurityPolicy(actor, w.ID, owner.ID, patch)
	require.ErrorIs(t, err, service.ErrRecentAuthenticationRequired)
	_, err = repo.PatchSecurityPolicy(service.WithRecentAuthentication(actor, time.Now()), w.ID, owner.ID, patch)
	require.ErrorIs(t, err, service.ErrRecentAuthenticationRequired, "time-only handler proof cannot become factor verification after enrollment")
	policy, err := repo.GetPolicy(ctx, w.ID, owner.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, policy.Revision)
	actor = service.WithRecentAuthentication(actor, time.Now(), true)
	policy, err = repo.PatchSecurityPolicy(actor, w.ID, owner.ID, patch)
	require.NoError(t, err, "trusted step-up remains usable without changing original session proof")
	require.Equal(t, disabled, policy.InvitationPolicy)
}

func TestWorkspaceSecurityInheritedExpiredGraceDoesNotBlockUnrelatedPatch(t *testing.T) {
	ctx, repo, owner, w, provider, _ := enterpriseIdentityFixture(t)
	link := enterpriseProvisionInput(w.ID, provider, "expired-grace-owner", owner.Email)
	link.LinkUserID = &owner.ID
	_, err := repo.CompleteIdentityLogin(ctx, link)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`INSERT INTO workspace_security_policies(workspace_id,require_sso,sso_grace_until) VALUES($1,true,now()-interval '1 hour')`, w.ID)
	require.NoError(t, err)
	actor := service.WithAuthenticationAssurance(workspaceSecurityActorContext(ctx), service.WorkspaceAssurance{WorkspaceID: w.ID, ProviderID: provider.ID, ProviderRevision: provider.Revision, AuthenticatedAt: time.Now(), AuthMethod: "oidc"})
	disabled := "disabled"
	policy, err := repo.PatchSecurityPolicy(actor, w.ID, owner.ID, service.WorkspaceSecurityPolicyPatch{ExpectedRevision: 1, InvitationPolicy: &disabled})
	require.NoError(t, err, "an elapsed grace period must enforce SSO without invalidating unrelated PATCH fields")
	require.True(t, policy.RequireSSO)
	require.Equal(t, "disabled", policy.InvitationPolicy)
	require.NotNil(t, policy.SSOGraceUntil, "omitted grace field retains its stored value")
	require.True(t, policy.SSOGraceUntil.Before(time.Now()))
	unchanged := service.WorkspaceSecurityPolicyPatch{ExpectedRevision: policy.Revision, SSOGraceUntil: service.NullableSecurityValue[time.Time]{Present: true, Value: policy.SSOGraceUntil}}
	preview, err := repo.PreviewSecurityPolicy(actor, w.ID, owner.ID, unchanged)
	require.NoError(t, err)
	require.Empty(t, preview.PrerequisiteReason)
	policy, err = repo.PatchSecurityPolicy(actor, w.ID, owner.ID, unchanged)
	require.NoError(t, err, "full editor payload may submit the identical inherited elapsed deadline")
	for _, deadline := range []time.Time{policy.SSOGraceUntil.Add(-time.Minute), time.Now().Add(8 * 24 * time.Hour)} {
		changed := service.WorkspaceSecurityPolicyPatch{ExpectedRevision: policy.Revision, SSOGraceUntil: service.NullableSecurityValue[time.Time]{Present: true, Value: &deadline}}
		_, err = repo.PreviewSecurityPolicy(actor, w.ID, owner.ID, changed)
		require.ErrorIs(t, err, service.ErrWorkspaceInvalid)
		_, err = repo.PatchSecurityPolicy(actor, w.ID, owner.ID, changed)
		require.ErrorIs(t, err, service.ErrWorkspaceInvalid, "newly changed grace remains bounded")
	}
}

func TestWorkspaceSecurityBreakGlassUsesLockedEnrollment(t *testing.T) {
	ctx, repo, owner, w, _, _ := enterpriseIdentityFixture(t)
	// A request began before enrollment. Its live hint is now stale, while the
	// enrolled factor must still be required by the locked recovery writer.
	actor := service.WithSessionAuthentication(ctx, service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: time.Now(), MFAEnrolled: false})
	_, err := integrationDB.Exec(`UPDATE users SET totp_enabled=true,totp_secret_encrypted='fixture-ciphertext' WHERE id=$1`, owner.ID)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`INSERT INTO workspace_security_policies(workspace_id,require_mfa) VALUES($1,true)`, w.ID)
	require.NoError(t, err)
	before := workspaceAdmissionSnapshot(t, w.ID)
	_, err = repo.BreakGlass(actor, w.ID, owner.ID, "factor enrollment raced recovery")
	require.ErrorIs(t, err, service.ErrRecentAuthenticationRequired, "password-only session cannot bypass the live enrolled factor during recovery")
	_, err = repo.BreakGlass(service.WithRecentAuthentication(actor, time.Now()), w.ID, owner.ID, "time-only proof cannot bypass factor")
	require.ErrorIs(t, err, service.ErrRecentAuthenticationRequired)
	require.Equal(t, before, workspaceAdmissionSnapshot(t, w.ID), "denial consumes no recovery allowance and writes no durable effects")
	var required bool
	require.NoError(t, integrationDB.QueryRow(`SELECT require_mfa FROM workspace_security_policies WHERE workspace_id=$1`, w.ID).Scan(&required))
	require.True(t, required)
	var allowances int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_identity_break_glass_limits WHERE workspace_id=$1`, w.ID).Scan(&allowances))
	require.Zero(t, allowances)
	_, err = repo.BreakGlass(service.WithRecentAuthentication(actor, time.Now(), true), w.ID, owner.ID, "verified recent factor recovery")
	require.NoError(t, err, "separate trusted step-up still permits recovery without forging durable Session MFA")
}

func TestWorkspaceSecurityPolicyWriterDoesNotDeadlockAuditedMutation(t *testing.T) {
	ctx, repo, owner, w, _, _ := enterpriseIdentityFixture(t)
	actor := workspaceSecurityActorContext(ctx)
	ws := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
	gateID := w.ID + 810000000
	// Pause the real mutation immediately before its audit's actor FK obtains
	// KEY SHARE. The policy writer can then own the actor lock while waiting
	// for the Workspace already owned by that mutation.
	_, err := integrationDB.Exec(fmt.Sprintf(`CREATE FUNCTION phase_d_pause_workspace_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.workspace_id=%d AND NEW.action='workspace_updated' THEN PERFORM pg_advisory_xact_lock(%d); END IF; RETURN NEW; END $$; CREATE TRIGGER phase_d_pause_workspace_audit BEFORE INSERT ON workspace_audit_logs FOR EACH ROW EXECUTE FUNCTION phase_d_pause_workspace_audit()`, w.ID, gateID))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := integrationDB.Exec(`DROP TRIGGER IF EXISTS phase_d_pause_workspace_audit ON workspace_audit_logs; DROP FUNCTION IF EXISTS phase_d_pause_workspace_audit()`)
		require.NoError(t, err)
	})
	gate, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer gate.Rollback()
	_, err = gate.Exec(`SELECT pg_advisory_xact_lock($1)`, gateID)
	require.NoError(t, err)
	var gatePID int
	require.NoError(t, gate.QueryRow(`SELECT pg_backend_pid()`).Scan(&gatePID))
	raceCtx, cancel := context.WithTimeout(actor, 20*time.Second)
	defer cancel()
	mutationDone := make(chan error, 1)
	go func() {
		_, err := ws.UpdateWorkspace(raceCtx, owner.ID, w.ID, "Audited lock order", w.Slug)
		mutationDone <- err
		close(mutationDone)
	}()
	defer finishAdmissionWriter(t, gate, mutationDone)
	var mutationPID int
	require.Eventually(t, func() bool {
		err := integrationDB.QueryRow(`SELECT pid FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'INSERT INTO workspace_audit_logs%' AND $1=ANY(pg_blocking_pids(pid))`, gatePID).Scan(&mutationPID)
		return err == nil
	}, 5*time.Second, 10*time.Millisecond, "actual Workspace mutation must reach its audit while owning the Workspace")
	policyDone := make(chan error, 1)
	go func() {
		disabled := "disabled"
		_, err := repo.PatchSecurityPolicy(raceCtx, w.ID, owner.ID, service.WorkspaceSecurityPolicyPatch{ExpectedRevision: 1, InvitationPolicy: &disabled})
		policyDone <- err
		close(policyDone)
	}()
	defer func() {
		_ = gate.Rollback()
		cancel()
		select {
		case <-policyDone:
		case <-time.After(10 * time.Second):
			t.Error("policy writer did not finish after audit gate release")
		}
	}()
	waitForAdmissionWorkspaceLock(t, mutationPID)
	require.NoError(t, gate.Commit())
	mutationErr, policyErr := <-mutationDone, <-policyDone
	require.NoError(t, mutationErr, "audited mutation must not deadlock against the policy actor lock")
	require.NoError(t, policyErr, "policy writer must serialize after the audited mutation")
	policy, err := repo.GetPolicy(ctx, w.ID, owner.ID)
	require.NoError(t, err)
	require.EqualValues(t, 2, policy.Revision)
	require.Equal(t, "disabled", policy.InvitationPolicy)
	var name string
	require.NoError(t, integrationDB.QueryRow(`SELECT name FROM workspaces WHERE id=$1`, w.ID).Scan(&name))
	require.Equal(t, "Audited lock order", name)
	var audits int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_audit_logs WHERE workspace_id=$1 AND action IN('workspace_updated','workspace.security_policy.updated')`, w.ID).Scan(&audits))
	require.Equal(t, 2, audits, "both actual mutations commit their audit")
}

func TestWorkspaceSecurityConcurrentProviderApprovalAndDisable(t *testing.T) {
	for attempt := 0; attempt < 4; attempt++ {
		ctx, repo, owner, w, p, _ := enterpriseIdentityFixture(t)
		link := enterpriseProvisionInput(w.ID, p, "provider-race-owner", owner.Email)
		link.LinkUserID = &owner.ID
		_, err := repo.CompleteIdentityLogin(ctx, link)
		require.NoError(t, err)
		actor := service.WithAuthenticationAssurance(workspaceSecurityActorContext(ctx), service.WorkspaceAssurance{WorkspaceID: w.ID, ProviderID: p.ID, ProviderRevision: p.Revision, AuthenticatedAt: time.Now(), AuthMethod: "oidc"})
		raceCtx, cancel := context.WithTimeout(actor, 10*time.Second)
		start, approved, disabled := make(chan struct{}), make(chan error, 1), make(chan error, 1)
		go func() {
			<-start
			required, mode, ids := true, "selected", []int64{p.ID}
			_, err := repo.PatchSecurityPolicy(raceCtx, w.ID, owner.ID, service.WorkspaceSecurityPolicyPatch{ExpectedRevision: 1, RequireSSO: &required, ApprovedIdentityProviderMode: &mode, ApprovedIdentityProviderIDs: &ids})
			approved <- err
		}()
		go func() { <-start; disabled <- repo.DisableProvider(raceCtx, w.ID, owner.ID, p.ID) }()
		close(start)
		approvalErr, disableErr := <-approved, <-disabled
		cancel()
		if approvalErr == nil {
			require.ErrorIs(t, disableErr, service.ErrWorkspaceConflict)
		} else {
			require.ErrorIs(t, approvalErr, service.ErrWorkspaceConflict)
			require.NoError(t, disableErr)
		}
		var unsafe bool
		require.NoError(t, integrationDB.QueryRow(`SELECT COALESCE(sp.require_sso,false) AND ip.status<>'active' FROM workspaces w LEFT JOIN workspace_security_policies sp ON sp.workspace_id=w.id JOIN workspace_identity_providers ip ON ip.workspace_id=w.id AND ip.id=$2 WHERE w.id=$1`, w.ID, p.ID).Scan(&unsafe))
		require.False(t, unsafe, "last approved provider disable and SSO requirement cannot both commit")
	}
}

func TestWorkspaceSecurityApprovedProvidersAndDisableRace(t *testing.T) {
	ctx, repo, owner, w, p, _ := enterpriseIdentityFixture(t)
	link := enterpriseProvisionInput(w.ID, p, "security-owner", owner.Email)
	link.LinkUserID = &owner.ID
	_, err := repo.CompleteIdentityLogin(ctx, link)
	require.NoError(t, err)
	actor := service.WithAuthenticationAssurance(workspaceSecurityActorContext(ctx), service.WorkspaceAssurance{WorkspaceID: w.ID, ProviderID: p.ID, ProviderRevision: p.Revision, AuthenticatedAt: time.Now(), AuthMethod: "oidc"})
	selected := "selected"
	ids := []int64{p.ID}
	enabled := true
	policy, err := repo.PatchSecurityPolicy(actor, w.ID, owner.ID, service.WorkspaceSecurityPolicyPatch{ExpectedRevision: 1, RequireSSO: &enabled, ApprovedIdentityProviderMode: &selected, ApprovedIdentityProviderIDs: &ids})
	require.NoError(t, err)
	require.Equal(t, ids, policy.ApprovedIdentityProviderIDs)
	input := enterpriseProviderInput(p)
	input.ProviderKey = "unapproved-alternative"
	input.Name = "Unapproved alternative"
	input.IsDefault = false
	_, err = repo.CreateProvider(ctx, w.ID, owner.ID, input, "cipher:test", p.Scopes)
	require.NoError(t, err)
	require.ErrorIs(t, repo.DisableProvider(ctx, w.ID, owner.ID, p.ID), service.ErrWorkspaceConflict, "unapproved active provider cannot protect lockout")
	none := []int64{}
	_, err = repo.PatchSecurityPolicy(actor, w.ID, owner.ID, service.WorkspaceSecurityPolicyPatch{ExpectedRevision: policy.Revision, ApprovedIdentityProviderIDs: &none})
	require.ErrorIs(t, err, service.ErrIdentityProviderNotApproved)
	other, err := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB)).CreateOrganization(ctx, owner.ID, "Other policy", "other-policy")
	require.NoError(t, err)
	_, err = integrationDB.Exec(`INSERT INTO workspace_security_policies(workspace_id) VALUES($1)`, other.ID)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`INSERT INTO workspace_security_approved_providers(workspace_id,provider_id) VALUES($1,$2)`, other.ID, p.ID)
	require.Error(t, err, "composite FK blocks cross-tenant provider")
}

func TestWorkspaceSecurityLegacyKeyVisibilityAndMachineRuntime(t *testing.T) {
	ctx, repo, owner, w, _, _ := enterpriseIdentityFixture(t)
	client := testEntClient(t)
	wr := NewWorkspaceRepository(integrationDB)
	ws := service.NewWorkspaceService(wr)
	p, err := ws.CreateProject(ctx, owner.ID, w.ID, service.ProjectInput{Name: "Security keys", Slug: "security-keys"})
	require.NoError(t, err)
	keys := service.NewAPIKeyService(NewAPIKeyRepository(client, integrationDB), NewUserRepository(client, integrationDB), NewGroupRepository(client, integrationDB), NewUserSubscriptionRepository(client), nil, nil, &config.Config{})
	keys.ConfigureWorkspaces(wr)
	keys.SetEnterpriseIdentityService(service.NewEnterpriseIdentityService(repo, service.NewWorkspaceAccessService(wr), NewUserRepository(client, integrationDB), enterpriseTestEncryptor{}))
	organization, err := keys.CreateForProject(ctx, owner.ID, w.ID, p.ID, service.CreateAPIKeyRequest{Name: "Security Organization"})
	require.NoError(t, err)
	personal, err := keys.Create(ctx, owner.ID, service.CreateAPIKeyRequest{Name: "Security Personal"})
	require.NoError(t, err)
	_, err = integrationDB.Exec(`INSERT INTO workspace_security_policies(workspace_id,require_mfa) VALUES($1,true) ON CONFLICT(workspace_id) DO UPDATE SET require_mfa=true`, w.ID)
	require.NoError(t, err)
	unsigned := service.WithSessionAuthentication(ctx, service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: time.Now(), MFAEnrolled: true})
	_, err = keys.GetForUser(unsigned, owner.ID, organization.ID)
	require.ErrorIs(t, err, service.ErrMFARequired)
	for _, sort := range []string{"created_at", "current_concurrency"} {
		rows, page, e := keys.List(unsigned, owner.ID, pagination.PaginationParams{Page: 1, PageSize: 20, SortBy: sort}, service.APIKeyListFilters{})
		require.NoError(t, e)
		require.EqualValues(t, 1, page.Total)
		require.Len(t, rows, 1)
		require.Equal(t, personal.ID, rows[0].ID)
	}
	rows, err := keys.SearchAPIKeys(unsigned, owner.ID, "", 20)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, personal.ID, rows[0].ID)
	_, err = keys.GetForUser(workspaceSecurityActorContext(ctx), owner.ID, organization.ID)
	require.NoError(t, err)
	_, err = keys.GetByKey(ctx, organization.Key)
	require.NoError(t, err, "direct-key runtime stays independent of human MFA")
}
