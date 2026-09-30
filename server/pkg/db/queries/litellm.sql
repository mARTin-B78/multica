-- name: GetLiteLLMConnectionByWorkspace :one
SELECT * FROM litellm_connection WHERE workspace_id = $1;

-- name: UpsertLiteLLMConnection :one
INSERT INTO litellm_connection (
    workspace_id, base_url, api_key_encrypted, connected_by_id
) VALUES (
    $1, $2, $3, sqlc.narg('connected_by_id')
)
ON CONFLICT (workspace_id) DO UPDATE SET
    base_url = EXCLUDED.base_url,
    api_key_encrypted = EXCLUDED.api_key_encrypted,
    connected_by_id = EXCLUDED.connected_by_id,
    updated_at = now()
RETURNING *;

-- name: DeleteLiteLLMConnection :exec
DELETE FROM litellm_connection WHERE workspace_id = $1;

-- name: UpdateLiteLLMManagementKey :one
UPDATE litellm_connection
SET management_api_key_encrypted = $2,
    updated_at = now()
WHERE workspace_id = $1
RETURNING *;
