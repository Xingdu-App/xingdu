package storage

import (
	"context"
	"time"
)

type ResourceUsage struct {
	Used  int `json:"used"`
	Limit int `json:"limit"`
}
type AuditEvent struct {
	Kind       string    `json:"kind"`
	ResourceID string    `json:"resource_id"`
	ActorID    string    `json:"actor_id"`
	Event      string    `json:"event"`
	CreatedAt  time.Time `json:"created_at"`
}
type Operations struct {
	Usage map[string]ResourceUsage `json:"usage"`
	Audit []AuditEvent             `json:"audit"`
}

func (s *Store) OrganizationOperations(ctx context.Context) (Operations, error) {
	out := Operations{Usage: map[string]ResourceUsage{}, Audit: []AuditEvent{}}
	tx, _, err := s.tenantTx(ctx, false, true)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT 'hosts', (SELECT count(*) FROM hosts WHERE organization_id=request_org_id()),coalesce((SELECT hosts FROM organization_limits WHERE organization_id=request_org_id()),25)
 UNION ALL SELECT 'deployments',(SELECT count(*) FROM protocol_deployments WHERE organization_id=request_org_id() AND state NOT IN ('removed','cancelled')),coalesce((SELECT deployments FROM organization_limits WHERE organization_id=request_org_id()),100)
 UNION ALL SELECT 'subscriptions',(SELECT count(*) FROM subscriptions WHERE organization_id=request_org_id()),coalesce((SELECT subscriptions FROM organization_limits WHERE organization_id=request_org_id()),100)
 UNION ALL SELECT 'members',(SELECT count(*) FROM memberships WHERE organization_id=request_org_id()),coalesce((SELECT members FROM organization_limits WHERE organization_id=request_org_id()),20)`)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var name string
		var r ResourceUsage
		if err = rows.Scan(&name, &r.Used, &r.Limit); err != nil {
			rows.Close()
			return out, err
		}
		out.Usage[name] = r
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if s.CloudBilling {
		hosts := out.Usage["hosts"]
		if hosts.Limit > 5 {
			hosts.Limit = 5
		}
		out.Usage["hosts"] = hosts
	}
	rows, err = tx.Query(ctx, `SELECT kind,resource_id,actor_id,event,created_at FROM (
 SELECT 'machine' AS kind,host_id::text AS resource_id,actor_id::text AS actor_id,event,created_at,id FROM machine_audit WHERE organization_id=request_org_id()
 UNION ALL SELECT 'subscription',subscription_id::text,actor_id::text,event,created_at,id FROM subscription_audit WHERE organization_id=request_org_id()
 UNION ALL SELECT 'organization',organization_id::text,actor_id::text,event,created_at,id FROM organization_audit WHERE organization_id=request_org_id()
 ) events ORDER BY created_at DESC,kind,id DESC LIMIT 100`)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var e AuditEvent
		if err = rows.Scan(&e.Kind, &e.ResourceID, &e.ActorID, &e.Event, &e.CreatedAt); err != nil {
			return out, err
		}
		out.Audit = append(out.Audit, e)
	}
	return out, rows.Err()
}
