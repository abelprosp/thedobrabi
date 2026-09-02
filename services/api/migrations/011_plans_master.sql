-- Planos TheDobra (essencial / pro / completo), testes de 7 dias e conta master.

ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS trial_ends_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS trial_features JSONB,
    ADD COLUMN IF NOT EXISTS billing_notes TEXT;

ALTER TABLE organizations ALTER COLUMN plan SET DEFAULT 'essencial';

UPDATE organizations SET plan = CASE
    WHEN plan IN ('growth') THEN 'pro'
    WHEN plan IN ('business', 'enterprise') THEN 'completo'
    WHEN plan IN ('starter', '') OR plan IS NULL THEN 'essencial'
    ELSE plan
END
WHERE plan IN ('starter', 'growth', 'business', 'enterprise', '');

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS is_platform_owner BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE users
SET is_platform_owner = TRUE
WHERE lower(email::text) IN ('redobrai@gmail.com', 'redorai@gmail.com');

UPDATE organizations o
SET plan = 'completo'
FROM organization_members om
JOIN users u ON u.id = om.user_id
WHERE om.org_id = o.id
  AND om.role = 'owner'
  AND lower(u.email::text) IN ('redobrai@gmail.com', 'redorai@gmail.com');
