-- name: CreateUserWallet :one
INSERT INTO user_wallet (id, user_id, wallet_id, monthly_budget)
VALUES (?, ?, ?, ?)
RETURNING id, user_id, wallet_id, monthly_budget, created_at, updated_at;

-- name: ListUserWallets :many
SELECT id, user_id, wallet_id, monthly_budget, created_at, updated_at
FROM user_wallet
ORDER BY created_at DESC;

-- name: ListUserWalletsByUser :many
SELECT id, user_id, wallet_id, monthly_budget, created_at, updated_at
FROM user_wallet
WHERE user_id = ?
ORDER BY created_at DESC;

-- name: ListUserWalletsByWallet :many
SELECT id, user_id, wallet_id, monthly_budget, created_at, updated_at
FROM user_wallet
WHERE wallet_id = ?
ORDER BY created_at DESC;

-- name: GetUserWallet :one
SELECT id, user_id, wallet_id, monthly_budget, created_at, updated_at
FROM user_wallet
WHERE id = ?;

-- name: GetUserWalletByUserAndWallet :one
SELECT id, user_id, wallet_id, monthly_budget, created_at, updated_at
FROM user_wallet
WHERE user_id = ? AND wallet_id = ?;

-- name: UpdateUserWalletMonthlyBudget :one
UPDATE user_wallet
SET monthly_budget = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING id, user_id, wallet_id, monthly_budget, created_at, updated_at;

-- name: DeleteUserWallet :exec
DELETE FROM user_wallet
WHERE id = ?;
