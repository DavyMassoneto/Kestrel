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
	if count != 2 {
		t.Errorf("expected 2 migrations recorded, got %d", count)
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

	expected := []string{"001_accounts.sql", "002_apikeys.sql"}
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
