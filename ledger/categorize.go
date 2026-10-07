// story: e07s02
package ledger

import (
	"context"
	"errors"

	"github.com/upareal/upareal/ledger/internal/store"
)

// ErrNotQueued means the transaction is not waiting on "A categorizar".
var ErrNotQueued = errors.New("ledger: transaction is not in the review queue")

// Categorize replaces the "A categorizar" leg of a queued (imported) line
// with the given expense/income accounts. The money account keeps the
// imported amount: the lines must add up to it. Time and description stay;
// the previous version goes to history.
func (s *Service) Categorize(ctx context.Context, userID, txID string, parts []Part) error {
	return s.inTx(ctx, func(q *store.Queries) error {
		cur, err := s.loadTransaction(ctx, q, userID, txID)
		if err != nil {
			return err
		}
		money, ok := queuedMoneyLeg(cur)
		if !ok {
			return ErrNotQueued
		}
		legs, err := s.entryLegs(ctx, q, userID, SplitEntry{AccountID: money.AccountID, Total: -money.Amount, Parts: parts}, 1)
		if err != nil {
			return err
		}
		return s.replaceTransaction(ctx, q, userID, cur, cur.At, cur.Description, legs)
	})
}

// queuedMoneyLeg returns the single asset/liability leg of a queued
// transaction: one money leg, every other leg on an "A categorizar" account.
func queuedMoneyLeg(tx Transaction) (TxLeg, bool) {
	var money []TxLeg
	for _, l := range tx.Legs {
		switch {
		case l.SystemRole == roleUncategorizedExpense || l.SystemRole == roleUncategorizedIncome:
		case l.SystemRole == "" && (l.AccountType == "asset" || l.AccountType == "liability"):
			money = append(money, l)
		default:
			return TxLeg{}, false
		}
	}
	if len(money) != 1 || len(tx.Legs) < 2 {
		return TxLeg{}, false
	}
	return money[0], true
}
