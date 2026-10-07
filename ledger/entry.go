// story: e06s01 e07s02
package ledger

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/upareal/upareal/ledger/internal/store"
)

const maxDescription = 200

// ErrInvalidEntry wraps every rejected entry; the message says why.
var ErrInvalidEntry = errors.New("ledger: invalid entry")

// SimpleEntry is one expense or income on an asset or liability account.
// Amount is positive; the counter account's type decides the direction.
type SimpleEntry struct {
	AccountID   string // asset or liability (where the money is)
	CounterID   string // expense or income (where it went / came from)
	Amount      int64  // > 0, centavos
	At          time.Time
	Description string
}

// RecordSimple posts a two-leg transaction:
// expense A → counter +A, account −A; income A → counter −A, account +A.
//
//	id, err := svc.RecordSimple(ctx, userID, ledger.SimpleEntry{AccountID: itau, CounterID: shell, Amount: 15000, At: now})
func (s *Service) RecordSimple(ctx context.Context, userID string, e SimpleEntry) (string, error) {
	if e.Amount <= 0 {
		return "", fmt.Errorf("%w: amount must be greater than zero", ErrInvalidEntry)
	}
	if err := validEntryText(e.At, e.Description); err != nil {
		return "", err
	}
	var id string
	err := s.inTx(ctx, func(q *store.Queries) error {
		acc, err := s.account(ctx, q, userID, e.AccountID)
		if err != nil {
			return err
		}
		counter, err := s.account(ctx, q, userID, e.CounterID)
		if err != nil {
			return err
		}
		if acc.Type != "asset" && acc.Type != "liability" {
			return fmt.Errorf("%w: the account must be an asset or liability", ErrInvalidEntry)
		}
		if counter.SystemRole.Valid {
			return fmt.Errorf("%w: system accounts cannot be chosen", ErrInvalidEntry)
		}
		sign := int64(1) // expense: money flows into the expense account
		switch counter.Type {
		case "expense":
		case "income":
			sign = -1
		default:
			return fmt.Errorf("%w: the counter account must be an income or expense", ErrInvalidEntry)
		}
		id, err = s.postTransaction(ctx, q, userID, e.At, e.Description, []Leg{
			{AccountID: e.CounterID, Amount: sign * e.Amount},
			{AccountID: e.AccountID, Amount: -sign * e.Amount},
		})
		return err
	})
	return id, err
}

// validEntryText checks the parts every entry shares.
func validEntryText(at time.Time, description string) error {
	if at.IsZero() {
		return fmt.Errorf("%w: date and time are required", ErrInvalidEntry)
	}
	if utf8.RuneCountInString(description) > maxDescription {
		return fmt.Errorf("%w: description must have at most %d characters", ErrInvalidEntry, maxDescription)
	}
	return nil
}
