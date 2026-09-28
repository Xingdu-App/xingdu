-- A narrowly scoped, non-login policy owner avoids recursive membership RLS.
-- It can only SELECT memberships; the API cannot SET ROLE to it.
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='xingdu_policy') THEN
  CREATE ROLE xingdu_policy NOLOGIN NOSUPERUSER BYPASSRLS NOCREATEDB NOCREATEROLE;
 END IF;
END $$;
GRANT USAGE ON SCHEMA public TO xingdu_policy;
GRANT SELECT ON memberships TO xingdu_policy;
CREATE FUNCTION public.tenant_role(org uuid) RETURNS text
 LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT role FROM public.memberships WHERE organization_id=org
 AND user_id=nullif(current_setting('app.user_id',true),'')::uuid
$$;
ALTER FUNCTION public.tenant_role(uuid) OWNER TO xingdu_policy;
REVOKE ALL ON FUNCTION public.tenant_role(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.tenant_role(uuid) TO xingdu_app;
DROP POLICY membership_read ON memberships;
CREATE POLICY membership_read ON memberships FOR SELECT USING(
 user_id=request_user_id() OR (organization_id=request_org_id() AND tenant_role(organization_id) IS NOT NULL)
);
DROP POLICY membership_insert ON memberships;
CREATE POLICY membership_insert ON memberships FOR INSERT WITH CHECK(
 organization_id=request_org_id() AND (
  (tenant_role(organization_id)='owner' AND role<>'owner') OR
  (tenant_role(organization_id)='admin' AND role IN ('member','viewer')) OR
  (user_id=request_user_id() AND role='owner' AND EXISTS(
   SELECT 1 FROM organizations o WHERE o.id=organization_id AND o.created_by=request_user_id()
  )) OR
  (user_id=request_user_id() AND EXISTS(
   SELECT 1 FROM invitations i WHERE i.organization_id=memberships.organization_id AND i.role=memberships.role
   AND i.token_hash=nullif(current_setting('app.invitation_hash',true),'')
   AND i.expires_at>now() AND i.accepted_at IS NULL AND i.revoked_at IS NULL
  ))
 )
);
DROP POLICY membership_update ON memberships;
CREATE POLICY membership_update ON memberships FOR UPDATE USING(
 organization_id=request_org_id() AND role<>'owner' AND (tenant_role(organization_id)='owner' OR (tenant_role(organization_id)='admin' AND role<>'admin'))
) WITH CHECK(
 organization_id=request_org_id() AND role<>'owner' AND (tenant_role(organization_id)='owner' OR (tenant_role(organization_id)='admin' AND role<>'admin'))
);
DROP POLICY membership_delete ON memberships;
CREATE POLICY membership_delete ON memberships FOR DELETE USING(
 organization_id=request_org_id() AND role<>'owner' AND (tenant_role(organization_id)='owner' OR (tenant_role(organization_id)='admin' AND role<>'admin'))
);
DROP POLICY invitation_scope ON invitations;
CREATE POLICY invitation_read ON invitations FOR SELECT USING(
 (organization_id=request_org_id() AND tenant_role(organization_id) IN ('owner','admin')) OR token_hash=nullif(current_setting('app.invitation_hash',true),'')
);
CREATE POLICY invitation_create ON invitations FOR INSERT WITH CHECK(
 organization_id=request_org_id() AND created_by=request_user_id() AND (tenant_role(organization_id)='owner' OR (tenant_role(organization_id)='admin' AND role<>'admin'))
);
CREATE POLICY invitation_update ON invitations FOR UPDATE USING(
 (organization_id=request_org_id() AND (tenant_role(organization_id)='owner' OR (tenant_role(organization_id)='admin' AND role<>'admin'))) OR token_hash=nullif(current_setting('app.invitation_hash',true),'')
) WITH CHECK(organization_id=request_org_id());
-- App code may change roles and lifecycle timestamps, never move rows between tenants.
REVOKE UPDATE ON memberships,invitations FROM xingdu_app;
GRANT UPDATE(role) ON memberships TO xingdu_app;
GRANT UPDATE(accepted_at,revoked_at) ON invitations TO xingdu_app;
