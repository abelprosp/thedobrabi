-- Apps public sharing, reports, sections and theming

ALTER TABLE apps
    ADD COLUMN IF NOT EXISTS public_token TEXT UNIQUE,
    ADD COLUMN IF NOT EXISTS theme TEXT NOT NULL DEFAULT 'indigo',
    ADD COLUMN IF NOT EXISTS cover_url TEXT,
    ADD COLUMN IF NOT EXISTS permissions_json JSONB NOT NULL DEFAULT '{"viewer": true, "analyst": true}'::jsonb;

CREATE UNIQUE INDEX IF NOT EXISTS idx_apps_public_token ON apps(public_token);

ALTER TABLE app_dashboards
    ADD COLUMN IF NOT EXISTS section TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS app_reports (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id          UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    report_id       UUID NOT NULL REFERENCES reports(id) ON DELETE CASCADE,
    report_order    INTEGER NOT NULL DEFAULT 0,
    section         TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (app_id, report_id)
);
CREATE INDEX IF NOT EXISTS idx_app_reports_app ON app_reports(app_id);
