package storage

import "context"

func (s *Store) TransferOwnership(ctx context.Context, target, passwordHash, sessionHash string) error {
	tx, role, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if role != "owner" {
		return ErrForbidden
	}
	var done bool
	if err = tx.QueryRow(ctx, `SELECT transfer_organization_owner($1,$2,$3)`, target, passwordHash, sessionHash).Scan(&done); err != nil {
		return mapError(err)
	}
	if !done {
		return ErrConflict
	}
	return tx.Commit(ctx)
}
