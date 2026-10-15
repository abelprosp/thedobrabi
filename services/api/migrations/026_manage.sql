-- Ciclo de gestão: resumo, radar, acções acompanhadas, revisão semanal e memória.

ALTER TABLE workspaces
    ADD COLUMN IF NOT EXISTS manage_cadence TEXT NOT NULL DEFAULT 'auto',
    ADD COLUMN IF NOT EXISTS manage_watch_json JSONB NOT NULL DEFAULT '{}'::jsonb;

DO $$ BEGIN
    ALTER TABLE workspaces
        ADD CONSTRAINT workspaces_manage_cadence_chk
        CHECK (manage_cadence IN ('auto', 'daily', 'weekly', 'monthly'));
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS manage_discoveries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    dataset_id      UUID REFERENCES datasets(id) ON DELETE SET NULL,
    kind            TEXT NOT NULL,
    title           TEXT NOT NULL,
    body            TEXT NOT NULL DEFAULT '',
    next_step       TEXT NOT NULL DEFAULT '',
    impact_score    DOUBLE PRECISION NOT NULL DEFAULT 0,
    evidence_json   JSONB NOT NULL DEFAULT '{}'::jsonb,
    fingerprint     TEXT NOT NULL,
    feedback        TEXT NOT NULL DEFAULT '',
    watching        BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, fingerprint)
);
CREATE INDEX IF NOT EXISTS idx_manage_discoveries_ws
    ON manage_discoveries(org_id, workspace_id, impact_score DESC, last_seen_at DESC);

CREATE TABLE IF NOT EXISTS manage_actions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id              UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    workspace_id        UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    discovery_id        UUID REFERENCES manage_discoveries(id) ON DELETE SET NULL,
    dataset_id          UUID REFERENCES datasets(id) ON DELETE SET NULL,
    created_by          UUID REFERENCES users(id) ON DELETE SET NULL,
    owner_id            UUID REFERENCES users(id) ON DELETE SET NULL,
    owner_name          TEXT NOT NULL DEFAULT '',
    title               TEXT NOT NULL,
    body                TEXT NOT NULL DEFAULT '',
    measure             TEXT NOT NULL DEFAULT '',
    expected_result     TEXT NOT NULL DEFAULT '',
    baseline_json       JSONB NOT NULL DEFAULT '{}'::jsonb,
    status              TEXT NOT NULL DEFAULT 'open'
                        CHECK (status IN ('open', 'in_progress', 'reviewed', 'done', 'cancelled')),
    due_at              DATE,
    review_at           DATE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_manage_actions_ws
    ON manage_actions(org_id, workspace_id, status, review_at);

CREATE TABLE IF NOT EXISTS manage_action_reviews (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                  UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    workspace_id            UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    action_id               UUID NOT NULL REFERENCES manage_actions(id) ON DELETE CASCADE,
    created_by              UUID REFERENCES users(id) ON DELETE SET NULL,
    executed_note           TEXT NOT NULL DEFAULT '',
    indicator_value         DOUBLE PRECISION,
    indicator_delta_pct     DOUBLE PRECISION,
    indicator_narrative     TEXT NOT NULL DEFAULT '',
    effect_narrative        TEXT NOT NULL DEFAULT '',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_manage_action_reviews_action
    ON manage_action_reviews(action_id, created_at DESC);

CREATE TABLE IF NOT EXISTS manage_reviews (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    created_by      UUID REFERENCES users(id) ON DELETE SET NULL,
    period          TEXT NOT NULL,
    agenda_json     JSONB NOT NULL DEFAULT '{}'::jsonb,
    share_token     TEXT NOT NULL UNIQUE,
    next_review_at  DATE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, period)
);
CREATE INDEX IF NOT EXISTS idx_manage_reviews_ws
    ON manage_reviews(org_id, workspace_id, created_at DESC);

CREATE TABLE IF NOT EXISTS manage_memories (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    dataset_id      UUID REFERENCES datasets(id) ON DELETE SET NULL,
    created_by      UUID REFERENCES users(id) ON DELETE SET NULL,
    note            TEXT NOT NULL,
    chart_key       TEXT NOT NULL DEFAULT '',
    occurred_on     DATE NOT NULL DEFAULT CURRENT_DATE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_manage_memories_ws
    ON manage_memories(org_id, workspace_id, occurred_on DESC);

ALTER TABLE metric_goals
    ADD COLUMN IF NOT EXISTS starts_at DATE,
    ADD COLUMN IF NOT EXISTS ends_at DATE;
