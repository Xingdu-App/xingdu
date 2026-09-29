package storage

import (
	"context"
	"xingdu.app/xingdu/internal/id"
	"xingdu.app/xingdu/internal/protocol"
)

func (s *Store) RestartDeployment(ctx context.Context, host, id string) error {
	tx, _, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var kind string
	if err = tx.QueryRow(ctx, `SELECT protocol FROM protocol_deployments WHERE id=$1 AND host_id=$2`, id, host).Scan(&kind); err != nil {
		return mapError(err)
	}
	hash, err := managingAgent(ctx, tx, host, protocol.MinimumAgentVersion(kind))
	if err != nil {
		return err
	}
	if err = cleanupDeployments(ctx, tx, host); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE protocol_deployments SET action='restart',state='queued',operation_id=$3,created_by=request_user_id(),agent_hash=$4,lease=NULL,lease_until=NULL,queued_at=now(),finished_at=NULL,result='',service_status='unknown',service_checked_at=NULL WHERE id=$1 AND host_id=$2 AND installed_at IS NOT NULL AND state IN ('succeeded','failed','interrupted') AND action<>'remove' AND encrypted IS NOT NULL`, id, host, NewID("op"), hash)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	if err = audit(ctx, tx, host, "protocol_restart_queued"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Reports can only update this authenticated machine's confirmed installations.
// Server receive time bounds staleness; no machine-supplied timestamps are trusted.
func (s *Store) ReportServices(ctx context.Context, hash string, reports []protocol.ServiceStatus) error {
	if len(reports) > 100 {
		return ErrInvalid
	}
	seen := map[string]bool{}
	for _, r := range reports {
		if !id.Valid("node", r.ID) || seen[r.ID] || (r.Status != "active" && r.Status != "inactive" && r.Status != "missing" && r.Status != "unknown") {
			return ErrInvalid
		}
		seen[r.ID] = true
	}
	tx, host, _, err := s.deploymentAgentTx(ctx, hash)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, r := range reports {
		tag, e := tx.Exec(ctx, `UPDATE protocol_deployments SET service_status=$3,service_checked_at=now() WHERE id=$1 AND host_id=$2 AND installed_at IS NOT NULL AND state<>'removed'`, r.ID, host, r.Status)
		if e != nil {
			return e
		}
		if tag.RowsAffected() != 1 {
			return ErrNotFound
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) ServiceInventory(ctx context.Context, hash string) ([]string, error) {
	tx, host, _, err := s.deploymentAgentTx(ctx, hash)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT id::text FROM protocol_deployments WHERE host_id=$1 AND installed_at IS NOT NULL AND state<>'removed' ORDER BY service_checked_at NULLS FIRST,id LIMIT 100`, host)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}

// Preflight is advisory; QueueDeployment repeats all state checks atomically.
func (s *Store) DeploymentPreflight(ctx context.Context, host string, port int, kind string) error {
	tx, _, err := s.tenantTx(ctx, false, true)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = managingAgent(ctx, tx, host, protocol.MinimumAgentVersion(kind)); err != nil {
		return err
	}
	var conflict bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM protocol_deployments WHERE host_id=$1 AND (state IN ('queued','running') OR (port=$2 AND state NOT IN ('removed','cancelled'))))`, host, port).Scan(&conflict); err != nil {
		return err
	}
	if conflict {
		return ErrConflict
	}
	return tx.Commit(ctx)
}
