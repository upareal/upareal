// story: e04s01
package web_test

import (
	"context"
	"net/http"
	"testing"
)

func TestSystemAccountsOnSignIn(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	b.post(t, "/auth/logout")
	b.get(t, "/auth/login") // second sign-in must not add accounts

	var userID string
	if err := b.app.conn.QueryRow(`SELECT id FROM users_user`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	accounts, err := b.app.ledger.Accounts(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	roles := map[string]bool{}
	for _, a := range accounts {
		roles[a.SystemRole] = true
	}
	if len(accounts) != 3 || !roles["opening_balance"] || !roles["uncategorized_income"] || !roles["uncategorized_expense"] {
		t.Errorf("accounts after two sign-ins = %+v, want exactly the 3 system accounts", accounts)
	}
}

func TestSystemAccountsOnSignIn_NoCategoriesPageYet(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	if res, _ := b.get(t, "/categorias"); res.StatusCode != http.StatusNotFound {
		t.Errorf("GET /categorias = %d, want 404 until the budget module (e05s01)", res.StatusCode)
	}
}
