-- story: e04s02
-- Transactions and legs: amounts are debit-positive int64 centavos (BRL).
-- Zero-sum (L1), same-user (L3) and archived (L6) checks run in ledger.postTransaction.

-- +goose Up
ALTER TABLE ledger_account ADD COLUMN on_budget INTEGER NOT NULL DEFAULT 0 CHECK (on_budget IN (0, 1));
ALTER TABLE ledger_account ADD COLUMN is_card INTEGER NOT NULL DEFAULT 0
    CHECK (is_card IN (0, 1) AND (is_card = 0 OR (type = 'liability' AND on_budget = 1)));

CREATE TABLE ledger_transaction (
    id          TEXT PRIMARY KEY,        -- UUIDv7
    user_id     TEXT NOT NULL,
    occurred_at TEXT NOT NULL,           -- UTC RFC 3339
    description BLOB NOT NULL,           -- sealed, field "description"
    created_at  TEXT NOT NULL
);
CREATE INDEX ledger_transaction_user ON ledger_transaction (user_id, occurred_at);

CREATE TABLE ledger_leg (
    id             TEXT PRIMARY KEY,     -- UUIDv7
    transaction_id TEXT NOT NULL REFERENCES ledger_transaction (id) ON DELETE CASCADE,
    account_id     TEXT NOT NULL REFERENCES ledger_account (id),
    amount         INTEGER NOT NULL CHECK (amount <> 0)  -- L2
);
CREATE INDEX ledger_leg_account ON ledger_leg (account_id);
CREATE INDEX ledger_leg_transaction ON ledger_leg (transaction_id);

-- +goose Down
DROP TABLE ledger_leg;
DROP TABLE ledger_transaction;
ALTER TABLE ledger_account DROP COLUMN is_card;
ALTER TABLE ledger_account DROP COLUMN on_budget;
