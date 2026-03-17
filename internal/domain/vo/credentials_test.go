package vo_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

func TestSensitiveString_Value(t *testing.T) {
	s := vo.NewSensitiveString("sk-ant-secret-key")
	if s.Value() != "sk-ant-secret-key" {
		t.Errorf("Value() = %q; want %q", s.Value(), "sk-ant-secret-key")
	}
}

func TestSensitiveString_String(t *testing.T) {
	s := vo.NewSensitiveString("sk-ant-secret-key")
	if s.String() != "[REDACTED]" {
		t.Errorf("String() = %q; want %q", s.String(), "[REDACTED]")
	}
}

func TestSensitiveString_GoString(t *testing.T) {
	s := vo.NewSensitiveString("sk-ant-secret-key")
	if s.GoString() != "[REDACTED]" {
		t.Errorf("GoString() = %q; want %q", s.GoString(), "[REDACTED]")
	}
}

func TestSensitiveString_Printf(t *testing.T) {
	s := vo.NewSensitiveString("sk-ant-secret-key")
	result := fmt.Sprintf("%v", s)
	if result != "[REDACTED]" {
		t.Errorf("Printf %%v = %q; want %q", result, "[REDACTED]")
	}
	result = fmt.Sprintf("%#v", s)
	if result != "[REDACTED]" {
		t.Errorf("Printf %%#v = %q; want %q", result, "[REDACTED]")
	}
}

func TestSensitiveString_MarshalJSON(t *testing.T) {
	s := vo.NewSensitiveString("sk-ant-secret-key")
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(b) != `"[REDACTED]"` {
		t.Errorf("MarshalJSON = %s; want %q", b, "[REDACTED]")
	}
}

func TestProviderCredentials(t *testing.T) {
	creds := vo.ProviderCredentials{
		APIKey:  vo.NewSensitiveString("sk-ant-secret"),
		BaseURL: "https://api.anthropic.com",
	}

	if creds.APIKey.Value() != "sk-ant-secret" {
		t.Errorf("APIKey.Value() = %q; want %q", creds.APIKey.Value(), "sk-ant-secret")
	}
	if creds.BaseURL != "https://api.anthropic.com" {
		t.Errorf("BaseURL = %q; want %q", creds.BaseURL, "https://api.anthropic.com")
	}
}
