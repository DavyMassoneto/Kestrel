CREATE TABLE IF NOT EXISTS request_log (
    id              TEXT PRIMARY KEY,
    api_key_id      TEXT NOT NULL,
    api_key_name    TEXT,
    account_id      TEXT,
    account_name    TEXT,
    model           TEXT NOT NULL,
    status          INTEGER NOT NULL,
    input_tokens    INTEGER,
    output_tokens   INTEGER,
    latency_ms      INTEGER,
    retries         INTEGER DEFAULT 0,
    error           TEXT,
    stream          BOOLEAN NOT NULL,
    created_at      TEXT NOT NULL,

    FOREIGN KEY (api_key_id) REFERENCES api_keys(id),
    FOREIGN KEY (account_id) REFERENCES accounts(id)
);
CREATE INDEX IF NOT EXISTS idx_request_log_created ON request_log(created_at);
CREATE INDEX IF NOT EXISTS idx_request_log_account ON request_log(account_id);
CREATE INDEX IF NOT EXISTS idx_request_log_apikey ON request_log(api_key_id);
CREATE INDEX IF NOT EXISTS idx_request_log_status ON request_log(status);
