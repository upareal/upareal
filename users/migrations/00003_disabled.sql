-- story: e09s02
-- A disabled user cannot sign in; their sessions are deleted when disabled.

-- +goose Up
ALTER TABLE users_user ADD COLUMN disabled_at TEXT;

-- +goose Down
ALTER TABLE users_user DROP COLUMN disabled_at;
