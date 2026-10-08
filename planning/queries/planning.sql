-- story: e08s01

-- name: UpsertMapping :exec
INSERT INTO planning_mapping (user_id, account_id, category_id, updated_at) VALUES (?, ?, ?, ?)
ON CONFLICT (user_id, account_id) DO UPDATE SET category_id = excluded.category_id, updated_at = excluded.updated_at;

-- name: DeleteMapping :exec
DELETE FROM planning_mapping WHERE user_id = ? AND account_id = ?;

-- name: ListMappings :many
SELECT account_id, category_id FROM planning_mapping WHERE user_id = ?;

-- story: e08s06
-- name: MoveCategoryMappings :exec
UPDATE planning_mapping SET category_id = sqlc.arg(target_id), updated_at = sqlc.arg(updated_at)
WHERE user_id = sqlc.arg(user_id) AND category_id = sqlc.arg(source_id);

-- name: DeleteCategoryMappings :exec
DELETE FROM planning_mapping WHERE user_id = ? AND category_id = ?;

-- story: e09s02
-- name: DeleteUserMappings :exec
DELETE FROM planning_mapping WHERE user_id = ?;
