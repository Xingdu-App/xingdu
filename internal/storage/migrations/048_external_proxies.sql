CREATE TABLE external_proxies (
 id text PRIMARY KEY CHECK(id ~ '^ext_[0-9a-f]{32}$'),
 organization_id text NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
 name text NOT NULL CHECK(length(name) BETWEEN 1 AND 64),
 address text NOT NULL, port integer NOT NULL CHECK(port BETWEEN 1 AND 65535),
 protocol text NOT NULL CHECK(protocol IN ('socks','http')),
 encrypted bytea NOT NULL,
 revision integer NOT NULL DEFAULT 1,
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(organization_id,id)
);
ALTER TABLE external_proxies ENABLE ROW LEVEL SECURITY;
ALTER TABLE external_proxies FORCE ROW LEVEL SECURITY;
CREATE POLICY external_proxy_read ON external_proxies FOR SELECT USING(organization_id=request_org_id() AND tenant_role(organization_id) IS NOT NULL);
CREATE POLICY external_proxy_manage ON external_proxies FOR ALL USING(organization_id=request_org_id() AND tenant_role(organization_id) IN ('owner','admin')) WITH CHECK(organization_id=request_org_id() AND tenant_role(organization_id) IN ('owner','admin'));
GRANT SELECT,INSERT,UPDATE,DELETE ON external_proxies TO xingdu_app;
CREATE TABLE external_proxy_audit (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 organization_id text NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
 proxy_id text NOT NULL, actor_id text NOT NULL REFERENCES users(id),
 event text NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE external_proxy_audit ENABLE ROW LEVEL SECURITY;
ALTER TABLE external_proxy_audit FORCE ROW LEVEL SECURITY;
CREATE POLICY external_proxy_audit_read ON external_proxy_audit FOR SELECT USING(organization_id=request_org_id() AND tenant_role(organization_id) IN ('owner','admin'));
CREATE POLICY external_proxy_audit_insert ON external_proxy_audit FOR INSERT WITH CHECK(organization_id=request_org_id() AND actor_id=request_user_id() AND tenant_role(organization_id) IN ('owner','admin'));
GRANT SELECT,INSERT ON external_proxy_audit TO xingdu_app;
GRANT USAGE ON SEQUENCE external_proxy_audit_id_seq TO xingdu_app;
ALTER TABLE protocol_deployments ADD COLUMN external_exit_id text;
ALTER TABLE protocol_deployments ADD FOREIGN KEY(organization_id,external_exit_id) REFERENCES external_proxies(organization_id,id);
ALTER TABLE protocol_deployments ADD CHECK(relay_exit_id IS NULL OR external_exit_id IS NULL);
ALTER TABLE protocol_revisions ADD COLUMN external_exit_id text;
ALTER TABLE protocol_revisions ADD CHECK(relay_exit_id IS NULL OR external_exit_id IS NULL);
ALTER TABLE api_keys DROP CONSTRAINT api_keys_scopes_check;
ALTER TABLE api_keys ADD CONSTRAINT api_keys_scopes_check CHECK(cardinality(scopes)>0 AND scopes <@ ARRAY['hosts:read','hosts:write','nodes:read','nodes:write','nodes:credentials','nodes:probe','subscriptions:read','subscriptions:write','subscriptions:export','certificates:read','certificates:write','exits:read','exits:write']::text[]);
