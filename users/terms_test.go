// story: e03s03
package users_test

import (
	"context"
	"testing"
)

func TestAcceptTerms_RecordsVersionVisibleOnUser(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	u, err := f.svc.SignIn(ctx, maria)
	if err != nil {
		t.Fatal(err)
	}
	if u.TermsVersion != "" {
		t.Fatalf("new user TermsVersion = %q, want empty", u.TermsVersion)
	}
	token, _ := f.svc.CreateSession(ctx, u.ID)

	if err := f.svc.AcceptTerms(ctx, u.ID, "2026-10-07"); err != nil {
		t.Fatalf("AcceptTerms: %v", err)
	}
	got, err := f.svc.UserBySession(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if got.TermsVersion != "2026-10-07" {
		t.Errorf("TermsVersion after accept = %q, want 2026-10-07", got.TermsVersion)
	}
	again, _ := f.svc.SignIn(ctx, maria)
	if again.TermsVersion != "2026-10-07" {
		t.Errorf("TermsVersion on next sign-in = %q, want 2026-10-07", again.TermsVersion)
	}

	var acceptedAt string
	if err := f.conn.QueryRow(`SELECT terms_accepted_at FROM users_user WHERE id = ?`, u.ID).Scan(&acceptedAt); err != nil || acceptedAt == "" {
		t.Errorf("terms_accepted_at = %q (%v), want a timestamp as consent evidence", acceptedAt, err)
	}
}

func TestAcceptTerms_UnknownUserFails(t *testing.T) {
	f := newFixture(t)
	if err := f.svc.AcceptTerms(context.Background(), "no-such-user", "v1"); err == nil {
		t.Error("AcceptTerms for a missing user: expected error, got nil")
	}
}
