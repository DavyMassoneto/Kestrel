package vo_test

import (
	"strings"
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

func TestNewAccountID(t *testing.T) {
	id := vo.NewAccountID()
	if !strings.HasPrefix(id.String(), "acc_") {
		t.Errorf("AccountID = %q; want prefix %q", id.String(), "acc_")
	}
	if len(id.String()) != 4+21 {
		t.Errorf("AccountID len = %d; want %d", len(id.String()), 25)
	}
}

func TestParseAccountID(t *testing.T) {
	id := vo.NewAccountID()
	parsed, err := vo.ParseAccountID(id.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed != id {
		t.Errorf("parsed = %q; want %q", parsed.String(), id.String())
	}
}

func TestParseAccountID_InvalidPrefix(t *testing.T) {
	_, err := vo.ParseAccountID("key_abcdefghijklmnopqrstu")
	if err == nil {
		t.Fatal("expected error for invalid prefix")
	}
}

func TestParseAccountID_InvalidLength(t *testing.T) {
	_, err := vo.ParseAccountID("acc_short")
	if err == nil {
		t.Fatal("expected error for invalid length")
	}
}

func TestNewAPIKeyID(t *testing.T) {
	id := vo.NewAPIKeyID()
	if !strings.HasPrefix(id.String(), "key_") {
		t.Errorf("APIKeyID = %q; want prefix %q", id.String(), "key_")
	}
	if len(id.String()) != 4+21 {
		t.Errorf("APIKeyID len = %d; want %d", len(id.String()), 25)
	}
}

func TestParseAPIKeyID(t *testing.T) {
	id := vo.NewAPIKeyID()
	parsed, err := vo.ParseAPIKeyID(id.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed != id {
		t.Errorf("parsed = %q; want %q", parsed.String(), id.String())
	}
}

func TestParseAPIKeyID_InvalidPrefix(t *testing.T) {
	_, err := vo.ParseAPIKeyID("acc_abcdefghijklmnopqrstu")
	if err == nil {
		t.Fatal("expected error for invalid prefix")
	}
}

func TestNewSessionID(t *testing.T) {
	id := vo.NewSessionID()
	if !strings.HasPrefix(id.String(), "ses_") {
		t.Errorf("SessionID = %q; want prefix %q", id.String(), "ses_")
	}
	if len(id.String()) != 4+21 {
		t.Errorf("SessionID len = %d; want %d", len(id.String()), 25)
	}
}

func TestParseSessionID(t *testing.T) {
	id := vo.NewSessionID()
	parsed, err := vo.ParseSessionID(id.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed != id {
		t.Errorf("parsed = %q; want %q", parsed.String(), id.String())
	}
}

func TestParseSessionID_InvalidPrefix(t *testing.T) {
	_, err := vo.ParseSessionID("acc_abcdefghijklmnopqrstu")
	if err == nil {
		t.Fatal("expected error for invalid prefix")
	}
}

func TestNewRequestID(t *testing.T) {
	id := vo.NewRequestID()
	if !strings.HasPrefix(id.String(), "req_") {
		t.Errorf("RequestID = %q; want prefix %q", id.String(), "req_")
	}
	if len(id.String()) != 4+21 {
		t.Errorf("RequestID len = %d; want %d", len(id.String()), 25)
	}
}

func TestParseRequestID(t *testing.T) {
	id := vo.NewRequestID()
	parsed, err := vo.ParseRequestID(id.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed != id {
		t.Errorf("parsed = %q; want %q", parsed.String(), id.String())
	}
}

func TestParseRequestID_Empty(t *testing.T) {
	_, err := vo.ParseRequestID("")
	if err == nil {
		t.Fatal("expected error for empty string")
	}
}
