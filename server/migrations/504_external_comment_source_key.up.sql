CREATE UNIQUE INDEX CONCURRENTLY external_comment_link_source_key
    ON external_comment_link (external_issue_id, external_comment_id);
