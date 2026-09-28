CREATE TABLE subscriptions (
 id uuid PRIMARY KEY, organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
 name text NOT NULL, rules jsonb NOT NULL DEFAULT '[]', final_action text NOT NULL CHECK(final_action IN ('proxy','direct')),
 enabled boolean NOT NULL DEFAULT true, token_hash text NOT NULL UNIQUE,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(organization_id,id)
);
ALTER TABLE protocol_deployments ADD CONSTRAINT deployment_org_id UNIQUE(organization_id,id);
CREATE TABLE subscription_nodes (
 organization_id uuid NOT NULL, subscription_id uuid NOT NULL, node_id uuid NOT NULL, position integer NOT NULL,
 PRIMARY KEY(subscription_id,node_id),
 FOREIGN KEY(organization_id,subscription_id) REFERENCES subscriptions(organization_id,id) ON DELETE CASCADE,
 FOREIGN KEY(organization_id,node_id) REFERENCES protocol_deployments(organization_id,id) ON DELETE CASCADE
);
CREATE TABLE subscription_audit (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
 subscription_id uuid NOT NULL, actor_id uuid NOT NULL REFERENCES users(id), event text NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE subscriptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE subscriptions FORCE ROW LEVEL SECURITY;
ALTER TABLE subscription_nodes ENABLE ROW LEVEL SECURITY;
ALTER TABLE subscription_nodes FORCE ROW LEVEL SECURITY;
ALTER TABLE subscription_audit ENABLE ROW LEVEL SECURITY;
ALTER TABLE subscription_audit FORCE ROW LEVEL SECURITY;
CREATE POLICY subscription_read ON subscriptions FOR SELECT USING(organization_id=request_org_id() AND tenant_role(organization_id) IS NOT NULL);
CREATE POLICY subscription_manage ON subscriptions FOR ALL USING(organization_id=request_org_id() AND tenant_role(organization_id) IN ('owner','admin')) WITH CHECK(organization_id=request_org_id() AND tenant_role(organization_id) IN ('owner','admin'));
CREATE POLICY subscription_token_read ON subscriptions FOR SELECT USING(organization_id=request_org_id() AND enabled AND id::text=current_setting('app.subscription_id',true) AND token_hash=current_setting('app.subscription_hash',true));
CREATE POLICY subscription_nodes_read ON subscription_nodes FOR SELECT USING(organization_id=request_org_id() AND EXISTS(SELECT 1 FROM subscriptions s WHERE s.id=subscription_id));
CREATE POLICY subscription_nodes_manage ON subscription_nodes FOR ALL USING(organization_id=request_org_id() AND tenant_role(organization_id) IN ('owner','admin')) WITH CHECK(organization_id=request_org_id() AND tenant_role(organization_id) IN ('owner','admin'));
CREATE POLICY subscription_audit_manage ON subscription_audit FOR ALL USING(organization_id=request_org_id() AND tenant_role(organization_id) IN ('owner','admin')) WITH CHECK(organization_id=request_org_id() AND tenant_role(organization_id) IN ('owner','admin'));
CREATE POLICY subscription_deployment_read ON protocol_deployments FOR SELECT USING(organization_id=request_org_id() AND EXISTS(SELECT 1 FROM subscription_nodes n WHERE n.node_id=protocol_deployments.id AND n.subscription_id::text=current_setting('app.subscription_id',true)));
CREATE POLICY subscription_host_read ON hosts FOR SELECT USING(organization_id=request_org_id() AND EXISTS(SELECT 1 FROM protocol_deployments d JOIN subscription_nodes n ON n.node_id=d.id WHERE d.host_id=hosts.id AND n.subscription_id::text=current_setting('app.subscription_id',true)));
GRANT SELECT,INSERT,UPDATE,DELETE ON subscriptions,subscription_nodes,subscription_audit TO xingdu_app;
GRANT USAGE ON SEQUENCE subscription_audit_id_seq TO xingdu_app;
GRANT SELECT ON subscriptions TO xingdu_policy;
-- Authenticate the bearer capability before resolving its tenant. All content reads remain RLS-scoped.
CREATE FUNCTION subscription_context(subscription uuid,identity_hash text) RETURNS uuid
 LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT s.organization_id FROM public.subscriptions s WHERE s.id=subscription AND s.token_hash=identity_hash AND s.enabled;
$$;
ALTER FUNCTION subscription_context(uuid,text) OWNER TO xingdu_policy;
REVOKE ALL ON FUNCTION subscription_context(uuid,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION subscription_context(uuid,text) TO xingdu_app;
