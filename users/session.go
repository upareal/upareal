// story: e03s01
package users

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/upareal/upareal/users/internal/store"
)

const sessionTokenBytes = 32

// ErrNoSession means the token does not identify a live session.
var ErrNoSession = errors.New("users: no session for this token")

// CreateSession starts a session for userID and returns the token for the
// cookie. Only SHA-256(token) is stored, so a leaked database cannot be used
// to hijack sessions.
func (s *Service) CreateSession(ctx context.Context, userID string) (string, error) {
	raw := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("users: session token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := s.now().UTC().Format(time.RFC3339)
	err := s.q.CreateSession(ctx, store.CreateSessionParams{
		TokenHash: hashToken(token), UserID: userID, CreatedAt: now, LastSeenAt: now,
	})
	if err != nil {
		return "", fmt.Errorf("users: create session for user %s: %w", userID, err)
	}
	return token, nil
}

// UserBySession returns the signed-in user for a cookie token, or ErrNoSession.
func (s *Service) UserBySession(ctx context.Context, token string) (User, error) {
	if token == "" {
		return User{}, ErrNoSession
	}
	row, err := s.q.GetUserBySessionHash(ctx, hashToken(token))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNoSession
	}
	if err != nil {
		return User{}, fmt.Errorf("users: resolve session: %w", err)
	}
	u, _, err := s.decrypt(row)
	return u, err
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
