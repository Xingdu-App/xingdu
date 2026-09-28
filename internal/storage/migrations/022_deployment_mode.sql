-- A private registry enforces instance-wide cardinality without exposing tenant
-- rows or weakening their RLS policies. Only the trigger writes this registry.
CREATE TABLE deployment_organizations AS SELECT id AS organization_id FROM organizations;
ALTER TABLE deployment_organizations ADD PRIMARY KEY (organization_id);
ALTER TABLE deployment_organizations ADD FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;
REVOKE ALL ON deployment_organizations FROM PUBLIC, xingdu_app;

CREATE FUNCTION deployment_organization_count() RETURNS bigint
LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
    SELECT count(*) FROM public.deployment_organizations
$$;
REVOKE ALL ON FUNCTION deployment_organization_count() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION deployment_organization_count() TO xingdu_app;

CREATE FUNCTION register_deployment_organization() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(94643822);
    IF current_setting('app.deployment_mode',true) = 'self_hosted'
       AND EXISTS(SELECT 1 FROM public.deployment_organizations) THEN
        RAISE EXCEPTION 'self-hosted instances support one organization' USING ERRCODE='P0006';
    END IF;
    INSERT INTO public.deployment_organizations VALUES(NEW.id);
    RETURN NEW;
END
$$;
REVOKE ALL ON FUNCTION register_deployment_organization() FROM PUBLIC;
CREATE TRIGGER deployment_organization_limit AFTER INSERT ON organizations
FOR EACH ROW EXECUTE FUNCTION register_deployment_organization();
