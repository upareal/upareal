// story: e03s02
package web

import (
	"fmt"
	"testing"
	"time"
)

// The limiter map must not grow forever with one-off client IPs.
func TestIPLimiter_PrunesIdleEntries(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := newIPLimiter()
	l.now = func() time.Time { return now }
	for i := 0; i <= limiterPruneAbove; i++ {
		l.allow(fmt.Sprintf("10.0.%d.%d", i/256, i%256))
	}
	now = now.Add(limiterIdleTTL + time.Minute)
	l.allow("192.0.2.1")
	if len(l.entries) != 1 {
		t.Errorf("entries after pruning = %d, want 1 (only the fresh IP)", len(l.entries))
	}
}
