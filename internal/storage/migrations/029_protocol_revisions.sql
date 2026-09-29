ALTER TABLE protocol_deployments DROP CONSTRAINT protocol_deployments_action_check;
ALTER TABLE protocol_deployments ADD CONSTRAINT protocol_deployments_action_check CHECK(action IN ('deploy','remove','restart','update'));
ALTER TABLE protocol_deployments ADD COLUMN revision integer NOT NULL DEFAULT 1;
ALTER TABLE protocol_deployments ADD COLUMN pending_revision integer;
CREATE TABLE protocol_revisions (
 organization_id text NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
 node_id text NOT NULL,
 revision integer NOT NULL CHECK(revision>0),
 name text NOT NULL, server_name text NOT NULL, certificate_expires_at timestamptz,
 encrypted bytea NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(organization_id,node_id) REFERENCES protocol_deployments(organization_id,id) ON DELETE CASCADE,
 PRIMARY KEY(node_id,revision)
);
ALTER TABLE protocol_revisions ENABLE ROW LEVEL SECURITY;
ALTER TABLE protocol_revisions FORCE ROW LEVEL SECURITY;
CREATE POLICY revision_manage ON protocol_revisions FOR ALL
 USING(organization_id=request_org_id() AND tenant_role(organization_id) IN ('owner','admin'))
 WITH CHECK(organization_id=request_org_id() AND tenant_role(organization_id) IN ('owner','admin') AND EXISTS(SELECT 1 FROM protocol_deployments d WHERE d.id=node_id AND d.organization_id=protocol_revisions.organization_id));
CREATE POLICY revision_agent ON protocol_revisions FOR SELECT USING(organization_id=request_org_id() AND EXISTS(SELECT 1 FROM protocol_deployments d JOIN machine_agents a ON a.host_id=d.host_id AND a.organization_id=d.organization_id WHERE d.id=node_id AND d.organization_id=protocol_revisions.organization_id AND a.token_hash=nullif(current_setting('app.agent_hash',true),'') AND a.revoked_at IS NULL AND a.mode='manage'));
GRANT SELECT,INSERT,UPDATE,DELETE ON protocol_revisions TO xingdu_app;
