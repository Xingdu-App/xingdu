package storage

import (
	"context"
	"time"
)

// Node is the non-secret inventory of a confirmed installation. HostStatus is
// Agent connectivity, not a health check of the protocol service.
type Node struct {
	Deployment
	HostName    string    `json:"host_name"`
	Address     string    `json:"address"`
	HostStatus  string    `json:"host_status"`
	InstalledAt time.Time `json:"installed_at"`
}

func (s *Store) Nodes(ctx context.Context) ([]Node, error) {
	out := make([]Node, 0)
	tx, _, err := s.tenantTx(ctx, false, false)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT d.id::text,d.host_id::text,d.name,d.protocol,d.port,d.server_name,
		d.state,d.action,d.result,d.created_at,d.finished_at,h.name,h.address,
		CASE WHEN h.status='online' AND h.last_seen_at<now()-interval '90 seconds' THEN 'offline' ELSE h.status END,
		d.installed_at,d.certificate_expires_at,d.service_status,d.service_checked_at,d.revision,d.pending_revision,COALESCE(d.relay_exit_id,''),d.probe_ok,d.probe_at,d.probe_latency_ms,d.probe_exit_ip
		FROM protocol_deployments d JOIN hosts h ON h.id=d.host_id AND h.organization_id=d.organization_id
		WHERE d.installed_at IS NOT NULL AND d.state <> 'removed'
		ORDER BY d.installed_at DESC,d.id`)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var n Node
		if err = rows.Scan(&n.ID, &n.HostID, &n.Name, &n.Protocol, &n.Port, &n.ServerName,
			&n.State, &n.Action, &n.Result, &n.CreatedAt, &n.FinishedAt, &n.HostName,
			&n.Address, &n.HostStatus, &n.InstalledAt, &n.CertificateExpiresAt, &n.ServiceStatus, &n.ServiceCheckedAt, &n.Revision, &n.PendingRevision, &n.RelayExitID, &n.ProbeOK, &n.ProbeAt, &n.ProbeLatencyMS, &n.ProbeExitIP); err != nil {
			rows.Close()
			return out, err
		}
		out = append(out, n)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
