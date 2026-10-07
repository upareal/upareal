// story: e07s03
package ledger

import (
	"context"
	"database/sql"
	"errors"
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

// ErrInvalidMatch means the chosen twin is not a valid candidate (any more).
var ErrInvalidMatch = errors.New("ledger: the chosen line is not a matching transfer")

// MarkTransfer turns queued line txID into a transfer between its account and
// otherAccountID. When matchTxID is set, it must be a current candidate: that
// twin (the same money seen from the other statement) is soft-deleted, keeping
// its import record so re-importing still skips it. All in one transaction.
func (s *Service) MarkTransfer(ctx context.Context, userID, txID, otherAccountID, matchTxID string) error {
	return s.inTx(ctx, func(q *store.Queries) error {
		tx, money, err := s.queuedLine(ctx, q, userID, txID)
		if err != nil {
			return err
		}
		if err := s.checkTransferTarget(ctx, q, userID, money.AccountID, otherAccountID); err != nil {
			return err
		}
		var twin Transaction
		if matchTxID != "" {
			if twin, err = s.validTwin(ctx, q, userID, tx, money, otherAccountID, matchTxID); err != nil {
				return err
			}
		}
		legs := []Leg{{AccountID: money.AccountID, Amount: money.Amount}, {AccountID: otherAccountID, Amount: -money.Amount}}
		if err := s.replaceTransaction(ctx, q, userID, tx, tx.At, tx.Description, legs); err != nil {
			return err
		}
		if matchTxID == "" {
			return nil
		}
		dk, err := s.userKey(ctx, q, userID)
		if err != nil {
			return err
		}
		if err := s.recordHistory(ctx, q, dk, userID, twin, "delete"); err != nil {
			return err
		}
		return q.SoftDeleteTransaction(ctx, store.SoftDeleteTransactionParams{
			DeletedAt: sql.NullString{String: stamp(s.now()), Valid: true}, ID: twin.ID, UserID: userID,
		})
	})
}

// validTwin checks that matchTxID is a queued line on otherAccountID with the
// opposite amount within the match window.
func (s *Service) validTwin(ctx context.Context, q *store.Queries, userID string, tx Transaction, money TxLeg, otherAccountID, matchTxID string) (Transaction, error) {
	if matchTxID == tx.ID {
		return Transaction{}, ErrInvalidMatch
	}
	twin, twinMoney, err := s.queuedLine(ctx, q, userID, matchTxID)
	if errors.Is(err, ErrTransactionNotFound) || errors.Is(err, ErrNotQueued) {
		return Transaction{}, ErrInvalidMatch
	}
	if err != nil {
		return Transaction{}, err
	}
	if twinMoney.AccountID != otherAccountID || twinMoney.Amount != -money.Amount || absDuration(twin.At.Sub(tx.At)) > matchWindow {
		return Transaction{}, ErrInvalidMatch
	}
	return twin, nil
}
