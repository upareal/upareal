// story: e04s03
package ledger

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/upareal/upareal/ledger/internal/store"
)

// Errors for account changes that would break an invariant.
var (
	ErrTypeLocked     = errors.New("ledger: an account with legs cannot change type") // L5
	ErrSystemAccount  = errors.New("ledger: system accounts cannot be changed")       // L7
	ErrNonZeroBalance = errors.New("ledger: only accounts with zero balance can be archived")
	ErrHasLegs        = errors.New("ledger: accounts with legs cannot be deleted; archive them")
)

// AccountUpdate is the editable part of an account.
type AccountUpdate struct {
	Name     string
	Type     string // changeable only while the account has no legs (L5)
	OnBudget bool
	IsCard   bool
}

// UpdateAccount edits one of userID's accounts under the same rules as
// CreateAccount, plus L5 (type locked once used) and L7 (system accounts).
func (s *Service) UpdateAccount(ctx context.Context, userID, id string, u AccountUpdate) (Account, error) {
	n := NewAccount{Name: strings.TrimSpace(u.Name), Type: u.Type, OnBudget: u.OnBudget, IsCard: u.IsCard}
	err := s.inTx(ctx, func(q *store.Queries) error {
		cur, err := s.loadUserAccount(ctx, q, userID, id) // not found / system first, whatever the input
		if err != nil {
			return err
		}
		if err := validateNewAccount(&n); err != nil {
			return err
		}
		if cur.Type != n.Type {
			legs, err := q.CountAccountLegs(ctx, id)
			if err != nil {
				return fmt.Errorf("ledger: count legs: %w", err)
			}
			if legs > 0 {
				return ErrTypeLocked
			}
		}
		dk, err := s.userKey(ctx, q, userID)
		if err != nil {
			return err
		}
		sealed, err := dk.Seal(fieldName, n.Name)
		if err != nil {
			return err
		}
		return q.UpdateAccount(ctx, store.UpdateAccountParams{
			Name: sealed, Type: n.Type, OnBudget: boolInt(n.OnBudget), IsCard: boolInt(n.IsCard), ID: id, UserID: userID,
		})
	})
	if err != nil {
		return Account{}, err
	}
	return Account{ID: id, Type: n.Type, Name: n.Name, OnBudget: n.OnBudget, IsCard: n.IsCard}, nil
}

// loadUserAccount guards every account mutation: other users' accounts are
// not found (X2) and system accounts cannot change (L7).
func (s *Service) loadUserAccount(ctx context.Context, q *store.Queries, userID, id string) (store.GetAccountRow, error) {
	acc, err := s.account(ctx, q, userID, id)
	if err != nil {
		return store.GetAccountRow{}, err
	}
	if acc.SystemRole.Valid {
		return store.GetAccountRow{}, ErrSystemAccount
	}
	return acc, nil
}
