// story: e06s03 e06s04
package ledger

import (
	"context"
	"fmt"
	"time"

	"github.com/upareal/upareal/ledger/internal/store"
)

// TransferEntry moves Amount between two of the user's asset/liability accounts.
type TransferEntry struct {
	FromID, ToID string
	Amount       int64 // > 0
	At           time.Time
	Description  string
}

// Transfer posts destination +A, source −A. Paying a card from a bank account
// is a transfer: the card's debt goes down.
//
//	id, err := svc.Transfer(ctx, userID, ledger.TransferEntry{FromID: itau, ToID: nubank, Amount: 80000, At: now})
func (s *Service) Transfer(ctx context.Context, userID string, e TransferEntry) (string, error) {
	var id string
	err := s.inTx(ctx, func(q *store.Queries) error {
		legs, err := s.transferLegs(ctx, q, userID, e)
		if err != nil {
			return err
		}
		id, err = s.postTransaction(ctx, q, userID, e.At, e.Description, legs)
		return err
	})
	return id, err
}

// transferLegs checks a transfer and returns destination +A, source −A.
func (s *Service) transferLegs(ctx context.Context, q *store.Queries, userID string, e TransferEntry) ([]Leg, error) {
	if e.Amount <= 0 {
		return nil, fmt.Errorf("%w: amount must be greater than zero", ErrInvalidEntry)
	}
	if e.FromID == e.ToID {
		return nil, fmt.Errorf("%w: source and destination must be different", ErrInvalidEntry)
	}
	if err := validEntryText(e.At, e.Description); err != nil {
		return nil, err
	}
	for _, accID := range []string{e.FromID, e.ToID} {
		acc, err := s.account(ctx, q, userID, accID)
		if err != nil {
			return nil, err
		}
		if acc.Type != "asset" && acc.Type != "liability" {
			return nil, fmt.Errorf("%w: transfers move money between asset and liability accounts", ErrInvalidEntry)
		}
	}
	return []Leg{{AccountID: e.ToID, Amount: e.Amount}, {AccountID: e.FromID, Amount: -e.Amount}}, nil
}
