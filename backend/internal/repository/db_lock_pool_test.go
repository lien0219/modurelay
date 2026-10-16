//go:build unit

package repository

import (
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pglock"
	"github.com/stretchr/testify/require"
)

func TestManagedDatabaseDriverClosesAndUnregistersLockPool(t *testing.T) {
	primary, pm, err := sqlmock.New()
	require.NoError(t, err)
	locks, lm, err := sqlmock.New()
	require.NoError(t, err)
	cleanup, err := pglock.Register(primary, locks)
	require.NoError(t, err)
	driver := &primaryWithReadReplicaDriver{Driver: entsql.OpenDB(dialect.Postgres, primary), closeLockPool: cleanup}
	client := dbent.NewClient(dbent.Driver(driver))
	// A managed driver without a replica must not install a typed-nil queryer.
	repo := NewUsageLogRepository(client, primary).(*usageLogRepository)
	require.Same(t, primary, repo.reportingSQL())
	lm.ExpectClose()
	pm.ExpectClose()
	require.NoError(t, client.Close())
	require.NoError(t, client.Close())
	require.Same(t, primary, pglock.PoolFor(primary))
	require.NoError(t, lm.ExpectationsWereMet())
	require.NoError(t, pm.ExpectationsWereMet())
}

func TestAdvisoryPoolHasIndependentBoundedCapacity(t *testing.T) {
	primary, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = primary.Close() }()
	primary.SetMaxOpenConns(1)
	cfg := &config.Config{Timezone: "UTC", Database: config.DatabaseConfig{Host: "localhost", Port: 5432, User: "postgres", DBName: "test", SSLMode: "disable"}}
	cleanup, err := initAdvisoryLockPool(primary, cfg)
	require.NoError(t, err)
	defer func() { require.NoError(t, cleanup()) }()
	locks := pglock.PoolFor(primary)
	require.NotSame(t, primary, locks)
	require.Equal(t, advisoryLockMaxOpenConns, locks.Stats().MaxOpenConnections)
	require.Equal(t, 1, primary.Stats().MaxOpenConnections)
}
