package recorder

import (
	"testing"
	"time"
)

func TestThrottle(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	th := NewThrottle(1500 * time.Millisecond)

	steps := []struct {
		at   time.Duration
		want bool
	}{
		{0, true},                       // first click always passes
		{100 * time.Millisecond, false}, // double click
		{1499 * time.Millisecond, false},
		{1500 * time.Millisecond, true}, // measured from the last ACCEPTED click
		{2000 * time.Millisecond, false},
		{5 * time.Second, true},
	}
	for _, s := range steps {
		if got := th.Allow(t0.Add(s.at)); got != s.want {
			t.Errorf("Allow(+%v) = %v, want %v", s.at, got, s.want)
		}
	}
}
