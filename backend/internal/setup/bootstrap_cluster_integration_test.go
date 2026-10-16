//go:build integration

package setup

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestBootstrapAdminConcurrentPods(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	probeCtx, probeCancel := context.WithTimeout(ctx, 5*time.Second)
	err := exec.CommandContext(probeCtx, "docker", "info").Run()
	probeCancel()
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("Docker is required in CI: %v", err)
		}
		t.Skip("Docker is required for concurrent PostgreSQL bootstrap coverage")
	}
	container, err := postgres.Run(ctx, "postgres:18.1-alpine3.23",
		postgres.WithDatabase("bootstrap_test"), postgres.WithUsername("postgres"),
		postgres.WithPassword("bootstrap-test-only"), postgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.ExecContext(ctx, `CREATE TABLE users (
		id BIGSERIAL PRIMARY KEY, email TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL,
		role TEXT NOT NULL, balance NUMERIC NOT NULL, concurrency INTEGER NOT NULL,
		status TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL
	)`)
	require.NoError(t, err)

	const pods = 16
	var created atomic.Int64
	errors := make(chan error, pods)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range pods {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			// Different credentials deliberately model independently configured pods;
			// an email uniqueness constraint alone must not be the coordination mechanism.
			cfg := &SetupConfig{Admin: AdminConfig{Email: fmt.Sprintf("pod-%d@example.com", i), Password: "bootstrap-test-password"}}
			didCreate, _, err := bootstrapAdminUser(ctx, db, cfg)
			if err != nil {
				errors <- err
			}
			if didCreate {
				created.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	require.EqualValues(t, 1, created.Load())
	var count int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count))
	require.Equal(t, 1, count)
}
