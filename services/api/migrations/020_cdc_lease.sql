-- Lease columns so multiple API replicas can claim CDC checkpoints exclusively.
ALTER TABLE cdc_checkpoints
    ADD COLUMN IF NOT EXISTS lease_owner TEXT,
    ADD COLUMN IF NOT EXISTS lease_until TIMESTAMPTZ;

-- Tracks successfully applied batch end-cursors for idempotent reprocessing
-- after a crash between ClickHouse insert and checkpoint advance.
CREATE TABLE IF NOT EXISTS cdc_applied_batches (
    checkpoint_id UUID NOT NULL REFERENCES cdc_checkpoints(id) ON DELETE CASCADE,
    end_cursor    TEXT NOT NULL,
    rows_applied  BIGINT NOT NULL DEFAULT 0,
    applied_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (checkpoint_id, end_cursor)
);
