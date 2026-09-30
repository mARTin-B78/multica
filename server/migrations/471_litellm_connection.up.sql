CREATE TABLE IF NOT EXISTS litellm_connection (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id          UUID NOT NULL UNIQUE,
    base_url              TEXT NOT NULL,
    api_key_encrypted     TEXT NOT NULL,
    connected_by_id       UUID,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
