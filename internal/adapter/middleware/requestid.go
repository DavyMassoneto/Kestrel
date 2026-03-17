package middleware

import (
	"context"
	"net/http"

	nanoid "github.com/matoous/go-nanoid/v2"
)

type ctxKeyRequestID struct{}

// RequestID is a middleware that injects a request ID into the context and
// sets the X-Request-ID response header. If the incoming request already has
// an X-Request-ID header, it is reused.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = "req_" + nanoid.Must()
		}

		ctx := context.WithValue(r.Context(), ctxKeyRequestID{}, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID retrieves the request ID from the context.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(ctxKeyRequestID{}).(string); ok {
		return id
	}
	return ""
}
