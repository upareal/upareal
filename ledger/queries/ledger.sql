-- story: e04s01

-- name: GetUserKey :one
SELECT data_key FROM ledger_user_key WHERE user_id = ?;

-- name: CreateUserKey :exec
INSERT INTO ledger_user_key (user_id, data_key, created_at) VALUES (?, ?, ?);

-- name: CountGroups :one
SELECT count(*) FROM ledger_group WHERE user_id = ?;

-- name: CreateGroup :exec
INSERT INTO ledger_group (id, user_id, kind, name, position, created_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: CreateAccount :exec
INSERT INTO ledger_account (id, user_id, type, group_id, name, position, system_role, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListSystemRoles :many
SELECT system_role FROM ledger_account WHERE user_id = ? AND system_role IS NOT NULL;

-- name: ListActiveCategories :many
SELECT g.id AS group_id, g.kind AS group_kind, g.name AS group_name,
       a.id AS account_id, a.name AS account_name
FROM ledger_group g
LEFT JOIN ledger_account a
       ON a.group_id = g.id AND a.archived_at IS NULL AND a.user_id = g.user_id
WHERE g.user_id = ? AND g.archived_at IS NULL
ORDER BY g.position, a.position;
