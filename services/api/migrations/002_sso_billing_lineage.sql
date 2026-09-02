ALTER TABLE users ADD COLUMN IF NOT EXISTS auth_provider TEXT NOT NULL DEFAULT 'password';
ALTER TABLE users ADD COLUMN IF NOT EXISTS external_id TEXT;

CREATE TABLE IF NOT EXISTS oauth_identities (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider        TEXT NOT NULL,
    subject         TEXT NOT NULL,
    email           CITEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, subject)
);

CREATE TABLE IF NOT EXISTS sso_connections (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    kind            TEXT NOT NULL CHECK (kind IN ('oidc','saml')),
    name            TEXT NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    issuer          TEXT,
    client_id       TEXT,
    client_secret_enc TEXT,
    metadata_xml    TEXT,
    domains         TEXT[] NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_sso_org_kind_name ON sso_connections(org_id, kind, name);

CREATE TABLE IF NOT EXISTS scim_tokens (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    token_hash      TEXT NOT NULL UNIQUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at    TIMESTAMPTZ,
    revoked_at      TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS stripe_customers (
    org_id              UUID PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    stripe_customer_id  TEXT NOT NULL UNIQUE,
    stripe_sub_id       TEXT,
    status              TEXT NOT NULL DEFAULT 'inactive',
    price_id            TEXT,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS billing_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID,
    stripe_event_id TEXT UNIQUE,
    type            TEXT NOT NULL,
    payload         JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS lineage_nodes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL,
    workspace_id    UUID NOT NULL,
    kind            TEXT NOT NULL, -- source, dataset, transformation, metric, dashboard, report, query
    ref_id          UUID,
    name            TEXT NOT NULL,
    meta            JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_lineage_nodes_ws ON lineage_nodes(org_id, workspace_id, kind);

CREATE TABLE IF NOT EXISTS lineage_edges (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL,
    workspace_id    UUID NOT NULL,
    from_id         UUID NOT NULL REFERENCES lineage_nodes(id) ON DELETE CASCADE,
    to_id           UUID NOT NULL REFERENCES lineage_nodes(id) ON DELETE CASCADE,
    relation        TEXT NOT NULL, -- extracts, transforms, defines, visualizes, reports
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_lineage_edges_ws ON lineage_edges(org_id, workspace_id);

CREATE TABLE IF NOT EXISTS cdc_checkpoints (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL,
    workspace_id    UUID NOT NULL,
    data_source_id  UUID NOT NULL REFERENCES data_sources(id) ON DELETE CASCADE,
    dataset_id      UUID REFERENCES datasets(id) ON DELETE SET NULL,
    table_name      TEXT NOT NULL,
    cursor_value    TEXT,
    lsn             TEXT,
    status          TEXT NOT NULL DEFAULT 'idle', -- idle, running, error
    last_error      TEXT,
    rows_applied    BIGINT NOT NULL DEFAULT 0,
    last_event_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (data_source_id, table_name)
);
