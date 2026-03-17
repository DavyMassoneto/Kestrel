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
	writer, err := openConn(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open writer: %w", err)
	}
	writer.SetMaxOpenConns(1)

	reader, err := openConn(dbPath)
	if err != nil {
		writer.Close()
		return nil, fmt.Errorf("open reader: %w", err)
	}

	// Configure WAL and pragmas on writer
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := writer.Exec(pragma); err != nil {
			writer.Close()
			reader.Close()
			return nil, fmt.Errorf("pragma %q: %w", pragma, err)
		}
	}

	// Configure pragmas on reader
	for _, pragma := range []string{
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := reader.Exec(pragma); err != nil {
			writer.Close()
			reader.Close()
			return nil, fmt.Errorf("reader pragma %q: %w", pragma, err)
		}
	}

	return &DB{writer: writer, reader: reader}, nil
}

// Writer returns the write connection (serialized, 1 conn).
func (db *DB) Writer() *sql.DB { return db.writer }

// Reader returns the reader connection pool.
func (db *DB) Reader() *sql.DB { return db.reader }

// Close closes both writer and reader connections.
func (db *DB) Close() error {
	wErr := db.writer.Close()
	rErr := db.reader.Close()
	if wErr != nil {
		return wErr
	}
	return rErr
}

func openConn(dbPath string) (*sql.DB, error) {
	return sql.Open("sqlite", dbPath)
}
