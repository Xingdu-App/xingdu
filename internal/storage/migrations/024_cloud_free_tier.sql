-- Keep the free server usable for deployments and client subscriptions. After
-- expiry, preserve existing resources but block additions until within quota.
CREATE OR REPLACE FUNCTION enforce_cloud_entitlement() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public AS $$
DECLARE entitled boolean; used bigint; host_limit integer;
BEGIN
 IF current_setting('app.billing_mode',true) IS DISTINCT FROM 'cloud' THEN RETURN NEW; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended(NEW.organization_id::text,0));
 SELECT status='active' AND period_end>extract(epoch FROM now()) INTO entitled FROM public.organization_billing WHERE organization_id=NEW.organization_id;
 host_limit := CASE WHEN coalesce(entitled,false) THEN 10 ELSE 1 END;
 SELECT count(*) INTO used FROM public.hosts WHERE organization_id=NEW.organization_id;
 IF TG_TABLE_NAME='hosts' THEN
  IF used>=host_limit THEN RAISE EXCEPTION 'organization quota exceeded' USING ERRCODE='P0004'; END IF;
 ELSIF used>host_limit THEN
  RAISE EXCEPTION 'cloud server quota exceeded' USING ERRCODE='P0004';
 END IF;
 RETURN NEW;
END $$;
