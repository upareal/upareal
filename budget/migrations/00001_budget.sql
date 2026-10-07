-- story: e05s01
-- Intent only: categories in one level of groups (B1). A category's kind is its
-- group's kind (B2), so it is not stored twice. Names are sealed with the budget's
-- own per-user data key. user_id references users by value: no cross-module FKs.

-- +goose Up
CREATE TABLE budget_user_key (
    user_id    TEXT PRIMARY KEY,
    data_key   BLOB NOT NULL,            -- wrapped per-user data key (budget's own)
    created_at TEXT NOT NULL
);

CREATE TABLE budget_group (
    id          TEXT PRIMARY KEY,        -- UUIDv7
    user_id     TEXT NOT NULL,
    kind        TEXT NOT NULL CHECK (kind IN ('expense', 'income')),
    name        BLOB NOT NULL,           -- sealed, field "name"
    position    INTEGER NOT NULL,
    archived_at TEXT,
    created_at  TEXT NOT NULL
);
CREATE INDEX budget_group_user ON budget_group (user_id, position);

CREATE TABLE budget_category (
    id          TEXT PRIMARY KEY,        -- UUIDv7
    user_id     TEXT NOT NULL,
    group_id    TEXT NOT NULL REFERENCES budget_group (id),
    name        BLOB NOT NULL,           -- sealed, field "name"
    position    INTEGER NOT NULL,
    archived_at TEXT,
    created_at  TEXT NOT NULL
);
CREATE INDEX budget_category_group ON budget_category (group_id, position);
CREATE INDEX budget_category_user ON budget_category (user_id);

-- +goose Down
DROP TABLE budget_category;
DROP TABLE budget_group;
DROP TABLE budget_user_key;
