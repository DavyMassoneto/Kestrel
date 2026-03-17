package entity

import (
	"fmt"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

// APIKey represents a proxy API key with behavior.
type APIKey struct {
	id            vo.APIKeyID
	keyHash       string
	keyPrefix     string
	name          string
	isActive      bool
	allowedModels []string
	lastUsedAt    *time.Time
}

// NewAPIKey creates a validated API key. Returns error if required fields are missing.
func NewAPIKey(id vo.APIKeyID, name string, keyHash string, keyPrefix string) (*APIKey, error) {
	if name == "" {
		return nil, fmt.Errorf("API key name is required")
	}
	if keyHash == "" {
		return nil, fmt.Errorf("API key hash is required")
	}
	if keyPrefix == "" {
		return nil, fmt.Errorf("API key prefix is required")
	}
	return &APIKey{
		id:       id,
		name:     name,
		keyHash:  keyHash,
		keyPrefix: keyPrefix,
		isActive: true,
	}, nil
}

// Getters

func (k *APIKey) ID() vo.APIKeyID      { return k.id }
func (k *APIKey) KeyHash() string       { return k.keyHash }
func (k *APIKey) KeyPrefix() string     { return k.keyPrefix }
func (k *APIKey) Name() string          { return k.name }
func (k *APIKey) IsActive() bool        { return k.isActive }
func (k *APIKey) LastUsedAt() *time.Time { return k.lastUsedAt }

func (k *APIKey) AllowedModels() []string {
	cp := make([]string, len(k.allowedModels))
	copy(cp, k.allowedModels)
	return cp
}

// SetAllowedModels sets the list of allowed models.
func (k *APIKey) SetAllowedModels(models []string) {
	k.allowedModels = make([]string, len(models))
	copy(k.allowedModels, models)
}

// Validate checks if rawKey matches the stored hash using the injected compare function.
func (k *APIKey) Validate(rawKey string, compareFn func(hash, raw string) bool) bool {
	return compareFn(k.keyHash, rawKey)
}

// IsModelAllowed returns true if the model is in the allowed list,
// or if the allowed list is empty (all models allowed).
func (k *APIKey) IsModelAllowed(model string) bool {
	if len(k.allowedModels) == 0 {
		return true
	}
	for _, m := range k.allowedModels {
		if m == model {
			return true
		}
	}
	return false
}

// RecordUsage updates LastUsedAt to now.
func (k *APIKey) RecordUsage(now time.Time) {
	k.lastUsedAt = &now
}
