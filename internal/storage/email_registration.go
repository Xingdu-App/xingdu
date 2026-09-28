package storage

import (
	"context"
	"crypto/subtle"
	"errors"
	"github.com/jackc/pgx/v5"
)

var ErrRegistrationLimited = errors.New("registration limited")
var ErrVerification = errors.New("invalid verification")

type PendingRegistration struct{ TokenHash, Email, PasswordHash, Organization, CodeHash string }

func (s *Store) BeginEmailRegistration(ctx context.Context, in PendingRegistration) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('registration:' || $1,0))`, in.Email); err != nil {
		return err
	}
	var count int
	var recent bool
	if err = tx.QueryRow(ctx, `SELECT count(*),coalesce(bool_or(created_at>now()-interval '60 seconds'),false) FROM pending_registrations WHERE email=$1 AND created_at>now()-interval '24 hours'`, in.Email).Scan(&count, &recent); err != nil {
		return err
	}
	if recent || count >= 5 {
		return ErrRegistrationLimited
	}
	if _, err = tx.Exec(ctx, `DELETE FROM pending_registrations WHERE created_at<now()-interval '24 hours'`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE pending_registrations SET consumed_at=now() WHERE email=$1 AND consumed_at IS NULL`, in.Email); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO pending_registrations(token_hash,email,password_hash,organization_name,code_hash) VALUES($1,$2,$3,$4,$5)`, in.TokenHash, in.Email, in.PasswordHash, in.Organization, in.CodeHash)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) DeliverEmailRegistration(ctx context.Context, hash string) error {
	result, err := s.Pool.Exec(ctx, `UPDATE pending_registrations SET delivered=true WHERE token_hash=$1 AND consumed_at IS NULL AND expires_at>now()`, hash)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrVerification
	}
	return nil
}
func (s *Store) VerifyEmailRegistration(ctx context.Context, tokenHash, codeHash string) (string, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var email, hash, orgName, expected string
	var valid bool
	var attempts int
	err = tx.QueryRow(ctx, `SELECT email,password_hash,organization_name,code_hash,attempts,delivered AND consumed_at IS NULL AND expires_at>now() FROM pending_registrations WHERE token_hash=$1 FOR UPDATE`, tokenHash).Scan(&email, &hash, &orgName, &expected, &attempts, &valid)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrVerification
	}
	if err != nil {
		return "", err
	}
	if !valid || attempts >= 5 {
		return "", ErrVerification
	}
	if _, err = tx.Exec(ctx, `UPDATE pending_registrations SET attempts=attempts+1 WHERE token_hash=$1`, tokenHash); err != nil {
		return "", err
	}
	if subtle.ConstantTimeCompare([]byte(codeHash), []byte(expected)) != 1 {
		if err = tx.Commit(ctx); err != nil {
			return "", err
		}
		return "", ErrVerification
	}
	if _, err = tx.Exec(ctx, `UPDATE pending_registrations SET consumed_at=now() WHERE token_hash=$1`, tokenHash); err != nil {
		return "", err
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE username=$1 OR lower(email)=$1)`, email).Scan(&exists); err != nil {
		return "", err
	}
	if exists {
		if err = tx.Commit(ctx); err != nil {
			return "", err
		}
		return "", ErrVerification
	}
	id, org := NewID("usr"), NewID("org")
	if err = setScope(ctx, tx, id, org); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO users(id,username,password_hash,email,email_verified_at) VALUES($1,$2,$3,$2,now())`, id, email, hash); err != nil {
		return "", mapError(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO organizations(id,name,created_by) VALUES($1,$2,$3)`, org, orgName, id); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO memberships(organization_id,user_id,role) VALUES($1,$2,'owner')`, org, id); err != nil {
		return "", err
	}
	return id, tx.Commit(ctx)
}
