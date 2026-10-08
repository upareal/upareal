-- story: e03s01 e03s02

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


-- name: GetSessionWithUser :one
SELECT sqlc.embed(s), sqlc.embed(u) FROM users_session s
JOIN users_user u ON u.id = s.user_id
WHERE s.token_hash = ?;

-- name: TouchSession :exec
UPDATE users_session SET last_seen_at = ? WHERE token_hash = ?;

-- name: DeleteSession :exec
DELETE FROM users_session WHERE token_hash = ?;

-- name: DeleteUserSessions :exec
DELETE FROM users_session WHERE user_id = ?;

-- name: DeleteExpiredUserSessions :exec
DELETE FROM users_session
WHERE user_id = sqlc.arg(user_id)
  AND (last_seen_at < sqlc.arg(idle_cutoff) OR created_at < sqlc.arg(absolute_cutoff));

-- name: AcceptTerms :execrows
UPDATE users_user SET terms_version = ?, terms_accepted_at = ? WHERE id = ?;

-- story: e09s01
-- name: ListUsers :many
SELECT sqlc.embed(u),
       CAST(COALESCE((SELECT MAX(s.last_seen_at) FROM users_session s WHERE s.user_id = u.id), '') AS TEXT) AS last_seen_at
FROM users_user u
ORDER BY u.created_at, u.id;

-- story: e09s02
-- name: GetUser :one
SELECT * FROM users_user WHERE id = ?;

-- name: SetUserDisabled :exec
UPDATE users_user SET disabled_at = ? WHERE id = ?;

-- name: DeleteUser :exec
DELETE FROM users_user WHERE id = ?;
