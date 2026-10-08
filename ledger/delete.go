// story: e09s02
package ledger

import (
	"context"
	"fmt"

	"github.com/upareal/upareal/ledger/internal/store"
)

// DeleteUserData removes every ledger row of userID, including the data key
// (crypto-shredding anything sealed), in one transaction. Idempotent.
func (s *Service) DeleteUserData(ctx context.Context, userID string) error {
	return s.inTx(ctx, func(q *store.Queries) error {
		for _, del := range []func(context.Context, string) error{
			q.DeleteUserImportLines, q.DeleteUserHistory, q.DeleteUserLegs,
			q.DeleteUserTransactions, q.DeleteUserAccounts, q.DeleteUserKey,
		} {
			if err := del(ctx, userID); err != nil {
				return fmt.Errorf("ledger: delete user data: %w", err)
			}
		}
		return nil
	})
}
