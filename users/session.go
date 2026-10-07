// story: e03s01 e03s02
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

const (
	sessionTokenBytes = 32
	sessionIdleLimit  = 30 * 24 * time.Hour
	sessionMaxAge     = 90 * 24 * time.Hour
	sessionTouchEvery = time.Hour // throttle last_seen_at writes
)

// ErrNoSession means the token does not identify a live session.
var ErrNoSession = errors.New("users: no session for this token")

// CreateSession starts a session for userID and returns the token for the
// cookie. Only SHA-256(token) is stored, so a leaked database cannot be used
// to hijack sessions. The user's expired sessions are purged.
func (s *Service) CreateSession(ctx context.Context, userID string) (string, error) {
	raw := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("users: session token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := s.now().UTC()
	err := s.q.CreateSession(ctx, store.CreateSessionParams{
		TokenHash: hashToken(token), UserID: userID, CreatedAt: stamp(now), LastSeenAt: stamp(now),
	})
	if err != nil {
		return "", fmt.Errorf("users: create session for user %s: %w", userID, err)
	}
	err = s.q.DeleteExpiredUserSessions(ctx, store.DeleteExpiredUserSessionsParams{
		UserID: userID, IdleCutoff: stamp(now.Add(-sessionIdleLimit)), AbsoluteCutoff: stamp(now.Add(-sessionMaxAge)),
	})
	if err != nil {
		return "", fmt.Errorf("users: purge expired sessions for user %s: %w", userID, err)
	}
	return token, nil
}

// UserBySession returns the signed-in user for a cookie token, or ErrNoSession.
// Sessions die after 30 idle days or 90 days total; dead ones are deleted.
// last_seen_at is refreshed at most hourly.
func (s *Service) UserBySession(ctx context.Context, token string) (User, error) {
	if token == "" {
		return User{}, ErrNoSession
	}
	hash := hashToken(token)
	row, err := s.q.GetSessionWithUser(ctx, hash)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNoSession
	}
	if err != nil {
		return User{}, fmt.Errorf("users: resolve session: %w", err)
	}
	created, err1 := time.Parse(time.RFC3339, row.UsersSession.CreatedAt)
	lastSeen, err2 := time.Parse(time.RFC3339, row.UsersSession.LastSeenAt)
	if err := errors.Join(err1, err2); err != nil {
		return User{}, fmt.Errorf("users: session timestamps: %w", err)
	}
	now := s.now().UTC()
	if now.Sub(lastSeen) > sessionIdleLimit || now.Sub(created) > sessionMaxAge {
		if err := s.q.DeleteSession(ctx, hash); err != nil {
			return User{}, fmt.Errorf("users: delete expired session: %w", err)
		}
		return User{}, ErrNoSession
	}
	if now.Sub(lastSeen) >= sessionTouchEvery {
		if err := s.q.TouchSession(ctx, store.TouchSessionParams{LastSeenAt: stamp(now), TokenHash: hash}); err != nil {
			return User{}, fmt.Errorf("users: touch session: %w", err)
		}
	}
	u, _, err := s.decrypt(row.UsersUser)
	return u, err
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// stamp formats t as UTC RFC 3339 (second precision), so stored timestamps
// compare correctly as strings in SQL.
func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }
