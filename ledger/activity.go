// story: e08s02 e08s05
package ledger

import (
	"context"
	"fmt"
	"time"

	"github.com/upareal/upareal/ledger/internal/store"
)

// Activity returns, per account of userID, the debit-positive sum of legs on
// live transactions that occurred in [from, to). Accounts without legs in
// the range are absent.
func (s *Service) Activity(ctx context.Context, userID string, from, to time.Time) (map[string]int64, error) {
	rows, err := s.q.ActivityBetween(ctx, store.ActivityBetweenParams{UserID: userID, FromAt: stamp(from), ToAt: stamp(to)})
	if err != nil {
		return nil, fmt.Errorf("ledger: activity: %w", err)
	}
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.AccountID] = r.Amount
	}
	return out, nil
}

// Flow is the money an account exchanged with on-budget accounts: the sum
// of its positive legs (Debit >= 0) and of its negative legs (Credit <= 0).
// story: e08s05
type Flow struct{ Debit, Credit int64 }

// BudgetFlows returns, per account of userID that is not on-budget, its
// legs in live transactions in [from, to) that also touch an on-budget
// account: the money crossing the budget boundary. Off-budget-only
// transactions (an investment's yield or opening balance) are left out.
func (s *Service) BudgetFlows(ctx context.Context, userID string, from, to time.Time) (map[string]Flow, error) {
	rows, err := s.q.BudgetFlows(ctx, store.BudgetFlowsParams{UserID: userID, FromAt: stamp(from), ToAt: stamp(to)})
	if err != nil {
		return nil, fmt.Errorf("ledger: budget flows: %w", err)
	}
	out := make(map[string]Flow, len(rows))
	for _, r := range rows {
		out[r.AccountID] = Flow{Debit: r.Debit, Credit: r.Credit}
	}
	return out, nil
}
