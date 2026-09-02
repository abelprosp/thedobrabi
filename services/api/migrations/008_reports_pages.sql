-- Reports multi-page editor support

ALTER TABLE reports
    ADD COLUMN IF NOT EXISTS pages_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE INDEX IF NOT EXISTS idx_reports_updated ON reports(updated_at);
