// story: e09s02
package budget

import (
	"context"
	"fmt"

	"github.com/upareal/upareal/budget/internal/store"
)

// DeleteUserData removes every budget row of userID, including the data key
// (crypto-shredding anything sealed), in one transaction. Idempotent.
func (s *Service) DeleteUserData(ctx context.Context, userID string) error {
	return s.inTx(ctx, func(q *store.Queries) error {
		for _, del := range []func(context.Context, string) error{
			q.DeleteUserAmounts, q.DeleteUserMonths, q.DeleteUserCategories,
			q.DeleteUserGroups, q.DeleteUserKey,
		} {
			if err := del(ctx, userID); err != nil {
				return fmt.Errorf("budget: delete user data: %w", err)
			}
		}
		return nil
	})
}
