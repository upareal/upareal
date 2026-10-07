// story: e05s03
package budget_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/upareal/upareal/budget"
)

func month(t *testing.T, s string) budget.Month {
	t.Helper()
	m, err := budget.ParseMonth(s)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestMonth_ParseFormatAndNavigate(t *testing.T) {
	for _, bad := range []string{"", "2026-13", "2026-00", "2026-1", "26-01", "2026/01", "2026-01-01", "abcd-ef"} {
		if _, err := budget.ParseMonth(bad); !errors.Is(err, budget.ErrInvalidMonth) {
			t.Errorf("ParseMonth(%q) = %v, want ErrInvalidMonth", bad, err)
		}
	}
	m := month(t, "2026-01")
	if m.String() != "2026-01" || m.Prev().String() != "2025-12" || m.Next().String() != "2026-02" {
		t.Errorf("2026-01: String %s Prev %s Next %s", m, m.Prev(), m.Next())
	}
	if month(t, "2026-12").Next().String() != "2027-01" {
		t.Error("December.Next() is not next January")
	}
}

func TestMonth_OfCountsInSaoPaulo(t *testing.T) {
	// 22:30 on 31 Jan in São Paulo is already 1 Feb in UTC; it belongs to January.
	utc := time.Date(2026, 2, 1, 1, 30, 0, 0, time.UTC)
	if got := budget.MonthOf(utc).String(); got != "2026-01" {
		t.Errorf("MonthOf(%s) = %s, want 2026-01 (São Paulo)", utc, got)
	}
}

func TestMonthlyAmounts_SetAndRead(t *testing.T) {
	f := seeded(t)
	ctx := context.Background()
	jan := month(t, "2026-01")
	fuel := categoryNamed(t, f, "Combustível").ID
	salary := categoryNamed(t, f, "Salário").ID

	if err := f.svc.SetAmount(ctx, userA, fuel, jan, 30000); err != nil {
		t.Fatalf("SetAmount: %v", err)
	}
	if err := f.svc.SetAmount(ctx, userA, salary, jan, 800000); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.SetAmount(ctx, userA, fuel, jan, 32000); err != nil { // replaces (B3)
		t.Fatal(err)
	}
	amounts, err := f.svc.OpenMonth(ctx, userA, jan)
	if err != nil {
		t.Fatalf("OpenMonth: %v", err)
	}
	if amounts[fuel] != 32000 || amounts[salary] != 800000 {
		t.Errorf("amounts = %v, want fuel 32000, salary 800000", amounts)
	}
	if n := f.count(t, `SELECT count(*) FROM budget_amount WHERE category_id = ?`, fuel); n != 1 {
		t.Errorf("rows for one category and month = %d, want 1 (B3)", n)
	}
	if err := f.svc.SetAmount(ctx, userA, fuel, jan, 0); err != nil {
		t.Errorf("zero amount: %v, want allowed", err)
	}
}

func TestMonthlyAmounts_RejectsNegativeAndForeign(t *testing.T) {
	f := seeded(t)
	ctx := context.Background()
	jan := month(t, "2026-01")
	fuel := categoryNamed(t, f, "Combustível").ID
	if err := f.svc.SetAmount(ctx, userA, fuel, jan, -1); !errors.Is(err, budget.ErrInvalidAmount) {
		t.Errorf("negative amount = %v, want ErrInvalidAmount (B4)", err)
	}
	if err := f.svc.SetAmount(ctx, userB, fuel, jan, 100); !errors.Is(err, budget.ErrNotFound) {
		t.Errorf("user B setting user A's category = %v, want ErrNotFound", err)
	}
	amountsB, err := f.svc.OpenMonth(ctx, userB, jan)
	if err != nil {
		t.Fatal(err)
	}
	if _, leaked := amountsB[fuel]; leaked {
		t.Error("user B sees an amount for user A's category")
	}
}
