// story: e03s01
package users_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/upareal/upareal/internal/db"
	"github.com/upareal/upareal/internal/vault"
	"github.com/upareal/upareal/users"
)

type fixture struct {
	conn *sql.DB
	svc  *users.Service
	path string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "users.db")
	conn, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(context.Background(), conn, "users", users.Migrations); err != nil {
		t.Fatal(err)
	}
	master := make([]byte, 32)
	master[0] = 1
	kr, err := vault.NewKeyring(master)
	if err != nil {
		t.Fatal(err)
	}
	return fixture{conn: conn, svc: users.NewService(conn, kr), path: path}
}

func (f fixture) userCount(t *testing.T) int {
	t.Helper()
	var n int
	if err := f.conn.QueryRow(`SELECT count(*) FROM users_user`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

var maria = users.Identity{Subject: "g-123", Email: "maria@example.com", Name: "Maria Silva", EmailVerified: true}

func TestSignIn_FirstSignInCreatesUser(t *testing.T) {
	f := newFixture(t)
	u, err := f.svc.SignIn(context.Background(), maria)
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}
	id, err := uuid.Parse(u.ID)
	if err != nil || id.Version() != 7 {
		t.Errorf("user ID %q is not a UUIDv7", u.ID)
	}
	if u.Email != maria.Email || u.Name != maria.Name {
		t.Errorf("user = %+v, want Maria's email and name", u)
	}
	if f.userCount(t) != 1 {
		t.Errorf("user count = %d, want 1", f.userCount(t))
	}
}

func TestSignIn_ReturningSubReusesUserAndUpdatesIdentity(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	first, err := f.svc.SignIn(ctx, maria)
	if err != nil {
		t.Fatal(err)
	}
	changed := maria
	changed.Email, changed.Name = "maria.nova@example.com", "Maria S. Souza"
	again, err := f.svc.SignIn(ctx, changed)
	if err != nil {
		t.Fatalf("second SignIn: %v", err)
	}
	if again.ID != first.ID {
		t.Errorf("second sign-in ID = %s, want %s", again.ID, first.ID)
	}
	if again.Email != changed.Email || again.Name != changed.Name {
		t.Errorf("identity not updated: %+v", again)
	}
	if f.userCount(t) != 1 {
		t.Errorf("user count = %d, want 1", f.userCount(t))
	}
}

func TestSignIn_DifferentSubCreatesDifferentUser(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a, _ := f.svc.SignIn(ctx, maria)
	joao := users.Identity{Subject: "g-456", Email: "maria@example.com", Name: "João", EmailVerified: true}
	b, err := f.svc.SignIn(ctx, joao)
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == b.ID {
		t.Error("a different Google sub (same email) reused the same user")
	}
}

func TestSignIn_RefusesUnverifiedOrIncompleteIdentity(t *testing.T) {
	f := newFixture(t)
	unverified := maria
	unverified.EmailVerified = false
	if _, err := f.svc.SignIn(context.Background(), unverified); !errors.Is(err, users.ErrEmailNotVerified) {
		t.Errorf("SignIn unverified = %v, want ErrEmailNotVerified", err)
	}
	noSub := maria
	noSub.Subject = ""
	if _, err := f.svc.SignIn(context.Background(), noSub); err == nil {
		t.Error("SignIn without subject: expected error, got nil")
	}
	if f.userCount(t) != 0 {
		t.Errorf("user count = %d, want 0 after refused sign-ins", f.userCount(t))
	}
}
