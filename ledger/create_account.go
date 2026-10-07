// story: e04s02
package ledger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/upareal/upareal/ledger/internal/store"
)

const (
	maxAccountName     = 100
	openingDescription = "Saldo inicial"
)

// ErrInvalidAccount wraps every rejected NewAccount; the message says why.
var ErrInvalidAccount = errors.New("ledger: invalid account")

// NewAccount describes an account to create. Opening is the debit-positive
// starting balance (assets positive, debts negative) and is only allowed for
// asset and liability accounts.
type NewAccount struct {
	Name      string
	Type      string // asset, liability, income, expense
	OnBudget  bool   // asset/liability; forced on for cards, ignored otherwise
	IsCard    bool   // liability only
	Opening   int64
	OpeningAt time.Time
}

// CreateAccount creates a user account and, for a non-zero Opening, one
// opening transaction against Saldo inicial — atomically.
//
//	acc, err := svc.CreateAccount(ctx, userID, ledger.NewAccount{Name: "Conta corrente", Type: "asset", OnBudget: true, Opening: 250000, OpeningAt: now})
func (s *Service) CreateAccount(ctx context.Context, userID string, n NewAccount) (Account, error) {
	n.Name = strings.TrimSpace(n.Name)
	if err := validateNewAccount(&n); err != nil {
		return Account{}, err
	}
	var acc Account
	err := s.inTx(ctx, func(q *store.Queries) error {
		dk, err := s.userKey(ctx, q, userID)
		if err != nil {
			return err
		}
		pos, err := q.NextAccountPosition(ctx, store.NextAccountPositionParams{UserID: userID, Type: n.Type})
		if err != nil {
			return fmt.Errorf("ledger: next position: %w", err)
		}
		id, err := s.insertAccount(ctx, q, dk, userID, accountRow{
			accountType: n.Type, name: n.Name, position: pos, onBudget: n.OnBudget, isCard: n.IsCard,
		})
		if err != nil {
			return err
		}
		acc = Account{ID: id, Type: n.Type, Name: n.Name, OnBudget: n.OnBudget, IsCard: n.IsCard}
		if n.Opening == 0 {
			return nil
		}
		if n.OpeningAt.IsZero() {
			n.OpeningAt = s.now()
		}
		return s.postAgainstOpeningBalance(ctx, q, userID, id, n.Opening, n.OpeningAt, openingDescription)
	})
	return acc, err
}

// validateNewAccount enforces the account rules and normalizes the flags.
func validateNewAccount(n *NewAccount) error {
	if n.Name == "" || utf8.RuneCountInString(n.Name) > maxAccountName {
		return fmt.Errorf("%w: name must have 1 to %d characters", ErrInvalidAccount, maxAccountName)
	}
	switch n.Type {
	case "asset":
		if n.IsCard {
			return fmt.Errorf("%w: only liability accounts can be cards", ErrInvalidAccount)
		}
	case "liability":
		if n.IsCard {
			n.OnBudget = true // cards are always on-budget
		}
	case "income", "expense":
		if n.IsCard {
			return fmt.Errorf("%w: only liability accounts can be cards", ErrInvalidAccount)
		}
		if n.Opening != 0 {
			return fmt.Errorf("%w: only asset and liability accounts have an opening balance", ErrInvalidAccount)
		}
		n.OnBudget = false
	default:
		return fmt.Errorf("%w: type %q must be asset, liability, income or expense", ErrInvalidAccount, n.Type)
	}
	return nil
}

// postAgainstOpeningBalance moves amount (debit-positive) into accountID,
// balanced by the user's Saldo inicial account.
func (s *Service) postAgainstOpeningBalance(ctx context.Context, q *store.Queries, userID, accountID string, amount int64, at time.Time, description string) error {
	equity, err := q.GetSystemAccountID(ctx, store.GetSystemAccountIDParams{
		UserID: userID, SystemRole: sql.NullString{String: roleOpeningBalance, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("ledger: Saldo inicial account (run EnsureDefaults first): %w", err)
	}
	_, err = s.postTransaction(ctx, q, userID, at, description, []Leg{
		{AccountID: accountID, Amount: amount}, {AccountID: equity, Amount: -amount},
	})
	return err
}
