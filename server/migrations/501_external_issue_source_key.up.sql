CREATE UNIQUE INDEX CONCURRENTLY external_issue_link_source_key
    ON external_issue_link (workspace_id, provider, source_connection_id, repo_owner, repo_name, issue_number);
