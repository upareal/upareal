// story: e09s02
package budget_test

import (
	"context"
	"testing"

	"github.com/upareal/upareal/internal/dbtest"
)

func TestDeleteUserData_RemovesEveryBudgetRowOfTheUserOnly(t *testing.T) {
	f := seeded(t) // users A and B
	ctx := context.Background()
	if err := f.svc.SetAmount(ctx, userA, categoryNamed(t, f, "Combustível").ID, month(t, "2026-01"), 30000); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ArchiveCategory(ctx, userA, categoryNamed(t, f, "Hobbies").ID); err != nil {
		t.Fatal(err)
	}
	before := dbtest.UserRows(t, f.conn, "budget_", userB)
	for range 2 {
		if err := f.svc.DeleteUserData(ctx, userA); err != nil {
			t.Fatalf("DeleteUserData: %v", err)
		}
	}
	dbtest.AssertGone(t, f.conn, "budget_", userA)
	for table, n := range dbtest.UserRows(t, f.conn, "budget_", userB) {
		if n != before[table] {
			t.Errorf("user B %s: %d rows, was %d", table, n, before[table])
		}
	}
}
