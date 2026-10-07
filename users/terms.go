// story: e03s03
package users

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/upareal/upareal/users/internal/store"
)

// AcceptTerms records that userID accepted terms version now (LGPD consent
// evidence). The latest acceptance replaces the previous one.
func (s *Service) AcceptTerms(ctx context.Context, userID, version string) error {
	n, err := s.q.AcceptTerms(ctx, store.AcceptTermsParams{
		TermsVersion:    sql.NullString{String: version, Valid: true},
		TermsAcceptedAt: sql.NullString{String: stamp(s.now()), Valid: true},
		ID:              userID,
	})
	if err != nil {
		return fmt.Errorf("users: accept terms for user %s: %w", userID, err)
	}
	if n != 1 {
		return fmt.Errorf("users: accept terms: user %s not found", userID)
	}
	return nil
}
