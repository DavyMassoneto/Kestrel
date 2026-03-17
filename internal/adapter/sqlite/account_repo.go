package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

// Encryptor abstracts encryption/decryption for API keys at rest (ISP).
type Encryptor interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

// AccountRepo implements AccountStore, AccountFinder, and AccountStatusWriter
// using SQLite with encrypted API key storage.
type AccountRepo struct {
	db        *DB
	encryptor Encryptor
}

// NewAccountRepo creates a new AccountRepo.
func NewAccountRepo(db *DB, encryptor Encryptor) *AccountRepo {
	return &AccountRepo{db: db, encryptor: encryptor}
}

// Create inserts a new account, encrypting the API key.
func (r *AccountRepo) Create(ctx context.Context, account *entity.Account) error {
	encKey, err := r.encryptor.Encrypt(account.Credentials().APIKey.Value())
	if err != nil {
		return fmt.Errorf("encrypt api key: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	var cooldownUntil, lastUsedAt, lastError, errClassification *string
	if cu := account.CooldownUntil(); cu != nil {
		s := cu.UTC().Format(time.RFC3339)
		cooldownUntil = &s
	}
	if lu := account.LastUsedAt(); lu != nil {
		s := lu.UTC().Format(time.RFC3339)
		lastUsedAt = &s
	}
	if le := account.LastError(); le != nil {
		lastError = le
	}
	if ec := account.ErrorClassification(); ec != nil {
		s := string(*ec)
		errClassification = &s
	}

	_, err = r.db.Writer().ExecContext(ctx,
		`INSERT INTO accounts (id, name, api_key, base_url, status, priority, cooldown_until, backoff_level, last_used_at, last_error, error_classification, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		account.ID().String(),
		account.Name(),
		encKey,
		account.BaseURL(),
		string(account.Status()),
		account.Priority(),
		cooldownUntil,
		account.BackoffLevel(),
		lastUsedAt,
		lastError,
		errClassification,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("insert account: %w", err)
	}
	return nil
}

// FindByID retrieves an account by ID, decrypting the API key.
func (r *AccountRepo) FindByID(ctx context.Context, id vo.AccountID) (*entity.Account, error) {
	row := r.db.Reader().QueryRowContext(ctx,
		`SELECT id, name, api_key, base_url, status, priority, cooldown_until, backoff_level, last_used_at, last_error, error_classification
		 FROM accounts WHERE id = ?`, id.String())

	return r.scanAccount(row)
}

// FindAll returns all accounts.
func (r *AccountRepo) FindAll(ctx context.Context) ([]*entity.Account, error) {
	rows, err := r.db.Reader().QueryContext(ctx,
		`SELECT id, name, api_key, base_url, status, priority, cooldown_until, backoff_level, last_used_at, last_error, error_classification
		 FROM accounts ORDER BY priority DESC, name`)
	if err != nil {
		return nil, fmt.Errorf("query accounts: %w", err)
	}
	defer rows.Close()

	return r.scanAccounts(rows)
}

// Save updates an existing account, encrypting the API key.
func (r *AccountRepo) Save(ctx context.Context, account *entity.Account) error {
	encKey, err := r.encryptor.Encrypt(account.Credentials().APIKey.Value())
	if err != nil {
		return fmt.Errorf("encrypt api key: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	var cooldownUntil, lastUsedAt, lastError, errClassification *string
	if cu := account.CooldownUntil(); cu != nil {
		s := cu.UTC().Format(time.RFC3339)
		cooldownUntil = &s
	}
	if lu := account.LastUsedAt(); lu != nil {
		s := lu.UTC().Format(time.RFC3339)
		lastUsedAt = &s
	}
	if le := account.LastError(); le != nil {
		lastError = le
	}
	if ec := account.ErrorClassification(); ec != nil {
		s := string(*ec)
		errClassification = &s
	}

	_, err = r.db.Writer().ExecContext(ctx,
		`UPDATE accounts SET name=?, api_key=?, base_url=?, status=?, priority=?, cooldown_until=?, backoff_level=?, last_used_at=?, last_error=?, error_classification=?, updated_at=?
		 WHERE id=?`,
		account.Name(),
		encKey,
		account.BaseURL(),
		string(account.Status()),
		account.Priority(),
		cooldownUntil,
		account.BackoffLevel(),
		lastUsedAt,
		lastError,
		errClassification,
		now,
		account.ID().String(),
	)
	if err != nil {
		return fmt.Errorf("update account: %w", err)
	}
	return nil
}

// Delete removes an account by ID.
func (r *AccountRepo) Delete(ctx context.Context, id vo.AccountID) error {
	_, err := r.db.Writer().ExecContext(ctx, `DELETE FROM accounts WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete account: %w", err)
	}
	return nil
}

// FindAvailable returns accounts that are active and not in cooldown.
// Optionally excludes an account by ID (for fallback rotation).
func (r *AccountRepo) FindAvailable(ctx context.Context, excludeID *vo.AccountID) ([]*entity.Account, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	query := `SELECT id, name, api_key, base_url, status, priority, cooldown_until, backoff_level, last_used_at, last_error, error_classification
		FROM accounts
		WHERE status != 'disabled'
		AND (cooldown_until IS NULL OR cooldown_until <= ?)`
	args := []any{now}

	if excludeID != nil {
		query += ` AND id != ?`
		args = append(args, excludeID.String())
	}

	query += ` ORDER BY priority DESC, last_used_at ASC NULLS FIRST`

	rows, err := r.db.Reader().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query available accounts: %w", err)
	}
	defer rows.Close()

	return r.scanAccounts(rows)
}

// UpdateStatus persists status, cooldown, backoff, and error fields.
func (r *AccountRepo) UpdateStatus(ctx context.Context, account *entity.Account) error {
	now := time.Now().UTC().Format(time.RFC3339)

	var cooldownUntil, lastError, errClassification *string
	if cu := account.CooldownUntil(); cu != nil {
		s := cu.UTC().Format(time.RFC3339)
		cooldownUntil = &s
	}
	if le := account.LastError(); le != nil {
		lastError = le
	}
	if ec := account.ErrorClassification(); ec != nil {
		s := string(*ec)
		errClassification = &s
	}

	_, err := r.db.Writer().ExecContext(ctx,
		`UPDATE accounts SET status=?, cooldown_until=?, backoff_level=?, last_error=?, error_classification=?, updated_at=?
		 WHERE id=?`,
		string(account.Status()),
		cooldownUntil,
		account.BackoffLevel(),
		lastError,
		errClassification,
		now,
		account.ID().String(),
	)
	if err != nil {
		return fmt.Errorf("update account status: %w", err)
	}
	return nil
}

// RecordSuccess clears error state and records last_used_at.
func (r *AccountRepo) RecordSuccess(ctx context.Context, accountID vo.AccountID, now time.Time) error {
	ts := now.UTC().Format(time.RFC3339)
	updated := time.Now().UTC().Format(time.RFC3339)

	_, err := r.db.Writer().ExecContext(ctx,
		`UPDATE accounts SET status='active', cooldown_until=NULL, backoff_level=0, last_error=NULL, error_classification=NULL, last_used_at=?, updated_at=?
		 WHERE id=?`,
		ts,
		updated,
		accountID.String(),
	)
	if err != nil {
		return fmt.Errorf("record success: %w", err)
	}
	return nil
}

// scanAccount scans a single row into an Account entity.
func (r *AccountRepo) scanAccount(row *sql.Row) (*entity.Account, error) {
	var (
		id, name, encKey, baseURL, status string
		priority, backoffLevel            int
		cooldownUntil, lastUsedAt         *string
		lastError, errClassification      *string
	)

	err := row.Scan(&id, &name, &encKey, &baseURL, &status, &priority, &cooldownUntil, &backoffLevel, &lastUsedAt, &lastError, &errClassification)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("account not found")
		}
		return nil, fmt.Errorf("scan account: %w", err)
	}

	return r.rehydrate(id, name, encKey, baseURL, status, priority, backoffLevel, cooldownUntil, lastUsedAt, lastError, errClassification)
}

// scanAccounts scans multiple rows into Account entities.
func (r *AccountRepo) scanAccounts(rows *sql.Rows) ([]*entity.Account, error) {
	var accounts []*entity.Account

	for rows.Next() {
		var (
			id, name, encKey, baseURL, status string
			priority, backoffLevel            int
			cooldownUntil, lastUsedAt         *string
			lastError, errClassification      *string
		)

		if err := rows.Scan(&id, &name, &encKey, &baseURL, &status, &priority, &cooldownUntil, &backoffLevel, &lastUsedAt, &lastError, &errClassification); err != nil {
			return nil, fmt.Errorf("scan account row: %w", err)
		}

		acc, err := r.rehydrate(id, name, encKey, baseURL, status, priority, backoffLevel, cooldownUntil, lastUsedAt, lastError, errClassification)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, acc)
	}

	return accounts, rows.Err()
}

func (r *AccountRepo) rehydrate(
	id, name, encKey, baseURL, status string,
	priority, backoffLevel int,
	cooldownUntil, lastUsedAt, lastError, errClassification *string,
) (*entity.Account, error) {
	decKey, err := r.encryptor.Decrypt(encKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt api key: %w", err)
	}

	accID, err := vo.ParseAccountID(id)
	if err != nil {
		return nil, fmt.Errorf("parse account id: %w", err)
	}

	var cooldown *vo.Cooldown
	if cooldownUntil != nil && errClassification != nil {
		t, err := time.Parse(time.RFC3339, *cooldownUntil)
		if err != nil {
			return nil, fmt.Errorf("parse cooldown_until: %w", err)
		}
		cd := vo.NewCooldown(t, backoffLevel, vo.ErrorClassification(*errClassification))
		cooldown = &cd
	}

	var lastUsed *time.Time
	if lastUsedAt != nil {
		t, err := time.Parse(time.RFC3339, *lastUsedAt)
		if err != nil {
			return nil, fmt.Errorf("parse last_used_at: %w", err)
		}
		lastUsed = &t
	}

	return entity.RehydrateAccount(
		accID,
		name,
		vo.NewSensitiveString(decKey),
		baseURL,
		entity.AccountStatus(status),
		priority,
		cooldown,
		lastUsed,
		lastError,
	)
}
