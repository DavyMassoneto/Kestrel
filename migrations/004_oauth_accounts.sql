ALTER TABLE accounts ADD COLUMN auth_type TEXT NOT NULL DEFAULT 'api_key';
ALTER TABLE accounts ADD COLUMN access_token TEXT;
ALTER TABLE accounts ADD COLUMN refresh_token TEXT;
ALTER TABLE accounts ADD COLUMN token_expires_at TEXT;
ALTER TABLE accounts ADD COLUMN oauth_email TEXT;
ALTER TABLE accounts ADD COLUMN oauth_scope TEXT;
