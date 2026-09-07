-- Scheduled reports, branding, embed tokens, monthly cadence.

ALTER TABLE reports
    ADD COLUMN IF NOT EXISTS email_to TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS whatsapp_to TEXT NOT NULL DEFAULT '';

ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS brand_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS brand_logo_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS brand_from_email TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS dashboard_embeds (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL,
    workspace_id    UUID NOT NULL,
    dashboard_id    UUID NOT NULL,
    token           TEXT NOT NULL UNIQUE,
    created_by      UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_dashboard_embeds_dash ON dashboard_embeds(dashboard_id);

ALTER TABLE sync_schedules DROP CONSTRAINT IF EXISTS sync_schedules_kind_check;
ALTER TABLE sync_schedules ADD CONSTRAINT sync_schedules_kind_check
    CHECK (kind IN ('connector', 'flow', 'dataset', 'report'));

ALTER TABLE sync_schedules DROP CONSTRAINT IF EXISTS sync_schedules_frequency_check;
ALTER TABLE sync_schedules ADD CONSTRAINT sync_schedules_frequency_check
    CHECK (frequency IN ('15m', 'hourly', 'daily', 'weekly', 'monthly'));
