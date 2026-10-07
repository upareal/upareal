// story: e05s03
package budget

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/upareal/upareal/budget/internal/store"
)

// ErrInvalidAmount means an amount is negative (B4).
var ErrInvalidAmount = errors.New("budget: amount must be zero or positive")

// OpenMonth opens month for userID if needed and returns category ID → amount.
func (s *Service) OpenMonth(ctx context.Context, userID string, m Month) (map[string]int64, error) {
	if err := s.inTx(ctx, func(q *store.Queries) error { return s.ensureMonth(ctx, q, userID, m) }); err != nil {
		return nil, err
	}
	rows, err := s.q.MonthAmounts(ctx, store.MonthAmountsParams{UserID: userID, Month: m.String()})
	if err != nil {
		return nil, fmt.Errorf("budget: month amounts: %w", err)
	}
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.CategoryID] = r.Amount
	}
	return out, nil
}

// SetAmount sets the amount (>= 0, B4) of one of userID's categories in a
// month, replacing any previous one (B3).
func (s *Service) SetAmount(ctx context.Context, userID, categoryID string, m Month, amount int64) error {
	if amount < 0 {
		return ErrInvalidAmount
	}
	return s.inTx(ctx, func(q *store.Queries) error {
		if _, err := s.category(ctx, q, userID, categoryID); err != nil {
			return err
		}
		if err := s.ensureMonth(ctx, q, userID, m); err != nil {
			return err
		}
		return q.UpsertAmount(ctx, store.UpsertAmountParams{UserID: userID, CategoryID: categoryID, Month: m.String(), Amount: amount})
	})
}

// ensureMonth opens month for userID exactly once: it copies the amounts of
// the latest earlier opened month (if any) and marks the month opened.
func (s *Service) ensureMonth(ctx context.Context, q *store.Queries, userID string, m Month) error {
	n, err := q.MonthOpened(ctx, store.MonthOpenedParams{UserID: userID, Month: m.String()})
	if err != nil {
		return fmt.Errorf("budget: month opened: %w", err)
	}
	if n > 0 {
		return nil
	}
	from, err := q.LatestOpenedMonthBefore(ctx, store.LatestOpenedMonthBeforeParams{UserID: userID, Month: m.String()})
	switch {
	case errors.Is(err, sql.ErrNoRows): // first month ever: start empty
	case err != nil:
		return fmt.Errorf("budget: latest opened month: %w", err)
	default:
		if err := q.CopyMonthAmounts(ctx, store.CopyMonthAmountsParams{ToMonth: m.String(), UserID: userID, FromMonth: from}); err != nil {
			return fmt.Errorf("budget: copy %s to %s: %w", from, m, err)
		}
	}
	return q.MarkMonthOpened(ctx, store.MarkMonthOpenedParams{UserID: userID, Month: m.String(), OpenedAt: stamp(s.now())})
}
