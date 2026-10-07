// story: e04s02
package ledger

import (
	"context"
	"time"

	"github.com/upareal/upareal/ledger/internal/store"
)

// PostTransaction exposes the single leg write path to tests (compiled only into tests).
func PostTransaction(ctx context.Context, s *Service, userID string, at time.Time, description string, legs []Leg) (string, error) {
	var id string
	err := s.inTx(ctx, func(q *store.Queries) error {
		var err error
		id, err = s.postTransaction(ctx, q, userID, at, description, legs)
		return err
	})
	return id, err
}
