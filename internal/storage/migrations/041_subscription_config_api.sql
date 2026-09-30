ALTER TABLE subscriptions ADD COLUMN revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0);
CREATE FUNCTION advance_subscription_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 NEW.revision := OLD.revision + 1;
 RETURN NEW;
END;
$$;
CREATE TRIGGER subscription_revision BEFORE UPDATE ON subscriptions FOR EACH ROW EXECUTE FUNCTION advance_subscription_revision();
ALTER TABLE api_keys DROP CONSTRAINT api_keys_scopes_check;
ALTER TABLE api_keys ADD CONSTRAINT api_keys_scopes_check CHECK(cardinality(scopes)>0 AND scopes <@ ARRAY['hosts:read','hosts:write','nodes:read','nodes:write','nodes:credentials','nodes:probe','subscriptions:read','subscriptions:write','subscriptions:export']::text[]);
