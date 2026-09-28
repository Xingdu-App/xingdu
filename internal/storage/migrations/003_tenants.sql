ALTER TABLE admins RENAME TO users;
ALTER TABLE users DROP COLUMN singleton;
ALTER TABLE sessions RENAME COLUMN admin_id TO user_id;
CREATE TABLE organizations (
 id uuid PRIMARY KEY, name text NOT NULL CHECK(length(name) BETWEEN 1 AND 64),
 created_by uuid NOT NULL REFERENCES users(id), created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE memberships (
 organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 role text NOT NULL CHECK(role IN ('owner','admin','member','viewer')),
 created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(organization_id,user_id)
);
CREATE UNIQUE INDEX one_owner_per_org ON memberships(organization_id) WHERE role='owner';
CREATE INDEX memberships_user_idx ON memberships(user_id, organization_id);
CREATE TABLE invitations (
 id uuid PRIMARY KEY, organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
 token_hash text NOT NULL UNIQUE, role text NOT NULL CHECK(role IN ('admin','member','viewer')),
 created_by uuid NOT NULL REFERENCES users(id), expires_at timestamptz NOT NULL,
 accepted_at timestamptz, revoked_at timestamptz, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX invitations_org_idx ON invitations(organization_id,created_at);
-- Preserve the original administrator and inventory in one organization.
INSERT INTO organizations(id,name,created_by)
 SELECT '00000000-0000-4000-8000-000000000001','默认组织',id FROM users ORDER BY created_at LIMIT 1;
INSERT INTO memberships(organization_id,user_id,role)
 SELECT id,created_by,'owner' FROM organizations;
ALTER TABLE hosts ADD COLUMN organization_id uuid REFERENCES organizations(id) ON DELETE CASCADE;
UPDATE hosts SET organization_id='00000000-0000-4000-8000-000000000001';
-- Abort rather than silently discard legacy inventory without an administrator.
ALTER TABLE hosts ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE hosts ALTER COLUMN organization_id SET DEFAULT nullif(current_setting('app.organization_id',true),'')::uuid;
DROP INDEX hosts_address_port_idx;
CREATE UNIQUE INDEX hosts_address_port_idx ON hosts(organization_id,lower(address),ssh_port) WHERE address<>'';
CREATE INDEX hosts_org_idx ON hosts(organization_id,created_at);
CREATE FUNCTION request_user_id() RETURNS uuid LANGUAGE sql STABLE AS $$ SELECT nullif(current_setting('app.user_id',true),'')::uuid $$;
CREATE FUNCTION request_org_id() RETURNS uuid LANGUAGE sql STABLE AS $$ SELECT nullif(current_setting('app.organization_id',true),'')::uuid $$;
ALTER TABLE organizations ENABLE ROW LEVEL SECURITY;
ALTER TABLE organizations FORCE ROW LEVEL SECURITY;
ALTER TABLE memberships ENABLE ROW LEVEL SECURITY;
ALTER TABLE memberships FORCE ROW LEVEL SECURITY;
ALTER TABLE invitations ENABLE ROW LEVEL SECURITY;
ALTER TABLE invitations FORCE ROW LEVEL SECURITY;
ALTER TABLE hosts ENABLE ROW LEVEL SECURITY;
ALTER TABLE hosts FORCE ROW LEVEL SECURITY;
CREATE POLICY organization_read ON organizations FOR SELECT USING (
 created_by=request_user_id() OR EXISTS(SELECT 1 FROM memberships m WHERE m.organization_id=id AND m.user_id=request_user_id())
);
CREATE POLICY organization_create ON organizations FOR INSERT WITH CHECK(created_by=request_user_id());
CREATE POLICY membership_read ON memberships FOR SELECT USING(user_id=request_user_id() OR organization_id=request_org_id());
CREATE POLICY membership_insert ON memberships FOR INSERT WITH CHECK(organization_id=request_org_id());
CREATE POLICY membership_update ON memberships FOR UPDATE USING(organization_id=request_org_id()) WITH CHECK(organization_id=request_org_id());
CREATE POLICY membership_delete ON memberships FOR DELETE USING(organization_id=request_org_id());
CREATE POLICY invitation_scope ON invitations USING (
 organization_id=request_org_id() OR token_hash=nullif(current_setting('app.invitation_hash',true),'')
) WITH CHECK(organization_id=request_org_id());
CREATE POLICY host_read ON hosts FOR SELECT USING(organization_id=request_org_id() AND EXISTS(
 SELECT 1 FROM memberships m WHERE m.organization_id=hosts.organization_id AND m.user_id=request_user_id()
));
CREATE POLICY host_write ON hosts FOR ALL USING(organization_id=request_org_id() AND EXISTS(
 SELECT 1 FROM memberships m WHERE m.organization_id=hosts.organization_id AND m.user_id=request_user_id() AND m.role<>'viewer'
)) WITH CHECK(organization_id=request_org_id() AND EXISTS(
 SELECT 1 FROM memberships m WHERE m.organization_id=hosts.organization_id AND m.user_id=request_user_id() AND m.role<>'viewer'
));
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='xingdu_app') THEN
  CREATE ROLE xingdu_app NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
 END IF;
END $$;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO xingdu_app;
GRANT SELECT ON schema_migrations TO xingdu_app;
GRANT SELECT,INSERT ON users,organizations TO xingdu_app;
GRANT SELECT,INSERT,UPDATE,DELETE ON sessions,memberships,invitations,hosts TO xingdu_app;
