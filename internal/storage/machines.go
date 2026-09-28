package storage

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
	"xingdu.app/xingdu/internal/machine"
)

type MachineState struct {
	RequiredAgentVersion string                   `json:"required_agent_version"`
	Agent                *machine.AgentInfo       `json:"agent"`
	Credential           *machine.SavedCredential `json:"credential"`
	Jobs                 []machine.Job            `json:"jobs"`
}

func audit(ctx context.Context, tx pgx.Tx, host, event string) error {
	_, e := tx.Exec(ctx, "INSERT INTO machine_audit(organization_id,host_id,actor_id,event) VALUES(request_org_id(),$1,request_user_id(),$2)", host, event)
	return e
}
func (s *Store) MachineTarget(ctx context.Context, id string) (machine.Target, error) {
	tx, _, err := s.tenantTx(ctx, false, true)
	if err != nil {
		return machine.Target{}, err
	}
	defer tx.Rollback(ctx)
	var t machine.Target
	err = tx.QueryRow(ctx, "SELECT address,ssh_port,ssh_user FROM hosts WHERE id=$1", id).Scan(&t.Address, &t.Port, &t.User)
	return t, mapError(err)
}
func (s *Store) MachineState(ctx context.Context, id string) (MachineState, error) {
	out := MachineState{Jobs: []machine.Job{}, RequiredAgentVersion: machine.Version}
	tx, role, err := s.tenantTx(ctx, false, false)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	var found string
	if err = tx.QueryRow(ctx, "SELECT id::text FROM hosts WHERE id=$1", id).Scan(&found); err != nil {
		return out, mapError(err)
	}
	var a machine.AgentInfo
	var raw []byte
	err = tx.QueryRow(ctx, "SELECT mode,enrolled_at,revoked_at,metrics FROM machine_agents WHERE host_id=$1", id).Scan(&a.Mode, &a.EnrolledAt, &a.RevokedAt, &raw)
	if err == nil {
		json.Unmarshal(raw, &a.Metrics)
		out.Agent = &a
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	if role != "owner" && role != "admin" {
		return out, nil
	}
	var c machine.SavedCredential
	err = tx.QueryRow(ctx, "SELECT method,fingerprint,saved_at FROM machine_credentials WHERE host_id=$1", id).Scan(&c.Method, &c.Fingerprint, &c.SavedAt)
	if err == nil {
		out.Credential = &c
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	rows, err := tx.Query(ctx, "SELECT id::text,host_id::text,mode,state,result,created_at,finished_at FROM machine_jobs WHERE host_id=$1 ORDER BY created_at DESC LIMIT 20", id)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var j machine.Job
		if err = rows.Scan(&j.ID, &j.HostID, &j.Mode, &j.State, &j.Result, &j.CreatedAt, &j.FinishedAt); err != nil {
			return out, err
		}
		out.Jobs = append(out.Jobs, j)
	}
	return out, rows.Err()
}
func (s *Store) Enrollment(ctx context.Context, id, mode, hash string) (time.Time, error) {
	var expires time.Time
	tx, _, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return expires, err
	}
	defer tx.Rollback(ctx)
	if !machine.ValidMode(mode) {
		return expires, ErrInvalid
	}
	var found string
	if err = tx.QueryRow(ctx, "SELECT id::text FROM hosts WHERE id=$1", id).Scan(&found); err != nil {
		return expires, mapError(err)
	}
	if _, err = tx.Exec(ctx, "DELETE FROM machine_enrollments WHERE host_id=$1", id); err != nil {
		return expires, err
	}
	err = tx.QueryRow(ctx, "INSERT INTO machine_enrollments(token_hash,organization_id,host_id,mode,expires_at,created_by) VALUES($1,request_org_id(),$2,$3,now()+interval '15 minutes',request_user_id()) RETURNING expires_at", hash, id, mode).Scan(&expires)
	if err != nil {
		return expires, err
	}
	if err = audit(ctx, tx, id, "enrollment_issued"); err != nil {
		return expires, err
	}
	return expires, tx.Commit(ctx)
}
func (s *Store) EnrollAgent(ctx context.Context, enrollmentHash, agentHash, mode string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT set_config('app.enrollment_hash',$1,true)", enrollmentHash); err != nil {
		return err
	}
	var host, org, expected string
	err = tx.QueryRow(ctx, "SELECT host_id::text,organization_id::text,mode FROM machine_enrollments WHERE token_hash=$1 AND expires_at>now() AND used_at IS NULL FOR UPDATE", enrollmentHash).Scan(&host, &org, &expected)
	if err != nil {
		return mapError(err)
	}
	if expected != mode {
		return ErrForbidden
	}
	_, err = tx.Exec(ctx, "INSERT INTO machine_agents(host_id,organization_id,token_hash,mode) VALUES($1,$2,$3,$4) ON CONFLICT(host_id) DO UPDATE SET token_hash=excluded.token_hash,mode=excluded.mode,revoked_at=NULL,enrolled_at=now(),metrics='{}'", host, org, agentHash, mode)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE machine_enrollments SET used_at=now() WHERE token_hash=$1", enrollmentHash); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) Heartbeat(ctx context.Context, hash string, m machine.Metrics) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT set_config('app.agent_hash',$1,true)", hash); err != nil {
		return err
	}
	var id string
	err = tx.QueryRow(ctx, "SELECT host_id::text FROM machine_agents WHERE token_hash=$1 AND revoked_at IS NULL FOR UPDATE", hash).Scan(&id)
	if err != nil {
		return mapError(err)
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE machine_agents SET metrics=$2 WHERE host_id=$1", id, b); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE hosts SET status='online',last_seen_at=now() WHERE id=$1", id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) RevokeMachine(ctx context.Context, id string) error {
	tx, _, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var found string
	if err = tx.QueryRow(ctx, "SELECT id::text FROM hosts WHERE id=$1", id).Scan(&found); err != nil {
		return mapError(err)
	}
	if _, err = tx.Exec(ctx, "DELETE FROM machine_enrollments WHERE host_id=$1", id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE machine_agents SET revoked_at=now() WHERE host_id=$1", id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE machine_jobs SET state='cancelled',encrypted=NULL,result='revoked',finished_at=now() WHERE host_id=$1 AND state IN ('queued','running')", id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE protocol_deployments SET state=CASE WHEN state='queued' AND action='deploy' THEN 'cancelled' ELSE 'interrupted' END,encrypted=CASE WHEN state='queued' AND action='deploy' THEN NULL ELSE encrypted END,result='revoked',finished_at=now() WHERE host_id=$1 AND state IN ('queued','running')`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE hosts SET status='offline' WHERE id=$1", id); err != nil {
		return err
	}
	if err = audit(ctx, tx, id, "agent_revoked"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) DeleteCredential(ctx context.Context, id string) error {
	tx, _, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	result, err := tx.Exec(ctx, "DELETE FROM machine_credentials WHERE host_id=$1", id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err = audit(ctx, tx, id, "credential_deleted"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) SavedCredential(ctx context.Context, id string) (machine.SavedCredential, error) {
	var c machine.SavedCredential
	tx, _, err := s.tenantTx(ctx, false, true)
	if err != nil {
		return c, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, "SELECT method,fingerprint,saved_at,encrypted FROM machine_credentials WHERE host_id=$1", id).Scan(&c.Method, &c.Fingerprint, &c.SavedAt, &c.Encrypted)
	return c, mapError(err)
}
func TenantOrg(ctx context.Context) string { v, _ := ctx.Value(scopeKey{}).(scope); return v.Org }
func (s *Store) QueueInstall(ctx context.Context, j machine.Job, enrollmentHash string, c *machine.SavedCredential, target machine.Target) error {
	tx, _, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var actual machine.Target
	err = tx.QueryRow(ctx, "SELECT address,ssh_port,ssh_user FROM hosts WHERE id=$1", j.HostID).Scan(&actual.Address, &actual.Port, &actual.User)
	if err != nil {
		return mapError(err)
	}
	if actual != target {
		return ErrConflict
	}
	if _, err = tx.Exec(ctx, "INSERT INTO machine_jobs(id,organization_id,host_id,created_by,mode,encrypted) VALUES($1,request_org_id(),$2,request_user_id(),$3,$4)", j.ID, j.HostID, j.Mode, j.Encrypted); err != nil {
		return mapError(err)
	}
	if _, err = tx.Exec(ctx, "DELETE FROM machine_enrollments WHERE host_id=$1", j.HostID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO machine_enrollments(token_hash,organization_id,host_id,mode,expires_at,created_by) VALUES($1,request_org_id(),$2,$3,now()+interval '30 minutes',request_user_id())", enrollmentHash, j.HostID, j.Mode); err != nil {
		return err
	}
	if c != nil {
		if _, err = tx.Exec(ctx, "INSERT INTO machine_credentials(host_id,organization_id,method,fingerprint,encrypted) VALUES($1,request_org_id(),$2,$3,$4) ON CONFLICT(host_id) DO UPDATE SET method=excluded.method,fingerprint=excluded.fingerprint,encrypted=excluded.encrypted,saved_at=now()", j.HostID, c.Method, c.Fingerprint, c.Encrypted); err != nil {
			return err
		}
		if err = audit(ctx, tx, j.HostID, "credential_saved"); err != nil {
			return err
		}
	}
	if err = audit(ctx, tx, j.HostID, "ssh_install_queued"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) ClaimMachineJob(ctx context.Context) (machine.Job, error) {
	var j machine.Job
	err := s.Pool.QueryRow(ctx, "SELECT id::text,organization_id::text,created_by::text,lease::text FROM claim_machine_job()").Scan(&j.ID, &j.OrgID, &j.UserID, &j.Lease)
	return j, mapError(err)
}
func (s *Store) LoadMachineJob(ctx context.Context, claim machine.Job) (machine.Job, error) {
	tx, _, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return claim, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, "SELECT host_id::text,mode,encrypted FROM machine_jobs WHERE id=$1 AND lease=$2 AND state='running' AND lease_until>now()", claim.ID, claim.Lease).Scan(&claim.HostID, &claim.Mode, &claim.Encrypted)
	return claim, mapError(err)
}
func (s *Store) FinishMachineJob(ctx context.Context, j machine.Job, state, result string) error {
	tx, _, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, "UPDATE machine_jobs SET state=$3,result=$4,encrypted=NULL,finished_at=now() WHERE id=$1 AND lease=$2 AND state='running' AND lease_until>now()", j.ID, j.Lease, state, result)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	if err = audit(ctx, tx, j.HostID, "ssh_install_"+state); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) FailUnauthorizedJob(ctx context.Context, j machine.Job) error {
	_, err := s.Pool.Exec(ctx, "SELECT fail_machine_job($1,$2)", j.ID, j.Lease)
	return err
}
