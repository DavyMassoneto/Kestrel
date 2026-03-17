package vo

import "time"

// Cooldown encapsulates the temporary unavailability state of an account.
// Value object — immutable, value receiver on all methods.
type Cooldown struct {
	until        time.Time
	backoffLevel int
	reason       ErrorClassification
}

func NewCooldown(until time.Time, backoffLevel int, reason ErrorClassification) Cooldown {
	return Cooldown{
		until:        until,
		backoffLevel: backoffLevel,
		reason:       reason,
	}
}

func (c Cooldown) Until() time.Time              { return c.until }
func (c Cooldown) BackoffLevel() int             { return c.backoffLevel }
func (c Cooldown) Reason() ErrorClassification   { return c.reason }

// IsExpired returns true if the cooldown has expired at the given time.
func (c Cooldown) IsExpired(now time.Time) bool {
	return !now.Before(c.until)
}
