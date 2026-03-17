package vo_test

import (
	"testing"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

func TestNewCooldown(t *testing.T) {
	now := time.Now()
	until := now.Add(30 * time.Second)
	cd := vo.NewCooldown(until, 2, vo.ErrRateLimit)

	if cd.Until() != until {
		t.Errorf("Until = %v; want %v", cd.Until(), until)
	}
	if cd.BackoffLevel() != 2 {
		t.Errorf("BackoffLevel = %d; want 2", cd.BackoffLevel())
	}
	if cd.Reason() != vo.ErrRateLimit {
		t.Errorf("Reason = %q; want %q", cd.Reason(), vo.ErrRateLimit)
	}
}

func TestCooldown_IsExpired_False(t *testing.T) {
	now := time.Now()
	until := now.Add(30 * time.Second)
	cd := vo.NewCooldown(until, 1, vo.ErrServer)

	if cd.IsExpired(now) {
		t.Error("cooldown should not be expired")
	}
}

func TestCooldown_IsExpired_True(t *testing.T) {
	now := time.Now()
	until := now.Add(-1 * time.Second)
	cd := vo.NewCooldown(until, 1, vo.ErrServer)

	if !cd.IsExpired(now) {
		t.Error("cooldown should be expired")
	}
}

func TestCooldown_IsExpired_Exact(t *testing.T) {
	now := time.Now()
	cd := vo.NewCooldown(now, 1, vo.ErrServer)

	// at exactly until time, should be expired (not before until)
	if !cd.IsExpired(now) {
		t.Error("cooldown at exact until time should be expired")
	}
}
