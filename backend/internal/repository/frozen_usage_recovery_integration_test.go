//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type frozenUsageRecoverer interface {
	RecoverFrozenTenantUsage(context.Context, int) (int, error)
}

func TestFrozenTenantUsageSQLRecoveryAfterFailureAndRestart(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, repo, a, res, cmd, log := tenantUsageFixture(t)
	recovery, ok := NewUsageBillingRepository(testEntClient(t), integrationDB).(frozenUsageRecoverer)
	require.True(t, ok, "SQL must retain and replay frozen synchronous tenant accounting")
	_, err := integrationDB.Exec(`CREATE FUNCTION fail_frozen_usage_test() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected usage write failure'; END $$;
	CREATE TRIGGER fail_frozen_usage_test BEFORE INSERT ON usage_logs FOR EACH ROW EXECUTE FUNCTION fail_frozen_usage_test()`)
	require.NoError(t, err)
	_, err = repo.ApplyTenantUsage(ctx, cmd, log)
	require.Error(t, err)
	assertTenantUsageTotals(t, a, res, cmd, 100, 0, 2, 0, 0)
	var state string
	require.NoError(t, integrationDB.QueryRow(`SELECT state FROM frozen_usage_recovery WHERE request_id=$1 AND api_key_id=$2`, cmd.RequestID, cmd.APIKeyID).Scan(&state))
	require.Equal(t, "pending", state)
	_, err = integrationDB.Exec(`DROP TRIGGER fail_frozen_usage_test ON usage_logs; DROP FUNCTION fail_frozen_usage_test()`)
	require.NoError(t, err)
	// Change current assignment and price-bearing model metadata. Recovery must
	// keep the original actor, payer, reservation, cost and allocation snapshot.
	_, err = integrationDB.Exec(`UPDATE workspaces SET billing_owner_user_id=$2 WHERE id=$1`, a.WorkspaceID, a.ActorUserID)
	require.NoError(t, err)
	count, err := recovery.RecoverFrozenTenantUsage(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	assertTenantUsageTotals(t, a, res, cmd, 99, 1, 0, 1, 1)
	count, err = recovery.RecoverFrozenTenantUsage(ctx, 10)
	require.NoError(t, err)
	require.Zero(t, count)
	require.NoError(t, integrationDB.QueryRow(`SELECT state FROM frozen_usage_recovery WHERE request_id=$1 AND api_key_id=$2`, cmd.RequestID, cmd.APIKeyID).Scan(&state))
	require.Equal(t, "settled", state)
	var snapshots int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM usage_allocation_snapshots s JOIN usage_logs u ON u.id=s.usage_log_id WHERE u.request_id=$1 AND u.api_key_id=$2`, cmd.RequestID, cmd.APIKeyID).Scan(&snapshots))
	require.Equal(t, 1, snapshots)
	_, err = integrationDB.Exec(`DELETE FROM frozen_usage_recovery WHERE request_id=$1 AND api_key_id=$2`, cmd.RequestID, cmd.APIKeyID)
	require.Error(t, err, "financial recovery evidence must resist deletion")
	_ = service.ErrUsageBillingRequestConflict
}
