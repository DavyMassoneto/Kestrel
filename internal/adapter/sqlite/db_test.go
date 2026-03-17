package sqlite_test

import (
	"path/filepath"
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/sqlite"
)

func TestNewDB_Success(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sqlite.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	if db.Writer() == nil {
		t.Error("Writer should not be nil")
	}
	if db.Reader() == nil {
		t.Error("Reader should not be nil")
	}
}

func TestNewDB_WriterHasSingleConn(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sqlite.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	// Verify writer works
	_, err = db.Writer().Exec("CREATE TABLE test (id INTEGER)")
	if err != nil {
		t.Fatalf("Writer exec: %v", err)
	}
}

func TestNewDB_WALMode(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sqlite.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	var mode string
	err = db.Writer().QueryRow("PRAGMA journal_mode").Scan(&mode)
	if err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q; want %q", mode, "wal")
	}
}

func TestNewDB_ForeignKeysEnabled(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sqlite.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	var fk int
	err = db.Writer().QueryRow("PRAGMA foreign_keys").Scan(&fk)
	if err != nil {
		t.Fatalf("query foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d; want 1", fk)
	}
}

func TestNewDB_Close(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sqlite.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// After close, operations should fail
	_, err = db.Writer().Exec("SELECT 1")
	if err == nil {
		t.Error("expected error after Close")
	}
}

func TestNewDB_ReaderCanRead(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sqlite.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	// Write with writer
	_, err = db.Writer().Exec("CREATE TABLE test (id INTEGER)")
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	_, err = db.Writer().Exec("INSERT INTO test (id) VALUES (42)")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Read with reader
	var id int
	err = db.Reader().QueryRow("SELECT id FROM test").Scan(&id)
	if err != nil {
		t.Fatalf("reader query: %v", err)
	}
	if id != 42 {
		t.Errorf("id = %d; want 42", id)
	}
}
