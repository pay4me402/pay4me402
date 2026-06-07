CREATE TABLE IF NOT EXISTS user_wallet (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    wallet_id TEXT NOT NULL,
    limit_amount REAL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES proxy_users(id) ON DELETE CASCADE,
    FOREIGN KEY (wallet_id) REFERENCES wallets(id) ON DELETE CASCADE,
    UNIQUE (user_id, wallet_id)
);

CREATE INDEX IF NOT EXISTS idx_user_wallet_user_id ON user_wallet(user_id);
CREATE INDEX IF NOT EXISTS idx_user_wallet_wallet_id ON user_wallet(wallet_id);
