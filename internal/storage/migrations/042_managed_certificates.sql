CREATE TABLE managed_certificates (
 id text PRIMARY KEY CHECK(id ~ '^cert_[0-9a-f]{32}$'),
 organization_id text NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
 created_by text NOT NULL REFERENCES users(id),
 domain text NOT NULL UNIQUE CHECK(length(domain) BETWEEN 3 AND 253),
 validation_target text NOT NULL UNIQUE,
 platform boolean NOT NULL DEFAULT false,
 host_id text,
 FOREIGN KEY(organization_id,host_id) REFERENCES hosts(organization_id,id) ON DELETE SET NULL (host_id),
 state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','queued','issuing','issued','failed','paused','deleted')),
 directory text NOT NULL DEFAULT '', encrypted bytea, expires_at timestamptz,
 error_code text NOT NULL DEFAULT '',
 lease text, lease_until timestamptz, attempts integer NOT NULL DEFAULT 0,
 next_attempt_at timestamptz NOT NULL DEFAULT now(),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(organization_id,id)
);
ALTER TABLE managed_certificates ENABLE ROW LEVEL SECURITY;
ALTER TABLE managed_certificates FORCE ROW LEVEL SECURITY;
CREATE POLICY certificate_read ON managed_certificates FOR SELECT USING(organization_id=request_org_id() AND tenant_role(organization_id) IS NOT NULL);
CREATE POLICY certificate_manage ON managed_certificates FOR ALL USING(organization_id=request_org_id() AND tenant_role(organization_id) IN ('owner','admin')) WITH CHECK(organization_id=request_org_id() AND tenant_role(organization_id) IN ('owner','admin'));
GRANT SELECT,INSERT,UPDATE ON managed_certificates TO xingdu_app;
CREATE TABLE certificate_audit (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 organization_id text NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
 certificate_id text NOT NULL,
 actor_id text NOT NULL REFERENCES users(id),
 event text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE certificate_audit ENABLE ROW LEVEL SECURITY;
ALTER TABLE certificate_audit FORCE ROW LEVEL SECURITY;
CREATE POLICY certificate_audit_read ON certificate_audit FOR SELECT USING(organization_id=request_org_id() AND tenant_role(organization_id) IN ('owner','admin'));
CREATE POLICY certificate_audit_insert ON certificate_audit FOR INSERT WITH CHECK(organization_id=request_org_id() AND actor_id=request_user_id() AND tenant_role(organization_id) IN ('owner','admin'));
GRANT SELECT,INSERT ON certificate_audit TO xingdu_app;
GRANT USAGE ON SEQUENCE certificate_audit_id_seq TO xingdu_app;

-- Discovery returns only job identities. Business reads and writes still use
-- transaction-local tenant scope and the caller's current membership.
DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='xingdu_certificate_scheduler') THEN CREATE ROLE xingdu_certificate_scheduler NOLOGIN NOSUPERUSER NOBYPASSRLS; END IF; END $$;
GRANT EXECUTE ON FUNCTION tenant_role(text),request_org_id() TO xingdu_certificate_scheduler;
GRANT USAGE ON SCHEMA public TO xingdu_certificate_scheduler;
GRANT SELECT(id,organization_id,created_by,state,next_attempt_at,lease_until,expires_at,updated_at) ON managed_certificates TO xingdu_certificate_scheduler;
GRANT SELECT(organization_id,user_id,role) ON memberships TO xingdu_certificate_scheduler;
CREATE POLICY certificate_discovery ON managed_certificates FOR SELECT TO xingdu_certificate_scheduler USING(true);
CREATE POLICY certificate_owner_discovery ON memberships FOR SELECT TO xingdu_certificate_scheduler USING(role='owner');
CREATE FUNCTION pending_certificate_jobs() RETURNS TABLE(id text,organization_id text,created_by text)
LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT c.id,c.organization_id,m.user_id FROM public.managed_certificates c
 JOIN public.memberships m ON m.organization_id=c.organization_id AND m.role='owner'
 WHERE ((c.state='queued' OR (c.state='issuing' AND c.lease_until<now())) AND c.next_attempt_at<=now())
 OR (c.state IN ('issued','paused') AND c.expires_at<now()+interval '30 days' AND c.next_attempt_at<=now())
 ORDER BY c.next_attempt_at,c.updated_at LIMIT 100;
$$;
ALTER FUNCTION pending_certificate_jobs() OWNER TO xingdu_certificate_scheduler;
REVOKE ALL ON FUNCTION pending_certificate_jobs() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION pending_certificate_jobs() TO xingdu_app;

ALTER TABLE api_keys DROP CONSTRAINT api_keys_scopes_check;
ALTER TABLE api_keys ADD CONSTRAINT api_keys_scopes_check CHECK(cardinality(scopes)>0 AND scopes <@ ARRAY['hosts:read','hosts:write','nodes:read','nodes:write','nodes:credentials','nodes:probe','subscriptions:read','subscriptions:write','subscriptions:export','certificates:read','certificates:write']::text[]);
