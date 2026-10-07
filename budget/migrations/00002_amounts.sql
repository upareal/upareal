-- story: e05s03
-- Amount allowed/expected per category per month (B3: one per category+month;
-- B4: int64 centavos >= 0). budget_month records which months were opened, so
-- "deliberately zero" is distinct from "never opened" and copying happens once.

-- +goose Up
CREATE TABLE budget_month (
    user_id   TEXT NOT NULL,
    month     TEXT NOT NULL CHECK (month GLOB '[0-9][0-9][0-9][0-9]-[01][0-9]'),
    opened_at TEXT NOT NULL,
    PRIMARY KEY (user_id, month)
);

CREATE TABLE budget_amount (
    user_id     TEXT NOT NULL,
    category_id TEXT NOT NULL REFERENCES budget_category (id),
    month       TEXT NOT NULL CHECK (month GLOB '[0-9][0-9][0-9][0-9]-[01][0-9]'),
    amount      INTEGER NOT NULL CHECK (amount >= 0),
    PRIMARY KEY (category_id, month)
);
CREATE INDEX budget_amount_user_month ON budget_amount (user_id, month);

-- +goose Down
DROP TABLE budget_amount;
DROP TABLE budget_month;
