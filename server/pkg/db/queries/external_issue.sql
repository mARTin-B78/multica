-- name: ReserveExternalIssueLink :one
INSERT INTO external_issue_link (
    workspace_id, provider, source_connection_id, repo_owner, repo_name,
    issue_number, issue_id, html_url, external_updated_at
) VALUES (
    @workspace_id, @provider, @source_connection_id, @repo_owner, @repo_name,
    @issue_number, @issue_id, @html_url, sqlc.narg('external_updated_at')
)
ON CONFLICT (workspace_id, provider, source_connection_id, repo_owner, repo_name, issue_number)
DO NOTHING
RETURNING *;

-- name: GetExternalIssueLink :one
SELECT * FROM external_issue_link
WHERE workspace_id = @workspace_id
  AND provider = @provider
  AND source_connection_id = @source_connection_id
  AND repo_owner = @repo_owner
  AND repo_name = @repo_name
  AND issue_number = @issue_number;

-- name: GetExternalIssueLinkByIssue :one
SELECT * FROM external_issue_link WHERE issue_id = $1;

-- name: UpdateExternalIssueLink :one
UPDATE external_issue_link
SET html_url = @html_url,
    external_updated_at = sqlc.narg('external_updated_at'),
    updated_at = now()
WHERE id = @id
RETURNING *;

-- name: DeleteExternalIssueLinkReservation :exec
DELETE FROM external_issue_link WHERE id = @id AND issue_id = @issue_id;

-- name: ReserveExternalCommentLink :one
INSERT INTO external_comment_link (external_issue_id, external_comment_id, comment_id)
VALUES (@external_issue_id, @external_comment_id, @comment_id)
ON CONFLICT (external_issue_id, external_comment_id) DO NOTHING
RETURNING *;

-- name: GetExternalCommentLink :one
SELECT * FROM external_comment_link
WHERE external_issue_id = @external_issue_id AND external_comment_id = @external_comment_id;

-- name: GetExternalCommentLinkByComment :one
SELECT * FROM external_comment_link WHERE comment_id = $1;

-- name: DeleteExternalCommentLinkReservation :exec
DELETE FROM external_comment_link WHERE id = @id AND comment_id = @comment_id;

-- name: DeleteExternalIssueLinksForIssue :exec
WITH deleted_comments AS (
    DELETE FROM external_comment_link
    WHERE external_issue_id IN (SELECT eil.id FROM external_issue_link eil WHERE eil.issue_id = $1)
)
DELETE FROM external_issue_link eil WHERE eil.issue_id = $1;

-- name: DeleteExternalIssueLinksForWorkspace :exec
WITH deleted_comments AS (
    DELETE FROM external_comment_link
    WHERE external_issue_id IN (SELECT eil.id FROM external_issue_link eil WHERE eil.workspace_id = $1)
)
DELETE FROM external_issue_link eil WHERE eil.workspace_id = $1;
