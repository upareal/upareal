-- story: e04s01
-- Money movement only: accounts (and transactions, from e04s02). Categories live in budget.
-- Names are sealed by internal/vault with the ledger's own per-user data key.
-- user_id references users by value only: no foreign keys across modules.

-- +goose Up
CREATE TABLE ledger_user_key (
    user_id    TEXT PRIMARY KEY,
    data_key   BLOB NOT NULL,            -- wrapped per-user data key (ledger's own)
    created_at TEXT NOT NULL
);

CREATE TABLE ledger_account (
    id          TEXT PRIMARY KEY,        -- UUIDv7
    user_id     TEXT NOT NULL,
    type        TEXT NOT NULL CHECK (type IN ('asset', 'liability', 'income', 'expense', 'equity')),
    name        BLOB NOT NULL,           -- sealed, field "name"
    position    INTEGER NOT NULL,
    system_role TEXT CHECK (system_role IN ('opening_balance', 'uncategorized_income', 'uncategorized_expense')),
    archived_at TEXT,
    created_at  TEXT NOT NULL
);
CREATE INDEX ledger_account_user ON ledger_account (user_id, type, position);
CREATE UNIQUE INDEX ledger_account_system_role ON ledger_account (user_id, system_role) WHERE system_role IS NOT NULL;

-- +goose Down
DROP TABLE ledger_account;
DROP TABLE ledger_user_key;
