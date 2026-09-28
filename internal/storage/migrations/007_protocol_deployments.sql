CREATE TABLE protocol_deployments (
 id uuid PRIMARY KEY, organization_id uuid NOT NULL, host_id uuid NOT NULL,
 created_by uuid NOT NULL REFERENCES users(id), name text NOT NULL, protocol text NOT NULL CHECK(protocol IN ('trojan','vless','vmess','hysteria2','tuic')),
 port integer NOT NULL CHECK(port BETWEEN 1 AND 65535), server_name text NOT NULL,
 state text NOT NULL DEFAULT 'queued' CHECK(state IN ('queued','running','succeeded','failed','interrupted','cancelled','removed')),
 action text NOT NULL DEFAULT 'deploy' CHECK(action IN ('deploy','remove')),
 operation_id uuid NOT NULL UNIQUE, result text NOT NULL DEFAULT '', encrypted bytea,
 agent_hash text NOT NULL, lease uuid, lease_until timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(), queued_at timestamptz NOT NULL DEFAULT now(), finished_at timestamptz,
 FOREIGN KEY(organization_id,host_id) REFERENCES hosts(organization_id,id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX one_protocol_operation ON protocol_deployments(host_id) WHERE state IN ('queued','running');
-- Reserve both TCP and UDP conservatively, including uncertain/failed installations until removed.
CREATE UNIQUE INDEX protocol_port_reserved ON protocol_deployments(host_id,port) WHERE state NOT IN ('removed','cancelled');
ALTER TABLE protocol_deployments ENABLE ROW LEVEL SECURITY;
ALTER TABLE protocol_deployments FORCE ROW LEVEL SECURITY;
CREATE POLICY deployment_read ON protocol_deployments FOR SELECT USING(organization_id=request_org_id() AND tenant_role(organization_id) IS NOT NULL);
CREATE POLICY deployment_manage ON protocol_deployments FOR ALL USING(organization_id=request_org_id() AND tenant_role(organization_id) IN ('owner','admin')) WITH CHECK(organization_id=request_org_id() AND tenant_role(organization_id) IN ('owner','admin'));
CREATE POLICY deployment_agent ON protocol_deployments FOR ALL USING(organization_id=request_org_id() AND EXISTS(SELECT 1 FROM machine_agents a WHERE a.host_id=protocol_deployments.host_id AND a.organization_id=protocol_deployments.organization_id AND a.token_hash=nullif(current_setting('app.agent_hash',true),'') AND a.revoked_at IS NULL AND a.mode='manage')) WITH CHECK(organization_id=request_org_id() AND EXISTS(SELECT 1 FROM machine_agents a WHERE a.host_id=protocol_deployments.host_id AND a.organization_id=protocol_deployments.organization_id AND a.token_hash=nullif(current_setting('app.agent_hash',true),'') AND a.revoked_at IS NULL AND a.mode='manage'));
GRANT SELECT,INSERT,UPDATE,DELETE ON protocol_deployments TO xingdu_app;
GRANT SELECT ON machine_agents TO xingdu_policy;
-- Resolves only an active managing identity, never accepts caller-provided tenant IDs.
CREATE FUNCTION deployment_agent_context(identity_hash text) RETURNS TABLE(host_id uuid,organization_id uuid)
 LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT a.host_id,a.organization_id FROM public.machine_agents a WHERE a.token_hash=identity_hash AND a.revoked_at IS NULL AND a.mode='manage';
$$;
ALTER FUNCTION deployment_agent_context(text) OWNER TO xingdu_policy;
REVOKE ALL ON FUNCTION deployment_agent_context(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION deployment_agent_context(text) TO xingdu_app;
