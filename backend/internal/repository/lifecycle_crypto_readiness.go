package repository

import (
	"context"
	"database/sql"
	"encoding/hex"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func validLifecycleCryptoReader(reader service.LifecycleCryptoReader) bool {
	fingerprint, err := hex.DecodeString(reader.Fingerprint)
	if err != nil || len(fingerprint) != 32 || !config.ValidLifecycleKeyIdentifier(reader.InstanceID) || !service.ValidLifecycleCryptoToken(reader.Token) || len(reader.ExpectedInstanceIDs) > 128 {
		return false
	}
	seen := map[string]bool{}
	for _, id := range reader.ExpectedInstanceIDs {
		if !config.ValidLifecycleKeyIdentifier(id) || seen[id] {
			return false
		}
		seen[id] = true
	}
	return len(seen) == 0 || seen[reader.InstanceID]
}

// Reader registration and V2 claim share a short advisory-locked transaction.
// A mismatched registration is still committed, so other writers observe it.
func (r *workspaceRepository) lifecycleCryptoTx(ctx context.Context, reader service.LifecycleCryptoReader, requireV2 bool) (*sql.Tx, bool, error) {
	if !validLifecycleCryptoReader(reader) {
		return nil, false, service.ErrLifecycleKeyRingNotReady
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	fail := func(err error) (*sql.Tx, bool, error) { _ = tx.Rollback(); return nil, false, err }
	if _, err = tx.ExecContext(ctx, `SET LOCAL statement_timeout='3s'; SET LOCAL lock_timeout='1s'; SELECT pg_advisory_xact_lock(305,1)`); err != nil {
		return fail(err)
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO workspace_export_crypto_readers(instance_id,reader_token,keyring_fingerprint,inventory_fingerprint,v2_readable,rollback_compatible,live_until)
	 VALUES($1,$2,$3,$4,$5,$6,clock_timestamp()+interval '30 seconds')
	 ON CONFLICT(instance_id) DO UPDATE SET reader_token=EXCLUDED.reader_token,keyring_fingerprint=EXCLUDED.keyring_fingerprint,
	 inventory_fingerprint=EXCLUDED.inventory_fingerprint,v2_readable=EXCLUDED.v2_readable,rollback_compatible=EXCLUDED.rollback_compatible,
	 live_until=EXCLUDED.live_until,updated_at=clock_timestamp()
	 WHERE workspace_export_crypto_readers.reader_token=$2 OR workspace_export_crypto_readers.live_until<=clock_timestamp()`, reader.InstanceID, reader.Token, reader.Fingerprint, reader.InventoryFingerprint(), reader.V2Readable, reader.RollbackCompatible)
	if err != nil {
		return fail(err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fail(err)
	}
	if n != 1 {
		return tx, false, nil
	}
	if !requireV2 {
		return tx, true, nil
	}
	if !reader.V2Readable || !reader.RollbackCompatible || len(reader.ExpectedInstanceIDs) == 0 {
		return tx, false, nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT instance_id,keyring_fingerprint,inventory_fingerprint,v2_readable,rollback_compatible FROM workspace_export_crypto_readers WHERE live_until>clock_timestamp() ORDER BY instance_id LIMIT 129`)
	if err != nil {
		return fail(err)
	}
	defer func() { _ = rows.Close() }()
	expected := map[string]bool{}
	for _, id := range reader.ExpectedInstanceIDs {
		expected[id] = true
	}
	ready := true
	for rows.Next() {
		var id, fingerprint, inventory string
		var readable, rollback bool
		if err = rows.Scan(&id, &fingerprint, &inventory, &readable, &rollback); err != nil {
			return fail(err)
		}
		if !expected[id] || fingerprint != reader.Fingerprint || inventory != reader.InventoryFingerprint() || !readable || !rollback {
			ready = false
		}
		delete(expected, id)
	}
	if err = rows.Err(); err != nil {
		return fail(err)
	}
	return tx, ready && len(expected) == 0, nil
}

func (r *workspaceRepository) RegisterLifecycleCryptoReader(ctx context.Context, reader service.LifecycleCryptoReader, requireV2 bool) error {
	tx, ready, err := r.lifecycleCryptoTx(ctx, reader, requireV2)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = tx.Commit(); err != nil {
		return err
	}
	if !ready {
		return service.ErrLifecycleKeyRingNotReady
	}
	return nil
}

func (r *workspaceRepository) ClaimLifecycleExportWithCrypto(ctx context.Context, reader service.LifecycleCryptoReader, requireV2 bool) (*service.LifecycleExportJob, error) {
	tx, ready, err := r.lifecycleCryptoTx(ctx, reader, requireV2)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if !ready {
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return nil, service.ErrLifecycleKeyRingNotReady
	}
	job, err := r.claimLifecycleExportTx(ctx, tx)
	if err != nil {
		return nil, err
	}
	return job, tx.Commit()
}
