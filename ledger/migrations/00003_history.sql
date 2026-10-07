-- story: e06s04
-- Soft delete and append-only, encrypted edit history (snapshot = sealed JSON
-- of the previous version: time, description, legs).

-- +goose Up
ALTER TABLE ledger_transaction ADD COLUMN deleted_at TEXT;

CREATE TABLE ledger_transaction_history (
    id             TEXT PRIMARY KEY,     -- UUIDv7
    transaction_id TEXT NOT NULL REFERENCES ledger_transaction (id),
    user_id        TEXT NOT NULL,
    action         TEXT NOT NULL CHECK (action IN ('edit', 'delete')),
    changed_at     TEXT NOT NULL,
    snapshot       BLOB NOT NULL         -- sealed, field "history"
);
CREATE INDEX ledger_history_transaction ON ledger_transaction_history (transaction_id, changed_at);

-- +goose Down
DROP TABLE ledger_transaction_history;
ALTER TABLE ledger_transaction DROP COLUMN deleted_at;
