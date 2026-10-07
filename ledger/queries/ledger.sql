-- story: e04s01 e04s02 e04s03

-- name: GetUserKey :one
SELECT data_key FROM ledger_user_key WHERE user_id = ?;

-- name: CreateUserKey :exec
INSERT INTO ledger_user_key (user_id, data_key, created_at) VALUES (?, ?, ?);

-- name: CreateAccount :exec
INSERT INTO ledger_account (id, user_id, type, name, position, system_role, on_budget, is_card, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListSystemRoles :many
SELECT system_role FROM ledger_account WHERE user_id = ? AND system_role IS NOT NULL;

-- name: ListActiveAccounts :many
SELECT id, type, name, system_role, on_budget, is_card FROM ledger_account
WHERE user_id = ? AND archived_at IS NULL
ORDER BY CASE type WHEN 'asset' THEN 1 WHEN 'liability' THEN 2 WHEN 'income' THEN 3
                   WHEN 'expense' THEN 4 ELSE 5 END,
         position, created_at;

-- name: GetAccount :one
SELECT id, type, system_role, archived_at, position FROM ledger_account WHERE id = ? AND user_id = ?;

-- name: GetSystemAccountID :one
SELECT id FROM ledger_account WHERE user_id = ? AND system_role = ?;

-- name: CountPostableAccounts :one
-- L3 + L6: accounts of this user that are not archived.
SELECT count(*) FROM ledger_account
WHERE user_id = sqlc.arg(user_id) AND archived_at IS NULL AND id IN (sqlc.slice('ids'));

-- name: NextAccountPosition :one
SELECT CAST(COALESCE(MAX(position), -1) + 1 AS INTEGER) FROM ledger_account WHERE user_id = ? AND type = ?;

-- name: CreateTransaction :exec
INSERT INTO ledger_transaction (id, user_id, occurred_at, description, created_at) VALUES (?, ?, ?, ?, ?);

-- name: CreateLeg :exec
INSERT INTO ledger_leg (id, transaction_id, account_id, amount) VALUES (?, ?, ?, ?);

-- name: AccountBalance :one
SELECT CAST(COALESCE(SUM(l.amount), 0) AS INTEGER) FROM ledger_leg l
JOIN ledger_account a ON a.id = l.account_id
WHERE a.user_id = ? AND l.account_id = ?;

-- name: AccountBalances :many
SELECT l.account_id, CAST(SUM(l.amount) AS INTEGER) AS balance FROM ledger_leg l
JOIN ledger_account a ON a.id = l.account_id
WHERE a.user_id = ?
GROUP BY l.account_id;

-- name: UpdateAccount :exec
UPDATE ledger_account SET name = ?, type = ?, on_budget = ?, is_card = ? WHERE id = ? AND user_id = ?;

-- name: CountAccountLegs :one
SELECT count(*) FROM ledger_leg WHERE account_id = ?;

-- name: PreviousAccount :one
SELECT id, position FROM ledger_account
WHERE user_id = ? AND type = ? AND archived_at IS NULL AND system_role IS NULL AND position < ?
ORDER BY position DESC LIMIT 1;

-- name: NextAccount :one
SELECT id, position FROM ledger_account
WHERE user_id = ? AND type = ? AND archived_at IS NULL AND system_role IS NULL AND position > ?
ORDER BY position ASC LIMIT 1;

-- name: SetAccountPosition :exec
UPDATE ledger_account SET position = ? WHERE id = ? AND user_id = ?;
