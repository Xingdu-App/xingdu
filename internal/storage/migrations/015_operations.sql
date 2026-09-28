-- Limits are operator-managed, not editable by tenant administrators.
CREATE TABLE organization_limits (
 organization_id uuid PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
 hosts integer NOT NULL DEFAULT 25 CHECK(hosts>=0),
 deployments integer NOT NULL DEFAULT 100 CHECK(deployments>=0),
 subscriptions integer NOT NULL DEFAULT 100 CHECK(subscriptions>=0),
 members integer NOT NULL DEFAULT 20 CHECK(members>=1)
);
ALTER TABLE organization_limits ENABLE ROW LEVEL SECURITY;
ALTER TABLE organization_limits FORCE ROW LEVEL SECURITY;
CREATE POLICY limits_read ON organization_limits FOR SELECT USING(organization_id=request_org_id() AND tenant_role(organization_id) IS NOT NULL);
GRANT SELECT ON organization_limits TO xingdu_app;
-- Invoker privileges and tenant RLS apply. The same organization lock as tenantTx
-- makes checking and inserting atomic across API instances.
CREATE FUNCTION enforce_organization_quota() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public AS $$
DECLARE used bigint; maximum integer;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtextextended(NEW.organization_id::text,0));
 CASE TG_TABLE_NAME
 WHEN 'hosts' THEN
 SELECT count(*) INTO used FROM public.hosts WHERE organization_id=NEW.organization_id;
 SELECT hosts INTO maximum FROM public.organization_limits WHERE organization_id=NEW.organization_id;
 maximum:=coalesce(maximum,25);
 WHEN 'protocol_deployments' THEN
 SELECT count(*) INTO used FROM public.protocol_deployments WHERE organization_id=NEW.organization_id AND state NOT IN ('removed','cancelled');
 SELECT deployments INTO maximum FROM public.organization_limits WHERE organization_id=NEW.organization_id;
 maximum:=coalesce(maximum,100);
 WHEN 'subscriptions' THEN
 SELECT count(*) INTO used FROM public.subscriptions WHERE organization_id=NEW.organization_id;
 SELECT subscriptions INTO maximum FROM public.organization_limits WHERE organization_id=NEW.organization_id;
 maximum:=coalesce(maximum,100);
 WHEN 'memberships' THEN
 SELECT count(*) INTO used FROM public.memberships WHERE organization_id=NEW.organization_id;
 SELECT members INTO maximum FROM public.organization_limits WHERE organization_id=NEW.organization_id;
 maximum:=coalesce(maximum,20);
 END CASE;
 IF used>=maximum THEN RAISE EXCEPTION 'organization quota exceeded' USING ERRCODE='P0004'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER hosts_quota BEFORE INSERT ON hosts FOR EACH ROW EXECUTE FUNCTION enforce_organization_quota();
CREATE TRIGGER deployments_quota BEFORE INSERT ON protocol_deployments FOR EACH ROW EXECUTE FUNCTION enforce_organization_quota();
CREATE TRIGGER subscriptions_quota BEFORE INSERT ON subscriptions FOR EACH ROW EXECUTE FUNCTION enforce_organization_quota();
CREATE TRIGGER members_quota BEFORE INSERT ON memberships FOR EACH ROW EXECUTE FUNCTION enforce_organization_quota();
