package sqlite

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/middleware"
)

// RequestLogFilters controls filtering and pagination for log queries.
type RequestLogFilters struct {
	Limit     int
	Offset    int
	Status    *int
	AccountID *string
	APIKeyID  *string
	Model     *string
	From      *time.Time
	To        *time.Time
}

// RequestLogRepo persists request log entries to SQLite.
type RequestLogRepo struct {
	db *DB
}

// NewRequestLogRepo creates a new RequestLogRepo.
func NewRequestLogRepo(db *DB) *RequestLogRepo {
	return &RequestLogRepo{db: db}
}

// LogRequest inserts a request log entry into the database.
func (r *RequestLogRepo) LogRequest(ctx context.Context, entry middleware.RequestLogEntry) error {
	var accountID, accountName, errStr *string
	if entry.AccountID != "" {
		accountID = &entry.AccountID
	}
	if entry.AccountName != "" {
		accountName = &entry.AccountName
	}
	if entry.Error != "" {
		errStr = &entry.Error
	}

	_, err := r.db.Writer().ExecContext(ctx,
		`INSERT INTO request_log (id, api_key_id, api_key_name, account_id, account_name, model, status, input_tokens, output_tokens, latency_ms, retries, error, stream, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.RequestID,
		entry.APIKeyID,
		entry.APIKeyName,
		accountID,
		accountName,
		entry.Model,
		entry.Status,
		entry.InputTokens,
		entry.OutputTokens,
		entry.LatencyMs,
		entry.Retries,
		errStr,
		entry.Stream,
		entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert request_log: %w", err)
	}
	return nil
}

// FindAll retrieves request log entries with filtering and pagination.
// Returns entries, total count (for pagination), and error.
func (r *RequestLogRepo) FindAll(ctx context.Context, filters RequestLogFilters) ([]middleware.RequestLogEntry, int, error) {
	limit := filters.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	var where []string
	var args []any

	if filters.Status != nil {
		where = append(where, "status = ?")
		args = append(args, *filters.Status)
	}
	if filters.AccountID != nil {
		where = append(where, "account_id = ?")
		args = append(args, *filters.AccountID)
	}
	if filters.APIKeyID != nil {
		where = append(where, "api_key_id = ?")
		args = append(args, *filters.APIKeyID)
	}
	if filters.Model != nil {
		where = append(where, "model = ?")
		args = append(args, *filters.Model)
	}
	if filters.From != nil {
		where = append(where, "created_at >= ?")
		args = append(args, filters.From.UTC().Format(time.RFC3339))
	}
	if filters.To != nil {
		where = append(where, "created_at <= ?")
		args = append(args, filters.To.UTC().Format(time.RFC3339))
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = " WHERE " + strings.Join(where, " AND ")
	}

	// Count total
	countQuery := "SELECT COUNT(*) FROM request_log" + whereClause
	var total int
	if err := r.db.Reader().QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count request_log: %w", err)
	}

	// Query entries
	query := "SELECT id, api_key_id, api_key_name, account_id, account_name, model, status, input_tokens, output_tokens, latency_ms, retries, error, stream, created_at FROM request_log" +
		whereClause + " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	queryArgs := append(args, limit, filters.Offset)

	rows, err := r.db.Reader().QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query request_log: %w", err)
	}
	defer rows.Close()

	var entries []middleware.RequestLogEntry
	for rows.Next() {
		var e middleware.RequestLogEntry
		var accountID, accountName, errStr, apiKeyName *string

		if err := rows.Scan(
			&e.RequestID,
			&e.APIKeyID,
			&apiKeyName,
			&accountID,
			&accountName,
			&e.Model,
			&e.Status,
			&e.InputTokens,
			&e.OutputTokens,
			&e.LatencyMs,
			&e.Retries,
			&errStr,
			&e.Stream,
			&e.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan request_log row: %w", err)
		}

		if apiKeyName != nil {
			e.APIKeyName = *apiKeyName
		}
		if accountID != nil {
			e.AccountID = *accountID
		}
		if accountName != nil {
			e.AccountName = *accountName
		}
		if errStr != nil {
			e.Error = *errStr
		}

		entries = append(entries, e)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows error request_log: %w", err)
	}

	return entries, total, nil
}
