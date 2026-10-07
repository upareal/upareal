// story: e06s03
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
	if e.Amount <= 0 {
		return "", fmt.Errorf("%w: amount must be greater than zero", ErrInvalidEntry)
	}
	if e.FromID == e.ToID {
		return "", fmt.Errorf("%w: source and destination must be different", ErrInvalidEntry)
	}
	if err := validEntryText(e.At, e.Description); err != nil {
		return "", err
	}
	var id string
	err := s.inTx(ctx, func(q *store.Queries) error {
		for _, accID := range []string{e.FromID, e.ToID} {
			acc, err := s.account(ctx, q, userID, accID)
			if err != nil {
				return err
			}
			if acc.Type != "asset" && acc.Type != "liability" {
				return fmt.Errorf("%w: transfers move money between asset and liability accounts", ErrInvalidEntry)
			}
		}
		var err error
		id, err = s.postTransaction(ctx, q, userID, e.At, e.Description, []Leg{
			{AccountID: e.ToID, Amount: e.Amount}, {AccountID: e.FromID, Amount: -e.Amount},
		})
		return err
	})
	return id, err
}
