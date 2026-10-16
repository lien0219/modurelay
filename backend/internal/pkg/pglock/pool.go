// Package pglock associates a primary business database with a dedicated pool
// for session advisory locks. Lock holders must not exhaust business connections.
package pglock

import (
	"database/sql"
	"errors"
	"sync"
)

var pools sync.Map // map[*sql.DB]*sql.DB; owned by the registered database driver

// Register transfers ownership of lockPool to the returned idempotent cleanup.
// The caller must use the primary's DSN; replicas cannot coordinate writes.
func Register(primary, lockPool *sql.DB) (func() error, error) {
	if primary == nil || lockPool == nil || primary == lockPool {
		return nil, errors.New("advisory locks require a distinct primary connection pool")
	}
	if _, loaded := pools.LoadOrStore(primary, lockPool); loaded {
		return nil, errors.New("advisory lock pool already registered for primary")
	}
	var once sync.Once
	var closeErr error
	return func() error {
		once.Do(func() {
			pools.CompareAndDelete(primary, lockPool)
			closeErr = lockPool.Close()
		})
		return closeErr
	}, nil
}

// PoolFor returns the dedicated lock pool, or the supplied database for callers
// that do not use application initialization (including repository test doubles).
// This is only for advisory locks; business queries must keep using primary.
func PoolFor(primary *sql.DB) *sql.DB {
	if pool, ok := pools.Load(primary); ok {
		if registered, ok := pool.(*sql.DB); ok {
			return registered
		}
	}
	return primary
}
