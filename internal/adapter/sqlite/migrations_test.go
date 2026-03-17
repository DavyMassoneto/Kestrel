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
	if count != 3 {
		t.Errorf("expected 3 migrations recorded, got %d", count)
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

	expected := []string{"001_accounts.sql", "002_apikeys.sql", "003_request_log.sql"}
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

func TestRunMigrations_InvalidMigrationSQL(t *testing.T) {
	db := tempDB(t)

	// First, run normal migrations
	if err := sqlite.RunMigrations(db); err != nil {
		t.Fatalf("first run: %v", err)
	}

	// Clear the schema_migrations to re-run, but corrupt the accounts table
	// so that re-running migration would fail at exec stage
	_, err := db.Exec(`DELETE FROM schema_migrations`)
	if err != nil {
		t.Fatalf("delete migrations: %v", err)
	}

	// The 001_accounts.sql uses CREATE TABLE IF NOT EXISTS, so it won't fail.
	// This test verifies idempotent behavior — migration SQL should still succeed.
	err = sqlite.RunMigrations(db)
	if err != nil {
		t.Fatalf("re-run after clearing versions should succeed: %v", err)
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
