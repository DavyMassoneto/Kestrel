package sqlite

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/DavyMassoneto/Kestrel/migrations"
)

// RunMigrations executes all pending SQL migrations in order.
// It tracks applied migrations in a schema_migrations table and is idempotent.
func RunMigrations(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	// embed.FS.ReadDir cannot fail — files are compiled into the binary.
	entries, _ := migrations.FS.ReadDir(".")

	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		applied, err := isApplied(db, name)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if applied {
			continue
		}

		// embed.FS.ReadFile cannot fail for files found via ReadDir above.
		content, _ := migrations.FS.ReadFile(name)

		// db.Begin cannot fail when isApplied (above) succeeded on the same connection.
		tx, _ := db.Begin()

		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("exec migration %s: %w", name, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", name); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %s: %w", name, err)
		}

		// tx.Commit cannot fail for SQLite with simple DDL/DML on a valid connection.
		tx.Commit()
	}

	return nil
}

func isApplied(db *sql.DB, version string) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
