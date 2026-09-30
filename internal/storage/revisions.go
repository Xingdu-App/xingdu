package storage

import (
	"bytes"
	"context"
	"time"
	"xingdu.app/xingdu/internal/machine"
	"xingdu.app/xingdu/internal/protocol"
)

type Revision struct {
	Revision   int       `json:"revision"`
	Name       string    `json:"name"`
	ServerName string    `json:"server_name"`
	CreatedAt  time.Time `json:"created_at"`
	Current    bool      `json:"current"`
	Pending    bool      `json:"pending"`
}

func (s *Store) Revisions(ctx context.Context, host, node string) ([]Revision, error) {
	tx, _, err := s.tenantTx(ctx, false, true)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT r.revision,r.name,r.server_name,r.created_at,r.revision=d.revision,COALESCE(r.revision=d.pending_revision,false) FROM protocol_revisions r JOIN protocol_deployments d ON d.id=r.node_id AND d.organization_id=r.organization_id WHERE d.id=$1 AND d.host_id=$2 ORDER BY r.revision DESC`, node, host)
	if err != nil {
		return nil, err
	}
	out := []Revision{}
	for rows.Next() {
		var r Revision
		if err = rows.Scan(&r.Revision, &r.Name, &r.ServerName, &r.CreatedAt, &r.Current, &r.Pending); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}
func (s *Store) RevisionSecret(ctx context.Context, host, node string, revision int) (Deployment, error) {
	d := Deployment{ID: node, HostID: host}
	tx, _, err := s.tenantTx(ctx, false, true)
	if err != nil {
		return d, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `SELECT r.encrypted FROM protocol_revisions r JOIN protocol_deployments d ON d.id=r.node_id AND d.organization_id=r.organization_id WHERE d.id=$1 AND d.host_id=$2 AND r.revision=$3 AND d.installed_at IS NOT NULL AND d.state<>'removed'`, node, host, revision).Scan(&d.Encrypted)
	if err != nil {
		return d, mapError(err)
	}
	return d, tx.Commit(ctx)
}

// The previous ciphertext is an optimistic concurrency token, never supplied by
// browsers. Keep the active configuration available until Agent acknowledgement.
func (s *Store) UpdateDeployment(ctx context.Context, d Deployment, previous []byte) error {
	tx, _, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if d.CertificateID != "" {
		if e := certificateEntitlement(ctx, tx, d.CertificateID); e != nil {
			return e
		}
		var valid bool
		if e := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM managed_certificates WHERE id=$1 AND state<>'deleted' AND domain=$2 AND encrypted=$3 AND expires_at>now())`, d.CertificateID, d.ServerName, d.CertificateCipher).Scan(&valid); e != nil {
			return e
		}
		if !valid {
			return ErrConflict
		}
	}
	if err = checkRelayDependents(ctx, tx, d.ID); err != nil {
		return err
	}
	if d.RelayExitID != "" {
		if err = checkRelayExit(ctx, tx, d); err != nil {
			return err
		}
	}
	minimum := "0.12.0-dev"
	if required := protocol.MinimumAgentVersion(d.Protocol); !machine.VersionAtLeast(minimum, required) {
		minimum = required
	}
	if d.RelayExitID != "" {
		var exitKind string
		if err := tx.QueryRow(ctx, `SELECT protocol FROM protocol_deployments WHERE id=$1`, d.RelayExitID).Scan(&exitKind); err != nil {
			return mapError(err)
		}
		if required := protocol.MinimumAgentVersion(exitKind); !machine.VersionAtLeast(minimum, required) {
			minimum = required
		}
	}
	hash, err := managingAgent(ctx, tx, d.HostID, minimum)
	if err != nil {
		return err
	}
	if err = cleanupDeployments(ctx, tx, d.HostID); err != nil {
		return err
	}
	var old []byte
	var kind string
	var port int
	var pending bool
	err = tx.QueryRow(ctx, `SELECT encrypted,protocol,port,pending_revision IS NOT NULL FROM protocol_deployments WHERE id=$1 AND host_id=$2 AND installed_at IS NOT NULL AND state IN ('succeeded','failed') AND action<>'remove' FOR UPDATE`, d.ID, d.HostID).Scan(&old, &kind, &port, &pending)
	if err != nil {
		return mapError(err)
	}
	if pending || !bytes.Equal(old, previous) || kind != d.Protocol {
		return ErrConflict
	}
	if port != d.Port {
		var occupied bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM protocol_deployments WHERE host_id=$1 AND id<>$2 AND port=$3 AND state NOT IN ('removed','cancelled'))`, d.HostID, d.ID, d.Port).Scan(&occupied); err != nil {
			return err
		}
		if occupied {
			return ErrConflict
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO protocol_revisions(organization_id,node_id,revision,name,server_name,certificate_expires_at,encrypted,relay_exit_id,port) SELECT organization_id,id,revision,name,server_name,certificate_expires_at,encrypted,relay_exit_id,port FROM protocol_deployments WHERE id=$1 ON CONFLICT DO NOTHING`, d.ID)
	if err != nil {
		return err
	}
	var rev int
	err = tx.QueryRow(ctx, `SELECT COALESCE(max(revision),0)+1 FROM protocol_revisions WHERE node_id=$1`, d.ID).Scan(&rev)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO protocol_revisions(organization_id,node_id,revision,name,server_name,certificate_expires_at,encrypted,relay_exit_id,port) VALUES(request_org_id(),$1,$2,$3,$4,$5,$6,NULLIF($7,''),$8)`, d.ID, rev, d.Name, d.ServerName, d.CertificateExpiresAt, d.Encrypted, d.RelayExitID, d.Port)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE protocol_deployments SET pending_revision=$2,action='update',state='queued',operation_id=$3,created_by=request_user_id(),agent_hash=$4,lease=NULL,lease_until=NULL,queued_at=now(),finished_at=NULL,result='' WHERE id=$1`, d.ID, rev, NewID("op"), hash)
	if err != nil {
		return mapError(err)
	}
	if err = audit(ctx, tx, d.HostID, "protocol_update_queued"); err != nil {
		return err
	}
	if d.CertificateID != "" {
		if err = certificateAudit(ctx, tx, d.CertificateID, "certificate_apply_queued"); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
