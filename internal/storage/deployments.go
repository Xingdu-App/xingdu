package storage

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
)

type Deployment struct {
	ID          string     `json:"id"`
	HostID      string     `json:"host_id"`
	Name        string     `json:"name"`
	Protocol    string     `json:"protocol"`
	Port        int        `json:"port"`
	ServerName  string     `json:"server_name"`
	State       string     `json:"state"`
	Action      string     `json:"action"`
	Result      string     `json:"result"`
	CreatedAt   time.Time  `json:"created_at"`
	FinishedAt  *time.Time `json:"finished_at"`
	OrgID       string     `json:"-"`
	UserID      string     `json:"-"`
	OperationID string     `json:"-"`
	Lease       string     `json:"-"`
	Encrypted   []byte     `json:"-"`
	Server      string     `json:"-"`
}

const deploymentColumns = "id::text,host_id::text,name,protocol,port,server_name,state,action,result,created_at,finished_at"

func scanDeployment(row pgx.Row, d *Deployment) error {
	return row.Scan(&d.ID, &d.HostID, &d.Name, &d.Protocol, &d.Port, &d.ServerName, &d.State, &d.Action, &d.Result, &d.CreatedAt, &d.FinishedAt)
}
func cleanupDeployments(ctx context.Context, tx pgx.Tx, host string) error {
	_, err := tx.Exec(ctx, `UPDATE protocol_deployments SET state=CASE WHEN state='queued' AND action='deploy' THEN 'cancelled' ELSE 'interrupted' END,result='interrupted_or_expired',encrypted=CASE WHEN state='queued' AND action='deploy' THEN NULL ELSE encrypted END,finished_at=now() WHERE host_id=$1 AND ((state='running' AND lease_until<=now()) OR (state='queued' AND queued_at<now()-interval '30 minutes'))`, host)
	return err
}
func managingAgent(ctx context.Context, tx pgx.Tx, host string) (string, error) {
	var hash string
	err := tx.QueryRow(ctx, `SELECT a.token_hash FROM machine_agents a JOIN hosts h ON h.id=a.host_id WHERE h.id=$1 AND a.mode='manage' AND a.revoked_at IS NULL AND h.last_seen_at>now()-interval '90 seconds' AND a.metrics->>'version'='0.5.0-dev' FOR UPDATE OF a`, host).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrConflict
	}
	return hash, err
}
func (s *Store) Deployments(ctx context.Context, host string) ([]Deployment, error) {
	out := []Deployment{}
	tx, role, err := s.tenantTx(ctx, false, false)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	var found string
	if err = tx.QueryRow(ctx, "SELECT id::text FROM hosts WHERE id=$1", host).Scan(&found); err != nil {
		return out, mapError(err)
	}
	if role == "owner" || role == "admin" {
		if err = cleanupDeployments(ctx, tx, host); err != nil {
			return out, err
		}
	}
	rows, err := tx.Query(ctx, "SELECT "+deploymentColumns+" FROM protocol_deployments WHERE host_id=$1 ORDER BY created_at DESC", host)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var d Deployment
		if err = scanDeployment(rows, &d); err != nil {
			rows.Close()
			return out, err
		}
		out = append(out, d)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
func (s *Store) QueueDeployment(ctx context.Context, d Deployment) error {
	tx, _, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	hash, err := managingAgent(ctx, tx, d.HostID)
	if err != nil {
		return err
	}
	if err = cleanupDeployments(ctx, tx, d.HostID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO protocol_deployments(id,organization_id,host_id,created_by,name,protocol,port,server_name,operation_id,encrypted,agent_hash) VALUES($1,request_org_id(),$2,request_user_id(),$3,$4,$5,$6,$7,$8,$9)`, d.ID, d.HostID, d.Name, d.Protocol, d.Port, d.ServerName, d.OperationID, d.Encrypted, hash)
	if err != nil {
		return mapError(err)
	}
	if err = audit(ctx, tx, d.HostID, "protocol_deploy_queued"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) DeploymentSecret(ctx context.Context, host, id string) (Deployment, error) {
	d := Deployment{ID: id, HostID: host}
	tx, _, err := s.tenantTx(ctx, false, true)
	if err != nil {
		return d, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `SELECT d.encrypted,h.address FROM protocol_deployments d JOIN hosts h ON h.id=d.host_id WHERE d.id=$1 AND d.host_id=$2 AND d.state='succeeded' AND d.action='deploy'`, id, host).Scan(&d.Encrypted, &d.Server)
	if err != nil {
		return d, mapError(err)
	}
	if err = audit(ctx, tx, host, "protocol_connection_revealed"); err != nil {
		return d, err
	}
	return d, tx.Commit(ctx)
}
func (s *Store) RemoveDeployment(ctx context.Context, host, id string) error {
	tx, _, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	hash, err := managingAgent(ctx, tx, host)
	if err != nil {
		return err
	}
	if err = cleanupDeployments(ctx, tx, host); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE protocol_deployments SET action='remove',state='queued',operation_id=$3,created_by=request_user_id(),agent_hash=$4,lease=NULL,lease_until=NULL,queued_at=now(),finished_at=NULL,result='' WHERE id=$1 AND host_id=$2 AND state IN ('succeeded','failed','interrupted') AND encrypted IS NOT NULL`, id, host, NewID(), hash)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	if err = audit(ctx, tx, host, "protocol_remove_queued"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) deploymentAgentTx(ctx context.Context, hash string) (pgx.Tx, string, string, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, "", "", err
	}
	fail := func(e error) (pgx.Tx, string, string, error) { tx.Rollback(ctx); return nil, "", "", e }
	var host, org string
	if err = tx.QueryRow(ctx, "SELECT host_id::text,organization_id::text FROM deployment_agent_context($1)", hash).Scan(&host, &org); err != nil {
		return fail(mapError(err))
	}
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", org); err != nil {
		return fail(err)
	}
	if err = setScope(ctx, tx, "", org); err != nil {
		return fail(err)
	}
	if _, err = tx.Exec(ctx, "SELECT set_config('app.agent_hash',$1,true)", hash); err != nil {
		return fail(err)
	}
	var id string
	if err = tx.QueryRow(ctx, "SELECT host_id::text FROM machine_agents WHERE host_id=$1 AND token_hash=$2 AND mode='manage' AND revoked_at IS NULL FOR UPDATE", host, hash).Scan(&id); err != nil {
		return fail(mapError(err))
	}
	return tx, host, org, nil
}
func (s *Store) ClaimDeployment(ctx context.Context, hash string) (*Deployment, error) {
	tx, host, org, err := s.deploymentAgentTx(ctx, hash)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = cleanupDeployments(ctx, tx, host); err != nil {
		return nil, err
	}
	var d Deployment
	d.OrgID = org
	d.HostID = host
	err = tx.QueryRow(ctx, `SELECT id::text,operation_id::text,created_by::text,action,encrypted,COALESCE(lease::text,'') FROM protocol_deployments WHERE host_id=$1 AND state IN ('queued','running') ORDER BY queued_at LIMIT 1 FOR UPDATE`, host).Scan(&d.ID, &d.OperationID, &d.UserID, &d.Action, &d.Encrypted, &d.Lease)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, tx.Commit(ctx)
	}
	if err != nil {
		return nil, err
	}
	if err = setScope(ctx, tx, d.UserID, org); err != nil {
		return nil, err
	}
	var allowed bool
	err = tx.QueryRow(ctx, `SELECT COALESCE(tenant_role(request_org_id()) IN ('owner','admin'),false) AND EXISTS(SELECT 1 FROM protocol_deployments WHERE id=$1 AND agent_hash=$2)`, d.ID, hash).Scan(&allowed)
	if err != nil {
		return nil, err
	}
	if !allowed {
		_, err = tx.Exec(ctx, `UPDATE protocol_deployments SET state=CASE WHEN state='queued' AND action='deploy' THEN 'cancelled' ELSE 'interrupted' END,encrypted=CASE WHEN state='queued' AND action='deploy' THEN NULL ELSE encrypted END,result='authorization_revoked',finished_at=now() WHERE id=$1`, d.ID)
		if err != nil {
			return nil, err
		}
		return nil, tx.Commit(ctx)
	}
	err = tx.QueryRow(ctx, `UPDATE protocol_deployments SET state='running',lease=COALESCE(lease,gen_random_uuid()),lease_until=COALESCE(lease_until,now()+interval '5 minutes') WHERE id=$1 RETURNING lease::text`, d.ID).Scan(&d.Lease)
	if err != nil {
		return nil, err
	}
	return &d, tx.Commit(ctx)
}
func (s *Store) FinishDeployment(ctx context.Context, hash, id, lease string, success bool, code string) error {
	tx, host, org, err := s.deploymentAgentTx(ctx, hash)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var d Deployment
	var live bool
	err = tx.QueryRow(ctx, `SELECT id::text,created_by::text,action,state,result,lease_until>now() FROM protocol_deployments WHERE operation_id=$1 AND host_id=$2 AND agent_hash=$3 AND lease=$4 FOR UPDATE`, id, host, hash, lease).Scan(&d.ID, &d.UserID, &d.Action, &d.State, &d.Result, &live)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	if d.State != "running" {
		if d.Result == code && ((success && (d.State == "succeeded" || d.State == "removed")) || (!success && d.State == "failed")) {
			return tx.Commit(ctx)
		}
		return ErrConflict
	}
	if !live {
		return ErrConflict
	}
	if success && ((d.Action == "deploy" && code != "deployed") || (d.Action == "remove" && code != "removed")) || !success && (code == "deployed" || code == "removed") {
		return ErrInvalid
	}
	if err = setScope(ctx, tx, d.UserID, org); err != nil {
		return err
	}
	var allowed bool
	if err = tx.QueryRow(ctx, "SELECT COALESCE(tenant_role(request_org_id()) IN ('owner','admin'),false)").Scan(&allowed); err != nil {
		return err
	}
	state := "failed"
	if !allowed {
		state = "interrupted"
		code = "authorization_revoked"
	} else if success {
		state = "succeeded"
		if d.Action == "remove" {
			state = "removed"
		}
	}
	_, err = tx.Exec(ctx, `UPDATE protocol_deployments SET state=$2,result=$3,finished_at=now(),encrypted=CASE WHEN $2='removed' THEN NULL ELSE encrypted END WHERE id=$1`, d.ID, state, code)
	if err != nil {
		return err
	}
	// Revoked initiators cannot write tenant audit rows; the durable sanitized result records their cancellation.
	if allowed {
		if err = audit(ctx, tx, host, "protocol_"+d.Action+"_"+state); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
