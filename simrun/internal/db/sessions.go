package db

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrSessionNotFound is returned when a session does not exist or has expired.
var ErrSessionNotFound = errors.New("session not found")

// AuthSession represents a user authentication session.
type AuthSession struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Picture   string    `json:"picture"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// SessionStore manages authentication session persistence.
type SessionStore interface {
	Create(ctx context.Context, email, name, picture string, ttl time.Duration) (string, error)
	Get(ctx context.Context, sessionID string) (*AuthSession, error)
	Delete(ctx context.Context, sessionID string) error
	DeleteExpired(ctx context.Context) error
}

type sessionStore struct {
	pool *pgxpool.Pool
}

// NewSessionStore creates a new SessionStore backed by PostgreSQL.
func NewSessionStore(pool *pgxpool.Pool) SessionStore {
	return &sessionStore{pool: pool}
}

func (s *sessionStore) Create(ctx context.Context, email, name, picture string, ttl time.Duration) (string, error) {
	id, err := generateSessionID()
	if err != nil {
		return "", err
	}

	expiresAt := time.Now().Add(ttl)
	_, err = s.pool.Exec(ctx,
		`INSERT INTO auth_sessions (id, email, name, picture, expires_at) VALUES ($1, $2, $3, $4, $5)`,
		id, email, name, picture, expiresAt,
	)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *sessionStore) Get(ctx context.Context, sessionID string) (*AuthSession, error) {
	var sess AuthSession
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, name, picture, created_at, expires_at FROM auth_sessions WHERE id = $1 AND expires_at > NOW()`,
		sessionID,
	).Scan(&sess.ID, &sess.Email, &sess.Name, &sess.Picture, &sess.CreatedAt, &sess.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	return &sess, nil
}

func (s *sessionStore) Delete(ctx context.Context, sessionID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM auth_sessions WHERE id = $1`, sessionID)
	return err
}

func (s *sessionStore) DeleteExpired(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM auth_sessions WHERE expires_at < NOW()`)
	return err
}

func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
