-- name: ListProjects :many
SELECT cp.*, n.name as network_name, n.platform as network_platform 
FROM chaincode_projects cp 
LEFT JOIN networks n ON cp.network_id = n.id 
ORDER BY cp.created_at DESC;

-- name: CreateProject :one
INSERT INTO chaincode_projects (name, description, boilerplate, slug, network_id, endorsement_policy) VALUES (?, ?, ?, ?, ?, ?) RETURNING *;

-- name: DeleteProject :exec
DELETE FROM chaincode_projects WHERE id = ?;

-- name: GetProject :one
SELECT cp.*, n.name as network_name, n.platform as network_platform 
FROM chaincode_projects cp 
LEFT JOIN networks n ON cp.network_id = n.id 
WHERE cp.id = ?;

-- name: GetProjectBySlug :one
SELECT cp.*, n.name as network_name, n.platform as network_platform 
FROM chaincode_projects cp 
LEFT JOIN networks n ON cp.network_id = n.id 
WHERE cp.slug = ?;

-- name: UpdateProjectEndorsementPolicy :one
UPDATE chaincode_projects
SET endorsement_policy = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: CreateConversation :one
INSERT INTO conversations (project_id) VALUES (?) RETURNING *;

-- name: GetDefaultConversationForProject :one
SELECT * FROM conversations WHERE project_id = ? ORDER BY started_at ASC LIMIT 1;

-- name: InsertMessage :one
INSERT INTO messages (conversation_id, parent_id, sender, content, enhanced_content, tool_arguments, is_internal) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING *;

-- name: ListMessagesForConversation :many
SELECT * FROM messages WHERE conversation_id = ? ORDER BY created_at ASC;

-- name: ListConversationsForProject :many
SELECT * FROM conversations WHERE project_id = ? ORDER BY started_at ASC;

-- name: InsertToolCall :one
INSERT INTO tool_calls (message_id, tool_name, arguments, result, error)
VALUES (?, ?, ?, ?, ?) RETURNING *;

-- name: ListToolCallsForMessage :many
SELECT * FROM tool_calls WHERE message_id = ? ORDER BY created_at ASC;

-- name: ListToolCallsForConversation :many
SELECT tc.* FROM tool_calls tc
JOIN messages m ON tc.message_id = m.id
WHERE m.conversation_id = ?
ORDER BY tc.created_at ASC;

-- name: UpdateProjectContainerInfo :exec
UPDATE chaincode_projects
SET
  container_id = ?,
  container_name = ?,
  status = ?,
  last_started_at = ?,
  last_stopped_at = ?,
  container_port = ?
WHERE id = ?;

-- name: UpdateMessageEnhancedContent :one
UPDATE messages SET enhanced_content = ? WHERE id = ? RETURNING *;

-- name: GetConversation :one
SELECT id, project_id, started_at FROM conversations WHERE id = ? LIMIT 1;
