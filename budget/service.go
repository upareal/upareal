// story: e05s01
package budget

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/upareal/upareal/budget/internal/store"
	"github.com/upareal/upareal/internal/vault"
)

// Sealed field names. Never change them: stored data depends on them.
const fieldName = "name"

// Service owns the budget tables and the budget's per-user data keys.
type Service struct {
	conn *sql.DB
	q    *store.Queries
	kr   *vault.Keyring
	now  func() time.Time
}

// NewService returns a budget service over a migrated database.
//
//	svc := budget.NewService(conn, keyring)
func NewService(conn *sql.DB, kr *vault.Keyring) *Service {
	return &Service{conn: conn, q: store.New(conn), kr: kr, now: time.Now}
}

// EnsureDefaults gives userID its budget data key and, if the user has no
// groups yet, the default groups and categories. Safe to call on every
// sign-in; concurrent calls create one set.
func (s *Service) EnsureDefaults(ctx context.Context, userID string) error {
	if userID == "" {
		return errors.New("budget: EnsureDefaults needs a user ID")
	}
	return s.inTx(ctx, func(q *store.Queries) error {
		dk, err := s.userKey(ctx, q, userID)
		if err != nil {
			return err
		}
		n, err := q.CountGroups(ctx, userID)
		if err != nil {
			return fmt.Errorf("budget: count groups: %w", err)
		}
		if n > 0 {
			return nil
		}
		for gi, g := range defaultGroups {
			groupID, err := s.insertGroup(ctx, q, dk, userID, g.kind, g.name, int64(gi))
			if err != nil {
				return err
			}
			for ci, name := range g.categories {
				if err := s.insertCategory(ctx, q, dk, userID, groupID, name, int64(ci)); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (s *Service) insertGroup(ctx context.Context, q *store.Queries, dk *vault.DataKey, userID, kind, name string, position int64) (string, error) {
	id, sealed, err := newSealedRow(dk, name)
	if err != nil {
		return "", err
	}
	err = q.CreateGroup(ctx, store.CreateGroupParams{ID: id, UserID: userID, Kind: kind, Name: sealed, Position: position, CreatedAt: stamp(s.now())})
	if err != nil {
		return "", fmt.Errorf("budget: create group: %w", err)
	}
	return id, nil
}

func (s *Service) insertCategory(ctx context.Context, q *store.Queries, dk *vault.DataKey, userID, groupID, name string, position int64) error {
	id, sealed, err := newSealedRow(dk, name)
	if err != nil {
		return err
	}
	err = q.CreateCategory(ctx, store.CreateCategoryParams{ID: id, UserID: userID, GroupID: groupID, Name: sealed, Position: position, CreatedAt: stamp(s.now())})
	if err != nil {
		return fmt.Errorf("budget: create category: %w", err)
	}
	return nil
}

// userKey returns the budget's data key for userID, creating it on first use.
func (s *Service) userKey(ctx context.Context, q *store.Queries, userID string) (*vault.DataKey, error) {
	wrapped, err := q.GetUserKey(ctx, userID)
	if err == nil {
		return s.kr.UnwrapDataKey(userID, wrapped)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("budget: load data key: %w", err)
	}
	dk, wrapped, err := s.kr.NewDataKey(userID)
	if err != nil {
		return nil, err
	}
	if err := q.CreateUserKey(ctx, store.CreateUserKeyParams{UserID: userID, DataKey: wrapped, CreatedAt: stamp(s.now())}); err != nil {
		return nil, fmt.Errorf("budget: store data key: %w", err)
	}
	return dk, nil
}

// existingKey returns userID's budget key, or nil when the user has no budget data yet.
func (s *Service) existingKey(ctx context.Context, userID string) (*vault.DataKey, error) {
	wrapped, err := s.q.GetUserKey(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("budget: load data key: %w", err)
	}
	return s.kr.UnwrapDataKey(userID, wrapped)
}

// inTx runs fn in one write transaction (BEGIN IMMEDIATE via the DSN).
func (s *Service) inTx(ctx context.Context, fn func(q *store.Queries) error) error {
	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("budget: begin: %w", err)
	}
	if err := fn(s.q.WithTx(tx)); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("budget: commit: %w", err)
	}
	return nil
}

func newSealedRow(dk *vault.DataKey, name string) (id string, sealed []byte, err error) {
	u, err := uuid.NewV7()
	if err != nil {
		return "", nil, fmt.Errorf("budget: new id: %w", err)
	}
	if sealed, err = dk.Seal(fieldName, name); err != nil {
		return "", nil, err
	}
	return u.String(), sealed, nil
}

// stamp formats t as UTC RFC 3339 (second precision).
func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }
