-- Onboarding state for organizations

ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS onboarding_step TEXT NOT NULL DEFAULT 'welcome',
    ADD COLUMN IF NOT EXISTS onboarding_completed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS onboarding_seen BOOLEAN NOT NULL DEFAULT FALSE;

-- Mark existing organizations as done so we don't force onboarding on legacy accounts.
UPDATE organizations
SET onboarding_step = 'done', onboarding_completed_at = now(), onboarding_seen = TRUE
WHERE onboarding_step = 'welcome' AND onboarding_completed_at IS NULL
  AND created_at < now() - interval '1 minute';
