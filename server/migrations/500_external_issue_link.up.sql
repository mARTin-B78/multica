CREATE TABLE external_issue_link (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id         UUID NOT NULL,
    provider             TEXT NOT NULL CHECK (provider IN ('github', 'forgejo', 'gitea')),
    source_connection_id UUID NOT NULL,
    repo_owner           TEXT NOT NULL,
    repo_name            TEXT NOT NULL,
    issue_number         INTEGER NOT NULL,
    issue_id             UUID NOT NULL,
    html_url             TEXT NOT NULL DEFAULT '',
    external_updated_at  TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
