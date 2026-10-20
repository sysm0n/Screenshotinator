package recorder

import "time"

// Throttle admits an event only if at least interval has passed since the last admitted one
// (leading edge: the first click of a burst wins). Not safe for concurrent use.
type Throttle struct {
	interval time.Duration
	last     time.Time
}

func NewThrottle(interval time.Duration) *Throttle { return &Throttle{interval: interval} }

func (t *Throttle) Allow(now time.Time) bool {
	if !t.last.IsZero() && now.Sub(t.last) < t.interval {
		return false
	}
	t.last = now
	return true
}
