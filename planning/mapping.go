// story: e08s01
package planning

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/upareal/upareal/budget"
	"github.com/upareal/upareal/ledger"
	"github.com/upareal/upareal/planning/internal/store"
)

// Errors for rejected mappings.
var (
	ErrNotFound     = errors.New("planning: account or category not found")
	ErrNotMappable  = errors.New("planning: only the user's income and expense accounts can be mapped")
	ErrKindMismatch = errors.New("planning: an account maps only to a category of the same kind") // P2
)

// Service joins ledger and budget through their public APIs (X1).
type Service struct {
	q    *store.Queries
	ledg *ledger.Service
	bud  *budget.Service
	now  func() time.Time
}

// NewService returns a planning service over a migrated database.
//
//	svc := planning.NewService(conn, ledgerSvc, budgetSvc)
func NewService(conn *sql.DB, ledg *ledger.Service, bud *budget.Service) *Service {
	return &Service{q: store.New(conn), ledg: ledg, bud: bud, now: time.Now}
}

// SetMapping maps one of userID's income/expense accounts to one of their
// active categories of the same kind (P1, P2). An empty categoryID removes
// the mapping, making the account "Sem categoria" again.
func (s *Service) SetMapping(ctx context.Context, userID, accountID, categoryID string) error {
	acc, err := s.mappableAccount(ctx, userID, accountID)
	if err != nil {
		return err
	}
	if categoryID == "" {
		return s.q.DeleteMapping(ctx, store.DeleteMappingParams{UserID: userID, AccountID: accountID})
	}
	kind, err := s.categoryKind(ctx, userID, categoryID)
	if err != nil {
		return err
	}
	if kind != acc.Type {
		return ErrKindMismatch
	}
	return s.q.UpsertMapping(ctx, store.UpsertMappingParams{
		UserID: userID, AccountID: accountID, CategoryID: categoryID, UpdatedAt: s.now().UTC().Format(time.RFC3339),
	})
}

// mappableAccount returns one of userID's active, non-system income/expense accounts.
func (s *Service) mappableAccount(ctx context.Context, userID, accountID string) (ledger.Account, error) {
	accounts, err := s.ledg.Accounts(ctx, userID)
	if err != nil {
		return ledger.Account{}, fmt.Errorf("planning: accounts: %w", err)
	}
	for _, a := range accounts {
		if a.ID != accountID {
			continue
		}
		if a.SystemRole != "" || (a.Type != "income" && a.Type != "expense") {
			return ledger.Account{}, ErrNotMappable
		}
		return a, nil
	}
	return ledger.Account{}, ErrNotFound
}

// categoryKind returns the kind of one of userID's active categories.
func (s *Service) categoryKind(ctx context.Context, userID, categoryID string) (string, error) {
	groups, err := s.bud.Categories(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("planning: categories: %w", err)
	}
	for _, g := range groups {
		for _, c := range g.Categories {
			if c.ID == categoryID {
				return g.Kind, nil
			}
		}
	}
	return "", ErrNotFound
}
