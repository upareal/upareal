// story: e10s01
package users_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/upareal/upareal/internal/dbtest"
)

func TestExportUserData_ProfileWithoutSecurityMaterial(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	m, _ := f.svc.SignIn(ctx, maria)
	if err := f.svc.AcceptTerms(ctx, m.ID, "2026-01"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.CreateSession(ctx, m.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.SignIn(ctx, joao); err != nil {
		t.Fatal(err)
	}
	ex, err := f.svc.ExportUserData(ctx, m.ID)
	if err != nil {
		t.Fatalf("ExportUserData: %v", err)
	}
	for table := range dbtest.UserRows(t, f.conn, "users_", m.ID) {
		if table != "users_session" { // tokens are security material; users_user is the profile itself
			t.Errorf("table %s is not covered by the export", table)
		}
	}
	raw, _ := json.Marshal(ex)
	doc := string(raw)
	for _, want := range []string{`"id":"` + m.ID + `"`, `"name":"Maria Silva"`, `"email":"maria@example.com"`, `"terms_version":"2026-01"`, `"terms_accepted_at":"`, `"created_at":"`} {
		if !strings.Contains(doc, want) {
			t.Errorf("export missing %s", want)
		}
	}
	if strings.Contains(doc, "joao") || strings.Contains(doc, "sub_idx") || strings.Contains(doc, "data_key") {
		t.Errorf("export leaks another user or security material: %s", doc)
	}
}
