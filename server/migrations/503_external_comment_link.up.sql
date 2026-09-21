CREATE TABLE external_comment_link (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_issue_id   UUID NOT NULL,
    external_comment_id TEXT NOT NULL,
    comment_id          UUID NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
