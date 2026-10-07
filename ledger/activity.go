// story: e08s02
package ledger

import (
	"context"
	"fmt"
	"time"

	"github.com/upareal/upareal/ledger/internal/store"
)

// Activity returns, per account of userID, the debit-positive sum of legs on
// live transactions that occurred in [from, to). Accounts without legs in
// the range are absent.
func (s *Service) Activity(ctx context.Context, userID string, from, to time.Time) (map[string]int64, error) {
	rows, err := s.q.ActivityBetween(ctx, store.ActivityBetweenParams{UserID: userID, FromAt: stamp(from), ToAt: stamp(to)})
	if err != nil {
		return nil, fmt.Errorf("ledger: activity: %w", err)
	}
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.AccountID] = r.Amount
	}
	return out, nil
}
