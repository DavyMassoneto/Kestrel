package vo

// ProviderCredentials encapsulates the credentials needed for provider communication.
type ProviderCredentials struct {
	APIKey  SensitiveString
	BaseURL string
}

// SensitiveString encapsulates sensitive values. String() returns "[REDACTED]".
type SensitiveString struct {
	value string
}

func NewSensitiveString(v string) SensitiveString     { return SensitiveString{value: v} }
func (s SensitiveString) Value() string               { return s.value }
func (s SensitiveString) String() string              { return "[REDACTED]" }
func (s SensitiveString) GoString() string            { return "[REDACTED]" }
func (s SensitiveString) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }
