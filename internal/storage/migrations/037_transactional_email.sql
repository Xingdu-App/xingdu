-- Authentication recovery data is global account data, like registration challenges.
CREATE TABLE password_recoveries (
 token_hash text PRIMARY KEY CHECK(length(token_hash)=64), email text NOT NULL,
 code_hash text NOT NULL CHECK(length(code_hash)=64), password_snapshot text NOT NULL DEFAULT '',
 attempts integer NOT NULL DEFAULT 0 CHECK(attempts BETWEEN 0 AND 5),
 delivered boolean NOT NULL DEFAULT false, consumed_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(), expires_at timestamptz NOT NULL DEFAULT now()+interval '10 minutes'
);
CREATE INDEX password_recoveries_email ON password_recoveries(email,created_at);
GRANT SELECT,INSERT,UPDATE,DELETE ON password_recoveries TO xingdu_app;

-- A non-login service role owns only narrowly scoped mail/recovery functions.
DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='xingdu_mail') THEN
 CREATE ROLE xingdu_mail NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
END IF; END $$;
GRANT USAGE ON SCHEMA public TO xingdu_mail;
GRANT SELECT ON users TO xingdu_mail;
GRANT UPDATE(password_hash) ON users TO xingdu_mail;
GRANT SELECT,DELETE ON sessions TO xingdu_mail;
GRANT SELECT,UPDATE ON password_recoveries TO xingdu_mail;

CREATE FUNCTION complete_password_recovery(challenge text, supplied_code text, replacement text)
RETURNS boolean LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE p password_recoveries%ROWTYPE; account text; address text;
BEGIN
 SELECT email INTO address FROM password_recoveries WHERE token_hash=challenge;
 IF address IS NULL THEN RETURN false; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended('recovery:' || address,0));
 SELECT * INTO p FROM password_recoveries WHERE token_hash=challenge FOR UPDATE;
 IF NOT p.delivered OR p.consumed_at IS NOT NULL OR p.expires_at<=now() OR p.attempts>=5 THEN RETURN false; END IF;
 UPDATE password_recoveries SET attempts=attempts+1 WHERE token_hash=challenge;
 IF p.code_hash<>supplied_code OR replacement !~ '^\$2[aby]\$[0-9]{2}\$.{53}$' THEN RETURN false; END IF;
 SELECT id INTO account FROM users WHERE lower(email)=p.email AND email_verified_at IS NOT NULL AND password_login_enabled;
 IF account IS NULL THEN RETURN false; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended('account:' || account,0));
 UPDATE users SET password_hash=replacement WHERE id=account AND password_login_enabled AND password_hash=p.password_snapshot;
 IF NOT FOUND THEN RETURN false; END IF;
 DELETE FROM sessions WHERE user_id=account;
 UPDATE password_recoveries SET consumed_at=now() WHERE email=address AND consumed_at IS NULL;
 RETURN true;
END $$;
ALTER FUNCTION complete_password_recovery(text,text,text) OWNER TO xingdu_mail;
REVOKE ALL ON FUNCTION complete_password_recovery(text,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION complete_password_recovery(text,text,text) TO xingdu_app;

-- Only fixed database events can enqueue notifications. No runtime generic send queue.
CREATE TABLE email_deliveries (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 recipient text NOT NULL, kind text NOT NULL, payload jsonb NOT NULL DEFAULT '{}',
 created_at timestamptz NOT NULL DEFAULT now(), available_at timestamptz NOT NULL DEFAULT now(),
 lease_until timestamptz, lease_token text, attempts integer NOT NULL DEFAULT 0,
 sent_at timestamptz, failed_at timestamptz
);
ALTER TABLE email_deliveries ENABLE ROW LEVEL SECURITY;
ALTER TABLE email_deliveries FORCE ROW LEVEL SECURITY;
CREATE POLICY mail_delivery_service ON email_deliveries TO xingdu_mail USING(true) WITH CHECK(true);
GRANT SELECT,INSERT,UPDATE,DELETE ON email_deliveries TO xingdu_mail;
GRANT USAGE ON SEQUENCE email_deliveries_id_seq TO xingdu_mail;
CREATE INDEX email_deliveries_pending ON email_deliveries(available_at) WHERE sent_at IS NULL AND failed_at IS NULL;

CREATE FUNCTION account_email_event() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
BEGIN
 IF NEW.email IS NULL OR NEW.email_verified_at IS NULL THEN RETURN NEW; END IF;
 IF TG_OP='INSERT' THEN
  INSERT INTO email_deliveries(recipient,kind) VALUES(NEW.email,'welcome');
 ELSIF NEW.password_hash IS DISTINCT FROM OLD.password_hash AND NEW.password_login_enabled THEN
  INSERT INTO email_deliveries(recipient,kind) VALUES(NEW.email,'password_changed');
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION account_email_event() OWNER TO xingdu_mail;
REVOKE ALL ON FUNCTION account_email_event() FROM PUBLIC;
CREATE TRIGGER account_email_event AFTER INSERT OR UPDATE OF password_hash ON users FOR EACH ROW EXECUTE FUNCTION account_email_event();

-- Billing trigger sets transaction-local organization scope for its non-owner role.
GRANT SELECT ON memberships,organizations TO xingdu_mail;
GRANT EXECUTE ON FUNCTION tenant_role(text) TO xingdu_mail;
CREATE POLICY mail_member_read ON memberships FOR SELECT TO xingdu_mail USING(organization_id=current_setting('app.mail_org',true));
CREATE POLICY mail_organization_read ON organizations FOR SELECT TO xingdu_mail USING(id=current_setting('app.mail_org',true));
CREATE FUNCTION billing_email_event() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE recipient_email text; organization_name text; event_kind text; previous_scope text;
BEGIN
 IF (NEW.status,NEW.plan,NEW.cancel_at_period_end) IS NOT DISTINCT FROM (OLD.status,OLD.plan,OLD.cancel_at_period_end) THEN RETURN NEW; END IF;
 IF NEW.status='none' THEN RETURN NEW; END IF;
 previous_scope:=current_setting('app.mail_org',true);
 PERFORM set_config('app.mail_org',NEW.organization_id,true);
 SELECT u.email,o.name INTO recipient_email,organization_name FROM memberships m JOIN users u ON u.id=m.user_id JOIN organizations o ON o.id=m.organization_id
 WHERE m.organization_id=NEW.organization_id AND m.role='owner' AND u.email_verified_at IS NOT NULL;
 PERFORM set_config('app.mail_org',coalesce(previous_scope,''),true);
 IF recipient_email IS NULL THEN RETURN NEW; END IF;
 event_kind:=CASE WHEN NEW.status IN ('past_due','unpaid','incomplete') THEN 'billing_attention'
 WHEN NEW.status IN ('canceled','incomplete_expired') THEN 'billing_ended'
 WHEN NEW.cancel_at_period_end THEN 'billing_canceling'
 WHEN NEW.status IN ('active','trialing') AND OLD.status NOT IN ('active','trialing') THEN 'billing_active'
 ELSE 'billing_changed' END;
 INSERT INTO email_deliveries(recipient,kind,payload) VALUES(recipient_email,event_kind,jsonb_build_object('Organization',organization_name,'Plan',NEW.plan,'Status',NEW.status,'PeriodEnd',CASE WHEN NEW.period_end>0 THEN to_char(to_timestamp(NEW.period_end) AT TIME ZONE 'UTC','YYYY-MM-DD HH24:MI') || ' UTC' ELSE '' END,'organization_id',NEW.organization_id));
 RETURN NEW;
END $$;
ALTER FUNCTION billing_email_event() OWNER TO xingdu_mail;
REVOKE ALL ON FUNCTION billing_email_event() FROM PUBLIC;
CREATE TRIGGER billing_email_event AFTER UPDATE ON organization_billing FOR EACH ROW EXECUTE FUNCTION billing_email_event();

CREATE FUNCTION claim_email_delivery() RETURNS SETOF email_deliveries LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
BEGIN
 DELETE FROM email_deliveries WHERE created_at<now()-interval '30 days';
 UPDATE email_deliveries SET failed_at=now() WHERE sent_at IS NULL AND failed_at IS NULL AND (attempts>=10 OR created_at<now()-interval '20 hours') AND (lease_until IS NULL OR lease_until<now());
 RETURN QUERY UPDATE email_deliveries SET attempts=attempts+1,lease_until=now()+interval '1 minute',lease_token=gen_random_uuid()::text
 WHERE id=(SELECT id FROM email_deliveries WHERE sent_at IS NULL AND failed_at IS NULL AND available_at<=now() AND (lease_until IS NULL OR lease_until<now()) ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING *;
END $$;
CREATE FUNCTION finish_email_delivery(delivery bigint, lease text, success boolean) RETURNS void LANGUAGE sql SECURITY DEFINER SET search_path=public,pg_temp AS $$
 UPDATE email_deliveries SET sent_at=CASE WHEN success THEN now() ELSE NULL END, available_at=now()+make_interval(secs=>LEAST(3600,30*power(2,attempts)::integer)),lease_until=NULL,lease_token=NULL
 WHERE id=delivery AND lease_token=lease AND sent_at IS NULL;
$$;
ALTER FUNCTION claim_email_delivery() OWNER TO xingdu_mail;
ALTER FUNCTION finish_email_delivery(bigint,text,boolean) OWNER TO xingdu_mail;
REVOKE ALL ON FUNCTION claim_email_delivery(),finish_email_delivery(bigint,text,boolean) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claim_email_delivery(),finish_email_delivery(bigint,text,boolean) TO xingdu_app;
