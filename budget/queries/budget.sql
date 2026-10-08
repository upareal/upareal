-- story: e05s01 e05s02 e05s03

-- name: GetUserKey :one
SELECT data_key FROM budget_user_key WHERE user_id = ?;

-- name: CreateUserKey :exec
INSERT INTO budget_user_key (user_id, data_key, created_at) VALUES (?, ?, ?);

-- name: CountGroups :one
SELECT count(*) FROM budget_group WHERE user_id = ?;

-- name: CreateGroup :exec
INSERT INTO budget_group (id, user_id, kind, name, position, created_at) VALUES (?, ?, ?, ?, ?, ?);

-- name: CreateCategory :exec
INSERT INTO budget_category (id, user_id, group_id, name, position, created_at) VALUES (?, ?, ?, ?, ?, ?);

-- name: ListActiveCategories :many
SELECT g.id AS group_id, g.kind AS group_kind, g.name AS group_name,
       c.id AS category_id, c.name AS category_name
FROM budget_group g
LEFT JOIN budget_category c
       ON c.group_id = g.id AND c.archived_at IS NULL AND c.user_id = g.user_id
WHERE g.user_id = ? AND g.archived_at IS NULL
ORDER BY g.position, c.position;

-- name: GetGroup :one
SELECT id, kind, position FROM budget_group WHERE id = ? AND user_id = ? AND archived_at IS NULL;

-- name: NextGroupPosition :one
SELECT CAST(COALESCE(MAX(position), -1) + 1 AS INTEGER) FROM budget_group WHERE user_id = ?;

-- name: RenameGroup :exec
UPDATE budget_group SET name = ? WHERE id = ? AND user_id = ?;

-- name: PreviousGroup :one
SELECT id, position FROM budget_group
WHERE user_id = ? AND archived_at IS NULL AND position < ? ORDER BY position DESC LIMIT 1;

-- name: NextGroup :one
SELECT id, position FROM budget_group
WHERE user_id = ? AND archived_at IS NULL AND position > ? ORDER BY position ASC LIMIT 1;

-- name: SetGroupPosition :exec
UPDATE budget_group SET position = ? WHERE id = ? AND user_id = ?;

-- name: GetCategory :one
SELECT c.id, c.group_id, c.position, g.kind FROM budget_category c
JOIN budget_group g ON g.id = c.group_id
WHERE c.id = ? AND c.user_id = ? AND c.archived_at IS NULL;

-- name: NextCategoryPosition :one
SELECT CAST(COALESCE(MAX(position), -1) + 1 AS INTEGER) FROM budget_category WHERE user_id = ? AND group_id = ?;

-- name: RenameCategory :exec
UPDATE budget_category SET name = ? WHERE id = ? AND user_id = ?;

-- name: SetCategoryGroup :exec
UPDATE budget_category SET group_id = ?, position = ? WHERE id = ? AND user_id = ?;

-- name: PreviousCategory :one
SELECT id, position FROM budget_category
WHERE user_id = ? AND group_id = ? AND archived_at IS NULL AND position < ? ORDER BY position DESC LIMIT 1;

-- name: NextCategory :one
SELECT id, position FROM budget_category
WHERE user_id = ? AND group_id = ? AND archived_at IS NULL AND position > ? ORDER BY position ASC LIMIT 1;

-- name: SetCategoryPosition :exec
UPDATE budget_category SET position = ? WHERE id = ? AND user_id = ?;

-- name: MonthOpened :one
SELECT count(*) FROM budget_month WHERE user_id = ? AND month = ?;

-- name: MarkMonthOpened :exec
INSERT INTO budget_month (user_id, month, opened_at) VALUES (?, ?, ?);

-- name: LatestOpenedMonthBefore :one
SELECT month FROM budget_month WHERE user_id = ? AND month < ? ORDER BY month DESC LIMIT 1;

-- name: CopyMonthAmounts :exec
INSERT INTO budget_amount (user_id, category_id, month, amount)
SELECT src.user_id, src.category_id, sqlc.arg(to_month), src.amount FROM budget_amount AS src
WHERE src.user_id = sqlc.arg(user_id) AND src.month = sqlc.arg(from_month);

-- name: UpsertAmount :exec
INSERT INTO budget_amount (user_id, category_id, month, amount) VALUES (?, ?, ?, ?)
ON CONFLICT (category_id, month) DO UPDATE SET amount = excluded.amount;

-- name: MonthAmounts :many
SELECT category_id, amount FROM budget_amount WHERE user_id = ? AND month = ?;

-- story: e08s03
-- name: AmountsBefore :many
SELECT category_id, CAST(SUM(amount) AS INTEGER) AS amount FROM budget_amount
WHERE user_id = ? AND month < ? GROUP BY category_id;

-- story: e08s06
-- name: SetCategoryArchived :exec
UPDATE budget_category SET archived_at = ? WHERE id = ? AND user_id = ?;

-- name: GetArchivedCategory :one
SELECT c.id, c.group_id FROM budget_category c
JOIN budget_group g ON g.id = c.group_id AND g.archived_at IS NULL
WHERE c.id = ? AND c.user_id = ? AND c.archived_at IS NOT NULL;

-- name: ListArchivedCategories :many
SELECT id, name FROM budget_category WHERE user_id = ? AND archived_at IS NOT NULL ORDER BY archived_at DESC, id;

-- name: MergeAmounts :exec
INSERT INTO budget_amount (user_id, category_id, month, amount)
SELECT user_id, sqlc.arg(target_id), month, amount FROM budget_amount
WHERE budget_amount.user_id = sqlc.arg(user_id) AND budget_amount.category_id = sqlc.arg(source_id)
ON CONFLICT (category_id, month) DO UPDATE SET amount = budget_amount.amount + excluded.amount;

-- name: DeleteCategoryAmounts :exec
DELETE FROM budget_amount WHERE user_id = ? AND category_id = ?;
