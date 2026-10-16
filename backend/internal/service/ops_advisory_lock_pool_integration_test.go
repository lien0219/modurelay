//go:build integration

package service

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pglock"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestAdvisoryLockDedicatedPoolAllowsSingleBusinessConnection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := exec.CommandContext(ctx, "docker", "info").Run(); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("docker is required for integration tests: %v", err)
		}
		t.Skip("docker is unavailable")
	}
	image := os.Getenv("SUB2API_TEST_POSTGRES_IMAGE")
	if image == "" {
		image = "postgres:18.1-alpine3.23"
	}
	container, err := tcpostgres.Run(ctx, image, tcpostgres.WithDatabase("lock_pool_test"), tcpostgres.WithUsername("postgres"), tcpostgres.WithPassword("postgres"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	primary, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = primary.Close() })
	primary.SetMaxOpenConns(1)
	locks, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	locks.SetMaxOpenConns(16)
	cleanup, err := pglock.Register(primary, locks)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cleanup()) })

	release, acquired, err := tryAcquireDBAdvisoryLockWithError(ctx, primary, 9182347)
	require.NoError(t, err)
	require.True(t, acquired)
	t.Cleanup(release)
	require.Equal(t, 1, locks.Stats().InUse)
	require.Equal(t, 0, primary.Stats().InUse)
	// This would deadlock until timeout if the advisory lock used the only
	// business connection. A second independent lock must not starve it either.
	releaseSecond, acquired, err := tryAcquireDBAdvisoryLockWithError(ctx, primary, 9182348)
	require.NoError(t, err)
	require.True(t, acquired)
	t.Cleanup(releaseSecond)
	queryCtx, queryCancel := context.WithTimeout(ctx, 2*time.Second)
	defer queryCancel()
	var one int
	require.NoError(t, primary.QueryRowContext(queryCtx, "SELECT 1").Scan(&one))
	require.Equal(t, 1, one)
	_, acquired, err = tryAcquireDBAdvisoryLockWithError(ctx, primary, 9182347)
	require.NoError(t, err)
	require.False(t, acquired, "another lock-pool session must respect the held lock")
	release()
	releaseSecond()
	require.NoError(t, cleanup())
	require.Same(t, primary, pglock.PoolFor(primary))
	require.Error(t, locks.PingContext(ctx))
	require.NoError(t, primary.PingContext(ctx))
}
