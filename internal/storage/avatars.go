package storage

import "context"

// Avatars belong to an account, independent of its current organization.
func (s *Store) Avatar(ctx context.Context, userID string) ([]byte, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = setScope(ctx, tx, userID, ""); err != nil {
		return nil, err
	}
	var image []byte
	err = tx.QueryRow(ctx, "SELECT image FROM user_avatars WHERE user_id=$1", userID).Scan(&image)
	if err != nil {
		return nil, mapError(err)
	}
	return image, tx.Commit(ctx)
}
func (s *Store) SaveAvatar(ctx context.Context, userID string, image []byte) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = setScope(ctx, tx, userID, ""); err != nil {
		return err
	}
	if len(image) == 0 {
		_, err = tx.Exec(ctx, "DELETE FROM user_avatars WHERE user_id=$1", userID)
	} else {
		_, err = tx.Exec(ctx, "INSERT INTO user_avatars(user_id,image) VALUES($1,$2) ON CONFLICT(user_id) DO UPDATE SET image=excluded.image,updated_at=now()", userID, image)
	}
	if err != nil {
		return mapError(err)
	}
	return tx.Commit(ctx)
}
