// story: e07s03
package ledger_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/upareal/upareal/ledger"
	"github.com/upareal/upareal/ledger/importer"
)

func day(d int) time.Time { return time.Date(2026, 2, d, 15, 0, 0, 0, time.UTC) }

// matchBook imports Itaú's TED out and several Poupança lines; returns IDs by description.
func matchBook(t *testing.T, f fixture) (book, ledger.Account, map[string]string) {
	t.Helper()
	ctx := context.Background()
	b := newBook(t, f)
	poupanca := create(t, f, ledger.NewAccount{Name: "Poupança", Type: "asset"})
	if _, err := f.svc.Import(ctx, userA, b.itau.ID, []importer.Line{{ImportID: "t1", At: day(10), Description: "TED para poupança", Amount: -100000}}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Import(ctx, userA, poupanca.ID, []importer.Line{
		{ImportID: "p1", At: day(11), Description: "TED recebida", Amount: 100000},
		{ImportID: "p5", At: day(13), Description: "Outra TED", Amount: 100000},
		{ImportID: "p2", At: day(20), Description: "Fora da janela", Amount: 100000},
		{ImportID: "p3", At: day(11), Description: "Valor diferente", Amount: 50000},
		{ImportID: "p4", At: day(9), Description: "Mesmo sentido", Amount: -100000},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Transfer(ctx, userA, ledger.TransferEntry{FromID: b.itau.ID, ToID: poupanca.ID, Amount: 100000, At: day(11), Description: "Manual"}); err != nil {
		t.Fatal(err)
	}
	q, _ := f.svc.ReviewQueue(ctx, userA)
	ids := map[string]string{}
	for _, l := range q {
		ids[l.Description] = l.TransactionID
	}
	return b, poupanca, ids
}

func TestTransferCandidates_OppositeAmountWithinThreeDays(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, poupanca, ids := matchBook(t, f)
	got, err := f.svc.TransferCandidates(ctx, userA, ids["TED para poupança"], poupanca.ID)
	if err != nil {
		t.Fatalf("TransferCandidates: %v", err)
	}
	if len(got) != 2 || got[0].Description != "TED recebida" || got[1].Description != "Outra TED" {
		var d []string
		for _, g := range got {
			d = append(d, g.Description)
		}
		t.Errorf("candidates = %v, want [TED recebida, Outra TED] (closest first; queued, opposite amount, ±3 days)", d)
	}
}

func TestTransferCandidates_GuardsAndIsolation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b, poupanca, ids := matchBook(t, f)
	if _, err := f.svc.TransferCandidates(ctx, userA, ids["TED para poupança"], b.itau.ID); !errors.Is(err, ledger.ErrInvalidEntry) {
		t.Errorf("same account = %v, want ErrInvalidEntry", err)
	}
	if _, err := f.svc.TransferCandidates(ctx, userA, ids["TED para poupança"], b.shell.ID); !errors.Is(err, ledger.ErrInvalidEntry) {
		t.Errorf("expense as the other account = %v, want ErrInvalidEntry", err)
	}
	manual := expense(t, f, b, 100, "x")
	if _, err := f.svc.TransferCandidates(ctx, userA, manual, poupanca.ID); !errors.Is(err, ledger.ErrNotQueued) {
		t.Errorf("non-queued line = %v, want ErrNotQueued", err)
	}
	_ = f.svc.EnsureDefaults(ctx, userB)
	if _, err := f.svc.TransferCandidates(ctx, userB, ids["TED para poupança"], poupanca.ID); !errors.Is(err, ledger.ErrTransactionNotFound) {
		t.Errorf("user B = %v, want ErrTransactionNotFound", err)
	}
}

func TestTransferMerge_WithTwinMakesOneTransfer(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b, poupanca, ids := matchBook(t, f)
	itauBefore, poupBefore := balance(t, f, b.itau.ID), balance(t, f, poupanca.ID)

	if err := f.svc.MarkTransfer(ctx, userA, ids["TED para poupança"], poupanca.ID, ids["TED recebida"]); err != nil {
		t.Fatalf("MarkTransfer: %v", err)
	}
	// Before: Itaú -100000 to A categorizar, Poupança +100000 from A categorizar.
	// After: one transfer, same money positions, nothing left on A categorizar from these two.
	if balance(t, f, b.itau.ID) != itauBefore || balance(t, f, poupanca.ID) != poupBefore {
		t.Errorf("balances changed: Itaú %d→%d, Poupança %d→%d", itauBefore, balance(t, f, b.itau.ID), poupBefore, balance(t, f, poupanca.ID))
	}
	q, _ := f.svc.ReviewQueue(ctx, userA)
	for _, l := range q {
		if l.TransactionID == ids["TED para poupança"] || l.TransactionID == ids["TED recebida"] {
			t.Errorf("%q still in the queue", l.Description)
		}
	}
	if _, err := f.svc.Transaction(ctx, userA, ids["TED recebida"]); !errors.Is(err, ledger.ErrTransactionNotFound) {
		t.Error("the twin was not removed")
	}
	tx, _ := f.svc.Transaction(ctx, userA, ids["TED para poupança"])
	if legOf(tx, poupanca.ID).Amount != 100000 || legOf(tx, b.itau.ID).Amount != -100000 || len(tx.Legs) != 2 {
		t.Errorf("transfer legs = %+v, want Itaú -100000 / Poupança +100000", tx.Legs)
	}
	// Re-importing either statement adds nothing.
	if r, _ := f.svc.Import(ctx, userA, b.itau.ID, []importer.Line{{ImportID: "t1", At: day(10), Amount: -100000}}); r.Imported != 0 {
		t.Error("re-importing Itaú brought the line back")
	}
	if r, _ := f.svc.Import(ctx, userA, poupanca.ID, []importer.Line{{ImportID: "p1", At: day(11), Amount: 100000}}); r.Imported != 0 {
		t.Error("re-importing Poupança brought the twin back")
	}
}

func TestTransferMerge_WithoutTwin(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b, poupanca, ids := matchBook(t, f)
	if err := f.svc.MarkTransfer(ctx, userA, ids["TED para poupança"], poupanca.ID, ""); err != nil {
		t.Fatalf("MarkTransfer without twin: %v", err)
	}
	tx, _ := f.svc.Transaction(ctx, userA, ids["TED para poupança"])
	if len(tx.Legs) != 2 || legOf(tx, poupanca.ID).Amount != 100000 || legOf(tx, b.itau.ID).Amount != -100000 {
		t.Errorf("legs = %+v, want Itaú -100000 / Poupança +100000 (no A categorizar)", tx.Legs)
	}
	q, _ := f.svc.ReviewQueue(ctx, userA)
	for _, l := range q {
		if l.TransactionID == ids["TED para poupança"] {
			t.Error("the line is still in the queue")
		}
	}
}

func TestTransferMerge_RejectsInvalidMatch(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, poupanca, ids := matchBook(t, f)
	for name, match := range map[string]string{
		"outside the window": ids["Fora da janela"],
		"different amount":   ids["Valor diferente"],
		"unknown":            "no-such",
	} {
		if err := f.svc.MarkTransfer(ctx, userA, ids["TED para poupança"], poupanca.ID, match); !errors.Is(err, ledger.ErrInvalidMatch) {
			t.Errorf("%s = %v, want ErrInvalidMatch", name, err)
		}
	}
	if q, _ := f.svc.ReviewQueue(ctx, userA); len(q) != 6 {
		t.Errorf("queue = %d after rejected matches, want 6 unchanged", len(q))
	}
}
