-- name: CreateProxyUser :one
INSERT INTO proxy_users (id, username, password_hash)
VALUES (?, ?, ?)
RETURNING id, username, password_hash, created_at, updated_at;

-- name: GetProxyUserByUsername :one
SELECT id, username, password_hash, created_at, updated_at
FROM proxy_users
WHERE username = ?;

-- name: ListProxyUsers :many
SELECT id, username, password_hash, created_at, updated_at
FROM proxy_users
ORDER BY username;

-- name: DeleteProxyUser :exec
DELETE FROM proxy_users
WHERE id = ?;
