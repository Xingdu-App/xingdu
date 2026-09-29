package storage

import (
	"context"
	"xingdu.app/xingdu/internal/machine"
)

// UpgradeIdentity is administrator-only; identity hashes never reach browser JSON.
func (s *Store) UpgradeIdentity(ctx context.Context, host string) (machine.Job, error) {
	var out machine.Job
	tx, _, err := s.tenantTx(ctx, false, true)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `SELECT token_hash,mode,COALESCE(metrics->>'arch',''),COALESCE(metrics->>'version','') FROM machine_agents WHERE host_id=$1 AND revoked_at IS NULL`, host).Scan(&out.AgentHash, &out.Mode, &out.Arch, &out.TargetVersion)
	return out, mapError(err)
}
func (s *Store) UpgradeReady(ctx context.Context, j machine.Job) (bool, error) {
	tx, _, err := s.tenantTx(ctx, false, true)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var ready bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM machine_agents a JOIN hosts h ON h.id=a.host_id JOIN machine_jobs j ON j.host_id=a.host_id WHERE j.id=$1 AND j.lease=$2 AND j.state='running' AND a.token_hash=j.agent_hash AND a.revoked_at IS NULL AND a.metrics->>'version'=j.target_version AND h.last_seen_at>j.created_at AND h.last_seen_at>now()-interval '90 seconds')`, j.ID, j.Lease).Scan(&ready)
	return ready, err
}
