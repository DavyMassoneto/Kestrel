package entity

import (
	"fmt"
	"math"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

// AccountStatus represents the current state of an account.
type AccountStatus string

const (
	StatusActive   AccountStatus = "active"
	StatusCooldown AccountStatus = "cooldown"
	StatusDisabled AccountStatus = "disabled"
)

// Account represents a Claude provider account with behavior.
type Account struct {
	id         vo.AccountID
	name       string
	apiKey     vo.SensitiveString
	baseURL    string
	status     AccountStatus
	priority   int
	cooldown   *vo.Cooldown
	lastUsedAt *time.Time
	lastError  *string
}

// NewAccount creates a validated account. Returns error if required fields are missing.
func NewAccount(id vo.AccountID, name string, apiKey vo.SensitiveString, baseURL string, priority int) (*Account, error) {
	if err := validateAccountFields(name, apiKey, baseURL); err != nil {
		return nil, err
	}
	return &Account{
		id:       id,
		name:     name,
		apiKey:   apiKey,
		baseURL:  baseURL,
		status:   StatusActive,
		priority: priority,
	}, nil
}

// RehydrateAccount reconstructs an account from persisted data.
// Applies the same validations as NewAccount but does not generate ID or defaults.
func RehydrateAccount(
	id vo.AccountID,
	name string,
	apiKey vo.SensitiveString,
	baseURL string,
	status AccountStatus,
	priority int,
	cooldown *vo.Cooldown,
	lastUsedAt *time.Time,
	lastError *string,
) (*Account, error) {
	if err := validateAccountFields(name, apiKey, baseURL); err != nil {
		return nil, err
	}
	return &Account{
		id:         id,
		name:       name,
		apiKey:     apiKey,
		baseURL:    baseURL,
		status:     status,
		priority:   priority,
		cooldown:   cooldown,
		lastUsedAt: lastUsedAt,
		lastError:  lastError,
	}, nil
}

func validateAccountFields(name string, apiKey vo.SensitiveString, baseURL string) error {
	if name == "" {
		return fmt.Errorf("account name is required")
	}
	if apiKey.Value() == "" {
		return fmt.Errorf("account API key is required")
	}
	if baseURL == "" {
		return fmt.Errorf("account base URL is required")
	}
	return nil
}

// Getters

func (a *Account) ID() vo.AccountID        { return a.id }
func (a *Account) Name() string             { return a.name }
func (a *Account) BaseURL() string          { return a.baseURL }
func (a *Account) Status() AccountStatus    { return a.status }
func (a *Account) Priority() int            { return a.priority }
func (a *Account) LastUsedAt() *time.Time   { return a.lastUsedAt }
func (a *Account) LastError() *string       { return a.lastError }

func (a *Account) CooldownUntil() *time.Time {
	if a.cooldown == nil {
		return nil
	}
	t := a.cooldown.Until()
	return &t
}

func (a *Account) BackoffLevel() int {
	if a.cooldown == nil {
		return 0
	}
	return a.cooldown.BackoffLevel()
}

func (a *Account) ErrorClassification() *vo.ErrorClassification {
	if a.cooldown == nil {
		return nil
	}
	r := a.cooldown.Reason()
	return &r
}

// Credentials returns ProviderCredentials for use by adapters.
func (a *Account) Credentials() vo.ProviderCredentials {
	return vo.ProviderCredentials{
		APIKey:  a.apiKey,
		BaseURL: a.baseURL,
	}
}

// ApplyCooldown applies exponential cooldown for transient errors.
// Rejects ErrAuth, ErrClient, and ErrUnknown — use Disable() for auth errors.
func (a *Account) ApplyCooldown(classification vo.ErrorClassification, now time.Time) error {
	switch classification {
	case vo.ErrAuth, vo.ErrClient, vo.ErrUnknown:
		return fmt.Errorf("cannot apply cooldown for classification %q; use Disable() instead", classification)
	}

	currentLevel := 0
	if a.cooldown != nil {
		currentLevel = a.cooldown.BackoffLevel()
	}
	newLevel := currentLevel + 1

	var duration time.Duration
	defaultDuration := classification.DefaultCooldownDuration()
	if defaultDuration > 0 {
		duration = defaultDuration
	} else {
		// Exponential backoff: min(2^level, 120) seconds
		seconds := math.Min(math.Pow(2, float64(newLevel)), 120)
		duration = time.Duration(seconds) * time.Second
	}

	cd := vo.NewCooldown(now.Add(duration), newLevel, classification)
	a.cooldown = &cd
	a.status = StatusCooldown
	errMsg := string(classification)
	a.lastError = &errMsg

	return nil
}

// ClearError resets backoff and status to active.
func (a *Account) ClearError() {
	a.cooldown = nil
	a.status = StatusActive
	a.lastError = nil
}

// Disable marks the account as disabled with a reason.
func (a *Account) Disable(reason string) {
	a.status = StatusDisabled
	a.lastError = &reason
}

// IsAvailable returns true if status != disabled AND cooldown is expired.
func (a *Account) IsAvailable(now time.Time) bool {
	if a.status == StatusDisabled {
		return false
	}
	if a.cooldown != nil && !a.cooldown.IsExpired(now) {
		return false
	}
	return true
}

// RecordUsage updates LastUsedAt to now.
func (a *Account) RecordUsage(now time.Time) {
	a.lastUsedAt = &now
}
