package sqlite_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/middleware"
	"github.com/DavyMassoneto/Kestrel/internal/adapter/sqlite"
)

func setupLogRepo(t *testing.T) *sqlite.RequestLogRepo {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sqlite.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Disable FK checks — we test the log repo in isolation.
	db.Writer().Exec("PRAGMA foreign_keys=OFF")
	db.Reader().Exec("PRAGMA foreign_keys=OFF")

	if err := sqlite.RunMigrations(db.Writer()); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	return sqlite.NewRequestLogRepo(db)
}

func makeLogEntry(id string, status int) middleware.RequestLogEntry {
	return middleware.RequestLogEntry{
		RequestID:    id,
		APIKeyID:     "key-1",
		APIKeyName:   "test-key",
		AccountID:    "acc-1",
		AccountName:  "test-account",
		Model:        "claude-sonnet-4-20250514",
		Status:       status,
		InputTokens:  100,
		OutputTokens: 50,
		LatencyMs:    250,
		Retries:      0,
		Error:        "",
		Stream:       false,
		CreatedAt:    "2026-03-17T10:00:00Z",
	}
}

func TestRequestLogRepo_LogAndFindAll(t *testing.T) {
	repo := setupLogRepo(t)
	ctx := context.Background()

	entry := makeLogEntry("req-1", 200)
	if err := repo.LogRequest(ctx, entry); err != nil {
		t.Fatalf("LogRequest: %v", err)
	}

	entries, total, err := repo.FindAll(ctx, sqlite.RequestLogFilters{})
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d; want 1", total)
	}
	if len(entries) != 1 {
		t.Fatalf("entries len = %d; want 1", len(entries))
	}

	got := entries[0]
	if got.RequestID != "req-1" {
		t.Errorf("RequestID = %q; want %q", got.RequestID, "req-1")
	}
	if got.APIKeyID != "key-1" {
		t.Errorf("APIKeyID = %q; want %q", got.APIKeyID, "key-1")
	}
	if got.APIKeyName != "test-key" {
		t.Errorf("APIKeyName = %q; want %q", got.APIKeyName, "test-key")
	}
	if got.AccountID != "acc-1" {
		t.Errorf("AccountID = %q; want %q", got.AccountID, "acc-1")
	}
	if got.AccountName != "test-account" {
		t.Errorf("AccountName = %q; want %q", got.AccountName, "test-account")
	}
	if got.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %q; want %q", got.Model, "claude-sonnet-4-20250514")
	}
	if got.Status != 200 {
		t.Errorf("Status = %d; want 200", got.Status)
	}
	if got.InputTokens != 100 {
		t.Errorf("InputTokens = %d; want 100", got.InputTokens)
	}
	if got.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d; want 50", got.OutputTokens)
	}
	if got.LatencyMs != 250 {
		t.Errorf("LatencyMs = %d; want 250", got.LatencyMs)
	}
	if got.Stream != false {
		t.Errorf("Stream = %v; want false", got.Stream)
	}
}

func TestRequestLogRepo_LogRequest_NullableFields(t *testing.T) {
	repo := setupLogRepo(t)
	ctx := context.Background()

	entry := middleware.RequestLogEntry{
		RequestID: "req-nullable",
		APIKeyID:  "key-1",
		Model:     "claude-sonnet-4-20250514",
		Status:    500,
		Stream:    true,
		CreatedAt: "2026-03-17T10:00:00Z",
		Error:     "provider error",
	}

	if err := repo.LogRequest(ctx, entry); err != nil {
		t.Fatalf("LogRequest: %v", err)
	}

	entries, total, err := repo.FindAll(ctx, sqlite.RequestLogFilters{})
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d; want 1", total)
	}

	got := entries[0]
	if got.AccountID != "" {
		t.Errorf("AccountID = %q; want empty", got.AccountID)
	}
	if got.AccountName != "" {
		t.Errorf("AccountName = %q; want empty", got.AccountName)
	}
	if got.APIKeyName != "" {
		t.Errorf("APIKeyName = %q; want empty", got.APIKeyName)
	}
	if got.Error != "provider error" {
		t.Errorf("Error = %q; want %q", got.Error, "provider error")
	}
	if got.Stream != true {
		t.Errorf("Stream = %v; want true", got.Stream)
	}
}

func TestRequestLogRepo_FilterByStatus(t *testing.T) {
	repo := setupLogRepo(t)
	ctx := context.Background()

	repo.LogRequest(ctx, makeLogEntry("req-ok", 200))

	errEntry := makeLogEntry("req-err", 500)
	errEntry.CreatedAt = "2026-03-17T10:01:00Z"
	repo.LogRequest(ctx, errEntry)

	status := 500
	entries, total, err := repo.FindAll(ctx, sqlite.RequestLogFilters{Status: &status})
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d; want 1", total)
	}
	if len(entries) != 1 {
		t.Fatalf("entries len = %d; want 1", len(entries))
	}
	if entries[0].RequestID != "req-err" {
		t.Errorf("RequestID = %q; want %q", entries[0].RequestID, "req-err")
	}
}

func TestRequestLogRepo_FilterByAccountID(t *testing.T) {
	repo := setupLogRepo(t)
	ctx := context.Background()

	e1 := makeLogEntry("req-a1", 200)
	e1.AccountID = "acc-1"
	repo.LogRequest(ctx, e1)

	e2 := makeLogEntry("req-a2", 200)
	e2.RequestID = "req-a2"
	e2.AccountID = "acc-2"
	e2.CreatedAt = "2026-03-17T10:01:00Z"
	repo.LogRequest(ctx, e2)

	accID := "acc-2"
	entries, total, err := repo.FindAll(ctx, sqlite.RequestLogFilters{AccountID: &accID})
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d; want 1", total)
	}
	if entries[0].RequestID != "req-a2" {
		t.Errorf("RequestID = %q; want %q", entries[0].RequestID, "req-a2")
	}
}

func TestRequestLogRepo_FilterByAPIKeyID(t *testing.T) {
	repo := setupLogRepo(t)
	ctx := context.Background()

	e1 := makeLogEntry("req-k1", 200)
	e1.APIKeyID = "key-1"
	repo.LogRequest(ctx, e1)

	e2 := makeLogEntry("req-k2", 200)
	e2.APIKeyID = "key-2"
	e2.CreatedAt = "2026-03-17T10:01:00Z"
	repo.LogRequest(ctx, e2)

	keyID := "key-1"
	entries, total, err := repo.FindAll(ctx, sqlite.RequestLogFilters{APIKeyID: &keyID})
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d; want 1", total)
	}
	if entries[0].RequestID != "req-k1" {
		t.Errorf("RequestID = %q; want %q", entries[0].RequestID, "req-k1")
	}
}

func TestRequestLogRepo_FilterByModel(t *testing.T) {
	repo := setupLogRepo(t)
	ctx := context.Background()

	e1 := makeLogEntry("req-m1", 200)
	e1.Model = "claude-sonnet-4-20250514"
	repo.LogRequest(ctx, e1)

	e2 := makeLogEntry("req-m2", 200)
	e2.Model = "claude-opus-4-20250514"
	e2.CreatedAt = "2026-03-17T10:01:00Z"
	repo.LogRequest(ctx, e2)

	model := "claude-opus-4-20250514"
	entries, total, err := repo.FindAll(ctx, sqlite.RequestLogFilters{Model: &model})
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d; want 1", total)
	}
	if entries[0].RequestID != "req-m2" {
		t.Errorf("RequestID = %q; want %q", entries[0].RequestID, "req-m2")
	}
}

func TestRequestLogRepo_FilterByDateRange(t *testing.T) {
	repo := setupLogRepo(t)
	ctx := context.Background()

	e1 := makeLogEntry("req-d1", 200)
	e1.CreatedAt = "2026-03-15T10:00:00Z"
	repo.LogRequest(ctx, e1)

	e2 := makeLogEntry("req-d2", 200)
	e2.CreatedAt = "2026-03-17T10:00:00Z"
	repo.LogRequest(ctx, e2)

	e3 := makeLogEntry("req-d3", 200)
	e3.CreatedAt = "2026-03-19T10:00:00Z"
	repo.LogRequest(ctx, e3)

	from := time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC)
	entries, total, err := repo.FindAll(ctx, sqlite.RequestLogFilters{From: &from, To: &to})
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d; want 1", total)
	}
	if entries[0].RequestID != "req-d2" {
		t.Errorf("RequestID = %q; want %q", entries[0].RequestID, "req-d2")
	}
}

func TestRequestLogRepo_Pagination(t *testing.T) {
	repo := setupLogRepo(t)
	ctx := context.Background()

	// Insert 5 entries
	for i := range 5 {
		e := makeLogEntry("req-p"+string(rune('0'+i)), 200)
		e.RequestID = fmt.Sprintf("req-p%d", i)
		e.CreatedAt = fmt.Sprintf("2026-03-17T10:%02d:00Z", i)
		repo.LogRequest(ctx, e)
	}

	// Page 1: limit=2, offset=0
	entries, total, err := repo.FindAll(ctx, sqlite.RequestLogFilters{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("FindAll page 1: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d; want 5", total)
	}
	if len(entries) != 2 {
		t.Fatalf("entries len = %d; want 2", len(entries))
	}

	// Page 2: limit=2, offset=2
	entries2, total2, err := repo.FindAll(ctx, sqlite.RequestLogFilters{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("FindAll page 2: %v", err)
	}
	if total2 != 5 {
		t.Errorf("total = %d; want 5", total2)
	}
	if len(entries2) != 2 {
		t.Fatalf("entries len = %d; want 2", len(entries2))
	}

	// Page 3: limit=2, offset=4
	entries3, _, err := repo.FindAll(ctx, sqlite.RequestLogFilters{Limit: 2, Offset: 4})
	if err != nil {
		t.Fatalf("FindAll page 3: %v", err)
	}
	if len(entries3) != 1 {
		t.Errorf("entries len = %d; want 1", len(entries3))
	}
}

func TestRequestLogRepo_OrderByCreatedAtDesc(t *testing.T) {
	repo := setupLogRepo(t)
	ctx := context.Background()

	e1 := makeLogEntry("req-old", 200)
	e1.CreatedAt = "2026-03-17T09:00:00Z"
	repo.LogRequest(ctx, e1)

	e2 := makeLogEntry("req-new", 200)
	e2.CreatedAt = "2026-03-17T11:00:00Z"
	repo.LogRequest(ctx, e2)

	entries, _, err := repo.FindAll(ctx, sqlite.RequestLogFilters{})
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries len = %d; want 2", len(entries))
	}
	// Newest first
	if entries[0].RequestID != "req-new" {
		t.Errorf("first entry = %q; want %q", entries[0].RequestID, "req-new")
	}
	if entries[1].RequestID != "req-old" {
		t.Errorf("second entry = %q; want %q", entries[1].RequestID, "req-old")
	}
}

func TestRequestLogRepo_DefaultLimit(t *testing.T) {
	repo := setupLogRepo(t)
	ctx := context.Background()

	// Insert 60 entries
	for i := range 60 {
		e := makeLogEntry(fmt.Sprintf("req-dl%d", i), 200)
		e.CreatedAt = fmt.Sprintf("2026-03-17T%02d:%02d:00Z", i/60, i%60)
		repo.LogRequest(ctx, e)
	}

	// Default limit=0 should return 50
	entries, total, err := repo.FindAll(ctx, sqlite.RequestLogFilters{})
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if total != 60 {
		t.Errorf("total = %d; want 60", total)
	}
	if len(entries) != 50 {
		t.Errorf("entries len = %d; want 50 (default limit)", len(entries))
	}
}

func TestRequestLogRepo_MaxLimit(t *testing.T) {
	repo := setupLogRepo(t)
	ctx := context.Background()

	e := makeLogEntry("req-max", 200)
	repo.LogRequest(ctx, e)

	// Limit > 500 should be capped at 500
	entries, _, err := repo.FindAll(ctx, sqlite.RequestLogFilters{Limit: 1000})
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	// Only 1 entry exists, so we just verify it doesn't error
	if len(entries) != 1 {
		t.Errorf("entries len = %d; want 1", len(entries))
	}
}

func TestRequestLogRepo_ImplementsRequestLogger(t *testing.T) {
	repo := setupLogRepo(t)

	// Compile-time check that RequestLogRepo implements middleware.RequestLogger
	var _ middleware.RequestLogger = repo
}

func TestRequestLogRepo_LogRequest_ClosedDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sqlite.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	db.Writer().Exec("PRAGMA foreign_keys=OFF")
	if err := sqlite.RunMigrations(db.Writer()); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	repo := sqlite.NewRequestLogRepo(db)
	db.Close()

	entry := makeLogEntry("req-closed", 200)
	err = repo.LogRequest(context.Background(), entry)
	if err == nil {
		t.Fatal("expected error for closed DB on LogRequest")
	}
}

func TestRequestLogRepo_FindAll_ClosedDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sqlite.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	db.Writer().Exec("PRAGMA foreign_keys=OFF")
	if err := sqlite.RunMigrations(db.Writer()); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	repo := sqlite.NewRequestLogRepo(db)
	db.Close()

	_, _, err = repo.FindAll(context.Background(), sqlite.RequestLogFilters{})
	if err == nil {
		t.Fatal("expected error for closed DB on FindAll")
	}
}

func TestRequestLogRepo_MultipleFilters(t *testing.T) {
	repo := setupLogRepo(t)
	ctx := context.Background()

	e1 := makeLogEntry("req-mf1", 200)
	e1.AccountID = "acc-1"
	e1.Model = "claude-sonnet-4-20250514"
	repo.LogRequest(ctx, e1)

	e2 := makeLogEntry("req-mf2", 500)
	e2.AccountID = "acc-1"
	e2.Model = "claude-sonnet-4-20250514"
	e2.CreatedAt = "2026-03-17T10:01:00Z"
	repo.LogRequest(ctx, e2)

	e3 := makeLogEntry("req-mf3", 200)
	e3.AccountID = "acc-2"
	e3.Model = "claude-sonnet-4-20250514"
	e3.CreatedAt = "2026-03-17T10:02:00Z"
	repo.LogRequest(ctx, e3)

	// Filter: status=200 AND account_id=acc-1
	status := 200
	accID := "acc-1"
	entries, total, err := repo.FindAll(ctx, sqlite.RequestLogFilters{
		Status:    &status,
		AccountID: &accID,
	})
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d; want 1", total)
	}
	if len(entries) != 1 {
		t.Fatalf("entries len = %d; want 1", len(entries))
	}
	if entries[0].RequestID != "req-mf1" {
		t.Errorf("RequestID = %q; want %q", entries[0].RequestID, "req-mf1")
	}
}
