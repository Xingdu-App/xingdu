-- Existing paid subscriptions keep their Start entitlement and price.
ALTER TABLE organization_billing ADD COLUMN plan text NOT NULL DEFAULT 'start' CHECK(plan IN ('start','premium'));
ALTER TABLE organization_billing ADD COLUMN checkout_plan text NOT NULL DEFAULT 'start' CHECK(checkout_plan IN ('start','premium'));

-- Invoker scope and existing RLS policies apply; absent or expired billing is free.
CREATE FUNCTION cloud_server_limit(org text) RETURNS integer
LANGUAGE sql STABLE SET search_path=pg_catalog,public AS $$
 SELECT coalesce((SELECT CASE WHEN status='active' AND period_end>extract(epoch FROM now())
 THEN CASE WHEN plan='premium' THEN 50 ELSE 10 END ELSE 1 END
 FROM public.organization_billing WHERE organization_id=org),1)
$$;
REVOKE ALL ON FUNCTION cloud_server_limit(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION cloud_server_limit(text) TO xingdu_app;

CREATE OR REPLACE FUNCTION enforce_cloud_entitlement() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public AS $$
DECLARE used bigint; host_limit integer;
BEGIN
 IF current_setting('app.billing_mode',true) IS DISTINCT FROM 'cloud' THEN RETURN NEW; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended(NEW.organization_id::text,0));
 host_limit := public.cloud_server_limit(NEW.organization_id);
 SELECT count(*) INTO used FROM public.hosts WHERE organization_id=NEW.organization_id;
 IF TG_TABLE_NAME='hosts' THEN
  IF used>=host_limit THEN RAISE EXCEPTION 'organization quota exceeded' USING ERRCODE='P0004'; END IF;
 ELSIF used>host_limit THEN
  RAISE EXCEPTION 'cloud server quota exceeded' USING ERRCODE='P0004';
 END IF;
 RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION enforce_organization_quota() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public AS $$
DECLARE used bigint; maximum integer;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtextextended(NEW.organization_id::text,0));
 CASE TG_TABLE_NAME
 WHEN 'hosts' THEN
 SELECT count(*) INTO used FROM public.hosts WHERE organization_id=NEW.organization_id;
 SELECT hosts INTO maximum FROM public.organization_limits WHERE organization_id=NEW.organization_id;
 maximum:=coalesce(maximum,CASE WHEN current_setting('app.billing_mode',true)='cloud' THEN public.cloud_server_limit(NEW.organization_id) ELSE 25 END);
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
