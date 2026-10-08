// story: e10s01
package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Export is the user's profile. Google's subject (stored only as a blind
// index), the wrapped data key and session tokens are security material and
// are not exported.
type Export struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Email           string `json:"email"`
	CreatedAt       string `json:"created_at"`
	TermsVersion    string `json:"terms_version,omitempty"`
	TermsAcceptedAt string `json:"terms_accepted_at,omitempty"`
	DisabledAt      string `json:"disabled_at,omitempty"`
}

// ExportUserData returns userID's profile, decrypted.
func (s *Service) ExportUserData(ctx context.Context, userID string) (Export, error) {
	row, err := s.q.GetUser(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return Export{}, ErrNotFound
	}
	if err != nil {
		return Export{}, fmt.Errorf("users: export: %w", err)
	}
	u, _, err := s.decrypt(row)
	if err != nil {
		return Export{}, err
	}
	return Export{ID: u.ID, Name: u.Name, Email: u.Email, CreatedAt: row.CreatedAt, TermsVersion: row.TermsVersion.String,
		TermsAcceptedAt: row.TermsAcceptedAt.String, DisabledAt: row.DisabledAt.String}, nil
}
