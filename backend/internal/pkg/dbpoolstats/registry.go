// Package dbpoolstats exposes process-local database pool statistics without
// opening connections or retaining credentials. Pools unregister at shutdown.
package dbpoolstats

import (
	"database/sql"
	"sync"
)

var mu sync.RWMutex
var pools = map[*sql.DB]map[string]*sql.DB{}

func Register(primary *sql.DB, role string, pool *sql.DB) func() {
	mu.Lock()
	if pools[primary] == nil {
		pools[primary] = map[string]*sql.DB{}
	}
	pools[primary][role] = pool
	mu.Unlock()
	return func() {
		mu.Lock()
		defer mu.Unlock()
		if pools[primary][role] == pool {
			delete(pools[primary], role)
		}
		if len(pools[primary]) == 0 {
			delete(pools, primary)
		}
	}
}

func Snapshot(primary *sql.DB) map[string]sql.DBStats {
	out := map[string]sql.DBStats{}
	if primary != nil {
		out["primary"] = primary.Stats()
	}
	mu.RLock()
	defer mu.RUnlock()
	for role, pool := range pools[primary] {
		out[role] = pool.Stats()
	}
	return out
}
