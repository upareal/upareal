// story: e09s01
package users

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Option configures a Service.
type Option func(*Service)

// AdminEmails makes the users with these emails admins (ADMIN_EMAILS);
// emails compare trimmed and lowercased. The role is never stored.
//
//	svc := users.NewService(conn, kr, users.AdminEmails(cfg.AdminEmails...))
func AdminEmails(emails ...string) Option {
	return func(s *Service) {
		for _, e := range emails {
			if e = normalizeEmail(e); e != "" {
				s.admins[e] = true
			}
		}
	}
}

func normalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

// UserSummary is what an admin sees about a user: identity and activity,
// never financial data.
type UserSummary struct {
	ID, Name, Email string
	CreatedAt       time.Time
	LastSeenAt      time.Time // latest session activity; zero if none
	Admin           bool
}

// ListUsers returns every user, oldest first, for the admin page.
func (s *Service) ListUsers(ctx context.Context) ([]UserSummary, error) {
	rows, err := s.q.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("users: list: %w", err)
	}
	out := make([]UserSummary, 0, len(rows))
	for _, r := range rows {
		u, _, err := s.decrypt(r.UsersUser)
		if err != nil {
			return nil, err
		}
		sum := UserSummary{ID: u.ID, Name: u.Name, Email: u.Email, CreatedAt: u.CreatedAt, Admin: u.Admin}
		if r.LastSeenAt != "" {
			if sum.LastSeenAt, err = time.Parse(time.RFC3339, r.LastSeenAt); err != nil {
				return nil, fmt.Errorf("users: user %s last_seen_at: %w", u.ID, err)
			}
		}
		out = append(out, sum)
	}
	return out, nil
}
