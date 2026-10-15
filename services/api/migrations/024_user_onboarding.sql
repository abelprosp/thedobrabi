-- Onboarding passa a ser por utilizador: quem já viu/concluiu não volta a ver noutros logins.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS onboarding_step TEXT NOT NULL DEFAULT 'welcome',
    ADD COLUMN IF NOT EXISTS onboarding_completed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS onboarding_dismissed_at TIMESTAMPTZ;

-- Contas existentes já passaram pelo onboarding (ou foram dispensadas pela migração 007).
UPDATE users
SET onboarding_step = 'done',
    onboarding_completed_at = COALESCE(onboarding_completed_at, now()),
    onboarding_dismissed_at = COALESCE(onboarding_dismissed_at, now())
WHERE onboarding_completed_at IS NULL
  AND created_at < now() - interval '1 minute';
