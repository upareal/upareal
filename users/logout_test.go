// story: e03s02
package users_test

import (
	"context"
	"errors"
	"testing"

	"github.com/upareal/upareal/users"
)

func TestLogout_EndSessionKillsOnlyThatSession(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	u, _ := f.svc.SignIn(ctx, maria)
	a, _ := f.svc.CreateSession(ctx, u.ID)
	b, _ := f.svc.CreateSession(ctx, u.ID)

	if err := f.svc.EndSession(ctx, a); err != nil {
		t.Fatalf("EndSession: %v", err)
	}
	if _, err := f.svc.UserBySession(ctx, a); !errors.Is(err, users.ErrNoSession) {
		t.Errorf("ended session still resolves: %v", err)
	}
	if _, err := f.svc.UserBySession(ctx, b); err != nil {
		t.Errorf("other session of the same user died: %v", err)
	}
	if err := f.svc.EndSession(ctx, "unknown-token"); err != nil {
		t.Errorf("EndSession on unknown token = %v, want nil (idempotent)", err)
	}
}

func TestLogout_EndAllSessionsKillsOnlyThatUsersSessions(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	u, _ := f.svc.SignIn(ctx, maria)
	other, _ := f.svc.SignIn(ctx, users.Identity{Subject: "g-2", Email: "o@example.com", Name: "Outro", EmailVerified: true})
	a, _ := f.svc.CreateSession(ctx, u.ID)
	b, _ := f.svc.CreateSession(ctx, u.ID)
	keep, _ := f.svc.CreateSession(ctx, other.ID)

	if err := f.svc.EndAllSessions(ctx, u.ID); err != nil {
		t.Fatalf("EndAllSessions: %v", err)
	}
	for _, tok := range []string{a, b} {
		if _, err := f.svc.UserBySession(ctx, tok); !errors.Is(err, users.ErrNoSession) {
			t.Errorf("session survived EndAllSessions: %v", err)
		}
	}
	if _, err := f.svc.UserBySession(ctx, keep); err != nil {
		t.Errorf("another user's session died: %v", err)
	}
}
