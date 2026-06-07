-- name: CreateWallet :one
INSERT INTO wallets (id, name, chain, private_key, rpc_endpoint, rpc_token)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id, name, chain, private_key, rpc_endpoint, rpc_token, created_at, updated_at;

-- name: ListWallets :many
SELECT id, name, chain, private_key, rpc_endpoint, rpc_token, created_at, updated_at
FROM wallets
ORDER BY name;

-- name: GetWalletByChain :one
SELECT id, name, chain, private_key, rpc_endpoint, rpc_token, created_at, updated_at
FROM wallets
WHERE chain = ?
ORDER BY created_at
LIMIT 1;

-- name: DeleteWallet :exec
DELETE FROM wallets
WHERE id = ?;
