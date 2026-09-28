CREATE TABLE organization_billing (
 organization_id uuid PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
 customer_id text UNIQUE,
 attempt text NOT NULL,
 checkout_id text NOT NULL DEFAULT '',
 checkout_interval text NOT NULL DEFAULT '',
 subscription_id text NOT NULL DEFAULT '',
 status text NOT NULL DEFAULT 'none',
 interval text NOT NULL DEFAULT '',
 period_end bigint NOT NULL DEFAULT 0,
 cancel_at_period_end boolean NOT NULL DEFAULT false,
 updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE organization_billing ENABLE ROW LEVEL SECURITY;
ALTER TABLE organization_billing FORCE ROW LEVEL SECURITY;
CREATE POLICY billing_read ON organization_billing FOR SELECT USING(organization_id=request_org_id() AND tenant_role(organization_id) IS NOT NULL);
CREATE POLICY billing_owner ON organization_billing FOR ALL USING(organization_id=request_org_id() AND tenant_role(organization_id)='owner') WITH CHECK(organization_id=request_org_id() AND tenant_role(organization_id)='owner');
-- Verified webhook handling sets this transaction-local customer scope. It never
-- accepts an organization or user scope from the webhook metadata.
CREATE POLICY billing_webhook ON organization_billing FOR ALL
 USING(customer_id=nullif(current_setting('app.billing_customer',true),''))
 WITH CHECK(customer_id=nullif(current_setting('app.billing_customer',true),''));
GRANT SELECT,INSERT,UPDATE ON organization_billing TO xingdu_app;
CREATE TABLE billing_events (
 id text PRIMARY KEY,
 customer_id text NOT NULL REFERENCES organization_billing(customer_id) ON DELETE CASCADE,
 processed_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE billing_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE billing_events FORCE ROW LEVEL SECURITY;
CREATE POLICY event_customer ON billing_events FOR ALL
 USING(customer_id=nullif(current_setting('app.billing_customer',true),''))
 WITH CHECK(customer_id=nullif(current_setting('app.billing_customer',true),''));
GRANT SELECT,INSERT ON billing_events TO xingdu_app;

CREATE FUNCTION enforce_cloud_entitlement() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public AS $$
DECLARE entitled boolean; used bigint;
BEGIN
 IF current_setting('app.billing_mode',true) IS DISTINCT FROM 'cloud' THEN RETURN NEW; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended(NEW.organization_id::text,0));
 SELECT status='active' AND period_end>extract(epoch FROM now()) INTO entitled FROM public.organization_billing WHERE organization_id=NEW.organization_id;
 IF NOT coalesce(entitled,false) THEN RAISE EXCEPTION 'cloud subscription required' USING ERRCODE='P0005'; END IF;
 IF TG_TABLE_NAME='hosts' THEN
  SELECT count(*) INTO used FROM public.hosts WHERE organization_id=NEW.organization_id;
  IF used>=5 THEN RAISE EXCEPTION 'organization quota exceeded' USING ERRCODE='P0004'; END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER hosts_billing BEFORE INSERT ON hosts FOR EACH ROW EXECUTE FUNCTION enforce_cloud_entitlement();
CREATE TRIGGER deployments_billing BEFORE INSERT ON protocol_deployments FOR EACH ROW EXECUTE FUNCTION enforce_cloud_entitlement();
CREATE TRIGGER subscriptions_billing BEFORE INSERT ON subscriptions FOR EACH ROW EXECUTE FUNCTION enforce_cloud_entitlement();
