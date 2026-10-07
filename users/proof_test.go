// story: e03s01
package users_test

import (
	"bytes"
	"context"
	"os"
	"testing"
)

// TestUsersTableHasNoPlaintext proves a copy of the database reveals no
// email, name, Google sub or session token after a real sign-in.
func TestUsersTableHasNoPlaintext(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	u, err := f.svc.SignIn(ctx, maria)
	if err != nil {
		t.Fatal(err)
	}
	token, err := f.svc.CreateSession(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := f.svc.UserBySession(ctx, token); err != nil || got.Email != maria.Email {
		t.Fatalf("round trip = %+v, %v; the scan below would prove nothing", got, err)
	}
	if err := f.conn.Close(); err != nil {
		t.Fatal(err)
	}

	var raw []byte
	for _, p := range []string{f.path, f.path + "-wal", f.path + "-shm"} {
		if b, err := os.ReadFile(p); err == nil {
			raw = append(raw, b...)
		}
	}
	if !bytes.Contains(raw, []byte("users_user")) {
		t.Fatal("control: schema text not found; the scan is not reading the database")
	}
	for label, secret := range map[string]string{
		"email": maria.Email, "name": maria.Name, "google sub": maria.Subject, "session token": token,
	} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Errorf("database files contain the %s in plaintext", label)
		}
	}
}
