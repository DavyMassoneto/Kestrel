package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
)

type apiKeyCtxKey struct{}

// Authenticator abstracts the authentication use case.
type Authenticator interface {
	Execute(ctx context.Context, rawKey string) (*entity.APIKey, error)
}

// Auth returns a middleware that validates Bearer tokens via the Authenticator.
func Auth(auth Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				writeAuthError(w)
				return
			}

			token, ok := parseBearerToken(header)
			if !ok {
				writeAuthError(w)
				return
			}

			apiKey, err := auth.Execute(r.Context(), token)
			if err != nil {
				writeAuthError(w)
				return
			}

			ctx := context.WithValue(r.Context(), apiKeyCtxKey{}, apiKey)
			ctx = WithAPIKeyID(ctx, apiKey.ID())
			ctx = WithAPIKeyName(ctx, apiKey.Name())
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// APIKeyFromContext retrieves the authenticated API key from the context.
func APIKeyFromContext(ctx context.Context) *entity.APIKey {
	if key, ok := ctx.Value(apiKeyCtxKey{}).(*entity.APIKey); ok {
		return key
	}
	return nil
}

func parseBearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	token := header[len(prefix):]
	if token == "" {
		return "", false
	}
	return token, true
}

func writeAuthError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(errorResponse{
		Error: errorDetail{
			Message: "Invalid API key",
			Type:    "authentication_error",
			Code:    "invalid_api_key",
		},
	})
}
