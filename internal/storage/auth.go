package storage

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"time"
)

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("conflict")

type User struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
}

func NewID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
func mapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "23505" || pgErr.Code == "23503") {
		return ErrConflict
	}
	return err
}
func (s *Store) CreateAdmin(ctx context.Context, username, hash string) error {
	_, err := s.Register(ctx, username, hash, "默认组织")
	return err
}
func (s *Store) Credentials(ctx context.Context, username string) (User, error) {
	var a User
	err := s.Pool.QueryRow(ctx, "SELECT id::text,username,password_hash FROM users WHERE username=$1", username).Scan(&a.ID, &a.Username, &a.PasswordHash)
	return a, mapError(err)
}
func (s *Store) Session(ctx context.Context, hash string) (User, error) {
	var a User
	err := s.Pool.QueryRow(ctx, "SELECT a.id::text,a.username FROM sessions s JOIN users a ON a.id=s.user_id WHERE s.token_hash=$1 AND s.expires_at>now()", hash).Scan(&a.ID, &a.Username)
	return a, mapError(err)
}
func (s *Store) NewSession(ctx context.Context, hash, id string, expires time.Time) error {
	// Remove expired sessions to keep storage bounded over normal operation.
	if _, err := s.Pool.Exec(ctx, "DELETE FROM sessions WHERE expires_at<=now()"); err != nil {
		return err
	}
	_, err := s.Pool.Exec(ctx, "INSERT INTO sessions(token_hash,user_id,expires_at) VALUES($1,$2,$3)", hash, id, expires)
	return err
}
func (s *Store) DeleteSession(ctx context.Context, hash string) error {
	_, err := s.Pool.Exec(ctx, "DELETE FROM sessions WHERE token_hash=$1", hash)
	return err
}
