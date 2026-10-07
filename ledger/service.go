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

// EnsureDefaults gives userID its ledger data key and any missing system
// account. Safe to call on every sign-in; concurrent calls create one set.
func (s *Service) EnsureDefaults(ctx context.Context, userID string) error {
	if userID == "" {
		return errors.New("ledger: EnsureDefaults needs a user ID")
	}
	return s.inTx(ctx, func(q *store.Queries) error {
		dk, err := s.userKey(ctx, q, userID)
		if err != nil {
			return err
		}
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
			row := accountRow{accountType: sa.accountType, name: sa.name, role: sql.NullString{String: sa.role, Valid: true}}
			if _, err := s.insertAccount(ctx, q, dk, userID, row); err != nil {
				return err
			}
		}
		return nil
	})
}

// accountRow is what insertAccount writes.
type accountRow struct {
	accountType, name string
	position          int64
	role              sql.NullString
	onBudget, isCard  bool
}

func (s *Service) insertAccount(ctx context.Context, q *store.Queries, dk *vault.DataKey, userID string, r accountRow) (string, error) {
	id, sealed, err := newSealedRow(dk, r.name)
	if err != nil {
		return "", err
	}
	err = q.CreateAccount(ctx, store.CreateAccountParams{
		ID: id, UserID: userID, Type: r.accountType, Name: sealed,
		Position: r.position, SystemRole: r.role, CreatedAt: stamp(s.now()),
		OnBudget: boolInt(r.onBudget), IsCard: boolInt(r.isCard),
	})
	if err != nil {
		return "", fmt.Errorf("ledger: create %s account: %w", r.accountType, err)
	}
	return id, nil
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
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
	return newSealedField(dk, fieldName, name)
}

// newSealedField returns a fresh UUIDv7 and value sealed as field.
func newSealedField(dk *vault.DataKey, field, value string) (id string, sealed []byte, err error) {
	if id, err = newID(); err != nil {
		return "", nil, err
	}
	if sealed, err = dk.Seal(field, value); err != nil {
		return "", nil, err
	}
	return id, sealed, nil
}

func newID() (string, error) {
	u, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("ledger: new id: %w", err)
	}
	return u.String(), nil
}

// stamp formats t as UTC RFC 3339 (second precision).
func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }
