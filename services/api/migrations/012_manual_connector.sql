-- Planilha manual: linhas preenchidas por formulário, ligadas a um data_source.

CREATE TABLE manual_rows (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    source_id       UUID NOT NULL REFERENCES data_sources(id) ON DELETE CASCADE,
    payload         JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by      UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_manual_rows_source ON manual_rows (org_id, workspace_id, source_id, created_at DESC);
