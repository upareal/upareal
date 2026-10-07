// story: e03s02
package users_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/upareal/upareal/users"
)

// clock is a settable test clock.
type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newClockedFixture(t *testing.T) (fixture, *clock, users.User) {
	t.Helper()
	f := newFixture(t)
	c := &clock{t: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	users.SetNow(f.svc, c.now)
	u, err := f.svc.SignIn(context.Background(), maria)
	if err != nil {
		t.Fatal(err)
	}
	return f, c, u
}

func (f fixture) sessionCount(t *testing.T) int {
	t.Helper()
	var n int
	if err := f.conn.QueryRow(`SELECT count(*) FROM users_session`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (f fixture) lastSeen(t *testing.T) string {
	t.Helper()
	var s string
	if err := f.conn.QueryRow(`SELECT last_seen_at FROM users_session`).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSessionExpiry_IdleSessionDiesAfter30Days(t *testing.T) {
	f, c, u := newClockedFixture(t)
	ctx := context.Background()
	token, _ := f.svc.CreateSession(ctx, u.ID)

	c.advance(30*24*time.Hour - time.Minute)
	if _, err := f.svc.UserBySession(ctx, token); err != nil {
		t.Fatalf("session idle 30d-1m: %v, want alive", err)
	}
	c.advance(30*24*time.Hour + time.Minute)
	if _, err := f.svc.UserBySession(ctx, token); !errors.Is(err, users.ErrNoSession) {
		t.Errorf("session idle 30d+1m: %v, want ErrNoSession", err)
	}
	if f.sessionCount(t) != 0 {
		t.Error("expired session row was not deleted")
	}
}

func TestSessionExpiry_AbsoluteLimitIs90Days(t *testing.T) {
	f, c, u := newClockedFixture(t)
	ctx := context.Background()
	token, _ := f.svc.CreateSession(ctx, u.ID)

	for elapsed := 20 * 24 * time.Hour; elapsed < 90*24*time.Hour; elapsed += 20 * 24 * time.Hour {
		c.advance(20 * 24 * time.Hour) // used regularly, never idle
		if _, err := f.svc.UserBySession(ctx, token); err != nil {
			t.Fatalf("active session at %v: %v, want alive", elapsed, err)
		}
	}
	c.t = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC).Add(90*24*time.Hour + time.Minute)
	if _, err := f.svc.UserBySession(ctx, token); !errors.Is(err, users.ErrNoSession) {
		t.Errorf("session older than 90d: %v, want ErrNoSession", err)
	}
}

func TestSessionExpiry_TouchIsThrottledToHourly(t *testing.T) {
	f, c, u := newClockedFixture(t)
	ctx := context.Background()
	token, _ := f.svc.CreateSession(ctx, u.ID)
	created := f.lastSeen(t)

	c.advance(59 * time.Minute)
	_, _ = f.svc.UserBySession(ctx, token)
	if f.lastSeen(t) != created {
		t.Error("last_seen_at written within the first hour; want throttled")
	}
	c.advance(2 * time.Minute)
	_, _ = f.svc.UserBySession(ctx, token)
	if f.lastSeen(t) == created {
		t.Error("last_seen_at not refreshed after an hour")
	}
}

func TestSessionExpiry_NewSessionPurgesUsersExpiredSessions(t *testing.T) {
	f, c, u := newClockedFixture(t)
	ctx := context.Background()
	_, _ = f.svc.CreateSession(ctx, u.ID)
	c.advance(31 * 24 * time.Hour)
	_, _ = f.svc.CreateSession(ctx, u.ID)
	if f.sessionCount(t) != 1 {
		t.Errorf("sessions after re-login = %d, want 1 (expired one purged)", f.sessionCount(t))
	}
}
