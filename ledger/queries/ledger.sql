-- story: e04s01

-- name: GetUserKey :one
SELECT data_key FROM ledger_user_key WHERE user_id = ?;

-- name: CreateUserKey :exec
INSERT INTO ledger_user_key (user_id, data_key, created_at) VALUES (?, ?, ?);

-- name: CreateAccount :exec
INSERT INTO ledger_account (id, user_id, type, name, position, system_role, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListSystemRoles :many
SELECT system_role FROM ledger_account WHERE user_id = ? AND system_role IS NOT NULL;
