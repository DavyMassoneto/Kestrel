package vo

import (
	"fmt"

	nanoid "github.com/matoous/go-nanoid/v2"
)

const idLen = 21

// AccountID identifies an Account.
type AccountID struct{ value string }

func NewAccountID() AccountID          { return AccountID{value: "acc_" + nanoid.Must(idLen)} }
func (id AccountID) String() string    { return id.value }

func ParseAccountID(s string) (AccountID, error) {
	if err := validateID(s, "acc_"); err != nil {
		return AccountID{}, err
	}
	return AccountID{value: s}, nil
}

// APIKeyID identifies an API Key.
type APIKeyID struct{ value string }

func NewAPIKeyID() APIKeyID           { return APIKeyID{value: "key_" + nanoid.Must(idLen)} }
func (id APIKeyID) String() string    { return id.value }

func ParseAPIKeyID(s string) (APIKeyID, error) {
	if err := validateID(s, "key_"); err != nil {
		return APIKeyID{}, err
	}
	return APIKeyID{value: s}, nil
}

// SessionID identifies a Session.
type SessionID struct{ value string }

func NewSessionID() SessionID          { return SessionID{value: "ses_" + nanoid.Must(idLen)} }
func (id SessionID) String() string    { return id.value }

func ParseSessionID(s string) (SessionID, error) {
	if err := validateID(s, "ses_"); err != nil {
		return SessionID{}, err
	}
	return SessionID{value: s}, nil
}

// RequestID identifies a Request.
type RequestID struct{ value string }

func NewRequestID() RequestID          { return RequestID{value: "req_" + nanoid.Must(idLen)} }
func (id RequestID) String() string    { return id.value }

func ParseRequestID(s string) (RequestID, error) {
	if err := validateID(s, "req_"); err != nil {
		return RequestID{}, err
	}
	return RequestID{value: s}, nil
}

func validateID(s, prefix string) error {
	if len(s) != len(prefix)+idLen {
		return fmt.Errorf("invalid ID length: got %d, want %d", len(s), len(prefix)+idLen)
	}
	if s[:len(prefix)] != prefix {
		return fmt.Errorf("invalid ID prefix: got %q, want %q", s[:len(prefix)], prefix)
	}
	return nil
}
