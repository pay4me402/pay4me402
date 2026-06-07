CREATE TABLE IF NOT EXISTS wallets (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    chain TEXT NOT NULL CHECK (chain IN ('algorand', 'solana')),
    private_key TEXT NOT NULL,
    rpc_endpoint TEXT NOT NULL,
    rpc_token TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
