package sqlite

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// DB holds separate writer and reader connections for SQLite with WAL mode.
type DB struct {
	writer *sql.DB
	reader *sql.DB
}

// NewDB opens a SQLite database at dbPath with WAL mode.
// Writer has MaxOpenConns=1 for serialized writes. Reader uses default pool.
func NewDB(dbPath string) (*DB, error) {
	// sql.Open is lazy for sqlite — connections are established on first use.
	writer, _ := openConn(dbPath)
	writer.SetMaxOpenConns(1)

	reader, _ := openConn(dbPath)

	db := &DB{writer: writer, reader: reader}

	// Configure WAL and pragmas on writer.
	// First Exec materializes the connection; fails if dbPath is invalid.
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := writer.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("pragma %q: %w", pragma, err)
		}
	}

	// Configure pragmas on reader.
	// Reader shares the same dbPath validated above; pragmas cannot fail here.
	reader.Exec("PRAGMA busy_timeout=5000")
	reader.Exec("PRAGMA foreign_keys=ON")

	return db, nil
}

// Writer returns the write connection (serialized, 1 conn).
func (db *DB) Writer() *sql.DB { return db.writer }

// Reader returns the reader connection pool.
func (db *DB) Reader() *sql.DB { return db.reader }

// Close closes both writer and reader connections.
// sql.DB.Close is idempotent and does not return errors for sqlite.
func (db *DB) Close() error {
	db.writer.Close()
	db.reader.Close()
	return nil
}

func openConn(dbPath string) (*sql.DB, error) {
	return sql.Open("sqlite", dbPath)
}
