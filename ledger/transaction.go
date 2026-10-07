// story: e04s02
package ledger

import (
	"context"
	"errors"
	"fmt"
	"math/bits"
	"time"

	"github.com/upareal/upareal/ledger/internal/store"
)

const fieldDescription = "description"

// Errors for transactions that would break a ledger invariant.
var (
	ErrTooFewLegs         = errors.New("ledger: a transaction needs at least 2 legs")                       // L1
	ErrUnbalanced         = errors.New("ledger: transaction legs do not sum to zero")                       // L1
	ErrZeroLeg            = errors.New("ledger: a leg amount cannot be zero")                               // L2
	ErrAccountNotPostable = errors.New("ledger: account is unknown, archived or belongs to another user") // L3, L6
)

// Leg moves Amount centavos (debit-positive) into or out of AccountID.
type Leg struct {
	AccountID string
	Amount    int64
}

// postTransaction is the ONLY way legs are written. It enforces L1–L3 and L6
// inside the caller's database transaction, so an invalid transaction leaves
// nothing behind.
func (s *Service) postTransaction(ctx context.Context, q *store.Queries, userID string, at time.Time, description string, legs []Leg) (string, error) {
	if err := checkLegs(legs); err != nil {
		return "", err
	}
	ids := map[string]bool{}
	for _, l := range legs {
		ids[l.AccountID] = true
	}
	distinct := make([]string, 0, len(ids))
	for id := range ids {
		distinct = append(distinct, id)
	}
	n, err := q.CountPostableAccounts(ctx, store.CountPostableAccountsParams{UserID: userID, Ids: distinct})
	if err != nil {
		return "", fmt.Errorf("ledger: check accounts: %w", err)
	}
	if int(n) != len(distinct) {
		return "", ErrAccountNotPostable
	}

	dk, err := s.userKey(ctx, q, userID)
	if err != nil {
		return "", err
	}
	txID, sealed, err := newSealedField(dk, fieldDescription, description)
	if err != nil {
		return "", err
	}
	err = q.CreateTransaction(ctx, store.CreateTransactionParams{
		ID: txID, UserID: userID, OccurredAt: stamp(at), Description: sealed, CreatedAt: stamp(s.now()),
	})
	if err != nil {
		return "", fmt.Errorf("ledger: create transaction: %w", err)
	}
	for _, l := range legs {
		legID, err := newID()
		if err != nil {
			return "", err
		}
		if err := q.CreateLeg(ctx, store.CreateLegParams{ID: legID, TransactionID: txID, AccountID: l.AccountID, Amount: l.Amount}); err != nil {
			return "", fmt.Errorf("ledger: create leg: %w", err)
		}
	}
	return txID, nil
}

// checkLegs enforces L1 and L2 without touching the database.
func checkLegs(legs []Leg) error {
	if len(legs) < 2 {
		return ErrTooFewLegs
	}
	var sum int64
	for _, l := range legs {
		if l.Amount == 0 {
			return ErrZeroLeg
		}
		next, overflow := addInt64(sum, l.Amount)
		if overflow {
			return fmt.Errorf("%w (amounts overflow)", ErrUnbalanced)
		}
		sum = next
	}
	if sum != 0 {
		return ErrUnbalanced
	}
	return nil
}

// addInt64 adds with overflow detection.
func addInt64(a, b int64) (int64, bool) {
	s, _ := bits.Add64(uint64(a), uint64(b), 0)
	r := int64(s)
	return r, (a >= 0) == (b >= 0) && (r >= 0) != (a >= 0)
}
