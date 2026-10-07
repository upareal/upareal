// story: e05s01
package budget

import (
	"context"
	"fmt"
)

// Group is a category group with its active categories, in display order.
type Group struct {
	ID         string
	Kind       string // "expense" or "income"; every category in the group has this kind (B2)
	Name       string
	Categories []Category
}

// Category is a budget category inside a group.
type Category struct {
	ID   string
	Name string
}

// Categories returns userID's active groups and categories in display order,
// names decrypted.
func (s *Service) Categories(ctx context.Context, userID string) ([]Group, error) {
	dk, err := s.existingKey(ctx, userID)
	if err != nil || dk == nil {
		return nil, err
	}
	rows, err := s.q.ListActiveCategories(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("budget: list categories: %w", err)
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
		if !r.CategoryID.Valid {
			continue // empty group
		}
		name, err := dk.Open(fieldName, r.CategoryName)
		if err != nil {
			return nil, err
		}
		g := &groups[len(groups)-1]
		g.Categories = append(g.Categories, Category{ID: r.CategoryID.String, Name: name})
	}
	return groups, nil
}
