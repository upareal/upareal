// story: e13s01
package users

import (
	"context"
	"fmt"
)

// Stats is the users module's public aggregate: numbers only.
type Stats struct {
	Users int64
}

// PublicStats counts registered users across everyone (the X2 exception:
// aggregate numbers only, nothing per user).
func (s *Service) PublicStats(ctx context.Context) (Stats, error) {
	n, err := s.q.CountUsers(ctx)
	if err != nil {
		return Stats{}, fmt.Errorf("users: public stats: %w", err)
	}
	return Stats{Users: n}, nil
}
