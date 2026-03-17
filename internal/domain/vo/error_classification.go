package vo

import "time"

// ErrorClassification categorizes provider errors for fallback decisions.
// Pure domain enum — does NOT know HTTP status codes.
type ErrorClassification string

const (
	ErrRateLimit      ErrorClassification = "rate_limit"
	ErrQuotaExhausted ErrorClassification = "quota_exhausted"
	ErrAuth           ErrorClassification = "authentication_error"
	ErrServer         ErrorClassification = "server_error"
	ErrOverloaded     ErrorClassification = "overloaded"
	ErrClient         ErrorClassification = "client_error"
	ErrUnknown        ErrorClassification = "unknown"
)

// ShouldFallback returns true if the classification justifies trying another account.
func (c ErrorClassification) ShouldFallback() bool {
	switch c {
	case ErrRateLimit, ErrQuotaExhausted, ErrServer, ErrOverloaded:
		return true
	default:
		return false
	}
}

// DefaultCooldownDuration returns the base cooldown duration for this classification.
// If 0, exponential backoff is used instead.
func (c ErrorClassification) DefaultCooldownDuration() time.Duration {
	switch c {
	case ErrQuotaExhausted:
		return 5 * time.Minute
	case ErrServer:
		return 60 * time.Second
	case ErrOverloaded:
		return 30 * time.Second
	default:
		return 0
	}
}
