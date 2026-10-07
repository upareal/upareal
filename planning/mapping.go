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

// Mapping is one mappable account and its category ("" = Sem categoria).
type Mapping struct {
	Account      ledger.Account
	CategoryID   string
	CategoryName string
	GroupName    string
}

// Mappings lists every active, non-system income/expense account of userID
// with its category. A mapping to a category or account that is no longer
// active is ignored, so the account shows as Sem categoria (P3, X1).
func (s *Service) Mappings(ctx context.Context, userID string) ([]Mapping, error) {
	accounts, err := s.ledg.Accounts(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("planning: accounts: %w", err)
	}
	groups, err := s.bud.Categories(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("planning: categories: %w", err)
	}
	type cat struct{ name, group, kind string }
	cats := map[string]cat{}
	for _, g := range groups {
		for _, c := range g.Categories {
			cats[c.ID] = cat{name: c.Name, group: g.Name, kind: g.Kind}
		}
	}
	rows, err := s.q.ListMappings(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("planning: mappings: %w", err)
	}
	mapped := make(map[string]string, len(rows))
	for _, r := range rows {
		mapped[r.AccountID] = r.CategoryID
	}
	var out []Mapping
	for _, a := range accounts {
		if a.SystemRole != "" || (a.Type != "income" && a.Type != "expense") {
			continue
		}
		m := Mapping{Account: a}
		if c, ok := cats[mapped[a.ID]]; ok && c.kind == a.Type {
			m.CategoryID, m.CategoryName, m.GroupName = mapped[a.ID], c.name, c.group
		}
		out = append(out, m)
	}
	return out, nil
}
