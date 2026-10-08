-- story: e04s01 e04s02 e04s03 e06s01 e06s04 e06s05 e07s01

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
JOIN ledger_transaction t ON t.id = l.transaction_id
WHERE a.user_id = ? AND l.account_id = ? AND t.deleted_at IS NULL;

-- name: AccountBalances :many
SELECT l.account_id, CAST(SUM(l.amount) AS INTEGER) AS balance FROM ledger_leg l
JOIN ledger_account a ON a.id = l.account_id
JOIN ledger_transaction t ON t.id = l.transaction_id
WHERE a.user_id = ? AND t.deleted_at IS NULL
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

-- name: SetAccountArchived :exec
UPDATE ledger_account SET archived_at = ? WHERE id = ? AND user_id = ?;

-- name: ListArchivedAccounts :many
SELECT id, type, name, system_role, on_budget, is_card FROM ledger_account
WHERE user_id = ? AND archived_at IS NOT NULL
ORDER BY CASE type WHEN 'asset' THEN 1 WHEN 'liability' THEN 2 WHEN 'income' THEN 3
                   WHEN 'expense' THEN 4 ELSE 5 END,
         position, created_at;

-- name: DeleteAccount :exec
DELETE FROM ledger_account WHERE id = ? AND user_id = ?;

-- name: BalanceBefore :one
SELECT CAST(COALESCE(SUM(l.amount), 0) AS INTEGER) FROM ledger_leg l
JOIN ledger_transaction t ON t.id = l.transaction_id
WHERE t.user_id = ? AND l.account_id = ? AND t.occurred_at < ? AND t.deleted_at IS NULL;

-- name: RegisterEntries :many
SELECT t.id, t.occurred_at, t.description, CAST(SUM(l.amount) AS INTEGER) AS amount
FROM ledger_leg l
JOIN ledger_transaction t ON t.id = l.transaction_id
WHERE t.user_id = sqlc.arg(user_id) AND l.account_id = sqlc.arg(account_id)
  AND t.occurred_at >= sqlc.arg(from_at) AND t.occurred_at < sqlc.arg(to_at)
  AND t.deleted_at IS NULL
GROUP BY t.id
ORDER BY t.occurred_at, t.created_at, t.id;

-- name: Counterparts :many
SELECT l.transaction_id, a.name FROM ledger_leg l
JOIN ledger_account a ON a.id = l.account_id
WHERE a.user_id = sqlc.arg(user_id) AND l.account_id <> sqlc.arg(account_id)
  AND l.transaction_id IN (sqlc.slice('transaction_ids'))
ORDER BY l.transaction_id, a.position, a.id;

-- name: GetTransaction :one
SELECT id, occurred_at, description FROM ledger_transaction
WHERE id = ? AND user_id = ? AND deleted_at IS NULL;

-- name: TransactionLegs :many
SELECT l.account_id, l.amount, a.type, a.system_role FROM ledger_leg l
JOIN ledger_account a ON a.id = l.account_id
WHERE l.transaction_id = ? AND a.user_id = ?
ORDER BY l.id;

-- name: UpdateTransaction :exec
UPDATE ledger_transaction SET occurred_at = ?, description = ? WHERE id = ? AND user_id = ?;

-- name: DeleteLegs :exec
DELETE FROM ledger_leg WHERE transaction_id = ?;

-- name: SoftDeleteTransaction :exec
UPDATE ledger_transaction SET deleted_at = ? WHERE id = ? AND user_id = ?;

-- name: InsertHistory :exec
INSERT INTO ledger_transaction_history (id, transaction_id, user_id, action, changed_at, snapshot)
VALUES (?, ?, ?, ?, ?, ?);

-- name: ListHistory :many
SELECT action, changed_at, snapshot FROM ledger_transaction_history
WHERE transaction_id = ? AND user_id = ?
ORDER BY changed_at DESC, id DESC;

-- name: SearchTransactions :many
-- Plaintext filters only; text matching happens in Go after decryption.
SELECT t.id, t.occurred_at, t.description,
       CAST(SUM(CASE WHEN l.amount > 0 THEN l.amount ELSE 0 END) AS INTEGER) AS amount
FROM ledger_transaction t
JOIN ledger_leg l ON l.transaction_id = t.id
WHERE t.user_id = sqlc.arg(user_id) AND t.deleted_at IS NULL
  AND t.occurred_at >= sqlc.arg(from_at) AND t.occurred_at < sqlc.arg(to_at)
  AND (CAST(sqlc.arg(account_id) AS TEXT) = ''
       OR EXISTS (SELECT 1 FROM ledger_leg x WHERE x.transaction_id = t.id AND x.account_id = CAST(sqlc.arg(account_id) AS TEXT)))
GROUP BY t.id
-- Use the expression, not the alias: in HAVING, "amount" would resolve to ledger_leg.amount.
HAVING (CAST(sqlc.arg(min_amount) AS INTEGER) = 0
        OR SUM(CASE WHEN l.amount > 0 THEN l.amount ELSE 0 END) >= CAST(sqlc.arg(min_amount) AS INTEGER))
   AND (CAST(sqlc.arg(max_amount) AS INTEGER) = 0
        OR SUM(CASE WHEN l.amount > 0 THEN l.amount ELSE 0 END) <= CAST(sqlc.arg(max_amount) AS INTEGER))
ORDER BY t.occurred_at DESC, t.id DESC
LIMIT 2000;

-- name: LegsOfTransactions :many
SELECT l.transaction_id, a.id AS account_id, a.type, a.name, a.system_role FROM ledger_leg l
JOIN ledger_account a ON a.id = l.account_id
WHERE a.user_id = sqlc.arg(user_id) AND l.transaction_id IN (sqlc.slice('transaction_ids'))
ORDER BY l.transaction_id, a.type, a.position;

-- name: ImportLineExists :one
SELECT count(*) FROM ledger_import_line WHERE account_id = ? AND id_idx = ?;

-- name: InsertImportLine :exec
INSERT INTO ledger_import_line (id, user_id, account_id, transaction_id, id_idx, raw, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ReviewQueue :many
-- Transactions still on an "A categorizar" account, seen from the account that holds the money.
SELECT t.id, t.occurred_at, t.description, a.id AS account_id, a.name AS account_name, l.amount, il.raw
FROM ledger_transaction t
JOIN ledger_leg l ON l.transaction_id = t.id
JOIN ledger_account a ON a.id = l.account_id AND a.type IN ('asset', 'liability') AND a.system_role IS NULL
LEFT JOIN ledger_import_line il ON il.transaction_id = t.id
WHERE t.user_id = ? AND t.deleted_at IS NULL
  AND EXISTS (SELECT 1 FROM ledger_leg u JOIN ledger_account ua ON ua.id = u.account_id
              WHERE u.transaction_id = t.id AND ua.system_role IN ('uncategorized_income', 'uncategorized_expense'))
ORDER BY t.occurred_at DESC, t.id DESC;

-- story: e08s02
-- name: ActivityBetween :many
SELECT l.account_id, CAST(SUM(l.amount) AS INTEGER) AS amount FROM ledger_leg l
JOIN ledger_transaction t ON t.id = l.transaction_id
WHERE t.user_id = sqlc.arg(user_id) AND t.occurred_at >= sqlc.arg(from_at) AND t.occurred_at < sqlc.arg(to_at)
  AND t.deleted_at IS NULL
GROUP BY l.account_id;

-- story: e08s05
-- name: BudgetFlows :many
SELECT l.account_id,
       CAST(COALESCE(SUM(CASE WHEN l.amount > 0 THEN l.amount END), 0) AS INTEGER) AS debit,
       CAST(COALESCE(SUM(CASE WHEN l.amount < 0 THEN l.amount END), 0) AS INTEGER) AS credit
FROM ledger_leg l
JOIN ledger_transaction t ON t.id = l.transaction_id
JOIN ledger_account a ON a.id = l.account_id
WHERE t.user_id = sqlc.arg(user_id) AND t.occurred_at >= sqlc.arg(from_at) AND t.occurred_at < sqlc.arg(to_at)
  AND t.deleted_at IS NULL AND a.on_budget = 0
  AND EXISTS (SELECT 1 FROM ledger_leg ob JOIN ledger_account oa ON oa.id = ob.account_id
              WHERE ob.transaction_id = t.id AND oa.on_budget = 1)
GROUP BY l.account_id;
