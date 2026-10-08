package storage

import (
	"bytes"
	"context"
	"github.com/jackc/pgx/v5"
)

func checkRelayDependents(ctx context.Context, tx pgx.Tx, node string) error {
	var used bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM protocol_deployments d LEFT JOIN protocol_revisions r ON r.node_id=d.id AND r.revision=d.pending_revision AND r.organization_id=d.organization_id WHERE d.state<>'removed' AND (d.relay_exit_id=$1 OR r.relay_exit_id=$1))`, node).Scan(&used)
	if err != nil {
		return err
	}
	if used {
		return ErrConflict
	}
	return nil
}
func checkRelayExit(ctx context.Context, tx pgx.Tx, d Deployment) error {
	var cipher []byte
	err := tx.QueryRow(ctx, `SELECT encrypted FROM protocol_deployments WHERE id=$1 AND host_id<>$2 AND installed_at IS NOT NULL AND state='succeeded' AND action='deploy' AND pending_revision IS NULL AND relay_exit_id IS NULL AND external_exit_id IS NULL FOR UPDATE`, d.RelayExitID, d.HostID).Scan(&cipher)
	if err != nil {
		return mapError(err)
	}
	if !bytes.Equal(cipher, d.ExitCipher) {
		return ErrConflict
	}
	return nil
}

func checkHostRelayDependents(ctx context.Context, tx pgx.Tx, host string) error {
	rows, err := tx.Query(ctx, `SELECT id FROM protocol_deployments WHERE host_id=$1`, host)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err = checkRelayDependents(ctx, tx, id); err != nil {
			return err
		}
	}
	return nil
}
