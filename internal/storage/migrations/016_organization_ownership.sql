CREATE TABLE organization_audit (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
 actor_id uuid NOT NULL REFERENCES users(id), target_id uuid NOT NULL REFERENCES users(id),
 event text NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE organization_audit ENABLE ROW LEVEL SECURITY;
ALTER TABLE organization_audit FORCE ROW LEVEL SECURITY;
CREATE POLICY organization_audit_read ON organization_audit FOR SELECT TO xingdu_app
 USING(organization_id=request_org_id() AND tenant_role(organization_id) IN ('owner','admin'));
GRANT SELECT ON organization_audit TO xingdu_app;
-- Dedicated function role is non-login and cannot bypass RLS. The app cannot
-- assume it or directly mutate owner rows through ordinary membership APIs.
DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='xingdu_ownership') THEN
 CREATE ROLE xingdu_ownership NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
END IF; END $$;
GRANT USAGE ON SCHEMA public TO xingdu_ownership;
GRANT SELECT ON memberships,users,sessions TO xingdu_ownership;
GRANT UPDATE(role) ON memberships TO xingdu_ownership;
GRANT INSERT ON organization_audit TO xingdu_ownership;
GRANT USAGE ON SEQUENCE organization_audit_id_seq TO xingdu_ownership;
GRANT EXECUTE ON FUNCTION tenant_role(uuid) TO xingdu_ownership;
CREATE POLICY ownership_member_read ON memberships FOR SELECT TO xingdu_ownership USING(organization_id=request_org_id());
CREATE POLICY ownership_member_update ON memberships FOR UPDATE TO xingdu_ownership USING(organization_id=request_org_id()) WITH CHECK(organization_id=request_org_id());
CREATE POLICY ownership_audit_insert ON organization_audit FOR INSERT TO xingdu_ownership WITH CHECK(organization_id=request_org_id() AND actor_id=request_user_id());
CREATE FUNCTION transfer_organization_owner(target uuid, verified_hash text, session_hash text) RETURNS boolean
LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE org uuid := request_org_id(); actor uuid := request_user_id();
BEGIN
 IF org IS NULL OR actor IS NULL OR target=actor THEN RETURN false; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended(org::text,0));
 PERFORM pg_advisory_xact_lock(hashtextextended('account:' || actor::text,0));
 IF NOT EXISTS(SELECT 1 FROM memberships WHERE organization_id=org AND user_id=actor AND role='owner')
 OR NOT EXISTS(SELECT 1 FROM memberships WHERE organization_id=org AND user_id=target AND role<>'owner')
 OR NOT EXISTS(SELECT 1 FROM users WHERE id=actor AND password_hash=verified_hash)
 OR NOT EXISTS(SELECT 1 FROM sessions WHERE user_id=actor AND token_hash=session_hash AND expires_at>now()) THEN RETURN false; END IF;
 UPDATE memberships SET role='admin' WHERE organization_id=org AND user_id=actor;
 UPDATE memberships SET role='owner' WHERE organization_id=org AND user_id=target;
 INSERT INTO organization_audit(organization_id,actor_id,target_id,event) VALUES(org,actor,target,'ownership_transferred');
 RETURN true;
END $$;
ALTER FUNCTION transfer_organization_owner(uuid,text,text) OWNER TO xingdu_ownership;
REVOKE ALL ON FUNCTION transfer_organization_owner(uuid,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION transfer_organization_owner(uuid,text,text) TO xingdu_app;
