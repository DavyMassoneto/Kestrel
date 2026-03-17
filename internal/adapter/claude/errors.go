package claude

import (
	"fmt"
	"strings"

	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

// ProviderError is the adapter's internal error type that carries an ErrorClassification.
// The use case extracts the classification via errors.As without importing this type.
type ProviderError struct {
	StatusCode     int
	Message        string
	classification vo.ErrorClassification
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("claude: %d - %s", e.StatusCode, e.Message)
}

func (e *ProviderError) Classification() vo.ErrorClassification {
	return e.classification
}

// NewProviderError creates a ProviderError by classifying the HTTP status and body.
func NewProviderError(status int, body string) *ProviderError {
	return &ProviderError{
		StatusCode:     status,
		Message:        body,
		classification: classifyHTTPError(status, body),
	}
}

// classifyHTTPError converts an HTTP status + body into a domain ErrorClassification.
func classifyHTTPError(status int, body string) vo.ErrorClassification {
	switch {
	case status == 429:
		if strings.Contains(strings.ToLower(body), "quota") {
			return vo.ErrQuotaExhausted
		}
		return vo.ErrRateLimit
	case status == 401, status == 403:
		return vo.ErrAuth
	case status == 529:
		return vo.ErrOverloaded
	case status == 500, status == 502, status == 503:
		return vo.ErrServer
	case status >= 400 && status < 500:
		return vo.ErrClient
	default:
		return vo.ErrUnknown
	}
}
