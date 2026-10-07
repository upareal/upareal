// story: e04s01 e04s02 e04s03
package ledger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/upareal/upareal/ledger/internal/store"
)

// Account is a decrypted ledger account. SystemRole is "" for user accounts.
type Account struct {
	ID         string
	Type       string // asset, liability, income, expense, equity
	Name       string
	SystemRole string
	OnBudget   bool // asset/liability only: counts in the budget
	IsCard     bool // liability only: credit card (always on-budget)
}

// Accounts returns userID's active accounts, ordered by type (asset,
// liability, income, expense, equity) and then position, names decrypted.
func (s *Service) Accounts(ctx context.Context, userID string) ([]Account, error) {
	return s.listAccounts(ctx, userID, false)
}

// ArchivedAccounts returns userID's archived accounts in the same order.
func (s *Service) ArchivedAccounts(ctx context.Context, userID string) ([]Account, error) {
	return s.listAccounts(ctx, userID, true)
}

func (s *Service) listAccounts(ctx context.Context, userID string, archived bool) ([]Account, error) {
	wrapped, err := s.q.GetUserKey(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil // user has no ledger data yet
	}
	if err != nil {
		return nil, fmt.Errorf("ledger: load data key: %w", err)
	}
	dk, err := s.kr.UnwrapDataKey(userID, wrapped)
	if err != nil {
		return nil, err
	}
	var rows []store.ListActiveAccountsRow
	if archived {
		arch, err := s.q.ListArchivedAccounts(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("ledger: list archived accounts: %w", err)
		}
		for _, r := range arch {
			rows = append(rows, store.ListActiveAccountsRow(r))
		}
	} else if rows, err = s.q.ListActiveAccounts(ctx, userID); err != nil {
		return nil, fmt.Errorf("ledger: list accounts: %w", err)
	}
	accounts := make([]Account, 0, len(rows))
	for _, r := range rows {
		name, err := dk.Open(fieldName, r.Name)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, Account{ID: r.ID, Type: r.Type, Name: name, SystemRole: r.SystemRole.String,
			OnBudget: r.OnBudget == 1, IsCard: r.IsCard == 1})
	}
	return accounts, nil
}
