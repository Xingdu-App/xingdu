package storage

import "context"

// DiagnosticDeployment reads the installed revision under admin and tenant
// scope, without exposing credentials or recording a credential-reveal event.
func (s *Store) DiagnosticDeployment(ctx context.Context, host, node string) (Deployment, error) {
	d := Deployment{ID: node, HostID: host}
	tx, _, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return d, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `SELECT d.encrypted,h.address,d.revision FROM protocol_deployments d JOIN hosts h ON h.id=d.host_id AND h.organization_id=d.organization_id WHERE d.id=$1 AND d.host_id=$2 AND d.installed_at IS NOT NULL AND d.state<>'removed' AND d.action<>'remove'`, node, host).Scan(&d.Encrypted, &d.Server, &d.Revision)
	if err != nil {
		return d, mapError(err)
	}
	if err = audit(ctx, tx, host, "node_diagnosis_requested"); err != nil {
		return d, err
	}
	return d, tx.Commit(ctx)
}
