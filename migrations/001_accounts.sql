CREATE TABLE IF NOT EXISTS accounts (
    id                   TEXT PRIMARY KEY,
    name                 TEXT NOT NULL,
    api_key              TEXT NOT NULL,
    base_url             TEXT NOT NULL DEFAULT 'https://api.anthropic.com',
    status               TEXT NOT NULL DEFAULT 'active',
    priority             INTEGER NOT NULL DEFAULT 0,
    cooldown_until       TEXT,
    backoff_level        INTEGER NOT NULL DEFAULT 0,
    last_used_at         TEXT,
    last_error           TEXT,
    error_classification TEXT,
    created_at           TEXT NOT NULL,
    updated_at           TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_accounts_status ON accounts(status);
CREATE INDEX IF NOT EXISTS idx_accounts_priority ON accounts(priority);
