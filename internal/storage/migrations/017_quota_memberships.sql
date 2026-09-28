-- Invite acceptance inserts a not-yet-member, so invoker RLS cannot count the
-- existing members. This non-login reader remains subject to scoped RLS and
-- has no write grants; ordinary membership INSERT policies still apply.
DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='xingdu_quota') THEN
 CREATE ROLE xingdu_quota NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
END IF; END $$;
GRANT USAGE ON SCHEMA public TO xingdu_quota;
GRANT SELECT ON memberships,organization_limits TO xingdu_quota;
GRANT EXECUTE ON FUNCTION tenant_role(uuid) TO xingdu_quota;
CREATE POLICY quota_members_read ON memberships FOR SELECT TO xingdu_quota
 USING(organization_id=nullif(current_setting('app.quota_org_id',true),'')::uuid);
CREATE POLICY quota_limits_read ON organization_limits FOR SELECT TO xingdu_quota
 USING(organization_id=nullif(current_setting('app.quota_org_id',true),'')::uuid);
CREATE FUNCTION enforce_membership_quota() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE used bigint; maximum integer; previous_scope text;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtextextended(NEW.organization_id::text,0));
 previous_scope:=current_setting('app.quota_org_id',true);
 PERFORM set_config('app.quota_org_id',NEW.organization_id::text,true);
 SELECT count(*) INTO used FROM public.memberships WHERE organization_id=NEW.organization_id;
 SELECT members INTO maximum FROM public.organization_limits WHERE organization_id=NEW.organization_id;
 PERFORM set_config('app.quota_org_id',coalesce(previous_scope,''),true);
 IF used>=coalesce(maximum,20) THEN RAISE EXCEPTION 'organization quota exceeded' USING ERRCODE='P0004'; END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION enforce_membership_quota() OWNER TO xingdu_quota;
REVOKE ALL ON FUNCTION enforce_membership_quota() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION enforce_membership_quota() TO xingdu_app;
DROP TRIGGER members_quota ON memberships;
CREATE TRIGGER members_quota BEFORE INSERT ON memberships FOR EACH ROW EXECUTE FUNCTION enforce_membership_quota();
