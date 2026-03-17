package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

// APIKeyRepo implements APIKeyFinder and APIKeyStore using SQLite.
type APIKeyRepo struct {
	db *DB
}

// NewAPIKeyRepo creates a new APIKeyRepo.
func NewAPIKeyRepo(db *DB) *APIKeyRepo {
	return &APIKeyRepo{db: db}
}

// FindByPrefix finds an active API key by its prefix.
func (r *APIKeyRepo) FindByPrefix(ctx context.Context, prefix string) (*entity.APIKey, error) {
	row := r.db.Reader().QueryRowContext(ctx,
		`SELECT id, key_hash, key_prefix, name, is_active, allowed_models, last_used_at
		FROM api_keys WHERE key_prefix = ? AND is_active = 1`, prefix)
	return r.scanKey(row)
}

// FindByID finds an API key by ID.
func (r *APIKeyRepo) FindByID(ctx context.Context, id vo.APIKeyID) (*entity.APIKey, error) {
	row := r.db.Reader().QueryRowContext(ctx,
		`SELECT id, key_hash, key_prefix, name, is_active, allowed_models, last_used_at
		FROM api_keys WHERE id = ?`, id.String())
	return r.scanKey(row)
}

// FindAll returns all API keys.
func (r *APIKeyRepo) FindAll(ctx context.Context) ([]*entity.APIKey, error) {
	rows, err := r.db.Reader().QueryContext(ctx,
		`SELECT id, key_hash, key_prefix, name, is_active, allowed_models, last_used_at
		FROM api_keys ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query api_keys: %w", err)
	}
	defer rows.Close()

	var keys []*entity.APIKey
	for rows.Next() {
		key, err := r.scanKeyFromRows(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	if keys == nil {
		keys = []*entity.APIKey{}
	}
	return keys, rows.Err()
}

// Create inserts a new API key.
func (r *APIKeyRepo) Create(ctx context.Context, key *entity.APIKey) error {
	modelsJSON := marshalModels(key.AllowedModels())
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := r.db.Writer().ExecContext(ctx,
		`INSERT INTO api_keys (id, key_hash, key_prefix, name, is_active, allowed_models, created_at, last_used_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		key.ID().String(),
		key.KeyHash(),
		key.KeyPrefix(),
		key.Name(),
		key.IsActive(),
		modelsJSON,
		now,
		nil,
	)
	if err != nil {
		return fmt.Errorf("insert api_key: %w", err)
	}
	return nil
}

// Delete removes an API key by ID.
func (r *APIKeyRepo) Delete(ctx context.Context, id vo.APIKeyID) error {
	result, err := r.db.Writer().ExecContext(ctx,
		`DELETE FROM api_keys WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete api_key: %w", err)
	}

	// RowsAffected never returns error for the sqlite driver.
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("API key not found")
	}
	return nil
}

func (r *APIKeyRepo) scanKey(row *sql.Row) (*entity.APIKey, error) {
	var (
		id         string
		keyHash    string
		keyPrefix  string
		name       string
		isActive   bool
		modelsJSON sql.NullString
		lastUsedAt sql.NullString
	)

	if err := row.Scan(&id, &keyHash, &keyPrefix, &name, &isActive, &modelsJSON, &lastUsedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("API key not found")
		}
		return nil, fmt.Errorf("scan api_key: %w", err)
	}

	return r.buildKey(id, keyHash, keyPrefix, name, isActive, modelsJSON, lastUsedAt)
}

func (r *APIKeyRepo) scanKeyFromRows(rows *sql.Rows) (*entity.APIKey, error) {
	var (
		id         string
		keyHash    string
		keyPrefix  string
		name       string
		isActive   bool
		modelsJSON sql.NullString
		lastUsedAt sql.NullString
	)

	if err := rows.Scan(&id, &keyHash, &keyPrefix, &name, &isActive, &modelsJSON, &lastUsedAt); err != nil {
		return nil, fmt.Errorf("scan api_key row: %w", err)
	}

	return r.buildKey(id, keyHash, keyPrefix, name, isActive, modelsJSON, lastUsedAt)
}

func (r *APIKeyRepo) buildKey(id, keyHash, keyPrefix, name string, isActive bool, modelsJSON, lastUsedAt sql.NullString) (*entity.APIKey, error) {
	keyID, err := vo.ParseAPIKeyID(id)
	if err != nil {
		return nil, fmt.Errorf("parse api_key id: %w", err)
	}

	key, err := entity.NewAPIKey(keyID, name, keyHash, keyPrefix)
	if err != nil {
		return nil, fmt.Errorf("build api_key: %w", err)
	}

	if modelsJSON.Valid && modelsJSON.String != "" {
		models, err := unmarshalModels(modelsJSON.String)
		if err != nil {
			return nil, err
		}
		if len(models) > 0 {
			key.SetAllowedModels(models)
		}
	}

	if lastUsedAt.Valid {
		t, err := time.Parse(time.RFC3339, lastUsedAt.String)
		if err == nil {
			key.RecordUsage(t)
		}
	}

	return key, nil
}

// marshalModels serializes a string slice to JSON.
// json.Marshal never fails for []string — all strings are valid JSON.
func marshalModels(models []string) sql.NullString {
	if len(models) == 0 {
		return sql.NullString{}
	}
	data, _ := json.Marshal(models)
	return sql.NullString{String: string(data), Valid: true}
}

func unmarshalModels(s string) ([]string, error) {
	var models []string
	if err := json.Unmarshal([]byte(s), &models); err != nil {
		return nil, fmt.Errorf("unmarshal allowed_models: %w", err)
	}
	return models, nil
}
