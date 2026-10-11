// story: e16s01
package budget

import (
	"context"
	"errors"
	"fmt"

	"github.com/upareal/upareal/budget/internal/store"
)

var (
	// ErrTargetIncome means a target was set on an income category (B5).
	ErrTargetIncome = errors.New("budget: targets are for expense categories only")
	// ErrTargetPast means a target's due month is before the current month.
	ErrTargetPast = errors.New("budget: target due month must be the current month or later")
)

// Target is a category's savings target (meta): Amount centavos by Due (B5).
type Target struct {
	Amount int64
	Due    Month
}

// SetTarget creates or replaces the target of one of userID's active expense
// categories. amount must be > 0 and due must not be before the current month.
func (s *Service) SetTarget(ctx context.Context, userID, categoryID string, amount int64, due Month) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	if due.String() < MonthOf(s.now()).String() {
		return ErrTargetPast
	}
	return s.inTx(ctx, func(q *store.Queries) error {
		c, err := s.category(ctx, q, userID, categoryID)
		if err != nil {
			return err
		}
		if c.Kind != "expense" {
			return ErrTargetIncome
		}
		return q.UpsertTarget(ctx, store.UpsertTargetParams{UserID: userID, CategoryID: categoryID, Amount: amount, DueMonth: due.String(), CreatedAt: stamp(s.now())})
	})
}

// RemoveTarget deletes the target of userID's category, if any.
func (s *Service) RemoveTarget(ctx context.Context, userID, categoryID string) error {
	if err := s.q.DeleteTarget(ctx, store.DeleteTargetParams{UserID: userID, CategoryID: categoryID}); err != nil {
		return fmt.Errorf("budget: remove target: %w", err)
	}
	return nil
}

// Targets returns category ID → target for userID's active categories.
func (s *Service) Targets(ctx context.Context, userID string) (map[string]Target, error) {
	rows, err := s.q.ListActiveTargets(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("budget: targets: %w", err)
	}
	out := make(map[string]Target, len(rows))
	for _, r := range rows {
		due, err := ParseMonth(r.DueMonth)
		if err != nil {
			return nil, fmt.Errorf("budget: target %s due %q: %w", r.CategoryID, r.DueMonth, err)
		}
		out[r.CategoryID] = Target{Amount: r.Amount, Due: due}
	}
	return out, nil
}
