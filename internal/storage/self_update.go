package storage

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"xingdu.app/xingdu/internal/machine"
)

// Resolve identity through the existing token RLS policy, then take the same
// organization lock used by membership changes. No caller chooses tenant scope.
func (s *Store) updaterTx(ctx context.Context, hash string) (pgx.Tx, string, string, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, "", "", err
	}
	fail := func(e error) (pgx.Tx, string, string, error) { tx.Rollback(ctx); return nil, "", "", e }
	if _, err = tx.Exec(ctx, `SELECT set_config('app.agent_hash',$1,true)`, hash); err != nil {
		return fail(err)
	}
	var host, org string
	if err = tx.QueryRow(ctx, `SELECT host_id,organization_id FROM machine_agents WHERE token_hash=$1 AND revoked_at IS NULL`, hash).Scan(&host, &org); err != nil {
		return fail(mapError(err))
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, org); err != nil {
		return fail(err)
	}
	if err = setScope(ctx, tx, "", org); err != nil {
		return fail(err)
	}
	if err = tx.QueryRow(ctx, `SELECT host_id FROM machine_agents WHERE token_hash=$1 AND revoked_at IS NULL FOR UPDATE`, hash).Scan(&host); err != nil {
		return fail(mapError(err))
	}
	return tx, host, org, nil
}

func (s *Store) ClaimSelfUpdate(ctx context.Context, hash string) (*machine.UpdateTask, error) {
	tx, host, org, err := s.updaterTx(ctx, hash)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE machine_agents SET updater_seen_at=now() WHERE host_id=$1`, host); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE machine_jobs SET state='failed',result='upgrade_unconfirmed',finished_at=now() WHERE host_id=$1 AND transport='agent' AND ((state='running' AND lease_until<now()) OR (state='queued' AND created_at<now()-interval '30 minutes'))`, host); err != nil {
		return nil, err
	}
	var task machine.UpdateTask
	var user, bound string
	err = tx.QueryRow(ctx, `SELECT id,created_by,agent_hash,target_version,arch,artifact_sha256 FROM machine_jobs WHERE host_id=$1 AND transport='agent' AND state='queued' ORDER BY created_at LIMIT 1 FOR UPDATE`, host).Scan(&task.ID, &user, &bound, &task.Version, &task.Arch, &task.SHA256)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, tx.Commit(ctx)
	}
	if err != nil {
		return nil, err
	}
	if err = setScope(ctx, tx, user, org); err != nil {
		return nil, err
	}
	var allowed bool
	if err = tx.QueryRow(ctx, `SELECT COALESCE(tenant_role(request_org_id()) IN ('owner','admin'),false)`).Scan(&allowed); err != nil {
		return nil, err
	}
	if !allowed || bound != hash {
		_, err = tx.Exec(ctx, `UPDATE machine_jobs SET state='cancelled',result='authorization_revoked',finished_at=now() WHERE id=$1`, task.ID)
		if err != nil {
			return nil, err
		}
		return nil, tx.Commit(ctx)
	}
	if err = tx.QueryRow(ctx, `UPDATE machine_jobs SET state='running',lease=new_resource_id('lease'),lease_until=now()+interval '5 minutes' WHERE id=$1 RETURNING lease`, task.ID).Scan(&task.Lease); err != nil {
		return nil, err
	}
	if err = audit(ctx, tx, host, "agent_upgrade_started"); err != nil {
		return nil, err
	}
	return &task, tx.Commit(ctx)
}

// Called again immediately before execution. A claim is not perpetual authority.
func (s *Store) CheckSelfUpdate(ctx context.Context, hash, id, lease string, phase string) error {
	tx, host, org, err := s.updaterTx(ctx, hash)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var user string
	if err = tx.QueryRow(ctx, `SELECT created_by FROM machine_jobs WHERE id=$1 AND host_id=$2 AND transport='agent' AND agent_hash=$3 AND lease=$4 AND state='running' AND lease_until>now() FOR UPDATE`, id, host, hash, lease).Scan(&user); err != nil {
		return mapError(err)
	}
	if err = setScope(ctx, tx, user, org); err != nil {
		return err
	}
	var allowed bool
	if err = tx.QueryRow(ctx, `SELECT COALESCE(tenant_role(request_org_id()) IN ('owner','admin'),false)`).Scan(&allowed); err != nil {
		return err
	}
	if !allowed {
		return ErrForbidden
	}
	if phase == "applied" {
		if _, err = tx.Exec(ctx, `UPDATE machine_jobs SET result='awaiting_heartbeat' WHERE id=$1`, id); err != nil {
			return err
		}
	}
	if phase == "failed" {
		if _, err = tx.Exec(ctx, `UPDATE machine_jobs SET state='failed',result='self_update_failed',finished_at=now() WHERE id=$1`, id); err != nil {
			return err
		}
		if err = audit(ctx, tx, host, "agent_upgrade_failed"); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Only a newly authenticated heartbeat, never a helper exit code, completes it.
func (s *Store) CompleteSelfUpdate(ctx context.Context, hash, version string) error {
	tx, host, org, err := s.updaterTx(ctx, hash)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id, user string
	err = tx.QueryRow(ctx, `SELECT id,created_by FROM machine_jobs WHERE host_id=$1 AND transport='agent' AND agent_hash=$2 AND state='running' AND target_version=$3 AND result='awaiting_heartbeat' AND lease_until>now() FOR UPDATE`, host, hash, version).Scan(&id, &user)
	if errors.Is(err, pgx.ErrNoRows) {
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	if err = setScope(ctx, tx, user, org); err != nil {
		return err
	}
	var allowed bool
	if err = tx.QueryRow(ctx, `SELECT COALESCE(tenant_role(request_org_id()) IN ('owner','admin'),false)`).Scan(&allowed); err != nil {
		return err
	}
	state, result := "installed", "agent_upgraded"
	if !allowed {
		state, result = "cancelled", "authorization_revoked"
	}
	if _, err = tx.Exec(ctx, `UPDATE machine_jobs SET state=$2,result=$3,finished_at=now() WHERE id=$1`, id, state, result); err != nil {
		return err
	}
	if allowed {
		if err = audit(ctx, tx, host, "agent_upgrade_installed"); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
