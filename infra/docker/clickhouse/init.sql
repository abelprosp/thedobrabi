CREATE DATABASE IF NOT EXISTS thedobra;

-- Shared telemetry / IoT events. Dataset tables are created per ingest.
CREATE TABLE IF NOT EXISTS thedobra.iot_events
(
    org_id UUID,
    workspace_id UUID,
    device_id String,
    topic String,
    metric String,
    value Float64,
    status String,
    ts DateTime64(3),
    ingested_at DateTime64(3) DEFAULT now64(3)
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(ts)
ORDER BY (org_id, workspace_id, device_id, metric, ts)
TTL toDateTime(ts) + INTERVAL 180 DAY;

CREATE TABLE IF NOT EXISTS thedobra.query_telemetry
(
    org_id UUID,
    workspace_id UUID,
    fingerprint String,
    duration_ms UInt32,
    row_count UInt32,
    cache_hit UInt8,
    ts DateTime64(3) DEFAULT now64(3)
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(ts)
ORDER BY (org_id, fingerprint, ts)
TTL toDateTime(ts) + INTERVAL 90 DAY;
