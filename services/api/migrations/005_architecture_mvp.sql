-- TheDobra architectural MVP: Flows, storage modes, semantic enhancements, RLS, apps, gateway.

-- 1. Dataset storage modes
ALTER TABLE datasets ADD COLUMN IF NOT EXISTS storage_mode TEXT NOT NULL DEFAULT 'import' CHECK (storage_mode IN ('import', 'direct_query', 'composite'));
ALTER TABLE datasets ADD COLUMN IF NOT EXISTS source_table TEXT;
ALTER TABLE datasets ADD COLUMN IF NOT EXISTS source_query TEXT;

-- 2. Semantic model enhancements: hierarchy and relationship support in a side table.
CREATE TABLE IF NOT EXISTS semantic_hierarchies (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    dataset_id      UUID NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    levels          JSONB NOT NULL DEFAULT '[]', -- ordered columns
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (dataset_id, name)
);

CREATE TABLE IF NOT EXISTS semantic_relationships (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id              UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    workspace_id        UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    from_dataset_id     UUID NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    from_column         TEXT NOT NULL,
    to_dataset_id       UUID NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    to_column           TEXT NOT NULL,
    relationship_type   TEXT NOT NULL DEFAULT 'many_to_one' CHECK (relationship_type IN ('one_to_one', 'many_to_one', 'one_to_many', 'many_to_many')),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_sem_rel_from ON semantic_relationships(from_dataset_id);
CREATE INDEX IF NOT EXISTS idx_sem_rel_to ON semantic_relationships(to_dataset_id);

-- 3. Dobra Flow: visual ETL/ELT pipelines
CREATE TABLE IF NOT EXISTS flows (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'paused', 'failed')),
    schedule        TEXT,
    source_dataset_id UUID REFERENCES datasets(id) ON DELETE SET NULL,
    target_dataset_id UUID REFERENCES datasets(id) ON DELETE SET NULL,
    created_by      UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_flows_ws ON flows(org_id, workspace_id);

CREATE TABLE IF NOT EXISTS flow_steps (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    flow_id         UUID NOT NULL REFERENCES flows(id) ON DELETE CASCADE,
    step_order      INTEGER NOT NULL,
    kind            TEXT NOT NULL CHECK (kind IN ('extract', 'transform', 'validate', 'load')),
    subkind         TEXT NOT NULL DEFAULT '', -- rename, filter, change_type, join, append, aggregate, dedup, fill_null, conditional, sql, source
    name            TEXT NOT NULL,
    config          JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (flow_id, step_order)
);
CREATE INDEX IF NOT EXISTS idx_flow_steps_flow ON flow_steps(flow_id);

CREATE TABLE IF NOT EXISTS flow_runs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    flow_id         UUID NOT NULL REFERENCES flows(id) ON DELETE CASCADE,
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    started_at      TIMESTAMPTZ,
    finished_at     TIMESTAMPTZ,
    rows_processed  BIGINT,
    error           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_flow_runs_flow ON flow_runs(flow_id);

CREATE TABLE IF NOT EXISTS flow_run_logs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id          UUID NOT NULL REFERENCES flow_runs(id) ON DELETE CASCADE,
    step_id         UUID REFERENCES flow_steps(id) ON DELETE SET NULL,
    level           TEXT NOT NULL DEFAULT 'info' CHECK (level IN ('info', 'warn', 'error')),
    message         TEXT NOT NULL,
    details         JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_flow_run_logs_run ON flow_run_logs(run_id);

-- 4. Row-level security policies per dataset
CREATE TABLE IF NOT EXISTS dataset_rls (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    dataset_id      UUID NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    role            TEXT NOT NULL DEFAULT 'viewer' CHECK (role IN ('owner','admin','analyst','viewer')),
    column_name     TEXT NOT NULL,
    expression      TEXT NOT NULL, -- e.g. "user_id = current_user_id()" or "tenant_id = 'x'"
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_dataset_rls_ds ON dataset_rls(dataset_id);

-- 5. Workspace apps / packaged dashboards
CREATE TABLE IF NOT EXISTS apps (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    icon            TEXT,
    status          TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    created_by      UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_apps_ws ON apps(org_id, workspace_id);

CREATE TABLE IF NOT EXISTS app_dashboards (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id          UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    dashboard_id    UUID NOT NULL REFERENCES dashboards(id) ON DELETE CASCADE,
    dashboard_order INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (app_id, dashboard_id)
);
CREATE INDEX IF NOT EXISTS idx_app_dashboards_app ON app_dashboards(app_id);

-- 6. On-premise gateway registry skeleton
CREATE TABLE IF NOT EXISTS gateway_instances (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID REFERENCES organizations(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    token_hash      TEXT NOT NULL UNIQUE,
    status          TEXT NOT NULL DEFAULT 'offline' CHECK (status IN ('online', 'offline', 'error')),
    version         TEXT,
    last_ping_at    TIMESTAMPTZ,
    metadata        JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_gateway_instances_org ON gateway_instances(org_id);

-- 7. Query history improvements: add bytes_read and cache_hit already exists; add planner choice.
ALTER TABLE query_history ADD COLUMN IF NOT EXISTS bytes_read BIGINT;
ALTER TABLE query_history ADD COLUMN IF NOT EXISTS planner_choice TEXT;
ALTER TABLE query_history ADD COLUMN IF NOT EXISTS source_type TEXT;
