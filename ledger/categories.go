// story: e04s01
package ledger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Group is a category group with its active categories, in display order.
type Group struct {
	ID         string
	Kind       string // "expense" or "income"
	Name       string
	Categories []Category
}

// Category is an income or expense account inside a group.
type Category struct {
	ID   string
	Name string
}

// Categories returns userID's active groups and categories in display order,
// names decrypted. System accounts are not categories and are not listed.
func (s *Service) Categories(ctx context.Context, userID string) ([]Group, error) {
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
	rows, err := s.q.ListActiveCategories(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("ledger: list categories: %w", err)
	}
	var groups []Group
	for _, r := range rows {
		if len(groups) == 0 || groups[len(groups)-1].ID != r.GroupID {
			name, err := dk.Open(fieldName, r.GroupName)
			if err != nil {
				return nil, err
			}
			groups = append(groups, Group{ID: r.GroupID, Kind: r.GroupKind, Name: name})
		}
		if !r.AccountID.Valid {
			continue // empty group
		}
		name, err := dk.Open(fieldName, r.AccountName)
		if err != nil {
			return nil, err
		}
		g := &groups[len(groups)-1]
		g.Categories = append(g.Categories, Category{ID: r.AccountID.String, Name: name})
	}
	return groups, nil
}
