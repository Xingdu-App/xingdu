CREATE TABLE api_keys (
 id text PRIMARY KEY CHECK(id ~ '^key_[0-9a-f]{32}$'),
 organization_id text NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
 created_by text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 name text NOT NULL CHECK(length(name) BETWEEN 1 AND 64),
 token_hash text NOT NULL UNIQUE CHECK(token_hash ~ '^[0-9a-f]{64}$'),
 prefix text NOT NULL,
 scopes text[] NOT NULL CHECK(cardinality(scopes)>0 AND scopes <@ ARRAY['hosts:read','hosts:write','nodes:read','nodes:write','nodes:credentials']::text[]),
 created_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL,
 last_used_at timestamptz,
 revoked_at timestamptz,
 revoked_by text REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX api_keys_organization ON api_keys(organization_id);
ALTER TABLE api_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE api_keys FORCE ROW LEVEL SECURITY;
CREATE POLICY key_read ON api_keys FOR SELECT USING(
 (organization_id=request_org_id() AND tenant_role(organization_id) IN ('owner','admin'))
 OR token_hash=nullif(current_setting('app.api_key_hash',true),'')
);
CREATE POLICY key_create ON api_keys FOR INSERT WITH CHECK(
 organization_id=request_org_id() AND created_by=request_user_id()
 AND tenant_role(organization_id) IN ('owner','admin')
);
CREATE POLICY key_update ON api_keys FOR UPDATE USING(
 organization_id=request_org_id() AND tenant_role(organization_id) IN ('owner','admin')
) WITH CHECK(organization_id=request_org_id());
GRANT SELECT,INSERT ON api_keys TO xingdu_app;
GRANT UPDATE(last_used_at,revoked_at,revoked_by) ON api_keys TO xingdu_app;
