// story: e09s02
package users_test

import (
	"context"
	"errors"
	"testing"

	"github.com/upareal/upareal/users"
)

func TestDisable_RemovesSessionsAndRefusesSignIn(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	svc := adminService(t, f, "maria@example.com")
	j, _ := svc.SignIn(ctx, joao)
	token, _ := svc.CreateSession(ctx, j.ID)

	if err := svc.DisableUser(ctx, j.ID); err != nil {
		t.Fatalf("DisableUser: %v", err)
	}
	if _, err := svc.UserBySession(ctx, token); !errors.Is(err, users.ErrNoSession) {
		t.Errorf("disabled user's session = %v, want ErrNoSession", err)
	}
	if _, err := svc.SignIn(ctx, joao); !errors.Is(err, users.ErrDisabled) {
		t.Errorf("disabled SignIn = %v, want ErrDisabled", err)
	}
	if list, _ := svc.ListUsers(ctx); !list[0].Disabled {
		t.Errorf("list = %+v, want João disabled", list)
	}
	if err := svc.EnableUser(ctx, j.ID); err != nil {
		t.Fatalf("EnableUser: %v", err)
	}
	if _, err := svc.SignIn(ctx, joao); err != nil {
		t.Errorf("SignIn after enable = %v", err)
	}
}

func TestDisable_AdminsAreProtectedAndUnknownIsNotFound(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	svc := adminService(t, f, "maria@example.com")
	m, _ := svc.SignIn(ctx, maria)
	if err := svc.DisableUser(ctx, m.ID); !errors.Is(err, users.ErrProtectedUser) {
		t.Errorf("disable admin = %v, want ErrProtectedUser", err)
	}
	if err := svc.DeleteUser(ctx, m.ID, func(context.Context, string) error { t.Error("purge ran for an admin"); return nil }); !errors.Is(err, users.ErrProtectedUser) {
		t.Errorf("delete admin = %v, want ErrProtectedUser", err)
	}
	for _, err := range []error{svc.DisableUser(ctx, "no-such"), svc.EnableUser(ctx, "no-such"), svc.DeleteUser(ctx, "no-such", nil)} {
		if !errors.Is(err, users.ErrNotFound) {
			t.Errorf("unknown user = %v, want ErrNotFound", err)
		}
	}
}

func TestDeleteUser_PurgesFirstThenRemovesUserAndSessions(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	svc := adminService(t, f, "maria@example.com")
	j, _ := svc.SignIn(ctx, joao)
	token, _ := svc.CreateSession(ctx, j.ID)

	boom := errors.New("ledger down")
	if err := svc.DeleteUser(ctx, j.ID, func(context.Context, string) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("failed purge = %v, want it returned", err)
	}
	if f.userCount(t) != 1 {
		t.Fatal("user row deleted although the purge failed")
	}
	var purged string
	if err := svc.DeleteUser(ctx, j.ID, func(_ context.Context, id string) error { purged = id; return nil }); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if purged != j.ID || f.userCount(t) != 0 {
		t.Errorf("purged %q, users left %d; want purge of João and no users", purged, f.userCount(t))
	}
	if _, err := svc.UserBySession(ctx, token); !errors.Is(err, users.ErrNoSession) {
		t.Errorf("deleted user's session = %v, want ErrNoSession", err)
	}
}
