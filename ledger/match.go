// story: e07s03
package ledger

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/upareal/upareal/ledger/internal/store"
)

const matchWindow = 3 * 24 * time.Hour

// TransferCandidates lists queued lines on otherAccountID that could be the
// other side of queued line txID: opposite amount, within ±3 days, closest first.
func (s *Service) TransferCandidates(ctx context.Context, userID, txID, otherAccountID string) ([]QueuedLine, error) {
	tx, money, err := s.queuedLine(ctx, s.q, userID, txID)
	if err != nil {
		return nil, err
	}
	if err := s.checkTransferTarget(ctx, s.q, userID, money.AccountID, otherAccountID); err != nil {
		return nil, err
	}
	queue, err := s.ReviewQueue(ctx, userID)
	if err != nil {
		return nil, err
	}
	var out []QueuedLine
	for _, q := range queue {
		if q.AccountID == otherAccountID && q.Amount == -money.Amount && absDuration(q.At.Sub(tx.At)) <= matchWindow {
			out = append(out, q)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return absDuration(out[i].At.Sub(tx.At)) < absDuration(out[j].At.Sub(tx.At))
	})
	return out, nil
}

// queuedLine loads a queued transaction and its single money leg.
func (s *Service) queuedLine(ctx context.Context, q *store.Queries, userID, txID string) (Transaction, TxLeg, error) {
	tx, err := s.loadTransaction(ctx, q, userID, txID)
	if err != nil {
		return Transaction{}, TxLeg{}, err
	}
	money, ok := queuedMoneyLeg(tx)
	if !ok {
		return Transaction{}, TxLeg{}, ErrNotQueued
	}
	return tx, money, nil
}

// checkTransferTarget requires another asset/liability account of the user.
func (s *Service) checkTransferTarget(ctx context.Context, q *store.Queries, userID, fromID, otherID string) error {
	if otherID == fromID {
		return fmt.Errorf("%w: choose a different account", ErrInvalidEntry)
	}
	other, err := s.account(ctx, q, userID, otherID)
	if err != nil {
		return err
	}
	if other.SystemRole.Valid || (other.Type != "asset" && other.Type != "liability") {
		return fmt.Errorf("%w: transfers go to asset or liability accounts", ErrInvalidEntry)
	}
	return nil
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
