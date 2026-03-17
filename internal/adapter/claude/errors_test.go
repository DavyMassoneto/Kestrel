package claude

import (
	"errors"
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

func TestClassifyHTTPError_429_RateLimit(t *testing.T) {
	got := classifyHTTPError(429, `{"error":{"message":"rate limited"}}`)
	if got != vo.ErrRateLimit {
		t.Errorf("429 = %q, want %q", got, vo.ErrRateLimit)
	}
}

func TestClassifyHTTPError_429_Quota(t *testing.T) {
	got := classifyHTTPError(429, `{"error":{"message":"quota exceeded"}}`)
	if got != vo.ErrQuotaExhausted {
		t.Errorf("429 with quota = %q, want %q", got, vo.ErrQuotaExhausted)
	}
}

func TestClassifyHTTPError_429_QuotaCaseInsensitive(t *testing.T) {
	got := classifyHTTPError(429, `{"error":{"message":"Your Quota has been exceeded"}}`)
	if got != vo.ErrQuotaExhausted {
		t.Errorf("429 with Quota = %q, want %q", got, vo.ErrQuotaExhausted)
	}
}

func TestClassifyHTTPError_429_EmptyBody(t *testing.T) {
	got := classifyHTTPError(429, "")
	if got != vo.ErrRateLimit {
		t.Errorf("429 empty body = %q, want %q", got, vo.ErrRateLimit)
	}
}

func TestClassifyHTTPError_401(t *testing.T) {
	got := classifyHTTPError(401, `{"error":{"message":"invalid api key"}}`)
	if got != vo.ErrAuth {
		t.Errorf("401 = %q, want %q", got, vo.ErrAuth)
	}
}

func TestClassifyHTTPError_403(t *testing.T) {
	got := classifyHTTPError(403, `{"error":{"message":"forbidden"}}`)
	if got != vo.ErrAuth {
		t.Errorf("403 = %q, want %q", got, vo.ErrAuth)
	}
}

func TestClassifyHTTPError_529_Overloaded(t *testing.T) {
	got := classifyHTTPError(529, `{"error":{"message":"overloaded"}}`)
	if got != vo.ErrOverloaded {
		t.Errorf("529 = %q, want %q", got, vo.ErrOverloaded)
	}
}

func TestClassifyHTTPError_500(t *testing.T) {
	got := classifyHTTPError(500, "")
	if got != vo.ErrServer {
		t.Errorf("500 = %q, want %q", got, vo.ErrServer)
	}
}

func TestClassifyHTTPError_502(t *testing.T) {
	got := classifyHTTPError(502, "")
	if got != vo.ErrServer {
		t.Errorf("502 = %q, want %q", got, vo.ErrServer)
	}
}

func TestClassifyHTTPError_503(t *testing.T) {
	got := classifyHTTPError(503, "")
	if got != vo.ErrServer {
		t.Errorf("503 = %q, want %q", got, vo.ErrServer)
	}
}

func TestClassifyHTTPError_400(t *testing.T) {
	got := classifyHTTPError(400, `{"error":{"message":"bad request"}}`)
	if got != vo.ErrClient {
		t.Errorf("400 = %q, want %q", got, vo.ErrClient)
	}
}

func TestClassifyHTTPError_404(t *testing.T) {
	got := classifyHTTPError(404, "")
	if got != vo.ErrClient {
		t.Errorf("404 = %q, want %q", got, vo.ErrClient)
	}
}

func TestClassifyHTTPError_422(t *testing.T) {
	got := classifyHTTPError(422, "")
	if got != vo.ErrClient {
		t.Errorf("422 = %q, want %q", got, vo.ErrClient)
	}
}

func TestClassifyHTTPError_UnknownStatus(t *testing.T) {
	got := classifyHTTPError(600, "")
	if got != vo.ErrUnknown {
		t.Errorf("600 = %q, want %q", got, vo.ErrUnknown)
	}
}

func TestProviderError_Error(t *testing.T) {
	err := &ProviderError{
		StatusCode:     429,
		Message:        "rate limited",
		classification: vo.ErrRateLimit,
	}

	got := err.Error()
	if got != "claude: 429 - rate limited" {
		t.Errorf("Error() = %q", got)
	}
}

func TestProviderError_Classification(t *testing.T) {
	err := &ProviderError{
		StatusCode:     401,
		Message:        "invalid key",
		classification: vo.ErrAuth,
	}

	if err.Classification() != vo.ErrAuth {
		t.Errorf("Classification() = %q, want %q", err.Classification(), vo.ErrAuth)
	}
}

func TestProviderError_ImplementsError(t *testing.T) {
	var err error = &ProviderError{
		StatusCode:     500,
		Message:        "server error",
		classification: vo.ErrServer,
	}

	if err.Error() == "" {
		t.Error("Should implement error interface")
	}
}

func TestProviderError_ErrorsAs(t *testing.T) {
	original := &ProviderError{
		StatusCode:     429,
		Message:        "rate limited",
		classification: vo.ErrRateLimit,
	}

	var wrapped error = original

	var target *ProviderError
	if !errors.As(wrapped, &target) {
		t.Fatal("errors.As should find ProviderError")
	}
	if target.Classification() != vo.ErrRateLimit {
		t.Errorf("Classification = %q, want %q", target.Classification(), vo.ErrRateLimit)
	}
}

func TestNewProviderError(t *testing.T) {
	err := NewProviderError(503, `{"error":{"message":"service unavailable"}}`)

	if err.StatusCode != 503 {
		t.Errorf("StatusCode = %d, want 503", err.StatusCode)
	}
	if err.Classification() != vo.ErrServer {
		t.Errorf("Classification = %q, want %q", err.Classification(), vo.ErrServer)
	}
}
