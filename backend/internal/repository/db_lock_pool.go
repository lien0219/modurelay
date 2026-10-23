package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/dbpoolstats"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pglock"
	"github.com/lib/pq"
)

const (
	advisoryLockMaxOpenConns = 16
	advisoryLockMaxIdleConns = 2
)

func initAdvisoryLockPool(primary *sql.DB, cfg *config.Config) (func() error, error) {
	// Always connect to the writer. This separate bounded pool prevents jobs
	// holding different locks from consuming every connection needed by their
	// own work, especially when the business pool has MaxOpenConns == 1.
	dsn := cfg.Database.DSNWithTimezone(cfg.Timezone) + " connect_timeout=5 application_name=modurelay-advisory-lock"
	connector, err := pq.NewConnector(dsn)
	if err != nil {
		return nil, fmt.Errorf("configure advisory-lock database pool: %w", err)
	}
	lockPool := sql.OpenDB(connector)
	lockPool.SetMaxOpenConns(advisoryLockMaxOpenConns)
	lockPool.SetMaxIdleConns(advisoryLockMaxIdleConns)
	lockPool.SetConnMaxLifetime(30 * time.Minute)
	lockPool.SetConnMaxIdleTime(5 * time.Minute)
	cleanup, err := pglock.Register(primary, lockPool)
	if err != nil {
		_ = lockPool.Close()
		return nil, err
	}
	unregister := dbpoolstats.Register(primary, "advisory_lock", lockPool)
	return func() error { unregister(); return cleanup() }, nil
}
