//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func setWorkspaceAdmissionPolicy(t *testing.T, w int64, invitations string, external, jit bool, providers string) {
	t.Helper()
	_, err := integrationDB.Exec(`INSERT INTO workspace_security_policies(workspace_id,invitation_policy,allow_external_members,workspace_jit_enabled,approved_identity_provider_mode) VALUES($1,$2,$3,$4,$5) ON CONFLICT(workspace_id) DO UPDATE SET invitation_policy=EXCLUDED.invitation_policy,allow_external_members=EXCLUDED.allow_external_members,workspace_jit_enabled=EXCLUDED.workspace_jit_enabled,approved_identity_provider_mode=EXCLUDED.approved_identity_provider_mode,revision=workspace_security_policies.revision+1`, w, invitations, external, jit, providers)
	require.NoError(t, err)
}

func workspaceAdmissionSnapshot(t *testing.T, w int64) string {
	t.Helper()
	var result string
	require.NoError(t, integrationDB.QueryRow(`SELECT jsonb_build_object('members',(SELECT jsonb_agg(to_jsonb(m) ORDER BY id) FROM workspace_members m WHERE workspace_id=$1),'sources',(SELECT jsonb_agg(to_jsonb(s) ORDER BY id) FROM workspace_membership_sources s WHERE workspace_id=$1),'invitations',(SELECT jsonb_agg(to_jsonb(i) ORDER BY id) FROM workspace_invitations i WHERE workspace_id=$1),'scim_users',(SELECT jsonb_agg(to_jsonb(s) ORDER BY id) FROM workspace_scim_users s WHERE workspace_id=$1),'audits',(SELECT count(*) FROM workspace_audit_logs WHERE workspace_id=$1),'events',(SELECT count(*) FROM domain_events WHERE workspace_id=$1),'outbox',(SELECT count(*) FROM domain_event_outbox o JOIN domain_events e ON e.id=o.event_id WHERE e.workspace_id=$1))::text`, w).Scan(&result))
	return result
}

func TestWorkspaceMemberAdmissionTightenedPendingInvitations(t *testing.T) {
	for _, tc := range []struct {
		name, invitations string
		external          bool
		want              error
	}{
		{"disabled", "disabled", true, service.ErrInvitationsDisabled},
		{"verified only", "verified_domains_only", true, service.ErrMemberDomainNotAllowed},
		{"external prohibited", "any", false, service.ErrExternalMemberNotAllowed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _, owner, w, _, _ := enterpriseIdentityFixture(t)
			ws := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
			user := mustCreateUser(t, testEntClient(t), &service.User{Email: fmt.Sprintf("pending-%d@outside.example", w.ID)})
			_, token, err := ws.CreateInvitation(ctx, owner.ID, w.ID, user.Email, "viewer", time.Hour)
			require.NoError(t, err)
			setWorkspaceAdmissionPolicy(t, w.ID, tc.invitations, tc.external, true, "any_active")
			before := workspaceAdmissionSnapshot(t, w.ID)
			_, err = ws.AcceptInvitation(ctx, user.ID, token)
			require.ErrorIs(t, err, tc.want)
			require.Equal(t, before, workspaceAdmissionSnapshot(t, w.ID), "denial preserves token, membership and durable events")
			_, _, err = ws.CreateInvitation(ctx, owner.ID, w.ID, fmt.Sprintf("another-%d@outside.example", w.ID), "viewer", time.Hour)
			require.ErrorIs(t, err, tc.want)
			require.Equal(t, before, workspaceAdmissionSnapshot(t, w.ID))
			setWorkspaceAdmissionPolicy(t, w.ID, "any", true, true, "any_active")
			_, err = ws.AcceptInvitation(ctx, user.ID, token)
			require.NoError(t, err, "the same pending token remains usable after an authorized relaxation")
		})
	}
}

func TestWorkspaceMemberAdmissionInvitationExactVerifiedDomain(t *testing.T) {
	ctx, _, owner, w, _, _ := enterpriseIdentityFixture(t)
	ws := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
	setWorkspaceAdmissionPolicy(t, w.ID, "verified_domains_only", true, true, "any_active")
	_, _, err := ws.CreateInvitation(ctx, owner.ID, w.ID, fmt.Sprintf("suffix@sub.enterprise-%d.example.com", w.ID), "viewer", time.Hour)
	require.ErrorIs(t, err, service.ErrMemberDomainNotAllowed)
	user := mustCreateUser(t, testEntClient(t), &service.User{Email: fmt.Sprintf("internal@enterprise-%d.example.com", w.ID)})
	_, token, err := ws.CreateInvitation(ctx, owner.ID, w.ID, user.Email, "viewer", time.Hour)
	require.NoError(t, err)
	_, err = ws.AcceptInvitation(ctx, user.ID, token)
	require.NoError(t, err)
}

func TestWorkspaceMemberAdmissionInvitationAcceptanceChecksHumanSession(t *testing.T) {
	for _, mode := range []string{"sso", "mfa", "age"} {
		t.Run(mode, func(t *testing.T) {
			ctx, _, owner, w, _, _ := enterpriseIdentityFixture(t)
			ws := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
			user := mustCreateUser(t, testEntClient(t), &service.User{Email: fmt.Sprintf("human-%d@outside.example", w.ID)})
			_, token, err := ws.CreateInvitation(ctx, owner.ID, w.ID, user.Email, "viewer", time.Hour)
			require.NoError(t, err)
			setWorkspaceAdmissionPolicy(t, w.ID, "any", true, true, "any_active")
			var want error
			switch mode {
			case "sso":
				_, err = integrationDB.Exec(`UPDATE workspace_security_policies SET require_sso=true WHERE workspace_id=$1`, w.ID)
				want = service.ErrSSORequired
			case "mfa":
				_, err = integrationDB.Exec(`UPDATE workspace_security_policies SET require_mfa=true WHERE workspace_id=$1`, w.ID)
				require.NoError(t, err)
				_, err = integrationDB.Exec(`UPDATE users SET totp_enabled=true,totp_secret_encrypted='encrypted-test-factor' WHERE id=$1`, user.ID)
				want = service.ErrMFARequired
			case "age":
				_, err = integrationDB.Exec(`UPDATE workspace_security_policies SET session_max_age_seconds=900 WHERE workspace_id=$1`, w.ID)
				want = service.ErrWorkspaceReauthRequired
			}
			require.NoError(t, err)
			acceptCtx := service.WithSessionAuthentication(ctx, service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: time.Now().Add(-time.Hour)})
			before := workspaceAdmissionSnapshot(t, w.ID)
			_, err = ws.AcceptInvitation(acceptCtx, user.ID, token)
			require.ErrorIs(t, err, want)
			require.Equal(t, before, workspaceAdmissionSnapshot(t, w.ID))
		})
	}
}

func TestWorkspaceMemberAdmissionJITReactivationUsesCurrentPolicy(t *testing.T) {
	for _, protocol := range []string{"oidc", "saml"} {
		for _, mode := range []string{"workspace_off", "provider_off", "unapproved", "domain_revoked"} {
			t.Run(protocol+"/"+mode, func(t *testing.T) {
				ctx, identity, owner, w, provider, _ := enterpriseIdentityFixture(t)
				if protocol == "saml" {
					var err error
					provider, err = identity.CreateProvider(ctx, w.ID, owner.ID, samlRepositoryInput("admission-saml"), "cipher:keys", nil)
					require.NoError(t, err)
				}
				input := enterpriseProvisionInput(w.ID, provider, "reactivation-subject", fmt.Sprintf("jit-reactivate@enterprise-%d.example.com", w.ID))
				input.Protocol = protocol
				uid, err := identity.CompleteIdentityLogin(ctx, input)
				require.NoError(t, err)
				_, err = integrationDB.Exec(`UPDATE workspace_membership_sources SET active=false WHERE workspace_id=$1 AND member_id=(SELECT id FROM workspace_members WHERE workspace_id=$1 AND user_id=$2)`, w.ID, uid)
				require.NoError(t, err)
				_, err = integrationDB.Exec(`UPDATE workspace_members SET status='suspended' WHERE workspace_id=$1 AND user_id=$2`, w.ID, uid)
				require.NoError(t, err)
				var want error
				switch mode {
				case "workspace_off":
					setWorkspaceAdmissionPolicy(t, w.ID, "any", true, false, "any_active")
					want = service.ErrWorkspaceJITDisabled
				case "provider_off":
					require.NoError(t, integrationDB.QueryRow(`UPDATE workspace_identity_providers SET jit_config=jsonb_set(jit_config,'{enabled}','false'),revision=revision+1 WHERE workspace_id=$1 AND id=$2 RETURNING revision`, w.ID, provider.ID).Scan(&input.ProviderRevision))
					want = service.ErrOIDCAccountLinkRequired
				case "unapproved":
					setWorkspaceAdmissionPolicy(t, w.ID, "any", true, true, "selected")
					want = service.ErrIdentityProviderNotApproved
				case "domain_revoked":
					setWorkspaceAdmissionPolicy(t, w.ID, "any", false, true, "any_active")
					_, err = integrationDB.Exec(`UPDATE workspace_domains SET status='revoked',verified_at=NULL WHERE workspace_id=$1`, w.ID)
					require.NoError(t, err)
					want = service.ErrExternalMemberNotAllowed
				}
				before := workspaceAdmissionSnapshot(t, w.ID)
				_, err = identity.CompleteIdentityLogin(ctx, input)
				require.ErrorIs(t, err, want)
				require.Equal(t, before, workspaceAdmissionSnapshot(t, w.ID))
			})
		}
	}
}

func TestWorkspaceMemberAdmissionNewJITDoesNotCreateGlobalUser(t *testing.T) {
	for _, protocol := range []string{"oidc", "saml"} {
		t.Run(protocol, func(t *testing.T) {
			ctx, identity, owner, w, provider, _ := enterpriseIdentityFixture(t)
			if protocol == "saml" {
				var err error
				provider, err = identity.CreateProvider(ctx, w.ID, owner.ID, samlRepositoryInput("new-admission-saml"), "cipher:keys", nil)
				require.NoError(t, err)
			}
			setWorkspaceAdmissionPolicy(t, w.ID, "any", true, false, "any_active")
			email := fmt.Sprintf("jit-new-denied@enterprise-%d.example.com", w.ID)
			input := enterpriseProvisionInput(w.ID, provider, "new-denied-subject", email)
			input.Protocol = protocol
			before := workspaceAdmissionSnapshot(t, w.ID)
			_, err := identity.CompleteIdentityLogin(ctx, input)
			require.ErrorIs(t, err, service.ErrWorkspaceJITDisabled)
			require.Equal(t, before, workspaceAdmissionSnapshot(t, w.ID))
			var count int
			require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM users WHERE email=$1`, email).Scan(&count))
			require.Zero(t, count)
		})
	}
}

func TestWorkspaceMemberAdmissionRetainsActiveJITSources(t *testing.T) {
	ctx, identity, _, w, provider, _ := enterpriseIdentityFixture(t)
	input := enterpriseProvisionInput(w.ID, provider, "retained-subject", fmt.Sprintf("jit-retained@enterprise-%d.example.com", w.ID))
	uid, err := identity.CompleteIdentityLogin(ctx, input)
	require.NoError(t, err)
	setWorkspaceAdmissionPolicy(t, w.ID, "disabled", false, false, "selected")
	_, err = integrationDB.Exec(`UPDATE workspace_domains SET status='revoked',verified_at=NULL WHERE workspace_id=$1`, w.ID)
	require.NoError(t, err)
	input.Claims.EmailVerified = false
	again, err := identity.CompleteIdentityLogin(ctx, input)
	require.NoError(t, err)
	require.Equal(t, uid, again)
	var status string
	require.NoError(t, integrationDB.QueryRow(`SELECT status FROM workspace_members WHERE workspace_id=$1 AND user_id=$2`, w.ID, uid).Scan(&status))
	require.Equal(t, "active", status)
}

func TestWorkspaceMemberAdmissionAdministrativeRestoration(t *testing.T) {
	ctx, _, owner, w, _, _ := enterpriseIdentityFixture(t)
	ws := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
	user := workspaceJoin(t, ctx, ws, owner.ID, w.ID, "viewer")
	setWorkspaceAdmissionPolicy(t, w.ID, "any", false, true, "any_active")
	require.NoError(t, ws.UpdateMember(ctx, owner.ID, w.ID, user.ID, "developer", "active"), "existing active external member is retained")
	require.NoError(t, ws.RemoveMember(ctx, owner.ID, w.ID, user.ID))
	before := workspaceAdmissionSnapshot(t, w.ID)
	require.ErrorIs(t, ws.UpdateMember(ctx, owner.ID, w.ID, user.ID, "viewer", "active"), service.ErrExternalMemberNotAllowed)
	require.Equal(t, before, workspaceAdmissionSnapshot(t, w.ID), "denial retains administrative tombstone and provenance")
}

func TestWorkspaceMemberAdmissionSCIMRetainedAndReactivation(t *testing.T) {
	ctx, scim, _, w, _, principal := scimFixture(t)
	in := scimUserInput(w.ID, "scim-admission")
	created, err := scim.MutateUser(ctx, principal, "", service.SCIMUserMutation{Action: "create", User: in})
	require.NoError(t, err)
	setWorkspaceAdmissionPolicy(t, w.ID, "disabled", false, false, "selected")
	_, err = integrationDB.Exec(`UPDATE workspace_domains SET status='revoked',verified_at=NULL WHERE workspace_id=$1`, w.ID)
	require.NoError(t, err)
	in.DisplayName = "Retained metadata"
	_, err = scim.MutateUser(ctx, principal, created.ID, service.SCIMUserMutation{Action: "replace", User: in})
	require.NoError(t, err)
	inactive := false
	in.Active = &inactive
	_, err = scim.MutateUser(ctx, principal, created.ID, service.SCIMUserMutation{Action: "replace", User: in})
	require.NoError(t, err)
	before := workspaceAdmissionSnapshot(t, w.ID)
	active := true
	in.Active = &active
	_, err = scim.MutateUser(ctx, principal, created.ID, service.SCIMUserMutation{Action: "replace", User: in})
	assertSCIMStatus(t, err, 409)
	require.Equal(t, before, workspaceAdmissionSnapshot(t, w.ID))
}

func TestWorkspaceMemberAdmissionNewSCIMResourceAlwaysChecksCurrentPolicy(t *testing.T) {
	ctx, scim, owner, w, _, principal := scimFixture(t)
	in := scimUserInput(w.ID, "second-resource")
	created, err := scim.MutateUser(ctx, principal, "", service.SCIMUserMutation{Action: "create", User: in})
	require.NoError(t, err)
	connector, err := scim.CreateConnector(ctx, w.ID, owner.ID, service.SCIMConnectorInput{Name: "Other provisioning", DefaultRole: "viewer"})
	require.NoError(t, err)
	token, err := service.NewEnterpriseSCIMService(scim).CreateToken(ctx, w.ID, owner.ID, connector.ID, service.SCIMTokenInput{})
	require.NoError(t, err)
	other, err := service.NewEnterpriseSCIMService(scim).Authenticate(ctx, connector.PublicID, token.Secret)
	require.NoError(t, err)
	setWorkspaceAdmissionPolicy(t, w.ID, "any", false, true, "any_active")
	_, err = integrationDB.Exec(`UPDATE users SET email=$2 WHERE id=$1`, created.UserID, fmt.Sprintf("changed-%d@outside.example", w.ID))
	require.NoError(t, err)
	in.Emails[0].Value = fmt.Sprintf("changed-%d@outside.example", w.ID)
	before := workspaceAdmissionSnapshot(t, w.ID)
	_, err = scim.MutateUser(ctx, other, "", service.SCIMUserMutation{Action: "create", User: in})
	var policyErr *service.SCIMError
	require.ErrorAs(t, err, &policyErr)
	require.Equal(t, 409, policyErr.Status)
	require.Equal(t, "invalidValue", policyErr.Type, "policy rejection occurs before identity resolution/adoption")
	require.Equal(t, before, workspaceAdmissionSnapshot(t, w.ID))
}

func TestWorkspaceMemberAdmissionSCIMBearerHasNoHumanSessionRequirements(t *testing.T) {
	ctx, scim, _, w, _, principal := scimFixture(t)
	setWorkspaceAdmissionPolicy(t, w.ID, "disabled", false, false, "selected")
	_, err := integrationDB.Exec(`UPDATE workspace_security_policies SET require_sso=true,require_mfa=true,session_max_age_seconds=900 WHERE workspace_id=$1`, w.ID)
	require.NoError(t, err)
	_, err = scim.MutateUser(ctx, principal, "", service.SCIMUserMutation{Action: "create", User: scimUserInput(w.ID, "machine-proof")})
	require.NoError(t, err)
}

func TestWorkspaceMemberAdmissionSCIMAdministrativeRemovalIsIndependent(t *testing.T) {
	ctx, scim, _, w, _, principal := scimFixture(t)
	in := scimUserInput(w.ID, "removed-only")
	created, err := scim.MutateUser(ctx, principal, "", service.SCIMUserMutation{Action: "create", User: in})
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE workspace_members SET status='suspended',administratively_removed=true,administratively_suspended=false,effective_membership_source_id=NULL WHERE workspace_id=$1 AND id=$2`, w.ID, created.MemberID)
	require.NoError(t, err)
	in.DisplayName = "Source metadata while removed"
	_, err = scim.MutateUser(ctx, principal, created.ID, service.SCIMUserMutation{Action: "replace", User: in})
	require.NoError(t, err)
	var status string
	var removed bool
	require.NoError(t, integrationDB.QueryRow(`SELECT status,administratively_removed FROM workspace_members WHERE workspace_id=$1 AND id=$2`, w.ID, created.MemberID).Scan(&status, &removed))
	require.Equal(t, "suspended", status)
	require.True(t, removed)
}

func TestWorkspaceMemberAdmissionDomainRevocationSerializesInvitationAcceptance(t *testing.T) {
	ctx, _, owner, w, _, _ := enterpriseIdentityFixture(t)
	ws := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
	setWorkspaceAdmissionPolicy(t, w.ID, "verified_domains_only", false, true, "any_active")
	user := mustCreateUser(t, testEntClient(t), &service.User{Email: fmt.Sprintf("serial@enterprise-%d.example.com", w.ID)})
	_, token, err := ws.CreateInvitation(ctx, owner.ID, w.ID, user.Email, "viewer", time.Hour)
	require.NoError(t, err)
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()
	require.NoError(t, lockWorkspace(ctx, tx, w.ID, true))
	var writerPID int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&writerPID))
	_, err = tx.ExecContext(ctx, `UPDATE workspace_domains SET status='revoked',verified_at=NULL WHERE workspace_id=$1`, w.ID)
	require.NoError(t, err)
	started, done := make(chan struct{}), make(chan error, 1)
	go func() { close(started); _, e := ws.AcceptInvitation(ctx, user.ID, token); done <- e; close(done) }()
	defer finishAdmissionWriter(t, tx, done)
	<-started
	waitForAdmissionWorkspaceLock(t, writerPID)
	require.NoError(t, tx.Commit())
	require.ErrorIs(t, <-done, service.ErrExternalMemberNotAllowed)
	var accepted sql.NullTime
	require.NoError(t, integrationDB.QueryRow(`SELECT accepted_at FROM workspace_invitations WHERE workspace_id=$1 AND token_hash=$2`, w.ID, service.HashEnterpriseToken(token)).Scan(&accepted))
	require.False(t, accepted.Valid)
}

func waitForAdmissionWorkspaceLock(t *testing.T, writerPID int) {
	t.Helper()
	require.Eventually(t, func() bool {
		var waiting bool
		err := integrationDB.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT id FROM workspaces WHERE id=$1 FOR %' AND $1=ANY(pg_blocking_pids(pid)))`, writerPID).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 10*time.Millisecond, "admission must be waiting for the Workspace lock held by the policy/domain writer")
}

func finishAdmissionWriter(t *testing.T, tx *sql.Tx, done <-chan error) {
	t.Helper()
	// Release the writer and finish any admission even when the lock probe fails.
	// This keeps fixture cleanup from racing a newly unblocked mutation.
	_ = tx.Rollback()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Error("admission did not finish after the Workspace writer lock was released")
	}
}

func TestWorkspaceMemberAdmissionPolicyTighteningSerializesEveryEntry(t *testing.T) {
	for _, entry := range []string{"invitation_create", "invitation_accept", "oidc", "saml", "scim_create", "scim_reactivate", "admin_restore"} {
		t.Run(entry, func(t *testing.T) {
			ctx, identity, owner, w, provider, _ := enterpriseIdentityFixture(t)
			ws := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
			var admit func() error
			var want error
			var update string
			var deniedEmail string
			switch entry {
			case "invitation_create":
				email := fmt.Sprintf("race-invitation-%d@outside.example", w.ID)
				admit = func() error {
					_, _, err := ws.CreateInvitation(ctx, owner.ID, w.ID, email, "viewer", time.Hour)
					return err
				}
				update = `UPDATE workspace_security_policies SET invitation_policy='disabled',revision=revision+1 WHERE workspace_id=$1`
				want = service.ErrInvitationsDisabled
			case "invitation_accept":
				user := mustCreateUser(t, testEntClient(t), &service.User{Email: fmt.Sprintf("race-invitation-%d@outside.example", w.ID)})
				_, token, err := ws.CreateInvitation(ctx, owner.ID, w.ID, user.Email, "viewer", time.Hour)
				require.NoError(t, err)
				admit = func() error { _, err := ws.AcceptInvitation(ctx, user.ID, token); return err }
				update = `UPDATE workspace_security_policies SET invitation_policy='disabled',revision=revision+1 WHERE workspace_id=$1`
				want = service.ErrInvitationsDisabled
			case "oidc", "saml":
				if entry == "saml" {
					var err error
					provider, err = identity.CreateProvider(ctx, w.ID, owner.ID, samlRepositoryInput("race-saml"), "cipher:keys", nil)
					require.NoError(t, err)
				}
				input := enterpriseProvisionInput(w.ID, provider, "race-new-subject", fmt.Sprintf("race-jit@enterprise-%d.example.com", w.ID))
				input.Protocol = entry
				deniedEmail = input.Claims.Email
				admit = func() error { _, err := identity.CompleteIdentityLogin(ctx, input); return err }
				update = `UPDATE workspace_security_policies SET workspace_jit_enabled=false,revision=revision+1 WHERE workspace_id=$1`
				want = service.ErrWorkspaceJITDisabled
			case "scim_create", "scim_reactivate":
				scim := &enterpriseSCIMRepository{db: integrationDB}
				serviceSCIM := service.NewEnterpriseSCIMService(scim)
				connector, err := serviceSCIM.CreateConnector(ctx, w.ID, owner.ID, service.SCIMConnectorInput{Name: "Race provisioning", DefaultRole: "viewer"})
				require.NoError(t, err)
				token, err := serviceSCIM.CreateToken(ctx, w.ID, owner.ID, connector.ID, service.SCIMTokenInput{})
				require.NoError(t, err)
				principal, err := serviceSCIM.Authenticate(ctx, connector.PublicID, token.Secret)
				require.NoError(t, err)
				input := scimUserInput(w.ID, "race-scim")
				update = `UPDATE workspace_security_policies SET allow_external_members=false,revision=revision+1 WHERE workspace_id=$1`
				if entry == "scim_create" {
					deniedEmail = fmt.Sprintf("race-scim-%d@outside.example", w.ID)
					input.Emails[0].Value = deniedEmail
					admit = func() error {
						_, err := scim.MutateUser(ctx, principal, "", service.SCIMUserMutation{Action: "create", User: input})
						return err
					}
					break
				}
				created, err := scim.MutateUser(ctx, principal, "", service.SCIMUserMutation{Action: "create", User: input})
				require.NoError(t, err)
				inactive := false
				input.Active = &inactive
				_, err = scim.MutateUser(ctx, principal, created.ID, service.SCIMUserMutation{Action: "replace", User: input})
				require.NoError(t, err)
				_, err = integrationDB.Exec(`UPDATE users SET email=$2 WHERE id=$1`, created.UserID, fmt.Sprintf("race-scim-%d@outside.example", w.ID))
				require.NoError(t, err)
				active := true
				input.Active = &active
				admit = func() error {
					_, err := scim.MutateUser(ctx, principal, created.ID, service.SCIMUserMutation{Action: "replace", User: input})
					return err
				}
			case "admin_restore":
				user := workspaceJoin(t, ctx, ws, owner.ID, w.ID, "viewer")
				require.NoError(t, ws.RemoveMember(ctx, owner.ID, w.ID, user.ID))
				admit = func() error { return ws.UpdateMember(ctx, owner.ID, w.ID, user.ID, "viewer", "active") }
				update = `UPDATE workspace_security_policies SET allow_external_members=false,revision=revision+1 WHERE workspace_id=$1`
				want = service.ErrExternalMemberNotAllowed
			}
			// New organizations receive default behavior even before their first
			// saved policy; create the row so this transaction can tighten it.
			_, err := integrationDB.Exec(`INSERT INTO workspace_security_policies(workspace_id) VALUES($1) ON CONFLICT DO NOTHING`, w.ID)
			require.NoError(t, err)
			before := workspaceAdmissionSnapshot(t, w.ID)
			tx, err := integrationDB.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer tx.Rollback()
			require.NoError(t, lockWorkspace(ctx, tx, w.ID, true))
			var writerPID int
			require.NoError(t, tx.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&writerPID))
			_, err = tx.ExecContext(ctx, update, w.ID)
			require.NoError(t, err)
			done := make(chan error, 1)
			go func() { done <- admit(); close(done) }()
			defer finishAdmissionWriter(t, tx, done)
			waitForAdmissionWorkspaceLock(t, writerPID)
			require.NoError(t, tx.Commit())
			err = <-done
			if entry == "scim_create" || entry == "scim_reactivate" {
				assertSCIMStatus(t, err, 409)
			} else {
				require.ErrorIs(t, err, want)
			}
			require.Equal(t, before, workspaceAdmissionSnapshot(t, w.ID), "blocked admission consumes no token and writes no partial state/events")
			if deniedEmail != "" {
				var count int
				require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM users WHERE email=$1`, deniedEmail).Scan(&count))
				require.Zero(t, count, "denied admission cannot leave a Global User or its personal workspace")
			}
		})
	}
}

func TestWorkspaceMemberAdmissionHelperRejectsCrossTenantMember(t *testing.T) {
	ctx, _, _, w, _, _ := enterpriseIdentityFixture(t)
	_, ws, owner, other := workspaceFixture(t)
	user := workspaceJoin(t, ctx, ws, owner.ID, other.ID, "viewer")
	var member int64
	require.NoError(t, integrationDB.QueryRow(`SELECT id FROM workspace_members WHERE workspace_id=$1 AND user_id=$2`, other.ID, user.ID).Scan(&member))
	tx, err := integrationDB.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	defer tx.Rollback()
	require.NoError(t, lockWorkspace(ctx, tx, w.ID, true))
	require.ErrorIs(t, checkWorkspaceMemberAdmissionTx(ctx, tx, w.ID, service.WorkspaceMemberAdmissionRequest{Source: service.AdmissionAdminRestore, MemberID: member}), service.ErrWorkspaceNotFound)
}
