package storage

import (
	"context"
	"xingdu.app/xingdu/internal/hosts"
)

const hostColumns = "id::text,name,address,ssh_port,ssh_user,tags,notes,CASE WHEN status='online' AND last_seen_at<now()-interval '90 seconds' THEN 'offline' ELSE status END,last_seen_at,(SELECT NULLIF(btrim(a.metrics->>'version'),'') FROM machine_agents a WHERE a.host_id=hosts.id AND a.organization_id=hosts.organization_id AND a.revoked_at IS NULL)"

type scanner interface{ Scan(...any) error }

func scanHost(row scanner) (Host, error) {
	var h Host
	err := row.Scan(&h.ID, &h.Name, &h.Address, &h.SSHPort, &h.SSHUser, &h.Tags, &h.Notes, &h.Status, &h.LastSeenAt, &h.AgentVersion)
	return h, mapError(err)
}
func (s *Store) Hosts(ctx context.Context) ([]Host, error) {
	tx, _, err := s.tenantTx(ctx, false, false)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, "SELECT "+hostColumns+" FROM hosts ORDER BY created_at DESC,id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Host, 0)
	for rows.Next() {
		h, err := scanHost(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, h)
	}
	return result, rows.Err()
}
func (s *Store) CreateHost(ctx context.Context, in hosts.Input) (Host, error) {
	tx, _, err := s.tenantTx(ctx, true, false)
	if err != nil {
		return Host{}, err
	}
	defer tx.Rollback(ctx)

	h, err := scanHost(tx.QueryRow(ctx, "INSERT INTO hosts(id,name,address,ssh_port,ssh_user,tags,notes) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING "+hostColumns, NewID("srv"), in.Name, in.Address, in.SSHPort, in.SSHUser, in.Tags, in.Notes))
	if err != nil {
		return h, err
	}
	return h, tx.Commit(ctx)
}
func (s *Store) UpdateHost(ctx context.Context, id string, in hosts.Input) (Host, error) {
	tx, _, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return Host{}, err
	}
	defer tx.Rollback(ctx)
	if err = checkHostRelayDependents(ctx, tx, id); err != nil {
		return Host{}, err
	}

	h, err := scanHost(tx.QueryRow(ctx, "UPDATE hosts SET name=$2,address=$3,ssh_port=$4,ssh_user=$5,tags=$6,notes=$7 WHERE id=$1 RETURNING "+hostColumns, id, in.Name, in.Address, in.SSHPort, in.SSHUser, in.Tags, in.Notes))
	if err != nil {
		return h, err
	}
	return h, tx.Commit(ctx)
}
func (s *Store) DeleteHost(ctx context.Context, id string) error {
	tx, _, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = checkHostRelayDependents(ctx, tx, id); err != nil {
		return err
	}

	result, err := tx.Exec(ctx, "DELETE FROM hosts WHERE id=$1", id)
	if err != nil {
		return mapError(err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err = audit(ctx, tx, id, "host_deleted"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
