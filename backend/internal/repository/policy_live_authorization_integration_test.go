//go:build integration

package repository

import (
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func policyMutationCounts(t *testing.T, workspaceID int64) (audit, events, outbox int) {
	t.Helper()
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_audit_logs WHERE workspace_id=$1 AND action IN ('workspace_policy.updated','project_policy.updated','service_account_policy.updated')`, workspaceID).Scan(&audit))
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_events WHERE workspace_id=$1 AND event_type='policy.updated'`, workspaceID).Scan(&events))
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_event_outbox o JOIN domain_events e ON e.id=o.event_id WHERE e.workspace_id=$1 AND e.event_type='policy.updated'`, workspaceID).Scan(&outbox))
	return
}

func TestPolicyRepositoryRechecksLiveTenantAndHumanSecurity(t *testing.T) {
	for _, scope := range []domain.PolicyScope{domain.PolicyScopeWorkspace, domain.PolicyScopeProject, domain.PolicyScopeServiceAccount} {
		for _, change := range []struct {
			name string
			want error
		}{
			{"foreign_actor", service.ErrWorkspaceNotFound},
			{"global_admin_without_membership", service.ErrWorkspaceNotFound},
			{"user_disabled", service.ErrWorkspaceNotFound},
			{"user_deleted", service.ErrWorkspaceNotFound},
			{"member_suspended", service.ErrWorkspaceForbidden},
			{"member_removed", service.ErrWorkspaceNotFound},
			{"member_demoted", service.ErrWorkspaceForbidden},
			{"workspace_archived", service.ErrWorkspaceConflict},
			{"workspace_suspended", service.ErrWorkspaceConflict},
			{"human_mfa_required", service.ErrMFARequired},
			{"human_authentication_expired", service.ErrWorkspaceReauthRequired},
			{"human_sso_required", service.ErrSSORequired},
		} {
			t.Run(string(scope)+"/"+change.name, func(t *testing.T) {
				isolateWorkspaceTestFixtures(t)
				f := serviceAccountSchemaFixture(t)
				ctx := context.Background()
				ws := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
				actor := workspaceJoin(t, ctx, ws, f.payer, f.workspace, "admin")
				ref := domain.PolicyRef{Scope: scope, ScopeID: f.workspace}
				if scope == domain.PolicyScopeProject {
					ref.ScopeID = f.project
				} else if scope == domain.PolicyScopeServiceAccount {
					ref.ScopeID = f.sa
				}
				repo := NewPolicyRepository(integrationDB)
				created, err := repo.UpdatePolicy(service.WithPolicyActor(ctx, f.payer), ref, 0, domain.Policy{AllowedModels: []string{"before"}})
				require.NoError(t, err)
				require.EqualValues(t, 1, created.Revision)
				// The caller was authorized before any of these live changes.
				require.NoError(t, ws.RequirePolicyWorkspace(ctx, actor.ID, f.workspace, "workspace_policy.update"))
				switch change.name {
				case "foreign_actor", "global_admin_without_membership":
					role := "user"
					if change.name == "global_admin_without_membership" {
						role = "admin"
					}
					actor = mustCreateUser(t, testEntClient(t), &service.User{Role: role})
				case "user_disabled":
					_, err = integrationDB.Exec(`UPDATE users SET status='disabled' WHERE id=$1`, actor.ID)
				case "user_deleted":
					_, err = integrationDB.Exec(`UPDATE users SET deleted_at=now() WHERE id=$1`, actor.ID)
				case "member_suspended":
					err = ws.UpdateMember(ctx, f.payer, f.workspace, actor.ID, "admin", "suspended")
				case "member_removed":
					err = ws.RemoveMember(ctx, f.payer, f.workspace, actor.ID)
				case "member_demoted":
					err = ws.UpdateMember(ctx, f.payer, f.workspace, actor.ID, "viewer", "active")
				case "workspace_archived":
					err = ws.ArchiveWorkspace(ctx, f.payer, f.workspace)
				case "workspace_suspended":
					_, err = integrationDB.Exec(`UPDATE workspaces SET status='suspended' WHERE id=$1`, f.workspace)
				case "human_mfa_required":
					_, err = integrationDB.Exec(`UPDATE users SET totp_enabled=true WHERE id=$1`, actor.ID)
					require.NoError(t, err)
					_, err = integrationDB.Exec(`INSERT INTO workspace_security_policies(workspace_id,require_mfa) VALUES($1,true) ON CONFLICT(workspace_id) DO UPDATE SET require_mfa=true`, f.workspace)
					ctx = service.WithSessionAuthentication(ctx, service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: time.Now(), MFAEnrolled: true})
				case "human_authentication_expired":
					_, err = integrationDB.Exec(`INSERT INTO workspace_security_policies(workspace_id,session_max_age_seconds) VALUES($1,900) ON CONFLICT(workspace_id) DO UPDATE SET session_max_age_seconds=900`, f.workspace)
					ctx = service.WithSessionAuthentication(ctx, service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: time.Now().Add(-time.Hour), MFASatisfied: true})
				case "human_sso_required":
					_, err = integrationDB.Exec(`INSERT INTO workspace_security_policies(workspace_id,require_sso) VALUES($1,true) ON CONFLICT(workspace_id) DO UPDATE SET require_sso=true`, f.workspace)
				}
				require.NoError(t, err)
				beforeAudit, beforeEvents, beforeOutbox := policyMutationCounts(t, f.workspace)
				_, err = repo.UpdatePolicy(service.WithPolicyActor(ctx, actor.ID), ref, 1, domain.Policy{AllowedModels: []string{}})
				require.ErrorIs(t, err, change.want)
				current, err := repo.GetPolicy(ctx, ref)
				require.NoError(t, err)
				require.EqualValues(t, 1, current.Revision)
				require.Equal(t, []string{"before"}, current.AllowedModels)
				audit, events, outbox := policyMutationCounts(t, f.workspace)
				require.Equal(t, beforeAudit, audit)
				require.Equal(t, beforeEvents, events)
				require.Equal(t, beforeOutbox, outbox)
			})
		}
	}
}

type pausedPolicyBody struct {
	io.Reader
	started, resume chan struct{}
	once            sync.Once
}

func (b *pausedPolicyBody) Read(p []byte) (int, error) {
	b.once.Do(func() { close(b.started); <-b.resume })
	return b.Reader.Read(p)
}

func TestPolicyHTTPRechecksGrantRevokedAfterAuthorization(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	gin.SetMode(gin.TestMode)
	f := serviceAccountSchemaFixture(t)
	ctx := context.Background()
	svc, _, ws := serviceAccountTestService(t)
	actor := workspaceJoin(t, ctx, ws, f.payer, f.workspace, "developer")
	_, err := ws.SetProjectAccessMode(ctx, f.payer, f.workspace, service.ProjectAccessModeAssigned)
	require.NoError(t, err)
	grant, err := ws.CreateProjectAccessGrant(ctx, f.payer, f.workspace, f.project, service.ProjectAccessGrantInput{SubjectType: "member", SubjectID: otherMemberWorkspaceMemberID(t, ctx, f.workspace, actor.ID), Role: "developer"})
	require.NoError(t, err)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: actor.ID})
	})
	repo := NewPolicyRepository(integrationDB)
	handler.NewPolicyHandler(ws, svc, repo, nil).RegisterTenantRoutes(router.Group("/api/v1"))
	body := &pausedPolicyBody{Reader: strings.NewReader(`{"expected_revision":0,"allowed_models":[]}`), started: make(chan struct{}), resume: make(chan struct{})}
	var resume sync.Once
	request := httptest.NewRequest("PATCH", fmt.Sprintf("/api/v1/workspaces/%d/projects/%d/service-accounts/%d/policy", f.workspace, f.project, f.sa), body)
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	t.Cleanup(func() {
		resume.Do(func() { close(body.resume) })
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("policy handler did not finish after releasing body gate")
		}
	})
	go func() { defer close(done); router.ServeHTTP(rec, request) }()
	select {
	case <-body.started:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not pass authorization and reach body decoding")
	}
	require.NoError(t, ws.DeleteProjectAccessGrant(ctx, f.payer, f.workspace, f.project, grant.ID))
	resume.Do(func() { close(body.resume) })
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("delayed request did not finish")
	}
	require.Equal(t, 403, rec.Code, rec.Body.String())
	policy, err := repo.GetPolicy(ctx, domain.PolicyRef{Scope: domain.PolicyScopeServiceAccount, ScopeID: f.sa})
	require.NoError(t, err)
	require.Nil(t, policy)
	audit, events, outbox := policyMutationCounts(t, f.workspace)
	require.Zero(t, audit)
	require.Zero(t, events)
	require.Zero(t, outbox)
}

func TestPolicyRepositoryConcurrentRevokeAndUserDisable(t *testing.T) {
	for _, change := range []string{"project_grant", "global_user", "project_archive", "security_tightening"} {
		t.Run(change, func(t *testing.T) {
			isolateWorkspaceTestFixtures(t)
			f := serviceAccountSchemaFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
			defer cancel()
			ws := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
			actor := workspaceJoin(t, ctx, ws, f.payer, f.workspace, "developer")
			_, err := ws.SetProjectAccessMode(ctx, f.payer, f.workspace, service.ProjectAccessModeAssigned)
			require.NoError(t, err)
			grant, err := ws.CreateProjectAccessGrant(ctx, f.payer, f.workspace, f.project, service.ProjectAccessGrantInput{SubjectType: "member", SubjectID: otherMemberWorkspaceMemberID(t, ctx, f.workspace, actor.ID), Role: "developer"})
			require.NoError(t, err)
			if change == "security_tightening" {
				_, err = integrationDB.ExecContext(ctx, `UPDATE users SET totp_enabled=true WHERE id=$1`, actor.ID)
				require.NoError(t, err)
			}
			tx, err := integrationDB.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer func() { _ = tx.Rollback() }()
			want := service.ErrWorkspaceForbidden
			if change != "global_user" {
				require.NoError(t, lockWorkspace(ctx, tx, f.workspace, true))
				switch change {
				case "project_grant":
					_, err = tx.ExecContext(ctx, `DELETE FROM project_access_grants WHERE id=$1`, grant.ID)
				case "project_archive":
					_, err = tx.ExecContext(ctx, `UPDATE projects SET status='archived' WHERE id=$1`, f.project)
					want = service.ErrWorkspaceConflict
				case "security_tightening":
					_, err = tx.ExecContext(ctx, `INSERT INTO workspace_security_policies(workspace_id,require_mfa) VALUES($1,true) ON CONFLICT(workspace_id) DO UPDATE SET require_mfa=true`, f.workspace)
					want = service.ErrMFARequired
				}
			} else {
				// FOR UPDATE also blocks audit foreign-key reads in the unfixed writer.
				var id int64
				require.NoError(t, tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, actor.ID).Scan(&id))
				_, err = tx.ExecContext(ctx, `UPDATE users SET status='disabled' WHERE id=$1`, actor.ID)
				want = service.ErrWorkspaceNotFound
			}
			require.NoError(t, err)
			done := make(chan error, 1)
			finished := make(chan struct{})
			ref := domain.PolicyRef{Scope: domain.PolicyScopeServiceAccount, ScopeID: f.sa}
			repo := NewPolicyRepository(integrationDB)
			go func() {
				defer close(finished)
				_, e := repo.UpdatePolicy(service.WithPolicyActor(ctx, actor.ID), ref, 0, domain.Policy{AllowedModels: []string{}})
				done <- e
			}()
			defer func() {
				_ = tx.Rollback()
				cancel()
				select {
				case <-finished:
				case <-time.After(10 * time.Second):
					t.Error("policy writer did not finish after releasing revocation gate")
				}
			}()
			require.Eventually(t, func() bool {
				var waiting bool
				e := integrationDB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND state='active')`).Scan(&waiting)
				return e == nil && waiting
			}, 4*time.Second, 10*time.Millisecond, "policy writer must be in flight behind the revocation transaction")
			require.NoError(t, tx.Commit())
			select {
			case e := <-done:
				require.ErrorIs(t, e, want)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			policy, err := repo.GetPolicy(ctx, ref)
			require.NoError(t, err)
			require.Nil(t, policy)
			audit, events, outbox := policyMutationCounts(t, f.workspace)
			require.Zero(t, audit)
			require.Zero(t, events)
			require.Zero(t, outbox)
		})
	}
}
