-- Scheduled connector / flow / dataset refreshes (product-level automation).

CREATE TABLE IF NOT EXISTS sync_schedules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    kind            TEXT NOT NULL CHECK (kind IN ('connector', 'flow', 'dataset')),
    target_id       UUID NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    frequency       TEXT NOT NULL CHECK (frequency IN ('15m', 'hourly', 'daily', 'weekly')),
    hour_local      INTEGER NOT NULL DEFAULT 6 CHECK (hour_local BETWEEN 0 AND 23),
    weekday         INTEGER NOT NULL DEFAULT 1 CHECK (weekday BETWEEN 0 AND 6),
    timezone        TEXT NOT NULL DEFAULT 'America/Sao_Paulo',
    incremental     BOOLEAN NOT NULL DEFAULT TRUE,
    table_name      TEXT NOT NULL DEFAULT '',
    last_run_at     TIMESTAMPTZ,
    next_run_at     TIMESTAMPTZ,
    last_status     TEXT NOT NULL DEFAULT 'idle',
    last_error      TEXT,
    last_mode       TEXT NOT NULL DEFAULT '',
    created_by      UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, workspace_id, kind, target_id)
);
CREATE INDEX IF NOT EXISTS idx_sync_schedules_due ON sync_schedules (enabled, next_run_at) WHERE enabled;
CREATE INDEX IF NOT EXISTS idx_sync_schedules_ws ON sync_schedules (org_id, workspace_id);

CREATE TABLE IF NOT EXISTS sync_schedule_runs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    schedule_id     UUID NOT NULL REFERENCES sync_schedules(id) ON DELETE CASCADE,
    status          TEXT NOT NULL CHECK (status IN ('running', 'ok', 'error')),
    mode            TEXT NOT NULL DEFAULT 'full',
    error           TEXT,
    rows_affected   BIGINT,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at     TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_sync_schedule_runs_sched ON sync_schedule_runs (schedule_id, started_at DESC);
