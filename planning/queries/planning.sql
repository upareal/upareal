-- story: e08s01

-- name: UpsertMapping :exec
INSERT INTO planning_mapping (user_id, account_id, category_id, updated_at) VALUES (?, ?, ?, ?)
ON CONFLICT (user_id, account_id) DO UPDATE SET category_id = excluded.category_id, updated_at = excluded.updated_at;

-- name: DeleteMapping :exec
DELETE FROM planning_mapping WHERE user_id = ? AND account_id = ?;

-- name: ListMappings :many
SELECT account_id, category_id FROM planning_mapping WHERE user_id = ?;
