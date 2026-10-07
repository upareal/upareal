-- story: e03s03
-- Latest accepted terms version and when (LGPD consent evidence). Not personal text: plaintext.

-- +goose Up
ALTER TABLE users_user ADD COLUMN terms_version TEXT;
ALTER TABLE users_user ADD COLUMN terms_accepted_at TEXT;

-- +goose Down
ALTER TABLE users_user DROP COLUMN terms_accepted_at;
ALTER TABLE users_user DROP COLUMN terms_version;
