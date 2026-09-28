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

type Admin struct {
	ID           string `json:"-"`
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
	_, err := s.Pool.Exec(ctx, "INSERT INTO admins(id,username,password_hash) VALUES($1,$2,$3)", NewID(), username, hash)
	return mapError(err)
}
func (s *Store) Credentials(ctx context.Context, username string) (Admin, error) {
	var a Admin
	err := s.Pool.QueryRow(ctx, "SELECT id::text,username,password_hash FROM admins WHERE username=$1", username).Scan(&a.ID, &a.Username, &a.PasswordHash)
	return a, mapError(err)
}
func (s *Store) Session(ctx context.Context, hash string) (Admin, error) {
	var a Admin
	err := s.Pool.QueryRow(ctx, "SELECT a.id::text,a.username FROM sessions s JOIN admins a ON a.id=s.admin_id WHERE s.token_hash=$1 AND s.expires_at>now()", hash).Scan(&a.ID, &a.Username)
	return a, mapError(err)
}
func (s *Store) NewSession(ctx context.Context, hash, id string, expires time.Time) error {
	// Remove expired sessions to keep storage bounded over normal operation.
	if _, err := s.Pool.Exec(ctx, "DELETE FROM sessions WHERE expires_at<=now()"); err != nil {
		return err
	}
	_, err := s.Pool.Exec(ctx, "INSERT INTO sessions(token_hash,admin_id,expires_at) VALUES($1,$2,$3)", hash, id, expires)
	return err
}
func (s *Store) DeleteSession(ctx context.Context, hash string) error {
	_, err := s.Pool.Exec(ctx, "DELETE FROM sessions WHERE token_hash=$1", hash)
	return err
}
