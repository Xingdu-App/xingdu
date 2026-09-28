# SaaS operations

The hosted service is still in development. A green readiness endpoint is not a public launch or proof of node connectivity.

## Organization limits

PostgreSQL enforces per-organization limits for new resources: 25 hosts, 100 active deployment records, 100 subscriptions and 20 members. Removed/cancelled deployments release capacity; disabled subscriptions still count. Existing resources are not removed when limits decrease. These defaults are operational caps, not billed plans.

`organization_limits` overrides defaults for an organization. Only the deployment database administrator can change limits. The application role has read permission under RLS; organization owners cannot raise their own limit. Operators can use a parameterized SQL statement equivalent to:

```sql
INSERT INTO organization_limits(organization_id, hosts, deployments, subscriptions, members)
VALUES ('org_00000000000000000000000000000000', 25, 100, 100, 20)
ON CONFLICT (organization_id) DO UPDATE
SET hosts=excluded.hosts, deployments=excluded.deployments,
    subscriptions=excluded.subscriptions, members=excluded.members;
```

Replace the placeholder with the intended organization's complete `org_` ID (see [ID design](ID-DESIGN.md)). An advisory transaction lock serializes resource creation and the quota check across API processes. API exhaustion returns HTTP 409 `quota_exceeded`. Limits do not cap total organization/account creation, storage or bandwidth. Registration should remain closed until operator-level limits and abuse controls are configured.

## Audit and monitoring

Organization owners/admins can view usage and the most recent 100 machine, subscription and organization events in Settings. This is a bounded view, not an immutable compliance ledger or an archival search engine. Resource and actor IDs identify deleted resources without exposing credentials.

- `/health/live`: process liveness, no database readiness guarantee.
- `/health/ready`: all migrations embedded in the running binary must exist; use it for load-balancer readiness.
- Agent heartbeat freshness and managed service status are distinct. An `active` service is not proof of Internet reachability, protocol authentication, throughput or the exit address.
- Certificate expiry is recorded for new deployments; older installations may have no metadata until explicitly updated.

Run external uptime checks from a separate network. Alert delivery integrations, persistent cross-instance rate limits, metrics export and credential-vault key rotation are still pending. Current authentication throttling is process-local; horizontal scaling alone does not provide distributed throttling.

## Recovery and releases

See [Backup and restore](BACKUP.md) for encrypted database archives and isolated restores. Keep the credential vault key separately: database backup alone cannot recover encrypted SSH/runtime credentials. Agent disks and deployed services are separate state and must be reconciled after restoring the control plane. Do not let two restored control planes operate the same agents.

Before a public release, run the full PostgreSQL integration suite, the supported architecture/runtime matrix and real client import/forwarding checks. Build artifacts need signed provenance and a tested update/rollback path. The existing CI checks source and builds; it does not yet publish a verified release.
