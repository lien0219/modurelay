-- Add durable scheduling and explicit terminal failure without rewriting evidence.
ALTER TABLE finops_anomaly_detection_leases
 ADD COLUMN IF NOT EXISTS available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 ADD COLUMN IF NOT EXISTS failed_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS finops_anomaly_leases_recovery_due
 ON finops_anomaly_detection_leases(bucket_start,detector_version,available_at,workspace_id)
 WHERE completed_at IS NULL AND failed_at IS NULL;

CREATE INDEX IF NOT EXISTS finops_anomaly_leases_failed_scope
 ON finops_anomaly_detection_leases(workspace_id,bucket_start,updated_at)
 WHERE completed_at IS NULL AND failed_at IS NOT NULL;
