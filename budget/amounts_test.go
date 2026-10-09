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

func TestCopyPreviousMonth_FirstOpenCopiesLatestEarlierMonth(t *testing.T) {
	f := seeded(t)
	ctx := context.Background()
	fuel := categoryNamed(t, f, "Combustível").ID
	_ = f.svc.SetAmount(ctx, userA, fuel, month(t, "2026-01"), 30000)

	feb, err := f.svc.OpenMonth(ctx, userA, month(t, "2026-02"))
	if err != nil || feb[fuel] != 30000 {
		t.Fatalf("first open of 2026-02 = %v, %v; want Combustível 30000 copied", feb, err)
	}
	// Skipped month: April copies from February, the latest opened month before it.
	_ = f.svc.SetAmount(ctx, userA, fuel, month(t, "2026-02"), 31000)
	apr, _ := f.svc.OpenMonth(ctx, userA, month(t, "2026-04"))
	if apr[fuel] != 31000 {
		t.Errorf("2026-04 = %d, want 31000 from 2026-02", apr[fuel])
	}
}

func TestCopyPreviousMonth_CopiesOnlyOnce(t *testing.T) {
	f := seeded(t)
	ctx := context.Background()
	fuel := categoryNamed(t, f, "Combustível").ID
	_ = f.svc.SetAmount(ctx, userA, fuel, month(t, "2026-01"), 30000)
	_, _ = f.svc.OpenMonth(ctx, userA, month(t, "2026-02"))

	_ = f.svc.SetAmount(ctx, userA, fuel, month(t, "2026-01"), 35000) // edit January later
	feb, _ := f.svc.OpenMonth(ctx, userA, month(t, "2026-02"))
	if feb[fuel] != 30000 {
		t.Errorf("2026-02 after editing January = %d, want 30000 (copied once)", feb[fuel])
	}
	_ = f.svc.SetAmount(ctx, userA, fuel, month(t, "2026-02"), 0) // deliberately zero
	feb, _ = f.svc.OpenMonth(ctx, userA, month(t, "2026-02"))
	if v, ok := feb[fuel]; !ok || v != 0 {
		t.Errorf("2026-02 deliberately zero = %d (present %v), want 0 kept", v, ok)
	}
}

func TestCopyPreviousMonth_NoEarlierMonthOpensEmpty(t *testing.T) {
	f := seeded(t)
	got, err := f.svc.OpenMonth(context.Background(), userA, month(t, "2026-01"))
	if err != nil || len(got) != 0 {
		t.Errorf("first ever month = %v, %v; want empty", got, err)
	}
}

func TestCopyPreviousMonth_NeverCopiesAcrossUsers(t *testing.T) {
	f := seeded(t)
	ctx := context.Background()
	_ = f.svc.SetAmount(ctx, userA, categoryNamed(t, f, "Combustível").ID, month(t, "2026-01"), 30000)
	got, _ := f.svc.OpenMonth(ctx, userB, month(t, "2026-02"))
	if len(got) != 0 {
		t.Errorf("user B's new month = %v, want empty (nothing copied from user A)", got)
	}
}

// story: e08s03
func TestAmountsBefore_SumsEarlierMonthsPerCategory(t *testing.T) {
	f := seeded(t)
	ctx := context.Background()
	fuel := categoryNamed(t, f, "Combustível").ID
	for m, v := range map[string]int64{"2025-12": 10000, "2026-01": 30000, "2026-02": 99900} {
		if err := f.svc.SetAmount(ctx, userA, fuel, month(t, m), v); err != nil {
			t.Fatal(err)
		}
	}
	got, err := f.svc.AmountsBefore(ctx, userA, month(t, "2026-02"))
	if err != nil {
		t.Fatalf("AmountsBefore: %v", err)
	}
	if got[fuel] != 40000 {
		t.Errorf("before February = %d, want 40000 (December + January, February excluded)", got[fuel])
	}
	if other, err := f.svc.AmountsBefore(ctx, userB, month(t, "2026-02")); err != nil || len(other) != 0 {
		t.Errorf("user B = %v, %v; want empty", other, err)
	}
}

// story: e14s01
func TestAmounts_ReadsWithoutOpening(t *testing.T) {
	f := seeded(t)
	ctx := context.Background()
	fuel := categoryNamed(t, f, "Combustível").ID
	if err := f.svc.SetAmount(ctx, userA, fuel, month(t, "2026-01"), 30000); err != nil {
		t.Fatal(err)
	}
	amounts, opened, err := f.svc.Amounts(ctx, userA, month(t, "2026-01"))
	if err != nil || !opened || amounts[fuel] != 30000 {
		t.Errorf("January = %v, opened %v, %v; want fuel 30000, opened", amounts, opened, err)
	}
	before := f.count(t, `SELECT count(*) FROM budget_month`)
	amounts, opened, err = f.svc.Amounts(ctx, userA, month(t, "2026-03"))
	if err != nil || opened || len(amounts) != 0 {
		t.Errorf("unopened March = %v, opened %v, %v; want empty, not opened", amounts, opened, err)
	}
	if after := f.count(t, `SELECT count(*) FROM budget_month`); after != before {
		t.Errorf("Amounts opened a month: %d → %d rows", before, after)
	}
}
