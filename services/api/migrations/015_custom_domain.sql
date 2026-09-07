ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS custom_domain TEXT NOT NULL DEFAULT '';
