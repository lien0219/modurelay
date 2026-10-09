-- Metadata-only readiness for explicit multi-instance V2 promotion. Key bytes
-- and secret configuration are deliberately absent. No financial data changes.
CREATE TABLE IF NOT EXISTS workspace_export_crypto_readers (
    instance_id VARCHAR(64) PRIMARY KEY CHECK (instance_id ~ '^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$'),
    reader_token UUID NOT NULL,
    keyring_fingerprint CHAR(64) NOT NULL CHECK (keyring_fingerprint ~ '^[0-9a-f]{64}$'),
    inventory_fingerprint CHAR(64) NOT NULL CHECK (inventory_fingerprint ~ '^[0-9a-f]{64}$'),
    v2_readable BOOLEAN NOT NULL,
    rollback_compatible BOOLEAN NOT NULL,
    live_until TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX IF NOT EXISTS workspace_export_crypto_readers_live ON workspace_export_crypto_readers(live_until,instance_id);
