// story: e09s01
package users_test

import (
	"context"
	"testing"
	"time"

	"github.com/upareal/upareal/internal/vault"
	"github.com/upareal/upareal/users"
)

// adminService is a second service on the fixture's database with admins.
func adminService(t *testing.T, f fixture, admins ...string) *users.Service {
	t.Helper()
	master := make([]byte, 32)
	master[0] = 1
	kr, err := vault.NewKeyring(master)
	if err != nil {
		t.Fatal(err)
	}
	return users.NewService(f.conn, kr, users.AdminEmails(admins...))
}

var joao = users.Identity{Subject: "g-456", Email: "joao@example.com", Name: "João Souza", EmailVerified: true}

func TestAdmin_RoleComesFromAdminEmails(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	svc := adminService(t, f, " MARIA@example.com ")
	m, err := svc.SignIn(ctx, maria)
	if err != nil {
		t.Fatal(err)
	}
	j, _ := svc.SignIn(ctx, joao)
	if !m.Admin || j.Admin {
		t.Errorf("Maria admin %v, João admin %v; want true, false", m.Admin, j.Admin)
	}
	token, _ := svc.CreateSession(ctx, m.ID)
	if u, err := svc.UserBySession(ctx, token); err != nil || !u.Admin {
		t.Errorf("UserBySession admin = %v, %v; want true", u.Admin, err)
	}
	if u, _ := f.svc.SignIn(ctx, maria); u.Admin {
		t.Error("a service without ADMIN_EMAILS made Maria admin")
	}
}

func TestListUsers_IdentityAndLastAccessOnly(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	svc := adminService(t, f, "maria@example.com")
	m, _ := svc.SignIn(ctx, maria)
	if _, err := svc.CreateSession(ctx, m.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SignIn(ctx, joao); err != nil {
		t.Fatal(err)
	}
	list, err := svc.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(list) != 2 || list[0].Email != "maria@example.com" || list[0].Name != "Maria Silva" || !list[0].Admin {
		t.Fatalf("list = %+v; want Maria (admin) first", list)
	}
	if time.Since(list[0].LastSeenAt) > time.Minute {
		t.Errorf("Maria last seen %s, want just now", list[0].LastSeenAt)
	}
	if list[1].Email != "joao@example.com" || list[1].Admin || !list[1].LastSeenAt.IsZero() {
		t.Errorf("João = %+v; want not admin, never seen", list[1])
	}
}
