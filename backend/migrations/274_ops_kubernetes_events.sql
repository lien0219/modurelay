CREATE TABLE IF NOT EXISTS ops_kubernetes_events (
    cluster VARCHAR(253) NOT NULL,
    namespace VARCHAR(253) NOT NULL,
    event_uid VARCHAR(253) NOT NULL,
    pod VARCHAR(253) NOT NULL,
    pod_uid VARCHAR(253) NOT NULL,
    reason VARCHAR(253) NOT NULL DEFAULT '',
    event_type VARCHAR(32) NOT NULL DEFAULT '',
    message TEXT NOT NULL DEFAULT '',
    source VARCHAR(253) NOT NULL DEFAULT '',
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    occurrence_count BIGINT NOT NULL DEFAULT 1 CHECK (occurrence_count >= 0),
    collected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (cluster, namespace, event_uid)
);
CREATE INDEX IF NOT EXISTS ops_kubernetes_events_pod_history_idx
    ON ops_kubernetes_events(cluster, namespace, pod_uid, last_seen_at DESC);
CREATE INDEX IF NOT EXISTS ops_kubernetes_events_retention_idx
    ON ops_kubernetes_events(last_seen_at);

