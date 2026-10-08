// story: e13s01
package budget

import (
	"context"
	"fmt"
)

// Stats is the budget's public aggregate: numbers only.
type Stats struct {
	Categories      int64 // active categories
	PlannedCentavos int64 // sum of every amount planned in the month
}

// PublicStats aggregates across all users for month m (the X2 exception:
// aggregate numbers only, nothing per user, nothing decrypted).
func (s *Service) PublicStats(ctx context.Context, m Month) (Stats, error) {
	var st Stats
	var err error
	if st.Categories, err = s.q.CountActiveCategories(ctx); err != nil {
		return Stats{}, fmt.Errorf("budget: public stats: categories: %w", err)
	}
	if st.PlannedCentavos, err = s.q.SumMonthAmounts(ctx, m.String()); err != nil {
		return Stats{}, fmt.Errorf("budget: public stats: planned: %w", err)
	}
	return st, nil
}
