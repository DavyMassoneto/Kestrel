package errs

import "errors"

var (
	ErrAllAccountsExhausted = errors.New("all accounts exhausted")
	ErrInvalidRequest       = errors.New("invalid request")
	ErrModelNotAllowed      = errors.New("model not allowed")
	ErrAccountNotFound      = errors.New("account not found")
	ErrAPIKeyNotFound       = errors.New("API key not found")
	ErrInvalidAPIKey        = errors.New("invalid API key")
	ErrSessionExpired       = errors.New("session expired")
)
