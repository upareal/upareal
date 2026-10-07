// story: e03s01
package users_test

import (
	"context"
	"errors"
	"testing"

	"github.com/upareal/upareal/users"
)

func TestSession_CreateAndResolve(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	u, err := f.svc.SignIn(ctx, maria)
	if err != nil {
		t.Fatal(err)
	}
	token, err := f.svc.CreateSession(ctx, u.ID)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if len(token) < 43 {
		t.Errorf("token length = %d, want ≥43 (32 random bytes encoded)", len(token))
	}
	got, err := f.svc.UserBySession(ctx, token)
	if err != nil {
		t.Fatalf("UserBySession: %v", err)
	}
	if got.ID != u.ID || got.Name != "Maria Silva" {
		t.Errorf("UserBySession = %+v, want Maria (%s)", got, u.ID)
	}
}

func TestSession_TokensAreUniqueAndNotStored(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	u, _ := f.svc.SignIn(ctx, maria)
	a, _ := f.svc.CreateSession(ctx, u.ID)
	b, _ := f.svc.CreateSession(ctx, u.ID)
	if a == b {
		t.Error("two sessions got the same token")
	}
	var n int
	if err := f.conn.QueryRow(`SELECT count(*) FROM users_session WHERE token_hash = ? OR token_hash = ?`, []byte(a), a).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("the raw session token is stored; only its hash may be")
	}
}

func TestSession_UnknownTokensResolveToNoSession(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	u, _ := f.svc.SignIn(ctx, maria)
	token, _ := f.svc.CreateSession(ctx, u.ID)
	tampered := token[:len(token)-1] + "x"
	if tampered == token {
		tampered = token[:len(token)-1] + "y"
	}
	for name, tok := range map[string]string{"empty": "", "unknown": "not-a-real-token", "tampered": tampered} {
		t.Run(name, func(t *testing.T) {
			if _, err := f.svc.UserBySession(ctx, tok); !errors.Is(err, users.ErrNoSession) {
				t.Errorf("UserBySession(%s) = %v, want ErrNoSession", name, err)
			}
		})
	}
}

func TestSession_RequiresExistingUser(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.CreateSession(context.Background(), "no-such-user"); err == nil {
		t.Error("CreateSession for a missing user: expected error (foreign key), got nil")
	}
}
