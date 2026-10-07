-- story: e03s01

-- name: CreateUser :exec
INSERT INTO users_user (id, sub_idx, data_key, email, name, created_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetUserBySubIdx :one
SELECT * FROM users_user WHERE sub_idx = ?;

-- name: UpdateUserIdentity :exec
UPDATE users_user SET email = ?, name = ? WHERE id = ?;

-- name: CreateSession :exec
INSERT INTO users_session (token_hash, user_id, created_at, last_seen_at)
VALUES (?, ?, ?, ?);

-- name: GetUserBySessionHash :one
SELECT u.* FROM users_session s
JOIN users_user u ON u.id = s.user_id
WHERE s.token_hash = ?;
