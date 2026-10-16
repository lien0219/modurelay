package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/lib/pq"
)

const readReplicaRetryInterval = 30 * time.Second

// primaryWithReadReplicaDriver delegates every Ent operation and transaction to
// its embedded primary driver and owns auxiliary pool lifetimes. The optional
// replica is only accessible through the reporting query interface; it is never
// a default driver or an SQL router. A nil reporting field disables replica reads.
type primaryWithReadReplicaDriver struct {
	*entsql.Driver
	reporting     *reportingQueryer
	closeLockPool func() error
	closeOnce     sync.Once
	closeErr      error
}

func (d *primaryWithReadReplicaDriver) Close() error {
	d.closeOnce.Do(func() {
		var lockErr, replicaErr error
		if d.closeLockPool != nil {
			lockErr = d.closeLockPool()
		}
		if d.reporting != nil {
			replicaErr = d.reporting.replica.Close()
		}
		d.closeErr = errors.Join(lockErr, replicaErr, d.Driver.Close())
	})
	return d.closeErr
}

// reportingQueryer deliberately has no Exec or Begin method. Call sites must
// explicitly opt into eventually consistent reporting reads. On replica errors
// it retries on the primary and backs off to avoid adding connection storms.
type reportingQueryer struct {
	primary    *sql.DB
	replica    *sql.DB
	retryAfter atomic.Int64
}

func (q *reportingQueryer) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if time.Now().UnixNano() >= q.retryAfter.Load() {
		rows, err := q.replica.QueryContext(ctx, query, args...)
		if err == nil {
			return rows, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		q.retryAfter.Store(time.Now().Add(readReplicaRetryInterval).UnixNano())
		slog.Warn("database reporting replica unavailable; falling back to primary", "error", err)
	}
	return q.primary.QueryContext(ctx, query, args...)
}

func initReadReplica(primary *entsql.Driver, cfg *config.Config) (*primaryWithReadReplicaDriver, error) {
	connector, err := pq.NewConnector(cfg.Database.ReadReplicaDSNWithTimezone(cfg.Timezone))
	if err != nil {
		return nil, fmt.Errorf("configure database read replica: %w", err)
	}
	var replica *sql.DB
	if cfg.Server.EnableServerTiming {
		replica = sql.OpenDB(newServerTimingConnector(connector))
	} else {
		replica = sql.OpenDB(connector)
	}
	replicaCfg := *cfg
	replicaCfg.Database = cfg.Database.ReadReplicaDatabase()
	applyDBPoolSettings(replica, &replicaCfg)

	reporting := &reportingQueryer{primary: primary.DB(), replica: replica}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Database.ReadReplica.ConnectTimeoutSeconds)*time.Second)
	defer cancel()
	if err := replica.PingContext(ctx); err != nil {
		reporting.retryAfter.Store(time.Now().Add(readReplicaRetryInterval).UnixNano())
		slog.Warn("database reporting replica unavailable at startup; using primary until retry", "error", err)
	}
	return &primaryWithReadReplicaDriver{Driver: primary, reporting: reporting}, nil
}

// reportingSQL is used only by dashboard and trend methods. All usage log
// mutations, detail lookups, account windows and quota/scheduler reads keep r.sql.
func (r *usageLogRepository) reportingSQL() sqlQueryer {
	if r.reporting != nil {
		return r.reporting
	}
	return r.sql
}
