package storage

import (
	"context"
	"time"
)

type AccountSession struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Current   bool      `json:"current"`
}

func (s *Store) AccountProfile(ctx context.Context, userID string) (string, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if err = setScope(ctx, tx, userID, ""); err != nil {
		return "", err
	}
	var name string
	err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT display_name FROM user_profiles WHERE user_id=$1),'')`, userID).Scan(&name)
	if err != nil {
		return "", err
	}
	return name, tx.Commit(ctx)
}
func (s *Store) SaveAccountProfile(ctx context.Context, userID, name string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = setScope(ctx, tx, userID, ""); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO user_profiles(user_id,display_name) VALUES($1,$2) ON CONFLICT(user_id) DO UPDATE SET display_name=excluded.display_name,updated_at=now()`, userID, name)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) AccountSessions(ctx context.Context, userID, currentHash string) ([]AccountSession, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id::text,created_at,expires_at,token_hash=$2 FROM sessions WHERE user_id=$1 AND expires_at>now() ORDER BY created_at DESC`, userID, currentHash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AccountSession{}
	for rows.Next() {
		var v AccountSession
		if err = rows.Scan(&v.ID, &v.CreatedAt, &v.ExpiresAt, &v.Current); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) RevokeAccountSession(ctx context.Context, userID, id string) error {
	result, err := s.Pool.Exec(ctx, `DELETE FROM sessions WHERE user_id=$1 AND id::text=$2`, userID, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Compare-and-swap prevents concurrent password changes based on stale verification.
// Locking the account also serializes verified login session issuance.
func (s *Store) ChangeAccountPassword(ctx context.Context, userID, oldHash, newHash, currentSession string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = setScope(ctx, tx, userID, ""); err != nil {
		return err
	}
	var changed bool
	err = tx.QueryRow(ctx, `SELECT change_account_password($1,$2,$3,$4)`, userID, oldHash, newHash, currentSession).Scan(&changed)
	if err != nil {
		return err
	}
	if !changed {
		return ErrConflict
	}
	return tx.Commit(ctx)
}

// Called only after bcrypt verification; confirms the verified password is still current.
func (s *Store) NewVerifiedSession(ctx context.Context, tokenHash, userID, passwordHash string, expires time.Time) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('account:' || $1, 0))`, userID); err != nil {
		return err
	}
	var hash string
	if err = tx.QueryRow(ctx, `SELECT password_hash FROM users WHERE id=$1`, userID).Scan(&hash); err != nil {
		return mapError(err)
	}
	if hash != passwordHash {
		return ErrConflict
	}
	if _, err = tx.Exec(ctx, `DELETE FROM sessions WHERE expires_at<=now()`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO sessions(token_hash,user_id,expires_at) VALUES($1,$2,$3)`, tokenHash, userID, expires); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
