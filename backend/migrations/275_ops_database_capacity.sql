-- Keep application pool budgets and shared database instance limits distinct.
-- Nullable: historical rows and restricted database roles remain Unknown.
ALTER TABLE ops_system_metrics ADD COLUMN IF NOT EXISTS capacity JSONB;
