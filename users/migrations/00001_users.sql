-- story: e03s01
-- Personal fields are sealed by internal/vault (BLOB); the Google sub is stored only as a blind index.

-- +goose Up
CREATE TABLE users_user (
    id         TEXT PRIMARY KEY,              -- UUIDv7
    sub_idx    BLOB NOT NULL UNIQUE,          -- vault.BlindIndex("google-sub", sub)
    data_key   BLOB NOT NULL,                 -- wrapped per-user data key
    email      BLOB NOT NULL,                 -- sealed, field "email"
    name       BLOB NOT NULL,                 -- sealed, field "name"
    created_at TEXT NOT NULL                  -- UTC RFC 3339
);

CREATE TABLE users_session (
    token_hash   BLOB PRIMARY KEY,            -- SHA-256 of the cookie token
    user_id      TEXT NOT NULL REFERENCES users_user (id) ON DELETE CASCADE,
    created_at   TEXT NOT NULL,
    last_seen_at TEXT NOT NULL
);
CREATE INDEX users_session_user_id ON users_session (user_id);

-- +goose Down
DROP TABLE users_session;
DROP TABLE users_user;
