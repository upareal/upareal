// story: e09s02
package planning_test

import (
	"context"
	"testing"

	"github.com/upareal/upareal/internal/dbtest"
	"github.com/upareal/upareal/ledger"
)

func TestDeleteUserData_RemovesPlanningRowsOfTheUserOnly(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	newMonthBook(t, f) // user A mappings
	shellB, err := f.ledg.CreateAccount(ctx, userB, ledger.NewAccount{Name: "Posto B", Type: "expense"})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.SetMapping(ctx, userB, shellB.ID, f.category(t, userB, "Combustível")); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := f.svc.DeleteUserData(ctx, userA); err != nil {
			t.Fatalf("DeleteUserData: %v", err)
		}
	}
	dbtest.AssertGone(t, f.conn, "planning_", userA)
	if n := dbtest.UserRows(t, f.conn, "planning_", userB)["planning_mapping"]; n != 1 {
		t.Errorf("user B mappings = %d, want 1", n)
	}
}
