//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestTOTPDisableAndWorkspaceRequirementRace(t *testing.T) {
	ctx, repo, owner, workspace, _, _ := enterpriseIdentityFixture(t)
	actor := workspaceSecurityActorContext(ctx)
	factors := NewUserFactorDependencyRepository(integrationDB)
	for attempt := 0; attempt < 8; attempt++ {
		_, err := integrationDB.Exec(`UPDATE users SET totp_enabled=true,totp_secret_encrypted='fixture-factor',password_hash='fixture-password-hash' WHERE id=$1`, owner.ID)
		require.NoError(t, err)
		_, err = integrationDB.Exec(`INSERT INTO workspace_security_policies(workspace_id,require_mfa) VALUES($1,false) ON CONFLICT(workspace_id) DO UPDATE SET require_mfa=false`, workspace.ID)
		require.NoError(t, err)
		policy, err := repo.GetPolicy(ctx, workspace.ID, owner.ID)
		require.NoError(t, err)
		raceCtx, cancel := context.WithTimeout(actor, 10*time.Second)
		start := make(chan struct{})
		enableResult := make(chan error, 1)
		disableResult := make(chan error, 1)
		go func() {
			<-start
			enabled := true
			_, err := repo.PatchSecurityPolicy(raceCtx, workspace.ID, owner.ID, service.WorkspaceSecurityPolicyPatch{ExpectedRevision: policy.Revision, RequireMFA: &enabled})
			enableResult <- err
		}()
		go func() { <-start; disableResult <- factors.DisableTOTP(raceCtx, owner.ID, "fixture-password-hash") }()
		close(start)
		enableErr, disableErr := <-enableResult, <-disableResult
		cancel()
		if enableErr == nil {
			require.ErrorIs(t, disableErr, service.ErrWorkspaceMFARequired)
		} else {
			require.ErrorIs(t, enableErr, service.ErrMFARequired)
			require.NoError(t, disableErr)
		}
		var required, enrolled bool
		require.NoError(t, integrationDB.QueryRow(`SELECT p.require_mfa,u.totp_enabled FROM workspace_security_policies p JOIN users u ON u.id=$2 WHERE p.workspace_id=$1`, workspace.ID, owner.ID).Scan(&required, &enrolled))
		require.False(t, required && !enrolled, "requirement and factor removal cannot both commit")
	}
}
