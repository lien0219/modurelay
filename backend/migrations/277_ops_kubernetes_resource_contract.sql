-- Extend archived events without rewriting or deleting existing Pod history.
ALTER TABLE ops_kubernetes_events ADD COLUMN IF NOT EXISTS object_kind VARCHAR(64) NOT NULL DEFAULT 'Pod';
ALTER TABLE ops_kubernetes_events ADD COLUMN IF NOT EXISTS object_name VARCHAR(253) NOT NULL DEFAULT '';
ALTER TABLE ops_kubernetes_events ADD COLUMN IF NOT EXISTS object_uid VARCHAR(253) NOT NULL DEFAULT '';
UPDATE ops_kubernetes_events SET object_name=pod,object_uid=pod_uid WHERE object_uid='';
CREATE INDEX IF NOT EXISTS ops_kubernetes_events_object_history_idx ON ops_kubernetes_events(cluster,namespace,object_kind,object_uid,last_seen_at DESC);

-- Only the collector writes this current snapshot. Prometheus retains history.
CREATE TABLE IF NOT EXISTS ops_kubernetes_pod_limits (
 cluster VARCHAR(253) NOT NULL,
 namespace VARCHAR(253) NOT NULL,
 pod VARCHAR(253) NOT NULL,
 uid VARCHAR(253) NOT NULL,
 resource VARCHAR(16) NOT NULL CHECK(resource IN ('cpu','memory')),
 limit_value DOUBLE PRECISION,
 bounded BOOLEAN NOT NULL,
 observed_at TIMESTAMPTZ NOT NULL,
 PRIMARY KEY(cluster,namespace,uid,resource),
 CHECK ((bounded AND limit_value > 0) OR (NOT bounded AND limit_value IS NULL))
);
CREATE INDEX IF NOT EXISTS ops_kubernetes_pod_limits_expiry_idx ON ops_kubernetes_pod_limits(observed_at);
