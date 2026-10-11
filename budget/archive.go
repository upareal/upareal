// story: e08s06 e16s01
package budget

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/upareal/upareal/budget/internal/store"
)

// ErrSameCategory means a category cannot be merged into itself.
var ErrSameCategory = errors.New("budget: cannot merge a category into itself")

// ArchiveCategory hides one of userID's active categories; its amounts stay.
// Callers check that its envelope is empty first (planning.ArchiveCategory).
func (s *Service) ArchiveCategory(ctx context.Context, userID, id string) error {
	return s.inTx(ctx, func(q *store.Queries) error {
		if _, err := s.category(ctx, q, userID, id); err != nil {
			return err
		}
		return q.SetCategoryArchived(ctx, store.SetCategoryArchivedParams{ArchivedAt: sql.NullString{String: stamp(s.now()), Valid: true}, ID: id, UserID: userID})
	})
}

// RestoreCategory brings back one of userID's archived categories at the end
// of its (active) group.
func (s *Service) RestoreCategory(ctx context.Context, userID, id string) error {
	return s.inTx(ctx, func(q *store.Queries) error {
		c, err := q.GetArchivedCategory(ctx, store.GetArchivedCategoryParams{ID: id, UserID: userID})
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("budget: load archived category: %w", err)
		}
		pos, err := q.NextCategoryPosition(ctx, store.NextCategoryPositionParams{UserID: userID, GroupID: c.GroupID})
		if err != nil {
			return fmt.Errorf("budget: next category position: %w", err)
		}
		if err := q.SetCategoryPosition(ctx, store.SetCategoryPositionParams{Position: pos, ID: id, UserID: userID}); err != nil {
			return err
		}
		return q.SetCategoryArchived(ctx, store.SetCategoryArchivedParams{ID: id, UserID: userID})
	})
}

// ArchivedCategories lists userID's archived categories, most recent first.
func (s *Service) ArchivedCategories(ctx context.Context, userID string) ([]Category, error) {
	dk, err := s.existingKey(ctx, userID)
	if err != nil || dk == nil {
		return nil, err
	}
	rows, err := s.q.ListArchivedCategories(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("budget: list archived categories: %w", err)
	}
	out := make([]Category, 0, len(rows))
	for _, r := range rows {
		name, err := dk.Open(fieldName, r.Name)
		if err != nil {
			return nil, err
		}
		out = append(out, Category{ID: r.ID, Name: name})
	}
	return out, nil
}

// MergeCategory adds every monthly amount of source to target (same kind,
// both active), deletes source's amounts and meta and archives source, atomically.
func (s *Service) MergeCategory(ctx context.Context, userID, sourceID, targetID string) error {
	if sourceID == targetID {
		return ErrSameCategory
	}
	return s.inTx(ctx, func(q *store.Queries) error {
		src, err := s.category(ctx, q, userID, sourceID)
		if err != nil {
			return err
		}
		dst, err := s.category(ctx, q, userID, targetID)
		if err != nil {
			return err
		}
		if src.Kind != dst.Kind {
			return ErrKindMismatch
		}
		if err := q.MergeAmounts(ctx, store.MergeAmountsParams{TargetID: targetID, UserID: userID, SourceID: sourceID}); err != nil {
			return fmt.Errorf("budget: merge amounts: %w", err)
		}
		if err := q.DeleteCategoryAmounts(ctx, store.DeleteCategoryAmountsParams{UserID: userID, CategoryID: sourceID}); err != nil {
			return fmt.Errorf("budget: delete merged amounts: %w", err)
		}
		// e16s01: the merge target keeps its own meta; the source's is dropped.
		if err := q.DeleteTarget(ctx, store.DeleteTargetParams{UserID: userID, CategoryID: sourceID}); err != nil {
			return fmt.Errorf("budget: delete merged target: %w", err)
		}
		return q.SetCategoryArchived(ctx, store.SetCategoryArchivedParams{ArchivedAt: sql.NullString{String: stamp(s.now()), Valid: true}, ID: sourceID, UserID: userID})
	})
}
