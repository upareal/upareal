// story: e09s02
package planning

import (
	"context"
	"fmt"
)

// DeleteUserData removes every planning row of userID. Idempotent.
func (s *Service) DeleteUserData(ctx context.Context, userID string) error {
	if err := s.q.DeleteUserMappings(ctx, userID); err != nil {
		return fmt.Errorf("planning: delete user data: %w", err)
	}
	return nil
}
