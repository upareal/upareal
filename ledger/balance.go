// story: e04s02
package ledger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/upareal/upareal/ledger/internal/store"
)

const adjustmentDescription = "Ajuste de saldo"

// ErrAccountNotFound means the account does not exist for this user.
var ErrAccountNotFound = errors.New("ledger: account not found")

// Balance returns the debit-positive balance of one of userID's accounts:
// the sum of its legs (L4), never stored.
func (s *Service) Balance(ctx context.Context, userID, accountID string) (int64, error) {
	if _, err := s.account(ctx, s.q, userID, accountID); err != nil {
		return 0, err
	}
	b, err := s.q.AccountBalance(ctx, store.AccountBalanceParams{UserID: userID, AccountID: accountID})
	if err != nil {
		return 0, fmt.Errorf("ledger: balance: %w", err)
	}
	return b, nil
}

// Balances returns the debit-positive balance of every account of userID
// that has legs; accounts without legs are absent (balance 0).
func (s *Service) Balances(ctx context.Context, userID string) (map[string]int64, error) {
	rows, err := s.q.AccountBalances(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("ledger: balances: %w", err)
	}
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.AccountID] = r.Balance
	}
	return out, nil
}

// AdjustBalance sets an asset or liability account to target (debit-positive)
// by posting the difference against Saldo inicial. No-op when already there.
func (s *Service) AdjustBalance(ctx context.Context, userID, accountID string, target int64, at time.Time) error {
	return s.inTx(ctx, func(q *store.Queries) error {
		acc, err := s.account(ctx, q, userID, accountID)
		if err != nil {
			return err
		}
		if acc.SystemRole.Valid || (acc.Type != "asset" && acc.Type != "liability") {
			return fmt.Errorf("%w: only asset and liability accounts can be adjusted", ErrInvalidAccount)
		}
		current, err := q.AccountBalance(ctx, store.AccountBalanceParams{UserID: userID, AccountID: accountID})
		if err != nil {
			return fmt.Errorf("ledger: balance: %w", err)
		}
		diff, overflow := addInt64(target, -current)
		if overflow || current == minInt64 {
			return fmt.Errorf("%w: adjustment out of range", ErrInvalidAccount)
		}
		if diff == 0 {
			return nil
		}
		return s.postAgainstOpeningBalance(ctx, q, userID, accountID, diff, at, adjustmentDescription)
	})
}

const minInt64 = -1 << 63

// account loads one of userID's accounts or returns ErrAccountNotFound.
func (s *Service) account(ctx context.Context, q *store.Queries, userID, accountID string) (store.GetAccountRow, error) {
	acc, err := q.GetAccount(ctx, store.GetAccountParams{ID: accountID, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return store.GetAccountRow{}, ErrAccountNotFound
	}
	if err != nil {
		return store.GetAccountRow{}, fmt.Errorf("ledger: load account: %w", err)
	}
	return acc, nil
}
