package storage

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
	"xingdu.app/xingdu/internal/machine"
	"xingdu.app/xingdu/internal/protocol"
)

type Deployment struct {
	MinimumAgentVersion  string     `json:"-"`
	CertificateID        string     `json:"-"`
	CertificateCipher    []byte     `json:"-"`
	RuntimeVersion       string     `json:"runtime_version"`
	ProbeOK              *bool      `json:"probe_ok"`
	ProbeAt              *time.Time `json:"probe_at"`
	ProbeLatencyMS       *int       `json:"probe_latency_ms"`
	ProbeExitIP          *string    `json:"probe_exit_ip"`
	ExternalExitID       string     `json:"external_exit_id,omitempty"`
	ExternalRevision     int        `json:"-"`
	ExternalCipher       []byte     `json:"-"`
	RelayExitID          string     `json:"relay_exit_id,omitempty"`
	ExitCipher           []byte     `json:"-"`
	ID                   string     `json:"id"`
	HostID               string     `json:"host_id"`
	Name                 string     `json:"name"`
	Protocol             string     `json:"protocol"`
	Port                 int        `json:"port"`
	ServerName           string     `json:"server_name"`
	State                string     `json:"state"`
	Action               string     `json:"action"`
	Result               string     `json:"result"`
	CreatedAt            time.Time  `json:"created_at"`
	FinishedAt           *time.Time `json:"finished_at"`
	OrgID                string     `json:"-"`
	UserID               string     `json:"-"`
	OperationID          string     `json:"-"`
	Lease                string     `json:"-"`
	Encrypted            []byte     `json:"-"`
	Server               string     `json:"-"`
	CertificateExpiresAt *time.Time `json:"certificate_expires_at"`
	ServiceStatus        string     `json:"service_status"`
	ServiceCheckedAt     *time.Time `json:"service_checked_at"`
	Revision             int        `json:"revision"`
	PendingRevision      *int       `json:"pending_revision"`
}

const deploymentColumns = "id::text,host_id::text,name,protocol,port,server_name,state,action,result,created_at,finished_at,certificate_expires_at,service_status,service_checked_at,revision,pending_revision,COALESCE(relay_exit_id,''),probe_ok,probe_at,probe_latency_ms,probe_exit_ip,runtime_version,COALESCE(external_exit_id,'')"

func scanDeployment(row pgx.Row, d *Deployment) error {
	return row.Scan(&d.ID, &d.HostID, &d.Name, &d.Protocol, &d.Port, &d.ServerName, &d.State, &d.Action, &d.Result, &d.CreatedAt, &d.FinishedAt, &d.CertificateExpiresAt, &d.ServiceStatus, &d.ServiceCheckedAt, &d.Revision, &d.PendingRevision, &d.RelayExitID, &d.ProbeOK, &d.ProbeAt, &d.ProbeLatencyMS, &d.ProbeExitIP, &d.RuntimeVersion, &d.ExternalExitID)
}
func cleanupDeployments(ctx context.Context, tx pgx.Tx, host string) error {
	_, err := tx.Exec(ctx, `UPDATE protocol_deployments SET state=CASE WHEN state='queued' AND action='deploy' THEN 'cancelled' ELSE 'interrupted' END,result='interrupted_or_expired',encrypted=CASE WHEN state='queued' AND action='deploy' THEN NULL ELSE encrypted END,finished_at=now() WHERE host_id=$1 AND ((state='running' AND lease_until<=now()) OR (state='queued' AND queued_at<now()-interval '30 minutes'))`, host)
	return err
}
func managingAgent(ctx context.Context, tx pgx.Tx, host string, minimum ...string) (string, error) {
	required := machine.MinimumDeploymentVersion
	if len(minimum) > 0 {
		required = minimum[0]
	}
	var pending bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM machine_jobs WHERE host_id=$1 AND action='upgrade' AND state IN ('queued','running'))`, host).Scan(&pending); err != nil {
		return "", err
	}
	if pending {
		return "", ErrConflict
	}
	var hash, version string
	err := tx.QueryRow(ctx, `SELECT a.token_hash,COALESCE(a.metrics->>'version','') FROM machine_agents a JOIN hosts h ON h.id=a.host_id WHERE h.id=$1 AND a.mode='manage' AND a.revoked_at IS NULL AND h.last_seen_at>now()-interval '90 seconds' FOR UPDATE OF a`, host).Scan(&hash, &version)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !machine.VersionAtLeast(version, required)) {
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
	if d.CertificateID != "" {
		if err = validateDeploymentCertificate(ctx, tx, d); err != nil {
			return err
		}
	}
	minimum := protocol.MinimumAgentVersion(d.Protocol)
	if machine.VersionAtLeast(d.MinimumAgentVersion, minimum) {
		minimum = d.MinimumAgentVersion
	}
	hash, err := managingAgent(ctx, tx, d.HostID, minimum)
	if err != nil {
		return err
	}
	if err = cleanupDeployments(ctx, tx, d.HostID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO protocol_deployments(id,organization_id,host_id,created_by,name,protocol,port,server_name,operation_id,encrypted,agent_hash,certificate_expires_at,minimum_agent_version) VALUES($1,request_org_id(),$2,request_user_id(),$3,$4,$5,$6,$7,$8,$9,$10,$11)`, d.ID, d.HostID, d.Name, d.Protocol, d.Port, d.ServerName, d.OperationID, d.Encrypted, hash, d.CertificateExpiresAt, minimum)
	if err != nil {
		return mapError(err)
	}
	if err = audit(ctx, tx, d.HostID, "protocol_deploy_queued"); err != nil {
		return err
	}
	if d.CertificateID != "" {
		if err = certificateAudit(ctx, tx, d.CertificateID, "certificate_deploy_queued"); err != nil {
			return err
		}
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
	err = tx.QueryRow(ctx, `SELECT d.encrypted,h.address,COALESCE(d.relay_exit_id,''),COALESCE(d.external_exit_id,'') FROM protocol_deployments d JOIN hosts h ON h.id=d.host_id WHERE d.id=$1 AND d.host_id=$2 AND d.installed_at IS NOT NULL AND d.state <> 'removed' AND d.action <> 'remove'`, id, host).Scan(&d.Encrypted, &d.Server, &d.RelayExitID, &d.ExternalExitID)
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
	if err = checkRelayDependents(ctx, tx, id); err != nil {
		return err
	}
	var kind, minimum string
	if err = tx.QueryRow(ctx, `SELECT protocol,minimum_agent_version FROM protocol_deployments WHERE id=$1 AND host_id=$2`, id, host).Scan(&kind, &minimum); err != nil {
		return mapError(err)
	}
	if !machine.VersionAtLeast(minimum, protocol.MinimumAgentVersion(kind)) {
		minimum = protocol.MinimumAgentVersion(kind)
	}
	hash, err := managingAgent(ctx, tx, host, minimum)
	if err != nil {
		return err
	}
	if err = cleanupDeployments(ctx, tx, host); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE protocol_deployments SET action='remove',state='queued',operation_id=$3,created_by=request_user_id(),agent_hash=$4,lease=NULL,lease_until=NULL,queued_at=now(),finished_at=NULL,result='' WHERE id=$1 AND host_id=$2 AND state IN ('succeeded','failed','interrupted') AND encrypted IS NOT NULL`, id, host, NewID("op"), hash)
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
	var version string
	if err = tx.QueryRow(ctx, `SELECT COALESCE(metrics->>'version','') FROM machine_agents WHERE token_hash=$1`, hash).Scan(&version); err != nil {
		return nil, err
	}
	if !machine.VersionAtLeast(version, machine.MinimumDeploymentVersion) {
		return nil, ErrConflict
	}
	if err = cleanupDeployments(ctx, tx, host); err != nil {
		return nil, err
	}
	var d Deployment
	d.OrgID = org
	d.HostID = host
	err = tx.QueryRow(ctx, `SELECT id::text,operation_id::text,created_by::text,action,encrypted,COALESCE(lease::text,''),protocol,COALESCE(relay_exit_id,''),minimum_agent_version FROM protocol_deployments WHERE host_id=$1 AND state IN ('queued','running') ORDER BY queued_at LIMIT 1 FOR UPDATE`, host).Scan(&d.ID, &d.OperationID, &d.UserID, &d.Action, &d.Encrypted, &d.Lease, &d.Protocol, &d.RelayExitID, &d.MinimumAgentVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, tx.Commit(ctx)
	}
	if err != nil {
		return nil, err
	}
	if !machine.VersionAtLeast(version, protocol.MinimumAgentVersion(d.Protocol)) || !machine.VersionAtLeast(version, d.MinimumAgentVersion) {
		return nil, ErrConflict
	}
	if d.Action == "update" {
		if !machine.VersionAtLeast(version, "0.12.0-dev") {
			return nil, ErrConflict
		}
		if err = tx.QueryRow(ctx, `SELECT r.encrypted,COALESCE(r.relay_exit_id,'') FROM protocol_revisions r JOIN protocol_deployments d ON d.id=r.node_id AND d.organization_id=r.organization_id WHERE d.id=$1 AND r.revision=d.pending_revision`, d.ID).Scan(&d.Encrypted, &d.RelayExitID); err != nil {
			return nil, err
		}
	}
	if err = setScope(ctx, tx, d.UserID, org); err != nil {
		return nil, err
	}
	if d.RelayExitID != "" {
		var exitKind string
		if err = tx.QueryRow(ctx, `SELECT protocol FROM protocol_deployments WHERE id=$1`, d.RelayExitID).Scan(&exitKind); err != nil {
			return nil, err
		}
		if !machine.VersionAtLeast(version, protocol.MinimumAgentVersion(exitKind)) {
			return nil, ErrConflict
		}
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
	err = tx.QueryRow(ctx, `UPDATE protocol_deployments SET state='running',lease=COALESCE(lease,new_resource_id('lease')),lease_until=COALESCE(lease_until,now()+$2*interval '1 second') WHERE id=$1 RETURNING lease::text`, d.ID, int(protocol.DeploymentLease/time.Second)).Scan(&d.Lease)
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
	if success && ((d.Action == "deploy" && code != "deployed") || (d.Action == "remove" && code != "removed") || (d.Action == "restart" && code != "restarted") || (d.Action == "update" && code != "updated")) || !success && (code == "deployed" || code == "removed" || code == "restarted" || code == "updated") {
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
	if d.Action == "update" && allowed {
		if success {
			_, err = tx.Exec(ctx, `UPDATE protocol_deployments d SET encrypted=r.encrypted,port=r.port,name=r.name,server_name=r.server_name,certificate_expires_at=r.certificate_expires_at,probe_ok=NULL,probe_at=NULL,revision=r.revision,relay_exit_id=r.relay_exit_id,external_exit_id=r.external_exit_id,pending_revision=NULL,action='deploy' FROM protocol_revisions r WHERE d.id=$1 AND r.node_id=d.id AND r.organization_id=d.organization_id AND r.revision=d.pending_revision`, d.ID)
		} else if code != "rollback_failed" && code != "interrupted" && code != "journal_unavailable" {
			_, err = tx.Exec(ctx, `UPDATE protocol_deployments SET pending_revision=NULL WHERE id=$1`, d.ID)
		}
		if err != nil {
			return err
		}
	}
	if state == "removed" {
		if _, err = tx.Exec(ctx, `DELETE FROM protocol_revisions WHERE node_id=$1`, d.ID); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE protocol_deployments SET state=$2,result=$3,action=CASE WHEN action='restart' AND $2='succeeded' THEN 'deploy' ELSE action END,finished_at=now(),installed_at=CASE WHEN $2='succeeded' AND action='deploy' THEN COALESCE(installed_at,now()) ELSE installed_at END,encrypted=CASE WHEN $2='removed' THEN NULL ELSE encrypted END,relay_exit_id=CASE WHEN $2='removed' THEN NULL ELSE relay_exit_id END,external_exit_id=CASE WHEN $2='removed' THEN NULL ELSE external_exit_id END WHERE id=$1`, d.ID, state, code)
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
