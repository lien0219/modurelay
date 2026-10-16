//go:build unit

package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestReadReplicaKeepsBillingAndQuotaReadsOnPrimary(t *testing.T) {
	primary, pm, err := sqlmock.New()
	require.NoError(t, err)
	replica, rm, err := sqlmock.New()
	require.NoError(t, err)
	driver := &primaryWithReadReplicaDriver{
		Driver:    entsql.OpenDB(dialect.Postgres, primary),
		reporting: &reportingQueryer{primary: primary, replica: replica},
	}
	client := dbent.NewClient(dbent.Driver(driver))
	defer func() { _ = client.Close() }()
	ctx := context.Background()
	injected, err := ProvideSQLDB(client)
	require.NoError(t, err)
	require.Same(t, primary, injected)

	// A lagging replica has no current balance or billing dedup rows. None of the
	// authorization/billing paths below may query it, even with a replica enabled.
	pm.ExpectQuery(`SELECT .*balance.* FROM "users"`).WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(10.0))
	balance, err := client.User.Query().Where(user.IDEQ(42)).Select(user.FieldBalance).Float64(ctx)
	require.NoError(t, err)
	require.Equal(t, 10.0, balance)

	usage := NewUsageLogRepository(client, injected)
	start := time.Now().Add(-time.Hour)
	pm.ExpectQuery(`(?s)FROM usage_logs.*WHERE account_id`).WithArgs(int64(7), start).
		WillReturnRows(sqlmock.NewRows([]string{"requests", "tokens", "cost", "standard_cost", "user_cost"}).AddRow(10, 100, 2, 2, 2))
	stats, err := usage.GetAccountWindowStats(ctx, 7, start)
	require.NoError(t, err)
	require.EqualValues(t, 10, stats.Requests)

	pm.ExpectBegin()
	pm.ExpectQuery(`INSERT INTO usage_billing_dedup`).WithArgs("request-1", int64(9), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	pm.ExpectQuery(`(?s)SELECT request_fingerprint.*FROM usage_billing_dedup_archive`).WithArgs("request-1", int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"request_fingerprint"}))
	pm.ExpectQuery(conditionalBalanceDeductSQL).WithArgs(1.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(9.0))
	pm.ExpectCommit()
	result, err := NewUsageBillingRepository(client, injected).Apply(ctx, &service.UsageBillingCommand{
		RequestID: "request-1", APIKeyID: 9, UserID: 42, BalanceCost: 1,
	})
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Equal(t, 9.0, *result.NewBalance)

	// Only explicitly selected reports use the replica.
	end := time.Now()
	rm.ExpectQuery(`(?s)FROM usage_logs.*WHERE user_id`).WithArgs(int64(42), start, end).
		WillReturnRows(sqlmock.NewRows([]string{"date", "requests", "input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens", "total_tokens", "cost", "actual_cost"}))
	_, err = usage.GetUserUsageTrendByUserID(ctx, 42, start, end, "hour")
	require.NoError(t, err)
	require.NoError(t, pm.ExpectationsWereMet())
	require.NoError(t, rm.ExpectationsWereMet())
}

func TestReportingQueryerFallsBackAndBacksOff(t *testing.T) {
	primary, pm, err := sqlmock.New()
	require.NoError(t, err)
	defer primary.Close()
	replica, rm, err := sqlmock.New()
	require.NoError(t, err)
	defer replica.Close()
	reader := &reportingQueryer{primary: primary, replica: replica}
	rm.ExpectQuery("SELECT 1").WillReturnError(errors.New("replica unavailable"))
	for range 2 {
		pm.ExpectQuery("SELECT 1").WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(1))
	}
	for range 2 {
		var got int
		require.NoError(t, scanSingleRow(context.Background(), reader, "SELECT 1", nil, &got))
		require.Equal(t, 1, got)
	}
	require.Greater(t, reader.retryAfter.Load(), time.Now().UnixNano())
	require.NoError(t, pm.ExpectationsWereMet())
	require.NoError(t, rm.ExpectationsWereMet())
	// A recovered replica is used again after the backoff expires.
	reader.retryAfter.Store(0)
	rm.ExpectQuery("SELECT 1").WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(2))
	var got int
	require.NoError(t, scanSingleRow(context.Background(), reader, "SELECT 1", nil, &got))
	require.Equal(t, 2, got)
	require.NoError(t, rm.ExpectationsWereMet())
}

func TestReportingQueryerCanceledContextDoesNotRetryOnPrimary(t *testing.T) {
	primary, pm, err := sqlmock.New()
	require.NoError(t, err)
	defer primary.Close()
	replica, rm, err := sqlmock.New()
	require.NoError(t, err)
	defer replica.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = (&reportingQueryer{primary: primary, replica: replica}).QueryContext(ctx, "SELECT 1")
	require.ErrorIs(t, err, context.Canceled)
	require.NoError(t, pm.ExpectationsWereMet())
	require.NoError(t, rm.ExpectationsWereMet())
}

func TestReadReplicaDriverClosesBothPools(t *testing.T) {
	primary, pm, err := sqlmock.New()
	require.NoError(t, err)
	replica, rm, err := sqlmock.New()
	require.NoError(t, err)
	driver := &primaryWithReadReplicaDriver{Driver: entsql.OpenDB(dialect.Postgres, primary), reporting: &reportingQueryer{primary: primary, replica: replica}}
	rm.ExpectClose().WillReturnError(errors.New("replica close failed"))
	pm.ExpectClose()
	require.ErrorContains(t, driver.Close(), "replica close failed")
	require.NoError(t, pm.ExpectationsWereMet())
	require.NoError(t, rm.ExpectationsWereMet())
}

func TestReportingDisabledUsesExistingPrimary(t *testing.T) {
	primary, _, err := sqlmock.New()
	require.NoError(t, err)
	defer primary.Close()
	repo := NewUsageLogRepository(nil, primary).(*usageLogRepository)
	require.Same(t, primary, repo.reportingSQL())
	var _ sqlQueryer = (*sql.DB)(nil)
}
