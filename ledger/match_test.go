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
