// story: e09s02
package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/upareal/upareal/users/internal/store"
)

// Errors for admin actions on users.
var (
	ErrNotFound      = errors.New("users: user not found")
	ErrProtectedUser = errors.New("users: admins cannot be disabled or deleted")
	ErrDisabled      = errors.New("users: account is disabled")
)

// DisableUser locks a non-admin user out: sign-in is refused and every
// session is deleted. Sessions surviving a failure here are still refused,
// because UserBySession checks disabled_at.
func (s *Service) DisableUser(ctx context.Context, id string) error {
	if _, err := s.unprotected(ctx, id); err != nil {
		return err
	}
	if err := s.q.SetUserDisabled(ctx, store.SetUserDisabledParams{DisabledAt: sql.NullString{String: stamp(s.now()), Valid: true}, ID: id}); err != nil {
		return fmt.Errorf("users: disable: %w", err)
	}
	return s.EndAllSessions(ctx, id)
}

// EnableUser lets a disabled user sign in again.
func (s *Service) EnableUser(ctx context.Context, id string) error {
	if _, err := s.unprotected(ctx, id); err != nil {
		return err
	}
	if err := s.q.SetUserDisabled(ctx, store.SetUserDisabledParams{ID: id}); err != nil {
		return fmt.Errorf("users: enable: %w", err)
	}
	return nil
}

// DeleteUser removes a non-admin user: purge first deletes their data in the
// other modules, then the user row goes (sessions cascade). If purge fails
// the user remains, so the deletion can be retried.
//
//	err := svc.DeleteUser(ctx, id, func(ctx context.Context, id string) error { return ledgerSvc.DeleteUserData(ctx, id) })
func (s *Service) DeleteUser(ctx context.Context, id string, purge func(context.Context, string) error) error {
	if _, err := s.unprotected(ctx, id); err != nil {
		return err
	}
	if err := purge(ctx, id); err != nil {
		return fmt.Errorf("users: purge user data: %w", err)
	}
	if err := s.q.DeleteUser(ctx, id); err != nil {
		return fmt.Errorf("users: delete: %w", err)
	}
	return nil
}

// unprotected loads a user that admins may act on.
func (s *Service) unprotected(ctx context.Context, id string) (User, error) {
	row, err := s.q.GetUser(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("users: load: %w", err)
	}
	u, _, err := s.decrypt(row)
	if err != nil {
		return User{}, err
	}
	if u.Admin {
		return User{}, ErrProtectedUser
	}
	return u, nil
}
