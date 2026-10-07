// story: e04s01
package ledger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/upareal/upareal/internal/vault"
	"github.com/upareal/upareal/ledger/internal/store"
)

// Sealed field names. Never change them: stored data depends on them.
const fieldName = "name"

// Service owns the ledger tables and the ledger's per-user data keys.
type Service struct {
	conn *sql.DB
	q    *store.Queries
	kr   *vault.Keyring
	now  func() time.Time
}

// NewService returns a ledger service over a migrated database.
//
//	svc := ledger.NewService(conn, keyring)
func NewService(conn *sql.DB, kr *vault.Keyring) *Service {
	return &Service{conn: conn, q: store.New(conn), kr: kr, now: time.Now}
}

// EnsureDefaults gives userID its ledger data key, the default groups and
// categories (only if the user has no groups yet) and any missing system
// accounts. Safe to call on every sign-in; concurrent calls create one set.
func (s *Service) EnsureDefaults(ctx context.Context, userID string) error {
	if userID == "" {
		return errors.New("ledger: EnsureDefaults needs a user ID")
	}
	return s.inTx(ctx, func(q *store.Queries) error {
		dk, err := s.userKey(ctx, q, userID)
		if err != nil {
			return err
		}
		n, err := q.CountGroups(ctx, userID)
		if err != nil {
			return fmt.Errorf("ledger: count groups: %w", err)
		}
		if n == 0 {
			if err := s.createDefaultGroups(ctx, q, dk, userID); err != nil {
				return err
			}
		}
		return s.createMissingSystemAccounts(ctx, q, dk, userID)
	})
}

func (s *Service) createDefaultGroups(ctx context.Context, q *store.Queries, dk *vault.DataKey, userID string) error {
	for gi, g := range defaultGroups {
		groupID, err := s.insertGroup(ctx, q, dk, userID, g.kind, g.name, int64(gi))
		if err != nil {
			return err
		}
		for ci, name := range g.categories {
			if err := s.insertAccount(ctx, q, dk, userID, g.kind, sql.NullString{String: groupID, Valid: true}, name, int64(ci), sql.NullString{}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) createMissingSystemAccounts(ctx context.Context, q *store.Queries, dk *vault.DataKey, userID string) error {
	existing, err := q.ListSystemRoles(ctx, userID)
	if err != nil {
		return fmt.Errorf("ledger: list system accounts: %w", err)
	}
	have := map[string]bool{}
	for _, r := range existing {
		have[r.String] = true
	}
	for _, sa := range systemAccounts {
		if have[sa.role] {
			continue
		}
		if err := s.insertAccount(ctx, q, dk, userID, sa.accountType, sql.NullString{}, sa.name, 0, sql.NullString{String: sa.role, Valid: true}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) insertGroup(ctx context.Context, q *store.Queries, dk *vault.DataKey, userID, kind, name string, position int64) (string, error) {
	id, sealed, err := newSealedRow(dk, name)
	if err != nil {
		return "", err
	}
	err = q.CreateGroup(ctx, store.CreateGroupParams{
		ID: id, UserID: userID, Kind: kind, Name: sealed, Position: position, CreatedAt: stamp(s.now()),
	})
	if err != nil {
		return "", fmt.Errorf("ledger: create group: %w", err)
	}
	return id, nil
}

func (s *Service) insertAccount(ctx context.Context, q *store.Queries, dk *vault.DataKey, userID, accountType string,
	groupID sql.NullString, name string, position int64, role sql.NullString) error {
	id, sealed, err := newSealedRow(dk, name)
	if err != nil {
		return err
	}
	err = q.CreateAccount(ctx, store.CreateAccountParams{
		ID: id, UserID: userID, Type: accountType, GroupID: groupID, Name: sealed,
		Position: position, SystemRole: role, CreatedAt: stamp(s.now()),
	})
	if err != nil {
		return fmt.Errorf("ledger: create %s account: %w", accountType, err)
	}
	return nil
}

// userKey returns the ledger's data key for userID, creating it on first use.
func (s *Service) userKey(ctx context.Context, q *store.Queries, userID string) (*vault.DataKey, error) {
	wrapped, err := q.GetUserKey(ctx, userID)
	if err == nil {
		return s.kr.UnwrapDataKey(userID, wrapped)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("ledger: load data key: %w", err)
	}
	dk, wrapped, err := s.kr.NewDataKey(userID)
	if err != nil {
		return nil, err
	}
	if err := q.CreateUserKey(ctx, store.CreateUserKeyParams{UserID: userID, DataKey: wrapped, CreatedAt: stamp(s.now())}); err != nil {
		return nil, fmt.Errorf("ledger: store data key: %w", err)
	}
	return dk, nil
}

// inTx runs fn in one write transaction (BEGIN IMMEDIATE via the DSN).
func (s *Service) inTx(ctx context.Context, fn func(q *store.Queries) error) error {
	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ledger: begin: %w", err)
	}
	if err := fn(s.q.WithTx(tx)); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ledger: commit: %w", err)
	}
	return nil
}

func newSealedRow(dk *vault.DataKey, name string) (id string, sealed []byte, err error) {
	u, err := uuid.NewV7()
	if err != nil {
		return "", nil, fmt.Errorf("ledger: new id: %w", err)
	}
	if sealed, err = dk.Seal(fieldName, name); err != nil {
		return "", nil, err
	}
	return u.String(), sealed, nil
}

// stamp formats t as UTC RFC 3339 (second precision).
func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }
