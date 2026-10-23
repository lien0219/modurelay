package repository

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Use the production runner, which executes each whole SQL file, rather than
// a test-only Goose Up parser. A fresh database also covers historical schema
// interactions (notably the legacy silence table) before migrations 271–275.
func TestMonitoringProductionMigrationsPostgres(t *testing.T) {
	dsn := os.Getenv("MODURELAY_MONITORING_TEST_DSN")
	if dsn == "" {
		t.Skip("set MODURELAY_MONITORING_TEST_DSN to a disposable PostgreSQL with CREATEDB privilege")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.Close() })
	name := fmt.Sprintf("monitoring_migrations_%d", time.Now().UnixNano())
	_, err = admin.Exec("CREATE DATABASE " + name)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, cleanupErr := admin.Exec("DROP DATABASE " + name)
		if cleanupErr != nil {
			t.Errorf("remove disposable migration database: %v", cleanupErr)
		}
	})
	targetDSN := dsn + " dbname=" + name
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, parseErr := url.Parse(dsn)
		require.NoError(t, parseErr)
		u.Path = "/" + name
		query := u.Query()
		query.Del("dbname")
		u.RawQuery = query.Encode()
		targetDSN = u.String()
	}
	db, err := sql.Open("postgres", targetDSN)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	require.NoError(t, ApplyMigrations(ctx, db))
	// Restarting an upgraded instance must pass checksum validation as well.
	require.NoError(t, ApplyMigrations(ctx, db))
	for _, table := range []string{
		"ops_alert_rule_states", "ops_alert_resource_states", "ops_alert_silences",
		"ops_alert_deliveries", "ops_alert_incidents", "ops_billing_reconciliation_snapshot", "ops_kubernetes_events",
		"ops_billing_intents", "ops_billing_compensation_outbox", "ops_billing_operation_audit", "ops_billing_operational_snapshot",
		"ops_account_ledger_heads", "ops_account_ledger_entries", "ops_billing_recording_gaps", "ops_billing_recording_gap_audit",
	} {
		var exists bool
		require.NoError(t, db.QueryRowContext(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&exists))
		require.True(t, exists, table+" must survive the entire migration file")
	}
	for table, column := range map[string]string{"ops_alert_rules": "min_requests", "ops_system_metrics": "capacity"} {
		var exists bool
		require.NoError(t, db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns
			WHERE table_schema='public' AND table_name=$1 AND column_name=$2)`, table, column).Scan(&exists))
		require.True(t, exists, table+"."+column)
	}
}
