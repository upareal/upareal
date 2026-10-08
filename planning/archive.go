// story: e08s06
package planning

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/upareal/upareal/budget"
	"github.com/upareal/upareal/planning/internal/store"
)

// ErrEnvelopeNotEmpty means an expense category still has money (or debt)
// this month; move it or merge the category instead.
var ErrEnvelopeNotEmpty = errors.New("planning: only an expense category with Disponível zero can be archived")

// ArchiveCategory archives one of userID's categories. An expense category
// must have Disponível zero in the current São Paulo month, so A distribuir
// does not change. Its mappings are dropped: its accounts become Sem
// categoria (X1).
func (s *Service) ArchiveCategory(ctx context.Context, userID, categoryID string) error {
	p, err := s.Month(ctx, userID, budget.MonthOf(s.now()))
	if err != nil {
		return err
	}
	if p.Categories[categoryID].Available != 0 {
		return ErrEnvelopeNotEmpty
	}
	if err := s.bud.ArchiveCategory(ctx, userID, categoryID); err != nil {
		return err
	}
	if err := s.q.DeleteCategoryMappings(ctx, store.DeleteCategoryMappingsParams{UserID: userID, CategoryID: categoryID}); err != nil {
		return fmt.Errorf("planning: drop mappings: %w", err)
	}
	return nil
}

// MergeCategory moves source's accounts and monthly amounts to target (same
// kind) and archives source; every month's A distribuir stays the same.
// The two modules commit separately: the pair is validated first, mappings
// move next (idempotent), then budget merges atomically, so a failure in
// between leaves a consistent plan the user can retry.
func (s *Service) MergeCategory(ctx context.Context, userID, sourceID, targetID string) error {
	if sourceID == targetID {
		return budget.ErrSameCategory
	}
	groups, err := s.bud.Categories(ctx, userID)
	if err != nil {
		return fmt.Errorf("planning: categories: %w", err)
	}
	kinds := map[string]string{}
	for _, g := range groups {
		for _, c := range g.Categories {
			kinds[c.ID] = g.Kind
		}
	}
	switch {
	case kinds[sourceID] == "" || kinds[targetID] == "":
		return budget.ErrNotFound
	case kinds[sourceID] != kinds[targetID]:
		return budget.ErrKindMismatch
	}
	if err := s.q.MoveCategoryMappings(ctx, store.MoveCategoryMappingsParams{
		TargetID: targetID, UpdatedAt: s.now().UTC().Format(time.RFC3339), UserID: userID, SourceID: sourceID,
	}); err != nil {
		return fmt.Errorf("planning: move mappings: %w", err)
	}
	return s.bud.MergeCategory(ctx, userID, sourceID, targetID)
}
