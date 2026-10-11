-- story: e16s01
-- Savings target (meta) of an expense category: amount by a due month (B5).
-- At most one per category, never repeats. Amounts and months are plaintext,
-- like budget_amount; planning computes needed, progress and status (P4).

-- +goose Up
CREATE TABLE budget_target (
    user_id     TEXT NOT NULL,
    category_id TEXT NOT NULL PRIMARY KEY REFERENCES budget_category (id),
    amount      INTEGER NOT NULL CHECK (amount > 0),
    due_month   TEXT NOT NULL CHECK (due_month GLOB '[0-9][0-9][0-9][0-9]-[01][0-9]'),
    created_at  TEXT NOT NULL
);
CREATE INDEX budget_target_user ON budget_target (user_id);

-- +goose Down
DROP TABLE budget_target;
