package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

type ctxKeyAPIKeyID struct{}
type ctxKeyAPIKeyName struct{}

// WithAPIKeyID stores the API key ID in the context.
func WithAPIKeyID(ctx context.Context, id interface{ String() string }) context.Context {
	return context.WithValue(ctx, ctxKeyAPIKeyID{}, id.String())
}

// GetAPIKeyID retrieves the API key ID string from the context.
func GetAPIKeyID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyAPIKeyID{}).(string); ok {
		return v
	}
	return ""
}

// WithAPIKeyName stores the API key name in the context.
func WithAPIKeyName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, ctxKeyAPIKeyName{}, name)
}

// GetAPIKeyName retrieves the API key name from the context.
func GetAPIKeyName(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyAPIKeyName{}).(string); ok {
		return v
	}
	return ""
}

type ctxKeyLogData struct{}

// LogData is a mutable, request-scoped store injected into the context by the
// logging middleware. Handlers populate it after receiving use-case results;
// the middleware reads it after next.ServeHTTP returns.
type LogData struct {
	Model        string
	AccountID    string
	AccountName  string
	InputTokens  int
	OutputTokens int
	Retries      int
	Stream       bool
	Error        string
}

// LogDataFromContext retrieves the mutable LogData from the context.
func LogDataFromContext(ctx context.Context) *LogData {
	if ld, ok := ctx.Value(ctxKeyLogData{}).(*LogData); ok {
		return ld
	}
	return nil
}

// RequestLogEntry contains structured data for request logging.
type RequestLogEntry struct {
	RequestID    string
	APIKeyID     string
	APIKeyName   string
	AccountID    string
	AccountName  string
	Model        string
	Status       int
	InputTokens  int
	OutputTokens int
	LatencyMs    int64
	Retries      int
	Error        string
	Stream       bool
	CreatedAt    string
	Method       string
	Path         string
}

// RequestLogger persists request log entries.
type RequestLogger interface {
	LogRequest(ctx context.Context, entry RequestLogEntry) error
}

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

// RequestLogReader retrieves request log entries with filtering.
type RequestLogReader interface {
	FindAll(ctx context.Context, filters RequestLogFilters) ([]RequestLogEntry, int, error)
}

// NewLogging creates a logging middleware.
// If logger is nil, only slog output is produced.
func NewLogging(logger RequestLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			ld := &LogData{}
			ctx := context.WithValue(r.Context(), ctxKeyLogData{}, ld)
			r = r.WithContext(ctx)

			sw := &statusWriter{ResponseWriter: w}
			next.ServeHTTP(sw, r)

			latency := time.Since(start).Milliseconds()
			status := sw.status()

			slog.Info("request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", status),
				slog.Int64("latency_ms", latency),
				slog.String("request_id", GetRequestID(r.Context())),
			)

			apiKeyID := GetAPIKeyID(r.Context())
			if logger != nil && apiKeyID != "" {
				entry := RequestLogEntry{
					RequestID:    GetRequestID(r.Context()),
					APIKeyID:     apiKeyID,
					APIKeyName:   GetAPIKeyName(r.Context()),
					AccountID:    ld.AccountID,
					AccountName:  ld.AccountName,
					Model:        ld.Model,
					Status:       status,
					InputTokens:  ld.InputTokens,
					OutputTokens: ld.OutputTokens,
					LatencyMs:    latency,
					Retries:      ld.Retries,
					Error:        ld.Error,
					Stream:       ld.Stream,
					CreatedAt:    start.UTC().Format(time.RFC3339),
					Method:       r.Method,
					Path:         r.URL.Path,
				}
				if err := logger.LogRequest(r.Context(), entry); err != nil {
					slog.Error("failed to persist request log",
						slog.String("error", err.Error()),
						slog.String("request_id", GetRequestID(r.Context())),
					)
				}
			}
		})
	}
}

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	code    int
	written bool
}

func (w *statusWriter) WriteHeader(code int) {
	if w.written {
		return
	}
	w.code = code
	w.written = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) status() int {
	if !w.written {
		return http.StatusOK
	}
	return w.code
}
