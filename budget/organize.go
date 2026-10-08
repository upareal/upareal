// story: e05s02
package budget

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/upareal/upareal/budget/internal/store"
)

const maxName = 100

// Errors for rejected changes.
var (
	ErrInvalidName  = errors.New("budget: name must have 1 to 100 characters")
	ErrInvalidKind  = errors.New("budget: kind must be expense or income")
	ErrKindMismatch = errors.New("budget: expense and income never mix") // B2
	ErrNotFound     = errors.New("budget: not found")
)

// validName trims name and enforces 1–100 characters.
func validName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxName {
		return "", ErrInvalidName
	}
	return name, nil
}

// CreateGroup adds a group of kind ("expense" or "income") as the last group.
func (s *Service) CreateGroup(ctx context.Context, userID, kind, name string) (Group, error) {
	name, err := validName(name)
	if err != nil {
		return Group{}, err
	}
	if kind != "expense" && kind != "income" {
		return Group{}, ErrInvalidKind
	}
	var g Group
	err = s.inTx(ctx, func(q *store.Queries) error {
		dk, err := s.userKey(ctx, q, userID)
		if err != nil {
			return err
		}
		pos, err := q.NextGroupPosition(ctx, userID)
		if err != nil {
			return fmt.Errorf("budget: next group position: %w", err)
		}
		id, err := s.insertGroup(ctx, q, dk, userID, kind, name, pos)
		g = Group{ID: id, Kind: kind, Name: name}
		return err
	})
	return g, err
}

// RenameGroup renames one of userID's groups.
func (s *Service) RenameGroup(ctx context.Context, userID, id, name string) error {
	name, err := validName(name)
	if err != nil {
		return err
	}
	return s.inTx(ctx, func(q *store.Queries) error {
		if _, err := s.group(ctx, q, userID, id); err != nil {
			return err
		}
		sealed, err := s.seal(ctx, q, userID, name)
		if err != nil {
			return err
		}
		return q.RenameGroup(ctx, store.RenameGroupParams{Name: sealed, ID: id, UserID: userID})
	})
}

// MoveGroup swaps a group with its neighbour (up = earlier); no-op at the ends.
func (s *Service) MoveGroup(ctx context.Context, userID, id string, up bool) error {
	return s.inTx(ctx, func(q *store.Queries) error {
		cur, err := s.group(ctx, q, userID, id)
		if err != nil {
			return err
		}
		var nID string
		var nPos int64
		if up {
			n, e := q.PreviousGroup(ctx, store.PreviousGroupParams{UserID: userID, Position: cur.Position})
			nID, nPos, err = n.ID, n.Position, e
		} else {
			n, e := q.NextGroup(ctx, store.NextGroupParams{UserID: userID, Position: cur.Position})
			nID, nPos, err = n.ID, n.Position, e
		}
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("budget: find neighbour group: %w", err)
		}
		if err := q.SetGroupPosition(ctx, store.SetGroupPositionParams{Position: nPos, ID: id, UserID: userID}); err != nil {
			return err
		}
		return q.SetGroupPosition(ctx, store.SetGroupPositionParams{Position: cur.Position, ID: nID, UserID: userID})
	})
}

// group loads one of userID's active groups or returns ErrNotFound (X2).
func (s *Service) group(ctx context.Context, q *store.Queries, userID, id string) (store.GetGroupRow, error) {
	g, err := q.GetGroup(ctx, store.GetGroupParams{ID: id, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return store.GetGroupRow{}, ErrNotFound
	}
	if err != nil {
		return store.GetGroupRow{}, fmt.Errorf("budget: load group: %w", err)
	}
	return g, nil
}

// seal encrypts a name with userID's budget key.
func (s *Service) seal(ctx context.Context, q *store.Queries, userID, name string) ([]byte, error) {
	dk, err := s.userKey(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	return dk.Seal(fieldName, name)
}

// CreateCategory adds a category as the last one of one of userID's groups.
func (s *Service) CreateCategory(ctx context.Context, userID, groupID, name string) (Category, error) {
	name, err := validName(name)
	if err != nil {
		return Category{}, err
	}
	var c Category
	err = s.inTx(ctx, func(q *store.Queries) error {
		if _, err := s.group(ctx, q, userID, groupID); err != nil {
			return err
		}
		dk, err := s.userKey(ctx, q, userID)
		if err != nil {
			return err
		}
		pos, err := q.NextCategoryPosition(ctx, store.NextCategoryPositionParams{UserID: userID, GroupID: groupID})
		if err != nil {
			return fmt.Errorf("budget: next category position: %w", err)
		}
		id, sealed, err := newSealedRow(dk, name)
		if err != nil {
			return err
		}
		c = Category{ID: id, Name: name}
		return q.CreateCategory(ctx, store.CreateCategoryParams{ID: id, UserID: userID, GroupID: groupID, Name: sealed, Position: pos, CreatedAt: stamp(s.now())})
	})
	return c, err
}

// RenameCategory renames one of userID's categories.
func (s *Service) RenameCategory(ctx context.Context, userID, id, name string) error {
	name, err := validName(name)
	if err != nil {
		return err
	}
	return s.inTx(ctx, func(q *store.Queries) error {
		if _, err := s.category(ctx, q, userID, id); err != nil {
			return err
		}
		sealed, err := s.seal(ctx, q, userID, name)
		if err != nil {
			return err
		}
		return q.RenameCategory(ctx, store.RenameCategoryParams{Name: sealed, ID: id, UserID: userID})
	})
}

// SetCategoryGroup moves a category to the end of another group of the same
// kind (B2).
func (s *Service) SetCategoryGroup(ctx context.Context, userID, id, groupID string) error {
	return s.inTx(ctx, func(q *store.Queries) error {
		c, err := s.category(ctx, q, userID, id)
		if err != nil {
			return err
		}
		target, err := s.group(ctx, q, userID, groupID)
		if err != nil {
			return err
		}
		if target.Kind != c.Kind {
			return ErrKindMismatch
		}
		if target.ID == c.GroupID {
			return nil
		}
		pos, err := q.NextCategoryPosition(ctx, store.NextCategoryPositionParams{UserID: userID, GroupID: groupID})
		if err != nil {
			return fmt.Errorf("budget: next category position: %w", err)
		}
		return q.SetCategoryGroup(ctx, store.SetCategoryGroupParams{GroupID: groupID, Position: pos, ID: id, UserID: userID})
	})
}

// MoveCategory swaps a category with its neighbour in the same group (up =
// earlier); no-op at the ends.
func (s *Service) MoveCategory(ctx context.Context, userID, id string, up bool) error {
	return s.inTx(ctx, func(q *store.Queries) error {
		cur, err := s.category(ctx, q, userID, id)
		if err != nil {
			return err
		}
		var nID string
		var nPos int64
		if up {
			n, e := q.PreviousCategory(ctx, store.PreviousCategoryParams{UserID: userID, GroupID: cur.GroupID, Position: cur.Position})
			nID, nPos, err = n.ID, n.Position, e
		} else {
			n, e := q.NextCategory(ctx, store.NextCategoryParams{UserID: userID, GroupID: cur.GroupID, Position: cur.Position})
			nID, nPos, err = n.ID, n.Position, e
		}
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("budget: find neighbour category: %w", err)
		}
		if err := q.SetCategoryPosition(ctx, store.SetCategoryPositionParams{Position: nPos, ID: id, UserID: userID}); err != nil {
			return err
		}
		return q.SetCategoryPosition(ctx, store.SetCategoryPositionParams{Position: cur.Position, ID: nID, UserID: userID})
	})
}

// category loads one of userID's active categories (with its group's kind)
// or returns ErrNotFound (X2).
func (s *Service) category(ctx context.Context, q *store.Queries, userID, id string) (store.GetCategoryRow, error) {
	c, err := q.GetCategory(ctx, store.GetCategoryParams{ID: id, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return store.GetCategoryRow{}, ErrNotFound
	}
	if err != nil {
		return store.GetCategoryRow{}, fmt.Errorf("budget: load category: %w", err)
	}
	return c, nil
}
