-- story: e07s01
-- One row per imported statement line. id_idx is the blind index of
-- account ID + the file's row id (dedupe per account, never plaintext);
-- raw holds the sealed extra fields (e.g. the "conta" text) for future
-- auto-categorization.

-- +goose Up
CREATE TABLE ledger_import_line (
    id             TEXT PRIMARY KEY,     -- UUIDv7
    user_id        TEXT NOT NULL,
    account_id     TEXT NOT NULL REFERENCES ledger_account (id),
    transaction_id TEXT NOT NULL REFERENCES ledger_transaction (id),
    id_idx         BLOB NOT NULL,        -- vault.BlindIndex("import-id", account_id ‖ 0x00 ‖ row id)
    raw            BLOB NOT NULL,        -- sealed JSON, field "import"
    created_at     TEXT NOT NULL,
    UNIQUE (account_id, id_idx)
);
CREATE INDEX ledger_import_line_transaction ON ledger_import_line (transaction_id);

-- +goose Down
DROP TABLE ledger_import_line;
