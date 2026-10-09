//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPolicyRepositoryConcurrentRevisionAndOutboxRollback(t *testing.T) {
	for _, scope := range []domain.PolicyScope{domain.PolicyScopeWorkspace, domain.PolicyScopeProject, domain.PolicyScopeServiceAccount} {
		t.Run(string(scope), func(t *testing.T) {
			isolateWorkspaceTestFixtures(t)
			f := serviceAccountSchemaFixture(t)
			ctx, cancel := context.WithTimeout(service.WithPolicyActor(context.Background(), f.payer), 15*time.Second)
			defer cancel()
			ref := domain.PolicyRef{Scope: scope, ScopeID: f.workspace}
			if scope == domain.PolicyScopeProject {
				ref.ScopeID = f.project
			} else if scope == domain.PolicyScopeServiceAccount {
				ref.ScopeID = f.sa
			}
			repo := NewPolicyRepository(integrationDB)
			start, done := make(chan struct{}), make(chan error, 2)
			for _, model := range []string{"first", "second"} {
				go func(model string) {
					<-start
					_, err := repo.UpdatePolicy(ctx, ref, 0, domain.Policy{AllowedModels: []string{model}})
					done <- err
				}(model)
			}
			close(start)
			successes, conflicts := 0, 0
			for range 2 {
				err := <-done
				if err == nil {
					successes++
				} else {
					require.True(t, errors.Is(err, domain.ErrPolicyRevisionConflict), "unexpected concurrent write error: %v", err)
					conflicts++
				}
			}
			require.Equal(t, 1, successes)
			require.Equal(t, 1, conflicts)
			before, err := repo.GetPolicy(ctx, ref)
			require.NoError(t, err)
			require.EqualValues(t, 1, before.Revision)
			require.Len(t, before.AllowedModels, 1)
			require.Contains(t, []string{"first", "second"}, before.AllowedModels[0])
			audit, events, outbox := policyMutationCounts(t, f.workspace)
			require.Equal(t, 1, audit)
			require.Equal(t, 1, events)
			require.Equal(t, 1, outbox)
			_, err = integrationDB.Exec(`CREATE FUNCTION phase_i_policy_outbox_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF EXISTS(SELECT 1 FROM domain_events WHERE id=NEW.event_id AND event_type='policy.updated') THEN RAISE EXCEPTION 'injected policy outbox failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER phase_i_policy_outbox_failure BEFORE INSERT ON domain_event_outbox FOR EACH ROW EXECUTE FUNCTION phase_i_policy_outbox_failure()`)
			require.NoError(t, err)
			_, err = repo.UpdatePolicy(ctx, ref, 1, domain.Policy{AllowedModels: []string{}})
			require.ErrorContains(t, err, "injected policy outbox failure")
			after, err := repo.GetPolicy(ctx, ref)
			require.NoError(t, err)
			require.Equal(t, before, after, "failed evidence insertion must roll back the policy revision and values")
			audit, events, outbox = policyMutationCounts(t, f.workspace)
			require.Equal(t, 1, audit)
			require.Equal(t, 1, events)
			require.Equal(t, 1, outbox)
			_, err = integrationDB.Exec(`DROP TRIGGER phase_i_policy_outbox_failure ON domain_event_outbox; DROP FUNCTION phase_i_policy_outbox_failure()`)
			require.NoError(t, err)
			updated, err := repo.UpdatePolicy(ctx, ref, 1, domain.Policy{})
			require.NoError(t, err, "same expected revision remains valid after rollback")
			require.EqualValues(t, 2, updated.Revision)
			require.Nil(t, updated.AllowedModels)
			audit, events, outbox = policyMutationCounts(t, f.workspace)
			require.Equal(t, 2, audit)
			require.Equal(t, 2, events)
			require.Equal(t, 2, outbox)
		})
	}
}

func TestPolicyRepositoryUsesActualSessionProofAndPersonalIsolation(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	f := serviceAccountSchemaFixture(t)
	ctx := context.Background()
	_, err := integrationDB.Exec(`UPDATE users SET totp_enabled=true WHERE id=$1`, f.payer)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`INSERT INTO workspace_security_policies(workspace_id,require_mfa,session_max_age_seconds) VALUES($1,true,900)`, f.workspace)
	require.NoError(t, err)
	actor := service.WithSessionAuthentication(service.WithPolicyActor(ctx, f.payer), service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: time.Now(), MFASatisfied: true})
	repo := NewPolicyRepository(integrationDB)
	ref := domain.PolicyRef{Scope: domain.PolicyScopeWorkspace, ScopeID: f.workspace}
	policy, err := repo.UpdatePolicy(actor, ref, 0, domain.Policy{AllowedModels: []string{}})
	require.NoError(t, err)
	require.EqualValues(t, 1, policy.Revision)
	// Enabling organization-only security must not turn a Personal workspace
	// management write into an organization Session admission path.
	var personalID int64
	require.NoError(t, integrationDB.QueryRow(`SELECT id FROM workspaces WHERE type='personal' AND owner_user_id=$1`, f.payer).Scan(&personalID))
	policy, err = repo.UpdatePolicy(service.WithPolicyActor(ctx, f.payer), domain.PolicyRef{Scope: domain.PolicyScopeWorkspace, ScopeID: personalID}, 0, domain.Policy{})
	require.NoError(t, err)
	require.EqualValues(t, 1, policy.Revision)
}
