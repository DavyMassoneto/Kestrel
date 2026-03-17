package errs_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/domain/errs"
)

func TestSentinelErrors_AreDistinct(t *testing.T) {
	sentinels := []error{
		errs.ErrAllAccountsExhausted,
		errs.ErrInvalidRequest,
		errs.ErrModelNotAllowed,
		errs.ErrAccountNotFound,
		errs.ErrAPIKeyNotFound,
		errs.ErrInvalidAPIKey,
		errs.ErrSessionExpired,
	}

	for i, a := range sentinels {
		for j, b := range sentinels {
			if i != j && errors.Is(a, b) {
				t.Errorf("sentinel %q should not equal %q", a, b)
			}
		}
	}
}

func TestSentinelErrors_HaveMessages(t *testing.T) {
	tests := []struct {
		err     error
		message string
	}{
		{errs.ErrAllAccountsExhausted, "all accounts exhausted"},
		{errs.ErrInvalidRequest, "invalid request"},
		{errs.ErrModelNotAllowed, "model not allowed"},
		{errs.ErrAccountNotFound, "account not found"},
		{errs.ErrAPIKeyNotFound, "API key not found"},
		{errs.ErrInvalidAPIKey, "invalid API key"},
		{errs.ErrSessionExpired, "session expired"},
	}

	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			if tt.err.Error() != tt.message {
				t.Errorf("Error() = %q; want %q", tt.err.Error(), tt.message)
			}
		})
	}
}

func TestSentinelErrors_WrappedMatching(t *testing.T) {
	wrapped := fmt.Errorf("context: %w", errs.ErrAccountNotFound)
	if !errors.Is(wrapped, errs.ErrAccountNotFound) {
		t.Error("wrapped error should match sentinel via errors.Is")
	}
}
