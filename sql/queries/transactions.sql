-- name: CreateTransaction :one
INSERT INTO transactions (id, user_id, wallet_id, resource, amount, status)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id, user_id, wallet_id, resource, amount, status, created_at;

-- name: GetTransaction :one
SELECT id, user_id, wallet_id, resource, amount, status, created_at
FROM transactions
WHERE id = ?;

-- name: ListTransactions :many
SELECT id, user_id, wallet_id, resource, amount, status, created_at
FROM transactions
ORDER BY created_at DESC;

-- name: ListTransactionsByUser :many
SELECT id, user_id, wallet_id, resource, amount, status, created_at
FROM transactions
WHERE user_id = ?
ORDER BY created_at DESC;

-- name: ListTransactionsByWallet :many
SELECT id, user_id, wallet_id, resource, amount, status, created_at
FROM transactions
WHERE wallet_id = ?
ORDER BY created_at DESC;

-- name: DeleteTransaction :exec
DELETE FROM transactions
WHERE id = ?;
