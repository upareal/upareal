// story: e13s01
package users_test

import (
	"context"
	"testing"

	"github.com/upareal/upareal/internal/dbtest"
)

func TestPublicStats_CountsUsers(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.svc.SignIn(ctx, maria); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.SignIn(ctx, joao); err != nil {
		t.Fatal(err)
	}
	s, err := f.svc.PublicStats(ctx)
	if err != nil {
		t.Fatalf("PublicStats: %v", err)
	}
	dbtest.AssertNumbersOnly(t, s)
	if s.Users != 2 {
		t.Errorf("Users = %d, want 2", s.Users)
	}
}
