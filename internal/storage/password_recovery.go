package storage

import (
	"context"
)

func (s *Store) BeginPasswordRecovery(ctx context.Context, token, email, code string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended('recovery:' || $1,0))", email); err != nil {
		return err
	}
	var count int
	var recent bool
	if err = tx.QueryRow(ctx, `SELECT count(*),coalesce(bool_or(created_at>now()-interval '60 seconds'),false) FROM password_recoveries WHERE email=$1 AND created_at>now()-interval '24 hours'`, email).Scan(&count, &recent); err != nil {
		return err
	}
	if count >= 5 || recent {
		return ErrRegistrationLimited
	}
	if _, err = tx.Exec(ctx, `DELETE FROM password_recoveries WHERE created_at<now()-interval '24 hours'`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE password_recoveries SET consumed_at=now() WHERE email=$1 AND consumed_at IS NULL`, email); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO password_recoveries(token_hash,email,code_hash,password_snapshot) VALUES($1,$2,$3,coalesce((SELECT password_hash FROM users WHERE lower(email)=$2 AND email_verified_at IS NOT NULL AND password_login_enabled),''))`, token, email, code)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) DeliverPasswordRecovery(ctx context.Context, token string) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE password_recoveries SET delivered=true WHERE token_hash=$1 AND consumed_at IS NULL AND expires_at>now()`, token)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrVerification
	}
	return nil
}
func (s *Store) CompletePasswordRecovery(ctx context.Context, token, code, password string) error {
	var ok bool
	if err := s.Pool.QueryRow(ctx, `SELECT complete_password_recovery($1,$2,$3)`, token, code, password).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return ErrVerification
	}
	return nil
}
