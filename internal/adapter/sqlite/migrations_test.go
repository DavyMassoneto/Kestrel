package sqlite_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/sqlite"
)

func tempDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestRunMigrations_CreatesTablesAndIndexes(t *testing.T) {
	db := tempDB(t)

	if err := sqlite.RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	// Verify accounts table
	var name string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='accounts'").Scan(&name)
	if err != nil {
		t.Fatalf("accounts table not found: %v", err)
	}

	// Verify api_keys table
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='api_keys'").Scan(&name)
	if err != nil {
		t.Fatalf("api_keys table not found: %v", err)
	}

	// Verify request_log table
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='request_log'").Scan(&name)
	if err != nil {
		t.Fatalf("request_log table not found: %v", err)
	}

	// Verify schema_migrations table
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='schema_migrations'").Scan(&name)
	if err != nil {
		t.Fatalf("schema_migrations table not found: %v", err)
	}

	// Verify indexes
	for _, idx := range []string{
		"idx_accounts_status",
		"idx_accounts_priority",
		"idx_api_keys_prefix",
		"idx_api_keys_active",
		"idx_request_log_created",
		"idx_request_log_account",
		"idx_request_log_apikey",
		"idx_request_log_status",
	} {
		err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name=?", idx).Scan(&name)
		if err != nil {
			t.Errorf("index %s not found: %v", idx, err)
		}
	}
}

func TestRunMigrations_OAuthColumnsExist(t *testing.T) {
	db := tempDB(t)

	if err := sqlite.RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	// Verify OAuth columns exist by inserting a row with all columns
	_, err := db.Exec(`INSERT INTO accounts (id, name, api_key, base_url, status, priority, created_at, updated_at, auth_type, access_token, refresh_token, token_expires_at, oauth_email, oauth_scope)
		VALUES ('acc-1', 'test', 'key', 'https://api.anthropic.com', 'active', 0, datetime('now'), datetime('now'), 'oauth', 'access-tok', 'refresh-tok', '2026-01-01T00:00:00Z', 'user@test.com', 'org:read')`)
	if err != nil {
		t.Fatalf("insert with OAuth columns: %v", err)
	}

	// Verify auth_type default is 'api_key'
	_, err = db.Exec(`INSERT INTO accounts (id, name, api_key, base_url, created_at, updated_at)
		VALUES ('acc-2', 'default-auth', 'key2', 'https://api.anthropic.com', datetime('now'), datetime('now'))`)
	if err != nil {
		t.Fatalf("insert with default auth_type: %v", err)
	}

	var authType string
	err = db.QueryRow("SELECT auth_type FROM accounts WHERE id = 'acc-2'").Scan(&authType)
	if err != nil {
		t.Fatalf("query auth_type: %v", err)
	}
	if authType != "api_key" {
		t.Errorf("default auth_type = %q; want api_key", authType)
	}

	// Verify nullable OAuth columns default to NULL
	var accessToken, refreshToken, tokenExpiresAt, email, scope *string
	err = db.QueryRow("SELECT access_token, refresh_token, token_expires_at, oauth_email, oauth_scope FROM accounts WHERE id = 'acc-2'").
		Scan(&accessToken, &refreshToken, &tokenExpiresAt, &email, &scope)
	if err != nil {
		t.Fatalf("query nullable columns: %v", err)
	}
	if accessToken != nil || refreshToken != nil || tokenExpiresAt != nil || email != nil || scope != nil {
		t.Error("nullable OAuth columns should default to NULL")
	}
}

func TestRunMigrations_Idempotent(t *testing.T) {
	db := tempDB(t)

	if err := sqlite.RunMigrations(db); err != nil {
		t.Fatalf("first run: %v", err)
	}

	if err := sqlite.RunMigrations(db); err != nil {
		t.Fatalf("second run should be idempotent: %v", err)
	}

	// Verify migrations recorded
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if count != 4 {
		t.Errorf("expected 4 migrations recorded, got %d", count)
	}
}

func TestRunMigrations_TracksVersions(t *testing.T) {
	db := tempDB(t)

	if err := sqlite.RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	rows, err := db.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		t.Fatalf("query versions: %v", err)
	}
	defer rows.Close()

	expected := []string{"001_accounts.sql", "002_apikeys.sql", "003_request_log.sql", "004_oauth_accounts.sql"}
	var got []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, v)
	}

	if len(got) != len(expected) {
		t.Fatalf("expected %d versions, got %d: %v", len(expected), len(got), got)
	}
	for i, v := range expected {
		if got[i] != v {
			t.Errorf("version[%d] = %q; want %q", i, got[i], v)
		}
	}
}

func TestRunMigrations_ClosedDB(t *testing.T) {
	db := tempDB(t)
	db.Close()

	err := sqlite.RunMigrations(db)
	if err == nil {
		t.Error("expected error for closed database")
	}
}

func TestRunMigrations_CorruptedMigrationsTable(t *testing.T) {
	db := tempDB(t)

	// Create schema_migrations with incompatible schema (no version column)
	_, err := db.Exec(`CREATE TABLE schema_migrations (bad_col TEXT)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	// RunMigrations should fail when querying isApplied because the
	// SELECT COUNT(*) FROM schema_migrations WHERE version = ? will fail
	// since version column doesn't exist.
	err = sqlite.RunMigrations(db)
	if err == nil {
		t.Fatal("expected error for corrupted schema_migrations table")
	}
}

func TestRunMigrations_RerunAfterClearFails(t *testing.T) {
	db := tempDB(t)

	// First, run normal migrations
	if err := sqlite.RunMigrations(db); err != nil {
		t.Fatalf("first run: %v", err)
	}

	// Clear the schema_migrations to simulate version tracking loss
	_, err := db.Exec(`DELETE FROM schema_migrations`)
	if err != nil {
		t.Fatalf("delete migrations: %v", err)
	}

	// Re-running should fail because ALTER TABLE ADD COLUMN (migration 004)
	// cannot be re-applied when columns already exist.
	// This is expected — the migration system relies on schema_migrations
	// tracking to prevent re-execution.
	err = sqlite.RunMigrations(db)
	if err == nil {
		t.Fatal("expected error when re-running migrations with ALTER TABLE columns already present")
	}
}

func TestRunMigrations_ExecMigrationFails(t *testing.T) {
	db := tempDB(t)

	// Create schema_migrations with correct schema so isApplied works
	db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`)

	// Create a VIEW called 'accounts' — CREATE TABLE IF NOT EXISTS fails
	// when a VIEW with the same name already exists.
	db.Exec(`CREATE VIEW accounts AS SELECT 1 AS id`)

	err := sqlite.RunMigrations(db)
	if err == nil {
		t.Fatal("expected error when migration exec fails due to VIEW conflict")
	}
}

func TestRunMigrations_InsertVersionFails(t *testing.T) {
	db := tempDB(t)

	// Run migrations normally first so tables exist
	if err := sqlite.RunMigrations(db); err != nil {
		t.Fatalf("first run: %v", err)
	}

	// Clear versions but make schema_migrations read-only by replacing it
	// with a VIEW that rejects INSERT
	db.Exec(`DELETE FROM schema_migrations`)
	db.Exec(`ALTER TABLE schema_migrations RENAME TO schema_migrations_old`)
	db.Exec(`CREATE VIEW schema_migrations AS SELECT version, applied_at FROM schema_migrations_old`)

	// isApplied query works (SELECT from VIEW), but INSERT will fail
	err := sqlite.RunMigrations(db)
	if err == nil {
		t.Fatal("expected error when INSERT INTO schema_migrations fails (VIEW is read-only)")
	}
}
