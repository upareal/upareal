// story: e03s01 e03s03
package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/upareal/upareal/internal/vault"
	"github.com/upareal/upareal/users/internal/store"
)

// Blind-index purpose and sealed field names. Never change them: stored data depends on them.
const (
	subIndexPurpose = "google-sub"
	fieldEmail      = "email"
	fieldName       = "name"
)

// ErrEmailNotVerified means the identity provider has not verified the email.
var ErrEmailNotVerified = errors.New("users: email is not verified by the identity provider")

// Identity is what a successful Google sign-in proves about a person.
type Identity struct {
	Subject       string // Google "sub": permanent account ID
	Email         string
	Name          string
	EmailVerified bool
}

// User is a decrypted user record.
type User struct {
	ID           string
	Email        string
	Name         string
	CreatedAt    time.Time
	TermsVersion string // latest accepted terms version; "" if never accepted
	Admin        bool   // email is in ADMIN_EMAILS (e09s01); never stored
}

// Service owns user records and sessions.
type Service struct {
	q      *store.Queries
	kr     *vault.Keyring
	now    func() time.Time
	admins map[string]bool // normalized ADMIN_EMAILS
}

// NewService returns a users service over a migrated database.
//
//	svc := users.NewService(conn, keyring)
func NewService(conn *sql.DB, kr *vault.Keyring, opts ...Option) *Service {
	s := &Service{q: store.New(conn), kr: kr, now: time.Now, admins: map[string]bool{}}
	for _, o := range opts {
		o(s)
	}
	return s
}

// SignIn returns the user for id.Subject, creating it on first sign-in and
// refreshing the stored email and name when they changed. Unverified emails
// are refused with ErrEmailNotVerified.
func (s *Service) SignIn(ctx context.Context, id Identity) (User, error) {
	if !id.EmailVerified {
		return User{}, ErrEmailNotVerified
	}
	if id.Subject == "" {
		return User{}, errors.New("users: identity has no subject")
	}
	subIdx := s.kr.BlindIndex(subIndexPurpose, id.Subject)
	row, err := s.q.GetUserBySubIdx(ctx, subIdx)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return s.createUser(ctx, subIdx, id)
	case err != nil:
		return User{}, fmt.Errorf("users: look up by sub: %w", err)
	}
	return s.refreshIdentity(ctx, row, id)
}

// ponytail: two simultaneous first sign-ins for one sub hit the UNIQUE
// constraint and one fails; retry-on-conflict if that ever shows up in logs.
func (s *Service) createUser(ctx context.Context, subIdx []byte, id Identity) (User, error) {
	userID, err := uuid.NewV7()
	if err != nil {
		return User{}, fmt.Errorf("users: new id: %w", err)
	}
	dk, wrapped, err := s.kr.NewDataKey(userID.String())
	if err != nil {
		return User{}, err
	}
	email, name, err := sealIdentity(dk, id)
	if err != nil {
		return User{}, err
	}
	created := s.now().UTC().Truncate(time.Second)
	err = s.q.CreateUser(ctx, store.CreateUserParams{
		ID: userID.String(), SubIdx: subIdx, DataKey: wrapped,
		Email: email, Name: name, CreatedAt: created.Format(time.RFC3339),
	})
	if err != nil {
		return User{}, fmt.Errorf("users: create user: %w", err)
	}
	return User{ID: userID.String(), Email: id.Email, Name: id.Name, CreatedAt: created, Admin: s.admins[normalizeEmail(id.Email)]}, nil
}

func (s *Service) refreshIdentity(ctx context.Context, row store.UsersUser, id Identity) (User, error) {
	u, dk, err := s.decrypt(row)
	if err != nil {
		return User{}, err
	}
	if u.Email == id.Email && u.Name == id.Name {
		return u, nil
	}
	email, name, err := sealIdentity(dk, id)
	if err != nil {
		return User{}, err
	}
	if err := s.q.UpdateUserIdentity(ctx, store.UpdateUserIdentityParams{Email: email, Name: name, ID: row.ID}); err != nil {
		return User{}, fmt.Errorf("users: update identity: %w", err)
	}
	u.Email, u.Name, u.Admin = id.Email, id.Name, s.admins[normalizeEmail(id.Email)]
	return u, nil
}

// decrypt unwraps the row's data key and opens its sealed fields.
func (s *Service) decrypt(row store.UsersUser) (User, *vault.DataKey, error) {
	dk, err := s.kr.UnwrapDataKey(row.ID, row.DataKey)
	if err != nil {
		return User{}, nil, fmt.Errorf("users: user %s: %w", row.ID, err)
	}
	email, err := dk.Open(fieldEmail, row.Email)
	if err != nil {
		return User{}, nil, err
	}
	name, err := dk.Open(fieldName, row.Name)
	if err != nil {
		return User{}, nil, err
	}
	created, err := time.Parse(time.RFC3339, row.CreatedAt)
	if err != nil {
		return User{}, nil, fmt.Errorf("users: user %s created_at %q: %w", row.ID, row.CreatedAt, err)
	}
	return User{ID: row.ID, Email: email, Name: name, CreatedAt: created, TermsVersion: row.TermsVersion.String,
		Admin: s.admins[normalizeEmail(email)]}, dk, nil
}

func sealIdentity(dk *vault.DataKey, id Identity) (email, name []byte, err error) {
	if email, err = dk.Seal(fieldEmail, id.Email); err != nil {
		return nil, nil, err
	}
	if name, err = dk.Seal(fieldName, id.Name); err != nil {
		return nil, nil, err
	}
	return email, name, nil
}
