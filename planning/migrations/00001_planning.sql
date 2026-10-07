-- story: e08s01
-- planning joins ledger and budget by ID only (no foreign keys across
-- modules, no personal text). P1: one category per account.

-- +goose Up
CREATE TABLE planning_mapping (
    user_id     TEXT NOT NULL,
    account_id  TEXT NOT NULL,   -- ledger account (income or expense)
    category_id TEXT NOT NULL,   -- budget category of the same kind
    updated_at  TEXT NOT NULL,
    PRIMARY KEY (user_id, account_id)
);

-- +goose Down
DROP TABLE planning_mapping;
