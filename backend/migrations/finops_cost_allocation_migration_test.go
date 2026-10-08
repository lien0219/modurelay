package migrations

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFinOpsCostAllocationMigrationIsAppendOnlyAndTenantScoped(t *testing.T) {
	source, err := os.ReadFile("299_finops_cost_allocation.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(source))
	for _, table := range []string{
		"workspace_cost_centers", "workspace_allocation_tags", "project_cost_allocations",
		"project_cost_allocation_tags", "api_key_allocation_overrides",
		"service_account_allocation_overrides", "budget_reservation_allocation_snapshots",
		"usage_allocation_snapshots", "usage_allocation_hourly_rollups",
	} {
		require.Contains(t, sql, "create table if not exists "+table)
	}
	require.Contains(t, sql, "foreign key(workspace_id,project_id)")
	require.Contains(t, sql, "foreign key(workspace_id,cost_center_id)")
	require.Contains(t, sql, "foreign key(api_key_id,project_id)")
	require.Contains(t, sql, "create trigger usage_logs_allocation_snapshot_capture")
	require.Contains(t, sql, "create trigger usage_allocation_snapshot_rollup_capture")
	require.Contains(t, sql, "async video may first persist a zero-cost placeholder")
	require.Contains(t, sql, "finops allocation snapshot is immutable")
	require.Contains(t, sql, "allocation tag is still used by active allocation configuration")
	require.Contains(t, sql, "workspace_allocation_tags_archive_guard")
	require.Contains(t, sql, "usage_logs_allocation_scope_time")
	require.NotContains(t, sql, "\nupdate usage_logs")
	require.NotContains(t, sql, "\ndelete from usage_logs")
}

func TestFinOpsCostAllocationMigrationDefinesBoundedTagsAndEnvironments(t *testing.T) {
	source, err := os.ReadFile("299_finops_cost_allocation.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(source))
	require.Contains(t, sql, "(select count(*) from jsonb_object_keys(value)) <= 32")
	require.Contains(t, sql, "tag_key !~* '(secret|token|password|credential|api[_-]?key)'")
	require.Contains(t, sql, "environment in ('production','staging','development','testing')")
	require.NotContains(t, sql, "^custom:[a-z0-9]")
}
