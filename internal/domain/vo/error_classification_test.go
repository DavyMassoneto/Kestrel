package vo_test

import (
	"testing"
	"time"

	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

func TestShouldFallback(t *testing.T) {
	tests := []struct {
		classification vo.ErrorClassification
		want           bool
	}{
		{vo.ErrRateLimit, true},
		{vo.ErrQuotaExhausted, true},
		{vo.ErrServer, true},
		{vo.ErrOverloaded, true},
		{vo.ErrAuth, false},
		{vo.ErrClient, false},
		{vo.ErrUnknown, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.classification), func(t *testing.T) {
			got := tt.classification.ShouldFallback()
			if got != tt.want {
				t.Errorf("ShouldFallback() = %v; want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultCooldownDuration(t *testing.T) {
	tests := []struct {
		classification vo.ErrorClassification
		want           time.Duration
	}{
		{vo.ErrRateLimit, 0},
		{vo.ErrQuotaExhausted, 5 * time.Minute},
		{vo.ErrServer, 60 * time.Second},
		{vo.ErrOverloaded, 30 * time.Second},
		{vo.ErrAuth, 0},
		{vo.ErrClient, 0},
		{vo.ErrUnknown, 0},
	}

	for _, tt := range tests {
		t.Run(string(tt.classification), func(t *testing.T) {
			got := tt.classification.DefaultCooldownDuration()
			if got != tt.want {
				t.Errorf("DefaultCooldownDuration() = %v; want %v", got, tt.want)
			}
		})
	}
}
